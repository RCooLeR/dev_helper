package provision

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"devhelper/internal/app"

	"github.com/rs/zerolog/log"
)

type mysqlTarget struct {
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
		return mysqlTarget{service: cfg.MariaDB10Service, host: cfg.MySQLHost, port: cfg.MariaDB10Port, rootPass: cfg.MySQLRootPass}, true
	case "mariadb12":
		return mysqlTarget{service: cfg.MariaDB12Service, host: cfg.MySQLHost, port: cfg.MariaDB12Port, rootPass: cfg.MySQLRootPass}, true
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

func dockerCompose(cfg app.Config, args ...string) (string, error) {
	exe := strings.TrimSpace(cfg.DockerCli)
	if exe == "" {
		exe = "docker"
	}
	cmd := exec.Command(exe, append([]string{"compose"}, args...)...)
	cmd.Dir = cfg.ComposeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker compose %s failed: %w; output=%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// ensureServiceUp starts only the service needed for the requested operation.
// This keeps "create project with MySQL 8.4" from paying the cost of starting
// Postgres, Redis, every PHP version, and other services in docker-compose.yml.
func ensureServiceUp(cfg app.Config, service string) error {
	if strings.TrimSpace(cfg.ComposeDir) == "" {
		return fmt.Errorf("compose_dir is empty (run: devhelper init)")
	}
	_, err := dockerCompose(cfg, "up", "-d", service)
	return err
}

func waitTCP(host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("tcp not ready: %s", addr)
}

func mysqlExe(cfg app.Config) string {
	if strings.TrimSpace(cfg.MySQLCli) != "" {
		return cfg.MySQLCli
	}
	return "mysql"
}

func psqlExe(cfg app.Config) string {
	if strings.TrimSpace(cfg.PSQLCli) != "" {
		return cfg.PSQLCli
	}
	return "psql"
}

// mysqlPingTarget/psqlPingTarget verify authentication after TCP starts
// accepting connections. Container ports can open before the database has
// finished bootstrapping users, so a socket check alone is not enough.
func mysqlPingTarget(cfg app.Config, t mysqlTarget) error {
	sql := "SELECT 1;"
	args := []string{
		"--protocol=tcp",
		"-h", t.host,
		"-P", fmt.Sprintf("%d", t.port),
		"-uroot",
		"-p" + t.rootPass,
		"-e", sql,
	}
	cmd := exec.Command(mysqlExe(cfg), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysql-8.4 ping failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func psqlPingTarget(cfg app.Config, t pgTarget) error {
	args := []string{
		"-h", t.host,
		"-p", fmt.Sprintf("%d", t.port),
		"-U", "postgres",
		"-d", "postgres",
		"-c", "SELECT 1;",
	}
	cmd := exec.Command(psqlExe(cfg), args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql ping failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func psqlPing(cfg app.Config) error {
	t, _ := resolvePGTarget(cfg, "postgres")
	return psqlPingTarget(cfg, t)
}

func waitMySQLReady(cfg app.Config, t mysqlTarget) error {
	if err := waitTCP(t.host, t.port, 60*time.Second); err != nil {
		return err
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if err := mysqlPingTarget(cfg, t); err == nil {
			return nil
		}
		time.Sleep(800 * time.Millisecond)
	}
	return fmt.Errorf("mysql-8.4 auth not ready on %s:%d (check root host and password)", t.host, t.port)
}

func waitPostgresReady(cfg app.Config, t pgTarget) error {
	if err := waitTCP(t.host, t.port, 60*time.Second); err != nil {
		return err
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if err := psqlPingTarget(cfg, t); err == nil {
			return nil
		}
		time.Sleep(800 * time.Millisecond)
	}
	return fmt.Errorf("postgres auth not ready on %s:%d (check password)", t.host, t.port)
}

func CreateMySQLDBForEngine(cfg app.Config, engine, dbName, user, pass string) error {
	engine = normalizeDBEngine(engine)
	t, ok := resolveMySQLTarget(cfg, engine)
	if !ok {
		return fmt.Errorf("unknown engine: %s", engine)
	}
	if strings.TrimSpace(t.service) == "" {
		return fmt.Errorf("service name missing for engine %s", engine)
	}
	if err := ensureServiceUp(cfg, t.service); err != nil {
		return err
	}
	if err := waitMySQLReady(cfg, t); err != nil {
		return err
	}
	sql := strings.Join([]string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", mysqlIdent(dbName)),
		fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'%%' IDENTIFIED BY %s;", mysqlString(user), mysqlString(pass)),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'%%';", mysqlIdent(dbName), mysqlString(user)),
		"FLUSH PRIVILEGES;",
	}, " ")
	log.Info().Msgf("Creating MySQL/MariaDB database '%s' and user '%s' (engine=%s)", dbName, user, engine)
	args := []string{
		"--protocol=tcp",
		"-h", t.host,
		"-P", fmt.Sprintf("%d", t.port),
		"-uroot",
		"-p" + t.rootPass,
		"-e", sql,
	}
	cmd := exec.Command(mysqlExe(cfg), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("MySQL create failed: %s", strings.TrimSpace(string(out)))
		return fmt.Errorf("mysql-8.4 create failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	log.Info().Msgf("DB ready (engine=%s) db=%s user=%s", engine, dbName, user)
	return nil
}

func CreateMySQLDB(cfg app.Config, dbName, user, pass string) error {
	return CreateMySQLDBForEngine(cfg, "mysql-8.4", dbName, user, pass)
}

func CreatePostgresDB(cfg app.Config, dbName, user, pass string) error {
	t, _ := resolvePGTarget(cfg, "postgres")
	if err := ensureServiceUp(cfg, t.service); err != nil {
		return err
	}
	if err := waitPostgresReady(cfg, t); err != nil {
		return err
	}
	log.Info().Msgf("Creating Postgres database '%s' and user '%s'", dbName, user)
	// Create role if missing. PostgreSQL supports this safely inside a DO block,
	// unlike CREATE DATABASE, which must be executed as a top-level statement.
	sqlRole := fmt.Sprintf(
		"DO $$BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = %s) THEN CREATE ROLE %s LOGIN PASSWORD %s; END IF; END$$;",
		pgString(user), pgIdent(user), pgString(pass),
	)
	cmdRole := exec.Command(psqlExe(cfg),
		"-h", t.host,
		"-p", fmt.Sprintf("%d", t.port),
		"-U", "postgres",
		"-d", "postgres",
		"-v", "ON_ERROR_STOP=1",
		"-c", sqlRole,
	)
	cmdRole.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	out, err := cmdRole.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("Postgres create role failed: %s", strings.TrimSpace(string(out)))
		return fmt.Errorf("postgres create role failed: %w; %s", err, strings.TrimSpace(string(out)))
	}

	// CREATE DATABASE cannot run inside a function/transaction, so we do the
	// existence check in Go and then run plain CREATE DATABASE separately.
	cmdCheck := exec.Command(psqlExe(cfg),
		"-h", t.host,
		"-p", fmt.Sprintf("%d", t.port),
		"-U", "postgres",
		"-d", "postgres",
		"-tAc", fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname=%s", pgString(dbName)),
	)
	cmdCheck.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	chkOut, err := cmdCheck.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("Postgres db exists check failed: %s", strings.TrimSpace(string(chkOut)))
		return fmt.Errorf("postgres db exists check failed: %w; %s", err, strings.TrimSpace(string(chkOut)))
	}
	if strings.TrimSpace(string(chkOut)) != "" {
		return nil
	}

	cmdDB := exec.Command(psqlExe(cfg),
		"-h", t.host,
		"-p", fmt.Sprintf("%d", t.port),
		"-U", "postgres",
		"-d", "postgres",
		"-v", "ON_ERROR_STOP=1",
		"-c", fmt.Sprintf("CREATE DATABASE %s OWNER %s", pgIdent(dbName), pgIdent(user)),
	)
	cmdDB.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	out, err = cmdDB.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msgf("Postgres create db failed: %s", strings.TrimSpace(string(out)))
		return fmt.Errorf("postgres create db failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	log.Info().Msgf("Postgres database '%s' and user '%s' created or already exist", dbName, user)
	return nil
}

func createDB(cfg app.Config, engine, dbName, user, pass string) error {
	engine = normalizeDBEngine(engine)
	switch engine {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12":
		return CreateMySQLDBForEngine(cfg, engine, dbName, user, pass)
	case "postgres":
		return CreatePostgresDB(cfg, dbName, user, pass)
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}

func DropMySQLDBForEngine(cfg app.Config, engine, dbName, user string) error {
	engine = normalizeDBEngine(engine)
	t, ok := resolveMySQLTarget(cfg, engine)
	if !ok {
		return fmt.Errorf("unknown engine: %s", engine)
	}
	if strings.TrimSpace(t.service) == "" {
		return fmt.Errorf("service name missing for engine %s", engine)
	}
	if err := ensureServiceUp(cfg, t.service); err != nil {
		return err
	}
	if err := waitMySQLReady(cfg, t); err != nil {
		return err
	}
	sql := strings.Join([]string{
		fmt.Sprintf("DROP DATABASE IF EXISTS %s;", mysqlIdent(dbName)),
		fmt.Sprintf("DROP USER IF EXISTS %s@'%%';", mysqlString(user)),
		"FLUSH PRIVILEGES;",
	}, " ")
	args := []string{
		"--protocol=tcp",
		"-h", t.host,
		"-P", fmt.Sprintf("%d", t.port),
		"-uroot",
		"-p" + t.rootPass,
		"-e", sql,
	}
	cmd := exec.Command(mysqlExe(cfg), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysql-8.4 drop failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func DropMySQLDB(cfg app.Config, dbName, user string) error {
	return DropMySQLDBForEngine(cfg, "mysql-8.4", dbName, user)
}

func DropPostgresDB(cfg app.Config, dbName, user string) error {
	t, _ := resolvePGTarget(cfg, "postgres")
	if err := ensureServiceUp(cfg, t.service); err != nil {
		return err
	}
	if err := waitPostgresReady(cfg, t); err != nil {
		return err
	}

	baseArgs := []string{
		"-h", t.host,
		"-p", fmt.Sprintf("%d", t.port),
		"-U", "postgres",
		"-d", "postgres",
		"-v", "ON_ERROR_STOP=1",
	}

	// Terminate any open connections to the DB (required before DROP DATABASE).
	cmdTerm := exec.Command(psqlExe(cfg), append(baseArgs,
		"-c", fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=%s AND pid <> pg_backend_pid();", pgString(dbName)),
	)...)
	cmdTerm.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	_, _ = cmdTerm.CombinedOutput() // best-effort

	// Drop database (cannot run inside a function/transaction).
	cmdDB := exec.Command(psqlExe(cfg), append(baseArgs,
		"-c", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", pgIdent(dbName)),
	)...)
	cmdDB.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	out, err := cmdDB.CombinedOutput()
	if err != nil {
		return fmt.Errorf("postgres drop db failed: %w; %s", err, strings.TrimSpace(string(out)))
	}

	// Drop role if exists.
	sqlRole := fmt.Sprintf("DO $$BEGIN IF EXISTS (SELECT FROM pg_roles WHERE rolname=%s) THEN DROP ROLE %s; END IF; END$$;", pgString(user), pgIdent(user))
	cmdRole := exec.Command(psqlExe(cfg), append(baseArgs,
		"-c", sqlRole,
	)...)
	cmdRole.Env = append(os.Environ(), "PGPASSWORD="+t.superPass)
	out, err = cmdRole.CombinedOutput()
	if err != nil {
		return fmt.Errorf("postgres drop role failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func dropDB(cfg app.Config, engine, dbName, user string) error {
	engine = normalizeDBEngine(engine)
	switch engine {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12":
		return DropMySQLDBForEngine(cfg, engine, dbName, user)
	case "postgres":
		return DropPostgresDB(cfg, dbName, user)
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}

// ReplaceSQL drops the project's DB (and role/user), recreates it, and imports the given SQL dump.
// This is useful when you want to "reset" a project to a known state.
func ReplaceSQL(cfg app.Config, engine, dbName, dbUser, dbPass, dumpFile string) error {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if err := dropDB(cfg, engine, dbName, dbUser); err != nil {
		return err
	}
	if err := createDB(cfg, engine, dbName, dbUser, dbPass); err != nil {
		return err
	}
	return ImportSQL(cfg, engine, dbName, dumpFile)
}

func ImportSQL(cfg app.Config, engine, dbName, dumpFile string) error {
	engine = normalizeDBEngine(engine)
	dumpFile = strings.TrimSpace(dumpFile)
	if dumpFile == "" {
		return fmt.Errorf("dump file is required")
	}

	f, err := os.Open(dumpFile)
	if err != nil {
		return err
	}
	defer f.Close()

	switch engine {
	case "mysql-8.4", "mysql-9.7", "mariadb10", "mariadb12":
		t, ok := resolveMySQLTarget(cfg, engine)
		if !ok {
			return fmt.Errorf("unknown engine: %s", engine)
		}
		if err := ensureServiceUp(cfg, t.service); err != nil {
			return err
		}
		if err := waitMySQLReady(cfg, t); err != nil {
			return err
		}
		cmd := exec.Command(mysqlExe(cfg),
			"--protocol=tcp",
			"-h", t.host,
			"-P", fmt.Sprintf("%d", t.port),
			"-uroot",
			"-p"+t.rootPass,
			dbName,
		)
		cmd.Stdin = f
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mysql-8.4 import failed: %w; %s", err, strings.TrimSpace(string(out)))
		}
		return nil

	case "postgres":
		t, _ := resolvePGTarget(cfg, "postgres")
		if err := ensureServiceUp(cfg, t.service); err != nil {
			return err
		}
		if err := waitPostgresReady(cfg, t); err != nil {
			return err
		}
		cmd := exec.Command(psqlExe(cfg),
			"-h", t.host,
			"-p", fmt.Sprintf("%d", t.port),
			"-U", "postgres",
			"-d", dbName,
			"-v", "ON_ERROR_STOP=1",
			"-f", dumpFile,
		)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.PostgresSuperPass)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("postgres import failed: %w; %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}
