//go:build !windows

package pathsafe

import "syscall"

// noFollowFlag makes open(2) fail on a symlink leaf (ELOOP).
const noFollowFlag = syscall.O_NOFOLLOW

// refuseLeafSymlink is a no-op on Unix: O_NOFOLLOW is enforced by the kernel
// atomically at open time, so no separate (racy) Lstat is needed.
func refuseLeafSymlink(string) error { return nil }
