package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AllowedLocalBasesEnv names the environment variable holding the directories under which a "local" storage root may live
// (colon-separated absolute paths). It is unset by default, and unset means NO local root is allowed: a local root exposes a host
// directory to the API, so the operator must name the data directories explicitly (deny by default, CAT-001 review F1).
const AllowedLocalBasesEnv = "CATALOGIZER_LOCAL_ROOT_BASES"

// ErrLocalRootNotAllowed is returned for a local root outside the allowed base directories.
var ErrLocalRootNotAllowed = errors.New("local storage root is outside the allowed base directories")

// AllowedLocalBases returns the configured allowed base directories (absolute, cleaned). Relative entries are ignored.
func AllowedLocalBases() []string {
	var out []string
	for _, b := range strings.Split(os.Getenv(AllowedLocalBasesEnv), string(os.PathListSeparator)) {
		b = strings.TrimSpace(b)
		if b != "" && filepath.IsAbs(b) {
			out = append(out, filepath.Clean(b))
		}
	}
	return out
}

// CheckLocalRoot reports whether path, with every symbolic link resolved, lies inside one of the allowed bases (also resolved).
// An empty bases list denies everything. The path must exist: a root that cannot be resolved cannot be proven confined.
func CheckLocalRoot(path string, bases []string) error {
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("%w: path must be absolute", ErrLocalRootNotAllowed)
	}
	if len(bases) == 0 {
		return fmt.Errorf("%w: no base directory is configured (set %s)", ErrLocalRootNotAllowed, AllowedLocalBasesEnv)
	}
	// The deepest existing ancestor is resolved, the not-yet-existing tail is appended: a root may be registered before its directory
	// exists, and it is re-checked every time it is used (clientForRoot / QueueScan), when a symlink could have appeared.
	real, err := resolveExistingPrefix(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("%w: cannot resolve path: %v", ErrLocalRootNotAllowed, err)
	}
	for _, b := range bases {
		rb, err := filepath.EvalSymlinks(b)
		if err != nil {
			continue
		}
		if withinDir(rb, real) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrLocalRootNotAllowed, path)
}

// resolveExistingPrefix runs EvalSymlinks on the deepest existing ancestor of p and re-appends the missing tail.
func resolveExistingPrefix(p string) (string, error) {
	tail := ""
	cur := p
	for {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(real, tail), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", err
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}
