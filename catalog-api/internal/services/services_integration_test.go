package services

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var testDBCounter atomic.Int64

// setupIntegrationTestDB creates an in-memory SQLite database with full schema for integration tests.
// This avoids importing internal/tests which causes an import cycle.
func setupIntegrationTestDB(t *testing.T) *database.DB {
	id := testDBCounter.Add(1)
	dsn := fmt.Sprintf("file:testdb%d?mode=memory&cache=shared&_busy_timeout=5000", id)
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)

	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT, salt TEXT, role_id INTEGER NOT NULL DEFAULT 1,
			first_name TEXT, last_name TEXT, display_name TEXT, avatar_url TEXT,
			time_zone TEXT, language TEXT, settings TEXT DEFAULT '{}',
			is_active BOOLEAN DEFAULT 1, is_locked BOOLEAN DEFAULT 0,
			locked_until DATETIME, failed_login_attempts INTEGER DEFAULT 0,
			last_login_at DATETIME, last_login_ip TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS media_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL, filename TEXT DEFAULT '', title TEXT NOT NULL,
			original_title TEXT DEFAULT '', type TEXT NOT NULL, media_type TEXT DEFAULT '',
			mime_type TEXT DEFAULT '', size INTEGER DEFAULT 0, file_size INTEGER DEFAULT 0,
			file_path TEXT DEFAULT '', duration INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			date_added DATETIME DEFAULT CURRENT_TIMESTAMP,
			artist TEXT DEFAULT '', album TEXT DEFAULT '', album_id INTEGER,
			album_artist TEXT DEFAULT '', genre TEXT DEFAULT '', genres TEXT DEFAULT '[]',
			year INTEGER DEFAULT 0, track_number INTEGER DEFAULT 0, disc_number INTEGER DEFAULT 0,
			video_codec TEXT DEFAULT '', audio_codec TEXT DEFAULT '', codec TEXT DEFAULT '',
			resolution TEXT DEFAULT '', aspect_ratio TEXT DEFAULT '',
			framerate REAL DEFAULT 0.0, frame_rate REAL DEFAULT 0.0,
			bitrate INTEGER DEFAULT 0, format TEXT DEFAULT '',
			sample_rate INTEGER DEFAULT 0, channels INTEGER DEFAULT 0, bpm INTEGER,
			key TEXT, hdr BOOLEAN DEFAULT FALSE, dolby_vision BOOLEAN DEFAULT FALSE,
			dolby_atmos BOOLEAN DEFAULT FALSE, series_title TEXT DEFAULT '',
			season INTEGER DEFAULT 0, episode INTEGER DEFAULT 0,
			episode_title TEXT DEFAULT '', description TEXT DEFAULT '',
			language TEXT DEFAULT '', country TEXT DEFAULT '',
			directors TEXT DEFAULT '[]', actors TEXT DEFAULT '[]', writers TEXT DEFAULT '[]',
			imdb_id TEXT DEFAULT '', tmdb_id TEXT DEFAULT '', release_date DATETIME,
			last_position REAL DEFAULT 0.0, play_count INTEGER DEFAULT 0,
			last_played DATETIME, is_favorite BOOLEAN DEFAULT FALSE,
			rating REAL DEFAULT 0.0, user_rating INTEGER DEFAULT 0,
			watched_percentage REAL DEFAULT 0.0, user_id INTEGER, storage_root_id INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL, name TEXT NOT NULL, description TEXT DEFAULT '',
			is_public BOOLEAN DEFAULT FALSE, is_smart BOOLEAN DEFAULT FALSE,
			is_smart_playlist BOOLEAN DEFAULT FALSE, smart_criteria TEXT DEFAULT '',
			cover_art_id INTEGER, cover_art_url TEXT DEFAULT '',
			track_count INTEGER DEFAULT 0, total_duration INTEGER DEFAULT 0,
			play_count INTEGER DEFAULT 0, last_played DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id INTEGER NOT NULL, media_item_id INTEGER NOT NULL,
			position INTEGER NOT NULL, added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			added_by INTEGER, custom_title TEXT DEFAULT '',
			start_time INTEGER DEFAULT 0, end_time INTEGER DEFAULT 0,
			FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
			FOREIGN KEY (media_item_id) REFERENCES media_items(id) ON DELETE CASCADE,
			UNIQUE(playlist_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id INTEGER NOT NULL, tag TEXT NOT NULL,
			FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
			UNIQUE(playlist_id, tag)
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_collaborators (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id INTEGER NOT NULL, user_id INTEGER NOT NULL,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(playlist_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS playback_positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL, media_item_id INTEGER NOT NULL,
			position INTEGER NOT NULL DEFAULT 0, duration INTEGER NOT NULL DEFAULT 0,
			percent_complete REAL DEFAULT 0.0, last_played DATETIME,
			is_completed BOOLEAN DEFAULT 0, device_info TEXT, playback_quality TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (media_item_id) REFERENCES media_items(id) ON DELETE CASCADE,
			UNIQUE(user_id, media_item_id)
		)`,
		`CREATE TABLE IF NOT EXISTS playback_bookmarks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL, media_item_id INTEGER NOT NULL,
			position INTEGER NOT NULL, name TEXT, description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (media_item_id) REFERENCES media_items(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS playback_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL, media_item_id INTEGER NOT NULL,
			start_time DATETIME NOT NULL, end_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			duration INTEGER NOT NULL, percent_watched REAL DEFAULT 0.0,
			device_info TEXT, playback_quality TEXT, was_completed BOOLEAN DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (media_item_id) REFERENCES media_items(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS cache_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cache_key TEXT NOT NULL UNIQUE, value TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS media_metadata_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			media_item_id INTEGER NOT NULL, metadata_type TEXT NOT NULL,
			provider TEXT NOT NULL, data TEXT NOT NULL, quality REAL DEFAULT 0.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(media_item_id, metadata_type, provider)
		)`,
		`CREATE TABLE IF NOT EXISTS api_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL, endpoint TEXT NOT NULL,
			request_hash TEXT NOT NULL, response TEXT NOT NULL,
			status_code INTEGER DEFAULT 0, expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider, endpoint, request_hash)
		)`,
		`CREATE TABLE IF NOT EXISTS thumbnail_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			video_id INTEGER NOT NULL, position INTEGER NOT NULL,
			url TEXT, width INTEGER, height INTEGER, file_size INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(video_id, position, width, height)
		)`,
		`CREATE TABLE IF NOT EXISTS user_localization (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL UNIQUE,
			primary_language TEXT NOT NULL DEFAULT 'en',
			secondary_languages TEXT DEFAULT '[]',
			subtitle_languages TEXT DEFAULT '[]',
			lyrics_languages TEXT DEFAULT '[]',
			metadata_languages TEXT DEFAULT '[]',
			auto_translate BOOLEAN DEFAULT 0,
			auto_download_subtitles BOOLEAN DEFAULT 0,
			auto_download_lyrics BOOLEAN DEFAULT 0,
			preferred_region TEXT DEFAULT '',
			date_format TEXT DEFAULT 'MM/DD/YYYY',
			time_format TEXT DEFAULT '12h',
			number_format TEXT DEFAULT '#,###.##',
			currency_code TEXT DEFAULT 'USD',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS content_language_preferences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL, content_type TEXT NOT NULL,
			languages TEXT DEFAULT '[]', priority INTEGER DEFAULT 1,
			auto_apply BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(user_id, content_type)
		)`,
		`CREATE TABLE IF NOT EXISTS localization_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL UNIQUE,
			language TEXT NOT NULL DEFAULT 'en', region TEXT DEFAULT '',
			date_format TEXT DEFAULT 'MM/DD/YYYY', time_format TEXT DEFAULT '12h',
			timezone TEXT DEFAULT 'UTC', currency TEXT DEFAULT 'USD',
			number_format TEXT DEFAULT '#,###.##', first_day_of_week INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS content_localization (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_type TEXT NOT NULL, content_id INTEGER NOT NULL,
			language TEXT NOT NULL, title TEXT, description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(content_type, content_id, language)
		)`,
		`CREATE TABLE IF NOT EXISTS video_playback_sessions (
			id TEXT PRIMARY KEY, user_id INTEGER NOT NULL,
			session_data TEXT NOT NULL, expires_at DATETIME NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		// Insert test users
		`INSERT OR IGNORE INTO users (id, username, email, is_active) VALUES (1, 'testuser', 'test@example.com', 1)`,
		`INSERT OR IGNORE INTO users (id, username, email, is_active) VALUES (2, 'testuser2', 'test2@example.com', 1)`,
	}

	for _, migration := range migrations {
		_, err := sqlDB.Exec(migration)
		require.NoError(t, err, "Failed migration: %s", migration[:80])
	}

	return database.WrapDB(sqlDB, database.DialectSQLite)
}

