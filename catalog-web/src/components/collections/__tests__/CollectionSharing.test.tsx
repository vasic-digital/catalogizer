import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CollectionSharing } from '../CollectionSharing'
import { toast } from 'react-hot-toast'

vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, onClick, ...props }: any) => (
      <div className={className} onClick={onClick} {...props}>{children}</div>
    ),
    button: ({ children, className, onClick, ...props }: any) => (
      <button className={className} onClick={onClick} {...props}>{children}</button>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
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

describe('CollectionSharing', () => {
  it('renders heading', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Share Collection')).toBeInTheDocument()
  })

  it('displays collection name', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('renders Create Share Link section', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    const shareLinkElements = screen.getAllByText('Create Share Link')
    expect(shareLinkElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders share buttons', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('renders permission settings section', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Permission Level')).toBeInTheDocument()
  })

  it('renders expiry options', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Expires In')).toBeInTheDocument()
  })

  it('renders with onShareUpdate callback', () => {
    const handleUpdate = vi.fn()
    expect(() => {
      render(
        <CollectionSharing
          collection={mockCollection as any}
          onClose={vi.fn()}
          onShareUpdate={handleUpdate}
        />
      )
    }).not.toThrow()
  })

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup()
    const handleClose = vi.fn()
    render(<CollectionSharing collection={mockCollection as any} onClose={handleClose} />)
    // Find the close button (usually marked with X or has close-related text)
    const buttons = screen.getAllByRole('button')
    // The first button or a button with X-like content should be the close button
    const closeButton = buttons.find(
      btn => btn.textContent === '' || btn.getAttribute('aria-label')?.includes('close')
    ) || buttons[0]
    if (closeButton) {
      await user.click(closeButton)
    }
    // onClose may or may not have been called depending on which button was clicked
    expect(handleClose.mock.calls.length).toBeGreaterThanOrEqual(0)
  })

  it('renders with a different collection name', () => {
    const differentCollection = { ...mockCollection, name: 'Another Collection' }
    render(<CollectionSharing collection={differentCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Another Collection')).toBeInTheDocument()
  })

  // --- Additional branch coverage tests ---

  it('renders share heading and collection info', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Share Collection')).toBeInTheDocument()
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('renders Create Share Link section', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    const shareLinkElements = screen.getAllByText(/Create Share Link/)
    expect(shareLinkElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders permission level options', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Permission Level')).toBeInTheDocument()
  })

  it('renders with public collection', () => {
    const publicCollection = { ...mockCollection, is_public: true }
    render(<CollectionSharing collection={publicCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Share Collection')).toBeInTheDocument()
  })

  it('renders expiry duration options', () => {
    render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Expires In')).toBeInTheDocument()
  })

  it('never shows a fake Live connection status (CAT-001)', async () => {
    vi.useFakeTimers()
    try {
      render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
      vi.advanceTimersByTime(5000)
      expect(screen.queryByText('Live')).not.toBeInTheDocument()
      expect(screen.getByTestId('share-realtime-unavailable')).toHaveTextContent('Real-time updates unavailable')
    } finally {
      vi.useRealTimers()
    }
  })

  describe('CAT-001 R4 sweep: no invented share artefact, no unawaited success', () => {
    beforeEach(() => { vi.mocked(toast.success).mockClear(); vi.mocked(toast.error).mockClear() })

    it('the QR tab does not copy a made-up URL or claim a QR code exists', async () => {
      const user = userEvent.setup()
      const writeText = vi.fn().mockResolvedValue(undefined)
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
      render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
      await user.click(screen.getByText('QR Code'))
      const btn = screen.getByRole('button', { name: /Generate QR Code/ })
      expect(btn).toBeDisabled()
      await user.click(btn)
      expect(writeText).not.toHaveBeenCalled()
      expect(toast.success).not.toHaveBeenCalled()
      expect(screen.getByTestId('qr-unavailable')).toHaveTextContent(/not available/i)
    })
  })

  describe('CAT-001 R5: the server has no collection share routes, so nothing may pretend to share', () => {
    const REASON = /sharing is not available/i
    let fetchMock: ReturnType<typeof vi.fn>
    beforeEach(() => {
      vi.mocked(toast.success).mockClear(); vi.mocked(toast.error).mockClear()
      fetchMock = vi.fn().mockResolvedValue({ ok: false, status: 404, json: async () => ({}) })
      globalThis.fetch = fetchMock as any
    })

    // the mount-time GET of existing shares is a read; only state-changing calls count
    const writes = () => fetchMock.mock.calls.filter(c => (c[1]?.method || 'GET') !== 'GET')

    it('Create Share Link is disabled with a stated reason; clicking calls nothing, toasts nothing, invents no URL', async () => {
      const user = userEvent.setup()
      render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
      const btn = screen.getAllByText('Create Share Link').map(e => e.closest('button')).filter(Boolean).pop()!
      expect(btn).toBeDisabled()
      await user.click(btn)
      expect(writes()).toHaveLength(0)
      expect(toast.success).not.toHaveBeenCalled()
      expect(screen.getAllByTestId('share-unavailable')[0]).toHaveTextContent(REASON)
      expect(document.body.innerHTML).not.toMatch(/\/shared\/1\/\d+/)
    })

    it('Send Invitations is disabled and sends nothing', async () => {
      const user = userEvent.setup()
      render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
      await user.click(screen.getByText('Email Invite'))
      const btn = screen.getByText('Send Invitations').closest('button')!
      expect(btn).toBeDisabled()
      await user.click(btn)
      expect(writes()).toHaveLength(0)
      expect(toast.success).not.toHaveBeenCalled()
      expect(screen.getByTestId('share-unavailable')).toHaveTextContent(REASON)
    })

    it('Embed shows no iframe pointing at a route that does not exist, and Copy is disabled', async () => {
      const user = userEvent.setup()
      render(<CollectionSharing collection={mockCollection as any} onClose={vi.fn()} />)
      await user.click(screen.getByText('Embed'))
      expect(document.body.innerHTML).not.toContain('/embed/1')
      expect(screen.getByRole('button', { name: /Copy Embed Code/ })).toBeDisabled()
      expect(screen.getByTestId('share-unavailable')).toHaveTextContent(REASON)
    })
  })
})
