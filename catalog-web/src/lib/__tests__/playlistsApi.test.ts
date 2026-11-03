import { playlistsApi, playlistApi } from '../playlistsApi'
import { api } from '../api'

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

const mockApi = vi.mocked(api)

describe('playlistsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('exports playlistApi as an alias for playlistsApi', () => {
    expect(playlistApi).toBe(playlistsApi)
  })

  describe('getPlaylists', () => {
    it('calls GET /playlists with no params', async () => {
      const mockResponse = {
        playlists: [{ id: '1', name: 'My Playlist' }],
        total: 1,
        limit: 20,
        offset: 0,
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await playlistsApi.getPlaylists()

      expect(mockApi.get).toHaveBeenCalledWith(expect.stringContaining('/playlists'))
      expect(result).toEqual(mockResponse)
    })

    it('appends query params when provided', async () => {
      mockApi.get.mockResolvedValue({ data: { playlists: [], total: 0 } })

      await playlistsApi.getPlaylists({ limit: 10, offset: 5, type: 'favorites' })

      const calledUrl = mockApi.get.mock.calls[0][0] as string
      expect(calledUrl).toContain('limit=10')
      expect(calledUrl).toContain('offset=5')
      expect(calledUrl).toContain('type=favorites')
    })

    it('appends include_smart param when set', async () => {
      mockApi.get.mockResolvedValue({ data: { playlists: [], total: 0 } })

      await playlistsApi.getPlaylists({ include_smart: true })

      const calledUrl = mockApi.get.mock.calls[0][0] as string
      expect(calledUrl).toContain('include_smart=true')
    })

    it('propagates errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Server error'))

      await expect(playlistsApi.getPlaylists()).rejects.toThrow('Server error')
    })
  })

  describe('getPlaylist', () => {
    it('calls GET /playlists/:id', async () => {
      const mockPlaylist = { id: 'abc', name: 'Test Playlist' }
      mockApi.get.mockResolvedValue({ data: mockPlaylist })

      const result = await playlistsApi.getPlaylist('abc')

      expect(mockApi.get).toHaveBeenCalledWith('/playlists/abc')
      expect(result).toEqual(mockPlaylist)
    })
  })

  describe('getPlaylistItems', () => {
    it('calls GET /playlists/:id/items with no extra params', async () => {
      const mockItems = { items: [], total: 0, playlist: { id: 'abc' } }
      mockApi.get.mockResolvedValue({ data: mockItems })

      const result = await playlistsApi.getPlaylistItems('abc')

      expect(mockApi.get).toHaveBeenCalledWith(expect.stringContaining('/playlists/abc/items'))
      expect(result).toEqual(mockItems)
    })

    it('appends sort params when provided', async () => {
      mockApi.get.mockResolvedValue({ data: { items: [], total: 0 } })

      await playlistsApi.getPlaylistItems('abc', {
        sort_by: 'title',
        sort_order: 'desc',
        limit: 50,
      })

      const calledUrl = mockApi.get.mock.calls[0][0] as string
      expect(calledUrl).toContain('sort_by=title')
      expect(calledUrl).toContain('sort_order=desc')
      expect(calledUrl).toContain('limit=50')
    })
  })

  describe('createPlaylist', () => {
    it('calls POST /playlists with request body', async () => {
      const request = { name: 'New Playlist', description: 'A test playlist' }
      const created = { id: 'new-1', ...request }
      mockApi.post.mockResolvedValue({ data: created })

      const result = await playlistsApi.createPlaylist(request as any)

      expect(mockApi.post).toHaveBeenCalledWith('/playlists', request)
      expect(result).toEqual(created)
    })
  })

  describe('updatePlaylist', () => {
    it('calls PUT /playlists/:id with update data', async () => {
      const update = { name: 'Updated Name' }
      mockApi.put.mockResolvedValue({ data: { id: 'abc', ...update } })

      const result = await playlistsApi.updatePlaylist('abc', update)

      expect(mockApi.put).toHaveBeenCalledWith('/playlists/abc', update)
      expect(result.name).toBe('Updated Name')
    })
  })

  describe('deletePlaylist', () => {
    it('calls DELETE /playlists/:id', async () => {
      mockApi.delete.mockResolvedValue({})

      await playlistsApi.deletePlaylist('abc')

      expect(mockApi.delete).toHaveBeenCalledWith('/playlists/abc')
    })

    it('propagates error on delete failure', async () => {
      mockApi.delete.mockRejectedValue(new Error('Forbidden'))

      await expect(playlistsApi.deletePlaylist('abc')).rejects.toThrow('Forbidden')
    })
  })

  describe('addItemsToPlaylist', () => {
    it('calls POST /playlists/:id/items with media IDs', async () => {
      mockApi.post.mockResolvedValue({ data: { added: 3, failed: 0 } })

      const result = await playlistsApi.addItemsToPlaylist('abc', [1, 2, 3])

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/abc/items', {
        media_ids: [1, 2, 3],
      })
      expect(result).toEqual({ added: 3, failed: 0 })
    })
  })

  describe('removeFromPlaylist', () => {
    it('calls DELETE /playlists/:playlistId/items/:itemId', async () => {
      mockApi.delete.mockResolvedValue({})

      await playlistsApi.removeFromPlaylist('abc', 'item-1')

      expect(mockApi.delete).toHaveBeenCalledWith('/playlists/abc/items/item-1')
    })
  })

  describe('reorderPlaylistItems', () => {
    it('calls PUT /playlists/:id/items/reorder', async () => {
      const itemOrders = [
        { id: 'a', position: 0 },
        { id: 'b', position: 1 },
      ]
      mockApi.put.mockResolvedValue({})

      await playlistsApi.reorderPlaylistItems('abc', itemOrders)

      expect(mockApi.put).toHaveBeenCalledWith('/playlists/abc/items/reorder', {
        items: itemOrders,
      })
    })
  })

  describe('sharePlaylist', () => {
    it('calls POST /playlists/:id/share with permissions', async () => {
      const permissions = { can_view: true, can_comment: false, can_download: true }
      const mockShareInfo = {
        share_url: 'http://localhost/shared/abc',
        share_token: 'token123',
        permissions,
      }
      mockApi.post.mockResolvedValue({ data: mockShareInfo })

      const result = await playlistsApi.sharePlaylist('abc', permissions)

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/abc/share', permissions)
      expect(result).toEqual(mockShareInfo)
    })
  })

  describe('getSharedPlaylist', () => {
    it('calls GET /playlists/shared/:token', async () => {
      const mockData = { items: [], total: 0, playlist: { id: 'abc' } }
      mockApi.get.mockResolvedValue({ data: mockData })

      const result = await playlistsApi.getSharedPlaylist('token123')

      expect(mockApi.get).toHaveBeenCalledWith('/playlists/shared/token123')
      expect(result).toEqual(mockData)
    })
  })

  describe('unsharePlaylist', () => {
    it('calls DELETE /playlists/:id/share', async () => {
      mockApi.delete.mockResolvedValue({})

      await playlistsApi.unsharePlaylist('abc')

      expect(mockApi.delete).toHaveBeenCalledWith('/playlists/abc/share')
    })
  })

  describe('getPlaylistAnalytics', () => {
    it('calls GET /playlists/:id/analytics', async () => {
      const mockAnalytics = {
        playlist_id: 'abc',
        total_plays: 100,
        unique_viewers: 50,
      }
      mockApi.get.mockResolvedValue({ data: mockAnalytics })

      const result = await playlistsApi.getPlaylistAnalytics('abc')

      expect(mockApi.get).toHaveBeenCalledWith('/playlists/abc/analytics')
      expect(result).toEqual(mockAnalytics)
    })
  })

  describe('duplicatePlaylist', () => {
    it('calls POST /playlists/:id/duplicate with optional name', async () => {
      const duplicated = { id: 'dup-1', name: 'Copy of Playlist' }
      mockApi.post.mockResolvedValue({ data: duplicated })

      const result = await playlistsApi.duplicatePlaylist('abc', 'Copy of Playlist')

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/abc/duplicate', {
        name: 'Copy of Playlist',
      })
      expect(result).toEqual(duplicated)
    })

    it('sends undefined name when not provided', async () => {
      mockApi.post.mockResolvedValue({ data: { id: 'dup-1' } })

      await playlistsApi.duplicatePlaylist('abc')

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/abc/duplicate', {
        name: undefined,
      })
    })
  })

  describe('exportPlaylist', () => {
    it('calls GET /playlists/:id/export with format and blob responseType', async () => {
      const mockBlob = new Blob(['data'])
      mockApi.get.mockResolvedValue({ data: mockBlob })

      const result = await playlistsApi.exportPlaylist('abc', 'm3u')

      expect(mockApi.get).toHaveBeenCalledWith(
        '/playlists/abc/export?format=m3u',
        { responseType: 'blob' }
      )
      expect(result).toEqual(mockBlob)
    })

    it('defaults to json format', async () => {
      mockApi.get.mockResolvedValue({ data: {} })

      await playlistsApi.exportPlaylist('abc')

      expect(mockApi.get).toHaveBeenCalledWith(
        '/playlists/abc/export?format=json',
        { responseType: 'blob' }
      )
    })
  })

  describe('playPlaylist', () => {
    it('calls POST /playlists/:id/play', async () => {
      mockApi.post.mockResolvedValue({})

      await playlistsApi.playPlaylist('abc')

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/abc/play')
    })
  })

  describe('shufflePlaylist', () => {
    it('calls POST /playlists/:id/shuffle', async () => {
      mockApi.post.mockResolvedValue({})

      await playlistsApi.shufflePlaylist('abc')

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/abc/shuffle')
    })
  })

  describe('reorderPlaylist', () => {
    it('calls PUT /playlists/:id/reorder with item IDs', async () => {
      mockApi.put.mockResolvedValue({})

      await playlistsApi.reorderPlaylist('abc', ['item-1', 'item-3', 'item-2'])

      expect(mockApi.put).toHaveBeenCalledWith('/playlists/abc/reorder', {
        itemIds: ['item-1', 'item-3', 'item-2'],
      })
    })
  })

  describe('importPlaylist', () => {
    it('calls POST /playlists/import with FormData', async () => {
      const mockFile = new File(['playlist data'], 'playlist.m3u', { type: 'audio/x-mpegurl' })
      const mockResult = { playlist: { id: 'imported-1' }, imported: 10, failed: 1 }
      mockApi.post.mockResolvedValue({ data: mockResult })

      const result = await playlistsApi.importPlaylist(mockFile, 'My Import')

      expect(mockApi.post).toHaveBeenCalledWith(
        '/playlists/import',
        expect.any(FormData),
        { headers: { 'Content-Type': 'multipart/form-data' } }
      )
      expect(result).toEqual(mockResult)
    })

    it('sends FormData without name when not provided', async () => {
      const mockFile = new File(['data'], 'test.m3u')
      mockApi.post.mockResolvedValue({ data: { playlist: {}, imported: 0, failed: 0 } })

      await playlistsApi.importPlaylist(mockFile)

      const formData = mockApi.post.mock.calls[0][1] as FormData
      expect(formData.get('file')).toEqual(mockFile)
      expect(formData.get('name')).toBeNull()
    })
  })

  describe('validateSmartRules', () => {
    it('calls POST /playlists/validate-smart-rules', async () => {
      const rules = [
        { field: 'media_type' as const, operator: 'equals' as const, value: 'movie' },
      ]
      mockApi.post.mockResolvedValue({ data: { valid: true, errors: [] } })

      const result = await playlistsApi.validateSmartRules(rules)

      expect(mockApi.post).toHaveBeenCalledWith('/playlists/validate-smart-rules', {
        rules,
      })
      expect(result).toEqual({ valid: true, errors: [] })
    })

    it('returns validation errors for invalid rules', async () => {
      const rules = [
        { field: 'rating' as const, operator: 'greater_than' as const, value: 'not_a_number' },
      ]
      mockApi.post.mockResolvedValue({
        data: { valid: false, errors: ['Rating value must be a number'] },
      })

      const result = await playlistsApi.validateSmartRules(rules)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('Rating value must be a number')
    })
  })
})
