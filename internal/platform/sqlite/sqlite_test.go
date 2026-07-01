package sqlite

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCreatesBillSnapshotSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir, filepath.Join(dir, "gocanto.sqlite3"))

	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	defer db.Close()

	for _, name := range []string{
		"bill_snapshots",
		"idx_bill_snapshots_bill_id",
		"idx_bill_snapshots_state",
		"idx_bill_snapshots_period_end",
		"idx_bill_snapshots_latest",
		"trg_bill_snapshots_prevent_update",
		"trg_bill_snapshots_prevent_delete",
	} {
		if !objectExists(t, db, name) {
			t.Fatalf("expected database object %q to exist", name)
		}
	}
}

func TestMigrateRejectsBillSnapshotMutations(t *testing.T) {
	db := openMemoryDB(t, "append-only-mutations")

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	insertBillSnapshot(t, db, "bill-append-only", "open")

	if _, err := db.Exec("UPDATE bill_snapshots SET state = ? WHERE bill_id = ?", "closed", "bill-append-only"); !isAppendOnlyError(err) {
		t.Fatalf("expected append-only update error, got %v", err)
	}

	if _, err := db.Exec("DELETE FROM bill_snapshots WHERE bill_id = ?", "bill-append-only"); !isAppendOnlyError(err) {
		t.Fatalf("expected append-only delete error, got %v", err)
	}
}

func TestMigratePreservesLegacyBillSnapshots(t *testing.T) {
	db := openMemoryDB(t, "legacy-bill-snapshots")

	if _, err := db.Exec(`CREATE TABLE bill_snapshots (
		id TEXT PRIMARY KEY,
		state TEXT NOT NULL,
		period_start TEXT NOT NULL,
		period_end TEXT NOT NULL,
		created_at TEXT NOT NULL,
		closed_at TEXT,
		summary_json TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO bill_snapshots (
		id, state, period_start, period_end, created_at, closed_at, summary_json, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-bill", "closed", "2026-06-01T00:00:00Z", "2026-07-01T00:00:00Z", "2026-06-01T00:00:00Z", "2026-06-30T00:00:00Z", `{"id":"legacy-bill"}`, "2026-06-30T00:00:00Z"); err != nil {
		t.Fatalf("insert legacy snapshot: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	if exists, err := columnExists(db, "bill_snapshots", "id"); err != nil {
		t.Fatalf("check legacy id column: %v", err)
	} else if exists {
		t.Fatal("expected legacy id column to be replaced")
	}

	var (
		billID        string
		billCreatedAt string
		recordedAt    string
		summaryJSON   string
	)

	if err := db.QueryRow(`SELECT bill_id, bill_created_at, recorded_at, summary_json
		FROM bill_snapshots
		WHERE bill_id = ?`, "legacy-bill").Scan(&billID, &billCreatedAt, &recordedAt, &summaryJSON); err != nil {
		t.Fatalf("query migrated snapshot: %v", err)
	}

	if billID != "legacy-bill" || billCreatedAt != "2026-06-01T00:00:00Z" || recordedAt != "2026-06-30T00:00:00Z" || summaryJSON != `{"id":"legacy-bill"}` {
		t.Fatalf("unexpected migrated snapshot: bill_id=%q bill_created_at=%q recorded_at=%q summary_json=%q", billID, billCreatedAt, recordedAt, summaryJSON)
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

func openMemoryDB(t *testing.T, name string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+name+"?mode=memory&cache=shared")

	if err != nil {
		t.Fatalf("open memory database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close memory database: %v", err)
		}
	})

	return db
}

func insertBillSnapshot(t *testing.T, db *sql.DB, billID string, state string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO bill_snapshots (
		bill_id, state, period_start, period_end, bill_created_at, closed_at, summary_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		billID, state, "2026-06-01T00:00:00Z", "2026-07-01T00:00:00Z", "2026-06-01T00:00:00Z", nil, `{"id":"`+billID+`"}`, "2026-06-01T00:00:00Z"); err != nil {
		t.Fatalf("insert bill snapshot: %v", err)
	}
}

func isAppendOnlyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "append-only")
}
