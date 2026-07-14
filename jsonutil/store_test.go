package jsonutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dangernoodle-io/shesha/jsonutil"
	"github.com/dangernoodle-io/shesha/store/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failStore is a store.Store test double whose every method fails with
// errGetSet, for exercising jsonutil's error-wrapping paths against a
// failing backend.
type failStore struct{}

var errGetSet = errors.New("errStore: boom")

func (failStore) Get(context.Context, string) (string, bool, error) {
	return "", false, errGetSet
}

func (failStore) Load(context.Context) (map[string]string, error) {
	return nil, errGetSet
}

func (failStore) Set(context.Context, string, string) error {
	return errGetSet
}

func (failStore) Save(context.Context) error {
	return errGetSet
}

func (failStore) Delete(context.Context, string) error {
	return errGetSet
}

func TestSetJSON_GetJSON_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := defaults.New(nil)

	in := widget{Name: "gizmo", Count: 3}
	require.NoError(t, jsonutil.SetJSON(ctx, s, "widget", in))

	out, err := jsonutil.GetJSON[widget](ctx, s, "widget")
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestGetJSON_MissingKey(t *testing.T) {
	ctx := context.Background()
	s := defaults.New(nil)

	_, err := jsonutil.GetJSON[widget](ctx, s, "missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, jsonutil.ErrNotFound)
}

func TestGetJSON_BadJSON(t *testing.T) {
	ctx := context.Background()
	s := defaults.New(map[string]string{"widget": "not json"})

	_, err := jsonutil.GetJSON[widget](ctx, s, "widget")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonutil: get \"widget\"")
}

func TestGetJSON_SourceError(t *testing.T) {
	_, err := jsonutil.GetJSON[widget](context.Background(), failStore{}, "widget")
	require.Error(t, err)
	assert.ErrorIs(t, err, errGetSet)
	assert.Contains(t, err.Error(), "jsonutil: get \"widget\"")
}

func TestSetJSON_MarshalError(t *testing.T) {
	err := jsonutil.SetJSON(context.Background(), defaults.New(nil), "bad", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonutil: set \"bad\"")
}

func TestSetJSON_StoreError(t *testing.T) {
	err := jsonutil.SetJSON(context.Background(), failStore{}, "widget", widget{Name: "gizmo"})
	require.Error(t, err)
	assert.ErrorIs(t, err, errGetSet)
}
