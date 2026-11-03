package media

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-001: MediaManager.getQualityDistribution returned hardcoded zeros.
func TestCAT001_Manager_QualityDistributionCountsRealRows(t *testing.T) {
	mm := buildTestManager(t)
	for i, qi := range []string{
		`{"resolution":{"width":3840,"height":2160}}`,
		`{"resolution":{"width":1920,"height":1080}}`,
		`{"resolution":{"width":1280,"height":720}}`,
		`{"resolution":{"width":720,"height":480}}`,
		`{}`,
	} {
		_, err := mm.mediaDB.GetDB().Exec(`INSERT INTO media_files (media_item_id, file_path, smb_root, filename, file_size, quality_info)
			VALUES (1, ?, 'r', 'f', 10, ?)`, fmt.Sprintf("/p/%d", i), qi)
		require.NoError(t, err)
	}
	got, err := mm.getQualityDistribution()
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"4K/UHD": 1, "1080p": 1, "720p": 1, "DVD": 1, "Other": 1}, got)
}