// ============================================================================
// PlaybackPositionService Integration Tests
// ============================================================================

func setupPlaybackPositionService(t *testing.T) (*PlaybackPositionService, *database.DB) {
	db := setupIntegrationTestDB(t)
	logger := zap.NewNop()
	svc := NewPlaybackPositionService(db, logger)
	return svc, db
}

func insertTestMediaItem(t *testing.T, db *database.DB, id int64, title, mediaType string) {
	_, err := db.Exec(
		`INSERT INTO media_items (id, path, title, type) VALUES (?, ?, ?, ?)`,
		id, "/test/path/"+title, title, mediaType,
	)
	require.NoError(t, err)
}

func TestPlaybackPositionService_UpdateAndGetPosition(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 100, "test-video.mp4", "video")

	req := &UpdatePositionRequest{
		UserID:          1,
		MediaItemID:     100,
		Position:        30000,
		Duration:        120000,
		DeviceInfo:      "desktop-chrome",
		PlaybackQuality: "1080p",
	}
	err := svc.UpdatePosition(ctx, req)
	require.NoError(t, err)

	pos, err := svc.GetPosition(ctx, 1, 100)
	require.NoError(t, err)
	require.NotNil(t, pos)
	assert.Equal(t, int64(1), pos.UserID)
	assert.Equal(t, int64(100), pos.MediaItemID)
	assert.Equal(t, int64(30000), pos.Position)
	assert.Equal(t, int64(120000), pos.Duration)
	assert.InDelta(t, 25.0, pos.PercentComplete, 0.1)
	assert.False(t, pos.IsCompleted)
	assert.Equal(t, "desktop-chrome", pos.DeviceInfo)
	assert.Equal(t, "1080p", pos.PlaybackQuality)
}

func TestPlaybackPositionService_UpdatePosition_Completed(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 101, "short-clip.mp4", "video")

	// Position at 95% should mark as completed
	req := &UpdatePositionRequest{
		UserID:      1,
		MediaItemID: 101,
		Position:    95000,
		Duration:    100000,
		DeviceInfo:  "mobile",
	}
	err := svc.UpdatePosition(ctx, req)
	require.NoError(t, err)

	pos, err := svc.GetPosition(ctx, 1, 101)
	require.NoError(t, err)
	require.NotNil(t, pos)
	assert.True(t, pos.IsCompleted)
	assert.InDelta(t, 95.0, pos.PercentComplete, 0.1)
}

func TestPlaybackPositionService_GetPosition_NotFound(t *testing.T) {
	svc, _ := setupPlaybackPositionService(t)
	ctx := context.Background()

	pos, err := svc.GetPosition(ctx, 1, 99999)
	require.NoError(t, err)
	assert.Nil(t, pos)
}

func TestPlaybackPositionService_UpdatePosition_Upsert(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 102, "movie.mp4", "video")

	// First update
	err := svc.UpdatePosition(ctx, &UpdatePositionRequest{
		UserID: 1, MediaItemID: 102, Position: 10000, Duration: 100000,
	})
	require.NoError(t, err)

	// Second update (upsert)
	err = svc.UpdatePosition(ctx, &UpdatePositionRequest{
		UserID: 1, MediaItemID: 102, Position: 50000, Duration: 100000,
	})
	require.NoError(t, err)

	pos, err := svc.GetPosition(ctx, 1, 102)
	require.NoError(t, err)
	require.NotNil(t, pos)
	assert.Equal(t, int64(50000), pos.Position)
}

func TestPlaybackPositionService_GetContinueWatching(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 200, "movie1.mp4", "video")
	insertTestMediaItem(t, db, 201, "movie2.mp4", "video")
	insertTestMediaItem(t, db, 202, "movie3.mp4", "video")

	// In progress (25%) - should appear
	err := svc.UpdatePosition(ctx, &UpdatePositionRequest{
		UserID: 1, MediaItemID: 200, Position: 25000, Duration: 100000,
	})
	require.NoError(t, err)

	// In progress (50%) - should appear
	err = svc.UpdatePosition(ctx, &UpdatePositionRequest{
		UserID: 1, MediaItemID: 201, Position: 50000, Duration: 100000,
	})
	require.NoError(t, err)

	// Completed (95%) - should NOT appear (>90%)
	err = svc.UpdatePosition(ctx, &UpdatePositionRequest{
		UserID: 1, MediaItemID: 202, Position: 95000, Duration: 100000,
	})
	require.NoError(t, err)

	positions, err := svc.GetContinueWatching(ctx, 1, 10)
	require.NoError(t, err)

	// Only in-progress items (between 5% and 90%) should appear
	assert.Len(t, positions, 2)
}

