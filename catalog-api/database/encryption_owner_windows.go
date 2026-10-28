//go:build windows

package database

import (
	"errors"
	"os"
)

// errKeySymlink reports that the key path is a symbolic link.
var errKeySymlink = errors.New("symlink")

// keyPermissionsEnforced is false on Windows: POSIX mode bits and ownership
// do not apply (os.FileMode permission bits are synthetic), so the key file
// mode/owner/parent-directory checks are skipped there. Protect the key file
// with NTFS ACLs (docs/DATABASE_ENCRYPTION.md).
func keyPermissionsEnforced() bool { return false }

// openKeyFileNoFollow refuses a symlink (checked with Lstat) and opens the file.
func openKeyFileNoFollow(path string) (*os.File, error) {
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return nil, errKeySymlink
	}
	return os.Open(path)
}

func checkKeyFileOwner(os.FileInfo) error { return nil }

func checkKeyDir(string) error { return nil }
