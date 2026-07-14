package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dangernoodle-io/shesha/store"
	"github.com/dangernoodle-io/shesha/store/defaults"
	"github.com/dangernoodle-io/shesha/store/env"
	"github.com/dangernoodle-io/shesha/store/xdgfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChain_ThreeLayerPrecedence_DefaultsFileEnv(t *testing.T) {
	def := defaults.New(map[string]string{"a": "defaults", "b": "defaults", "c": "defaults"})

	path := filepath.Join(t.TempDir(), "settings.json")
	file := xdgfile.NewAt(path)
	require.NoError(t, file.Set(context.Background(), "b", "file"))
	require.NoError(t, file.Set(context.Background(), "c", "file"))
	require.NoError(t, file.Save(context.Background()))

	t.Setenv("WIDGET_C", "env")
	envSrc := env.New("WIDGET_")

	c := store.NewChain(store.Read(def), store.Read(file), store.Read(envSrc))

	v, ok, err := c.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "defaults", v)

	v, ok, err = c.Get(context.Background(), "b")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "file", v, "file overrides defaults")

	v, ok, err = c.Get(context.Background(), "c")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "env", v, "env overrides both defaults and file")

	m, err := c.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "defaults", m["a"])
	assert.Equal(t, "file", m["b"])
	assert.Equal(t, "env", m["c"])
}

func TestChain_Get_PrecedenceLastWins(t *testing.T) {
	low := defaults.New(map[string]string{"a": "low", "shared": "low"})
	high := defaults.New(map[string]string{"b": "high", "shared": "high"})

	c := store.NewChain(store.Read(low), store.Read(high))

	v, ok, err := c.Get(context.Background(), "shared")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "high", v)

	v, ok, err = c.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "low", v)

	_, ok, err = c.Get(context.Background(), "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestChain_Load_MergesFirstToLast(t *testing.T) {
	low := defaults.New(map[string]string{"a": "low", "shared": "low"})
	high := defaults.New(map[string]string{"b": "high", "shared": "high"})

	c := store.NewChain(store.Read(low), store.Read(high))

	m, err := c.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "low", "b": "high", "shared": "high"}, m)
}

func TestChain_WritesRouteToWritableLayer(t *testing.T) {
	ro := defaults.New(map[string]string{"a": "1"})
	rw := defaults.New(map[string]string{"a": "2"})

	c := store.NewChain(store.Read(ro), store.Write(rw))

	require.NoError(t, c.Set(context.Background(), "a", "3"))
	require.NoError(t, c.Save(context.Background()))

	v, ok, err := rw.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "3", v, "write must land on the writable layer, not the read-only one")

	v, ok, err = ro.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "1", v, "read-only layer must be untouched")

	require.NoError(t, c.Delete(context.Background(), "a"))
	_, ok, err = rw.Get(context.Background(), "a")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestNewChain_PanicsOnTwoWritableLayers(t *testing.T) {
	a := defaults.New(nil)
	b := defaults.New(nil)

	assert.Panics(t, func() {
		store.NewChain(store.Write(a), store.Write(b))
	})
}

func TestNewChain_PanicsOnWritableLayerNotAStore(t *testing.T) {
	notAStore := readOnlySource{}

	assert.Panics(t, func() {
		store.NewChain(store.Layer{Source: notAStore, Writable: true})
	})
}

func TestChain_ReadOnly_ZeroWritableLayers_IsValid(t *testing.T) {
	a := defaults.New(map[string]string{"x": "1"})
	b := defaults.New(map[string]string{"y": "2"})

	c := store.NewChain(store.Read(a), store.Read(b))

	v, ok, err := c.Get(context.Background(), "x")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "1", v)

	m, err := c.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"x": "1", "y": "2"}, m)

	err = c.Set(context.Background(), "x", "new")
	assert.ErrorIs(t, err, store.ErrUnsupported)

	err = c.Save(context.Background())
	assert.ErrorIs(t, err, store.ErrUnsupported)

	err = c.Delete(context.Background(), "x")
	assert.ErrorIs(t, err, store.ErrUnsupported)
}

func TestChain_Get_PropagatesSourceError(t *testing.T) {
	c := store.NewChain(store.Read(erroringSource{}))

	_, _, err := c.Get(context.Background(), "any")
	assert.ErrorIs(t, err, errBoom)
}

func TestChain_Get_TopmostLayerError_FailsFastWithoutFallingThrough(t *testing.T) {
	// low has the key; high (highest precedence, probed first) errors.
	// Get must fail fast on high's error and never fall through to low.
	low := defaults.New(map[string]string{"a": "low"})
	high := erroringSource{}

	c := store.NewChain(store.Read(low), store.Read(high))

	_, ok, err := c.Get(context.Background(), "a")
	assert.ErrorIs(t, err, errBoom)
	assert.False(t, ok, "must not fall through to a lower layer after a higher layer errors")
}

func TestChain_Load_PropagatesSourceError(t *testing.T) {
	c := store.NewChain(store.Read(erroringSource{}))

	_, err := c.Load(context.Background())
	assert.ErrorIs(t, err, errBoom)
}

var errBoom = errors.New("boom")

type erroringSource struct{}

func (erroringSource) Get(context.Context, string) (string, bool, error) {
	return "", false, errBoom
}

func (erroringSource) Load(context.Context) (map[string]string, error) {
	return nil, errBoom
}

type readOnlySource struct{}

func (readOnlySource) Get(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (readOnlySource) Load(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
