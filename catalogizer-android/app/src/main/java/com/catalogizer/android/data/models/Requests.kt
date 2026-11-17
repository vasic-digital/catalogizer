package com.catalogizer.android.data.models

import kotlinx.serialization.Serializable

/**
 * Typed request bodies for the user-interaction endpoints. They replace `Map<String, Any>`, which Retrofit
 * refuses at runtime ("Parameter type must not include a type variable or wildcard", ANDROID-RETROFIT-MAP-1)
 * and which kotlinx.serialization cannot encode (`Any` has no serializer).
 *
 * Server contract (catalog-api/handlers/media_handler.go): PUT /media/:id/favorite {"favorite": bool},
 * PUT /media/:id/progress {"progress": 0.0..1.0}. Null optional fields are omitted from the JSON
 * (kotlinx default encodeDefaults = false).
 */
@Serializable
data class FavoriteRequest(val favorite: Boolean)

@Serializable
data class WatchProgressRequest(
    val progress: Double,
    val timestamp: String? = null,
    /** Playback position in SECONDS (the server route takes seconds; do not pass milliseconds). */
    val position: Long? = null,
    /** WPROG3-FU: total duration in SECONDS; the server stores a first watch only when it is known. */
    val duration: Double? = null
)

@Serializable
/**
 * Server contract (CAT-RATING-1): PUT /api/v1/media/:id/rating {"rating": N}, N an integer 0..10; a non-integral
 * or out-of-range value is a 400. Kept as Int so 4.5 cannot be built (RATING1-R-F3).
 */
data class RatingRequest(val rating: Int)
