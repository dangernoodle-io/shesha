package hooks

import (
	"bytes"
	"io"
	"os"

	"github.com/dangernoodle-io/shesha/jsonutil"
	"github.com/spf13/cobra"
)

// Claude Code's documented hook_event_name values, used both as the
// hookSpecificOutput.hookEventName wire value and (kebab-cased) as each
// leaf command's Use.
const (
	eventStop             = "Stop"
	eventSubagentStop     = "SubagentStop"
	eventSubagentStart    = "SubagentStart"
	eventUserPromptSubmit = "UserPromptSubmit"
	eventPreToolUse       = "PreToolUse"
	eventPostToolUse      = "PostToolUse"
	eventPreCompact       = "PreCompact"
	eventSessionStart     = "SessionStart"
)

// Command builds the `hooks` command group: one leaf subcommand per event
// registered on reg. An event with no registered handler gets no leaf at
// all, so `hooks --help` (and command lookup) only ever shows events the
// consumer actually wired up.
func Command(reg *Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Handle Claude Code plugin hook events",
	}

	if reg.stop != nil {
		cmd.AddCommand(leaf("stop", eventStop, reg.stop))
	}
	if reg.subagentStop != nil {
		cmd.AddCommand(leaf("subagent-stop", eventSubagentStop, reg.subagentStop))
	}
	if reg.subagentStart != nil {
		cmd.AddCommand(leaf("subagent-start", eventSubagentStart, reg.subagentStart))
	}
	if reg.userPromptSubmit != nil {
		cmd.AddCommand(leaf("user-prompt-submit", eventUserPromptSubmit, reg.userPromptSubmit))
	}
	if reg.preToolUse != nil {
		cmd.AddCommand(leaf("pre-tool-use", eventPreToolUse, reg.preToolUse))
	}
	if reg.postToolUse != nil {
		cmd.AddCommand(leaf("post-tool-use", eventPostToolUse, reg.postToolUse))
	}
	if reg.preCompact != nil {
		cmd.AddCommand(leaf("pre-compact", eventPreCompact, reg.preCompact))
	}
	if reg.sessionStart != nil {
		cmd.AddCommand(leaf("session-start", eventSessionStart, reg.sessionStart))
	}

	return cmd
}

// leaf builds one event's cobra.Command: read stdin fully, decode into P,
// dispatch to h inside FailOpen, write at most one Response line. RunE
// always returns nil — FailOpen's contract — so a stdin-decode error, a
// panicking handler, or a handler-returned error never surfaces as a
// non-zero exit.
func leaf[P any](use, ccEvent string, h Handler[P]) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: "Handle the Claude Code " + ccEvent + " hook event",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return FailOpen(func() error {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}

				var payload P
				if err := jsonutil.Unmarshal(data, &payload); err != nil {
					return err
				}

				if p, ok := any(&payload).(interface{ commonPtr() *Common }); ok {
					p.commonPtr().ProjectDir = os.Getenv("CLAUDE_PROJECT_DIR")
				}

				resp := h(cmd.Context(), bytes.NewReader(data), payload)
				return resp.write(cmd.OutOrStdout(), ccEvent)
			})
		},
	}
}
