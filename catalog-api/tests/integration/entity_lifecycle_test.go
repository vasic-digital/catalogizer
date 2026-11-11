package integration

import (
	"fmt"
	"strings"
	"testing"

	"catalogizer/internal/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// INTEGRATION TEST: Full Entity Lifecycle
// (create root -> scan -> detect entities -> browse -> search -> details -> stream)
// =============================================================================

func TestEntityLifecycle_CreateStorageRootToStream(t *testing.T) {
	db := tests.SetupTestDB(t)
	defer db.Close()

	// Step 1: Create a storage root
	result, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, path, enabled)
		VALUES ('media-library', 'local', '/media/library', 1)
	`)
	require.NoError(t, err)
	rootID, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, rootID, int64(0), "Storage root should be created")

	// Step 2: Simulate scanning - insert file records
	scanFiles := []struct {
		path string
		name string
		ext  string
		size int64
	}{
		{"/media/library/movies/Inception.2010.mkv", "Inception.2010.mkv", ".mkv", 4500000000},
		{"/media/library/movies/The.Matrix.1999.mp4", "The.Matrix.1999.mp4", ".mp4", 3200000000},
		{"/media/library/tv/Breaking.Bad.S01E01.mkv", "Breaking.Bad.S01E01.mkv", ".mkv", 1200000000},
		{"/media/library/tv/Breaking.Bad.S01E02.mkv", "Breaking.Bad.S01E02.mkv", ".mkv", 1100000000},
		{"/media/library/tv/Breaking.Bad.S02E01.mkv", "Breaking.Bad.S02E01.mkv", ".mkv", 1300000000},
		{"/media/library/music/Radiohead/OK.Computer/01.Airbag.flac", "01.Airbag.flac", ".flac", 45000000},
		{"/media/library/music/Radiohead/OK.Computer/02.Paranoid.Android.flac", "02.Paranoid.Android.flac", ".flac", 52000000},
		{"/media/library/books/Dune.epub", "Dune.epub", ".epub", 2000000},
	}

	for _, f := range scanFiles {
		_, err := db.Exec(`
			INSERT INTO files (storage_root_id, path, name, extension, size, modified_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`, rootID, f.path, f.name, f.ext, f.size)
		require.NoError(t, err)
	}

	var fileCount int
	err = db.QueryRow("SELECT COUNT(*) FROM files WHERE storage_root_id = ?", rootID).Scan(&fileCount)
	require.NoError(t, err)
	assert.Equal(t, len(scanFiles), fileCount, "All files should be inserted")

	// Step 3: Simulate entity detection - create media items from scanned files
	entities := []struct {
		path      string
		title     string
		mediaType string
		year      int
		season    int
		episode   int
	}{
		{"/media/library/movies/Inception.2010.mkv", "Inception", "movie", 2010, 0, 0},
		{"/media/library/movies/The.Matrix.1999.mp4", "The Matrix", "movie", 1999, 0, 0},
		{"/media/library/tv/Breaking.Bad.S01E01.mkv", "Breaking Bad S01E01", "tv_episode", 2008, 1, 1},
		{"/media/library/tv/Breaking.Bad.S01E02.mkv", "Breaking Bad S01E02", "tv_episode", 2008, 1, 2},
		{"/media/library/tv/Breaking.Bad.S02E01.mkv", "Breaking Bad S02E01", "tv_episode", 2009, 2, 1},
	}

	for _, e := range entities {
		_, err := db.Exec(`
			INSERT INTO media_items
			(path, title, type, media_type, year, season, episode, storage_root_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, e.path, e.title, e.mediaType, e.mediaType, e.year, e.season, e.episode, rootID)
		require.NoError(t, err)
	}

	// Step 4: Browse - list entities by type
	t.Run("BrowseByType", func(t *testing.T) {
		var movieCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE type = 'movie' AND storage_root_id = ?",
			rootID,
		).Scan(&movieCount)
		require.NoError(t, err)
		assert.Equal(t, 2, movieCount, "Should have 2 movie entities")

		var episodeCount int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE type = 'tv_episode' AND storage_root_id = ?",
			rootID,
		).Scan(&episodeCount)
		require.NoError(t, err)
		assert.Equal(t, 3, episodeCount, "Should have 3 TV episode entities")
	})

	// Step 5: Search entities
	t.Run("SearchEntities", func(t *testing.T) {
		rows, err := db.Query(
			"SELECT title, type FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%Matrix%", rootID,
		)
		require.NoError(t, err)
		defer rows.Close()

		var results []string
		for rows.Next() {
			var title, mediaType string
			require.NoError(t, rows.Scan(&title, &mediaType))
			results = append(results, title)
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, 1, len(results), "Search for 'Matrix' should return 1 result")
		assert.Equal(t, "The Matrix", results[0])
	})

	// Step 6: Get entity details
	t.Run("GetEntityDetails", func(t *testing.T) {
		var id int64
		var title, mediaType string
		var year int

		err := db.QueryRow(
			"SELECT id, title, type, year FROM media_items WHERE title = 'Inception'",
		).Scan(&id, &title, &mediaType, &year)
		require.NoError(t, err)

		assert.Greater(t, id, int64(0))
		assert.Equal(t, "Inception", title)
		assert.Equal(t, "movie", mediaType)
		assert.Equal(t, 2010, year)
	})

	// Step 7: Simulate stream setup (create playback session)
	t.Run("SetupStreamSession", func(t *testing.T) {
		var mediaID int64
		err := db.QueryRow(
			"SELECT id FROM media_items WHERE title = 'Inception'",
		).Scan(&mediaID)
		require.NoError(t, err)

		userID := 1 // Default test user

		_, err = db.Exec(`
			INSERT INTO playback_sessions
			(user_id, media_item_id, current_position, state, volume, playback_rate)
			VALUES (?, ?, 0.0, 'playing', 1.0, 1.0)
		`, userID, mediaID)
		require.NoError(t, err)

		// Verify session
		var state string
		var position float64
		err = db.QueryRow(
			"SELECT state, current_position FROM playback_sessions WHERE user_id = ? AND media_item_id = ?",
			userID, mediaID,
		).Scan(&state, &position)
		require.NoError(t, err)
		assert.Equal(t, "playing", state)
		assert.InDelta(t, 0.0, position, 0.01)

		// Update position to simulate playback
		_, err = db.Exec(`
			UPDATE playback_sessions SET current_position = 1800.0
			WHERE user_id = ? AND media_item_id = ?
		`, userID, mediaID)
		require.NoError(t, err)

		err = db.QueryRow(
			"SELECT current_position FROM playback_sessions WHERE user_id = ? AND media_item_id = ?",
			userID, mediaID,
		).Scan(&position)
		require.NoError(t, err)
		assert.InDelta(t, 1800.0, position, 0.01,
			"Playback position should be updated to 30 minutes")
	})
}

