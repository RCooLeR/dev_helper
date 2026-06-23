package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"devhelper/internal/app"
	"devhelper/internal/platform"

	"github.com/urfave/cli/v2"
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
		Action: func(cctx *cli.Context) error {
			cfg := a.Cfg

			cfg.RootDir = cctx.String("wsl-root")
			cfg.HostMirrorDir = cctx.String("host-mirror")
			cfg.ComposeDir = cctx.String("compose-dir")
			// Keep compose_dir stable and derived from the repo location unless the user explicitly overrides it.
			// This prevents accidental "compose_dir points somewhere else" when the app is started from a different CWD.
			if runtime.GOOS == "windows" && !cctx.IsSet("compose-dir") {
				cfg.ComposeDir = filepath.Join(a.RepoRoot, "projects")
			}
			cfg.MySQLCli = cctx.String("mysql-8.4-cli")
			cfg.PSQLCli = cctx.String("psql-cli")
			cfg.MkcertCli = cctx.String("mkcert-cli")
			cfg.DockerCli = cctx.String("docker-cli")

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

			if err := app.Save(a.RepoRoot, cfg); err != nil {
				return err
			}
			a.Cfg = cfg

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

			fmt.Println("Initialized.")
			fmt.Println("WSL root:", cfg.RootDir)
			fmt.Println("Host mirror:", cfg.HostMirrorDir)
			fmt.Println("Compose dir:", cfg.ComposeDir)
			return nil
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
