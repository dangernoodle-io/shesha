package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/testkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRootCmd_Subcommands proves the assembled root carries the
// standard command set: server (MC-30/MC-31), version, and the claude
// namespace (hooks + statusline), mounted via cli.MountProviders.
func TestNewRootCmd_Subcommands(t *testing.T) {
	root := newRootCmd()

	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}

	assert.True(t, names["server"], "root must carry the server command")
	assert.True(t, names["version"], "root must carry the version command")
	assert.True(t, names["claude"], "root must carry the claude namespace")

	_, _, err := root.Find([]string{"claude", "hooks"})
	assert.NoError(t, err, "claude namespace must carry hooks")

	_, _, err = root.Find([]string{"claude", "statusline"})
	assert.NoError(t, err, "claude namespace must carry statusline")
}

// TestNewRootCmd_Version proves `version` prints the static version string
// this example advertises.
func TestNewRootCmd_Version(t *testing.T) {
	root := newRootCmd()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})

	require.NoError(t, root.Execute())
	assert.Equal(t, "example-mcp 0.0.0-example\n", out.String())
}

// TestNewRootCmd_ServerHelpListsHTTPFlags proves MC-31's HTTP transport
// selection reaches the example: `server --help` lists --http and
// --stateless.
func TestNewRootCmd_ServerHelpListsHTTPFlags(t *testing.T) {
	root := newRootCmd()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"server", "--help"})

	require.NoError(t, root.Execute())

	help := out.String()
	assert.True(t, strings.Contains(help, "--http"), "server --help must list --http")
	assert.True(t, strings.Contains(help, "--stateless"), "server --help must list --stateless")
}

// TestNewRootCmd_BareInvocationShowsHelp proves the dangernoodle convention
// this example follows: UseAsDefault is NOT called, so a bare invocation
// (no subcommand) shows help rather than starting the server.
func TestNewRootCmd_BareInvocationShowsHelp(t *testing.T) {
	root := newRootCmd()
	assert.Nil(t, root.RunE, "bare invocation must show help, not run the server")
}

// TestPingCap_Attach proves the example's "ping" tool actually replies with
// "pong", via testkit's in-memory harness (no subprocess, no real
// transport) — exercised independently of newRootCmd/ServerCmd, which never
// actually calls the tool.
func TestPingCap_Attach(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "ping-test", Version: "0.0.0"}, generic.New(), pingCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	res, err := h.CallTool(context.Background(), "ping", pingIn{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	out := testkit.DecodeToolResult[pingOut](t, res)
	assert.Equal(t, "pong", out.Message)
}

// TestClaudeHooksStop proves newHooksRegistry's Stop handler is actually
// wired end to end: `claude hooks stop` dispatches to it and its
// (zero-value) Response reaches stdout without error.
func TestClaudeHooksStop(t *testing.T) {
	root := newRootCmd()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetIn(strings.NewReader(`{"session_id":"abc123"}`))
	root.SetArgs([]string{"claude", "hooks", "stop"})

	require.NoError(t, root.Execute())
}

// TestClaudeStatuslinePlain proves newStatuslineProvider's closure is
// actually wired end to end: `claude statusline --plain` dispatches to it
// and, since it returns (nil, nil), renders nothing without error.
func TestClaudeStatuslinePlain(t *testing.T) {
	root := newRootCmd()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetIn(strings.NewReader(`{}`))
	root.SetArgs([]string{"claude", "statusline", "--plain"})

	require.NoError(t, root.Execute())
	assert.Empty(t, out.String())
}

// TestMust_NilErrIsNoop proves must does nothing when err is nil.
func TestMust_NilErrIsNoop(t *testing.T) {
	assert.NotPanics(t, func() { must(nil, "ctx") })
}

// TestMust_PanicsOnError proves must panics, wrapping err with msg, when
// err is non-nil.
func TestMust_PanicsOnError(t *testing.T) {
	boom := errors.New("boom")

	assert.PanicsWithError(t, "ctx: boom", func() { must(boom, "ctx") })
}

// TestRun_SuccessReturnsZero proves run returns exit code 0 and writes
// nothing to stderr on a successful command (version).
func TestRun_SuccessReturnsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)

	assert.Equal(t, 0, code)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "example-mcp 0.0.0-example\n", stdout.String())
}

// TestRun_ErrorReturnsOne proves run returns exit code 1 and writes the
// error to stderr when the command fails (an unknown subcommand).
func TestRun_ErrorReturnsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"no-such-command"}, &stdout, &stderr)

	assert.Equal(t, 1, code)
	assert.NotEmpty(t, stderr.String())
}
