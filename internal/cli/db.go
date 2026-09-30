package cli

import (
	"context"
	"fmt"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/provision"
	"devhelper/internal/store"

	"github.com/urfave/cli/v3"
)

func cmdDB(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "db",
		Usage: "DB utilities",
		Commands: []*cli.Command{
			{
				Name:  "ensure",
				Usage: "Ensure a saved project's database exists using its stored credentials",
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
					company, project := cmd.String("company"), cmd.String("project")
					if err := provision.EnsureProjectDBContext(ctx, cfg, st, company, project); err != nil {
						return err
					}
					return writeJSON(cmd, map[string]any{"ok": true, "company": company, "project": project})
				},
			},
			{
				Name:  "import",
				Usage: "Import .sql dump into project's DB",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "company", Required: true},
					&cli.StringFlag{Name: "project", Required: true},
					&cli.StringFlag{Name: "file", Required: true, TakesFile: true},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					unlock, err := st.LockOperations(ctx)
					if err != nil {
						return err
					}
					defer unlock()
					p, ok, err := st.Get(cmd.String("company"), cmd.String("project"))
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("project not found")
					}
					if err := provision.ValidateManagedDatabase(p); err != nil {
						return err
					}
					if p.DB == "none" {
						return fmt.Errorf("project has no DB")
					}
					return provision.ImportSQLContext(ctx, cfg, p.DB, p.DBName, cmd.String("file"))
				},
			},
		},
	}
}
