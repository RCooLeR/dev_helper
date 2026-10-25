//go:build !windows

package store

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func tryLockFile(f *os.File) (bool, error) {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		if !errors.Is(err, unix.EINTR) {
			return err == nil, err
		}
	}
}

func unlockFile(f *os.File) {
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

func replaceFile(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	// Flush the renamed directory entry as well as the file contents so a
	// successful transaction survives a subsequent power loss.
	dir, err := os.Open(filepath.Dir(to))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
