// Package xdgfile provides a store.Store adapter backed by a flat JSON
// object on disk, located via xdgpath.
package xdgfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/dangernoodle-io/shesha/store"
	"github.com/dangernoodle-io/shesha/xdgpath"
)

// New returns a store.Store backed by app's config file name, resolved via
// xdgpath.ConfigFile. The file is loaded eagerly, surfacing a malformed
// file at construction time. A missing file is treated as empty, not an
// error.
func New(app, name string) (store.Store, error) {
	path := xdgpath.ConfigFile(app, name)

	f := &fileStore{path: path}
	if err := f.load(); err != nil {
		return nil, err
	}

	return f, nil
}

// NewAt returns a store.Store backed by the JSON file at path. Unlike New,
// loading is lazy (deferred to the first operation), and a missing or
// unreadable file is treated as empty rather than surfaced as an error;
// only a malformed-JSON file surfaces an error, on first use.
func NewAt(path string) store.Store {
	return &fileStore{path: path}
}

type fileStore struct {
	mu     sync.Mutex
	path   string
	data   map[string]string
	loaded bool
}

// load reads and parses the backing file, if not already loaded. A missing
// file is treated as empty.
func (f *fileStore) load() error {
	if f.loaded {
		return nil
	}

	b, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			f.data = make(map[string]string)
			f.loaded = true

			return nil
		}

		return err
	}

	m := make(map[string]string)
	if len(b) > 0 {
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
	}

	f.data = m
	f.loaded = true

	return nil
}

// Get returns the value for key, and whether it was found.
func (f *fileStore) Get(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.load(); err != nil {
		return "", false, err
	}

	v, ok := f.data[key]

	return v, ok, nil
}

// Load returns a copy of every key/value pair currently held.
func (f *fileStore) Load(_ context.Context) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.load(); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(f.data))
	for k, v := range f.data {
		out[k] = v
	}

	return out, nil
}

// Set stages value for key in memory (read-your-writes); call Save to
// flush to disk.
func (f *fileStore) Set(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.load(); err != nil {
		return err
	}

	f.data[key] = value

	return nil
}

// Delete removes key from the in-memory map; call Save to make it
// durable.
func (f *fileStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.load(); err != nil {
		return err
	}

	delete(f.data, key)

	return nil
}

// Save writes the current in-memory map to disk as a JSON object,
// creating the parent directory if needed.
func (f *fileStore) Save(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.load(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(f.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.path, b, 0o644)
}
