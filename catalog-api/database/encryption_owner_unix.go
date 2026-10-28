//go:build !windows

package database

import (
	"fmt"
	"os"
	"syscall"
)

// checkKeyFileOwner fails closed when the key file is owned by another user.
func checkKeyFileOwner(info os.FileInfo) error {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return fmt.Errorf("database key file is owned by uid %d, expected the current user (uid %d)", st.Uid, os.Getuid())
		}
	}
	return nil
}
