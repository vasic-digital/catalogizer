import {
  WebSocketClient as BaseWebSocketClient,
  type WebSocketMessage,
  type ConnectionState,
} from '@vasic-digital/websocket-client'
import { api } from '@/lib/api'

/**
 * SEC-WS-1 F2: the session JWT must never appear in the /ws URL (it leaks into proxy and server logs and browser history).
 * The server issues a short-lived (30 s), single-use ticket to an authenticated POST; the socket is opened with ?ticket=.
 * A ticket is consumed by the connection attempt, so every (re)connect needs a fresh one. The base client reuses one fixed URL
 * for its own reconnects, which would replay a spent ticket, so this wrapper turns the base client's reconnect off and runs the
 * reconnect loop itself, minting a new ticket and a new base client per attempt.
 */
export async function mintWsTicket(): Promise<string> {
  const res = await api.post('/ws/ticket', {})
  const ticket = res.data?.ticket
  if (typeof ticket !== 'string' || !ticket) throw new Error('websocket ticket missing from response')
  return ticket
}

export const wsUrlWithTicket = (wsUrl: string, ticket: string) =>
  `${wsUrl}${wsUrl.includes('?') ? '&' : '?'}ticket=${encodeURIComponent(ticket)}`

type Handler = (...args: any[]) => void

export interface TicketSocketOptions {
  url: string
  reconnectAttempts?: number
  reconnectInterval?: number
  maxReconnectInterval?: number
}

export class TicketWebSocketClient {
  private inner: BaseWebSocketClient | null = null
  private handlers: Record<string, Set<Handler>> = {}
  private buffer: string[] = []
  private attempts = 0
  private timer: ReturnType<typeof setTimeout> | null = null
  private disposed = false
  private intentional = false
  private opening = false
  private state: ConnectionState = 'disconnected'

  constructor(private readonly opts: TicketSocketOptions) {}

  on(event: string, handler: Handler): void {
    ;(this.handlers[event] ??= new Set()).add(handler)
  }

  private emit(event: string, ...args: unknown[]): void {
    this.handlers[event]?.forEach((h) => h(...args))
  }

  getState(): ConnectionState {
    return this.inner?.getState() ?? this.state
  }

  connect(): void {
    if (this.disposed) throw new Error('TicketWebSocketClient has been disposed')
    if (this.opening || this.inner) return
    // a manual connect() supersedes a pending backoff timer (otherwise the timer would open a second socket) and, after
    // reconnect_failed, starts a fresh attempt budget
    if (this.timer) clearTimeout(this.timer)
    this.timer = null
    this.attempts = 0
    this.intentional = false
    void this.open()
  }

  sendJSON(message: unknown): void {
    if (this.inner && this.inner.getState() === 'connected') {
      this.inner.sendJSON(message)
    } else if (this.buffer.length < 100) {
      this.buffer.push(JSON.stringify(message))
    }
  }

  dispose(): void {
    this.disposed = true
    this.intentional = true
    if (this.timer) clearTimeout(this.timer)
    this.timer = null
    this.inner?.dispose()
    this.inner = null
    this.handlers = {}
    this.buffer = []
  }

  private async open(): Promise<void> {
    this.opening = true
    this.state = 'connecting'
    let ticket: string
    try {
      ticket = await mintWsTicket()
    } catch {
      this.opening = false
      this.state = 'disconnected'
      this.scheduleReconnect()
      return
    }
    this.opening = false
    if (this.disposed || this.intentional) return
    let client: BaseWebSocketClient
    try {
      client = new BaseWebSocketClient({
        url: wsUrlWithTicket(this.opts.url, ticket),
        reconnect: false, // a spent ticket cannot be replayed: this wrapper reconnects with a new one
        bufferWhileDisconnected: false,
      })
    } catch (e) {
      // e.g. a SecurityError / SyntaxError from `new WebSocket`: no half-built wrapper is kept; retry through our own loop
      this.inner = null
      this.state = 'disconnected'
      this.emit('error', e)
      this.scheduleReconnect()
      return
    }
    client.on('message', (m: WebSocketMessage) => this.emit('message', m))
    client.on('error', (e: unknown) => this.emit('error', e))
    client.on('connected', () => {
      this.attempts = 0
      const pending = this.buffer
      this.buffer = []
      this.emit('connected')
      pending.forEach((raw) => client.sendJSON(JSON.parse(raw)))
    })
    client.on('disconnected', (d: unknown) => {
      if (this.inner === client) this.inner = null
      this.state = 'disconnected'
      client.dispose()
      this.emit('disconnected', d)
      if (!this.intentional && !this.disposed) this.scheduleReconnect()
    })
    this.inner = client
    try {
      client.connect()
    } catch (e) {
      if (this.inner === client) this.inner = null
      client.dispose()
      this.state = 'disconnected'
      this.emit('error', e)
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    const max = this.opts.reconnectAttempts ?? 5
    if (this.disposed || this.attempts >= max) {
      if (!this.disposed) this.emit('reconnect_failed', { attempts: this.attempts })
      return
    }
    this.attempts++
    const base = this.opts.reconnectInterval ?? 1000
    const delay = Math.min(base * 2 ** (this.attempts - 1), this.opts.maxReconnectInterval ?? 30000)
    this.emit('reconnecting', { attempt: this.attempts, maxAttempts: max, delay })
    this.timer = setTimeout(() => {
      this.timer = null
      void this.open()
    }, delay)
  }
}
