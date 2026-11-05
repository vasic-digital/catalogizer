import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { FavoritesGrid } from '../FavoritesGrid'

const renderWithRouter = (ui: React.ReactElement) => render(<MemoryRouter>{ui}</MemoryRouter>)

const mockToggleFavorite = vi.fn()
const mockFavorites = [
  {
    id: 1,
    media_item: {
      id: 101,
      title: 'The Matrix',
      media_type: 'movie',
      year: 1999,
      cover_image: '/img/matrix.jpg',
      duration: 136,
      rating: 4.5,
      quality: '1080p',
    },
    created_at: '2024-01-15T10:00:00Z',
    updated_at: '2024-01-15T10:00:00Z',
  },
  {
    id: 2,
    media_item: {
      id: 102,
      title: 'Dark Side of the Moon',
      media_type: 'music',
      year: 1973,
      cover_image: '/img/dsotm.jpg',
      duration: 43,
      rating: 5,
      quality: 'FLAC',
    },
    created_at: '2024-01-16T10:00:00Z',
    updated_at: '2024-01-16T10:00:00Z',
  },
]

const mockStats = {
  total_count: 42,
  media_type_breakdown: {
    movie: 20,
    tv_show: 10,
    music: 8,
    game: 2,
    documentary: 1,
    anime: 1,
    concert: 0,
    other: 0,
  },
  recent_additions: [],
}

vi.mock('@/hooks/useFavorites', () => ({
  useFavorites: vi.fn(() => ({
    favorites: mockFavorites,
    total: 2,
    isLoading: false,
    error: null,
    stats: mockStats,
    toggleFavorite: mockToggleFavorite,
    bulkRemoveFromFavorites: vi.fn(),
  })),
}))

vi.mock('@/components/media/MediaGrid', () => ({
  MediaGrid: ({ media, viewMode }: any) => (
    <div data-testid="media-grid" data-view-mode={viewMode}>
      {media.map((item: any) => (
        <div key={item.id} data-testid="media-item">
          {item.title}
        </div>
      ))}
    </div>
  ),
}))

vi.mock('../FavoriteToggle', () => ({
  FavoriteToggle: ({ mediaId }: any) => (
    <button data-testid={`fav-toggle-${mediaId}`}>Toggle</button>
  ),
}))

vi.mock('@/lib/utils', () => ({
  cn: (...classes: any[]) => classes.filter(Boolean).join(' '),
}))

vi.mock('@/lib/mediaApi', () => ({
  mediaApi: {
    downloadMedia: vi.fn(),
  },
}))

