package jsonutil_test

import (
	"encoding/json"
	"testing"

	"github.com/dangernoodle-io/shesha/jsonutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type widget struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestMarshal_Unmarshal_RoundTrip(t *testing.T) {
	in := widget{Name: "gizmo", Count: 3}

	b, err := jsonutil.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"gizmo","count":3}`, string(b))

	var out widget
	err = jsonutil.Unmarshal(b, &out)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestMarshal_Compact(t *testing.T) {
	b, err := jsonutil.Marshal(widget{Name: "gizmo", Count: 3})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "\n", "Marshal must produce compact, single-line output")
}

func TestMarshalIndent_Indented(t *testing.T) {
	b, err := jsonutil.MarshalIndent(widget{Name: "gizmo", Count: 3})
	require.NoError(t, err)
	assert.Contains(t, string(b), "\n  \"name\"")

	var out widget
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, widget{Name: "gizmo", Count: 3}, out)
}

func TestMarshal_ErrorWrapped(t *testing.T) {
	_, err := jsonutil.Marshal(make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonutil: marshal")
}

func TestMarshalIndent_ErrorWrapped(t *testing.T) {
	_, err := jsonutil.MarshalIndent(make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonutil: marshal indent")
}

func TestUnmarshal_ErrorWrapped(t *testing.T) {
	var out widget
	err := jsonutil.Unmarshal([]byte("not json"), &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonutil: unmarshal into")
}
