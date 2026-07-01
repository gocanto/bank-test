package bills

import (
	"errors"
	"time"
)

var errDatabaseRequired = errors.New("bills database is required")

var ErrNotFound = errors.New("bill snapshot not found")

func (s *Store) ready() error {
	if s == nil || s.repository == nil {
		return errDatabaseRequired
	}

	return nil
}

func (r *Repository) ready() error {
	if r == nil || r.db == nil {
		return errDatabaseRequired
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
