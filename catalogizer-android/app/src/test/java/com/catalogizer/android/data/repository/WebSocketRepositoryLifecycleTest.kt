package com.catalogizer.android.data.repository

import com.catalogizer.android.data.repository.WebSocketRepository.WebSocketEvent
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import java.net.URI

/**
 * WSF2-B: lifecycle regressions for WebSocketRepository (replaced/disconnected client cleanup, cancellation of an
 * in-flight ticket mint, attempts counter handling). Includes the reviewer probes P1/P2.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class WebSocketRepositoryLifecycleTest {

    private class RecSocket(uri: URI, val l: WsListener) : WsClient(uri, l) {
        var closes = 0
        override fun connect() { connects++ }
        override fun close() { closes++ }
        companion object {
            var connects = 0
            val made = mutableListOf<RecSocket>()
        }
    }

    private class GateMinter(private val gateFirst: Boolean = false) : WsTicketMinter {
        var calls = 0
        val gate = CompletableDeferred<Unit>()
        override suspend fun mint(token: String): String {
            calls++
            if (gateFirst && calls == 1) gate.await()
            return "t$calls"
        }
    }

    @Before fun reset() { RecSocket.connects = 0; RecSocket.made.clear() }

    private fun repo(s: TestScope, m: WsTicketMinter, max: Int = 3) =
        WebSocketRepository("https://h", { "jwt" }, m, s.backgroundScope, max, 1000L).also {
            it.clientFactory = { u, l -> RecSocket(u, l).also(RecSocket.made::add) }
        }

    @Test fun `P1 disconnect during mint leaves state Disconnected`() = runTest {
        val m = GateMinter(true); val r = repo(this, m)
        r.connect(); runCurrent()
        r.disconnect(); runCurrent()
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
    }

    @Test fun `P2 connect during mint does not schedule an extra reconnect`() = runTest {
        val m = GateMinter(true); val r = repo(this, m)
        r.connect(); runCurrent()
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals("mints", 2, m.calls)
        assertEquals("sockets", 1, RecSocket.connects)
        r.disconnect()
    }

    @Test fun `F1 onOpen from a replaced client closes it and publishes nothing`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.connect(); runCurrent()
        val old = RecSocket.made[0]
        r.connect(); runCurrent()
        val before = r.events.value
        val closesBefore = old.closes
        old.l.onOpen("late")
        assertEquals(before, r.events.value)
        assertEquals("stale in-flight client must be closed on open", closesBefore + 1, old.closes)
        r.disconnect()
    }

    @Test fun `F1 onOpen after disconnect closes the dropped client and stays Disconnected`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.connect(); runCurrent()
        val old = RecSocket.made[0]
        r.disconnect()
        val closesAfterDisconnect = old.closes
        old.l.onOpen("late")
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
        assertEquals(closesAfterDisconnect + 1, old.closes)
    }

    @Test fun `onOpen of the current client publishes Connected and does not close it`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.connect(); runCurrent()
        val cur = RecSocket.made[0]
        cur.l.onOpen("OK")
        assertEquals(WebSocketEvent.Connected("OK"), r.events.value)
        assertEquals(0, cur.closes)
        r.disconnect()
    }

    @Test fun `replacing a client closes the old one exactly once`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.connect(); runCurrent()
        r.connect(); runCurrent()
        assertEquals(1, RecSocket.made[0].closes)
        assertEquals(0, RecSocket.made[1].closes)
        r.disconnect()
    }

    @Test fun `onOpen resets the reconnect attempts so repeated drops keep reconnecting`() = runTest {
        val m = GateMinter(); val r = repo(this, m, max = 2)
        r.connect(); runCurrent()
        repeat(5) {
            val cur = RecSocket.made.last()
            cur.l.onOpen("ok")
            cur.l.onClose(1006, true)
            advanceTimeBy(1001); runCurrent()
        }
        assertEquals(6, m.calls)
        r.disconnect()
    }

    @Test fun `connect resets exhausted reconnect attempts`() = runTest {
        val m = GateMinter(); val r = repo(this, m, max = 2)
        r.connect(); runCurrent()
        repeat(3) {
            RecSocket.made.last().l.onClose(1006, true)
            advanceTimeBy(5_000); runCurrent()
        }
        assertEquals("exhausted after max attempts", 3, m.calls)
        r.connect(); runCurrent()
        assertEquals(4, m.calls)
        RecSocket.made.last().l.onClose(1006, true)
        advanceTimeBy(1001); runCurrent()
        assertEquals("fresh budget after connect()", 5, m.calls)
        r.disconnect()
    }

    @Test fun `disconnect cancels an in-flight mint so no client is built`() = runTest {
        val m = GateMinter(true); val r = repo(this, m)
        r.connect(); runCurrent()
        r.disconnect(); runCurrent()
        m.gate.complete(Unit); runCurrent()
        assertEquals(0, RecSocket.made.size)
        assertEquals(0, RecSocket.connects)
    }

    @Test fun `disconnect clears the client so its late close is ignored`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.connect(); runCurrent()
        val old = RecSocket.made[0]
        r.disconnect()
        old.l.onMessage("""{"type":"late"}""")
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
    }

    /** Minter whose mint() runs [hook] (a disconnect/connect on the repository) and then returns a ticket without suspending. */
    private class HookMinter(val hook: () -> Unit) : WsTicketMinter {
        var calls = 0
        override suspend fun mint(token: String): String { calls++; if (calls == 1) hook(); return "t$calls" }
    }

    @Test fun `WSF2B2-F1 disconnect inside mint returns builds no socket and stays Disconnected`() = runTest {
        lateinit var r: WebSocketRepository
        val m = HookMinter { r.disconnect() }
        r = repo(this, m)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent() // WSF2B2R-m1: a cancelled attempt must not schedule a reconnect either
        assertEquals("no extra mint after the cancelled attempt", 1, m.calls)
        assertEquals("no socket may be built after disconnect", 0, RecSocket.made.size)
        assertEquals(0, RecSocket.connects)
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
    }

    @Test fun `WSF2B2-F2 a cancelled attempt does not install its client after mint returns`() = runTest {
        lateinit var r: WebSocketRepository
        val m = HookMinter { r.connect() } // replaces the in-flight attempt; the old one then gets its ticket
        r = repo(this, m)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent() // WSF2B2R-m1: the cancelled attempt must not schedule a reconnect after its mint
        assertEquals("exactly the cancelled attempt and the live attempt minted, no reconnect mint", 2, m.calls)
        assertEquals("only the live attempt builds a client", 1, RecSocket.made.size)
        assertEquals(1, RecSocket.connects)
        assertEquals("old client must never be closed because it never existed", 0, RecSocket.made[0].closes)
        r.disconnect()
    }

    @Test fun `WSF2B2-F2 a minter-raised timeout cancellation is a mint failure not a silent cancel`() = runTest {
        val m = object : WsTicketMinter {
            var calls = 0
            override suspend fun mint(token: String): String {
                calls++
                if (calls == 1) kotlinx.coroutines.withTimeout(10) { kotlinx.coroutines.delay(1000) }
                return "t$calls"
            }
        }
        val r = repo(this, m)
        r.connect(); runCurrent()
        advanceTimeBy(20); runCurrent()
        assertEquals(WebSocketEvent.Error::class, r.events.value::class)
        advanceTimeBy(1_500); runCurrent()
        assertEquals("a reconnect must follow the timed-out mint", 2, m.calls)
        assertEquals(1, RecSocket.connects)
        r.disconnect()
    }

    @Test fun `WSF2B2R-m2 an attempt cancelled between factory and install does not install its client`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        var first = true
        val base = r.clientFactory
        // Cancels THIS attempt (connect() replaces the job) after its socket was built and before the install lock.
        r.clientFactory = { u, l -> base(u, l).also { if (first) { first = false; r.connect() } } }
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals("cancelled attempt built its socket, live attempt built its own", 2, RecSocket.made.size)
        assertEquals("only the live client connects", 1, RecSocket.connects)
        assertEquals("the cancelled attempt's client was never installed, so never replaced or closed", 0, RecSocket.made[0].closes)
        r.disconnect()
    }

    @Test fun `WSF2B2R-m3 a mint IOException after the attempt was cancelled publishes no Error and schedules no reconnect`() = runTest {
        lateinit var r: WebSocketRepository
        val m = object : WsTicketMinter {
            var calls = 0
            override suspend fun mint(token: String): String {
                calls++
                if (calls == 1) { r.connect(); throw java.io.IOException("raced the cancel") }
                return "t$calls"
            }
        }
        r = repo(this, m)
        r.connect(); runCurrent()
        assertEquals("a cancelled attempt must not publish Error", WebSocketEvent.Disconnected, r.events.value)
        advanceTimeBy(60_000); runCurrent()
        assertEquals("a cancelled attempt must not schedule a reconnect mint", 2, m.calls)
        assertEquals("the newer live client is the only one", 1, RecSocket.made.size)
        assertEquals(1, RecSocket.connects)
        assertEquals(0, RecSocket.made[0].closes)
        r.disconnect()
    }
}
