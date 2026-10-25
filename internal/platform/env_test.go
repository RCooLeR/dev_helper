package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"devhelper/internal/app"
)

func TestWriteEnvPreservesLiteralValues(t *testing.T) {
	for _, password := range []string{
		"simple",
		`pa$$word ${UNSET_PASSWORD} $HOME # comment`,
		`single' and double" quotes`,
		`trailing\`,
		`backslash\"quote`,
		" leading and trailing ",
		"line1\nline2\r\ttab",
		"",
	} {
		t.Run(password, func(t *testing.T) {
			cfg := app.Config{
				ComposeDir:        t.TempDir(),
				AppsRoot:          `/data/projects with spaces/apps`,
				DataRoot:          `C:\Data\folder\`,
				NginxConfRoot:     `/data/projects/#config`,
				NginxExternalRoot: `/data/projects/$external`,
				MySQLRootPass:     password,
				PostgresSuperPass: password,
			}
			if err := WriteEnv(cfg); err != nil {
				t.Fatal(err)
			}
			got, err := ReadComposeEnv(cfg)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{
				"APPS_ROOT": cfg.AppsRoot, "DATA_ROOT": cfg.DataRoot,
				"NGINX_CONF_ROOT": cfg.NginxConfRoot, "NGINX_EXTERNAL_ROOT": cfg.NginxExternalRoot,
				"MYSQL_ROOT_PASSWORD": password, "POSTGRES_SUPER_PASSWORD": password,
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Compose .env round trip: got %#v, want %#v", got, want)
			}
		})
	}
}

