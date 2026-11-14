import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// WSF2-R2-F1: exercises the REAL @vasic-digital/websocket-client (no mock of it); only the ticket mint and the
// global WebSocket constructor are controlled.
const post = vi.fn()
vi.mock('@/lib/api', () => ({ api: { post: (...a: any[]) => post(...a) } }))

import { TicketWebSocketClient } from '../wsTicketClient'

const flush = async () => { for (let i = 0; i < 8; i++) await Promise.resolve() }

class ThrowingWS {
  static calls = 0
  constructor() { ThrowingWS.calls++; throw new DOMException('blocked', 'SecurityError') }
}

describe('TicketWebSocketClient over the real base client (WSF2-R2-F1)', () => {
  beforeEach(() => {
    ThrowingWS.calls = 0
    post.mockReset()
    post.mockImplementation(async () => ({ data: { ticket: 'tk-' + Math.random() } }))
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', ThrowingWS)
  })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('a synchronous `new WebSocket` throw is retried by the wrapper loop, ends in reconnect_failed, and each attempt mints a fresh ticket', async () => {
    const c = new TicketWebSocketClient({ url: 'ws://h/ws', reconnectInterval: 10, reconnectAttempts: 2 })
    const failed = vi.fn()
    const errors = vi.fn()
    c.on('reconnect_failed', failed)
    c.on('error', errors)
    c.connect()
    await flush()
    await vi.advanceTimersByTimeAsync(1000)
    await flush()
    expect(failed).toHaveBeenCalledTimes(1)
    expect(errors).toHaveBeenCalled()
    // 1 initial + 2 retries, one ticket each; the base library must not have retried the spent URL on its own
    expect(post).toHaveBeenCalledTimes(3)
    expect(ThrowingWS.calls).toBe(3)
    expect(c.getState()).toBe('disconnected')
    c.dispose()
  })

  it('after reconnect_failed a manual connect() starts a fresh attempt (does not no-op on a stale inner client)', async () => {
    const c = new TicketWebSocketClient({ url: 'ws://h/ws', reconnectInterval: 10, reconnectAttempts: 1 })
    const failed = vi.fn()
    c.on('reconnect_failed', failed)
    c.connect()
    await flush()
    await vi.advanceTimersByTimeAsync(1000)
    await flush()
    expect(failed).toHaveBeenCalledTimes(1)
    const before = post.mock.calls.length
    c.connect()
    await flush()
    expect(post.mock.calls.length).toBe(before + 1)
    c.dispose()
  })
})
