package platform

import (
	"path/filepath"
	"runtime"
	"testing"

	"devhelper/internal/app"
)

func TestWSLToHostPathBoundaries(t *testing.T) {
	mirror := filepath.Join(t.TempDir(), "mirror")
	tests := []struct {
		name string
		root string
		path string
		want string
	}{
		{"root", "/data/projects", "/data/projects", mirror},
		{"child", "/data/projects", "/data/projects/apps/acme", filepath.Join(mirror, "apps", "acme")},
		{"trailing root slash", "/data/projects/", "/data/projects/apps", filepath.Join(mirror, "apps")},
		{"sibling prefix", "/data/projects", "/data/projects-backup/app", "/data/projects-backup/app"},
		{"traversal", "/data/projects", "/data/projects/../private", "/data/projects/../private"},
		{"clean child", "/data/projects", "/data/projects/apps/../data", filepath.Join(mirror, "data")},
		{"empty root", "", "/etc/hosts", "/etc/hosts"},
		{"relative root", "data/projects", "data/projects/apps", "data/projects/apps"},
		{"relative path", "/data/projects", "data/projects/apps", "data/projects/apps"},
		{"filesystem root", "/", "/data/projects", filepath.Join(mirror, "data", "projects")},
		{"backslash traversal", "/data/projects", `/data/projects/apps\..\..\private`, `/data/projects/apps\..\..\private`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := app.Config{RootDir: tt.root, HostMirrorDir: mirror}
			want := tt.want
			if runtime.GOOS != "windows" {
				want = tt.path
			}
			if got := WSLToHost(cfg, tt.path); got != want {
				t.Fatalf("WSLToHost(%q, %q) = %q, want %q", tt.root, tt.path, got, want)
			}
		})
	}
}
