package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"devhelper/internal/app"
	"devhelper/internal/platform"
	"devhelper/internal/store"
)

type DropRequest struct {
	Company string
	Project string
}

type DropResult struct {
	Warnings []string
}

// Drop removes a project (dirs, nginx conf, certs, hosts entry, db) and deletes it from the store.
func Drop(a *app.App, st *store.Store, req DropRequest) (DropResult, error) {
	c := sanitize(req.Company)
	pn := sanitize(req.Project)
	p, ok := st.Get(c, pn)
	if !ok {
		return DropResult{}, fmt.Errorf("project not found")
	}

	warns := []string{}

	// Drop DB (best-effort; keep going on failure).
	if strings.ToLower(strings.TrimSpace(p.DB)) == "mysql-8.4" || strings.ToLower(strings.TrimSpace(p.DB)) == "mysql-9.6" || strings.ToLower(strings.TrimSpace(p.DB)) == "mariadb10" || strings.ToLower(strings.TrimSpace(p.DB)) == "mariadb12" || strings.ToLower(strings.TrimSpace(p.DB)) == "postgres" {
		if err := dropDB(a.Cfg, p.DB, p.DBName, p.DBUser); err != nil {
			warns = append(warns, "db: "+err.Error())
		}
	}

	// Remove hosts entry (best-effort).
	if err := platform.RemoveHost(p.Domain); err != nil {
		warns = append(warns, "hosts: "+err.Error())
	}

	// Remove nginx conf.
	confHost := p.NginxConf
	if runtime.GOOS == "windows" {
		confHost = platform.WSLToHost(a.Cfg, p.NginxConf)
	}
	if err := os.Remove(confHost); err != nil && !os.IsNotExist(err) {
		warns = append(warns, "nginx_conf: "+err.Error())
	}

	// Remove cert dir.
	certRuntime := filepath.ToSlash(filepath.Join(a.Cfg.NginxExternalRoot, "certs", p.Domain))
	certHost := certRuntime
	if runtime.GOOS == "windows" {
		certHost = platform.WSLToHost(a.Cfg, certRuntime)
	}
	if err := os.RemoveAll(certHost); err != nil {
		warns = append(warns, "certs: "+err.Error())
	}

	// Remove app dir.
	if err := os.RemoveAll(p.HostPath); err != nil {
		warns = append(warns, "app_dir: "+err.Error())
	}

	// Remove optional Windows-side mirror inside repo.
	if runtime.GOOS == "windows" {
		winDir := filepath.Join(a.RepoRoot, "projects", "apps", p.Company, p.Name)
		if err := os.RemoveAll(winDir); err != nil {
			warns = append(warns, "mirror_dir: "+err.Error())
		}
	}

	// Remove from store.
	_, _, err := st.Delete(p.Company, p.Name)
	if err != nil {
		return DropResult{}, err
	}
	return DropResult{Warnings: warns}, nil
}
