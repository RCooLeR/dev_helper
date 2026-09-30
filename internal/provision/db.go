package provision

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"devhelper/internal/app"

	"github.com/rs/zerolog/log"
)

type mysqlTarget struct {
	client   string
	service  string
	host     string
	port     int
	rootPass string
}

type pgTarget struct {
	service   string
	host      string
	port      int
	superPass string
}

// normalizeDBEngine is the single place where UI/CLI aliases become the
// canonical names stored in projects. Keeping this small translation layer
// prevents version bumps (for example MySQL 9.6 -> 9.7) from leaking through
// the rest of the codebase.
func normalizeDBEngine(engine string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "mysql", "mysql-8", "mysql-8.4":
		return "mysql-8.4"
	case "mysql9", "mysql-9", "mysql-9.6", "mysql-9.7":
		return "mysql-9.7"
	case "mariadb10", "mariadb-10", "mariadb-10.6":
		return "mariadb10"
	case "mariadb12", "mariadb-12":
		return "mariadb12"
	case "postgres", "postgresql", "postgresql18":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(engine))
	}
}

func isSupportedDB(engine string) bool {
	switch normalizeDBEngine(engine) {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12", "postgres":
		return true
	default:
		return false
	}
}

// mysqlIdent/mysqlString/pgIdent/pgString are tiny quoting helpers for DDL.
// The current project names are sanitized before reaching this layer, but the
// store is user-editable JSON and older stores may contain values created by
// older versions. Quoting here keeps DB operations safe at the final boundary.
func mysqlIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func mysqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func pgIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func pgString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func resolveMySQLTarget(cfg app.Config, engine string) (mysqlTarget, bool) {
	engine = normalizeDBEngine(engine)
	switch engine {
	case "mysql-8.4":
		return mysqlTarget{service: cfg.MySQLService, host: cfg.MySQLHost, port: cfg.MySQLPort, rootPass: cfg.MySQLRootPass}, true
	case "mysql-9.7":
		return mysqlTarget{service: cfg.MySQL9Service, host: cfg.MySQLHost, port: cfg.MySQL9Port, rootPass: cfg.MySQLRootPass}, true
	case "mariadb10":
		return mysqlTarget{client: "mariadb", service: cfg.MariaDB10Service, host: cfg.MySQLHost, port: cfg.MariaDB10Port, rootPass: cfg.MySQLRootPass}, true
	case "mariadb12":
		return mysqlTarget{client: "mariadb", service: cfg.MariaDB12Service, host: cfg.MySQLHost, port: cfg.MariaDB12Port, rootPass: cfg.MySQLRootPass}, true
	default:
		return mysqlTarget{}, false
	}
}

func resolvePGTarget(cfg app.Config, engine string) (pgTarget, bool) {
	engine = normalizeDBEngine(engine)
	if engine != "postgres" {
		return pgTarget{}, false
	}
	return pgTarget{service: cfg.PostgresService, host: cfg.PostgresHost, port: cfg.PostgresPort, superPass: cfg.PostgresSuperPass}, true
}

const (
	databaseReadyTimeout = 90 * time.Second
	databaseQueryTimeout = 30 * time.Second
)

func dockerComposeCommand(ctx context.Context, cfg app.Config, args ...string) *exec.Cmd {
	exe := strings.TrimSpace(cfg.DockerCli)
	if exe == "" {
		exe = "docker"
	}
	cmd := exec.CommandContext(ctx, exe, append([]string{"compose"}, args...)...)
	cmd.Dir = cfg.ComposeDir
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

func commandOutput(cmd *exec.Cmd, operation string, secrets ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		for _, secret := range secrets {
			if secret != "" {
				detail = strings.ReplaceAll(detail, secret, "[redacted]")
				detail = strings.ReplaceAll(detail, strings.ReplaceAll(secret, "'", "''"), "[redacted]")
			}
		}
		return "", fmt.Errorf("%s: %w; %s", operation, err, detail)
	}
	return stdout.String(), nil
}

// Old releases confused image versions with Compose service names. Preserve
// exact custom services; fall back only when the old name is absent.
func selectComposeService(configured string, services []string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return "", fmt.Errorf("database service name is empty")
	}
	if slices.Contains(services, configured) {
		return configured, nil
	}
	legacy := map[string]string{"mysql-8.4": "mysql", "mysql-9.6": "mysql9", "mysql-9.7": "mysql9"}
	if fallback := legacy[configured]; fallback != "" && slices.Contains(services, fallback) {
		return fallback, nil
	}
	return "", fmt.Errorf("database service %q is missing from Compose (available: %s)", configured, strings.Join(services, ", "))
}

