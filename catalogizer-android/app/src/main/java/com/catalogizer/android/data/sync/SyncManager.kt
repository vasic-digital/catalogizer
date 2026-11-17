package com.catalogizer.android.data.sync

import com.catalogizer.android.data.models.FavoriteRequest
import com.catalogizer.android.data.models.RatingRequest
import com.catalogizer.android.data.models.WatchProgressRequest
import android.content.Context
import android.util.Log
import androidx.work.*
import androidx.work.ExistingPeriodicWorkPolicy
import com.catalogizer.android.data.local.CatalogizerDatabase
import com.catalogizer.android.data.remote.CatalogizerApi
import com.catalogizer.android.data.remote.toApiResult
import com.catalogizer.android.data.repository.AuthRepository
import com.catalogizer.android.data.repository.MediaRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.flow.*
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.encodeToString
import java.util.concurrent.TimeUnit
import kotlin.coroutines.coroutineContext

/**
 * Current state of the synchronization process including running status,
 * last sync timestamp, result, and pending operation count.
 */
@Serializable
data class SyncStatus(
    val isRunning: Boolean = false,
    val lastSyncTime: Long? = null,
    val lastSyncResult: SyncResult? = null,
    val pendingOperations: Int = 0
)

/**
 * Result of a sync operation indicating success/failure, item counts, and any error message.
 */
@Serializable
data class SyncResult(
    val success: Boolean,
    val timestamp: Long,
    val syncedItems: Int = 0,
    val failedItems: Int = 0,
    val errorMessage: String? = null
)

/**
 * Orchestrates bidirectional data synchronization between the local Room database
 * and the remote Catalogizer API. Supports periodic background sync via [WorkManager],
 * manual sync, and queuing of offline operations (favorites, progress, ratings)
 * with exponential backoff retry.
 */
