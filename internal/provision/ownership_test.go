package provision

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devhelper/internal/app"
	"devhelper/internal/store"
)

func TestValidateDBReplacementProtectsLegacyOwnership(t *testing.T) {
	for _, tc := range []struct {
		name         string
		engine       string
		otherEngine  string
		otherDB      string
		otherUser    string
		sameProject  bool
		wantConflict bool
	}{
		{"shared database", "mysql-8.4", "mysql-8.4", "acme_app", "other_user", false, true},
		{"shared user", "mysql-8.4", "mysql-8.4", "other_app", "app_u", false, true},
		{"mysql alias", "mysql", "mysql-8.4", "other_app", "app_u", false, true},
		{"old mysql9 alias", "mysql-9.6", "mysql9", "acme_app", "other_user", false, true},
		{"postgres alias", "postgres", "postgresql18", "other_app", "app_u", false, true},
		{"mariadb alias", "mariadb10", "mariadb-10.6", "acme_app", "other_user", false, true},
		{"separate users and databases", "mysql", "mysql", "other_app", "other_user", false, false},
		{"separate engines", "mysql-8.4", "mysql-9.7", "acme_app", "app_u", false, false},
		{"record itself", "mysql", "mysql-8.4", "acme_app", "app_u", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := store.Project{Company: "acme", Name: "app", DB: tc.engine, DBName: "acme_app", DBUser: "app_u"}
			other := store.Project{Company: "other", Name: "app", DB: tc.otherEngine, DBName: tc.otherDB, DBUser: tc.otherUser}
			if tc.sameProject {
				other.Company = project.Company
			}
			err := ValidateDBReplacement(project, []store.Project{project, other})
			if (err != nil) != tc.wantConflict {
				t.Fatalf("ValidateDBReplacement() = %v, wantConflict=%v", err, tc.wantConflict)
			}
			if tc.wantConflict && !strings.Contains(err.Error(), "other/app") {
				t.Fatalf("conflict does not identify the affected project: %v", err)
			}
		})
	}
}

// Seed a stored project and sentinel files without running host/certificate or
// Docker operations. Guard failures must preserve every one of these resources.
func seedOwnershipProject(t *testing.T, a *app.App, st *store.Store, company, name, engine string) (store.Project, map[string][]byte) {
	t.Helper()
	p := store.Project{
		Company: company, Name: name, Domain: "owned.example.local", PHP: "8.4", DB: engine,
		DBName: "owned_db", DBUser: "owned_user", DBPass: "preserve-password",
		HostPath:  filepath.Join(a.Cfg.AppsRoot, company, name),
		NginxConf: filepath.Join(a.Cfg.NginxConfRoot, company+"__"+name+".conf"),
	}
	files := map[string][]byte{
		filepath.Join(p.HostPath, "application.txt"): []byte("keep application"),
		p.NginxConf: []byte("keep nginx configuration"),
		filepath.Join(a.Cfg.NginxExternalRoot, "certs", p.Domain, "cert.pem"): []byte("keep certificate"),
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := st.Upsert(p)
	if err != nil {
		t.Fatal(err)
	}
	return saved, files
}

func assertOwnershipResourcesUnchanged(t *testing.T, a *app.App, files map[string][]byte, storeBefore []byte) {
	t.Helper()
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("resource changed: %s: got %q, %v; want %q", path, got, err, want)
		}
	}
	storeAfter, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil || !bytes.Equal(storeAfter, storeBefore) {
		t.Fatalf("failed operation changed project metadata: %v", err)
	}
}

func TestCreateRejectsAmbiguousNginxOwnership(t *testing.T) {
	a, st := testProjectApp(t)
	_, files := seedOwnershipProject(t, a, st, "a__b", "c", "none")
	before, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreateContext(t.Context(), a, st, CreateRequest{
		Company: "a", Project: "b__c", Domain: "separate.example.local", PHP: "8.4", DB: "mysql",
	})
	if err == nil || !strings.Contains(err.Error(), "nginx configuration name") {
		t.Fatalf("ambiguous nginx ownership accepted or reached Docker: %v", err)
	}
	assertOwnershipResourcesUnchanged(t, a, files, before)
}

func TestDropUnknownEngineRetainsFilesAndMetadata(t *testing.T) {
	a, st := testProjectApp(t)
	p, files := seedOwnershipProject(t, a, st, "acme", "app", "misspelled-mysql")
	before, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DropContext(t.Context(), a, st, DropRequest{Company: p.Company, Project: p.Name})
	if err == nil || !strings.Contains(err.Error(), "unsupported stored database engine") {
		t.Fatalf("unknown engine was not rejected before deletion: %v", err)
	}
	assertOwnershipResourcesUnchanged(t, a, files, before)
}

func TestFailedDatabaseDropRetainsFilesAndMetadata(t *testing.T) {
	a, st := testProjectApp(t)
	p, files := seedOwnershipProject(t, a, st, "acme", "app", "mysql")
	before, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's Docker executable does not exist. This is a real failed
	// subprocess startup, not a mocked successful drop.
	_, err = DropContext(t.Context(), a, st, DropRequest{Company: p.Company, Project: p.Name})
	if err == nil || !strings.Contains(err.Error(), "drop database") {
		t.Fatalf("database failure was not returned: %v", err)
	}
	assertOwnershipResourcesUnchanged(t, a, files, before)
}

func TestDropRejectsSharedLegacyNginxConfiguration(t *testing.T) {
	a, st := testProjectApp(t)
	p, files := seedOwnershipProject(t, a, st, "a__b", "c", "none")
	if _, err := st.Upsert(store.Project{Company: "a", Name: "b__c", Domain: "other.example.local", DB: "none", NginxConf: p.NginxConf}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DropContext(t.Context(), a, st, DropRequest{Company: p.Company, Project: p.Name})
	if err == nil || !strings.Contains(err.Error(), "nginx configuration is also used") {
		t.Fatalf("shared nginx configuration was not protected: %v", err)
	}
	assertOwnershipResourcesUnchanged(t, a, files, before)
}
