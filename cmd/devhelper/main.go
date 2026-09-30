package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"devhelper/internal/app"
	"devhelper/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Once graceful shutdown starts, a second interrupt should terminate immediately.
	go func() {
		<-ctx.Done()
		stop()
	}()
	a, err := app.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cli.RunContext(ctx, os.Args, a); err != nil {
		a.Log.Error().Err(err).Msg("fatal")
		os.Exit(1)
	}
}
