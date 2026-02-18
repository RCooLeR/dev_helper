package cli

import (
	"encoding/json"
	"fmt"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/provision"
	"devhelper/internal/store"

	"github.com/urfave/cli/v2"
)

func cmdProject(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "project",
		Usage: "Manage projects",
		Subcommands: []*cli.Command{
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
				},
				Action: func(cctx *cli.Context) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					res, err := provision.Create(a, st, provision.CreateRequest{
						Company: cctx.String("company"),
						Project: cctx.String("project"),
						Type:    cctx.String("type"),
						Domain:  cctx.String("domain"),
						PHP:     cctx.String("php"),
						DB:      cctx.String("db"),
					})
					if err != nil {
						return err
					}
					b, _ := json.MarshalIndent(map[string]any{"project": res.Project, "warnings": res.Warnings}, "", "  ")
					fmt.Println(string(b))
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "List projects",
				Action: func(cctx *cli.Context) error {
					cfg := a.Cfg
					st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
					if err != nil {
						return err
					}
					b, _ := json.MarshalIndent(st.List(), "", "  ")
					fmt.Println(string(b))
					return nil
				},
			},
		},
	}
}
