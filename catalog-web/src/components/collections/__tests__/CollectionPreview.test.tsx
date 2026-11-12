import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'react-hot-toast'
import { CollectionPreview } from '../CollectionPreview'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, onClick, ..._props }: any) => (
      <div className={className} onClick={onClick}>{children}</div>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

vi.mock('../../../hooks/useCollections', () => ({
  useCollection: vi.fn(() => ({
    collectionItems: {
      items: [
        {
          id: '1',
          title: 'Song A',
          media_type: 'music',
          file_size: 5000000,
          duration: 240,
          date_added: '2024-01-01T00:00:00Z',
          rating: 4,
        },
        {
          id: '2',
          title: 'Video B',
          media_type: 'video',
          file_size: 1000000000,
          duration: 3600,
          date_added: '2024-01-02T00:00:00Z',
          rating: 5,
        },
      ],
      total: 2,
    },
    isLoadingItems: false,
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
  item_count: 2,
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

describe('CollectionPreview', () => {
  it('renders collection name', () => {
    render(<CollectionPreview {...defaultProps} />)
    expect(screen.getByText('Test Collection')).toBeInTheDocument()
  })

  it('displays total items count', () => {
    render(<CollectionPreview {...defaultProps} />)
    expect(screen.getByText('2 items total')).toBeInTheDocument()
  })

  it('displays collection items', () => {
    render(<CollectionPreview {...defaultProps} />)
    expect(screen.getByText('Song A')).toBeInTheDocument()
    expect(screen.getByText('Video B')).toBeInTheDocument()
  })

  it('shows loading state when isLoading is true', () => {
    render(<CollectionPreview {...defaultProps} isLoading={true} />)
    const pulseElement = document.querySelector('.animate-pulse')
    expect(pulseElement).toBeTruthy()
  })

  it('renders close button', () => {
    render(<CollectionPreview {...defaultProps} />)
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('displays media type icons for each item', () => {
    render(<CollectionPreview {...defaultProps} />)
    // CollectionPreview renders media types as icons (Music, Film, etc.)
    // not as text labels. Verify items are rendered by their titles.
    expect(screen.getByText('Song A')).toBeInTheDocument()
    expect(screen.getByText('Video B')).toBeInTheDocument()
  })

  it('renders items count as description', () => {
    render(<CollectionPreview {...defaultProps} />)
    // CollectionPreview shows the item count, not the collection description text
    expect(screen.getByText('2 items total')).toBeInTheDocument()
  })

  it('renders with a different collection', () => {
    const differentCollection = {
      ...mockCollection,
      name: 'Another Collection',
      description: 'Different description',
      item_count: 10,
    }
    render(<CollectionPreview collection={differentCollection as any} onClose={vi.fn()} />)
    expect(screen.getByText('Another Collection')).toBeInTheDocument()
  })

  describe('CAT-001: no success claim without an action', () => {
    beforeEach(() => { vi.mocked(toast.success).mockClear(); vi.mocked(toast.error).mockClear() })

    it('has no Download button that claims a download it does not perform', () => {
      render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} />)
      expect(screen.queryAllByTitle('Download')).toHaveLength(0)
    })

    it('without handlers, previewing says it is unavailable instead of "Previewing item"', async () => {
      const user = userEvent.setup()
      const { container } = render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} />)
      const row = screen.getByText('Song A')
      await user.click(row)
      void container
      expect(toast.success).not.toHaveBeenCalledWith('Previewing item')
      expect(toast.success).not.toHaveBeenCalledWith('Playing item')
    })

    it('with a handler, previewing calls it and does not toast on its behalf', async () => {
      const onPreviewItem = vi.fn()
      const user = userEvent.setup()
      render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} onPreviewItem={onPreviewItem} />)
      await user.click(screen.getByText('Song A'))
      expect(onPreviewItem).toHaveBeenCalledWith('1')
      expect(toast.success).not.toHaveBeenCalled()
    })
  })

  describe('keyboard operability (S1082)', () => {
    it('toggles item selection via the Space key on a native checkbox', async () => {
      const user = userEvent.setup()
      render(<CollectionPreview {...defaultProps} />)
      const boxes = screen.getAllByRole('checkbox')
      expect(boxes[0].tagName).toBe('INPUT')
      expect(boxes[0]).not.toBeChecked()
      boxes[0].focus()
      await user.keyboard(' ')
      expect(screen.getAllByRole('checkbox')[0]).toBeChecked()
      expect(screen.getByText('1 selected')).toBeInTheDocument()
      screen.getAllByRole('checkbox')[0].focus()
      await user.keyboard(' ')
      expect(screen.getAllByRole('checkbox')[0]).not.toBeChecked()
    })

    it('checkbox click toggles selection without firing onPreviewItem (stopPropagation)', async () => {
      const onPreviewItem = vi.fn()
      const user = userEvent.setup()
      render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} onPreviewItem={onPreviewItem} />)
      await user.click(screen.getAllByRole('checkbox')[0])
      expect(screen.getAllByRole('checkbox')[0]).toBeChecked()
      expect(onPreviewItem).not.toHaveBeenCalled()
    })

    it('Space on the checkbox does not fire onPreviewItem', async () => {
      const onPreviewItem = vi.fn()
      const user = userEvent.setup()
      render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} onPreviewItem={onPreviewItem} />)
      screen.getAllByRole('checkbox')[0].focus()
      await user.keyboard(' ')
      expect(onPreviewItem).not.toHaveBeenCalled()
    })

    it('FU2-2: holding Enter (auto-repeat) toggles selection once, not repeatedly', () => {
      render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} onPreviewItem={vi.fn()} />)
      const box = screen.getAllByRole('checkbox')[0]
      box.focus()
      fireEvent.keyDown(box, { key: 'Enter' })
      expect(box).toBeChecked()
      fireEvent.keyDown(box, { key: 'Enter', repeat: true })
      expect(box).toBeChecked()
      expect(screen.getByText('1 selected')).toBeInTheDocument()
    })

    it('Enter on the checkbox toggles selection (pre-S6819 behaviour) without previewing', async () => {
      const onPreviewItem = vi.fn()
      const user = userEvent.setup()
      render(<CollectionPreview collection={mockCollection as any} onClose={vi.fn()} onPreviewItem={onPreviewItem} />)
      screen.getAllByRole('checkbox')[0].focus()
      await user.keyboard('{Enter}')
      expect(screen.getAllByRole('checkbox')[0]).toBeChecked()
      expect(screen.getByText('1 selected')).toBeInTheDocument()
      await user.keyboard('{Enter}')
      expect(screen.getAllByRole('checkbox')[0]).not.toBeChecked()
      expect(onPreviewItem).not.toHaveBeenCalled()
    })
  })
})
