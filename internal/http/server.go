package httpserver

import (
	"embed"
	"encoding/json"
	"net/http"
	"path"
	"strings"

	"devhelper/internal/app"
	"devhelper/internal/provision"
	"devhelper/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed ui/*.html ui/*.css ui/*.png
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
	r.Use(middleware.Recoverer, middleware.RealIP, middleware.RequestID, middleware.Logger)

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

	return r
}

func (s *Server) pageIndex(w http.ResponseWriter, r *http.Request) {
	b, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

func (s *Server) apiList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"projects": s.Store.List()})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad json"})
		return
	}
	res, err := provision.Create(s.App, s.Store, provision.CreateRequest{
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad json"})
		return
	}
	res, err := provision.Drop(s.App, s.Store, provision.DropRequest{
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad json"})
		return
	}
	p, ok := s.Store.Get(req.Company, req.Project)
	if !ok {
		writeJSON(w, 404, map[string]any{"error": "project not found"})
		return
	}
	if p.DB == "none" {
		writeJSON(w, 400, map[string]any{"error": "project has no db"})
		return
	}
	if req.Replace {
		if err := provision.ReplaceSQL(s.App.Cfg, p.DB, p.DBName, p.DBUser, p.DBPass, req.File); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
	} else {
		if err := provision.ImportSQL(s.App.Cfg, p.DB, p.DBName, req.File); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) apiCertInit(w http.ResponseWriter, r *http.Request) {
	if err := provision.CertInit(s.App.Cfg); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad json"})
		return
	}
	if err := provision.CertIssue(s.App.Cfg, req.Domain); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
