//go:build windows

package database

import "os"

// checkKeyFileOwner is a no-op on Windows (POSIX ownership does not apply).
func checkKeyFileOwner(os.FileInfo) error { return nil }
