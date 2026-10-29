//go:build darwin || freebsd

package filesystem

import "syscall"

// freeSpace returns the bytes available to an unprivileged process in dir. The Statfs_t of darwin and freebsd has no Frsize field: Bsize is
// the unit of Bavail there. (Bavail is signed on freebsd and may be negative; availBytes maps that to 0.)
func freeSpace(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return availBytes(0, uint64(st.Bsize), uint64(st.Bavail)), nil
}
