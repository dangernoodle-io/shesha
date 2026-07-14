package shesha_test

import (
	"context"
	"testing"

	"github.com/dangernoodle-io/shesha"
	"github.com/dangernoodle-io/shesha/host/generic"
	"github.com/dangernoodle-io/shesha/mcpx"
	"github.com/dangernoodle-io/shesha/testkit"
	"github.com/stretchr/testify/require"
)

type gateIn struct{}

type gateOut struct{}

func gateHandler(_ context.Context, _ *mcpx.CallToolRequest, _ gateIn) (*mcpx.CallToolResult, gateOut, error) {
	return nil, gateOut{}, nil
}

// riskGateCap registers one ungrouped tool per Risk value.
type riskGateCap struct{}

func (riskGateCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{Name: "ro", Description: "d"}, shesha.ReadOnly, gateHandler)
	shesha.AddTool(r, &mcpx.Tool{Name: "write", Description: "d"}, shesha.Write, gateHandler)
	shesha.AddTool(r, &mcpx.Tool{Name: "destructive", Description: "d"}, shesha.Destructive, gateHandler)
	return nil
}

// groupGateCap registers a ReadOnly tool in group "x", a ReadOnly tool in
// group "y", and an ungrouped ReadOnly tool.
type groupGateCap struct{}

func (groupGateCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{Name: "x-tool", Description: "d"}, shesha.ReadOnly, gateHandler, shesha.Group("x"))
	shesha.AddTool(r, &mcpx.Tool{Name: "y-tool", Description: "d"}, shesha.ReadOnly, gateHandler, shesha.Group("y"))
	shesha.AddTool(r, &mcpx.Tool{Name: "ungrouped", Description: "d"}, shesha.ReadOnly, gateHandler)
	return nil
}

// mixedGateCap combines the risk and group axes for the combined-gate test.
type mixedGateCap struct{}

func (mixedGateCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{Name: "ro-x", Description: "d"}, shesha.ReadOnly, gateHandler, shesha.Group("x"))
	shesha.AddTool(r, &mcpx.Tool{Name: "ro-y", Description: "d"}, shesha.ReadOnly, gateHandler, shesha.Group("y"))
	shesha.AddTool(r, &mcpx.Tool{Name: "write-y", Description: "d"}, shesha.Write, gateHandler, shesha.Group("y"))
	return nil
}

// TestGateReadOnlyModeBlocksNonReadOnly proves ReadOnlyMode hard-blocks
// every non-ReadOnly tool at startup: only the ReadOnly tool is advertised
// in tools/list once the app connects.
func TestGateReadOnlyModeBlocksNonReadOnly(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "gate-ro", Version: "0.0.1"}, generic.New(), riskGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.Gate(shesha.ReadOnlyMode()))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ro")
}

// TestBlockGroupsExcludesNamedGroup proves BlockGroups hard-blocks every
// tool tagged with a blocked group while leaving other groups/ungrouped
// tools advertised.
func TestBlockGroupsExcludesNamedGroup(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "gate-group", Version: "0.0.1"}, generic.New(), groupGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.BlockGroups("x"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "y-tool", "ungrouped")
}

// TestGateReadOnlyAndBlockGroupsCombine proves the risk axis and the group
// axis intersect: a tool must pass both to be advertised.
func TestGateReadOnlyAndBlockGroupsCombine(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "gate-combo", Version: "0.0.1"}, generic.New(), mixedGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.Gate(shesha.ReadOnlyMode()))
	require.NoError(t, app.BlockGroups("y"))

	h := testkit.New(t, app)
	// ro-x: ReadOnly, group x (not blocked) -> advertised.
	// ro-y: ReadOnly, but group y (blocked) -> excluded.
	// write-y: Write (blocked by ReadOnlyMode) and group y (blocked) -> excluded.
	testkit.AssertToolSet(t, h, "ro-x")
}

// TestNoGateAdvertisesEverything is a regression guard: an App that never
// calls Gate/BlockGroups preserves MC-43's register-everything behavior.
func TestNoGateAdvertisesEverything(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "gate-none", Version: "0.0.1"}, generic.New(), riskGateCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ro", "write", "destructive")
}

