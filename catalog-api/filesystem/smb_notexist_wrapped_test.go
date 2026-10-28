package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"testing"
)

// Found by the live SMB tests: go-smb2 reports a missing path as a *fs.PathError ("stat X: file does not exist") whose text
// carries the operation and path, so an exact-string comparison never matched and FileExists returned an error instead of false.
func TestIsNotExistError_WrappedPathError(t *testing.T) {
	cases := map[string]error{
		"PathError wrapping ErrNotExist": &fs.PathError{Op: "stat", Path: "some/dir", Err: fs.ErrNotExist},
		"os.ErrNotExist wrapped":         fmt.Errorf("outer: %w", os.ErrNotExist),
	}
	for name, err := range cases {
		if !isNotExistError(err) {
			t.Errorf("%s: isNotExistError = false, want true (%v)", name, err)
		}
	}
	if isNotExistError(&fs.PathError{Op: "open", Path: "x", Err: fs.ErrPermission}) {
		t.Errorf("permission error must not be classified as not-exist")
	}
}
