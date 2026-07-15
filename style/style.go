package style

import (
	"io"

	"github.com/muesli/termenv"
)

// Level is the color capability tier a terminal (or a writer standing in
// for one) supports, from none — bare text, no escapes at all — up to
// 24-bit truecolor.
type Level int

const (
	// LevelNone means no color support: Render always returns bare text.
	LevelNone Level = iota
	// LevelBasic is 16-color ANSI.
	LevelBasic
	// Level256 is 8-bit ANSI256 color.
	Level256
	// LevelTrueColor is 24-bit RGB color.
	LevelTrueColor
)

// Style describes one styled chunk of text, independent of any rendering
// library. Fg is a hex color ("#rrggbb") or a termenv ANSI color index
// ("0"-"255"); empty means no color. Dim/Bold apply regardless of Fg.
type Style struct {
	Fg   string
	Dim  bool
	Bold bool
}

// Renderer renders text under Style at some resolved Level. A consumer may
// supply its own implementation; New returns shesha's default.
type Renderer interface {
	// Render styles text per s, degrading to bare text (no escapes) when
	// Level() is LevelNone.
	Render(text string, s Style) string
	// Level reports the capability tier this Renderer resolved to.
	Level() Level
}

// Detect resolves the color capability tier for w: it honors NO_COLOR and
// CLICOLOR/CLICOLOR_FORCE, treats TERM=dumb (and any non-terminal writer)
// as no color, and otherwise inspects w itself — via termenv.NewOutput,
// which is fd/TTY-aware — rather than the process's stdout/stderr.
func Detect(w io.Writer) Level {
	return levelFromProfile(termenv.NewOutput(w).EnvColorProfile())
}

func levelFromProfile(p termenv.Profile) Level {
	switch p {
	case termenv.TrueColor:
		return LevelTrueColor
	case termenv.ANSI256:
		return Level256
	case termenv.ANSI:
		return LevelBasic
	default:
		return LevelNone
	}
}

// profileFromLevel is the inverse of levelFromProfile: it builds the
// termenv.Profile a resolved Level renders through.
func profileFromLevel(l Level) termenv.Profile {
	switch l {
	case LevelTrueColor:
		return termenv.TrueColor
	case Level256:
		return termenv.ANSI256
	case LevelBasic:
		return termenv.ANSI
	default:
		return termenv.Ascii
	}
}

type config struct {
	level       Level
	levelForced bool
}

// Option configures New.
type Option func(*config)

// WithLevel forces a specific capability tier, bypassing Detect. Tests
// (and explicit --plain/--no-color flags) use this to get deterministic
// behavior independent of the process environment or TTY state.
func WithLevel(l Level) Option {
	return func(c *config) {
		c.level = l
		c.levelForced = true
	}
}

// renderer is style's default, termenv-backed Renderer.
type renderer struct {
	level   Level
	profile termenv.Profile
}

// New returns shesha's default Renderer for w: it detects w's color
// capability tier via Detect unless overridden with WithLevel.
func New(w io.Writer, opts ...Option) Renderer {
	var cfg config

	for _, opt := range opts {
		opt(&cfg)
	}

	level := cfg.level
	if !cfg.levelForced {
		level = Detect(w)
	}

	return &renderer{level: level, profile: profileFromLevel(level)}
}

// Level reports the capability tier this renderer resolved to.
func (r *renderer) Level() Level {
	return r.level
}

// Render styles text under s, degrading to bare text at LevelNone.
// Otherwise it builds the termenv style for the resolved profile: a
// foreground color (when s.Fg is set), Faint (when s.Dim), and Bold (when
// s.Bold) — the same shape as statusline's styleSegment, generalized
// behind this seam.
func (r *renderer) Render(text string, s Style) string {
	if r.level == LevelNone {
		return text
	}

	styled := r.profile.String(text)

	if s.Fg != "" {
		styled = styled.Foreground(r.profile.Color(s.Fg))
	}
	if s.Dim {
		styled = styled.Faint()
	}
	if s.Bold {
		styled = styled.Bold()
	}

	return styled.String()
}
