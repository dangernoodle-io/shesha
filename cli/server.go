// Package cli provides cobra.Command factories for mounting a *shesha.App
// into a consumer's own root command. shesha does not own the root: the
// consumer builds its own cobra root and mounts ServerCmd (and, optionally,
// VersionCmd) as subcommands of it.
//
// shesha owns transport selection, running the server, and graceful
// shutdown; the consumer extends the server command via Server.App plus the
// OnStart/OnShutdown hooks, Flags, and Subcommands (arbitrary cobra
// subtrees — see CommandProvider/MountProviders for the uniform mounting
// API). Use UseAsDefault to make a bare invocation of the consumer's binary
// (no subcommand given) run the server command directly — useful for a
// Claude Code plugin launched without an explicit subcommand.
//
// cobra auto-provides a `completion` command on any root with subcommands,
// so this package does not add one.
//
// stdio is the default transport. Setting Server.HTTP registers a `--http
// <addr>` flag (and a `--stateless` flag) on the built command; passing
// --http switches that single invocation to serving MCP over HTTP via
// App.HTTPHandler mounted on an httpx mux, instead of stdio. Nothing about
// UseAsDefault changes: a bare invocation still runs whichever mode the
// copied flags select.
//
// ServerCmd also always registers a `--read-only` bool flag (default
// false), regardless of transport: passing --read-only calls
// App.Gate(shesha.ReadOnlyMode()) before the transport starts, hard-blocking
// every non-ReadOnly tool from ever being registered — shesha's one
// built-in risk-gating axis. --read-only, --http, and --stateless are all
// reserved flag names: a Server.Flags registration of any of them collides
// and pflag panics at command construction.
//
// Server.ReadOnlyEnv opts a built command into also entering read-only mode
// when that named environment variable is set to a truthy value, OR'd with
// --read-only — see Server.ReadOnlyEnv's doc comment.
package cli

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/httpx"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Server configures the command ServerCmd builds. Only App is required.
type Server struct {
	// App is the composed server to run. Required.
	App *shesha.App

	// Use is the command name. Defaults to "server".
	Use string

	// Short is the one-line help text. A sensible default is used if empty.
	Short string

	// OnStart runs before the transport starts (e.g. open a DB connection).
	// Nil-safe. An error aborts before the server runs and before
	// OnShutdown runs.
	OnStart func(context.Context) error

	// OnShutdown runs after the server stops, on every path where the
	// server actually started. Nil-safe.
	OnShutdown func(context.Context) error

	// Flags registers consumer flags on the built command. Nil-safe.
	Flags func(*pflag.FlagSet)

	// Subcommands are attached under the built command via AddCommand.
	// Each entry may carry its own subtree of any depth — cobra's
	// AddCommand nests freely, so grandchildren (and deeper) work the same
	// as leaves; nothing here flattens them.
	Subcommands []*cobra.Command

	// HTTP opts the built command into HTTP serving via a --http flag.
	// Nil (the default) means stdio only — no --http/--stateless flags
	// are registered at all. When HTTP is non-nil, ServerCmd reserves the
	// flag names "http" and "stateless": a Server.Flags registration of
	// either name collides and pflag panics at command construction.
	HTTP *ServerHTTP

	// ReadOnlyEnv, if non-empty, names an environment variable that also
	// puts the built server command into read-only mode when set to a
	// truthy value (1/true/yes/on, case-insensitive, surrounding
	// whitespace trimmed), OR'd with the --read-only flag: either one
	// alone is enough to gate. Empty (the default) means flag-only —
	// behavior is byte-identical to a Server with no ReadOnlyEnv set.
	ReadOnlyEnv string
}

// ServerHTTP configures optional HTTP serving for the server command. When
// Server.HTTP is non-nil, ServerCmd registers a `--http <addr>` string flag
// (empty default) and a `--stateless` bool flag; stdio remains the default
// transport whenever --http is not given a value.
type ServerHTTP struct {
	// MCPPath is the path the MCP endpoint is mounted at, e.g. "/mcp".
	// Defaults to "/mcp" when empty.
	MCPPath string

	// Routes optionally mounts additional consumer routes on the same mux
	// the MCP endpoint is served from — for a consumer that wants to
	// co-host other HTTP routes alongside MCP on one listener. Nil-safe.
	Routes func(*http.ServeMux)

	// Stateless sets the --stateless flag's default value. false (the
	// zero value) means the server is stateful by default; --stateless
	// flips a single invocation to stateless (and JSON responses instead
	// of SSE). --stateless applies only in HTTP mode; it has no effect
	// on the stdio transport.
	Stateless bool
}

