//go:build windows

package services

import "os"

// Windows has no O_NOFOLLOW/O_NONBLOCK; the resolve + regular-file + SameFile checks apply.
const sidecarOpenFlags = 0

func descriptorInsideRoot(f *os.File, realRoot string) bool { return true }
