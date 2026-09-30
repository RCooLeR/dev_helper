package httpserver

import (
	"bytes"
	"embed"
	"encoding/json/v2"
	"html/template"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"devhelper/internal/app"
	"devhelper/internal/provision"
	"devhelper/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed ui
var uiFS embed.FS

type Server struct {
	App   *app.App
	Store *store.Store
}

func New(a *app.App, st *store.Store) (*Server, error) {
	return &Server{App: a, Store: st}, nil
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, s.requestLog, middleware.Recoverer)

	r.Get("/", s.pageIndex)
	r.Get("/new", s.pageIndex)

	r.Route("/api", func(api chi.Router) {
		api.Get("/projects", s.apiList)
		api.Post("/projects", s.apiCreate)
		api.Post("/projects/drop", s.apiDrop)
		api.Post("/projects/import", s.apiImport)
		api.Post("/cert/init", s.apiCertInit)
		api.Post("/cert/issue", s.apiCertIssue)
	})

	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		p := chi.URLParam(r, "*")
		if p == "" {
			http.NotFound(w, r)
			return
		}
		p = path.Clean(p)
		if strings.Contains(p, "..") {
			http.Error(w, "bad path", 400)
			return
		}
		b, err := uiFS.ReadFile("ui/" + p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ext := strings.ToLower(path.Ext(p))
		switch ext {
		case ".js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".ico":
			w.Header().Set("Content-Type", "image/x-icon")
		}
		w.Write(b)
	})

	// This localhost app performs privileged filesystem and DB operations.
	// Go's browser-origin middleware rejects cross-site mutation requests.
	return http.NewCrossOriginProtection().Handler(r)
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		logger := s.App.Log.With().Str("request_id", middleware.GetReqID(r.Context())).Logger()
		next.ServeHTTP(ww, r.WithContext(logger.WithContext(r.Context())))
		logger.Info().Str("method", r.Method).Str("path", r.URL.Path).
			Int("status", ww.Status()).Int("bytes", ww.BytesWritten()).
			Dur("duration_ms", time.Since(start)).Msg("HTTP request")
	})
}

func (s *Server) pageIndex(w http.ResponseWriter, r *http.Request) {
	b, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("index").Parse(string(b))
	if err != nil {
		http.Error(w, "invalid page template", http.StatusInternalServerError)
		return
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, map[string]any{
		"Metadata": map[string]string{"defaultDomainPattern": s.App.Cfg.DefaultDomainPattern},
	}); err != nil {
		http.Error(w, "page rendering failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rendered.WriteTo(w)
}

func (s *Server) apiList(w http.ResponseWriter, r *http.Request) {
	projects, err := s.Store.List()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"projects": projects})
}

type createReq struct {
	Company string `json:"company"`
	Project string `json:"project"`
	Type    string `json:"type"`
	Domain  string `json:"domain"`
	PHP     string `json:"php"`
	DB      string `json:"db"`

	// Optional DB import on create (host file path)
	ImportFile     string `json:"import_file"`
	ImportOnCreate bool   `json:"import_on_create"`
}

func (s *Server) apiCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if !readJSON(w, r, &req) {
		return
	}
	res, err := provision.CreateContext(r.Context(), s.App, s.Store, provision.CreateRequest{
		Company:        req.Company,
		Project:        req.Project,
		Type:           req.Type,
		Domain:         req.Domain,
		PHP:            req.PHP,
		DB:             req.DB,
		ImportFile:     req.ImportFile,
		ImportOnCreate: req.ImportOnCreate,
	})
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"project": res.Project, "warnings": res.Warnings})
}

type dropReq struct {
	Company string `json:"company"`
	Project string `json:"project"`
}

func (s *Server) apiDrop(w http.ResponseWriter, r *http.Request) {
	var req dropReq
	if !readJSON(w, r, &req) {
		return
	}
	res, err := provision.DropContext(r.Context(), s.App, s.Store, provision.DropRequest{
		Company: req.Company,
		Project: req.Project,
	})
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "warnings": res.Warnings})
}

type importReq struct {
	Company string `json:"company"`
	Project string `json:"project"`
	File    string `json:"file"`
	Replace bool   `json:"replace"`
}

func (s *Server) apiImport(w http.ResponseWriter, r *http.Request) {
	var req importReq
	if !readJSON(w, r, &req) {
		return
	}
	unlock, err := s.Store.LockOperations(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	defer unlock()
	p, ok, err := s.Store.Get(req.Company, req.Project)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, 404, map[string]any{"error": "project not found"})
		return
	}
	if err := provision.ValidateManagedDatabase(p); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if p.DB == "none" {
		writeJSON(w, 400, map[string]any{"error": "project has no db"})
		return
	}
	if req.Replace {
		projects, err := s.Store.List()
		if err == nil {
			err = provision.ValidateDBReplacement(p, projects)
		}
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		if err := provision.ReplaceSQLContext(r.Context(), s.App.Cfg, p.DB, p.DBName, p.DBUser, p.DBPass, req.File); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
	} else {
		if err := provision.ImportSQLContext(r.Context(), s.App.Cfg, p.DB, p.DBName, req.File); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) apiCertInit(w http.ResponseWriter, r *http.Request) {
	if err := provision.CertInitContext(r.Context(), s.App.Cfg); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

type certIssueReq struct {
	Domain string `json:"domain"`
}

func (s *Server) apiCertIssue(w http.ResponseWriter, r *http.Request) {
	var req certIssueReq
	if !readJSON(w, r, &req) {
		return
	}
	if err := provision.CertIssueContext(r.Context(), s.App.Cfg, req.Domain); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "response encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(append(b, '\n'))
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	const maxRequestBytes = 1 << 20
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "request body exceeds 1 MiB or could not be read"})
		return false
	}
	// v2 rejects duplicate keys and invalid UTF-8; Unmarshal also rejects
	// concatenated documents instead of silently ignoring trailing input.
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) || json.Unmarshal(b, v, json.RejectUnknownMembers(true)) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "expected one JSON object with valid fields"})
		return false
	}
	return true
}