// ServerCmd builds the server command: runs App.Run(ctx) with signal-driven
// graceful shutdown, calling OnStart before and OnShutdown after.
func ServerCmd(s Server) *cobra.Command {
	use := s.Use
	if use == "" {
		use = "server"
	}

	short := s.Short
	if short == "" {
		short = "Run the MCP server over the configured transport"
	}

	var httpAddr string
	var stateless bool
	var readOnly bool

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			ro := readOnly || (s.ReadOnlyEnv != "" && envTruthy(os.Getenv(s.ReadOnlyEnv)))
			if ro {
				if err := s.App.Gate(shesha.ReadOnlyMode()); err != nil {
					return err
				}
			}

			run := s.App.Run
			if s.HTTP != nil && httpAddr != "" {
				run = httpRun(s, httpAddr, stateless)
			}

			return runLifecycle(ctx, run, s.OnStart, s.OnShutdown)
		},
	}

	// --read-only applies to every server command regardless of transport
	// (it gates tool registration, not transport selection), so — unlike
	// --http/--stateless — it is registered unconditionally, not gated on
	// Server.HTTP. Registering it here, before Server.Flags runs below,
	// reserves the name: a Server.Flags registration of "read-only" would
	// collide on the same FlagSet and pflag panics at construction.
	cmd.Flags().BoolVar(&readOnly, "read-only", false,
		"gate the app to ReadOnly tools only; write/destructive tools are never registered")

	if s.HTTP != nil {
		cmd.Flags().StringVar(&httpAddr, "http", "",
			"serve MCP over HTTP at this address instead of stdio (e.g. :8080)")
		cmd.Flags().BoolVar(&stateless, "stateless", s.HTTP.Stateless,
			"serve HTTP sessions statelessly, with JSON responses instead of SSE (only applies with --http; ignored in stdio mode)")
	}

	if s.Flags != nil {
		s.Flags(cmd.Flags())
	}

	for _, sub := range s.Subcommands {
		cmd.AddCommand(sub)
	}

	return cmd
}

// httpRun builds the run func for HTTP mode: assembles the MCP handler
// (applying stateless/JSON-response options when stateless is set), mounts
// it on an httpx mux at s.HTTP.MCPPath (default "/mcp"), applies
// s.HTTP.Routes if non-nil, and serves via httpx.Serve on addr. Returned as
// a run func so runLifecycle wraps it with OnStart/OnShutdown identically
// to the stdio path.
func httpRun(s Server, addr string, stateless bool) func(context.Context) error {
	return func(ctx context.Context) error {
		var opts []mcpx.HTTPOption
		if stateless {
			opts = append(opts, mcpx.WithStateless(true), mcpx.WithJSONResponse(true))
		}

		mcpPath := s.HTTP.MCPPath
		if mcpPath == "" {
			mcpPath = "/mcp"
		}

		mux := httpx.NewMux(mcpPath, s.App.HTTPHandler(opts...))
		if s.HTTP.Routes != nil {
			s.HTTP.Routes(mux)
		}

		return httpx.Serve(ctx, addr, mux)
	}
}

// runLifecycle drives the start->run->shutdown sequence, App-free so tests
// can exercise it with fake funcs. Order: onStart (if non-nil; an error
// aborts here, skipping run and onShutdown) -> run -> onShutdown (if
// non-nil, always, once run has started, with a context that outlives ctx's
// cancellation).
//
// A ctx-cancellation exit is treated as clean: if run's error is (or wraps)
// context.Canceled, or ctx was already cancelled by the time run returns,
// the run is not treated as a failure. The result joins (via errors.Join)
// the run failure, if any real one occurred, with the onShutdown error, if
// any — so a double failure (run fails for real AND onShutdown fails)
// surfaces both instead of one silently swallowing the other. A clean run
// with a nil onShutdown yields a nil result; errors.Is still matches either
// wrapped error individually.
func runLifecycle(ctx context.Context, run, onStart, onShutdown func(context.Context) error) error {
	if onStart != nil {
		if err := onStart(ctx); err != nil {
			return err
		}
	}

	runErr := run(ctx)

	var shutdownErr error
	if onShutdown != nil {
		shutdownErr = onShutdown(context.WithoutCancel(ctx))
	}

	var runFail error
	if runErr != nil && !errors.Is(runErr, context.Canceled) && ctx.Err() == nil {
		runFail = runErr
	}

	return errors.Join(runFail, shutdownErr)
}

// UseAsDefault makes bare invocation of root (no subcommand) run cmd: it
// sets root.RunE = cmd.RunE and copies cmd's flags onto root's local
// FlagSet. The flag copy matters because cobra parses argv against the
// FlagSet of whichever command is actually executing — on bare invocation
// that's root, not cmd — so without it any flag registered via Server.Flags
// or Server.HTTP (e.g. --http) would be rejected as unknown on bare
// invocation.
//
// Call UseAsDefault AFTER ServerCmd, once cmd.Flags() already carries the
// flags Server.Flags registered. The normal pattern keeps the server
// reachable both ways:
//
//	sc := cli.ServerCmd(s)
//	root.AddCommand(sc)
//	cli.UseAsDefault(root, sc)
//
// so `myapp server --foo` and bare `myapp --foo` both work. Without
// UseAsDefault, bare invocation shows help (cobra's default).
func UseAsDefault(root, cmd *cobra.Command) {
	root.RunE = cmd.RunE
	root.Flags().AddFlagSet(cmd.Flags())
}

// envTruthy reports whether raw (an environment variable's value, trimmed
// and lowercased) is one of the truthy tokens: "1", "true", "yes", "on".
// Everything else, including the empty string, is false.
func envTruthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
