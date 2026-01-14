package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed sql/*.sql
var migrationsFS embed.FS

// Migration represents a database migration
type Migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
}

// Migrator handles database migrations
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// RunMigrations runs all pending migrations
func (m *Migrator) RunMigrations() error {
	// Ensure schema_migrations table exists
	if err := m.ensureMigrationTable(); err != nil {
		return fmt.Errorf("failed to ensure migration table: %w", err)
	}

	// Get all migrations from embedded files
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	log.Printf("Current migration version: %d", currentVersion)

	// Run pending migrations
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			log.Printf("Running migration %d: %s", migration.Version, migration.Name)
			if err := m.runMigration(migration); err != nil {
				return fmt.Errorf("failed to run migration %d: %w", migration.Version, err)
			}
			log.Printf("Migration %d completed successfully", migration.Version)
		}
	}

	return nil
}

// ensureMigrationTable creates the schema_migrations table if it doesn't exist
func (m *Migrator) ensureMigrationTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN DEFAULT false,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// getCurrentVersion returns the current migration version
func (m *Migrator) getCurrentVersion() (int64, error) {
	var version sql.NullInt64
	err := m.db.QueryRow(`
		SELECT MAX(version) FROM schema_migrations WHERE NOT dirty
	`).Scan(&version)
	if err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return version.Int64, nil
}

// loadMigrations loads all migration files from embedded filesystem
func (m *Migrator) loadMigrations() ([]Migration, error) {
	migrations := make(map[int64]*Migration)

	err := fs.WalkDir(migrationsFS, "sql", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		filename := filepath.Base(path)
		if !strings.HasSuffix(filename, ".sql") {
			return nil
		}

		// Parse version from filename (e.g., 000001_init_schema.up.sql)
		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			return nil
		}

		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil
		}

		// Read file content
		content, err := fs.ReadFile(migrationsFS, path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		// Initialize migration if not exists
		if _, ok := migrations[version]; !ok {
			nameParts := strings.Split(strings.TrimSuffix(filename, ".sql"), ".")
			name := strings.Join(strings.Split(nameParts[0], "_")[1:], "_")
			migrations[version] = &Migration{
				Version: version,
				Name:    name,
			}
		}

		// Set up or down SQL
		if strings.Contains(filename, ".up.") {
			migrations[version].UpSQL = string(content)
		} else if strings.Contains(filename, ".down.") {
			migrations[version].DownSQL = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to sorted slice
	var result []Migration
	for _, m := range migrations {
		if m.UpSQL != "" { // Only include migrations with up SQL
			result = append(result, *m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// runMigration executes a single migration
func (m *Migrator) runMigration(migration Migration) error {
	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Mark migration as in progress (dirty)
	_, err = tx.Exec(`
		INSERT INTO schema_migrations (version, dirty) VALUES ($1, true)
		ON CONFLICT (version) DO UPDATE SET dirty = true
	`, migration.Version)
	if err != nil {
		return fmt.Errorf("failed to mark migration as dirty: %w", err)
	}

	// Execute migration SQL
	_, err = tx.Exec(migration.UpSQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Mark migration as complete
	_, err = tx.Exec(`
		UPDATE schema_migrations SET dirty = false, applied_at = CURRENT_TIMESTAMP WHERE version = $1
	`, migration.Version)
	if err != nil {
		return fmt.Errorf("failed to mark migration as complete: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback rolls back the last migration
func (m *Migrator) Rollback() error {
	// Get current version
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if currentVersion == 0 {
		log.Println("No migrations to rollback")
		return nil
	}

	// Load migrations
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Find migration to rollback
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == currentVersion {
			migration = &migrations[i]
			break
		}
	}

	if migration == nil || migration.DownSQL == "" {
		return fmt.Errorf("no down migration found for version %d", currentVersion)
	}

	log.Printf("Rolling back migration %d: %s", migration.Version, migration.Name)

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute down migration
	_, err = tx.Exec(migration.DownSQL)
	if err != nil {
		return fmt.Errorf("failed to execute down migration: %w", err)
	}

	// Remove migration record
	_, err = tx.Exec(`DELETE FROM schema_migrations WHERE version = $1`, migration.Version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Rollback of migration %d completed successfully", migration.Version)
	return nil
}
