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

	if err := migrateBillSnapshots(db); err != nil {
		return err
	}

	for _, statement := range billSnapshotSchemaStatements {
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

func migrateBillSnapshots(db *sql.DB) error {
	exists, err := databaseObjectExists(db, "table", "bill_snapshots")

	if err != nil {
		return err
	}

	if !exists {
		if _, err := db.Exec(createBillSnapshotsTable); err != nil {
			return fmt.Errorf("create bill snapshots table: %w", err)
		}

		return nil
	}

	hasLegacyID, err := columnExists(db, "bill_snapshots", "id")

	if err != nil {
		return err
	}

	hasBillID, err := columnExists(db, "bill_snapshots", "bill_id")

	if err != nil {
		return err
	}

	if !hasLegacyID || hasBillID {
		return nil
	}

	statements := []string{
		"ALTER TABLE bill_snapshots RENAME TO bill_snapshots_legacy",
		createBillSnapshotsTable,
		`INSERT INTO bill_snapshots (
			bill_id, state, period_start, period_end, bill_created_at, closed_at, summary_json, recorded_at
		)
		SELECT id, state, period_start, period_end, created_at, closed_at, summary_json, updated_at
		FROM bill_snapshots_legacy
		ORDER BY updated_at, id`,
		"DROP TABLE bill_snapshots_legacy",
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("migrate bill snapshots table: %w", err)
		}
	}

	return nil
}

func databaseObjectExists(db *sql.DB, objectType string, name string) (bool, error) {
	var count int

	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?", objectType, name).Scan(&count); err != nil {
		return false, fmt.Errorf("query sqlite_master: %w", err)
	}

	return count == 1, nil
}

func columnExists(db *sql.DB, tableName string, columnName string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")

	if err != nil {
		return false, fmt.Errorf("query table info: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)

		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return false, fmt.Errorf("scan table info: %w", err)
		}

		if name == columnName {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table info: %w", err)
	}

	return false, nil
}

const createBillSnapshotsTable = `CREATE TABLE IF NOT EXISTS bill_snapshots (
	snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
	bill_id TEXT NOT NULL,
	state TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	bill_created_at TEXT NOT NULL,
	closed_at TEXT,
	summary_json TEXT NOT NULL,
	recorded_at TEXT NOT NULL
)`

var billSnapshotSchemaStatements = []string{
	"CREATE INDEX IF NOT EXISTS idx_bill_snapshots_bill_id ON bill_snapshots(bill_id)",
	"CREATE INDEX IF NOT EXISTS idx_bill_snapshots_state ON bill_snapshots(state)",
	"CREATE INDEX IF NOT EXISTS idx_bill_snapshots_period_end ON bill_snapshots(period_end)",
	"CREATE INDEX IF NOT EXISTS idx_bill_snapshots_latest ON bill_snapshots(bill_id, recorded_at DESC, snapshot_id DESC)",
	`CREATE TRIGGER IF NOT EXISTS trg_bill_snapshots_prevent_update
	BEFORE UPDATE ON bill_snapshots
	BEGIN
		SELECT RAISE(ABORT, 'bill_snapshots is append-only');
	END`,
	`CREATE TRIGGER IF NOT EXISTS trg_bill_snapshots_prevent_delete
	BEFORE DELETE ON bill_snapshots
	BEGIN
		SELECT RAISE(ABORT, 'bill_snapshots is append-only');
	END`,
}
