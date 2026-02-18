package cli

import (
	"fmt"
	"net/http"
	"time"

	"devhelper/internal/app"
	httpserver "devhelper/internal/http"
	"devhelper/internal/platform"
	"devhelper/internal/store"

	"github.com/urfave/cli/v2"
)

func cmdServe(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run the web UI/API server",
		Flags: []cli.Flag{&cli.StringFlag{Name: "http", Value: ":8787"}},
		Action: func(cctx *cli.Context) error {
			cfg := a.Cfg
			st, err := store.Open(platform.WSLToHost(cfg, cfg.StoreFile))
			if err != nil {
				return err
			}

			srv, err := httpserver.New(a, st)
			if err != nil {
				return err
			}

			addr := cctx.String("http")
			server := &http.Server{
				Addr:         addr,
				Handler:      srv.Router(),
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 60 * time.Second,
			}
			fmt.Println("Listening on", addr)
			return server.ListenAndServe()
		},
	}
}
