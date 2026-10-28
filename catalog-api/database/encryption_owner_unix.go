//go:build !windows

package database

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// errKeySymlink reports that the key path is a symbolic link.
var errKeySymlink = errors.New("symlink")

// keyPermissionsEnforced: on POSIX systems the key file mode, owner and parent
// directory are verified (fail closed).
func keyPermissionsEnforced() bool { return true }

// openKeyFileNoFollow opens path read-only and refuses to follow a symlink in
// the final component (O_NOFOLLOW), so the checks done on the returned file
// descriptor (fstat) are about the very file that is then read: no
// check-then-use window.
func openKeyFileNoFollow(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil && errors.Is(err, syscall.ELOOP) {
		return nil, errKeySymlink
	}
	return f, err
}

// checkKeyFileOwner fails closed when the key file is owned by another user.
func checkKeyFileOwner(info os.FileInfo) error {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return checkOwnerUID(st.Uid, os.Getuid())
	}
	return nil
}

func checkOwnerUID(fileUID uint32, want int) error {
	if int(fileUID) != want {
		return fmt.Errorf("database key file is owned by uid %d, expected the current user (uid %d)", fileUID, want)
	}
	return nil
}

// checkKeyDir fails closed when the directory holding the key is writable by
// group or others: anyone who can write there can replace the key file.
func checkKeyDir(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("database key directory %q: %w", dir, err)
	}
	if st.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("database key directory %q is writable by group/others (mode %04o): use a directory only you can write (chmod 700)", dir, st.Mode().Perm())
	}
	return nil
}
