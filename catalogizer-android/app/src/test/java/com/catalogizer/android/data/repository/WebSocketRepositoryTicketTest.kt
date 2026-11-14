package com.catalogizer.android.data.repository

import com.catalogizer.android.data.models.WsTicketResponse
import com.catalogizer.android.data.remote.CatalogizerApi
import com.catalogizer.android.data.repository.WebSocketRepository.WebSocketEvent
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import retrofit2.Response
import java.io.IOException
import java.net.URI

/**
 * SEC-WS-1 F2: the session token must never be in the /ws URL; a fresh single-use ticket is minted for every
 * connection attempt, reconnects included; a 401 on the ticket request is final.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class WebSocketRepositoryTicketTest {

    private val jwt = "eyJhbGciOiJIUzI1NiJ9.SECRET-PAYLOAD.SIGNATURE"

    /** A socket that records the URI it was asked to connect to and never touches the network. */
    private class FakeSocket(uri: URI, val l: WsListener) : WsClient(uri, l) {
        override fun connect() { connects.add(uri); lastListener = l }
        override fun close() {}
        companion object { val connects = mutableListOf<URI>(); var lastListener: WsListener? = null }
    }

    private class CountingMinter(private val failWith: Exception? = null) : WsTicketMinter {
        val tokensSeen = mutableListOf<String>()
        var n = 0
        override suspend fun mint(token: String): String {
            tokensSeen.add(token)
            failWith?.let { throw it }
            n++
            return "ticket-$n"
        }
    }

    private fun repo(scope: TestScope, minter: WsTicketMinter?, base: String = "https://host:8443") =
        WebSocketRepository(base, { jwt }, minter, scope.backgroundScope, 3, 1000L).also {
            FakeSocket.connects.clear()
            it.clientFactory = { uri, l -> FakeSocket(uri, l) }
        }

    @Test
    fun `connect mints a ticket and connects with ticket query and no JWT in the URL`() = runTest {
        val minter = CountingMinter()
        val r = repo(this, minter)

        r.connect(); runCurrent()

        assertEquals(listOf(jwt), minter.tokensSeen)
        assertEquals(1, FakeSocket.connects.size)
        val uri = FakeSocket.connects[0]
        assertEquals("wss://host:8443/ws?ticket=ticket-1", uri.toString())
        assertFalse("JWT leaked into URL", uri.toString().contains(jwt))
        assertFalse(uri.toString().contains("token="))
        r.disconnect()
    }

    @Test
    fun `reconnect after an unexpected close mints a new ticket`() = runTest {
        val minter = CountingMinter()
        val r = repo(this, minter)
        r.connect(); runCurrent()

        FakeSocket.connects.last().let { /* first ticket */ assertTrue(it.rawQuery.endsWith("ticket-1")) }
        // Simulate server-side drop of the first socket.
        r.simulateRemoteClose()
        advanceTimeBy(1001); runCurrent()

        assertEquals(2, minter.n)
        assertEquals(2, FakeSocket.connects.size)
        assertTrue(FakeSocket.connects[1].rawQuery.endsWith("ticket-2"))
        assertTrue(FakeSocket.connects.none { it.toString().contains(jwt) })
        assertTrue("tickets must differ", FakeSocket.connects[0] != FakeSocket.connects[1])
        r.disconnect()
    }

    @Test
    fun `401 from the ticket request is final - Error event no socket no reconnect`() = runTest {
        val minter = CountingMinter(WsTicketUnauthorizedException())
        val r = repo(this, minter)

        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()

        assertTrue(r.events.value is WebSocketEvent.Error)
        assertEquals(1, minter.tokensSeen.size)
        assertTrue(FakeSocket.connects.isEmpty())
    }

    @Test
    fun `transient mint failure retries with backoff and never connects without a ticket`() = runTest {
        val minter = CountingMinter(IOException("boom"))
        val r = repo(this, minter)

        r.connect(); runCurrent()
        advanceTimeBy(60_000); runCurrent()

        assertTrue(minter.tokensSeen.size in 2..4)
        assertTrue(FakeSocket.connects.isEmpty())
        r.disconnect()
    }

    @Test
    fun `no minter configured fails closed instead of falling back to a token URL`() = runTest {
        val r = repo(this, null)
        r.connect(); runCurrent()
        assertTrue(r.events.value is WebSocketEvent.Error)
        assertTrue(FakeSocket.connects.isEmpty())
    }

    @Test
    fun `disconnect stops reconnecting`() = runTest {
        val minter = CountingMinter()
        val r = repo(this, minter)
        r.connect(); runCurrent()
        r.disconnect()
        advanceTimeBy(60_000); runCurrent()
        assertEquals(1, minter.n)
    }

    // --- ApiWsTicketMinter (Retrofit layer) ---

    @Test
    fun `ApiWsTicketMinter sends the token as a Bearer header and returns the ticket`() = runTest {
        val api = mockk<CatalogizerApi>()
        coEvery { api.mintWsTicket("Bearer $jwt") } returns Response.success(WsTicketResponse("abc", null, 30))
        assertEquals("abc", ApiWsTicketMinter(api).mint(jwt))
        coVerify(exactly = 1) { api.mintWsTicket("Bearer $jwt") }
    }

    @Test(expected = WsTicketUnauthorizedException::class)
    fun `ApiWsTicketMinter maps 401 to WsTicketUnauthorizedException`() = runTest {
        val api = mockk<CatalogizerApi>()
        coEvery { api.mintWsTicket(any()) } returns Response.error(401, "".toResponseBody(null))
        ApiWsTicketMinter(api).mint(jwt)
    }

    @Test(expected = IOException::class)
    fun `ApiWsTicketMinter rejects a response without a ticket`() = runTest {
        val api = mockk<CatalogizerApi>()
        coEvery { api.mintWsTicket(any()) } returns Response.success(WsTicketResponse("", null, 30))
        ApiWsTicketMinter(api).mint(jwt)
    }

    private fun WebSocketRepository.simulateRemoteClose() {
        FakeSocket.lastListener!!.onClose(1006, true)
    }
}
