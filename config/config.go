// Package config provides a layered, typed config loader: defaults < file <
// env, in that fixed precedence regardless of the order options are passed
// to Load. Each layer only overwrites what it explicitly sets — a layer
// with nothing to say about a field leaves the prior layer's value intact.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dangernoodle-io/shesha/xdgpath"
)

// plan accumulates the options passed to Load before any layer is applied.
// It is intentionally non-generic — the typed merge happens inside Load[T],
// which reads plan's slots (not the call order) and applies them in the
// fixed precedence: defaults, file, env. Each layer has a single slot; if
// its option is passed more than once, the last one wins.
type plan struct {
	filePath string
	hasFile  bool

	envPrefix string
	hasEnv    bool
}

// Option configures a Load call. Options are order-independent: Load
// applies the file and env layers in a fixed precedence (defaults < file <
// env), not the order they're passed. Each layer has one slot; passing the
// same option kind more than once means the last one wins.
type Option func(*plan)

// WithFile overlays the JSON document at path onto the current value. Only
// fields present in the document are overwritten; a missing file is not an
// error (skipped silently), but malformed JSON is a hard error wrapping
// path.
func WithFile(path string) Option {
	return func(p *plan) {
		p.filePath = path
		p.hasFile = true
	}
}

// WithXDGFile is WithFile(xdgpath.ConfigFile(app, name)): it resolves name
// inside app's XDG config directory, then applies the same skip/error
// contract as WithFile.
func WithXDGFile(app, name string) Option {
	return WithFile(xdgpath.ConfigFile(app, name))
}

// Load builds a T starting from defaults and applying opts in fixed
// precedence: defaults < file < env, regardless of the order opts are
// passed. defaults is positional so T is inferred from it — options
// themselves carry no type parameter. With no opts, Load returns defaults
// unchanged and performs zero disk I/O and zero env reads.
//
// On error, Load returns the partially-built T alongside a non-nil error;
// callers should treat a non-nil error as fatal rather than trust the
// returned value.
func Load[T any](defaults T, opts ...Option) (T, error) {
	var p plan
	for _, o := range opts {
		o(&p)
	}

	out := defaults

	if p.hasFile {
		if err := loadFile(p.filePath, &out); err != nil {
			return out, fmt.Errorf("config: load file %s: %w", p.filePath, err)
		}
	}

	if p.hasEnv {
		if err := overlayEnv(p.envPrefix, &out); err != nil {
			return out, fmt.Errorf("config: load env: %w", err)
		}
	}

	return out, nil
}

// loadFile overlays the JSON document at path onto out. A missing file is
// skipped (returns nil); any other read error or malformed JSON is
// returned as-is for the caller to wrap.
func loadFile[T any](path string, out *T) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	return json.Unmarshal(data, out)
}

// ExpandHome expands a leading "~/" in path to the current user's home
// directory. Every other shape — absolute paths, plain relative paths, "",
// and "~user/"-style paths — is returned unchanged. Load never calls this:
// it can't know which string fields are filesystem paths, so expansion is
// left to the consumer to apply to its own path fields after Load returns.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[len("~/"):])
}
