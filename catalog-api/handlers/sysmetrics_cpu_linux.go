//go:build linux

package handlers

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func readCPUTimes() (idle, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0, false
	}
	for i, v := range f[1:] {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		total += n
		if i == 3 || i == 4 { // idle + iowait
			idle += n
		}
	}
	return idle, total, true
}

// hostCPUPercent measures the host CPU busy percentage over a short window from /proc/stat. ok is false when it cannot be measured.
func hostCPUPercent() (float64, bool) {
	i0, t0, ok := readCPUTimes()
	if !ok {
		return 0, false
	}
	time.Sleep(200 * time.Millisecond)
	i1, t1, ok := readCPUTimes()
	if !ok || t1 <= t0 {
		return 0, false
	}
	busy := 100 * (1 - float64(i1-i0)/float64(t1-t0))
	if busy < 0 {
		busy = 0
	}
	if busy > 100 {
		busy = 100
	}
	return busy, true
}
