package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

// Run executes all pending migrations in order
func Run(db *sql.DB) error {
	log.Println("🔄 Running database migrations...")

	// Create migrations tracking table
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Read migration files
	entries, err := migrationFiles.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migration files by name
	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			fileNames = append(fileNames, entry.Name())
		}
	}
	sort.Strings(fileNames)

	// Run each migration that hasn't been applied
	appliedCount := 0
	for _, fileName := range fileNames {
		if applied[fileName] {
			continue
		}

		log.Printf("  Applying: %s", fileName)

		content, err := migrationFiles.ReadFile(filepath.Join("sql", fileName))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", fileName, err)
		}

		// Execute migration in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", fileName, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
		}

		// Record migration as applied
		if _, err := tx.Exec("INSERT INTO schema_migrations (filename) VALUES ($1)", fileName); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", fileName, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", fileName, err)
		}

		appliedCount++
	}

	if appliedCount > 0 {
		log.Printf("✅ Applied %d migration(s)", appliedCount)
	} else {
		log.Println("✅ Database schema is up to date")
	}

	return nil
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func createMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id SERIAL PRIMARY KEY,
			filename VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}

// getAppliedMigrations returns a map of already-applied migration filenames
func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	applied := make(map[string]bool)

	rows, err := db.Query("SELECT filename FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		applied[filename] = true
	}

	return applied, rows.Err()
}
