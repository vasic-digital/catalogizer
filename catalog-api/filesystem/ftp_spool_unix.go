//go:build !windows

package filesystem

import "syscall"

// freeSpace returns the bytes available to an unprivileged process in dir.
func freeSpace(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return availBytes(uint64(st.Frsize), uint64(st.Bsize), uint64(st.Bavail)), nil
}

// availBytes turns statfs numbers into bytes. f_bavail counts fragment-size blocks (f_frsize), not f_bsize, so frsize wins and bsize is
// only the fallback for a zero frsize; some BSDs report a negative bavail (as a huge unsigned value) when the reserve is overdrawn,
// which is 0 bytes, not exabytes.
func availBytes(frsize, bsize, bavail uint64) uint64 {
	unit := frsize
	if unit == 0 {
		unit = bsize
	}
	if int64(bavail) < 0 {
		return 0
	}
	return bavail * unit
}
