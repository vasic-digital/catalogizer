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
		if pgid := cmd.Process.Pid; pgid > 1 {
			if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				return cmd.Process.Kill()
			}
			return nil
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 5 * time.Second
}
