package statusline_test

import (
	"testing"

	"github.com/dangernoodle-io/shesha/host/claudecode/statusline"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func profilePtr(p termenv.Profile) *termenv.Profile { return &p }

func TestRender_JoinsSegmentsWithNoImplicitSeparator(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "a"},
		{Text: "|"},
		{Text: "b"},
	}

	got := statusline.Render(segs, statusline.RenderOptions{Plain: true})

	assert.Equal(t, "a|b", got)
}

func TestRender_PlainStripsAllStyling(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "red", Color: "#ff0000", Bold: true, Dim: true},
	}

	got := statusline.Render(segs, statusline.RenderOptions{Plain: true})

	assert.Equal(t, "red", got)
}

func TestRender_AsciiProfileStripsAllStyling(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "red", Color: "#ff0000", Bold: true},
	}

	got := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.Ascii)})

	assert.Equal(t, "red", got)
}

func TestRender_TrueColorEmitsRGBEscapeSequence(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "red", Color: "#ff0000"},
	}

	got := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.TrueColor)})

	assert.NotEqual(t, "red", got, "TrueColor must apply an escape sequence")
	assert.Contains(t, got, "red")
	assert.Contains(t, got, "\x1b[")
}

func TestRender_DegradesTrueColorToANSI256(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "red", Color: "#ff0000"},
	}

	trueColor := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.TrueColor)})
	ansi256 := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.ANSI256)})

	assert.NotEqual(t, trueColor, ansi256, "ANSI256 must degrade the RGB sequence to an 8-bit one")
	assert.Contains(t, ansi256, "red")
}

func TestRender_DegradesToANSI16(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "red", Color: "#ff0000"},
	}

	ansi256 := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.ANSI256)})
	ansi16 := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.ANSI)})

	assert.NotEqual(t, ansi256, ansi16)
	assert.Contains(t, ansi16, "red")
}

func TestRender_DimAppliesFaintStyleWhenColored(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "dim", Dim: true},
	}

	got := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.ANSI)})

	assert.NotEqual(t, "dim", got, "Dim must apply a faint escape even with no Color set")
	assert.Contains(t, got, "dim")
}

func TestRender_EmptySegmentsRendersEmptyString(t *testing.T) {
	got := statusline.Render(nil, statusline.RenderOptions{})

	assert.Empty(t, got)
}

func TestRender_InvalidColorStringNeverPanicsAndRendersBareText(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "oops", Color: "notacolor"},
	}

	var got string
	assert.NotPanics(t, func() {
		got = statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.TrueColor)})
	})
	assert.Equal(t, "oops", got, "an unparseable Color must degrade to termenv's nil Color (no-op)")
}

func TestRender_NoColorSegmentIgnoresColorFieldEvenWhenColored(t *testing.T) {
	segs := []statusline.Segment{
		{Text: "plain"},
	}

	got := statusline.Render(segs, statusline.RenderOptions{Profile: profilePtr(termenv.TrueColor)})

	assert.Equal(t, "plain", got, "a Segment with no Color/Dim/Bold must render as bare text")
}
