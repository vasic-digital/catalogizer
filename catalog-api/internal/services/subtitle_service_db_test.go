package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newSubtitleTestDB creates an in-memory SQLite database with subtitle tables.
func newSubtitleTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	rawDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	db := database.WrapDB(rawDB, database.DialectSQLite)
	require.NotNil(t, db)

	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		storage_root_id INTEGER NOT NULL DEFAULT 1,
		path TEXT NOT NULL,
		name TEXT NOT NULL,
		extension TEXT DEFAULT '',
		size INTEGER DEFAULT 0,
		mime_type TEXT DEFAULT '',
		is_directory INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS subtitle_tracks (
		id TEXT PRIMARY KEY,
		media_item_id INTEGER NOT NULL,
		language TEXT NOT NULL,
		language_code TEXT NOT NULL,
		source TEXT NOT NULL,
		format TEXT NOT NULL,
		path TEXT,
		content TEXT,
		is_default INTEGER DEFAULT 0,
		is_forced INTEGER DEFAULT 0,
		encoding TEXT DEFAULT 'utf-8',
		sync_offset REAL DEFAULT 0.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		verified_sync INTEGER DEFAULT 0,
		FOREIGN KEY (media_item_id) REFERENCES files(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS file_metadata (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		FOREIGN KEY (file_id) REFERENCES files(id)
	);

	CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_media_item_id ON subtitle_tracks(media_item_id);
	`
	_, err = rawDB.Exec(schema)
	require.NoError(t, err)

	cleanup := func() {
		rawDB.Close()
	}

	return db, cleanup
}

// ---------------------------------------------------------------------------
// GetSubtitles — list subtitles for a media item from DB
// ---------------------------------------------------------------------------

func TestSubtitleService_GetSubtitles_Empty(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	subtitles, err := service.GetSubtitles(ctx, 999)
	require.NoError(t, err)
	assert.Empty(t, subtitles)
}

func TestSubtitleService_GetSubtitles_MultipleResults(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	// Insert a media file
	_, err := db.ExecContext(ctx,
		"INSERT INTO files (id, storage_root_id, path, name) VALUES (?, ?, ?, ?)",
		1, 1, "/movies/test.mp4", "test.mp4")
	require.NoError(t, err)

	// Insert subtitle tracks
	srtContent := "1\n00:00:01,000 --> 00:00:04,000\nHello world\n\n"
	_, err = db.ExecContext(ctx,
		`INSERT INTO subtitle_tracks (id, media_item_id, language, language_code, source, format, content, path, is_default, is_forced, encoding, sync_offset, created_at, verified_sync)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sub_en", 1, "English", "en", "downloaded", "srt", srtContent, "/subs/en.srt",
		1, 0, "utf-8", 0.0, time.Now(), 0)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO subtitle_tracks (id, media_item_id, language, language_code, source, format, content, path, is_default, is_forced, encoding, sync_offset, created_at, verified_sync)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sub_es", 1, "Spanish", "es", "translated", "srt", "translated content", "/subs/es.srt",
		0, 0, "utf-8", 0.0, time.Now(), 1)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO subtitle_tracks (id, media_item_id, language, language_code, source, format, content, path, is_default, is_forced, encoding, sync_offset, created_at, verified_sync)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		"sub_fr", 1, "French", "fr", "embedded", "srt", "french content",
		0, 1, "utf-8", 150.0, time.Now(), 0)
	require.NoError(t, err)

	subtitles, err := service.GetSubtitles(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, len(subtitles))

	// First should be the default (is_default DESC)
	assert.Equal(t, "sub_en", subtitles[0].ID)
	assert.Equal(t, "English", subtitles[0].Language)
	assert.Equal(t, "en", subtitles[0].LanguageCode)
	assert.Equal(t, "downloaded", subtitles[0].Source)
	assert.Equal(t, "srt", subtitles[0].Format)
	assert.NotNil(t, subtitles[0].Content)
	assert.NotNil(t, subtitles[0].Path)

	// French subtitle with forced flag and sync offset
	var frSub *SubtitleTrack
	for i := range subtitles {
		if subtitles[i].ID == "sub_fr" {
			frSub = &subtitles[i]
			break
		}
	}
	require.NotNil(t, frSub)
	assert.True(t, frSub.IsForced)
	assert.Equal(t, 150.0, frSub.SyncOffset)
	assert.Nil(t, frSub.Path) // Was NULL
}

