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
  describe('FAVR3R2-1 / FAVR3-8 / FAVR3-9 (PROVISIONAL)', () => {
    const SORTED = { sort_by: 'title' as const }
    const REVERSED = { ...LIST, items: [...LIST.items].reverse() }
    const idsAt = (qc: QueryClient, key: unknown[]) =>
      ((qc.getQueryData(key) as typeof LIST).items).map((f) => `${f.media_item.media_type}-${f.media_id}`)
    const mountDistinct = async () => {
      const s = setup()
      // the two cached keys hold DIFFERENT lists (reversed), so a blanket restore of one snapshot is detectable
      api.getFavorites.mockImplementation(async (p: any) => (p?.sort_by ? REVERSED : LIST) as any)
      const h = renderHook(() => ({ a: useFavorites(), b: useFavorites(SORTED) }), { wrapper: s.wrapper })
      await waitFor(() => { expect(h.result.current.a.favorites).toHaveLength(3); expect(h.result.current.b.favorites).toHaveLength(3) })
      return { ...s, h }
    }

    it('FAVR3R2-1: after a failed toggle each key is restored to its OWN snapshot (distinct lists)', async () => {
      const { qc, h } = await mountDistinct()
      api.getFavorites.mockImplementation(() => new Promise(() => {}) as any)
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => {
        expect(idsAt(qc, ['favorites', undefined])).toEqual(['movie-7', 'song-7', 'movie-8'])
        expect(idsAt(qc, ['favorites', SORTED])).toEqual(['movie-8', 'song-7', 'movie-7'])
      })
    })

    it('FAVR3-9: optimistic removal decrements total, rollback restores it', async () => {
      const { qc, h } = await mountDistinct()
      api.getFavorites.mockImplementation(() => new Promise(() => {}) as any)
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect((qc.getQueryData(['favorites', undefined]) as typeof LIST).total).toBe(2)
      expect((qc.getQueryData(['favorites', SORTED]) as typeof LIST).total).toBe(2)
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => expect((qc.getQueryData(['favorites', undefined]) as typeof LIST).total).toBe(3))
    })

    it('FAVR3-8: rollback never duplicates a row the cache already holds again', async () => {
      const { qc, h } = await mountDistinct()
      api.getFavorites.mockImplementation(() => new Promise(() => {}) as any)
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      act(() => { qc.setQueryData(['favorites', undefined], LIST) }) // row came back (e.g. server data landed)
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(idsAt(qc, ['favorites', undefined])).toEqual(['movie-7', 'song-7', 'movie-8'])
      expect((qc.getQueryData(['favorites', undefined]) as typeof LIST).total).toBe(3)
    })

    it('FAVR3-8: a failed toggle does not resurrect a row removed by another in-flight toggle', async () => {
      const { qc, h } = await mountDistinct()
      api.getFavorites.mockImplementation(() => new Promise(() => {}) as any)
      const calls: { res: (v?: unknown) => void; rej: (e?: unknown) => void }[] = []
      api.toggleFavorite.mockImplementation(() => new Promise((res, rej) => { calls.push({ res, rej }) }) as any)
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(calls).toHaveLength(1))
      act(() => h.result.current.a.toggleFavorite(8, true, 'movie'))
      await waitFor(() => expect(calls).toHaveLength(2))
      expect(idsAt(qc, ['favorites', undefined])).toEqual(['song-7'])
      await act(async () => { calls[0].rej(new Error('boom')) })
      await waitFor(() => expect(idsAt(qc, ['favorites', undefined])).toContain('movie-7'))
      expect(idsAt(qc, ['favorites', undefined])).toEqual(['movie-7', 'song-7'])
      expect(idsAt(qc, ['favorites', SORTED])).toEqual(['song-7', 'movie-7'])
      expect((qc.getQueryData(['favorites', undefined]) as typeof LIST).total).toBe(2)
      await act(async () => { calls[1].res({ is_favorite: false }) })
    })
  })

  describe('several useFavorites instances (FavoriteToggle + FavoritesGrid shape) in one QueryClient', () => {
    const SORTED = { sort_by: 'title' as const }
    const mountTwo = async () => {
      const s = setup()
      api.getFavorites.mockImplementation(async () => LIST as any)
      const h = renderHook(() => ({ a: useFavorites(), b: useFavorites(SORTED) }), { wrapper: s.wrapper })
      await waitFor(() => { expect(h.result.current.a.favorites).toHaveLength(3); expect(h.result.current.b.favorites).toHaveLength(3) })
      return { ...s, h }
    }
    const idsAt = (qc: QueryClient, key: unknown[]) =>
      ((qc.getQueryData(key) as typeof LIST).items).map((f) => `${f.media_item.media_type}-${f.media_id}`)

    it('toggling from the no-params instance updates the OTHER key before the server answers', async () => {
      const { qc, h } = await mountTwo()
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(idsAt(qc, ['favorites', undefined])).toEqual(['song-7', 'movie-8'])
      expect(idsAt(qc, ['favorites', SORTED])).toEqual(['song-7', 'movie-8'])
      expect(h.result.current.b.favorites).toHaveLength(2)
      await act(async () => { release({ is_favorite: false }) })
    })

    it('after a failed mutation BOTH keys are restored', async () => {
      const { qc, h } = await mountTwo()
      api.getFavorites.mockImplementation(() => new Promise(() => {}) as any)
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(idsAt(qc, ['favorites', SORTED])).toEqual(['song-7', 'movie-8'])
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => {
        expect(idsAt(qc, ['favorites', undefined])).toEqual(['movie-7', 'song-7', 'movie-8'])
        expect(idsAt(qc, ['favorites', SORTED])).toEqual(['movie-7', 'song-7', 'movie-8'])
      })
    })

    it('a refetch pending when the toggle starts is cancelled and cannot overwrite the optimistic list', async () => {
      const { qc, h } = await mountTwo()
      let resolveRefetch: (v: unknown) => void = () => {}
      api.getFavorites.mockImplementation(() => new Promise((res) => { resolveRefetch = res }) as any)
      act(() => { void qc.invalidateQueries(['favorites']) })
      await waitFor(() => expect(api.getFavorites).toHaveBeenCalledTimes(4))
      act(() => h.result.current.a.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      await act(async () => { resolveRefetch(LIST) })
      expect(idsAt(qc, ['favorites', undefined])).toEqual(['song-7', 'movie-8'])
      expect(idsAt(qc, ['favorites', SORTED])).toEqual(['song-7', 'movie-8'])
      await act(async () => { release({ is_favorite: false }) })
    })
  })
  describe('FAVR3R3FU-1/2: rollback on inconsistent cached data (PROVISIONAL)', () => {
    const mountWith = async (list: any) => {
      const s = setup()
      api.getFavorites.mockImplementation(async () => list as any)
      const h = renderHook(() => useFavorites(), { wrapper: s.wrapper })
      await waitFor(() => expect(h.result.current.favorites).toHaveLength(list.items.length))
      api.getFavorites.mockImplementation(() => new Promise(() => {}) as any)
      return { ...s, h }
    }
    const totalAt = (qc: QueryClient) => (qc.getQueryData(['favorites', undefined]) as any).total
    const rowsAt = (qc: QueryClient) => ((qc.getQueryData(['favorites', undefined]) as any).items).map((f: any) => f.id)

    it('P1: list with items but total 0 ends at 0 after rollback (not 1)', async () => {
      const { qc, h } = await mountWith({ items: [fav('movie', 7), fav('movie', 8)], total: 0, limit: 20, offset: 0 })
      act(() => h.result.current.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(totalAt(qc)).toBe(0)
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => expect(rowsAt(qc)).toEqual(['movie-7', 'movie-8']))
      expect(totalAt(qc)).toBe(0)
    })

    it('P1b: clamp case, 2 matching rows but total 1: optimistic 0 (never negative), rollback restores exactly 1', async () => {
      const dup = fav('movie', 7)
      const { qc, h } = await mountWith({ items: [dup, { ...dup, id: 'movie-7b' }, fav('movie', 8)], total: 1, limit: 20, offset: 0 })
      act(() => h.result.current.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(totalAt(qc)).toBe(0)
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => expect(rowsAt(qc)).toHaveLength(3))
      expect(totalAt(qc)).toBe(1)
    })

    it('P2: duplicate rows with the same id are BOTH restored on rollback', async () => {
      const a = fav('movie', 7)
      const { qc, h } = await mountWith({ items: [a, { ...a }, fav('movie', 8)], total: 3, limit: 20, offset: 0 })
      act(() => h.result.current.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(rowsAt(qc)).toEqual(['movie-8'])
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => expect(rowsAt(qc)).toEqual(['movie-7', 'movie-7', 'movie-8']))
      expect(totalAt(qc)).toBe(3)
    })

    it('a row already back in the cache is still not duplicated (dedup against ids present before the loop)', async () => {
      const { qc, h } = await mountWith({ items: [fav('movie', 7), fav('movie', 8)], total: 2, limit: 20, offset: 0 })
      act(() => h.result.current.toggleFavorite(7, true, 'movie'))
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      act(() => { qc.setQueryData(['favorites', undefined], { items: [fav('movie', 7), fav('movie', 8)], total: 2 }) })
      await act(async () => { fail(new Error('boom')) })
      await waitFor(() => expect(api.toggleFavorite).toHaveBeenCalled())
      expect(rowsAt(qc)).toEqual(['movie-7', 'movie-8'])
      expect(totalAt(qc)).toBe(2)
    })
  })
})
