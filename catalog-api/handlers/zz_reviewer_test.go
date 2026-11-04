package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reviewer-authored: BOM-prefixed SRT must upload and measure.
func TestReviewer_BOMSRTUploadAndMeasure(t *testing.T) {
	h := newSubHarness(t)
	id := h.uploadOK("2", h.media, "bom.srt", "\xef\xbb\xbf"+srtShifted(7))
	code, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", id, h.media), "2", nil, "")
	require.Equal(t, 200, code, raw)
	sr := syncOf(js)
	assert.Equal(t, "measured", sr["status"], raw)
	assert.InDelta(t, 7000, sr["sync_offset"].(float64), 300, raw)
}

// Reviewer-authored: admin checker error must fail closed (403, not 204).
func TestReviewer_AdminCheckErrorFailsClosed(t *testing.T) {
	h := newSubHarness(t)
	a := h.uploadOK("2", h.media, "a.srt", srtShifted(0))
	h.h.WithAdminChecker(func(_ context.Context, _ int) (bool, error) { return true, errors.New("db down") })
	code, _, raw := h.do("DELETE", "/api/v1/subtitles/"+a, "3", nil, "")
	assert.Equal(t, 403, code, raw)
}
