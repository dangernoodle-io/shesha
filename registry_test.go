package shesha

import (
	"context"
	"testing"

	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/stretchr/testify/require"
)

// TestRegistryFinalizeIdempotent proves finalize registers each pending
// tool exactly once even when called twice (the MC-43 guard Run+Connect,
// or Run+Run, both rely on).
func TestRegistryFinalizeIdempotent(t *testing.T) {
	reg := &registry{}

	calls := 0
	reg.add(pendingTool{name: "t1", register: func(_ *mcpx.Server) { calls++ }})

	reg.finalize(nil)
	reg.finalize(nil)

	require.Equal(t, 1, calls, "finalize must not re-register a pending tool on a second call")
	require.True(t, reg.started)
}

// TestRegistryByGroup proves finalize buckets registered tool names by
// group, including the "" (ungrouped) bucket, and never touches a group a
// tool wasn't assigned to.
func TestRegistryByGroup(t *testing.T) {
	reg := &registry{}
	reg.add(pendingTool{name: "a", group: "g1", register: func(_ *mcpx.Server) {}})
	reg.add(pendingTool{name: "b", group: "g1", register: func(_ *mcpx.Server) {}})
	reg.add(pendingTool{name: "c", group: "g2", register: func(_ *mcpx.Server) {}})
	reg.add(pendingTool{name: "d", register: func(_ *mcpx.Server) {}})

	reg.finalize(nil)

	require.ElementsMatch(t, []string{"a", "b"}, reg.byGroup["g1"])
	require.ElementsMatch(t, []string{"c"}, reg.byGroup["g2"])
	require.ElementsMatch(t, []string{"d"}, reg.byGroup[""])
	require.Len(t, reg.byGroup, 3, "no extra/ghost group buckets")
}

type groupedCap struct{}

func (groupedCap) Attach(r *Registrar) error {
	AddTool(r, &mcpx.Tool{Name: "grouped-a", Description: "a"}, ReadOnly,
		func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		}, Group("alpha"))
	AddTool(r, &mcpx.Tool{Name: "grouped-b", Description: "b"}, Write,
		func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		}, Group("alpha"))
	AddTool(r, &mcpx.Tool{Name: "ungrouped", Description: "c"}, ReadOnly,
		func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		})
	return nil
}

// TestDeferredRegistrationPreFinalize proves registration is genuinely
// deferred: right after New (capabilities have already called AddTool),
// the registry holds pending entries but has not started, i.e. nothing has
// registered against the live server yet. This is asserted at the registry
// (the actual seam MC-43 introduces) rather than through the wire protocol,
// because connecting a session to observe tools/list would itself trigger
// finalize — the very thing under test.
func TestDeferredRegistrationPreFinalize(t *testing.T) {
	app, err := New(Info{Name: "deferred", Version: "0.0.1"}, generic.New(), groupedCap{})
	require.NoError(t, err)

	require.False(t, app.reg.started, "registration must not have run before finalize")
	require.Len(t, app.reg.pending, 3, "AddTool must capture pending tools without registering them")
	require.Empty(t, app.reg.byGroup, "byGroup is only populated by finalize")

	app.finalize()

	require.True(t, app.reg.started)
	require.ElementsMatch(t, []string{"grouped-a", "grouped-b"}, app.reg.byGroup["alpha"])
	require.ElementsMatch(t, []string{"ungrouped"}, app.reg.byGroup[""])
}

// TestAppFinalizeIdempotent proves App.finalize itself is guarded: calling
// it directly more than once (the same path Run/Connect/HTTPHandler share)
// does not re-register tools against the live mcpx.Server, which would
// otherwise panic on a duplicate tool name.
func TestAppFinalizeIdempotent(t *testing.T) {
	app, err := New(Info{Name: "idempotent", Version: "0.0.1"}, generic.New(), groupedCap{})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		app.finalize()
		app.finalize()
		app.finalize()
	})
}

