package testkit

import (
	"context"
	"testing"
	"time"

	"github.com/dangernoodle-io/shesha/jsonutil"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResultText concatenates the text content of a CallToolResult.
var ResultText = mcpx.ResultText

// DecodeToolResult json-unmarshals a tool result's text content into T via
// jsonutil.Unmarshal, failing the test on decode error.
func DecodeToolResult[T any](t testing.TB, res *mcpx.CallToolResult) T {
	t.Helper()
	var out T
	err := jsonutil.Unmarshal([]byte(mcpx.ResultText(res)), &out)
	require.NoError(t, err, "decode tool result")
	return out
}

// EventuallyContains polls fn until it returns a slice containing want, or
// fails the test after timeout.
func EventuallyContains(t testing.TB, timeout, interval time.Duration, fn func() []string, want string) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, got := range fn() {
			if got == want {
				return true
			}
		}
		return false
	}, timeout, interval, "expected %q to eventually appear", want)
}

// AssertToolListChanged fails the test unless the harness observes at least
// one notifications/tools/list_changed notification within timeout. The
// go-sdk debounces rapid successive list changes into a single
// notification, so this asserts "at least one arrived," never an exact
// count.
func AssertToolListChanged(t testing.TB, h *Harness, timeout time.Duration) {
	t.Helper()
	require.True(t, h.WaitForToolListChanged(timeout),
		"expected a tools/list_changed notification within %s", timeout)
}

// AssertToolSet asserts that the app's advertised tools/list is exactly
// want, guarding against silent drift.
func AssertToolSet(t testing.TB, h *Harness, want ...string) {
	t.Helper()
	res, err := h.ListTools(context.Background())
	require.NoError(t, err, "list tools")

	got := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	assert.ElementsMatch(t, want, got, "tool set drift")
}
