import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CollectionTemplates } from '../CollectionTemplates'
import { toast } from 'react-hot-toast'

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('framer-motion', () => {
  const MotionDiv = ({ children, onClick, ...props }: any) => (
    <div onClick={onClick} {...props}>{children}</div>
  )
  return {
    motion: { div: MotionDiv },
    AnimatePresence: ({ children }: any) => <>{children}</>,
  }
})

vi.mock('lucide-react', () => {
  const icon = (name: string) => {
    const IconComponent = () => <span data-testid={`icon-${name}`}>{name}</span>
    IconComponent.displayName = `Icon_${name}`
    return IconComponent
  }
  return {
    Plus: icon('plus'),
    Search: icon('search'),
    Filter: icon('filter'),
    Star: icon('star'),
    Heart: icon('heart'),
    Clock: icon('clock'),
    TrendingUp: icon('trending'),
    Folder: icon('folder'),
    Music: icon('music'),
    Video: icon('video'),
    Image: icon('image'),
    FileText: icon('filetext'),
    Grid: icon('grid'),
    List: icon('list'),
    Copy: icon('copy'),
    Download: icon('download'),
    Eye: icon('eye'),
    Calendar: icon('calendar'),
    Tag: icon('tag'),
    BarChart3: icon('barchart'),
    Settings: icon('settings'),
    ChevronRight: icon('chevron'),
    ChevronDown: icon('chevrondown'),
    Zap: icon('zap'),
    BookOpen: icon('book'),
    Film: icon('film'),
    Tv: icon('tv'),
    Headphones: icon('headphones'),
    Camera: icon('camera'),
    Archive: icon('archive'),
    Users: icon('users'),
    Globe: icon('globe'),
    Shield: icon('shield'),
    Sparkles: icon('sparkles'),
    Package: icon('package'),
    X: icon('x'),
  }
})

