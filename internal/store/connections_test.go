package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDatabaseInventorySurvivesStoreTransactions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	st := openTestStore(t, path)
	want, err := st.Upsert(Project{
		Company: "acme", Name: "app", DB: "mysql", DBName: "main",
		DBHost: "external.example", DBPort: 3307, DBExternal: true,
		DBConnections: []DatabaseConnection{{Name: "reporting", Engine: "postgres", Host: "postgres", Port: 5432, Database: "reports", User: "reader", Source: "config/db.php"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	other := openTestStore(t, path)
	if _, err := other.Upsert(Project{Company: "acme", Name: "other"}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.Get(want.Company, want.Name)
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatal("store transaction changed creation timestamp")
	}
	// Serialization intentionally removes the in-process monotonic timestamp.
	want.CreatedAt = got.CreatedAt
	if err != nil || !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("database inventory did not survive another transaction: %+v, %v", got, err)
	}
}
