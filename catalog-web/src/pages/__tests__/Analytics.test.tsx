import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Analytics } from '../Analytics'
import { mediaApi } from '@/lib/mediaApi'
import { statsApi } from '@/lib/statsApi'

// Mock dependencies
vi.mock('@/lib/mediaApi', async () => ({
  mediaApi: {
    getMediaStats: vi.fn(),
    getRecentMedia: vi.fn(),
  },
}))

vi.mock('@/lib/statsApi', async () => ({
  statsApi: {
    getGrowthTrends: vi.fn().mockResolvedValue([
      { date: '2026-03-01', files_added: 10, bytes_added: 1000 },
    ]),
    getFileTypeStats: vi.fn().mockResolvedValue([
      { extension: '.mp4', count: 100, total_size: 50000000 },
      { extension: '.jpg', count: 200, total_size: 10000000 },
    ]),
    getDuplicateStats: vi.fn().mockResolvedValue({
      total_groups: 5, total_duplicates: 12, wasted_bytes: 500000,
    }),
    getOverallStats: vi.fn().mockResolvedValue({
      total_files: 1000, total_dirs: 50, total_size: 100000000,
    }),
    getScanHistory: vi.fn().mockResolvedValue([]),
  },
}))

vi.mock('@/lib/analyticsApi', async () => ({
  analyticsApi: {
    getSystemAnalytics: vi.fn().mockResolvedValue({
      popular_media: [], recent_activity: [],
      total_views: 0, total_events: 0,
    }),
  },
}))

vi.mock('framer-motion', async () => {
  const MockMotionDiv = ({ children, ...props }: any) => <div {...props}>{children}</div>
  MockMotionDiv.displayName = 'MockMotionDiv'
  return {
    motion: {
      div: MockMotionDiv,
    },
  }
})

vi.mock('recharts', async () => ({
  ResponsiveContainer: ({ children }: any) => <div data-testid="responsive-container">{children}</div>,
  PieChart: ({ children }: any) => <div data-testid="pie-chart">{children}</div>,
  Pie: ({ children }: any) => <div data-testid="pie">{children}</div>,
  Cell: () => <div data-testid="cell" />,
  BarChart: ({ children, data }: any) => <div data-testid="bar-chart" data-points={JSON.stringify(data ?? null)}>{children}</div>,
  Bar: () => <div data-testid="bar" />,
  LineChart: ({ children }: any) => <div data-testid="line-chart">{children}</div>,
  Line: () => <div data-testid="line" />,
  AreaChart: ({ children, data }: any) => <div data-testid="area-chart" data-points={JSON.stringify(data ?? null)}>{children}</div>,
  Area: () => <div data-testid="area" />,
  CartesianGrid: () => <div data-testid="cartesian-grid" />,
  XAxis: () => <div data-testid="x-axis" />,
  YAxis: () => <div data-testid="y-axis" />,
  Tooltip: () => <div data-testid="tooltip" />,
}))

vi.mock('lucide-react', async () => ({
  TrendingUp: () => <div data-testid="icon-trending">TrendingUp Icon</div>,
  Database: () => <div data-testid="icon-database">Database Icon</div>,
  HardDrive: () => <div data-testid="icon-harddrive">HardDrive Icon</div>,
  Clock: () => <div data-testid="icon-clock">Clock Icon</div>,
  Star: () => <div data-testid="icon-star">Star Icon</div>,
  Film: () => <div data-testid="icon-film">Film Icon</div>,
  Music: () => <div data-testid="icon-music">Music Icon</div>,
  Gamepad2: () => <div data-testid="icon-gamepad">Gamepad Icon</div>,
  Monitor: () => <div data-testid="icon-monitor">Monitor Icon</div>,
  Copy: () => <div data-testid="icon-copy">Copy Icon</div>,
  FileType: () => <div data-testid="icon-filetype">FileType Icon</div>,
}))

const mockMediaApi = vi.mocked(mediaApi)

// Create a test wrapper with QueryClient
const createTestWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  )
  Wrapper.displayName = 'TestWrapper'
  return Wrapper
}

