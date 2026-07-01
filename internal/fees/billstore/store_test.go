package billstore_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gocanto.sh/bank/internal/fees/billstore"
	"gocanto.sh/bank/internal/fees/domain"
	"gocanto.sh/bank/internal/platform/sqlite"
)

func TestStoreFindReturnsNotFound(t *testing.T) {
	store, _ := openStore(t)

	_, err := store.Find(context.Background(), "missing")

	if !errors.Is(err, billstore.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreSavesAndFindsBillSnapshot(t *testing.T) {
	store, _ := openStore(t)
	bill := newBill(t, "bill-sqlite")

	if err := store.Save(context.Background(), bill); err != nil {
		t.Fatalf("save bill: %v", err)
	}

	got, err := store.Find(context.Background(), bill.ID)

	if err != nil {
		t.Fatalf("find bill: %v", err)
	}

	if got.ID != bill.ID {
		t.Fatalf("expected bill ID %q, got %q", bill.ID, got.ID)
	}

	if got.State != domain.StateOpen {
		t.Fatalf("expected open state, got %q", got.State)
	}

	if len(got.LineItems) != 1 {
		t.Fatalf("expected one line item, got %d", len(got.LineItems))
	}

	if got.Totals[0].Amount != 1250 || got.Totals[0].Currency != "USD" {
		t.Fatalf("unexpected totals: %#v", got.Totals)
	}
}

func TestStoreAppendsAndFindsLatestBillSnapshot(t *testing.T) {
	store, db := openStore(t)
	bill := newBill(t, "bill-close")

	if err := store.Save(context.Background(), bill); err != nil {
		t.Fatalf("save open bill: %v", err)
	}

	closed, err := bill.Close(time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC))

	if err != nil {
		t.Fatalf("close bill: %v", err)
	}

	if err := store.Save(context.Background(), closed.Summary()); err != nil {
		t.Fatalf("save closed bill: %v", err)
	}

	got, err := store.Find(context.Background(), bill.ID)

	if err != nil {
		t.Fatalf("find bill: %v", err)
	}

	if got.State != domain.StateClosed {
		t.Fatalf("expected closed state, got %q", got.State)
	}

	if got.ClosedAt == nil {
		t.Fatal("expected closed_at timestamp")
	}

	if count := countSnapshots(t, db, bill.ID); count != 2 {
		t.Fatalf("expected two appended snapshots, got %d", count)
	}
}

func openStore(t *testing.T) (*billstore.Store, *sql.DB) {
	t.Helper()

	dir := t.TempDir()
	db, err := sqlite.Open(dir, filepath.Join(dir, "gocanto.sqlite3"))

	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})

	return billstore.New(db), db
}

func countSnapshots(t *testing.T, db *sql.DB, billID string) int {
	t.Helper()

	var count int

	if err := db.QueryRow("SELECT COUNT(*) FROM bill_snapshots WHERE bill_id = ?", billID).Scan(&count); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}

	return count
}

func newBill(t *testing.T, billID string) domain.Bill {
	t.Helper()

	now := time.Date(2026, time.June, 30, 10, 0, 0, 0, time.UTC)
	bill, err := domain.NewBill(domain.CreateBill{
		BillID:      billID,
		PeriodStart: now,
		PeriodEnd:   now.Add(24 * time.Hour),
	}, now)

	if err != nil {
		t.Fatalf("new bill: %v", err)
	}

	amount, err := domain.NewMoney(1250, "USD")

	if err != nil {
		t.Fatalf("new money: %v", err)
	}

	if _, err := bill.AddLineItem(domain.AddLineItem{
		ID:          "li-001",
		Description: "Card processing fee",
		Amount:      amount,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("add line item: %v", err)
	}

	return bill.Summary()
}
