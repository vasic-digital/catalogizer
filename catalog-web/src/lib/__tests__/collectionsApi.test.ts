import { collectionsApi } from '../collectionsApi'

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

import { api } from '../api'
const mockApi = vi.mocked(api)

describe('collectionsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getCollections', () => {
    it('unwraps the {items,total} envelope and normalises backend collections', async () => {
      // wire shape captured from catalog-api GET /collections
      mockApi.get.mockResolvedValue({
        data: {
          items: [
            { id: 1, name: 'Test Collection', collection_type: 'custom', total_items: 5, cover_url: null,
              created_at: '2026-11-03T00:00:00Z', updated_at: '2026-11-03T00:00:00Z' },
          ],
          total: 1, limit: 24, offset: 0,
        },
      })

      const result = await collectionsApi.getCollections()

      expect(mockApi.get).toHaveBeenCalledWith('/collections')
      expect(result).toHaveLength(1)
      expect(result[0]).toMatchObject({ id: '1', name: 'Test Collection', item_count: 5, is_smart: false, smart_rules: [], primary_media_type: 'mixed' })
    })

    it('returns [] for the empty envelope and for a null body', async () => {
      mockApi.get.mockResolvedValue({ data: { items: [], total: 0, limit: 24, offset: 0 } })
      expect(await collectionsApi.getCollections()).toEqual([])
      mockApi.get.mockResolvedValue({ data: null })
      expect(await collectionsApi.getCollections()).toEqual([])
    })

    it('propagates errors on failure', async () => {
      const error = new Error('Server error')
      ;(error as any).response = { status: 500 }
      mockApi.get.mockRejectedValue(error)

      await expect(collectionsApi.getCollections()).rejects.toThrow('Server error')
    })
  })

  describe('getCollection', () => {
    it('calls GET /api/collections/:id and returns single collection', async () => {
      const mockCollection = { id: '1', name: 'Test Collection', item_count: 5 }
      mockApi.get.mockResolvedValue({ data: mockCollection })

      const result = await collectionsApi.getCollection('1')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1')
      expect(result).toMatchObject({ id: '1', name: 'Test Collection', item_count: 5 })
    })

    it('propagates 404 errors', async () => {
      const error = new Error('Not found')
      ;(error as any).response = { status: 404 }
      mockApi.get.mockRejectedValue(error)

      await expect(collectionsApi.getCollection('1')).rejects.toThrow('Not found')
    })
  })

  describe('createCollection', () => {
    it('calls POST /api/collections with collection data', async () => {
      const newCollection = {
        name: 'New Collection',
        description: 'A test collection',
        is_public: false,
        is_smart: true as const,
        smart_rules: [],
      }
      const created = { ...newCollection, id: '3', item_count: 0 }
      mockApi.post.mockResolvedValue({ data: created })

      const result = await collectionsApi.createCollection(newCollection)

      expect(mockApi.post).toHaveBeenCalledWith('/collections', newCollection)
      expect(result).toEqual(created)
    })
  })

  describe('updateCollection', () => {
    it('calls PUT /api/collections/:id with updates', async () => {
      const updates = { name: 'Updated Name' }
      const updated = { id: '1', name: 'Updated Name', item_count: 5 }
      mockApi.put.mockResolvedValue({ data: updated })

      const result = await collectionsApi.updateCollection('1', updates)

      expect(mockApi.put).toHaveBeenCalledWith('/collections/1', updates)
      expect(result).toEqual(updated)
    })
  })

  describe('deleteCollection', () => {
    it('calls DELETE /api/collections/:id', async () => {
      mockApi.delete.mockResolvedValue({})

      await collectionsApi.deleteCollection('1')

      expect(mockApi.delete).toHaveBeenCalledWith('/collections/1')
    })
  })

  describe('getCollectionItems', () => {
    it('calls GET /api/collections/:id/items with pagination params', async () => {
      const mockItems = {
        items: [{ id: 'item1', title: 'Item 1' }],
        total: 1,
        page: 1,
        limit: 50,
      }
      mockApi.get.mockResolvedValue({ data: mockItems })

      const result = await collectionsApi.getCollectionItems('1', 1, 50)

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/items', {
        params: { page: 1, limit: 50 },
      })
      expect(result).toEqual(mockItems)
    })

    it('uses default pagination parameters', async () => {
      mockApi.get.mockResolvedValue({ data: { items: [], total: 0 } })

      await collectionsApi.getCollectionItems('1')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/items', {
        params: { page: 1, limit: 50 },
      })
    })

    it('passes custom page and limit', async () => {
      mockApi.get.mockResolvedValue({ data: { items: [], total: 0 } })

      await collectionsApi.getCollectionItems('2', 3, 25)

      expect(mockApi.get).toHaveBeenCalledWith('/collections/2/items', {
        params: { page: 3, limit: 25 },
      })
    })
  })

  describe('refreshCollection', () => {
    it('calls POST /api/collections/:id/refresh', async () => {
      const refreshed = { id: '1', name: 'Test', item_count: 15, last_updated: '2024-01-01' }
      mockApi.post.mockResolvedValue({ data: refreshed })

      const result = await collectionsApi.refreshCollection('1')

      expect(mockApi.post).toHaveBeenCalledWith('/collections/1/refresh')
      expect(result).toEqual(refreshed)
    })
  })

  describe('getCollectionAnalytics', () => {
    it('calls GET /api/collections/:id/analytics', async () => {
      const mockAnalytics = {
        collection_id: '1',
        total_items: 100,
        media_type_distribution: { music: 50, video: 30, image: 20, document: 0 },
      }
      mockApi.get.mockResolvedValue({ data: mockAnalytics })

      const result = await collectionsApi.getCollectionAnalytics('1')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/analytics')
      expect(result).toEqual(mockAnalytics)
    })
  })

  describe('shareCollection', () => {
    it('calls POST /api/collections/:id/share with share request', async () => {
      const shareRequest = { can_view: true, can_comment: false, can_download: true }
      const shareInfo = {
        share_id: 'share_1',
        share_url: 'http://localhost/shared/share_1',
        permissions: shareRequest,
      }
      mockApi.post.mockResolvedValue({ data: shareInfo })

      const result = await collectionsApi.shareCollection('1', shareRequest)

      expect(mockApi.post).toHaveBeenCalledWith('/collections/1/share', shareRequest)
      expect(result).toEqual(shareInfo)
    })
  })

  describe('getSharedCollection', () => {
    it('calls GET /api/collections/shared/:shareId', async () => {
      const mockShared = { id: '3', name: 'Shared Collection' }
      mockApi.get.mockResolvedValue({ data: mockShared })

      const result = await collectionsApi.getSharedCollection('share_abc')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/shared/share_abc')
      expect(result).toEqual(mockShared)
    })

    it('propagates errors on failure', async () => {
      const error = new Error('Unauthorized')
      ;(error as any).response = { status: 403 }
      mockApi.get.mockRejectedValue(error)

      await expect(collectionsApi.getSharedCollection('invalid')).rejects.toThrow('Unauthorized')
    })
  })

  describe('exportCollection', () => {
    it('calls GET /api/collections/:id/export with format param', async () => {
      const blob = new Blob(['test data'], { type: 'application/json' })
      mockApi.get.mockResolvedValue({ data: blob })

      const result = await collectionsApi.exportCollection('1', 'json')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/export', {
        params: { format: 'json' },
        responseType: 'blob',
      })
      expect(result).toBeInstanceOf(Blob)
    })

    it('defaults to json format', async () => {
      const blob = new Blob(['{}'], { type: 'application/json' })
      mockApi.get.mockResolvedValue({ data: blob })

      await collectionsApi.exportCollection('1')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/export', {
        params: { format: 'json' },
        responseType: 'blob',
      })
    })

    it('supports csv format', async () => {
      const blob = new Blob(['name,desc'], { type: 'text/csv' })
      mockApi.get.mockResolvedValue({ data: blob })

      await collectionsApi.exportCollection('1', 'csv')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/export', {
        params: { format: 'csv' },
        responseType: 'blob',
      })
    })

    it('supports m3u format', async () => {
      const blob = new Blob(['#EXTM3U'], { type: 'audio/x-mpegurl' })
      mockApi.get.mockResolvedValue({ data: blob })

      await collectionsApi.exportCollection('1', 'm3u')

      expect(mockApi.get).toHaveBeenCalledWith('/collections/1/export', {
        params: { format: 'm3u' },
        responseType: 'blob',
      })
    })
  })

  describe('importCollection', () => {
    it('calls POST /api/collections/import with FormData', async () => {
      const mockFile = new File(['{"name":"imported"}'], 'collection.json', {
        type: 'application/json',
      })
      const imported = { id: 'imported-1', name: 'imported' }
      mockApi.post.mockResolvedValue({ data: imported })

      const result = await collectionsApi.importCollection(mockFile)

      expect(mockApi.post).toHaveBeenCalledWith(
        '/collections/import',
        expect.any(FormData),
        { headers: { 'Content-Type': 'multipart/form-data' } }
      )
      expect(result).toEqual(imported)
    })

    it('sends FormData with the file appended', async () => {
      const mockFile = new File(['{}'], 'test.json', { type: 'application/json' })
      mockApi.post.mockResolvedValue({ data: {} })

      await collectionsApi.importCollection(mockFile)

      const formData = mockApi.post.mock.calls[0][1] as FormData
      expect(formData.get('file')).toEqual(mockFile)
    })
  })

  describe('duplicateCollection', () => {
    it('calls POST /api/collections/:id/duplicate with new name', async () => {
      const duplicated = { id: '4', name: 'Copy of Collection' }
      mockApi.post.mockResolvedValue({ data: duplicated })

      const result = await collectionsApi.duplicateCollection('1', 'Copy of Collection')

      expect(mockApi.post).toHaveBeenCalledWith('/collections/1/duplicate', {
        name: 'Copy of Collection',
      })
      expect(result).toEqual(duplicated)
    })

    it('sends undefined name when not provided', async () => {
      const duplicated = { id: '5', name: 'Original (Copy)' }
      mockApi.post.mockResolvedValue({ data: duplicated })

      await collectionsApi.duplicateCollection('1')

      expect(mockApi.post).toHaveBeenCalledWith('/collections/1/duplicate', {
        name: undefined,
      })
    })
  })

  describe('addItemsToCollection', () => {
    it('calls POST /api/collections/:id/items with item IDs', async () => {
      mockApi.post.mockResolvedValue({})

      await collectionsApi.addItemsToCollection('1', ['item-a', 'item-b', 'item-c'])

      expect(mockApi.post).toHaveBeenCalledWith('/collections/1/items', {
        item_ids: ['item-a', 'item-b', 'item-c'],
      })
    })

    it('handles empty item IDs array', async () => {
      mockApi.post.mockResolvedValue({})

      await collectionsApi.addItemsToCollection('1', [])

      expect(mockApi.post).toHaveBeenCalledWith('/collections/1/items', {
        item_ids: [],
      })
    })
  })

  describe('removeItemsFromCollection', () => {
    it('calls DELETE /api/collections/:id/items with item IDs', async () => {
      mockApi.delete.mockResolvedValue({})

      await collectionsApi.removeItemsFromCollection('1', ['item-a', 'item-b'])

      expect(mockApi.delete).toHaveBeenCalledWith('/collections/1/items', {
        data: { item_ids: ['item-a', 'item-b'] },
      })
    })
  })

  describe('getCollectionSuggestions', () => {
    it('calls GET /api/collections/suggestions and returns string array', async () => {
      const suggestions = ['Summer Hits', 'Best of 2024']
      mockApi.get.mockResolvedValue({ data: suggestions })

      const result = await collectionsApi.getCollectionSuggestions()

      expect(mockApi.get).toHaveBeenCalledWith('/collections/suggestions')
      expect(result).toEqual(suggestions)
    })

    it('propagates errors on failure', async () => {
      const error = new Error('Network error')
      ;(error as any).response = { status: 500 }
      mockApi.get.mockRejectedValue(error)

      await expect(collectionsApi.getCollectionSuggestions()).rejects.toThrow('Network error')
    })
  })

  describe('getTemplates', () => {
    it('calls GET /api/collections/templates and returns templates', async () => {
      const templates = [
        { id: 'recently_added', name: 'Recently Added', description: 'Last 30 days' },
      ]
      mockApi.get.mockResolvedValue({ data: templates })

      const result = await collectionsApi.getTemplates()

      expect(mockApi.get).toHaveBeenCalledWith('/collections/templates')
      expect(result).toEqual(templates)
    })
  })

  describe('testRules', () => {
    it('calls POST /api/collections/test-rules with rules array', async () => {
      const rules = [{ field: 'genre', operator: 'equals', value: 'rock' }]
      const testResult = { valid: true, sample_count: 25 }
      mockApi.post.mockResolvedValue({ data: testResult })

      const result = await collectionsApi.testRules(rules as any)

      expect(mockApi.post).toHaveBeenCalledWith('/collections/test-rules', { rules })
      expect(result).toEqual(testResult)
    })

    it('returns errors for invalid rules', async () => {
      const rules = [{ field: 'rating', operator: 'equals', value: '' }]
      const testResult = { valid: false, sample_count: 0, errors: ['Value is required'] }
      mockApi.post.mockResolvedValue({ data: testResult })

      const result = await collectionsApi.testRules(rules as any)

      expect(result.valid).toBe(false)
      expect(result.errors).toContain('Value is required')
    })
  })

  describe('bulkDeleteCollections', () => {
    it('calls DELETE /api/collections/bulk with collection IDs', async () => {
      mockApi.delete.mockResolvedValue({})

      await collectionsApi.bulkDeleteCollections(['1', '2', '3'])

      expect(mockApi.delete).toHaveBeenCalledWith('/collections/bulk', {
        data: { collection_ids: ['1', '2', '3'] },
      })
    })
  })

  describe('bulkShareCollections', () => {
    it('calls POST /api/collections/bulk/share with IDs and share request', async () => {
      const shareRequest = { can_view: true, can_comment: false, can_download: true }
      const shareInfos = [
        { share_id: 'share_1', share_url: 'http://localhost/shared/1' },
        { share_id: 'share_2', share_url: 'http://localhost/shared/2' },
      ]
      mockApi.post.mockResolvedValue({ data: shareInfos })

      const result = await collectionsApi.bulkShareCollections(['1', '2'], shareRequest)

      expect(mockApi.post).toHaveBeenCalledWith('/collections/bulk/share', {
        collection_ids: ['1', '2'],
        share_request: shareRequest,
      })
      expect(result).toEqual(shareInfos)
    })
  })

  describe('bulkExportCollections', () => {
    it('calls POST /api/collections/bulk/export with IDs and format', async () => {
      const blob = new Blob(['[{}]'], { type: 'application/json' })
      mockApi.post.mockResolvedValue({ data: blob })

      const result = await collectionsApi.bulkExportCollections(['1', '2'], 'json')

      expect(mockApi.post).toHaveBeenCalledWith(
        '/collections/bulk/export',
        { collection_ids: ['1', '2'], format: 'json' },
        { responseType: 'blob' }
      )
      expect(result).toBeInstanceOf(Blob)
    })

    it('defaults to json format', async () => {
      const blob = new Blob(['[]'], { type: 'application/json' })
      mockApi.post.mockResolvedValue({ data: blob })

      await collectionsApi.bulkExportCollections(['1'])

      expect(mockApi.post).toHaveBeenCalledWith(
        '/collections/bulk/export',
        { collection_ids: ['1'], format: 'json' },
        { responseType: 'blob' }
      )
    })

    it('supports csv format', async () => {
      const blob = new Blob(['name,desc'], { type: 'text/csv' })
      mockApi.post.mockResolvedValue({ data: blob })

      await collectionsApi.bulkExportCollections(['1', '2'], 'csv')

      expect(mockApi.post).toHaveBeenCalledWith(
        '/collections/bulk/export',
        { collection_ids: ['1', '2'], format: 'csv' },
        { responseType: 'blob' }
      )
    })
  })

  describe('bulkUpdateCollections', () => {
    it('calls PUT /api/collections/bulk with IDs, action, and updates', async () => {
      const updates = { name: 'Bulk Updated' }
      const result_data = [{ id: '1', name: 'Bulk Updated' }, { id: '2', name: 'Bulk Updated' }]
      mockApi.put.mockResolvedValue({ data: result_data })

      const result = await collectionsApi.bulkUpdateCollections(['1', '2'], 'update', updates)

      expect(mockApi.put).toHaveBeenCalledWith('/collections/bulk', {
        collection_ids: ['1', '2'],
        action: 'update',
        updates,
      })
      expect(result).toEqual(result_data)
    })

    it('calls with duplicate action and no updates', async () => {
      const result_data = [{ id: '3', name: 'Copy' }]
      mockApi.put.mockResolvedValue({ data: result_data })

      const result = await collectionsApi.bulkUpdateCollections(['1'], 'duplicate')

      expect(mockApi.put).toHaveBeenCalledWith('/collections/bulk', {
        collection_ids: ['1'],
        action: 'duplicate',
        updates: undefined,
      })
      expect(result).toEqual(result_data)
    })
  })

  describe('error propagation', () => {
    it('does not swallow non-404 errors', async () => {
      const error = new Error('Internal server error')
      ;(error as any).response = { status: 500 }
      mockApi.get.mockRejectedValue(error)

      await expect(collectionsApi.getCollection('1')).rejects.toThrow('Internal server error')
    })

    it('propagates network errors without response', async () => {
      const error = new Error('Network error')
      mockApi.get.mockRejectedValue(error)

      await expect(collectionsApi.getCollection('1')).rejects.toThrow('Network error')
    })
  })
})