describe('Analytics', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Loading State', () => {
    it('renders loading skeleton when stats are loading', () => {
      mockMediaApi.getMediaStats.mockReturnValue(new Promise(() => { /* noop */ })) // Never resolves
      mockMediaApi.getRecentMedia.mockReturnValue(new Promise(() => { /* noop */ }))

      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      // Loading skeleton should be present
      const skeletonElements = document.querySelectorAll('.animate-pulse')
      expect(skeletonElements.length).toBeGreaterThan(0)
    })
  })

  describe('Data Rendering', () => {
    const mockStats = {
      total_items: 1234,
      total_size: 5368709120, // 5GB in bytes
      recent_additions: 25,
      by_type: {
        movie: 500,
        tv_show: 300,
        music: 200,
        game: 150,
        software: 84,
      },
      by_quality: {
        '1080p': 400,
        '720p': 350,
        '4k': 200,
        '480p': 150,
        dvd: 134,
      },
    }

    const mockRecentMedia = [
      {
        id: '1',
        title: 'The Matrix',
        media_type: 'movie',
        year: 1999,
        quality: '1080p',
        file_size: 2097152000, // 2GB
        created_at: '2024-01-01T10:00:00Z',
      },
      {
        id: '2',
        title: 'Dark Side of the Moon',
        media_type: 'music',
        year: 1973,
        quality: 'FLAC',
        file_size: 104857600, // 100MB
        created_at: '2024-01-02T10:00:00Z',
      },
    ]

    beforeEach(() => {
      mockMediaApi.getMediaStats.mockResolvedValue(mockStats)
      mockMediaApi.getRecentMedia.mockResolvedValue(mockRecentMedia)
    })

    it('renders the Analytics component with data', async () => {
      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('Analytics Dashboard')).toBeInTheDocument()
      })

      expect(screen.getByText('Insights and statistics about your media collection')).toBeInTheDocument()
    })

    it('displays key metrics correctly', async () => {
      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('1,234')).toBeInTheDocument() // Total items
      })

      expect(screen.getByText('5.0 GB')).toBeInTheDocument() // Storage used
      expect(screen.getByText('25')).toBeInTheDocument() // Recent additions
      expect(screen.getByText('5')).toBeInTheDocument() // Media types count
    })

    it('renders chart containers', async () => {
      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('Media Types Distribution')).toBeInTheDocument()
      })

      expect(screen.getByText('Quality Distribution')).toBeInTheDocument()
      expect(screen.getByText('Collection Growth')).toBeInTheDocument()
      expect(screen.getByText('Weekly Activity')).toBeInTheDocument()
    })

    it('renders recent media activity', async () => {
      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('Recently Added Media')).toBeInTheDocument()
      })

      expect(screen.getByText('The Matrix')).toBeInTheDocument()
      expect(screen.getByText('(1999)')).toBeInTheDocument()
      expect(screen.getByText('Dark Side of the Moon')).toBeInTheDocument()
      expect(screen.getByText('movie')).toBeInTheDocument()
      expect(screen.getByText('music')).toBeInTheDocument()
    })

    it('displays file sizes correctly', async () => {
      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('2000 MB')).toBeInTheDocument() // The Matrix file size
      })

      expect(screen.getByText('100 MB')).toBeInTheDocument() // Album file size
    })

    it('renders media type icons', async () => {
      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        const filmIcons = screen.getAllByTestId('icon-film')
        expect(filmIcons.length).toBeGreaterThan(0)
      })

      expect(screen.getAllByTestId('icon-music').length).toBeGreaterThan(0)
    })
  })

  describe('Error Handling', () => {
    it('handles API errors gracefully', async () => {
      mockMediaApi.getMediaStats.mockRejectedValue(new Error('API Error'))
      mockMediaApi.getRecentMedia.mockRejectedValue(new Error('API Error'))

      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      // Should still render the component structure even with errors
      await waitFor(() => {
        expect(screen.getByText('Analytics Dashboard')).toBeInTheDocument()
      })

      // Should show 0 values for missing data - check for Total Media Items showing 0
      expect(screen.getByText('Total Media Items')).toBeInTheDocument()
      const statValues = screen.getAllByText('0')
      expect(statValues.length).toBeGreaterThan(0)
    })
  })

  describe('Empty Data Handling', () => {
    it('handles empty stats data', async () => {
      const emptyStats = {
        total_items: 0,
        total_size: 0,
        recent_additions: 0,
        by_type: {},
        by_quality: {},
      }

      mockMediaApi.getMediaStats.mockResolvedValue(emptyStats)
      mockMediaApi.getRecentMedia.mockResolvedValue([])

      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('Total Media Items')).toBeInTheDocument()
      })

      // Should show empty state for charts
      expect(screen.getByText('Media Types Distribution')).toBeInTheDocument()
      expect(screen.getByText('Quality Distribution')).toBeInTheDocument()
    })
  })

  describe('Layout and Structure', () => {
    it('renders main container with correct classes', async () => {
      mockMediaApi.getMediaStats.mockResolvedValue({
        total_items: 0,
        total_size: 0,
        recent_additions: 0,
        by_type: {},
        by_quality: {},
      })
      mockMediaApi.getRecentMedia.mockResolvedValue([])

      const TestWrapper = createTestWrapper()
      const { container } = render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        const mainDiv = container.firstChild as HTMLElement
        expect(mainDiv).toHaveClass('max-w-7xl')
        expect(mainDiv).toHaveClass('mx-auto')
      })
    })

    it('renders all sections in correct order', async () => {
      const mockStats = {
        total_items: 1,
        total_size: 1024,
        recent_additions: 1,
        by_type: { movie: 1 },
        by_quality: { '1080p': 1 },
      }

      mockMediaApi.getMediaStats.mockResolvedValue(mockStats)
      mockMediaApi.getRecentMedia.mockResolvedValue([])

      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('Analytics Dashboard')).toBeInTheDocument()
      })

      // Check that all major sections are present
      expect(screen.getByText('Total Media Items')).toBeInTheDocument()
      expect(screen.getByText('Media Types Distribution')).toBeInTheDocument()
      expect(screen.getByText('Recently Added Media')).toBeInTheDocument()
    })
  })

  describe('StatCard Component', () => {
    it('renders StatCard with all props', async () => {
      const mockStats = {
        total_items: 100,
        total_size: 1073741824, // 1GB in bytes
        recent_additions: 5,
        by_type: { movie: 1 },
        by_quality: { '1080p': 1 },
      }

      mockMediaApi.getMediaStats.mockResolvedValue(mockStats)
      mockMediaApi.getRecentMedia.mockResolvedValue([])

      const TestWrapper = createTestWrapper()
      render(
        <TestWrapper>
          <Analytics />
        </TestWrapper>
      )

      await waitFor(() => {
        expect(screen.getByText('100')).toBeInTheDocument()
      })

      expect(screen.getByText('1.0 GB')).toBeInTheDocument()
      expect(screen.getByTestId('icon-database')).toBeInTheDocument()
    })
  })

  describe('CAT-001: no invented figures', () => {
    const stats = { total_items: 10, total_size: 1024 ** 3, recent_additions: 2, by_type: { movie: 10 }, by_quality: { '1080p': 10 } }

    it('draws no growth points when the API returns no growth trends (never random ones)', async () => {
      mockMediaApi.getMediaStats.mockResolvedValue(stats as any)
      mockMediaApi.getRecentMedia.mockResolvedValue([])
      vi.mocked(statsApi.getGrowthTrends).mockResolvedValue([] as any)
      const Wrapper = createTestWrapper()
      render(<Wrapper><Analytics /></Wrapper>)
      await waitFor(() => expect(screen.getAllByTestId('area-chart').length).toBeGreaterThan(0))
      // run twice: random data would differ between renders and would be non-empty
      for (const el of screen.getAllByTestId('area-chart')) expect(el.getAttribute('data-points')).toBe('[]')
      for (const el of screen.getAllByTestId('bar-chart')) {
        const pts = JSON.parse(el.getAttribute('data-points') || 'null')
        if (Array.isArray(pts) && pts.length && 'additions' in pts[0]) throw new Error('weekly additions were invented')
      }
    })

    it('shows no hard-coded change strings under the key metrics', async () => {
      mockMediaApi.getMediaStats.mockResolvedValue(stats as any)
      mockMediaApi.getRecentMedia.mockResolvedValue([])
      const Wrapper = createTestWrapper()
      render(<Wrapper><Analytics /></Wrapper>)
      await waitFor(() => expect(screen.getByText('Total Media Items')).toBeInTheDocument())
      for (const invented of [/from last month/, /\+[\d.]+ (GB|TB|MB) this week/, /from yesterday/, /new types detected/]) {
        expect(screen.queryByText(invented)).not.toBeInTheDocument()
      }
    })
  })
})
