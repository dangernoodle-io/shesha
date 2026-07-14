package httpx_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// freeAddr binds an ephemeral port, immediately releases it, and returns
// its address for a test to hand to Serve. There is an inherent (tiny)
// TOCTOU race against another process grabbing the same port before Serve
// binds it, same as any test using this pattern.
func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	return addr
}

func TestNewMux_MountsHandlerAtPath(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	mux := httpx.NewMux("/mcp", stub)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTeapot, rec.Code)
}

func TestNewMux_ConsumerRouteReached(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	mux := httpx.NewMux("/mcp", stub)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestNewMux_UnregisteredPath404s(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	mux := httpx.NewMux("/mcp", stub)

	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// testTimeout bounds how long any Serve-lifecycle subtest waits for Serve
// to return, so a regression that breaks graceful shutdown fails the test
// instead of hanging the suite.
const testTimeout = 5 * time.Second

func TestServe_CtxCancelShutsDownCleanly(t *testing.T) {
	handler := http.NewServeMux()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- httpx.Serve(ctx, "127.0.0.1:0", handler)
	}()

	// Give ListenAndServe a moment to actually start listening before
	// cancelling, so this exercises the running-server shutdown path
	// rather than racing a bind that hasn't happened yet.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-resultCh:
		assert.NoError(t, err)
	case <-time.After(testTimeout):
		t.Fatal("Serve did not return within timeout after ctx cancel")
	}
}

func TestServe_WithShutdownTimeout_CleanShutdown(t *testing.T) {
	handler := http.NewServeMux()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- httpx.Serve(ctx, "127.0.0.1:0", handler, httpx.WithShutdownTimeout(500*time.Millisecond))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-resultCh:
		assert.NoError(t, err)
	case <-time.After(testTimeout):
		t.Fatal("Serve did not return within timeout after ctx cancel")
	}
}

// TestServe_ShutdownTimeoutExceeded proves Serve returns the Shutdown
// error when graceful shutdown can't finish within the configured
// timeout: a handler holds a request open past a very short
// WithShutdownTimeout, forcing srv.Shutdown to give up with
// context.DeadlineExceeded.
func TestServe_ShutdownTimeoutExceeded(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.NewServeMux()
	handler.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	defer close(release)

	addr := freeAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- httpx.Serve(ctx, addr, handler, httpx.WithShutdownTimeout(50*time.Millisecond))
	}()

	// Poll until the server accepts connections, then fire the slow
	// request in the background and wait for the handler to confirm it's
	// actually in flight before triggering shutdown.
	require.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, testTimeout, 5*time.Millisecond)

	go func() {
		client := &http.Client{}
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/slow", nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	select {
	case <-started:
	case <-time.After(testTimeout):
		t.Fatal("handler never observed an in-flight request")
	}

	cancel()

	select {
	case err := <-resultCh:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(testTimeout):
		t.Fatal("Serve did not return within timeout after ctx cancel")
	}
}

func TestServe_BindError(t *testing.T) {
	handler := http.NewServeMux()

	ctx := context.Background()

	// An address with an invalid/unparseable port fails at listen time,
	// well before ctx would ever be cancelled - covers the "server exited
	// on its own" branch of Serve's select.
	err := httpx.Serve(ctx, "127.0.0.1:not-a-port", handler)
	require.Error(t, err)
}
