package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path containment helpers shared by the copy handlers. Every path that
// originates outside the process (HTTP form fields, JSON bodies, names listed
// by a remote SMB server) must pass through one of these before it is joined
// onto a base path or handed to the OS / an SMB client.

var errPathEscapes = errors.New("path escapes the permitted root")

// safeChildName validates a single path element (an uploaded file name or a
// name returned by a directory listing). It rejects empty names, "." / "..",
// NUL bytes and anything containing a forward or back slash (backslash is a
// separator on SMB/Windows even though it is an ordinary byte on Linux).
func safeChildName(name string) (string, error) {
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid file name %q", name)
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return "", fmt.Errorf("file name %q contains a path separator or NUL", name)
	}
	return name, nil
}

// safeRemotePath validates a path on a remote (SMB) share: no NUL bytes and no
// ".." segment, with either slash style treated as a separator.
func safeRemotePath(p string) error {
	if strings.ContainsRune(p, 0) {
		return errors.New("path contains NUL byte")
	}
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return fmt.Errorf("path %q contains a parent-directory segment", p)
		}
	}
	return nil
}

// containedJoin resolves rel against root and guarantees that the result lies
// inside root, both lexically and after resolving symlinks of the deepest
// existing ancestor. rel may be absolute only if it already lies inside root.
func containedJoin(root, rel string) (string, error) {
	if root == "" {
		return "", errors.New("local destination root is not configured")
	}
	if strings.ContainsRune(rel, 0) || strings.ContainsRune(root, 0) {
		return "", errors.New("path contains NUL byte")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absRoot = filepath.Clean(absRoot)

	var candidate string
	if filepath.IsAbs(rel) {
		candidate = filepath.Clean(rel)
	} else {
		candidate = filepath.Join(absRoot, rel) // Join cleans, collapsing ".."
	}
	if !within(absRoot, candidate) {
		return "", errPathEscapes
	}

	// Symlink check: resolve the root and the deepest existing ancestor of the
	// candidate; the resolved ancestor must still be inside the resolved root.
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("cannot resolve root: %w", err)
	}
	existing := candidate
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		existing = parent
	}
	realExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}
	if !within(realRoot, realExisting) {
		return "", errPathEscapes
	}
	return candidate, nil
}

func within(root, p string) bool {
	if p == root {
		return true
	}
	return strings.HasPrefix(p, root+string(os.PathSeparator))
}
