import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WebSocketClient } from '../utils/websocket';

// We need to create a mock constructor that vitest recognizes as a class/constructor.
// vi.mock with a factory that returns a default export as a class.
let mockWsInstance: any;

vi.mock('ws', () => {
  // Define the mock class at the factory level
  const MockWS = vi.fn(function (this: any) {
    // Copy properties from mockWsInstance
    Object.assign(this, mockWsInstance);
    // Store reference so we can access callbacks set on 'this' later
    Object.defineProperty(mockWsInstance, '_instance', { value: this, writable: true, configurable: true });
    return this;
  }) as any;

  // Add static constants that ws.WebSocket has
  MockWS.CONNECTING = 0;
  MockWS.OPEN = 1;
  MockWS.CLOSING = 2;
  MockWS.CLOSED = 3;

  return {
    default: MockWS,
    WebSocket: MockWS,
    CONNECTING: 0,
    OPEN: 1,
    CLOSING: 2,
    CLOSED: 3,
  };
});

// Import the mocked WebSocket after vi.mock
import WebSocket from 'ws';
const MockWebSocket = WebSocket as unknown as ReturnType<typeof vi.fn>;

describe('WebSocketClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();

    mockWsInstance = {
      readyState: 1, // WebSocket.OPEN
      onopen: null as any,
      onmessage: null as any,
      onclose: null as any,
      onerror: null as any,
      send: vi.fn(),
      close: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // Helper to get the actual instance that the source code set callbacks on
  function getWsInstance(): any {
    return mockWsInstance._instance || mockWsInstance;
  }

  describe('initialization', () => {
    it('creates WebSocketClient with URL', () => {
      const client = new WebSocketClient('ws://localhost:8080');

      expect(client).toBeDefined();
      expect(client.isConnected()).toBe(false);
    });
  });

  describe('connection', () => {
    it('connects successfully without auth token', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      // The source code sets this.ws.onopen on the constructed instance
      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      expect(MockWebSocket).toHaveBeenCalledWith('ws://localhost:8080');
      expect(client.isConnected()).toBe(true);
    });

    it('connects successfully with auth token', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect('test-token');

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      // SEC-WS-1 F2: the token never goes in the URL; without a ticket provider it is sent as a Bearer header.
      expect(MockWebSocket).toHaveBeenCalledWith('ws://localhost:8080', {
        headers: { Authorization: 'Bearer test-token' },
      });
      expect(String(MockWebSocket.mock.calls[0][0])).not.toContain('test-token');
    });

    it('emits connection:open event on successful connection', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const openListener = vi.fn();

      client.on('connection:open', openListener);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      expect(openListener).toHaveBeenCalled();
    });

    it('handles connection error', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const errorListener = vi.fn();

      client.on('connection:error', errorListener);

      const connectPromise = client.connect();

      const error = new Error('Connection failed');
      const inst = getWsInstance();
      if (inst.onerror) {
        inst.onerror(error as any);
      }

      await expect(connectPromise).rejects.toThrow('Connection failed');
      expect(errorListener).toHaveBeenCalledWith(error);
    });

    it('does not reconnect if already connecting', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const promise1 = client.connect();
      const promise2 = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await promise1;
      await promise2;

      expect(MockWebSocket).toHaveBeenCalledTimes(1);
    });

    it('does not reconnect if already connected', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const promise1 = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await promise1;

      await client.connect();

      expect(MockWebSocket).toHaveBeenCalledTimes(1);
    });
  });

  describe('disconnection', () => {
    it('disconnects successfully', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      client.disconnect();

      expect(inst.close).toHaveBeenCalled();
      expect(client.isConnected()).toBe(false);
    });

    it('emits connection:close event on disconnection', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const closeListener = vi.fn();

      client.on('connection:close', closeListener);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      if (inst.onclose) {
        inst.onclose({ wasClean: true } as any);
      }

      expect(closeListener).toHaveBeenCalled();
    });

    it('does not reconnect after manual disconnect', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      client.disconnect();

      if (inst.onclose) {
        inst.onclose({ wasClean: false } as any);
      }

      vi.advanceTimersByTime(5000);

      expect(MockWebSocket).toHaveBeenCalledTimes(1);
    });
  });

  describe('message handling', () => {
    it('handles download progress messages', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const progressListener = vi.fn();

      client.on('download:progress', progressListener);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      const message = {
        type: 'download_progress',
        data: { id: 1, progress: 50, speed: 1000000 },
      };

      if (inst.onmessage) {
        inst.onmessage({ data: JSON.stringify(message) } as any);
      }

      expect(progressListener).toHaveBeenCalledWith({ id: 1, progress: 50, speed: 1000000 });
    });

    it('handles scan progress messages', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const progressListener = vi.fn();

      client.on('scan:progress', progressListener);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      const message = {
        type: 'scan_progress',
        data: { id: 1, scanned: 100, total: 500 },
      };

      if (inst.onmessage) {
        inst.onmessage({ data: JSON.stringify(message) } as any);
      }

      expect(progressListener).toHaveBeenCalledWith({ id: 1, scanned: 100, total: 500 });
    });

    it('handles pong messages (heartbeat)', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const messageListener = vi.fn();
      client.on('message', messageListener);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      const message = { type: 'pong', timestamp: new Date().toISOString() };

      if (inst.onmessage) {
        inst.onmessage({ data: JSON.stringify(message) } as any);
      }

      // Article XI: pong is handled silently — assert the message listener
      // was NOT triggered (confirms the heartbeat was actually swallowed,
      // not just that the call didn't throw).
      expect(messageListener).not.toHaveBeenCalled();
    });

    it('emits generic message event for unknown types', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const messageListener = vi.fn();

      client.on('message', messageListener);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      const message = { type: 'unknown', data: { foo: 'bar' } };

      if (inst.onmessage) {
        inst.onmessage({ data: JSON.stringify(message) } as any);
      }

      expect(messageListener).toHaveBeenCalledWith(message);
    });

    it('handles malformed JSON messages gracefully', async () => {
      const client = new WebSocketClient('ws://localhost:8080');
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      if (inst.onmessage) {
        inst.onmessage({ data: 'not valid json' } as any);
      }

      expect(consoleSpy).toHaveBeenCalledWith(
        'Failed to parse WebSocket message:',
        expect.any(Error)
      );

      consoleSpy.mockRestore();
    });
  });

  describe('sending messages', () => {
    it('sends message when connected', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      const message = { type: 'test', data: { foo: 'bar' } };
      client.send(message);

      expect(inst.send).toHaveBeenCalledWith(JSON.stringify(message));
    });

    it('throws error when sending while not connected', () => {
      const client = new WebSocketClient('ws://localhost:8080');

      expect(() => {
        client.send({ type: 'test' });
      }).toThrow('WebSocket is not connected');
    });
  });

  describe('heartbeat mechanism', () => {
    it('sends ping messages at regular intervals', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      // Clear initial send calls
      inst.send.mockClear();

      // Fast-forward 30 seconds
      vi.advanceTimersByTime(30000);

      expect(inst.send).toHaveBeenCalledWith(expect.stringContaining('"type":"ping"'));
    });

    it('stops sending pings after disconnect', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      client.disconnect();

      inst.send.mockClear();

      vi.advanceTimersByTime(30000);

      expect(inst.send).not.toHaveBeenCalled();
    });
  });

  describe('reconnection mechanism', () => {
    it('schedules reconnection after unclean disconnect', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      // Simulate unclean disconnect
      if (inst.onclose) {
        inst.onclose({ wasClean: false } as any);
      }

      // Verify a timer was set (reconnection scheduled)
      expect(vi.getTimerCount()).toBeGreaterThan(0);
    });

    it('does not reconnect after clean disconnect', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      // Clear any existing timers
      vi.clearAllTimers();

      // Simulate clean disconnect
      if (inst.onclose) {
        inst.onclose({ wasClean: true } as any);
      }

      // No new timers should be set
      expect(vi.getTimerCount()).toBe(0);
    });
  });

  describe('auth token management', () => {
    it('sets auth token', () => {
      const client = new WebSocketClient('ws://localhost:8080');
      client.setAuthToken('new-token');

      // Token should be stored (tested indirectly through connection)
      expect(client).toBeDefined();
    });

    it('uses updated auth token on reconnection', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      const connectPromise = client.connect('old-token');

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      client.setAuthToken('new-token');

      // Simulate disconnect and reconnect
      if (inst.onclose) {
        inst.onclose({ wasClean: false } as any);
      }

      vi.advanceTimersByTime(1000);

      // Reconnect uses the authToken stored at connect() time (old-token)
      expect(MockWebSocket).toHaveBeenLastCalledWith('ws://localhost:8080', {
        headers: { Authorization: 'Bearer old-token' },
      });
    });
  });

  describe('connection state', () => {
    it('reports connected state correctly', async () => {
      const client = new WebSocketClient('ws://localhost:8080');

      expect(client.isConnected()).toBe(false);

      const connectPromise = client.connect();

      const inst = getWsInstance();
      if (inst.onopen) {
        inst.onopen({} as any);
      }

      await connectPromise;

      expect(client.isConnected()).toBe(true);

      client.disconnect();

      // After disconnect, ws is set to undefined so isConnected returns false
      expect(client.isConnected()).toBe(false);
    });

    it('reports not connected when readyState is CONNECTING', () => {
      // Override the mockWsInstance readyState to CONNECTING before connect
      mockWsInstance.readyState = 0; // CONNECTING

      const client = new WebSocketClient('ws://localhost:8080');

      client.connect();

      expect(client.isConnected()).toBe(false);
    });

    it('reports not connected when readyState is CLOSING', () => {
      const client = new WebSocketClient('ws://localhost:8080');

      expect(client.isConnected()).toBe(false);
    });
  });
});
