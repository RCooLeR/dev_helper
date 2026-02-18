package cli

import (
	"fmt"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/provision"
	"devhelper/internal/store"

	"github.com/urfave/cli/v2"
)

func cmdDB(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "db",
		Usage: "DB utilities",
		Subcommands: []*cli.Command{
			{
				Name:  "import",
				Usage: "Import .sql dump into project's DB",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "company", Required: true},
					&cli.StringFlag{Name: "project", Required: true},
					&cli.StringFlag{Name: "file", Required: true},
				},
				Action: func(cctx *cli.Context) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					p, ok := st.Get(cctx.String("company"), cctx.String("project"))
					if !ok {
						return fmt.Errorf("project not found")
					}
					if p.DB == "none" {
						return fmt.Errorf("project has no DB")
					}
					return provision.ImportSQL(cfg, p.DB, p.DBName, cctx.String("file"))
				},
			},
		},
	}
}
