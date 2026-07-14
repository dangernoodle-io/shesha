package generic_test

import (
	"testing"

	"github.com/dangernoodle-io/shesha/host"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/assert"
)

// compile-time check that adapter satisfies host.Adapter.
var _ host.Adapter = generic.New()

func TestNew(t *testing.T) {
	adapter := generic.New()

	assert.Equal(t, "generic", adapter.Name())
	assert.Equal(t, mcpx.Stdio(), adapter.Transport())
}
