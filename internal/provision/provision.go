package provision

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/store"
	"devhelper/internal/templates"

	"github.com/rs/zerolog/log"
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

// randomPass creates credentials for per-project DB users. It uses only
// shell/URL-friendly characters because these values are commonly copied into
// .env files and command lines during local development.
func randomPass(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	b := make([]byte, n)
	rb := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, rb); err != nil {
		// crypto/rand should only fail when the OS CSPRNG is unavailable. We
		// panic instead of silently creating weak credentials because a project
		// DB password is security-sensitive and no caller can repair a bad one.
		panic(fmt.Errorf("generate random password: %w", err))
	}
	for i := range b {
		b[i] = chars[int(rb[i])%len(chars)]
	}
	return string(b)
}

func Create(a *app.App, st *store.Store, req CreateRequest) (CreateResult, error) {
	cfg := a.Cfg
	company := sanitize(req.Company)
	project := sanitize(req.Project)

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		domain = defaultDomain(cfg.DefaultDomainPattern, company, project)
	}
	log.Info().Msgf("Creating project %s/%s with domain %s", company, project, domain)
	phpSvc := phpServiceName(req.PHP)
	phpUpstream := phpSvc + ":9000"

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
		_ = os.MkdirAll(winDir, 0o755)
		_ = os.WriteFile(filepath.Join(winDir, ".wsl-path"), []byte(runtimeAppDir+"\n"), 0o644)
	}
	log.Info().Msg("Directories created")
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
	_ = os.MkdirAll(hostCertDir, 0o755)
	log.Info().Msg("Cert directory created")
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
	log.Info().Msg("Nginx config rendered")

	if err := os.MkdirAll(filepath.Dir(hostConfPath), 0o755); err != nil {
		return CreateResult{}, err
	}
	if err := os.WriteFile(hostConfPath, []byte(rendered), 0o644); err != nil {
		return CreateResult{}, err
	}

	warnings := []string{}
	// Hosts/certs/DB setup are useful, but they are not prerequisites for
	// writing the store entry. We collect warnings so the UI can tell the user
	// exactly what needs manual attention while still preserving the project.
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
		if err := CertInit(cfg); err != nil {
			warnings = append(warnings, "cert: "+err.Error())
		} else if err := CertIssue(cfg, domain); err != nil {
			warnings = append(warnings, "cert: "+err.Error())
		}
	}
	log.Info().Msgf("Created cert.pem and key.pem for domain %s", domain)
	dbName, dbUser, dbPass := "", "", ""
	db := normalizeDBEngine(req.DB)
	if isSupportedDB(db) {
		dbName = dbIdent(company + "_" + project)
		dbUser = dbIdent(project + "_u")
		dbPass = randomPass(16)
		if err := createDB(cfg, db, dbName, dbUser, dbPass); err != nil {
			warnings = append(warnings, "db: "+err.Error())
		}
	} else {
		db = "none"
	}

	// Optional: import dump after create (using superuser/root over TCP from host).
	if db != "none" && req.ImportOnCreate && strings.TrimSpace(req.ImportFile) != "" {
		if err := ImportSQL(cfg, db, dbName, req.ImportFile); err != nil {
			warnings = append(warnings, "db import: "+err.Error())
		}
	}
	log.Info().Msgf("Created db %s", dbName)
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
	log.Info().Msgf("Upserted project %s", p2.Name)
	return CreateResult{Project: p2, Warnings: warnings}, nil
}