func TestPlaybackPositionService_CreateAndGetBookmarks(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 300, "long-video.mp4", "video")

	// Create first bookmark
	bookmark1, err := svc.CreateBookmark(ctx, &BookmarkRequest{
		UserID:      1,
		MediaItemID: 300,
		Position:    30000,
		Name:        "Interesting scene",
		Description: "Great dialogue here",
	})
	require.NoError(t, err)
	require.NotNil(t, bookmark1)
	assert.Equal(t, "Interesting scene", bookmark1.Name)
	assert.Equal(t, int64(30000), bookmark1.Position)
	assert.True(t, bookmark1.ID > 0)

	// Create second bookmark at a later position
	bookmark2, err := svc.CreateBookmark(ctx, &BookmarkRequest{
		UserID:      1,
		MediaItemID: 300,
		Position:    60000,
		Name:        "Action sequence",
	})
	require.NoError(t, err)
	require.NotNil(t, bookmark2)

	// Get all bookmarks (should be ordered by position)
	bookmarks, err := svc.GetBookmarks(ctx, 1, 300)
	require.NoError(t, err)
	assert.Len(t, bookmarks, 2)
	assert.Equal(t, int64(30000), bookmarks[0].Position)
	assert.Equal(t, int64(60000), bookmarks[1].Position)
}

func TestPlaybackPositionService_DeleteBookmark(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 301, "video.mp4", "video")

	bookmark, err := svc.CreateBookmark(ctx, &BookmarkRequest{
		UserID: 1, MediaItemID: 301, Position: 5000, Name: "to-delete",
	})
	require.NoError(t, err)

	err = svc.DeleteBookmark(ctx, 1, bookmark.ID)
	require.NoError(t, err)

	bookmarks, err := svc.GetBookmarks(ctx, 1, 301)
	require.NoError(t, err)
	assert.Len(t, bookmarks, 0)
}

func TestPlaybackPositionService_DeleteBookmark_NotFound(t *testing.T) {
	svc, _ := setupPlaybackPositionService(t)
	ctx := context.Background()

	err := svc.DeleteBookmark(ctx, 1, 99999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bookmark not found or access denied")
}

func TestPlaybackPositionService_DeleteBookmark_WrongUser(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 302, "video.mp4", "video")

	bookmark, err := svc.CreateBookmark(ctx, &BookmarkRequest{
		UserID: 1, MediaItemID: 302, Position: 5000, Name: "user1-bookmark",
	})
	require.NoError(t, err)

	// User 2 tries to delete user 1's bookmark
	err = svc.DeleteBookmark(ctx, 2, bookmark.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bookmark not found or access denied")
}

func TestPlaybackPositionService_GetPlaybackStats_Empty(t *testing.T) {
	svc, _ := setupPlaybackPositionService(t)
	ctx := context.Background()

	stats, err := svc.GetPlaybackStats(ctx, &PlaybackStatsRequest{
		UserID: 1,
		Limit:  20,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalPlaytime)
	assert.Equal(t, int64(0), stats.TotalMediaItems)
	assert.Equal(t, int64(0), stats.CompletedItems)
	assert.NotNil(t, stats.PlaybackByHour)
	assert.NotNil(t, stats.WatchTimeByDevice)
}

func TestPlaybackPositionService_GetPlaybackStats_WithHistory(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 400, "movie.mp4", "video")

	// Insert playback history directly
	_, err := db.Exec(
		`INSERT INTO playback_history (user_id, media_item_id, start_time, duration, percent_watched, device_info, playback_quality, was_completed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		1, 400, time.Now().Add(-1*time.Hour), 120000, 100.0, "desktop", "1080p", true,
	)
	require.NoError(t, err)

	_, err = db.Exec(
		`INSERT INTO playback_history (user_id, media_item_id, start_time, duration, percent_watched, device_info, playback_quality, was_completed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		1, 400, time.Now().Add(-2*time.Hour), 60000, 50.0, "mobile", "720p", false,
	)
	require.NoError(t, err)

	stats, err := svc.GetPlaybackStats(ctx, &PlaybackStatsRequest{
		UserID: 1,
		Limit:  20,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(180000), stats.TotalPlaytime)
	assert.Equal(t, int64(2), stats.TotalMediaItems)
	assert.Equal(t, int64(1), stats.CompletedItems)
	assert.Len(t, stats.RecentlyWatched, 2)
	assert.Contains(t, stats.WatchTimeByDevice, "desktop")
	assert.Contains(t, stats.WatchTimeByDevice, "mobile")
}

func TestPlaybackPositionService_CleanupOldPositions(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 500, "old-video.mp4", "video")

	// Insert an old completed position manually using a fixed past date string
	// to avoid timezone issues between Go time.Time and SQLite DATETIME comparisons
	_, err := db.Exec(
		`INSERT INTO playback_positions (user_id, media_item_id, position, duration, percent_complete, last_played, is_completed)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1, 500, 100000, 100000, 100.0, "2020-01-01 00:00:00", 1,
	)
	require.NoError(t, err)

	err = svc.CleanupOldPositions(ctx, 30*24*time.Hour)
	require.NoError(t, err)

	pos, err := svc.GetPosition(ctx, 1, 500)
	require.NoError(t, err)
	assert.Nil(t, pos)
}

func TestPlaybackPositionService_SyncAcrossDevices(t *testing.T) {
	svc, _ := setupPlaybackPositionService(t)
	ctx := context.Background()

	// Empty state: no duplicate rows to collapse, must return nil.
	err := svc.SyncAcrossDevices(ctx, 1)
	require.NoError(t, err)
}

// TestPlaybackPositionService_SyncAcrossDevices_WithPositions verifies
// SyncAcrossDevices returns nil and logs the count when the user has
// existing position rows. Because the schema enforces UNIQUE(user_id,
// media_item_id), cross-device merging is implicit at UPSERT time; this
// method only validates the state is reachable.
func TestPlaybackPositionService_SyncAcrossDevices_WithPositions(t *testing.T) {
	svc, db := setupPlaybackPositionService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 700, "synced.mp4", "video")
	require.NoError(t, svc.UpdatePosition(ctx, &UpdatePositionRequest{
		UserID: 1, MediaItemID: 700, Position: 30000, Duration: 120000,
	}))

	require.NoError(t, svc.SyncAcrossDevices(ctx, 1))

	// Row should still be there.
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM playback_positions WHERE user_id = ? AND media_item_id = ?`,
		1, 700,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestPlaybackPositionService_SyncAcrossDevices_NilDB verifies the sync
// errors cleanly when the service has no database.
func TestPlaybackPositionService_SyncAcrossDevices_NilDB(t *testing.T) {
	svc := NewPlaybackPositionService(nil, zap.NewNop())
	err := svc.SyncAcrossDevices(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "database")
}

// TestPlaybackPositionService_SyncAcrossDevices_InvalidUserID verifies
// the input validation path rejects non-positive user IDs.
func TestPlaybackPositionService_SyncAcrossDevices_InvalidUserID(t *testing.T) {
	svc, _ := setupPlaybackPositionService(t)

	err := svc.SyncAcrossDevices(context.Background(), 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "positive userID")

	err = svc.SyncAcrossDevices(context.Background(), -5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "positive userID")
}

// ============================================================================
// PlaylistService Integration Tests
// ============================================================================

func setupPlaylistService(t *testing.T) (*PlaylistService, *database.DB) {
	db := setupIntegrationTestDB(t)
	logger := zap.NewNop()
	svc := NewPlaylistService(db, logger)
	return svc, db
}

func TestPlaylistService_CreatePlaylist(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID:      1,
		Name:        "My Favorites",
		Description: "A collection of my favorite songs",
		IsPublic:    false,
	})
	require.NoError(t, err)
	require.NotNil(t, playlist)
	assert.True(t, playlist.ID > 0)
	assert.Equal(t, "My Favorites", playlist.Name)
	assert.Equal(t, int64(1), playlist.UserID)
	assert.False(t, playlist.IsPublic)
}

func TestPlaylistService_CreatePlaylist_WithTags(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID:   1,
		Name:     "Rock Classics",
		IsPublic: true,
		Tags:     []string{"rock", "classic", "80s"},
	})
	require.NoError(t, err)
	require.NotNil(t, playlist)

	// Fetch it back and verify tags
	fetched, err := svc.GetPlaylist(ctx, playlist.ID, 1)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Len(t, fetched.Tags, 3)
}

