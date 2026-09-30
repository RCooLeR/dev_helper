package provision

import (
	"context"
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
	return DropContext(context.Background(), a, st, req)
}

func DropContext(ctx context.Context, a *app.App, st *store.Store, req DropRequest) (DropResult, error) {
	c, err := projectSlug(req.Company)
	if err != nil {
		return DropResult{}, err
	}
	pn, err := projectSlug(req.Project)
	if err != nil {
		return DropResult{}, err
	}
	unlock, err := st.LockOperations(ctx)
	if err != nil {
		return DropResult{}, err
	}
	defer unlock()
	p, ok, err := st.Get(c, pn)
	if err != nil {
		return DropResult{}, err
	}
	if !ok {
		return DropResult{}, fmt.Errorf("project not found")
	}
	if !p.DBExternal && p.DB != "" && p.DB != "none" && !isSupportedDB(p.DB) {
		return DropResult{}, fmt.Errorf("unsupported stored database engine %q; refusing to delete", p.DB)
	}
	if err := validateDomain(p.Domain); err != nil {
		return DropResult{}, err
	}
	appRoot := platform.WSLToHost(a.Cfg, a.Cfg.AppsRoot)
	confRoot := platform.WSLToHost(a.Cfg, a.Cfg.NginxConfRoot)
	confName := c + "__" + pn + ".conf"
	if !samePath(p.HostPath, filepath.Join(appRoot, c, pn)) ||
		!samePath(platform.WSLToHost(a.Cfg, p.NginxConf), filepath.Join(confRoot, confName)) {
		return DropResult{}, fmt.Errorf("stored project paths do not match the configured roots; refusing to delete")
	}
	projects, err := st.List()
	if err != nil {
		return DropResult{}, err
	}
	sharedDomain := false
	sharedUser := false
	for _, other := range projects {
		if other.Company == c && other.Name == pn {
			continue
		}
		if samePath(other.NginxConf, p.NginxConf) {
			return DropResult{}, fmt.Errorf("nginx configuration is also used by %s/%s; refusing to delete", other.Company, other.Name)
		}
		sharedDomain = sharedDomain || strings.EqualFold(other.Domain, p.Domain)
		if !p.DBExternal && isSupportedDB(p.DB) {
			for _, connection := range localDBConnections(other) {
				if normalizeDBEngine(connection.Engine) != normalizeDBEngine(p.DB) {
					continue
				}
				if p.DBName != "" && connection.Database == p.DBName {
					return DropResult{}, fmt.Errorf("database is also used by %s/%s; refusing to delete", other.Company, other.Name)
				}
				sharedUser = sharedUser || (p.DBUser != "" && connection.User == p.DBUser)
			}
		}
	}

	warns := []string{}

	// Keep metadata and files when a required deletion fails so it is possible
	// to retry instead of orphaning a database and its credentials.
	if !p.DBExternal && isSupportedDB(p.DB) {
		user := p.DBUser
		if sharedUser {
			user = ""
		}
		if err := dropDBContext(ctx, a.Cfg, p.DB, p.DBName, user); err != nil {
			return DropResult{}, fmt.Errorf("drop database: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return DropResult{}, err
	}

	// Remove hosts entry (best-effort).
	if !sharedDomain {
		if err := platform.RemoveHost(p.Domain); err != nil {
			warns = append(warns, "hosts: "+err.Error())
		}
	}

	if err := removeWithin(confRoot, confName, false); err != nil {
		return DropResult{}, fmt.Errorf("nginx_conf: %w", err)
	}

	if !sharedDomain {
		certRoot := platform.WSLToHost(a.Cfg, filepath.ToSlash(filepath.Join(a.Cfg.NginxExternalRoot, "certs")))
		if err := removeWithin(certRoot, p.Domain, true); err != nil {
			return DropResult{}, fmt.Errorf("certs: %w", err)
		}
	}

	if err := removeWithin(appRoot, filepath.Join(c, pn), true); err != nil {
		return DropResult{}, fmt.Errorf("app_dir: %w", err)
	}

	// Remove optional Windows-side mirror inside repo.
	if runtime.GOOS == "windows" {
		if err := removeWithin(filepath.Join(a.RepoRoot, "projects", "apps"), filepath.Join(c, pn), true); err != nil {
			return DropResult{}, fmt.Errorf("mirror_dir: %w", err)
		}
	}

	// Remove from store.
	_, _, err = st.Delete(p.Company, p.Name)
	if err != nil {
		return DropResult{}, err
	}
	return DropResult{Warnings: warns}, nil
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// os.Root confines deletion even if an intermediate component is a symlink.
func removeWithin(base, relative string, recursive bool) error {
	if !filepath.IsLocal(relative) || relative == "." {
		return fmt.Errorf("unsafe deletion path %q", relative)
	}
	root, err := os.OpenRoot(base)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	if recursive {
		err = root.RemoveAll(relative)
	} else {
		err = root.Remove(relative)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