// =============================================================================
// INTEGRATION TEST: Entity Hierarchy (show -> season -> episode)
// =============================================================================

func TestEntityLifecycle_TVShowHierarchy(t *testing.T) {
	db := tests.SetupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, path, enabled)
		VALUES ('tv-test', 'local', '/tv', 1)
	`)
	require.NoError(t, err)

	var rootID int64
	err = db.QueryRow("SELECT id FROM storage_roots WHERE name = 'tv-test'").Scan(&rootID)
	require.NoError(t, err)

	t.Run("CreateFullHierarchy", func(t *testing.T) {
		// Create TV show
		showResult, err := db.Exec(`
			INSERT INTO media_items (path, title, type, media_type, year, storage_root_id)
			VALUES ('/tv/breaking_bad', 'Breaking Bad', 'tv_show', 'tv_show', 2008, ?)
		`, rootID)
		require.NoError(t, err)
		showID, _ := showResult.LastInsertId()

		// Create seasons
		seasonIDs := make(map[int]int64)
		for s := 1; s <= 5; s++ {
			seasonResult, err := db.Exec(`
				INSERT INTO media_items
				(path, title, type, media_type, season, storage_root_id)
				VALUES (?, ?, 'tv_season', 'tv_season', ?, ?)
			`, fmt.Sprintf("/tv/breaking_bad/s%02d", s),
				fmt.Sprintf("Season %d", s), s, rootID)
			require.NoError(t, err)
			seasonIDs[s], _ = seasonResult.LastInsertId()
		}

		// Create episodes per season
		episodesPerSeason := map[int]int{1: 7, 2: 13, 3: 13, 4: 13, 5: 16}
		for s, epCount := range episodesPerSeason {
			for ep := 1; ep <= epCount; ep++ {
				_, err := db.Exec(`
					INSERT INTO media_items
					(path, title, type, media_type, season, episode, storage_root_id)
					VALUES (?, ?, 'tv_episode', 'tv_episode', ?, ?, ?)
				`, fmt.Sprintf("/tv/breaking_bad/s%02d/e%02d.mkv", s, ep),
					fmt.Sprintf("Episode %d", ep), s, ep, rootID)
				require.NoError(t, err)
			}
		}

		// Verify show
		assert.Greater(t, showID, int64(0))

		// Verify season count
		var seasonCount int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE type = 'tv_season' AND storage_root_id = ?",
			rootID,
		).Scan(&seasonCount)
		require.NoError(t, err)
		assert.Equal(t, 5, seasonCount, "Should have 5 seasons")

		// Verify total episode count
		var totalEpisodes int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE type = 'tv_episode' AND storage_root_id = ?",
			rootID,
		).Scan(&totalEpisodes)
		require.NoError(t, err)
		expectedTotal := 7 + 13 + 13 + 13 + 16
		assert.Equal(t, expectedTotal, totalEpisodes,
			"Should have correct total episode count")
	})

	t.Run("QueryEpisodesBySeason", func(t *testing.T) {
		for s := 1; s <= 5; s++ {
			var count int
			err := db.QueryRow(
				"SELECT COUNT(*) FROM media_items WHERE type = 'tv_episode' AND season = ? AND storage_root_id = ?",
				s, rootID,
			).Scan(&count)
			require.NoError(t, err)

			expected := map[int]int{1: 7, 2: 13, 3: 13, 4: 13, 5: 16}[s]
			assert.Equal(t, expected, count,
				"Season %d should have %d episodes", s, expected)
		}
	})

	t.Run("QuerySpecificEpisode", func(t *testing.T) {
		var title, mediaType string
		var season, episode int

		err := db.QueryRow(
			"SELECT title, type, season, episode FROM media_items WHERE season = 3 AND episode = 7 AND type = 'tv_episode' AND storage_root_id = ?",
			rootID,
		).Scan(&title, &mediaType, &season, &episode)
		require.NoError(t, err)

		assert.Equal(t, "Episode 7", title)
		assert.Equal(t, "tv_episode", mediaType)
		assert.Equal(t, 3, season)
		assert.Equal(t, 7, episode)
	})
}

// =============================================================================
// INTEGRATION TEST: Entity Search with Special Characters
// =============================================================================

func TestEntityLifecycle_SearchSpecialCharacters(t *testing.T) {
	db := tests.SetupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, path, enabled)
		VALUES ('search-test', 'local', '/search', 1)
	`)
	require.NoError(t, err)

	var rootID int64
	err = db.QueryRow("SELECT id FROM storage_roots WHERE name = 'search-test'").Scan(&rootID)
	require.NoError(t, err)

	// Insert entities with special characters
	specialEntities := []struct {
		title     string
		mediaType string
	}{
		// Cyrillic titles
		{"Война и Мир", "movie"},
		{"Москва слезам не верит", "movie"},
		{"Брат", "movie"},
		// CJK (Chinese/Japanese/Korean)
		{"千と千尋の神隠し", "movie"},       // Spirited Away
		{"寄生虫", "movie"},               // Parasite
		{"진격의 거인", "tv_show"},           // Attack on Titan
		// Emoji in titles
		{"Movie With Stars ★★★★★", "movie"},
		// Accented characters
		{"Amélie", "movie"},
		{"Crouching Tiger, Hidden Dragon (卧虎藏龙)", "movie"},
		// SQL injection payloads (should be safely stored)
		{"Robert'); DROP TABLE media_items;--", "movie"},
		{"<script>alert('xss')</script>", "movie"},
		// Path traversal
		{"../../../etc/passwd", "movie"},
		// Very long title
		{strings.Repeat("A", 500), "movie"},
		// Empty-like titles
		{"   ", "movie"},
		{"", "movie"},
	}

	for i, e := range specialEntities {
		_, err := db.Exec(`
			INSERT INTO media_items
			(path, title, type, media_type, storage_root_id)
			VALUES (?, ?, ?, ?, ?)
		`, fmt.Sprintf("/search/special_%d.mkv", i), e.title, e.mediaType, e.mediaType, rootID)
		require.NoError(t, err)
	}

	t.Run("SearchCyrillic", func(t *testing.T) {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%Война%", rootID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Should find Cyrillic title with LIKE")
	})

	t.Run("SearchCJK", func(t *testing.T) {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%千尋%", rootID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Should find CJK (Japanese) title with LIKE")
	})

	t.Run("SearchKorean", func(t *testing.T) {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%거인%", rootID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Should find Korean title with LIKE")
	})

	t.Run("SearchAccented", func(t *testing.T) {
		var title string
		err := db.QueryRow(
			"SELECT title FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%mélie%", rootID,
		).Scan(&title)
		require.NoError(t, err)
		assert.Equal(t, "Amélie", title, "Should find accented title")
	})

	t.Run("SearchEmoji", func(t *testing.T) {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%★★★%", rootID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Should find title with emoji/symbols")
	})

	t.Run("SQLInjectionPayloadStoredSafely", func(t *testing.T) {
		// The injection payload should be stored as a literal string
		var title string
		err := db.QueryRow(
			"SELECT title FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%DROP TABLE%", rootID,
		).Scan(&title)
		require.NoError(t, err)
		assert.Contains(t, title, "DROP TABLE",
			"SQL injection payload should be stored as literal text")

		// Verify the table still exists
		var tableCount int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE storage_root_id = ?",
			rootID,
		).Scan(&tableCount)
		require.NoError(t, err)
		assert.Greater(t, tableCount, 0,
			"media_items table should still exist after injection payload")
	})

	t.Run("XSSPayloadStoredSafely", func(t *testing.T) {
		var title string
		err := db.QueryRow(
			"SELECT title FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%<script>%", rootID,
		).Scan(&title)
		require.NoError(t, err)
		assert.Contains(t, title, "<script>",
			"XSS payload should be stored as literal text, not executed")
	})

	t.Run("LongTitleStoredCorrectly", func(t *testing.T) {
		var title string
		err := db.QueryRow(
			"SELECT title FROM media_items WHERE LENGTH(title) = 500 AND storage_root_id = ?",
			rootID,
		).Scan(&title)
		require.NoError(t, err)
		assert.Equal(t, 500, len(title), "500-char title should be stored fully")
	})

	t.Run("MixedScriptSearch", func(t *testing.T) {
		var title string
		err := db.QueryRow(
			"SELECT title FROM media_items WHERE title LIKE ? AND storage_root_id = ?",
			"%卧虎藏龙%", rootID,
		).Scan(&title)
		require.NoError(t, err)
		assert.Contains(t, title, "Crouching Tiger",
			"Mixed Latin/CJK title should be searchable by CJK portion")
	})
}

