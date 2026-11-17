package com.catalogizer.androidtv.data.repository

import android.content.Context
import com.catalogizer.androidtv.data.models.MediaItem
import com.catalogizer.androidtv.data.models.MediaSearchRequest
import com.catalogizer.androidtv.data.models.MediaSearchResponse
import com.catalogizer.androidtv.data.models.PlaybackProgress
import com.catalogizer.androidtv.data.remote.CatalogizerApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.first
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull

/**
 * Handles media data operations for the Android TV app via the remote [CatalogizerApi].
 * Supports entity browsing, search, favorites, collections, recommendations,
 * playlists, and watch progress tracking.
 */
class MediaRepository(
    private val context: Context,
    // RULE-TV-002 (catalog-loads-against-stale-server fix) — resolve the
    // CatalogizerApi lazily per call rather than capturing one instance at
    // construction. HomeViewModel (and its MediaRepository) are built once in
    // MainActivity.onCreate, BEFORE the user picks/enters a server URL via the
    // login screen's Connect / Sign In. switchServer() rebuilds the Retrofit
    // client and AuthRepository picks up the new one via setApi(), so login
    // hits the chosen host — but a captured `api` field kept every catalog
    // request pointed at the stale startup URL, so the request silently timed
    // out against an unreachable host with no request ever reaching the new
    // server (infinite "Loading your media collection…" spinner). Reading the
    // current api through a provider makes every catalog call follow the active
    // base URL set by the most recent switchServer().
    private val apiProvider: () -> CatalogizerApi,
) {
    // Convenience: deprecated direct-instance constructor kept for callers/tests
    // that pass a fixed api. Wraps it in a constant provider.
    constructor(context: Context, api: CatalogizerApi) : this(context, { api })

    private val api: CatalogizerApi
        get() = apiProvider()


    /**
     * Browse entities by type (movie, tv_show, etc.) — includes external metadata with cover URLs.
     */
    suspend fun browseEntities(type: String, limit: Int = 20, sortBy: String = "title", sortOrder: String = "asc"): Flow<List<MediaItem>> {
        try {
            val params = mapOf("limit" to limit.toString(), "sort_by" to sortBy, "sort_order" to sortOrder)
            val response = api.browseEntities(type, params)
            if (response.isSuccessful) {
                val items = response.body()?.allItems ?: emptyList()
                return flowOf(items)
            } else {
                android.util.Log.w("MediaRepo", "Browse entities failed: ${response.code()}")
                return flowOf(emptyList())
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "Browse entities error: ${e.message}")
            return flowOf(emptyList())
        }
    }

    suspend fun searchMedia(request: MediaSearchRequest): Flow<List<MediaItem>> {
        try {
            val params = mutableMapOf<String, String>()
            request.query?.let { params["query"] = it }
            params["limit"] = request.limit.toString()
            params["offset"] = request.offset.toString()
            request.mediaType?.let { params["media_types"] = it }
            request.sortBy?.let { params["sort_by"] = it }
            request.sortOrder?.let { params["sort_order"] = it }

            val response = api.searchMedia(params)
            if (response.isSuccessful) {
                val searchResponse = response.body()
                val mediaItems = searchResponse?.allItems ?: emptyList()
                return flowOf(mediaItems)
            } else {
                android.util.Log.w("MediaRepo", "Search failed: ${response.code()} ${response.message()}")
                return flowOf(emptyList())
            }
        } catch (e: Exception) {
            return flowOf(emptyList())
        }
    }

    /**
     * Search entities (aggregated media items with titles, covers, metadata).
     * Uses GET /api/v1/entities?query=... which searches entity titles
     * via LIKE matching. This is the user-facing search — returns movies,
     * TV shows, music albums, etc. by title.
     */
    suspend fun searchEntities(request: MediaSearchRequest): Flow<List<MediaItem>> {
        try {
            val params = mutableMapOf<String, String>()
            request.query?.let { params["query"] = it }
            params["limit"] = request.limit.toString()
            params["offset"] = request.offset.toString()
            request.mediaType?.let { params["type"] = it }

            val response = api.searchEntities(params)
            if (response.isSuccessful) {
                val searchResponse = response.body()
                val mediaItems = searchResponse?.allItems ?: emptyList()
                return flowOf(mediaItems)
            } else {
                android.util.Log.w("MediaRepo", "Entity search failed: ${response.code()}")
                return flowOf(emptyList())
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "Entity search error: ${e.message}")
            return flowOf(emptyList())
        }
    }

    /**
     * Fetch child entities of a parent (e.g. episodes of a season).
     * Returns an empty list on any error so callers degrade gracefully.
     */
    suspend fun getEntityChildren(parentId: Long): List<MediaItem> {
        return try {
            val response = api.getEntityChildren(parentId)
            if (response.isSuccessful) {
                response.body()?.allItems ?: emptyList()
            } else {
                android.util.Log.w("MediaRepo", "getEntityChildren failed: ${response.code()}")
                emptyList()
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getEntityChildren error: ${e.message}")
            emptyList()
        }
    }

    suspend fun getMediaById(mediaId: Long): Flow<MediaItem?> {
        try {
            // Try entity endpoint first (for entities from browse)
            val entityResponse = api.getEntityById(mediaId)
            if (entityResponse.isSuccessful) {
                return flowOf(entityResponse.body())
            }
            // Fallback to media/file endpoint
            val response = api.getMediaById(mediaId)
            if (response.isSuccessful) {
                return flowOf(response.body())
            }
            android.util.Log.w("MediaRepo", "getMediaById failed: entity=${entityResponse.code()} media=${response.code()}")
            return flowOf(null)
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getMediaById error: ${e.message}")
            return flowOf(null)
        }
    }

    /**
     * PUT /api/v1/media/:id/progress. Sends the fraction and, when the player knows it, position and duration in
     * seconds (WPROG3-FU-1: without a duration the server answers 200 persisted=false for an item that has no row
     * yet, so a first watch was silently not stored). Only persisted=true is reported as [WatchProgressOutcome.Saved]
     * (WPROG3-FU-3); a 409 concurrent_update is re-sent (the server re-reads and re-plans) up to
     * [WATCH_PROGRESS_MAX_ATTEMPTS] times. Other non-2xx answers and transport errors throw.
     */
    suspend fun saveWatchProgress(
        mediaId: Long,
        progress: Double,
        positionMs: Long? = null,
        durationMs: Long? = null,
    ): WatchProgressOutcome {
        val body = buildWatchProgressBody(progress, positionMs, durationMs)
        var lastReason: String? = null
        for (attempt in 1..WATCH_PROGRESS_MAX_ATTEMPTS) {
            val response = api.updateWatchProgress(mediaId, body)
            if (response.code() == 409) {
                lastReason = parseReason(response.errorBody()?.string()) ?: "concurrent_update"
                continue
            }
            if (!response.isSuccessful) {
                throw Exception("Failed to update watch progress: ${response.message()}")
            }
            val json = response.body()
            val persisted = (json?.get("persisted") as? JsonPrimitive)?.booleanOrNull == true
            return if (persisted) {
                WatchProgressOutcome.Saved
            } else {
                WatchProgressOutcome.NotStored((json?.get("reason") as? JsonPrimitive)?.contentOrNull)
            }
        }
        return WatchProgressOutcome.NotStored(lastReason)
    }

    /** Fraction-only convenience kept for existing callers; throws when the server did not store the progress. */
    suspend fun updateWatchProgress(mediaId: Long, progress: Double) {
        val out = saveWatchProgress(mediaId, progress)
        if (out is WatchProgressOutcome.NotStored) {
            throw WatchProgressNotStoredException(out.reason)
        }
    }

    private fun parseReason(raw: String?): String? = try {
        if (raw.isNullOrBlank()) null
        else ((Json.parseToJsonElement(raw) as? JsonObject)?.get("reason") as? JsonPrimitive)?.contentOrNull
    } catch (_: Exception) {
        null
    }

    companion object {
        const val WATCH_PROGRESS_MAX_ATTEMPTS = 3

        /** Position and duration are sent in SECONDS (player values are milliseconds); omitted when unknown. */
        fun buildWatchProgressBody(progress: Double, positionMs: Long?, durationMs: Long?): Map<String, Double> {
            val body = linkedMapOf("progress" to progress)
            if (durationMs != null && durationMs > 0) {
                body["duration"] = durationMs / 1000.0
                if (positionMs != null && positionMs >= 0) body["position"] = positionMs / 1000.0
            }
            return body
        }
    }

    /**
     * Check whether an entity is favorited by the current user.
     * Returns false on any error (network, auth, etc.) so the UI degrades gracefully.
     */
    suspend fun checkFavorite(entityType: String, entityId: Long): Boolean {
        return try {
            val response = api.checkFavorite(entityType, entityId)
            if (response.isSuccessful) {
                response.body()?.get("is_favorite") ?: false
            } else {
                android.util.Log.w("MediaRepo", "checkFavorite failed: ${response.code()}")
                false
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "checkFavorite error: ${e.message}")
            false
        }
    }

    /**
     * Add an entity to the current user's favorites.
     */
    suspend fun addFavorite(entityType: String, entityId: Long) {
        val body = com.catalogizer.androidtv.data.remote.FavoriteRequest(entityType, entityId)
        val response = api.addFavorite(body)
        if (!response.isSuccessful) {
            throw Exception("Failed to add favorite: ${response.code()} ${response.message()}")
        }
    }

    /**
     * Remove an entity from the current user's favorites.
     */
    suspend fun removeFavorite(entityType: String, entityId: Long) {
        val response = api.removeFavorite(entityType, entityId)
        if (!response.isSuccessful) {
            throw Exception("Failed to remove favorite: ${response.code()} ${response.message()}")
        }
    }

    /**
     * Toggle favorite status: adds if not favorited, removes if already favorited.
     * Returns the new favorite state.
     */
    suspend fun toggleFavorite(entityType: String, entityId: Long, currentlyFavorite: Boolean): Boolean {
        if (currentlyFavorite) {
            removeFavorite(entityType, entityId)
            return false
        } else {
            addFavorite(entityType, entityId)
            return true
        }
    }

    suspend fun getEntityStats(): Pair<Int, Map<String, Int>> {
        val response = api.getEntityStats()
        if (response.isSuccessful) {
            val body = response.body()
            return Pair(body?.totalEntities ?: 0, body?.byType ?: emptyMap())
        }
        return Pair(0, emptyMap())
    }

    // --- Collections ---

    /**
     * Fetch all collections from the server.
     */
    suspend fun getCollections(): kotlinx.serialization.json.JsonObject? {
        return try {
            val response = api.getCollections()
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "getCollections failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getCollections error: ${e.message}")
            null
        }
    }

    /**
     * Fetch a single collection by ID.
     */
    suspend fun getCollection(id: Long): kotlinx.serialization.json.JsonObject? {
        return try {
            val response = api.getCollection(id)
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "getCollection failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getCollection error: ${e.message}")
            null
        }
    }

    // --- Recommendations ---

    /**
     * Fetch media items similar to the given media ID.
     */
    suspend fun getSimilarMedia(mediaId: Long): MediaSearchResponse? {
        return try {
            val response = api.getSimilarMedia(mediaId)
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "getSimilarMedia failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getSimilarMedia error: ${e.message}")
            null
        }
    }

    /**
     * Fetch currently trending media.
     */
    suspend fun getTrendingMedia(): MediaSearchResponse? {
        return try {
            val response = api.getTrendingMedia()
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "getTrendingMedia failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getTrendingMedia error: ${e.message}")
            null
        }
    }

    suspend fun getSimilarItems(mediaId: Long): List<MediaItem> {
        return try {
            val body = api.getSimilarMedia(mediaId).takeIf { it.isSuccessful }?.body() ?: return emptyList()
            body.allItems
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getSimilarItems error: ${e.message}")
            emptyList()
        }
    }

    suspend fun getTrendingItems(limit: Int = 10): List<MediaItem> {
        return try {
            val body = api.getTrendingMedia().takeIf { it.isSuccessful }?.body() ?: return emptyList()
            body.allItems.take(limit)
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getTrendingItems error: ${e.message}")
            emptyList()
        }
    }

    private fun parseRecommendationItem(item: Any?): MediaItem? {
        val map = item as? Map<*, *> ?: return null
        val id = (map["id"] as? Number)?.toLong() ?: return null
        val title = map["title"] as? String ?: return null
        return MediaItem(
            id = id,
            title = title,
            mediaType = map["media_type"] as? String,
            year = (map["year"] as? Number)?.toInt(),
            coverUrl = map["poster_url"] as? String,
            rating = (map["rating"] as? Number)?.toDouble(),
            genre = (map["genres"] as? List<*>)?.filterIsInstance<String>()
        )
    }

    // --- Playlists ---

    /**
     * Fetch all playlists for the current user.
     */
    suspend fun getPlaylists(): kotlinx.serialization.json.JsonObject? {
        return try {
            val response = api.getPlaylists()
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "getPlaylists failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "getPlaylists error: ${e.message}")
            null
        }
    }

    /**
     * Create a new playlist with the given name (and optional description).
     */
    suspend fun createPlaylist(name: String, description: String? = null): kotlinx.serialization.json.JsonObject? {
        return try {
            val body = com.catalogizer.androidtv.data.remote.CreatePlaylistRequest(name, description)
            val response = api.createPlaylist(body)
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "createPlaylist failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "createPlaylist error: ${e.message}")
            null
        }
    }

    /**
     * Add a media item to a playlist.
     */
    suspend fun addPlaylistItem(playlistId: Long, mediaId: Long, entityType: String = "movie"): kotlinx.serialization.json.JsonObject? {
        return try {
            val body = com.catalogizer.androidtv.data.remote.PlaylistItemRequest(entity_id = mediaId, entity_type = entityType)
            val response = api.addPlaylistItem(playlistId, body)
            if (response.isSuccessful) {
                response.body()
            } else {
                android.util.Log.w("MediaRepo", "addPlaylistItem failed: ${response.code()}")
                null
            }
        } catch (e: Exception) {
            android.util.Log.w("MediaRepo", "addPlaylistItem error: ${e.message}")
            null
        }
    }
}
/** Result of storing watch progress on the server (WPROG3). */
sealed class WatchProgressOutcome {
    /** The server answered persisted=true. */
    object Saved : WatchProgressOutcome()

    /** The server did not store it (200 persisted=false, or 409 concurrent_update after all attempts). */
    data class NotStored(val reason: String?) : WatchProgressOutcome()
}

class WatchProgressNotStoredException(val reason: String?) :
    Exception("Watch progress not stored: ${reason ?: "unknown"}")
