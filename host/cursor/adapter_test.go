package cursor_test

import (
	"testing"

	"github.com/dangernoodle-io/shesha/host"
	"github.com/dangernoodle-io/shesha/host/cursor"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/assert"
)

// compile-time check that adapter satisfies host.Adapter.
var _ host.Adapter = cursor.New()

func TestNew(t *testing.T) {
	adapter := cursor.New()

	assert.Equal(t, "cursor", adapter.Name())
	assert.Equal(t, mcpx.Stdio(), adapter.Transport())
}
