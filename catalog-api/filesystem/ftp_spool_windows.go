//go:build windows

package filesystem

import "errors"

// freeSpace is unknown on this platform: the up-front check is skipped and a full disk is reported by the copy itself.
func freeSpace(dir string) (uint64, error) {
	return 0, errors.New("free space not available on this platform")
}
