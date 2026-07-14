// Package shesha is the composition root of a host-agnostic MCP server:
// compile-time plugin-style composition of Capabilities over a pluggable
// host.Adapter, built on the mcpx protocol seam.
package shesha

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dangernoodle-io/shesha/host"
	"github.com/dangernoodle-io/shesha/mcpx"
)

// Info identifies the server being composed.
type Info struct {
	Name    string
	Version string

	// Instructions is optional MCP server instructions advertised to
	// clients (tool-selection guidance); empty = none.
	Instructions string

	// KeepAlive configures go-sdk's built-in server keepalive: 0 disables
	// keepalive (default); >0 pings the client on that interval and closes
	// the session on ping-failure.
	KeepAlive time.Duration
}

// Risk classifies a tool's blast radius, from a client's perspective, for
// gating and annotation purposes. Every AddTool call requires one
// (fail-closed): a caller can't omit risk classification and accidentally
// ship a write/destructive tool unannotated.
type Risk int

const (
	// ReadOnly tools only observe state; they never mutate anything a
	// client cares about.
	ReadOnly Risk = iota
	// Write tools mutate state, but the mutation is reversible/benign
	// enough not to warrant a destructive warning.
	Write
	// Destructive tools perform an irreversible or high-blast-radius
	// mutation (data loss, external side effects that can't be undone).
	Destructive
)

// ToolOption configures optional per-tool metadata at AddTool time.
type ToolOption interface {
	applyTool(*toolMeta)
}

// toolMeta is the mutable state ToolOption values apply to.
type toolMeta struct {
	group string
}

// groupOption is the ToolOption Group returns.
type groupOption string

func (g groupOption) applyTool(m *toolMeta) {
	m.group = string(g)
}

// Group tags a tool with an arbitrary consumer-defined group name, recorded
// in the registry's byGroup bookkeeping post-registration. shesha imposes no
// meaning on the group string; a consumer's own gating (MC-44/MC-45) is what
// interprets it.
func Group(name string) ToolOption {
	return groupOption(name)
}

// GateOption configures the startup tool gate applied by App.Gate.
type GateOption func(*gateState)

// gateState holds the startup-gate predicates registry.finalize evaluates
// per pending tool. Its zero value blocks nothing, preserving MC-43's
// register-everything behavior for an App that never calls
// Gate/BlockGroups/BlockTools.
type gateState struct {
	readOnly      bool
	blockedGroups map[string]bool

	// blockedTools is MC-49's third gate axis: a per-tool-name hard block,
	// alongside the risk (readOnly) and group (blockedGroups) axes. Runtime
	// per-tool lock/unlock (mirroring MC-45's group-level Lock/Unlock) is
	// deliberately out of scope here; this is startup-only.
	blockedTools map[string]bool
}

// ReadOnlyMode is a GateOption that hard-blocks every pending tool whose
// risk is not ReadOnly: it is never registered against the underlying
// server, so it can't appear in tools/list or be called.
func ReadOnlyMode() GateOption {
	return func(g *gateState) {
		g.readOnly = true
	}
}

// Registrar is what a Capability's Attach method uses to register itself
// against the underlying server and inspect the target host. Capabilities
// register tools through the package-level AddTool, not against mcpx
// directly, so shesha owns a single tool-registration chokepoint.
type Registrar struct {
	server *mcpx.Server
	host   host.Adapter
	reg    *registry
}

// Host returns the host.Adapter the app is composed for.
func (r *Registrar) Host() host.Adapter {
	return r.host
}

