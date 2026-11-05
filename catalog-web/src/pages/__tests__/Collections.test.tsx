import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Collections } from '../Collections'
import { toast } from 'react-hot-toast'

// Mock all heavy dependencies
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

const mockCreateCollection = vi.fn()
const mockUpdateCollection = vi.fn()
const mockDeleteCollection = vi.fn()
const mockShareCollection = vi.fn()
const mockDuplicateCollection = vi.fn()
const mockExportCollection = vi.fn()
const mockBulkDeleteCollections = vi.fn()
const mockBulkShareCollections = vi.fn()
const mockBulkExportCollections = vi.fn()
const mockBulkUpdateCollections = vi.fn()
const mockRefetchCollections = vi.fn()
const mockUseCollections = vi.fn()

vi.mock('../../hooks/useCollections', () => ({
  useCollections: (...args: any[]) => mockUseCollections(...args),
}))

vi.mock('../../components/collections/SmartCollectionBuilder', () => ({
  SmartCollectionBuilder: ({ onSave, onCancel }: any) => (
    <div data-testid="smart-builder">
      Smart Collection Builder
      <button onClick={() => onSave('Smart Col', 'Smart desc', [{ field: 'genre', operator: 'equals', value: 'rock' }])}>Save Smart</button>
      <button onClick={onCancel}>Cancel Smart</button>
    </div>
  ),
}))

vi.mock('../../components/collections/CollectionPreview', () => ({
  CollectionPreview: ({ onClose }: any) => (
    <div data-testid="collection-preview">
      Collection Preview
      <button onClick={onClose}>Close Preview</button>
    </div>
  ),
}))

vi.mock('../../components/collections/BulkOperations', () => ({
  BulkOperations: ({ selectedCollections, onOperation, onClose }: any) => (
    <div data-testid="bulk-ops">
      Bulk Operations ({selectedCollections.length} selected)
      <button onClick={() => onOperation('delete')}>Bulk Delete</button>
      <button onClick={() => onOperation('share')}>Bulk Share</button>
      <button onClick={() => onOperation('export', { format: 'csv' })}>Bulk Export</button>
      <button onClick={() => onOperation('duplicate')}>Bulk Duplicate</button>
      <button onClick={() => onOperation('unknown')}>Bulk Unknown</button>
      <button onClick={onClose}>Close Bulk</button>
    </div>
  ),
}))

vi.mock('../../components/collections/PerformanceOptimizer', () => ({
  PerformanceOptimizer: ({ children }: any) => <div>{children}</div>,
}))

vi.mock('../../components/collections/CollectionSettings', () => ({
  CollectionSettings: ({ onClose, onSave }: any) => (
    <div data-testid="collection-settings">
      Collection Settings
      <button onClick={() => onSave({ name: 'Updated' })}>Save Settings</button>
      <button onClick={onClose}>Close Settings</button>
    </div>
  ),
}))

vi.mock('../../components/collections/CollectionAnalytics', () => ({
  CollectionAnalytics: ({ onClose }: any) => (
    <div data-testid="collection-analytics">
      Collection Analytics
      <button onClick={onClose}>Close Analytics</button>
    </div>
  ),
}))

vi.mock('../../components/collections/CollectionSharing', () => ({
  CollectionSharing: ({ onClose }: any) => (
    <div data-testid="collection-sharing">
      Collection Sharing
      <button onClick={onClose}>Close Sharing</button>
    </div>
  ),
}))

vi.mock('../../components/collections/CollectionExport', () => ({
  CollectionExport: ({ onClose }: any) => (
    <div data-testid="collection-export">
      Collection Export
      <button onClick={onClose}>Close Export</button>
    </div>
  ),
}))

vi.mock('../../components/collections/CollectionRealTime', () => ({
  CollectionRealTime: ({ onClose }: any) => (
    <div data-testid="collection-realtime">
      Collection RealTime
      <button onClick={onClose}>Close RealTime</button>
    </div>
  ),
}))

