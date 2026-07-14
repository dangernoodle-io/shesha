package mcpx

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// httpConfig holds the resolved options for HTTPHandler. The zero value is
// the default: stateful sessions, SSE responses.
type httpConfig struct {
	stateless    bool
	jsonResponse bool
}

// HTTPOption configures the streamable-HTTP handler returned by
// (*Server).HTTPHandler. It is mcpx-owned and maps internally to go-sdk's
// mcp.StreamableHTTPOptions, so callers never need to import go-sdk.
type HTTPOption func(*httpConfig)

// WithStateless controls whether the streamable-HTTP session is stateless.
// Default is false (stateful): the server validates sessions and can send
// server-initiated messages, including tools/list_changed notifications.
// Set true for a fire-and-forget deployment that does not need that.
func WithStateless(stateless bool) HTTPOption {
	return func(c *httpConfig) {
		c.stateless = stateless
	}
}

// WithJSONResponse controls whether streamable responses are returned as a
// single application/json body rather than Server-Sent Events. Default is
// false (SSE).
func WithJSONResponse(jsonResponse bool) HTTPOption {
	return func(c *httpConfig) {
		c.jsonResponse = jsonResponse
	}
}

// HTTPHandler returns a bare, path-agnostic http.Handler serving s over the
// MCP streamable-HTTP transport. shesha imposes no route: the consumer
// mounts it wherever it wants (any path, root, or under a subtree), or does
// not mount it at all. Calling this is entirely opt-in.
func (s *Server) HTTPHandler(opts ...HTTPOption) http.Handler {
	var cfg httpConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.srv
	}, &mcp.StreamableHTTPOptions{
		Stateless:    cfg.stateless,
		JSONResponse: cfg.jsonResponse,
	})
}
