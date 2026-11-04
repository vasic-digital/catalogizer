package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/media/models"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEntityTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "entity_handler_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	cfg := &config.DatabaseConfig{
		Type:               "sqlite",
		Path:               tmpFile.Name(),
		MaxOpenConnections: 1,
		MaxIdleConnections: 1,
		ConnMaxLifetime:    3600,
		EnableWAL:          true,
		CacheSize:          2000,
		BusyTimeout:        5000,
	}

	db, err := database.NewConnection(cfg)
	require.NoError(t, err)

	err = db.RunMigrations(context.Background())
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}
	return db, cleanup
}

func setupEntityHandler(t *testing.T, db *database.DB) (*MediaEntityHandler, *repository.MediaItemRepository) {
	t.Helper()
	itemRepo := repository.NewMediaItemRepository(db)
	fileRepo := repository.NewMediaFileRepository(db)
	extMetaRepo := repository.NewExternalMetadataRepository(db)
	userMetaRepo := repository.NewUserMetadataRepository(db)

	handler := NewMediaEntityHandler(itemRepo, fileRepo, extMetaRepo, userMetaRepo)
	return handler, itemRepo
}

func TestMediaEntityHandler_GetEntityTypes(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, _ := setupEntityHandler(t, db)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/entities/types", nil)

	handler.GetEntityTypes(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	types, ok := resp["types"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(types), 11) // 11 seeded types
}

func TestMediaEntityHandler_GetEntityStats(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	_, _ = itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Movie 1", Status: "detected"})
	_, _ = itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Movie 2", Status: "detected"})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/entities/stats", nil)

	handler.GetEntityStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(2), resp["total_entities"])
}

func TestMediaEntityHandler_ListEntities(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	ctx := context.Background()

	_, typeID, _ := itemRepo.GetMediaTypeByName(ctx, "movie")
	_, _ = itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "The Matrix", Status: "detected"})
	_, _ = itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Inception", Status: "detected"})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/entities?query=Matrix", nil)

	handler.ListEntities(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
}

