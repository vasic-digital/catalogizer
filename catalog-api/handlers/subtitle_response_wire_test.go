package handlers

import (
	"encoding/json"
	"testing"

	"catalogizer/internal/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalog-web iterates these lists unguarded; an empty result must be "[]",
// never an omitted key and never null.
func TestSubtitleResponses_EmptyListsAreEmptyArrays(t *testing.T) {
	cases := map[string]struct {
		body any
		key  string
	}{
		"list nil":    {newSubtitleListResponse(7, nil), "subtitles"},
		"list empty":  {newSubtitleListResponse(7, []services.SubtitleTrack{}), "subtitles"},
		"search nil":  {newSubtitleSearchResponse(nil), "results"},
		"search zero": {newSubtitleSearchResponse([]services.SubtitleSearchResult{}), "results"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(c.body)
			require.NoError(t, err)
			var m map[string]any
			require.NoError(t, json.Unmarshal(raw, &m))
			v, present := m[c.key]
			require.True(t, present, "%s must be present in %s", c.key, raw)
			arr, isArr := v.([]any)
			assert.True(t, isArr, "%s must be a JSON array, got %T in %s", c.key, v, raw)
			assert.Len(t, arr, 0)
		})
	}
}

func TestSubtitleResponses_KeepItemsAndCount(t *testing.T) {
	r := newSubtitleSearchResponse([]services.SubtitleSearchResult{{}, {}})
	assert.Equal(t, 2, r.Count)
	assert.Len(t, r.Results, 2)
	l := newSubtitleListResponse(9, []services.SubtitleTrack{{}})
	assert.Equal(t, int64(9), l.MediaItemID)
	assert.Len(t, l.Subtitles, 1)
}