// Starting only the requested service avoids rebuilding unrelated PHP services.
func ensureServiceUpContext(ctx context.Context, cfg app.Config, service string) (string, error) {
	if strings.TrimSpace(cfg.ComposeDir) == "" {
		return "", fmt.Errorf("compose_dir is empty (run: devhelper init)")
	}
	if strings.TrimSpace(service) == "" {
		return "", fmt.Errorf("database service name is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	out, err := commandOutput(dockerComposeCommand(ctx, cfg, "config", "--services"), "read Compose services")
	if err != nil {
		return "", err
	}
	service, err = selectComposeService(service, strings.Fields(out))
	if err != nil {
		return "", err
	}
	_, err = commandOutput(dockerComposeCommand(ctx, cfg, "up", "-d", "--no-deps", service), "start database service "+service)
	return service, err
}

func mysqlCommand(ctx context.Context, cfg app.Config, t mysqlTarget, dbName string, input io.Reader) *exec.Cmd {
	host, port := t.host, t.port
	exe := strings.TrimSpace(cfg.MySQLCli)
	container := exe == ""
	if container {
		host, port = "127.0.0.1", 3306
		exe = t.client
		if exe == "" {
			exe = "mysql"
		}
	}
	args := []string{
		"--no-defaults", "--protocol=tcp", "--connect-timeout=3",
		"--host=" + host, "--port=" + strconv.Itoa(port), "--user=root",
		"--batch", "--skip-column-names", "--binary-mode=1", "--default-character-set=utf8mb4",
	}
	if dbName != "" {
		args = append(args, "--database="+dbName)
	}
	var cmd *exec.Cmd
	if container {
		cmd = dockerComposeCommand(ctx, cfg, append([]string{"exec", "-T", "--env", "MYSQL_PWD", t.service, exe}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, exe, args...)
		cmd.WaitDelay = 2 * time.Second
	}
	// An empty password stays noninteractive; "-p" alone would prompt forever.
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+t.rootPass)
	cmd.Stdin = input
	return cmd
}

func postgresCommand(ctx context.Context, cfg app.Config, t pgTarget, dbName string, input io.Reader) *exec.Cmd {
	host, port := t.host, t.port
	exe := strings.TrimSpace(cfg.PSQLCli)
	container := exe == ""
	if container {
		host, port, exe = "127.0.0.1", 5432, "psql"
	}
	args := []string{"-X", "-w", "-h", host, "-p", strconv.Itoa(port), "-U", "postgres", "-d", dbName, "-v", "ON_ERROR_STOP=1", "-qAt", "-f", "-"}
	var cmd *exec.Cmd
	if container {
		cmd = dockerComposeCommand(ctx, cfg, append([]string{"exec", "-T", "--env", "PGPASSWORD", "--env", "PGCONNECT_TIMEOUT", t.service, exe}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, exe, args...)
		cmd.WaitDelay = 2 * time.Second
	}
	cmd.Env = append(os.Environ(), "PGPASSWORD="+t.superPass, "PGCONNECT_TIMEOUT=3")
	cmd.Stdin = input
	return cmd
}

func mysqlQuery(ctx context.Context, cfg app.Config, t mysqlTarget, sql, operation string, secrets ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, databaseQueryTimeout)
	defer cancel()
	return commandOutput(mysqlCommand(ctx, cfg, t, "", strings.NewReader(sql)), operation+" ("+t.service+")", append(secrets, t.rootPass)...)
}

func postgresQuery(ctx context.Context, cfg app.Config, t pgTarget, sql, operation string, secrets ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, databaseQueryTimeout)
	defer cancel()
	return commandOutput(postgresCommand(ctx, cfg, t, "postgres", strings.NewReader(sql)), operation+" ("+t.service+")", append(secrets, t.superPass)...)
}

// Readiness requires an authenticated query, not an open Docker port or a
// socket on the image's temporary initialization server. Each attempt and the
// total wait are bounded. Keep the last error so bad credentials are actionable.
func waitDatabaseReady(ctx context.Context, name string, probe func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, databaseReadyTimeout)
	defer cancel()
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%s not ready: %w (last client error: %v)", name, err, lastErr)
		}
		attempt, cancelAttempt := context.WithTimeout(ctx, 5*time.Second)
		lastErr = probe(attempt)
		cancelAttempt()
		if lastErr == nil {
			return nil
		}
		var missing *exec.Error
		var pathErr *os.PathError
		if errors.As(lastErr, &missing) || errors.As(lastErr, &pathErr) {
			return lastErr
		}
		timer := time.NewTimer(800 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%s not ready: %w (last client error: %v)", name, ctx.Err(), lastErr)
		case <-timer.C:
		}
	}
}

