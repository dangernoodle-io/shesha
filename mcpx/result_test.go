package mcpx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/require"
)

func TestTextResult(t *testing.T) {
	res := mcpx.TextResult("hello")
	require.False(t, res.IsError)
	require.Equal(t, "hello", mcpx.ResultText(res))
}

func TestErrorResult(t *testing.T) {
	res := mcpx.ErrorResult("boom")
	require.True(t, res.IsError)
	require.Equal(t, "boom", mcpx.ResultText(res))
}

// TestResultTextRoundTrip proves TextResult and ResultText round-trip.
func TestResultTextRoundTrip(t *testing.T) {
	require.Equal(t, "x", mcpx.ResultText(mcpx.TextResult("x")))
}

// TestHandlerErrorBecomesIsErrorResult probes go-sdk's error-as-tool-result
// semantics: a typed handler that returns a plain Go error (return nil,
// zero, err) is surfaced to the client as a normal (non-protocol-error)
// CallTool response with IsError=true and the error's text as content, not
// as a JSON-RPC protocol-level error. This informs the coming ouroboros
// migration: `return nil, zero, err` is sufficient for tool-level errors;
// mcpx.ErrorResult is only needed when a result must be built directly
// (e.g. the MC-8 recover hook, which has no error to return through the
// generic AddTool signature after a panic).
func TestHandlerErrorBecomesIsErrorResult(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "err-server", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "boom",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, errors.New("boom")
	})

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	client := mcpx.NewClient(mcpx.Implementation{Name: "err-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	// No protocol-level error: the call itself succeeds.
	res, err := clientSess.CallTool(ctx, "boom", nil)
	require.NoError(t, err, "handler error must not surface as a protocol-level error")
	require.True(t, res.IsError, "handler error must surface as an IsError tool result")
	require.Equal(t, "boom", mcpx.ResultText(res))
}
