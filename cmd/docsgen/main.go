// Command docsgen regenerates shesha's per-package README.md files (MC-11).
// Run via `make docs`; `make docs-check` runs it and then git-diffs the
// result to catch drift. All logic lives in internal/docsgen.Run so it's
// covered by that package's tests.
package main

import (
	"fmt"
	"os"

	"github.com/dangernoodle-io/shesha/internal/docsgen"
)

func main() {
	if err := docsgen.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "docsgen:", err)
		os.Exit(1)
	}
}
