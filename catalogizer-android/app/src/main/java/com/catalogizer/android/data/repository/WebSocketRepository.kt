package com.catalogizer.android.data.repository

import com.catalogizer.android.data.remote.CatalogizerApi
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import org.java_websocket.client.WebSocketClient
import org.java_websocket.handshake.ServerHandshake
import java.io.IOException
import java.net.URI
import java.net.URLEncoder

/** Mints a single-use WebSocket ticket (SEC-WS-1 F2). The session token is sent in a header, never in the /ws URL. */
fun interface WsTicketMinter {
    @Throws(Exception::class)
    suspend fun mint(token: String): String
}

/** The server refused the session token (HTTP 401): retrying with the same token cannot succeed. */
class WsTicketUnauthorizedException(message: String = "WebSocket ticket request unauthorized (401)") : Exception(message)

/** [WsTicketMinter] backed by `POST /api/v1/ws/ticket` through the app's Retrofit layer. */
class ApiWsTicketMinter(private val api: CatalogizerApi) : WsTicketMinter {
    override suspend fun mint(token: String): String {
        val response = api.mintWsTicket("Bearer $token")
        if (response.code() == 401) throw WsTicketUnauthorizedException()
        if (!response.isSuccessful) throw IOException("WebSocket ticket request failed: HTTP ${response.code()}")
        val ticket = response.body()?.ticket
        if (ticket.isNullOrBlank()) throw IOException("WebSocket ticket missing from response")
        return ticket
    }
}

/** Socket callbacks, kept apart from java_websocket so the connect path can be exercised without a network. */
internal interface WsListener {
    fun onOpen(statusMessage: String)
    fun onMessage(message: String)
    fun onClose(code: Int, remote: Boolean)
    fun onError(message: String)
}

internal open class WsClient(uri: URI, private val listener: WsListener) : WebSocketClient(uri) {
    override fun onOpen(handshakedata: ServerHandshake?) {
        listener.onOpen(handshakedata?.httpStatusMessage ?: "")
        send("""{"type":"subscribe","channel":"media_updates"}""")
        send("""{"type":"subscribe","channel":"system_updates"}""")
    }

    override fun onMessage(message: String?) {
        message?.let { listener.onMessage(it) }
    }

    override fun onClose(code: Int, reason: String?, remote: Boolean) {
        listener.onClose(code, remote)
    }

    override fun onError(ex: Exception?) {
        listener.onError(ex?.message ?: "Unknown WebSocket error")
    }
}

/**
 * Manages a WebSocket connection to the Catalogizer API for real-time event streaming.
 * Automatically subscribes to media and system update channels on connection and
 * exposes events as a [StateFlow] of [WebSocketEvent].
 *
 * SEC-WS-1 F2: the session token never appears in the /ws URL. Every connection attempt, a reconnect
 * included, first mints a fresh single-use ticket (30 s TTL) and connects with `?ticket=`.
 */
class WebSocketRepository(
    private val baseUrl: String,
    private val tokenProvider: () -> String?,
    private val ticketMinter: WsTicketMinter? = null,
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
    private val maxReconnectAttempts: Int = 5,
    private val reconnectBaseDelayMs: Long = 1000L
) {
    sealed class WebSocketEvent {
        data class Connected(val serverInfo: String) : WebSocketEvent()
        data object Disconnected : WebSocketEvent()
        data class MessageReceived(val type: String, val payload: String) : WebSocketEvent()
        data class Error(val message: String) : WebSocketEvent()
    }

    private val _events = MutableStateFlow<WebSocketEvent>(WebSocketEvent.Disconnected)
    val events: StateFlow<WebSocketEvent> = _events.asStateFlow()

    private var client: WebSocketClient? = null
    private var job: Job? = null
    private var intentional = false
    private var attempts = 0

    /** Test seam: builds the socket for a URI; tests substitute a client that does not touch the network. */
    internal var clientFactory: (URI, WsListener) -> WebSocketClient = { uri, l -> WsClient(uri, l) }

    fun connect() {
        if (tokenProvider() == null) return
        intentional = false
        attempts = 0
        job?.cancel()
        open()
    }

    private fun open() {
        job = scope.launch {
            // Re-read the token on every attempt so a refreshed session is used.
            val token = tokenProvider() ?: return@launch
            val minter = ticketMinter
            if (minter == null) {
                _events.value = WebSocketEvent.Error("WebSocket ticket minter not configured")
                return@launch
            }
            val ticket = try {
                minter.mint(token)
            } catch (e: WsTicketUnauthorizedException) {
                _events.value = WebSocketEvent.Error(e.message ?: "Unauthorized")
                return@launch // a 401 is final: no reconnect loop with a token the server refuses
            } catch (e: Exception) {
                _events.value = WebSocketEvent.Error(e.message ?: "Ticket request failed")
                scheduleReconnect()
                return@launch
            }
            if (intentional) return@launch
            try {
                val wsUrl = baseUrl.replace("http://", "ws://").replace("https://", "wss://")
                val uri = URI("$wsUrl/ws?ticket=${URLEncoder.encode(ticket, "UTF-8")}")
                client?.close()
                val c = clientFactory(uri, listener)
                client = c
                c.connect()
            } catch (e: Exception) {
                _events.value = WebSocketEvent.Error(e.message ?: "Connection failed")
            }
        }
    }

    private val listener = object : WsListener {
        override fun onOpen(statusMessage: String) {
            attempts = 0
            _events.value = WebSocketEvent.Connected(statusMessage)
        }

        override fun onMessage(message: String) {
            // Extract type from JSON simply
            val typeMatch = Regex(""""type"\s*:\s*"([^"]+)"""").find(message)
            val type = typeMatch?.groupValues?.getOrNull(1) ?: "unknown"
            _events.value = WebSocketEvent.MessageReceived(type, message)
        }

        override fun onClose(code: Int, remote: Boolean) {
            _events.value = WebSocketEvent.Disconnected
            if (!intentional) scheduleReconnect()
        }

        override fun onError(message: String) {
            _events.value = WebSocketEvent.Error(message)
        }
    }

    private fun scheduleReconnect() {
        if (intentional || attempts >= maxReconnectAttempts) return
        val delayMs = reconnectBaseDelayMs * (1L shl attempts)
        attempts++
        job = scope.launch {
            delay(delayMs)
            if (!intentional) open()
        }
    }

    fun disconnect() {
        intentional = true
        job?.cancel()
        job = null
        client?.close()
        client = null
        _events.value = WebSocketEvent.Disconnected
    }
}
