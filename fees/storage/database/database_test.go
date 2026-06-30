package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesBillSnapshotSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir, filepath.Join(dir, "pavebank.sqlite3"))

	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	defer db.Close()

	for _, name := range []string{
		"bill_snapshots",
		"idx_bill_snapshots_state",
		"idx_bill_snapshots_period_end",
	} {
		if !objectExists(t, db, name) {
			t.Fatalf("expected database object %q to exist", name)
		}
	}
}

func objectExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()

	var count int

	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = ?", name).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}

	return count == 1
}
