package services

import "errors"

// Sentinel errors the subtitle HTTP handlers map to status codes. Callers test with errors.Is;
// the wrapped chain may carry internal detail that MUST NOT reach a client body.
var (
	// ErrMediaNotFound means the media (files) row does not exist.
	ErrMediaNotFound = errors.New("media item not found")
	// ErrSubtitleNotFound means no subtitle track has that id (or it belongs to another media item).
	ErrSubtitleNotFound = errors.New("subtitle not found")
	// ErrInvalidSubtitleContent means the uploaded bytes are not a valid subtitle of the stated format.
	ErrInvalidSubtitleContent = errors.New("invalid subtitle content")
	// ErrNoTranslationProvider means no translation provider is configured on this server.
	ErrNoTranslationProvider = errors.New("no translation provider configured")
	// ErrSubtitleForbidden means the caller may not modify that subtitle.
	ErrSubtitleForbidden = errors.New("subtitle modification forbidden")
	// ErrSyncBusy means the bounded media-analysis capacity is in use; the caller should retry shortly.
	ErrSyncBusy = errors.New("synchronization analysis busy")
)
