package cli

import (
	"fmt"

	"devhelper/internal/app"
	"devhelper/internal/provision"

	"github.com/urfave/cli/v2"
)

func cmdCert(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "cert",
		Usage: "TLS cert utilities using mkcert",
		Subcommands: []*cli.Command{
			{
				Name:  "init",
				Usage: "mkcert -install",
				Action: func(cctx *cli.Context) error {
					if err := provision.CertInit(a.Cfg); err != nil {
						return err
					}
					fmt.Println("mkcert CA installed.")
					return nil
				},
			},
			{
				Name:  "issue",
				Usage: "Generate cert+key for a domain",
				Flags: []cli.Flag{&cli.StringFlag{Name: "domain", Required: true}},
				Action: func(cctx *cli.Context) error {
					if err := provision.CertIssue(a.Cfg, cctx.String("domain")); err != nil {
						return err
					}
					fmt.Println("cert generated.")
					return nil
				},
			},
		},
	}
}
