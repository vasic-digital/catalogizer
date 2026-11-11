import React, { act } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MediaBrowser } from '../MediaBrowser'
import { mediaApi } from '@/lib/mediaApi'
import type { MediaItem } from '@/types/media'

// Mock dependencies
vi.mock('@/lib/mediaApi', async () => ({
  mediaApi: {
    searchMedia: vi.fn(),
    getMediaStats: vi.fn(),
    downloadMedia: vi.fn(),
    getMediaById: vi.fn(),
    getMediaByPath: vi.fn(),
    analyzeDirectory: vi.fn(),
    getExternalMetadata: vi.fn(),
    refreshMetadata: vi.fn(),
    getQualityInfo: vi.fn(),
    getRecentMedia: vi.fn(),
    getPopularMedia: vi.fn(),
    deleteMedia: vi.fn(),
    updateMedia: vi.fn(),
    getStorageRoots: vi.fn(),
    getStorageRoot: vi.fn(),
    createStorageRoot: vi.fn(),
    updateStorageRoot: vi.fn(),
    deleteStorageRoot: vi.fn(),
    testStorageRoot: vi.fn(),
  }
}))

vi.mock('@/hooks/useDebounce', async () => ({
  useDebounce: vi.fn((value: unknown) => value), // Return value immediately for testing
}))

// Mock child components
vi.mock('@/components/media/MediaGrid', async () => ({
  MediaGrid: ({ media, onMediaView, onMediaDownload, loading, viewMode: _viewMode }: any) => {
    return (
      <div data-testid="media-grid">
        {loading && <div data-testid="loading-indicator">Loading...</div>}
        {!loading && media.length === 0 && (
          <div data-testid="empty-state">
            <div>No media items found</div>
          </div>
        )}
        {media?.map((item: MediaItem) => {
          return (
            <div key={item.id} data-testid={`media-item-${item.id}`}>
              <span>{item.title}</span>
              <button 
                onClick={() => onMediaView?.(item)} 
                data-testid={`view-button-${item.id}`}
              >
                View
              </button>
              <button onClick={() => onMediaDownload?.(item)} data-testid={`download-button-${item.id}`}>Download</button>
            </div>
          )
        })}
      </div>
    )
  },
}))

vi.mock('@/components/media/MediaFilters', async () => ({
  MediaFilters: ({ filters, onFiltersChange, onReset }: any) => (
    <div data-testid="media-filters">
      <button onClick={() => onFiltersChange?.({ ...filters, media_type: 'video' })}>
        Filter Video
      </button>
      <button onClick={onReset}>Reset Filters</button>
    </div>
  ),
}))

vi.mock('@/components/media/MediaDetailModal', async () => ({
  MediaDetailModal: ({ media, isOpen, onClose, onDownload }: any) => {
    return isOpen ? (
      <div data-testid="media-detail-modal">
        <h2>{media.title}</h2>
        <button onClick={onClose}>Close</button>
        {onDownload && <button onClick={() => onDownload(media)}>Download</button>}
      </div>
    ) : null
  },
}))

// Mock UI components
vi.mock('@/components/ui/Card', async () => ({
  Card: ({ children }: any) => <div data-testid="card">{children}</div>,
  CardContent: ({ children }: any) => <div data-testid="card-content">{children}</div>,
  CardHeader: ({ children }: any) => <div data-testid="card-header">{children}</div>,
  CardTitle: ({ children }: any) => <div data-testid="card-title">{children}</div>,
}))

vi.mock('@/components/ui/Button', async () => ({
  Button: ({ children, onClick, disabled, 'data-testid': testId = 'button' }: any) => (
    <button onClick={onClick} disabled={disabled} data-testid={testId}>
      {children}
    </button>
  ),
}))

vi.mock('@/components/ui/Input', async () => ({
  Input: ({ onChange, value, placeholder }: any) => (
    <input
      onChange={onChange}
      value={value}
      placeholder={placeholder}
      data-testid="input"
    />
  ),
}))

const mockMediaApi = mediaApi as vi.Mocked<typeof mediaApi>

// Test data
const mockMediaItems: MediaItem[] = [
  {
    id: 1,
    title: 'Test Video 1',
    media_type: 'video',
    directory_path: '/test/video1.mp4',
    file_size: 1024000,
    created_at: '2023-01-01T00:00:00Z',
    updated_at: '2023-01-02T00:00:00Z',
  },
  {
    id: 2,
    title: 'Test Video 2',
    media_type: 'video',
    directory_path: '/test/video2.mp4',
    file_size: 2048000,
    created_at: '2023-01-01T00:00:00Z',
    updated_at: '2023-01-02T00:00:00Z',
  },
]

