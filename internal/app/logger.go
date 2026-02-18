package app

import (
	"os"

	"github.com/rs/zerolog"
)

type App struct {
	Log      zerolog.Logger
	RepoRoot string
	Cfg      Config
}

func NewLogger(configLevel string) zerolog.Logger {
	// Default for this project is debug.
	lvl := zerolog.DebugLevel
	if configLevel != "" {
		if parsed, err := zerolog.ParseLevel(configLevel); err == nil {
			lvl = parsed
		}
	}
	// Environment override (handy for quick debugging)
	if v := os.Getenv("DEVHELPER_LOG_LEVEL"); v != "" {
		if parsed, err := zerolog.ParseLevel(v); err == nil {
			lvl = parsed
		}
	}
	zerolog.SetGlobalLevel(lvl)
	return zerolog.New(os.Stdout).With().Timestamp().Str("app", "devhelper").Logger().Level(lvl)
}

func New() *App {
	repoRoot, _ := os.Getwd()
	cfg, _, _ := LoadOrDefault(repoRoot)
	return &App{
		Log:      NewLogger(cfg.LogLevel),
		RepoRoot: repoRoot,
		Cfg:      cfg,
	}
}
