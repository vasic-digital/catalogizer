import { recommendationsApi } from '../recommendationsApi'
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

describe('recommendationsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getSimilar', () => {
    it('calls GET /recommendations/similar/:mediaId and flattens local_items', async () => {
      mockApi.get.mockResolvedValue({
        data: {
          local_items: [
            { media_id: '2', media_metadata: { id: 2, title: 'Similar Movie', media_type: 'movie' }, similarity_score: 0.85, similarity_reasons: ['Same genre'] },
          ],
          external_items: [],
        },
      })

      const result = await recommendationsApi.getSimilar(1)

      expect(mockApi.get).toHaveBeenCalledWith('/recommendations/similar/1')
      expect(result).toEqual([
        { id: 2, title: 'Similar Movie', media_type: 'movie', similarity_score: 0.85, reason: 'Same genre' },
      ])
    })

    it('propagates errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Not found'))

      await expect(recommendationsApi.getSimilar(999)).rejects.toThrow('Not found')
    })
  })

  describe('getTrending', () => {
    it('calls GET /recommendations/trending with default limit', async () => {
      const mockItems = [
        { id: 1, title: 'Trending Movie', media_type: 'movie', access_count: 100, trending_score: 9.5 },
      ]
      mockApi.get.mockResolvedValue({ data: mockItems })

      const result = await recommendationsApi.getTrending()

      expect(mockApi.get).toHaveBeenCalledWith('/recommendations/trending', { params: { limit: 10 } })
      expect(result).toEqual(mockItems)
    })

    it('calls GET /recommendations/trending with custom limit', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      await recommendationsApi.getTrending(5)

      expect(mockApi.get).toHaveBeenCalledWith('/recommendations/trending', { params: { limit: 5 } })
    })

    it('propagates errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Server error'))

      await expect(recommendationsApi.getTrending()).rejects.toThrow('Server error')
    })
  })

  describe('getPersonalized', () => {
    it('calls GET /recommendations/personalized/:userId with default limit', async () => {
      const mockItems = [
        { id: 5, title: 'For You', media_type: 'tv_show', recommendation_reason: 'Based on watch history' },
      ]
      mockApi.get.mockResolvedValue({ data: mockItems })

      const result = await recommendationsApi.getPersonalized(1)

      expect(mockApi.get).toHaveBeenCalledWith('/recommendations/personalized/1', { params: { limit: 10 } })
      expect(result).toEqual(mockItems)
    })

    it('calls GET /recommendations/personalized/:userId with custom limit', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      await recommendationsApi.getPersonalized(1, 20)

      expect(mockApi.get).toHaveBeenCalledWith('/recommendations/personalized/1', { params: { limit: 20 } })
    })

    it('propagates errors for invalid user', async () => {
      mockApi.get.mockRejectedValue(new Error('User not found'))

      await expect(recommendationsApi.getPersonalized(999)).rejects.toThrow('User not found')
    })
  })
})
