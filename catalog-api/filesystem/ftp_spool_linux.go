//go:build linux

package filesystem

import "syscall"

// freeSpace returns the bytes available to an unprivileged process in dir (f_bavail blocks of f_frsize bytes).
func freeSpace(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return availBytes(uint64(st.Frsize), uint64(st.Bsize), uint64(st.Bavail)), nil
}
