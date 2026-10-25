package httpserver

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"devhelper/internal/app"
	"devhelper/internal/store"
	"github.com/rs/zerolog"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(&app.App{Cfg: app.DefaultConfig(root), Log: zerolog.Nop()}, st)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJSONMutationsRejectAmbiguousInput(t *testing.T) {
	s := testServer(t)
	for _, body := range []string{
		`{"company":"a","company":"b"}`, `{} {}`, `null`, `{"typo":true}`, `[]`,
		"{\"company\":\"\xff\"}",
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(body))
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "expected one JSON object") {
			t.Fatalf("body %q: %d %s", body, w.Code, w.Body.String())
		}
	}
}

func TestJSONBodyLimit(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(strings.Repeat(" ", (1<<20)+1)))
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized input: %d", w.Code)
	}
}

func TestCrossOriginMutationsRejected(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest(http.MethodPost, "http://localhost/api/projects/drop", strings.NewReader(`{}`))
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Origin", "https://unrelated.example")
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation accepted: %d", w.Code)
	}
}

func TestIndexRendersSafeConfiguredMetadata(t *testing.T) {
	s := testServer(t)
	want := `<project>.<company>.test</script><script>alert("x")</script>`
	s.App.Cfg.DefaultDomainPattern = want
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), `</script><script>alert`) {
		t.Fatalf("unsafe or failed page: %d", w.Code)
	}
	m := regexp.MustCompile(`(?s)<script id="devhelper-meta" type="application/json">\s*(.*?)\s*</script>`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 {
		t.Fatal("metadata missing")
	}
	var meta map[string]string
	if err := json.Unmarshal([]byte(m[1]), &meta); err != nil || meta["defaultDomainPattern"] != want {
		t.Fatalf("metadata: %v %v", meta, err)
	}
}

func TestListReportsStoreCorruption(t *testing.T) {
	s := testServer(t)
	// Locate the known store through a fresh setup to exercise read errors.
	file := filepath.Join(t.TempDir(), "state.json")
	st, err := store.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	s.Store = st
	if err := os.WriteFile(file, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if w.Code != 500 {
		t.Fatalf("corrupt store returned success: %d", w.Code)
	}
}
