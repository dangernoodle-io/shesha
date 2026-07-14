// Package xdgpath resolves per-application config/cache/data directories
// following the XDG Base Directory precedence, forced to ~/.config,
// ~/.cache, and ~/.local/share on every platform (including macOS, which
// otherwise defaults to ~/Library/Application Support and ~/Library/Caches).
//
// Resolution order (most-specific first), for config:
//
//  1. $<APP>_CONFIG_DIR, if set and absolute — used verbatim as the app dir.
//  2. $XDG_CONFIG_HOME, if set and absolute — joined with app.
//  3. ~/.config, joined with app.
//
// Cache mirrors this with $<APP>_CACHE_DIR / $XDG_CACHE_HOME / ~/.cache.
// Data mirrors this with $<APP>_DATA_DIR / $XDG_DATA_HOME / ~/.local/share.
// Per the XDG Base Directory spec, a $XDG_* value that is not an absolute
// path is treated as unset.
package xdgpath

import (
	"os"
	"path/filepath"

	"github.com/dangernoodle-io/shesha/internal/keyname"
)

// ConfigDir returns the resolved config directory for app.
func ConfigDir(app string) string {
	return resolve(app, "CONFIG_DIR", "XDG_CONFIG_HOME", ".config")
}

// CacheDir returns the resolved cache directory for app.
func CacheDir(app string) string {
	return resolve(app, "CACHE_DIR", "XDG_CACHE_HOME", ".cache")
}

// DataDir returns the resolved data directory for app.
func DataDir(app string) string {
	return resolve(app, "DATA_DIR", "XDG_DATA_HOME", filepath.Join(".local", "share"))
}

// ConfigFile returns the path to name inside app's config directory.
func ConfigFile(app, name string) string {
	return filepath.Join(ConfigDir(app), name)
}

// CacheFile returns the path to name inside app's cache directory.
func CacheFile(app, name string) string {
	return filepath.Join(CacheDir(app), name)
}

// DataFile returns the path to name inside app's data directory.
func DataFile(app, name string) string {
	return filepath.Join(DataDir(app), name)
}

// resolve implements the shared precedence for ConfigDir, CacheDir, and
// DataDir. appSuffix is "CONFIG_DIR", "CACHE_DIR", or "DATA_DIR"; xdgVar is
// the XDG_* env var name; homeSub is the fallback subdirectory of the
// user's home directory ("." + "config"/"cache", or ".local/share").
func resolve(app, appSuffix, xdgVar, homeSub string) string {
	appEnv := envName(app) + "_" + appSuffix
	if v := os.Getenv(appEnv); v != "" && filepath.IsAbs(v) {
		return v
	}

	if v := os.Getenv(xdgVar); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, app)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	return filepath.Join(home, homeSub, app)
}

// envName upper-cases app and replaces every run of non-alphanumeric
// characters with a single underscore, for building <APP>_CONFIG_DIR /
// <APP>_CACHE_DIR env var names (e.g. "pogopin" -> "POGOPIN").
func envName(app string) string {
	return keyname.Upper(app)
}
