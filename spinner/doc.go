// Package spinner is shesha's seam over a terminal progress indicator:
// Spinner is the interface (Start/Message/Stop/Success/Fail); a consumer
// may supply its own implementation. New returns shesha's default, built
// on package style for coloring.
//
// The default writes to os.Stderr only — never stdout, which carries MCP
// stdio protocol traffic in shesha's servers — and degrades to a single
// plain line (no animation, no escape sequences) whenever its
// style.Renderer resolves to style.LevelNone (non-TTY, NO_COLOR, CI). It
// is distinct from mcpx's MCP-protocol progress notifications and from
// the statusline renderer, which target different surfaces entirely.
//
// The spinner installs NO signal handlers — interrupt policy belongs to
// the caller. Go has no per-registrant signal.Reset, so an in-library
// handler would deregister a consumer's own SIGINT/SIGTERM handling, which
// this package must never do. On a normal Stop/Success/Fail the cursor is
// restored; on an uncaught SIGINT/SIGTERM the terminal may be left with a
// hidden cursor. To restore it on interrupt, call Stop from your own
// signal handler:
//
//	sig := make(chan os.Signal, 1)
//	signal.Notify(sig, os.Interrupt)
//	go func() { <-sig; sp.Stop(); os.Exit(1) }()
//
// Writer errors (e.g. a broken pipe) are intentionally ignored — terminal
// output here is best-effort, not a data path.
package spinner
