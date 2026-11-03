//go:build linux

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// CAT-001 review F11 (M6): iowait counts as idle.
func TestCAT001_ParseCPUStatCountsIowaitAsIdle(t *testing.T) {
	// user nice system idle iowait irq softirq
	idle, total, ok := parseCPUStat("cpu  10 20 30 40 50 60 70\ncpu0 1 1 1 1 1 1 1\n")
	assert.True(t, ok)
	assert.Equal(t, uint64(90), idle, "idle = idle(40) + iowait(50)")
	assert.Equal(t, uint64(280), total)
	_, _, ok = parseCPUStat("cpu0 1 2 3 4 5")
	assert.False(t, ok)
	_, _, ok = parseCPUStat("cpu 1 2 x 4 5")
	assert.False(t, ok)
	_, _, ok = parseCPUStat("cpu 1 2 3")
	assert.False(t, ok)
}

// CAT-001 review F11 (M7c): avail follows Bavail (not Bfree); used = total - Bfree.
func TestCAT001_StatfsFigures(t *testing.T) {
	total, used, avail := statfsFigures(1000, 300, 200, 4096)
	assert.Equal(t, int64(1000*4096), total)
	assert.Equal(t, int64(700*4096), used)
	assert.Equal(t, int64(200*4096), avail, "available must use Bavail, not Bfree")
}
