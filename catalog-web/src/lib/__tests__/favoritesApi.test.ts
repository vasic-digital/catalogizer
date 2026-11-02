import { favoritesApi } from '../favoritesApi'
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

describe('favoritesApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getFavorites', () => {
    it('calls GET /favorites with query params', async () => {
      const mockResponse = {
        items: [{ id: '1', media_id: 42, created_at: '2024-01-01' }],
        total: 1,
        limit: 20,
        offset: 0,
      }
      mockApi.get.mockResolvedValue({ data: mockResponse })

      const result = await favoritesApi.getFavorites({
        limit: 20,
        offset: 0,
        media_type: 'movie',
        sort_by: 'created_at',
        sort_order: 'desc',
      })

      expect(mockApi.get).toHaveBeenCalledWith(
        expect.stringContaining('/favorites')
      )
      expect(result).toEqual(mockResponse)
    })

    it('calls without params when none provided', async () => {
      mockApi.get.mockResolvedValue({ data: { items: [], total: 0 } })

      await favoritesApi.getFavorites()

      expect(mockApi.get).toHaveBeenCalledWith('/favorites?')
    })
  })

  describe('getFavoriteStats', () => {
    it('calls GET /favorites/stats', async () => {
      const stats = {
        total_count: 42,
        media_type_breakdown: { movie: 20, tv_show: 15, music: 7 },
        recent_additions: [],
      }
      mockApi.get.mockResolvedValue({ data: stats })

      const result = await favoritesApi.getFavoriteStats()

      expect(mockApi.get).toHaveBeenCalledWith('/favorites/stats')
      expect(result).toEqual(stats)
    })
  })

  describe('toggleFavorite', () => {
    it('calls POST /favorites/toggle with request data', async () => {
      const toggleResponse = { is_favorite: true }
      mockApi.post.mockResolvedValue({ data: toggleResponse })

      const result = await favoritesApi.toggleFavorite({
        media_id: 42,
        is_favorite: true,
      })

      expect(mockApi.post).toHaveBeenCalledWith('/favorites/toggle', {
        media_id: 42,
        is_favorite: true,
      })
      expect(result).toEqual(toggleResponse)
    })
  })

  describe('checkFavorite', () => {
    it('calls GET /favorites/check/:mediaId', async () => {
      mockApi.get.mockResolvedValue({ data: { is_favorite: true } })

      const result = await favoritesApi.checkFavorite(42)

      expect(mockApi.get).toHaveBeenCalledWith('/favorites/check/42')
      expect(result.is_favorite).toBe(true)
    })

    it('returns false for non-favorited media', async () => {
      mockApi.get.mockResolvedValue({ data: { is_favorite: false } })

      const result = await favoritesApi.checkFavorite(999)

      expect(result.is_favorite).toBe(false)
    })
  })

  describe('addToFavorites', () => {
    it('calls POST /favorites with media_id', async () => {
      const favorite = { id: '1', media_id: 42, user_id: 1, created_at: '2024-01-01' }
      mockApi.post.mockResolvedValue({ data: favorite })

      const result = await favoritesApi.addToFavorites(42)

      expect(mockApi.post).toHaveBeenCalledWith('/favorites', { media_id: 42 })
      expect(result).toEqual(favorite)
    })
  })

  describe('removeFromFavorites', () => {
    it('calls DELETE /favorites/:mediaId', async () => {
      mockApi.delete.mockResolvedValue({})

      await favoritesApi.removeFromFavorites(42)

      expect(mockApi.delete).toHaveBeenCalledWith('/favorites/42')
    })
  })

  describe('bulkAddToFavorites', () => {
    it('calls POST /favorites/bulk with media IDs', async () => {
      mockApi.post.mockResolvedValue({ data: { added: 3, failed: 0 } })

      const result = await favoritesApi.bulkAddToFavorites([1, 2, 3])

      expect(mockApi.post).toHaveBeenCalledWith('/favorites/bulk', {
        media_ids: [1, 2, 3],
      })
      expect(result).toEqual({ added: 3, failed: 0 })
    })
  })

  describe('bulkRemoveFromFavorites', () => {
    it('calls DELETE /favorites/bulk with media IDs', async () => {
      mockApi.delete.mockResolvedValue({ data: { removed: 2, failed: 0 } })

      const result = await favoritesApi.bulkRemoveFromFavorites([1, 2])

      expect(mockApi.delete).toHaveBeenCalledWith('/favorites/bulk', {
        data: { media_ids: [1, 2] },
      })
      expect(result).toEqual({ removed: 2, failed: 0 })
    })
  })
})
