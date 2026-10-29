//go:build !windows

package filesystem

import "syscall"

// freeSpace returns the bytes available to an unprivileged process in dir.
func freeSpace(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	// f_bavail counts fragment-size blocks (f_frsize), not f_bsize; some BSDs report a negative bavail when the reserve is overdrawn
	bsize := uint64(st.Frsize)
	if bsize == 0 {
		bsize = uint64(st.Bsize)
	}
	if int64(st.Bavail) < 0 {
		return 0, nil
	}
	return uint64(st.Bavail) * bsize, nil
}
