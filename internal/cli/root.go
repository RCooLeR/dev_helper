package cli

import (
	"context"

	"devhelper/internal/app"

	"github.com/urfave/cli/v3"
)

func Run(argv []string, a *app.App) error {
	return RunContext(context.Background(), argv, a)
}

// RunContext runs the CLI using the caller's cancellation and deadline.
func RunContext(ctx context.Context, argv []string, a *app.App) error {
	return newCommand(a).Run(ctx, argv)
}

func newCommand(a *app.App) *cli.Command {
	return &cli.Command{
		Name:                  "devhelper",
		Usage:                 "Local PHP dev environment helper (web UI + CLI)",
		EnableShellCompletion: true,
		Suggest:               true,
		Before: func(ctx context.Context, _ *cli.Command) (context.Context, error) {
			return ctx, ctx.Err()
		},
		// Errors belong to our caller, which logs them and selects the exit code.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Commands: []*cli.Command{
			cmdInit(a),
			cmdServe(a),
			cmdProject(a),
			cmdDB(a),
			cmdCert(a),
		},
	}
}
