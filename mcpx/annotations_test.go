package mcpx_test

import (
	"testing"

	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/require"
)

// TestBoolPtr proves BoolPtr returns a pointer that dereferences to the
// given value, for populating ToolAnnotations' *bool hint fields without a
// go-sdk import.
func TestBoolPtr(t *testing.T) {
	p := mcpx.BoolPtr(true)
	require.NotNil(t, p)
	require.True(t, *p)

	p = mcpx.BoolPtr(false)
	require.NotNil(t, p)
	require.False(t, *p)
}

// TestToolAnnotationsAlias proves mcpx.ToolAnnotations is settable with the
// full hint field set a consumer needs, without importing go-sdk.
func TestToolAnnotationsAlias(t *testing.T) {
	ann := &mcpx.ToolAnnotations{
		ReadOnlyHint:    true,
		IdempotentHint:  true,
		DestructiveHint: mcpx.BoolPtr(false),
		OpenWorldHint:   mcpx.BoolPtr(true),
	}

	require.True(t, ann.ReadOnlyHint)
	require.True(t, ann.IdempotentHint)
	require.NotNil(t, ann.DestructiveHint)
	require.False(t, *ann.DestructiveHint)
	require.NotNil(t, ann.OpenWorldHint)
	require.True(t, *ann.OpenWorldHint)
}

// TestRiskAnnotations proves RiskAnnotations maps the three risk classes a
// caller uses (read, write, destructive) to the expected ReadOnlyHint and
// DestructiveHint values.
func TestRiskAnnotations(t *testing.T) {
	tests := map[string]struct {
		readOnly        bool
		destructive     bool
		wantReadOnly    bool
		wantDestructive bool
	}{
		"read": {
			readOnly:        true,
			destructive:     false,
			wantReadOnly:    true,
			wantDestructive: false,
		},
		"write": {
			readOnly:        false,
			destructive:     false,
			wantReadOnly:    false,
			wantDestructive: false,
		},
		"destructive": {
			readOnly:        false,
			destructive:     true,
			wantReadOnly:    false,
			wantDestructive: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ann := mcpx.RiskAnnotations(tc.readOnly, tc.destructive)

			require.NotNil(t, ann)
			require.Equal(t, tc.wantReadOnly, ann.ReadOnlyHint)
			require.NotNil(t, ann.DestructiveHint)
			require.Equal(t, tc.wantDestructive, *ann.DestructiveHint)
		})
	}
}
