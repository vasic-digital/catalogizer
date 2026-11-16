package com.catalogizer.android.data.repository

import com.catalogizer.android.data.repository.WebSocketRepository.WebSocketEvent
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
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

    @Test fun `WSF2B2RR-m1 disconnect then a mint IOException publishes no Error and mints and builds nothing more`() = runTest {
        lateinit var r: WebSocketRepository
        val m = object : WsTicketMinter {
            var calls = 0
            override suspend fun mint(token: String): String {
                calls++
                r.disconnect(); throw java.io.IOException("raced disconnect")
            }
        }
        r = repo(this, m)
        r.connect(); runCurrent()
        assertEquals("disconnect() race: event must stay Disconnected", WebSocketEvent.Disconnected, r.events.value)
        advanceTimeBy(60_000); runCurrent()
        assertEquals("no reconnect mint after disconnect", 1, m.calls)
        assertEquals("no socket after disconnect", 0, RecSocket.made.size)
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
    }

    @Test fun `WSF2B2RR-m2 a 401 raised by a cancelled attempt publishes no Error`() = runTest {
        lateinit var r: WebSocketRepository
        val m = object : WsTicketMinter {
            var calls = 0
            override suspend fun mint(token: String): String {
                calls++
                if (calls == 1) { r.connect(); throw WsTicketUnauthorizedException() }
                return "t$calls"
            }
        }
        r = repo(this, m)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals("stale 401 from a superseded attempt must not publish Error", WebSocketEvent.Disconnected, r.events.value)
        assertEquals("the newer live attempt still connects", 1, RecSocket.connects)
        r.disconnect()
    }

    @Test fun `WSF2B2RR-m2 a 401 after disconnect publishes no Error`() = runTest {
        lateinit var r: WebSocketRepository
        val m = object : WsTicketMinter {
            override suspend fun mint(token: String): String { r.disconnect(); throw WsTicketUnauthorizedException() }
        }
        r = repo(this, m)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
    }

    @Test fun `WSF2B2RR-m2 a clientFactory failure in a cancelled attempt publishes no Error`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        var first = true
        val base = r.clientFactory
        r.clientFactory = { u, l ->
            if (first) { first = false; r.connect(); throw IllegalStateException("factory raced the cancel") }
            base(u, l)
        }
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals("stale factory failure must not publish Error", WebSocketEvent.Disconnected, r.events.value)
        assertEquals("only the live attempt built a socket", 1, RecSocket.made.size)
        assertEquals(1, RecSocket.connects)
        r.disconnect()
    }

    @Test fun `WSF2B2RR-m2 a clientFactory failure in a live attempt still publishes Error`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.clientFactory = { _, _ -> throw IllegalStateException("boom") }
        r.connect(); runCurrent()
        assertEquals(WebSocketEvent.Error("boom"), r.events.value)
        r.disconnect()
    }

    /** Records every value the repository publishes, in order (StateFlow conflation avoided by an unconfined collector). */
    private fun TestScope.record(r: WebSocketRepository): MutableList<WebSocketEvent> {
        val seen = mutableListOf<WebSocketEvent>()
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { r.events.collect { seen.add(it) } }
        return seen
    }

    private fun errors(seen: List<WebSocketEvent>) = seen.filterIsInstance<WebSocketEvent.Error>().map { it.message }

    // WSF2B2RRR-f1: ordered sequences. A stale failure must never appear; the superseding attempt still publishes its own.
    @Test fun `WSF2B2RRR-f1 stale 401 then live 401 records only the live Error in order`() = runTest {
        lateinit var r: WebSocketRepository
        val m = object : WsTicketMinter {
            var calls = 0
            override suspend fun mint(token: String): String {
                calls++
                if (calls == 1) { r.connect(); throw WsTicketUnauthorizedException("stale-401") }
                throw WsTicketUnauthorizedException("live-401")
            }
        }
        r = repo(this, m)
        val seen = record(r)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(listOf<WebSocketEvent>(WebSocketEvent.Disconnected, WebSocketEvent.Error("live-401")), seen.toList())
        assertEquals("a 401 is final: no reconnect mint", 2, m.calls)
        r.disconnect()
    }

    @Test fun `WSF2B2RRR-f1 stale factory failure then live factory failure records only the live Error`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        var n = 0
        r.clientFactory = { _, _ ->
            n++
            if (n == 1) { r.connect(); throw IllegalStateException("stale-factory") }
            throw IllegalStateException("live-factory")
        }
        val seen = record(r)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(listOf("live-factory"), errors(seen))
        assertEquals(WebSocketEvent.Error("live-factory"), seen.last())
        r.disconnect()
    }

    // WSF2B2RRR-n1: second discriminating tests per guard.
    @Test fun `WSF2B2RRR-n1 a live 401 publishes Error once and mints no more`() = runTest {
        val m = object : WsTicketMinter {
            var calls = 0
            override suspend fun mint(token: String): String { calls++; throw WsTicketUnauthorizedException("live-401") }
        }
        val r = repo(this, m)
        val seen = record(r)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(listOf("live-401"), errors(seen))
        assertEquals(1, m.calls)
        r.disconnect()
    }

    @Test fun `WSF2B2RRR-n1 a clientFactory failure after disconnect records no Error`() = runTest {
        val m = GateMinter(); val r = repo(this, m)
        r.clientFactory = { _, _ -> r.disconnect(); throw IllegalStateException("factory raced disconnect") }
        val seen = record(r)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(emptyList<String>(), errors(seen))
        assertEquals(WebSocketEvent.Disconnected, r.events.value)
        assertEquals(1, m.calls)
    }

    @Test fun `WSF2B2RRR-n1 a plain mint failure after disconnect records no Error even though the scope stays active`() = runTest {
        val m = object : WsTicketMinter {
            override suspend fun mint(token: String): String { r0.disconnect(); throw java.io.IOException("x") }
            lateinit var r0: WebSocketRepository
        }
        val r = repo(this, m); m.r0 = r
        val seen = record(r)
        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(emptyList<String>(), errors(seen))
    }
}
