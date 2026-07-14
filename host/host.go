// Package host defines the seam shesha uses to adapt a server to a specific
// MCP host (generic stdio, Claude Code, Cursor, ...). Subpackages provide
// separable stub implementations so a consumer imports only the host it
// targets.
package host

import "github.com/dangernoodle-io/shesha/mcpx"

// Adapter binds shesha's composition root to a specific MCP host.
//
// host/cursor and host/generic are minimal stdio stubs: they only satisfy
// this interface and carry no host-specific behavior. The real Claude Code
// host integration — the hooks command tree today, with the statusline
// command tree as a forward-compatible extension point — lives in
// host/claudecode and is mounted via claudecode.NewProvider.
type Adapter interface {
	// Name identifies the host, e.g. "generic", "claude-code", "cursor".
	Name() string
	// Transport returns the transport the app should serve over for this
	// host.
	Transport() mcpx.Transport
}
