package services

import (
	"catalogizer/database"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockCacheService is a mock implementation of CacheServiceInterface
type MockCacheService struct {
	mock.Mock
}

func (m *MockCacheService) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	args := m.Called(ctx, key, dest)
	return args.Bool(0), args.Error(1)
}

func (m *MockCacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

// MockTranslationService is a mock implementation of TranslationService
type MockTranslationService struct {
	mock.Mock
}

func (m *MockTranslationService) TranslateText(ctx context.Context, request TranslationRequest) (*TranslationResult, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*TranslationResult), args.Error(1)
}

func (m *MockTranslationService) GetSupportedLanguages() []SupportedLanguage {
	// Return a mock list for testing
	return []SupportedLanguage{
		{Code: "en", Name: "English", NativeName: "English", Flag: "🇺🇸", Direction: "ltr", IsPopular: true},
		{Code: "es", Name: "Spanish", NativeName: "Español", Flag: "🇪🇸", Direction: "ltr", IsPopular: true},
	}
}

func (m *MockTranslationService) DetectLanguage(ctx context.Context, text string) (*LanguageDetectionResult, error) {
	args := m.Called(ctx, text)
	return args.Get(0).(*LanguageDetectionResult), args.Error(1)
}

func TestNewSubtitleService(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	assert.NotNil(t, service)
	assert.Equal(t, mockDB, service.db)
	assert.Equal(t, mockLogger, service.logger)
	assert.Equal(t, mockCache, service.cacheService)
	assert.NotNil(t, service.translationService)
	assert.NotNil(t, service.httpClient)
	assert.NotNil(t, service.apiKeys)
	assert.Equal(t, "./cache/subtitles", service.cacheDir)
}

// CAT-001: no provider has a real client yet, so a search must surface an
// honest error, never invented subtitle results.
func TestSubtitleService_SearchSubtitles_NoFabricatedResults(t *testing.T) {
	service := NewSubtitleService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop(), &MockCacheService{})

	for _, providers := range [][]SubtitleProvider{
		nil,
		{ProviderOpenSubtitles},
		{ProviderOpenSubtitles, ProviderSubDB},
	} {
		results, err := service.SearchSubtitles(context.Background(), &SubtitleSearchRequest{
			MediaPath: "/path/to/movie.mp4",
			Title:     stringPtr("Test Movie"),
			Languages: []string{"en"},
			Providers: providers,
		})
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrSubtitleProviderNotImplemented)
		assert.Empty(t, results)
	}
}

func TestSubtitleService_ParseSRT(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	srtContent := `1
00:00:01,000 --> 00:00:04,000
Hello world

2
00:00:05,000 --> 00:00:08,000
This is a test
`

	lines, err := service.parseSRT(srtContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, 1, lines[0].Index)
	assert.Equal(t, "00:00:01,000", lines[0].StartTime)
	assert.Equal(t, "00:00:04,000", lines[0].EndTime)
	assert.Equal(t, "Hello world", lines[0].Text)
}

func TestSubtitleService_ParseSubtitle(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	srtContent := `1
00:00:01,000 --> 00:00:04,000
Test subtitle
`

	// Test SRT parsing
	result, err := service.parseSubtitle(srtContent, "srt")
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Test unsupported format
	_, err = service.parseSubtitle(srtContent, "unsupported")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported subtitle format")
}

func TestSubtitleService_ReconstructSRT(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	lines := []SubtitleLine{
		{
			Index:     1,
			StartTime: "00:00:01,000",
			EndTime:   "00:00:04,000",
			Text:      "Hello world",
		},
		{
			Index:     2,
			StartTime: "00:00:05,000",
			EndTime:   "00:00:08,000",
			Text:      "This is a test",
		},
	}

	result := service.reconstructSRT(lines)

	assert.NotNil(t, result)
	assert.Contains(t, result, "1")
	assert.Contains(t, result, "00:00:01,000 --> 00:00:04,000")
	assert.Contains(t, result, "Hello world")
	assert.Contains(t, result, "2")
	assert.Contains(t, result, "00:00:05,000 --> 00:00:08,000")
	assert.Contains(t, result, "This is a test")
}

func TestSubtitleService_SortSubtitleResults(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	results := []SubtitleSearchResult{
		{
			ID:         "result2",
			MatchScore: 0.7,
			Rating:     4.0,
		},
		{
			ID:         "result1",
			MatchScore: 0.9,
			Rating:     3.5,
		},
		{
			ID:         "result3",
			MatchScore: 0.8,
			Rating:     4.5,
		},
	}

	service.sortSubtitleResults(results)

	// Should be sorted by match score descending
	assert.Equal(t, "result1", results[0].ID) // 0.9
	assert.Equal(t, "result3", results[1].ID) // 0.8
	assert.Equal(t, "result2", results[2].ID) // 0.7
}

