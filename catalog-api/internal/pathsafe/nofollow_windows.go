//go:build windows

package pathsafe

import (
	"fmt"
	"os"
)

// noFollowFlag is 0: Windows has no O_NOFOLLOW. New files are still created
// with O_EXCL (which does not follow an existing link) when overwrite=false.
const noFollowFlag = 0

// refuseLeafSymlink rejects an existing symlink / reparse-point leaf before
// opening. Unlike the Unix path this is an Lstat-then-open sequence, so a leaf
// swapped between the check and the open is a tracked residual on Windows.
func refuseLeafSymlink(p string) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return nil // does not exist (or unreadable): let OpenFile report it
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink %q", p)
	}
	return nil
}
