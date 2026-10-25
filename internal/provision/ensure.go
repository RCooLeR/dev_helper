package provision

import (
	"context"
	"fmt"
	"strings"

	"devhelper/internal/app"
	"devhelper/internal/store"
)

// EnsureProjectDBContext repairs a saved project's database using its existing
// identity and credentials. It never drops data, imports a dump, or rewrites the
// store, so projects saved by older releases after failed provisioning can be
// recovered without deleting and recreating their project metadata.
func EnsureProjectDBContext(ctx context.Context, cfg app.Config, st *store.Store, company, project string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	company, err := projectSlug(company)
	if err != nil {
		return fmt.Errorf("company: %w", err)
	}
	project, err = projectSlug(project)
	if err != nil {
		return fmt.Errorf("project: %w", err)
	}
	unlock, err := st.LockOperations(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	p, ok, err := st.Get(company, project)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("project %s/%s not found", company, project)
	}
	if err := ValidateManagedDatabase(p); err != nil {
		return err
	}
	engine := normalizeDBEngine(p.DB)
	if engine == "" || engine == "none" {
		return fmt.Errorf("project %s/%s has no database", company, project)
	}
	if !isSupportedDB(engine) {
		return fmt.Errorf("unsupported stored database engine %q", p.DB)
	}
	if strings.TrimSpace(p.DBName) == "" || strings.TrimSpace(p.DBUser) == "" || p.DBPass == "" {
		return fmt.Errorf("project %s/%s has incomplete database credentials: stored name, user and password are required", company, project)
	}
	projects, err := st.List()
	if err != nil {
		return err
	}
	if err := ValidateDBReplacement(p, projects); err != nil {
		return fmt.Errorf("cannot repair shared database ownership: %w", err)
	}
	if err := createDBContext(ctx, cfg, engine, p.DBName, p.DBUser, p.DBPass); err != nil {
		return fmt.Errorf("ensure %s database for %s/%s: %w", engine, company, project, err)
	}
	return nil
}