// TestRegistryFinalizeGateExcludesBlockedFromByGroup proves the MC-44 gate
// predicate in finalize is a true hard block: a gated-off tool's register
// closure is never invoked (never reaches the live server) and its name
// never lands in byGroup, so a startup-blocked tool can't later be
// resurrected by MC-45's runtime Unlock (which will only ever see byGroup).
func TestRegistryFinalizeGateExcludesBlockedFromByGroup(t *testing.T) {
	registered := map[string]bool{}

	reg := &registry{
		gate: gateState{
			readOnly:      true,
			blockedGroups: map[string]bool{"x": true},
		},
	}
	reg.add(pendingTool{name: "ro", group: "a", risk: ReadOnly, register: func(_ *mcpx.Server) { registered["ro"] = true }})
	reg.add(pendingTool{name: "write", group: "a", risk: Write, register: func(_ *mcpx.Server) { registered["write"] = true }})
	reg.add(pendingTool{name: "ro-x", group: "x", risk: ReadOnly, register: func(_ *mcpx.Server) { registered["ro-x"] = true }})

	reg.finalize(nil)

	require.True(t, registered["ro"], "an allowed tool must still register")
	require.False(t, registered["write"], "ReadOnlyMode must gate off a non-ReadOnly tool before register runs")
	require.False(t, registered["ro-x"], "BlockGroups must gate off a blocked-group tool before register runs")

	require.Equal(t, []string{"ro"}, reg.byGroup["a"])
	require.Empty(t, reg.byGroup["x"], "byGroup must not track a gated-off tool")
}

// TestRegistryApplyGateAfterStartedErrors proves applyGate/blockGroups are
// pre-start-only: once finalize has run, mutating the gate can no longer
// affect the already-registered set, so both return an error instead of a
// silent no-op.
func TestRegistryApplyGateAfterStartedErrors(t *testing.T) {
	reg := &registry{}
	reg.finalize(nil)

	err := reg.applyGate(ReadOnlyMode())
	require.Error(t, err)

	err = reg.blockGroups("x")
	require.Error(t, err)
}

// TestRegistryApplyGateBeforeStarted proves applyGate/blockGroups succeed
// pre-finalize and actually mutate reg.gate.
func TestRegistryApplyGateBeforeStarted(t *testing.T) {
	reg := &registry{}

	require.NoError(t, reg.applyGate(ReadOnlyMode()))
	require.True(t, reg.gate.readOnly)

	require.NoError(t, reg.blockGroups("x", "y"))
	require.True(t, reg.gate.blockedGroups["x"])
	require.True(t, reg.gate.blockedGroups["y"])
}

// TestRegistryShouldRegister table-drives the MC-45 shouldRegister
// predicate: a tool must clear both the permanent gate axis and the runtime
// lock axis to be eligible.
func TestRegistryShouldRegister(t *testing.T) {
	cases := []struct {
		name string
		reg  *registry
		t    pendingTool
		want bool
	}{
		{
			name: "no gate, no lock",
			reg:  &registry{},
			t:    pendingTool{name: "t", group: "g", risk: ReadOnly},
			want: true,
		},
		{
			name: "readOnly gate blocks Write",
			reg:  &registry{gate: gateState{readOnly: true}},
			t:    pendingTool{name: "t", group: "g", risk: Write},
			want: false,
		},
		{
			name: "blockedGroups blocks its group",
			reg:  &registry{gate: gateState{blockedGroups: map[string]bool{"g": true}}},
			t:    pendingTool{name: "t", group: "g", risk: ReadOnly},
			want: false,
		},
		{
			name: "locked group blocks regardless of gate",
			reg:  &registry{locked: map[string]bool{"g": true}},
			t:    pendingTool{name: "t", group: "g", risk: ReadOnly},
			want: false,
		},
		{
			name: "locked ungrouped group name does not affect other group",
			reg:  &registry{locked: map[string]bool{"other": true}},
			t:    pendingTool{name: "t", group: "g", risk: ReadOnly},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.reg.shouldRegister(tc.t))
		})
	}
}