func TestReadComposeEnvUsesComposeSyntax(t *testing.T) {
	cfg := app.Config{ComposeDir: t.TempDir()}
	t.Setenv("DEVHELPER_ENV_TEST_BASE", "from-shell")
	content := "\ufeff" + `# comment
DEVHELPER_ENV_TEST_BASE=from-file
EXPANDED="${DEVHELPER_ENV_TEST_BASE} # literal"
DEFAULT=${DEVHELPER_ENV_TEST_UNSET:-fallback}
SINGLE='${DEVHELPER_ENV_TEST_BASE} # literal'
TRAILING=hello # comment
INLINE_HASH=hello#world
export EXPORTED='works'
MULTILINE='first
second'
`
	if err := os.WriteFile(filepath.Join(cfg.ComposeDir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadComposeEnv(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"DEVHELPER_ENV_TEST_BASE": "from-file", "EXPANDED": "from-shell # literal",
		"DEFAULT": "fallback", "SINGLE": "${DEVHELPER_ENV_TEST_BASE} # literal",
		"TRAILING": "hello", "INLINE_HASH": "hello#world", "EXPORTED": "works", "MULTILINE": "first\nsecond",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadComposeEnv() = %#v, want %#v", got, want)
	}
}

func TestReadComposeEnvErrorsAndLongValues(t *testing.T) {
	cfg := app.Config{ComposeDir: t.TempDir()}
	if got, err := ReadComposeEnv(cfg); err != nil || len(got) != 0 {
		t.Fatalf("missing .env: got %v, %v", got, err)
	}
	path := filepath.Join(cfg.ComposeDir, ".env")
	if err := os.WriteFile(path, []byte("PASSWORD='unterminated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadComposeEnv(cfg); err == nil {
		t.Fatal("invalid .env was accepted")
	}
	long := strings.Repeat("x", 70000)
	if err := os.WriteFile(path, []byte("LONG="+long+"\nAFTER=present\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadComposeEnv(cfg)
	if err != nil || got["LONG"] != long || got["AFTER"] != "present" {
		t.Fatalf("long .env line truncated parsing: LONG len=%d, AFTER=%q, err=%v", len(got["LONG"]), got["AFTER"], err)
	}
}

func TestGetComposeEnvDoesNotTrimCredentials(t *testing.T) {
	cfg := app.Config{ComposeDir: t.TempDir()}
	const key = "DEVHELPER_ENV_TEST_PASSWORD"
	for _, value := range []string{" spaced password ", ""} {
		t.Setenv(key, value)
		if got := GetComposeEnv(cfg, key, "fallback"); got != value {
			t.Fatalf("GetComposeEnv() = %q, want %q", got, value)
		}
	}
}

func TestWriteEnvPreservesCustomContentAcrossReinitialization(t *testing.T) {
	cfg := app.Config{ComposeDir: t.TempDir(), MySQLRootPass: "first"}
	path := filepath.Join(cfg.ComposeDir, ".env")
	const custom = "# customer setting\r\nCUSTOM='multi\nline # value'\r\nREFERENCE=${CUSTOM}\r\n"
	if err := os.WriteFile(path, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"first", "new $password"} {
		cfg.MySQLRootPass = password
		if err := WriteEnv(cfg); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), custom) || strings.Count(string(b), composeEnvBegin) != 1 || strings.Count(string(b), composeEnvEnd) != 1 {
			t.Fatalf("custom .env was rewritten or managed block duplicated: %q", b)
		}
		parsed, err := ReadComposeEnv(cfg)
		if err != nil || parsed["MYSQL_ROOT_PASSWORD"] != password || parsed["REFERENCE"] != "multi\nline # value" {
			t.Fatalf("unexpected .env values: %#v, %v", parsed, err)
		}
	}
}

func TestWriteEnvReplacesManagedBlockInPlace(t *testing.T) {
	cfg := app.Config{ComposeDir: t.TempDir(), MySQLRootPass: "updated"}
	path := filepath.Join(cfg.ComposeDir, ".env")
	prefix := "\ufeff# preserve prefix\n"
	suffix := "# preserve suffix\nCUSTOM='trailing'\n"
	existing := prefix + composeEnvBegin + "\nMYSQL_ROOT_PASSWORD='old'\n" + composeEnvEnd + "\n" + suffix
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteEnv(cfg); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(result), prefix) || !strings.HasSuffix(string(result), suffix) || strings.Contains(string(result), "'old'") {
		t.Fatalf("managed block replacement changed surrounding content: %q", result)
	}
}

func TestWriteEnvRejectsMalformedManagedBlocksWithoutChangingFile(t *testing.T) {
	for _, existing := range []string{
		composeEnvBegin + "\nKEY=1\n",
		composeEnvEnd + "\n",
		composeEnvEnd + "\n" + composeEnvBegin + "\n",
		composeEnvBegin + "\n" + composeEnvEnd + "\n" + composeEnvBegin + "\n" + composeEnvEnd + "\n",
		"CUSTOM='unterminated",
		"CUSTOM='multi\n" + composeEnvBegin + "\nA=1\n" + composeEnvEnd + "\nline'\n",
	} {
		t.Run(existing, func(t *testing.T) {
			cfg := app.Config{ComposeDir: t.TempDir()}
			path := filepath.Join(cfg.ComposeDir, ".env")
			if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := WriteEnv(cfg); err == nil {
				t.Fatal("ambiguous or malformed existing .env was accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != existing {
				t.Fatalf("failed init changed existing .env: %q, %v", after, err)
			}
		})
	}
}

func TestWriteEnvMatchesInstalledCompose(t *testing.T) {
	if os.Getenv("DEVHELPER_TEST_COMPOSE") != "1" {
		t.Skip("set DEVHELPER_TEST_COMPOSE=1 to verify against the installed Docker Compose parser")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.Config{
		ComposeDir:    t.TempDir(),
		MySQLRootPass: " spaces $HOME ${OTHER} # ' \" \\ trailing\\ ",
	}
	if err := WriteEnv(cfg); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(cfg.ComposeDir, "compose.yaml")
	const compose = "services:\n  test:\n    image: scratch\n    environment:\n      PASSWORD: ${MYSQL_ROOT_PASSWORD}\n"
	if err := os.WriteFile(composePath, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, docker, "compose", "-p", "devhelper-env-test", "--project-directory", cfg.ComposeDir, "-f", composePath, "--env-file", filepath.Join(cfg.ComposeDir, ".env"), "config", "--environment")
	// Avoid inheriting a real credential that would override the test .env.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "MYSQL_ROOT_PASSWORD") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("docker compose config: %v", err)
	}
	// --environment returns interpolation values. Normal config output escapes
	// dollars again so its result can itself be used as another Compose file.
	for line := range strings.SplitSeq(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if value, ok := strings.CutPrefix(line, "MYSQL_ROOT_PASSWORD="); ok {
			if value != cfg.MySQLRootPass {
				t.Fatalf("Docker Compose changed literal password: got %q, want %q", value, cfg.MySQLRootPass)
			}
			return
		}
	}
	t.Fatal("Docker Compose omitted test password")
}
