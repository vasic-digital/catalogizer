package com.catalogizer.androidtv.data.remote

import com.catalogizer.androidtv.data.models.MediaItem
import com.catalogizer.androidtv.data.models.MediaSearchRequest
import com.catalogizer.androidtv.data.models.MediaSearchResponse
import retrofit2.Response
import retrofit2.http.*

/**
 * Retrofit interface defining all Catalogizer REST API endpoints for the Android TV client
 * including authentication, entity browsing, media search, favorites, collections,
 * recommendations, playlists, and streaming.
 */
interface CatalogizerApi {

    // Authentication endpoints
    @POST("api/v1/auth/login")
    suspend fun login(@Body credentials: Map<String, String>): Response<LoginResponse>

    @POST("api/v1/auth/refresh")
    suspend fun refreshToken(@Body token: Map<String, String>): Response<LoginResponse>

    @POST("api/v1/auth/logout")
    suspend fun logout(): Response<Unit>

    // Catalog endpoints(@Body token: Map<String, String>): Response<LoginResponse>

    // Catalog endpoints
    @GET("api/v1/catalog")
    suspend fun getCatalog(): Response<List<String>>

    @GET("api/v1/media/search")
    suspend fun searchMedia(@QueryMap params: Map<String, String>): Response<MediaSearchResponse>

    @GET("api/v1/entities")
    suspend fun searchEntities(@QueryMap params: Map<String, String>): Response<MediaSearchResponse>

    @GET("api/v1/entities/browse/{type}")
    suspend fun browseEntities(@Path("type") type: String, @QueryMap params: Map<String, String>): Response<MediaSearchResponse>

    @GET("api/v1/entities/{id}")
    suspend fun getEntityById(@Path("id") id: Long): Response<MediaItem>

    @GET("api/v1/entities/{id}/children")
    suspend fun getEntityChildren(@Path("id") id: Long): Response<MediaSearchResponse>

    @GET("api/v1/entities/{id}/stream")
    suspend fun getEntityStream(@Path("id") id: Long): Response<kotlinx.serialization.json.JsonObject>

    // Comic reader: lists the page entries of a .cbz archive as
    // {"total_pages": N, "pages": [{"index": 0, "name": "001.png"}, ...]}.
    // Returns HTTP 501 for .cbr archives (no RAR decoder wired yet) — the
    // ComicReaderScreen surfaces that honestly rather than blanking. Individual
    // page bytes stream from GET .../pages/{n} (0-based), loaded by Coil with the
    // same Bearer auth as /stream.
    @GET("api/v1/entities/{id}/pages")
    suspend fun getComicPages(@Path("id") id: Long): Response<kotlinx.serialization.json.JsonObject>

    // Book reader: returns the rendered-page COUNT of a PDF book as
    // {"total_pages": N}. catalog-api renders each page server-side via poppler
    // (pdftoppm) over the same multi-protocol filesystem client as /stream, so
    // SMB/FTP/NFS-hosted books work. Individual page images stream from
    // GET .../pdf-pages/{n} (0-based) as image/png, loaded by Coil with the
    // same Bearer auth as /stream. A non-PDF entity returns HTTP 400 and a
    // missing file HTTP 404 — the BookReaderScreen surfaces those honestly
    // rather than blanking (§11.4.1). Mirrors getComicPages.
    @GET("api/v1/entities/{id}/pdf-pages")
    suspend fun getPdfPages(@Path("id") id: Long): Response<kotlinx.serialization.json.JsonObject>

    // Playback session tracking — records one row in the
    // backend playback_sessions table for every reproduction
    // session and surfaces the rolled-up per-user progress
    // summary the card ProgressBadge consumes.
    @POST("api/v1/playback/sessions/start")
    suspend fun startPlaybackSession(
        @Body body: Map<String, @JvmSuppressWildcards Any>
    ): Response<kotlinx.serialization.json.JsonObject>

    @POST("api/v1/playback/sessions/progress")
    suspend fun progressPlaybackSession(
        @Body body: Map<String, @JvmSuppressWildcards Any>
    ): Response<Unit>

    @POST("api/v1/playback/sessions/end")
    suspend fun endPlaybackSession(
        @Body body: Map<String, @JvmSuppressWildcards Any>
    ): Response<Unit>

    @GET("api/v1/entities/{id}/progress")
    suspend fun getEntityProgress(
        @Path("id") id: Long
    ): Response<kotlinx.serialization.json.JsonObject>

