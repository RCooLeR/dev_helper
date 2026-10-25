package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"devhelper/internal/store"
)

func TestExternalDatabaseImportAndReplaceAreRejected(t *testing.T) {
	for _, replace := range []string{"false", "true"} {
		t.Run(replace, func(t *testing.T) {
			s := testServer(t)
			if _, err := s.Store.Upsert(store.Project{Company: "acme", Name: "app", DB: "mysql", DBName: "remote", DBExternal: true}); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPost, "/api/projects/import", strings.NewReader(`{"company":"acme","project":"app","file":"missing.sql","replace":`+replace+`}`))
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "externally managed") {
				t.Fatalf("external import reached local DB handling: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