// TestRegistryLockGroupHardBlockedErrors proves lockGroup refuses to touch a
// group the startup gate already hard-blocks: a hard block always wins and
// can't be runtime-toggled.
func TestRegistryLockGroupHardBlockedErrors(t *testing.T) {
	reg := &registry{gate: gateState{blockedGroups: map[string]bool{"x": true}}}

	err := reg.lockGroup(nil, "x")
	require.Error(t, err)
	require.False(t, reg.locked["x"], "a rejected Lock must not mutate locked")
}

// TestRegistryUnlockGroupHardBlockedErrors is unlockGroup's mirror of
// TestRegistryLockGroupHardBlockedErrors.
func TestRegistryUnlockGroupHardBlockedErrors(t *testing.T) {
	reg := &registry{gate: gateState{blockedGroups: map[string]bool{"x": true}}}

	err := reg.unlockGroup(nil, "x")
	require.Error(t, err)
}

// TestRegistryLockGroupPreStartIdempotent proves lockGroup succeeds
// pre-finalize (srv is never touched, so a nil srv is safe), sets
// reg.locked, and is a safe no-op when called a second time.
func TestRegistryLockGroupPreStartIdempotent(t *testing.T) {
	reg := &registry{}

	require.NoError(t, reg.lockGroup(nil, "g"))
	require.True(t, reg.locked["g"])

	require.NotPanics(t, func() {
		require.NoError(t, reg.lockGroup(nil, "g"))
	})
	require.True(t, reg.locked["g"])
}

// TestRegistryLockGroupPreStartSkipsAtFinalize proves a pre-start Lock makes
// finalize skip the group's tools while keeping them in pending (so a later
// Unlock can still register them).
func TestRegistryLockGroupPreStartSkipsAtFinalize(t *testing.T) {
	reg := &registry{}
	require.NoError(t, reg.lockGroup(nil, "hw"))

	registered := map[string]bool{}
	reg.add(pendingTool{name: "hw-tool", group: "hw", register: func(_ *mcpx.Server) { registered["hw-tool"] = true }})
	reg.add(pendingTool{name: "plain", group: "", register: func(_ *mcpx.Server) { registered["plain"] = true }})

	reg.finalize(nil)

	require.False(t, registered["hw-tool"], "a pre-start-locked group's tool must not register at finalize")
	require.True(t, registered["plain"])
	require.Empty(t, reg.byGroup["hw"])
	require.Equal(t, []string{"plain"}, reg.byGroup[""])
	require.Len(t, reg.pending, 2, "a locked (not gate-blocked) tool must stay in pending for a later Unlock")
}

// TestRegistryLockGroupPostStartRemovesAndClearsByGroup proves a post-start
// Lock genuinely unregisters the group's tools against the live server (via
// mcpx.Server.RemoveTools) and clears the byGroup bookkeeping.
func TestRegistryLockGroupPostStartRemovesAndClearsByGroup(t *testing.T) {
	srv := mcpx.NewServer(mcpx.Implementation{Name: "lock-post-start", Version: "0.0.1"}, "", 0)
	mcpx.AddTool(srv, &mcpx.Tool{Name: "hw-tool"}, func(_ context.Context, _ *mcpx.CallToolRequest, _ struct{}) (*mcpx.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})

	reg := &registry{started: true, byGroup: map[string][]string{"hw": {"hw-tool"}}}

	require.NoError(t, reg.lockGroup(srv, "hw"))
	require.True(t, reg.locked["hw"])
	require.Empty(t, reg.byGroup["hw"], "byGroup must be cleared for a locked group")
}

// TestRegistryLockGroupPostStartEmptyGroupIsSafe proves lockGroup on a group
// with no currently-registered tools (e.g. every tool in it was already
// gate-blocked at finalize) does not call srv.RemoveTools at all — a nil
// srv is used here to prove that.
func TestRegistryLockGroupPostStartEmptyGroupIsSafe(t *testing.T) {
	reg := &registry{started: true}

	require.NotPanics(t, func() {
		require.NoError(t, reg.lockGroup(nil, "empty"))
	})
	require.True(t, reg.locked["empty"])
}