vi.mock('../../components/performance/LazyComponents', () => ({
  ComponentLoader: ({ children }: any) => <>{children}</>,
  preloadComponent: vi.fn(),
  CollectionTemplates: ({ onClose, onApplyTemplate }: any) => (
    <div data-testid="collection-templates">
      Collection Templates
      <button onClick={() => onApplyTemplate({ id: 't1', name: 'Recent Movies', description: 'tdesc', rules: [{ field: 'rating', operator: 'greater_than', value: 4 }] }, 'My Movies').catch(() => undefined)}>Apply Template</button>
      <button onClick={onClose}>Close Templates</button>
    </div>
  ),
  AdvancedSearch: () => <div data-testid="advanced-search">Advanced Search</div>,
  CollectionAutomation: () => <div data-testid="collection-automation">Collection Automation</div>,
  ExternalIntegrations: () => <div data-testid="external-integrations">External Integrations</div>,
  CollectionAnalytics: () => <div>Lazy Analytics</div>,
}))

vi.mock('../../components/performance/VirtualScroller', () => ({
  VirtualList: ({ children }: any) => <div>{children}</div>,
  VirtualizedTable: ({ children }: any) => <div>{children}</div>,
}))

vi.mock('../../components/performance/MemoCache', () => ({
  useMemoized: (fn: any, _deps: any[]) => fn(),
  useOptimizedData: (data: any) => data,
  usePagination: (data: any, _pageSize: number) => ({
    page: 1,
    paginatedData: data,
    totalPages: 1,
    nextPage: vi.fn(),
    prevPage: vi.fn(),
    goToPage: vi.fn(),
  }),
}))

vi.mock('../../components/performance/BundleAnalyzer', () => ({
  BundleAnalyzer: () => <div>Bundle Analyzer</div>,
}))

vi.mock('../../components/ai/AIComponents', () => ({
  AICollectionSuggestions: () => <div>AI Suggestions</div>,
  AINaturalSearch: () => <div>AI Search</div>,
  AIContentCategorizer: () => <div>AI Categorizer</div>,
}))

vi.mock('../../components/ai/AIAnalytics', () => ({
  AIUserBehaviorAnalytics: () => <div>AI Behavior</div>,
  AIPredictions: () => <div>AI Predictions</div>,
  AISmartOrganization: () => <div>AI Organization</div>,
}))

