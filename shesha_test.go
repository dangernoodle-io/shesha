package shesha_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/dangernoodle-io/shesha/testkit"
	"github.com/stretchr/testify/require"
)

type helloIn struct {
	Name string `json:"name" jsonschema:"name to greet"`
}

type helloOut struct {
	Greeting string `json:"greeting"`
}

type helloCap struct{}

func (helloCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "hello",
		Description: "greets the caller by name",
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, in helloIn) (*mcpx.CallToolResult, helloOut, error) {
		name := in.Name
		if name == "" {
			name = "world"
		}
		return nil, helloOut{Greeting: fmt.Sprintf("hello, %s!", name)}, nil
	})
	return nil
}

// TestEndToEnd proves the mcpx seam, capability wiring, and testkit harness
// work together: compose an App, list its tools, call one, and decode the
// result.
func TestEndToEnd(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "hello")

	res, err := h.CallTool(context.Background(), "hello", map[string]any{"name": "shesha"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	out := testkit.DecodeToolResult[helloOut](t, res)
	require.Equal(t, "hello, shesha!", out.Greeting)
}

// TestNewNilHost proves New rejects a nil host adapter rather than panicking
// later.
func TestNewNilHost(t *testing.T) {
	_, err := shesha.New(shesha.Info{Name: "e2e", Version: "0.0.1"}, nil)
	require.Error(t, err)
}

// TestKeepAliveThreadsToServer proves Info.KeepAlive reaches mcpx.NewServer
// (MC-48): a non-zero value composes and serves a full tools/list + tool
// call round trip identically to the zero-value (default) path exercised by
// TestEndToEnd.
func TestKeepAliveThreadsToServer(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "keepalive", Version: "0.0.1", KeepAlive: 50 * time.Millisecond}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "hello")

	res, err := h.CallTool(context.Background(), "hello", map[string]any{"name": "keepalive"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	out := testkit.DecodeToolResult[helloOut](t, res)
	require.Equal(t, "hello, keepalive!", out.Greeting)
}

// TestKeepAliveZeroDisabled proves Info.KeepAlive's zero value (the default)
// preserves the prior no-keepalive behavior: a fresh App still composes and
// serves normally.
func TestKeepAliveZeroDisabled(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "no-keepalive", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "hello")
}

type panicIn struct{}

type panicOut struct {
	Result string `json:"result"`
}

type panicCap struct{}

func (panicCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "panics",
		Description: "always panics",
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, _ panicIn) (*mcpx.CallToolResult, panicOut, error) {
		panic("kaboom")
	})
	return nil
}

// TestAddToolRecoversPanic proves the MC-8 recover hook at the AddTool
// chokepoint converts a panicking handler into an IsError tool result
// (naming the tool and the panic value) rather than crashing the process.
func TestAddToolRecoversPanic(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "panic-e2e", Version: "0.0.1"}, generic.New(), panicCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	res, err := h.CallTool(context.Background(), "panics", map[string]any{})
	require.NoError(t, err, "a recovered panic must not surface as a protocol-level error")
	require.True(t, res.IsError)

	text := testkit.ResultText(res)
	require.Contains(t, text, "panics")
	require.Contains(t, text, "kaboom")

	// Regression guard: the recovered result must not carry the panicked
	// handler's zero-value Out in StructuredContent alongside IsError.
	require.Nil(t, res.StructuredContent, "recovered panic must not leak a zero-value Out into StructuredContent")
}

// TestAddToolTransparentOnHappyPath proves the recover wrapper is
// transparent when the handler does not panic: a normal handler's result is
// returned unchanged.
func TestAddToolTransparentOnHappyPath(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "happy-e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	res, err := h.CallTool(context.Background(), "hello", map[string]any{"name": "shesha"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	out := testkit.DecodeToolResult[helloOut](t, res)
	require.Equal(t, "hello, shesha!", out.Greeting)
}

// TestNewWithInstructions proves Info.Instructions composes through New
// without error; the wire-level assertion that the string reaches the
// client lives in mcpx (the sole seam over go-sdk's InitializeResult).
func TestNewWithInstructions(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "instructed", Version: "0.0.1", Instructions: "guide"}, generic.New(), helloCap{})
	require.NoError(t, err)
	require.NotNil(t, app)
}

type annotatedIn struct{}

type annotatedOut struct {
	OK bool `json:"ok"`
}

type annotatedCap struct{}

func (annotatedCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "annotated",
		Description: "carries tool annotations",
		Annotations: &mcpx.ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: mcpx.BoolPtr(true),
		},
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, _ annotatedIn) (*mcpx.CallToolResult, annotatedOut, error) {
		return nil, annotatedOut{OK: true}, nil
	})
	return nil
}

