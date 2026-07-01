package bills

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

type Snapshot struct {
	BillID        string
	State         string
	PeriodStart   string
	PeriodEnd     string
	BillCreatedAt string
	ClosedAt      any
	SummaryJSON   string
	RecordedAt    string
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	if err := r.ready(); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO bill_snapshots (
	bill_id, state, period_start, period_end, bill_created_at, closed_at, summary_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, snapshot.BillID, snapshot.State, snapshot.PeriodStart, snapshot.PeriodEnd, snapshot.BillCreatedAt, snapshot.ClosedAt, snapshot.SummaryJSON, snapshot.RecordedAt)

	if err != nil {
		return fmt.Errorf("insert bill snapshot: %w", err)
	}

	return nil
}

func (r *Repository) FindLatestSnapshot(ctx context.Context, billID string) (Snapshot, error) {
	if err := r.ready(); err != nil {
		return Snapshot{}, err
	}

	var snapshot Snapshot

	err := r.db.QueryRowContext(ctx, `
SELECT summary_json
FROM bill_snapshots
WHERE bill_id = ?
ORDER BY recorded_at DESC, snapshot_id DESC
LIMIT 1
`, billID).Scan(&snapshot.SummaryJSON)

	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, ErrNotFound
	}

	if err != nil {
		return Snapshot{}, fmt.Errorf("select latest bill snapshot: %w", err)
	}

	snapshot.BillID = billID

	return snapshot, nil
}

func (r *Repository) ready() error {
	if r == nil || r.db == nil {
		return errors.New("bills database is required")
	}

	return nil
}
