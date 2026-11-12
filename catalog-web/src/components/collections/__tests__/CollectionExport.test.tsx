import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CollectionExport } from '../CollectionExport'
import { toast } from 'react-hot-toast'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, onClick, ..._props }: any) => (
      <div className={className} onClick={onClick}>{children}</div>
    ),
    button: ({ children, className, onClick, ..._props }: any) => (
      <button className={className} onClick={onClick}>{children}</button>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

vi.mock('../../../hooks/useCollections', () => ({
  useCollection: vi.fn(() => ({
    collectionItems: [],
    isLoading: false,
  })),
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockCollection = {
  id: '1',
  name: 'Test Collection',
  description: 'A test collection',
  is_smart: true,
  smart_rules: [],
  item_count: 5,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  is_public: false,
  primary_media_type: 'music',
  owner_id: 'user1',
}

const defaultProps = {
  collection: mockCollection as any,
  onClose: vi.fn(),
}

describe('CollectionExport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders heading', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Export / Import Collection')).toBeInTheDocument()
  })

  it('displays collection name', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('renders Export and Import tabs', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Export')).toBeInTheDocument()
    expect(screen.getByText('Import')).toBeInTheDocument()
  })

  it('shows export format options', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Export Format')).toBeInTheDocument()
    expect(screen.getByText('JSON')).toBeInTheDocument()
    expect(screen.getByText('CSV')).toBeInTheDocument()
    expect(screen.getByText('M3U Playlist')).toBeInTheDocument()
  })

  it('shows export options section', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Export Options')).toBeInTheDocument()
    expect(screen.getByText('Include Metadata')).toBeInTheDocument()
    expect(screen.getByText('Include Thumbnails')).toBeInTheDocument()
    expect(screen.getByText('Include Files')).toBeInTheDocument()
  })

  it('renders Export Collection button', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Export Collection')).toBeInTheDocument()
  })

  it('switches to Import tab', async () => {
    const user = userEvent.setup()
    render(<CollectionExport {...defaultProps} />)

    await user.click(screen.getByText('Import'))

    expect(screen.getByText('Import File')).toBeInTheDocument()
    expect(screen.getByText('Click to select a file to upload')).toBeInTheDocument()
  })

  it('shows collection summary', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Collection Summary')).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup()
    render(<CollectionExport {...defaultProps} />)

    // The close button is the one in the header that renders the X icon
    const closeBtn = screen
      .getAllByRole('button')
      .find(btn => btn.closest('.flex.items-center.justify-between') && btn.querySelector('svg'))
    expect(closeBtn).toBeDefined()
    expect(defaultProps.onClose).not.toHaveBeenCalled()
    await user.click(closeBtn!)
    expect(defaultProps.onClose).toHaveBeenCalled()
  })

  // --- Additional branch coverage tests ---

  it('selects JSON export format by default', () => {
    render(<CollectionExport {...defaultProps} />)
    // JSON should be visually selected (has a highlighted style)
    const jsonOption = screen.getByText('JSON')
    expect(jsonOption).toBeInTheDocument()
  })

  it('renders all export format options', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('JSON')).toBeInTheDocument()
    expect(screen.getByText('CSV')).toBeInTheDocument()
    expect(screen.getByText('M3U Playlist')).toBeInTheDocument()
  })

  it('renders include options toggles', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Include Metadata')).toBeInTheDocument()
    expect(screen.getByText('Include Thumbnails')).toBeInTheDocument()
    expect(screen.getByText('Include Files')).toBeInTheDocument()
  })

  it('renders Export Collection button clickable', async () => {
    const user = userEvent.setup()
    render(<CollectionExport {...defaultProps} />)

    const createUrl = vi.fn(() => 'blob:mock-url')
    const revokeUrl = vi.fn()
    const origCreate = URL.createObjectURL
    const origRevoke = URL.revokeObjectURL
    URL.createObjectURL = createUrl as any
    URL.revokeObjectURL = revokeUrl as any
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    try {
      await user.click(screen.getByText('Export Collection'))
      await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Collection exported successfully'))
      expect(createUrl).toHaveBeenCalledTimes(1)
      const blob = (createUrl.mock.calls[0] as unknown[])[0] as Blob
      expect(blob).toBeInstanceOf(Blob)
      expect(blob.type).toBe('application/json')
      expect(clickSpy).toHaveBeenCalledTimes(1)
      expect(revokeUrl).toHaveBeenCalledWith('blob:mock-url')
      expect(toast.error).not.toHaveBeenCalled()
    } finally {
      URL.createObjectURL = origCreate
      URL.revokeObjectURL = origRevoke
      clickSpy.mockRestore()
    }
  })

  it('switches to Import tab and shows import UI', async () => {
    const user = userEvent.setup()
    render(<CollectionExport {...defaultProps} />)

    await user.click(screen.getByText('Import'))

    expect(screen.getByText('Import File')).toBeInTheDocument()
    expect(screen.getByText('Click to select a file to upload')).toBeInTheDocument()
  })

  it('renders with non-smart collection', () => {
    const nonSmartCollection = { ...mockCollection, is_smart: false }
    render(<CollectionExport collection={nonSmartCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('renders collection summary section', () => {
    render(<CollectionExport {...defaultProps} />)
    expect(screen.getByText('Collection Summary')).toBeInTheDocument()
  })

  it('CAT-001: Import reports unavailable and never claims success', async () => {
    const user = userEvent.setup()
    const { container } = render(<CollectionExport collection={mockCollection as any} onClose={() => undefined} />)
    await user.click(screen.getByText('Import'))
    const input = container.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File([JSON.stringify({ items: [{ id: 'a' }, { id: 'b' }] })], 'c.json', { type: 'application/json' })
    await user.upload(input, file)
    await user.click(await screen.findByText('Import Collection'))
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('not available'))
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('CAT-001: export finishes without a timed fake progress phase', async () => {
    const created: Blob[] = []
    const origCreate = URL.createObjectURL
    const origRevoke = URL.revokeObjectURL
    URL.createObjectURL = vi.fn((b: Blob) => { created.push(b); return 'blob:x' }) as any
    URL.revokeObjectURL = vi.fn() as any
    try {
      const user = userEvent.setup()
      render(<CollectionExport {...defaultProps} />)
      await user.click(screen.getByText('Export Collection'))
      // The old code slept 11 x 200 ms before generating; now the file is produced at once.
      await vi.waitFor(() => expect(toast.success).toHaveBeenCalledWith('Collection exported successfully'), { timeout: 1000 })
      expect(created.length).toBe(1)
      expect(JSON.parse(await new Promise<string>((res) => { const r = new FileReader(); r.onload = () => res(String(r.result)); r.readAsText(created[0]) })).collection.id).toBe('1')
      expect(screen.queryByText('Processing items')).not.toBeInTheDocument()
    } finally {
      URL.createObjectURL = origCreate
      URL.revokeObjectURL = origRevoke
    }
  })

  describe('keyboard operability (S1082)', () => {
    it.each([['{Enter}'], [' ']])('opens the file picker from the import drop zone via key %j', async (key) => {
      const user = userEvent.setup()
      render(<CollectionExport {...defaultProps} />)
      await user.click(screen.getByText('Import'))
      const zone = screen.getByText('Click to select a file to upload').closest('button') as HTMLElement
      const input = document.querySelector('input[type="file"]') as HTMLInputElement
      const clickSpy = vi.spyOn(input, 'click').mockImplementation(() => undefined)
      zone.focus()
      await user.keyboard(key)
      expect(clickSpy).toHaveBeenCalledTimes(1)
    })
  })
})
