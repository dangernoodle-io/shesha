package claudecode_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/dangernoodle-io/shesha/cli"
	"github.com/dangernoodle-io/shesha/host/claudecode"
	"github.com/dangernoodle-io/shesha/host/claudecode/hooks"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewProvider_MountsReturnsClaudeTree proves Mounts() returns a single
// root-mounted Mount holding the `claude` namespace command, which in turn
// holds `hooks`.
func TestNewProvider_MountsReturnsClaudeTree(t *testing.T) {
	reg := hooks.NewRegistry().Stop(func(_ context.Context, _ io.Reader, _ hooks.StopPayload) hooks.Response {
		return hooks.Response{}
	})

	p := claudecode.NewProvider(reg)
	mounts := p.Mounts()

	require.Len(t, mounts, 1)
	assert.Empty(t, mounts[0].Under, "the claude tree mounts at root")
	assert.Equal(t, "claude", mounts[0].Cmd.Use)

	names := map[string]bool{}
	for _, c := range mounts[0].Cmd.Commands() {
		names[c.Use] = true
	}
	assert.True(t, names["hooks"])
}

// TestNewProvider_ExtraSubtreesAppendUnderClaude proves the extension
// point: extra cobra.Command trees passed to NewProvider mount alongside
// hooks under the same `claude` namespace (the seam a later statusline PR
// uses).
func TestNewProvider_ExtraSubtreesAppendUnderClaude(t *testing.T) {
	extra := &cobra.Command{Use: "statusline"}

	p := claudecode.NewProvider(hooks.NewRegistry(), extra)
	mounts := p.Mounts()

	require.Len(t, mounts, 1)

	names := map[string]bool{}
	for _, c := range mounts[0].Cmd.Commands() {
		names[c.Use] = true
	}
	assert.True(t, names["hooks"])
	assert.True(t, names["statusline"])
}

// TestMountProviders_EndToEndDispatch proves the full mount path: root ->
// cli.MountProviders(root, claudecode.NewProvider(reg)) -> `root claude
// hooks stop` dispatches to the registered handler and its Response
// reaches stdout, fed via real stdin bytes. It also asserts every mounted
// path resolves via root.Find.
func TestMountProviders_EndToEndDispatch(t *testing.T) {
	var gotSessionID string
	reg := hooks.NewRegistry().Stop(func(_ context.Context, _ io.Reader, p hooks.StopPayload) hooks.Response {
		gotSessionID = p.SessionID
		return hooks.Response{AdditionalContext: "seen"}
	})

	extra := &cobra.Command{Use: "statusline", RunE: func(*cobra.Command, []string) error { return nil }}

	root := &cobra.Command{Use: "root"}
	err := cli.MountProviders(root, claudecode.NewProvider(reg, extra))
	require.NoError(t, err)

	_, _, findErr := root.Find([]string{"claude"})
	require.NoError(t, findErr)

	_, _, findErr = root.Find([]string{"claude", "hooks"})
	require.NoError(t, findErr)

	_, _, findErr = root.Find([]string{"claude", "statusline"})
	require.NoError(t, findErr)

	var out bytes.Buffer
	root.SetArgs([]string{"claude", "hooks", "stop"})
	root.SetIn(strings.NewReader(`{"session_id":"abc123"}`))
	root.SetOut(&out)

	execErr := root.Execute()

	require.NoError(t, execErr)
	assert.Equal(t, "abc123", gotSessionID)
	assert.Contains(t, out.String(), `"hookEventName":"Stop"`)
	assert.Contains(t, out.String(), `"additionalContext":"seen"`)
}
