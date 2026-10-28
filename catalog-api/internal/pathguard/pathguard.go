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

func getenv(name string) string { return os.Getenv(name) }

// ErrOutsideRoots is returned when a path resolves outside every allowed root.
var ErrOutsideRoots = errors.New("path is outside the allowed roots")

// RootsFromEnv returns the colon-separated absolute roots in env var name, or the
// defaults (made absolute against the working directory) when it is unset/empty.
func RootsFromEnv(name string, defaults ...string) []string {
	raw := getenv(name)
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
	// p is resolved exactly as given, never filepath.Clean'ed first: Clean folds "lnk/.."
	// lexically, while the kernel follows lnk and then steps up from its target (review round
	// 3, N1: root/lnk/../x with lnk -> /outside/sub was accepted as root/x and written to
	// /outside/x). resolve handles "", "." and ".." components itself.
	resolved, err := resolve(p)
	if err != nil {
		return "", fmt.Errorf("pathguard: %w", err)
	}
	for _, r := range roots {
		rr, err := resolve(r)
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

// maxLinkDepth bounds the number of symlinks resolve follows (loop guard).
const maxLinkDepth = 40

// resolve walks p component by component the way the kernel does: every existing symlink is
// followed (a dangling one included), a relative link target is resolved against the link's
// already-resolved directory, and a ".." steps up from the directory reached so far, i.e. from
// the target of a symlink that preceded it, not from the symlink's own parent. Callers pass
// the raw path; it must not be filepath.Clean'ed first (Clean removes "x/.." pairs without
// looking at x). Once an element does not exist, the remainder is appended lexically; a ".."
// after a missing element is refused, because its meaning depends on what is created later.
//
// Review round 1 (F1) made dangling links followed; review round 2 (S1) found that the
// target was then joined to the LEXICAL parent and cleaned lexically, so a link reached
// through a symlinked directory, or a target such as "s/../x" with s a symlink, resolved
// inside the root while the kernel wrote outside it. The walk below has neither shortcut.
func resolve(p string) (string, error) {
	sep := string(filepath.Separator)
	pending := splitPath(p)
	cur := sep
	links := 0
	for i := 0; i < len(pending); i++ {
		c := pending[i]
		if c == ".." {
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, c)
		fi, err := os.Lstat(next)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			out := next
			for _, r := range pending[i+1:] {
				if r == ".." {
					return "", fmt.Errorf("%q: \"..\" after a missing element %q", p, next)
				}
				out = filepath.Join(out, r)
			}
			return out, nil
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			cur = next
			continue
		}
		links++
		if links > maxLinkDepth {
			return "", fmt.Errorf("too many levels of symbolic links resolving %q", p)
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if target == "" {
			return "", fmt.Errorf("empty symlink target at %q", next)
		}
		if filepath.IsAbs(target) {
			cur = sep
		}
		rest := append(splitPath(target), pending[i+1:]...)
		pending = rest
		i = -1
	}
	return cur, nil
}

// splitPath returns the non-empty, non-"." components of p ("..", if any, is kept).
func splitPath(p string) []string {
	var out []string
	for _, c := range strings.Split(p, string(filepath.Separator)) {
		if c != "" && c != "." {
			out = append(out, c)
		}
	}
	return out
}