class SyncManager(
    private val database: CatalogizerDatabase,
    private val api: CatalogizerApi,
    private val authRepository: AuthRepository,
    private val mediaRepository: MediaRepository,
    private val context: Context
) {

    private val _syncStatus = MutableStateFlow(SyncStatus())
    val syncStatus: StateFlow<SyncStatus> = _syncStatus.asStateFlow()
    private val syncOperationDao = database.syncOperationDao()
    private var manualSyncJob: Job? = null

    companion object {
        private const val TAG = "SyncManager"
        private const val SYNC_WORK_NAME = "catalogizer_sync"
        private const val BACKGROUND_SYNC_INTERVAL_HOURS = 6L
        private const val MAX_RETRY_ATTEMPTS = 3
        private val json = Json { ignoreUnknownKeys = true }
    }

    fun startPeriodicSync() {
        val constraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .setRequiresBatteryNotLow(true)
            .build()

        val syncRequest = PeriodicWorkRequestBuilder<SyncWorker>(
            BACKGROUND_SYNC_INTERVAL_HOURS,
            TimeUnit.HOURS
        )
            .setConstraints(constraints)
            .setBackoffCriteria(
                BackoffPolicy.EXPONENTIAL,
                WorkRequest.MIN_BACKOFF_MILLIS,
                TimeUnit.MILLISECONDS
            )
            .build()

        WorkManager.getInstance(context).enqueueUniquePeriodicWork(
            SYNC_WORK_NAME,
            ExistingPeriodicWorkPolicy.KEEP,
            syncRequest
        )
    }

    fun stopPeriodicSync() {
        WorkManager.getInstance(context).cancelUniqueWork(SYNC_WORK_NAME)
    }

    fun cancelSync() {
        manualSyncJob?.cancel()
        manualSyncJob = null
        WorkManager.getInstance(context).cancelUniqueWork(SYNC_WORK_NAME)
        _syncStatus.update { it.copy(isRunning = false) }
    }

    suspend fun performManualSync(): SyncResult {
        coroutineContext.ensureActive()

        if (_syncStatus.value.isRunning) {
            return SyncResult(
                success = false,
                timestamp = System.currentTimeMillis(),
                errorMessage = "Sync already in progress"
            )
        }

        _syncStatus.update { it.copy(isRunning = true) }

        return try {
            coroutineContext.ensureActive()
            val result = performSyncInternal()

            _syncStatus.update {
                it.copy(
                    isRunning = false,
                    lastSyncTime = result.timestamp,
                    lastSyncResult = result
                )
            }

            result
        } catch (e: Exception) {
            val result = SyncResult(
                success = false,
                timestamp = System.currentTimeMillis(),
                errorMessage = e.message ?: "Unknown sync error"
            )

            _syncStatus.update {
                it.copy(
                    isRunning = false,
                    lastSyncTime = result.timestamp,
                    lastSyncResult = result
                )
            }

            result
        }
    }

    private suspend fun performSyncInternal(): SyncResult {
        // Check authentication
        if (!authRepository.isTokenValid()) {
            return SyncResult(
                success = false,
                timestamp = System.currentTimeMillis(),
                errorMessage = "Not authenticated"
            )
        }

        var syncedItems = 0
        var failedItems = 0

        try {
            // 1. Sync pending operations (uploads, favorites, progress updates)
            val pendingOps = syncOperationDao.getPendingOperations()
            _syncStatus.update { it.copy(pendingOperations = pendingOps.size) }

            for (operation in pendingOps) {
                try {
                    when (operation.type) {
                        SyncOperationType.UPDATE_PROGRESS -> {
                            syncWatchProgress(operation)
                        }
                        SyncOperationType.TOGGLE_FAVORITE -> {
                            syncFavoriteStatus(operation)
                        }
                        SyncOperationType.UPLOAD_RATING -> {
                            syncRating(operation)
                        }
                        SyncOperationType.UPDATE_METADATA -> {
                            syncMetadataUpdate(operation)
                        }
                        SyncOperationType.DELETE_MEDIA -> {
                            syncMediaDeletion(operation)
                        }
                    }

                    // Mark operation as completed
                    syncOperationDao.deleteOperation(operation.id)
                    syncedItems++
                } catch (e: Exception) {
                    // Update retry count
                    if (operation.retryCount < MAX_RETRY_ATTEMPTS) {
                        syncOperationDao.updateRetryCount(
                            operation.id,
                            operation.retryCount + 1
                        )
                    } else {
                        // Max retries reached, delete operation
                        syncOperationDao.deleteOperation(operation.id)
                    }
                    failedItems++
                }
            }

            // 2. Download media updates from server
            val lastSyncTime = _syncStatus.value.lastSyncTime ?: 0L
            val updatedMedia = api.getUpdatedMedia(lastSyncTime).toApiResult().data ?: emptyList()

            // Update local database with server changes
            for (mediaItem in updatedMedia) {
                try {
                    database.mediaDao().insertOrUpdate(mediaItem)
                    syncedItems++
                } catch (e: Exception) {
                    failedItems++
                }
            }

            // 3. Sync user preferences and settings
            syncUserPreferences()

            return SyncResult(
                success = true,
                timestamp = System.currentTimeMillis(),
                syncedItems = syncedItems,
                failedItems = failedItems
            )

        } catch (e: Exception) {
            return SyncResult(
                success = false,
                timestamp = System.currentTimeMillis(),
                syncedItems = syncedItems,
                failedItems = failedItems,
                errorMessage = e.message
            )
        }
    }

    private suspend fun syncWatchProgress(operation: SyncOperation) {
        val progressData = operation.data?.let {
            Json.Default.decodeFromString<WatchProgressData>(it)
        } ?: return

        // A non-2xx answer must not be treated as synced (RFM-R1-7): throwing lets the caller keep the
        // operation queued and count a retry; after MAX_RETRY_ATTEMPTS it is dropped (existing policy).
        val response = api.updateUserWatchProgress(
            progressData.mediaId,
            WatchProgressRequest(
                progress = progressData.progress,
                timestamp = progressData.timestamp.toString()
            )
        )
        if (!response.isSuccessful) {
            throw java.io.IOException("Watch progress sync failed for media ${progressData.mediaId}: HTTP ${response.code()}")
        }
    }

    private suspend fun syncFavoriteStatus(operation: SyncOperation) {
        val favoriteData = operation.data?.let {
            Json.Default.decodeFromString<FavoriteData>(it)
        } ?: return

        // Server contract: PUT /media/:id/favorite {"favorite": bool}. A non-2xx answer must not be treated as
        // synced: throwing lets the caller keep the operation queued and count a retry.
        val response = api.setFavoriteStatus(favoriteData.mediaId, FavoriteRequest(favoriteData.isFavorite))
        if (!response.isSuccessful) {
            throw java.io.IOException("Favorite sync failed for media ${favoriteData.mediaId}: HTTP ${response.code()}")
        }
    }

    private suspend fun syncRating(operation: SyncOperation) {
        val ratingData = operation.data?.let {
            Json.Default.decodeFromString<RatingData>(it)
        } ?: return

        // The server takes an integer 0..10 only. An older queued operation can hold 4.5 or an out-of-range value;
        // it can never be accepted, so it is dropped with a logged reason instead of retried (RATING1-R-F3).
        val asInt = ratingData.rating.toInt()
        if (asInt.toDouble() != ratingData.rating || asInt !in 0..10) {
            Log.w(TAG, "Dropping rating sync for media ${ratingData.mediaId}: ${ratingData.rating} is not an integer 0..10")
            return
        }
        // Same contract as syncFavoriteStatus (RFM-R1-7): non-2xx throws so the operation stays queued.
        val response = api.rateMedia(ratingData.mediaId, RatingRequest(asInt))
        if (response.code() == 400) {
            // The server rejected the value itself; resending the same body can never succeed.
            Log.w(TAG, "Dropping rating sync for media ${ratingData.mediaId}: server answered HTTP 400 (invalid rating)")
            return
        }
        if (!response.isSuccessful) {
            throw java.io.IOException("Rating sync failed for media ${ratingData.mediaId}: HTTP ${response.code()}")
        }
    }

    private suspend fun syncMetadataUpdate(operation: SyncOperation) {
        val metadataData = operation.data?.let {
            Json.Default.decodeFromString<MetadataUpdateData>(it)
        } ?: return

        // Get current media item from local database
        val mediaItem = database.mediaDao().getMediaById(metadataData.mediaId)
        mediaItem?.let { media ->
            // Parse the metadata JSON string and update the media item
            val metadataMap = Json.Default.decodeFromString<Map<String, String>>(metadataData.metadata)
            val updatedMedia = media.copy(
                title = metadataMap["title"] ?: media.title,
                description = metadataMap["description"] ?: media.description,
                year = metadataMap["year"]?.toIntOrNull() ?: media.year,
                rating = metadataMap["rating"]?.toDoubleOrNull() ?: media.rating
            )

            // Send update to server
            api.updateMedia(metadataData.mediaId, updatedMedia)

            // Update local database
            database.mediaDao().insertOrUpdate(updatedMedia)
        }
    }

    private suspend fun syncMediaDeletion(operation: SyncOperation) {
        val deletionData = operation.data?.let {
            Json.Default.decodeFromString<MediaDeletionData>(it)
        } ?: return

        try {
            // Send deletion request to server
            api.deleteMedia(deletionData.mediaId)

            // Delete from local database
            database.mediaDao().deleteById(deletionData.mediaId)

            // Delete associated data (favorites, watch progress, etc.)
            database.watchProgressDao().deleteByMediaId(deletionData.mediaId)
            database.favoriteDao().deleteByMediaId(deletionData.mediaId)

        } catch (e: Exception) {
            // If server deletion fails but it's marked as local-only deletion,
            // still delete from local database
            if (deletionData.localOnly) {
                database.mediaDao().deleteById(deletionData.mediaId)
                database.watchProgressDao().deleteByMediaId(deletionData.mediaId)
                database.favoriteDao().deleteByMediaId(deletionData.mediaId)
            } else {
                throw e
            }
        }
    }

    private suspend fun syncUserPreferences() {
        try {
            val serverPrefs = api.getUserPreferences().toApiResult().data
            serverPrefs?.let {
                // Update local preferences with server data
                // This would involve updating SharedPreferences or DataStore
            }
        } catch (e: Exception) {
            // Ignore preference sync errors for now
        }
    }

    // Public methods for queueing offline operations
    suspend fun queueWatchProgressUpdate(mediaId: Long, progress: Double, timestamp: Long) {
        val data = WatchProgressData(mediaId, progress, timestamp)
        val operation = SyncOperation(
            type = SyncOperationType.UPDATE_PROGRESS,
            mediaId = mediaId,
            data = Json.Default.encodeToString(data),
            timestamp = System.currentTimeMillis()
        )

        syncOperationDao.insertOperation(operation)
        updatePendingOperationsCount()
    }

    suspend fun queueFavoriteToggle(mediaId: Long, isFavorite: Boolean) {
        val data = FavoriteData(mediaId, isFavorite)
        val operation = SyncOperation(
            type = SyncOperationType.TOGGLE_FAVORITE,
            mediaId = mediaId,
            data = Json.Default.encodeToString(data),
            timestamp = System.currentTimeMillis()
        )

        syncOperationDao.insertOperation(operation)
        updatePendingOperationsCount()
    }

    suspend fun queueRatingUpdate(mediaId: Long, rating: Int) {
        require(rating in 0..10) { "rating must be an integer 0..10, got $rating" }
        val data = RatingData(mediaId, rating.toDouble())
        val operation = SyncOperation(
            type = SyncOperationType.UPLOAD_RATING,
            mediaId = mediaId,
            data = Json.Default.encodeToString(data),
            timestamp = System.currentTimeMillis()
        )

        syncOperationDao.insertOperation(operation)
        updatePendingOperationsCount()
    }

    suspend fun queueMetadataUpdate(mediaId: Long, metadata: Map<String, Any>) {
        // Convert Map to JSON string for serialization
        val metadataJson = Json.Default.encodeToString(metadata.mapValues { it.value.toString() })
        val data = MetadataUpdateData(mediaId, metadataJson)
        val operation = SyncOperation(
            type = SyncOperationType.UPDATE_METADATA,
            mediaId = mediaId,
            data = Json.Default.encodeToString(data),
            timestamp = System.currentTimeMillis()
        )

        syncOperationDao.insertOperation(operation)
        updatePendingOperationsCount()
    }

    suspend fun queueMediaDeletion(mediaId: Long, localOnly: Boolean = false) {
        val data = MediaDeletionData(mediaId, localOnly)
        val operation = SyncOperation(
            type = SyncOperationType.DELETE_MEDIA,
            mediaId = mediaId,
            data = Json.Default.encodeToString(data),
            timestamp = System.currentTimeMillis()
        )

        syncOperationDao.insertOperation(operation)
        updatePendingOperationsCount()
    }

    private suspend fun updatePendingOperationsCount() {
        val count = syncOperationDao.getPendingOperationsCount()
        _syncStatus.update { it.copy(pendingOperations = count) }
    }

    suspend fun clearFailedOperations() {
        syncOperationDao.deleteFailedOperations(MAX_RETRY_ATTEMPTS)
        updatePendingOperationsCount()
    }

    suspend fun retryFailedOperations() {
        syncOperationDao.resetRetryCount()
        updatePendingOperationsCount()
    }
}

@Serializable
data class WatchProgressData(
    val mediaId: Long,
    val progress: Double,
    val timestamp: Long
)

@Serializable
data class FavoriteData(
    val mediaId: Long,
    val isFavorite: Boolean
)

@Serializable
data class RatingData(
    val mediaId: Long,
    val rating: Double
)

@Serializable
data class MetadataUpdateData(
    val mediaId: Long,
    val metadata: String // JSON string instead of Map<String, Any>
)

@Serializable
data class MediaDeletionData(
    val mediaId: Long,
    val localOnly: Boolean = false
)