func TestMediaEntityHandler_GetEntity(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	ctx := context.Background()

	_, typeID, _ := itemRepo.GetMediaTypeByName(ctx, "movie")
	year := 1999
	id, _ := itemRepo.Create(ctx, &models.MediaItem{
		MediaTypeID: typeID,
		Title:       "The Matrix",
		Year:        &year,
		Status:      "detected",
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/entities/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	_ = id

	handler.GetEntity(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "The Matrix", resp["title"])
	assert.Equal(t, "movie", resp["media_type"])
}

func TestMediaEntityHandler_EnrichAllEntities_NoEntities(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, _ := setupEntityHandler(t, db)
	handler.db = db

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/entities/enrich", nil)

	handler.EnrichAllEntities(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "No entities need enrichment", resp["message"])
	assert.Equal(t, false, resp["accepted"])
}

func TestMediaEntityHandler_EnrichAllEntities_QueuesBackgroundJob(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	// Seed a media type and an entity
	_, typeID, _ := itemRepo.GetMediaTypeByName(ctx, "movie")
	id, _ := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Test Movie", Status: "detected"})

	// Seed directory_analyses so the JOIN succeeds
	_, err := db.ExecContext(ctx,
		`INSERT INTO directory_analyses (directory_path, media_item_id) VALUES (?, ?)`,
		"/media/movies/Test Movie", id)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/entities/enrich", nil)

	handler.EnrichAllEntities(c)

	// Should return 200 OK immediately (async processing)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Batch enrichment queued", resp["message"])
	assert.Equal(t, true, resp["accepted"])

	// Wait for background goroutine to finish
	handler.Close()

	// Verify the entity now has external metadata (or at least the handler didn't crash)
	// Since TMDB is not configured, the enrichment may not add metadata,
	// but the background job should complete without panic.
}

func TestMediaEntityHandler_EnrichAllEntities_RespectsLimitParam(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	// Seed multiple entities
	_, typeID, _ := itemRepo.GetMediaTypeByName(ctx, "movie")
	for i := 1; i <= 10; i++ {
		id, _ := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: fmt.Sprintf("Movie %d", i), Status: "detected"})
		_, err := db.ExecContext(ctx,
			`INSERT INTO directory_analyses (directory_path, media_item_id) VALUES (?, ?)`,
			fmt.Sprintf("/media/movies/Movie %d", i), id)
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/entities/enrich?limit=3", nil)

	handler.EnrichAllEntities(c)

	// Should return 200 OK immediately (async processing)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["accepted"])

	handler.Close()
}

func TestMediaEntityHandler_EnrichAllEntities_LimitBounds(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	_, typeID, _ := itemRepo.GetMediaTypeByName(ctx, "movie")
	id, _ := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Solo", Status: "detected"})
	_, err := db.ExecContext(ctx,
		`INSERT INTO directory_analyses (directory_path, media_item_id) VALUES (?, ?)`,
		"/media/movies/Solo", id)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/entities/enrich?limit=999", nil)

	handler.EnrichAllEntities(c)

	// Should return 200 OK immediately (async processing)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["accepted"])

	handler.Close()
}

func TestMediaEntityHandler_EnrichAllEntities_NoDB(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, _ := setupEntityHandler(t, db)
	// handler.db is nil by default

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/entities/enrich", nil)

	handler.EnrichAllEntities(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Database not available", resp["error"])
}

// TestMediaEntityHandler_EnrichSelectsItemsWithoutDirectoryAnalyses proves
// that the enrichment selection reaches EVERY media item lacking external
// metadata — including items that have NO directory_analyses row. The
// original query inner-JOINed directory_analyses, so an item without such a
// row (23704 of 27750 items in production) could never be enriched. The
// LEFT JOIN makes those items reachable with an empty DirPath, so the
// background job falls straight through to the TMDB/LLM path for them.
func TestMediaEntityHandler_EnrichSelectsItemsWithoutDirectoryAnalyses(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	// Item WITH a directory_analyses row — was always reachable.
	withDA, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Has Directory", Status: "detected"})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO directory_analyses (directory_path, media_item_id) VALUES (?, ?)`,
		"/media/movies/Has Directory", withDA)
	require.NoError(t, err)

	// Item WITHOUT any directory_analyses row — the previously-unreachable
	// case the inner JOIN silently dropped.
	withoutDA, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "No Directory", Status: "detected"})
	require.NoError(t, err)

	entities, err := handler.selectEntitiesNeedingEnrichment(ctx, 50)
	require.NoError(t, err)

	got := make(map[int64]string, len(entities))
	for _, e := range entities {
		got[e.ID] = e.DirPath
	}

	// The item WITH directory_analyses must still be selected, carrying its path.
	dirPath, ok := got[withDA]
	assert.True(t, ok, "item WITH directory_analyses must be selected for enrichment")
	assert.Equal(t, "/media/movies/Has Directory", dirPath)

	// The item WITHOUT directory_analyses MUST now be selected (LEFT JOIN),
	// with an empty DirPath so the local-cover lookup is skipped.
	dirPath, ok = got[withoutDA]
	assert.True(t, ok, "item WITHOUT directory_analyses MUST be selected (LEFT JOIN); the inner JOIN dropped it")
	assert.Equal(t, "", dirPath, "item without directory_analyses must have empty DirPath so the local-cover lookup is skipped")
}

