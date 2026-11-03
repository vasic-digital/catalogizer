import { mediaApi, entityApi } from '../mediaApi'
import apiDefault from '../api'

// Mock the api module
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

describe('mediaApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('searchMedia', () => {
    it('calls GET /media/search with params and returns data', async () => {
      const mockResponse = {
        data: {
          items: [{ id: 1, title: 'Test Movie' }],
          total: 1,
          limit: 20,
          offset: 0,
        },
      }
      mockApi.get.mockResolvedValue(mockResponse)

      const params = { query: 'test', media_type: 'movie', limit: 20 }
      const result = await mediaApi.searchMedia(params)

      expect(mockApi.get).toHaveBeenCalledWith('/media/search', { params })
      expect(result).toEqual(mockResponse.data)
    })

    it('propagates errors on search failure', async () => {
      const error = new Error('Network error')
      mockApi.get.mockRejectedValue(error)

      await expect(mediaApi.searchMedia({ query: 'test' })).rejects.toThrow('Network error')
    })
  })

  describe('getMediaById', () => {
    it('calls GET /media/:id and returns the media item', async () => {
      const mockItem = { id: 42, title: 'Test Movie', media_type: 'movie' }
      mockApi.get.mockResolvedValue({ data: mockItem })

      const result = await mediaApi.getMediaById(42)

      expect(mockApi.get).toHaveBeenCalledWith('/media/42')
      expect(result).toEqual(mockItem)
    })

    it('propagates 404 error for non-existent media', async () => {
      const error = { response: { status: 404 }, message: 'Not found' }
      mockApi.get.mockRejectedValue(error)

      await expect(mediaApi.getMediaById(999)).rejects.toEqual(error)
    })
  })

  describe('getMediaByPath', () => {
    it('calls GET /media/by-path with path param', async () => {
      const mockItem = { id: 1, title: 'Test', directory_path: '/movies/test' }
      mockApi.get.mockResolvedValue({ data: mockItem })

      const result = await mediaApi.getMediaByPath('/movies/test')

      expect(mockApi.get).toHaveBeenCalledWith('/media/by-path', {
        params: { path: '/movies/test' },
      })
      expect(result).toEqual(mockItem)
    })
  })

  describe('analyzeDirectory', () => {
    it('calls POST /media/analyze with directory path', async () => {
      const mockResponse = { message: 'Analysis started', analysis_id: 'abc123' }
      mockApi.post.mockResolvedValue({ data: mockResponse })

      const result = await mediaApi.analyzeDirectory('/media/movies')

      expect(mockApi.post).toHaveBeenCalledWith('/media/analyze', {
        directory_path: '/media/movies',
      })
      expect(result).toEqual(mockResponse)
    })
  })

  describe('getExternalMetadata', () => {
    it('calls GET /media/:id/metadata', async () => {
      const mockMetadata = [
        { id: 1, provider: 'tmdb', external_id: '12345', title: 'Test' },
      ]
      mockApi.get.mockResolvedValue({ data: mockMetadata })

      const result = await mediaApi.getExternalMetadata(42)

      expect(mockApi.get).toHaveBeenCalledWith('/media/42/metadata')
      expect(result).toEqual(mockMetadata)
    })
  })

  describe('refreshMetadata', () => {
    it('calls POST /media/:id/refresh', async () => {
      mockApi.post.mockResolvedValue({ data: { message: 'Metadata refreshed' } })

      const result = await mediaApi.refreshMetadata(42)

      expect(mockApi.post).toHaveBeenCalledWith('/media/42/refresh')
      expect(result).toEqual({ message: 'Metadata refreshed' })
    })
  })

  describe('getQualityInfo', () => {
    it('calls GET /media/:id/quality', async () => {
      const mockQuality = {
        overall_score: 85,
        resolution: '1080p',
        codec: 'h264',
        file_size: 1500000000,
      }
      mockApi.get.mockResolvedValue({ data: mockQuality })

      const result = await mediaApi.getQualityInfo(42)

      expect(mockApi.get).toHaveBeenCalledWith('/media/42/quality')
      expect(result).toEqual(mockQuality)
    })
  })

  describe('getMediaStats', () => {
    it('calls GET /media/stats and returns statistics', async () => {
      const mockStats = {
        total_items: 100,
        by_type: { movie: 50, tv_show: 30, music: 20 },
        by_quality: { '1080p': 60, '720p': 40 },
        total_size: 500000000000,
        recent_additions: 5,
      }
      mockApi.get.mockResolvedValue({ data: mockStats })

      const result = await mediaApi.getMediaStats()

      expect(mockApi.get).toHaveBeenCalledWith('/media/stats')
      expect(result).toEqual(mockStats)
    })
  })

  describe('getRecentMedia', () => {
    it('calls GET /media/recent with default limit', async () => {
      // wire shape: {items,total,limit,offset}, not a bare array
      mockApi.get.mockResolvedValue({ data: { items: [{ id: 1, title: 'Recent Movie' }], total: 1, limit: 10, offset: 0 } })

      const result = await mediaApi.getRecentMedia()

      expect(mockApi.get).toHaveBeenCalledWith('/media/recent', { params: { limit: 10 } })
      expect(Array.isArray(result)).toBe(true)
      expect(result).toHaveLength(1)
      expect(result[0]).toMatchObject({ id: 1, title: 'Recent Movie' })
    })

    it('calls GET /media/recent with custom limit', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      await mediaApi.getRecentMedia(5)

      expect(mockApi.get).toHaveBeenCalledWith('/media/recent', { params: { limit: 5 } })
    })
  })

  describe('getPopularMedia', () => {
    it('calls GET /media/popular with default limit', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      await mediaApi.getPopularMedia()

      expect(mockApi.get).toHaveBeenCalledWith('/media/popular', { params: { limit: 10 } })
    })

    it('calls GET /media/popular with custom limit', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      await mediaApi.getPopularMedia(25)

      expect(mockApi.get).toHaveBeenCalledWith('/media/popular', { params: { limit: 25 } })
    })
  })

  describe('deleteMedia', () => {
    it('calls DELETE /media/:id', async () => {
      mockApi.delete.mockResolvedValue({})

      await mediaApi.deleteMedia(42)

      expect(mockApi.delete).toHaveBeenCalledWith('/media/42')
    })

    it('propagates errors on delete failure', async () => {
      mockApi.delete.mockRejectedValue(new Error('Forbidden'))

      await expect(mediaApi.deleteMedia(42)).rejects.toThrow('Forbidden')
    })
  })

  describe('updateMedia', () => {
    it('calls PUT /media/:id with partial data', async () => {
      const updatedItem = { id: 42, title: 'Updated Title', media_type: 'movie' }
      mockApi.put.mockResolvedValue({ data: updatedItem })

      const result = await mediaApi.updateMedia(42, { title: 'Updated Title' } as any)

      expect(mockApi.put).toHaveBeenCalledWith('/media/42', { title: 'Updated Title' })
      expect(result).toEqual(updatedItem)
    })
  })

  describe('Storage root management', () => {
    it('getStorageRoots calls GET /storage/roots and extracts data', async () => {
      const mockRoots = [{ id: 1, name: 'Movies', protocol: 'smb' }]
      mockApi.get.mockResolvedValue({ data: { data: mockRoots } })

      const result = await mediaApi.getStorageRoots()

      expect(mockApi.get).toHaveBeenCalledWith('/storage/roots')
      expect(result).toEqual(mockRoots)
    })

    it('getStorageRoot calls GET /storage/roots/:id', async () => {
      const mockRoot = { id: 1, name: 'Movies', protocol: 'smb' }
      mockApi.get.mockResolvedValue({ data: mockRoot })

      const result = await mediaApi.getStorageRoot(1)

      expect(mockApi.get).toHaveBeenCalledWith('/storage/roots/1')
      expect(result).toEqual(mockRoot)
    })

    it('createStorageRoot calls POST /storage/roots', async () => {
      const newRoot = { name: 'New Root', protocol: 'nfs', enabled: true, max_depth: 5, enable_duplicate_detection: false, enable_metadata_extraction: true }
      const created = { ...newRoot, id: 3, created_at: '2024-01-01', updated_at: '2024-01-01' }
      mockApi.post.mockResolvedValue({ data: created })

      const result = await mediaApi.createStorageRoot(newRoot as any)

      expect(mockApi.post).toHaveBeenCalledWith('/storage/roots', newRoot)
      expect(result).toEqual(created)
    })

    it('updateStorageRoot calls PUT /storage/roots/:id', async () => {
      const updates = { name: 'Updated Root' }
      mockApi.put.mockResolvedValue({ data: { id: 1, ...updates } })

      const result = await mediaApi.updateStorageRoot(1, updates as any)

      expect(mockApi.put).toHaveBeenCalledWith('/storage/roots/1', updates)
      expect(result.name).toBe('Updated Root')
    })

    it('deleteStorageRoot calls DELETE /storage/roots/:id', async () => {
      mockApi.delete.mockResolvedValue({})

      await mediaApi.deleteStorageRoot(1)

      expect(mockApi.delete).toHaveBeenCalledWith('/storage/roots/1')
    })

    it('testStorageRoot calls POST /storage/roots/:id/test', async () => {
      mockApi.post.mockResolvedValue({ data: { success: true, message: 'Connection OK' } })

      const result = await mediaApi.testStorageRoot(1)

      expect(mockApi.post).toHaveBeenCalledWith('/storage/roots/1/test')
      expect(result).toEqual({ success: true, message: 'Connection OK' })
    })
  })

  describe('downloadMedia', () => {
    let createObjectURLMock: vi.Mock
    let revokeObjectURLMock: vi.Mock

    beforeEach(() => {
      createObjectURLMock = vi.fn(() => 'blob:http://localhost/fake-url')
      revokeObjectURLMock = vi.fn()
      Object.defineProperty(window, 'URL', {
        value: {
          createObjectURL: createObjectURLMock,
          revokeObjectURL: revokeObjectURLMock,
        },
        writable: true,
      })
    })

    it('downloads a media file and triggers a link click', async () => {
      const blobData = new Blob(['file content'])
      mockApi.get.mockResolvedValue({ data: blobData })

      const clickMock = vi.fn()
      const appendChildMock = vi.spyOn(document.body, 'appendChild').mockImplementation((node) => node as any)
      const removeChildMock = vi.spyOn(document.body, 'removeChild').mockImplementation((node) => node as any)

      // Mock createElement to capture the anchor
      const originalCreateElement = document.createElement.bind(document)
      vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
        const el = originalCreateElement(tag)
        if (tag === 'a') {
          el.click = clickMock
        }
        return el
      })

      const media = {
        id: 1,
        title: 'Test Movie',
        media_type: 'movie',
        directory_path: '/movies/test-movie.mkv',
        storage_root_name: 'main',
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      }

      await mediaApi.downloadMedia(media as any)

      expect(mockApi.get).toHaveBeenCalledWith('/download', {
        params: { path: '/movies/test-movie.mkv', storage: 'main' },
        responseType: 'blob',
      })
      expect(clickMock).toHaveBeenCalled()
      expect(revokeObjectURLMock).toHaveBeenCalled()

      appendChildMock.mockRestore()
      removeChildMock.mockRestore()
      ;(document.createElement as vi.Mock).mockRestore()
    })

    it('extracts filename from directory path', async () => {
      const blobData = new Blob(['data'])
      mockApi.get.mockResolvedValue({ data: blobData })

      let capturedDownloadAttr = ''
      const appendChildMock = vi.spyOn(document.body, 'appendChild').mockImplementation((node) => {
        capturedDownloadAttr = (node as HTMLAnchorElement).getAttribute('download') || ''
        return node as any
      })
      const removeChildMock = vi.spyOn(document.body, 'removeChild').mockImplementation((node) => node as any)

      const media = {
        id: 2,
        title: 'My Song',
        media_type: 'audio',
        directory_path: '/music/artist/album/track01.mp3',
        storage_root_name: 'nas',
      }

      await mediaApi.downloadMedia(media as any)

      expect(capturedDownloadAttr).toBe('track01.mp3')

      appendChildMock.mockRestore()
      removeChildMock.mockRestore()
    })
  })
})

