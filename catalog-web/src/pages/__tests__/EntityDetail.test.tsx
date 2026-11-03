import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { EntityDetail } from '../EntityDetail'
import type { MediaEntityDetail, MediaEntity, EntityFile } from '@/types/media'

vi.mock('@/lib/mediaApi', () => ({
  entityApi: {
    getEntity: vi.fn().mockResolvedValue({
      id: 1,
      title: 'The Matrix',
      original_title: 'The Matrix',
      media_type: 'movie',
      media_type_id: 1,
      year: 1999,
      rating: 8.7,
      runtime: 136,
      language: 'English',
      genre: ['Sci-Fi', 'Action'],
      director: 'Wachowskis',
      description: 'A computer hacker learns about the true nature of reality.',
      status: 'movie',
      file_count: 2,
      children_count: 0,
      external_metadata: [],
      first_detected: '2024-01-01T00:00:00Z',
      last_updated: '2024-01-02T00:00:00Z',
    }),
    getEntityChildren: vi.fn().mockResolvedValue({
      items: [],
      total: 0,
      limit: 24,
      offset: 0,
    }),
    getEntityFiles: vi.fn().mockResolvedValue({
      files: [
        {
          id: 1,
          media_item_id: 1,
          file_id: 101,
          quality_info: '1080p',
          language: 'English',
          is_primary: true,
          created_at: '2024-01-01T00:00:00Z',
        },
      ],
      total: 1,
    }),
    getEntityDuplicates: vi.fn().mockResolvedValue({
      duplicates: [],
      total: 0,
    }),
    refreshEntityMetadata: vi.fn().mockResolvedValue({
      message: 'Metadata refresh queued',
      entity_id: 1,
    }),
    updateUserMetadata: vi.fn().mockResolvedValue({ message: 'Updated' }),
  },
}))

vi.mock('@/contexts/AuthContext', () => ({ useAuth: () => ({ isAdmin: true }) }))

vi.mock('react-hot-toast', () => ({
  default: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock entity sub-components to avoid deep dependency issues
vi.mock('@/components/entity/EntityDetailView', async () => ({
  EntityHero: ({
    entity,
    files: _files,
    duplicateCount: _duplicateCount,
    onFavorite,
    onRefresh,
    refreshPending,
  }: {
    entity: MediaEntityDetail
    files: EntityFile[]
    duplicateCount: number
    onFavorite: () => void
    onRefresh: () => void
    refreshPending: boolean
  }) => (
    <div data-testid="entity-hero">
      <h1>{entity.title}</h1>
      <span data-testid="media-type-badge">{entity.media_type.replace(/_/g, ' ')}</span>
      {entity.year && <span data-testid="entity-year">{entity.year}</span>}
      {entity.rating != null && <span data-testid="entity-rating">{entity.rating.toFixed(1)}</span>}
      {entity.runtime != null && <span data-testid="entity-runtime">{entity.runtime} min</span>}
      {entity.language && <span data-testid="entity-language">{entity.language}</span>}
      {entity.genre && entity.genre.map((g: string) => (
        <span key={g} data-testid={`genre-${g}`}>{g}</span>
      ))}
      {entity.director && (
        <p>Directed by <span>{entity.director}</span></p>
      )}
      {entity.description && <p data-testid="entity-description">{entity.description}</p>}
      <div data-testid="entity-file-count">{entity.file_count}</div>
      <div data-testid="entity-file-count-label">Files</div>
      <div data-testid="entity-children-count">{entity.children_count}</div>
      <div data-testid="entity-children-count-label">Children</div>
      <button onClick={onFavorite}>Favorite</button>
      <button onClick={onRefresh} disabled={refreshPending}>Refresh</button>
    </div>
  ),
  ChildrenList: ({ items }: { items: MediaEntity[] }) => (
    items.length > 0 ? <div data-testid="children-list">{items.length} children</div> : null
  ),
  FilesList: ({ files }: { files: EntityFile[] }) => (
    files.length > 0 ? (
      <div data-testid="files-list">
        {files.map((file: EntityFile) => (
          <div key={file.id} data-testid={`file-${file.id}`}>
            <span>File #{file.file_id}</span>
            {file.quality_info && <span>{file.quality_info}</span>}
            {file.language && <span>{file.language}</span>}
            {file.is_primary && <span>Primary</span>}
          </div>
        ))}
        <span>({files.length})</span>
      </div>
    ) : null
  ),
  DuplicatesList: ({ duplicates }: { duplicates: MediaEntity[] }) => (
    duplicates.length > 0 ? <div data-testid="duplicates-list">{duplicates.length} duplicates</div> : null
  ),
}))

// Mock UI components
vi.mock('@/components/ui/Card', async () => ({
  Card: ({ children, className, onClick }: any) => (
    <div data-testid="card" className={className} onClick={onClick}>{children}</div>
  ),
  CardContent: ({ children, className }: any) => (
    <div data-testid="card-content" className={className}>{children}</div>
  ),
  CardHeader: ({ children }: any) => <div data-testid="card-header">{children}</div>,
  CardTitle: ({ children, className }: any) => (
    <div data-testid="card-title" className={className}>{children}</div>
  ),
}))

vi.mock('@/components/ui/Button', async () => ({
  Button: ({ children, onClick, disabled, variant: _variant, size: _size, className, ...rest }: any) => (
    <button onClick={onClick} disabled={disabled} className={className} {...rest}>
      {children}
    </button>
  ),
  buttonVariants: () => '',
}))

function renderWithProviders(entityId: string = '1') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/entity/${entityId}`]}>
        <Routes>
          <Route path="/entity/:id" element={<EntityDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('EntityDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders entity title', async () => {
    renderWithProviders()
    await waitFor(() => {
      // Title appears in both the breadcrumb and the hero component
      const titles = screen.getAllByText('The Matrix')
      expect(titles.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('displays media type badge', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('media-type-badge')).toHaveTextContent('movie')
    })
  })

  it('shows year', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-year')).toHaveTextContent('1999')
    })
  })

  it('shows rating with one decimal place', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-rating')).toHaveTextContent('8.7')
    })
  })

  it('shows runtime', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-runtime')).toHaveTextContent('136 min')
    })
  })

  it('shows language', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-language')).toHaveTextContent('English')
    })
  })

  it('shows genre badges', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByText('Sci-Fi')).toBeInTheDocument()
      expect(screen.getByText('Action')).toBeInTheDocument()
    })
  })

  it('shows director', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByText('Wachowskis')).toBeInTheDocument()
      expect(screen.getByText(/Directed by/)).toBeInTheDocument()
    })
  })

  it('shows description', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(
        screen.getByText('A computer hacker learns about the true nature of reality.')
      ).toBeInTheDocument()
    })
  })

  it('shows file count in stats', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-file-count')).toHaveTextContent('2')
      expect(screen.getByTestId('entity-file-count-label')).toHaveTextContent('Files')
    })
  })

  it('shows children count in stats', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-children-count')).toHaveTextContent('0')
      expect(screen.getByTestId('entity-children-count-label')).toHaveTextContent('Children')
    })
  })

  it('shows files list when files exist', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('files-list')).toBeInTheDocument()
      expect(screen.getByText('File #101')).toBeInTheDocument()
      expect(screen.getByText('1080p')).toBeInTheDocument()
      expect(screen.getByText('Primary')).toBeInTheDocument()
    })
  })

  it('shows the files section with count', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByText('(1)')).toBeInTheDocument()
    })
  })

  it('shows favorite and refresh action buttons', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByText('Favorite')).toBeInTheDocument()
      expect(screen.getByText('Refresh')).toBeInTheDocument()
    })
  })

  it('does not show children list when there are no children', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-hero')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('children-list')).not.toBeInTheDocument()
  })

  it('does not show duplicates list when there are no duplicates', async () => {
    renderWithProviders()
    await waitFor(() => {
      expect(screen.getByTestId('entity-hero')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('duplicates-list')).not.toBeInTheDocument()
  })

  it('shows not found for invalid entity', async () => {
    const { entityApi } = await import('@/lib/mediaApi')
    ;(entityApi.getEntity as any).mockRejectedValueOnce(new Error('Not found'))
    renderWithProviders('999')
    await waitFor(() => {
      expect(screen.getByText('Entity not found')).toBeInTheDocument()
    })
  })

  it('shows back to browse button when entity not found', async () => {
    const { entityApi } = await import('@/lib/mediaApi')
    ;(entityApi.getEntity as any).mockRejectedValueOnce(new Error('Not found'))
    renderWithProviders('999')
    await waitFor(() => {
      expect(screen.getByText('Back to Browse')).toBeInTheDocument()
    })
  })

  it('shows breadcrumb with media type', async () => {
    renderWithProviders()
    await waitFor(() => {
      // The breadcrumb in EntityDetail shows media_type with underscores replaced
      // and the entity title as separate spans
      const headings = screen.getAllByText('The Matrix')
      // At least 2: one in breadcrumb, one in EntityHero h1
      expect(headings.length).toBeGreaterThanOrEqual(2)
    })
  })
})