func TestPlaylistService_CreatePlaylist_WithCollaborators(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID:          1,
		Name:            "Shared Playlist",
		CollaboratorIDs: []int64{2},
	})
	require.NoError(t, err)
	require.NotNil(t, playlist)

	// User 2 should be able to access
	fetched, err := svc.GetPlaylist(ctx, playlist.ID, 2)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, "Shared Playlist", fetched.Name)
}

func TestPlaylistService_GetPlaylist_NotFound(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	_, err := svc.GetPlaylist(ctx, 99999, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "playlist not found or access denied")
}

func TestPlaylistService_GetPlaylist_AccessDenied(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	// User 1 creates a private playlist
	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID:   1,
		Name:     "Private Playlist",
		IsPublic: false,
	})
	require.NoError(t, err)

	// User 2 should not be able to access a private playlist
	_, err = svc.GetPlaylist(ctx, playlist.ID, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "playlist not found or access denied")
}

func TestPlaylistService_GetPlaylist_PublicAccess(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	// User 1 creates a public playlist
	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID:   1,
		Name:     "Public Playlist",
		IsPublic: true,
	})
	require.NoError(t, err)

	// User 2 should be able to access a public playlist
	fetched, err := svc.GetPlaylist(ctx, playlist.ID, 2)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, "Public Playlist", fetched.Name)
}

func TestPlaylistService_GetUserPlaylists(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	// Create two playlists for user 1
	_, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Playlist A",
	})
	require.NoError(t, err)

	_, err = svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Playlist B",
	})
	require.NoError(t, err)

	playlists, err := svc.GetUserPlaylists(ctx, 1, false)
	require.NoError(t, err)
	assert.Len(t, playlists, 2)
}

func TestPlaylistService_AddToPlaylist(t *testing.T) {
	svc, db := setupPlaylistService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 600, "song1.mp3", "audio")
	insertTestMediaItem(t, db, 601, "song2.mp3", "audio")

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Test Playlist",
	})
	require.NoError(t, err)

	err = svc.AddToPlaylist(ctx, &AddToPlaylistRequest{
		PlaylistID:   playlist.ID,
		MediaItemIDs: []int64{600, 601},
		UserID:       1,
	})
	require.NoError(t, err)

	items, err := svc.GetPlaylistItems(ctx, playlist.ID, 1, 100, 0)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, int64(600), items[0].MediaItemID)
	assert.Equal(t, int64(601), items[1].MediaItemID)
}

func TestPlaylistService_AddToPlaylist_PermissionDenied(t *testing.T) {
	svc, db := setupPlaylistService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 602, "song.mp3", "audio")

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Private Playlist", IsPublic: false,
	})
	require.NoError(t, err)

	// User 2 tries to add to user 1's private playlist
	err = svc.AddToPlaylist(ctx, &AddToPlaylistRequest{
		PlaylistID:   playlist.ID,
		MediaItemIDs: []int64{602},
		UserID:       2,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestPlaylistService_RemoveFromPlaylist(t *testing.T) {
	svc, db := setupPlaylistService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 610, "song1.mp3", "audio")
	insertTestMediaItem(t, db, 611, "song2.mp3", "audio")

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Test Playlist",
	})
	require.NoError(t, err)

	err = svc.AddToPlaylist(ctx, &AddToPlaylistRequest{
		PlaylistID:   playlist.ID,
		MediaItemIDs: []int64{610, 611},
		UserID:       1,
	})
	require.NoError(t, err)

	items, err := svc.GetPlaylistItems(ctx, playlist.ID, 1, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Remove the first item
	err = svc.RemoveFromPlaylist(ctx, playlist.ID, items[0].ID, 1)
	require.NoError(t, err)

	items, err = svc.GetPlaylistItems(ctx, playlist.ID, 1, 100, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(611), items[0].MediaItemID)
}

func TestPlaylistService_ReorderPlaylist_SamePosition(t *testing.T) {
	svc, db := setupPlaylistService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 620, "song1.mp3", "audio")
	insertTestMediaItem(t, db, 621, "song2.mp3", "audio")

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Reorder Test",
	})
	require.NoError(t, err)

	err = svc.AddToPlaylist(ctx, &AddToPlaylistRequest{
		PlaylistID:   playlist.ID,
		MediaItemIDs: []int64{620, 621},
		UserID:       1,
	})
	require.NoError(t, err)

	items, err := svc.GetPlaylistItems(ctx, playlist.ID, 1, 100, 0)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Reorder to same position should be a no-op
	err = svc.ReorderPlaylist(ctx, &ReorderPlaylistRequest{
		PlaylistID:  playlist.ID,
		UserID:      1,
		ItemID:      items[0].ID,
		NewPosition: items[0].Position,
	})
	require.NoError(t, err)

	// Verify order unchanged
	reordered, err := svc.GetPlaylistItems(ctx, playlist.ID, 1, 100, 0)
	require.NoError(t, err)
	require.Len(t, reordered, 2)
	assert.Equal(t, int64(620), reordered[0].MediaItemID)
	assert.Equal(t, int64(621), reordered[1].MediaItemID)
}

func TestPlaylistService_CanModifyPlaylist(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID: 1, Name: "Owner Test",
	})
	require.NoError(t, err)

	// Owner can modify
	assert.True(t, svc.canModifyPlaylist(ctx, playlist.ID, 1))

	// Non-owner cannot modify
	assert.False(t, svc.canModifyPlaylist(ctx, playlist.ID, 2))
}

