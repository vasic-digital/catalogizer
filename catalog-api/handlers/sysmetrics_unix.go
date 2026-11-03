//go:build !windows

package handlers

import "syscall"

// diskUsageOf returns the total / used / available bytes of the file system holding path (statfs). ok is false when it cannot be measured.
func diskUsageOf(path string) (total, used, avail int64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, false
	}
	total, used, avail = statfsFigures(uint64(st.Blocks), uint64(st.Bfree), uint64(st.Bavail), uint64(st.Bsize))
	return total, used, avail, true
}

// statfsFigures converts statfs block counts to bytes: used is total minus ALL free blocks, avail is what an unprivileged user may use (Bavail).
func statfsFigures(blocks, bfree, bavail, bsize uint64) (total, used, avail int64) {
	t := blocks * bsize
	f := bfree * bsize
	a := bavail * bsize
	return int64(t), int64(t - f), int64(a)
}
