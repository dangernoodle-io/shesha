// Package mcpx is the sole seam over github.com/modelcontextprotocol/go-sdk:
// no other shesha package imports go-sdk directly. It stays deliberately
// thin in phase 0, type-aliasing a handful of go-sdk types.
//
// MC-8: harden the aliases below into owned types once the surface stabilizes.
package mcpx

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Implementation describes an MCP client or server implementation.
type Implementation = mcp.Implementation

// Tool describes a tool exposed by a server.
type Tool = mcp.Tool

// CallToolRequest is the request passed to a tool handler.
type CallToolRequest = mcp.CallToolRequest

// CallToolResult is a tool handler's protocol-level result.
type CallToolResult = mcp.CallToolResult

// ListToolsResult is the response to a tools/list request.
type ListToolsResult = mcp.ListToolsResult

// InitializeResult is the server's response to a client's initialize
// request, including the Instructions string a NewServer caller set.
type InitializeResult = mcp.InitializeResult

// ToolAnnotations are hints about a tool's behavior (destructive,
// idempotent, open-world, read-only), surfaced to clients via Tool.Annotations.
// Type-aliased here so a caller can set them without importing go-sdk.
type ToolAnnotations = mcp.ToolAnnotations

// BoolPtr returns a pointer to b, for populating the *bool hint fields on
// ToolAnnotations (DestructiveHint, OpenWorldHint) without importing go-sdk.
func BoolPtr(b bool) *bool {
	return &b
}

// Handler mirrors go-sdk's typed tool handler shape.
type Handler[In, Out any] = mcp.ToolHandlerFor[In, Out]

// Server wraps an mcp.Server, keeping the go-sdk type unexported.
type Server struct {
	srv *mcp.Server
}

// NewServer constructs a Server advertising the given implementation. If
// instructions is non-empty, it is advertised to clients in the
// InitializeResult (see InitializeResult.Instructions) as guidance on how to
// use the server and its tools; an empty string preserves the prior
// behavior of passing nil ServerOptions. If keepAlive is > 0, go-sdk pings
// each session's peer on that interval and closes the session on
// ping-failure; 0 disables keepalive (the default).
func NewServer(impl Implementation, instructions string, keepAlive time.Duration) *Server {
	var opts *mcp.ServerOptions
	if instructions != "" || keepAlive > 0 {
		opts = &mcp.ServerOptions{Instructions: instructions, KeepAlive: keepAlive}
	}
	return &Server{srv: mcp.NewServer(&impl, opts)}
}

// AddTool registers a typed tool handler on s. It is a top-level function,
// mirroring go-sdk's mcp.AddTool, because Go methods cannot be generic.
func AddTool[In, Out any](s *Server, t *Tool, h Handler[In, Out]) {
	mcp.AddTool(s.srv, t, h)
}

// RemoveTools removes the named tools from s. It is not an error to remove
// a nonexistent tool. Connected clients that set OnToolListChanged are
// notified automatically (go-sdk internal).
func (s *Server) RemoveTools(names ...string) {
	s.srv.RemoveTools(names...)
}

// Run serves s over t until the client disconnects or ctx is cancelled.
func (s *Server) Run(ctx context.Context, t Transport) error {
	return s.srv.Run(ctx, t.transport())
}

// Session is a live server-side connection to a single client.
type Session struct {
	sess *mcp.ServerSession
}

// Connect connects s over t and returns a live session without blocking.
func (s *Server) Connect(ctx context.Context, t Transport) (*Session, error) {
	sess, err := s.srv.Connect(ctx, t.transport(), nil)
	if err != nil {
		return nil, err
	}
	return &Session{sess: sess}, nil
}

// Close closes the underlying session.
func (s *Session) Close() error {
	return s.sess.Close()
}

// Wait blocks until the client terminates the connection.
func (s *Session) Wait() error {
	return s.sess.Wait()
}

// NotifyProgress sends a progress notification for the call req represents,
// keyed to the progress token the client set on the request (if any). It is
// a no-op-on-the-wire-but-still-an-error call if the client sent no
// progress token; callers that want to skip unconditionally can check
// ProgressToken(req) first.
func NotifyProgress(ctx context.Context, req *CallToolRequest, message string, progress, total float64) error {
	return req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: req.Params.GetProgressToken(),
		Message:       message,
		Progress:      progress,
		Total:         total,
	})
}

// ProgressToken returns the progress token the client attached to req, or
// nil if it didn't request progress tracking.
func ProgressToken(req *CallToolRequest) any {
	return req.Params.GetProgressToken()
}

// ResultText concatenates the text of every mcp.TextContent block in a
// CallToolResult, ignoring non-text content. Kept in mcpx so callers never
// need to type-assert against go-sdk's Content interface.
func ResultText(res *CallToolResult) string {
	if res == nil {
		return ""
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
