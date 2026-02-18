package main

import (
	"os"

	"devhelper/internal/app"
	"devhelper/internal/cli"
)

func main() {
	a := app.New()
	if err := cli.Run(os.Args, a); err != nil {
		a.Log.Error().Err(err).Msg("fatal")
		os.Exit(1)
	}
}
