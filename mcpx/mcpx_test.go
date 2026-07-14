package mcpx_test

import (
	"context"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/require"
)

type echoIn struct {
	Text string `json:"text"`
}

type echoOut struct {
	Text string `json:"text"`
}

func TestServerClientRoundTrip(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "test-server", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{
		Name:        "echo",
		Description: "echoes text back",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, in echoIn) (*mcpx.CallToolResult, echoOut, error) {
		return nil, echoOut(in), nil
	})

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	client := mcpx.NewClient(mcpx.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	tools, err := clientSess.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "echo", tools.Tools[0].Name)

	res, err := clientSess.CallTool(ctx, "echo", map[string]any{"text": "hi"})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.JSONEq(t, `{"text":"hi"}`, mcpx.ResultText(res))
}

// TestNotifyProgress proves the progress round trip is genuinely keyed to
// the token the client set on its request: the handler reads the token back
// off req via ProgressToken/NotifyProgress, and the client's OnProgress
// callback receives that same token.
func TestNotifyProgress(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "progress-server", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "work",
	}, func(ctx context.Context, req *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		require.NotNil(t, mcpx.ProgressToken(req))
		err := mcpx.NotifyProgress(ctx, req, "halfway", 50, 100)
		return nil, struct{}{}, err
	})

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	type event struct {
		token    any
		message  string
		progress float64
		total    float64
	}
	events := make(chan event, 1)

	client := mcpx.NewClient(mcpx.Implementation{Name: "progress-client", Version: "0.0.1"}, &mcpx.ClientOptions{
		OnProgress: func(_ context.Context, token any, message string, progress, total float64) {
			events <- event{token: token, message: message, progress: progress, total: total}
		},
	})
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	res, err := clientSess.CallToolWithProgressToken(ctx, "work", nil, "tok-1")
	require.NoError(t, err)
	require.False(t, res.IsError)

	select {
	case got := <-events:
		require.Equal(t, "tok-1", got.token)
		require.Equal(t, "halfway", got.message)
		require.InDelta(t, 50, got.progress, 0.001)
		require.InDelta(t, 100, got.total, 0.001)
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive progress notification")
	}
}

// TestServerInstructions proves the instructions string passed to NewServer
// reaches the client via the InitializeResult, and that a non-empty value
// composes without error.
func TestServerInstructions(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "instructions-server", Version: "0.0.1"}, "call echo before anything else", 0)

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	client := mcpx.NewClient(mcpx.Implementation{Name: "instructions-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	res := clientSess.InitializeResult()
	require.NotNil(t, res)
	require.Equal(t, "call echo before anything else", res.Instructions)
}

// TestServerKeepAlive proves a non-zero keepAlive reaches the underlying
// mcp.ServerOptions.KeepAlive without breaking composition or a normal
// client round trip (MC-48). go-sdk's actual ping/close-on-failure behavior
// is go-sdk's own tested concern; this only exercises the plumbing.
func TestServerKeepAlive(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "keepalive-server", Version: "0.0.1"}, "", 50*time.Millisecond)

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	client := mcpx.NewClient(mcpx.Implementation{Name: "keepalive-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	tools, err := clientSess.ListTools(ctx)
	require.NoError(t, err)
	require.Empty(t, tools.Tools)
}

// TestServerNoInstructions proves NewServer(impl, "", 0) is behaviorally
// identical to the prior single-arg NewServer: no instructions are
// advertised (empty string, the nil-ServerOptions path), keepalive is
// disabled, and the server still composes and serves normally.
func TestServerNoInstructions(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "no-instructions-server", Version: "0.0.1"}, "", 0)

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	client := mcpx.NewClient(mcpx.Implementation{Name: "no-instructions-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	res := clientSess.InitializeResult()
	require.NotNil(t, res)
	require.Empty(t, res.Instructions)
}

func TestStdioTransport(t *testing.T) {
	// Stdio just needs to construct a non-nil Transport; exercising it over
	// real stdin/stdout belongs to examples/minimal, not unit tests.
	require.NotNil(t, mcpx.Stdio())
}

// TestRunUnwindsOnClientClose exercises the real blocking entrypoint,
// (*Server).Run: it must block while the client is connected, and return
// cleanly once the client disconnects.
func TestRunUnwindsOnClientClose(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "run-server", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "noop",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})

	serverT, clientT := mcpx.InMemoryPair()

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- srv.Run(context.Background(), serverT)
	}()

	ctx := context.Background()
	client := mcpx.NewClient(mcpx.Implementation{Name: "run-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)

	_, err = clientSess.CallTool(ctx, "noop", nil)
	require.NoError(t, err)

	require.NoError(t, clientSess.Close())

	select {
	case err := <-runErrCh:
		require.NoError(t, err, "Run should unwind cleanly once the client disconnects")
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not unwind after the client closed the connection")
	}
}

// TestRemoveTools proves RemoveTools drops a previously-added tool from the
// server's advertised tool list.
func TestRemoveTools(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "remove-tools-server", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "gone",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})
	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "stays",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})

	srv.RemoveTools("gone")

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	client := mcpx.NewClient(mcpx.Implementation{Name: "remove-tools-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	tools, err := clientSess.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "stays", tools.Tools[0].Name)
}

// TestOnToolListChanged proves a client with OnToolListChanged set observes
// the notification when the server's tool list changes mid-session. go-sdk
// debounces list-changed notifications by 10ms, so this asserts on the
// callback firing and the resulting tool list, not on notification count.
func TestOnToolListChanged(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "list-changed-server", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "initial",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	changed := make(chan struct{}, 4)

	client := mcpx.NewClient(mcpx.Implementation{Name: "list-changed-client", Version: "0.0.1"}, &mcpx.ClientOptions{
		OnToolListChanged: func(_ context.Context) {
			changed <- struct{}{}
		},
	})
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSess.Close() })

	mcpx.AddTool(srv, &mcpx.Tool{
		Name: "added",
	}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})

	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive tool list changed notification after AddTool")
	}

	tools, err := clientSess.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 2)

	srv.RemoveTools("added")

	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive tool list changed notification after RemoveTools")
	}

	tools, err = clientSess.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "initial", tools.Tools[0].Name)
}

// TestSessionWait exercises (*Session).Wait, the non-blocking-connect
// counterpart to Run: it must block while the client is connected, and
// return once the client closes.
func TestSessionWait(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "wait-server", Version: "0.0.1"}, "", 0)

	serverT, clientT := mcpx.InMemoryPair()

	ctx := context.Background()
	sess, err := srv.Connect(ctx, serverT)
	require.NoError(t, err)

	waitErrCh := make(chan error, 1)
	go func() {
		waitErrCh <- sess.Wait()
	}()

	client := mcpx.NewClient(mcpx.Implementation{Name: "wait-client", Version: "0.0.1"}, nil)
	clientSess, err := client.Connect(ctx, clientT)
	require.NoError(t, err)
	require.NoError(t, clientSess.Close())

	select {
	case err := <-waitErrCh:
		require.NoError(t, err, "Wait should unwind cleanly once the client disconnects")
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not unwind after the client closed the connection")
	}
}
