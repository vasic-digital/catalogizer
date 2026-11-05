import { describe, it, expect, vi, beforeEach } from 'vitest'
import { invoke } from '@tauri-apps/api/core'
import { apiService } from '../apiService'

const mockInvoke = vi.mocked(invoke)

describe('apiService', () => {
  const defaultConfig = {
    server_url: 'http://localhost:8080',
    auth_token: 'test-token',
    theme: 'dark',
    auto_start: false,
  }

  beforeEach(() => {
    mockInvoke.mockImplementation(async (cmd, args?: any) => {
      if (cmd === 'get_config') {
        return defaultConfig as any
      }
      if (cmd === 'make_http_request') {
        return '{}' as any
      }
      return undefined as any
    })
  })

  describe('entity endpoints (CAT-TAURI-1)', () => {
    it('requests entity progress at /api/v1/entities/:id/progress (no doubled v1)', async () => {
      await apiService.getEntityProgress(7)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/entities/7/progress',
      }))
    })
  })

  describe('makeRequest (via public methods)', () => {
    it('throws when server URL is not configured', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return { theme: 'dark', auto_start: false } as any
        return undefined as any
      })

      await expect(apiService.healthCheck()).rejects.toThrow('Server URL not configured')
    })

    it('includes auth token in Authorization header when available', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ status: 'ok' }) as any
        return undefined as any
      })

      await apiService.healthCheck()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer test-token',
        }),
      }))
    })

    it('does not include Authorization header when no token', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return { server_url: 'http://localhost:8080', theme: 'dark', auto_start: false } as any
        if (cmd === 'make_http_request') return JSON.stringify({ status: 'ok' }) as any
        return undefined as any
      })

      await apiService.healthCheck()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        headers: expect.not.objectContaining({
          Authorization: expect.any(String),
        }),
      }))
    })

    it('throws on invalid JSON response', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return 'not-json' as any
        return undefined as any
      })

      await expect(apiService.healthCheck()).rejects.toThrow('Invalid response format')
    })

    it('always includes Content-Type: application/json header', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.healthCheck()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
        }),
      }))
    })
  })

  describe('login', () => {
    it('sends POST to /api/auth/login with credentials', async () => {
      const loginResponse = {
        token: 'jwt-token',
        refresh_token: 'refresh',
        expires_in: 3600,
        user: { id: 1, username: 'admin' },
      }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(loginResponse) as any
        return undefined as any
      })

      const result = await apiService.login({ username: 'admin', password: 'pass' })

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/auth/login',
        method: 'POST',
        body: JSON.stringify({ username: 'admin', password: 'pass' }),
      }))
      expect(result.token).toBe('jwt-token')
    })
  })

  describe('logout', () => {
    it('sends POST to /api/auth/logout', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.logout()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/auth/logout',
        method: 'POST',
      }))
    })
  })

  describe('getAuthStatus', () => {
    it('sends GET to /api/auth/status', async () => {
      const statusResponse = { authenticated: true, user: { id: 1 } }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(statusResponse) as any
        return undefined as any
      })

      const result = await apiService.getAuthStatus()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/auth/status',
        method: 'GET',
      }))
      expect(result.authenticated).toBe(true)
    })
  })

  describe('getProfile', () => {
    it('sends GET to /api/auth/profile', async () => {
      const user = { id: 1, username: 'admin', email: 'a@b.com' }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(user) as any
        return undefined as any
      })

      const result = await apiService.getProfile()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/auth/profile',
        method: 'GET',
      }))
      expect(result.username).toBe('admin')
    })
  })

  describe('searchMedia', () => {
    it('sends GET to /api/media/search with no params by default', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/search',
        method: 'GET',
      }))
    })

    it('appends query parameters when provided', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 10, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia({ query: 'test', media_type: 'movie', limit: 10 })

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: expect.stringContaining('query=test'),
      }))
      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: expect.stringContaining('media_type=movie'),
      }))
      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: expect.stringContaining('limit=10'),
      }))
    })

    it('excludes undefined and null params', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia({ query: 'test', media_type: undefined })

      const call = mockInvoke.mock.calls.find(
        c => c[0] === 'make_http_request'
      )
      const url = (call?.[1] as any)?.url as string
      expect(url).not.toContain('media_type')
    })

    it('returns parsed response', async () => {
      const response = { items: [{ id: 1, title: 'Movie' }], total: 1, limit: 50, offset: 0 }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(response) as any
        return undefined as any
      })

      const result = await apiService.searchMedia({ query: 'Movie' })

      expect(result.items).toHaveLength(1)
      expect(result.items[0].title).toBe('Movie')
      expect(result.total).toBe(1)
    })
  })

  describe('getMediaById', () => {
    it('sends GET to /api/media/:id', async () => {
      const media = { id: 42, title: 'Test Film' }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(media) as any
        return undefined as any
      })

      const result = await apiService.getMediaById(42)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/42',
        method: 'GET',
      }))
      expect(result.title).toBe('Test Film')
    })
  })

  describe('getMediaStats', () => {
    it('sends GET to /api/media/stats', async () => {
      const stats = { total_items: 100, by_type: {}, by_quality: {}, total_size: 0, recent_additions: 5 }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(stats) as any
        return undefined as any
      })

      const result = await apiService.getMediaStats()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/stats',
      }))
      expect(result.total_items).toBe(100)
    })
  })

  describe('updateWatchProgress', () => {
    it('sends PUT to /api/media/:id/progress with body', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      const progress = { media_id: 1, position: 120, duration: 7200, timestamp: Date.now() }
      await apiService.updateWatchProgress(1, progress)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/1/progress',
        method: 'PUT',
        body: JSON.stringify(progress),
      }))
    })
  })

  describe('toggleFavorite', () => {
    it('sends POST to /api/media/:id/favorite', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.toggleFavorite(5)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/5/favorite',
        method: 'POST',
      }))
    })
  })

  describe('getMediaUrl', () => {
    it('sends GET to /api/media/:id/stream', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ url: 'http://stream/1' }) as any
        return undefined as any
      })

      const result = await apiService.getMediaUrl(1)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/1/stream',
      }))
      expect(result.url).toBe('http://stream/1')
    })
  })

  describe('downloadMedia', () => {
    it('sends POST to /api/media/:id/download', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ job_id: 99 }) as any
        return undefined as any
      })

      const result = await apiService.downloadMedia(7)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/7/download',
        method: 'POST',
      }))
      expect(result.job_id).toBe(99)
    })
  })

  describe('SMB endpoints', () => {
    beforeEach(() => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '[]' as any
        return undefined as any
      })
    })

    it('getSMBConfigs sends GET to /api/smb/configs', async () => {
      await apiService.getSMBConfigs()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/configs',
        method: 'GET',
      }))
    })

    it('createSMBConfig sends POST to /api/smb/configs with body', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ id: 1 }) as any
        return undefined as any
      })

      const config = {
        name: 'test', host: '192.168.1.1', port: 445,
        share_name: 'media', username: 'user', password: 'pass',
        is_active: true, mount_point: '/mnt/media',
      }

      await apiService.createSMBConfig(config)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/configs',
        method: 'POST',
        body: JSON.stringify(config),
      }))
    })

    it('updateSMBConfig sends PUT to /api/smb/configs/:id', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ id: 1 }) as any
        return undefined as any
      })

      await apiService.updateSMBConfig(1, { name: 'updated' })

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/configs/1',
        method: 'PUT',
        body: JSON.stringify({ name: 'updated' }),
      }))
    })

    it('deleteSMBConfig sends DELETE to /api/smb/configs/:id', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.deleteSMBConfig(3)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/configs/3',
        method: 'DELETE',
      }))
    })

    it('getSMBStatus sends GET to /api/smb/status without configId', async () => {
      await apiService.getSMBStatus()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/status',
      }))
    })

    it('getSMBStatus sends GET to /api/smb/status/:id with configId', async () => {
      await apiService.getSMBStatus(2)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/status/2',
      }))
    })

    it('connectSMB sends POST to /api/smb/connect/:id', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.connectSMB(1)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/connect/1',
        method: 'POST',
      }))
    })

    it('disconnectSMB sends POST to /api/smb/disconnect/:id', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.disconnectSMB(1)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/disconnect/1',
        method: 'POST',
      }))
    })

    it('scanSMB sends POST to /api/smb/scan/:id', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ job_id: 10 }) as any
        return undefined as any
      })

      const result = await apiService.scanSMB(1)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/scan/1',
        method: 'POST',
      }))
      expect(result.job_id).toBe(10)
    })
  })

  describe('getSystemInfo', () => {
    it('calls three invoke commands and returns combined result', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_app_version') return '1.0.0' as any
        if (cmd === 'get_platform') return 'linux' as any
        if (cmd === 'get_arch') return 'x86_64' as any
        return undefined as any
      })

      const result = await apiService.getSystemInfo()

      expect(mockInvoke).toHaveBeenCalledWith('get_app_version')
      expect(mockInvoke).toHaveBeenCalledWith('get_platform')
      expect(mockInvoke).toHaveBeenCalledWith('get_arch')
      expect(result).toEqual({
        version: '1.0.0',
        platform: 'linux',
        arch: 'x86_64',
      })
    })

    it('throws when any invoke call fails', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_app_version') return '1.0.0' as any
        if (cmd === 'get_platform') throw new Error('Not available')
        if (cmd === 'get_arch') return 'x86_64' as any
        return undefined as any
      })

      await expect(apiService.getSystemInfo()).rejects.toThrow('Not available')
    })
  })

  describe('healthCheck', () => {
    it('sends GET to /api/health', async () => {
      const response = { status: 'healthy', timestamp: '2023-01-01T00:00:00Z' }

      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify(response) as any
        return undefined as any
      })

      const result = await apiService.healthCheck()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/health',
        method: 'GET',
      }))
      expect(result.status).toBe('healthy')
    })
  })
})
