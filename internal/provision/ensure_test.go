package provision

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"devhelper/internal/store"
)

func TestEnsureProjectDBCancellation(t *testing.T) {
	a, st := testProjectApp(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := EnsureProjectDBContext(ctx, a.Cfg, st, "acme", "app"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if _, err := os.Stat(a.Cfg.StoreFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled repair wrote metadata: %v", err)
	}
}

func TestEnsureProjectDBNotFound(t *testing.T) {
	a, st := testProjectApp(t)
	err := EnsureProjectDBContext(t.Context(), a.Cfg, st, "acme", "app")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing project was not rejected: %v", err)
	}
	if _, err := os.Stat(a.Cfg.StoreFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing project repair wrote metadata: %v", err)
	}
}

func TestEnsureProjectDBValidatesSavedConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*store.Project)
		want string
	}{
		{"external", func(p *store.Project) { p.DBExternal = true }, "externally managed"},
		{"none", func(p *store.Project) { p.DB = "none" }, "has no database"},
		{"empty engine", func(p *store.Project) { p.DB = "" }, "has no database"},
		{"unknown engine", func(p *store.Project) { p.DB = "not-mysql" }, "unsupported stored database engine"},
		{"missing database name", func(p *store.Project) { p.DBName = "" }, "incomplete database credentials"},
		{"missing database user", func(p *store.Project) { p.DBUser = " " }, "incomplete database credentials"},
		{"missing database password", func(p *store.Project) { p.DBPass = "" }, "incomplete database credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, st := testProjectApp(t)
			p := store.Project{Company: "acme", Name: "app", DB: "mysql", DBName: "legacy_db", DBUser: "app_u", DBPass: "keep-password"}
			tc.edit(&p)
			if _, err := st.Upsert(p); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(a.Cfg.StoreFile)
			if err != nil {
				t.Fatal(err)
			}
			err = EnsureProjectDBContext(t.Context(), a.Cfg, st, p.Company, p.Name)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid stored configuration reached provisioning: %v; want %q", err, tc.want)
			}
			after, err := os.ReadFile(a.Cfg.StoreFile)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("validation changed metadata: %v", err)
			}
		})
	}
}

func TestEnsureProjectDBProtectsSharedLegacyOwnership(t *testing.T) {
	for _, sharedField := range []string{"database", "user"} {
		t.Run(sharedField, func(t *testing.T) {
			a, st := testProjectApp(t)
			p := store.Project{Company: "acme", Name: "app", DB: "mysql", DBName: "legacy_db", DBUser: "app_u", DBPass: "keep-password"}
			other := store.Project{Company: "other", Name: "app", DB: "mysql-8.4", DBName: "other_db", DBUser: "other_u", DBPass: "other-password"}
			if sharedField == "database" {
				other.DBName = p.DBName
			} else {
				other.DBUser = p.DBUser
			}
			for _, project := range []store.Project{p, other} {
				if _, err := st.Upsert(project); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(a.Cfg.StoreFile)
			if err != nil {
				t.Fatal(err)
			}
			err = EnsureProjectDBContext(t.Context(), a.Cfg, st, p.Company, p.Name)
			if err == nil || !strings.Contains(err.Error(), "shared database ownership") || !strings.Contains(err.Error(), "other/app") {
				t.Fatalf("shared ownership reached provisioning: %v", err)
			}
			after, err := os.ReadFile(a.Cfg.StoreFile)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("shared-ownership rejection changed metadata: %v", err)
			}
		})
	}
}

func TestEnsureProjectDBFailureRetainsExistingIdentity(t *testing.T) {
	a, st := testProjectApp(t)
	p, files := seedOwnershipProject(t, a, st, "acme", "app", "mysql")
	before, err := os.ReadFile(a.Cfg.StoreFile)
	if err != nil {
		t.Fatal(err)
	}
	// The configured missing Docker executable produces a real startup error.
	// Neither a new identity nor a new password may be saved after this failure.
	err = EnsureProjectDBContext(t.Context(), a.Cfg, st, p.Company, p.Name)
	if err == nil || !strings.Contains(err.Error(), "ensure mysql-8.4 database for acme/app") {
		t.Fatalf("database startup failure was not returned: %v", err)
	}
	assertOwnershipResourcesUnchanged(t, a, files, before)
}
