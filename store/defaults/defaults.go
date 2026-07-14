// Package defaults provides an in-memory store.Store adapter, typically
// used as the lowest-precedence layer in a store.Chain.
package defaults

import (
	"context"
	"sync"

	"github.com/dangernoodle-io/shesha/store"
)

// New returns a store.Store backed by an in-memory copy of kv. A nil kv is
// treated as empty. Save is a no-op: writes are already in memory.
func New(kv map[string]string) store.Store {
	m := make(map[string]string, len(kv))
	for k, v := range kv {
		m[k] = v
	}

	return &memStore{data: m}
}

type memStore struct {
	mu   sync.RWMutex
	data map[string]string
}

// Get returns the value for key, and whether it was found.
func (m *memStore) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.data[key]

	return v, ok, nil
}

// Load returns a copy of every key/value pair currently held.
func (m *memStore) Load(_ context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]string, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}

	return out, nil
}

// Set stores value for key, effective immediately.
func (m *memStore) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value

	return nil
}

// Save is a no-op: writes are already durable in memory for the lifetime
// of the process.
func (m *memStore) Save(_ context.Context) error {
	return nil
}

// Delete removes key.
func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)

	return nil
}
