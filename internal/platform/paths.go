package platform

import (
	"path"
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
	// Match whole path components after cleaning POSIX paths. A plain prefix
	// would map /data/projects-backup into /data/projects, and joining an
	// unclean suffix could escape the configured host mirror via "..".
	root := path.Clean(cfg.RootDir)
	if !path.IsAbs(root) || !path.IsAbs(wslPath) || strings.ContainsAny(wslPath, `\`) {
		return wslPath
	}
	clean := path.Clean(wslPath)
	if clean == root {
		return filepath.Clean(cfg.HostMirrorDir)
	}
	rel, ok := strings.CutPrefix(clean, strings.TrimSuffix(root, "/")+"/")
	if !ok {
		return wslPath
	}
	return filepath.Join(cfg.HostMirrorDir, filepath.FromSlash(rel))
}