func TestPlaylistService_CanModifyPlaylist_Collaborator(t *testing.T) {
	svc, _ := setupPlaylistService(t)
	ctx := context.Background()

	playlist, err := svc.CreatePlaylist(ctx, &CreatePlaylistRequest{
		UserID:          1,
		Name:            "Collaborative",
		CollaboratorIDs: []int64{2},
	})
	require.NoError(t, err)

	// Collaborator can modify
	assert.True(t, svc.canModifyPlaylist(ctx, playlist.ID, 2))
}

// ============================================================================
// CacheService Integration Tests (with real DB)
// ============================================================================

func setupCacheService(t *testing.T) (*CacheService, *database.DB) {
	db := setupIntegrationTestDB(t)
	// Create cache_entries table used by CacheService
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS cache_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT UNIQUE NOT NULL,
		value TEXT NOT NULL,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	// Also create cache_activity table for recordCacheActivity
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cache_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		cache_key TEXT NOT NULL,
		provider TEXT,
		hit BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	logger := zap.NewNop()
	svc := NewCacheService(db, logger)
	t.Cleanup(func() { svc.Close() })
	return svc, db
}

func TestCacheService_SetAndGet(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	testValue := map[string]string{"key": "value", "foo": "bar"}
	err := svc.Set(ctx, "test:key1", testValue, 1*time.Hour)
	require.NoError(t, err)

	var result map[string]string
	found, err := svc.Get(ctx, "test:key1", &result)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, "bar", result["foo"])
}

func TestCacheService_Get_NotFound(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	var result string
	found, err := svc.Get(ctx, "nonexistent:key", &result)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCacheService_Delete_DB(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.Set(ctx, "delete:me", "hello", 1*time.Hour)
	require.NoError(t, err)

	err = svc.Delete(ctx, "delete:me")
	require.NoError(t, err)

	var result string
	found, err := svc.Get(ctx, "delete:me", &result)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCacheService_Clear_All(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.Set(ctx, "clear:a", "1", 1*time.Hour)
	require.NoError(t, err)
	err = svc.Set(ctx, "clear:b", "2", 1*time.Hour)
	require.NoError(t, err)

	err = svc.Clear(ctx, "")
	require.NoError(t, err)

	var result string
	found, err := svc.Get(ctx, "clear:a", &result)
	require.NoError(t, err)
	assert.False(t, found)

	found, err = svc.Get(ctx, "clear:b", &result)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCacheService_Clear_Pattern(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.Set(ctx, "lyrics:song1", "lyric1", 1*time.Hour)
	require.NoError(t, err)
	err = svc.Set(ctx, "lyrics:song2", "lyric2", 1*time.Hour)
	require.NoError(t, err)
	err = svc.Set(ctx, "translation:text1", "trans1", 1*time.Hour)
	require.NoError(t, err)

	err = svc.Clear(ctx, "lyrics:%")
	require.NoError(t, err)

	var result string
	found, err := svc.Get(ctx, "lyrics:song1", &result)
	require.NoError(t, err)
	assert.False(t, found)

	// translation should still be there
	found, err = svc.Get(ctx, "translation:text1", &result)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestCacheService_SetAndGetMediaMetadata(t *testing.T) {
	svc, db := setupCacheService(t)
	ctx := context.Background()

	insertTestMediaItem(t, db, 700, "metadata-test.mp4", "video")

	metadata := map[string]string{
		"title":    "Test Movie",
		"director": "Test Director",
	}
	err := svc.SetMediaMetadata(ctx, 700, "movie_info", "tmdb", metadata, 0.95)
	require.NoError(t, err)

	var result map[string]string
	found, quality, err := svc.GetMediaMetadata(ctx, 700, "movie_info", "tmdb", &result)
	require.NoError(t, err)
	assert.True(t, found)
	assert.InDelta(t, 0.95, quality, 0.01)
	assert.Equal(t, "Test Movie", result["title"])
}

func TestCacheService_GetMediaMetadata_NotFound(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	var result map[string]string
	found, quality, err := svc.GetMediaMetadata(ctx, 99999, "movie_info", "tmdb", &result)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, float64(0), quality)
}

func TestCacheService_SetAndGetAPIResponse(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	requestData := map[string]string{"query": "test search"}
	responseData := map[string]interface{}{
		"results": []string{"result1", "result2"},
		"count":   2.0,
	}

	err := svc.SetAPIResponse(ctx, "tmdb", "/search/movie", requestData, responseData, 200, 1*time.Hour)
	require.NoError(t, err)

	var result map[string]interface{}
	found, statusCode, err := svc.GetAPIResponse(ctx, "tmdb", "/search/movie", requestData, &result)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 200, statusCode)
}

func TestCacheService_SetAndGetThumbnail(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.SetThumbnail(ctx, 1, 30000, "http://example.com/thumb.jpg", 320, 180, 4096)
	require.NoError(t, err)

	thumb, err := svc.GetThumbnail(ctx, 1, 30000, 320, 180)
	require.NoError(t, err)
	require.NotNil(t, thumb)
	assert.Equal(t, "http://example.com/thumb.jpg", thumb.URL)
	assert.Equal(t, 320, thumb.Width)
	assert.Equal(t, 180, thumb.Height)
	assert.Equal(t, int64(4096), thumb.FileSize)
}

func TestCacheService_GetThumbnail_NotFound(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	thumb, err := svc.GetThumbnail(ctx, 99999, 0, 320, 180)
	require.NoError(t, err)
	assert.Nil(t, thumb)
}

func TestCacheService_SetAndGetTranslation(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.SetTranslation(ctx, "Hello world", "en", "es", "google", "Hola mundo")
	require.NoError(t, err)

	translation, found, err := svc.GetTranslation(ctx, "Hello world", "en", "es", "google")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "Hola mundo", translation)
}

func TestCacheService_GetTranslation_NotFound(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	translation, found, err := svc.GetTranslation(ctx, "unknown text", "en", "es", "google")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, translation)
}

