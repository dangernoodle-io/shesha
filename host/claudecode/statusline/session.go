package statusline

import "github.com/dangernoodle-io/shesha/identity"

// Resolve resolves the session identity a statusline invocation should
// filter its rendered data by, generalizing pogopin's BR-76 precedence to
// any consumer via appPrefix (e.g. "OUROBOROS", "POGOPIN"):
//
//  1. <appPrefix>_SESSION_ID env var — host-agnostic override, set by the
//     consumer's own long-running process or a caller that wants to pin a
//     session explicitly.
//  2. payload.SessionID — the session_id Claude Code passed on the
//     statusline stdin contract for this invocation.
//  3. CLAUDE_CODE_SESSION_ID env var — Claude Code's own session env var,
//     when the stdin contract didn't carry one.
//  4. "" — no session could be resolved; consumers should render nothing
//     rather than fall back to an unfiltered view.
func Resolve(payload Payload, appPrefix string) string {
	var sources []identity.Source

	if appPrefix != "" {
		sources = append(sources, identity.Env(appPrefix+"_SESSION_ID"))
	}

	sources = append(sources,
		identity.Static(payload.SessionID),
		identity.Env("CLAUDE_CODE_SESSION_ID"),
	)

	return identity.Resolve(sources...)
}
