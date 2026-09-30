package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"devhelper/internal/app"
	"devhelper/internal/platform"

	"github.com/urfave/cli/v3"
)

func cmdInit(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize runtime config + write projects/.env next to docker-compose.yml",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "wsl-root", Value: a.Cfg.RootDir},
			&cli.StringFlag{Name: "host-mirror", Value: a.Cfg.HostMirrorDir},
			&cli.StringFlag{Name: "compose-dir", Value: a.Cfg.ComposeDir},
			&cli.StringFlag{Name: "mysql-8.4-cli", Value: a.Cfg.MySQLCli},
			&cli.StringFlag{Name: "psql-cli", Value: a.Cfg.PSQLCli},
			&cli.StringFlag{Name: "mkcert-cli", Value: a.Cfg.MkcertCli},
			&cli.StringFlag{Name: "docker-cli", Value: a.Cfg.DockerCli},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg := a.Cfg

			cfg.RootDir = cmd.String("wsl-root")
			cfg.HostMirrorDir = cmd.String("host-mirror")
			cfg.ComposeDir = cmd.String("compose-dir")
			cfg.MySQLCli = cmd.String("mysql-8.4-cli")
			cfg.PSQLCli = cmd.String("psql-cli")
			cfg.MkcertCli = cmd.String("mkcert-cli")
			cfg.DockerCli = cmd.String("docker-cli")

			// Normalize derived WSL paths (Windows uses WSL runtime root by design).
			if runtime.GOOS == "windows" {
				cfg.AppsRoot = cfg.RootDir + "/apps"
				cfg.DataRoot = cfg.RootDir + "/data"
				cfg.NginxConfRoot = cfg.RootDir + "/containers/nginx/conf.d"
				cfg.NginxExternalRoot = cfg.RootDir + "/containers/nginx/external"
				cfg.StoreFile = filepath.Join(cfg.ComposeDir, "data", "devhelper.store.json")
			} else {
				// On Linux/macOS keep within repo unless user changed it.
				if cfg.RootDir == "" {
					cfg.RootDir = filepath.Join(a.RepoRoot, "projects")
				}
				if cfg.ComposeDir == "" {
					cfg.ComposeDir = filepath.Join(a.RepoRoot, "projects")
				}
				cfg.AppsRoot = filepath.Join(cfg.RootDir, "apps")
				cfg.DataRoot = filepath.Join(cfg.RootDir, "data")
				cfg.NginxConfRoot = filepath.Join(cfg.RootDir, "containers", "nginx", "conf.d")
				cfg.NginxExternalRoot = filepath.Join(cfg.RootDir, "containers", "nginx", "external")
				cfg.StoreFile = filepath.Join(cfg.DataRoot, "devhelper.store.json")
			}

			// Create runtime dirs (on Windows via \\wsl$ mirror, no WSL exec needed).
			dirs := []string{
				cfg.RootDir,
				cfg.AppsRoot,
				cfg.DataRoot,
				cfg.NginxConfRoot,
				cfg.NginxExternalRoot,
				cfg.NginxExternalRoot + "/certs",
			}
			for _, d := range dirs {
				p := d
				if runtime.GOOS == "windows" {
					p = platform.WSLToHost(cfg, d)
				}
				if err := os.MkdirAll(p, 0o755); err != nil {
					return err
				}
			}

			// docker-compose.yml is intentionally local-only because developers
			// often add private services, mounts, or tunnels. A fresh clone still
			// needs a starting point, so init copies the tracked example only when
			// the local compose file does not already exist.
			if err := ensureComposeFile(a.RepoRoot, cfg.ComposeDir); err != nil {
				return err
			}

			// Write .env next to docker-compose.yml (ComposeDir).
			if err := platform.WriteEnv(cfg); err != nil {
				return err
			}

			// Activate the configuration only when the Compose file and .env
			// were successfully prepared. A setup failure keeps the old config.
			if err := app.Save(a.RepoRoot, cfg); err != nil {
				return err
			}
			a.Cfg = cfg

			_, err := fmt.Fprintf(cmd.Writer, "Initialized.\nWSL root: %s\nHost mirror: %s\nCompose dir: %s\n", cfg.RootDir, cfg.HostMirrorDir, cfg.ComposeDir)
			return err
		},
	}
}

func ensureComposeFile(repoRoot, composeDir string) error {
	dst := filepath.Join(composeDir, "docker-compose.yml")
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	src := filepath.Join(repoRoot, "projects", "docker-compose.example.yml")
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read compose example: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}
