//go:build linux

package services

import "syscall"

// setParentDeathSignal asks the kernel to SIGKILL the converter child the moment its parent
// (the server) dies for any reason, including SIGKILL/OOM-kill where no Go cleanup runs; without it
// the child is reparented to init and keeps running with nobody tracking its output.
//
// Honest limits: the signal goes to the direct child only (helpers the converter forks itself,
// e.g. soffice.bin, are not signalled; they are in the child's process group but only
// the normal cancellation path, which kills the whole group, reaches them), and Linux ties
// it to the parent THREAD that forked the child. The Go runtime keeps its OS threads alive
// for the life of the process unless a goroutine exits while locked to one (the converter
// workers do not lock OS threads), so the child is not signalled early.
func setParentDeathSignal(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGKILL }
