package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	// Runtime root (WSL path when running on Windows, regular host path on Linux/macOS)
	RootDir       string `json:"root_dir"`
	HostMirrorDir string `json:"host_mirror_dir,omitempty"` // Windows UNC path to WSL root (\\wsl$\\...); used for file operations
	WSLDistro     string `json:"wsl_distro,omitempty"`      // kept for future use; not required for DB ops

	// Project/runtime directories (WSL paths on Windows; host paths on Linux/macOS)
	AppsRoot          string `json:"apps_root"`
	DataRoot          string `json:"data_root"`
	NginxConfRoot     string `json:"nginx_conf_root"`
	NginxExternalRoot string `json:"nginx_external_root"`

	// Where docker-compose.yml lives (HOST path on Windows, e.g. E:\Development\projects)
	ComposeDir string `json:"compose_dir"`

	DefaultDomainPattern string `json:"default_domain_pattern"`

	// Docker compose service names (used to bring DB containers up).
	//
	// MySQL9Service is deliberately named after the "mysql9" compose service
	// instead of a concrete version. The image tag can move from 9.6 to 9.7
	// without changing the store schema or every CLI/API caller.
	MySQLService     string `json:"mysql_service"`
	MySQL9Service    string `json:"mysql9_service"`
	MariaDB10Service string `json:"mariadb10_service"`
	MariaDB12Service string `json:"mariadb12_service"`
	PostgresService  string `json:"postgres_service"`

	// DB access from HOST (Windows) via TCP
	MySQLHost     string `json:"mysql_host"`
	MySQLPort     int    `json:"mysql_port"`
	MySQL9Port    int    `json:"mysql9_port"`
	MariaDB10Port int    `json:"mariadb10_port"`
	MariaDB12Port int    `json:"mariadb12_port"`
	MySQLRootPass string `json:"mysql_root_pass"`
	MySQLCli      string `json:"mysql_cli"` // full path to mysql-8.4.exe (recommended) or empty to use PATH

	PostgresHost      string `json:"postgres_host"`
	PostgresPort      int    `json:"postgres_port"`
	PostgresSuperPass string `json:"postgres_super_pass"`
	PSQLCli           string `json:"psql_cli"` // full path to psql.exe (recommended) or empty to use PATH

	// Tools executed on HOST
	DockerCli string `json:"docker_cli"` // docker.exe or docker (PATH); default "docker"
	MkcertCli string `json:"mkcert_cli"` // mkcert.exe or mkcert (PATH); default "mkcert"

	StoreFile string `json:"store_file"`
	LogLevel  string `json:"log_level"`
}

func DefaultConfig(repoRoot string) Config {
	c := Config{
		DefaultDomainPattern: "<project>.<company>.local",
		MySQLService:         "mysql-8.4",
		MySQL9Service:        "mysql-9.7",
		MariaDB10Service:     "mariadb10",
		MariaDB12Service:     "mariadb12",
		PostgresService:      "postgres",
		MySQLHost:            "127.0.0.1",
		MySQLPort:            3384,
		MySQL9Port:           3396,
		MariaDB10Port:        33106,
		MariaDB12Port:        3312,
		MySQLRootPass:        "Passw0rd",
		PostgresHost:         "127.0.0.1",
		PostgresPort:         5434,
		PostgresSuperPass:    "Passw0rd",
		DockerCli:            "docker",
		MkcertCli:            "mkcert",
		LogLevel:             "debug",
	}

	// On Linux/macOS we keep everything inside the repo by default.
	c.RootDir = filepath.Join(repoRoot, "projects")
	c.ComposeDir = filepath.Join(repoRoot, "projects")
	c.AppsRoot = filepath.Join(c.RootDir, "apps")
	c.DataRoot = filepath.Join(c.RootDir, "data")
	c.NginxConfRoot = filepath.Join(c.RootDir, "containers", "nginx", "conf.d")
	c.NginxExternalRoot = filepath.Join(c.RootDir, "containers", "nginx", "external")
	c.StoreFile = filepath.Join(c.DataRoot, "devhelper.store.json")

	if runtime.GOOS == "windows" {
		// Runtime lives in WSL filesystem for performance.
		c.WSLDistro = "Ubuntu"
		c.RootDir = "/data/projects"
		c.HostMirrorDir = `\\wsl$\Ubuntu\data\projects`

		c.AppsRoot = c.RootDir + "/apps"
		c.DataRoot = c.RootDir + "/data"
		c.NginxConfRoot = c.RootDir + "/containers/nginx/conf.d"
		c.NginxExternalRoot = c.RootDir + "/containers/nginx/external"
		c.StoreFile = filepath.Join(repoRoot, "projects", "data", "devhelper.store.json")

		// Compose file lives on Windows side in the repo.
		c.ComposeDir = filepath.Join(repoRoot, "projects")
	}

	return c
}

func ConfigPath(repoRoot string) string {
	// Keep config next to the executable/repo (same path on all OS for simplicity).
	return filepath.Join(repoRoot, ".devhelper-config.json")
}

func LoadOrDefault(repoRoot string) (Config, bool, error) {
	p := ConfigPath(repoRoot)
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(repoRoot), false, nil
		}
		return Config{}, false, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, false, err
	}
	// Apply defaults for new fields if they are missing in older configs.
	def := DefaultConfig(repoRoot)
	if c.DefaultDomainPattern == "" {
		c.DefaultDomainPattern = def.DefaultDomainPattern
	}
	if c.MySQLService == "" {
		c.MySQLService = def.MySQLService
	}
	if c.MySQL9Service == "" || c.MySQL9Service == "mysql-9.6" {
		c.MySQL9Service = def.MySQL9Service
	}
	if c.MariaDB10Service == "" {
		c.MariaDB10Service = def.MariaDB10Service
	}
	if c.MariaDB12Service == "" {
		c.MariaDB12Service = def.MariaDB12Service
	}
	if c.PostgresService == "" {
		c.PostgresService = def.PostgresService
	}
	if c.MySQLHost == "" {
		c.MySQLHost = def.MySQLHost
	}
	if c.MySQLPort == 0 {
		c.MySQLPort = def.MySQLPort
	}
	if c.MySQL9Port == 0 {
		c.MySQL9Port = def.MySQL9Port
	}
	if c.MariaDB10Port == 0 {
		c.MariaDB10Port = def.MariaDB10Port
	}
	if c.MariaDB12Port == 0 {
		c.MariaDB12Port = def.MariaDB12Port
	}
	if c.PostgresHost == "" {
		c.PostgresHost = def.PostgresHost
	}
	if c.PostgresPort == 0 {
		c.PostgresPort = def.PostgresPort
	}
	if c.DockerCli == "" {
		c.DockerCli = def.DockerCli
	}
	if c.MkcertCli == "" {
		c.MkcertCli = def.MkcertCli
	}
	if c.LogLevel == "" {
		c.LogLevel = def.LogLevel
	}
	return c, true, nil
}

func Save(repoRoot string, c Config) error {
	p := ConfigPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0o644)
}
