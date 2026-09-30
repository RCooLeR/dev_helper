package provision

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/store"
	"devhelper/internal/templates"
)

type CreateRequest struct {
	Company string
	Project string
	Type    string
	Domain  string
	PHP     string
	DB      string

	// Optional: import SQL dump after project creation (host file path)
	ImportFile     string
	ImportOnCreate bool
}

type CreateResult struct {
	Project  store.Project
	Warnings []string
}

var safeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
var safeDB = regexp.MustCompile(`[^a-z0-9_]+`)

// sanitize turns user-facing company/project text into a filesystem and nginx
// friendly slug. It is intentionally stricter than "whatever the OS accepts":
// the same value is later reused in paths, nginx config names, log file names,
// and default domains, so one predictable alphabet keeps every downstream
// consumer simple.
func sanitize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = safeName.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "proj"
	}
	return s
}

func dbIdent(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = safeDB.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "db"
	}
	return s
}

// defaultDomain expands the user-configurable pattern. The config normally
// looks like "<project>.<company>.local"; keeping this as replacement text
// instead of fmt.Sprintf lets users reorder or omit either token.
func defaultDomain(pattern, company, project string) string {
	d := strings.ReplaceAll(pattern, "<company>", company)
	d = strings.ReplaceAll(d, "<project>", project)
	return d
}

func phpServiceName(php string) string {
	php = strings.TrimSpace(strings.TrimPrefix(php, "php"))
	php = strings.ReplaceAll(php, ".", "")
	return "php" + php
}

// Include both ownership components and a hash: punctuation, ambiguous
// separators, long names, and the same project in different companies must
// never silently reuse another project's database or login.
func projectDBNames(company, project string) (string, string) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(company+"\x00"+project)))[:12]
	prefix := dbIdent(company + "_" + project)
	name := prefix[:min(len(prefix), 50)] + "_" + hash
	user := prefix[:min(len(prefix), 19)] + "_" + hash
	return name, user
}

func Create(a *app.App, st *store.Store, req CreateRequest) (CreateResult, error) {
	return CreateContext(context.Background(), a, st, req)
}

