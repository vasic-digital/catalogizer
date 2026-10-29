//go:build !windows

package filesystem

import "testing"

// F6: f_bavail counts f_frsize-sized blocks; f_bsize is only the fallback; a negative bavail is 0 bytes (not exabytes).
func TestAvailBytes(t *testing.T) {
	neg := ^uint64(0) // int64(-1)
	for name, c := range map[string]struct{ frsize, bsize, bavail, want uint64 }{
		"frsize wins over bsize":     {4096, 512, 10, 40960},
		"zero frsize falls back":     {0, 512, 10, 5120},
		"negative bavail is zero":    {4096, 512, neg, 0},
		"large positive bavail kept": {1, 1, 1 << 40, 1 << 40},
		"zero bavail":                {4096, 4096, 0, 0},
	} {
		if got := availBytes(c.frsize, c.bsize, c.bavail); got != c.want {
			t.Errorf("%s: availBytes(%d,%d,%d) = %d, want %d", name, c.frsize, c.bsize, c.bavail, got, c.want)
		}
	}
}