func prepareMySQL(ctx context.Context, cfg app.Config, engine string) (mysqlTarget, error) {
	t, ok := resolveMySQLTarget(cfg, engine)
	if !ok {
		return t, fmt.Errorf("unknown engine: %s", engine)
	}
	var err error
	t.service, err = ensureServiceUpContext(ctx, cfg, t.service)
	if err != nil {
		return t, err
	}
	err = waitDatabaseReady(ctx, engine, func(ctx context.Context) error {
		_, err := mysqlQuery(ctx, cfg, t, "SELECT 1;", "authenticate database")
		return err
	})
	return t, err
}

func preparePostgres(ctx context.Context, cfg app.Config) (pgTarget, error) {
	t, _ := resolvePGTarget(cfg, "postgres")
	var err error
	t.service, err = ensureServiceUpContext(ctx, cfg, t.service)
	if err != nil {
		return t, err
	}
	err = waitDatabaseReady(ctx, "postgres", func(ctx context.Context) error {
		_, err := postgresQuery(ctx, cfg, t, "SELECT 1;", "authenticate database")
		return err
	})
	return t, err
}

func CreateMySQLDBForEngineContext(ctx context.Context, cfg app.Config, engine, dbName, user, pass string) error {
	engine = normalizeDBEngine(engine)
	t, err := prepareMySQL(ctx, cfg, engine)
	if err != nil {
		return err
	}
	// Set string-literal semantics explicitly so passwords containing backslashes
	// or quotes retain their exact value regardless of the server's sql_mode.
	sql := strings.Join([]string{
		"SET SESSION sql_mode = 'NO_BACKSLASH_ESCAPES';",
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", mysqlIdent(dbName)),
		fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'%%' IDENTIFIED BY %s;", mysqlString(user), mysqlString(pass)),
		fmt.Sprintf("ALTER USER %s@'%%' IDENTIFIED BY %s;", mysqlString(user), mysqlString(pass)),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'%%';", mysqlIdent(dbName), mysqlString(user)),
	}, "\n")
	_, err = mysqlQuery(ctx, cfg, t, sql, "create database and user", pass)
	if err == nil {
		log.Info().Msgf("DB ready (engine=%s) db=%s user=%s", engine, dbName, user)
	}
	return err
}

func CreateMySQLDBForEngine(cfg app.Config, engine, dbName, user, pass string) error {
	return CreateMySQLDBForEngineContext(context.Background(), cfg, engine, dbName, user, pass)
}

func CreateMySQLDB(cfg app.Config, dbName, user, pass string) error {
	return CreateMySQLDBForEngine(cfg, "mysql-8.4", dbName, user, pass)
}

func CreatePostgresDBContext(ctx context.Context, cfg app.Config, dbName, user, pass string) error {
	t, err := preparePostgres(ctx, cfg)
	if err != nil {
		return err
	}
	// Quote the complete PL/pgSQL body instead of using a fixed dollar delimiter,
	// which can occur in an otherwise valid password.
	body := fmt.Sprintf("BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = %s) THEN CREATE ROLE %s LOGIN PASSWORD %s; ELSE ALTER ROLE %s LOGIN PASSWORD %s; END IF; END;",
		pgString(user), pgIdent(user), pgString(pass), pgIdent(user), pgString(pass))
	_, err = postgresQuery(ctx, cfg, t, "SET standard_conforming_strings = on;\nDO "+pgString(body)+";", "create database role", pass)
	if err != nil {
		return err
	}
	out, err := postgresQuery(ctx, cfg, t, "SET standard_conforming_strings = on;\nSELECT 1 FROM pg_database WHERE datname="+pgString(dbName)+";", "check database")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return nil
	}
	// PostgreSQL CREATE DATABASE must run outside a transaction.
	_, err = postgresQuery(ctx, cfg, t, fmt.Sprintf("CREATE DATABASE %s OWNER %s;", pgIdent(dbName), pgIdent(user)), "create database")
	return err
}

func CreatePostgresDB(cfg app.Config, dbName, user, pass string) error {
	return CreatePostgresDBContext(context.Background(), cfg, dbName, user, pass)
}

