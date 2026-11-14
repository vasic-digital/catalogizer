package com.catalogizer.android.data.remote

import com.catalogizer.android.data.models.*
import retrofit2.Response
import retrofit2.http.*

/**
 * Retrofit interface defining all Catalogizer REST API endpoints including
 * authentication, media CRUD, user preferences, favorites, streaming, and system health.
 */
interface CatalogizerApi {

    // Authentication endpoints
    @POST("auth/login")
    suspend fun login(@Body loginRequest: LoginRequest): Response<LoginResponse>

    @POST("auth/register")
    suspend fun register(@Body registerRequest: RegisterRequest): Response<User>

    @POST("auth/logout")
    suspend fun logout(): Response<Unit>

    @GET("auth/profile")
    suspend fun getProfile(): Response<User>

    @PUT("auth/profile")
    suspend fun updateProfile(@Body updateProfileRequest: UpdateProfileRequest): Response<User>

    @POST("auth/change-password")
    suspend fun changePassword(@Body changePasswordRequest: ChangePasswordRequest): Response<Unit>

    /** SEC-WS-1 F2: mint a single-use WebSocket ticket; the session token goes in the Authorization header only. */
    @POST("ws/ticket")
    suspend fun mintWsTicket(@Header("Authorization") authorization: String): Response<WsTicketResponse>

    @GET("auth/status")
    suspend fun getAuthStatus(): Response<AuthStatus>

    @GET("auth/permissions")
    suspend fun getPermissions(): Response<Map<String, Any>>

    // Media endpoints
    @GET("media/search")
    suspend fun searchMedia(
        @Query("query") query: String? = null,
        @Query("media_type") mediaType: String? = null,
        @Query("year_min") yearMin: Int? = null,
        @Query("year_max") yearMax: Int? = null,
        @Query("rating_min") ratingMin: Double? = null,
        @Query("quality") quality: String? = null,
        @Query("sort_by") sortBy: String? = null,
        @Query("sort_order") sortOrder: String? = null,
        @Query("limit") limit: Int = 20,
        @Query("offset") offset: Int = 0
    ): Response<MediaSearchResponse>

    @GET("media/{id}")
    suspend fun getMediaById(@Path("id") id: Long): Response<MediaItem>

    @GET("media/{id}/metadata")
    suspend fun getExternalMetadata(@Path("id") id: Long): Response<List<ExternalMetadata>>

    @POST("media/{id}/refresh")
    suspend fun refreshMetadata(@Path("id") id: Long): Response<Map<String, String>>

    @GET("media/{id}/quality")
    suspend fun getQualityInfo(@Path("id") id: Long): Response<Map<String, Any>>

    @GET("media/stats")
    suspend fun getMediaStats(): Response<MediaStats>

    @GET("media/recent")
    suspend fun getRecentMedia(@Query("limit") limit: Int = 10): Response<List<MediaItem>>

    @GET("media/popular")
    suspend fun getPopularMedia(@Query("limit") limit: Int = 10): Response<List<MediaItem>>

    @DELETE("media/{id}")
    suspend fun deleteMedia(@Path("id") id: Long): Response<Unit>

    @PUT("media/{id}")
    suspend fun updateMedia(@Path("id") id: Long, @Body mediaItem: MediaItem): Response<MediaItem>

    // User interaction endpoints
    @PUT("media/{id}/progress")
    suspend fun updateWatchProgress(@Path("id") id: Long, @Body progressData: Map<String, Any>): Response<Unit>

    @PUT("media/{id}/favorite")
    suspend fun setFavoriteStatus(@Path("id") id: Long, @Body favoriteData: Map<String, Any>): Response<Unit>

    @PUT("media/{id}/rating")
    suspend fun rateMedia(@Path("id") id: Long, @Body ratingData: Map<String, Any>): Response<Unit>

    @GET("user/preferences")
    suspend fun getUserPreferences(): Response<Map<String, Any>>

    @GET("media/updated")
    suspend fun getUpdatedMedia(@Query("since") since: Long): Response<List<MediaItem>>

    // SMB endpoints
    @GET("smb/sources/status")
    suspend fun getSMBStatus(): Response<Map<String, Any>>

    @POST("smb/sources/{id}/reconnect")
    suspend fun reconnectSMBSource(@Path("id") id: String): Response<Unit>

    @GET("smb/health")
    suspend fun getSMBHealth(): Response<Map<String, Any>>

    // Analytics endpoints
    @GET("analytics/dashboard")
    suspend fun getDashboardData(): Response<Map<String, Any>>

