package com.catalogizer.androidtv.data.playback

import android.util.Log
import com.catalogizer.androidtv.data.repository.MediaRepository
import com.catalogizer.androidtv.data.repository.WatchProgressOutcome
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.withContext

/**
 * One tick of the player's periodic watch-progress save (extracted from VLCPlayerActivity so it is testable).
 * Returns the new "last saved" fraction: it advances ONLY when the server answered persisted=true; NotStored or an
 * error leaves it unchanged so the next tick tries again (WPROG3-FU-3: never claim saved).
 */
object WatchProgressTick {
    const val MIN_PROGRESS_TO_SAVE = 0.05 // only save if > 5% watched
    const val MAX_PROGRESS_TO_SAVE = 0.95 // don't save if > 95% (completed)
    private const val MIN_DELTA = 0.01

    suspend fun run(
        lastSaved: Double,
        positionMs: Long,
        durationMs: Long,
        save: suspend (progress: Double, positionMs: Long, durationMs: Long) -> WatchProgressOutcome,
    ): Double {
        if (durationMs <= 0) return lastSaved
        val progress = positionMs.toDouble() / durationMs.toDouble()
        if (progress !in MIN_PROGRESS_TO_SAVE..MAX_PROGRESS_TO_SAVE ||
            kotlin.math.abs(progress - lastSaved) <= MIN_DELTA
        ) return lastSaved
        return try {
            when (save(progress, positionMs, durationMs)) {
                is WatchProgressOutcome.Saved -> progress
                is WatchProgressOutcome.NotStored -> lastSaved
            }
        } catch (e: CancellationException) {
            throw e // never swallow cancellation (CLPROG-R2-n1)
        } catch (e: Exception) {
            lastSaved
        }
    }

    /**
     * The tick wired to the repository: this call, not a lambda in the activity, passes position and duration to
     * [MediaRepository.saveWatchProgress] (CLPROG-R2-m1), so a swap of the two is caught by the unit test.
     */
    suspend fun runWith(
        repository: MediaRepository,
        mediaId: Long,
        lastSaved: Double,
        positionMs: Long,
        durationMs: Long,
    ): Double = run(lastSaved, positionMs, durationMs) { progress, posMs, durMs ->
        val outcome = repository.saveWatchProgress(mediaId, progress, positionMs = posMs, durationMs = durMs)
        if (outcome is WatchProgressOutcome.NotStored) {
            Log.w("WatchProgressTick", "Watch progress NOT stored by the server: ${outcome.reason}")
        }
        outcome
    }

    /** Final save at destroy: runs to completion even when the caller's scope is already cancelled (CLPROG-R2-n1). */
    suspend fun finalSave(
        repository: MediaRepository,
        mediaId: Long,
        lastSaved: Double,
        positionMs: Long,
        durationMs: Long,
    ): Double = withContext(NonCancellable) { runWith(repository, mediaId, lastSaved, positionMs, durationMs) }
}
