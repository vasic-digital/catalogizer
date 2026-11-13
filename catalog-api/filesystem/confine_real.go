// confine_real.go - CAT-FS-SYMLINK-1: real-path (symlink-aware) confinement shared by every client that maps a remote path onto a local
// directory tree (LocalClient, NFSClient on linux and darwin). ConfinePath (confine.go) is LEXICAL: it keeps ".." from climbing out, but a
// symbolic link inside the root that points outside it is still followed by the kernel. This file closes that.
//
// Design (the same walk-and-Lstat approach as the configuration wizard's root confinement, OBF-FU): the path is walked component by component
// with Lstat; every symbolic link met on the way is read and its target is walked in turn (a link limit stops loops); a component that does
// not exist yet ends the walk and the rest is appended lexically (so a file or directory that is about to be created is judged by its
// existing parent, and a DANGLING link is judged by where it points, which EvalSymlinks cannot do - it reports "not exist" and the old
// code then judged the link's parent instead). The resulting real path must be the real root or lie below it.
//
// Hard links are not inspected (operator decision 2026-11-13: no st_nlink > 1 refusal). A per-root opt-out exists (AllowSymlinkEscape,
// settings key allow_symlink_escape, default false = confinement ON).
//
// Residual (stated, not closed): the check and the following open/create/remove are separate system calls, so a link swapped in between them
// (check-to-use race) is not prevented; it needs a party that can write inside the storage root while the scan runs.
package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscapesRoot is returned when a path, after symbolic links are resolved, lies outside the storage root.
var ErrPathEscapesRoot = errors.New("path resolves outside the storage root")

// ErrSymlinkLoop is returned when more than maxSymlinkHops links are met while resolving one path.
var ErrSymlinkLoop = errors.New("too many levels of symbolic links")

const maxSymlinkHops = 40

// realpathLax resolves every symbolic link of the absolute path p, like filepath.EvalSymlinks, but a component that does not exist is not an
// error: the walk stops there and the remaining components are appended lexically. A dangling link is followed to its (missing) target.
func realpathLax(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("realpath: %q is not absolute", p)
	}
	pending := splitComponents(p)
	cur := string(filepath.Separator)
	hops := 0
	for len(pending) > 0 {
		comp := pending[0]
		pending = pending[1:]
		switch comp {
		case "", ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, comp)
		fi, err := os.Lstat(next)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cur = next // missing: the rest is lexical
				continue
			}
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			cur = next
			continue
		}
		hops++
		if hops > maxSymlinkHops {
			return "", ErrSymlinkLoop
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			cur = string(filepath.Separator)
		}
		pending = append(splitComponents(target), pending...)
	}
	return cur, nil
}

func splitComponents(p string) []string {
	return strings.Split(filepath.ToSlash(p), "/")
}

// confineReal checks that full (a lexically confined absolute path below root) does not leave the real root through a symbolic link, and
// returns full unchanged. root must exist. When followLeaf is false the last component is NOT followed (delete/rename of a link removes the
// link itself, not its target), so only the directory that holds it is checked. allowEscape is the per-root opt-out.
func confineReal(root, full, shown string, allowEscape, followLeaf bool) (string, error) {
	if allowEscape {
		return full, nil
	}
	realRoot, err := realpathLax(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %s: %w", root, err)
	}
	probe := full
	leaf := ""
	if filepath.Clean(full) == filepath.Clean(root) {
		followLeaf = true // the root itself: it is the trusted, configured entry (it may be a link on purpose)
	}
	if !followLeaf {
		probe = filepath.Dir(full)
		leaf = filepath.Base(full)
	}
	real, err := realpathLax(probe)
	if err != nil {
		return "", err
	}
	if leaf != "" {
		real = filepath.Join(real, leaf)
	}
	if !withinDir(realRoot, real) {
		return "", fmt.Errorf("%w: %s", ErrPathEscapesRoot, shown)
	}
	return full, nil
}

// withinDir reports whether target is dir or lies below it (both already symlink-resolved).
func withinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
