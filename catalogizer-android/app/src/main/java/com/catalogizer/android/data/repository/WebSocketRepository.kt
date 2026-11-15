package com.catalogizer.android.data.repository

import com.catalogizer.android.data.remote.CatalogizerApi
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.isActive
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import org.java_websocket.client.WebSocketClient
import org.java_websocket.handshake.ServerHandshake
import java.io.IOException
import java.net.URI
import java.net.URLEncoder
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

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

    // WSF2-R2-F2: state is touched from the caller thread, the coroutine pool and the socket threads.
    // lock guards client/job; intentional is volatile; attempts is atomic.
    private val lock = Any()
    private var client: WebSocketClient? = null
    private var job: Job? = null
    // @Volatile: read lock-free by socket threads in the listener's onClose; written under lock.
    @Volatile private var intentional = false
    private val attempts = AtomicInteger(0)

    /** Test seam: builds the socket for a URI; tests substitute a client that does not touch the network. */
    internal var clientFactory: (URI, WsListener) -> WebSocketClient = { uri, l -> WsClient(uri, l) }

    fun connect() {
        if (tokenProvider() == null) return
        synchronized(lock) {
            intentional = false
            attempts.set(0)
            job?.cancel()
            open()
        }
    }

    private fun open() {
        val newJob = scope.launch {
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
                // WSF2B2-F2: rethrow only when THIS attempt was cancelled (connect()/disconnect()); a
                // CancellationException the minter raised itself (withTimeout) is an ordinary mint failure.
                if (e is CancellationException && !isActive) throw e
                _events.value = WebSocketEvent.Error(e.message ?: "Ticket request failed")
                scheduleReconnect()
                return@launch
            }
            // WSF2B2-F1/F2: mint() may return without a suspension point after this attempt was cancelled
            // (disconnect()/connect()); build and install nothing for a cancelled attempt.
            ensureActive()
            try {
                val wsUrl = baseUrl.replace("http://", "ws://").replace("https://", "wss://")
                val uri = URI("$wsUrl/ws?ticket=${URLEncoder.encode(ticket, "UTF-8")}")
                val holder = AtomicReference<WebSocketClient?>(null)
                val c = clientFactory(uri, currentOnly(holder))
                holder.set(c)
                val old: WebSocketClient?
                synchronized(lock) {
                    if (intentional || !isActive) return@launch
                    old = client
                    client = c
                }
                old?.close()
                c.connect()
            } catch (e: Exception) {
                _events.value = WebSocketEvent.Error(e.message ?: "Connection failed")
            }
        }
        job = newJob
    }

    /**
     * Each socket gets its own listener view: callbacks from a client that is no longer the current one
     * (replaced by connect(), or dropped by disconnect()) are ignored, so a replaced client's onClose can
     * never trigger scheduleReconnect or overwrite the event state.
     */
    private fun currentOnly(holder: AtomicReference<WebSocketClient?>): WsListener = object : WsListener {
        private fun me(): WebSocketClient? = holder.get()

        private fun current(): Boolean {
            val me = me() ?: return false
            return synchronized(lock) { client === me }
        }

        override fun onOpen(statusMessage: String) {
            // An in-flight replaced/disconnected client can still complete its handshake (Java-WebSocket 1.5.4
            // close() is a no-op until its writeThread is set), so close it here or it stays open and subscribes.
            if (current()) listener.onOpen(statusMessage) else me()?.close()
        }
        override fun onMessage(message: String) { if (current()) listener.onMessage(message) }
        override fun onClose(code: Int, remote: Boolean) { if (current()) listener.onClose(code, remote) }
        override fun onError(message: String) { if (current()) listener.onError(message) }
    }

    private val listener = object : WsListener {
        override fun onOpen(statusMessage: String) {
            attempts.set(0)
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
        synchronized(lock) {
            if (intentional || attempts.get() >= maxReconnectAttempts) return
            val delayMs = reconnectBaseDelayMs * (1L shl attempts.getAndIncrement())
            job = scope.launch {
                delay(delayMs)
                synchronized(lock) { if (!intentional) open() }
            }
        }
    }

    fun disconnect() {
        val old: WebSocketClient?
        synchronized(lock) {
            intentional = true
            job?.cancel()
            job = null
            old = client
            client = null
        }
        old?.close()
        _events.value = WebSocketEvent.Disconnected
    }
}