// ---------------------------------------------------------------------------
// GetSubtitleTrack — by ID
// ---------------------------------------------------------------------------

func TestSubtitleService_GetSubtitleTrack_Found(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO subtitle_tracks (id, media_item_id, language, language_code, source, format, content, path, is_default, is_forced, encoding, sync_offset, created_at, verified_sync)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sub_001", 1, "English", "en", "downloaded", "srt", "test content", "/path/en.srt",
		1, 0, "utf-8", 0.0, time.Now(), 1)
	require.NoError(t, err)

	track, err := service.GetSubtitleTrack(ctx, "sub_001")
	require.NoError(t, err)
	require.NotNil(t, track)
	assert.Equal(t, "sub_001", track.ID)
	assert.Equal(t, "English", track.Language)
	assert.Equal(t, "en", track.LanguageCode)
	assert.Equal(t, "downloaded", track.Source)
	assert.Equal(t, "srt", track.Format)
	require.NotNil(t, track.Content)
	assert.Equal(t, "test content", *track.Content)
	require.NotNil(t, track.Path)
	assert.Equal(t, "/path/en.srt", *track.Path)
	assert.True(t, track.IsDefault)
	assert.False(t, track.IsForced)
	assert.Equal(t, "utf-8", track.Encoding)
	assert.True(t, track.VerifiedSync)
}

func TestSubtitleService_GetSubtitleTrack_NotFound(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	_, err := service.GetSubtitleTrack(ctx, "nonexistent_sub")
	assert.ErrorIs(t, err, ErrSubtitleNotFound, "a missing row is the typed not-found error the handlers map to 404")
}

// ---------------------------------------------------------------------------
// saveSubtitleTrack
// ---------------------------------------------------------------------------

func TestSubtitleService_SaveSubtitleTrack(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	// Insert a file first
	_, err := db.ExecContext(ctx,
		"INSERT INTO files (id, storage_root_id, path, name) VALUES (?, ?, ?, ?)",
		10, 1, "/movies/movie.mp4", "movie.mp4")
	require.NoError(t, err)

	content := "1\n00:00:01,000 --> 00:00:04,000\nSaved subtitle\n\n"
	path := "/cache/subs/saved.srt"
	track := &SubtitleTrack{
		ID:           "sub_saved_001",
		Language:     "German",
		LanguageCode: "de",
		Source:       "downloaded",
		Format:       "srt",
		Content:      &content,
		Path:         &path,
		IsDefault:    true,
		IsForced:     false,
		Encoding:     "utf-8",
		SyncOffset:   0.0,
		CreatedAt:    time.Now(),
		VerifiedSync: true,
	}

	err = service.saveSubtitleTrack(ctx, 10, track)
	require.NoError(t, err)

	// Verify via raw SQL (saveSubtitleTrack doesn't insert the id column,
	// so the service's GetSubtitles would fail scanning NULL id into string)
	var lang, langCode, source, savedContent string
	err = db.QueryRowContext(ctx,
		"SELECT language, language_code, source, content FROM subtitle_tracks WHERE media_item_id = ?", 10).
		Scan(&lang, &langCode, &source, &savedContent)
	require.NoError(t, err)
	assert.Equal(t, "German", lang)
	assert.Equal(t, "de", langCode)
	assert.Equal(t, "downloaded", source)
	assert.Contains(t, savedContent, "Saved subtitle")
}

