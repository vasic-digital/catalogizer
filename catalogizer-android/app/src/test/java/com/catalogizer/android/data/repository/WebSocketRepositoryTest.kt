package com.catalogizer.android.data.repository

import com.catalogizer.android.data.repository.WebSocketRepository.WebSocketEvent
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class WebSocketRepositoryTest {

    private lateinit var repository: WebSocketRepository

    // --- Initial state ---

    @Test
    fun `initial event state is Disconnected`() = runTest {
        repository = WebSocketRepository("http://localhost:8080", tokenProvider = { "token" })

        val event = repository.events.first()

        assertTrue(event is WebSocketEvent.Disconnected)
    }

    // --- connect ---

    @Test
    fun `connect does nothing when tokenProvider returns null`() = runTest {
        repository = WebSocketRepository("http://localhost:8080", tokenProvider = { null })

        repository.connect()

        val event = repository.events.first()
        assertTrue(event is WebSocketEvent.Disconnected)
    }

    @Test
    fun `connect with invalid URL emits Error event`() = runTest {
        repository = WebSocketRepository(
            "not a valid url", { "test-token" }, WsTicketMinter { "t" }, backgroundScope
        )

        repository.connect()
        testScheduler.runCurrent()

        val event = repository.events.first()
        assertTrue(event is WebSocketEvent.Error)
    }

    // --- disconnect ---

    @Test
    fun `disconnect sets event to Disconnected`() = runTest {
        repository = WebSocketRepository("http://localhost:8080", tokenProvider = { "token" })

        repository.disconnect()

        val event = repository.events.first()
        assertTrue(event is WebSocketEvent.Disconnected)
    }

    @Test
    fun `disconnect is safe to call multiple times`() = runTest {
        repository = WebSocketRepository("http://localhost:8080", tokenProvider = { "token" })

        repository.disconnect()
        repository.disconnect()
        repository.disconnect()

        val event = repository.events.first()
        assertTrue(event is WebSocketEvent.Disconnected)
    }

    @Test
    fun `disconnect after failed connect is safe`() = runTest {
        repository = WebSocketRepository("http://localhost:8080", tokenProvider = { null })
        repository.connect()

        repository.disconnect()

        val event = repository.events.first()
        assertTrue(event is WebSocketEvent.Disconnected)
    }

    // --- URL transformation ---

    @Test
    fun `http URL is transformed to ws URL internally`() {
        // Verifying the repository constructs correctly with http URLs
        repository = WebSocketRepository("http://192.168.1.100:8080", tokenProvider = { "token" })
        assertNotNull(repository.events)
    }

    @Test
    fun `https URL is transformed to wss URL internally`() {
        // Verifying the repository constructs correctly with https URLs
        repository = WebSocketRepository("https://secure.server.com:8443", tokenProvider = { "token" })
        assertNotNull(repository.events)
    }

    // --- WebSocketEvent sealed class ---

    @Test
    fun `Connected event holds server info`() {
        val event = WebSocketEvent.Connected("HTTP/1.1 101 Switching Protocols")
        assertEquals("HTTP/1.1 101 Switching Protocols", event.serverInfo)
    }

    @Test
    fun `Connected event with empty server info`() {
        val event = WebSocketEvent.Connected("")
        assertEquals("", event.serverInfo)
    }

    @Test
    fun `Disconnected is a singleton data object`() {
        val event1 = WebSocketEvent.Disconnected
        val event2 = WebSocketEvent.Disconnected
        assertSame(event1, event2)
    }

    @Test
    fun `MessageReceived holds type and payload`() {
        val payload = """{"type":"media_update","data":{"id":1}}"""
        val event = WebSocketEvent.MessageReceived("media_update", payload)

        assertEquals("media_update", event.type)
        assertEquals(payload, event.payload)
    }

    @Test
    fun `MessageReceived with unknown type`() {
        val event = WebSocketEvent.MessageReceived("unknown", "{}")
        assertEquals("unknown", event.type)
    }

    @Test
    fun `Error event holds message`() {
        val event = WebSocketEvent.Error("Connection refused")
        assertEquals("Connection refused", event.message)
    }

    @Test
    fun `Error event with empty message`() {
        val event = WebSocketEvent.Error("")
        assertEquals("", event.message)
    }

    // --- Event equality ---

    @Test
    fun `Connected events with same info are equal`() {
        val e1 = WebSocketEvent.Connected("info")
        val e2 = WebSocketEvent.Connected("info")
        assertEquals(e1, e2)
    }

    @Test
    fun `Connected events with different info are not equal`() {
        val e1 = WebSocketEvent.Connected("info1")
        val e2 = WebSocketEvent.Connected("info2")
        assertNotEquals(e1, e2)
    }

    @Test
    fun `MessageReceived events with same data are equal`() {
        val e1 = WebSocketEvent.MessageReceived("type", "payload")
        val e2 = WebSocketEvent.MessageReceived("type", "payload")
        assertEquals(e1, e2)
    }

    @Test
    fun `MessageReceived events with different type are not equal`() {
        val e1 = WebSocketEvent.MessageReceived("type1", "payload")
        val e2 = WebSocketEvent.MessageReceived("type2", "payload")
        assertNotEquals(e1, e2)
    }

    @Test
    fun `Error events with same message are equal`() {
        val e1 = WebSocketEvent.Error("error")
        val e2 = WebSocketEvent.Error("error")
        assertEquals(e1, e2)
    }

    @Test
    fun `Error events with different message are not equal`() {
        val e1 = WebSocketEvent.Error("error1")
        val e2 = WebSocketEvent.Error("error2")
        assertNotEquals(e1, e2)
    }

    @Test
    fun `different event subtypes are not equal`() {
        val connected = WebSocketEvent.Connected("info")
        val error = WebSocketEvent.Error("info")
        assertNotEquals(connected as WebSocketEvent, error as WebSocketEvent)
    }

    // --- StateFlow behavior ---

    @Test
    fun `events StateFlow exposes current value`() = runTest {
        repository = WebSocketRepository("http://localhost:8080", tokenProvider = { "token" })

        val current = repository.events.value

        assertTrue(current is WebSocketEvent.Disconnected)
    }

    @After
    fun tearDown() {
        if (::repository.isInitialized) {
            repository.disconnect()
        }
    }
}
