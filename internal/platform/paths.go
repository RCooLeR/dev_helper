package platform

import (
	"path/filepath"
	"runtime"
	"strings"

	"devhelper/internal/app"
)

func HostRoot(cfg app.Config) string {
	if runtime.GOOS == "windows" && cfg.HostMirrorDir != "" {
		return cfg.HostMirrorDir
	}
	return cfg.RootDir
}

func WSLToHost(cfg app.Config, wslPath string) string {
	if runtime.GOOS != "windows" || cfg.HostMirrorDir == "" {
		return wslPath
	}
	if !strings.HasPrefix(wslPath, cfg.RootDir) {
		return wslPath
	}
	rel := strings.TrimPrefix(wslPath, cfg.RootDir)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.ReplaceAll(rel, "/", `\`)
	return filepath.Join(cfg.HostMirrorDir, rel)
}