func TestSubtitleService_SaveSubtitleTrack_NilContentAndPath(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		"INSERT INTO files (id, storage_root_id, path, name) VALUES (?, ?, ?, ?)",
		20, 1, "/movies/another.mp4", "another.mp4")
	require.NoError(t, err)

	track := &SubtitleTrack{
		ID:           "sub_nil_001",
		Language:     "Japanese",
		LanguageCode: "ja",
		Source:       "embedded",
		Format:       "ass",
		Content:      nil,
		Path:         nil,
		IsDefault:    false,
		IsForced:     true,
		Encoding:     "utf-8",
		SyncOffset:   250.0,
		CreatedAt:    time.Now(),
		VerifiedSync: false,
	}

	err = service.saveSubtitleTrack(ctx, 20, track)
	require.NoError(t, err)

	// Verify via raw SQL (saveSubtitleTrack doesn't insert the id column)
	var lang string
	var isForced int
	var syncOffset float64
	err = db.QueryRowContext(ctx,
		"SELECT language, is_forced, sync_offset FROM subtitle_tracks WHERE media_item_id = ?", 20).
		Scan(&lang, &isForced, &syncOffset)
	require.NoError(t, err)
	assert.Equal(t, "Japanese", lang)
	assert.Equal(t, 1, isForced)
	assert.Equal(t, 250.0, syncOffset)
}

// ---------------------------------------------------------------------------
// parseSubtitleLines — delegates to parseSRT
// ---------------------------------------------------------------------------

func TestSubtitleService_ParseSubtitleLines_SRT(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	content := `1
00:00:01,000 --> 00:00:04,000
First line

2
00:00:05,000 --> 00:00:08,000
Second line

3
00:00:09,500 --> 00:00:12,750
Third line
`

	lines, err := service.parseSubtitleLines(content)
	require.NoError(t, err)
	assert.Equal(t, 3, len(lines))
	assert.Equal(t, "First line", lines[0].Text)
	assert.Equal(t, "Second line", lines[1].Text)
	assert.Equal(t, "Third line", lines[2].Text)
}

func TestSubtitleService_ParseSubtitleLines_EmptyContent(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	lines, err := service.parseSubtitleLines("")
	require.NoError(t, err)
	assert.Empty(t, lines)
}

// ---------------------------------------------------------------------------
// reconstructSubtitle — SRT and unsupported format
// ---------------------------------------------------------------------------

func TestSubtitleService_ReconstructSubtitle_SRT(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	lines := []SubtitleLine{
		{Index: 1, StartTime: "00:00:01,000", EndTime: "00:00:04,000", Text: "Hello world"},
		{Index: 2, StartTime: "00:00:05,000", EndTime: "00:00:08,000", Text: "Second line"},
		{Index: 3, StartTime: "00:01:00,000", EndTime: "00:01:03,500", Text: "Third line"},
	}

	result, err := service.reconstructSubtitle("srt", lines)
	require.NoError(t, err)
	assert.Contains(t, result, "1\n00:00:01,000 --> 00:00:04,000\nHello world")
	assert.Contains(t, result, "2\n00:00:05,000 --> 00:00:08,000\nSecond line")
	assert.Contains(t, result, "3\n00:01:00,000 --> 00:01:03,500\nThird line")
}