func TestSubtitleService_GetDownloadInfo_OpenSubtitles(t *testing.T) {
	mockCache := &MockCacheService{}
	service := NewSubtitleService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop(), mockCache)
	mockCache.On("Get", mock.Anything, "subtitle_download_info:opensubtitles_123", mock.AnythingOfType("*services.SubtitleSearchResult")).Return(false, nil)

	// CAT-001: no OpenSubtitles client exists, so an uncached id must not be
	// turned into an invented "Unknown Title" result with a fixed URL.
	result, err := service.getDownloadInfo(context.Background(), "opensubtitles_123")
	assert.ErrorIs(t, err, ErrSubtitleProviderNotImplemented)
	assert.Nil(t, result)
}

func TestSubtitleService_DownloadContent_NoCannedBody(t *testing.T) {
	service := NewSubtitleService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop(), &MockCacheService{})
	content, _, err := service.downloadContent(context.Background(), "https://dl.opensubtitles.org/1.srt")
	assert.NotContains(t, content, "Test subtitle line")
	_ = err // network/SSRF outcome is irrelevant; only canned content is forbidden
}

func TestSubtitleService_GetDownloadInfo_SubDB(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Mock cache miss
	mockCache.On("Get", mock.Anything, "subtitle_download_info:subdb_abc123", mock.AnythingOfType("*services.SubtitleSearchResult")).Return(false, nil)
	mockCache.On("Set", mock.Anything, "subtitle_download_info:subdb_abc123", mock.AnythingOfType("services.SubtitleSearchResult"), mock.AnythingOfType("time.Duration")).Return(nil)

	result, err := service.getDownloadInfo(context.Background(), "subdb_abc123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "subdb_abc123", result.ID)
	assert.Equal(t, ProviderSubDB, result.Provider)
	assert.Contains(t, result.DownloadURL, "api.thesubdb.com")
}

func TestSubtitleService_GetDownloadInfo_InvalidFormat(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Mock cache miss
	mockCache.On("Get", mock.Anything, "subtitle_download_info:invalid_format", mock.AnythingOfType("*services.SubtitleSearchResult")).Return(false, nil)

	_, err := service.getDownloadInfo(context.Background(), "invalid_format")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestSubtitleService_GetDownloadInfo_UnsupportedProvider(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Mock cache miss
	mockCache.On("Get", mock.Anything, "subtitle_download_info:unknown_123", mock.AnythingOfType("*services.SubtitleSearchResult")).Return(false, nil)

	_, err := service.getDownloadInfo(context.Background(), "unknown_123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestSubtitleService_ExtractSamplePoints(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	lines := []SubtitleLine{
		{Index: 1, StartTime: "00:00:01,000", EndTime: "00:00:04,000", Text: "Line 1"},
		{Index: 2, StartTime: "00:00:05,000", EndTime: "00:00:08,000", Text: "Line 2"},
		{Index: 3, StartTime: "00:00:09,000", EndTime: "00:00:12,000", Text: "Line 3"},
		{Index: 4, StartTime: "00:00:13,000", EndTime: "00:00:16,000", Text: "Line 4"},
		{Index: 5, StartTime: "00:00:17,000", EndTime: "00:00:20,000", Text: "Line 5"},
		{Index: 6, StartTime: "00:00:21,000", EndTime: "00:00:24,000", Text: "Line 6"},
		{Index: 7, StartTime: "00:00:25,000", EndTime: "00:00:28,000", Text: "Line 7"},
		{Index: 8, StartTime: "00:00:29,000", EndTime: "00:00:32,000", Text: "Line 8"},
		{Index: 9, StartTime: "00:00:33,000", EndTime: "00:00:36,000", Text: "Line 9"},
		{Index: 10, StartTime: "00:00:37,000", EndTime: "00:00:40,000", Text: "Line 10"},
	}

	points := service.extractSamplePoints(lines, 120.0)

	assert.NotNil(t, points)
	assert.Greater(t, len(points), 0)
	assert.LessOrEqual(t, len(points), 10) // Should sample at most 10 points
}

func TestSubtitleService_CalculateSyncOffset(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	points := []SyncPoint{
		{SubtitleTime: 1.0, VideoTime: 1.0, Confidence: 0.8},
		{SubtitleTime: 5.0, VideoTime: 5.2, Confidence: 0.9},
		{SubtitleTime: 10.0, VideoTime: 9.8, Confidence: 0.7},
	}

	videoInfo := &VideoInfo{
		Duration:  120.0,
		FrameRate: 24.0,
		Width:     1920,
		Height:    1080,
	}

	offset, confidence := service.calculateSyncOffset(points, videoInfo)

	assert.NotNil(t, offset)
	assert.NotNil(t, confidence)
	assert.Greater(t, confidence, 0.0)
	assert.LessOrEqual(t, confidence, 1.0)
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		timestamp string
		expected  float64
		hasError  bool
	}{
		{"00:01:23,456", 83.456, false},
		{"01:02:03,789", 3723.789, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, test := range tests {
		result, err := parseTimestamp(test.timestamp)
		if test.hasError {
			assert.Error(t, err, "Expected error for timestamp: %s", test.timestamp)
		} else {
			assert.NoError(t, err, "Expected no error for timestamp: %s", test.timestamp)
			assert.Equal(t, test.expected, result, "Incorrect parsing for timestamp: %s", test.timestamp)
		}
	}
}

func TestDetectEncoding(t *testing.T) {
	// Test UTF-8 BOM
	dataWithBOM := []byte{0xEF, 0xBB, 0xBF, 'H', 'e', 'l', 'l', 'o'}
	encoding := detectEncoding(dataWithBOM)
	assert.Equal(t, "utf-8", encoding)

	// Test without BOM
	dataWithoutBOM := []byte{'H', 'e', 'l', 'l', 'o'}
	encoding = detectEncoding(dataWithoutBOM)
	assert.Equal(t, "utf-8", encoding) // Default
}

func TestGenerateSubtitleID(t *testing.T) {
	id1 := generateSubtitleID()
	id2 := generateSubtitleID()

	assert.NotNil(t, id1)
	assert.NotNil(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "sub_")
	assert.Contains(t, id2, "sub_")
}

func TestSubtitleService_ParseVTT(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test basic VTT parsing
	vttContent := `WEBVTT

00:00:01.000 --> 00:00:04.000
Hello world

00:00:05.000 --> 00:00:08.000
This is a test
`

	lines, err := service.parseVTT(vttContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, 1, lines[0].Index)
	assert.Equal(t, "00:00:01,000", lines[0].StartTime)
	assert.Equal(t, "00:00:04,000", lines[0].EndTime)
	assert.Equal(t, "Hello world", lines[0].Text)
	assert.Equal(t, 2, lines[1].Index)
	assert.Equal(t, "00:00:05,000", lines[1].StartTime)
	assert.Equal(t, "00:00:08,000", lines[1].EndTime)
	assert.Equal(t, "This is a test", lines[1].Text)
}

func TestSubtitleService_ParseVTT_WithCueIdentifiers(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test VTT with cue identifiers
	vttContent := `WEBVTT

cue-1
00:00:01.000 --> 00:00:04.000
First line

cue-2
00:00:05.000 --> 00:00:08.000
Second line
`

	lines, err := service.parseVTT(vttContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "First line", lines[0].Text)
	assert.Equal(t, "Second line", lines[1].Text)
}

func TestSubtitleService_ParseVTT_WithCueSettings(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test VTT with cue settings (position, align, etc.)
	vttContent := `WEBVTT

00:00:01.000 --> 00:00:04.000 line:0 position:50% align:center
Centered text

00:00:05.000 --> 00:00:08.000 vertical:rl
Vertical text
`

	lines, err := service.parseVTT(vttContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "Centered text", lines[0].Text)
	assert.Equal(t, "Vertical text", lines[1].Text)
}

func TestSubtitleService_ParseVTT_WithTags(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test VTT with voice tags and formatting
	vttContent := `WEBVTT

00:00:01.000 --> 00:00:04.000
<v Speaker1>Hello there

00:00:05.000 --> 00:00:08.000
<c.highlighted>Important text</c>
`

	lines, err := service.parseVTT(vttContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	// Tags should be stripped
	assert.Equal(t, "Hello there", lines[0].Text)
	assert.Equal(t, "Important text", lines[1].Text)
}

func TestSubtitleService_ParseVTT_ShortTimestamps(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test VTT with MM:SS.mmm format (no hours)
	vttContent := `WEBVTT

01:23.000 --> 01:27.000
Short timestamp format
`

	lines, err := service.parseVTT(vttContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, "00:01:23,000", lines[0].StartTime)
	assert.Equal(t, "00:01:27,000", lines[0].EndTime)
}

func TestSubtitleService_ParseVTT_MissingHeader(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test VTT without WEBVTT header (invalid)
	vttContent := `00:00:01.000 --> 00:00:04.000
Invalid VTT
`

	_, err := service.parseVTT(vttContent)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing WEBVTT header")
}

func TestSubtitleService_ParseVTT_EmptyCues(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test VTT with only header, no cues
	vttContent := `WEBVTT

NOTE This is a comment
`

	_, err := service.parseVTT(vttContent)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid subtitle cues found")
}

func TestSubtitleService_ParseASS(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test basic ASS parsing
	assContent := `[Script Info]
ScriptType: v4.00+
PlayResX: 384
PlayResY: 288

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H0000FFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,1,0,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello world
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,This is a test
`

	lines, err := service.parseASS(assContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, 1, lines[0].Index)
	assert.Equal(t, "00:00:01,000", lines[0].StartTime)
	assert.Equal(t, "00:00:04,000", lines[0].EndTime)
	assert.Equal(t, "Hello world", lines[0].Text)
	assert.Equal(t, 2, lines[1].Index)
	assert.Equal(t, "00:00:05,000", lines[1].StartTime)
	assert.Equal(t, "00:00:08,000", lines[1].EndTime)
	assert.Equal(t, "This is a test", lines[1].Text)
}

func TestSubtitleService_ParseASS_WithFormattingCodes(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test ASS with override tags
	assContent := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,{\an8}Top aligned text
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,{\b1}Bold text{\b0}
Dialogue: 0,0:00:09.00,0:00:12.00,Default,,0,0,0,,{\pos(192,144)}Positioned text
`

	lines, err := service.parseASS(assContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 3, len(lines))
	// Formatting codes should be stripped
	assert.Equal(t, "Top aligned text", lines[0].Text)
	assert.Equal(t, "Bold text", lines[1].Text)
	assert.Equal(t, "Positioned text", lines[2].Text)
}

func TestSubtitleService_ParseASS_WithNewlines(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test ASS with \N and \n newlines
	assContent := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Line 1\NLine 2
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,Line A\nLine B
`

	lines, err := service.parseASS(assContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Contains(t, lines[0].Text, "\n")
	assert.Equal(t, "Line 1\nLine 2", lines[0].Text)
	assert.Equal(t, "Line A\nLine B", lines[1].Text)
}

func TestSubtitleService_ParseASS_WithCommasInText(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test ASS with commas in the text field
	assContent := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello, how are you?
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,Yes, I am fine, thank you!
`

	lines, err := service.parseASS(assContent)

	assert.NoError(t, err)
	assert.NotNil(t, lines)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "Hello, how are you?", lines[0].Text)
	assert.Equal(t, "Yes, I am fine, thank you!", lines[1].Text)
}

func TestSubtitleService_ParseASS_MissingEvents(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test ASS without [Events] section
	assContent := `[Script Info]
ScriptType: v4.00+
`

	_, err := service.parseASS(assContent)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing [Events] section")
}

func TestSubtitleService_ParseASS_EmptyDialogues(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	// Test ASS with empty dialogue entries
	assContent := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Comment: This is a comment
`

	_, err := service.parseASS(assContent)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid dialogue entries found")
}

func TestSubtitleService_ParseASS_Timestamp(t *testing.T) {
	// Test ASS timestamp parsing (H:MM:SS.cc format)
	tests := []struct {
		timestamp string
		expected  string
		hasError  bool
	}{
		{"0:00:01.00", "00:00:01,000", false},
		{"0:01:23.45", "00:01:23,450", false},
		{"1:02:03.99", "01:02:03,990", false},
		{"12:34:56.78", "12:34:56,780", false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, test := range tests {
		result, err := parseASSTimestamp(test.timestamp)
		if test.hasError {
			assert.Error(t, err, "Expected error for timestamp: %s", test.timestamp)
		} else {
			assert.NoError(t, err, "Expected no error for timestamp: %s", test.timestamp)
			assert.Equal(t, test.expected, result, "Incorrect parsing for timestamp: %s", test.timestamp)
		}
	}
}

func TestSubtitleService_ParseSubtitle_VTT(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	vttContent := `WEBVTT

00:00:01.000 --> 00:00:04.000
Test VTT subtitle
`

	result, err := service.parseSubtitle(vttContent, "vtt")
	assert.NoError(t, err)
	assert.NotNil(t, result)

	lines, ok := result.([]SubtitleLine)
	assert.True(t, ok)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, "Test VTT subtitle", lines[0].Text)
}

func TestSubtitleService_ParseSubtitle_ASS(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	assContent := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Test ASS subtitle
`

	result, err := service.parseSubtitle(assContent, "ass")
	assert.NoError(t, err)
	assert.NotNil(t, result)

	lines, ok := result.([]SubtitleLine)
	assert.True(t, ok)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, "Test ASS subtitle", lines[0].Text)
}

func TestSubtitleService_ParseSubtitle_SSA(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	mockLogger := zap.NewNop()
	mockCache := &MockCacheService{}

	service := NewSubtitleService(mockDB, mockLogger, mockCache)

	ssaContent := `[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Test SSA subtitle
`

	result, err := service.parseSubtitle(ssaContent, "ssa")
	assert.NoError(t, err)
	assert.NotNil(t, result)

	lines, ok := result.([]SubtitleLine)
	assert.True(t, ok)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, "Test SSA subtitle", lines[0].Text)
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
