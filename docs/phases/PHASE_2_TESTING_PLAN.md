# Phase 2 Testing Plan - Core Functionality Implementation

**Document Version:** 1.0
**Date:** 2025-11-11
**Status:** Ready for Testing
**Coverage:** 16/16 Implemented Features

---

## Table of Contents

1. [Testing Overview](#testing-overview)
2. [Android TV Testing](#android-tv-testing)
3. [Recommendation Service Testing](#recommendation-service-testing)
4. [Subtitle Service Testing](#subtitle-service-testing)
5. [Web UI Testing](#web-ui-testing)
6. [Android Sync Manager Testing](#android-sync-manager-testing)
7. [User Settings API Testing](#user-settings-api-testing)
8. [Integration Testing](#integration-testing)
9. [Performance Testing](#performance-testing)
10. [Security Testing](#security-testing)

---

## Testing Overview

### Test Environment Setup

**Prerequisites:**
- catalog-api server running (port 8080)
- PostgreSQL database initialized
- Redis cache running
- catalog-web dev server (port 5173)
- Android emulator/device with API 26+
- Android TV emulator/device

**Test Data Requirements:**
- Sample media files (movies, music, documents)
- Test user accounts with different roles
- SMB/FTP test shares
- Mock external metadata (TMDB, IMDB)

### Test Levels

| Level | Description | Coverage Goal |
|-------|-------------|---------------|
| **Unit** | Individual function testing | 90%+ |
| **Integration** | Component interaction testing | 80%+ |
| **E2E** | Complete user workflows | Key scenarios |
| **Performance** | Load and stress testing | Baseline metrics |
| **Security** | Authentication and authorization | Critical paths |

---

## Android TV Testing

### Test Suite 1: MediaRepository.searchMedia()

**File:** `catalogizer-androidtv/.../MediaRepository.kt`

#### Test Case 1.1: Basic Search
```kotlin
@Test
fun `searchMedia returns results for valid query`() = runTest {
    // Given
    val request = MediaSearchRequest(
        query = "Inception",
        limit = 20,
        offset = 0
    )

    // When
    val results = repository.searchMedia(request).first()

    // Then
    assertTrue(results.isNotEmpty())
    assertTrue(results.any { it.title.contains("Inception", ignoreCase = true) })
}
```

**Expected Result:** ✅ Returns matching media items
**Test Data:** Media items with title "Inception" in database
**Priority:** High

#### Test Case 1.2: Search with Filters
```kotlin
@Test
fun `searchMedia applies all filter parameters`() = runTest {
    val request = MediaSearchRequest(
        query = "Action",
        mediaType = "movie",
        yearMin = 2020,
        yearMax = 2023,
        ratingMin = 7.0,
        quality = "1080p",
        sortBy = "rating",
        sortOrder = "desc",
        limit = 10
    )

    val results = repository.searchMedia(request).first()

    assertTrue(results.all { it.media_type == "movie" })
    assertTrue(results.all { it.year in 2020..2023 })
    assertTrue(results.all { it.rating >= 7.0 })
}
```

**Expected Result:** ✅ Filters applied correctly
**Priority:** High

#### Test Case 1.3: Empty Results
```kotlin
@Test
fun `searchMedia returns empty list for no matches`() = runTest {
    val request = MediaSearchRequest(query = "NonExistentMovie12345")
    val results = repository.searchMedia(request).first()
    assertTrue(results.isEmpty())
}
```

**Expected Result:** ✅ Empty list without errors
**Priority:** Medium

#### Test Case 1.4: Network Error Handling
```kotlin
@Test
fun `searchMedia returns empty list on network error`() = runTest {
    // Simulate network failure
    coEvery { api.searchMedia(any()) } throws IOException()

    val results = repository.searchMedia(MediaSearchRequest()).first()

    assertTrue(results.isEmpty())
    // Verify error was logged
}
```

**Expected Result:** ✅ Graceful error handling
**Priority:** High

---

### Test Suite 2: MediaRepository.getMediaById()

#### Test Case 2.1: Valid Media ID
```kotlin
@Test
fun `getMediaById returns media for valid ID`() = runTest {
    val mediaId = 123L
    val media = repository.getMediaById(mediaId).first()

    assertNotNull(media)
    assertEquals(mediaId, media?.id)
}
```

**Manual Test Steps:**
1. Navigate to Android TV app
2. Open media browser
3. Select a media item
4. Verify media details load correctly
5. Check all fields are populated (title, description, rating, etc.)

**Expected Result:** ✅ Media details displayed
**Priority:** Critical

#### Test Case 2.2: Invalid Media ID
```kotlin
@Test
fun `getMediaById returns null for invalid ID`() = runTest {
    val media = repository.getMediaById(999999L).first()
    assertNull(media)
}
```

**Expected Result:** ✅ Returns null gracefully
**Priority:** Medium

---

### Test Suite 3: Watch Progress Updates

#### Test Case 3.1: Update Progress
```kotlin
@Test
fun `updateWatchProgress sends correct data to API`() = runTest {
    val mediaId = 123L
    val progress = 0.65

    repository.updateWatchProgress(mediaId, progress)

    coVerify {
        api.updateWatchProgress(
            mediaId,
            match { it["progress"] == progress }
        )
    }
}
```

**Manual Test Steps:**
1. Start playing a video
2. Watch for 30 seconds
3. Exit playback
4. Reopen the video
5. Verify playback resumes at correct position

**Expected Result:** ✅ Progress saved and restored
**Priority:** High

#### Test Case 3.2: Progress Validation
```kotlin
@Test
fun `updateWatchProgress handles edge cases`() = runTest {
    // Test 0% progress
    repository.updateWatchProgress(123L, 0.0)

    // Test 100% progress
    repository.updateWatchProgress(123L, 1.0)

    // Test mid-progress
    repository.updateWatchProgress(123L, 0.5)

    // All should succeed
}
```

**Expected Result:** ✅ All progress values accepted
**Priority:** Medium

---

### Test Suite 4: Favorite Status

#### Test Case 4.1: Toggle Favorite
```kotlin
@Test
fun `updateFavoriteStatus toggles favorite correctly`() = runTest {
    val mediaId = 123L

    // Add to favorites
    repository.updateFavoriteStatus(mediaId, true)
    coVerify { api.updateFavoriteStatus(mediaId, match { it["is_favorite"] == true }) }

    // Remove from favorites
    repository.updateFavoriteStatus(mediaId, false)
    coVerify { api.updateFavoriteStatus(mediaId, match { it["is_favorite"] == false }) }
}
```

**Manual Test Steps:**
1. Open media detail screen
2. Click favorite/heart icon
3. Verify icon changes to filled state
4. Navigate to favorites section
5. Verify media appears in favorites list
6. Click favorite icon again
7. Verify media removed from favorites

**Expected Result:** ✅ Favorite state persists
**Priority:** High

---

### Test Suite 5: Authentication

#### Test Case 5.1: Successful Login
```kotlin
@Test
fun `login saves token and user info on success`() = runTest {
    val username = "testuser"
    val password = "password123"

    coEvery { api.login(any()) } returns Response.success(
        LoginResponse(
            token = "jwt_token_here",
            userId = 1L,
            username = username
        )
    )

    val result = authRepository.login(username, password)

    assertTrue(result.isSuccess)
    // Verify token saved to DataStore
}
```

**Manual Test Steps:**
1. Open Android TV app
2. Enter the admin credentials created at first start from ADMIN_USERNAME / ADMIN_PASSWORD (docker compose reads them from .env; there is no default login; for a forgotten password see docs/guides/TROUBLESHOOTING.md: `catalog-api admin-reset-password <username>`)
3. Click "Sign In"
4. Verify navigation to home screen
5. Close app completely
6. Reopen app
7. Verify user still logged in

**Expected Result:** ✅ User authenticated and session persisted
**Priority:** Critical

#### Test Case 5.2: Invalid Credentials
```kotlin
@Test
fun `login returns failure for invalid credentials`() = runTest {
    coEvery { api.login(any()) } returns Response.error(401, "".toResponseBody())

    val result = authRepository.login("invalid", "wrong")

    assertTrue(result.isFailure)
}
```

**Expected Result:** ✅ Error message displayed
**Priority:** High

#### Test Case 5.3: Token Refresh
```kotlin
@Test
fun `refreshToken updates stored token`() = runTest {
    // Set up existing token
    dataStore.edit { it[TOKEN_KEY] = "old_token" }

    coEvery { api.refreshToken(any()) } returns Response.success(
        LoginResponse(token = "new_token", userId = 1L, username = "user")
    )

    val result = authRepository.refreshToken()

    assertTrue(result.isSuccess)
    // Verify new token stored
}
```

**Manual Test Steps:**
1. Login with valid credentials
2. Wait for token to approach expiration (or modify token expiry for testing)
3. Perform an API operation
4. Verify token refresh happens automatically
5. Verify operation succeeds

**Expected Result:** ✅ Seamless token refresh
**Priority:** High

---

## Recommendation Service Testing

### Test Suite 6: MediaType-Based Routing

**File:** `catalog-api/internal/services/recommendation_service.go`

#### Test Case 6.1: Movie Recommendations
```bash
curl -X POST http://localhost:8080/api/v1/media/similar \
  -H "Content-Type: application/json" \
  -d '{
    "media_metadata": {
      "media_type": "movie",
      "title": "The Matrix",
      "genre": "Action/Sci-Fi"
    },
    "max_external_items": 5,
    "include_external": true
  }'
```

**Expected Response:**
```json
{
  "local_items": [...],
  "external_items": [
    {
      "provider": "tmdb",
      "media_type": "movie",
      ...
    }
  ]
}
```

**Validation:**
- ✅ Only movie recommendations returned
- ✅ No music or other types mixed in
- ✅ Similarity score > threshold

**Priority:** High

#### Test Case 6.2: Music Recommendations
```bash
curl -X POST http://localhost:8080/api/v1/media/similar \
  -H "Content-Type: application/json" \
  -d '{
    "media_metadata": {
      "media_type": "music",
      "title": "Bohemian Rhapsody",
      "genre": "Rock"
    }
  }'
```

**Expected Result:** ✅ Music-specific recommendations
**Priority:** Medium

#### Test Case 6.3: Unknown Type Fallback
```bash
curl -X POST http://localhost:8080/api/v1/media/similar \
  -H "Content-Type: application/json" \
  -d '{
    "media_metadata": {
      "media_type": "unknown_type",
      "title": "Test Item"
    }
  }'
```

**Expected Result:** ✅ Falls back to checking both movie and music sources
**Priority:** Low

---

### Test Suite 7: Real Metadata Fetching

**File:** `catalog-api/internal/handlers/recommendation_handler.go`

#### Test Case 7.1: Get Similar Items with Real Metadata
```bash
curl -X GET "http://localhost:8080/api/v1/media/123/similar?max_local=10&include_external=true" \
  -H "Authorization: Bearer <token>"
```

**Expected Behavior:**
1. Fetches media ID 123 from database
2. Extracts metadata (title, genre, year, rating)
3. Uses metadata for similarity matching
4. Returns relevant recommendations

**Validation Checklist:**
- ✅ Media metadata loaded from database (not mock data)
- ✅ `convertFileToMediaMetadata()` correctly maps fields
- ✅ MediaType inferred from MIME type if not set
- ✅ All metadata fields populated (title, genre, cast, etc.)

**Priority:** Critical

#### Test Case 7.2: Metadata Conversion Accuracy
```go
func TestConvertFileToMediaMetadata(t *testing.T) {
    fileWithMetadata := &models.FileWithMetadata{
        File: models.File{
            ID: 123,
            Name: "movie.mp4",
            MimeType: "video/mp4",
        },
        Metadata: []models.FileMetadata{
            {Key: "title", Value: "The Matrix"},
            {Key: "year", Value: 1999.0},
            {Key: "rating", Value: 8.7},
            {Key: "genre", Value: "Sci-Fi"},
            {Key: "cast", Value: []interface{}{"Keanu Reeves", "Laurence Fishburne"}},
        },
    }

    metadata := convertFileToMediaMetadata(fileWithMetadata)

    assert.Equal(t, "The Matrix", metadata.Title)
    assert.Equal(t, 1999, *metadata.Year)
    assert.Equal(t, 8.7, *metadata.Rating)
    assert.Equal(t, "Sci-Fi", metadata.Genre)
    assert.Equal(t, "movie", metadata.MediaType)
    assert.Len(t, metadata.Cast, 2)
}
```

**Expected Result:** ✅ All fields correctly converted
**Priority:** High

---

## Subtitle Service Testing

### Test Suite 8: Subtitle Download Info Cache

**File:** `catalog-api/internal/services/subtitle_service.go`

#### Test Case 8.1: Cache Hit
```go
func TestGetDownloadInfo_CacheHit(t *testing.T) {
    ctx := context.Background()
    resultID := "subtitle_123"

    // Pre-populate cache
    expectedResult := &SubtitleSearchResult{
        ID: resultID,
        Provider: ProviderOpenSubtitles,
        Language: "English",
        DownloadURL: "https://example.com/subtitle.srt",
    }
    cacheService.Set(ctx, "subtitle_download_info:"+resultID, expectedResult, 1*time.Hour)

    // Test
    result, err := subtitleService.getDownloadInfo(ctx, resultID)

    assert.NoError(t, err)
    assert.Equal(t, expectedResult.ID, result.ID)
    assert.Equal(t, expectedResult.DownloadURL, result.DownloadURL)
}
```

**Expected Result:** ✅ Retrieved from cache without API call
**Priority:** High

#### Test Case 8.2: Cache Miss
```go
func TestGetDownloadInfo_CacheMiss(t *testing.T) {
    ctx := context.Background()
    resultID := "nonexistent_subtitle"

    result, err := subtitleService.getDownloadInfo(ctx, resultID)

    assert.Error(t, err)
    assert.Nil(t, result)
    assert.Contains(t, err.Error(), "not found")
}
```

**Expected Result:** ✅ Returns error for missing info
**Priority:** Medium

---

### Test Suite 9: Translation Caching

#### Test Case 9.1: Save and Retrieve Translation
```go
func TestTranslationCaching(t *testing.T) {
    ctx := context.Background()
    subtitleID := "sub_123"
    targetLang := "es"

    track := &SubtitleTrack{
        ID: "translated_sub_123",
        Language: "Spanish",
        LanguageCode: "es",
        Content: &translatedContent,
    }

    // Save
    err := subtitleService.saveCachedTranslation(ctx, subtitleID, targetLang, track)
    assert.NoError(t, err)

    // Retrieve
    retrieved := subtitleService.getCachedTranslation(ctx, subtitleID, targetLang)
    assert.NotNil(t, retrieved)
    assert.Equal(t, track.ID, retrieved.ID)
    assert.Equal(t, "Spanish", retrieved.Language)
}
```

**Expected Result:** ✅ Translation cached for 30 days
**Priority:** High

#### Test Case 9.2: Cache Expiration
```bash
# Manual test - modify TTL to 5 seconds for testing
# Wait 6 seconds
# Verify translation no longer in cache
```

**Expected Result:** ✅ Expired translations not returned
**Priority:** Low

---

### Test Suite 10: Video Info Retrieval

#### Test Case 10.1: Complete Video Metadata
```go
func TestGetVideoInfo_CompleteMetadata(t *testing.T) {
    ctx := context.Background()
    mediaItemID := int64(123)

    // Setup test data in file_metadata table
    // ...

    videoInfo, err := subtitleService.getVideoInfo(ctx, mediaItemID)

    assert.NoError(t, err)
    assert.Equal(t, 7200.0, videoInfo.Duration) // 2 hours
    assert.Equal(t, 23.976, videoInfo.FrameRate)
    assert.Equal(t, 1920, videoInfo.Width)
    assert.Equal(t, 1080, videoInfo.Height)
}
```

**Manual Test:**
1. Add media file with complete metadata
2. Call subtitle sync verification endpoint
3. Verify video info extracted correctly
4. Check logs for "Retrieved video info" message

**Expected Result:** ✅ All video properties extracted
**Priority:** High

#### Test Case 10.2: Resolution String Parsing
```go
func TestGetVideoInfo_ResolutionParsing(t *testing.T) {
    testCases := []struct {
        resolution string
        width      int
        height     int
    }{
        {"1920x1080", 1920, 1080},
        {"1280x720", 1280, 720},
        {"3840x2160", 3840, 2160},
    }

    for _, tc := range testCases {
        // Test regex parsing
        // ...
    }
}
```

**Expected Result:** ✅ All resolution formats parsed
**Priority:** Medium

---

## Web UI Testing

### Test Suite 11: MediaDetailModal Component

**File:** `catalog-web/src/components/media/MediaDetailModal.tsx`

#### Test Case 11.1: Modal Display
**Manual Test Steps:**
1. Open MediaBrowser page
2. Click on any media card
3. Verify modal opens with animation
4. Check all sections visible:
   - ✅ Backdrop image (if available)
   - ✅ Poster image
   - ✅ Title and metadata (year, rating, type, quality)
   - ✅ Genres as chips
   - ✅ Description text
   - ✅ Play and Download buttons
   - ✅ Technical details (file size, duration, storage, protocol)
   - ✅ Cast members (if available)
   - ✅ Available versions (if multiple)

**Expected Result:** ✅ All information displayed correctly
**Priority:** Critical

#### Test Case 11.2: Modal Interactions
```typescript
describe('MediaDetailModal', () => {
  it('calls onDownload when download button clicked', () => {
    const onDownload = jest.fn()
    render(<MediaDetailModal media={mockMedia} isOpen={true} onDownload={onDownload} />)

    fireEvent.click(screen.getByText('Download'))

    expect(onDownload).toHaveBeenCalledWith(mockMedia)
  })

  it('calls onClose when X button clicked', () => {
    const onClose = jest.fn()
    render(<MediaDetailModal media={mockMedia} isOpen={true} onClose={onClose} />)

    fireEvent.click(screen.getByLabelText('Close'))

    expect(onClose).toHaveBeenCalled()
  })

  it('formats file size correctly', () => {
    const media = { ...mockMedia, file_size: 1073741824 } // 1 GB
    render(<MediaDetailModal media={media} isOpen={true} />)

    expect(screen.getByText(/1\.00 GB/)).toBeInTheDocument()
  })
})
```

**Expected Result:** ✅ All interactions work
**Priority:** High

#### Test Case 11.3: Responsive Design
**Manual Test:**
1. Open modal on desktop (1920x1080)
2. Open modal on tablet (768x1024)
3. Open modal on mobile (375x667)
4. Verify layout adapts correctly at each breakpoint

**Expected Result:** ✅ Responsive on all screen sizes
**Priority:** Medium

---

### Test Suite 12: Download Functionality

**File:** `catalog-web/src/lib/mediaApi.ts`

#### Test Case 12.1: Successful Download
```typescript
describe('downloadMedia', () => {
  it('downloads file and triggers browser download', async () => {
    const media = {
      id: 123,
      title: 'Test Movie',
      directory_path: '/movies/test.mp4',
      storage_root_name: 'main_storage'
    }

    // Mock axios response
    axios.get.mockResolvedValue({
      data: new Blob(['test content'])
    })

    // Mock DOM
    const link = document.createElement('a')
    jest.spyOn(document, 'createElement').mockReturnValue(link)
    jest.spyOn(link, 'click')

    await mediaApi.downloadMedia(media)

    expect(axios.get).toHaveBeenCalledWith('/download', {
      params: {
        path: '/movies/test.mp4',
        storage: 'main_storage'
      },
      responseType: 'blob'
    })

    expect(link.click).toHaveBeenCalled()
  })
})
```

**Manual Test Steps:**
1. Open media detail modal
2. Click "Download" button
3. Verify download starts in browser
4. Check downloaded file:
   - ✅ Correct filename
   - ✅ Correct file size
   - ✅ File plays correctly

**Expected Result:** ✅ File downloads successfully
**Priority:** Critical

#### Test Case 12.2: Download Error Handling
```typescript
it('handles download errors gracefully', async () => {
  axios.get.mockRejectedValue(new Error('Network error'))

  const consoleSpy = jest.spyOn(console, 'error')

  await mediaApi.downloadMedia(mockMedia)

  expect(consoleSpy).toHaveBeenCalledWith('Download failed:', expect.any(Error))
})
```

**Manual Test:**
1. Disconnect network
2. Try to download a file
3. Verify error message displayed
4. Reconnect network
5. Retry download
6. Verify success

**Expected Result:** ✅ Graceful error handling with retry
**Priority:** High

---

## Android Sync Manager Testing

### Test Suite 13: Metadata Sync

**File:** `catalogizer-android/.../SyncManager.kt`

#### Test Case 13.1: Queue and Sync Metadata
```kotlin
@Test
fun `queueMetadataUpdate and syncMetadataUpdate work correctly`() = runTest {
    val mediaId = 123L
    val metadata = mapOf(
        "title" to "Updated Title",
        "description" to "New description",
        "year" to 2024,
        "rating" to 9.5
    )

    // Queue operation
    syncManager.queueMetadataUpdate(mediaId, metadata)

    // Verify queued
    val pendingOps = syncOperationDao.getPendingOperations()
    assertTrue(pendingOps.any { it.type == SyncOperationType.UPDATE_METADATA })

    // Perform sync
    val result = syncManager.performManualSync()

    assertTrue(result.success)
    assertEquals(1, result.syncedItems)

    // Verify API was called
    coVerify { api.updateMediaMetadata(mediaId, metadata) }

    // Verify local database updated
    val updatedMedia = database.mediaDao().getMediaById(mediaId)
    assertEquals("Updated Title", updatedMedia?.title)
}
```

**Expected Result:** ✅ Metadata synced to server and local DB
**Priority:** Critical

#### Test Case 13.2: Offline Metadata Queue
**Manual Test:**
1. Enable airplane mode on device
2. Edit media metadata (title, description)
3. Save changes
4. Verify "Pending sync" indicator shown
5. Disable airplane mode
6. Trigger manual sync or wait for auto-sync
7. Verify changes synced to server
8. Check server database for updated values

**Expected Result:** ✅ Offline changes queued and synced when online
**Priority:** High

---

### Test Suite 14: Media Deletion Sync

#### Test Case 14.1: Delete with Server Sync
```kotlin
@Test
fun `queueMediaDeletion removes media from server and local DB`() = runTest {
    val mediaId = 123L

    // Queue deletion
    syncManager.queueMediaDeletion(mediaId, localOnly = false)

    // Perform sync
    val result = syncManager.performManualSync()

    assertTrue(result.success)

    // Verify server deletion
    coVerify { api.deleteMedia(mediaId) }

    // Verify local deletion
    assertNull(database.mediaDao().getMediaById(mediaId))
    assertNull(database.watchProgressDao().getByMediaId(mediaId))
    assertNull(database.favoriteDao().getByMediaId(mediaId))
}
```

**Expected Result:** ✅ Media deleted from server and all local tables
**Priority:** Critical

#### Test Case 14.2: Local-Only Deletion
```kotlin
@Test
fun `local-only deletion removes from DB even if server fails`() = runTest {
    val mediaId = 123L

    // Simulate server error
    coEvery { api.deleteMedia(mediaId) } throws IOException()

    // Queue local-only deletion
    syncManager.queueMediaDeletion(mediaId, localOnly = true)

    // Perform sync
    val result = syncManager.performManualSync()

    // Should still succeed for local deletion
    assertTrue(result.success)
    assertNull(database.mediaDao().getMediaById(mediaId))
}
```

**Expected Result:** ✅ Local deletion succeeds even if server unavailable
**Priority:** High

#### Test Case 14.3: Cascade Deletion
**Manual Test:**
1. Select a media item with:
   - Watch progress
   - Favorite status
   - Multiple versions
2. Delete the media
3. Verify all related data deleted:
   - ✅ Media entry removed
   - ✅ Watch progress cleared
   - ✅ Favorite status removed
   - ✅ All versions deleted

**Expected Result:** ✅ Complete cascade deletion
**Priority:** High

---

## User Settings API Testing

### Test Suite 15: Settings Update

**File:** `catalog-api/handlers/user_handler.go`

#### Test Case 15.1: Update Settings
```bash
curl -X PUT http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "default_share": "main_storage",
      "auto_sync": true,
      "sync_interval_minutes": 30,
      "download_quality": "1080p",
      "cache": {
        "max_cache_size": 10737418240,
        "cache_thumbnails": true,
        "auto_clear_cache": false
      },
      "security": {
        "require_pin": true,
        "pin_timeout_minutes": 5
      }
    }
  }'
```

**Expected Response:**
```json
{
  "id": 1,
  "username": "admin",
  "settings": "{\"default_share\":\"main_storage\",\"auto_sync\":true,...}",
  ...
}
```

**Validation:**
- ✅ Settings JSON properly marshaled
- ✅ Stored in database as string
- ✅ No data loss in conversion

**Priority:** Critical

#### Test Case 15.2: Settings Retrieval
```bash
curl -X GET http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer <token>"
```

**Expected Response:**
```json
{
  "id": 1,
  "settings": "{\"default_share\":\"main_storage\",...}",
  ...
}
```

**Validation:**
- ✅ Settings returned as JSON string
- ✅ Client can parse back to object

**Priority:** High

#### Test Case 15.3: Invalid Settings JSON
```bash
curl -X PUT http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "invalid_field": [circular_reference]
    }
  }'
```

**Expected Response:** 500 Internal Server Error
**Expected Message:** "Failed to marshal settings"

**Priority:** Medium

---

## Integration Testing

### Test Suite 16: End-to-End Workflows

#### Workflow 1: Complete Media Discovery and Playback
```
1. User opens Android TV app
2. Authenticates with credentials
3. Browses media catalog
4. Searches for "Inception"
5. Selects movie from results
6. Views detailed information
7. Starts playback
8. Pauses at 50% progress
9. Exits playback
10. Progress saved to server
11. Reopens same movie
12. Playback resumes at 50%
13. Completes watching
14. Movie marked as watched
```

**Validation Points:**
- ✅ Authentication successful
- ✅ Search returns accurate results
- ✅ Media details load correctly
- ✅ Playback starts without errors
- ✅ Progress saves on pause
- ✅ Progress restores on resume
- ✅ Completion status updated

**Priority:** Critical

#### Workflow 2: Offline Media Management
```
1. User enables airplane mode
2. Edits media title
3. Marks media as favorite
4. Updates watch progress
5. Deletes a media item
6. All operations queued
7. Disables airplane mode
8. Automatic sync triggered
9. All queued operations processed
10. Server state matches local state
```

**Validation Points:**
- ✅ All operations queue offline
- ✅ UI shows pending sync indicator
- ✅ Sync completes on reconnection
- ✅ No data loss

**Priority:** High

#### Workflow 3: Web UI Media Browse and Download
```
1. User opens web browser
2. Navigates to MediaBrowser
3. Applies filters (type=movie, year=2020-2023)
4. Searches for "Matrix"
5. Clicks on "The Matrix"
6. Modal opens with full details
7. Clicks "Download" button
8. File download starts
9. Verifies downloaded file plays
```

**Validation Points:**
- ✅ Filters applied correctly
- ✅ Search results accurate
- ✅ Modal displays all info
- ✅ Download completes
- ✅ File integrity maintained

**Priority:** High

---

## Performance Testing

### Test Suite 17: Load Testing

#### Test 17.1: Concurrent API Requests
```bash
# Use Apache Bench or similar
ab -n 1000 -c 100 http://localhost:8080/api/v1/media/search?query=test
```

**Acceptance Criteria:**
- Response time p95 < 500ms
- Response time p99 < 1000ms
- 0% error rate
- Server CPU < 80%
- Memory usage stable

**Priority:** Medium

#### Test 17.2: Large Media Catalog
```
Database: 100,000 media items
Test: Search performance
```

**Validation:**
- ✅ Search returns in < 200ms
- ✅ Pagination works correctly
- ✅ Database indexes used

**Priority:** Medium

#### Test 17.3: Download Performance
```
File sizes: 100MB, 1GB, 5GB
Network: 100Mbps, 1Gbps
```

**Metrics:**
- Download speed matches network capacity
- No memory leaks during large downloads
- Proper chunk streaming

**Priority:** Low

---

## Security Testing

### Test Suite 18: Authentication and Authorization

#### Test 18.1: Unauthorized Access
```bash
# Without token
curl -X GET http://localhost:8080/api/v1/media/search

# Expected: 401 Unauthorized
```

**Priority:** Critical

#### Test 18.2: Expired Token
```bash
# With expired token
curl -X GET http://localhost:8080/api/v1/media/search \
  -H "Authorization: Bearer <expired_token>"

# Expected: 401 Unauthorized or automatic refresh
```

**Priority:** High

#### Test 18.3: SQL Injection Prevention
```bash
curl -X GET "http://localhost:8080/api/v1/media/search?query=';DROP TABLE media;--"

# Expected: Escaped query, no SQL execution
```

**Priority:** Critical

#### Test 18.4: XSS Prevention
```bash
curl -X PUT http://localhost:8080/api/v1/users/1 \
  -d '{"settings": {"default_share": "<script>alert(1)</script>"}}'

# Expected: Escaped/sanitized before storage
```

**Priority:** High

---

## Test Execution Plan

### Phase 1: Unit Tests (Days 1-2)
- Run all automated unit tests
- Fix failing tests
- Achieve 90%+ code coverage

### Phase 2: Integration Tests (Days 3-4)
- Execute API integration tests
- Test component interactions
- Verify data flow

### Phase 3: Manual/E2E Tests (Days 5-7)
- Execute manual test cases
- Run E2E workflows
- User acceptance testing

### Phase 4: Performance Tests (Day 8)
- Load testing
- Stress testing
- Benchmark establishment

### Phase 5: Security Tests (Day 9)
- Security scan
- Penetration testing
- Vulnerability assessment

### Phase 6: Regression Tests (Day 10)
- Full regression suite
- Bug fixes
- Final validation

---

## Test Results Template

### Test Execution Record

| Test ID | Test Name | Status | Date | Tester | Notes |
|---------|-----------|--------|------|--------|-------|
| ATV-1.1 | Basic Search | ⬜ Pass ⬜ Fail | | | |
| ATV-1.2 | Search Filters | ⬜ Pass ⬜ Fail | | | |
| ... | ... | | | | |

### Bug Report Template

**Bug ID:** BUG-XXX
**Severity:** Critical / High / Medium / Low
**Test Case:** Test ID
**Description:** What went wrong
**Steps to Reproduce:**
1. Step 1
2. Step 2
3. ...

**Expected Result:** What should happen
**Actual Result:** What actually happened
**Screenshots:** Attach if applicable
**Environment:** Android TV / Web / Android
**Assigned To:** Developer name
**Status:** Open / In Progress / Fixed / Verified

---

## Success Criteria

### Phase 2 Testing Complete When:

✅ All unit tests passing (90%+ coverage)
✅ All integration tests passing (80%+ coverage)
✅ Critical E2E workflows verified
✅ Performance benchmarks met
✅ Security tests passed
✅ Zero critical bugs remaining
✅ < 5 high-priority bugs
✅ Test report generated

---

## Appendix

### Testing Tools

- **Unit Testing:** JUnit, Jest, Go testing package
- **API Testing:** Postman, cURL, Insomnia
- **Load Testing:** Apache Bench, K6, JMeter
- **Security Testing:** OWASP ZAP, Snyk, SonarQube
- **Mobile Testing:** Android Emulator, Firebase Test Lab
- **Web Testing:** Chrome DevTools, React Testing Library

### Test Data Sets

Located in: `/tests/fixtures/`

- `media_items.json` - Sample media catalog
- `users.json` - Test user accounts
- `metadata_samples.json` - External metadata examples

---

**Document End**
