import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useFavorites, useFavoriteStatus } from '../useFavorites'
import { favoritesApi } from '@/lib/favoritesApi'

// Mock favoritesApi
vi.mock('@/lib/favoritesApi', async () => ({
  favoritesApi: {
    getFavorites: vi.fn(),
    getFavoriteStats: vi.fn(),
    toggleFavorite: vi.fn(),
    checkFavorite: vi.fn(),
    bulkAddToFavorites: vi.fn(),
    bulkRemoveFromFavorites: vi.fn(),
  },
}))

// Mock react-hot-toast
const { mockToast } = vi.hoisted(() => ({
  mockToast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('react-hot-toast', () => ({
  __esModule: true,
  default: mockToast,
}))

const mockFavoritesApi = vi.mocked(favoritesApi)

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        cacheTime: 0,
      },
      mutations: {
        retry: false,
      },
    },
  })

  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  return { Wrapper, queryClient }
}

const mockFavorite = {
  id: 'fav-1',
  user_id: 1,
  media_id: 42,
  media_item: {
    id: 42,
    title: 'Test Movie',
    media_type: 'movie',
    year: 2024,
    rating: 8.5,
  },
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockFavoritesResponse = {
  items: [mockFavorite],
  total: 1,
  limit: 20,
  offset: 0,
}

const mockStats = {
  total_count: 5,
  media_type_breakdown: {
    movie: 3,
    tv_show: 1,
    music: 1,
    game: 0,
    documentary: 0,
    anime: 0,
    concert: 0,
    other: 0,
  },
  recent_additions: [mockFavorite],
}

describe('useFavorites', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockFavoritesApi.getFavorites.mockResolvedValue(mockFavoritesResponse)
    mockFavoritesApi.getFavoriteStats.mockResolvedValue(mockStats)
  })

  describe('Fetching favorites', () => {
    it('fetches favorites and stats on mount', async () => {
      const { Wrapper } = createWrapper()

      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      // Initially loading
      expect(result.current.isLoading).toBe(true)

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(result.current.favorites).toEqual([mockFavorite])
      expect(result.current.total).toBe(1)
      expect(result.current.stats).toEqual(mockStats)
    })

    it('passes params to getFavorites', async () => {
      const { Wrapper } = createWrapper()
      const params = { limit: 10, offset: 0, media_type: 'movie' }

      renderHook(() => useFavorites(params), { wrapper: Wrapper })

      await waitFor(() => {
        expect(mockFavoritesApi.getFavorites).toHaveBeenCalledWith(params)
      })
    })

    it('returns empty array when no favorites data', async () => {
      mockFavoritesApi.getFavorites.mockResolvedValue(null as any)
      mockFavoritesApi.getFavoriteStats.mockResolvedValue(null as any)

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(result.current.favorites).toEqual([])
      expect(result.current.total).toBe(0)
    })

    it('exposes error when fetch fails', async () => {
      const error = new Error('Network error')
      mockFavoritesApi.getFavorites.mockRejectedValue(error)

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.error).toBeTruthy()
      })
    })
  })

  describe('checkFavoriteStatus', () => {
    it('returns true when media is in favorites', async () => {
      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(result.current.checkFavoriteStatus(42)).toBe(true)
    })

    it('returns false when media is not in favorites', async () => {
      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(result.current.checkFavoriteStatus(999)).toBe(false)
    })

    it('returns false when favorites data is empty', async () => {
      mockFavoritesApi.getFavorites.mockResolvedValue({ items: [], total: 0 })

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(result.current.checkFavoriteStatus(42)).toBe(false)
    })
  })

  describe('toggleFavorite', () => {
    it('calls toggleFavorite mutation to remove a favorite', async () => {
      mockFavoritesApi.toggleFavorite.mockResolvedValue({ is_favorite: false })

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.toggleFavorite(42)
      })

      await waitFor(() => {
        expect(mockFavoritesApi.toggleFavorite).toHaveBeenCalledWith({
          media_id: 42,
          is_favorite: false,
        })
      })
    })

    it('calls toggleFavorite mutation to add a favorite when not in list', async () => {
      mockFavoritesApi.toggleFavorite.mockResolvedValue({ is_favorite: true })

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.toggleFavorite(999)
      })

      await waitFor(() => {
        expect(mockFavoritesApi.toggleFavorite).toHaveBeenCalledWith({
          media_id: 999,
          is_favorite: true,
        })
      })
    })

    it('CAT-001 R5: a rejected add shows only the error toast, never "Added to favorites"', async () => {
      mockToast.success.mockClear(); mockToast.error.mockClear()
      mockFavoritesApi.toggleFavorite.mockRejectedValue(new Error('nope'))
      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })
      await waitFor(() => { expect(result.current.isLoading).toBe(false) })
      act(() => { result.current.toggleFavorite(999) })
      await waitFor(() => { expect(mockToast.error).toHaveBeenCalledWith('Failed to update favorite status') })
      expect(mockToast.success).not.toHaveBeenCalledWith('Added to favorites')
    })

    it('CAT-001 R5: "Added to favorites" is toasted only after the server accepted the add', async () => {
      mockToast.success.mockClear()
      let release: (v: unknown) => void = () => {}
      mockFavoritesApi.toggleFavorite.mockReturnValue(new Promise(r => { release = r }) as any)
      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })
      await waitFor(() => { expect(result.current.isLoading).toBe(false) })
      act(() => { result.current.toggleFavorite(999) })
      await waitFor(() => { expect(mockFavoritesApi.toggleFavorite).toHaveBeenCalled() })
      expect(mockToast.success).not.toHaveBeenCalled()
      release({ is_favorite: true })
      await waitFor(() => { expect(mockToast.success).toHaveBeenCalledWith('Added to favorites') })
    })

    it('respects explicit currentStatus parameter', async () => {
      mockFavoritesApi.toggleFavorite.mockResolvedValue({ is_favorite: false })

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.toggleFavorite(42, true)
      })

      await waitFor(() => {
        expect(mockFavoritesApi.toggleFavorite).toHaveBeenCalledWith({
          media_id: 42,
          is_favorite: false,
        })
      })
    })

    it('shows error toast on toggle failure', async () => {
      mockFavoritesApi.toggleFavorite.mockRejectedValue(new Error('Server error'))

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.toggleFavorite(42)
      })

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Failed to update favorite status')
      })
    })
  })

  describe('bulkAddToFavorites', () => {
    it('calls bulkAddToFavorites API and shows success toast', async () => {
      mockFavoritesApi.bulkAddToFavorites.mockResolvedValue({ added: 3, failed: 0 })

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.bulkAddToFavorites([1, 2, 3])
      })

      await waitFor(() => {
        expect(mockFavoritesApi.bulkAddToFavorites).toHaveBeenCalledWith([1, 2, 3])
        expect(mockToast.success).toHaveBeenCalledWith('Added 3 items to favorites')
      })
    })

    it('shows error toast on bulk add failure', async () => {
      mockFavoritesApi.bulkAddToFavorites.mockRejectedValue(new Error('Server error'))

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.bulkAddToFavorites([1, 2, 3])
      })

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Failed to add items to favorites')
      })
    })
  })

  describe('bulkRemoveFromFavorites', () => {
    it('calls bulkRemoveFromFavorites API and shows success toast', async () => {
      mockFavoritesApi.bulkRemoveFromFavorites.mockResolvedValue({ removed: 2, failed: 0 })

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.bulkRemoveFromFavorites([1, 2])
      })

      await waitFor(() => {
        expect(mockFavoritesApi.bulkRemoveFromFavorites).toHaveBeenCalledWith([1, 2])
        expect(mockToast.success).toHaveBeenCalledWith('Removed 2 items from favorites')
      })
    })

    it('shows error toast on bulk remove failure', async () => {
      mockFavoritesApi.bulkRemoveFromFavorites.mockRejectedValue(new Error('Server error'))

      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      act(() => {
        result.current.bulkRemoveFromFavorites([1, 2])
      })

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Failed to remove items from favorites')
      })
    })
  })

  describe('refetch functions', () => {
    it('exposes refetchFavorites function', async () => {
      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(typeof result.current.refetchFavorites).toBe('function')
    })

    it('exposes refetchStats function', async () => {
      const { Wrapper } = createWrapper()
      const { result } = renderHook(() => useFavorites(), { wrapper: Wrapper })

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false)
      })

      expect(typeof result.current.refetchStats).toBe('function')
    })
  })
})

describe('useFavoriteStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches favorite status for a media ID', async () => {
    mockFavoritesApi.checkFavorite.mockResolvedValue({ is_favorite: true })

    const { Wrapper } = createWrapper()
    const { result } = renderHook(() => useFavoriteStatus(42), { wrapper: Wrapper })

    await waitFor(() => {
      expect(result.current.data).toEqual({ is_favorite: true })
    })

    expect(mockFavoritesApi.checkFavorite).toHaveBeenCalledWith(42)
  })

  it('does not fetch when mediaId is falsy', () => {
    const { Wrapper } = createWrapper()
    renderHook(() => useFavoriteStatus(0), { wrapper: Wrapper })

    expect(mockFavoritesApi.checkFavorite).not.toHaveBeenCalled()
  })
})
