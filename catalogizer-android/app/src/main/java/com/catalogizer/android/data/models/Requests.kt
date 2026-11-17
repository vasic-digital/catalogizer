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
    val position: Long? = null
)

@Serializable
data class RatingRequest(val rating: Double)
