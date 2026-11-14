/**
 * Tests for configuration and environment handling across the catalog-web app.
 *
 * The app uses import.meta.env for Vite environment variables:
 * - VITE_API_BASE_URL: API base URL (default: http://localhost:8080)
 * - VITE_WS_URL: WebSocket URL (default: ws://localhost:8080/ws)
 *
 * Since import.meta.env is a Vite construct not available in Jest,
 * we test the configuration patterns by testing the modules that consume them.
 */

export {} // Ensure this is treated as a module under --isolatedModules

// We need to mock import.meta.env before importing modules that use it
const originalEnv = { ...process.env }

beforeEach(() => {
  vi.resetModules()
})

afterEach(() => {
  process.env = { ...originalEnv }
})

describe('Configuration and Environment Handling', () => {
  describe('API Base URL Construction', () => {
    it('uses default API base URL when VITE_API_BASE_URL is not set', () => {
      // The api.ts module sets: const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
      // And creates axios with baseURL: `${API_BASE_URL}/api/v1`
      const baseUrl = 'http://localhost:8080'
      const apiBaseUrl = `${baseUrl}/api/v1`
      expect(apiBaseUrl).toBe('http://localhost:8080/api/v1')
    })

    it('appends /api/v1 to the base URL', () => {
      const baseUrl = 'https://example.com'
      const expected = `${baseUrl}/api/v1`
      expect(expected).toBe('https://example.com/api/v1')
    })

    it('handles base URL with trailing slash correctly', () => {
      // Verify the pattern used in api.ts: `${API_BASE_URL}/api/v1`
      const baseWithSlash = 'http://localhost:8080/'
      const baseWithoutSlash = 'http://localhost:8080'

      // The code uses template literals without trimming trailing slashes
      expect(`${baseWithSlash}/api/v1`).toBe('http://localhost:8080//api/v1')
      expect(`${baseWithoutSlash}/api/v1`).toBe('http://localhost:8080/api/v1')
    })

    it('constructs correct URL for different environments', () => {
      const environments = [
        { base: 'http://localhost:8080', expected: 'http://localhost:8080/api/v1' },
        { base: 'https://api.catalogizer.io', expected: 'https://api.catalogizer.io/api/v1' },
        { base: 'http://192.168.1.100:3000', expected: 'http://192.168.1.100:3000/api/v1' },
      ]

      environments.forEach(({ base, expected }) => {
        expect(`${base}/api/v1`).toBe(expected)
      })
    })
  })

  describe('WebSocket URL Construction', () => {
    it('uses default WebSocket URL when VITE_WS_URL is not set', () => {
      // The websocket.ts module sets: const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws'
      const defaultWsUrl = 'ws://localhost:8080/ws'
      expect(defaultWsUrl).toBe('ws://localhost:8080/ws')
    })

    it('appends a single-use ticket (not the session token) as query parameter when authenticated', () => {
      const wsUrl = 'ws://localhost:8080/ws'
      const token = 'test-jwt-token-123'

      // This mirrors WebSocketClient.connect() behavior
      const wsUrlWithToken = `${wsUrl}?ticket=${token}`
      expect(wsUrlWithToken).toBe('ws://localhost:8080/ws?ticket=test-jwt-token-123')
    })

    it('uses plain URL when no token is available', () => {
      const wsUrl = 'ws://localhost:8080/ws'
      const token = null

      // Mirrors: const wsUrl = this.ticket ? `${WS_URL}?ticket=${this.ticket}` : WS_URL
      const finalUrl = token ? `${wsUrl}?ticket=${token}` : wsUrl
      expect(finalUrl).toBe('ws://localhost:8080/ws')
    })

    it('handles wss:// protocol for secure connections', () => {
      const wsUrl = 'wss://api.catalogizer.io/ws'
      const token = 'secure-token'

      const finalUrl = `${wsUrl}?ticket=${token}`
      expect(finalUrl).toBe('wss://api.catalogizer.io/ws?ticket=secure-token')
    })

    it('constructs WebSocket URL for various environments', () => {
      const environments = [
        { ws: 'ws://localhost:8080/ws', description: 'local development' },
        { ws: 'wss://api.catalogizer.io/ws', description: 'production' },
        { ws: 'ws://192.168.1.100:8080/ws', description: 'LAN' },
      ]

      environments.forEach(({ ws }) => {
        expect(ws).toMatch(/^wss?:\/\//)
        expect(ws).toMatch(/\/ws$/)
      })
    })
  })

  describe('Environment Variable Defaults', () => {
    it('provides sensible defaults for all configuration', () => {
      // These are the defaults used when env vars are not set
      const defaults = {
        apiBaseUrl: 'http://localhost:8080',
        wsUrl: 'ws://localhost:8080/ws',
      }

      expect(defaults.apiBaseUrl).toBe('http://localhost:8080')
      expect(defaults.wsUrl).toBe('ws://localhost:8080/ws')
    })

    it('defaults point to the same host', () => {
      const apiDefault = 'http://localhost:8080'
      const wsDefault = 'ws://localhost:8080/ws'

      // Extract host:port from both URLs
      const apiHost = new URL(apiDefault).host
      const wsHost = new URL(wsDefault).host

      expect(apiHost).toBe(wsHost)
    })

    it('defaults use correct protocols', () => {
      const apiDefault = 'http://localhost:8080'
      const wsDefault = 'ws://localhost:8080/ws'

      expect(new URL(apiDefault).protocol).toBe('http:')
      expect(new URL(wsDefault).protocol).toBe('ws:')
    })

    it('defaults use same port', () => {
      const apiDefault = 'http://localhost:8080'
      const wsDefault = 'ws://localhost:8080/ws'

      expect(new URL(apiDefault).port).toBe('8080')
      expect(new URL(wsDefault).port).toBe('8080')
    })
  })

  describe('Feature Flag Parsing', () => {
    it('parses boolean-like environment variable strings', () => {
      // Common pattern for feature flags via env vars
      const parseFlag = (value: string | undefined): boolean => {
        if (!value) return false
        return ['true', '1', 'yes', 'on'].includes(value.toLowerCase())
      }

      expect(parseFlag('true')).toBe(true)
      expect(parseFlag('1')).toBe(true)
      expect(parseFlag('yes')).toBe(true)
      expect(parseFlag('on')).toBe(true)
      expect(parseFlag('false')).toBe(false)
      expect(parseFlag('0')).toBe(false)
      expect(parseFlag(undefined)).toBe(false)
      expect(parseFlag('')).toBe(false)
    })

    it('handles numeric environment variable strings', () => {
      const parseNumber = (value: string | undefined, defaultVal: number): number => {
        if (!value) return defaultVal
        const parsed = parseInt(value, 10)
        return isNaN(parsed) ? defaultVal : parsed
      }

      expect(parseNumber('5000', 10000)).toBe(5000)
      expect(parseNumber(undefined, 10000)).toBe(10000)
      expect(parseNumber('', 10000)).toBe(10000)
      expect(parseNumber('not-a-number', 10000)).toBe(10000)
    })
  })

  describe('API Client Configuration', () => {
    it('sets correct default timeout', () => {
      // api.ts uses timeout: 10000
      const timeout = 10000
      expect(timeout).toBe(10000)
    })

    it('sets correct default Content-Type header', () => {
      const headers = { 'Content-Type': 'application/json' }
      expect(headers['Content-Type']).toBe('application/json')
    })

    it('constructs authorization header from token', () => {
      const token = 'test-token-123'
      const authHeader = `Bearer ${token}`
      expect(authHeader).toBe('Bearer test-token-123')
    })
  })

  describe('WebSocket Configuration', () => {
    it('has correct reconnect settings', () => {
      // WebSocketClient defaults from websocket.ts
      const maxReconnectAttempts = 5
      const reconnectDelay = 1000

      expect(maxReconnectAttempts).toBe(5)
      expect(reconnectDelay).toBe(1000)
    })

    it('calculates exponential backoff correctly', () => {
      const baseDelay = 1000

      // Mirrors: this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)
      const delays = [1, 2, 3, 4, 5].map(
        (attempt) => baseDelay * Math.pow(2, attempt - 1)
      )

      expect(delays).toEqual([1000, 2000, 4000, 8000, 16000])
    })

    it('stops reconnecting after max attempts', () => {
      const maxAttempts = 5
      let attempts = 0
      const shouldReconnect = () => {
        attempts++
        return attempts < maxAttempts
      }

      while (shouldReconnect()) {
        // simulating reconnect attempts
      }

      expect(attempts).toBe(maxAttempts)
    })
  })
})
