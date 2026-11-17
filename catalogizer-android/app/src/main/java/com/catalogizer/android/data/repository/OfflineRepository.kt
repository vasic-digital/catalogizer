package com.catalogizer.android.data.repository

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.*
import androidx.datastore.preferences.preferencesDataStore
import com.catalogizer.android.data.local.CatalogizerDatabase
import com.catalogizer.android.data.models.MediaItem
import com.catalogizer.android.data.sync.SyncManager
import kotlinx.coroutines.flow.*
import kotlinx.serialization.json.Json
import kotlinx.serialization.encodeToString
import kotlinx.serialization.decodeFromString
val Context.offlineDataStore: DataStore<Preferences> by preferencesDataStore(name = "offline_settings")

/**
 * Manages offline data operations including local media caching, search history,
 * storage limits, and queuing user actions (favorites, ratings, progress) for
 * deferred synchronization via [SyncManager].
 */
class OfflineRepository(
    private val database: CatalogizerDatabase,
    private val syncManager: SyncManager,
    private val context: Context
) {
    private val syncOperationDao = database.syncOperationDao()

    private val json = Json { ignoreUnknownKeys = true }
    private val dataStore = context.offlineDataStore

    companion object {
        private val OFFLINE_MODE_KEY = booleanPreferencesKey("offline_mode")
        private val AUTO_DOWNLOAD_KEY = booleanPreferencesKey("auto_download")
        private val DOWNLOAD_QUALITY_KEY = stringPreferencesKey("download_quality")
        private val WIFI_ONLY_KEY = booleanPreferencesKey("wifi_only")
        private val STORAGE_LIMIT_KEY = longPreferencesKey("storage_limit_mb")
        private val CACHED_SEARCH_QUERIES_KEY = stringPreferencesKey("cached_search_queries")
    }

    // Settings flows
    val isOfflineModeEnabled: Flow<Boolean> = dataStore.data.map { prefs ->
        prefs[OFFLINE_MODE_KEY] ?: false
    }

    val isAutoDownloadEnabled: Flow<Boolean> = dataStore.data.map { prefs ->
        prefs[AUTO_DOWNLOAD_KEY] ?: false
    }

    val downloadQuality: Flow<String> = dataStore.data.map { prefs ->
        prefs[DOWNLOAD_QUALITY_KEY] ?: "1080p"
    }

    val isWifiOnlyEnabled: Flow<Boolean> = dataStore.data.map { prefs ->
        prefs[WIFI_ONLY_KEY] ?: true
    }

    val storageLimitMB: Flow<Long> = dataStore.data.map { prefs ->
        prefs[STORAGE_LIMIT_KEY] ?: 5000L // 5GB default
    }

    // Offline operations
    suspend fun setOfflineMode(enabled: Boolean) {
        dataStore.edit { prefs ->
            prefs[OFFLINE_MODE_KEY] = enabled
        }

        if (enabled) {
            syncManager.startPeriodicSync()
        } else {
            syncManager.stopPeriodicSync()
        }
    }

    suspend fun setAutoDownload(enabled: Boolean) {
        dataStore.edit { prefs ->
            prefs[AUTO_DOWNLOAD_KEY] = enabled
        }
    }

    suspend fun setDownloadQuality(quality: String) {
        dataStore.edit { prefs ->
            prefs[DOWNLOAD_QUALITY_KEY] = quality
        }
    }

    suspend fun setWifiOnly(wifiOnly: Boolean) {
        dataStore.edit { prefs ->
            prefs[WIFI_ONLY_KEY] = wifiOnly
        }
    }

    suspend fun setStorageLimit(limitMB: Long) {
        dataStore.edit { prefs ->
            prefs[STORAGE_LIMIT_KEY] = limitMB
        }
    }

    // Cached data management
    suspend fun cacheMediaItems(items: List<MediaItem>) {
        database.mediaDao().insertAllMedia(items)
    }

    suspend fun getCachedMediaItems(): List<MediaItem> {
        return database.mediaDao().getAllCached()
    }

    suspend fun getCachedMediaById(id: Long): MediaItem? {
        return database.mediaDao().getById(id)
    }

    suspend fun getCachedMediaByType(type: String): List<MediaItem> {
        return database.mediaDao().getByType(type)
    }

    // Search history and caching
    suspend fun cacheSearchQuery(query: String, results: List<MediaItem>) {
        // Cache the search results
        cacheMediaItems(results)

        // Save the search query for offline suggestions
        val currentQueries = getCachedSearchQueries().toMutableSet()
        currentQueries.add(query)

        // Keep only the last 50 search queries
        val limitedQueries = currentQueries.toList().takeLast(50)

        dataStore.edit { prefs ->
            prefs[CACHED_SEARCH_QUERIES_KEY] = json.encodeToString(limitedQueries)
        }
    }

    suspend fun getCachedSearchQueries(): List<String> {
        return dataStore.data.map { prefs ->
            prefs[CACHED_SEARCH_QUERIES_KEY]?.let { queriesJson ->
                try {
                    json.decodeFromString<List<String>>(queriesJson)
                } catch (e: Exception) {
                    emptyList()
                }
            } ?: emptyList()
        }.first()
    }

    suspend fun searchCachedMedia(query: String): List<MediaItem> {
        return database.mediaDao().searchCached("%$query%")
    }

    // Offline progress tracking
    suspend fun updateWatchProgressOffline(mediaId: Long, progress: Double) {
        // Update local database immediately
        database.mediaDao().updateWatchProgress(mediaId, progress, System.currentTimeMillis().toString())

        // Queue for sync when online
        // syncManager.queueWatchProgressUpdate(mediaId, progress, System.currentTimeMillis())
    }

    suspend fun getUserRating(mediaId: Long): Int? = database.userRatingDao().get(mediaId)?.rating

    suspend fun toggleFavoriteOffline(mediaId: Long): Boolean {
        // Get current favorite status and toggle it
        val currentItem = database.mediaDao().getById(mediaId)
        val newFavoriteStatus = !(currentItem?.isFavorite ?: false)

        // Update local database immediately
        database.mediaDao().updateFavoriteStatus(mediaId, newFavoriteStatus)

        // Queue for sync when online
        syncManager.queueFavoriteToggle(mediaId, newFavoriteStatus)

        return newFavoriteStatus
    }

    /**
     * The user's own rating, integer 0..10 (server contract CAT-RATING-1). It is stored in the per-user
     * user_ratings table, NOT in media_items.rating (the community rating shown to everyone) (RATING1-R-F3).
     */
    suspend fun rateMediaOffline(mediaId: Long, rating: Int) {
        require(rating in 0..10) { "rating must be an integer 0..10, got $rating" }
        // Update local database immediately
        database.userRatingDao().upsert(com.catalogizer.android.data.local.UserRating(mediaId, rating))

        // Queue for sync when online
        syncManager.queueRatingUpdate(mediaId, rating)
    }

    // Storage management
    suspend fun getUsedStorageBytes(): Long {
        return database.mediaDao().getTotalDownloadSize() ?: 0L
    }

    suspend fun getAvailableStorageBytes(): Long {
        val limitBytes = storageLimitMB.first() * 1024 * 1024
        val usedBytes = getUsedStorageBytes()
        return maxOf(0L, limitBytes - usedBytes)
    }

    suspend fun isStorageAvailable(requiredBytes: Long): Boolean {
        return getAvailableStorageBytes() >= requiredBytes
    }

    suspend fun cleanupOldCache() {
        val thirtyDaysAgo = System.currentTimeMillis() - (30 * 24 * 60 * 60 * 1000L)

        // Remove old cached items that aren't favorites or recently watched
        database.mediaDao().deleteOldCachedItems(thirtyDaysAgo)

        // Cleanup old sync operations
        syncOperationDao.cleanupOldOperations(thirtyDaysAgo)
    }

    // Network status awareness
    suspend fun onNetworkAvailable() {
        // Trigger sync when network becomes available
        if (isOfflineModeEnabled.first()) {
            syncManager.performManualSync()
        }
    }

    suspend fun onNetworkUnavailable() {
        // Handle network loss - ensure we're in offline mode
        // No action needed as operations will queue automatically
    }

    // Data export/import for backup
    suspend fun exportOfflineData(): String {
        val mediaItems = getCachedMediaItems()
        val syncOperations = syncOperationDao.getAllOperations()
        val searchQueries = getCachedSearchQueries()

        val exportData = OfflineDataExport(
            mediaItems = mediaItems,
            syncOperations = syncOperations,
            searchQueries = searchQueries,
            exportTimestamp = System.currentTimeMillis()
        )

        return json.encodeToString(exportData)
    }

    suspend fun importOfflineData(exportedData: String): Boolean {
        return try {
            val importData = json.decodeFromString<OfflineDataExport>(exportedData)

            // Import media items
            database.mediaDao().insertAllMedia(importData.mediaItems)

            // Import sync operations
            syncOperationDao.insertOperations(importData.syncOperations)

            // Import search queries
            dataStore.edit { prefs ->
                prefs[CACHED_SEARCH_QUERIES_KEY] = json.encodeToString(importData.searchQueries)
            }

            true
        } catch (e: Exception) {
            false
        }
    }

    // Get offline statistics
    suspend fun getOfflineStats(): OfflineStats {
        val totalCachedItems = database.mediaDao().getCachedItemsCount()
        val pendingSync = syncOperationDao.getPendingOperationsCount()
        val failedSync = syncOperationDao.getFailedOperationsCount()
        val usedStorage = getUsedStorageBytes()
        val totalStorage = storageLimitMB.first() * 1024 * 1024

        return OfflineStats(
            cachedItems = totalCachedItems,
            pendingSyncOperations = pendingSync,
            failedSyncOperations = failedSync,
            usedStorageBytes = usedStorage,
            totalStorageBytes = totalStorage,
            storagePercentageUsed = if (totalStorage > 0) (usedStorage * 100) / totalStorage else 0
        )
    }
}

/**
 * Serializable container for exporting offline data (media items, sync operations,
 * and search queries) for backup and restore purposes.
 */
@kotlinx.serialization.Serializable
data class OfflineDataExport(
    val mediaItems: List<MediaItem>,
    val syncOperations: List<com.catalogizer.android.data.sync.SyncOperation>,
    val searchQueries: List<String>,
    val exportTimestamp: Long
)

/**
 * Summary statistics for the offline cache including item counts,
 * pending/failed sync operations, and storage usage.
 */
data class OfflineStats(
    val cachedItems: Int,
    val pendingSyncOperations: Int,
    val failedSyncOperations: Int,
    val usedStorageBytes: Long,
    val totalStorageBytes: Long,
    val storagePercentageUsed: Long
)