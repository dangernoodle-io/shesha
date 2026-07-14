// Package env provides a read-only store.Source adapter backed by process
// environment variables. It deliberately does not implement store.Store:
// environment variables are never a write target.
package env

import (
	"context"
	"os"
	"strings"

	"github.com/dangernoodle-io/shesha/internal/keyname"
	"github.com/dangernoodle-io/shesha/store"
)

// Option configures a Source built by New.
type Option func(*Source)

// WithKeyFunc overrides the default key transform (uppercase, every
// non-alphanumeric run collapsed to a single underscore) used to derive an
// env var name from a store key.
func WithKeyFunc(f func(string) string) Option {
	return func(s *Source) {
		s.keyFunc = f
	}
}

// Source is a read-only store.Source over environment variables sharing a
// common prefix.
type Source struct {
	prefix  string
	keyFunc func(string) string
}

// New returns a Source that reads keys as <prefix><KeyFunc(key)>.
func New(prefix string, opts ...Option) store.Source {
	s := &Source{prefix: prefix, keyFunc: defaultKeyFunc}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Get looks up the environment variable for key.
func (s *Source) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := os.LookupEnv(s.prefix + s.keyFunc(key))
	return v, ok, nil
}

// Load is best-effort: it scans os.Environ() for names starting with the
// configured prefix and reverse-maps each to a lowercase key by stripping
// the prefix. This reverse transform is lossy (the forward keyFunc is not
// generally invertible — e.g. "db.path" and "db_path" both map to
// "DB_PATH"), so Load should be treated as a convenience for
// enumeration/debugging, not as a precise inverse of Get.
func (s *Source) Load(_ context.Context) (map[string]string, error) {
	out := make(map[string]string)

	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, s.prefix) {
			continue
		}

		key := strings.ToLower(strings.TrimPrefix(name, s.prefix))
		out[key] = value
	}

	return out, nil
}

// defaultKeyFunc uppercases key and collapses every run of
// non-alphanumeric characters to a single underscore (e.g. "db.path" ->
// "DB_PATH").
func defaultKeyFunc(key string) string {
	return keyname.Upper(key)
}
