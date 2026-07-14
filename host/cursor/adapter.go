// Package cursor provides a Cursor host.Adapter stub.
package cursor

import (
	"github.com/dangernoodle-io/shesha/host"
	"github.com/dangernoodle-io/shesha/mcpx"
)

type adapter struct{}

// New returns a Cursor host.Adapter. Currently a stdio stub; MC-7/MC-10 fill
// in session-identity and statusline-presence hooks.
func New() host.Adapter {
	return adapter{}
}

func (adapter) Name() string {
	return "cursor"
}

func (adapter) Transport() mcpx.Transport {
	return mcpx.Stdio()
}
