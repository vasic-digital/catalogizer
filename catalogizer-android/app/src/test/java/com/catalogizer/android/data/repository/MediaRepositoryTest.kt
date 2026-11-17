package com.catalogizer.android.data.repository

import com.catalogizer.android.MainDispatcherRule
import com.catalogizer.android.data.local.MediaDao
import com.catalogizer.android.data.models.*
import com.catalogizer.android.data.remote.ApiResult
import com.catalogizer.android.data.remote.CatalogizerApi
import io.mockk.*
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
class MediaRepositoryTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var repository: MediaRepository
    private val mockApi = mockk<CatalogizerApi>(relaxed = true)
    private val mockMediaDao = mockk<MediaDao>(relaxed = true)

    private val testMediaItem = MediaItem(
        id = 1L,
        title = "Test Movie",
        mediaType = "movie",
        year = 2024,
        description = "A test movie",
        directoryPath = "/media/movies/test",
        createdAt = "2024-01-01T00:00:00Z",
        updatedAt = "2024-01-01T00:00:00Z",
        isFavorite = false,
        watchProgress = 0.0,
        rating = 8.5
    )

    private val testMediaItem2 = testMediaItem.copy(
        id = 2L,
        title = "Test Movie 2",
        year = 2023
    )

    @Before
    fun setup() {
        repository = MediaRepository(mockApi, mockMediaDao)
    }

    @After
    fun tearDown() {
        clearAllMocks()
    }

    // --- getMediaById Tests ---

    @Test
    fun `getMediaById should return local data when available and not force refresh`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns testMediaItem

        val result = repository.getMediaById(1L, forceRefresh = false)

        assertTrue(result.isSuccess)
        assertEquals("Test Movie", result.data?.title)
        coVerify(exactly = 0) { mockApi.getMediaById(any()) }
    }

    @Test
    fun `getMediaById should fetch from API when local data not available`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns null
        coEvery { mockApi.getMediaById(1L) } returns Response.success(testMediaItem)
        coEvery { mockMediaDao.insertMedia(any()) } just Runs

        val result = repository.getMediaById(1L, forceRefresh = false)

        assertTrue(result.isSuccess)
        assertEquals("Test Movie", result.data?.title)
        coVerify { mockApi.getMediaById(1L) }
        coVerify { mockMediaDao.insertMedia(testMediaItem) }
    }

    @Test
    fun `getMediaById with forceRefresh should always call API`() = runTest {
        coEvery { mockApi.getMediaById(1L) } returns Response.success(testMediaItem)
        coEvery { mockMediaDao.insertMedia(any()) } just Runs

        val result = repository.getMediaById(1L, forceRefresh = true)

        assertTrue(result.isSuccess)
        coVerify { mockApi.getMediaById(1L) }
    }

    @Test
    fun `getMediaById API failure should return error`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns null
        coEvery { mockApi.getMediaById(1L) } returns Response.error(
            404,
            okhttp3.ResponseBody.create(null, "Not found")
        )

        val result = repository.getMediaById(1L)

        assertFalse(result.isSuccess)
    }

    @Test
    fun `getMediaById with exception should return error`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns null
        coEvery { mockApi.getMediaById(1L) } throws RuntimeException("Network error")

        val result = repository.getMediaById(1L)

        assertFalse(result.isSuccess)
        assertEquals("Network error", result.error)
    }

    // --- getRecentMedia Tests ---

    @Test
    fun `getRecentMedia should fetch from API and cache locally`() = runTest {
        val items = listOf(testMediaItem, testMediaItem2)
        coEvery { mockApi.getRecentMedia(10) } returns Response.success(items)
        coEvery { mockMediaDao.insertAllMedia(any()) } just Runs

        val result = repository.getRecentMedia(10)

        assertTrue(result.isSuccess)
        assertEquals(2, result.data?.size)
        coVerify { mockMediaDao.insertAllMedia(items) }
    }

    @Test
    fun `getRecentMedia should fallback to local data on exception`() = runTest {
        val localItems = listOf(testMediaItem)
        coEvery { mockApi.getRecentMedia(any()) } throws RuntimeException("Network error")
        every { mockMediaDao.getRecentlyAdded(10) } returns flowOf(localItems)

        val result = repository.getRecentMedia(10)

        assertTrue(result.isSuccess)
        assertEquals(1, result.data?.size)
    }

    // --- getPopularMedia Tests ---

    @Test
    fun `getPopularMedia should fetch from API and cache locally`() = runTest {
        val items = listOf(testMediaItem)
        coEvery { mockApi.getPopularMedia(10) } returns Response.success(items)
        coEvery { mockMediaDao.insertAllMedia(any()) } just Runs

        val result = repository.getPopularMedia(10)

        assertTrue(result.isSuccess)
        assertEquals(1, result.data?.size)
    }

    @Test
    fun `getPopularMedia should fallback to local on exception`() = runTest {
        val localItems = listOf(testMediaItem)
        coEvery { mockApi.getPopularMedia(any()) } throws RuntimeException("Timeout")
        every { mockMediaDao.getTopRated(10) } returns flowOf(localItems)

        val result = repository.getPopularMedia(10)

        assertTrue(result.isSuccess)
        assertEquals(1, result.data?.size)
    }

    // --- toggleFavorite Tests ---

    @Test
    fun `toggleFavorite should add to favorites when not favorited`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns testMediaItem.copy(isFavorite = false)
        coEvery { mockMediaDao.updateFavoriteStatus(any(), any()) } just Runs
        coEvery { mockApi.setFavoriteStatus(1L, FavoriteRequest(true)) } returns Response.success(Unit)

        val result = repository.toggleFavorite(1L)

        assertTrue(result.isSuccess)
        coVerify { mockMediaDao.updateFavoriteStatus(1L, true) }
        coVerify { mockApi.setFavoriteStatus(1L, FavoriteRequest(true)) }
    }

    @Test
    fun `toggleFavorite should remove from favorites when favorited`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns testMediaItem.copy(isFavorite = true)
        coEvery { mockMediaDao.updateFavoriteStatus(any(), any()) } just Runs
        coEvery { mockApi.setFavoriteStatus(1L, FavoriteRequest(false)) } returns Response.success(Unit)

        val result = repository.toggleFavorite(1L)

        assertTrue(result.isSuccess)
        coVerify { mockMediaDao.updateFavoriteStatus(1L, false) }
        coVerify { mockApi.setFavoriteStatus(1L, FavoriteRequest(false)) }
    }

    @Test
    fun `toggleFavorite should revert local state on API failure`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns testMediaItem.copy(isFavorite = false)
        coEvery { mockMediaDao.updateFavoriteStatus(any(), any()) } just Runs
        coEvery { mockApi.setFavoriteStatus(1L, FavoriteRequest(true)) } returns Response.error(
            500,
            okhttp3.ResponseBody.create(null, "Server error")
        )

        val result = repository.toggleFavorite(1L)

        assertFalse(result.isSuccess)
        // Should revert: first call sets to true, second reverts to false
        coVerify(exactly = 2) { mockMediaDao.updateFavoriteStatus(1L, any()) }
    }

    @Test
    fun `toggleFavorite should revert local state on IOException (AFAV-5)`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } returns testMediaItem.copy(isFavorite = false)
        coEvery { mockMediaDao.updateFavoriteStatus(any(), any()) } just Runs
        coEvery { mockApi.setFavoriteStatus(1L, FavoriteRequest(true)) } throws java.io.IOException("offline")

        val result = repository.toggleFavorite(1L)

        assertFalse(result.isSuccess)
        coVerifyOrder {
            mockMediaDao.updateFavoriteStatus(1L, true)
            mockMediaDao.updateFavoriteStatus(1L, false)
        }
    }

    @Test
    fun `toggleFavorite with exception should return error`() = runTest {
        coEvery { mockMediaDao.getMediaById(1L) } throws RuntimeException("DB error")

        val result = repository.toggleFavorite(1L)

        assertFalse(result.isSuccess)
        assertEquals("DB error", result.error)
    }

    // --- updateWatchProgress Tests ---

    @Test
    fun `updateWatchProgress should update locally and sync with server`() = runTest {
        coEvery { mockMediaDao.updateWatchProgress(any(), any(), any()) } just Runs
        coEvery { mockApi.updateUserWatchProgress(any(), any()) } returns Response.success(Unit)

        val result = repository.updateWatchProgress(1L, 0.5)

        assertTrue(result.isSuccess)
        coVerify { mockMediaDao.updateWatchProgress(1L, 0.5, any()) }
        coVerify { mockApi.updateUserWatchProgress(1L, any()) }
    }

    @Test
    fun `updateWatchProgress with exception should return error`() = runTest {
        coEvery { mockMediaDao.updateWatchProgress(any(), any(), any()) } throws RuntimeException("IO error")

        val result = repository.updateWatchProgress(1L, 0.75)

        assertFalse(result.isSuccess)
    }

    // --- refreshAllMedia Tests ---

    @Test
    fun `refreshAllMedia should replace all local data`() = runTest {
        val searchResponse = MediaSearchResponse(
            items = listOf(testMediaItem, testMediaItem2),
            total = 2,
            limit = 1000,
            offset = 0
        )
        coEvery { mockApi.searchMedia(limit = 1000) } returns Response.success(searchResponse)
        coEvery { mockMediaDao.refreshMedia(any()) } just Runs

        val result = repository.refreshAllMedia()

        assertTrue(result.isSuccess)
        coVerify { mockMediaDao.refreshMedia(listOf(testMediaItem, testMediaItem2)) }
    }

    @Test
    fun `refreshAllMedia with API failure should return error`() = runTest {
        coEvery { mockApi.searchMedia(limit = 1000) } returns Response.error(
            500,
            okhttp3.ResponseBody.create(null, "Internal error")
        )

        val result = repository.refreshAllMedia()

        assertFalse(result.isSuccess)
    }

    // --- clearCache Tests ---

    @Test
    fun `clearCache should delete all media`() = runTest {
        coEvery { mockMediaDao.deleteAllMedia() } just Runs

        repository.clearCache()

        coVerify { mockMediaDao.deleteAllMedia() }
    }

    // --- getStreamUrl Tests ---

    @Test
    fun `getStreamUrl should return URL from response`() = runTest {
        val urlMap = mapOf("url" to "http://example.com/stream/1")
        coEvery { mockApi.getStreamUrl(1L) } returns Response.success(urlMap)

        val result = repository.getStreamUrl(1L)

        assertTrue(result.isSuccess)
        assertEquals("http://example.com/stream/1", result.data)
    }

    @Test
    fun `getStreamUrl should handle stream_url key`() = runTest {
        val urlMap = mapOf("stream_url" to "http://example.com/stream/1")
        coEvery { mockApi.getStreamUrl(1L) } returns Response.success(urlMap)

        val result = repository.getStreamUrl(1L)

        assertTrue(result.isSuccess)
        assertEquals("http://example.com/stream/1", result.data)
    }

    @Test
    fun `getStreamUrl with missing URL should return error`() = runTest {
        val urlMap = mapOf("other_key" to "value")
        coEvery { mockApi.getStreamUrl(1L) } returns Response.success(urlMap)

        val result = repository.getStreamUrl(1L)

        assertFalse(result.isSuccess)
        assertEquals("Stream URL not found in response", result.error)
    }

    // --- getDownloadUrl Tests ---

    @Test
    fun `getDownloadUrl should return URL from response`() = runTest {
        val urlMap = mapOf("url" to "http://example.com/download/1")
        coEvery { mockApi.getDownloadUrl(1L) } returns Response.success(urlMap)

        val result = repository.getDownloadUrl(1L)

        assertTrue(result.isSuccess)
        assertEquals("http://example.com/download/1", result.data)
    }

    @Test
    fun `getDownloadUrl with exception should return error`() = runTest {
        coEvery { mockApi.getDownloadUrl(1L) } throws RuntimeException("Network error")

        val result = repository.getDownloadUrl(1L)

        assertFalse(result.isSuccess)
        assertEquals("Network error", result.error)
    }

    // --- Flow Tests ---

    @Test
    fun `getMediaByIdFlow should return flow from DAO`() = runTest {
        every { mockMediaDao.getMediaByIdFlow(1L) } returns flowOf(testMediaItem)

        val result = repository.getMediaByIdFlow(1L).first()

        assertNotNull(result)
        assertEquals("Test Movie", result?.title)
    }

    @Test
    fun `getAllMediaTypes should return flow from DAO`() = runTest {
        every { mockMediaDao.getAllMediaTypes() } returns flowOf(listOf("movie", "tv_show", "music"))

        val result = repository.getAllMediaTypes().first()

        assertEquals(3, result.size)
        assertTrue(result.contains("movie"))
    }

    @Test
    fun `getTotalCount should return flow from DAO`() = runTest {
        every { mockMediaDao.getTotalCount() } returns flowOf(42)

        val result = repository.getTotalCount().first()

        assertEquals(42, result)
    }
}
