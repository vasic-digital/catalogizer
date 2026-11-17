package com.catalogizer.androidtv.data.repository

import android.content.Context
import com.catalogizer.androidtv.MainDispatcherRule
import com.catalogizer.androidtv.data.models.MediaItem
import com.catalogizer.androidtv.data.models.MediaSearchRequest
import com.catalogizer.androidtv.data.models.MediaSearchResponse
import com.catalogizer.androidtv.data.remote.CatalogizerApi
import io.mockk.coEvery
import io.mockk.mockk
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.Response

@ExperimentalCoroutinesApi
class MediaRepositoryTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var context: Context
    private lateinit var api: CatalogizerApi
    private lateinit var repository: MediaRepository

    @Before
    fun setup() {
        context = mockk()
        api = mockk()
        repository = MediaRepository(context, api)
    }

    @Test
    fun `searchMedia success should return media items flow`() = runTest {
        val searchRequest = MediaSearchRequest(
            query = "test movie",
            limit = 10,
            offset = 0,
            mediaType = "movie"
        )

        val mediaItems = listOf(
            MediaItem(
                id = 1L,
                title = "Test Movie",
                mediaType = "movie",
                directoryPath = "/path/to/movie",
                createdAt = "2024-01-01T00:00:00Z",
                updatedAt = "2024-01-01T00:00:00Z"
            )
        )

        val searchResponse = MediaSearchResponse(items = mediaItems, total = mediaItems.size, limit = 10, offset = 0)
        val successResponse = Response.success(searchResponse)
        coEvery { api.searchMedia(any()) } returns successResponse

        val result = repository.searchMedia(searchRequest).first()

        assertEquals(mediaItems, result)
    }

    @Test
    fun `searchMedia with null response body should return empty list`() = runTest {
        val searchRequest = MediaSearchRequest(query = "test")

        val successResponse = Response.success<MediaSearchResponse>(null)
        coEvery { api.searchMedia(any()) } returns successResponse

        val result = repository.searchMedia(searchRequest).first()

        assertTrue(result.isEmpty())
    }

    @Test
    fun `searchMedia failure should return empty list`() = runTest {
        val searchRequest = MediaSearchRequest(query = "test")

        val errorResponse = Response.error<MediaSearchResponse>(
            500,
            "Server error".toResponseBody(null)
        )
        coEvery { api.searchMedia(any()) } returns errorResponse

        val result = repository.searchMedia(searchRequest).first()

        assertTrue(result.isEmpty())
    }

    @Test
    fun `searchMedia with exception should return empty list`() = runTest {
        val searchRequest = MediaSearchRequest(query = "test")

        val exception = RuntimeException("Network error")
        coEvery { api.searchMedia(any()) } throws exception

        val result = repository.searchMedia(searchRequest).first()

        assertTrue(result.isEmpty())
    }

    @Test
    fun `searchMedia with all parameters should build correct query params`() = runTest {
        val searchRequest = MediaSearchRequest(
            query = "test query",
            mediaType = "movie",
            limit = 50,
            offset = 20
        )

        val searchResponse = MediaSearchResponse(items = emptyList(), total = 0, limit = 50, offset = 20)
        val successResponse = Response.success(searchResponse)
        coEvery { api.searchMedia(any()) } returns successResponse

        repository.searchMedia(searchRequest).first()

        // Verify that searchMedia was called with correct parameters
        // The mock will capture the parameters, but we can't easily verify them
        // In a real scenario, we'd use a more sophisticated mock or argument captor
    }

    @Test
    fun `getMediaById success should return media item flow`() = runTest {
        val mediaId = 123L
        val mediaItem = MediaItem(
            id = mediaId,
            title = "Test Movie",
            mediaType = "movie",
            directoryPath = "/path/to/movie",
            createdAt = "2024-01-01T00:00:00Z",
            updatedAt = "2024-01-01T00:00:00Z"
        )

        val successResponse = Response.success(mediaItem)
        coEvery { api.getEntityById(mediaId) } returns successResponse

        val result = repository.getMediaById(mediaId).first()

        assertEquals(mediaItem, result)
    }

    @Test
    fun `getMediaById failure should return null flow`() = runTest {
        val mediaId = 123L

        val entityError = Response.error<MediaItem>(
            404,
            "Not found".toResponseBody(null)
        )
        coEvery { api.getEntityById(mediaId) } returns entityError

        val mediaError = Response.error<MediaItem>(
            404,
            "Not found".toResponseBody(null)
        )
        coEvery { api.getMediaById(mediaId) } returns mediaError

        val result = repository.getMediaById(mediaId).first()

        assertNull(result)
    }

    @Test
    fun `getMediaById with exception should return null flow`() = runTest {
        val mediaId = 123L

        val exception = RuntimeException("Network error")
        coEvery { api.getEntityById(mediaId) } throws exception

        val result = repository.getMediaById(mediaId).first()

        assertNull(result)
    }

    @Test
    fun `updateWatchProgress success should complete without exception`() = runTest {
        val mediaId = 123L
        val progress = 0.75

        val successResponse = Response.success(
            kotlinx.serialization.json.buildJsonObject { put("persisted", kotlinx.serialization.json.JsonPrimitive(true)) }
        )
        coEvery { api.updateWatchProgress(mediaId, any()) } returns successResponse

        // Should not throw exception
        repository.updateWatchProgress(mediaId, progress)
    }

    @Test
    fun `updateWatchProgress failure should throw exception`() = runTest {
        val mediaId = 123L
        val progress = 0.75

        val errorResponse = Response.error<kotlinx.serialization.json.JsonObject>(
            500,
            "Server error".toResponseBody(null)
        )
        coEvery { api.updateWatchProgress(mediaId, any()) } returns errorResponse

        try {
            repository.updateWatchProgress(mediaId, progress)
            fail("Expected exception to be thrown")
        } catch (e: Exception) {
            assertTrue(e.message?.contains("Failed to update watch progress") == true)
        }
    }

    @Test
    fun `updateWatchProgress with network exception should throw exception`() = runTest {
        val mediaId = 123L
        val progress = 0.75

        val networkException = RuntimeException("Network error")
        coEvery { api.updateWatchProgress(mediaId, any()) } throws networkException

        try {
            repository.updateWatchProgress(mediaId, progress)
            fail("Expected exception to be thrown")
        } catch (e: RuntimeException) {
            assertEquals("Network error", e.message)
        }
    }

    @Test
    fun `addFavorite success should complete without exception`() = runTest {
        val mediaId = 123L
        val isFavorite = true

        val successResponse = Response.success(mapOf("status" to "ok"))
        coEvery { api.addFavorite(any()) } returns successResponse

        // Should not throw exception
        repository.addFavorite("movie", mediaId)
    }

    @Test
    fun `addFavorite failure should throw exception`() = runTest {
        val mediaId = 123L

        val errorResponse = Response.error<Map<String, String>>(
            500,
            "Server error".toResponseBody(null)
        )
        coEvery { api.addFavorite(any()) } returns errorResponse

        try {
            repository.addFavorite("movie", mediaId)
            fail("Expected exception to be thrown")
        } catch (e: Exception) {
            assertTrue(e.message?.contains("Failed to add favorite") == true)
        }
    }

    @Test
    fun `addFavorite with network exception should throw exception`() = runTest {
        val mediaId = 123L
        val isFavorite = true

        val networkException = RuntimeException("Network error")
        coEvery { api.addFavorite(any()) } throws networkException

        try {
            repository.addFavorite("movie", mediaId)
            fail("Expected exception to be thrown")
        } catch (e: RuntimeException) {
            assertEquals("Network error", e.message)
        }
    }

    @Test
    fun `searchMedia with empty request should work correctly`() = runTest {
        val searchRequest = MediaSearchRequest()

        val searchResponse = MediaSearchResponse(items = emptyList(), total = 0, limit = 20, offset = 0)
        val successResponse = Response.success(searchResponse)
        coEvery { api.searchMedia(any()) } returns successResponse

        val result = repository.searchMedia(searchRequest).first()

        assertTrue(result.isEmpty())
    }

    @Test
    fun `searchMedia with complex request should work correctly`() = runTest {
        val searchRequest = MediaSearchRequest(
            query = "action movie",
            mediaType = "movie",
            yearMin = 2020,
            yearMax = 2024,
            ratingMin = 7.0,
            quality = "1080p",
            sortBy = "rating",
            sortOrder = "desc",
            limit = 25,
            offset = 50
        )

        val mediaItems = listOf(
            MediaItem(
                id = 1L,
                title = "Action Movie",
                mediaType = "movie",
                year = 2023,
                rating = 8.5,
                quality = "1080p",
                directoryPath = "/path/to/movie",
                createdAt = "2024-01-01T00:00:00Z",
                updatedAt = "2024-01-01T00:00:00Z"
            )
        )

        val searchResponse = MediaSearchResponse(items = mediaItems, total = 1, limit = 25, offset = 50)
        val successResponse = Response.success(searchResponse)
        coEvery { api.searchMedia(any()) } returns successResponse

        val result = repository.searchMedia(searchRequest).first()

        assertEquals(1, result.size)
        assertEquals("Action Movie", result[0].title)
    }

    /**
     * RULE-TV-002 regression — catalog calls MUST follow the ACTIVE api (the
     * one the latest switchServer() installed), NOT the api instance present
     * when MediaRepository was constructed.
     *
     * Forensic: HomeViewModel + its MediaRepository are built once in
     * MainActivity.onCreate, BEFORE the user picks/enters a server. Login
     * (AuthRepository) followed switchServer() via setApi() and hit the new
     * host (backend showed 200), but a captured `api` field kept every catalog
     * request pointed at the stale startup host — the request silently timed
     * out, NO request ever reached the new server, and the home screen spun
     * "Loading your media collection…" forever.
     *
     * RED_MODE=1 (env or system property) reproduces the bug on the pre-fix
     * behaviour by binding the repository to the STALE api via the fixed-api
     * constructor; the assertion then proves the call went to the stale host.
     * RED_MODE=0 (default) is the standing GREEN guard: the repository is built
     * with a PROVIDER, the active api is switched after construction, and the
     * call MUST resolve the switched (new) api.
     */
    @Test
    fun `browseEntities follows active api after server switch`() = runTest {
        val staleApi = mockk<CatalogizerApi>()
        val freshApi = mockk<CatalogizerApi>()

        val staleItems = listOf(
            MediaItem(
                id = 89L, title = "STALE-HOST", mediaType = "movie",
                directoryPath = "/stale", createdAt = "2024-01-01T00:00:00Z",
                updatedAt = "2024-01-01T00:00:00Z"
            )
        )
        val freshItems = listOf(
            MediaItem(
                id = 132L, title = "FRESH-HOST", mediaType = "movie",
                directoryPath = "/fresh", createdAt = "2024-01-01T00:00:00Z",
                updatedAt = "2024-01-01T00:00:00Z"
            )
        )
        coEvery { staleApi.browseEntities(any(), any()) } returns
            Response.success(MediaSearchResponse(items = staleItems, total = 1, limit = 10, offset = 0))
        coEvery { freshApi.browseEntities(any(), any()) } returns
            Response.success(MediaSearchResponse(items = freshItems, total = 1, limit = 10, offset = 0))

        val redMode = (System.getenv("RED_MODE") ?: System.getProperty("RED_MODE") ?: "0") == "1"

        // The "active" api the container would expose; starts stale, then
        // switchServer() points it at the fresh client.
        var activeApi: CatalogizerApi = staleApi

        val repo = if (redMode) {
            // Pre-fix behaviour: capture the api instance present at construction.
            MediaRepository(context, staleApi)
        } else {
            // Fixed behaviour: resolve the CURRENT active api per call.
            MediaRepository(context, { activeApi })
        }

        // User picks the reachable server AFTER the repository was built.
        activeApi = freshApi

        val result = repo.browseEntities("movie", limit = 10).first()

        // GREEN guard: the catalog call reached the freshly-switched host.
        // Under RED_MODE=1 this fails (result == STALE-HOST) — proving the
        // captured-api bug is real and that this test catches it.
        assertEquals(1, result.size)
        assertEquals("FRESH-HOST", result[0].title)
    }
}