package platform

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"devhelper/internal/app"

	"github.com/rs/zerolog/log"
)

// SyncIfMissing copies a file or directory from Windows FS into the WSL runtime root
// if the destination doesn't exist.
//
// This helps keep the WSL runtime stack (docker-compose.yml + containers/...) present
// without forcing the user to manually copy it.
func SyncIfMissing(cfg app.Config, srcWindowsPath string, dstWSLPath string) error {
	// Convert WSL path to a host-accessible UNC path.
	dstHost := WSLToHost(cfg, dstWSLPath)
	if _, err := os.Stat(dstHost); err == nil {
		return nil
	}

	st, err := os.Stat(srcWindowsPath)
	if err != nil {
		log.Err(err).Msg("failed to stat source windows path")
		return err
	}
	if st.IsDir() {
		return copyDir(srcWindowsPath, dstHost)
	}
	return copyFile(srcWindowsPath, dstHost)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		log.Err(err).Msg("failed to create directory")
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		log.Err(err).Msg("failed to open file")
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		log.Err(err).Msg("failed to open file")
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		log.Err(err).Msg("failed to copy file")
		return err
	}
	return out.Chmod(0o644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Err(err).Msg("failed to walk directory")
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			log.Err(err).Msg("failed to get relative path")
			return err
		}
		// Normalize separators for UNC paths.
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}