describe('entityApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getEntities', () => {
    it('calls GET /entities with query params', async () => {
      const mockResponse = {
        entities: [{ id: 1, title: 'Movie Entity', type: 'movie' }],
        total: 1,
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const params = { query: 'movie', type: 'movie', limit: 20, offset: 0 }
      const result = await entityApi.getEntities(params)

      expect(mockApi.get).toHaveBeenCalledWith('/entities', { params })
      expect(result).toEqual(mockResponse)
    })

    it('calls GET /entities with no params', async () => {
      mockApi.get.mockResolvedValue({ data: { entities: [], total: 0 } })

      await entityApi.getEntities({})

      expect(mockApi.get).toHaveBeenCalledWith('/entities', { params: {} })
    })
  })

  describe('getEntity', () => {
    it('calls GET /entities/:id and returns entity detail', async () => {
      const mockEntity = { id: 5, title: 'Inception', type: 'movie', year: 2010 }
      mockApi.get.mockResolvedValue({ data: mockEntity })

      const result = await entityApi.getEntity(5)

      expect(mockApi.get).toHaveBeenCalledWith('/entities/5')
      expect(result).toEqual(mockEntity)
    })

    it('propagates errors for non-existent entity', async () => {
      mockApi.get.mockRejectedValue(new Error('Not found'))

      await expect(entityApi.getEntity(999)).rejects.toThrow('Not found')
    })
  })

  describe('getEntityChildren', () => {
    it('calls GET /entities/:id/children with params', async () => {
      const mockResponse = {
        entities: [{ id: 10, title: 'Season 1' }],
        total: 1,
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.getEntityChildren(5, { limit: 10, offset: 0 })

      expect(mockApi.get).toHaveBeenCalledWith('/entities/5/children', {
        params: { limit: 10, offset: 0 },
      })
      expect(result).toEqual(mockResponse)
    })

    it('calls GET /entities/:id/children without params', async () => {
      mockApi.get.mockResolvedValue({ data: { entities: [], total: 0 } })

      await entityApi.getEntityChildren(5)

      expect(mockApi.get).toHaveBeenCalledWith('/entities/5/children', { params: undefined })
    })
  })

  describe('getEntityFiles', () => {
    it('calls GET /entities/:id/files', async () => {
      const mockResponse = {
        files: [{ id: 1, path: '/media/movie.mkv', size: 1000000 }],
        total: 1,
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.getEntityFiles(5)

      expect(mockApi.get).toHaveBeenCalledWith('/entities/5/files')
      expect(result).toEqual(mockResponse)
    })
  })

  describe('getEntityMetadata', () => {
    it('calls GET /entities/:id/metadata', async () => {
      const mockResponse = {
        metadata: [{ provider: 'tmdb', external_id: '12345' }],
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.getEntityMetadata(5)

      expect(mockApi.get).toHaveBeenCalledWith('/entities/5/metadata')
      expect(result).toEqual(mockResponse)
    })
  })

  describe('getEntityDuplicates', () => {
    it('calls GET /entities/:id/duplicates', async () => {
      const mockResponse = {
        duplicates: [{ id: 6, title: 'Inception (Duplicate)' }],
        total: 1,
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.getEntityDuplicates(5)

      expect(mockApi.get).toHaveBeenCalledWith('/entities/5/duplicates')
      expect(result).toEqual(mockResponse)
    })
  })

  describe('getEntityTypes', () => {
    it('calls GET /entities/types', async () => {
      const mockResponse = {
        types: [
          { type: 'movie', label: 'Movies', count: 100 },
          { type: 'tv_show', label: 'TV Shows', count: 50 },
        ],
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.getEntityTypes()

      expect(mockApi.get).toHaveBeenCalledWith('/entities/types')
      expect(result).toEqual(mockResponse)
    })
  })

  describe('browseByType', () => {
    it('calls GET /entities/browse/:type with params', async () => {
      const mockResponse = {
        entities: [{ id: 1, title: 'Action Movie' }],
        total: 50,
        type: 'movie',
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.browseByType('movie', { limit: 20, offset: 0 })

      expect(mockApi.get).toHaveBeenCalledWith('/entities/browse/movie', {
        params: { limit: 20, offset: 0 },
      })
      expect(result).toEqual(mockResponse)
    })

    it('calls GET /entities/browse/:type without params', async () => {
      mockApi.get.mockResolvedValue({ data: { entities: [], total: 0, type: 'tv_show' } })

      await entityApi.browseByType('tv_show')

      expect(mockApi.get).toHaveBeenCalledWith('/entities/browse/tv_show', {
        params: undefined,
      })
    })
  })

  describe('getEntityStats', () => {
    it('calls GET /entities/stats', async () => {
      const mockStats = {
        total_entities: 500,
        by_type: { movie: 200, tv_show: 100, music_album: 200 },
      }
      mockApi.get.mockResolvedValue({ data: mockStats })

      const result = await entityApi.getEntityStats()

      expect(mockApi.get).toHaveBeenCalledWith('/entities/stats')
      expect(result).toEqual(mockStats)
    })
  })

  describe('refreshEntityMetadata', () => {
    it('calls POST /entities/:id/metadata/refresh', async () => {
      const mockResponse = { message: 'Metadata refresh started', entity_id: 5 }
      mockApi.post.mockResolvedValue({ data: mockResponse })

      const result = await entityApi.refreshEntityMetadata(5)

      expect(mockApi.post).toHaveBeenCalledWith('/entities/5/metadata/refresh')
      expect(result).toEqual(mockResponse)
    })

    it('propagates errors on refresh failure', async () => {
      mockApi.post.mockRejectedValue(new Error('Service unavailable'))

      await expect(entityApi.refreshEntityMetadata(5)).rejects.toThrow('Service unavailable')
    })
  })

  describe('updateUserMetadata', () => {
    it('calls PUT /entities/:id/user-metadata with data', async () => {
      const userMetadata = { rating: 5, notes: 'Great movie' }
      mockApi.put.mockResolvedValue({ data: { message: 'Metadata updated' } })

      const result = await entityApi.updateUserMetadata(5, userMetadata as any)

      expect(mockApi.put).toHaveBeenCalledWith('/entities/5/user-metadata', userMetadata)
      expect(result).toEqual({ message: 'Metadata updated' })
    })

    it('propagates errors on update failure', async () => {
      mockApi.put.mockRejectedValue(new Error('Forbidden'))

      await expect(entityApi.updateUserMetadata(5, {} as any)).rejects.toThrow('Forbidden')
    })
  })
})
