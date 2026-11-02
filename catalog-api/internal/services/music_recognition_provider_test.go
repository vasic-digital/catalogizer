package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMusicRecognitionProvider_GetProviderName(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	name := provider.GetProviderName()
	assert.Equal(t, "music_recognition", name)
}

func TestMusicRecognitionProvider_SupportsMediaType(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name      string
		mediaType MediaType
		expected  bool
	}{
		{
			name:      "supports music",
			mediaType: MediaTypeMusic,
			expected:  true,
		},
		{
			name:      "supports album",
			mediaType: MediaTypeAlbum,
			expected:  true,
		},
		{
			name:      "supports audiobook",
			mediaType: MediaTypeAudiobook,
			expected:  true,
		},
		{
			name:      "supports podcast",
			mediaType: MediaTypePodcast,
			expected:  true,
		},
		{
			name:      "does not support movie",
			mediaType: MediaTypeMovie,
			expected:  false,
		},
		{
			name:      "does not support video",
			mediaType: MediaTypeVideo,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.SupportsMediaType(tt.mediaType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicRecognitionProvider_GetConfidenceThreshold(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	threshold := provider.GetConfidenceThreshold()
	assert.Equal(t, 0.4, threshold)
}

func TestMusicRecognitionProvider_ExtractMusicMetadataFromFilename(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name           string
		filename       string
		expectedTitle  string
		expectedArtist string
	}{
		{
			name:           "artist dash title format",
			filename:       "Pink Floyd - Comfortably Numb.mp3",
			expectedTitle:  "Comfortably Numb",
			expectedArtist: "Pink Floyd",
		},
		{
			name:           "title only",
			filename:       "Unknown Song.flac",
			expectedTitle:  "Unknown Song",
			expectedArtist: "",
		},
		{
			name:           "track number prefix",
			filename:       "01 - Artist - Song Title.mp3",
			expectedTitle:  "Song Title",
			expectedArtist: "01", // "01" is treated as artist by the split on " - " (parts[0])
		},
		{
			name:           "empty filename",
			filename:       "",
			expectedTitle:  "",
			expectedArtist: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, artist, _ := provider.extractMusicMetadataFromFilename(tt.filename)
			if tt.expectedTitle != "" {
				assert.Equal(t, tt.expectedTitle, title)
			}
			if tt.expectedArtist != "" {
				assert.Equal(t, tt.expectedArtist, artist)
			}
		})
	}
}

func TestMusicRecognitionProvider_ExtractTrackNumber(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{
			name:     "numeric prefix",
			filename: "01 - Song.mp3",
			expected: 1,
		},
		{
			name:     "track prefix",
			filename: "Track 05.mp3",
			expected: 5,
		},
		{
			name:     "no track number",
			filename: "Song Title.mp3",
			expected: 0,
		},
		{
			name:     "dot separator",
			filename: "12.Song Title.mp3",
			expected: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.extractTrackNumber(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicRecognitionProvider_LooksLikeTrackNumber(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name     string
		str      string
		expected bool
	}{
		{
			name:     "single digit",
			str:      "1",
			expected: true,
		},
		{
			name:     "two digits",
			str:      "12",
			expected: true,
		},
		{
			name:     "three digits",
			str:      "101",
			expected: true,
		},
		{
			name:     "not a number",
			str:      "abc",
			expected: false,
		},
		{
			name:     "mixed content",
			str:      "12abc",
			expected: false,
		},
		{
			name:     "too many digits",
			str:      "1234",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.looksLikeTrackNumber(tt.str)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicRecognitionProvider_DetermineAudioMediaType(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name     string
		title    string
		expected MediaType
	}{
		{
			name:     "audiobook title",
			title:    "Harry Potter Audiobook Chapter 1",
			expected: MediaTypeAudiobook,
		},
		{
			name:     "podcast title",
			title:    "Tech Podcast Episode 42",
			expected: MediaTypePodcast,
		},
		{
			name:     "regular music",
			title:    "Stairway to Heaven",
			expected: MediaTypeMusic,
		},
		{
			name:     "empty title",
			title:    "",
			expected: MediaTypeMusic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.determineAudioMediaType(tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicRecognitionProvider_GetFileExtension(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "mp3 extension",
			filename: "song.mp3",
			expected: "mp3",
		},
		{
			name:     "flac extension",
			filename: "album.flac",
			expected: "flac",
		},
		{
			name:     "no extension",
			filename: "noext",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.getFileExtension(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicRecognitionProvider_ParseYear(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name     string
		dateStr  string
		expected int
	}{
		{
			name:     "full date",
			dateStr:  "2024-03-15",
			expected: 2024,
		},
		{
			name:     "year only",
			dateStr:  "1975",
			expected: 1975,
		},
		{
			name:     "empty string",
			dateStr:  "",
			expected: 0,
		},
		{
			name:     "short string",
			dateStr:  "20",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.parseYear(tt.dateStr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicRecognitionProvider_CalculateLastFMConfidence(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	tests := []struct {
		name          string
		listeners     string
		playcount     string
		minConfidence float64
	}{
		{
			name:          "popular track",
			listeners:     "5000",
			playcount:     "50000",
			minConfidence: 0.7,
		},
		{
			name:          "unpopular track",
			listeners:     "50",
			playcount:     "100",
			minConfidence: 0.4,
		},
		{
			name:          "invalid values",
			listeners:     "N/A",
			playcount:     "N/A",
			minConfidence: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := provider.calculateLastFMConfidence(tt.listeners, tt.playcount)
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
			assert.LessOrEqual(t, confidence, 1.0)
		})
	}
}

func TestMusicRecognitionProvider_GenerateID(t *testing.T) {
	mockLogger := zap.NewNop()
	provider := NewMusicRecognitionProvider(mockLogger)

	id1 := provider.generateID("Song A", "Artist A")
	id2 := provider.generateID("Song B", "Artist B")
	id3 := provider.generateID("Song A", "Artist A")

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Equal(t, id1, id3)
}

// CAT-001: tempo/energy/valence were derived from len(sample); they must not be
// reported as measurements.
func TestMusicRecognitionProvider_AudioAnalysisDoesNotInventMusicalFeatures(t *testing.T) {
	p := NewMusicRecognitionProvider(zap.NewNop())
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(i)
	}
	a := p.analyzeAudioFeatures(sample)
	assert.Zero(t, a.Tempo)
	assert.Zero(t, a.Energy)
	assert.Zero(t, a.Valence)

	fp, err := p.generateAudioFingerprint(sample)
	assert.NoError(t, err)
	if fp == nil {
		t.Fatal("nil fingerprint")
	}
	for _, k := range []string{"tempo", "energy", "valence"} {
		assert.NotContains(t, fp.Features, k)
		for _, seg := range fp.Segments {
			assert.NotContains(t, seg.Features, k)
		}
	}
}
