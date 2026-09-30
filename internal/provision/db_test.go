package provision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"devhelper/internal/app"
)

func TestNormalizeDBEngine(t *testing.T) {
	tests := map[string]string{
		"mysql": "mysql-8.4", "mysql-8.4": "mysql-8.4",
		"mysql9": "mysql-9.7", "mysql-9.6": "mysql-9.7", "mysql-9.7": "mysql-9.7",
		"mariadb-10.6": "mariadb10", "postgresql": "postgres", "none": "none",
	}
	for in, want := range tests {
		if got := normalizeDBEngine(in); got != want {
			t.Fatalf("normalizeDBEngine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSQLQuotingHelpers(t *testing.T) {
	if got := mysqlIdent("a`b"); got != "`a``b`" {
		t.Fatalf("mysqlIdent escaped incorrectly: %q", got)
	}
	if got := mysqlString("a'b"); got != "'a''b'" {
		t.Fatalf("mysqlString escaped incorrectly: %q", got)
	}
	if got := pgIdent("a\"b"); got != "\"a\"\"b\"" {
		t.Fatalf("pgIdent escaped incorrectly: %q", got)
	}
	if got := pgString("a'b"); got != "'a''b'" {
		t.Fatalf("pgString escaped incorrectly: %q", got)
	}
}

func TestSelectComposeService(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		services   []string
		want       string
	}{
		{"legacy mysql", "mysql-8.4", []string{"mysql", "mysql9"}, "mysql"},
		{"legacy mysql9", "mysql-9.7", []string{"mysql", "mysql9"}, "mysql9"},
		{"older mysql9", "mysql-9.6", []string{"mysql", "mysql9"}, "mysql9"},
		{"custom legacy name wins", "mysql-8.4", []string{"mysql", "mysql-8.4"}, "mysql-8.4"},
		{"custom", "local-db", []string{"mysql", "local-db"}, "local-db"},
		{"unknown", "misspelled-db", []string{"mysql"}, ""},
		{"missing mysql", "mysql-8.4", []string{"mysql9"}, ""},
		{"empty", "", []string{"mysql"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectComposeService(tt.configured, tt.services)
			if got != tt.want || (err != nil) != (tt.want == "") {
				t.Fatalf("got (%q, %v), want service %q", got, err, tt.want)
			}
		})
	}
}

func TestMySQLTargetsStaySeparate(t *testing.T) {
	cfg := app.Config{
		MySQLService: "mysql", MySQL9Service: "mysql9",
		MariaDB10Service: "mariadb10", MariaDB12Service: "mariadb12",
		MySQLHost: "localhost", MySQLPort: 3384, MySQL9Port: 3396,
		MariaDB10Port: 33106, MariaDB12Port: 3312,
	}
	for _, tt := range []struct {
		engine, service string
		port            int
	}{
		{"mysql", "mysql", 3384}, {"mysql9", "mysql9", 3396},
		{"mariadb10", "mariadb10", 33106}, {"mariadb12", "mariadb12", 3312},
	} {
		got, ok := resolveMySQLTarget(cfg, tt.engine)
		if !ok || got.service != tt.service || got.port != tt.port {
			t.Fatalf("%s resolved to %+v", tt.engine, got)
		}
	}
}

func TestDatabaseCommandsAreNoninteractive(t *testing.T) {
	cfg := app.Config{MySQLCli: "mysql-client", PSQLCli: "psql-client"}
	target := mysqlTarget{service: "mysql", host: "db-host", port: 3384, rootPass: "private-password"}
	sql := "SELECT 'private-sql';"
	cmd := mysqlCommand(t.Context(), cfg, target, "", strings.NewReader(sql))
	for _, required := range []string{"--no-defaults", "--protocol=tcp", "--connect-timeout=3", "--host=db-host", "--port=3384"} {
		if !slices.Contains(cmd.Args, required) {
			t.Fatalf("missing %s: %v", required, cmd.Args)
		}
	}
	if strings.Contains(strings.Join(cmd.Args, " "), "private") {
		t.Fatalf("credentials or SQL leaked into arguments: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Env, "MYSQL_PWD=private-password") {
		t.Fatal("missing password environment")
	}
	target.rootPass = ""
	cmd = mysqlCommand(t.Context(), cfg, target, "", strings.NewReader(sql))
	if slices.Contains(cmd.Args, "-p") || slices.Contains(cmd.Args, "--password") {
		t.Fatal("empty password must not prompt")
	}
	pg := postgresCommand(t.Context(), cfg, pgTarget{host: "pg-host", port: 5434}, "mydb", strings.NewReader(sql))
	for _, required := range []string{"-X", "-w", "ON_ERROR_STOP=1", "-f", "-"} {
		if !slices.Contains(pg.Args, required) {
			t.Fatalf("missing psql safety option %s: %v", required, pg.Args)
		}
	}
}

func TestDefaultClientUsesSelectedContainer(t *testing.T) {
	cfg := app.Config{DockerCli: "docker", ComposeDir: t.TempDir()}
	target := mysqlTarget{client: "mariadb", service: "custom-mariadb", host: "host-db", port: 3312}
	cmd := mysqlCommand(t.Context(), cfg, target, "app-db", strings.NewReader("SELECT 1;"))
	for _, required := range []string{"compose", "exec", "-T", "MYSQL_PWD", "custom-mariadb", "mariadb", "--host=127.0.0.1", "--port=3306", "--database=app-db"} {
		if !slices.Contains(cmd.Args, required) {
			t.Fatalf("missing %s: %v", required, cmd.Args)
		}
	}
	if cmd.Dir != cfg.ComposeDir {
		t.Fatalf("wrong Compose directory: %s", cmd.Dir)
	}
}

func TestWaitDatabaseReadyRetriesAuthentication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		start := time.Now()
		err := waitDatabaseReady(t.Context(), "mysql", func(context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("initializing root account")
			}
			return nil
		})
		if err != nil || attempts != 3 || time.Since(start) != 1600*time.Millisecond {
			t.Fatalf("err=%v attempts=%d elapsed=%v", err, attempts, time.Since(start))
		}
	})
}

func TestWaitDatabaseReadyRetainsLastError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		err := waitDatabaseReady(t.Context(), "mysql-8.4", func(context.Context) error {
			return errors.New("access denied for user root")
		})
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "access denied for user root") {
			t.Fatalf("unhelpful readiness error: %v", err)
		}
		if elapsed := time.Since(start); elapsed != databaseReadyTimeout {
			t.Fatalf("timeout took %v", elapsed)
		}
	})
}

func TestWaitDatabaseReadyMissingClientFailsImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		err := waitDatabaseReady(t.Context(), "mysql", func(context.Context) error {
			attempts++
			return fmt.Errorf("client: %w", &exec.Error{Name: "missing-mysql", Err: exec.ErrNotFound})
		})
		if attempts != 1 || !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("err=%v attempts=%d", err, attempts)
		}
	})
}

func TestWaitDatabaseReadyHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := waitDatabaseReady(ctx, "mysql", func(context.Context) error {
		t.Fatal("probe executed after cancellation")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
}

func TestReplaceSQLValidatesDumpBeforeDatabaseOperations(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.sql")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"", filepath.Join(dir, "missing.sql"), dir, empty} {
		err := ReplaceSQLContext(t.Context(), app.Config{}, "mysql", "db", "user", "pass", file)
		if err == nil || strings.Contains(err.Error(), "compose_dir") {
			t.Fatalf("file %q reached database operations: %v", file, err)
		}
	}
}
