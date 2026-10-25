package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaultsAndCustomServices(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(ConfigPath(root), []byte(`{"mysql_root_pass":"","mysql_service":"custom-mysql","mysql9_service":"mysql-9.6"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, exists, err := LoadOrDefault(root)
	if err != nil || !exists {
		t.Fatalf("load: %v, %v", exists, err)
	}
	if cfg.MySQLService != "custom-mysql" || cfg.MySQL9Service != "mysql-9.6" || cfg.MySQLRootPass != "" {
		t.Fatalf("explicit config overwritten: %#v", cfg)
	}
	if cfg.AppsRoot == "" || cfg.StoreFile == "" || cfg.MySQLPort != 3384 {
		t.Fatalf("missing fields were not defaulted: %#v", cfg)
	}
	defaults := DefaultConfig(root)
	if defaults.MySQLService != "mysql" || defaults.MySQL9Service != "mysql9" {
		t.Fatal("default DB services must match the supplied Compose file")
	}
}

func TestLoadConfigRejectsDuplicateKeys(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(ConfigPath(root), []byte(`{"mysql_service":"mysql","mysql_service":"mysql9"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrDefault(root); err == nil {
		t.Fatal("ambiguous config accepted")
	}
}

func TestLoadConfigRejectsNull(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(ConfigPath(root), []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrDefault(root); err == nil {
		t.Fatal("null must not clear every configured path")
	}
}

func TestNewReportsCorruptConfig(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, ".devhelper-config.json"), []byte(`{"mysql_port":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(); err == nil {
		t.Fatal("startup swallowed invalid configuration")
	}
}
