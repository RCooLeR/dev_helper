package app

import (
	"errors"
	"os"
	"path/filepath"

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
	repoRoot := detectRepoRoot()
	cfg, _, _ := LoadOrDefault(repoRoot)
	return &App{
		Log:      NewLogger(cfg.LogLevel),
		RepoRoot: repoRoot,
		Cfg:      cfg,
	}
}

// detectRepoRoot tries hard to find the repo root even if the process is started
// from a different working directory (e.g. shortcuts, services).
//
// Strategy:
// 1) Try current working dir (and parents).
// 2) Try executable dir (and parents).
//
// A directory is treated as a repo root if it contains at least one marker:
// - .devhelper-config.json
// - go.mod
// - projects/docker-compose.yml
func detectRepoRoot() string {
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	for _, start := range []string{cwd, exeDir} {
		if start == "" {
			continue
		}
		d := start
		for {
			if isRepoMarkerDir(d) {
				return d
			}
			p := filepath.Dir(d)
			if p == d {
				break
			}
			d = p
		}
	}
	if cwd != "" {
		return cwd
	}
	return "."
}

func isRepoMarkerDir(dir string) bool {
	if fileExists(filepath.Join(dir, ".devhelper-config.json")) {
		return true
	}
	if fileExists(filepath.Join(dir, "go.mod")) {
		return true
	}
	if fileExists(filepath.Join(dir, "projects", "docker-compose.yml")) {
		return true
	}
	return false
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	if err == nil {
		return !st.IsDir()
	}
	return !errors.Is(err, os.ErrNotExist)
}