describe('FavoritesGrid', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the favorites grid', () => {
    renderWithRouter(<FavoritesGrid />)
    expect(screen.getByText(/Favorites/)).toBeInTheDocument()
  })

  it('shows stats section by default', () => {
    renderWithRouter(<FavoritesGrid />)
    expect(screen.getByText('Total')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('shows movie count in stats', () => {
    renderWithRouter(<FavoritesGrid />)
    // "Movies" appears both in the stats section and in the filter select options
    const moviesElements = screen.getAllByText('Movies')
    expect(moviesElements.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('20')).toBeInTheDocument()
  })

  it('shows TV show count in stats', () => {
    renderWithRouter(<FavoritesGrid />)
    // "TV Shows" appears both in the stats section and in the filter select options
    const tvShowElements = screen.getAllByText('TV Shows')
    expect(tvShowElements.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('10')).toBeInTheDocument()
  })

  it('shows music count in stats', () => {
    renderWithRouter(<FavoritesGrid />)
    // "Music" appears both in the stats section and in the filter select options
    const musicElements = screen.getAllByText('Music')
    expect(musicElements.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('8')).toBeInTheDocument()
  })

  it('hides stats section when showStats is false', () => {
    renderWithRouter(<FavoritesGrid showStats={false} />)
    expect(screen.queryByText('Total')).not.toBeInTheDocument()
  })

  it('shows filters section by default', () => {
    renderWithRouter(<FavoritesGrid />)
    expect(screen.getByText('Filters & Search')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Search favorites...')).toBeInTheDocument()
  })

  it('hides filters section when showFilters is false', () => {
    renderWithRouter(<FavoritesGrid showFilters={false} />)
    expect(screen.queryByText('Filters & Search')).not.toBeInTheDocument()
  })

  it('renders MediaGrid with favorite items', () => {
    renderWithRouter(<FavoritesGrid />)
    expect(screen.getByTestId('media-grid')).toBeInTheDocument()
    expect(screen.getByText('The Matrix')).toBeInTheDocument()
    expect(screen.getByText('Dark Side of the Moon')).toBeInTheDocument()
  })

  it('shows favorites count in header', () => {
    renderWithRouter(<FavoritesGrid />)
    expect(screen.getByText('Favorites (2)')).toBeInTheDocument()
  })

  it('renders in grid view mode by default', () => {
    renderWithRouter(<FavoritesGrid />)
    const grid = screen.getByTestId('media-grid')
    expect(grid).toHaveAttribute('data-view-mode', 'grid')
  })

  it('does not show Select All button when not selectable', () => {
    renderWithRouter(<FavoritesGrid selectable={false} />)
    expect(screen.queryByText('Select All')).not.toBeInTheDocument()
  })

  it('shows Select All button when selectable', () => {
    renderWithRouter(<FavoritesGrid selectable={true} />)
    expect(screen.getByText('Select All')).toBeInTheDocument()
  })

  describe('error state', () => {
    it('shows error message when favorites fail to load', async () => {
      const { useFavorites } = await import('@/hooks/useFavorites')
      vi.mocked(useFavorites).mockReturnValue({
        favorites: [],
        total: 0,
        isLoading: false,
        error: new Error('Failed to fetch'),
        stats: null,
        toggleFavorite: vi.fn(),
        bulkRemoveFromFavorites: vi.fn(),
      } as any)

      renderWithRouter(<FavoritesGrid />)
      expect(screen.getByText('Failed to load favorites')).toBeInTheDocument()
    })
  })

  describe('loading state', () => {
    it('shows loading skeletons when loading', async () => {
      const { useFavorites } = await import('@/hooks/useFavorites')
      vi.mocked(useFavorites).mockReturnValue({
        favorites: [],
        total: 0,
        isLoading: true,
        error: null,
        stats: mockStats,
        toggleFavorite: vi.fn(),
        bulkRemoveFromFavorites: vi.fn(),
      } as any)

      const { container } = renderWithRouter(<FavoritesGrid />)
      const skeletons = container.querySelectorAll('.animate-pulse')
      expect(skeletons.length).toBeGreaterThan(0)
    })
  })

  describe('empty state', () => {
    it('shows empty state when no favorites', async () => {
      const { useFavorites } = await import('@/hooks/useFavorites')
      vi.mocked(useFavorites).mockReturnValue({
        favorites: [],
        total: 0,
        isLoading: false,
        error: null,
        stats: mockStats,
        toggleFavorite: vi.fn(),
        bulkRemoveFromFavorites: vi.fn(),
      } as any)

      renderWithRouter(<FavoritesGrid />)
      expect(screen.getByText('No favorites yet')).toBeInTheDocument()
      expect(
        screen.getByText('Start adding items to your favorites to see them here')
      ).toBeInTheDocument()
    })
  })

  describe('search', () => {
    it('renders search input', () => {
      renderWithRouter(<FavoritesGrid />)
      expect(screen.getByPlaceholderText('Search favorites...')).toBeInTheDocument()
    })
  })
})

describe('FavoritesGrid accessible names (CAT-UI-D5D6)', () => {
  it('names the filter and sort selects by purpose', () => {
    renderWithRouter(<FavoritesGrid />)
    expect(screen.getByRole('combobox', { name: 'Filter by media type' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Sort favorites' })).toBeInTheDocument()
  })
})
