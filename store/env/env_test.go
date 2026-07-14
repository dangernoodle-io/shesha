package env_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dangernoodle-io/shesha/store"
	"github.com/dangernoodle-io/shesha/store/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_IsNotAStore(t *testing.T) {
	_, ok := env.New("WIDGET_").(store.Store)
	assert.False(t, ok, "env.Source must never satisfy store.Store — env is never a write target")
}

func TestGet_DefaultKeyFunc(t *testing.T) {
	t.Setenv("WIDGET_DB_PATH", "/tmp/db")

	s := env.New("WIDGET_")

	v, ok, err := s.Get(context.Background(), "db.path")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "/tmp/db", v)
}

func TestGet_MissingKey(t *testing.T) {
	s := env.New("WIDGET_MISSING_PREFIX_")

	_, ok, err := s.Get(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGet_CustomKeyFunc(t *testing.T) {
	t.Setenv("WIDGET_dbpath", "/tmp/db")

	s := env.New("WIDGET_", env.WithKeyFunc(func(k string) string {
		return strings.ReplaceAll(k, ".", "")
	}))

	v, ok, err := s.Get(context.Background(), "db.path")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "/tmp/db", v)
}

func TestLoad_BestEffort(t *testing.T) {
	t.Setenv("WIDGET_DB_PATH", "/tmp/db")
	t.Setenv("WIDGET_PORT", "8080")
	t.Setenv("OTHER_VAR", "ignored")

	s := env.New("WIDGET_")

	m, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/tmp/db", m["db_path"])
	assert.Equal(t, "8080", m["port"])
	_, ok := m["OTHER_VAR"]
	assert.False(t, ok)
}