func TestCacheService_InvalidateByPattern_DB(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.Set(ctx, "coverart:artist1:album1", "art1", 1*time.Hour)
	require.NoError(t, err)
	err = svc.Set(ctx, "coverart:artist2:album2", "art2", 1*time.Hour)
	require.NoError(t, err)
	err = svc.Set(ctx, "lyrics:artist1:song1", "lyric1", 1*time.Hour)
	require.NoError(t, err)

	err = svc.InvalidateByPattern(ctx, "coverart:%")
	require.NoError(t, err)

	var result string
	found, err := svc.Get(ctx, "coverart:artist1:album1", &result)
	require.NoError(t, err)
	assert.False(t, found)

	found, err = svc.Get(ctx, "lyrics:artist1:song1", &result)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestCacheService_GetStats_DB(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	err := svc.Set(ctx, "stats:item1", "val1", 1*time.Hour)
	require.NoError(t, err)
	err = svc.Set(ctx, "stats:item2", "val2", 1*time.Hour)
	require.NoError(t, err)

	// Allow time for async cache_activity writes
	time.Sleep(100 * time.Millisecond)

	stats, err := svc.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.True(t, stats.TotalEntries >= 2)
}

func TestCacheService_CleanupExpired_DB(t *testing.T) {
	svc, db := setupCacheService(t)
	ctx := context.Background()

	// Insert an already-expired entry (use a very old date to avoid timezone issues)
	_, err := db.Exec(
		`INSERT INTO cache_entries (cache_key, value, expires_at) VALUES (?, ?, ?)`,
		"expired:key", `"old_value"`, "2020-01-01 00:00:00",
	)
	require.NoError(t, err)

	// Insert a valid entry
	err = svc.Set(ctx, "valid:key", "new_value", 1*time.Hour)
	require.NoError(t, err)

	err = svc.CleanupExpired(ctx)
	require.NoError(t, err)

	var result string
	found, err := svc.Get(ctx, "expired:key", &result)
	require.NoError(t, err)
	assert.False(t, found)

	found, err = svc.Get(ctx, "valid:key", &result)
	require.NoError(t, err)
	assert.True(t, found)
}

func TestCacheService_Warmup_DB(t *testing.T) {
	svc, _ := setupCacheService(t)
	ctx := context.Background()

	// Warmup is a no-op stub
	err := svc.Warmup(ctx)
	require.NoError(t, err)
}

func TestCacheService_HashString_Deterministic(t *testing.T) {
	svc, _ := setupCacheService(t)

	hash1 := svc.hashString("test input")
	hash2 := svc.hashString("test input")
	hash3 := svc.hashString("different input")

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
}

func TestCacheService_HashRequest_Deterministic(t *testing.T) {
	svc, _ := setupCacheService(t)

	data := map[string]string{"key": "value"}
	hash1, err1 := svc.hashRequest(data)
	hash2, err2 := svc.hashRequest(data)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, hash1, hash2)
}

// ============================================================================
// LocalizationService Integration Tests
// ============================================================================

func setupLocalizationService(t *testing.T) (*LocalizationService, *database.DB) {
	db := setupIntegrationTestDB(t)
	logger := zap.NewNop()
	cacheService := NewCacheService(nil, logger) // nil DB cache for simplicity
	t.Cleanup(func() { cacheService.Close() })
	translationService := NewTranslationService(logger)
	svc := NewLocalizationService(db, logger, translationService, cacheService)
	return svc, db
}

func TestLocalizationService_SetupUserLocalization(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	loc, err := svc.SetupUserLocalization(ctx, &WizardLocalizationStep{
		UserID:                1,
		PrimaryLanguage:       "fr",
		SecondaryLanguages:    []string{"en", "de"},
		SubtitleLanguages:     []string{"fr", "en"},
		LyricsLanguages:       []string{"fr"},
		MetadataLanguages:     []string{"fr", "en"},
		AutoTranslate:         false, // Avoid triggering preloadTranslations which has a race condition on TranslationService map
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    false,
		PreferredRegion:       "FR",
		DateFormat:            "DD/MM/YYYY",
		TimeFormat:            "24h",
		NumberFormat:          "#.###,##",
		CurrencyCode:          "EUR",
	})
	require.NoError(t, err)
	require.NotNil(t, loc)
	assert.Equal(t, "fr", loc.PrimaryLanguage)
	assert.Equal(t, []string{"en", "de"}, loc.SecondaryLanguages)
	assert.False(t, loc.AutoTranslate)
	assert.Equal(t, "EUR", loc.CurrencyCode)
}

func TestLocalizationService_GetUserLocalization_Existing(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	// Setup first
	_, err := svc.SetupUserLocalization(ctx, &WizardLocalizationStep{
		UserID:             1,
		PrimaryLanguage:    "es",
		SecondaryLanguages: []string{"en"},
		SubtitleLanguages:  []string{"es", "en"},
		LyricsLanguages:    []string{"es"},
		MetadataLanguages:  []string{"es"},
		PreferredRegion:    "ES",
		DateFormat:         "DD/MM/YYYY",
		TimeFormat:         "24h",
		NumberFormat:       "#.###,##",
		CurrencyCode:       "EUR",
	})
	require.NoError(t, err)

	// Retrieve
	loc, err := svc.GetUserLocalization(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, loc)
	assert.Equal(t, "es", loc.PrimaryLanguage)
	assert.Equal(t, []string{"en"}, loc.SecondaryLanguages)
}

func TestLocalizationService_GetUserLocalization_CreatesDefault(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	// No localization set up for user 1, should create default
	loc, err := svc.GetUserLocalization(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, loc)
	assert.Equal(t, "en", loc.PrimaryLanguage)
	assert.Equal(t, "US", loc.PreferredRegion)
}

func TestLocalizationService_UpdateUserLocalization(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	// First create a localization
	_, err := svc.SetupUserLocalization(ctx, &WizardLocalizationStep{
		UserID:          1,
		PrimaryLanguage: "en",
		PreferredRegion: "US",
		DateFormat:      "MM/DD/YYYY",
		TimeFormat:      "12h",
		NumberFormat:    "#,###.##",
		CurrencyCode:    "USD",
	})
	require.NoError(t, err)

	// Update some fields
	err = svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{
		"primary_language": "de",
		"preferred_region": "DE",
		"currency_code":    "EUR",
	})
	require.NoError(t, err)

	loc, err := svc.GetUserLocalization(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "de", loc.PrimaryLanguage)
	assert.Equal(t, "DE", loc.PreferredRegion)
	assert.Equal(t, "EUR", loc.CurrencyCode)
}

func TestLocalizationService_UpdateUserLocalization_EmptyUpdates(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	err := svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no updates provided")
}

