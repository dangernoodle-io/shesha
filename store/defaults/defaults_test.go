package defaults_test

import (
	"context"
	"sync"
	"testing"

	"github.com/dangernoodle-io/shesha/store/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NilMap_IsEmptyNotNil(t *testing.T) {
	s := defaults.New(nil)

	m, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestNew_CopiesInput(t *testing.T) {
	src := map[string]string{"a": "1"}
	s := defaults.New(src)

	src["a"] = "mutated"

	v, ok, err := s.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "1", v, "New must copy the input map, not alias it")
}

func TestGet_MissingKey(t *testing.T) {
	s := defaults.New(nil)

	_, ok, err := s.Get(context.Background(), "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSet_ThenGet(t *testing.T) {
	s := defaults.New(nil)

	require.NoError(t, s.Set(context.Background(), "a", "1"))

	v, ok, err := s.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "1", v)
}

func TestSave_IsNoOp(t *testing.T) {
	s := defaults.New(nil)

	require.NoError(t, s.Set(context.Background(), "a", "1"))
	require.NoError(t, s.Save(context.Background()))

	v, ok, err := s.Get(context.Background(), "a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "1", v)
}

func TestDelete_RemovesKey(t *testing.T) {
	s := defaults.New(map[string]string{"a": "1"})

	require.NoError(t, s.Delete(context.Background(), "a"))

	_, ok, err := s.Get(context.Background(), "a")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestLoad_ReturnsAllPairs(t *testing.T) {
	s := defaults.New(map[string]string{"a": "1", "b": "2"})

	m, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, m)
}

func TestConcurrentReadsAndWrites_AreRace_Safe(t *testing.T) {
	s := defaults.New(map[string]string{"a": "1"})

	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()

			_, _, _ = s.Get(context.Background(), "a") //nolint:errcheck
		}()

		go func() {
			defer wg.Done()

			_ = s.Set(context.Background(), "a", "concurrent") //nolint:errcheck
		}()
	}

	wg.Wait()
}