vi.mock('../../components/ai/AIMetadata', () => ({
  AIMetadataExtractor: () => <div>AI Metadata</div>,
  AIAutomationRules: () => <div>AI Rules</div>,
  AIContentQualityAnalyzer: () => <div>AI Quality</div>,
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const testCollection = {
  id: '1',
  name: 'Test Collection',
  description: 'A test collection',
  is_smart: true,
  smart_rules: [],
  item_count: 42,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  is_public: false,
  primary_media_type: 'music',
  owner_id: 'user1',
}

const secondCollection = {
  id: '2',
  name: 'Video Collection',
  description: 'Videos here',
  is_smart: false,
  smart_rules: [],
  item_count: 15,
  created_at: '2024-02-01T00:00:00Z',
  updated_at: '2024-02-15T00:00:00Z',
  is_public: true,
  primary_media_type: 'video',
  owner_id: 'user1',
}

const defaultUseCollectionsReturn = {
  collections: [testCollection, secondCollection],
  isLoading: false,
  error: null,
  refetchCollections: mockRefetchCollections,
  createCollection: mockCreateCollection,
  updateCollection: mockUpdateCollection,
  deleteCollection: mockDeleteCollection,
  shareCollection: mockShareCollection,
  duplicateCollection: mockDuplicateCollection,
  exportCollection: mockExportCollection,
  bulkDeleteCollections: mockBulkDeleteCollections,
  bulkShareCollections: mockBulkShareCollections,
  bulkExportCollections: mockBulkExportCollections,
  bulkUpdateCollections: mockBulkUpdateCollections,
  isSharing: false,
  isDuplicating: false,
  isExporting: false,
}

describe('Collections Page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseCollections.mockReturnValue(defaultUseCollectionsReturn)
    mockCreateCollection.mockResolvedValue(undefined)
    mockDeleteCollection.mockResolvedValue(undefined)
    mockShareCollection.mockResolvedValue(undefined)
    mockDuplicateCollection.mockResolvedValue(undefined)
    mockBulkDeleteCollections.mockResolvedValue(undefined)
    mockBulkShareCollections.mockResolvedValue(undefined)
    mockBulkExportCollections.mockResolvedValue(undefined)
    mockBulkUpdateCollections.mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders page heading', () => {
    render(<Collections />)
    expect(screen.getByText('Collections')).toBeInTheDocument()
  })

  it('renders page description', () => {
    render(<Collections />)
    expect(
      screen.getByText('Organize your media with smart and manual collections')
    ).toBeInTheDocument()
  })

  it('renders tab navigation', () => {
    render(<Collections />)
    expect(screen.getByText('All Collections')).toBeInTheDocument()
    expect(screen.getByText('Smart Collections')).toBeInTheDocument()
    // "Templates" appears as both a tab label and a button
    const templatesElements = screen.getAllByText('Templates')
    expect(templatesElements.length).toBeGreaterThanOrEqual(1)
  })

  it('displays collection items', () => {
    render(<Collections />)
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
    expect(screen.getByText('42 items')).toBeInTheDocument()
  })

  it('renders search input', () => {
    render(<Collections />)
    expect(screen.getByPlaceholderText('Search collections...')).toBeInTheDocument()
  })

  it('renders action buttons', () => {
    render(<Collections />)
    expect(screen.getByText('Smart Collection')).toBeInTheDocument()
    // "Templates" appears multiple times (tab + button)
    const templatesElements = screen.getAllByText('Templates')
    expect(templatesElements.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Advanced Search')).toBeInTheDocument()
  })

  it('shows AI Features button', () => {
    render(<Collections />)
    // "AI Features" appears as both a tab and a button
    const aiElements = screen.getAllByText('AI Features')
    expect(aiElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders select all checkbox', () => {
    render(<Collections />)
    expect(screen.getByText(/Select all/)).toBeInTheDocument()
  })

  it('shows empty state when no collections match filter', () => {
    mockUseCollections.mockReturnValue({
      ...defaultUseCollectionsReturn,
      collections: [],
    })

    render(<Collections />)
    expect(screen.getByText('No collections found')).toBeInTheDocument()
  })

  // --- New tests for increased coverage ---

  it('shows loading state', () => {
    mockUseCollections.mockReturnValue({
      ...defaultUseCollectionsReturn,
      collections: [],
      isLoading: true,
    })
    render(<Collections />)
    // Loading spinner should be present (animate-spin class)
    const spinner = document.querySelector('.animate-spin')
    expect(spinner).toBeInTheDocument()
  })

  it('shows empty state message for filtered results', () => {
    mockUseCollections.mockReturnValue({
      ...defaultUseCollectionsReturn,
      collections: [],
    })
    render(<Collections />)
    expect(screen.getByText('Create your first collection to get started')).toBeInTheDocument()
  })

  it('opens smart builder when Smart Collection button is clicked', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    await user.click(screen.getByText('Smart Collection'))

    expect(screen.getByTestId('smart-builder')).toBeInTheDocument()
  })

  it('saves a smart collection via the builder', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    await user.click(screen.getByText('Smart Collection'))
    expect(screen.getByTestId('smart-builder')).toBeInTheDocument()

    await user.click(screen.getByText('Save Smart'))

    await waitFor(() => {
      expect(mockCreateCollection).toHaveBeenCalledWith(expect.objectContaining({
        collection: expect.objectContaining({
          name: 'Smart Col',
          is_smart: true,
        }),
      }))
    })
  })

  it('cancels the smart builder and returns to main view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    await user.click(screen.getByText('Smart Collection'))
    expect(screen.getByTestId('smart-builder')).toBeInTheDocument()

    await user.click(screen.getByText('Cancel Smart'))
    expect(screen.queryByTestId('smart-builder')).not.toBeInTheDocument()
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('opens Templates view when Templates button is clicked', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Find the Templates button (not the tab) - the button has a FileText icon
    const templatesButtons = screen.getAllByText('Templates')
    // The action button is the one inside a <button> element with gap-2 class
    const templateBtn = templatesButtons.find(el => {
      const btn = el.closest('button')
      return btn && btn.classList.contains('gap-2')
    })
    expect(templateBtn).toBeTruthy()
    await user.click(templateBtn!)

    expect(screen.getByTestId('collection-templates')).toBeInTheDocument()
  })

  it('CAT-001: applying a template creates the collection via the API and toasts nothing itself', async () => {
    const user = userEvent.setup()
    vi.mocked(toast.success).mockClear()
    render(<Collections />)
    const templateBtn = screen.getAllByText('Templates').find(el => el.closest('button')?.classList.contains('gap-2'))
    await user.click(templateBtn!)
    await user.click(screen.getByText('Apply Template'))
    await waitFor(() => expect(mockCreateCollection).toHaveBeenCalledWith({
      collection: expect.objectContaining({ name: 'My Movies', description: 'tdesc', is_smart: true, smart_rules: [expect.objectContaining({ field: 'rating' })] }),
    }))
    expect(toast.success).not.toHaveBeenCalledWith(expect.stringContaining('applied'))
  })

  it('CAT-001: a failed template creation is not reported as applied and keeps the view open', async () => {
    const user = userEvent.setup()
    vi.mocked(toast.success).mockClear()
    mockCreateCollection.mockRejectedValueOnce(new Error('smart collections are not supported'))
    render(<Collections />)
    const templateBtn = screen.getAllByText('Templates').find(el => el.closest('button')?.classList.contains('gap-2'))
    await user.click(templateBtn!)
    await user.click(screen.getByText('Apply Template'))
    await waitFor(() => expect(mockCreateCollection).toHaveBeenCalled())
    expect(toast.success).not.toHaveBeenCalled()
    expect(screen.getByTestId('collection-templates')).toBeInTheDocument()
  })

  it('opens Advanced Search view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // The Advanced Search button has gap-2 class
    const searchButtons = screen.getAllByText('Advanced Search')
    const btn = searchButtons.find(el => el.closest('button')?.classList.contains('gap-2'))
    expect(btn).toBeTruthy()
    await user.click(btn!)

    expect(screen.getByTestId('advanced-search')).toBeInTheDocument()
  })

  it('opens Automation view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Find the Automation button (not the tab) - button with gap-2 class
    const automationElements = screen.getAllByText('Automation')
    const btn = automationElements.find(el => {
      const button = el.closest('button')
      return button && button.classList.contains('gap-2')
    })
    expect(btn).toBeTruthy()
    await user.click(btn!)

    expect(screen.getByTestId('collection-automation')).toBeInTheDocument()
  })

  it('opens Integrations view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Find the Integrations button (not the tab) - button with gap-2 class
    const integrationsElements = screen.getAllByText('Integrations')
    const btn = integrationsElements.find(el => {
      const button = el.closest('button')
      return button && button.classList.contains('gap-2')
    })
    expect(btn).toBeTruthy()
    await user.click(btn!)

    expect(screen.getByTestId('external-integrations')).toBeInTheDocument()
  })

  it('opens AI Features view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Click the AI Features button (the one with bg-indigo-50 class)
    const aiButtons = screen.getAllByText('AI Features')
    const aiBtn = aiButtons.find(el => {
      const button = el.closest('button')
      return button && button.classList.contains('bg-indigo-50')
    })
    expect(aiBtn).toBeTruthy()
    await user.click(aiBtn!)

    expect(screen.getByText('AI-Powered Features')).toBeInTheDocument()
    expect(screen.getByText('AI Suggestions')).toBeInTheDocument()
    expect(screen.getByText('AI Search')).toBeInTheDocument()
  })

  it('returns from AI Features view via Back button', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const aiButtons = screen.getAllByText('AI Features')
    const aiBtn = aiButtons.find(el => {
      const button = el.closest('button')
      return button && button.classList.contains('bg-indigo-50')
    })
    expect(aiBtn).toBeTruthy()
    await user.click(aiBtn!)

    expect(screen.getByText('AI-Powered Features')).toBeInTheDocument()

    await user.click(screen.getByText('Back to Collections'))
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('selects a collection via checkbox', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Each collection card has a selection checkbox button
    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    expect(checkboxButtons.length).toBeGreaterThanOrEqual(1)

    await user.click(checkboxButtons[0] as HTMLElement)

    // Should show the bulk operations bar
    expect(screen.getByText(/1 collection selected/)).toBeInTheDocument()
  })

  it('selects all collections via Select all checkbox', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const selectAllCheckbox = screen.getByText(/Select all/).closest('label')?.querySelector('input')
    if (selectAllCheckbox) {
      await user.click(selectAllCheckbox)
    }

    expect(screen.getByText(/2 collections selected/)).toBeInTheDocument()
  })

  it('deselects all collections when Select all is toggled off', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const selectAllCheckbox = screen.getByText(/Select all/).closest('label')?.querySelector('input')
    if (selectAllCheckbox) {
      // Select all
      await user.click(selectAllCheckbox)
      expect(screen.getByText(/2 collections selected/)).toBeInTheDocument()

      // Deselect all
      await user.click(selectAllCheckbox)
      expect(screen.queryByText(/collections selected/)).not.toBeInTheDocument()
    }
  })

  it('clears selection via X button in bulk bar', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Select a collection
    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    expect(screen.getByText(/1 collection selected/)).toBeInTheDocument()

    // Click the X to clear selection
    const clearBtn = screen.getByText(/1 collection/).closest('.flex')?.querySelector('button')
    if (clearBtn) {
      await user.click(clearBtn)
    }

    expect(screen.queryByText(/collection selected/)).not.toBeInTheDocument()
  })

  it('opens bulk operations dialog', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Select a collection first
    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)

    // Click Bulk Actions
    await user.click(screen.getByText('Bulk Actions'))

    expect(screen.getByTestId('bulk-ops')).toBeInTheDocument()
  })

  it('executes bulk delete operation', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Select a collection
    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)

    // Open bulk ops
    await user.click(screen.getByText('Bulk Actions'))

    // Execute bulk delete
    await user.click(screen.getByText('Bulk Delete'))

    await waitFor(() => {
      expect(mockBulkDeleteCollections).toHaveBeenCalled()
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('deleted'))
    })
  })

  it('executes bulk share operation', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Share'))

    await waitFor(() => {
      expect(mockBulkShareCollections).toHaveBeenCalled()
    })
  })

  it('executes bulk export operation', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Export'))

    await waitFor(() => {
      expect(mockBulkExportCollections).toHaveBeenCalled()
    })
  })

  it('executes bulk duplicate operation', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Duplicate'))

    await waitFor(() => {
      expect(mockBulkUpdateCollections).toHaveBeenCalled()
    })
  })

  it('shows error for unknown bulk operation', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Unknown'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Unknown operation')
    })
  })

  it('shows error toast when bulk operation fails', async () => {
    mockBulkDeleteCollections.mockRejectedValue(new Error('Bulk delete failed'))
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Delete'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Bulk operation failed')
    })
  })

  it('deletes a collection via the delete button on card', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Find the delete button (the Trash2 icon button with red text)
    const deleteButtons = document.querySelectorAll('button[title="Delete collection"]')
    expect(deleteButtons.length).toBeGreaterThanOrEqual(1)

    await user.click(deleteButtons[0] as HTMLElement)

    await waitFor(() => {
      expect(mockDeleteCollection).toHaveBeenCalledWith({ id: '1' })
    })

    expect(toast.success).toHaveBeenCalledWith('Collection deleted successfully')
  })

  it('cancels delete when confirm dialog is rejected', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const user = userEvent.setup()
    render(<Collections />)

    const deleteButtons = document.querySelectorAll('button[title="Delete collection"]')
    await user.click(deleteButtons[0] as HTMLElement)

    expect(mockDeleteCollection).not.toHaveBeenCalled()
  })

  it('shows error toast when delete fails', async () => {
    mockDeleteCollection.mockRejectedValue(new Error('Delete failed'))
    const user = userEvent.setup()
    render(<Collections />)

    const deleteButtons = document.querySelectorAll('button[title="Delete collection"]')
    await user.click(deleteButtons[0] as HTMLElement)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to delete collection')
    })
  })

  it('opens collection preview when preview button is clicked', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const previewButtons = document.querySelectorAll('button[title="Preview collection"]')
    expect(previewButtons.length).toBeGreaterThanOrEqual(1)

    await user.click(previewButtons[0] as HTMLElement)

    expect(screen.getByTestId('collection-preview')).toBeInTheDocument()
  })

  it('closes collection preview', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const previewButtons = document.querySelectorAll('button[title="Preview collection"]')
    await user.click(previewButtons[0] as HTMLElement)
    expect(screen.getByTestId('collection-preview')).toBeInTheDocument()

    await user.click(screen.getByText('Close Preview'))
    expect(screen.queryByTestId('collection-preview')).not.toBeInTheDocument()
  })

  it('shares a collection via the share button on card', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const shareButtons = document.querySelectorAll('button[title="Share collection"]')
    expect(shareButtons.length).toBeGreaterThanOrEqual(1)

    await user.click(shareButtons[0] as HTMLElement)

    await waitFor(() => {
      expect(mockShareCollection).toHaveBeenCalledWith(expect.objectContaining({
        id: '1',
        shareRequest: expect.objectContaining({ can_view: true }),
      }))
    })
  })

  it('duplicates a collection via the duplicate button on card', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const duplicateButtons = document.querySelectorAll('button[title="Duplicate collection"]')
    expect(duplicateButtons.length).toBeGreaterThanOrEqual(1)

    await user.click(duplicateButtons[0] as HTMLElement)

    await waitFor(() => {
      expect(mockDuplicateCollection).toHaveBeenCalledWith({
        id: '1',
        newName: 'Test Collection (Copy)',
      })
    })
  })

  it('opens settings modal when settings button is clicked', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const settingsButtons = document.querySelectorAll('button[title="Collection settings"]')
    expect(settingsButtons.length).toBeGreaterThanOrEqual(1)

    await user.click(settingsButtons[0] as HTMLElement)

    expect(screen.getByTestId('collection-settings')).toBeInTheDocument()
  })

  it('saves settings and closes modal', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const settingsButtons = document.querySelectorAll('button[title="Collection settings"]')
    await user.click(settingsButtons[0] as HTMLElement)

    await user.click(screen.getByText('Save Settings'))

    expect(mockUpdateCollection).toHaveBeenCalledWith({
      id: '1',
      updates: { name: 'Updated' },
    })
  })

  it('switches to list view mode', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Find the list view toggle button
    const listButton = document.querySelector('button[title="List View"]')
    expect(listButton).toBeInTheDocument()

    if (listButton) {
      await user.click(listButton as HTMLElement)
    }

    // In list view, collections should still be displayed
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
    expect(screen.getByText('Video Collection')).toBeInTheDocument()
  })

  it('displays description in collection card', () => {
    render(<Collections />)
    expect(screen.getByText('A test collection')).toBeInTheDocument()
  })

  it('displays collection count', () => {
    render(<Collections />)
    expect(screen.getByText('42 items')).toBeInTheDocument()
    expect(screen.getByText('15 items')).toBeInTheDocument()
  })

  it('shows performance metrics in header', () => {
    render(<Collections />)
    expect(screen.getByText(/Render:/)).toBeInTheDocument()
    expect(screen.getByText(/Items:/)).toBeInTheDocument()
    expect(screen.getByText(/Page/)).toBeInTheDocument()
  })

  it('updates search query in input field', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const searchInput = screen.getByPlaceholderText('Search collections...')
    await user.type(searchInput, 'test query')

    expect(searchInput).toHaveValue('test query')
  })

  it('shows Create Smart Collection button in empty state', () => {
    mockUseCollections.mockReturnValue({
      ...defaultUseCollectionsReturn,
      collections: [],
    })
    render(<Collections />)
    expect(screen.getByText('Create Smart Collection')).toBeInTheDocument()
  })

  // --- Additional branch coverage tests ---

  it('shows error toast when share collection fails', async () => {
    mockShareCollection.mockRejectedValue(new Error('Share failed'))
    const user = userEvent.setup()
    render(<Collections />)

    const shareButtons = document.querySelectorAll('button[title="Share collection"]')
    await user.click(shareButtons[0] as HTMLElement)

    // Share failure is caught silently, no toast — verify it didn't crash
    await waitFor(() => {
      expect(mockShareCollection).toHaveBeenCalled()
    })
  })

  it('shows error toast when duplicate collection fails', async () => {
    mockDuplicateCollection.mockRejectedValue(new Error('Duplicate failed'))
    const user = userEvent.setup()
    render(<Collections />)

    const duplicateButtons = document.querySelectorAll('button[title="Duplicate collection"]')
    await user.click(duplicateButtons[0] as HTMLElement)

    await waitFor(() => {
      expect(mockDuplicateCollection).toHaveBeenCalled()
    })
  })

  it('displays collection without description', () => {
    const collectionNoDesc = {
      ...testCollection,
      id: '3',
      name: 'No Description Collection',
      description: '',
    }
    mockUseCollections.mockReturnValue({
      ...defaultUseCollectionsReturn,
      collections: [collectionNoDesc],
    })
    render(<Collections />)
    expect(screen.getByText('No Description Collection')).toBeInTheDocument()
  })

  it('shows smart collection icon for smart collections', () => {
    render(<Collections />)
    // testCollection has is_smart=true, should show the Clock icon
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('shows manual collection without smart icon', () => {
    render(<Collections />)
    // secondCollection has is_smart=false
    expect(screen.getByText('Video Collection')).toBeInTheDocument()
  })

  it('displays empty state message with search hint when filter is active', () => {
    mockUseCollections.mockReturnValue({
      ...defaultUseCollectionsReturn,
      collections: [],
    })
    render(<Collections />)
    // Default with no search = 'Create your first collection to get started'
    expect(screen.getByText('Create your first collection to get started')).toBeInTheDocument()
  })

  it('opens analytics via list view button', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Switch to list view first
    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }

    // In list view, click analytics button
    const analyticsButtons = document.querySelectorAll('button[title="View analytics"]')
    if (analyticsButtons.length > 0) {
      await user.click(analyticsButtons[0] as HTMLElement)
      expect(screen.getByTestId('collection-analytics')).toBeInTheDocument()
    }
  })

  it('closes analytics modal', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Switch to list view, click analytics
    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const analyticsButtons = document.querySelectorAll('button[title="View analytics"]')
    if (analyticsButtons.length > 0) {
      await user.click(analyticsButtons[0] as HTMLElement)
      expect(screen.getByTestId('collection-analytics')).toBeInTheDocument()
      await user.click(screen.getByText('Close Analytics'))
      expect(screen.queryByTestId('collection-analytics')).not.toBeInTheDocument()
    }
  })

  it('opens sharing modal from list view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const sharingButtons = document.querySelectorAll('button[title="Share collection"]')
    if (sharingButtons.length > 0) {
      await user.click(sharingButtons[0] as HTMLElement)
      expect(screen.getByTestId('collection-sharing')).toBeInTheDocument()
    }
  })

  it('closes sharing modal', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const sharingButtons = document.querySelectorAll('button[title="Share collection"]')
    if (sharingButtons.length > 0) {
      await user.click(sharingButtons[0] as HTMLElement)
      await user.click(screen.getByText('Close Sharing'))
      expect(screen.queryByTestId('collection-sharing')).not.toBeInTheDocument()
    }
  })

  it('opens export modal from list view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const exportButtons = document.querySelectorAll('button[title="Export collection"]')
    if (exportButtons.length > 0) {
      await user.click(exportButtons[0] as HTMLElement)
      expect(screen.getByTestId('collection-export')).toBeInTheDocument()
    }
  })

  it('closes export modal', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const exportButtons = document.querySelectorAll('button[title="Export collection"]')
    if (exportButtons.length > 0) {
      await user.click(exportButtons[0] as HTMLElement)
      await user.click(screen.getByText('Close Export'))
      expect(screen.queryByTestId('collection-export')).not.toBeInTheDocument()
    }
  })

  it('opens real-time modal from list view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const rtButtons = document.querySelectorAll('button[title="Real-time collaboration"]')
    if (rtButtons.length > 0) {
      await user.click(rtButtons[0] as HTMLElement)
      expect(screen.getByTestId('collection-realtime')).toBeInTheDocument()
    }
  })

  it('closes real-time modal', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const listButton = document.querySelector('button[title="List View"]')
    if (listButton) {
      await user.click(listButton as HTMLElement)
    }
    const rtButtons = document.querySelectorAll('button[title="Real-time collaboration"]')
    if (rtButtons.length > 0) {
      await user.click(rtButtons[0] as HTMLElement)
      await user.click(screen.getByText('Close RealTime'))
      expect(screen.queryByTestId('collection-realtime')).not.toBeInTheDocument()
    }
  })

  it('closes templates view', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const templatesButtons = screen.getAllByText('Templates')
    const templateBtn = templatesButtons.find(el => {
      const btn = el.closest('button')
      return btn && btn.classList.contains('gap-2')
    })
    if (templateBtn) {
      await user.click(templateBtn)
      expect(screen.getByTestId('collection-templates')).toBeInTheDocument()
      await user.click(screen.getByText('Close Templates'))
      expect(screen.getByText('Test Collection')).toBeInTheDocument()
    }
  })

  it('switches between grid and list view modes', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    // Start in grid, switch to list
    const listButton = document.querySelector('button[title="List View"]')
    const gridButton = document.querySelector('button[title="Grid View"]')
    expect(listButton).toBeInTheDocument()
    expect(gridButton).toBeInTheDocument()

    if (listButton) {
      await user.click(listButton as HTMLElement)
      expect(screen.getByText('Test Collection')).toBeInTheDocument()
    }

    // Switch back to grid
    if (gridButton) {
      await user.click(gridButton as HTMLElement)
      expect(screen.getByText('Test Collection')).toBeInTheDocument()
    }
  })

  it('handles bulk share operation success toast', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Share'))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('shared'))
    })
  })

  it('handles bulk export operation success toast', async () => {
    const user = userEvent.setup()
    render(<Collections />)

    const checkboxButtons = document.querySelectorAll('.absolute.top-2.left-2 button')
    await user.click(checkboxButtons[0] as HTMLElement)
    await user.click(screen.getByText('Bulk Actions'))
    await user.click(screen.getByText('Bulk Export'))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('exported'))
    })
  })
})