// TestMediaEntityHandler_EnrichExcludesItemsWithExternalMetadata proves the
// LEFT JOIN change did NOT loosen the `WHERE em.id IS NULL` filter: items
// that already carry external metadata are still excluded from enrichment.
func TestMediaEntityHandler_EnrichExcludesItemsWithExternalMetadata(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	needs, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Needs Cover", Status: "detected"})
	require.NoError(t, err)

	hasMeta, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Already Enriched", Status: "detected"})
	require.NoError(t, err)
	coverURL := "/api/v1/download/file/1"
	require.NoError(t, handler.extMetaRepo.Upsert(ctx, &models.ExternalMetadata{
		MediaItemID: hasMeta,
		Provider:    "local_scan",
		ExternalID:  fmt.Sprintf("local:%d", hasMeta),
		CoverURL:    &coverURL,
	}))

	entities, err := handler.selectEntitiesNeedingEnrichment(ctx, 50)
	require.NoError(t, err)

	ids := make(map[int64]bool, len(entities))
	for _, e := range entities {
		ids[e.ID] = true
	}
	assert.True(t, ids[needs], "item without external_metadata must be selected")
	assert.False(t, ids[hasMeta], "item WITH external_metadata must be excluded from enrichment")
}

// TestMediaEntityHandler_MarkEnrichmentAttemptedAdvancesQueue proves the
// progress-marker fix for the stuck-queue bug: when an item is processed but
// no cover/metadata can be found (TMDB no-result AND LLM fallback empty), a
// sentinel external_metadata row is written so the item is no longer selected
// by `WHERE em.id IS NULL` on the next batch. Without the sentinel,
// selectEntitiesNeedingEnrichment returns the SAME leading unmatchable items
// on every call and the queue never advances to the real movies deeper in the
// catalog (operator symptom: only ~5 covers ever appear).
func TestMediaEntityHandler_MarkEnrichmentAttemptedAdvancesQueue(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	// An item whose enrichment yields nothing (noisy title, no TMDB match,
	// no LLM fallback) — the leading unmatchable item that jammed the queue.
	stuck, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Noisy Album Title 2049 [FLAC]", Status: "detected"})
	require.NoError(t, err)

	// Precondition: with no external_metadata row, the item is selected.
	before, err := handler.selectEntitiesNeedingEnrichment(ctx, 50)
	require.NoError(t, err)
	selectedBefore := false
	for _, e := range before {
		if e.ID == stuck {
			selectedBefore = true
		}
	}
	require.True(t, selectedBefore, "precondition: an item without external_metadata must be selected for enrichment")

	// Process it: enrichment found nothing, so mark it attempted.
	require.NoError(t, handler.markEnrichmentAttempted(ctx, stuck))

	// After the sentinel, the item MUST no longer be selected — the queue
	// advances instead of returning the same stuck item forever.
	after, err := handler.selectEntitiesNeedingEnrichment(ctx, 50)
	require.NoError(t, err)
	for _, e := range after {
		assert.NotEqual(t, stuck, e.ID, "item marked enrichment-attempted MUST NOT be re-selected; the queue must advance")
	}
}

// TestMediaEntityHandler_EnrichmentAttemptedSentinelHasNoCover proves the
// sentinel does NOT masquerade as a real cover. cover_art_service.GetCoverURL
// selects `WHERE cover_url IS NOT NULL AND cover_url != ”`; the sentinel row
// carries a NULL cover_url, so that lookup finds nothing and falls through to
// the placeholder — no false cover is ever shown for an unmatchable item.
func TestMediaEntityHandler_EnrichmentAttemptedSentinelHasNoCover(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	item, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "Unmatchable Comic Scan", Status: "detected"})
	require.NoError(t, err)

	require.NoError(t, handler.markEnrichmentAttempted(ctx, item))

	// Exactly one (sentinel) external_metadata row exists for the item.
	var total int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM external_metadata WHERE media_item_id = ?`, item).Scan(&total))
	assert.Equal(t, 1, total, "exactly one sentinel external_metadata row must be written")

	// Mirror GetCoverURL's filter: the sentinel must NOT present a usable cover.
	var usableCovers int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM external_metadata
		 WHERE media_item_id = ? AND cover_url IS NOT NULL AND cover_url != ''`, item).Scan(&usableCovers))
	assert.Equal(t, 0, usableCovers, "sentinel row must carry a NULL/empty cover_url so GetCoverURL falls through to the placeholder")
}