// AddTool captures a typed tool handler through r for deferred registration
// against the underlying server. This is the only tool-registration
// chokepoint capabilities should use; MC-8 wraps every handler in a
// panic-recover here so a panicking tool surfaces as an IsError result
// instead of crashing the server process. Registration itself is deferred
// until the App's finalize runs (MC-43) so a later gate (MC-44) can filter
// which pending tools actually register before anything is exposed to a
// client.
//
// risk is required (fail-closed): a caller cannot omit a tool's risk
// classification. When t.Annotations is nil, AddTool derives it from risk
// via mcpx.RiskAnnotations; an explicitly-set Annotations is left untouched.
func AddTool[In, Out any](r *Registrar, t *mcpx.Tool, risk Risk, h mcpx.Handler[In, Out], opts ...ToolOption) {
	if t.Annotations == nil {
		t.Annotations = mcpx.RiskAnnotations(risk == ReadOnly, risk == Destructive)
	}

	meta := toolMeta{}
	for _, opt := range opts {
		opt.applyTool(&meta)
	}

	wrapped := func(ctx context.Context, req *mcpx.CallToolRequest, in In) (res *mcpx.CallToolResult, out Out, err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("tool %q panicked: %v", t.Name, p)
			}
		}()
		return h(ctx, req, in)
	}

	r.reg.add(pendingTool{
		name:  t.Name,
		group: meta.group,
		risk:  risk,
		register: func(s *mcpx.Server) {
			mcpx.AddTool(s, t, wrapped)
		},
	})
}

// pendingTool is one captured-but-not-yet-registered AddTool call. register
// closes over the tool's In/Out type parameters (erased here) and the
// panic-recover-wrapped handler; calling it performs the actual
// mcpx.AddTool registration against a live server.
type pendingTool struct {
	name, group string
	risk        Risk
	register    func(*mcpx.Server)
}

// registry accumulates pending tool registrations shared between a
// Registrar (during composition, via AddTool) and its App (at finalize
// time). Deferring registration out of AddTool is what lets a later gate
// (MC-44) filter which pending tools actually register before finalize ever
// touches the live server.
type registry struct {
	mu      sync.Mutex
	pending []pendingTool
	byGroup map[string][]string
	started bool
	gate    gateState

	// locked tracks MC-45's runtime, per-group soft lock: distinct from
	// gate, which is the permanent startup hard block. A locked group's
	// pending tools stay in pending (never discarded) so a later Unlock can
	// still register them.
	locked map[string]bool
}

// add appends t to the pending set. Safe for concurrent Attach calls.
func (reg *registry) add(t pendingTool) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.pending = append(reg.pending, t)
}

// applyGate applies opts to reg's gate state. Gate/BlockGroups are
// pre-start-only mechanisms (MC-44 is a startup gate, not the runtime
// Lock/Unlock MC-45 adds later): once finalize has run, mutating the gate
// would have no effect on already-registered tools, so applyGate errors
// instead of silently no-op'ing.
func (reg *registry) applyGate(opts ...GateOption) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.started {
		return fmt.Errorf("shesha: Gate must be called before Run/Connect/HTTPHandler")
	}

	for _, opt := range opts {
		opt(&reg.gate)
	}
	return nil
}

// blockGroups hard-blocks the named groups at startup, same pre-start-only
// contract as applyGate.
func (reg *registry) blockGroups(groups ...string) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.started {
		return fmt.Errorf("shesha: BlockGroups must be called before Run/Connect/HTTPHandler")
	}

	if reg.gate.blockedGroups == nil {
		reg.gate.blockedGroups = make(map[string]bool)
	}
	for _, g := range groups {
		reg.gate.blockedGroups[g] = true
	}
	return nil
}

// blockTools hard-blocks the named tools at startup, same pre-start-only
// contract as applyGate/blockGroups. This is MC-49's per-tool-name gate axis:
// a blocked tool is never registered against the underlying server and,
// because gateBlocked is what shouldRegister/unlockGroup both route through,
// it can't later be resurrected by MC-45's Unlock (hard-block-wins, same
// invariant BlockGroups already gives per-group).
func (reg *registry) blockTools(names ...string) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.started {
		return fmt.Errorf("shesha: BlockTools must be called before Run/Connect/HTTPHandler")
	}

	if reg.gate.blockedTools == nil {
		reg.gate.blockedTools = make(map[string]bool)
	}
	for _, name := range names {
		reg.gate.blockedTools[name] = true
	}
	return nil
}

// gateBlocked reports whether the startup gate (MC-44/MC-49) hard-blocks t:
// its risk fails ReadOnlyMode, its group is in gate.blockedGroups, or its
// name is in gate.blockedTools. This is a permanent block for the lifetime
// of the App — MC-45's Lock/Unlock never override it. Callers must hold
// reg.mu.
func (reg *registry) gateBlocked(t pendingTool) bool {
	return (reg.gate.readOnly && t.risk != ReadOnly) || reg.gate.blockedGroups[t.group] || reg.gate.blockedTools[t.name]
}

