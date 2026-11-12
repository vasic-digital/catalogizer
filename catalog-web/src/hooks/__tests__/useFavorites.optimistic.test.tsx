// FAVR3-1/2 (PROVISIONAL): the optimistic update + rollback must act on the LIVE query key
// ['favorites', params], not the dead exact key ['favorites'].
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useFavorites } from '../useFavorites'
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
const PARAMS = { limit: 20 }
const LIST = { items: [fav('movie', 7), fav('song', 7), fav('movie', 8)], total: 3, limit: 20, offset: 0 }

const setup = () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  )
  return { qc, wrapper }
}
const ids = (qc: QueryClient) =>
  ((qc.getQueryData(['favorites', PARAMS]) as typeof LIST).items).map((f) => `${f.media_item.media_type}-${f.media_id}`)

let release: (v?: unknown) => void
let fail: (e?: unknown) => void

describe('useFavorites optimistic update on the live key', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getFavorites.mockResolvedValue(LIST as any)
    api.getFavoriteStats.mockResolvedValue({ total_count: 3, media_type_breakdown: {}, recent_additions: [] } as any)
    api.toggleFavorite.mockImplementation(() => new Promise((res, rej) => { release = res; fail = rej }) as any)
  })
  const mount = async () => {
    const s = setup()
    const h = renderHook(() => useFavorites(PARAMS), { wrapper: s.wrapper })
    await waitFor(() => expect(h.result.current.favorites).toHaveLength(3))
    return { ...s, h }
  }

  it('removal with a type is visible in the cache BEFORE the server answers', async () => {
    const { qc, h } = await mount()
    act(() => h.result.current.toggleFavorite(7, true, 'movie'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(ids(qc)).toEqual(['song-7', 'movie-8'])
    await act(async () => { release({ is_favorite: false }) })
  })

  it('legacy removal without a type removes every row with that media_id', async () => {
    const { qc, h } = await mount()
    act(() => h.result.current.toggleFavorite(7, true))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(ids(qc)).toEqual(['movie-8'])
    await act(async () => { release({ is_favorite: false }) })
  })

  it('onError rolls the live-key list back to the snapshot', async () => {
    const { qc, h } = await mount()
    api.getFavorites.mockImplementation(() => new Promise(() => {}) as any) // keep refetch from masking rollback
    act(() => h.result.current.toggleFavorite(7, true, 'movie'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(ids(qc)).toEqual(['song-7', 'movie-8'])
    await act(async () => { fail(new Error('boom')) })
    await waitFor(() => expect(ids(qc)).toEqual(['movie-7', 'song-7', 'movie-8']))
  })

  it('adding leaves the cached list untouched', async () => {
    const { qc, h } = await mount()
    act(() => h.result.current.toggleFavorite(7, false, 'movie'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(ids(qc)).toEqual(['movie-7', 'song-7', 'movie-8'])
    await act(async () => { release({ is_favorite: true }) })
  })

  it('settle invalidates the live key (list refetched)', async () => {
    const { h } = await mount()
    const before = api.getFavorites.mock.calls.length
    act(() => h.result.current.toggleFavorite(7, true, 'movie'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    await act(async () => { release({ is_favorite: false }) })
    await waitFor(() => expect(api.getFavorites.mock.calls.length).toBeGreaterThan(before))
  })

  it('does not touch the unrelated favorites-stats entry optimistically', async () => {
    const { qc, h } = await mount()
    const statsBefore = qc.getQueryData(['favorites-stats'])
    act(() => h.result.current.toggleFavorite(7, true, 'movie'))
    await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
    expect(qc.getQueryData(['favorites-stats'])).toBe(statsBefore)
    await act(async () => { release({ is_favorite: false }) })
  })
})
