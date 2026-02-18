package provision

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"devhelper/internal/app"
)

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

func ensureServiceUp(cfg app.Config, service string) error {
	if strings.TrimSpace(cfg.ComposeDir) == "" {
		return fmt.Errorf("compose_dir is empty (run: devhelper init)")
	}
	_, err := dockerCompose(cfg, "up", "-d", service)
	return err
}

func waitTCP(host string, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", host, port)
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

func mysqlPing(cfg app.Config) error {
	sql := "SELECT 1;"
	args := []string{
		"--protocol=tcp",
		"-h", cfg.MySQLHost,
		"-P", fmt.Sprintf("%d", cfg.MySQLPort),
		"-uroot",
		"-p" + cfg.MySQLRootPass,
		"-e", sql,
	}
	cmd := exec.Command(mysqlExe(cfg), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysql ping failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func psqlPing(cfg app.Config) error {
	args := []string{
		"-h", cfg.PostgresHost,
		"-p", fmt.Sprintf("%d", cfg.PostgresPort),
		"-U", "postgres",
		"-d", "postgres",
		"-c", "SELECT 1;",
	}
	cmd := exec.Command(psqlExe(cfg), args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.PostgresSuperPass)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql ping failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitMySQLReady(cfg app.Config) error {
	if err := waitTCP(cfg.MySQLHost, cfg.MySQLPort, 60*time.Second); err != nil {
		return err
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if err := mysqlPing(cfg); err == nil {
			return nil
		}
		time.Sleep(800 * time.Millisecond)
	}
	return fmt.Errorf("mysql auth not ready on %s:%d (check MYSQL_ROOT_HOST and password)", cfg.MySQLHost, cfg.MySQLPort)
}

func waitPostgresReady(cfg app.Config) error {
	if err := waitTCP(cfg.PostgresHost, cfg.PostgresPort, 60*time.Second); err != nil {
		return err
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if err := psqlPing(cfg); err == nil {
			return nil
		}
		time.Sleep(800 * time.Millisecond)
	}
	return fmt.Errorf("postgres auth not ready on %s:%d (check password)", cfg.PostgresHost, cfg.PostgresPort)
}

func CreateMySQLDB(cfg app.Config, dbName, user, pass string) error {
	if err := ensureServiceUp(cfg, cfg.MySQLService); err != nil {
		return err
	}
	if err := waitMySQLReady(cfg); err != nil {
		return err
	}
	sql := strings.Join([]string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';", user, pass),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%';", dbName, user),
		"FLUSH PRIVILEGES;",
	}, " ")

	args := []string{
		"--protocol=tcp",
		"-h", cfg.MySQLHost,
		"-P", fmt.Sprintf("%d", cfg.MySQLPort),
		"-uroot",
		"-p" + cfg.MySQLRootPass,
		"-e", sql,
	}
	cmd := exec.Command(mysqlExe(cfg), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mysql create failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func CreatePostgresDB(cfg app.Config, dbName, user, pass string) error {
	if err := ensureServiceUp(cfg, cfg.PostgresService); err != nil {
		return err
	}
	if err := waitPostgresReady(cfg); err != nil {
		return err
	}

	// Create role if missing + create DB if missing.
	sqlRole := fmt.Sprintf("DO $$BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN CREATE ROLE %s LOGIN PASSWORD '%s'; END IF; END$$;", user, user, pass)
	sqlDB := fmt.Sprintf("DO $$BEGIN IF NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s') THEN EXECUTE format('CREATE DATABASE %%I OWNER %%I', '%s', '%s'); END IF; END$$;", dbName, dbName, user)

	cmd := exec.Command(psqlExe(cfg),
		"-h", cfg.PostgresHost,
		"-p", fmt.Sprintf("%d", cfg.PostgresPort),
		"-U", "postgres",
		"-d", "postgres",
		"-v", "ON_ERROR_STOP=1",
		"-c", sqlRole,
		"-c", sqlDB,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.PostgresSuperPass)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("postgres create failed: %w; %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func createDB(cfg app.Config, engine, dbName, user, pass string) error {
	engine = strings.ToLower(strings.TrimSpace(engine))
	switch engine {
	case "mysql":
		return CreateMySQLDB(cfg, dbName, user, pass)
	case "postgres":
		return CreatePostgresDB(cfg, dbName, user, pass)
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}
}

func ImportSQL(cfg app.Config, engine, dbName, dumpFile string) error {
	engine = strings.ToLower(strings.TrimSpace(engine))
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
	case "mysql":
		if err := ensureServiceUp(cfg, cfg.MySQLService); err != nil {
			return err
		}
		if err := waitMySQLReady(cfg); err != nil {
			return err
		}
		cmd := exec.Command(mysqlExe(cfg),
			"--protocol=tcp",
			"-h", cfg.MySQLHost,
			"-P", fmt.Sprintf("%d", cfg.MySQLPort),
			"-uroot",
			"-p"+cfg.MySQLRootPass,
			dbName,
		)
		cmd.Stdin = f
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mysql import failed: %w; %s", err, strings.TrimSpace(string(out)))
		}
		return nil

	case "postgres":
		if err := ensureServiceUp(cfg, cfg.PostgresService); err != nil {
			return err
		}
		if err := waitPostgresReady(cfg); err != nil {
			return err
		}
		cmd := exec.Command(psqlExe(cfg),
			"-h", cfg.PostgresHost,
			"-p", fmt.Sprintf("%d", cfg.PostgresPort),
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