// shouldRegister reports whether t should be registered against the live
// server right now: it must clear both the permanent startup gate and the
// MC-45 runtime per-group lock. Callers must hold reg.mu.
func (reg *registry) shouldRegister(t pendingTool) bool {
	return !reg.gateBlocked(t) && !reg.locked[t.group]
}

// finalize registers every pending tool against srv exactly once, skipping
// any tool shouldRegister excludes: a gate (MC-44) hard block, or a
// pre-start MC-45 Lock on its group. A gated-off tool is never registered
// against srv and never recorded in byGroup, so it can't appear in
// tools/list, can't be called, and (per MC-44's hard-block contract) can't
// later be resurrected by MC-45's runtime Unlock. A locked-but-not-blocked
// tool is likewise skipped here but stays in pending, so Unlock can register
// it later. finalize is idempotent: a second call (e.g. Run then Connect, or
// Run called twice) is a guarded no-op rather than double-registering or
// panicking.
func (reg *registry) finalize(srv *mcpx.Server) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.started {
		return
	}
	reg.started = true

	if reg.byGroup == nil {
		reg.byGroup = make(map[string][]string)
	}

	for _, t := range reg.pending {
		if !reg.shouldRegister(t) {
			continue
		}
		t.register(srv)
		reg.byGroup[t.group] = append(reg.byGroup[t.group], t.name)
	}
}

// lockGroup implements MC-45's runtime App.Lock: it hard-disables group on
// the live server. If group is startup-gate-hard-blocked (MC-44), Lock
// returns an error rather than pretending to toggle an already-permanent
// block. Otherwise it marks group locked and, if the registry has already
// started, removes its currently-registered tools from srv (firing
// notifications/tools/list_changed via mcpx.Server.RemoveTools) and clears
// its byGroup bucket. Locking an already-locked group is a no-op. Safe to
// call before or after finalize.
func (reg *registry) lockGroup(srv *mcpx.Server, group string) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.gate.blockedGroups[group] {
		return fmt.Errorf("shesha: group %q is hard-blocked by the startup gate; Lock has no effect", group)
	}

	if reg.locked == nil {
		reg.locked = make(map[string]bool)
	}
	if reg.locked[group] {
		return nil
	}
	reg.locked[group] = true

	if reg.started {
		if names := reg.byGroup[group]; len(names) > 0 {
			srv.RemoveTools(names...)
		}
		delete(reg.byGroup, group)
	}
	return nil
}

// unlockGroup implements MC-45's runtime App.Unlock: it reverses lockGroup.
// If group is startup-gate-hard-blocked (MC-44), Unlock returns an error —
// a hard block always wins and cannot be runtime-toggled. Otherwise it
// clears group's lock and, if the registry has already started, registers
// every one of group's pending tools that shouldRegister still allows
// (i.e. not gate-blocked — this is what keeps a ReadOnlyMode-blocked Write
// tool from being resurrected) against srv, recording each in byGroup.
// Unlocking an already-unlocked group is a no-op. Safe to call before or
// after finalize.
func (reg *registry) unlockGroup(srv *mcpx.Server, group string) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if reg.gate.blockedGroups[group] {
		return fmt.Errorf("shesha: group %q is hard-blocked by the startup gate; Unlock has no effect", group)
	}

	if !reg.locked[group] {
		return nil
	}
	reg.locked[group] = false

	if reg.started {
		for _, t := range reg.pending {
			if t.group != group || reg.gateBlocked(t) {
				continue
			}
			t.register(srv)
			reg.byGroup[group] = append(reg.byGroup[group], t.name)
		}
	}
	return nil
}

// Capability is a self-contained unit of server functionality, attached to
// the composition root at build time.
type Capability interface {
	Attach(r *Registrar) error
}

// App is a fully composed MCP server, ready to run.
type App struct {
	server *mcpx.Server
	host   host.Adapter
	reg    *registry
}

