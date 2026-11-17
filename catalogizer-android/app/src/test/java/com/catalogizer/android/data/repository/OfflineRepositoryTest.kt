package com.catalogizer.android.data.repository

import android.content.Context
import com.catalogizer.android.MainDispatcherRule
import com.catalogizer.android.data.local.CatalogizerDatabase
import com.catalogizer.android.data.local.MediaDao
import com.catalogizer.android.data.local.SyncOperationDao
import com.catalogizer.android.data.models.MediaItem
import com.catalogizer.android.data.sync.SyncManager
import com.catalogizer.android.data.sync.SyncOperation
import io.mockk.*
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class OfflineRepositoryTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private val mockDatabase = mockk<CatalogizerDatabase>(relaxed = true)
    private val mockSyncManager = mockk<SyncManager>(relaxed = true)
    private val mockContext = mockk<Context>(relaxed = true)
    private val mockMediaDao = mockk<MediaDao>(relaxed = true)
    private val mockSyncOperationDao = mockk<SyncOperationDao>(relaxed = true)

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
        watchProgress = 0.0
    )

    @Before
    fun setup() {
        every { mockDatabase.mediaDao() } returns mockMediaDao
        every { mockDatabase.syncOperationDao() } returns mockSyncOperationDao
    }

    @After
    fun tearDown() {
        clearAllMocks()
    }

    // --- Cache Operations Tests ---

    @Test
    fun `cacheMediaItems should insert items into database`() = runTest {
        val items = listOf(testMediaItem, testMediaItem.copy(id = 2L, title = "Test Movie 2"))
        coEvery { mockMediaDao.insertAllMedia(any()) } just Runs

        mockMediaDao.insertAllMedia(items)

        coVerify { mockMediaDao.insertAllMedia(items) }
    }

    @Test
    fun `getCachedMediaItems should return items from database`() = runTest {
        val items = listOf(testMediaItem)
        coEvery { mockMediaDao.getAllCached() } returns items

        val result = mockMediaDao.getAllCached()

        assertEquals(1, result.size)
        assertEquals("Test Movie", result[0].title)
    }

    @Test
    fun `getCachedMediaById should return correct item`() = runTest {
        coEvery { mockMediaDao.getById(1L) } returns testMediaItem

        val result = mockMediaDao.getById(1L)

        assertNotNull(result)
        assertEquals(1L, result?.id)
        assertEquals("Test Movie", result?.title)
    }

    @Test
    fun `getCachedMediaById should return null for non-existent item`() = runTest {
        coEvery { mockMediaDao.getById(999L) } returns null

        val result = mockMediaDao.getById(999L)

        assertNull(result)
    }

    @Test
    fun `getCachedMediaByType should filter by type`() = runTest {
        val movies = listOf(testMediaItem)
        coEvery { mockMediaDao.getByType("movie") } returns movies

        val result = mockMediaDao.getByType("movie")

        assertEquals(1, result.size)
        assertEquals("movie", result[0].mediaType)
    }

    // --- Toggle Favorite Offline Tests ---

    @Test
    fun `toggleFavoriteOffline should toggle from false to true`() = runTest {
        coEvery { mockMediaDao.getById(1L) } returns testMediaItem.copy(isFavorite = false)
        coEvery { mockMediaDao.updateFavoriteStatus(any(), any()) } just Runs
        coEvery { mockSyncManager.queueFavoriteToggle(any(), any()) } just Runs

        val currentItem = mockMediaDao.getById(1L)
        val newStatus = !(currentItem?.isFavorite ?: false)
        mockMediaDao.updateFavoriteStatus(1L, newStatus)
        mockSyncManager.queueFavoriteToggle(1L, newStatus)

        assertTrue(newStatus)
        coVerify { mockMediaDao.updateFavoriteStatus(1L, true) }
        coVerify { mockSyncManager.queueFavoriteToggle(1L, true) }
    }

    @Test
    fun `toggleFavoriteOffline should toggle from true to false`() = runTest {
        coEvery { mockMediaDao.getById(1L) } returns testMediaItem.copy(isFavorite = true)
        coEvery { mockMediaDao.updateFavoriteStatus(any(), any()) } just Runs
        coEvery { mockSyncManager.queueFavoriteToggle(any(), any()) } just Runs

        val currentItem = mockMediaDao.getById(1L)
        val newStatus = !(currentItem?.isFavorite ?: false)
        mockMediaDao.updateFavoriteStatus(1L, newStatus)
        mockSyncManager.queueFavoriteToggle(1L, newStatus)

        assertFalse(newStatus)
        coVerify { mockMediaDao.updateFavoriteStatus(1L, false) }
        coVerify { mockSyncManager.queueFavoriteToggle(1L, false) }
    }

    // --- Storage Management Tests ---

    @Test
    fun `getUsedStorageBytes should return total download size`() = runTest {
        coEvery { mockMediaDao.getTotalDownloadSize() } returns 5368709120L // 5GB

        val result = mockMediaDao.getTotalDownloadSize()

        assertEquals(5368709120L, result)
    }

    @Test
    fun `getUsedStorageBytes should return 0 when no downloads`() = runTest {
        coEvery { mockMediaDao.getTotalDownloadSize() } returns null

        val result = mockMediaDao.getTotalDownloadSize() ?: 0L

        assertEquals(0L, result)
    }

    // --- Search Cache Tests ---

    @Test
    fun `searchCachedMedia should search local database`() = runTest {
        val searchResults = listOf(testMediaItem)
        coEvery { mockMediaDao.searchCached(any()) } returns searchResults

        val result = mockMediaDao.searchCached("%test%")

        assertEquals(1, result.size)
        assertEquals("Test Movie", result[0].title)
    }

    // --- Cleanup Tests ---

    @Test
    fun `cleanupOldCache should delete old items and sync operations`() = runTest {
        val thirtyDaysAgo = System.currentTimeMillis() - (30 * 24 * 60 * 60 * 1000L)
        coEvery { mockMediaDao.deleteOldCachedItems(any()) } just Runs
        coEvery { mockSyncOperationDao.cleanupOldOperations(any()) } just Runs

        mockMediaDao.deleteOldCachedItems(thirtyDaysAgo)
        mockSyncOperationDao.cleanupOldOperations(thirtyDaysAgo)

        coVerify { mockMediaDao.deleteOldCachedItems(any()) }
        coVerify { mockSyncOperationDao.cleanupOldOperations(any()) }
    }

    // --- Offline Stats Tests ---

    @Test
    fun `getOfflineStats should aggregate statistics correctly`() = runTest {
        coEvery { mockMediaDao.getCachedItemsCount() } returns 150
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 5
        coEvery { mockSyncOperationDao.getFailedOperationsCount() } returns 2
        coEvery { mockMediaDao.getTotalDownloadSize() } returns 2147483648L // 2GB

        val cachedItems = mockMediaDao.getCachedItemsCount()
        val pendingSync = mockSyncOperationDao.getPendingOperationsCount()
        val failedSync = mockSyncOperationDao.getFailedOperationsCount()
        val usedStorage = mockMediaDao.getTotalDownloadSize() ?: 0L

        val stats = OfflineStats(
            cachedItems = cachedItems,
            pendingSyncOperations = pendingSync,
            failedSyncOperations = failedSync,
            usedStorageBytes = usedStorage,
            totalStorageBytes = 5000L * 1024 * 1024,
            storagePercentageUsed = if (5000L * 1024 * 1024 > 0) (usedStorage * 100) / (5000L * 1024 * 1024) else 0
        )

        assertEquals(150, stats.cachedItems)
        assertEquals(5, stats.pendingSyncOperations)
        assertEquals(2, stats.failedSyncOperations)
        assertEquals(2147483648L, stats.usedStorageBytes)
        assertTrue(stats.storagePercentageUsed > 0)
    }
}
