package identity_test

import (
	"regexp"
	"testing"

	"github.com/dangernoodle-io/shesha/identity"
	"github.com/stretchr/testify/assert"
)

func TestEnv_Set(t *testing.T) {
	t.Setenv("SHESHA_TEST_ENV_SET", "from-env")

	src := identity.Env("SHESHA_TEST_ENV_SET")

	assert.Equal(t, "from-env", src())
}

func TestEnv_Unset(t *testing.T) {
	t.Setenv("SHESHA_TEST_ENV_UNSET", "")

	src := identity.Env("SHESHA_TEST_ENV_UNSET")

	assert.Empty(t, src())
}

func TestStatic_NonEmpty(t *testing.T) {
	src := identity.Static("fixed-id")

	assert.Equal(t, "fixed-id", src())
}

func TestStatic_Empty(t *testing.T) {
	src := identity.Static("")

	assert.Empty(t, src())
}

func TestUUIDFunc_YieldsInjectedValue(t *testing.T) {
	src := identity.UUIDFunc(func() string { return "fixed" })

	assert.Equal(t, "fixed", src())
}

var uuidFormat = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestNewV4_Format(t *testing.T) {
	got := identity.NewV4()

	assert.Regexp(t, uuidFormat, got)
	assert.Equal(t, byte('4'), got[14], "version nibble must be 4")
	assert.Contains(t, "89ab", string(got[19]), "variant nibble must be 8/9/a/b")
}

func TestNewV4_NonConstant(t *testing.T) {
	a := identity.NewV4()
	b := identity.NewV4()

	assert.NotEqual(t, a, b)
}

func TestUUID_Format(t *testing.T) {
	src := identity.UUID()
	got := src()

	assert.Regexp(t, uuidFormat, got)
	assert.Equal(t, byte('4'), got[14], "version nibble must be 4")
	assert.Contains(t, "89ab", string(got[19]), "variant nibble must be 8/9/a/b")
}

func TestUUID_NonConstant(t *testing.T) {
	src := identity.UUID()

	a := src()
	b := src()

	assert.NotEqual(t, a, b)
}

func TestResolve_EmptySlice(t *testing.T) {
	assert.Empty(t, identity.Resolve())
}

func TestResolve_AllEmptySources(t *testing.T) {
	got := identity.Resolve(identity.Static(""), identity.Static(""))

	assert.Empty(t, got)
}

func TestResolve_FirstNonEmptyWins(t *testing.T) {
	got := identity.Resolve(
		identity.Static("override"),
		identity.Static("later"),
	)

	assert.Equal(t, "override", got)
}

func TestResolve_NilSourceSkipped(t *testing.T) {
	got := identity.Resolve(
		identity.Static(""),
		nil,
		identity.Static("after-nil"),
	)

	assert.Equal(t, "after-nil", got)
}

func TestResolve_FourTierChain(t *testing.T) {
	build := func(override, static, envProbe, uuid string) []identity.Source {
		return []identity.Source{
			identity.Static(override),
			identity.Static(static),
			identity.Static(envProbe),
			identity.UUIDFunc(func() string { return uuid }),
		}
	}

	t.Run("override tier wins", func(t *testing.T) {
		got := identity.Resolve(build("override", "static", "env", "uuid")...)
		assert.Equal(t, "override", got)
	})

	t.Run("static tier wins when no override", func(t *testing.T) {
		got := identity.Resolve(build("", "static", "env", "uuid")...)
		assert.Equal(t, "static", got)
	})

	t.Run("env-probe tier wins when no override or static", func(t *testing.T) {
		got := identity.Resolve(build("", "", "env", "uuid")...)
		assert.Equal(t, "env", got)
	})

	t.Run("uuid terminal fallback wins when nothing else resolves", func(t *testing.T) {
		got := identity.Resolve(build("", "", "", "uuid")...)
		assert.Equal(t, "uuid", got)
	})

	t.Run("empty when no uuid fallback and nothing resolves", func(t *testing.T) {
		got := identity.Resolve(build("", "", "", "")[:3]...)
		assert.Empty(t, got)
	})
}
