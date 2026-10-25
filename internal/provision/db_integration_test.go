package provision

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"devhelper/internal/app"
)

// Opt in with DEVHELPER_DB_INTEGRATION=1. All data resides in tmpfs in a uniquely
// named Compose project; the user's development stack is never addressed.
func TestDatabaseIntegration(t *testing.T) {
	if os.Getenv("DEVHELPER_DB_INTEGRATION") != "1" {
		t.Skip("set DEVHELPER_DB_INTEGRATION=1 to run isolated Docker database tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	project := "devhelper-db-test-" + strings.ToLower(rand.Text()[:10])
	compose := fmt.Sprintf("name: %s\nservices:\n  mysql:\n    image: mysql:8.4\n    environment:\n      MYSQL_ROOT_PASSWORD: integration-root-pass\n      MYSQL_ROOT_HOST: '%%'\n    ports: ['127.0.0.1::3306']\n    tmpfs: ['/var/lib/mysql']\n  postgres:\n    image: postgres:18-bookworm\n    environment:\n      POSTGRES_PASSWORD: integration-root-pass\n    tmpfs: ['/var/lib/postgresql']\n", project)
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(compose), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := app.DefaultConfig(dir)
	cfg.ComposeDir = dir
	cfg.MySQLService = "mysql-8.4" // Existing configs must migrate at runtime.
	cfg.MySQLRootPass, cfg.PostgresSuperPass = "integration-root-pass", "integration-root-pass"
	cfg.MySQLCli, cfg.PSQLCli = "", ""
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if _, err := commandOutput(dockerComposeCommand(cleanupCtx, cfg, "down", "--volumes", "--remove-orphans"), "remove isolated test databases"); err != nil {
			t.Error(err)
		}
	})
	t.Run("MySQL84", func(t *testing.T) {
		dbName, user := "project_db", "project_user"
		password := "pw\\ord'$$--\\end"
		if err := CreateMySQLDBForEngineContext(ctx, cfg, "mysql-8.4", dbName, user, password); err != nil {
			t.Fatal(err)
		}
		target, _ := resolveMySQLTarget(cfg, "mysql")
		target.service = "mysql"
		checkLogin := func(pass string) {
			t.Helper()
			projectTarget := target
			projectTarget.rootPass = pass
			cmd := mysqlCommand(ctx, cfg, projectTarget, dbName, strings.NewReader("SELECT DATABASE();"))
			for i, arg := range cmd.Args {
				if arg == "--user=root" {
					cmd.Args[i] = "--user=" + user
				}
			}
			out, err := commandOutput(cmd, "authenticate provisioned user", pass)
			if err != nil || strings.TrimSpace(out) != dbName {
				t.Fatalf("user login got %q, %v", out, err)
			}
		}
		checkLogin(password)
		// A retry after partial provisioning must install the requested credentials.
		password += "_retry"
		if err := CreateMySQLDBForEngineContext(ctx, cfg, "mysql-8.4", dbName, user, password); err != nil {
			t.Fatal(err)
		}
		checkLogin(password)
		dump := filepath.Join(dir, "mysql.sql")
		if err := os.WriteFile(dump, []byte("CREATE TABLE imported (id INT); INSERT INTO imported VALUES (42);\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := ImportSQLContext(ctx, cfg, "mysql", dbName, dump); err != nil {
			t.Fatal(err)
		}
		query := "SELECT id FROM " + mysqlIdent(dbName) + ".imported;"
		assertRows := func(want string) {
			t.Helper()
			out, err := mysqlQuery(ctx, cfg, target, query, "check imported data")
			if err != nil || strings.TrimSpace(out) != want {
				t.Fatalf("query got %q, %v; want %q", out, err, want)
			}
		}
		assertRows("42")
		if err := ReplaceSQLContext(ctx, cfg, "mysql", dbName, user, password, filepath.Join(dir, "absent.sql")); err == nil {
			t.Fatal("missing replacement dump accepted")
		}
		assertRows("42")
		if err := ReplaceSQLContext(ctx, cfg, "mysql", dbName, user, password, dump); err != nil {
			t.Fatal(err)
		}
		assertRows("42")
		// Exercise an explicitly configured native client when supplied by the caller.
		if cli := os.Getenv("DEVHELPER_MYSQL_CLI"); cli != "" {
			out, err := commandOutput(dockerComposeCommand(ctx, cfg, "port", "mysql", "3306"), "find test MySQL port")
			if err != nil {
				t.Fatal(err)
			}
			host, port, err := net.SplitHostPort(strings.TrimSpace(out))
			if err != nil {
				t.Fatal(err)
			}
			hostCfg := cfg
			hostCfg.MySQLCli, hostCfg.MySQLHost = cli, host
			hostCfg.MySQLPort, err = strconv.Atoi(port)
			if err != nil {
				t.Fatal(err)
			}
			if err := CreateMySQLDBForEngineContext(ctx, hostCfg, "mysql", "native_client_db", "native_client_user", "native-pass"); err != nil {
				t.Fatal(err)
			}
		}
		// Preserve a shared legacy user when only one project's database is dropped.
		if err := DropMySQLDBForEngineContext(ctx, cfg, "mysql", dbName, ""); err != nil {
			t.Fatal(err)
		}
		out, err := mysqlQuery(ctx, cfg, target, "SELECT COUNT(*) FROM mysql.user WHERE User = "+mysqlString(user)+";", "check preserved user")
		if err != nil || strings.TrimSpace(out) != "1" {
			t.Fatalf("shared user removed: %q, %v", out, err)
		}
		if err := DropMySQLDBForEngineContext(ctx, cfg, "mysql", dbName, user); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Postgres18", func(t *testing.T) {
		dbName, user := "project_db", "project_user"
		password := "pw\\ord'$$--\\end"
		if err := CreatePostgresDBContext(ctx, cfg, dbName, user, password); err != nil {
			t.Fatal(err)
		}
		target, _ := resolvePGTarget(cfg, "postgres")
		projectTarget := target
		projectTarget.superPass = password
		cmd := postgresCommand(ctx, cfg, projectTarget, dbName, strings.NewReader("SELECT current_database();"))
		for i, arg := range cmd.Args {
			if arg == "-U" {
				cmd.Args[i+1] = user
			}
		}
		out, err := commandOutput(cmd, "authenticate provisioned PostgreSQL user", password)
		if err != nil || strings.TrimSpace(out) != dbName {
			t.Fatalf("user login got %q, %v", out, err)
		}
		dump := filepath.Join(dir, "postgres.sql")
		if err := os.WriteFile(dump, []byte("CREATE TABLE imported (id INT); INSERT INTO imported VALUES (42);\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := ImportSQLContext(ctx, cfg, "postgres", dbName, dump); err != nil {
			t.Fatal(err)
		}
		if err := ReplaceSQLContext(ctx, cfg, "postgres", dbName, user, password, dump); err != nil {
			t.Fatal(err)
		}
		cmd = postgresCommand(ctx, cfg, target, dbName, strings.NewReader("SELECT id FROM imported;"))
		out, err = commandOutput(cmd, "read PostgreSQL import")
		if err != nil || strings.TrimSpace(out) != "42" {
			t.Fatalf("imported row got %q, %v", out, err)
		}
		if err := DropPostgresDBContext(ctx, cfg, dbName, ""); err != nil {
			t.Fatal(err)
		}
		out, err = postgresQuery(ctx, cfg, target, "SELECT COUNT(*) FROM pg_roles WHERE rolname="+pgString(user)+";", "check preserved role")
		if err != nil || strings.TrimSpace(out) != "1" {
			t.Fatalf("shared role removed: %q, %v", out, err)
		}
		if err := DropPostgresDBContext(ctx, cfg, dbName, user); err != nil {
			t.Fatal(err)
		}
	})
}
