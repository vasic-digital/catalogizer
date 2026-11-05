import { smbApi } from '../smbApi'
import apiDefault from '../api'

vi.mock('../api', async () => {
  const mockApi = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  }
  return {
    __esModule: true,
    default: mockApi,
    api: mockApi,
  }
})

const mockApi = vi.mocked(apiDefault)

describe('smbApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('discover', () => {
    it('calls POST /smb/discover and returns shares', async () => {
      const mockShares = [
        { name: 'media', path: '/media', host: '192.168.1.100' },
        { name: 'backup', path: '/backup', host: '192.168.1.100' },
      ]
      mockApi.post.mockResolvedValue({ data: mockShares })

      const result = await smbApi.discover()

      expect(mockApi.post).toHaveBeenCalledWith('/smb/discover')
      expect(result).toEqual(mockShares)
    })

    it('propagates errors on discovery failure', async () => {
      mockApi.post.mockRejectedValue(new Error('Discovery failed'))

      await expect(smbApi.discover()).rejects.toThrow('Discovery failed')
    })
  })

  describe('testConnection', () => {
    it('calls POST /smb/test with credentials and returns result', async () => {
      const request = { host: '192.168.1.100', share: 'media', username: 'user', password: 'pass' }
      const mockResult = { success: true, message: 'Connected', latency_ms: 45 }
      mockApi.post.mockResolvedValue({ data: mockResult })

      const result = await smbApi.testConnection(request)

      expect(mockApi.post).toHaveBeenCalledWith('/smb/test', request)
      expect(result).toEqual(mockResult)
    })

    it('returns failure result for bad credentials', async () => {
      const request = { host: '192.168.1.100', share: 'media' }
      const mockResult = { success: false, message: 'Access denied' }
      mockApi.post.mockResolvedValue({ data: mockResult })

      const result = await smbApi.testConnection(request)

      expect(result.success).toBe(false)
      expect(result.message).toBe('Access denied')
    })

    it('propagates errors on test failure', async () => {
      mockApi.post.mockRejectedValue(new Error('Network error'))

      await expect(smbApi.testConnection({ host: 'bad', share: 'x' })).rejects.toThrow('Network error')
    })
  })

  describe('browse', () => {
    it('calls POST /smb/browse with body and returns entries', async () => {
      const request = { host: '192.168.1.100', share: 'media', path: '/movies' }
      const mockEntries = [
        { name: 'movie1.mkv', path: '/movies/movie1.mkv', is_directory: false, size: 1024000 },
        { name: 'series', path: '/movies/series', is_directory: true, size: 0 },
      ]
      mockApi.post.mockResolvedValue({ data: mockEntries })

      const result = await smbApi.browse(request)

      expect(mockApi.post).toHaveBeenCalledWith('/smb/browse', request)
      expect(result).toEqual(mockEntries)
    })

    it('propagates errors on browse failure', async () => {
      mockApi.post.mockRejectedValue(new Error('Browse failed'))

      await expect(smbApi.browse({ host: 'x', share: 'y' })).rejects.toThrow('Browse failed')
    })
  })
})
