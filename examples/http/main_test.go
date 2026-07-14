package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/stretchr/testify/require"
)

func newTestApp(t *testing.T) *shesha.App {
	t.Helper()
	app, err := shesha.New(shesha.Info{Name: "http-demo-test", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)
	return app
}

// TestNewMuxHealthz proves the co-mount example's non-MCP route works.
func TestNewMuxHealthz(t *testing.T) {
	mux := newMux(newTestApp(t))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
}

// TestNewMuxMCPMounted proves /mcp is actually wired to the MCP handler: a
// bare GET without the streamable-HTTP Accept header is rejected with 400
// (go-sdk's documented behavior for a malformed streamable request), which
// only happens if the handler is reached at all — a 404 would mean the
// route isn't mounted.
func TestNewMuxMCPMounted(t *testing.T) {
	mux := newMux(newTestApp(t))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusNotFound, rec.Code)
}