// =============================================================================
// INTEGRATION TEST: Entity with Full Metadata Chain
// =============================================================================

func TestEntityLifecycle_FullMetadataChain(t *testing.T) {
	db := tests.SetupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, path, enabled)
		VALUES ('meta-lifecycle', 'local', '/meta', 1)
	`)
	require.NoError(t, err)

	var rootID int64
	err = db.QueryRow("SELECT id FROM storage_roots WHERE name = 'meta-lifecycle'").Scan(&rootID)
	require.NoError(t, err)

	t.Run("CreateEntityWithFullMetadata", func(t *testing.T) {
		// Create a movie with all metadata fields
		result, err := db.Exec(`
			INSERT INTO media_items
			(path, title, original_title, type, media_type, year, duration,
			 description, language, country, genre, rating,
			 video_codec, audio_codec, resolution, aspect_ratio,
			 framerate, bitrate, hdr, dolby_vision, dolby_atmos,
			 imdb_id, tmdb_id, storage_root_id)
			VALUES (?, ?, ?, 'movie', 'movie', ?, ?,
			 ?, ?, ?, ?, ?,
			 ?, ?, ?, ?,
			 ?, ?, ?, ?, ?,
			 ?, ?, ?)
		`,
			"/meta/inception.mkv", "Inception", "Inception", 2010, 8880,
			"A thief who steals corporate secrets through dream-sharing technology",
			"English", "US", "Sci-Fi", 8.8,
			"h265", "aac", "3840x2160", "2.39:1",
			23.976, 25000000, true, false, true,
			"tt1375666", "27205", rootID,
		)
		require.NoError(t, err)
		mediaID, _ := result.LastInsertId()

		// Add subtitle tracks
		subtitles := []struct{ lang, code string }{
			{"English", "en"},
			{"Spanish", "es"},
			{"French", "fr"},
			{"Japanese", "ja"},
			{"Russian", "ru"},
		}
		for _, sub := range subtitles {
			_, err := db.Exec(`
				INSERT INTO subtitle_tracks
				(media_item_id, language, language_code, source, format)
				VALUES (?, ?, ?, 'downloaded', 'srt')
			`, mediaID, sub.lang, sub.code)
			require.NoError(t, err)
		}

		// Add audio tracks
		audioTracks := []struct {
			lang     string
			code     string
			codec    string
			channels int
		}{
			{"English", "en", "truehd", 8},
			{"English", "en", "aac", 2},
			{"Spanish", "es", "ac3", 6},
		}
		for _, at := range audioTracks {
			_, err := db.Exec(`
				INSERT INTO audio_tracks
				(media_item_id, language, language_code, codec, channels)
				VALUES (?, ?, ?, ?, ?)
			`, mediaID, at.lang, at.code, at.codec, at.channels)
			require.NoError(t, err)
		}

		// Add cover art
		_, err = db.Exec(`
			INSERT INTO cover_art
			(media_item_id, source, url, width, height, format, is_default)
			VALUES (?, 'tmdb', 'https://image.tmdb.org/poster.jpg', 780, 1170, 'jpeg', 1)
		`, mediaID)
		require.NoError(t, err)

		// Verify the full entity
		var title, resolution, videoCodec, imdbID string
		var duration, year int
		var hdr, dolbyAtmos bool
		err = db.QueryRow(`
			SELECT title, year, duration, resolution, video_codec, hdr, dolby_atmos, imdb_id
			FROM media_items WHERE id = ?
		`, mediaID).Scan(&title, &year, &duration, &resolution, &videoCodec, &hdr, &dolbyAtmos, &imdbID)
		require.NoError(t, err)

		assert.Equal(t, "Inception", title)
		assert.Equal(t, 2010, year)
		assert.Equal(t, 8880, duration)
		assert.Equal(t, "3840x2160", resolution)
		assert.Equal(t, "h265", videoCodec)
		assert.True(t, hdr)
		assert.True(t, dolbyAtmos)
		assert.Equal(t, "tt1375666", imdbID)

		// Verify subtitle count
		var subCount int
		err = db.QueryRow("SELECT COUNT(*) FROM subtitle_tracks WHERE media_item_id = ?", mediaID).Scan(&subCount)
		require.NoError(t, err)
		assert.Equal(t, 5, subCount)

		// Verify audio track count
		var audioCount int
		err = db.QueryRow("SELECT COUNT(*) FROM audio_tracks WHERE media_item_id = ?", mediaID).Scan(&audioCount)
		require.NoError(t, err)
		assert.Equal(t, 3, audioCount)

		// Verify cover art
		var artSource string
		var artWidth int
		err = db.QueryRow(
			"SELECT source, width FROM cover_art WHERE media_item_id = ? AND is_default = 1",
			mediaID,
		).Scan(&artSource, &artWidth)
		require.NoError(t, err)
		assert.Equal(t, "tmdb", artSource)
		assert.Equal(t, 780, artWidth)
	})
}

// =============================================================================
// INTEGRATION TEST: Music Entity Hierarchy (artist -> album -> song)
// =============================================================================

func TestEntityLifecycle_MusicHierarchy(t *testing.T) {
	db := tests.SetupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, path, enabled)
		VALUES ('music-test', 'local', '/music', 1)
	`)
	require.NoError(t, err)

	var rootID int64
	err = db.QueryRow("SELECT id FROM storage_roots WHERE name = 'music-test'").Scan(&rootID)
	require.NoError(t, err)

	t.Run("CreateArtistAlbumSongHierarchy", func(t *testing.T) {
		// Create albums
		albums := []struct {
			title  string
			artist string
			year   int
			tracks []string
		}{
			{
				title: "OK Computer", artist: "Radiohead", year: 1997,
				tracks: []string{"Airbag", "Paranoid Android", "Subterranean Homesick Alien",
					"Exit Music", "Let Down", "Karma Police"},
			},
			{
				title: "Kid A", artist: "Radiohead", year: 2000,
				tracks: []string{"Everything in Its Right Place", "Kid A",
					"The National Anthem", "How to Disappear Completely"},
			},
		}

		for _, album := range albums {
			albumResult, err := db.Exec(`
				INSERT INTO albums (title, artist, genre, year, total_tracks)
				VALUES (?, ?, 'Alternative', ?, ?)
			`, album.title, album.artist, album.year, len(album.tracks))
			require.NoError(t, err)
			albumID, _ := albumResult.LastInsertId()

			for trackNum, trackTitle := range album.tracks {
				_, err := db.Exec(`
					INSERT INTO media_items
					(path, title, type, media_type, album_id, track_number,
					 artist, album, year, storage_root_id)
					VALUES (?, ?, 'song', 'song', ?, ?, ?, ?, ?, ?)
				`, fmt.Sprintf("/music/%s/%s/%02d.%s.flac",
					album.artist, album.title, trackNum+1, trackTitle),
					trackTitle, albumID, trackNum+1,
					album.artist, album.title, album.year, rootID)
				require.NoError(t, err)
			}
		}

		// Verify total track count
		var totalTracks int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE type = 'song' AND storage_root_id = ?",
			rootID,
		).Scan(&totalTracks)
		require.NoError(t, err)
		assert.Equal(t, 10, totalTracks, "Should have 10 total songs")

		// Verify tracks per album
		rows, err := db.Query(`
			SELECT a.title, COUNT(mi.id)
			FROM albums a
			JOIN media_items mi ON mi.album_id = a.id
			WHERE mi.storage_root_id = ?
			GROUP BY a.title
			ORDER BY a.title
		`, rootID)
		require.NoError(t, err)
		defer rows.Close()

		albumCounts := make(map[string]int)
		for rows.Next() {
			var albumTitle string
			var count int
			require.NoError(t, rows.Scan(&albumTitle, &count))
			albumCounts[albumTitle] = count
		}
		require.NoError(t, rows.Err())

		assert.Equal(t, 4, albumCounts["Kid A"])
		assert.Equal(t, 6, albumCounts["OK Computer"])
	})

	t.Run("SearchByArtist", func(t *testing.T) {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE artist = 'Radiohead' AND storage_root_id = ?",
			rootID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 10, count, "All tracks should belong to Radiohead")
	})

	t.Run("SearchByAlbumName", func(t *testing.T) {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE album = 'OK Computer' AND storage_root_id = ?",
			rootID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 6, count, "OK Computer should have 6 tracks")
	})
}

