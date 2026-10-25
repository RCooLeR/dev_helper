package provision

import (
	"os"
	"strings"
	"testing"

	"devhelper/internal/store"
)

func TestReplacementOwnershipIncludesLocalSecondaryDatabases(t *testing.T) {
	project := store.Project{Company: "acme", Name: "app", DB: "mysql", DBName: "main", DBUser: "managed"}
	for _, tc := range []struct {
		name         string
		other        store.Project
		wantConflict bool
	}{
		{"local secondary", store.Project{DB: "postgres", DBConnections: []store.DatabaseConnection{{Engine: "mysql-8.4", Database: "main"}}}, true},
		{"local secondary of external primary", store.Project{DBExternal: true, DBConnections: []store.DatabaseConnection{{Engine: "mysql", Database: "main"}}}, true},
		{"external secondary", store.Project{DBConnections: []store.DatabaseConnection{{Engine: "mysql", Database: "main", External: true}}}, false},
		{"external primary", store.Project{DB: "mysql", DBName: "main", DBUser: "managed", DBExternal: true}, false},
		{"observed secondary login", store.Project{DBConnections: []store.DatabaseConnection{{Engine: "mysql", Database: "separate", User: "managed"}}}, false},
		{"different engine", store.Project{DBConnections: []store.DatabaseConnection{{Engine: "mysql9", Database: "main"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.other.Company, tc.other.Name = "other", "consumer"
			err := ValidateDBReplacement(project, []store.Project{project, tc.other})
			if (err != nil) != tc.wantConflict {
				t.Fatalf("conflict = %v, want %v", err, tc.wantConflict)
			}
		})
	}
}

func TestDropProtectsLocalSecondaryDatabase(t *testing.T) {
	a, st := testProjectApp(t)
	p, files := seedOwnershipProject(t, a, st, "acme", "app", "mysql")
	_, err := st.Upsert(store.Project{Company: "other", Name: "consumer", DB: "none", DBConnections: []store.DatabaseConnection{{Engine: "mysql-8.4", Database: p.DBName}}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DropContext(t.Context(), a, st, DropRequest{Company: p.Company, Project: p.Name})
	if err == nil || !strings.Contains(err.Error(), "database is also used by other/consumer") {
		t.Fatalf("secondary database dependency reached deletion: %v", err)
	}
	assertOwnershipResourcesUnchanged(t, a, files, before)
}

func TestDropExternalDatabaseOnlyRemovesLocalProject(t *testing.T) {
	a, st := testProjectApp(t)
	p, _ := seedOwnershipProject(t, a, st, "acme", "app", "mysql")
	p.DBExternal = true
	if _, err := st.Upsert(p); err != nil {
		t.Fatal(err)
	}
	// Retain the shared domain so the test never edits the machine hosts file.
	// Its local DB identity deliberately matches the external one: no conflict.
	if _, err := st.Upsert(store.Project{Company: "other", Name: "consumer", Domain: p.Domain, DB: "mysql", DBName: p.DBName, DBUser: p.DBUser}); err != nil {
		t.Fatal(err)
	}
	if _, err := DropContext(t.Context(), a, st, DropRequest{Company: p.Company, Project: p.Name}); err != nil {
		t.Fatalf("external drop tried the missing Docker executable: %v", err)
	}
	for _, path := range []string{p.HostPath, p.NginxConf} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("local project resource remains: %s: %v", path, err)
		}
	}
	if _, ok, err := st.Get(p.Company, p.Name); err != nil || ok {
		t.Fatalf("external project metadata was not removed: %v", err)
	}
}
