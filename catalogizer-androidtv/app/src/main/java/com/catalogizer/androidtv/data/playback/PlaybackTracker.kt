package com.catalogizer.androidtv.data.playback

import android.util.Log
import com.catalogizer.androidtv.data.remote.CatalogizerApi
import com.catalogizer.androidtv.data.remote.EndPlaybackRequest
import com.catalogizer.androidtv.data.remote.ProgressPlaybackRequest
import com.catalogizer.androidtv.data.remote.StartPlaybackRequest
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.launch
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.longOrNull

/**
 * PlaybackTracker calls the /api/v1/playback/sessions endpoints
 * on behalf of the TV player so the backend records one row per
 * reproduction session, drives the ProgressBadge on every media
 * card, and powers the per-item history drawer.
 *
 * Lifecycle:
 *
 *   val tracker = PlaybackTracker(container.api)
 *   tracker.start(mediaId, fileId, "seconds", startPosition)
 *   tracker.startProgressTicker(
 *       intervalMs = 15_000,
 *       getCurrentPosition = { vlc.currentPosition.value / 1000 },
 *       getTotalAmount     = { vlc.currentPosition.value / 1000 },
 *   )
 *   // ... later, typically in onDestroy or on EndReached ...
 *   tracker.end(endPos, totalAmount, completed = didFinish)
 *
 * All network calls are fire-and-forget: the server is the
 * source of truth, and if a progress tick fails we'll try again
 * on the next tick. End() swallows errors so the player's
 * onDestroy path is never blocked on a network failure.
 */
class PlaybackTracker(
    private val api: CatalogizerApi,
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
) {

    @Volatile private var sessionId: Long = 0
    private var progressJob: Job? = null

    /**
     * Opens a new playback_sessions row and returns its id
     * (also cached internally so subsequent progress/end calls
     * don't need it). Returns 0 on failure so the player can
     * still run — tracking is informational, not blocking.
     */
    suspend fun start(
        mediaItemId: Long,
        fileId: Long?,
        positionUnit: String = "seconds",
        startPosition: Long = 0L,
    ): Long {
        return try {
            val body = StartPlaybackRequest(
                media_item_id = mediaItemId,
                file_id = fileId,
                position_unit = positionUnit,
                start_position = startPosition,
            )
            val resp = api.startPlaybackSession(body)
            if (!resp.isSuccessful) {
                Log.w(TAG, "start failed: HTTP ${resp.code()}")
                return 0
            }
            val idElement = resp.body()?.get("session_id")
            val id = idElement?.jsonPrimitive?.longOrNull ?: 0
            sessionId = id
            Log.d(TAG, "started session $id for media $mediaItemId")
            id
        } catch (t: Throwable) {
            Log.w(TAG, "start exception: ${t.message}")
            0
        }
    }

    /**
     * Starts a periodic progress ticker at the given interval.
     * The getters are called on every tick so the caller can
     * read live state from the player without holding a stale
     * snapshot. Cancelling the returned job (via stopProgressTicker
     * or end) stops further ticks.
     */
    fun startProgressTicker(
        intervalMs: Long = 15_000L,
        getCurrentPosition: () -> Long,
        getTotalAmount: () -> Long,
    ) {
        stopProgressTicker()
        progressJob = scope.launch {
            while (true) {
                ensureActive()
                delay(intervalMs)
                val id = sessionId
                if (id == 0L) break
                try {
                    api.progressPlaybackSession(
                        ProgressPlaybackRequest(
                            session_id = id,
                            end_position = getCurrentPosition(),
                            total_amount = getTotalAmount(),
                        ),
                    )
                } catch (t: CancellationException) {
                    throw t
                } catch (t: Throwable) {
                    // Next tick will retry; do not bubble the
                    // exception up through the coroutine scope.
                    Log.v(TAG, "progress tick failed: ${t.message}")
                }
            }
        }
    }

    fun stopProgressTicker() {
        progressJob?.cancel()
        progressJob = null
    }

    /**
     * Finalises the session and triggers the backend
     * media_progress upsert. Safe to call multiple times —
     * the second invocation is a no-op because sessionId is
     * zeroed after the first successful end.
     */
    suspend fun end(
        endPosition: Long,
        totalAmount: Long,
        completed: Boolean,
    ) {
        val id = sessionId
        if (id == 0L) return
        stopProgressTicker()
        try {
            api.endPlaybackSession(
                EndPlaybackRequest(
                    session_id = id,
                    end_position = endPosition,
                    total_amount = totalAmount,
                    completed = completed,
                ),
            )
            Log.d(TAG, "ended session $id at $endPosition (completed=$completed)")
        } catch (t: Throwable) {
            Log.w(TAG, "end exception: ${t.message}")
        } finally {
            sessionId = 0
        }
    }

    /**
     * True when a session has been opened and not yet ended.
     * Used by the VLCPlayerActivity to decide whether onDestroy
     * should call end() with the last-known position.
     */
    fun isActive(): Boolean = sessionId != 0L

    companion object {
        private const val TAG = "PlaybackTracker"
    }
}
