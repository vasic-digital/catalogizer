//go:build windows

package handlers

// diskUsageOf is not measured on Windows in this build; callers report the value as unavailable rather than 0.
func diskUsageOf(path string) (total, used, avail int64, ok bool) { return 0, 0, 0, false }
