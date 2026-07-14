package xdgfile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dangernoodle-io/shesha/store/xdgfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAt_MissingFile_IsEmptyNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	s := xdgfile.NewAt(path)

	m, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestNew_MissingFile_IsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WIDGET_CONFIG_DIR", dir)

	s, err := xdgfile.New("widget", "settings.json")
	require.NoError(t, err)

	m, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestNew_MalformedFile_ErrorsAtConstruction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))
	t.Setenv("WIDGET_CONFIG_DIR", dir)

	_, err := xdgfile.New("widget", "settings.json")
	assert.Error(t, err)
}

func TestSetSaveReload_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	s := xdgfile.NewAt(path)
	require.NoError(t, s.Set(context.Background(), "a", "1"))
	require.NoError(t, s.Set(context.Background(), "b", "2"))
	require.NoError(t, s.Save(context.Background()))

	reloaded := xdgfile.NewAt(path)
	m, err := reloaded.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, m)
}

func TestDelete_RemovesKey_DurableAfterSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	s := xdgfile.NewAt(path)
	require.NoError(t, s.Set(context.Background(), "a", "1"))
	require.NoError(t, s.Save(context.Background()))

	require.NoError(t, s.Delete(context.Background(), "a"))

	// Not yet saved: in-memory view already reflects the delete.
	_, ok, err := s.Get(context.Background(), "a")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.Save(context.Background()))

	reloaded := xdgfile.NewAt(path)
	_, ok, err = reloaded.Get(context.Background(), "a")
	require.NoError(t, err)
	assert.False(t, ok, "delete must be durable after Save")
}

func TestSave_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")

	s := xdgfile.NewAt(path)
	require.NoError(t, s.Set(context.Background(), "a", "1"))
	require.NoError(t, s.Save(context.Background()))

	_, err := os.Stat(path)
	require.NoError(t, err)
}

func TestGet_MissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := xdgfile.NewAt(path)

	_, ok, err := s.Get(context.Background(), "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}
