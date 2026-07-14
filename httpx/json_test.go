package httpx_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/dangernoodle-io/shesha/httpx"
	"github.com/dangernoodle-io/shesha/jsonutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type widget struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// errWriter wraps httptest.ResponseRecorder to force a Write error after
// headers have already gone out, exercising WriteJSON's final Write-error
// return path (distinct from the marshal-error, nothing-written path).
type errWriter struct {
	*httptest.ResponseRecorder
}

func (w *errWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

func TestWriteJSON_SetsStatusAndContentType(t *testing.T) {
	rec := httptest.NewRecorder()

	err := httpx.WriteJSON(rec, 201, widget{Name: "gizmo", Count: 3})
	require.NoError(t, err)

	assert.Equal(t, 201, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var out widget
	require.NoError(t, jsonutil.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, widget{Name: "gizmo", Count: 3}, out)
}

// TestWriteJSON_MarshalError proves the buffer-first contract: when
// encoding fails, WriteJSON writes nothing at all (no status flip, no
// headers, no body) rather than a partial or misleadingly-200 response.
func TestWriteJSON_MarshalError(t *testing.T) {
	rec := httptest.NewRecorder()

	err := httpx.WriteJSON(rec, 200, make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "httpx: write json")

	// httptest.ResponseRecorder.Code defaults to 200 whether or not
	// WriteHeader was ever called, so it can't distinguish "never written"
	// from "written 200" on its own; the empty body plus unset
	// Content-Type header are the externally observable proof that
	// WriteJSON wrote nothing at all.
	assert.Equal(t, 0, rec.Body.Len(), "no body must be written on marshal failure")
	assert.Empty(t, rec.Header().Get("Content-Type"), "no header must be written on marshal failure")
}

// TestWriteJSON_WriteError proves the final w.Write error is returned
// (wrapped) once encoding has already succeeded and headers/status have
// already gone out — the distinct failure mode from the marshal-error case.
func TestWriteJSON_WriteError(t *testing.T) {
	w := &errWriter{ResponseRecorder: httptest.NewRecorder()}

	err := httpx.WriteJSON(w, 200, widget{Name: "gizmo", Count: 3})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "httpx: write json")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "headers are already sent by the time Write fails")
}
