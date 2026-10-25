package provision

import (
	"fmt"

	"devhelper/internal/store"
)

// ValidateManagedDatabase prevents local provisioning tools from touching an
// external database or silently applying its identity to a local DB service.
func ValidateManagedDatabase(project store.Project) error {
	if project.DBExternal {
		return fmt.Errorf("database for %s/%s is externally managed; local database operations are disabled", project.Company, project.Name)
	}
	return nil
}

// localDBConnections includes secondary dependencies even when the project's
// primary database is external. A local schema must not be dropped merely
// because a different application uses it through a secondary connection.
func localDBConnections(project store.Project) []store.DatabaseConnection {
	connections := make([]store.DatabaseConnection, 0, len(project.DBConnections)+1)
	if !project.DBExternal && isSupportedDB(project.DB) {
		connections = append(connections, store.DatabaseConnection{Engine: project.DB, Database: project.DBName, User: project.DBUser})
	}
	for _, connection := range project.DBConnections {
		if !connection.External && isSupportedDB(connection.Engine) {
			// Secondary users are observed application logins (often root),
			// not the per-project login managed by provisioning.
			connection.User = ""
			connections = append(connections, connection)
		}
	}
	return connections
}

// ValidateDBReplacement protects legacy projects whose names or usernames
// collided before per-project DB identities were introduced. Call while holding
// the store operation lock so ownership cannot change during replacement.
func ValidateDBReplacement(project store.Project, projects []store.Project) error {
	if err := ValidateManagedDatabase(project); err != nil {
		return err
	}
	for _, other := range projects {
		if other.Company == project.Company && other.Name == project.Name {
			continue
		}
		for _, connection := range localDBConnections(other) {
			if normalizeDBEngine(connection.Engine) != normalizeDBEngine(project.DB) {
				continue
			}
			if (project.DBName != "" && connection.Database == project.DBName) || (project.DBUser != "" && connection.User == project.DBUser) {
				return fmt.Errorf("database or login is also used by %s/%s; resolve shared ownership before replacing", other.Company, other.Name)
			}
		}
	}
	return nil
}
