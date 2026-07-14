package testkit_test

import (
	"context"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/dangernoodle-io/shesha/testkit"
	"github.com/stretchr/testify/require"
)

type pingOut struct {
	Reply string `json:"reply"`
}

type pingCap struct{}

func (pingCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "ping",
		Description: "replies pong",
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, pingOut, error) {
		return nil, pingOut{Reply: "pong"}, nil
	})
	return nil
}

func TestHarness(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "harness-test", Version: "0.0.1"}, generic.New(), pingCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "ping")

	res, err := h.CallTool(context.Background(), "ping", nil)
	require.NoError(t, err)
	require.False(t, res.IsError)

	out := testkit.DecodeToolResult[pingOut](t, res)
	require.Equal(t, "pong", out.Reply)
}

type workOut struct {
	Done bool `json:"done"`
}

type workCap struct{}

func (workCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "work",
		Description: "emits a progress notification keyed to the caller's token, then completes",
	}, shesha.ReadOnly, func(ctx context.Context, req *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, workOut, error) {
		if err := mcpx.NotifyProgress(ctx, req, "halfway", 50, 100); err != nil {
			return nil, workOut{}, err
		}
		return nil, workOut{Done: true}, nil
	})
	return nil
}

// TestHarnessProgress proves the progress-notification round trip is
// genuinely keyed: the client sets a progress token on the request, the
// server's handler emits a notification carrying that same token (via
// mcpx.NotifyProgress, reading it back off the request), and the harness's
// OnProgress hook files it under that token for ProgressEvents to surface.
func TestHarnessProgress(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "progress-test", Version: "0.0.1"}, generic.New(), workCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	const token = "work-token-1"

	res, err := h.CallToolWithProgressToken(context.Background(), "work", nil, token)
	require.NoError(t, err)
	require.False(t, res.IsError)

	testkit.EventuallyContains(t, 2*time.Second, 10*time.Millisecond, func() []string {
		var msgs []string
		for _, ev := range h.ProgressEvents(token) {
			msgs = append(msgs, ev.Message)
		}
		return msgs
	}, "halfway")

	events := h.ProgressEvents(token)
	require.Len(t, events, 1)
	require.Equal(t, token, events[0].Token)
	require.InDelta(t, 50, events[0].Progress, 0.001)
	require.InDelta(t, 100, events[0].Total, 0.001)

	// A different token must not see this call's events.
	require.Empty(t, h.ProgressEvents("unrelated-token"))
}

// TestHarnessToolListChanged_Timeout proves WaitForToolListChanged returns
// false when no notifications/tools/list_changed notification arrives
// within the timeout (nothing in this test triggers one).
func TestHarnessToolListChanged_Timeout(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "no-change-test", Version: "0.0.1"}, generic.New(), pingCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	require.False(t, h.WaitForToolListChanged(20*time.Millisecond))
}

type lockedToolCap struct{}

func (lockedToolCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "locked-tool",
		Description: "d",
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, pingOut, error) {
		return nil, pingOut{}, nil
	}, shesha.Group("locked"))
	return nil
}

// TestHarnessToolListChanged_Signaled proves WaitForToolListChanged (and its
// AssertToolListChanged wrapper) observe a real
// notifications/tools/list_changed notification fired by a runtime Unlock.
func TestHarnessToolListChanged_Signaled(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "list-changed-test", Version: "0.0.1"}, generic.New(), lockedToolCap{})
	require.NoError(t, err)

	require.NoError(t, app.Lock("locked"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h)

	require.NoError(t, app.Unlock("locked"))

	testkit.AssertToolListChanged(t, h, 5*time.Second)
	testkit.AssertToolSet(t, h, "locked-tool")
}
