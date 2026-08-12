// Package discover resolves the GUI's runtime context: the mod root (and
// engine) and the library justfile. Kept free of the window package so tests
// run anywhere, including CI without webkit system libraries.
package discover

import (
	"os"
	"path/filepath"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/launcher"
)

// ModRoot resolves the mod root, preferring an explicit override. Returns
// empty strings when nothing is found so the GUI degrades to built-in mode
// instead of failing.
func ModRoot(override string) (modRoot, engineRoot, modID string) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	res, err := launcher.Resolve(cwd, override, os.Getenv("KRISTAL_ROOT"))
	if err != nil {
		return "", "", ""
	}
	return res.ModRoot, res.EngineRoot, res.ModID
}

// Justfile finds the library justfile: explicit flag/env, then
// modRoot/libraries/kristal-debug-tools/justfile, then a walk up from the
// executable's directory (development layout).
func Justfile(override, modRoot string) string {
	if override != "" {
		return override
	}
	if modRoot != "" {
		candidate := filepath.Join(modRoot, "libraries", "kristal-debug-tools", "justfile")
		if fileExists(candidate) {
			return candidate
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return ""
	}
	return walkUpJustfile(filepath.Dir(exe))
}

// walkUpJustfile walks up from dir looking for a justfile (the development
// layout: a build dropped into <repo>/dist finds <repo>/justfile).
func walkUpJustfile(dir string) string {
	for {
		candidate := filepath.Join(dir, "justfile")
		if fileExists(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnvOr returns flagValue, or the named environment variable when the flag
// was not provided.
func EnvOr(flagValue, name string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(name)
}