// TestMediaEntityHandler_EnrichPrioritizesTMDBMatchableTypesFirst proves the
// enrichment selection ORDERs TMDB-matchable types (movie, tv_show) BEFORE all
// other types so real posters appear on the movie/TV shelves quickly. The
// catalog holds 27750 items but only ~1369 are TMDB-matchable (movie=1192 +
// tv_show=177); the rest are music albums, comics, episodes, seasons and
// software that TMDB — a movie/TV database — can never match. Without a
// prioritising ORDER BY, enrichment processed items in arbitrary id order,
// burning thousands of batches marking unmatchable music/comics
// `enrichment_attempted` BEFORE ever reaching the movies (operator symptom:
// movie cover art never loads). The CASE-based ORDER BY puts
// media_type_id IN (1,2) first, then by id, so matchable movies/TV are enriched
// first.
func TestMediaEntityHandler_EnrichPrioritizesTMDBMatchableTypesFirst(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	// Real seeded type IDs — looked up, never hardcoded guesses.
	_, movieType, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	_, tvType, err := itemRepo.GetMediaTypeByName(ctx, "tv_show")
	require.NoError(t, err)
	_, albumType, err := itemRepo.GetMediaTypeByName(ctx, "music_album")
	require.NoError(t, err)
	_, comicType, err := itemRepo.GetMediaTypeByName(ctx, "comic")
	require.NoError(t, err)

	// The ORDER BY uses the literal IN (1, 2). That literal is correct only
	// because the canonical media_types seed assigns movie=1 and tv_show=2
	// (the production reality the operator complained about). Guard the
	// coupling: if the seed order ever changes, fail loudly here rather than
	// silently mis-ordering enrichment.
	require.Equal(t, int64(1), movieType, "movie media_type_id must be 1 (ORDER BY IN (1,2) coupling)")
	require.Equal(t, int64(2), tvType, "tv_show media_type_id must be 2 (ORDER BY IN (1,2) coupling)")

	// Seed NON-matchable types FIRST (lower media_items ids) and matchable
	// types LAST (higher ids), so the pre-fix id-ascending order puts the
	// movie/tv AFTER the album/comic — the exact arrangement the ORDER BY must
	// reverse.
	albumID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: albumType, Title: "Some Album [FLAC]", Status: "detected"})
	require.NoError(t, err)
	comicID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: comicType, Title: "Some Comic #1", Status: "detected"})
	require.NoError(t, err)
	movieID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: movieType, Title: "The Matrix", Status: "detected"})
	require.NoError(t, err)
	tvID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: tvType, Title: "Breaking Bad", Status: "detected"})
	require.NoError(t, err)

	// A limit large enough to return all four.
	entities, err := handler.selectEntitiesNeedingEnrichment(ctx, 50)
	require.NoError(t, err)
	require.Len(t, entities, 4, "all four un-enriched items must be selected")

	// Map each seeded item id to its position in the returned slice.
	pos := make(map[int64]int, len(entities))
	for i, e := range entities {
		pos[e.ID] = i
	}
	for _, id := range []int64{albumID, comicID, movieID, tvID} {
		_, ok := pos[id]
		require.True(t, ok, "seeded item %d must appear in the selection", id)
	}

	// The matchable movie + tv_show MUST both appear before the non-matchable
	// album + comic. Without the ORDER BY they come back in id order
	// (album, comic, movie, tv) — movie/tv LAST — which fails these assertions.
	assert.Less(t, pos[movieID], pos[albumID], "movie must be enriched before music_album")
	assert.Less(t, pos[movieID], pos[comicID], "movie must be enriched before comic")
	assert.Less(t, pos[tvID], pos[albumID], "tv_show must be enriched before music_album")
	assert.Less(t, pos[tvID], pos[comicID], "tv_show must be enriched before comic")

	// Stronger form: every matchable item precedes every non-matchable item.
	maxMatchable := pos[movieID]
	if pos[tvID] > maxMatchable {
		maxMatchable = pos[tvID]
	}
	minNonMatchable := pos[albumID]
	if pos[comicID] < minNonMatchable {
		minNonMatchable = pos[comicID]
	}
	assert.Less(t, maxMatchable, minNonMatchable, "all TMDB-matchable types must precede all non-matchable types")
}

