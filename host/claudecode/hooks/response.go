package hooks

import (
	"io"

	"github.com/dangernoodle-io/shesha/jsonutil"
)

// Response is the fail-open, event-agnostic result a Handler returns. The
// zero value is silent allow: nothing is written to stdout. At most one of
// Block, AdditionalContext, and PlainText should be set per Response (see
// write's precedence); SystemMessage may additionally be set alongside
// Block or AdditionalContext.
type Response struct {
	// Block, when non-empty, tells Claude Code to block with this reason
	// shown to the model. Valid on any blocking-capable event (Stop,
	// SubagentStop, UserPromptSubmit, PreToolUse, PostToolUse).
	Block string

	// AdditionalContext, when non-empty, emits a structurally valid
	// hookSpecificOutput.additionalContext shape for whatever event the
	// handler is registered against — write does not restrict which
	// events may set it. Whether Claude Code actually honors it is
	// event-dependent: it's documented on UserPromptSubmit and
	// SessionStart, and may be accepted by other events too; the consumer
	// is responsible for only setting it on events CC honors it for.
	AdditionalContext string

	// PlainText, when non-empty (and Block/AdditionalContext are empty),
	// is written to stdout verbatim with no JSON wrapper — Claude Code's
	// plain-text convention, documented on UserPromptSubmit/SessionStart;
	// other events may or may not treat bare stdout the same way.
	PlainText string

	// SystemMessage, when non-empty, is shown to the user as a warning
	// alongside whichever other field is set.
	SystemMessage string
}

// blockOutput is the CC JSON shape for a blocking decision.
type blockOutput struct {
	Decision      string `json:"decision"`
	Reason        string `json:"reason"`
	SystemMessage string `json:"systemMessage,omitempty"`
}

// hookSpecificOutput is the CC JSON shape nested under "hookSpecificOutput"
// for additional-context injection.
type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// additionalContextOutput is the CC JSON shape for additional-context
// injection.
type additionalContextOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
	SystemMessage      string             `json:"systemMessage,omitempty"`
}

// systemMessageOutput is the CC JSON shape for a systemMessage with no
// other Response field set.
type systemMessageOutput struct {
	SystemMessage string `json:"systemMessage"`
}

// write marshals r as the Claude Code hook-output JSON (or bare stdout
// text) appropriate to ccEventName and writes it to out, terminated with a
// trailing newline. Precedence: Block > AdditionalContext > PlainText >
// SystemMessage-alone > nothing (the zero value writes nothing at all).
//
// PlainText cannot carry a JSON systemMessage sibling — bare text and
// structured JSON are mutually exclusive on the wire — so when both
// PlainText and SystemMessage are set, SystemMessage wins (emitted as
// {"systemMessage":...}) and PlainText is dropped.
func (r Response) write(out io.Writer, ccEventName string) error {
	switch {
	case r.Block != "":
		return writeJSONLine(out, blockOutput{
			Decision:      "block",
			Reason:        r.Block,
			SystemMessage: r.SystemMessage,
		})

	case r.AdditionalContext != "":
		return writeJSONLine(out, additionalContextOutput{
			HookSpecificOutput: hookSpecificOutput{
				HookEventName:     ccEventName,
				AdditionalContext: r.AdditionalContext,
			},
			SystemMessage: r.SystemMessage,
		})

	case r.PlainText != "":
		if r.SystemMessage != "" {
			return writeJSONLine(out, systemMessageOutput{SystemMessage: r.SystemMessage})
		}
		_, err := io.WriteString(out, r.PlainText+"\n")
		return err

	case r.SystemMessage != "":
		return writeJSONLine(out, systemMessageOutput{SystemMessage: r.SystemMessage})

	default:
		return nil
	}
}

// writeJSONLine marshals v via jsonutil.Marshal and writes it to out
// followed by a trailing newline.
func writeJSONLine(out io.Writer, v any) error {
	b, err := jsonutil.Marshal(v)
	if err != nil {
		return err
	}

	_, err = out.Write(append(b, '\n'))
	return err
}
