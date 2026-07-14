// Package generic provides a host-agnostic stdio Adapter.
package generic

import (
	"github.com/dangernoodle-io/shesha/host"
	"github.com/dangernoodle-io/shesha/mcpx"
)

type adapter struct{}

// New returns a generic stdio host.Adapter.
func New() host.Adapter {
	return adapter{}
}

func (adapter) Name() string {
	return "generic"
}

func (adapter) Transport() mcpx.Transport {
	return mcpx.Stdio()
}
