package jsonutil

import (
	"context"
	"fmt"

	"github.com/dangernoodle-io/shesha/store"
)

// GetJSON reads key from s and JSON-decodes it into T. It returns
// ErrNotFound (wrapped) if s has no value for key, or a wrapped decode error
// if the stored value is not valid JSON for T.
func GetJSON[T any](ctx context.Context, s store.Source, key string) (T, error) {
	var out T

	raw, ok, err := s.Get(ctx, key)
	if err != nil {
		return out, fmt.Errorf("jsonutil: get %q: %w", key, err)
	}

	if !ok {
		return out, fmt.Errorf("jsonutil: get %q: %w", key, ErrNotFound)
	}

	if err := Unmarshal([]byte(raw), &out); err != nil {
		return out, fmt.Errorf("jsonutil: get %q: %w", key, err)
	}

	return out, nil
}

// SetJSON JSON-encodes v and stages it into s under key via Store.Set. Call
// s.Save separately to make the write durable, per the store.Store contract.
func SetJSON[T any](ctx context.Context, s store.Store, key string, v T) error {
	b, err := Marshal(v)
	if err != nil {
		return fmt.Errorf("jsonutil: set %q: %w", key, err)
	}

	return s.Set(ctx, key, string(b))
}
