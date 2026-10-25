package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"devhelper/internal/app"
)

func initTestApp(t *testing.T) *app.App {
	t.Helper()
	root := t.TempDir()
	cfg := app.DefaultConfig(root)
	cfg.ComposeDir = filepath.Join(root, "saved-compose-directory")
	cfg.RootDir = filepath.Join(root, "runtime")
	if runtime.GOOS == "windows" {
		cfg.RootDir = "/devhelper-test-runtime"
		cfg.HostMirrorDir = filepath.Join(root, "runtime")
	}
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "docker-compose.example.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.Save(root, cfg); err != nil {
		t.Fatal(err)
	}
	return &app.App{RepoRoot: root, Cfg: cfg}
}

func TestInitPreservesSavedComposeDirectory(t *testing.T) {
	a := initTestApp(t)
	want := a.Cfg.ComposeDir
	cmd := newCommand(a)
	cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
	if err := cmd.Run(t.Context(), []string{"devhelper", "init"}); err != nil {
		t.Fatal(err)
	}
	if a.Cfg.ComposeDir != want {
		t.Fatalf("init changed saved compose directory: got %q, want %q", a.Cfg.ComposeDir, want)
	}
	for _, name := range []string{"docker-compose.yml", ".env"} {
		if _, err := os.Stat(filepath.Join(want, name)); err != nil {
			t.Fatal(err)
		}
	}
	loaded, exists, err := app.LoadOrDefault(a.RepoRoot)
	if err != nil || !exists || loaded.ComposeDir != want {
		t.Fatalf("persisted init config changed directory: %+v, %v, %v", loaded, exists, err)
	}
}

func TestFailedInitPreservesActiveConfig(t *testing.T) {
	for _, failure := range []string{"missing-compose-example", "malformed-env"} {
		t.Run(failure, func(t *testing.T) {
			a := initTestApp(t)
			beforeCfg := a.Cfg
			beforeFile, err := os.ReadFile(app.ConfigPath(a.RepoRoot))
			if err != nil {
				t.Fatal(err)
			}
			if failure == "missing-compose-example" {
				if err := os.Remove(filepath.Join(a.RepoRoot, "projects", "docker-compose.example.yml")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(a.Cfg.ComposeDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(a.Cfg.ComposeDir, ".env"), []byte("CUSTOM='unterminated"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newCommand(a)
			cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
			if err := cmd.Run(t.Context(), []string{"devhelper", "init", "--mysql-8.4-cli", "changed-client"}); err == nil {
				t.Fatal("invalid init succeeded")
			}
			afterFile, err := os.ReadFile(app.ConfigPath(a.RepoRoot))
			if err != nil || !bytes.Equal(beforeFile, afterFile) || a.Cfg != beforeCfg {
				t.Fatalf("failed init activated changed configuration: err=%v", err)
			}
		})
	}
}
