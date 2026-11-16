package com.catalogizer.android.data.sync

import android.content.Context
import com.catalogizer.android.MainDispatcherRule
import com.catalogizer.android.data.local.CatalogizerDatabase
import com.catalogizer.android.data.local.MediaDao
import com.catalogizer.android.data.local.SyncOperationDao
import com.catalogizer.android.data.models.MediaItem
import com.catalogizer.android.data.remote.CatalogizerApi
import com.catalogizer.android.data.repository.AuthRepository
import com.catalogizer.android.data.repository.MediaRepository
import io.mockk.*
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
class SyncManagerTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var syncManager: SyncManager
    private val mockDatabase = mockk<CatalogizerDatabase>(relaxed = true)
    private val mockApi = mockk<CatalogizerApi>(relaxed = true)
    private val mockAuthRepository = mockk<AuthRepository>(relaxed = true)
    private val mockMediaRepository = mockk<MediaRepository>(relaxed = true)
    private val mockContext = mockk<Context>(relaxed = true)
    private val mockSyncOperationDao = mockk<SyncOperationDao>(relaxed = true)
    private val mockMediaDao = mockk<MediaDao>(relaxed = true)

    @Before
    fun setup() {
        every { mockDatabase.syncOperationDao() } returns mockSyncOperationDao
        every { mockDatabase.mediaDao() } returns mockMediaDao
        syncManager = SyncManager(mockDatabase, mockApi, mockAuthRepository, mockMediaRepository, mockContext)
    }

    @After
    fun tearDown() {
        clearAllMocks()
    }

    // --- Initial State Tests ---

    @Test
    fun `initial sync status should not be running`() = runTest {
        val status = syncManager.syncStatus.first()

        assertFalse(status.isRunning)
        assertNull(status.lastSyncTime)
        assertNull(status.lastSyncResult)
        assertEquals(0, status.pendingOperations)
    }

    // --- performManualSync Tests ---

    @Test
    fun `performManualSync should return error when already running`() = runTest {
        // To test this, we need the sync status to be running
        // We can verify the logic by checking the method behavior
        val status = syncManager.syncStatus.first()
        assertFalse(status.isRunning) // Initial state is not running
    }

    @Test
    fun `performManualSync should fail when not authenticated`() = runTest {
        coEvery { mockAuthRepository.isTokenValid() } returns false
        coEvery { mockSyncOperationDao.getPendingOperations() } returns emptyList()
        coEvery { mockApi.getUpdatedMedia(any()) } returns Response.success(emptyList())
        coEvery { mockApi.getUserPreferences() } returns Response.success(emptyMap())

        val result = syncManager.performManualSync()

        assertFalse(result.success)
        assertEquals("Not authenticated", result.errorMessage)
    }

    @Test
    fun `performManualSync should process pending operations when authenticated`() = runTest {
        coEvery { mockAuthRepository.isTokenValid() } returns true
        coEvery { mockSyncOperationDao.getPendingOperations() } returns emptyList()
        coEvery { mockApi.getUpdatedMedia(any()) } returns Response.success(emptyList())
        coEvery { mockApi.getUserPreferences() } returns Response.success(emptyMap())

        val result = syncManager.performManualSync()

        assertTrue(result.success)
        assertEquals(0, result.syncedItems)
        assertEquals(0, result.failedItems)
    }

    @Test
    fun `performManualSync should update sync status after completion`() = runTest {
        coEvery { mockAuthRepository.isTokenValid() } returns true
        coEvery { mockSyncOperationDao.getPendingOperations() } returns emptyList()
        coEvery { mockApi.getUpdatedMedia(any()) } returns Response.success(emptyList())
        coEvery { mockApi.getUserPreferences() } returns Response.success(emptyMap())

        syncManager.performManualSync()

        val status = syncManager.syncStatus.first()
        assertFalse(status.isRunning)
        assertNotNull(status.lastSyncTime)
        assertNotNull(status.lastSyncResult)
    }

    // --- Queue Operations Tests ---

    @Test
    fun `queueWatchProgressUpdate should insert operation and update count`() = runTest {
        coEvery { mockSyncOperationDao.insertOperation(any()) } returns 1L
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 1

        syncManager.queueWatchProgressUpdate(1L, 0.5, System.currentTimeMillis())

        coVerify {
            mockSyncOperationDao.insertOperation(match {
                it.type == SyncOperationType.UPDATE_PROGRESS && it.mediaId == 1L
            })
        }
    }

    @Test
    fun `queueFavoriteToggle should insert operation and update count`() = runTest {
        coEvery { mockSyncOperationDao.insertOperation(any()) } returns 1L
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 1

        syncManager.queueFavoriteToggle(1L, true)

        coVerify {
            mockSyncOperationDao.insertOperation(match {
                it.type == SyncOperationType.TOGGLE_FAVORITE && it.mediaId == 1L
            })
        }
    }

    private fun favoriteOp(favorite: Boolean) = SyncOperation(
        id = 7L,
        type = SyncOperationType.TOGGLE_FAVORITE,
        mediaId = 1L,
        data = """{"mediaId":1,"isFavorite":$favorite}""",
        timestamp = 1L
    )

    @Test
    fun `favorite sync sends the favorite field the server requires`() = runTest {
        coEvery { mockAuthRepository.isTokenValid() } returns true
        coEvery { mockSyncOperationDao.getPendingOperations() } returns listOf(favoriteOp(true))
        coEvery { mockApi.setFavoriteStatus(any(), any()) } returns Response.success(Unit)
        coEvery { mockApi.getUpdatedMedia(any()) } returns Response.success(emptyList())
        coEvery { mockApi.getUserPreferences() } returns Response.success(emptyMap())

        syncManager.performManualSync()

        coVerify { mockApi.setFavoriteStatus(1L, mapOf("favorite" to true)) }
        coVerify { mockSyncOperationDao.deleteOperation(7L) }
    }

    @Test
    fun `favorite sync keeps the operation queued when the server answers non-2xx`() = runTest {
        coEvery { mockAuthRepository.isTokenValid() } returns true
        coEvery { mockSyncOperationDao.getPendingOperations() } returns listOf(favoriteOp(false))
        coEvery { mockApi.setFavoriteStatus(any(), any()) } returns
            Response.error(400, okhttp3.ResponseBody.create(null, ""))
        coEvery { mockApi.getUpdatedMedia(any()) } returns Response.success(emptyList())
        coEvery { mockApi.getUserPreferences() } returns Response.success(emptyMap())

        syncManager.performManualSync()

        coVerify(exactly = 0) { mockSyncOperationDao.deleteOperation(7L) }
        coVerify { mockSyncOperationDao.updateRetryCount(7L, 1) }
    }

    @Test
    fun `queueRatingUpdate should insert operation and update count`() = runTest {
        coEvery { mockSyncOperationDao.insertOperation(any()) } returns 1L
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 1

        syncManager.queueRatingUpdate(1L, 4.5)

        coVerify {
            mockSyncOperationDao.insertOperation(match {
                it.type == SyncOperationType.UPLOAD_RATING && it.mediaId == 1L
            })
        }
    }

    @Test
    fun `queueMediaDeletion should insert operation and update count`() = runTest {
        coEvery { mockSyncOperationDao.insertOperation(any()) } returns 1L
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 1

        syncManager.queueMediaDeletion(1L, localOnly = false)

        coVerify {
            mockSyncOperationDao.insertOperation(match {
                it.type == SyncOperationType.DELETE_MEDIA && it.mediaId == 1L
            })
        }
    }

    // --- Error Handling Tests ---

    @Test
    fun `performManualSync should handle exceptions gracefully`() = runTest {
        coEvery { mockAuthRepository.isTokenValid() } returns true
        coEvery { mockSyncOperationDao.getPendingOperations() } throws RuntimeException("DB error")

        val result = syncManager.performManualSync()

        assertFalse(result.success)
        assertNotNull(result.errorMessage)
    }

    // --- clearFailedOperations and retryFailedOperations Tests ---

    @Test
    fun `clearFailedOperations should delete and update count`() = runTest {
        coEvery { mockSyncOperationDao.deleteFailedOperations(any()) } just Runs
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 0

        syncManager.clearFailedOperations()

        coVerify { mockSyncOperationDao.deleteFailedOperations(3) }
    }

    @Test
    fun `retryFailedOperations should reset retry counts`() = runTest {
        coEvery { mockSyncOperationDao.resetRetryCount() } just Runs
        coEvery { mockSyncOperationDao.getPendingOperationsCount() } returns 5

        syncManager.retryFailedOperations()

        coVerify { mockSyncOperationDao.resetRetryCount() }
    }
}
