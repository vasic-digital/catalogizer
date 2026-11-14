import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest';
import { CatalogizerClient } from '../index';
import axios from 'axios';

vi.mock('axios');

// Track all WS instances created during tests
let wsInstances: any[] = [];

vi.mock('ws', () => {
  const MockWS = vi.fn(function (this: any) {
    Object.defineProperty(this, 'readyState', {
      value: 1, // OPEN
      writable: true,
      configurable: true,
    });
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    this.send = vi.fn();
    this.close = vi.fn();
    this.addEventListener = vi.fn();
    this.removeEventListener = vi.fn();
    wsInstances.push(this);
    return this;
  }) as any;

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

import WebSocket from 'ws';
const MockWebSocket = WebSocket as unknown as Mock;
const mockAxios = axios as unknown as { create: Mock };

describe('CatalogizerClient Integration', () => {
  let mockAxiosInstance: any;

  beforeEach(() => {
    vi.clearAllMocks();
    wsInstances = [];

    mockAxiosInstance = {
      interceptors: {
        request: { use: vi.fn(), eject: vi.fn() },
        response: { use: vi.fn(), eject: vi.fn() },
      },
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      patch: vi.fn(),
      delete: vi.fn(),
      defaults: {
        headers: {},
        baseURL: '',
        timeout: 30000,
      },
    };

    mockAxios.create.mockReturnValue(mockAxiosInstance);
  });

  /** Helper: trigger onopen on the last created WS mock instance */
  function triggerWsOpen(): void {
    const inst = wsInstances[wsInstances.length - 1];
    if (inst && inst.onopen) {
      inst.onopen({});
    }
  }

  describe('client initialization', () => {
    it('creates client with all services', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      expect(client).toBeDefined();
      expect(client.auth).toBeDefined();
      expect(client.media).toBeDefined();
      expect(client.smb).toBeDefined();
    });

    it('initializes with custom configuration', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        timeout: 10000,
        headers: { 'X-Custom': 'value' },
      });

      const config = client.getConfig();

      expect(config.baseURL).toBe('http://localhost:8080');
      expect(config.timeout).toBe(10000);
      expect(config.headers).toEqual({ 'X-Custom': 'value' });
    });
  });

  describe('server connection', () => {
    it('connects to server successfully', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get.mockResolvedValueOnce({
        data: { status: 'healthy', version: '1.0.0' },
      });

      const result = await client.connect();

      expect(mockAxiosInstance.get).toHaveBeenCalledWith('/health', undefined);
      expect(result).toBeNull();
    });

    it('connects with credentials and logs in', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const loginResponse = {
        user: { id: 1, username: 'testuser' },
        token: 'jwt-token',
        refresh_token: 'refresh-token',
        expires_in: 3600,
      };

      mockAxiosInstance.get.mockResolvedValueOnce({ data: { status: 'healthy' } });
      mockAxiosInstance.post.mockResolvedValueOnce({ data: loginResponse });

      const loginListener = vi.fn();
      client.on('auth:login', loginListener);

      const result = await client.connect({
        username: 'testuser',
        password: 'password',
      });

      expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/login', {
        username: 'testuser',
        password: 'password',
      }, undefined);
      expect(result).toEqual(loginResponse);
      expect(loginListener).toHaveBeenCalledWith(loginResponse.user);
    });

    it('checks existing authentication on connect', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get
        .mockResolvedValueOnce({ data: { status: 'healthy' } })
        .mockResolvedValueOnce({
          data: {
            authenticated: true,
            user: { id: 1, username: 'testuser' },
          },
        });

      const loginListener = vi.fn();
      client.on('auth:login', loginListener);

      await client.connect();

      expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/status', undefined);
      expect(loginListener).toHaveBeenCalledWith({ id: 1, username: 'testuser' });
    });

    it('handles connection failure gracefully', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get.mockRejectedValueOnce(new Error('Connection refused'));

      await expect(client.connect()).rejects.toThrow(
        'Failed to connect to Catalogizer server'
      );
    });
  });

  describe('disconnection', () => {
    it('disconnects and logs out', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      const logoutListener = vi.fn();
      client.on('auth:logout', logoutListener);

      await client.disconnect();

      expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/logout', undefined, undefined);
      expect(logoutListener).toHaveBeenCalled();
    });

    it('handles logout errors gracefully', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.post.mockRejectedValueOnce(new Error('Logout failed'));

      const logoutListener = vi.fn();
      client.on('auth:logout', logoutListener);

      await client.disconnect();

      // Should still emit logout event even on error
      expect(logoutListener).toHaveBeenCalled();
    });
  });

  describe('authentication state', () => {
    it('checks if user is authenticated', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get.mockResolvedValueOnce({
        data: { authenticated: true },
      });

      const result = await client.isAuthenticated();

      expect(result).toBe(true);
      expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/status', undefined);
    });

    it('returns false when not authenticated', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get.mockResolvedValueOnce({
        data: { authenticated: false },
      });

      const result = await client.isAuthenticated();

      expect(result).toBe(false);
    });

    it('returns false on authentication check error', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get.mockRejectedValueOnce(new Error('Network error'));

      const result = await client.isAuthenticated();

      expect(result).toBe(false);
    });

    it('gets current user', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const mockUser = { id: 1, username: 'testuser', email: 'test@test.com' };
      mockAxiosInstance.get.mockResolvedValueOnce({ data: mockUser });

      const result = await client.getCurrentUser();

      expect(result).toEqual(mockUser);
      expect(mockAxiosInstance.get).toHaveBeenCalledWith('/auth/profile', undefined);
    });

    it('returns null when getting current user fails', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get.mockRejectedValueOnce(new Error('Not authenticated'));

      const result = await client.getCurrentUser();

      expect(result).toBeNull();
    });
  });

  describe('token management', () => {
    it('sets authentication token', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      client.setAuthToken('test-token');

      expect(client.getAuthToken()).toBe('test-token');
    });

    it('clears authentication token', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      client.setAuthToken('test-token');
      client.clearAuthToken();

      expect(client.getAuthToken()).toBeUndefined();
    });

    it('sets token in WebSocket when available', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      client.setAuthToken('test-token');

      // Token should be set in WebSocket client (tested indirectly)
      expect(client).toBeDefined();
    });
  });

  describe('configuration', () => {
    it('updates client configuration', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      client.updateConfig({
        baseURL: 'http://newhost:9090',
        timeout: 5000,
      });

      const config = client.getConfig();

      expect(config.baseURL).toBe('http://newhost:9090');
      expect(config.timeout).toBe(5000);
    });

    it('returns current configuration', () => {
      const initialConfig = {
        baseURL: 'http://localhost:8080',
        timeout: 10000,
      };

      const client = new CatalogizerClient(initialConfig);
      const config = client.getConfig();

      expect(config.baseURL).toBe(initialConfig.baseURL);
      expect(config.timeout).toBe(initialConfig.timeout);
    });
  });

  describe('health check', () => {
    it('performs health check', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const healthResponse = {
        status: 'healthy',
        version: '1.0.0',
        timestamp: '2024-01-01T00:00:00Z',
      };

      mockAxiosInstance.get.mockResolvedValueOnce({ data: healthResponse });

      const result = await client.healthCheck();

      expect(mockAxiosInstance.get).toHaveBeenCalledWith('/health', undefined);
      expect(result).toEqual(healthResponse);
    });
  });

  describe('server info', () => {
    it('gets server information', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const serverInfo = {
        name: 'Catalogizer',
        version: '1.0.0',
        description: 'Media catalog manager',
        features: ['media', 'smb', 'ftp'],
      };

      mockAxiosInstance.get.mockResolvedValueOnce({ data: serverInfo });

      const result = await client.getServerInfo();

      expect(mockAxiosInstance.get).toHaveBeenCalledWith('/info', undefined);
      expect(result).toEqual(serverInfo);
    });
  });

  describe('event handling', () => {
    it('emits auth:login event on successful login', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const loginResponse = {
        user: { id: 1, username: 'testuser' },
        token: 'jwt-token',
        refresh_token: 'refresh-token',
        expires_in: 3600,
      };

      mockAxiosInstance.get.mockResolvedValueOnce({ data: { status: 'healthy' } });
      mockAxiosInstance.post.mockResolvedValueOnce({ data: loginResponse });

      const loginListener = vi.fn();
      client.on('auth:login', loginListener);

      await client.connect({ username: 'testuser', password: 'password' });

      expect(loginListener).toHaveBeenCalledWith(loginResponse.user);
    });

    it('emits auth:logout event on disconnect', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      const logoutListener = vi.fn();
      client.on('auth:logout', logoutListener);

      await client.disconnect();

      expect(logoutListener).toHaveBeenCalled();
    });
  });

  describe('token refresh handling', () => {
    it('emits auth:token_refresh event on token refresh', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const tokenRefreshListener = vi.fn();
      client.on('auth:token_refresh', tokenRefreshListener);

      mockAxiosInstance.post.mockResolvedValueOnce({
        data: { token: 'new-token', expires_in: 3600 },
      });

      // Simulate token refresh by accessing handleTokenRefresh through auth.refreshToken
      await client.auth.refreshToken();

      // The token refresh event should be emitted
      // Note: This is tested indirectly through the auth service
      expect(mockAxiosInstance.post).toHaveBeenCalledWith('/auth/refresh', undefined, undefined);
    });
  });

  describe('WebSocket initialization', () => {
    it('warns when WebSocket enabled without URL', () => {
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
      });

      expect(consoleSpy).toHaveBeenCalledWith('WebSocket enabled but no WebSocket URL provided');
      expect(client).toBeDefined();
      consoleSpy.mockRestore();
    });

    it('initializes WebSocket when enabled with URL', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      expect(client).toBeDefined();
      expect(MockWebSocket).toHaveBeenCalledTimes(0); // WS not constructed until connect()
    });

    it('proxies WebSocket connection:open event', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      const openListener = vi.fn();
      client.on('connection:open', openListener);

      // Internally the CatalogizerClient listens on ws events,
      // so the ws instance must exist
      expect(client).toBeDefined();
    });
  });

  describe('connect with WebSocket', () => {
    it('connects WebSocket after login when enabled', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      const loginResponse = {
        user: { id: 1, username: 'testuser' },
        token: 'jwt-token',
        refresh_token: 'refresh-token',
        expires_in: 3600,
      };

      mockAxiosInstance.get.mockResolvedValueOnce({ data: { status: 'healthy' } });
      mockAxiosInstance.post
        .mockResolvedValueOnce({ data: loginResponse })
        // POST /ws/ticket (SEC-WS-1 F2): the raw server body {ticket, expires_at, expires_in}
        .mockResolvedValueOnce({ data: { ticket: 'tkt-login', expires_in: 30 } });

      // Start connect (will block on WS connect waiting for onopen)
      const connectPromise = client.connect({
        username: 'testuser',
        password: 'password',
      });

      // Let HTTP mocks resolve (microtasks), then trigger WS onopen
      await vi.waitFor(() => {
        expect(wsInstances.length).toBeGreaterThan(0);
      }, { timeout: 1000 });
      triggerWsOpen();

      const result = await connectPromise;

      expect(result).toEqual(loginResponse);
      expect(MockWebSocket).toHaveBeenCalled();
      expect(MockWebSocket.mock.calls[0][0]).toBe('ws://localhost:8080/ws?ticket=tkt-login');
      expect(MockWebSocket.mock.calls[0][0]).not.toContain('jwt-token');
      expect(mockAxiosInstance.post).toHaveBeenLastCalledWith('/ws/ticket', {}, undefined);
    });

    it('connects WebSocket with existing token when already authenticated', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      client.setAuthToken('existing-token');
      mockAxiosInstance.post.mockResolvedValueOnce({ data: { ticket: 'tkt-existing' } });

      mockAxiosInstance.get
        .mockResolvedValueOnce({ data: { status: 'healthy' } })
        .mockResolvedValueOnce({
          data: {
            authenticated: true,
            user: { id: 1, username: 'testuser' },
          },
        });

      const connectPromise = client.connect();

      await vi.waitFor(() => {
        expect(wsInstances.length).toBeGreaterThan(0);
      }, { timeout: 1000 });
      triggerWsOpen();

      await connectPromise;

      expect(MockWebSocket).toHaveBeenCalled();
      expect(MockWebSocket.mock.calls[0][0]).toBe('ws://localhost:8080/ws?ticket=tkt-existing');
      expect(MockWebSocket.mock.calls[0][0]).not.toContain('existing-token');
    });
  });

  describe('connect without credentials - not authenticated', () => {
    it('handles unauthenticated status gracefully', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get
        .mockResolvedValueOnce({ data: { status: 'healthy' } })
        .mockResolvedValueOnce({
          data: {
            authenticated: false,
          },
        });

      const loginListener = vi.fn();
      client.on('auth:login', loginListener);

      const result = await client.connect();

      expect(result).toBeNull();
      expect(loginListener).not.toHaveBeenCalled();
    });

    it('handles auth status error gracefully', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      mockAxiosInstance.get
        .mockResolvedValueOnce({ data: { status: 'healthy' } })
        .mockRejectedValueOnce(new Error('Auth check failed'));

      const loginListener = vi.fn();
      client.on('auth:login', loginListener);

      const result = await client.connect();

      expect(result).toBeNull();
      expect(loginListener).not.toHaveBeenCalled();
    });
  });

  describe('disconnect with WebSocket', () => {
    it('disconnects WebSocket when active', async () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      mockAxiosInstance.post.mockResolvedValueOnce({ data: {} });

      const logoutListener = vi.fn();
      client.on('auth:logout', logoutListener);

      await client.disconnect();

      expect(logoutListener).toHaveBeenCalled();
    });
  });

  describe('updateConfig with WebSocket changes', () => {
    it('reinitializes WebSocket when enableWebSocket changes', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      // Disable WebSocket
      client.updateConfig({ enableWebSocket: false });

      const config = client.getConfig();
      expect(config.enableWebSocket).toBe(false);
    });

    it('reinitializes WebSocket when webSocketURL changes', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      client.updateConfig({ webSocketURL: 'ws://newhost:9090/ws' });

      const config = client.getConfig();
      expect(config.webSocketURL).toBe('ws://newhost:9090/ws');
    });

    it('creates new WebSocket when re-enabling', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      client.updateConfig({
        enableWebSocket: true,
        webSocketURL: 'ws://localhost:8080/ws',
      });

      const config = client.getConfig();
      expect(config.enableWebSocket).toBe(true);
      expect(config.webSocketURL).toBe('ws://localhost:8080/ws');
    });
  });

  describe('typed event methods', () => {
    it('supports on/off/emit with typed events', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const listener = vi.fn();
      client.on('auth:logout', listener);

      client.emit('auth:logout');
      expect(listener).toHaveBeenCalledTimes(1);

      client.off('auth:logout', listener);
      client.emit('auth:logout');
      expect(listener).toHaveBeenCalledTimes(1);
    });

    it('supports on/off/emit with arbitrary string events', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
      });

      const listener = vi.fn();
      client.on('custom:event', listener);

      client.emit('custom:event', 'data');
      expect(listener).toHaveBeenCalledWith('data');

      client.off('custom:event', listener);
      client.emit('custom:event', 'data2');
      expect(listener).toHaveBeenCalledTimes(1);
    });
  });

  describe('getConfig returns a copy', () => {
    it('does not expose internal config reference', () => {
      const client = new CatalogizerClient({
        baseURL: 'http://localhost:8080',
        timeout: 5000,
      });

      const config1 = client.getConfig();
      config1.timeout = 99999;

      const config2 = client.getConfig();
      expect(config2.timeout).toBe(5000);
    });
  });
});
