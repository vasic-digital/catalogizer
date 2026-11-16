package handlers

import (
	"errors"
	"net/http"

	"catalogizer/filesystem"
)

// copyErrorStatus maps the typed copy refusals of the filesystem package (filesystem.ErrCopy*) to an HTTP status and a client-safe message
// (SMBCOPYR-F7, operator decision 2026-11-17). The raw error text is never sent: it carries paths. ok is false for any other error,
// which stays a 500.
//
//	ErrCopyDestReadOnly     403  the request is understood and refused: the destination forbids writing (operator decision: keep the refusal).
//	ErrCopySameFile         409  source and destination are one file: a conflict between the two named resources.
//	ErrCopyDestIsDirectory  409  the destination exists and is a directory: a conflict with the current state of the target.
//	ErrCopySourceNotRegular 422  the request is well-formed but the named source cannot be copied (directory, FIFO, device, socket).
func copyErrorStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, filesystem.ErrCopyDestReadOnly):
		return http.StatusForbidden, "Destination is read-only", true
	case errors.Is(err, filesystem.ErrCopySameFile):
		return http.StatusConflict, "Source and destination are the same file", true
	case errors.Is(err, filesystem.ErrCopyDestIsDirectory):
		return http.StatusConflict, "Destination is an existing directory", true
	case errors.Is(err, filesystem.ErrCopySourceNotRegular):
		return http.StatusUnprocessableEntity, "Source is not a regular file", true
	}
	return 0, "", false
}