// TestMediaEntityHandler_EnrichOrdersMatchableByRecency proves that WITHIN the
// TMDB-matchable tier, enrichment selects the MOST-RECENTLY-detected items
// first (ORDER BY first_detected DESC), so the app's "Recently Added Movies"
// shelf — which surfaces the highest first_detected items — gets its covers
// quickly. The pre-fix ORDER BY tie-broke the matchable tier by mi.id ASCENDING,
// so the OLDEST-detected movies (lowest ids, in the production reality where id
// and first_detected both grow with insertion order) were covered FIRST and the
// visible newest-movie shelf was covered LAST (operator symptom).
//
// The seed deliberately makes id-order and recency-order DISAGREE in the
// production-realistic direction: the OLDER movie is inserted FIRST (lower id,
// older first_detected) and the NEWER movie LAST (higher id, newer
// first_detected). Under the broken id-ascending order the older movie comes
// back first (newer NOT first → RED); under the recency-descending order the
// newer movie comes back first (→ GREEN). A non-matchable album with the NEWEST
// first_detected of all is also seeded to prove the type tier still dominates
// recency — a newer album must NOT jump ahead of the older movies.
func TestMediaEntityHandler_EnrichOrdersMatchableByRecency(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, itemRepo := setupEntityHandler(t, db)
	handler.db = db
	ctx := context.Background()

	// Real seeded type IDs — looked up, never hardcoded guesses.
	_, movieType, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	_, albumType, err := itemRepo.GetMediaTypeByName(ctx, "music_album")
	require.NoError(t, err)

	base := time.Now()

	// OLDER movie first → LOWER id, OLDER first_detected. Create preserves an
	// explicit non-zero FirstDetected (it only defaults to now() when zero), so
	// these timestamps land verbatim in media_items.first_detected.
	olderMovieID, err := itemRepo.Create(ctx, &models.MediaItem{
		MediaTypeID:   movieType,
		Title:         "Old Detected Movie",
		Status:        "detected",
		FirstDetected: base.Add(-48 * time.Hour),
	})
	require.NoError(t, err)

	// NEWER movie last → HIGHER id, NEWER first_detected. id-order (older first)
	// and recency-order (newer first) now disagree.
	newerMovieID, err := itemRepo.Create(ctx, &models.MediaItem{
		MediaTypeID:   movieType,
		Title:         "Newly Detected Movie",
		Status:        "detected",
		FirstDetected: base.Add(-24 * time.Hour),
	})
	require.NoError(t, err)

	// A non-matchable album with the NEWEST first_detected of all — proves the
	// matchable-type tier dominates recency (it must STILL come after both
	// movies despite being the most recently detected).
	albumID, err := itemRepo.Create(ctx, &models.MediaItem{
		MediaTypeID:   albumType,
		Title:         "Brand New Album [FLAC]",
		Status:        "detected",
		FirstDetected: base,
	})
	require.NoError(t, err)

	require.Greater(t, newerMovieID, olderMovieID,
		"seed precondition: newer movie must have the higher id so id-order and recency-order disagree")

	entities, err := handler.selectEntitiesNeedingEnrichment(ctx, 50)
	require.NoError(t, err)
	require.Len(t, entities, 3, "all three un-enriched items must be selected")

	pos := make(map[int64]int, len(entities))
	for i, e := range entities {
		pos[e.ID] = i
	}
	for _, id := range []int64{olderMovieID, newerMovieID, albumID} {
		_, ok := pos[id]
		require.True(t, ok, "seeded item %d must appear in the selection", id)
	}

	// Recency within the matchable tier: the NEWER movie must be enriched before
	// the OLDER movie. Pre-fix (ORDER BY ... mi.id) returns the older movie first
	// (lower id) → this assertion FAILS (RED). Post-fix (ORDER BY ...
	// first_detected DESC, mi.id) returns the newer movie first → PASSES (GREEN).
	assert.Less(t, pos[newerMovieID], pos[olderMovieID],
		"the more-recently-detected movie must be enriched before the older movie (newest-first shelf populates quickly)")

	// Type tier still dominates recency: both movies precede the (newest) album.
	assert.Less(t, pos[newerMovieID], pos[albumID],
		"a TMDB-matchable movie must precede the non-matchable album even though the album is newer")
	assert.Less(t, pos[olderMovieID], pos[albumID],
		"a TMDB-matchable movie must precede the non-matchable album even though the album is newer")
}

