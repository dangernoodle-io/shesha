// Package testkit is shesha's shipped in-memory MCP test harness, built on
// mcpx.InMemoryPair (no subprocess). It is used by shesha's own tests and is
// intended for reuse by downstream consumers.
package testkit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/require"
)

// ProgressEvent is one progress notification received for a tracked token.
type ProgressEvent struct {
	Token    any
	Message  string
	Progress float64
	Total    float64
}

// Harness wires an in-memory MCP client to a *shesha.App for testing.
type Harness struct {
	t       testing.TB
	session *mcpx.ClientSession

	mu       sync.Mutex
	progress map[any][]ProgressEvent

	// toolListChanged is signaled (non-blocking) each time the client
	// receives a notifications/tools/list_changed notification. Buffered so
	// the client's receive goroutine never blocks on a slow/absent waiter;
	// callers only need to observe that at least one arrived within a
	// timeout, not an exact count (the go-sdk debounces bursts ~10ms apart
	// into a single notification, so an exact-count assertion would be
	// flaky by design).
	toolListChanged chan struct{}
}

// New composes app over an in-memory transport pair, connects a client, and
// returns a ready Harness. The client is closed automatically via
// t.Cleanup.
func New(t testing.TB, app *shesha.App) *Harness {
	t.Helper()

	ctx := context.Background()
	serverT, clientT := mcpx.InMemoryPair()

	// Servers must connect before clients (go-sdk requirement).
	srvSess, err := app.Connect(ctx, serverT)
	require.NoError(t, err, "connect app to in-memory transport")
	t.Cleanup(func() {
		_ = srvSess.Close()
	})

	h := &Harness{
		t:               t,
		progress:        make(map[any][]ProgressEvent),
		toolListChanged: make(chan struct{}, 8),
	}

	client := mcpx.NewClient(mcpx.Implementation{Name: "testkit", Version: "0.0.0"}, &mcpx.ClientOptions{
		OnProgress:        h.recordProgress,
		OnToolListChanged: h.recordToolListChanged,
	})

	sess, err := client.Connect(ctx, clientT)
	require.NoError(t, err, "connect testkit client")

	h.session = sess
	t.Cleanup(func() {
		_ = sess.Close()
	})

	return h
}

func (h *Harness) recordProgress(_ context.Context, token any, message string, progress, total float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.progress[token] = append(h.progress[token], ProgressEvent{
		Token:    token,
		Message:  message,
		Progress: progress,
		Total:    total,
	})
}

// recordToolListChanged runs on the client's receive goroutine; it must
// never block, so the signal is a non-blocking send into a buffered
// channel.
func (h *Harness) recordToolListChanged(_ context.Context) {
	select {
	case h.toolListChanged <- struct{}{}:
	default:
	}
}

// WaitForToolListChanged blocks until the harness observes at least one
// notifications/tools/list_changed notification, or timeout elapses. It
// returns true on the former, false on the latter. Because the go-sdk
// debounces bursts of list changes into a single notification (~10ms),
// callers should not assert an exact count — one call observes "at least
// one arrived."
func (h *Harness) WaitForToolListChanged(timeout time.Duration) bool {
	select {
	case <-h.toolListChanged:
		return true
	case <-time.After(timeout):
		return false
	}
}

// CallTool calls the named tool with args, which must be JSON-marshalable.
func (h *Harness) CallTool(ctx context.Context, name string, args any) (*mcpx.CallToolResult, error) {
	return h.session.CallTool(ctx, name, args)
}

// CallToolWithProgressToken calls the named tool with args and attaches
// token as the request's progress token, so server-side progress
// notifications for this call can be retrieved via ProgressEvents(token).
func (h *Harness) CallToolWithProgressToken(ctx context.Context, name string, args any, token any) (*mcpx.CallToolResult, error) {
	return h.session.CallToolWithProgressToken(ctx, name, args, token)
}

// ListTools lists the tools the composed app advertises.
func (h *Harness) ListTools(ctx context.Context) (*mcpx.ListToolsResult, error) {
	return h.session.ListTools(ctx)
}

// ProgressEvents returns a snapshot of progress notifications recorded for
// token, in receipt order.
func (h *Harness) ProgressEvents(token any) []ProgressEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]ProgressEvent, len(h.progress[token]))
	copy(out, h.progress[token])
	return out
}
