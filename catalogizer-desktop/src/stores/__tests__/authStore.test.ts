import { describe, it, expect, vi, beforeEach } from 'vitest'
import { invoke } from '@tauri-apps/api/core'
import { useAuthStore } from '../authStore'

const mockInvoke = vi.mocked(invoke)

describe('authStore', () => {
  beforeEach(() => {
    // Reset store state between tests
    useAuthStore.setState({
      user: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,
      serverUrl: null,
    })
  })

  describe('initial state', () => {
    it('has correct default values', () => {
      const state = useAuthStore.getState()

      expect(state.user).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.isLoading).toBe(false)
      expect(state.error).toBeNull()
      expect(state.serverUrl).toBeNull()
    })
  })

  describe('setServerUrl', () => {
    it('sets the server URL', () => {
      useAuthStore.getState().setServerUrl('http://localhost:8080')

      expect(useAuthStore.getState().serverUrl).toBe('http://localhost:8080')
    })

    it('overwrites a previously set URL', () => {
      useAuthStore.getState().setServerUrl('http://old:8080')
      useAuthStore.getState().setServerUrl('http://new:9090')

      expect(useAuthStore.getState().serverUrl).toBe('http://new:9090')
    })
  })

  describe('setAuthToken', () => {
    it('sets isAuthenticated to true', () => {
      useAuthStore.getState().setAuthToken('some-token')

      expect(useAuthStore.getState().isAuthenticated).toBe(true)
    })

    it('does not modify user or error', () => {
      useAuthStore.getState().setAuthToken('some-token')

      expect(useAuthStore.getState().user).toBeNull()
      expect(useAuthStore.getState().error).toBeNull()
    })
  })

  describe('clearAuth', () => {
    it('resets user, isAuthenticated, and error', () => {
      useAuthStore.setState({
        user: { id: 1, username: 'test', email: 'test@test.com', first_name: 'Test', last_name: 'User', is_admin: false, created_at: '', updated_at: '' },
        isAuthenticated: true,
        error: 'some error',
      })

      useAuthStore.getState().clearAuth()

      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.error).toBeNull()
    })

    it('preserves serverUrl', () => {
      useAuthStore.setState({ serverUrl: 'http://localhost:8080', isAuthenticated: true })

      useAuthStore.getState().clearAuth()

      expect(useAuthStore.getState().serverUrl).toBe('http://localhost:8080')
    })
  })

  describe('login server contract (CAT-TAURI-1)', () => {
    const user = { id: 1, username: 'qa', email: 'q@x', first_name: 'Q', last_name: 'A', is_admin: true, created_at: '', updated_at: '' }

    it('uses the server_url from the Tauri config when the store has none, on /api/v1', async () => {
      useAuthStore.setState({ serverUrl: null })
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return { server_url: 'http://qa-host:38082', auth_token: null } as any
        if (cmd === 'make_http_request') return JSON.stringify({ token: 't', user })
        return undefined as any
      })

      await useAuthStore.getState().login('qa', 'pw')

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://qa-host:38082/api/v1/auth/login',
      }))
    })

    it('accepts the session_token field returned by catalog-api', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return { server_url: 'http://qa-host:38082', auth_token: null } as any
        if (cmd === 'make_http_request') return JSON.stringify({ session_token: 'sess-1', refresh_token: 'r', user })
        return undefined as any
      })

      await useAuthStore.getState().login('qa', 'pw')

      expect(mockInvoke).toHaveBeenCalledWith('set_auth_token', { token: 'sess-1' })
      expect(useAuthStore.getState().isAuthenticated).toBe(true)
    })
  })

  describe('login', () => {
    const mockUser = {
      id: 1,
      username: 'admin',
      email: 'admin@test.com',
      first_name: 'Admin',
      last_name: 'User',
      is_admin: true,
      created_at: '2023-01-01',
      updated_at: '2023-01-01',
    }

    beforeEach(() => {
      useAuthStore.setState({ serverUrl: 'http://localhost:8080' })
    })

    it('sets isLoading to true and clears error at start', async () => {
      let capturedLoading = false
      let capturedError: string | null = 'not-cleared'

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'make_http_request') {
          capturedLoading = useAuthStore.getState().isLoading
          capturedError = useAuthStore.getState().error
          return JSON.stringify({ token: 'abc', user: mockUser })
        }
        return undefined as any
      })

      await useAuthStore.getState().login('admin', 'password')

      expect(capturedLoading).toBe(true)
      expect(capturedError).toBeNull()
    })

    it('calls make_http_request with correct arguments', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'make_http_request') {
          return JSON.stringify({ token: 'abc', user: mockUser })
        }
        return undefined as any
      })

      await useAuthStore.getState().login('admin', 'password')

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', {
        url: 'http://localhost:8080/api/v1/auth/login',
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password: 'password' }),
      })
    })

    it('stores auth token via invoke after successful login', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'make_http_request') {
          return JSON.stringify({ token: 'my-jwt-token', user: mockUser })
        }
        return undefined as any
      })

      await useAuthStore.getState().login('admin', 'password')

      expect(mockInvoke).toHaveBeenCalledWith('set_auth_token', { token: 'my-jwt-token' })
    })

    it('sets user, isAuthenticated, and clears loading on success', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'make_http_request') {
          return JSON.stringify({ token: 'abc', user: mockUser })
        }
        return undefined as any
      })

      await useAuthStore.getState().login('admin', 'password')

      const state = useAuthStore.getState()
      expect(state.user).toEqual(mockUser)
      expect(state.isAuthenticated).toBe(true)
      expect(state.isLoading).toBe(false)
      expect(state.error).toBeNull()
    })

    it('sets error and throws on failure', async () => {
      mockInvoke.mockRejectedValue(new Error('Invalid credentials'))

      await expect(
        useAuthStore.getState().login('admin', 'wrong')
      ).rejects.toThrow('Invalid credentials')

      const state = useAuthStore.getState()
      expect(state.isAuthenticated).toBe(false)
      expect(state.isLoading).toBe(false)
      expect(state.error).toBe('Invalid credentials')
    })

    it('sets generic error message for non-Error throws', async () => {
      mockInvoke.mockRejectedValue('some string error')

      await expect(
        useAuthStore.getState().login('admin', 'wrong')
      ).rejects.toBe('some string error')

      expect(useAuthStore.getState().error).toBe('Login failed')
    })
  })

  describe('logout', () => {
    beforeEach(() => {
      useAuthStore.setState({
        user: { id: 1, username: 'admin', email: 'a@b.com', first_name: 'A', last_name: 'B', is_admin: false, created_at: '', updated_at: '' },
        isAuthenticated: true,
        serverUrl: 'http://localhost:8080',
      })
    })

    it('calls clear_auth_token', async () => {
      mockInvoke.mockResolvedValue(undefined as any)

      await useAuthStore.getState().logout()

      expect(mockInvoke).toHaveBeenCalledWith('clear_auth_token')
    })

    it('attempts to call the logout endpoint', async () => {
      mockInvoke.mockResolvedValue(undefined as any)

      await useAuthStore.getState().logout()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', {
        url: 'http://localhost:8080/api/v1/auth/logout',
        method: 'POST',
        headers: {},
      })
    })

    it('clears user and isAuthenticated on success', async () => {
      mockInvoke.mockResolvedValue(undefined as any)

      await useAuthStore.getState().logout()

      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.isAuthenticated).toBe(false)
      expect(state.error).toBeNull()
    })

    it('still clears state when the logout endpoint fails', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'make_http_request') {
          throw new Error('Network error')
        }
        return undefined as any
      })

      await useAuthStore.getState().logout()

      const state = useAuthStore.getState()
      expect(state.user).toBeNull()
      expect(state.isAuthenticated).toBe(false)
    })

    it('does not throw when clear_auth_token fails', async () => {
      mockInvoke.mockRejectedValue(new Error('Storage error'))

      // Should not throw
      await useAuthStore.getState().logout()
    })
  })

  describe('checkAuthStatus', () => {
    it('sets isAuthenticated to false when config has no auth_token', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') {
          return { server_url: 'http://localhost:8080' }
        }
        return undefined as any
      })

      await useAuthStore.getState().checkAuthStatus()

      expect(useAuthStore.getState().isAuthenticated).toBe(false)
    })

    it('sets isAuthenticated to false when config has no server_url', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') {
          return { auth_token: 'some-token' }
        }
        return undefined as any
      })

      await useAuthStore.getState().checkAuthStatus()

      expect(useAuthStore.getState().isAuthenticated).toBe(false)
    })

    it('sets authenticated state when server confirms token is valid', async () => {
      const mockUser = {
        id: 1, username: 'admin', email: 'a@b.com',
        first_name: 'A', last_name: 'B', is_admin: true,
        created_at: '', updated_at: '',
      }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') {
          return { server_url: 'http://localhost:8080', auth_token: 'valid-token' }
        }
        if (cmd === 'make_http_request') {
          return JSON.stringify({ authenticated: true, user: mockUser })
        }
        return undefined as any
      })

      await useAuthStore.getState().checkAuthStatus()

      const state = useAuthStore.getState()
      expect(state.isAuthenticated).toBe(true)
      expect(state.user).toEqual(mockUser)
      expect(state.error).toBeNull()
    })

    it('calls the auth status endpoint with Bearer token', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') {
          return { server_url: 'http://localhost:8080', auth_token: 'my-token' }
        }
        if (cmd === 'make_http_request') {
          return JSON.stringify({ authenticated: true, user: {} })
        }
        return undefined as any
      })

      await useAuthStore.getState().checkAuthStatus()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', {
        url: 'http://localhost:8080/api/v1/auth/status',
        method: 'GET',
        headers: { Authorization: 'Bearer my-token' },
      })
    })

    it('clears auth and token when server says not authenticated', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') {
          return { server_url: 'http://localhost:8080', auth_token: 'expired-token' }
        }
        if (cmd === 'make_http_request') {
          return JSON.stringify({ authenticated: false })
        }
        return undefined as any
      })

      await useAuthStore.getState().checkAuthStatus()

      expect(useAuthStore.getState().isAuthenticated).toBe(false)
      expect(mockInvoke).toHaveBeenCalledWith('clear_auth_token')
    })

    it('clears auth and token when the status request fails', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') {
          return { server_url: 'http://localhost:8080', auth_token: 'valid-token' }
        }
        if (cmd === 'make_http_request') {
          throw new Error('Network error')
        }
        return undefined as any
      })

      await useAuthStore.getState().checkAuthStatus()

      expect(useAuthStore.getState().isAuthenticated).toBe(false)
      expect(mockInvoke).toHaveBeenCalledWith('clear_auth_token')
    })
  })
})
