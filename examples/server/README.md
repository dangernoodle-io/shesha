# examples/server

Command `example-mcp` demonstrates assembling shesha's standard command set:

- A minimal `*shesha.App` — the Claude Code host adapter plus one trivial
  "ping" tool.
- A `server` command (`cli.ServerCmd`) that serves stdio by default and
  switches to streamable-HTTP via `--http <addr>` (`--stateless` for
  JSON-response, non-SSE sessions) — MC-31's transport selection.
- The Claude Code host's `claude` namespace (`claude hooks`, `claude
  statusline`), mounted onto the same root as `server` via
  `cli.MountProviders` — MC-30's unified mount.
- A `version` command (`cli.VersionCmd`).

This is a hand-authored example, not `internal/docsgen` output —
`examples/` is excluded from docsgen. MC-32's README quickstart lifts its
commands from here.

The example follows the dangernoodle convention of NOT calling
`cli.UseAsDefault`: a bare `go run . ` invocation shows help, and the server
must be started via the explicit `server` subcommand. `main.go` has a
commented-out `cli.UseAsDefault(root, sc)` call showing the opt-in shape
(useful for a Claude Code plugin binary launched without an explicit
subcommand).

## Run it

```sh
# stdio (default transport)
go run . server

# streamable-HTTP
go run . server --http :8080
# in another shell:
curl -s http://localhost:8080/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'

# name/version
go run . version

# the Claude Code plugin surface
go run . claude hooks --help
go run . claude statusline --help
```
