//go:build !windows

package services

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup starts the converter in its own process group and makes cancellation kill the
// whole group (LibreOffice, ebook-convert and pandoc fork helpers that would otherwise keep
// writing). The group id is the child's pid; it is validated (> 1) before any group signal
// (constitution 11.4.263: kill(-1, ...) would signal every process of the user).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if target, ok := groupKillTarget(cmd.Process.Pid); ok {
			if err := syscall.Kill(target, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				return cmd.Process.Kill()
			}
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 5 * time.Second
}

// groupKillTarget returns the argument for kill(2) that signals the whole process group of pid,
// or false when pid is not a safe group id: kill(-1, sig) signals every process of the user and
// kill(0/-0, sig) the caller's own group (constitution 11.4.263).
func groupKillTarget(pid int) (int, bool) {
	if pid <= 1 {
		return 0, false
	}
	return -pid, true
}
