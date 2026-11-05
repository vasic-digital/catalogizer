import { describe, it, expect, vi, beforeEach } from 'vitest'
import { invoke } from '@tauri-apps/api/core'
import { apiService } from '../apiService'

const mockInvoke = vi.mocked(invoke)

describe('apiService (extended edge cases)', () => {
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

  describe('searchMedia edge cases', () => {
    it('handles zero offset correctly', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia({ offset: 0 })

      const call = mockInvoke.mock.calls.find(c => c[0] === 'make_http_request')
      const url = (call?.[1] as any)?.url as string
      expect(url).toContain('offset=0')
    })

    it('handles multiple search params together', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 10, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia({
        query: 'action',
        media_type: 'movie',
        year_min: 2020,
        year_max: 2025,
        rating_min: 7.0,
        quality: '4k',
        sort_by: 'rating',
        sort_order: 'desc',
        limit: 10,
        offset: 20,
      })

      const call = mockInvoke.mock.calls.find(c => c[0] === 'make_http_request')
      const url = (call?.[1] as any)?.url as string
      expect(url).toContain('query=action')
      expect(url).toContain('media_type=movie')
      expect(url).toContain('year_min=2020')
      expect(url).toContain('year_max=2025')
      expect(url).toContain('rating_min=7')
      expect(url).toContain('quality=4k')
      expect(url).toContain('sort_by=rating')
      expect(url).toContain('sort_order=desc')
      expect(url).toContain('limit=10')
      expect(url).toContain('offset=20')
    })

    it('excludes null params from search query', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia({ query: null as any, media_type: 'movie' })

      const call = mockInvoke.mock.calls.find(c => c[0] === 'make_http_request')
      const url = (call?.[1] as any)?.url as string
      expect(url).not.toContain('query=')
      expect(url).toContain('media_type=movie')
    })

    it('constructs correct base URL without params', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ items: [], total: 0, limit: 50, offset: 0 }) as any
        return undefined as any
      })

      await apiService.searchMedia({})

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/search',
      }))
    })
  })

  describe('makeRequest error handling', () => {
    it('propagates invoke errors', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') throw new Error('Network timeout')
        return undefined as any
      })

      await expect(apiService.healthCheck()).rejects.toThrow('Network timeout')
    })

    it('rejects when config fetch fails', async () => {
      mockInvoke.mockRejectedValue(new Error('Storage corrupted'))

      await expect(apiService.healthCheck()).rejects.toThrow('Storage corrupted')
    })

    it('throws on empty string response', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '' as any
        return undefined as any
      })

      await expect(apiService.healthCheck()).rejects.toThrow('Invalid response format')
    })
  })

  describe('URL construction', () => {
    it('constructs correct URL for getMediaById', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ id: 99, title: 'Film' }) as any
        return undefined as any
      })

      await apiService.getMediaById(99)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/99',
      }))
    })

    it('constructs correct URL for toggleFavorite', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.toggleFavorite(42)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/42/favorite',
        method: 'POST',
      }))
    })

    it('constructs correct URL for updateWatchProgress', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      const progress = { media_id: 10, position: 60, duration: 120, timestamp: 1234567890 }
      await apiService.updateWatchProgress(10, progress)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/media/10/progress',
        method: 'PUT',
        body: JSON.stringify(progress),
      }))
    })
  })

  describe('server URL handling', () => {
    it('works with server URL that has trailing path', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return { ...defaultConfig, server_url: 'http://localhost:8080' } as any
        if (cmd === 'make_http_request') return JSON.stringify({ status: 'ok' }) as any
        return undefined as any
      })

      await apiService.healthCheck()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/health',
      }))
    })

    it('throws when server_url is empty string', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return { ...defaultConfig, server_url: '' } as any
        return undefined as any
      })

      await expect(apiService.healthCheck()).rejects.toThrow('Server URL not configured')
    })
  })

  describe('getSystemInfo', () => {
    it('resolves all three system info calls in parallel', async () => {
      const callOrder: string[] = []
      mockInvoke.mockImplementation(async (cmd) => {
        callOrder.push(cmd)
        if (cmd === 'get_app_version') return '2.0.0' as any
        if (cmd === 'get_platform') return 'darwin' as any
        if (cmd === 'get_arch') return 'arm64' as any
        return undefined as any
      })

      const result = await apiService.getSystemInfo()

      expect(result).toEqual({
        version: '2.0.0',
        platform: 'darwin',
        arch: 'arm64',
      })
      expect(callOrder).toContain('get_app_version')
      expect(callOrder).toContain('get_platform')
      expect(callOrder).toContain('get_arch')
    })

    it('rejects when get_arch fails', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_app_version') return '1.0.0' as any
        if (cmd === 'get_platform') return 'linux' as any
        if (cmd === 'get_arch') throw new Error('Arch detection failed')
        return undefined as any
      })

      await expect(apiService.getSystemInfo()).rejects.toThrow('Arch detection failed')
    })

    it('rejects when get_app_version fails', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_app_version') throw new Error('Version not found')
        if (cmd === 'get_platform') return 'linux' as any
        if (cmd === 'get_arch') return 'x86_64' as any
        return undefined as any
      })

      await expect(apiService.getSystemInfo()).rejects.toThrow('Version not found')
    })
  })

  describe('SMB endpoints edge cases', () => {
    it('getSMBStatus without configId hits base endpoint', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '[]' as any
        return undefined as any
      })

      await apiService.getSMBStatus()

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/status',
      }))
    })

    it('getSMBStatus with configId 0 hits base endpoint', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '[]' as any
        return undefined as any
      })

      await apiService.getSMBStatus(0)

      // 0 is falsy, so it should hit the base endpoint
      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/status',
      }))
    })

    it('deleteSMBConfig uses DELETE method', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return '{}' as any
        return undefined as any
      })

      await apiService.deleteSMBConfig(5)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/configs/5',
        method: 'DELETE',
      }))
    })

    it('updateSMBConfig passes partial config body', async () => {
      mockInvoke.mockImplementation(async (cmd) => {
        if (cmd === 'get_config') return defaultConfig as any
        if (cmd === 'make_http_request') return JSON.stringify({ id: 1 }) as any
        return undefined as any
      })

      const partialUpdate = { name: 'New Name', host: '10.0.0.1' }
      await apiService.updateSMBConfig(1, partialUpdate)

      expect(mockInvoke).toHaveBeenCalledWith('make_http_request', expect.objectContaining({
        url: 'http://localhost:8080/api/v1/smb/configs/1',
        method: 'PUT',
        body: JSON.stringify(partialUpdate),
      }))
    })
  })
})
