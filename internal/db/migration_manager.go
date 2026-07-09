package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"strings"
)

//go:embed migrations/*.surrealql
var migrationFS embed.FS

// GetLatestVersion returns the highest applied version in _sbl_migrations
func GetLatestVersion(ctx context.Context) (int, error) {
	type MigrationRecord struct {
		Version int `json:"version"`
	}

	// Fetch all records from the migrations table
	results, err := RepoQuery[[]MigrationRecord](ctx, "SELECT version FROM _sbl_migrations ORDER BY version DESC LIMIT 1;", nil)
	if err != nil {
		// If table doesn't exist yet, we are at version 0
		return 0, nil
	}

	if results == nil || len(*results) == 0 {
		return 0, nil
	}

	return (*results)[0].Version, nil
}

// BumpVersion inserts a new migration version record
func BumpVersion(ctx context.Context, version int) error {
	query := "CREATE type::record('_sbl_migrations', $version) SET version = $version, applied_at = time::now();"
	_, err := RepoQuery[any](ctx, query, map[string]any{"version": version})
	return err
}

// LowerVersion deletes the version record to rollback
func LowerVersion(ctx context.Context, version int) error {
	query := "DELETE type::record('_sbl_migrations', $version);"
	_, err := RepoQuery[any](ctx, query, map[string]any{"version": version})
	return err
}

// ReadMigrationFile reads clean SQL content from the embedded FS
func ReadMigrationFile(filename string) (string, error) {
	data, err := migrationFS.ReadFile("migrations/" + filename)
	if err != nil {
		return "", err
	}

	// Clean comments and combine lines
	lines := strings.Split(string(data), "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			cleaned = append(cleaned, trimmed)
		}
	}

	return strings.Join(cleaned, " "), nil
}

// NeedsMigration checks if there are migrations that need to be run (total 21 migrations)
func NeedsMigration(ctx context.Context) (bool, error) {
	current, err := GetLatestVersion(ctx)
	if err != nil {
		return false, err
	}
	return current < 21, nil
}

// RunMigrationUp runs all pending migrations sequentially
func RunMigrationUp(ctx context.Context) error {
	current, err := GetLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	log.Printf("[Migrations] Current database version: %d", current)

	if current >= 21 {
		log.Println("[Migrations] Database is already at the latest version. No migrations needed.")
		return nil
	}

	log.Printf("[Migrations] Pending migrations detected. Running migrations up...")

	for i := current; i < 21; i++ {
		version := i + 1
		filename := fmt.Sprintf("%d.surrealql", version)
		sql, err := ReadMigrationFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		log.Printf("[Migrations] Running migration %d...", version)

		_, err = RepoQuery[any](ctx, sql, nil)
		if err != nil {
			return fmt.Errorf("migration %d failed: %w", version, err)
		}

		// Bump version record
		if err := BumpVersion(ctx, version); err != nil {
			return fmt.Errorf("failed to bump database version after migration %d: %w", version, err)
		}

		log.Printf("[Migrations] Migration %d applied successfully.", version)
	}

	newVersion, err := GetLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify final database version: %w", err)
	}

	log.Printf("[Migrations] Migrations completed successfully. Database is now at version %d", newVersion)
	return nil
}

// RunMigrationDown rolls back the single latest migration
func RunMigrationDown(ctx context.Context) error {
	current, err := GetLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if current <= 0 {
		log.Println("[Migrations] No migrations to rollback.")
		return nil
	}

	filename := fmt.Sprintf("%d_down.surrealql", current)
	sql, err := ReadMigrationFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read rollback file %s: %w", filename, err)
	}

	log.Printf("[Migrations] Rolling back migration %d...", current)

	_, err = RepoQuery[any](ctx, sql, nil)
	if err != nil {
		return fmt.Errorf("rollback of migration %d failed: %w", current, err)
	}

	if err := LowerVersion(ctx, current); err != nil {
		return fmt.Errorf("failed to update version record after rollback: %w", err)
	}

	log.Printf("[Migrations] Rollback of migration %d completed successfully.", current)
	return nil
}

// CheckDatabaseHealth runs a lightweight query to verify connection status
func CheckDatabaseHealth(ctx context.Context) (string, error) {
	type CountResult struct {
		Count int `json:"count"`
	}

	// Lightweight query
	_, err := RepoQuery[any](ctx, "RETURN 1;", nil)
	if err != nil {
		return "offline", err
	}
	return "online", nil
}
