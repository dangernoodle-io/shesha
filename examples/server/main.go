// Command example-mcp demonstrates assembling shesha's standard command
// set: a stdio/HTTP-selectable server command (MC-31) plus the Claude Code
// host's `claude hooks`/`claude statusline` subtrees, mounted onto one
// cobra root via cli.MountProviders (MC-30's unified mount). It closes the
// "no CLI example / no Claude Code example" onboarding gap — MC-32's README
// quickstart lifts from this.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/cli"
	"github.com/dangernoodle-io/shesha/host/claudecode"
	"github.com/dangernoodle-io/shesha/host/claudecode/hooks"
	"github.com/dangernoodle-io/shesha/host/claudecode/statusline"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/spf13/cobra"
)

type pingIn struct{}

type pingOut struct {
	Message string `json:"message"`
}

type pingCap struct{}

func (pingCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{
		Name:        "ping",
		Description: "replies with pong",
	}, shesha.ReadOnly, func(_ context.Context, _ *mcpx.CallToolRequest, _ pingIn) (*mcpx.CallToolResult, pingOut, error) {
		return nil, pingOut{Message: "pong"}, nil
	})
	return nil
}

// newHooksRegistry builds a minimal Registry so `claude hooks` has at least
// one leaf command to demonstrate — a real consumer registers only the
// events it actually needs.
func newHooksRegistry() *hooks.Registry {
	return hooks.NewRegistry().Stop(func(_ context.Context, _ io.Reader, _ hooks.StopPayload) hooks.Response {
		return hooks.Response{}
	})
}

// newStatuslineProvider is a trivial StatuslineProvider that renders
// nothing — the shape a real consumer starts from before wiring in actual
// session/domain state (see the statusline package's own README/tests for
// a populated example).
func newStatuslineProvider() statusline.StatuslineProviderFunc {
	return func(_ context.Context, _ statusline.Payload, _ string) ([]statusline.Segment, error) {
		return nil, nil
	}
}

// newRootCmd assembles the example's cobra root: a minimal *shesha.App (one
// "ping" tool) mounted under `server` (stdio by default, `--http`/
// `--stateless` per MC-31), plus the Claude Code host's `claude` namespace
// (hooks + statusline) mounted via cli.MountProviders. Kept separate from
// main so tests can exercise the command tree without starting a server.
func newRootCmd() *cobra.Command {
	app, err := shesha.New(shesha.Info{Name: "example-mcp", Version: "0.0.0-example"}, claudecode.New(), pingCap{})
	must(err, "compose app")

	root := &cobra.Command{
		Use:   "example-mcp",
		Short: "Example shesha server assembling the standard command set",
	}

	sc := cli.ServerCmd(cli.Server{App: app, HTTP: &cli.ServerHTTP{}})
	root.AddCommand(sc)

	// cli.UseAsDefault(root, sc) is opt-in: it makes a bare `example-mcp`
	// invocation (no subcommand) run the server directly instead of
	// showing help — useful for a Claude Code plugin binary launched
	// without an explicit subcommand. The dangernoodle convention omits it
	// so bare invocation shows help and an explicit `server` subcommand is
	// required; uncomment to opt in:
	// cli.UseAsDefault(root, sc)

	claudeProvider := claudecode.NewProvider(newHooksRegistry(), statusline.Command(newStatuslineProvider()))
	must(cli.MountProviders(root, claudeProvider), "mount providers")

	root.AddCommand(cli.VersionCmd(shesha.Info{Name: "example-mcp", Version: "0.0.0-example"}))

	return root
}

// must panics on a non-nil err, wrapping it with msg. Both newRootCmd call
// sites (shesha.New, cli.MountProviders) are literal, static calls that
// cannot fail from user input — a non-nil err here is a programming bug in
// this file, not a runtime condition, so must panics rather than widening
// newRootCmd's signature with an error return.
func must(err error, msg string) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", msg, err))
	}
}

// run builds the root command, wires args/stdout/stderr, executes it, and
// returns the process exit code — split from main so tests can exercise
// both the success and failure paths without main's os.Exit tearing down
// the test binary.
func run(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