describe('CollectionTemplates', () => {
  const defaultProps = {
    onClose: vi.fn(),
    onApplyTemplate: vi.fn().mockResolvedValue(undefined),
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the dialog title', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(screen.getByText('Collection Templates')).toBeInTheDocument()
  })

  it('renders subtitle text', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(
      screen.getByText('Choose from pre-built templates to get started')
    ).toBeInTheDocument()
  })

  it('renders search input', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(screen.getByPlaceholderText('Search templates...')).toBeInTheDocument()
  })

  it('renders all templates initially', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(screen.getByText('Recent Movies')).toBeInTheDocument()
    expect(screen.getByText('Music by Genres')).toBeInTheDocument()
    expect(screen.getByText('Photo Library')).toBeInTheDocument()
    expect(screen.getByText('Watchlist Manager')).toBeInTheDocument()
    expect(screen.getByText('Content Review Queue')).toBeInTheDocument()
    expect(screen.getByText('Decades Collection')).toBeInTheDocument()
    expect(screen.getByText('Workspace Projects')).toBeInTheDocument()
    expect(screen.getByText('Smart Suggestions')).toBeInTheDocument()
    expect(screen.getByText('Trending Now')).toBeInTheDocument()
  })

  it('shows template count badge', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(screen.getByText('9 templates')).toBeInTheDocument()
  })

  it('shows All Templates heading by default', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(screen.getByText('All Templates')).toBeInTheDocument()
  })

  it('filters templates by search query', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    const searchInput = screen.getByPlaceholderText('Search templates...')
    await user.type(searchInput, 'movie')

    expect(screen.getByText('Recent Movies')).toBeInTheDocument()
    expect(screen.queryByText('Music by Genres')).not.toBeInTheDocument()
  })

  it('filters by tags when searching', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    const searchInput = screen.getByPlaceholderText('Search templates...')
    await user.type(searchInput, 'trending')

    expect(screen.getByText('Trending Now')).toBeInTheDocument()
    expect(screen.queryByText('Recent Movies')).not.toBeInTheDocument()
  })

  it('shows complexity badges', () => {
    render(<CollectionTemplates {...defaultProps} />)
    const simpleBadges = screen.getAllByText('Simple')
    const mediumBadges = screen.getAllByText('Medium')
    const advancedBadges = screen.getAllByText('Advanced')

    expect(simpleBadges.length).toBeGreaterThan(0)
    expect(mediumBadges.length).toBeGreaterThan(0)
    expect(advancedBadges.length).toBeGreaterThan(0)
  })

  it('shows no invented popularity percentages or sort option (CAT-001)', () => {
    render(<CollectionTemplates {...defaultProps} />)
    for (const invented of ['95%', '96%', '88%', '92%', '85%']) {
      expect(screen.queryByText(invented)).not.toBeInTheDocument()
    }
    expect(screen.queryByText('Most Popular')).not.toBeInTheDocument()
  })

  it('opens template detail when clicked', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))

    // Preview modal should show detailed info
    expect(screen.getByText('Description')).toBeInTheDocument()
    expect(screen.getByText('Metrics')).toBeInTheDocument()
    expect(screen.getByText('Tags')).toBeInTheDocument()
    expect(screen.getByText('Create Collection')).toBeInTheDocument()
  })

  it('shows collection name input in preview', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))

    expect(
      screen.getByPlaceholderText('Enter collection name...')
    ).toBeInTheDocument()
  })

  it('shows error toast when trying to create without name', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))

    // The Create Collection button is disabled when name is empty,
    // so the click handler won't fire - verify the button is disabled instead
    const createBtn = screen.getByText('Create Collection')
    expect(createBtn.closest('button')).toBeDisabled()
  })

  it('CAT-001 R5: Create Collection stays disabled with a stated reason even with a valid name (templates are smart collections, which the server refuses)', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))
    await user.type(screen.getByPlaceholderText('Enter collection name...'), 'My Movies')
    const btn = screen.getByText('Create Collection').closest('button')!
    expect(btn).toBeDisabled()
    await user.click(btn)

    expect(defaultProps.onApplyTemplate).not.toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
    expect(screen.getByTestId('template-apply-unavailable')).toHaveTextContent(/not available/i)
  })

  it('closes template preview when Cancel is clicked', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))
    expect(screen.getByText('Create Collection')).toBeInTheDocument()

    await user.click(screen.getByText('Cancel'))
    // After cancel, the create button should disappear
    expect(screen.queryByText('Create Collection')).not.toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    // The X close button
    const closeIcons = screen.getAllByTestId('icon-x')
    const closeBtn = closeIcons[0].closest('button')
    if (closeBtn) {
      await user.click(closeBtn)
    }

    expect(defaultProps.onClose).toHaveBeenCalled()
  })

  it('shows only the template type in the preview, never invented counts or sizes (CAT-001)', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))

    expect(screen.getByText('Preview')).toBeInTheDocument()
    for (const invented of ['127', '2847', '8472', '45.2 GB', '12.8 GB', '67.3 GB', '892.4 GB']) {
      expect(screen.queryByText(invented)).not.toBeInTheDocument()
    }
    expect(screen.queryByText(/\d+ items$/)).not.toBeInTheDocument()
    // "Video" appears both in the grid template preview and detail modal preview
    const videoElements = screen.getAllByText('Video')
    expect(videoElements.length).toBeGreaterThanOrEqual(1)
  })

  it('shows rules in template detail', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))

    expect(screen.getByText('Rules')).toBeInTheDocument()
  })

  it('shows settings in template detail', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Music by Genres'))

    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('disables Create Collection button when name is empty', async () => {
    const user = userEvent.setup()
    render(<CollectionTemplates {...defaultProps} />)

    await user.click(screen.getByText('Recent Movies'))

    const createBtn = screen.getByText('Create Collection')
    expect(createBtn.closest('button')).toBeDisabled()
  })

  it('shows no invented estimated item ranges (CAT-001)', () => {
    render(<CollectionTemplates {...defaultProps} />)
    expect(screen.queryByText('~50-200')).not.toBeInTheDocument()
  })

  describe('keyboard operability (S1082)', () => {
    it.each([['{Enter}'], [' ']])('opens a grid-view template via key %j', async (key) => {
      const user = userEvent.setup()
      render(<CollectionTemplates {...defaultProps} />)
      const card = screen.getByText('Recent Movies').closest('[role="button"]') as HTMLElement
      expect(card).toHaveAttribute('tabindex', '0')
      card.focus()
      await user.keyboard(key)
      expect(screen.getByText('Description')).toBeInTheDocument()
    })

    it.each([['{Enter}'], [' ']])('opens a list-view template via key %j', async (key) => {
      const user = userEvent.setup()
      render(<CollectionTemplates {...defaultProps} />)
      await user.click(screen.getByTestId('icon-list').closest('button') as HTMLElement)
      const card = screen.getByText('Recent Movies').closest('[role="button"]') as HTMLElement
      expect(card).toHaveAttribute('tabindex', '0')
      card.focus()
      await user.keyboard(key)
      expect(screen.getByText('Description')).toBeInTheDocument()
    })
  })
})
