// Package httpx is what a standalone HTTP server rides on: a turnkey mux
// (NewMux) for co-mounting shesha's bare MC-5 MCP handler alongside a
// consumer's own routes, a graceful Serve loop mirroring the stdio
// lifecycle in package cli, and a WriteJSON helper for JSON responses
// (closing the MC-19 jsonutil deferral). httpx is stdlib-only plus
// jsonutil — no cobra, no go-sdk, no shesha/mcpx — so it stays a leaf
// package any consumer can import without pulling in shesha's own
// dependency surface.
package httpx
