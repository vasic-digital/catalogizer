//go:build !linux && !windows

package services

import "syscall"

// setParentDeathSignal is a no-op where the kernel offers no parent-death signal (Pdeathsig is
// Linux-only): there the converter child can outlive an abruptly killed server.
func setParentDeathSignal(*syscall.SysProcAttr) {
	// Intentionally empty: there is no parent-death signal to set on this platform.
}