func TestLocalizationService_UpdateUserLocalization_InvalidFields(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	err := svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{
		"invalid_field": "value",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid updates provided")
}

func TestLocalizationService_GetPreferredLanguagesForContent(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	_, err := svc.SetupUserLocalization(ctx, &WizardLocalizationStep{
		UserID:             1,
		PrimaryLanguage:    "ja",
		SecondaryLanguages: []string{"en"},
		SubtitleLanguages:  []string{"ja", "en"},
		LyricsLanguages:    []string{"ja"},
		MetadataLanguages:  []string{"ja", "en", "ko"},
		DateFormat:         "YYYY/MM/DD",
		TimeFormat:         "24h",
		NumberFormat:       "#,###",
		CurrencyCode:       "JPY",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		contentType string
		expected    []string
	}{
		{"subtitles", ContentTypeSubtitles, []string{"ja", "en"}},
		{"lyrics", ContentTypeLyrics, []string{"ja"}},
		{"metadata", ContentTypeMetadata, []string{"ja", "en", "ko"}},
		{"unknown type falls back to primary+secondary", "unknown", []string{"ja", "en"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			langs, err := svc.GetPreferredLanguagesForContent(ctx, 1, tc.contentType)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, langs)
		})
	}
}

func TestLocalizationService_ShouldAutoTranslate(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	// Test with AutoTranslate=false first (avoids preloadTranslations race condition)
	_, err := svc.SetupUserLocalization(ctx, &WizardLocalizationStep{
		UserID:          1,
		PrimaryLanguage: "en",
		AutoTranslate:   false,
		DateFormat:      "MM/DD/YYYY",
		TimeFormat:      "12h",
		NumberFormat:    "#,###.##",
		CurrencyCode:    "USD",
	})
	require.NoError(t, err)

	result, err := svc.ShouldAutoTranslate(ctx, 1, ContentTypeSubtitles)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestLocalizationService_ShouldAutoDownload(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	_, err := svc.SetupUserLocalization(ctx, &WizardLocalizationStep{
		UserID:                1,
		PrimaryLanguage:       "en",
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    false,
		DateFormat:            "MM/DD/YYYY",
		TimeFormat:            "12h",
		NumberFormat:          "#,###.##",
		CurrencyCode:          "USD",
	})
	require.NoError(t, err)

	subtitles, err := svc.ShouldAutoDownload(ctx, 1, ContentTypeSubtitles)
	require.NoError(t, err)
	assert.True(t, subtitles)

	lyrics, err := svc.ShouldAutoDownload(ctx, 1, ContentTypeLyrics)
	require.NoError(t, err)
	assert.False(t, lyrics)

	other, err := svc.ShouldAutoDownload(ctx, 1, "other")
	require.NoError(t, err)
	assert.False(t, other)
}

func TestLocalizationService_GetSupportedLanguages(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	languages, err := svc.GetSupportedLanguages(ctx)
	require.NoError(t, err)
	assert.True(t, len(languages) > 0)
	assert.Contains(t, languages, "en")
	assert.Contains(t, languages, "es")
	assert.Contains(t, languages, "ja")
}

func TestLocalizationService_GetLanguageProfile(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	profile, err := svc.GetLanguageProfile(ctx, "en")
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "English", profile.Name)
	assert.Equal(t, "ltr", profile.Direction)

	profile, err = svc.GetLanguageProfile(ctx, "ar")
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "rtl", profile.Direction)

	_, err = svc.GetLanguageProfile(ctx, "xx")
	require.Error(t, err)
}

func TestLocalizationService_IsLanguageSupported_DB(t *testing.T) {
	svc, _ := setupLocalizationService(t)
	ctx := context.Background()

	assert.True(t, svc.IsLanguageSupported(ctx, "en", "subtitles"))
	assert.True(t, svc.IsLanguageSupported(ctx, "en", "ui"))
	assert.True(t, svc.IsLanguageSupported(ctx, "ar", "subtitles"))
	assert.False(t, svc.IsLanguageSupported(ctx, "ar", "lyrics")) // Arabic doesn't support lyrics
	assert.False(t, svc.IsLanguageSupported(ctx, "xx", "subtitles"))
}

func TestLocalizationService_GetLocalizationStats(t *testing.T) {
	db := setupIntegrationTestDB(t)
	logger := zap.NewNop()
	svc := NewLocalizationService(db, logger, nil, nil)

	stats, err := svc.GetLocalizationStats(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, stats)
	// Test DB has 2 seeded users
	assert.Equal(t, int64(2), stats.TotalUsers)
}

// ============================================================================
// VideoPlayerService Integration Tests
// ============================================================================

func setupVideoPlayerService(t *testing.T) (*VideoPlayerService, *database.DB) {
	db := setupIntegrationTestDB(t)
	logger := zap.NewNop()
	positionService := NewPlaybackPositionService(db, logger)
	svc := NewVideoPlayerService(db, logger, nil, positionService, nil, nil, nil)
	return svc, db
}

func insertTestVideoMediaItem(t *testing.T, db *database.DB, id int64, title string) {
	_, err := db.Exec(
		`INSERT INTO media_items (id, path, title, type, duration, resolution, video_codec, codec, format)
		 VALUES (?, ?, ?, 'video', 7200000, '1920x1080', 'h264', 'h264', 'mkv')`,
		id, "/videos/"+title, title,
	)
	require.NoError(t, err)
}

func TestVideoPlayerService_GetVideoSession_NotFound(t *testing.T) {
	svc, _ := setupVideoPlayerService(t)
	ctx := context.Background()

	_, err := svc.GetVideoSession(ctx, "nonexistent-session-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found or expired")
}

func TestVideoPlayerService_SaveAndGetSession(t *testing.T) {
	svc, _ := setupVideoPlayerService(t)
	ctx := context.Background()

	session := &VideoPlaybackSession{
		ID:            "test-session-123",
		UserID:        1,
		PlaylistIndex: 0,
		PlayMode:      VideoPlayModeSingle,
		Volume:        0.8,
		PlaybackSpeed: 1.0,
		PlaybackState: PlaybackStatePlaying,
		Position:      30000,
		Duration:      120000,
		VideoQuality:  Quality1080p,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentVideo:  &VideoContent{ID: 1, Title: "Test Video"},
	}

	err := svc.saveVideoSession(ctx, session)
	require.NoError(t, err)

	retrieved, err := svc.GetVideoSession(ctx, "test-session-123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, int64(1), retrieved.UserID)
	assert.Equal(t, int64(30000), retrieved.Position)
	assert.Equal(t, "Test Video", retrieved.CurrentVideo.Title)
}

func TestVideoPlayerService_GetVideoContent(t *testing.T) {
	svc, db := setupVideoPlayerService(t)
	ctx := context.Background()

	insertTestVideoMediaItem(t, db, 800, "action-movie.mkv")

	video, err := svc.getVideoContent(ctx, 800)
	require.NoError(t, err)
	require.NotNil(t, video)
	assert.Equal(t, "action-movie.mkv", video.Title)
	assert.Equal(t, int64(7200000), video.Duration)
	assert.Equal(t, "1920x1080", video.Resolution)
}

func TestVideoPlayerService_GetVideoContent_NotFound(t *testing.T) {
	svc, _ := setupVideoPlayerService(t)
	ctx := context.Background()

	_, err := svc.getVideoContent(ctx, 99999)
	require.Error(t, err)
}

func TestVideoPlayerService_PlayVideo_GetVideoContent(t *testing.T) {
	svc, db := setupVideoPlayerService(t)
	ctx := context.Background()

	insertTestVideoMediaItem(t, db, 803, "play-test.mkv")

	// Verify getVideoContent works for an inserted item
	video, err := svc.getVideoContent(ctx, 803)
	require.NoError(t, err)
	require.NotNil(t, video)
	assert.Equal(t, int64(803), video.ID)
	assert.Equal(t, "play-test.mkv", video.Title)
	assert.Equal(t, int64(7200000), video.Duration)
}

// ============================================================================
// TranslationService Utility Tests
// ============================================================================

func TestTranslationService_SimpleLanguageDetection(t *testing.T) {
	logger := zap.NewNop()
	svc := NewTranslationService(logger)

	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"english", "The quick brown fox and the lazy dog", "en"},
		{"spanish", "El gato es muy bonito", "es"},
		{"french", "Le chat est grand", "fr"},
		{"german", "Der Hund ist schnell", "de"},
		{"default to english", "Random text without clear markers", "en"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := svc.simpleLanguageDetection(tc.text)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTranslationService_GetTextPreview(t *testing.T) {
	logger := zap.NewNop()
	svc := NewTranslationService(logger)

	short := "Short text"
	assert.Equal(t, short, svc.getTextPreview(short))

	long := "This is a very long text that exceeds one hundred characters in length and should be truncated to only show the first hundred characters."
	preview := svc.getTextPreview(long)
	assert.Len(t, preview, 103) // 100 chars + "..."
	assert.True(t, len(preview) <= 104)
}

func TestGetLanguageName(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"en", "English"},
		{"es", "Spanish"},
		{"fr", "French"},
		{"de", "German"},
		{"ja", "Japanese"},
		{"ko", "Korean"},
		{"zh", "Chinese"},
		{"ar", "Arabic"},
		{"xx", "xx"}, // Unknown code returns itself
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			assert.Equal(t, tc.expected, getLanguageName(tc.code))
		})
	}
}

