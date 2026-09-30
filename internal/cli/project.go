package cli

import (
	"context"
	"encoding/json"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/provision"
	"devhelper/internal/store"

	"github.com/urfave/cli/v3"
)

func cmdProject(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "project",
		Usage: "Manage projects",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create a project (dirs, nginx conf, hosts, db)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "company", Required: true},
					&cli.StringFlag{Name: "project", Required: true},
					&cli.StringFlag{Name: "type", Value: "other"},
					&cli.StringFlag{Name: "domain"},
					&cli.StringFlag{Name: "php", Value: "8.3"},
					&cli.StringFlag{Name: "db", Value: "none"},
					&cli.StringFlag{Name: "import-file", TakesFile: true},
					&cli.BoolFlag{Name: "import"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					res, err := provision.CreateContext(ctx, a, st, provision.CreateRequest{
						Company:        cmd.String("company"),
						Project:        cmd.String("project"),
						Type:           cmd.String("type"),
						Domain:         cmd.String("domain"),
						PHP:            cmd.String("php"),
						DB:             cmd.String("db"),
						ImportFile:     cmd.String("import-file"),
						ImportOnCreate: cmd.Bool("import"),
					})
					if err != nil {
						return err
					}
					return writeJSON(cmd, map[string]any{"project": res.Project, "warnings": res.Warnings})
				},
			},
			{
				Name:  "drop",
				Usage: "Drop a project (dirs, nginx conf, certs, hosts, db)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "company", Required: true},
					&cli.StringFlag{Name: "project", Required: true},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					res, err := provision.DropContext(ctx, a, st, provision.DropRequest{
						Company: cmd.String("company"),
						Project: cmd.String("project"),
					})
					if err != nil {
						return err
					}
					return writeJSON(cmd, map[string]any{"ok": true, "warnings": res.Warnings})
				},
			},
			{
				Name:  "list",
				Usage: "List projects",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					projects, err := st.List()
					if err != nil {
						return err
					}
					return writeJSON(cmd, projects)
				},
			},
		},
	}
}

func writeJSON(cmd *cli.Command, value any) error {
	encoder := json.NewEncoder(cmd.Writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
