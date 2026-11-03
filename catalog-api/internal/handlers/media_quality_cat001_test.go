package handlers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertQualityFile(t *testing.T, h *MediaHandler, n int, qi interface{}) {
	t.Helper()
	_, err := h.mediaDB.GetDB().Exec(`INSERT INTO media_files (media_item_id, file_path, smb_root, filename, file_size, quality_info)
		VALUES (1, ?, 'r', 'f', 10, ?)`, fmt.Sprintf("/p/%d", n), qi)
	require.NoError(t, err)
}

// CAT-001: getQualityDistribution returned hardcoded zeros ("Simplified
// implementation for now"). It must count the real media_files rows.
func TestCAT001_MediaHandler_QualityDistributionCountsRealRows(t *testing.T) {
	h, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()
	insertQualityFile(t, h, 1, `{"resolution":{"width":3840,"height":2160}}`)
	insertQualityFile(t, h, 2, `{"resolution":{"width":1920,"height":1080}}`)
	insertQualityFile(t, h, 3, `{"resolution":{"width":1920,"height":800}}`) // scope film, 1080p wide
	insertQualityFile(t, h, 4, `{"resolution":{"width":1280,"height":720}}`)
	insertQualityFile(t, h, 5, `{"resolution":{"width":720,"height":480}}`) // DVD folds into Other here
	insertQualityFile(t, h, 6, nil)
	insertQualityFile(t, h, 7, `not json`)

	got, err := h.getQualityDistribution()
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"4K/UHD": 1, "1080p": 2, "720p": 1, "Other": 3}, got)
}

func TestCAT001_MediaHandler_QualityDistributionEmptyIsZerosNotError(t *testing.T) {
	h, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()
	got, err := h.getQualityDistribution()
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"4K/UHD": 0, "1080p": 0, "720p": 0, "Other": 0}, got)
}
