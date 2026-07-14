// Package claudecode provides a Claude Code host.Adapter stub.
package claudecode

import (
	"github.com/dangernoodle-io/shesha/host"
	"github.com/dangernoodle-io/shesha/mcpx"
)

type adapter struct{}

// New returns a Claude Code host.Adapter. Currently a stdio stub; MC-7/MC-10
// fill in session-identity and statusline-presence hooks.
func New() host.Adapter {
	return adapter{}
}

func (adapter) Name() string {
	return "claude-code"
}

func (adapter) Transport() mcpx.Transport {
	return mcpx.Stdio()
}
