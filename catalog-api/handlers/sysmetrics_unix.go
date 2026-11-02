//go:build !windows

package handlers

import "syscall"

// diskUsageOf returns the total / used / available bytes of the file system holding path (statfs). ok is false when it cannot be measured.
func diskUsageOf(path string) (total, used, avail int64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0, false
	}
	bsize := uint64(st.Bsize)
	t := uint64(st.Blocks) * bsize
	f := uint64(st.Bfree) * bsize
	a := uint64(st.Bavail) * bsize
	return int64(t), int64(t - f), int64(a), true
}
