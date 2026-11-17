package com.catalogizer.androidtv.data.playback

import android.util.Log
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.withContext

/**
 * Order of the player's shutdown at activity destroy (CLFU-R-3): read the position and duration, RELEASE the native
 * player, and only then do the network calls (playback-session end, final watch-progress save), which can take up to
 * 3 attempts x 30 s. The release must not wait behind them. The network calls get the snapshot taken before the
 * release, and run to completion even if the caller's scope is cancelled; a failure of one does not skip the next.
 */
object PlayerShutdown {
    data class Snapshot(val positionMs: Long, val durationMs: Long)

    suspend fun run(
        read: () -> Snapshot,
        release: () -> Unit,
        endSession: suspend (Snapshot) -> Unit,
        save: suspend (Snapshot) -> Unit,
    ) {
        val snapshot = read()
        try {
            release()
        } catch (e: Exception) {
            Log.w("PlayerShutdown", "player release failed: ${e.message}")
        }
        withContext(NonCancellable) {
            try {
                endSession(snapshot)
            } catch (e: Exception) {
                Log.w("PlayerShutdown", "playback session end failed: ${e.message}")
            }
            try {
                save(snapshot)
            } catch (e: Exception) {
                Log.w("PlayerShutdown", "final watch-progress save failed: ${e.message}")
            }
        }
    }
}