// TestRegistryUnlockGroupPostStartReregistersOnlyAllowed proves unlockGroup,
// once the registry has started, registers only the locked group's pending
// tools that shouldRegister still allows: a gate-blocked tool (Write under
// ReadOnlyMode) is never resurrected, but an allowed tool in the same group
// is registered and recorded in byGroup.
func TestRegistryUnlockGroupPostStartReregistersOnlyAllowed(t *testing.T) {
	registered := map[string]bool{}

	reg := &registry{
		started: true,
		byGroup: map[string][]string{},
		gate:    gateState{readOnly: true},
	}
	reg.add(pendingTool{name: "ro-hw", group: "hw", risk: ReadOnly, register: func(_ *mcpx.Server) { registered["ro-hw"] = true }})
	reg.add(pendingTool{name: "write-hw", group: "hw", risk: Write, register: func(_ *mcpx.Server) { registered["write-hw"] = true }})
	reg.add(pendingTool{name: "ro-other", group: "other", risk: ReadOnly, register: func(_ *mcpx.Server) { registered["ro-other"] = true }})

	require.NoError(t, reg.lockGroup(nil, "hw"))
	require.NoError(t, reg.unlockGroup(nil, "hw"))

	require.True(t, registered["ro-hw"], "an allowed tool in the unlocked group must register")
	require.False(t, registered["write-hw"], "ReadOnlyMode must still block a Write tool after Unlock")
	require.False(t, registered["ro-other"], "Unlock must not touch a different group")

	require.Equal(t, []string{"ro-hw"}, reg.byGroup["hw"])
	require.False(t, reg.locked["hw"])
}

// TestRegistryUnlockGroupNeverLockedIsNoop proves unlockGroup on a group
// that was never locked (the common case: Unlock without a prior Lock) is a
// safe no-op — no pending tool is touched.
func TestRegistryUnlockGroupNeverLockedIsNoop(t *testing.T) {
	registered := map[string]bool{}
	reg := &registry{started: true, byGroup: map[string][]string{}}
	reg.add(pendingTool{name: "t", group: "g", register: func(_ *mcpx.Server) { registered["t"] = true }})

	require.NoError(t, reg.unlockGroup(nil, "g"))
	require.False(t, registered["t"])
}

// TestRegistryUnlockGroupPreStartThenFinalizeRegisters proves a pre-start
// Lock followed by a pre-start Unlock leaves the group eligible again: the
// subsequent finalize registers its tools normally.
func TestRegistryUnlockGroupPreStartThenFinalizeRegisters(t *testing.T) {
	reg := &registry{}
	require.NoError(t, reg.lockGroup(nil, "hw"))
	require.NoError(t, reg.unlockGroup(nil, "hw"))
	require.False(t, reg.locked["hw"])

	registered := map[string]bool{}
	reg.add(pendingTool{name: "hw-tool", group: "hw", register: func(_ *mcpx.Server) { registered["hw-tool"] = true }})

	reg.finalize(nil)

	require.True(t, registered["hw-tool"], "unlocking pre-start must make the group register normally at finalize")
	require.Equal(t, []string{"hw-tool"}, reg.byGroup["hw"])
}

// TestRegistryUnlockGroupDoubleUnlockIsIdempotent proves calling
// unlockGroup twice in a row (after a real Lock) does not double-register
// or panic.
func TestRegistryUnlockGroupDoubleUnlockIsIdempotent(t *testing.T) {
	calls := 0
	reg := &registry{started: true, byGroup: map[string][]string{}}
	reg.add(pendingTool{name: "t", group: "g", register: func(_ *mcpx.Server) { calls++ }})

	require.NoError(t, reg.lockGroup(nil, "g"))
	require.NoError(t, reg.unlockGroup(nil, "g"))

	require.NotPanics(t, func() {
		require.NoError(t, reg.unlockGroup(nil, "g"))
	})
	require.Equal(t, 1, calls, "a second Unlock must not re-register")
}