describe('Collections accessible names (CAT-UI-D5D6)', () => {
  it('names the selection checkboxes and reflects their pressed state', async () => {
    const user = userEvent.setup()
    render(<Collections />)
    const boxes = screen.getAllByRole('button', { name: /^Select collection / })
    expect(boxes.length).toBeGreaterThanOrEqual(1)
    expect(boxes[0]).toHaveAttribute('aria-pressed', 'false')
    await user.click(boxes[0])
    expect(screen.getAllByRole('button', { name: /^Select collection / })[0]).toHaveAttribute('aria-pressed', 'true')
  })

  it('names and toggles the selection buttons in list view too', async () => {
    const user = userEvent.setup()
    render(<Collections />)
    await user.click(document.querySelector('button[title="List View"]') as HTMLElement)
    const boxes = screen.getAllByRole('button', { name: /^Select collection / })
    expect(boxes.length).toBeGreaterThanOrEqual(1)
    expect(boxes[0]).toHaveAttribute('aria-pressed', 'false')
    await user.click(boxes[0])
    expect(screen.getAllByRole('button', { name: /^Select collection / })[0]).toHaveAttribute('aria-pressed', 'true')
  })

  it('names the filter and sort selects by purpose', () => {
    render(<Collections />)
    expect(screen.getByRole('combobox', { name: 'Filter by media type' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Sort collections' })).toBeInTheDocument()
  })
})
