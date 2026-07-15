// Package statusline implements shesha's Claude Code statusLine adapter: the
// stdin payload contract, a session-identity resolver, a segment renderer
// built on the style seam (see style.Renderer), and a StatuslineProvider
// seam + cobra command factory that together produce a `statusline` command
// a host adapter mounts (e.g. as `claude statusline`, see
// host/claudecode/provider.go).
//
// # Fail-open contract
//
// Command's RunE always returns nil, even when stdin fails to decode, the
// registered StatuslineProvider panics or returns an error, or Render
// produces an empty line: failOpen recovers a panic and swallows a
// returned error, logging either to stderr. A statusline command must never
// print cobra usage text or exit non-zero — Claude Code's statusline
// invocation has nowhere to surface that, and a non-zero exit would blank
// the status line entirely. Mirrors pogopin's `pogo statusline` posture
// (BR-76) and the sibling host/claudecode/hooks package's FailOpen.
//
// # Session-identity resolution
//
// Resolve generalizes pogopin's BR-76 precedence with a consumer-supplied
// env-var prefix:
//
//	<appPrefix>_SESSION_ID env  >  payload.SessionID  >  CLAUDE_CODE_SESSION_ID env  >  ""
//
// An empty result means "no session resolved" — a StatuslineProvider should
// render nothing rather than fall back to an unfiltered/global view.
//
// # Segment / render model
//
// A StatuslineProvider returns a []Segment — one styled chunk per unit of
// output, including any literal separators as their own plain Segment.
// Render is the single place that turns Segments into a line, delegating
// each Segment's optional Color/Dim/Bold to a style.Renderer's tier-aware
// color degradation (LevelTrueColor -> Level256 -> LevelBasic ->
// LevelNone). A consumer that never sets Color (pogopin's plain segments)
// and one that always does (ouroboros's colored KB/backlog/priority
// segments) render through the exact same code path —
// LevelNone/--plain/NO_COLOR collapses every Segment to its bare Text,
// matching pogopin's existing no-color output and ouroboros's
// --plain/OUROBOROS_NO_COLOR mode.
//
// # Implementing StatuslineProvider
//
//	type myProvider struct{ /* ... */ }
//
//	func (p myProvider) Statusline(
//		ctx context.Context, payload statusline.Payload, sessionID string,
//	) ([]statusline.Segment, error) {
//		if sessionID == "" {
//			return nil, nil // render nothing
//		}
//		return []statusline.Segment{
//			{Text: "myapp: ", Dim: true},
//			{Text: "3 items", Color: "6"}, // ANSI cyan
//		}, nil
//	}
//
//	cmd := statusline.Command(myProvider{}, statusline.WithAppPrefix("MYAPP"))
//
// A host adapter's CommandProvider (e.g. claudecode.NewProvider) mounts cmd
// as an extra subcommand alongside `claude hooks`, so it appears as `<host>
// statusline` without this package needing any host-specific wiring.
package statusline
