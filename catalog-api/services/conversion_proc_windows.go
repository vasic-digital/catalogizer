//go:build windows

package services

import (
	"os/exec"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) { cmd.WaitDelay = 5 * time.Second }
