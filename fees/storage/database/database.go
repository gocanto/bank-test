package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const fileName = "gocanto.sqlite3"

func DefaultPath(repoRoot string) string {
	if value := strings.TrimSpace(os.Getenv("GOCANTO_DATABASE_PATH")); value != "" {
		return value
	}

	return filepath.Join(repoRoot, "storage", "database", fileName)
}

func Open(repoRoot string, databasePath string) (*sql.DB, error) {
	if strings.TrimSpace(databasePath) == "" {
		databasePath = DefaultPath(repoRoot)
	}

	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", databasePath)

	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := configure(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	if err := Migrate(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	if db == nil {
		return errors.New("database is required")
	}

	if err := configure(db); err != nil {
		return err
	}

	for _, statement := range migrationStatements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("run migration statement: %w", err)
		}
	}

	return nil
}

func configure(db *sql.DB) error {
	statements := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure sqlite: %w", err)
		}
	}

	return nil
}

var migrationStatements = []string{
	`CREATE TABLE IF NOT EXISTS bill_snapshots (
		id TEXT PRIMARY KEY,
		state TEXT NOT NULL,
		period_start TEXT NOT NULL,
		period_end TEXT NOT NULL,
		created_at TEXT NOT NULL,
		closed_at TEXT,
		summary_json TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	"CREATE INDEX IF NOT EXISTS idx_bill_snapshots_state ON bill_snapshots(state)",
	"CREATE INDEX IF NOT EXISTS idx_bill_snapshots_period_end ON bill_snapshots(period_end)",
}
