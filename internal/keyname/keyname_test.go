package keyname_test

import (
	"testing"

	"github.com/dangernoodle-io/shesha/internal/keyname"
	"github.com/stretchr/testify/assert"
)

func TestUpper(t *testing.T) {
	cases := map[string]string{
		"pogopin":     "POGOPIN",
		"my-cool-app": "MY_COOL_APP",
		"db.path":     "DB_PATH",
		"app.2":       "APP_2",
		"a--b..c":     "A_B_C",
		"":            "",
	}

	for in, want := range cases {
		assert.Equal(t, want, keyname.Upper(in), "Upper(%q)", in)
	}
}
