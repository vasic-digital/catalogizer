package com.catalogizer.androidtv.data.playback

import com.catalogizer.androidtv.data.repository.WatchProgressOutcome
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test

/** WPROG3-FU F2: the VLC activity's save tick; lastSaved may only advance on Saved. */
class WatchProgressTickTest {
    private class Calls {
        var n = 0
        var lastPos: Long? = null
        var lastDur: Long? = null
    }

    private suspend fun tick(
        last: Double, pos: Long, dur: Long, calls: Calls, outcome: WatchProgressOutcome,
    ): Double = WatchProgressTick.run(last, pos, dur) { _, p, d ->
        calls.n++; calls.lastPos = p; calls.lastDur = d
        outcome
    }

    @Test
    fun `Saved advances lastSaved`() = runTest {
        val c = Calls()
        assertEquals(0.5, tick(0.0, 50_000, 100_000, c, WatchProgressOutcome.Saved), 0.0)
        assertEquals(1, c.n)
        assertEquals(100_000L, c.lastDur)
    }

    @Test
    fun `NotStored keeps lastSaved unchanged so the next tick retries`() = runTest {
        val c = Calls()
        val out = tick(0.2, 50_000, 100_000, c, WatchProgressOutcome.NotStored("unknown_duration_no_row"))
        assertEquals(0.2, out, 0.0)
        assertEquals(1, c.n)
    }

    @Test
    fun `thrown error keeps lastSaved unchanged`() = runTest {
        val out = WatchProgressTick.run(0.2, 50_000, 100_000) { _, _, _ -> throw RuntimeException("net") }
        assertEquals(0.2, out, 0.0)
    }

    @Test
    fun `cancellation is not swallowed by the tick (K8)`() = runTest {
        var thrown: Throwable? = null
        try {
            WatchProgressTick.run(0.0, 50_000, 100_000) { _, _, _ -> throw kotlinx.coroutines.CancellationException("c") }
        } catch (e: kotlinx.coroutines.CancellationException) {
            thrown = e
        }
        assertEquals("c", thrown?.message)
    }

    @Test
    fun `no call when the duration is unknown, out of range, or the change is small`() = runTest {
        val c = Calls()
        assertEquals(0.0, tick(0.0, 5_000, 0, c, WatchProgressOutcome.Saved), 0.0)
        assertEquals(0.0, tick(0.0, 1_000, 100_000, c, WatchProgressOutcome.Saved), 0.0) // 1% < 5%
        assertEquals(0.0, tick(0.0, 99_000, 100_000, c, WatchProgressOutcome.Saved), 0.0) // 99% > 95%
        assertEquals(0.5, tick(0.5, 50_500, 100_000, c, WatchProgressOutcome.Saved), 0.0) // delta 0.005
        assertEquals(0, c.n)
    }
}

/**
 * CLPROG-R2-m1 / n1: the tick itself calls MediaRepository.saveWatchProgress (no lambda in the activity that
 * could swap position and duration), and the final save at destroy survives a cancelled scope.
 */
@OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
class WatchProgressTickRepositoryTest {
    @get:org.junit.Rule
    val mainDispatcherRule = com.catalogizer.androidtv.MainDispatcherRule()

    private val api = io.mockk.mockk<com.catalogizer.androidtv.data.remote.CatalogizerApi>()
    private val repository = com.catalogizer.androidtv.data.repository.MediaRepository(
        io.mockk.mockk<android.content.Context>(relaxed = true), api,
    )

    private fun ok(persisted: Boolean): retrofit2.Response<kotlinx.serialization.json.JsonObject> =
        retrofit2.Response.success(kotlinx.serialization.json.buildJsonObject {
            put("persisted", kotlinx.serialization.json.JsonPrimitive(persisted))
        })

    @Test
    fun `runWith sends the real duration and position in seconds to the server (K5)`() = runTest {
        val sent = io.mockk.slot<Map<String, Double>>()
        io.mockk.coEvery { api.updateWatchProgress(7L, capture(sent)) } returns ok(true)

        val out = WatchProgressTick.runWith(repository, 7L, 0.0, 50_000L, 200_000L)

        assertEquals(0.25, out, 0.0)
        assertEquals(200.0, sent.captured["duration"]!!, 0.0)
        assertEquals(50.0, sent.captured["position"]!!, 0.0)
        assertEquals(0.25, sent.captured["progress"]!!, 0.0)
    }

    @Test
    fun `runWith keeps lastSaved when the server answers persisted false`() = runTest {
        io.mockk.coEvery { api.updateWatchProgress(any(), any()) } returns ok(false)
        assertEquals(0.1, WatchProgressTick.runWith(repository, 7L, 0.1, 50_000L, 100_000L), 0.0)
    }

    @Test
    fun `finalSave still saves when its caller is cancelled (K7, onDestroy scope)`() = runTest {
        var called = 0
        io.mockk.coEvery { api.updateWatchProgress(any(), any()) } coAnswers {
            kotlinx.coroutines.delay(50); called++; ok(true)
        }
        var result = -1.0
        val job = launch(start = kotlinx.coroutines.CoroutineStart.UNDISPATCHED) {
            result = WatchProgressTick.finalSave(repository, 7L, 0.0, 50_000L, 100_000L)
        }
        job.cancel()
        job.join()
        advanceUntilIdle()
        assertEquals(1, called)
        assertEquals(0.5, result, 0.0)
    }
}