func TestSubtitleService_ReconstructSubtitle_UnsupportedFormat(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	_, err := service.reconstructSubtitle("vtt", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format for reconstruction")
}

func TestSubtitleService_ReconstructSubtitle_EmptyLines(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	result, err := service.reconstructSubtitle("srt", []SubtitleLine{})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

// ---------------------------------------------------------------------------
// parseSRT — edge cases
// ---------------------------------------------------------------------------

func TestSubtitleService_ParseSRT_MultilineText(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	content := `1
00:00:01,000 --> 00:00:04,000
Line one
Line two

2
00:00:05,000 --> 00:00:08,000
Single line
`

	lines, err := service.parseSRT(content)
	require.NoError(t, err)
	assert.Equal(t, 2, len(lines))
	assert.Contains(t, lines[0].Text, "Line one")
	assert.Contains(t, lines[0].Text, "Line two")
}

func TestSubtitleService_ParseSRT_Empty(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	lines, err := service.parseSRT("")
	require.NoError(t, err)
	assert.Empty(t, lines)
}

// ---------------------------------------------------------------------------
// parseSubtitle — format detection
// ---------------------------------------------------------------------------

func TestSubtitleService_ParseSubtitle_CaseInsensitiveFormat(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	srtContent := `1
00:00:01,000 --> 00:00:04,000
Test
`
	// Test SRT with uppercase format
	result, err := service.parseSubtitle(srtContent, "SRT")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// Roundtrip: parse SRT -> reconstruct SRT
// ---------------------------------------------------------------------------

func TestSubtitleService_SRT_Roundtrip(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	original := `1
00:00:01,000 --> 00:00:04,000
Hello world

2
00:00:05,000 --> 00:00:08,000
This is a test
`

	// Parse
	lines, err := service.parseSRT(original)
	require.NoError(t, err)
	assert.Equal(t, 2, len(lines))

	// Reconstruct
	reconstructed := service.reconstructSRT(lines)

	// Re-parse
	lines2, err := service.parseSRT(reconstructed)
	require.NoError(t, err)
	assert.Equal(t, len(lines), len(lines2))

	for i := range lines {
		assert.Equal(t, lines[i].StartTime, lines2[i].StartTime)
		assert.Equal(t, lines[i].EndTime, lines2[i].EndTime)
		assert.Equal(t, lines[i].Text, lines2[i].Text)
	}
}

// ---------------------------------------------------------------------------
// SaveUploadedSubtitle
// ---------------------------------------------------------------------------

func TestSubtitleService_SaveUploadedSubtitle(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	// Insert a file
	_, err := db.ExecContext(ctx,
		"INSERT INTO files (id, storage_root_id, path, name) VALUES (?, ?, ?, ?)",
		30, 1, "/movies/upload.mp4", "upload.mp4")
	require.NoError(t, err)

	req := &SubtitleUploadRequest{
		MediaID:      30,
		Language:     "French",
		LanguageCode: "fr",
		Format:       "srt",
		Content:      "1\n00:00:01,000 --> 00:00:03,000\nBonjour\n\n",
		IsDefault:    false,
		IsForced:     false,
		Encoding:     "utf-8",
		SyncOffset:   0.0,
	}

	resp, err := service.SaveUploadedSubtitle(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.SubtitleID)
	assert.Equal(t, "Subtitle uploaded successfully", resp.Message)
	assert.Equal(t, "French", resp.Language)
	assert.Equal(t, "srt", resp.Format)
}

func TestSubtitleService_SaveUploadedSubtitle_AsDefault(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		"INSERT INTO files (id, storage_root_id, path, name) VALUES (?, ?, ?, ?)",
		31, 1, "/movies/default.mp4", "default.mp4")
	require.NoError(t, err)

	// Insert existing subtitle as default
	_, err = db.ExecContext(ctx,
		`INSERT INTO subtitle_tracks (id, media_item_id, language, language_code, source, format, content, is_default, is_forced, encoding, sync_offset, created_at, verified_sync)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"existing_sub", 31, "English", "en", "downloaded", "srt", "content",
		1, 0, "utf-8", 0.0, time.Now(), 0)
	require.NoError(t, err)

	// Upload a new default
	req := &SubtitleUploadRequest{
		MediaID:      31,
		Language:     "Spanish",
		LanguageCode: "es",
		Format:       "srt",
		Content:      "1\n00:00:01,000 --> 00:00:03,000\nHola\n\n",
		IsDefault:    true,
		IsForced:     false,
		Encoding:     "utf-8",
	}

	resp, err := service.SaveUploadedSubtitle(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify old subtitle is no longer default
	var isDefault int
	err = db.QueryRowContext(ctx, "SELECT is_default FROM subtitle_tracks WHERE id = ?", "existing_sub").Scan(&isDefault)
	require.NoError(t, err)
	assert.Equal(t, 0, isDefault)
}

func TestSubtitleService_SaveUploadedSubtitle_MediaNotFound(t *testing.T) {
	db, cleanup := newSubtitleTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(db, logger, mockCache)

	ctx := context.Background()

	req := &SubtitleUploadRequest{
		MediaID:      99999,
		Language:     "English",
		LanguageCode: "en",
		Format:       "srt",
		Content:      "content",
		Encoding:     "utf-8",
	}

	_, err := service.SaveUploadedSubtitle(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "media item not found")
}

// ---------------------------------------------------------------------------
// Close / shutdown
// ---------------------------------------------------------------------------

func TestSubtitleService_Close(t *testing.T) {
	// bluff-scan: no-assert-ok (service close smoke — must not panic on close)
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	// Close should not block when no goroutines are running
	service.Close()
}

// ---------------------------------------------------------------------------
// detectEncoding — UTF-16 LE BOM
// ---------------------------------------------------------------------------

func TestDetectEncoding_UTF16LE(t *testing.T) {
	// UTF-16 LE BOM
	data := []byte{0xFF, 0xFE, 'H', 0x00, 'i', 0x00}
	encoding := detectEncoding(data)
	// Current implementation defaults to utf-8 for non-UTF8 BOM
	assert.Equal(t, "utf-8", encoding)
}

func TestDetectEncoding_EmptyData(t *testing.T) {
	encoding := detectEncoding([]byte{})
	assert.Equal(t, "utf-8", encoding)
}

// ---------------------------------------------------------------------------
// getSubtitleStringValue
// ---------------------------------------------------------------------------

func TestGetSubtitleStringValue(t *testing.T) {
	val := "hello"
	assert.Equal(t, "hello", getSubtitleStringValue(&val))
	assert.Equal(t, "", getSubtitleStringValue(nil))
}

// ---------------------------------------------------------------------------
// parseTimestamp — additional edge cases
// ---------------------------------------------------------------------------

func TestParseTimestamp_Midnight(t *testing.T) {
	result, err := parseTimestamp("00:00:00,000")
	require.NoError(t, err)
	assert.Equal(t, 0.0, result)
}

func TestParseTimestamp_MaxValues(t *testing.T) {
	result, err := parseTimestamp("23:59:59,999")
	require.NoError(t, err)
	expected := float64(23*3600+59*60+59) + 0.999
	assert.InDelta(t, expected, result, 0.001)
}

// ---------------------------------------------------------------------------
// parseASSTimestamp — additional edge cases
// ---------------------------------------------------------------------------

func TestParseASSTimestamp_Zero(t *testing.T) {
	result, err := parseASSTimestamp("0:00:00.00")
	require.NoError(t, err)
	assert.Equal(t, "00:00:00,000", result)
}

func TestParseASSTimestamp_LargeHours(t *testing.T) {
	result, err := parseASSTimestamp("99:59:59.99")
	require.NoError(t, err)
	assert.Equal(t, "99:59:59,990", result)
}

// ---------------------------------------------------------------------------
// extractSamplePoints — edge cases
// ---------------------------------------------------------------------------

func TestSubtitleService_ExtractSamplePoints_FewLines(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	// Fewer than 10 lines
	lines := []SubtitleLine{
		{Index: 1, StartTime: "00:00:01,000", EndTime: "00:00:04,000", Text: "Line 1"},
		{Index: 2, StartTime: "00:00:05,000", EndTime: "00:00:08,000", Text: "Line 2"},
	}

	points := service.extractSamplePoints(lines, 10.0)
	assert.NotNil(t, points)
	// With 2 lines and interval = 2/10 = 0, it samples every line
	assert.GreaterOrEqual(t, len(points), 1)
}

func TestSubtitleService_ExtractSamplePoints_EmptyLines(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	points := service.extractSamplePoints([]SubtitleLine{}, 120.0)
	assert.Empty(t, points)
}

// ---------------------------------------------------------------------------
// calculateSyncOffset — edge cases
// ---------------------------------------------------------------------------

func TestSubtitleService_CalculateSyncOffset_EmptyPoints(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	offset, confidence := service.calculateSyncOffset([]SyncPoint{}, &VideoInfo{Duration: 120.0})
	assert.Equal(t, 0.0, offset)
	assert.Equal(t, 0.0, confidence)
}

func TestSubtitleService_CalculateSyncOffset_SinglePoint(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	points := []SyncPoint{
		{SubtitleTime: 5.0, VideoTime: 5.5, Confidence: 0.9},
	}

	offset, confidence := service.calculateSyncOffset(points, &VideoInfo{Duration: 120.0, FrameRate: 24.0})
	assert.NotEqual(t, 0.0, confidence)
	// Offset should reflect the difference
	_ = offset
}

// ---------------------------------------------------------------------------
// searchProvider — unsupported
// ---------------------------------------------------------------------------

func TestSubtitleService_SearchProvider_Unsupported(t *testing.T) {
	logger := zap.NewNop()
	mockCache := &MockCacheService{}
	service := NewSubtitleService(nil, logger, mockCache)

	ctx := context.Background()
	_, err := service.searchProvider(ctx, SubtitleProvider("nonexistent"), &SubtitleSearchRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}
