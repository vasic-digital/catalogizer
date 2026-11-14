import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WebSocketClient } from '../utils/websocket';

// SEC-WS-1 F2: with a ticket provider every connection attempt (reconnects included) mints a fresh single-use
// ticket and connects with ?ticket=; the session JWT never appears in the URL.
const sockets: any[] = [];

vi.mock('ws', () => {
  const MockWS = vi.fn(function (this: any, url: string, opts?: any) {
    Object.assign(this, { readyState: 1, send: vi.fn(), close: vi.fn(), url, opts });
    sockets.push(this);
    return this;
  }) as any;
  MockWS.CONNECTING = 0;
  MockWS.OPEN = 1;
  MockWS.CLOSING = 2;
  MockWS.CLOSED = 3;
  return { default: MockWS, WebSocket: MockWS };
});

import WebSocket from 'ws';
const MockWebSocket = WebSocket as unknown as ReturnType<typeof vi.fn>;

const JWT = 'eyJhbGciOiJIUzI1NiJ9.SECRET-PAYLOAD.SIGNATURE';

async function flush() {
  for (let i = 0; i < 5; i++) await Promise.resolve();
}

describe('WebSocketClient ticket auth (SEC-WS-1 F2)', () => {
  beforeEach(() => {
    sockets.length = 0;
    vi.clearAllMocks();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('mints a ticket, connects with ?ticket= and keeps the JWT out of the URL', async () => {
    const mint = vi.fn().mockResolvedValue('tkt-1');
    const client = new WebSocketClient('ws://localhost:8080/ws', { ticketProvider: mint });

    const p = client.connect(JWT);
    await flush();
    sockets[0].onopen({});
    await p;

    expect(mint).toHaveBeenCalledTimes(1);
    expect(MockWebSocket).toHaveBeenCalledWith('ws://localhost:8080/ws?ticket=tkt-1');
    for (const c of MockWebSocket.mock.calls) {
      expect(String(c[0])).not.toContain(JWT);
      expect(String(c[0])).not.toContain('token=');
      expect(JSON.stringify(c[1] ?? {})).not.toContain(JWT);
    }
  });

  it('mints a NEW ticket on every reconnect', async () => {
    let n = 0;
    const mint = vi.fn(async () => `tkt-${++n}`);
    const client = new WebSocketClient('ws://localhost:8080/ws', { ticketProvider: mint });

    const p = client.connect(JWT);
    await flush();
    sockets[0].onopen({});
    await p;

    sockets[0].readyState = 3; // CLOSED, as a real dropped socket
    sockets[0].onclose({ wasClean: false });
    await vi.advanceTimersByTimeAsync(1000);
    await flush();

    expect(mint).toHaveBeenCalledTimes(2);
    expect(sockets).toHaveLength(2);
    expect(sockets[0].url).toBe('ws://localhost:8080/ws?ticket=tkt-1');
    expect(sockets[1].url).toBe('ws://localhost:8080/ws?ticket=tkt-2');
    expect(sockets.every((s) => !s.url.includes(JWT))).toBe(true);
  });

  it('a 401 from the ticket request rejects, emits connection:error and opens no socket', async () => {
    const err = Object.assign(new Error('unauthorized'), { response: { status: 401 } });
    const mint = vi.fn().mockRejectedValue(err);
    const client = new WebSocketClient('ws://localhost:8080/ws', { ticketProvider: mint });
    const onError = vi.fn();
    client.on('connection:error', onError);

    await expect(client.connect(JWT)).rejects.toBe(err);

    expect(onError).toHaveBeenCalledWith(err);
    expect(MockWebSocket).not.toHaveBeenCalled();
    expect(client.isConnected()).toBe(false);
  });

  it('a transient mint failure on reconnect is retried, a 401 stops the loop', async () => {
    let call = 0;
    const mint = vi.fn(async () => {
      call++;
      if (call === 1) return 'tkt-1';
      if (call === 2) throw new Error('network down');
      throw Object.assign(new Error('unauthorized'), { response: { status: 401 } });
    });
    const client = new WebSocketClient('ws://localhost:8080/ws', { ticketProvider: mint });
    vi.spyOn(console, 'error').mockImplementation(() => {});
    vi.spyOn(console, 'log').mockImplementation(() => {});

    const p = client.connect(JWT);
    await flush();
    sockets[0].onopen({});
    await p;
    sockets[0].readyState = 3;
    sockets[0].onclose({ wasClean: false });

    await vi.advanceTimersByTimeAsync(1000); // attempt 2: network down -> rescheduled
    await flush();
    await vi.advanceTimersByTimeAsync(2000); // attempt 3: 401 -> final
    await flush();
    await vi.advanceTimersByTimeAsync(60000);
    await flush();

    expect(mint).toHaveBeenCalledTimes(3);
    expect(sockets).toHaveLength(1);
  });

  it('disconnect() during ticket minting opens no socket', async () => {
    let release!: (t: string) => void;
    const mint = vi.fn(() => new Promise<string>((r) => (release = r)));
    const client = new WebSocketClient('ws://localhost:8080/ws', { ticketProvider: mint });

    const p = client.connect(JWT);
    client.disconnect();
    release('late');
    await p;

    expect(MockWebSocket).not.toHaveBeenCalled();
  });

  it('an empty ticket is refused (no fallback to a token URL)', async () => {
    const client = new WebSocketClient('ws://localhost:8080/ws', { ticketProvider: async () => '' });
    await expect(client.connect(JWT)).rejects.toThrow(/ticket missing/);
    expect(MockWebSocket).not.toHaveBeenCalled();
  });

  it('appends the ticket with & when the URL already has a query', async () => {
    const client = new WebSocketClient('ws://h/ws?x=1', { ticketProvider: async () => 'a b' });
    const p = client.connect(JWT);
    await flush();
    sockets[0].onopen({});
    await p;
    expect(sockets[0].url).toBe('ws://h/ws?x=1&ticket=a%20b');
  });
});