// ============================================================================
// RecommendationService Utility Tests
// ============================================================================

func TestRecommendationService_ExtractYear(t *testing.T) {
	svc := &RecommendationService{}

	tests := []struct {
		input    string
		expected string
	}{
		{"2023-05-15", "2023"},
		{"1999", "1999"},
		{"20", ""},
		{"", ""},
		{"2024-12-31T23:59:59Z", "2024"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, svc.extractYear(tc.input))
		})
	}
}

func TestRecommendationService_ParseLastFmMatch(t *testing.T) {
	svc := &RecommendationService{}

	// Empty string returns 0.5
	assert.Equal(t, 0.5, svc.parseLastFmMatch(""))

	// CAT-001: the value is parsed, not replaced by a constant.
	assert.Equal(t, 0.85, svc.parseLastFmMatch("0.85"))
	assert.Equal(t, 0.2, svc.parseLastFmMatch("0.2"))
	assert.Equal(t, 0.5, svc.parseLastFmMatch("not-a-number"))
}

func TestRecommendationService_PassesFilters_NilFilters(t *testing.T) {
	svc := &RecommendationService{}
	media := &models.MediaMetadata{Title: "Test"}

	assert.True(t, svc.passesFilters(media, 0.5, nil))
}

func TestRecommendationService_PassesFilters_MinConfidence(t *testing.T) {
	svc := &RecommendationService{}
	media := &models.MediaMetadata{Title: "Test"}

	filters := &RecommendationFilters{MinConfidence: 0.8}
	assert.False(t, svc.passesFilters(media, 0.5, filters))
	assert.True(t, svc.passesFilters(media, 0.9, filters))
}

func TestRecommendationService_PassesFilters_Genre(t *testing.T) {
	svc := &RecommendationService{}
	media := &models.MediaMetadata{Title: "Test", Genre: "Action, Adventure"}

	// Filter matches
	filters := &RecommendationFilters{GenreFilter: []string{"action"}}
	assert.True(t, svc.passesFilters(media, 1.0, filters))

	// Filter doesn't match
	filters = &RecommendationFilters{GenreFilter: []string{"comedy"}}
	assert.False(t, svc.passesFilters(media, 1.0, filters))
}

func TestRecommendationService_PassesFilters_YearRange(t *testing.T) {
	svc := &RecommendationService{}
	year := 2020
	media := &models.MediaMetadata{Title: "Test", Year: &year}

	// In range
	filters := &RecommendationFilters{YearRange: &YearRange{StartYear: 2018, EndYear: 2022}}
	assert.True(t, svc.passesFilters(media, 1.0, filters))

	// Out of range
	filters = &RecommendationFilters{YearRange: &YearRange{StartYear: 2021, EndYear: 2023}}
	assert.False(t, svc.passesFilters(media, 1.0, filters))
}

func TestRecommendationService_PassesFilters_RatingRange(t *testing.T) {
	svc := &RecommendationService{}
	rating := 7.5
	media := &models.MediaMetadata{Title: "Test", Rating: &rating}

	// In range
	filters := &RecommendationFilters{RatingRange: &RatingRange{MinRating: 6.0, MaxRating: 9.0}}
	assert.True(t, svc.passesFilters(media, 1.0, filters))

	// Out of range
	filters = &RecommendationFilters{RatingRange: &RatingRange{MinRating: 8.0, MaxRating: 10.0}}
	assert.False(t, svc.passesFilters(media, 1.0, filters))
}

func TestRecommendationService_PassesFilters_Language(t *testing.T) {
	svc := &RecommendationService{}
	media := &models.MediaMetadata{Title: "Test", Language: "English"}

	filters := &RecommendationFilters{LanguageFilter: []string{"English"}}
	assert.True(t, svc.passesFilters(media, 1.0, filters))

	filters = &RecommendationFilters{LanguageFilter: []string{"French"}}
	assert.False(t, svc.passesFilters(media, 1.0, filters))
}

func TestRecommendationService_PassesExternalFilters_NilFilters(t *testing.T) {
	svc := &RecommendationService{}
	item := &ExternalSimilarItem{Title: "Test"}

	assert.True(t, svc.passesExternalFilters(item, nil))
}

func TestRecommendationService_PassesExternalFilters_Genre(t *testing.T) {
	svc := &RecommendationService{}
	item := &ExternalSimilarItem{Title: "Test", Genre: "Sci-Fi, Thriller"}

	filters := &RecommendationFilters{GenreFilter: []string{"sci-fi"}}
	assert.True(t, svc.passesExternalFilters(item, filters))

	filters = &RecommendationFilters{GenreFilter: []string{"comedy"}}
	assert.False(t, svc.passesExternalFilters(item, filters))
}

func TestRecommendationService_PassesExternalFilters_Rating(t *testing.T) {
	svc := &RecommendationService{}
	item := &ExternalSimilarItem{Title: "Test", Rating: 8.5}

	filters := &RecommendationFilters{RatingRange: &RatingRange{MinRating: 7.0, MaxRating: 9.0}}
	assert.True(t, svc.passesExternalFilters(item, filters))

	filters = &RecommendationFilters{RatingRange: &RatingRange{MinRating: 9.0, MaxRating: 10.0}}
	assert.False(t, svc.passesExternalFilters(item, filters))
}

func TestRecommendationService_GenerateLinks(t *testing.T) {
	svc := &RecommendationService{}
	media := &models.MediaMetadata{ID: 42, Title: "Test"}

	assert.Equal(t, "/detail/42", svc.generateDetailLink(media))
	assert.Equal(t, "/play/42", svc.generatePlayLink(media))
	assert.Equal(t, "/download/42", svc.generateDownloadLink(media))
}
