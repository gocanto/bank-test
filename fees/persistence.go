package fees

import (
	"context"
	"errors"
)

type saver[T any] interface {
	Save(context.Context, T) error
}

type finder[T any] interface {
	Find(context.Context, string) (T, error)
}

func persist[T any](ctx context.Context, repo saver[T], value T) error {
	if repo == nil {
		return nil
	}

	return repo.Save(ctx, value)
}

func stored[T any](ctx context.Context, repo finder[T], id string, notFound error) (T, error) {
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
