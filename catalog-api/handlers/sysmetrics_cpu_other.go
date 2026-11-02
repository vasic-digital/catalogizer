//go:build !linux

package handlers

// hostCPUPercent is not measured on this platform; callers report the value as unavailable rather than 0.
func hostCPUPercent() (float64, bool) { return 0, false }
