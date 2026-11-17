package com.catalogizer.android.data.playback

import android.util.Log
import com.catalogizer.android.data.remote.CatalogizerApi
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.longOrNull

/**
 * Phone mirror of catalogizer-androidtv's PlaybackRepository. Fetches
 * /api/v1/entities/:id/{progress,history} and turns the raw JsonObject
 * into [UiPlaybackProgress] / [UiPlaybackSession] that Compose screens
 * can consume without touching serialization types.
 */
class PlaybackRepository(
    private val api: CatalogizerApi,
) {
    suspend fun getProgress(mediaItemId: Long): UiPlaybackProgress? {
        return try {
            val resp = api.getEntityProgress(mediaItemId)
            if (!resp.isSuccessful) {
                Log.w(TAG, "getProgress HTTP ${resp.code()} for $mediaItemId")
                return null
            }
            val body = resp.body() ?: return null
            val progEl = body["progress"] ?: return null
            if (progEl.toString() == "null") return null
            val prog = progEl.jsonObject
            UiPlaybackProgress(
                mediaItemId = mediaItemId,
                positionUnit = prog["position_unit"]?.jsonPrimitive?.content ?: "seconds",
                durationTotal = prog["duration_total"]?.jsonPrimitive?.longOrNull,
                lastPosition = prog["last_position"]?.jsonPrimitive?.longOrNull ?: 0L,
                lastSessionAmount = prog["last_session_amount"]?.jsonPrimitive?.longOrNull ?: 0L,
                totalReproductions = prog["total_reproductions"]?.jsonPrimitive?.longOrNull ?: 0L,
                aggregateAmount = prog["aggregate_amount"]?.jsonPrimitive?.longOrNull ?: 0L,
                lastSessionEndedAtMs = parseIso8601(prog["last_session_ended_at"]?.jsonPrimitive?.content),
            )
        } catch (t: Throwable) {
            Log.w(TAG, "getProgress exception: ${t.message}")
            null
        }
    }

    suspend fun listHistory(mediaItemId: Long, limit: Int = 50): List<UiPlaybackSession> {
        return try {
            val resp = api.getEntityHistory(mediaItemId, limit)
            if (!resp.isSuccessful) return emptyList()
            val body = resp.body() ?: return emptyList()
            val sessionsEl = body["sessions"] ?: return emptyList()
            sessionsEl.jsonArray.map { it.jsonObject.toSession() }
        } catch (t: Throwable) {
            Log.w(TAG, "listHistory exception: ${t.message}")
            emptyList()
        }
    }

    /**
     * Strict variants used by screens that must tell "nothing recorded" from "the request failed"
     * (AFAV-7b). [getProgress]/[listHistory] stay lenient on purpose for the Home cards, which
     * simply omit a progress bar when the lookup fails.
     */
    suspend fun loadProgress(mediaItemId: Long): PlaybackLoadResult<UiPlaybackProgress?> {
        return try {
            val resp = api.getEntityProgress(mediaItemId)
            if (!resp.isSuccessful) {
                return PlaybackLoadResult.Failure(resp.code(), "HTTP ${resp.code()}")
            }
            val body = resp.body() ?: return PlaybackLoadResult.Success(null)
            val progEl = body["progress"] ?: return PlaybackLoadResult.Success(null)
            if (progEl.toString() == "null") return PlaybackLoadResult.Success(null)
            PlaybackLoadResult.Success(progEl.jsonObject.toProgress(mediaItemId))
        } catch (t: Throwable) {
            Log.w(TAG, "loadProgress exception: ${t.message}")
            PlaybackLoadResult.Failure(null, t.message ?: t.javaClass.simpleName)
        }
    }

    suspend fun loadHistory(mediaItemId: Long, limit: Int = 50): PlaybackLoadResult<List<UiPlaybackSession>> {
        return try {
            val resp = api.getEntityHistory(mediaItemId, limit)
            if (!resp.isSuccessful) {
                return PlaybackLoadResult.Failure(resp.code(), "HTTP ${resp.code()}")
            }
            val body = resp.body() ?: return PlaybackLoadResult.Success(emptyList())
            val sessionsEl = body["sessions"] ?: return PlaybackLoadResult.Success(emptyList())
            PlaybackLoadResult.Success(sessionsEl.jsonArray.map { it.jsonObject.toSession() })
        } catch (t: Throwable) {
            Log.w(TAG, "loadHistory exception: ${t.message}")
            PlaybackLoadResult.Failure(null, t.message ?: t.javaClass.simpleName)
        }
    }

    private fun JsonObject.toProgress(mediaItemId: Long): UiPlaybackProgress = UiPlaybackProgress(
        mediaItemId = mediaItemId,
        positionUnit = this["position_unit"]?.jsonPrimitive?.content ?: "seconds",
        durationTotal = this["duration_total"]?.jsonPrimitive?.longOrNull,
        lastPosition = this["last_position"]?.jsonPrimitive?.longOrNull ?: 0L,
        lastSessionAmount = this["last_session_amount"]?.jsonPrimitive?.longOrNull ?: 0L,
        totalReproductions = this["total_reproductions"]?.jsonPrimitive?.longOrNull ?: 0L,
        aggregateAmount = this["aggregate_amount"]?.jsonPrimitive?.longOrNull ?: 0L,
        lastSessionEndedAtMs = parseIso8601(this["last_session_ended_at"]?.jsonPrimitive?.content),
    )

    private fun JsonObject.toSession(): UiPlaybackSession = UiPlaybackSession(
        id = this["id"]?.jsonPrimitive?.longOrNull ?: 0L,
        positionUnit = this["position_unit"]?.jsonPrimitive?.content ?: "seconds",
        startPosition = this["start_position"]?.jsonPrimitive?.longOrNull ?: 0L,
        endPosition = this["end_position"]?.jsonPrimitive?.longOrNull ?: 0L,
        totalAmount = this["total_amount"]?.jsonPrimitive?.longOrNull ?: 0L,
        startedAtMs = parseIso8601(this["started_at"]?.jsonPrimitive?.content) ?: 0L,
        endedAtMs = parseIso8601(this["ended_at"]?.jsonPrimitive?.content),
        completed = this["completed"]?.jsonPrimitive?.booleanOrNull ?: false,
    )

    private fun parseIso8601(raw: String?): Long? {
        if (raw.isNullOrBlank() || raw == "null") return null
        return try {
            java.time.OffsetDateTime.parse(raw).toInstant().toEpochMilli()
        } catch (_: Throwable) {
            null
        }
    }

    companion object {
        private const val TAG = "PlaybackRepository"
    }
}

/** Outcome of a strict playback load: data (possibly genuinely empty) or a failure that is not empty data. */
sealed class PlaybackLoadResult<out T> {
    data class Success<T>(val value: T) : PlaybackLoadResult<T>()
    data class Failure(val httpCode: Int?, val message: String) : PlaybackLoadResult<Nothing>()
}
