// CAT-UI-FAV-1: every favourite must open the route the real router config
// (App.tsx: <Route path="/entity/:id">) serves, per entity type, and a favourite
// without a usable entity id must not navigate to a bogus entity.
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import { FavoritesGrid } from '../FavoritesGrid'
import { useFavorites } from '@/hooks/useFavorites'
import { MEDIA_TYPES } from '@/types/media'
import toast from 'react-hot-toast'

vi.mock('react-hot-toast', () => ({ default: { error: vi.fn(), success: vi.fn() } }))

vi.mock('@/hooks/useFavorites', () => ({ useFavorites: vi.fn() }))
vi.mock('@/components/media/MediaGrid', () => ({
  MediaGrid: ({ media, onMediaView, onMediaPlay }: any) => (
    <div>
      {media.map((item: any) => (
        <div key={`${item.media_type}-${item.id}`}>
          <button data-testid={`view-${item.media_type}`} onClick={() => onMediaView(item)}>v</button>
          <button data-testid={`play-${item.media_type}`} onClick={() => onMediaPlay(item)}>p</button>
        </div>
      ))}
    </div>
  ),
}))
vi.mock('../FavoriteToggle', () => ({ FavoriteToggle: () => null }))
vi.mock('@/lib/mediaApi', () => ({ mediaApi: { downloadMedia: vi.fn() } }))

const Loc = () => {
  const l = useLocation()
  return <div data-testid="loc">{l.pathname + l.search}</div>
}

const fav = (type: string, id: number) => ({
  id: `${type}-${id}`,
  user_id: 1,
  media_id: id,
  media_item: { id, title: `${type} #${id}`, media_type: type },
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
})

const setFavs = (items: any[]) =>
  vi.mocked(useFavorites).mockReturnValue({
    favorites: items, total: items.length, isLoading: false, error: null,
    stats: undefined, toggleFavorite: vi.fn(), bulkRemoveFromFavorites: vi.fn(),
  } as any)

const renderGrid = () =>
  render(
    <MemoryRouter initialEntries={['/favorites']}>
      <Routes>
        <Route path="/favorites" element={<FavoritesGrid showStats={false} showFilters={false} />} />
        <Route path="/entity/:id" element={<Loc />} />
      </Routes>
    </MemoryRouter>,
  )

describe('FavoritesGrid entity navigation (CAT-UI-FAV-1)', () => {
  beforeEach(() => vi.clearAllMocks())
  it.each(MEDIA_TYPES.map((t, i) => [t, 200 + i] as const))(
    'view of a %s favourite opens /entity/<entity_id>', (type, id) => {
      setFavs([fav(type, id)])
      renderGrid()
      fireEvent.click(screen.getByTestId(`view-${type}`))
      expect(screen.getByTestId('loc').textContent).toBe(`/entity/${id}`)
    })

  it.each(MEDIA_TYPES.map((t, i) => [t, 300 + i] as const))(
    'play of a %s favourite opens /entity/<entity_id>?autoplay=true', (type, id) => {
      setFavs([fav(type, id)])
      renderGrid()
      fireEvent.click(screen.getByTestId(`play-${type}`))
      expect(screen.getByTestId('loc').textContent).toBe(`/entity/${id}?autoplay=true`)
    })

  it('two favourites route each to its own id (no swapped ids)', () => {
    setFavs([fav('movie', 11), fav('song', 22)])
    renderGrid()
    fireEvent.click(screen.getByTestId('view-song'))
    expect(screen.getByTestId('loc').textContent).toBe('/entity/22')
  })

  it.each([[0], [-5], [Number.NaN], [1.5], [1e21], [2 ** 53]])(
    'a favourite with unusable entity id %s: no navigation, visible feedback, warning with favourite id', (bad) => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
      setFavs([fav('other', bad)])
      renderGrid()
      fireEvent.click(screen.getByTestId('view-other'))
      fireEvent.click(screen.getByTestId('play-other'))
      expect(screen.queryByTestId('loc')).toBeNull()
      expect(toast.error).toHaveBeenCalledTimes(2)
      expect(vi.mocked(toast.error).mock.calls[0][0]).toMatch(/no valid item id/i)
      expect(warn.mock.calls[0][1]).toMatchObject({ favoriteId: 'other-' + bad })
      warn.mockRestore()
    })

  it('valid id gives no error feedback', () => {
    setFavs([fav('movie', 5)])
    renderGrid()
    fireEvent.click(screen.getByTestId('view-movie'))
    expect(toast.error).not.toHaveBeenCalled()
  })
})
