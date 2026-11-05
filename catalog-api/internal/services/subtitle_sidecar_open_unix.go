//go:build !windows

package services

import (
	"os"
	"runtime"
	"strconv"
	"syscall"
)

const sidecarOpenFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC

// descriptorInsideRoot re-verifies, on Linux through /proc/self/fd, that the OPEN descriptor
// still resolves inside the root (catches a swapped parent directory). Elsewhere the kernel-level
// O_NOFOLLOW and the SameFile check are the guard.
func descriptorInsideRoot(f *os.File, realRoot string) bool {
	if runtime.GOOS != "linux" {
		return true
	}
	target, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(int(f.Fd())))
	if err != nil {
		return false
	}
	return insideRoot(realRoot, target)
}
