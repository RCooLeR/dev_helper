package cli

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"devhelper/internal/app"
	"devhelper/internal/store"
)

func TestExternalDatabaseImportIsRejected(t *testing.T) {
	file := filepath.Join(t.TempDir(), "store.json")
	st, err := store.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Upsert(store.Project{Company: "acme", Name: "app", DB: "mysql", DBName: "remote", DBExternal: true}); err != nil {
		t.Fatal(err)
	}
	cmd := newCommand(&app.App{Cfg: app.Config{StoreFile: file}})
	cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
	err = cmd.Run(t.Context(), []string{"devhelper", "db", "import", "--company", "acme", "--project", "app", "--file", "missing.sql"})
	if err == nil || !strings.Contains(err.Error(), "externally managed") {
		t.Fatalf("external import reached local DB handling: %v", err)
	}
}
