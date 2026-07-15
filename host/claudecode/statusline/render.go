package statusline

import (
	"strings"

	"github.com/dangernoodle-io/shesha/style"
)

// Segment is one styled chunk of statusline output. A consumer builds the
// whole line as a slice of Segments — including any literal separators
// (e.g. " | ") as their own plain Segment — since Render performs no
// implicit joining beyond concatenation.
//
// Color is optional and consumer-agnostic: a hex string ("#ff0000") or a
// termenv ANSI color-code string ("0"-"255"), resolved through the style
// renderer's tier-aware degradation at render time. Leaving it empty
// (pogopin's plain segments) renders unstyled text; ouroboros's colored
// segments (its existing priorityColor semantics) populate it. Dim/Bold
// apply independent of Color.
type Segment struct {
	Text  string
	Color string
	Dim   bool
	Bold  bool
}

// Render concatenates segments into one line, applying each segment's
// Color/Dim/Bold through r's tier-aware color degradation. A Renderer
// resolved to style.LevelNone (Plain, NO_COLOR, or a dumb terminal)
// collapses every segment to its bare Text with no escape sequences at
// all — the same code path a plain-segment consumer (pogopin) and a
// colored-segment consumer (ouroboros) both render through; only what
// each populates on Segment differs.
func Render(segments []Segment, r style.Renderer) string {
	var sb strings.Builder

	for _, seg := range segments {
		sb.WriteString(r.Render(seg.Text, style.Style{Fg: seg.Color, Dim: seg.Dim, Bold: seg.Bold}))
	}

	return sb.String()
}
