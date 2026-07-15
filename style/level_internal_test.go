package style

import (
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// TestLevelFromProfile_MapsEveryTermenvProfile white-box tests
// levelFromProfile's full termenv.Profile -> Level mapping directly. This
// covers the TrueColor and ANSI256 branches, which Detect can never reach
// through a non-TTY writer (termenv's ColorProfile short-circuits to
// Ascii whenever isTTY() is false, regardless of env vars — see
// termenv_unix.go) — so a headless CI writer-based test can only ever
// observe LevelNone/LevelBasic. levelFromProfile itself is a pure,
// deterministic mapping, independently worth asserting in full.
func TestLevelFromProfile_MapsEveryTermenvProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile termenv.Profile
		want    Level
	}{
		{"truecolor", termenv.TrueColor, LevelTrueColor},
		{"ansi256", termenv.ANSI256, Level256},
		{"ansi", termenv.ANSI, LevelBasic},
		{"ascii", termenv.Ascii, LevelNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, levelFromProfile(tt.profile))
		})
	}
}
