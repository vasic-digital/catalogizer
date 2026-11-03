package services

import (
	"catalogizer/database"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLyricsService_ParseLyricsLines(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	service := NewLyricsService(mockDB, mockLogger)

	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "simple lyrics",
			content:  "Line one\nLine two\nLine three",
			expected: 3,
		},
		{
			name:     "lyrics with structure markers",
			content:  "[Verse 1]\nLine one\nLine two\n[Chorus]\nChorus line",
			expected: 3,
		},
		{
			name:     "lyrics with empty lines",
			content:  "Line one\n\nLine two\n\nLine three",
			expected: 3,
		},
		{
			name:     "empty content",
			content:  "",
			expected: 0,
		},
		{
			name:     "only structure markers",
			content:  "[Verse 1]\n[Chorus]\n[Bridge]",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.parseLyricsLines(tt.content)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestLyricsService_FilterSyncedLyrics(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	service := NewLyricsService(mockDB, mockLogger)

	tests := []struct {
		name     string
		results  []LyricsSearchResult
		expected int
	}{
		{
			name: "mixed synced and unsynced",
			results: []LyricsSearchResult{
				{IsSynced: true, Title: "Song A"},
				{IsSynced: false, Title: "Song B"},
				{IsSynced: true, Title: "Song C"},
			},
			expected: 2,
		},
		{
			name: "all synced",
			results: []LyricsSearchResult{
				{IsSynced: true, Title: "Song A"},
				{IsSynced: true, Title: "Song B"},
			},
			expected: 2,
		},
		{
			name: "none synced",
			results: []LyricsSearchResult{
				{IsSynced: false, Title: "Song A"},
				{IsSynced: false, Title: "Song B"},
			},
			expected: 0,
		},
		{
			name:     "empty results",
			results:  []LyricsSearchResult{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.filterSyncedLyrics(tt.results)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestLyricsService_SortLyricsResults(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	service := NewLyricsService(mockDB, mockLogger)

	tests := []struct {
		name    string
		results []LyricsSearchResult
	}{
		{
			name: "sort by match score",
			results: []LyricsSearchResult{
				{MatchScore: 0.5, Confidence: 0.8},
				{MatchScore: 0.9, Confidence: 0.7},
				{MatchScore: 0.3, Confidence: 0.9},
			},
		},
		{
			name: "sort by confidence when scores equal",
			results: []LyricsSearchResult{
				{MatchScore: 0.8, Confidence: 0.5},
				{MatchScore: 0.8, Confidence: 0.9},
				{MatchScore: 0.8, Confidence: 0.7},
			},
		},
		{
			name:    "empty results",
			results: []LyricsSearchResult{},
		},
		{
			name: "single result",
			results: []LyricsSearchResult{
				{MatchScore: 0.5, Confidence: 0.5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.sortLyricsResults(tt.results)
			// Verify sorted order: descending by match score, then confidence
			for i := 1; i < len(tt.results); i++ {
				assert.True(t,
					tt.results[i-1].MatchScore > tt.results[i].MatchScore ||
						(tt.results[i-1].MatchScore == tt.results[i].MatchScore &&
							tt.results[i-1].Confidence >= tt.results[i].Confidence),
					"results should be sorted by match score then confidence")
			}
		})
	}
}


func TestGenerateLyricsID(t *testing.T) {
	id1 := generateLyricsID()
	id2 := generateLyricsID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.Contains(t, id1, "lyrics_")
}