func createDBContext(ctx context.Context, cfg app.Config, engine, dbName, user, pass string) error {
	engine = normalizeDBEngine(engine)
	switch engine {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12":
		return CreateMySQLDBForEngineContext(ctx, cfg, engine, dbName, user, pass)
	case "postgres":
		return CreatePostgresDBContext(ctx, cfg, dbName, user, pass)
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}

func createDB(cfg app.Config, engine, dbName, user, pass string) error {
	return createDBContext(context.Background(), cfg, engine, dbName, user, pass)
}

func DropMySQLDBForEngineContext(ctx context.Context, cfg app.Config, engine, dbName, user string) error {
	engine = normalizeDBEngine(engine)
	t, err := prepareMySQL(ctx, cfg, engine)
	if err != nil {
		return err
	}
	sql := "SET SESSION sql_mode = 'NO_BACKSLASH_ESCAPES';\n" + fmt.Sprintf("DROP DATABASE IF EXISTS %s;", mysqlIdent(dbName))
	if user != "" {
		sql += fmt.Sprintf("\nDROP USER IF EXISTS %s@'%%';", mysqlString(user))
	}
	_, err = mysqlQuery(ctx, cfg, t, sql, "drop database and user")
	return err
}

func DropMySQLDBForEngine(cfg app.Config, engine, dbName, user string) error {
	return DropMySQLDBForEngineContext(context.Background(), cfg, engine, dbName, user)
}

func DropMySQLDB(cfg app.Config, dbName, user string) error {
	return DropMySQLDBForEngine(cfg, "mysql-8.4", dbName, user)
}

func DropPostgresDBContext(ctx context.Context, cfg app.Config, dbName, user string) error {
	t, err := preparePostgres(ctx, cfg)
	if err != nil {
		return err
	}
	// FORCE closes active sessions atomically with the drop.
	if _, err = postgresQuery(ctx, cfg, t, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE);", pgIdent(dbName)), "drop database"); err != nil {
		return err
	}
	if user == "" {
		return nil
	}
	_, err = postgresQuery(ctx, cfg, t, fmt.Sprintf("DROP ROLE IF EXISTS %s;", pgIdent(user)), "drop database role")
	return err
}

func DropPostgresDB(cfg app.Config, dbName, user string) error {
	return DropPostgresDBContext(context.Background(), cfg, dbName, user)
}

func dropDBContext(ctx context.Context, cfg app.Config, engine, dbName, user string) error {
	engine = normalizeDBEngine(engine)
	switch engine {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12":
		return DropMySQLDBForEngineContext(ctx, cfg, engine, dbName, user)
	case "postgres":
		return DropPostgresDBContext(ctx, cfg, dbName, user)
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}

func dropDB(cfg app.Config, engine, dbName, user string) error {
	return dropDBContext(context.Background(), cfg, engine, dbName, user)
}

func openSQLDump(dumpFile string) (*os.File, error) {
	if strings.TrimSpace(dumpFile) == "" {
		return nil, fmt.Errorf("dump file is required")
	}
	f, err := os.Open(dumpFile)
	if err != nil {
		return nil, fmt.Errorf("open SQL dump: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("SQL dump must be a regular file")
	}
	if info.Size() == 0 {
		f.Close()
		return nil, fmt.Errorf("SQL dump is empty")
	}
	return f, nil
}

// ReplaceSQLContext validates and opens the dump before dropping any data. Keep
// this same file descriptor through import so a renamed path cannot replace it.
func ReplaceSQLContext(ctx context.Context, cfg app.Config, engine, dbName, dbUser, dbPass, dumpFile string) error {
	f, err := openSQLDump(dumpFile)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := dropDBContext(ctx, cfg, engine, dbName, dbUser); err != nil {
		return err
	}
	if err := createDBContext(ctx, cfg, engine, dbName, dbUser, dbPass); err != nil {
		return err
	}
	return importSQLReader(ctx, cfg, engine, dbName, f)
}

func ReplaceSQL(cfg app.Config, engine, dbName, dbUser, dbPass, dumpFile string) error {
	return ReplaceSQLContext(context.Background(), cfg, engine, dbName, dbUser, dbPass, dumpFile)
}

func ImportSQLContext(ctx context.Context, cfg app.Config, engine, dbName, dumpFile string) error {
	f, err := openSQLDump(dumpFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return importSQLReader(ctx, cfg, engine, dbName, f)
}

func ImportSQL(cfg app.Config, engine, dbName, dumpFile string) error {
	return ImportSQLContext(context.Background(), cfg, engine, dbName, dumpFile)
}

func importSQLReader(ctx context.Context, cfg app.Config, engine, dbName string, input io.Reader) error {
	engine = normalizeDBEngine(engine)
	switch engine {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12":
		t, err := prepareMySQL(ctx, cfg, engine)
		if err != nil {
			return err
		}
		_, err = commandOutput(mysqlCommand(ctx, cfg, t, dbName, input), "import SQL ("+engine+")", t.rootPass)
		return err
	case "postgres":
		t, err := preparePostgres(ctx, cfg)
		if err != nil {
			return err
		}
		_, err = commandOutput(postgresCommand(ctx, cfg, t, dbName, input), "import SQL (postgres)", t.superPass)
		return err
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}
