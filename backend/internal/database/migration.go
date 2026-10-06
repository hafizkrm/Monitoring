package database

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"sort"
)

// Migrate runs all .sql files in the migrations directory
func Migrate(db *sql.DB) error {
	// Create migrations table to track applied migrations
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	migrationsDir := "internal/database/migrations"
	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		// Fallback for when running from root
		migrationsDir = "backend/internal/database/migrations"
		files, err = ioutil.ReadDir(migrationsDir)
		if err != nil {
			// Fallback for Docker deployment where it might be in ./migrations
			migrationsDir = "migrations"
			files, err = ioutil.ReadDir(migrationsDir)
			if err != nil {
				return fmt.Errorf("could not read migrations directory, exactly one authoritative migration source is required: %v", err)
			}
		}
	}

	var sqlFiles []string
	migrationRegex := regexp.MustCompile(`^\d{3}_.*\.sql$`)
	for _, f := range files {
		if !f.IsDir() && migrationRegex.MatchString(f.Name()) {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, file := range sqlFiles {
		var exists bool
		err := db.QueryRow("SELECT true FROM schema_migrations WHERE version = ?", file).Scan(&exists)
		if err == nil && exists {
			continue // Already applied
		}

		log.Printf("Applying migration: %s", file)
		content, err := ioutil.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", file, err)
		}

		var migrationErr error
		// Execute the entire file directly without splitting, requires multiStatements=true in DSN
		if _, err := tx.Exec(string(content)); err != nil {
			migrationErr = fmt.Errorf("error executing migration %s: %w", file, err)
		}

		if migrationErr != nil {
			tx.Rollback()
			return migrationErr
		}

		_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", file)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}
	}

	log.Println("Database migration completed successfully.")
	return nil
}
