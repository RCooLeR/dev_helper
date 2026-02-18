package cli

import (
	"devhelper/internal/app"
	"github.com/urfave/cli/v2"
)

func Run(argv []string, a *app.App) error {
	cmd := &cli.App{
		Name:  "devhelper",
		Usage: "Local PHP dev environment helper (web UI + CLI)",
		Commands: []*cli.Command{
			cmdInit(a),
			cmdServe(a),
			cmdProject(a),
			cmdDB(a),
			cmdCert(a),
		},
	}
	return cmd.Run(argv)
}
