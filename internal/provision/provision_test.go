package provision

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devhelper/internal/app"
	"devhelper/internal/store"
	"github.com/rs/zerolog"
)

func testProjectApp(t *testing.T) (*app.App, *store.Store) {
	t.Helper()
	root := t.TempDir()
	cfg := app.DefaultConfig(root)
	cfg.RootDir = root
	cfg.HostMirrorDir = ""
	cfg.AppsRoot = filepath.Join(root, "apps")
	cfg.NginxConfRoot = filepath.Join(root, "nginx")
	cfg.NginxExternalRoot = filepath.Join(root, "external")
	cfg.StoreFile = filepath.Join(root, "state.json")
	cfg.ComposeDir = root
	cfg.DockerCli = filepath.Join(root, "missing-docker")
	st, err := store.Open(cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	return &app.App{Cfg: cfg, RepoRoot: root, Log: zerolog.Nop()}, st
}

func TestCreateDatabaseFailureIsNotSuccess(t *testing.T) {
	a, st := testProjectApp(t)
	_, err := CreateContext(t.Context(), a, st, CreateRequest{Company: "acme", Project: "app", PHP: "8.4", DB: "mysql"})
	if err == nil || !strings.Contains(err.Error(), "create mysql-8.4 database") {
		t.Fatalf("wanted actionable DB failure, got %v", err)
	}
	projects, err := st.List()
	if err != nil || len(projects) != 0 {
		t.Fatalf("failed DB persisted as a project: %v, %v", projects, err)
	}
	if _, err := os.Stat(a.Cfg.AppsRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database failure created project files: %v", err)
	}
}

// Every supported PHP version must have its container template and Compose service in the
// repo, otherwise "project create" would point nginx at a service that cannot exist.
func TestSupportedPHPVersionsHaveContainerTemplates(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join("..", "..", "projects", "docker-compose.example.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range SupportedPHPVersions {
		dockerfile := filepath.Join("..", "..", "projects", "containers", "php", "php"+version, "Dockerfile")
		if _, err := os.Stat(dockerfile); err != nil {
			t.Errorf("PHP %s: missing container template %s", version, dockerfile)
		}
		service := phpServiceName(version)
		if !strings.Contains(string(compose), "\n  "+service+":") {
			t.Errorf("PHP %s: no %q service in docker-compose.example.yml", version, service)
		}
	}
}

func TestCreateRejectsUnsafeAndUnsupportedInputBeforeSideEffects(t *testing.T) {
	for _, tc := range []struct{ name, company, project, domain, php, db string }{
		{"parent", "..", "app", "", "8.4", "none"},
		{"empty", "", "app", "", "8.4", "none"},
		{"domain traversal", "acme", "app", "../../victim", "8.4", "none"},
		{"nginx injection", "acme", "app", "app.local; include other", "8.4", "none"},
		{"php", "acme", "app", "", "8.4;", "none"},
		{"php service disabled", "acme", "app", "", "7.3", "none"},
		{"engine typo", "acme", "app", "", "8.4", "mysqll"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, st := testProjectApp(t)
			if _, err := CreateContext(t.Context(), a, st, CreateRequest{Company: tc.company, Project: tc.project, Domain: tc.domain, PHP: tc.php, DB: tc.db}); err == nil {
				t.Fatal("invalid input accepted")
			}
			if _, err := os.Stat(a.Cfg.AppsRoot); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid request modified app files")
			}
		})
	}
}

func TestCreateRejectsExistingProjectAndDomain(t *testing.T) {
	a, st := testProjectApp(t)
	if _, err := st.Upsert(store.Project{Company: "acme", Name: "app", Domain: "app.acme.local", DBPass: "preserve-me"}); err != nil {
		t.Fatal(err)
	}
	for _, req := range []CreateRequest{
		{Company: "acme", Project: "app", PHP: "8.4", DB: "mysql"},
		{Company: "other", Project: "app", Domain: "app.acme.local", PHP: "8.4", DB: "mysql"},
	} {
		if _, err := CreateContext(t.Context(), a, st, req); err == nil || !strings.Contains(err.Error(), "already") {
			t.Fatalf("collision not rejected: %v", err)
		}
	}
	p, _, err := st.Get("acme", "app")
	if err != nil || p.DBPass != "preserve-me" {
		t.Fatal("duplicate creation changed credentials")
	}
}

func TestProjectDBNamesDoNotCollideOrExceedEngineLimits(t *testing.T) {
	seenNames, seenUsers := map[string]bool{}, map[string]bool{}
	for _, pair := range [][2]string{{"a", "b_c"}, {"a_b", "c"}, {"a", "b-c"}, {"b", "b-c"}, {strings.Repeat("a", 63), strings.Repeat("b", 63)}} {
		name, user := projectDBNames(pair[0], pair[1])
		if len(name) > 63 || len(user) > 32 || seenNames[name] || seenUsers[user] {
			t.Fatalf("unsafe DB identity: %q, %q", name, user)
		}
		seenNames[name], seenUsers[user] = true, true
	}
}

func TestCanceledCreateDoesNothing(t *testing.T) {
	a, st := testProjectApp(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := CreateContext(ctx, a, st, CreateRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestDropRejectsStoredPathOutsideRoot(t *testing.T) {
	a, st := testProjectApp(t)
	victim := t.TempDir()
	file := filepath.Join(victim, "keep.txt")
	if err := os.WriteFile(file, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Upsert(store.Project{Company: "acme", Name: "app", Domain: "app.acme.local", DB: "none", HostPath: victim, NginxConf: filepath.Join(a.Cfg.NginxConfRoot, "acme__app.conf")}); err != nil {
		t.Fatal(err)
	}
	if _, err := DropContext(t.Context(), a, st, DropRequest{Company: "acme", Project: "app"}); err == nil {
		t.Fatal("unsafe stored path accepted")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("unrelated data removed: %v", err)
	}
	if _, ok, _ := st.Get("acme", "app"); !ok {
		t.Fatal("failed deletion removed metadata")
	}
}

func TestRemoveWithinConfinesSymlinkTraversal(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := removeWithin(root, filepath.Join("link", "keep"), true); err == nil {
		t.Fatal("symlink escaped deletion root")
	}
	if _, err := os.Stat(filepath.Join(outside, "keep")); err != nil {
		t.Fatal(err)
	}
}
