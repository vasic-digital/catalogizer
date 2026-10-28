// Package pathguard confines request- or config-derived filesystem paths to a set of
// configured root directories (§11.4.115 path-traversal / unsafe-file-sink hardening).
//
// Confine resolves symlinks on the longest existing ancestor of the path, so neither a
// ".." segment nor a symlink planted inside a root can climb out of it. It is fail-closed:
// an empty root list, an empty/relative/NUL-bearing path, or any resolution error is a
// rejection, never an allow.
package pathguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutsideRoots is returned when a path resolves outside every allowed root.
var ErrOutsideRoots = errors.New("path is outside the allowed roots")

// RootsFromEnv returns the colon-separated absolute roots in env var name, or the
// defaults (made absolute against the working directory) when it is unset/empty.
func RootsFromEnv(name string, defaults ...string) []string {
	raw := os.Getenv(name)
	var parts []string
	if raw != "" {
		parts = strings.Split(raw, string(os.PathListSeparator))
	} else {
		parts = defaults
	}
	var roots []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if abs, err := filepath.Abs(p); err == nil {
			roots = append(roots, abs)
		}
	}
	return roots
}

// SafeComponent reports whether s is usable as a single path element: non-empty, not "."
// or "..", and free of separators and NUL bytes.
func SafeComponent(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.ContainsAny(s, "/\\\x00")
}

// Confine returns the symlink-resolved absolute form of p when it is equal to or below one
// of roots, and an error otherwise. p must be absolute.
func Confine(p string, roots []string) (string, error) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("pathguard: invalid path")
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("pathguard: path must be absolute")
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("pathguard: no allowed roots configured: %w", ErrOutsideRoots)
	}
	resolved, err := resolve(filepath.Clean(p))
	if err != nil {
		return "", fmt.Errorf("pathguard: %w", err)
	}
	for _, r := range roots {
		rr, err := resolve(filepath.Clean(r))
		if err != nil {
			continue
		}
		if within(rr, resolved) {
			return resolved, nil
		}
	}
	return "", ErrOutsideRoots
}

// within reports whether p equals base or is below it (component-wise, not by string prefix).
func within(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Within is the exported lexical containment check (no symlink resolution).
func Within(base, p string) bool { return within(filepath.Clean(base), filepath.Clean(p)) }

// resolve evaluates symlinks on the longest existing ancestor of p and re-appends the
// not-yet-existing remainder.
func resolve(p string) (string, error) {
	cur := p
	var tail []string
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				r = filepath.Join(r, tail[i])
			}
			return r, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor for %q", p)
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}