// TestGateAfterConnectErrors proves Gate/BlockGroups are pre-start-only:
// calling either after the App has already finalized registration (via
// Connect) returns an error and does not change the already-registered
// tool set.
func TestGateAfterConnectErrors(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "gate-late", Version: "0.0.1"}, generic.New(), riskGateCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ro", "write", "destructive")

	require.Error(t, app.Gate(shesha.ReadOnlyMode()))
	require.Error(t, app.BlockGroups("whatever"))

	// The already-registered set must be unchanged by the rejected calls.
	testkit.AssertToolSet(t, h, "ro", "write", "destructive")
}

// namedToolCap registers two ungrouped ReadOnly tools by name, for the
// MC-49 BlockTools tests.
type namedToolCap struct{}

func (namedToolCap) Attach(r *shesha.Registrar) error {
	shesha.AddTool(r, &mcpx.Tool{Name: "x", Description: "d"}, shesha.ReadOnly, gateHandler)
	shesha.AddTool(r, &mcpx.Tool{Name: "y", Description: "d"}, shesha.ReadOnly, gateHandler)
	return nil
}

// TestBlockToolsExcludesNamedTool proves BlockTools hard-blocks a tool by
// name while leaving other tools advertised.
func TestBlockToolsExcludesNamedTool(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "block-tools", Version: "0.0.1"}, generic.New(), namedToolCap{})
	require.NoError(t, err)

	require.NoError(t, app.BlockTools("x"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "y")
}

// TestBlockToolsExcludesFromByGroup proves a name-blocked tool is excluded
// from finalize's byGroup bookkeeping too, mirroring
// TestBlockGroupsExcludesNamedGroup: it never registers, so a group
// containing it never picks it up in byGroup either.
func TestBlockToolsExcludesFromByGroup(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "block-tools-group", Version: "0.0.1"}, generic.New(), groupGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.BlockTools("x-tool"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "y-tool", "ungrouped")
}

// TestBlockToolsHardBlockWinsOverUnlock proves a name-blocked tool that is
// also a member of a group cannot be resurrected by MC-45's Unlock: the
// name block is permanent, same invariant BlockGroups already gives at the
// group level.
func TestBlockToolsHardBlockWinsOverUnlock(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "block-tools-unlock", Version: "0.0.1"}, generic.New(), hwGroupCap{})
	require.NoError(t, err)

	require.NoError(t, app.BlockTools("hw-tool"))

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "ungrouped-tool")

	require.NoError(t, app.Lock("hw"))
	require.NoError(t, app.Unlock("hw"))
	testkit.AssertToolSet(t, h, "ungrouped-tool")
}

// TestBlockToolsAfterConnectErrors proves BlockTools is pre-start-only: a
// call after the App has already finalized registration returns the
// documented error and does not change the already-registered tool set.
func TestBlockToolsAfterConnectErrors(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "block-tools-late", Version: "0.0.1"}, generic.New(), namedToolCap{})
	require.NoError(t, err)

	h := testkit.New(t, app)
	testkit.AssertToolSet(t, h, "x", "y")

	require.Error(t, app.BlockTools("x"))

	testkit.AssertToolSet(t, h, "x", "y")
}

// TestBlockToolsCombinesWithBlockGroupsAndReadOnly proves all three gate
// axes (name, group, risk) compose: a tool blocked by any single axis stays
// out, and a tool that clears all three still advertises.
func TestBlockToolsCombinesWithBlockGroupsAndReadOnly(t *testing.T) {
	app, err := shesha.New(shesha.Info{Name: "block-tools-combo", Version: "0.0.1"}, generic.New(), mixedGateCap{})
	require.NoError(t, err)

	require.NoError(t, app.Gate(shesha.ReadOnlyMode()))
	require.NoError(t, app.BlockGroups("y"))
	require.NoError(t, app.BlockTools("ro-x"))

	h := testkit.New(t, app)
	// ro-x: ReadOnly, group x (not blocked), but name-blocked -> excluded.
	// ro-y: ReadOnly, but group y (blocked) -> excluded.
	// write-y: Write (blocked by ReadOnlyMode) and group y (blocked) -> excluded.
	testkit.AssertToolSet(t, h)
}