// New composes an App from a host.Adapter and zero or more Capabilities,
// attaching each capability in order. Tool registration is deferred: no
// tool is registered against the underlying server until Run, Connect, or
// HTTPHandler calls finalize.
func New(info Info, h host.Adapter, caps ...Capability) (*App, error) {
	if h == nil {
		return nil, fmt.Errorf("shesha: host adapter must not be nil")
	}

	srv := mcpx.NewServer(mcpx.Implementation{Name: info.Name, Version: info.Version}, info.Instructions, info.KeepAlive)
	reg := &registry{}
	r := &Registrar{server: srv, host: h, reg: reg}

	for _, c := range caps {
		if err := c.Attach(r); err != nil {
			return nil, fmt.Errorf("shesha: attach capability: %w", err)
		}
	}

	return &App{server: srv, host: h, reg: reg}, nil
}

// finalize registers every pending tool against a's server exactly once,
// regardless of how many of Run/Connect/HTTPHandler trigger it or in what
// order.
func (a *App) finalize() {
	a.reg.finalize(a.server)
}

// Gate applies opts to a's startup tool gate (e.g. ReadOnlyMode). A gated-off
// tool is never registered against the underlying server — it is a hard
// block, not a hidden-but-reachable tool. Gate is pre-start-only: it must be
// called before Run, Connect, or HTTPHandler (whichever finalizes
// registration first); calling it afterward returns an error rather than
// silently having no effect.
func (a *App) Gate(opts ...GateOption) error {
	return a.reg.applyGate(opts...)
}

// BlockGroups hard-blocks every pending tool tagged (via the Group
// ToolOption) with one of groups: it is never registered against the
// underlying server. Same pre-start-only contract as Gate — must be called
// before Run/Connect/HTTPHandler, or it returns an error.
func (a *App) BlockGroups(groups ...string) error {
	return a.reg.blockGroups(groups...)
}

// BlockTools hard-blocks every pending tool named in names: it is never
// registered against the underlying server. Same pre-start-only contract as
// Gate/BlockGroups — must be called before Run/Connect/HTTPHandler, or it
// returns an error. This is a per-tool-name complement to BlockGroups; there
// is no runtime per-tool Lock/Unlock (mirroring MC-45's group-level ones) —
// that is deliberately out of scope here.
func (a *App) BlockTools(names ...string) error {
	return a.reg.blockTools(names...)
}

// Lock hard-disables group g on a's live server at runtime: any of g's
// currently-registered tools are removed (firing
// notifications/tools/list_changed) and g's pending tools are skipped for
// registration until a matching Unlock. Lock returns an error if g is
// hard-blocked by the startup gate (Gate/BlockGroups) — a hard block always
// wins and can't be runtime-toggled. Lock is idempotent and may be called
// before or after Run/Connect/HTTPHandler, which is what lets a consumer
// start a group locked (the lazy tier) and unlock it mid-session.
func (a *App) Lock(group string) error {
	return a.reg.lockGroup(a.server, group)
}

// Unlock reverses Lock: every one of group's pending tools not otherwise
// hard-blocked by the startup gate is registered against a's live server
// (firing notifications/tools/list_changed once the app has started). A
// tool the startup gate hard-blocks (e.g. a Write tool under ReadOnlyMode)
// is never resurrected by Unlock. Unlock returns an error if group is
// hard-blocked by the startup gate. Unlock is idempotent and may be called
// before or after Run/Connect/HTTPHandler.
func (a *App) Unlock(group string) error {
	return a.reg.unlockGroup(a.server, group)
}

// Run serves the app over its host's transport until the client disconnects
// or ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	a.finalize()
	return a.server.Run(ctx, a.host.Transport())
}

// Connect connects the app over t without blocking, for use by testkit and
// other in-process harnesses.
func (a *App) Connect(ctx context.Context, t mcpx.Transport) (*mcpx.Session, error) {
	a.finalize()
	return a.server.Connect(ctx, t)
}

// HTTPHandler exposes the composed server over streamable-HTTP for the
// consumer to mount. shesha is path-agnostic: the returned handler is bare
// and MCP-over-HTTP is entirely opt-in — the consumer decides whether and
// where to mount it. finalize runs here too (not just Run/Connect) because
// an HTTP-only consumer (see cli.ServerCmd's --http path and
// examples/http) never calls Run or Connect at all — without this,
// deferred registration would silently ship zero tools over HTTP,
// regressing MC-43's behavior-preservation goal.
func (a *App) HTTPHandler(opts ...mcpx.HTTPOption) http.Handler {
	a.finalize()
	return a.server.HTTPHandler(opts...)
}
