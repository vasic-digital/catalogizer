import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const created: { url: string; handlers: Record<string, (...a: any[]) => void>; disposed: boolean }[] = []

vi.mock('@vasic-digital/websocket-client', () => ({
  WebSocketClient: class {
    url: string
    handlers: Record<string, (...a: any[]) => void> = {}
    disposed = false
    constructor(cfg: { url: string; reconnect?: boolean }) {
      this.url = cfg.url
      expect(cfg.reconnect).toBe(false)
      created.push(this as any)
    }
    on(e: string, h: (...a: any[]) => void) { this.handlers[e] = h }
    connect() { queueMicrotask(() => this.handlers['connected']?.()) }
    getState() { return 'connected' }
    sendJSON() {}
    dispose() { this.disposed = true }
  },
}))

const post = vi.fn()
vi.mock('@/lib/api', () => ({ api: { post: (...a: any[]) => post(...a) } }))

import { TicketWebSocketClient, wsUrlWithTicket } from '../wsTicketClient'

const flush = async () => { for (let i = 0; i < 5; i++) await Promise.resolve() }

describe('TicketWebSocketClient (SEC-WS-1 F2)', () => {
  beforeEach(() => { created.length = 0; post.mockReset(); vi.useFakeTimers() })
  afterEach(() => vi.useRealTimers())

  it('opens the socket with ?ticket= and never puts a session token in the URL', async () => {
    localStorage.setItem('auth_token', 'SECRET-JWT')
    post.mockResolvedValue({ data: { ticket: 'tk-1' } })
    const c = new TicketWebSocketClient({ url: 'ws://h/ws' })
    c.connect()
    await flush()
    expect(post).toHaveBeenCalledWith('/ws/ticket', {})
    expect(created[0].url).toBe('ws://h/ws?ticket=tk-1')
    expect(created[0].url).not.toContain('SECRET-JWT')
    expect(created[0].url).not.toContain('token=')
    c.dispose()
  })

  it('mints a FRESH ticket for every reconnect (a spent ticket is never replayed)', async () => {
    post.mockResolvedValueOnce({ data: { ticket: 'tk-1' } }).mockResolvedValueOnce({ data: { ticket: 'tk-2' } })
    const c = new TicketWebSocketClient({ url: 'ws://h/ws', reconnectInterval: 10 })
    c.connect()
    await flush()
    created[0].handlers['disconnected']({})
    await vi.advanceTimersByTimeAsync(20)
    await flush()
    expect(post).toHaveBeenCalledTimes(2)
    expect(created.map((x) => x.url)).toEqual(['ws://h/ws?ticket=tk-1', 'ws://h/ws?ticket=tk-2'])
    expect(created[0].disposed).toBe(true)
    c.dispose()
  })

  it('does not reconnect after dispose', async () => {
    post.mockResolvedValue({ data: { ticket: 'tk-1' } })
    const c = new TicketWebSocketClient({ url: 'ws://h/ws', reconnectInterval: 10 })
    c.connect()
    await flush()
    c.dispose()
    await vi.advanceTimersByTimeAsync(100)
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('retries when minting fails, up to the attempt limit', async () => {
    post.mockRejectedValue(new Error('401'))
    const c = new TicketWebSocketClient({ url: 'ws://h/ws', reconnectInterval: 10, reconnectAttempts: 2 })
    c.connect()
    await flush()
    await vi.advanceTimersByTimeAsync(500)
    expect(post.mock.calls.length).toBe(3)
    expect(created.length).toBe(0)
    c.dispose()
  })

  it('wsUrlWithTicket appends with the right separator and encodes', () => {
    expect(wsUrlWithTicket('ws://h/ws', 'a b')).toBe('ws://h/ws?ticket=a%20b')
    expect(wsUrlWithTicket('ws://h/ws?x=1', 't')).toBe('ws://h/ws?x=1&ticket=t')
  })
})
