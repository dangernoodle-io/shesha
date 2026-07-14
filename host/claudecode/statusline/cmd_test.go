package statusline_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dangernoodle-io/shesha/host/claudecode/statusline"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand_RendersProviderSegments(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(_ context.Context, payload statusline.Payload, sessionID string) ([]statusline.Segment, error) {
			assert.Equal(t, "sess-123", sessionID)
			assert.Equal(t, "/work/dir", payload.Cwd)

			return []statusline.Segment{{Text: "myapp: "}, {Text: "3 items"}}, nil
		},
	)

	cmd := statusline.Command(provider, statusline.WithAppPrefix("ACME"))
	cmd.SetIn(strings.NewReader(`{"session_id":"sess-123","cwd":"/work/dir"}`))

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--plain"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "myapp: 3 items\n", out.String())
}

func TestCommand_AppPrefixEnvOverridesStdinSessionID(t *testing.T) {
	t.Setenv("ACME_SESSION_ID", "from-env")

	var gotSessionID string
	provider := statusline.StatuslineProviderFunc(
		func(_ context.Context, _ statusline.Payload, sessionID string) ([]statusline.Segment, error) {
			gotSessionID = sessionID
			return nil, nil
		},
	)

	cmd := statusline.Command(provider, statusline.WithAppPrefix("ACME"))
	cmd.SetIn(strings.NewReader(`{"session_id":"from-stdin"}`))
	cmd.SetOut(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "from-env", gotSessionID)
}

func TestCommand_EmptyStdinFailsOpenAndRendersNothing(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(_ context.Context, payload statusline.Payload, _ string) ([]statusline.Segment, error) {
			assert.Equal(t, statusline.Payload{}, payload, "empty stdin must decode to a zero Payload")
			return nil, nil
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader(""))

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Empty(t, out.String())
}

func TestCommand_BadJSONStdinFailsOpen(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(_ context.Context, payload statusline.Payload, _ string) ([]statusline.Segment, error) {
			assert.Equal(t, statusline.Payload{}, payload, "unparseable stdin must decode to a zero Payload")
			return nil, nil
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader("not json"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Empty(t, out.String())
}

func TestCommand_ProviderErrorFailsOpenExitsZero(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return nil, errors.New("boom")
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader("{}"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err, "a provider error must never surface as a non-nil cobra error")
	assert.Empty(t, out.String())
}

func TestCommand_ProviderPanicFailsOpenExitsZero(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			panic("kaboom")
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader("{}"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	assert.NotPanics(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})
	assert.Empty(t, out.String())
}

func TestCommand_EmptySegmentsRendersNothing(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return []statusline.Segment{}, nil
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader("{}"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.Empty(t, out.String())
}

func TestCommand_NonEmptySegmentsRenderingToEmptyStringPrintsNothing(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return []statusline.Segment{{Text: ""}}, nil
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader("{}"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.Empty(t, out.String())
}

func TestCommand_WithForceProfileRendersColorOnNonTTYStdout(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return []statusline.Segment{{Text: "example", Color: "1"}}, nil
		},
	)

	cmd := statusline.Command(provider, statusline.WithForceProfile(termenv.ANSI))
	cmd.SetIn(strings.NewReader("{}"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "\x1b", "WithForceProfile must force ANSI escapes despite non-TTY stdout")
}

func TestCommand_PlainFlagWinsOverForceProfile(t *testing.T) {
	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return []statusline.Segment{{Text: "example", Color: "1"}}, nil
		},
	)

	cmd := statusline.Command(provider, statusline.WithForceProfile(termenv.TrueColor))
	cmd.SetIn(strings.NewReader("{}"))
	cmd.SetArgs([]string{"--plain"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "example\n", out.String(), "--plain must win over WithForceProfile")
}

func TestCommand_WithoutForceProfileKeepsAutoDetection(t *testing.T) {
	// Pin termenv.EnvColorProfile()'s auto-detection deterministically:
	// NO_COLOR is honored unconditionally, regardless of ambient
	// CLICOLOR_FORCE or TTY state in the process running this test.
	t.Setenv("NO_COLOR", "1")

	provider := statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return []statusline.Segment{{Text: "example", Color: "1"}}, nil
		},
	)

	cmd := statusline.Command(provider)
	cmd.SetIn(strings.NewReader("{}"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.NotContains(t, out.String(), "\x1b", "auto-detection on non-TTY test stdout must yield Ascii (no escapes)")
}

func TestCommand_UsePlainCommandName(t *testing.T) {
	cmd := statusline.Command(statusline.StatuslineProviderFunc(
		func(context.Context, statusline.Payload, string) ([]statusline.Segment, error) {
			return nil, nil
		},
	))

	assert.Equal(t, "statusline", cmd.Use)
}