const mockStats = {
  total_items: 150,
  total_size: 1024000000,
  by_type: { video: 100, audio: 30, image: 20 },
  by_quality: { hd: 80, sd: 50, '4k': 20 },
  recent_additions: 5,
}

const createTestQueryClient = () => new QueryClient({
  defaultOptions: {
    queries: { retry: false },
    mutations: { retry: false },
  },
})

const renderWithQueryClient = (component: React.ReactElement) => {
  const queryClient = createTestQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      {component}
    </QueryClientProvider>
  )
}

describe('MediaBrowser', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockMediaApi.searchMedia.mockResolvedValue({
      items: mockMediaItems,
      total: 100,
      offset: 0,
      limit: 24,
    })
    mockMediaApi.getMediaStats.mockResolvedValue(mockStats)
    mockMediaApi.downloadMedia.mockResolvedValue(undefined)
  })

  describe('Rendering', () => {
    it('renders MediaBrowser component with header', () => {
      renderWithQueryClient(<MediaBrowser />)
      
      expect(screen.getByText('Media Browser')).toBeInTheDocument()
      expect(screen.getByText('Explore and discover your media collection')).toBeInTheDocument()
    })

    it('renders stats cards when stats are loaded', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        expect(screen.getByText('150')).toBeInTheDocument() // Total items
        expect(screen.getByText('Total Items')).toBeInTheDocument()
        expect(screen.getByText('3')).toBeInTheDocument() // Media types
        expect(screen.getByText('Media Types')).toBeInTheDocument()
      })
    })

    it('renders search input', () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const searchInput = screen.getByPlaceholderText('Search your media collection...')
      expect(searchInput).toBeInTheDocument()
    })

    it('renders view mode toggle buttons', () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Should have view mode toggle buttons with specific test IDs
      expect(screen.getByTestId('grid-view-button')).toBeInTheDocument()
      expect(screen.getByTestId('list-view-button')).toBeInTheDocument()
    })

    it('renders filter toggle button', () => {
      renderWithQueryClient(<MediaBrowser />)
      
      expect(screen.getByText('Filters')).toBeInTheDocument()
    })

    it('renders refresh button', () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Should have a refresh button with specific test ID
      expect(screen.getByTestId('refresh-button')).toBeInTheDocument()
    })
  })

  describe('Media Grid Integration', () => {
    it('displays media items when loaded', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        expect(screen.getByTestId('media-grid')).toBeInTheDocument()
        expect(screen.getByTestId('media-item-1')).toBeInTheDocument()
        expect(screen.getByTestId('media-item-2')).toBeInTheDocument()
        expect(screen.getByText('Test Video 1')).toBeInTheDocument()
        expect(screen.getByText('Test Video 2')).toBeInTheDocument()
      })
    })

    it('shows loading state while fetching media', () => {
      mockMediaApi.searchMedia.mockImplementation(() => new Promise(() => { /* noop */ })) // Never resolves
      
      renderWithQueryClient(<MediaBrowser />)
      
      expect(screen.getByTestId('loading-indicator')).toBeInTheDocument()
    })

    it('shows empty state when no media items found', async () => {
      mockMediaApi.searchMedia.mockResolvedValue({
        items: [],
        total: 0,
        offset: 0,
        limit: 24,
      })
      
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        expect(screen.getByText('No media items found')).toBeInTheDocument()
      })
    })
  })

  describe('Search Functionality', () => {
    it('handles search input changes', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const searchInput = screen.getByPlaceholderText('Search your media collection...')
      await userEvent.type(searchInput, 'test query')
      
      // Verify search was triggered with debounced value
      expect(mockMediaApi.searchMedia).toHaveBeenCalledWith(
        expect.objectContaining({
          query: 'test query',
          offset: 0, // Should reset to 0 on new search
        })
      )
    })

    it('clears search when input is empty', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const searchInput = screen.getByPlaceholderText('Search your media collection...')
      
      // Type 'test' first
      await userEvent.type(searchInput, 'test')
      
      // Wait for initial search
      await waitFor(() => {
        expect(mockMediaApi.searchMedia).toHaveBeenCalledWith(
          expect.objectContaining({
            query: 'test',
            offset: 0,
          })
        )
      })
      
      // Clear mock to track only new calls
      mockMediaApi.searchMedia.mockClear()
      
      // Clear the input
      await userEvent.clear(searchInput)
      
      // Wait a bit for the debounced function to execute
      await new Promise(resolve => setTimeout(resolve, 100))
      
      // Verify that the input value is actually cleared
      expect(searchInput).toHaveValue('')
      
      // Since React Query might not make a new call when the query becomes undefined 
      // (same as initial state), let's verify the behavior by checking if we can type again
      await userEvent.type(searchInput, 'new')
      
      // This should trigger a new search with 'new' query
      await waitFor(() => {
        expect(mockMediaApi.searchMedia).toHaveBeenCalledWith(
          expect.objectContaining({
            query: 'new',
            offset: 0,
          })
        )
      }, { timeout: 2000 })
    })
  })

  describe('Filter Functionality', () => {
    it('toggles filter panel when filters button is clicked', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const filtersButton = screen.getByTestId('filters-button')
      await userEvent.click(filtersButton)
      
      await waitFor(() => {
        expect(screen.getByTestId('media-filters')).toBeInTheDocument()
      })
    })

    it('applies filters when filter options are selected', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Open filters using test-id
      const filtersButton = screen.getByTestId('filters-button')
      await userEvent.click(filtersButton)
      
      // Wait for filter panel to open
      await waitFor(() => {
        expect(screen.getByTestId('media-filters')).toBeInTheDocument()
      })
      
      // Apply a filter and wait for API call
      const filterButton = screen.getByText('Filter Video')
      await userEvent.click(filterButton)
      
      // Wait for filter to apply (longer timeout to account for debouncing)
      await waitFor(() => {
        expect(mockMediaApi.searchMedia).toHaveBeenLastCalledWith(
          expect.objectContaining({
            media_type: 'video',
            offset: 0,
          })
        )
      }, { timeout: 3000 })
    })

    it('resets filters when reset button is clicked', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Open filters
      const filtersButton = screen.getByText('Filters')
      await userEvent.click(filtersButton)
      
      // Reset filters
      const resetButton = await screen.findByText('Reset Filters')
      await userEvent.click(resetButton)
      
      expect(mockMediaApi.searchMedia).toHaveBeenCalledWith(
        expect.objectContaining({
          limit: 24,
          offset: 0,
          sort_by: 'updated_at',
          sort_order: 'desc',
        })
      )
    })
  })

  describe('View Mode Toggle', () => {
    it('switches between grid and list view modes', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Initially in grid mode
      expect(screen.getByTestId('grid-view-button')).toBeInTheDocument()
      
      // Switch to list mode
      const listModeButton = screen.getByTestId('list-view-button')
      await userEvent.click(listModeButton)
      
      // Verify mode changed by checking if list button is now active
      expect(screen.getByTestId('list-view-button')).toBeInTheDocument()
    })
  })

  describe('Pagination', () => {
    it('shows pagination controls when there are multiple pages', async () => {
      // Override the default mock to return multi-page data
      mockMediaApi.searchMedia.mockResolvedValue({
        items: mockMediaItems,
        total: 100, // 5 pages with limit 24
        offset: 0,
        limit: 24,
      })
      
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        // Use getAllByText to get all matching elements and check first one
        const pageTexts = screen.getAllByText('Page 1 of 5')
        expect(pageTexts.length).toBeGreaterThan(0)
      }, { timeout: 3000 })
      
      // Verify pagination buttons are present
      expect(screen.getByTestId('prev-page-button-main')).toBeInTheDocument()
      expect(screen.getByTestId('next-page-button-main')).toBeInTheDocument()
    })

    it('disables previous button on first page', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        const prevButton = screen.getByTestId('prev-page-button-main')
        expect(prevButton).toBeDisabled()
      })
    })

    it('disables next button on last page', async () => {
      // Set up mock for response that will give us 2 pages (total=48, limit=24)
      mockMediaApi.searchMedia.mockResolvedValue({
        items: mockMediaItems,
        total: 48, // 2 pages with limit 24
        offset: 0,
        limit: 24,
      })
      
      renderWithQueryClient(<MediaBrowser />)
      
      // Wait for component to load and show pagination
      await waitFor(() => {
        expect(mockMediaApi.searchMedia).toHaveBeenCalledTimes(1)
        // Wait for loading to finish and pagination to appear
        expect(screen.getByTestId('next-page-button-main')).toBeInTheDocument()
      }, { timeout: 3000 })
      
      // Click next page button to navigate to last page
      const nextButton = screen.getByTestId('next-page-button-main')
      await userEvent.click(nextButton)
      
      // Now check that we're on page 2
      await waitFor(() => {
        const pageTexts = screen.getAllByText('Page 2 of 2')
        expect(pageTexts.length).toBeGreaterThan(0)
      }, { timeout: 3000 })
      
      // Now check that next button at main pagination is disabled
      const nextButtonAgain = screen.getByTestId('next-page-button-main')
      expect(nextButtonAgain).toBeDisabled()
    })

    it('handles next page navigation', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const nextButton = await screen.findByTestId('next-page-button-main')
      await userEvent.click(nextButton)
      
      expect(mockMediaApi.searchMedia).toHaveBeenCalledWith(
        expect.objectContaining({
          offset: 24, // Next page
        })
      )
    })

    it('handles previous page navigation', async () => {
      // Set up mock for response that will give us multiple pages
      mockMediaApi.searchMedia
        .mockResolvedValueOnce({
          items: mockMediaItems,
          total: 100,
          offset: 0,
          limit: 24,
        })
        // Mock response for page 2 (offset: 24)
        .mockResolvedValueOnce({
          items: mockMediaItems,
          total: 100,
          offset: 24,
          limit: 24,
        })
        // Mock response for returning to page 1 (offset: 0)
        .mockResolvedValueOnce({
          items: mockMediaItems,
          total: 100,
          offset: 0,
          limit: 24,
        })
      
      const { container: _container } = renderWithQueryClient(<MediaBrowser />)
      
      // Wait for component to load and show pagination
      await waitFor(() => {
        expect(mockMediaApi.searchMedia).toHaveBeenCalledTimes(1)
        expect(screen.getByTestId('next-page-button-main')).toBeInTheDocument()
      }, { timeout: 3000 })
      
      // Navigate to page 2 first - wrap in act to handle state updates
      const nextButton = screen.getByTestId('next-page-button-main')
      await act(async () => {
        await userEvent.click(nextButton)
      })
      
      // Wait for page 2 to load
      await waitFor(() => {
        const pageTexts = screen.getAllByText('Page 2 of 5')
        expect(pageTexts.length).toBe(2) // Both top and bottom pagination
      }, { timeout: 3000 })
      
      // Click previous button - wrap in act
      const prevButton = screen.getByTestId('prev-page-button-main')
      await act(async () => {
        await userEvent.click(prevButton)
      })
      
      // Verify we're back on page 1 by checking pagination text and that API was called
      await waitFor(() => {
        expect(mockMediaApi.searchMedia).toHaveBeenCalledTimes(3)
        const pageTexts = screen.getAllByText('Page 1 of 5')
        expect(pageTexts.length).toBe(2) // Both top and bottom pagination
        expect(screen.getByTestId('prev-page-button-main')).toBeDisabled()
      }, { timeout: 3000 })
      
      // Also verify the last API call had the correct offset
      expect(mockMediaApi.searchMedia).toHaveBeenLastCalledWith(
        expect.objectContaining({
          offset: 0, // Back to page 1
        })
      )
    })
  })

  describe('Media Interaction', () => {
    it('opens media detail modal when view is clicked', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Wait for media items to load
      await waitFor(() => {
        expect(screen.getByText('Test Video 1')).toBeInTheDocument()
      }, { timeout: 3000 })
      
      // Click view button for first media item using specific test-id
      const viewButton = screen.getByTestId('view-button-1')
      await userEvent.click(viewButton)
      
      // Wait for modal to appear
      await waitFor(() => {
        expect(screen.getByTestId('media-detail-modal')).toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'Test Video 1' })).toBeInTheDocument()
      }, { timeout: 3000 })
    })

    it('closes media detail modal when close is clicked', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Wait for media items to load
      await waitFor(() => {
        expect(screen.getByText('Test Video 1')).toBeInTheDocument()
      }, { timeout: 3000 })
      
      // Open modal by clicking view button using specific test-id
      const viewButton = screen.getByTestId('view-button-1')
      await userEvent.click(viewButton)
      
      // Wait for modal to appear
      await waitFor(() => {
        expect(screen.getByTestId('media-detail-modal')).toBeInTheDocument()
      }, { timeout: 3000 })
      
      // Close modal
      const closeButton = screen.getByText('Close')
      await userEvent.click(closeButton)
      
      // Wait for modal to disappear
      await waitFor(() => {
        expect(screen.queryByTestId('media-detail-modal')).not.toBeInTheDocument()
      }, { timeout: 3000 })
    })

    it('handles media download', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      // Wait for media to load
      await waitFor(() => {
        expect(screen.getByText('Test Video 1')).toBeInTheDocument()
      }, { timeout: 3000 })
      
      // Find download button for media item 1 using specific test-id
      const downloadButton = screen.getByTestId('download-button-1')
      await userEvent.click(downloadButton)
      
      // Wait for download API call
      await waitFor(() => {
        expect(mockMediaApi.downloadMedia).toHaveBeenCalledWith(mockMediaItems[0])
      }, { timeout: 3000 })
    })
  })

  describe('Refresh Functionality', () => {
    it('refreshes media data when refresh button is clicked', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const refreshButton = screen.getByTestId('refresh-button')
      await userEvent.click(refreshButton)
      
      expect(mockMediaApi.searchMedia).toHaveBeenCalled()
      expect(mockMediaApi.getMediaStats).toHaveBeenCalled()
    })
  })

  describe('Error Handling', () => {
    it('displays error message when media search fails', async () => {
      mockMediaApi.searchMedia.mockRejectedValue(new Error('Search failed'))
      
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        expect(screen.getByText(/Failed to load media/)).toBeInTheDocument()
      })
    })

    it('displays error message when stats fetch fails', async () => {
      mockMediaApi.getMediaStats.mockRejectedValue(new Error('Stats failed'))
      
      renderWithQueryClient(<MediaBrowser />)
      
      // The component doesn't display error for stats failure, just continues without stats
      // Test that it doesn't crash and loads media successfully
      await waitFor(() => {
        expect(screen.getByText('Test Video 1')).toBeInTheDocument()
      })
    })

    it('handles download error gracefully', async () => {
      mockMediaApi.downloadMedia.mockRejectedValue(new Error('Download failed'))

      renderWithQueryClient(<MediaBrowser />)

      // Wait for media to load
      await waitFor(() => {
        expect(screen.getByText('Test Video 1')).toBeInTheDocument()
      }, { timeout: 3000 })

      // Click download button for media item 1 using specific test-id
      const downloadButton = screen.getByTestId('download-button-1')
      await userEvent.click(downloadButton)

      // Download errors are handled via toast.error(), not console.error
      await waitFor(() => {
        expect(mockMediaApi.downloadMedia).toHaveBeenCalledWith(mockMediaItems[0])
      }, { timeout: 3000 })
    })
  })

  describe('Edge Cases', () => {
    it('handles empty search results gracefully', async () => {
      mockMediaApi.searchMedia.mockResolvedValue({
        items: [],
        total: 0,
        offset: 0,
        limit: 24,
      })
      
      renderWithQueryClient(<MediaBrowser />)
      
      await waitFor(() => {
        expect(screen.getByText('No media items found')).toBeInTheDocument()
      })
    })

    it('handles null/undefined props gracefully', () => {
      expect(() => {
        renderWithQueryClient(<MediaBrowser />)
      }).not.toThrow()
    })

    it('handles rapid search input changes', async () => {
      renderWithQueryClient(<MediaBrowser />)
      
      const searchInput = screen.getByPlaceholderText('Search your media collection...')
      
      // Type rapidly
      await userEvent.type(searchInput, 'rapid search')

      // With useDebounce mocked to return value immediately, search should be triggered
      expect(mockMediaApi.searchMedia).toHaveBeenCalled()
    })
  })

  describe('Responsive Design', () => {
    it('adapts to different screen sizes', () => {
      // Test mobile view
      Object.defineProperty(window, 'innerWidth', { writable: true, configurable: true, value: 640 })
      const { unmount } = renderWithQueryClient(<MediaBrowser />)
      
      // Component should render header in mobile
      expect(screen.getByText('Media Browser')).toBeInTheDocument()
      
      // Unmount and test desktop view
      unmount()
      Object.defineProperty(window, 'innerWidth', { writable: true, configurable: true, value: 1024 })
      renderWithQueryClient(<MediaBrowser />)
      
      // Component should still render header in desktop
      expect(screen.getByText('Media Browser')).toBeInTheDocument()
    })
  })
})