// =============================================================================
// INTEGRATION TEST: Entity Lifecycle with Favorites and Playback History
// =============================================================================

func TestEntityLifecycle_FavoritesAndHistory(t *testing.T) {
	db := tests.SetupTestDB(t)
	defer db.Close()

	// Create favorites and playback_sessions tables (v11 service tables not in test helper)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS favorites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS playback_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		entity_id INTEGER NOT NULL,
		position_seconds REAL DEFAULT 0,
		duration_seconds REAL DEFAULT 0,
		completed INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	_, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, path, enabled)
		VALUES ('fav-test', 'local', '/fav', 1)
	`)
	require.NoError(t, err)

	var rootID int64
	err = db.QueryRow("SELECT id FROM storage_roots WHERE name = 'fav-test'").Scan(&rootID)
	require.NoError(t, err)

	userID := 1

	// Create media items
	for i := 0; i < 5; i++ {
		_, err := db.Exec(`
			INSERT INTO media_items
			(path, title, type, media_type, duration, storage_root_id)
			VALUES (?, ?, 'movie', 'movie', ?, ?)
		`, fmt.Sprintf("/fav/movie_%d.mkv", i),
			fmt.Sprintf("Movie %d", i), 7200+i*600, rootID)
		require.NoError(t, err)
	}

	t.Run("AddAndRemoveFavorites", func(t *testing.T) {
		// Get first media item
		var mediaID int64
		err := db.QueryRow(
			"SELECT id FROM media_items WHERE title = 'Movie 0' AND storage_root_id = ?",
			rootID,
		).Scan(&mediaID)
		require.NoError(t, err)

		// Add to favorites
		_, err = db.Exec(`
			INSERT INTO favorites (user_id, entity_type, entity_id)
			VALUES (?, 'movie', ?)
		`, userID, mediaID)
		require.NoError(t, err)

		// Verify favorite exists
		var favCount int
		err = db.QueryRow(
			"SELECT COUNT(*) FROM favorites WHERE user_id = ? AND entity_id = ?",
			userID, mediaID,
		).Scan(&favCount)
		require.NoError(t, err)
		assert.Equal(t, 1, favCount)

		// Remove from favorites
		_, err = db.Exec(
			"DELETE FROM favorites WHERE user_id = ? AND entity_id = ?",
			userID, mediaID,
		)
		require.NoError(t, err)

		err = db.QueryRow(
			"SELECT COUNT(*) FROM favorites WHERE user_id = ? AND entity_id = ?",
			userID, mediaID,
		).Scan(&favCount)
		require.NoError(t, err)
		assert.Equal(t, 0, favCount, "Favorite should be removed")
	})

	t.Run("TrackPlaybackHistory", func(t *testing.T) {
		var mediaID int64
		err := db.QueryRow(
			"SELECT id FROM media_items WHERE title = 'Movie 1' AND storage_root_id = ?",
			rootID,
		).Scan(&mediaID)
		require.NoError(t, err)

		// Record playback
		_, err = db.Exec(`
			INSERT INTO playback_positions
			(user_id, media_item_id, position, duration, percent_complete, last_played)
			VALUES (?, ?, 3600, 7800, 46.15, datetime('now'))
		`, userID, mediaID)
		require.NoError(t, err)

		// Verify
		var position, duration int
		var pct float64
		err = db.QueryRow(`
			SELECT position, duration, percent_complete
			FROM playback_positions
			WHERE user_id = ? AND media_item_id = ?
		`, userID, mediaID).Scan(&position, &duration, &pct)
		require.NoError(t, err)

		assert.Equal(t, 3600, position)
		assert.Equal(t, 7800, duration)
		assert.InDelta(t, 46.15, pct, 0.01)
	})
}
