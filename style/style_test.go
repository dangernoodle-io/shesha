package style_test

import (
	"bytes"
	"testing"

	"github.com/dangernoodle-io/shesha/style"
	"github.com/stretchr/testify/assert"
)

func TestNew_WithLevelForcesEachTier(t *testing.T) {
	tests := []struct {
		name  string
		level style.Level
	}{
		{"none", style.LevelNone},
		{"basic", style.LevelBasic},
		{"256", style.Level256},
		{"truecolor", style.LevelTrueColor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := style.New(&bytes.Buffer{}, style.WithLevel(tt.level))

			assert.Equal(t, tt.level, r.Level())
		})
	}
}

func TestDetect_NoColorEnvForcesLevelNone(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := style.Detect(&bytes.Buffer{})

	assert.Equal(t, style.LevelNone, got)
}

func TestNew_NoOptionsDetectsFromWriter(t *testing.T) {
	r := style.New(&bytes.Buffer{})

	assert.Equal(t, style.LevelNone, r.Level(), "a bytes.Buffer is never a TTY")
}

func TestDetect_NonTerminalWriterIsLevelNone(t *testing.T) {
	// A bytes.Buffer is never a TTY, so detection must degrade to
	// LevelNone regardless of the process environment.
	got := style.Detect(&bytes.Buffer{})

	assert.Equal(t, style.LevelNone, got)
}

func TestDetect_CliColorForceUpgradesNonTerminalToLevelBasic(t *testing.T) {
	// CLICOLOR_FORCE forces color even on a non-TTY writer, but termenv
	// only ever forces up to ANSI (never 256/TrueColor) for a
	// non-terminal writer — this exercises Detect's ANSI->LevelBasic
	// mapping through the real termenv env-var contract, not a TTY.
	t.Setenv("CLICOLOR_FORCE", "1")

	got := style.Detect(&bytes.Buffer{})

	assert.Equal(t, style.LevelBasic, got)
}

func TestRender_LevelNoneReturnsBareTextNoEscapes(t *testing.T) {
	r := style.New(&bytes.Buffer{}, style.WithLevel(style.LevelNone))

	got := r.Render("hello", style.Style{Fg: "#ff0000", Dim: true, Bold: true})

	assert.Equal(t, "hello", got)
	assert.NotContains(t, got, "\x1b")
}

func TestRender_ForcedTierWithHexColorEmitsEscapes(t *testing.T) {
	tests := []style.Level{style.LevelBasic, style.Level256, style.LevelTrueColor}

	for _, level := range tests {
		r := style.New(&bytes.Buffer{}, style.WithLevel(level))

		got := r.Render("red", style.Style{Fg: "#ff0000"})

		assert.NotEqual(t, "red", got, "level %v must apply an escape sequence", level)
		assert.Contains(t, got, "red")
		assert.Contains(t, got, "\x1b[")
	}
}

func TestRender_ForcedTierWithANSIIndexColorEmitsEscapes(t *testing.T) {
	r := style.New(&bytes.Buffer{}, style.WithLevel(style.LevelBasic))

	got := r.Render("red", style.Style{Fg: "1"})

	assert.NotEqual(t, "red", got)
	assert.Contains(t, got, "red")
	assert.Contains(t, got, "\x1b[")
}

func TestRender_DimAndBoldApplyWithoutColor(t *testing.T) {
	r := style.New(&bytes.Buffer{}, style.WithLevel(style.LevelTrueColor))

	got := r.Render("dim", style.Style{Dim: true, Bold: true})

	assert.NotEqual(t, "dim", got)
	assert.Contains(t, got, "dim")
}

func TestRender_NoStyleFieldsRendersBareTextEvenWhenColorCapable(t *testing.T) {
	r := style.New(&bytes.Buffer{}, style.WithLevel(style.LevelTrueColor))

	got := r.Render("plain", style.Style{})

	assert.Equal(t, "plain", got)
}