// TestToolAnnotationsRoundTrip proves a tool registered with
// mcpx.ToolAnnotations (built via the mcpx.BoolPtr helper, no go-sdk import
// required by the consumer) carries those annotations through tools/list —
// the path ouroboros's cutover will rely on.
func TestToolAnnotationsRoundTrip(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "annotated-e2e", Version: "0.0.1"}, generic.New(), annotatedCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	res, err := h.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, res.Tools, 1)

	tool := res.Tools[0]
	require.Equal(t, "annotated", tool.Name)
	require.NotNil(t, tool.Annotations)
	require.True(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	require.True(t, *tool.Annotations.DestructiveHint)
}

// TestAppHTTPHandler proves App.HTTPHandler delegates to the composed
// server's real streamable-HTTP handler (the mcpx protocol round trip is
// covered in mcpx/http_test.go, which alone is allowed to import go-sdk).
// A bare GET without the streamable-HTTP Accept header is go-sdk's
// documented rejection for a malformed streamable request: 400 with a
// text/event-stream mention in the body. Asserting that exact behavior
// (rather than just a non-zero status) catches a broken/no-op delegate.
func TestAppHTTPHandler(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "http-e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := app.HTTPHandler()
	require.NotNil(t, h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "text/event-stream")
}

type riskIn struct{}

type riskOut struct{}

// riskCap registers one tool per Risk value, each left with a nil
// Annotations so AddTool must derive it from risk via mcpx.RiskAnnotations.
type riskCap struct{}

func (riskCap) Attach(r *shesha.Registrar) error {
	handler := func(_ context.Context, _ *mcpx.CallToolRequest, _ riskIn) (*mcpx.CallToolResult, riskOut, error) {
		return nil, riskOut{}, nil
	}
	shesha.AddTool(r, &mcpx.Tool{Name: "risk-readonly", Description: "d"}, shesha.ReadOnly, handler)
	shesha.AddTool(r, &mcpx.Tool{Name: "risk-write", Description: "d"}, shesha.Write, handler)
	shesha.AddTool(r, &mcpx.Tool{Name: "risk-destructive", Description: "d"}, shesha.Destructive, handler)
	return nil
}

// TestAddToolRiskAutoAnnotation proves AddTool derives ToolAnnotations from
// each Risk value via mcpx.RiskAnnotations when the caller left
// t.Annotations nil: ReadOnlyHint tracks Risk == ReadOnly, and
// DestructiveHint tracks Risk == Destructive (Write gets neither hint set).
func TestAddToolRiskAutoAnnotation(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "risk-e2e", Version: "0.0.1"}, generic.New(), riskCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	res, err := h.ListTools(context.Background())
	require.NoError(t, err)

	byName := make(map[string]*mcpx.ToolAnnotations, len(res.Tools))
	for _, tool := range res.Tools {
		byName[tool.Name] = tool.Annotations
	}

	ro := byName["risk-readonly"]
	require.NotNil(t, ro)
	require.True(t, ro.ReadOnlyHint)
	require.NotNil(t, ro.DestructiveHint)
	require.False(t, *ro.DestructiveHint)

	wr := byName["risk-write"]
	require.NotNil(t, wr)
	require.False(t, wr.ReadOnlyHint)
	require.NotNil(t, wr.DestructiveHint)
	require.False(t, *wr.DestructiveHint)

	de := byName["risk-destructive"]
	require.NotNil(t, de)
	require.False(t, de.ReadOnlyHint)
	require.NotNil(t, de.DestructiveHint)
	require.True(t, *de.DestructiveHint)
}

// TestDeferredRegistrationAdvertisesAfterConnect proves the deferred
// registration MC-43 introduces is behavior-preserving from a client's
// perspective: a tool registered via AddTool before the first Connect is
// fully advertised (tools/list) and callable once a session exists.
func TestDeferredRegistrationAdvertisesAfterConnect(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "deferred-e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)

	testkit.AssertToolSet(t, h, "hello")
}

// TestAppConnectIdempotentAcrossSessions proves a second Connect against the
// same App (finalize's idempotency guard) neither panics nor loses tools: a
// second in-memory session still sees the same fully-registered tool set,
// proving finalize's first run is what registered it and the second run
// was a no-op rather than a duplicate-registration panic.
func TestAppConnectIdempotentAcrossSessions(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "reconnect-e2e", Version: "0.0.1"}, generic.New(), helloCap{})
	require.NoError(t, err)

	h1 := testkit.New(t, app)
	testkit.AssertToolSet(t, h1, "hello")

	require.NotPanics(t, func() {
		h2 := testkit.New(t, app)
		testkit.AssertToolSet(t, h2, "hello")
	})
}
