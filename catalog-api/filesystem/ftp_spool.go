package filesystem

// availBytes turns statfs numbers into bytes. f_bavail counts fragment-size blocks (f_frsize on Linux), not f_bsize, so frsize wins and
// bsize is only the fallback for a zero frsize (and the only unit on platforms whose Statfs_t has no frsize); some BSDs report a negative
// bavail (as a huge unsigned value) when the reserve is overdrawn, which is 0 bytes, not exabytes.
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
