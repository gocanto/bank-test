// Package database provides generic, storage-agnostic glue for persisting and
// reading back snapshots through a repository. The Saver and Finder interfaces
// let any typed store plug in without this package depending on a concrete
// bounded context.
package database

import (
	"context"
	"errors"
)

type Saver[T any] interface {
	Save(context.Context, T) error
}

type Finder[T any] interface {
	Find(context.Context, string) (T, error)
}

// Persist saves value through repo. A nil repo is a no-op so callers can run
// without a store configured.
func Persist[T any](ctx context.Context, repo Saver[T], value T) error {
	if repo == nil {
		return nil
	}

	return repo.Save(ctx, value)
}

// Stored looks up id through repo. A nil repo returns notFound, and a lookup
// that resolves to notFound is surfaced unchanged.
func Stored[T any](ctx context.Context, repo Finder[T], id string, notFound error) (T, error) {
	var zero T

	if repo == nil {
		return zero, notFound
	}

	value, err := repo.Find(ctx, id)

	if errors.Is(err, notFound) {
		return zero, err
	}

	return value, err
}
