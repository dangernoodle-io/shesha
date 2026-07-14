package statusline_test

import (
	"testing"

	"github.com/dangernoodle-io/shesha/host/claudecode/statusline"
	"github.com/stretchr/testify/assert"
)

func TestResolve_AppPrefixEnvWinsOverEverything(t *testing.T) {
	t.Setenv("ACME_SESSION_ID", "from-app-env")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-claude-env")

	payload := statusline.Payload{SessionID: "from-payload"}

	got := statusline.Resolve(payload, "ACME")

	assert.Equal(t, "from-app-env", got)
}

func TestResolve_PayloadWinsWhenNoAppEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-claude-env")

	payload := statusline.Payload{SessionID: "from-payload"}

	got := statusline.Resolve(payload, "ACME")

	assert.Equal(t, "from-payload", got)
}

func TestResolve_ClaudeCodeEnvWinsWhenNoAppEnvOrPayload(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-claude-env")

	got := statusline.Resolve(statusline.Payload{}, "ACME")

	assert.Equal(t, "from-claude-env", got)
}

func TestResolve_EmptyWhenNothingResolves(t *testing.T) {
	t.Setenv("ACME_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")

	got := statusline.Resolve(statusline.Payload{}, "ACME")

	assert.Empty(t, got)
}

func TestResolve_EmptyAppPrefixSkipsOverrideTier(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-claude-env")

	payload := statusline.Payload{SessionID: "from-payload"}

	got := statusline.Resolve(payload, "")

	assert.Equal(t, "from-payload", got, "empty appPrefix must not consult a bare _SESSION_ID env var")
}
