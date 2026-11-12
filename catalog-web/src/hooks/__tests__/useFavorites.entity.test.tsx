// CAT-UI-FAV-1b: the hook must carry entity_type to the API and keep per-type cache identity.
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useFavorites, useFavoriteStatus } from '../useFavorites'
import { favoritesApi } from '@/lib/favoritesApi'

vi.mock('@/lib/favoritesApi', () => ({
  favoritesApi: {
    getFavorites: vi.fn(), getFavoriteStats: vi.fn(), toggleFavorite: vi.fn(), checkFavorite: vi.fn(),
    bulkAddToFavorites: vi.fn(), bulkRemoveFromFavorites: vi.fn(),
  },
}))
vi.mock('react-hot-toast', () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))

const api = vi.mocked(favoritesApi)
const fav = (type: string, id: number) => ({
  id: `${type}-${id}`, user_id: 1, media_id: id,
  media_item: { id, title: 't', media_type: type }, created_at: '', updated_at: '',
})

const setup = () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  )
  return { qc, wrapper }
}

describe('useFavorites entity targeting', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getFavorites.mockResolvedValue({ items: [], total: 0, limit: 0, offset: 0 })
    api.getFavoriteStats.mockResolvedValue({ total_count: 0, media_type_breakdown: {}, recent_additions: [] } as any)
    api.toggleFavorite.mockResolvedValue({ is_favorite: true })
    api.checkFavorite.mockResolvedValue({ is_favorite: false })
  })

  it('toggleFavorite(900,false,"song") reaches the API with entity_type', async () => {
    const { wrapper } = setup()
    const { result } = renderHook(() => useFavorites(), { wrapper })
    act(() => result.current.toggleFavorite(900, false, 'song'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(api.toggleFavorite.mock.calls[0][0]).toEqual({ media_id: 900, is_favorite: true, entity_type: 'song' })
  })

  it('without a type the request carries no entity_type key', async () => {
    const { wrapper } = setup()
    const { result } = renderHook(() => useFavorites(), { wrapper })
    act(() => result.current.toggleFavorite(5, false))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(api.toggleFavorite.mock.calls[0][0]).toEqual({ media_id: 5, is_favorite: true })
  })

  it('favorite-status uses a distinct cache key per entity type for the same id', async () => {
    const { qc, wrapper } = setup()
    renderHook(() => ({ a: useFavoriteStatus(900, 'movie'), b: useFavoriteStatus(900, 'song') }), { wrapper })
    await waitFor(() => expect(api.checkFavorite).toHaveBeenCalledTimes(2))
    expect(qc.getQueryData(['favorite-status', 'movie', 900])).toBeDefined()
    expect(qc.getQueryData(['favorite-status', 'song', 900])).toBeDefined()
    expect(api.checkFavorite).toHaveBeenCalledWith(900, 'movie')
    expect(api.checkFavorite).toHaveBeenCalledWith(900, 'song')
  })

  it('optimistic removal of song 900 keeps movie 900 (id collision across types)', async () => {
    const { qc, wrapper } = setup()
    const { result } = renderHook(() => useFavorites(), { wrapper })
    let release: (v: any) => void = () => undefined
    api.toggleFavorite.mockReturnValue(new Promise((r) => { release = r }))
    qc.setQueryData(['favorites'], { items: [fav('movie', 900), fav('song', 900)] })
    act(() => result.current.toggleFavorite(900, true, 'song'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    const items = (qc.getQueryData(['favorites']) as any).items
    expect(items.map((f: any) => f.media_item.media_type)).toEqual(['movie'])
    release({ is_favorite: false })
  })

  it('checkFavoriteStatus distinguishes types with the same id', async () => {
    api.getFavorites.mockResolvedValue({ items: [fav('movie', 900)], total: 1, limit: 1, offset: 0 } as any)
    const { wrapper } = setup()
    const { result } = renderHook(() => useFavorites(), { wrapper })
    await waitFor(() => expect(result.current.favorites.length).toBe(1))
    expect(result.current.checkFavoriteStatus(900, 'movie')).toBe(true)
    expect(result.current.checkFavoriteStatus(900, 'song')).toBe(false)
  })

  it('toggle settle invalidates favorite-status', async () => {
    const { qc, wrapper } = setup()
    const spy = vi.spyOn(qc, 'invalidateQueries')
    const { result } = renderHook(() => useFavorites(), { wrapper })
    act(() => result.current.toggleFavorite(1, false, 'song'))
    await waitFor(() => expect(spy.mock.calls.some((c) => JSON.stringify(c[0]) === JSON.stringify(['favorite-status']))).toBe(true))
  })
})