func CreateContext(ctx context.Context, a *app.App, st *store.Store, req CreateRequest) (CreateResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}
	cfg := a.Cfg
	company, err := projectSlug(req.Company)
	if err != nil {
		return CreateResult{}, fmt.Errorf("company: %w", err)
	}
	project, err := projectSlug(req.Project)
	if err != nil {
		return CreateResult{}, fmt.Errorf("project: %w", err)
	}

	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	if domain == "" {
		domain = defaultDomain(cfg.DefaultDomainPattern, company, project)
	}
	if err := validateDomain(domain); err != nil {
		return CreateResult{}, err
	}
	if !slices.Contains([]string{"7.1", "7.2", "7.3", "7.4", "8.0", "8.1", "8.2", "8.3", "8.4"}, req.PHP) {
		return CreateResult{}, fmt.Errorf("unsupported PHP version %q", req.PHP)
	}
	db := normalizeDBEngine(req.DB)
	if db == "" {
		db = "none"
	}
	if db != "none" && !isSupportedDB(db) {
		return CreateResult{}, fmt.Errorf("unsupported database engine %q", req.DB)
	}
	var importDump *os.File
	if req.ImportOnCreate {
		if db == "none" {
			return CreateResult{}, fmt.Errorf("cannot import without a database")
		}
		importDump, err = openSQLDump(req.ImportFile)
		if err != nil {
			return CreateResult{}, fmt.Errorf("open import file: %w", err)
		}
		defer importDump.Close()
	}
	unlock, err := st.LockOperations(ctx)
	if err != nil {
		return CreateResult{}, err
	}
	defer unlock()
	projects, err := st.List()
	if err != nil {
		return CreateResult{}, err
	}
	for _, existing := range projects {
		if existing.Company == company && existing.Name == project {
			return CreateResult{}, fmt.Errorf("project %s/%s already exists", company, project)
		}
		if strings.EqualFold(existing.Domain, domain) {
			return CreateResult{}, fmt.Errorf("domain %s is already used by %s/%s", domain, existing.Company, existing.Name)
		}
		if existing.Company+"__"+existing.Name == company+"__"+project {
			return CreateResult{}, fmt.Errorf("nginx configuration name is already used by %s/%s", existing.Company, existing.Name)
		}
	}
	a.Log.Info().Str("company", company).Str("project", project).Str("domain", domain).Msg("Creating project")
	phpSvc := phpServiceName(req.PHP)
	phpUpstream := phpSvc + ":9000"

	// A selected database is required. Finish it before writing nginx, hosts,
	// certificates or successful metadata; a failed startup can then be retried.
	dbName, dbUser, dbPass := "", "", ""
	if db != "none" {
		dbName, dbUser = projectDBNames(company, project)
		dbPass = rand.Text()
		if err := createDBContext(ctx, cfg, db, dbName, dbUser, dbPass); err != nil {
			return CreateResult{}, fmt.Errorf("create %s database: %w", db, err)
		}
		a.Log.Info().Str("engine", db).Str("database", dbName).Msg("Created database")
	}
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}
	warnings := []string{}
	runtimeAppDir := filepath.ToSlash(filepath.Join(cfg.AppsRoot, company, project))
	hostAppDir := runtimeAppDir
	if runtime.GOOS == "windows" {
		hostAppDir = platform.WSLToHost(cfg, runtimeAppDir)
	}

	// Create the runtime project directory. On Windows this path points into
	// WSL through the \\wsl$ UNC mirror, because Docker Desktop performs much
	// better with Linux files than with bind mounts from NTFS.
	if err := os.MkdirAll(hostAppDir, 0o755); err != nil {
		return CreateResult{}, err
	}

	// Also create a tiny Windows-side mirror under projects/apps. It is not the
	// real app checkout; it is just a breadcrumb for editors/tools that start
	// from the repo and need to find the WSL path quickly.
	if runtime.GOOS == "windows" {
		winDir := filepath.Join(a.RepoRoot, "projects", "apps", company, project)
		if err := os.MkdirAll(winDir, 0o755); err != nil {
			warnings = append(warnings, "mirror: "+err.Error())
		} else if err := os.WriteFile(filepath.Join(winDir, ".wsl-path"), []byte(runtimeAppDir+"\n"), 0o644); err != nil {
			warnings = append(warnings, "mirror: "+err.Error())
		}
	}
	a.Log.Debug().Msg("Directories created")
	confName := fmt.Sprintf("%s__%s.conf", company, project)
	runtimeConfPath := filepath.ToSlash(filepath.Join(cfg.NginxConfRoot, confName))
	hostConfPath := runtimeConfPath
	if runtime.GOOS == "windows" {
		hostConfPath = platform.WSLToHost(cfg, runtimeConfPath)
	}

	certDirRuntime := filepath.ToSlash(filepath.Join(cfg.NginxExternalRoot, "certs", domain))
	hostCertDir := certDirRuntime
	if runtime.GOOS == "windows" {
		hostCertDir = platform.WSLToHost(cfg, certDirRuntime)
	}
	if err := os.MkdirAll(hostCertDir, 0o755); err != nil {
		return CreateResult{}, err
	}
	rendered, err := templates.RenderNginx(templates.NginxParams{
		Domain:        domain,
		Root:          "/var/www/apps/" + company + "/" + project,
		AccessLog:     "/var/log/nginx/" + company + "__" + project + ".access.log",
		ErrorLog:      "/var/log/nginx/" + company + "__" + project + ".error.log",
		PHPUpstream:   phpUpstream,
		ExternalCert:  "/etc/nginx/external/certs/" + domain + "/cert.pem",
		ExternalKey:   "/etc/nginx/external/certs/" + domain + "/key.pem",
		ExternalDh:    "/etc/nginx/external/dhparam.pem",
		AppEnv:        "dev",
		ClientMaxBody: "500M",
	})
	if err != nil {
		return CreateResult{}, err
	}
	a.Log.Debug().Msg("Nginx config rendered")

	if err := os.MkdirAll(filepath.Dir(hostConfPath), 0o755); err != nil {
		return CreateResult{}, err
	}
	if err := os.WriteFile(hostConfPath, []byte(rendered), 0o644); err != nil {
		return CreateResult{}, err
	}

	// Hosts and certificates may need administrator access; surface those
	// optional setup failures without hiding a required database failure.
	if err := platform.AddHost(domain); err != nil {
		warnings = append(warnings, "hosts: "+err.Error())
	}

	// Auto-issue HTTPS cert for the domain (mkcert). This keeps the nginx vhost consistent
	// (it always references external/certs/<domain>/cert.pem + key.pem).
	// If mkcert is not available or not yet installed, we surface it as a warning (project still created).
	certPem := filepath.Join(hostCertDir, "cert.pem")
	keyPem := filepath.Join(hostCertDir, "key.pem")
	_, certErr := os.Stat(certPem)
	_, keyErr := os.Stat(keyPem)
	if errors.Is(certErr, os.ErrNotExist) || errors.Is(keyErr, os.ErrNotExist) {
		if err := CertInitContext(ctx, cfg); err != nil {
			warnings = append(warnings, "cert: "+err.Error())
		} else if err := CertIssueContext(ctx, cfg, domain); err != nil {
			warnings = append(warnings, "cert: "+err.Error())
		}
	}
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}
	p := store.Project{
		Company:   company,
		Name:      project,
		Type:      req.Type,
		Domain:    domain,
		PHP:       req.PHP,
		DB:        db,
		DBName:    dbName,
		DBUser:    dbUser,
		DBPass:    dbPass,
		RootPath:  runtimeAppDir,
		HostPath:  hostAppDir,
		NginxConf: runtimeConfPath,
	}
	p2, err := st.Upsert(p)
	if err != nil {
		return CreateResult{}, err
	}
	a.Log.Info().Str("company", company).Str("project", p2.Name).Msg("Saved project")
	// Save the successfully provisioned database before importing. SQL dumps
	// can fail halfway through; retaining the project lets the user repair or
	// replace that database with the same credentials instead of stranding it.
	if importDump != nil {
		if err := importSQLReader(ctx, cfg, db, dbName, importDump); err != nil {
			warnings = append(warnings, "db import failed; project was saved so the import can be repaired or replaced: "+err.Error())
		}
	}
	return CreateResult{Project: p2, Warnings: warnings}, nil
}

func projectSlug(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("name required")
	}
	s := sanitize(value)
	if !filepath.IsLocal(s) || strings.Trim(s, ".") == "" || strings.HasSuffix(s, ".") || len(s) > 63 {
		return "", fmt.Errorf("invalid project name %q", value)
	}
	return s, nil
}

func validateDomain(domain string) error {
	if domain == "" || len(domain) > 253 {
		return fmt.Errorf("invalid domain %q", domain)
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("invalid domain %q", domain)
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return fmt.Errorf("invalid domain %q", domain)
			}
		}
	}
	return nil
}
