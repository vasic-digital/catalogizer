//go:build linux
// +build linux

package filesystem

import "syscall"

func syscallMkfifo(p string) error { return syscall.Mkfifo(p, 0o644) }
