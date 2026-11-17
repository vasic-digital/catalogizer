package com.catalogizer.androidtv.data.playback

import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * CLFU-R-3: at destroy the position/duration are read and the player is released BEFORE any network call
 * (tracker end, final save: up to 3 attempts x 30 s each), so the native player is not held for minutes.
 */
class PlayerShutdownTest {
    private val events = mutableListOf<String>()

    private suspend fun runIt(
        snapshot: PlayerShutdown.Snapshot = PlayerShutdown.Snapshot(50_000, 100_000),
        end: suspend (PlayerShutdown.Snapshot) -> Unit = { events += "end(${it.positionMs},${it.durationMs})"; delay(90_000) },
        save: suspend (PlayerShutdown.Snapshot) -> Unit = { events += "save(${it.positionMs},${it.durationMs})"; delay(90_000) },
    ) = PlayerShutdown.run(
        read = { events += "read"; snapshot },
        release = { events += "release" },
        endSession = end,
        save = save,
    )

    @Test
    fun `release comes after the read and before every network call`() = runTest {
        runIt()
        assertEquals(listOf("read", "release", "end(50000,100000)", "save(50000,100000)"), events)
    }

    @Test
    fun `the player is already released while the slow network call is still running`() = runTest {
        var releasedWhileEndRunning = false
        val job = launch {
            runIt(end = {
                releasedWhileEndRunning = events.contains("release")
                delay(90_000)
            })
        }
        advanceUntilIdle()
        assertTrue(releasedWhileEndRunning)
        job.cancel()
    }

    @Test
    fun `the snapshot read before release is what the network calls get`() = runTest {
        runIt(snapshot = PlayerShutdown.Snapshot(7_000, 9_000))
        assertTrue(events.contains("end(7000,9000)"))
        assertTrue(events.contains("save(7000,9000)"))
    }

    @Test
    fun `a failing tracker end does not skip the final save`() = runTest {
        runIt(end = { throw RuntimeException("net") })
        assertTrue(events.any { it.startsWith("save(") })
    }

    @Test
    fun `a failing save does not stop the shutdown and release already happened`() = runTest {
        runIt(save = { throw RuntimeException("net") })
        assertEquals(listOf("read", "release"), events.take(2))
    }

    @Test
    fun `cancellation after the start does not abort the final calls (NonCancellable)`() = runTest {
        val job = launch { runIt(end = { events += "end"; delay(10) }, save = { events += "save" }) }
        advanceUntilIdle()
        job.cancel()
        assertEquals(listOf("read", "release", "end", "save"), events)
    }

    @Test
    fun `a throwing release does not skip the network calls`() = runTest {
        PlayerShutdown.run(
            read = { PlayerShutdown.Snapshot(1, 2) },
            release = { throw IllegalStateException("native") },
            endSession = { events += "end" },
            save = { events += "save" },
        )
        assertEquals(listOf("end", "save"), events)
    }
}
