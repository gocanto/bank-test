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
	repository *Repository
}

var ErrNotFound = errors.New("bill snapshot not found")

func New(db *sql.DB) *Store {
	return NewWithRepository(NewRepository(db))
}

func NewWithRepository(repository *Repository) *Store {
	return &Store{repository: repository}
}

func (s *Store) Save(ctx context.Context, bill domain.Bill) error {
	if err := s.ready(); err != nil {
		return err
	}

	payload, err := json.Marshal(bill.Summary())

	if err != nil {
		return fmt.Errorf("marshal bill snapshot: %w", err)
	}

	if err := s.repository.SaveSnapshot(ctx, Snapshot{
		BillID:        bill.ID,
		State:         bill.State,
		PeriodStart:   formatTime(bill.PeriodStart),
		PeriodEnd:     formatTime(bill.PeriodEnd),
		BillCreatedAt: formatTime(bill.CreatedAt),
		ClosedAt:      nullableTime(bill.ClosedAt),
		SummaryJSON:   string(payload),
		RecordedAt:    formatTime(time.Now()),
	}); err != nil {
		return fmt.Errorf("save bill snapshot: %w", err)
	}

	return nil
}

func (s *Store) Find(ctx context.Context, billID string) (domain.Bill, error) {
	if err := s.ready(); err != nil {
		return domain.Bill{}, err
	}

	snapshot, err := s.repository.FindLatestSnapshot(ctx, billID)

	if err != nil {
		return domain.Bill{}, fmt.Errorf("find bill snapshot: %w", err)
	}

	var bill domain.Bill

	if err := json.Unmarshal([]byte(snapshot.SummaryJSON), &bill); err != nil {
		return domain.Bill{}, fmt.Errorf("unmarshal bill snapshot: %w", err)
	}

	return bill, nil
}

func (s *Store) ready() error {
	if s == nil || s.repository == nil {
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

	formatted := formatTime(*value)

	return formatted
}
