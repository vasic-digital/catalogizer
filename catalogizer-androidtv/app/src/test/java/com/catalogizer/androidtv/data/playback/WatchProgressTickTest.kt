package com.catalogizer.androidtv.data.playback

import com.catalogizer.androidtv.data.repository.WatchProgressOutcome
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
    fun `no call when the duration is unknown, out of range, or the change is small`() = runTest {
        val c = Calls()
        assertEquals(0.0, tick(0.0, 5_000, 0, c, WatchProgressOutcome.Saved), 0.0)
        assertEquals(0.0, tick(0.0, 1_000, 100_000, c, WatchProgressOutcome.Saved), 0.0) // 1% < 5%
        assertEquals(0.0, tick(0.0, 99_000, 100_000, c, WatchProgressOutcome.Saved), 0.0) // 99% > 95%
        assertEquals(0.5, tick(0.5, 50_500, 100_000, c, WatchProgressOutcome.Saved), 0.0) // delta 0.005
        assertEquals(0, c.n)
    }
}