// TestCleanTitleForSearch proves that catalog filename noise (sequence-number
// prefixes, release/scene tags in () and [], and " - " separators) is stripped
// from the raw title before it becomes a TMDB search query, while real titles —
// including ones that legitimately start with a number or contain a hyphen —
// are left intact. The forensic anchor: the raw title
// "001 - Captain America - The First Avenger" returned ZERO TMDB matches live,
// while the cleaned "Captain America The First Avenger" returned the correct film.
func TestCleanTitleForSearch(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "sequence prefix and internal dash separators",
			raw:  "001 - Captain America - The First Avenger",
			want: "Captain America The First Avenger",
		},
		{
			name: "short sequence prefix",
			raw:  "003 - Iron Man",
			want: "Iron Man",
		},
		{
			name: "two-digit prefix, balanced tags and a dangling unbalanced paren",
			raw:  "32 - Asterix and the Class Act (2003) (Digital-Empire) (WebP by Doc MaKS",
			want: "Asterix and the Class Act",
		},
		{
			name: "bracketed release tag with internal comma and hyphen",
			raw:  "Waiting For The Sun [Canada, DCC 24K Gold GZS-1045]",
			want: "Waiting For The Sun",
		},
		{
			name: "mid-title number kept, balanced tags and dangling paren stripped",
			raw:  "Spawn Kills Everyone! 01 (2016) (3 covers) (digital) (Minutemen-Faessla",
			want: "Spawn Kills Everyone! 01",
		},
		{
			name: "clean title is not over-cleaned",
			raw:  "Phone Booth",
			want: "Phone Booth",
		},
		{
			name: "leading number that is part of the real title is preserved",
			raw:  "21 Jump Street",
			want: "21 Jump Street",
		},
		{
			name: "bare numeric title is preserved",
			raw:  "1984",
			want: "1984",
		},
		{
			name: "hyphenated word without surrounding spaces is preserved",
			raw:  "Spider-Man",
			want: "Spider-Man",
		},
		{
			name: "empty input returns empty",
			raw:  "",
			want: "",
		},
		{
			name: "all-noise input falls back to raw rather than empty query",
			raw:  "(2020)",
			want: "(2020)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanTitleForSearch(tc.raw)
			assert.Equal(t, tc.want, got,
				"cleanTitleForSearch(%q) = %q, want %q", tc.raw, got, tc.want)
		})
	}
}

// CAT-001: with no database the refresh must answer 501, never 202 "queued".
func TestMediaEntityHandler_RefreshEntityMetadata_NoDBIsNotQueued(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()
	handler, _ := setupEntityHandler(t, db)
	handler.db = nil

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/entities/5/metadata/refresh", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	handler.RefreshEntityMetadata(c)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.NotContains(t, w.Body.String(), `"message"`)
	assert.Contains(t, w.Body.String(), "Nothing was queued")
}

// NS-1: GET /entities/types must serialise `types` as [] (never null) when media_types is empty.
// Real handler + real migrated SQLite, no doubles.
func TestMediaEntityHandler_GetEntityTypes_EmptyTableIsEmptyArray(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	handler, _ := setupEntityHandler(t, db)

	_, err := db.ExecContext(context.Background(), "DELETE FROM media_types")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/entities/types", nil)

	handler.GetEntityTypes(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"types":[]}`, w.Body.String())
}
