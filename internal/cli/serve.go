package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"devhelper/internal/app"
	httpserver "devhelper/internal/http"
	"devhelper/internal/platform"
	"devhelper/internal/store"

	"github.com/urfave/cli/v3"
)

func cmdServe(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run the web UI/API server",
		Flags: []cli.Flag{&cli.StringFlag{Name: "http", Value: "127.0.0.1:8787"}},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg := a.Cfg
			st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
			if err != nil {
				return err
			}

			srv, err := httpserver.New(a, st)
			if err != nil {
				return err
			}

			addr := cmd.String("http")
			server := &http.Server{
				Addr:              addr,
				Handler:           srv.Router(),
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       15 * time.Second,
				// DB startup and imports may take minutes. A response deadline
				// would hide their eventual result from the browser.
				IdleTimeout: 60 * time.Second,
			}
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.Writer, "Listening on", listener.Addr()); err != nil {
				_ = listener.Close()
				return err
			}
			return serveHTTP(ctx, server, listener)
		},
	}
}

// serveHTTP stops accepting requests on cancellation and waits for in-flight
// requests to finish before returning. A second interrupt can force an exit.
func serveHTTP(ctx context.Context, server *http.Server, listener net.Listener) error {
	if err := ctx.Err(); err != nil {
		_ = listener.Close()
		return err
	}

	shutdownDone := make(chan error, 1)
	stopShutdown := context.AfterFunc(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			_ = server.Close()
		}
		shutdownDone <- err
	})

	err := server.Serve(listener)
	if !stopShutdown() {
		if shutdownErr := <-shutdownDone; shutdownErr != nil {
			return fmt.Errorf("shut down HTTP server: %w", shutdownErr)
		}
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
