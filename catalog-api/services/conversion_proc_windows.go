//go:build windows

package services

import (
	"os/exec"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) { cmd.WaitDelay = 5 * time.Second }

// groupKillTarget exists for the shared unit tests; Windows has no process-group kill here.
func groupKillTarget(pid int) (int, bool) {
	if pid <= 1 {
		return 0, false
	}
	return -pid, true
}