    @GET("api/v1/entities/{id}/history")
    suspend fun getEntityHistory(
        @Path("id") id: Long,
        @Query("limit") limit: Int = 50
    ): Response<kotlinx.serialization.json.JsonObject>

    @GET("api/v1/media/{id}")
    suspend fun getMediaById(@Path("id") id: Long): Response<MediaItem>

    @GET("api/v1/catalog-info/{path}")
    suspend fun getMediaInfo(@Path("path") path: String): Response<MediaItem>

    @POST("api/v1/media/recognize")
    suspend fun recognizeMedia(@Body request: Map<String, Any>): Response<MediaItem>

    // Media management endpoints
    @PUT("api/v1/media/{id}/progress")
    suspend fun updateWatchProgress(@Path("id") id: Long, @Body progress: Map<String, Double>): Response<Unit>

    // Favorites endpoints
    @POST("api/v1/favorites")
    suspend fun addFavorite(@Body body: Map<String, @JvmSuppressWildcards Any>): Response<Map<String, String>>

    @DELETE("api/v1/favorites/{entity_type}/{entity_id}")
    suspend fun removeFavorite(
        @Path("entity_type") entityType: String,
        @Path("entity_id") entityId: Long
    ): Response<Map<String, String>>

    @GET("api/v1/favorites/check/{entity_type}/{entity_id}")
    suspend fun checkFavorite(
        @Path("entity_type") entityType: String,
        @Path("entity_id") entityId: Long
    ): Response<Map<String, Boolean>>

    @GET("api/v1/favorites")
    suspend fun listFavorites(): Response<Map<String, @JvmSuppressWildcards Any>>

    // Stats endpoint
    @GET("api/v1/entities/stats")
    suspend fun getEntityStats(): Response<EntityStatsResponse>

    // Collections endpoints
    @GET("api/v1/collections")
    suspend fun getCollections(): Response<Map<String, Any>>

    @GET("api/v1/collections/{id}")
    suspend fun getCollection(@Path("id") id: Long): Response<Map<String, Any>>

    // Recommendations endpoints
    @GET("api/v1/recommendations/similar/{mediaId}")
    suspend fun getSimilarMedia(@Path("mediaId") mediaId: Long): Response<MediaSearchResponse>

    @GET("api/v1/recommendations/trending")
    suspend fun getTrendingMedia(): Response<MediaSearchResponse>

    // Scan status endpoint
    @GET("api/v1/scans")
    suspend fun getScans(): Response<Map<String, Any>>

    // Playlists endpoints
    @GET("api/v1/playlists")
    suspend fun getPlaylists(): Response<Map<String, Any>>

    @POST("api/v1/playlists")
    suspend fun createPlaylist(@Body body: Map<String, String>): Response<Map<String, Any>>

    @POST("api/v1/playlists/{id}/items")
    suspend fun addPlaylistItem(@Path("id") id: Long, @Body body: Map<String, @JvmSuppressWildcards Any>): Response<Map<String, Any>>

    // Overall statistics endpoint
    @GET("api/v1/stats/overall")
    suspend fun getOverallStats(): Response<Map<String, Any>>

    // Subtitles endpoints
    @GET("api/v1/subtitles/media/{mediaId}")
    suspend fun getSubtitles(@Path("mediaId") mediaId: Long): Response<Map<String, Any>>

    @GET("api/v1/subtitles/languages")
    suspend fun getSubtitleLanguages(): Response<Map<String, Any>>
}

/**
 * API response model for entity statistics containing total count and per-type breakdown.
 */
@kotlinx.serialization.Serializable
data class EntityStatsResponse(
    @kotlinx.serialization.SerialName("total_entities")
    val totalEntities: Int = 0,
    @kotlinx.serialization.SerialName("by_type")
    val byType: Map<String, Int> = emptyMap()
)

/**
 * Authenticated user profile returned in the login response.
 */
@kotlinx.serialization.Serializable
data class LoginUser(
    val id: Long,
    val username: String,
    val email: String? = null,
    val display_name: String? = null
)

/**
 * API response model for successful authentication containing user profile,
 * session token, optional refresh token, and expiration timestamp.
 */
@kotlinx.serialization.Serializable
data class LoginResponse(
    val user: LoginUser,
    val session_token: String,
    val refresh_token: String? = null,
    val expires_at: String? = null
) {
    // Convenience properties for backward compatibility
    val token: String get() = session_token
    val userId: Long get() = user.id
    val username: String get() = user.username
    val expiresAt: String? get() = expires_at
}