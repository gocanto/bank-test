package bills

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"encore.app/fees/domain"
)

type Store struct {
	db *sql.DB
}

var ErrNotFound = errors.New("bill snapshot not found")

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Save(ctx context.Context, bill domain.Bill) error {
	if err := s.ready(); err != nil {
		return err
	}

	payload, err := json.Marshal(bill.Summary())

	if err != nil {
		return fmt.Errorf("marshal bill snapshot: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO bill_snapshots (
	id, state, period_start, period_end, created_at, closed_at, summary_json, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	state = excluded.state,
	period_start = excluded.period_start,
	period_end = excluded.period_end,
	created_at = excluded.created_at,
	closed_at = excluded.closed_at,
	summary_json = excluded.summary_json,
	updated_at = excluded.updated_at
`, bill.ID, bill.State, formatTime(bill.PeriodStart), formatTime(bill.PeriodEnd), formatTime(bill.CreatedAt), nullableTime(bill.ClosedAt), string(payload), formatTime(time.Now()))

	if err != nil {
		return fmt.Errorf("save bill snapshot: %w", err)
	}

	return nil
}

func (s *Store) Find(ctx context.Context, billID string) (domain.Bill, error) {
	if err := s.ready(); err != nil {
		return domain.Bill{}, err
	}

	var payload string
	err := s.db.QueryRowContext(ctx, `
SELECT summary_json
FROM bill_snapshots
WHERE id = ?
`, billID).Scan(&payload)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	}

	if err != nil {
		return domain.Bill{}, fmt.Errorf("find bill snapshot: %w", err)
	}

	var bill domain.Bill

	if err := json.Unmarshal([]byte(payload), &bill); err != nil {
		return domain.Bill{}, fmt.Errorf("unmarshal bill snapshot: %w", err)
	}

	return bill, nil
}

func (s *Store) ready() error {
	if s == nil || s.db == nil {
		return errors.New("bills database is required")
	}

	return nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return formatTime(*value)
}
