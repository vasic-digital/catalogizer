import { favoritesApi } from '../favoritesApi'
import apiDefault from '../api'

vi.mock('../api', async () => {
  const mockApi = { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }
  return { __esModule: true, default: mockApi, api: mockApi }
})

const mockApi = vi.mocked(apiDefault)

// The shared axios instance already has baseURL '/api/v1': a path that starts
// with '/api/v1' is requested as '/api/v1/api/v1/...' (404 on the real server).
const calledPaths = () =>
  [mockApi.get, mockApi.post, mockApi.delete].flatMap((m) => m.mock.calls.map((c) => String(c[0])))

describe('favoritesApi (backend contract: /favorites, entity_type + entity_id)', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(() => {
    for (const p of calledPaths()) expect(p).not.toContain('/api/v1')
  })

  it('maps GET /favorites {favorites,count,limit,offset} to FavoritesResponse', async () => {
    mockApi.get.mockResolvedValue({
      data: { favorites: [{ id: 7, user_id: 1, entity_type: 'movie', entity_id: 42, created_at: '2026-11-03T00:00:00Z', updated_at: '2026-11-03T00:00:00Z' }], count: 1, limit: 20, offset: 0 },
    })
    const res = await favoritesApi.getFavorites({ limit: 20, offset: 0 })
    expect(String(mockApi.get.mock.calls[0][0])).toMatch(/^\/favorites\?/)
    expect(res).toMatchObject({ total: 1, limit: 20, offset: 0 })
    expect(res.items[0]).toMatchObject({ id: '7', media_id: 42, media_item: { id: 42, media_type: 'movie' } })
  })

  it('treats an empty backend list as an empty page (no crash)', async () => {
    mockApi.get.mockResolvedValue({ data: { favorites: [], count: 0, limit: 20, offset: 0 } })
    expect((await favoritesApi.getFavorites()).items).toEqual([])
    mockApi.get.mockResolvedValue({ data: { favorites: null, count: 0 } })
    expect((await favoritesApi.getFavorites()).items).toEqual([])
  })

  it('derives stats client-side (the backend has no /favorites/stats)', async () => {
    mockApi.get.mockResolvedValue({ data: { favorites: [{ id: 1, entity_type: 'movie', entity_id: 1 }, { id: 2, entity_type: 'song', entity_id: 2 }], count: 2 } })
    const stats = await favoritesApi.getFavoriteStats()
    expect(mockApi.get.mock.calls[0][0]).toBe('/favorites')
    expect(stats.total_count).toBe(2)
    expect(stats.media_type_breakdown.movie).toBe(1)
    expect(stats.media_type_breakdown.other).toBe(1)
  })

  it('add / remove / check use entity_type + entity_id', async () => {
    mockApi.post.mockResolvedValue({ data: { id: 1, entity_type: 'movie', entity_id: 42 } })
    await favoritesApi.addToFavorites(42)
    expect(mockApi.post).toHaveBeenCalledWith('/favorites', { entity_type: 'movie', entity_id: 42 })

    mockApi.delete.mockResolvedValue({ data: {} })
    await favoritesApi.removeFromFavorites(42)
    expect(mockApi.delete).toHaveBeenCalledWith('/favorites/movie/42')

    mockApi.get.mockResolvedValue({ data: { is_favorite: true } })
    expect(await favoritesApi.checkFavorite(42)).toEqual({ is_favorite: true })
    expect(mockApi.get).toHaveBeenCalledWith('/favorites/check/movie/42')
  })

  it('toggle is composed from add/remove (no /favorites/toggle route)', async () => {
    mockApi.post.mockResolvedValue({ data: {} })
    mockApi.delete.mockResolvedValue({ data: {} })
    expect(await favoritesApi.toggleFavorite({ media_id: 5, is_favorite: true })).toEqual({ is_favorite: true })
    expect(await favoritesApi.toggleFavorite({ media_id: 5, is_favorite: false })).toEqual({ is_favorite: false })
    expect(mockApi.post).toHaveBeenCalledWith('/favorites', { entity_type: 'movie', entity_id: 5 })
    expect(mockApi.delete).toHaveBeenCalledWith('/favorites/movie/5')
  })

  it('bulk operations fan out per id and count failures', async () => {
    mockApi.post.mockResolvedValueOnce({ data: {} }).mockRejectedValueOnce(new Error('x'))
    expect(await favoritesApi.bulkAddToFavorites([1, 2])).toEqual({ added: 1, failed: 1 })
    mockApi.delete.mockResolvedValue({ data: {} })
    expect(await favoritesApi.bulkRemoveFromFavorites([1, 2])).toEqual({ removed: 2, failed: 0 })
  })

  it.each([[undefined], [null], [0], ['0x10'], [true], ['1e3'], [[5]], [1.5], [1e21], ['']])(
    'a wire entity_id of %j maps to an explicit invalid id (NaN), never 0 or a coerced id', async (bad) => {
      mockApi.get.mockResolvedValue({ data: { favorites: [{ id: 1, entity_type: 'movie', entity_id: bad }], count: 1 } })
      const res = await favoritesApi.getFavorites()
      expect(Number.isNaN(res.items[0].media_id)).toBe(true)
      expect(Number.isNaN(res.items[0].media_item.id)).toBe(true)
      expect(res.items[0].media_item.title).toMatch(/no valid item id/)
    })

  it('a numeric-string entity_id "42" is accepted as 42', async () => {
    mockApi.get.mockResolvedValue({ data: { favorites: [{ id: 1, entity_type: 'song', entity_id: '42' }], count: 1 } })
    const res = await favoritesApi.getFavorites()
    expect(res.items[0].media_id).toBe(42)
  })

  it('toggle add/remove and check use the supplied entity type (default movie only when omitted)', async () => {
    mockApi.post.mockResolvedValue({ data: {} })
    mockApi.delete.mockResolvedValue({ data: {} })
    mockApi.get.mockResolvedValue({ data: { is_favorite: true } })
    await favoritesApi.toggleFavorite({ media_id: 9, is_favorite: true, entity_type: 'song' })
    expect(mockApi.post).toHaveBeenCalledWith('/favorites', { entity_type: 'song', entity_id: 9 })
    await favoritesApi.toggleFavorite({ media_id: 9, is_favorite: false, entity_type: 'song' })
    expect(mockApi.delete).toHaveBeenCalledWith('/favorites/song/9')
    await favoritesApi.checkFavorite(9, 'song')
    expect(String(mockApi.get.mock.calls.at(-1)![0])).toBe('/favorites/check/song/9')
    await favoritesApi.toggleFavorite({ media_id: 9, is_favorite: true })
    expect(mockApi.post).toHaveBeenLastCalledWith('/favorites', { entity_type: 'movie', entity_id: 9 })
  })
})