    @GET("analytics/charts")
    suspend fun getChartsData(): Response<Map<String, Any>>

    // System endpoints
    @GET("health")
    suspend fun getSystemHealth(): Response<Map<String, Any>>

    @GET("status")
    suspend fun getSystemStatus(): Response<Map<String, Any>>

    // File streaming
    @GET("stream/{mediaId}")
    suspend fun getStreamUrl(@Path("mediaId") mediaId: Long): Response<Map<String, String>>

    @GET("download/{mediaId}")
    suspend fun getDownloadUrl(@Path("mediaId") mediaId: Long): Response<Map<String, String>>

    // User preferences
    @GET("user/favorites")
    suspend fun getFavorites(
        @Query("limit") limit: Int = 20,
        @Query("offset") offset: Int = 0
    ): Response<MediaSearchResponse>

    @POST("user/favorites/{mediaId}")
    suspend fun addToFavorites(@Path("mediaId") mediaId: Long): Response<Unit>

    @DELETE("user/favorites/{mediaId}")
    suspend fun removeFromFavorites(@Path("mediaId") mediaId: Long): Response<Unit>

    @GET("user/watchlist")
    suspend fun getWatchlist(
        @Query("limit") limit: Int = 20,
        @Query("offset") offset: Int = 0
    ): Response<MediaSearchResponse>

    @POST("user/watchlist/{mediaId}")
    suspend fun addToWatchlist(@Path("mediaId") mediaId: Long): Response<Unit>

    @DELETE("user/watchlist/{mediaId}")
    suspend fun removeFromWatchlist(@Path("mediaId") mediaId: Long): Response<Unit>

    @PUT("user/progress/{mediaId}")
    suspend fun updateUserWatchProgress(
        @Path("mediaId") mediaId: Long,
        @Body progressData: Map<String, Any>
    ): Response<Unit>

    @GET("user/continue-watching")
    suspend fun getContinueWatching(): Response<List<MediaItem>>

    // Playback history / progress endpoints (mirrored from catalog-api's
    // /api/v1/entities/:id/{progress,history}). Returns raw JsonObject so
    // callers can tolerate schema drift without recompiling DTOs.
    @GET("api/v1/entities/{id}/progress")
    suspend fun getEntityProgress(
        @Path("id") id: Long
    ): Response<kotlinx.serialization.json.JsonObject>

    @GET("api/v1/entities/{id}/history")
    suspend fun getEntityHistory(
        @Path("id") id: Long,
        @Query("limit") limit: Int = 50
    ): Response<kotlinx.serialization.json.JsonObject>
}

/**
 * Generic API response wrapper providing consistent error handling.
 * Encapsulates either a successful [data] payload or an [error] message.
 */
data class ApiResult<T>(
    val data: T? = null,
    val error: String? = null,
    val isSuccess: Boolean = data != null && error == null
) {
    companion object {
        fun <T> success(data: T): ApiResult<T> = ApiResult(data = data)
        fun <T> error(error: String): ApiResult<T> = ApiResult(error = error)
    }
}

// Extension functions for easy API result handling
suspend fun <T> Response<T>.toApiResult(): ApiResult<T> {
    return try {
        if (isSuccessful) {
            body()?.let { ApiResult.success(it) } ?: ApiResult.error("Empty response")
        } else {
            val errorMsg = errorBody()?.string() ?: "Unknown error (${code()})"
            ApiResult.error(errorMsg)
        }
    } catch (e: Exception) {
        ApiResult.error(e.message ?: "Network error")
    }
}

/**
 * Sealed class hierarchy representing WebSocket events for real-time updates
 * including media changes, system status, analysis completion, and notifications.
 */
sealed class WebSocketEvent {
    data class MediaUpdate(
        val action: String,
        val mediaId: Long,
        val media: MediaItem?
    ) : WebSocketEvent()

    data class SystemUpdate(
        val action: String,
        val component: String,
        val status: String,
        val message: String?
    ) : WebSocketEvent()

    data class AnalysisComplete(
        val analysisId: String,
        val itemsProcessed: Int,
        val newItems: Int,
        val updatedItems: Int
    ) : WebSocketEvent()

    data class Notification(
        val type: String,
        val title: String,
        val message: String,
        val level: String
    ) : WebSocketEvent()

    object Connected : WebSocketEvent()
    object Disconnected : WebSocketEvent()
    data class Error(val message: String) : WebSocketEvent()
}