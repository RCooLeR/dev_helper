package cli

import (
	"context"
	"fmt"

	"devhelper/internal/app"
	"devhelper/internal/provision"

	"github.com/urfave/cli/v3"
)

func cmdCert(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "cert",
		Usage: "TLS cert utilities using mkcert",
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "mkcert -install",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := provision.CertInitContext(ctx, a.Cfg); err != nil {
						return err
					}
					_, err := fmt.Fprintln(cmd.Writer, "mkcert CA installed.")
					return err
				},
			},
			{
				Name:  "issue",
				Usage: "Generate cert+key for a domain",
				Flags: []cli.Flag{&cli.StringFlag{Name: "domain", Required: true}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := provision.CertIssueContext(ctx, a.Cfg, cmd.String("domain")); err != nil {
						return err
					}
					_, err := fmt.Fprintln(cmd.Writer, "cert generated.")
					return err
				},
			},
		},
	}
}
