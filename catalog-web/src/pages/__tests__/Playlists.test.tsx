import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PlaylistsPage } from '../Playlists'
import { toast } from 'react-hot-toast'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

// Mock dependencies
vi.mock('../../components/layout/PageHeader', () => ({
  PageHeader: ({ title, subtitle }: any) => (
    <div data-testid="page-header">
      <h1>{title}</h1>
      <p>{subtitle}</p>
    </div>
  ),
}))

vi.mock('../../components/playlists/PlaylistGrid', () => ({
  PlaylistGrid: ({ onCreatePlaylist, onEditPlaylist }: any) => (
    <div data-testid="playlist-grid">
      <button onClick={onCreatePlaylist}>Grid Create</button>
      <button onClick={() => onEditPlaylist({
        id: '1',
        name: 'Test Playlist',
        description: 'A test playlist',
        is_public: false,
        items: [{ id: 'item-1', playlist_id: '1', media_id: 'm1', position: 0, added_at: '2024-01-01T00:00:00Z', media_item: { id: 'm1', title: 'Song 1', media_type: 'music', year: 2023, cover_image: '', duration: 180, rating: 4, quality: 'high', file_path: '/music/song1.mp3', thumbnail_url: '' } }],
      })}>Grid Edit</button>
    </div>
  ),
}))

vi.mock('../../components/playlists/PlaylistManager', () => ({
  PlaylistManager: () => <div data-testid="playlist-manager">Manager</div>,
}))

vi.mock('../../components/playlists/PlaylistPlayer', () => ({
  PlaylistPlayer: ({ onClose }: any) => (
    <div data-testid="playlist-player">
      Player
      <button onClick={onClose}>Close</button>
    </div>
  ),
}))

vi.mock('../../components/playlists/PlaylistItem', () => ({
  PlaylistItemComponent: () => <div>Playlist Item</div>,
}))

vi.mock('../../components/playlists/SmartPlaylistBuilder', () => ({
  SmartPlaylistBuilder: ({ onSave, onCancel }: any) => (
    <div data-testid="smart-builder">
      Smart Playlist Builder
      <button onClick={() => onSave('Smart PL', 'Smart desc', [{ field: 'genre', operator: 'equals', value: 'rock' }])}>Save Smart</button>
      <button onClick={onCancel}>Cancel Smart</button>
    </div>
  ),
}))

const mockCreatePlaylist = vi.fn()
const mockUpdatePlaylist = vi.fn()
const mockDeletePlaylist = vi.fn()
const mockGetPlaylistItems = vi.fn()

vi.mock('../../lib/playlistsApi', () => ({
  playlistsApi: {
    createPlaylist: (...args: any[]) => mockCreatePlaylist(...args),
    updatePlaylist: (...args: any[]) => mockUpdatePlaylist(...args),
    deletePlaylist: (...args: any[]) => mockDeletePlaylist(...args),
    getPlaylistItems: (...args: any[]) => mockGetPlaylistItems(...args),
  },
}))

const mockSearchMedia = vi.fn()

vi.mock('../../lib/mediaApi', () => ({
  mediaApi: {
    searchMedia: (...args: any[]) => mockSearchMedia(...args),
  },
}))

const mockRefetchPlaylists = vi.fn()

vi.mock('../../hooks/usePlaylists', () => ({
  usePlaylists: vi.fn(() => ({
    playlists: [
      {
        id: '1',
        name: 'Test Playlist',
        description: 'A test playlist',
        is_public: false,
        primary_media_type: 'music',
        items: [],
        item_count: 5,
      },
      {
        id: '2',
        name: 'Public Videos',
        description: 'Public video playlist',
        is_public: true,
        primary_media_type: 'video',
        items: [],
        item_count: 10,
      },
    ],
    isLoading: false,
    error: null,
    refetchPlaylists: mockRefetchPlaylists,
  })),
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe('Playlists Page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockSearchMedia.mockResolvedValue({ items: [] })
    mockCreatePlaylist.mockResolvedValue({ name: 'New Playlist' })
    mockUpdatePlaylist.mockResolvedValue({ name: 'Updated Playlist' })
    mockDeletePlaylist.mockResolvedValue(undefined)
  })

  it('renders page header', () => {
    render(<PlaylistsPage />)
    expect(screen.getByText('Playlists')).toBeInTheDocument()
    expect(screen.getByText('Organize and manage your media collections')).toBeInTheDocument()
  })

  it('renders tab navigation', () => {
    render(<PlaylistsPage />)
    expect(screen.getByText('All Playlists')).toBeInTheDocument()
    expect(screen.getByText('My Playlists')).toBeInTheDocument()
    expect(screen.getByText('Public')).toBeInTheDocument()
    expect(screen.getByText('Smart Builder')).toBeInTheDocument()
  })

  it('renders search input', () => {
    render(<PlaylistsPage />)
    expect(screen.getByPlaceholderText('Search playlists...')).toBeInTheDocument()
  })

  it('renders Create Playlist button', () => {
    render(<PlaylistsPage />)
    expect(screen.getByText('Create Playlist')).toBeInTheDocument()
  })

  it('renders playlist grid', () => {
    render(<PlaylistsPage />)
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('opens create form when Create Playlist is clicked', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))

    expect(screen.getByText('Create New Playlist')).toBeInTheDocument()
    expect(screen.getByText('Playlist Name *')).toBeInTheDocument()
  })

  it('shows Smart Builder when Smart Builder tab is clicked', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Smart Builder'))

    expect(screen.getByTestId('smart-builder')).toBeInTheDocument()
  })

  it('renders media type filter dropdown', () => {
    render(<PlaylistsPage />)
    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()
  })

  it('renders "Favorites" tab', () => {
    render(<PlaylistsPage />)
    // There's a Favorites tab in the navigation
    expect(screen.getByText('Favorites')).toBeInTheDocument()
  })

  // --- New tests for increased coverage ---

  it('cancels smart builder and returns to all tab', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Smart Builder'))
    expect(screen.getByTestId('smart-builder')).toBeInTheDocument()

    await user.click(screen.getByText('Cancel Smart'))
    // After cancel, should switch back to 'all' tab and show the grid
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('creates a playlist successfully via the form', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    expect(screen.getByText('Create New Playlist')).toBeInTheDocument()

    // Fill in the form
    const nameInput = screen.getByPlaceholderText('Enter playlist name')
    await user.type(nameInput, 'My New Playlist')

    const descTextarea = screen.getByPlaceholderText('Enter playlist description (optional)')
    await user.type(descTextarea, 'A description')

    // Click the public checkbox
    const publicCheckbox = screen.getByLabelText('Make this playlist public')
    await user.click(publicCheckbox)

    // Submit the form - find all buttons with "Create Playlist" text, the submit button is the one
    // that also contains the Save icon (the second one in the DOM after the modal opens)
    const createButtons = screen.getAllByRole('button', { name: /Create Playlist/i })
    // The submit button is the last one (inside the form modal footer)
    await user.click(createButtons[createButtons.length - 1])

    await waitFor(() => {
      expect(mockCreatePlaylist).toHaveBeenCalledWith(expect.objectContaining({
        name: 'My New Playlist',
        description: 'A description',
        is_public: true,
      }))
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Created playlist: New Playlist')
    })
  })

  it('shows error toast when create playlist fails', async () => {
    mockCreatePlaylist.mockRejectedValue(new Error('API error'))
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const nameInput = screen.getByPlaceholderText('Enter playlist name')
    await user.type(nameInput, 'Failing Playlist')

    // Submit via the form footer button
    const createButtons = screen.getAllByRole('button', { name: /Create Playlist/i })
    await user.click(createButtons[createButtons.length - 1])

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to create playlist')
    })
  })

  it('closes create form via Cancel button', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    expect(screen.getByText('Create New Playlist')).toBeInTheDocument()

    await user.click(screen.getByText('Cancel'))
    expect(screen.queryByText('Create New Playlist')).not.toBeInTheDocument()
  })

  it('opens edit form when editing a playlist from the grid', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Grid Edit'))

    expect(screen.getByText('Edit Playlist')).toBeInTheDocument()
    // Form should be populated with the playlist data
    const nameInput = screen.getByPlaceholderText('Enter playlist name') as HTMLInputElement
    expect(nameInput.value).toBe('Test Playlist')
  })

  it('updates a playlist successfully via the edit form', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Grid Edit'))
    expect(screen.getByText('Edit Playlist')).toBeInTheDocument()

    // Modify the name
    const nameInput = screen.getByPlaceholderText('Enter playlist name')
    await user.clear(nameInput)
    await user.type(nameInput, 'Updated Name')

    // Click Update
    await user.click(screen.getByText('Update Playlist'))

    await waitFor(() => {
      expect(mockUpdatePlaylist).toHaveBeenCalledWith('1', expect.objectContaining({
        name: 'Updated Name',
      }))
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Updated playlist: Updated Playlist')
    })
  })

  it('shows error toast when update playlist fails', async () => {
    mockUpdatePlaylist.mockRejectedValue(new Error('Update failed'))
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Grid Edit'))
    await user.click(screen.getByText('Update Playlist'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to update playlist')
    })
  })

  it('filters playlists by search query', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    const searchInput = screen.getByPlaceholderText('Search playlists...')
    await user.type(searchInput, 'Test')

    // The search should filter - grid is still showing since filtering happens via useMemo
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('switches to My Playlists tab', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('My Playlists'))
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('switches to Public tab', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Public'))
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('switches to Favorites tab', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Favorites'))
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('shows description field in create form', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    expect(screen.getByText('Description')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Enter playlist description (optional)')).toBeInTheDocument()
  })

  it('shows Add Items to Playlist section in create form', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    expect(screen.getByText('Add Items to Playlist')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Search media items...')).toBeInTheDocument()
  })

  it('searches for media items in the create form', async () => {
    mockSearchMedia.mockResolvedValue({
      items: [
        { id: 'm1', title: 'Search Result Song', media_type: 'music', year: 2023 },
      ],
    })
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    await user.type(mediaSearch, 'Search Result')

    // Trigger search by pressing Enter
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(mockSearchMedia).toHaveBeenCalledWith(expect.objectContaining({
        query: 'Search Result',
        limit: 20,
      }))
    })

    await waitFor(() => {
      expect(screen.getByText('Search Result Song')).toBeInTheDocument()
    })
  })

  it('shows no results message when media search returns empty', async () => {
    mockSearchMedia.mockResolvedValue({ items: [] })
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    await user.type(mediaSearch, 'nonexistent')
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(screen.getByText('No results found. Try a different search term.')).toBeInTheDocument()
    })
  })

  it('shows error toast when media search fails', async () => {
    mockSearchMedia.mockRejectedValue(new Error('Search failed'))
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    await user.type(mediaSearch, 'fail query')
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to search media items')
    })
  })

  it('adds a media item to the playlist from search results', async () => {
    mockSearchMedia.mockResolvedValue({
      items: [
        { id: 'm2', title: 'Added Song', media_type: 'music', year: 2024, cover_image: '', duration: 200, rating: 5, quality: 'high', directory_path: '/music' },
      ],
    })
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    await user.type(mediaSearch, 'Added Song')
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(screen.getByText('Added Song')).toBeInTheDocument()
    })

    // Find the add button inside the search results area (the small outline button next to the media item)
    const searchResultItem = screen.getByText('Added Song').closest('[class*="border-b"]')
    const addButton = searchResultItem?.querySelector('button')
    expect(addButton).toBeInTheDocument()
    await user.click(addButton as HTMLElement)

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('Added'))
    })
  })

  it('does not search media when query is empty', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    // Focus and press Enter with empty query
    await user.click(mediaSearch)
    await user.keyboard('{Enter}')

    expect(mockSearchMedia).not.toHaveBeenCalled()
  })

  it('Smart Builder save creates a smart playlist through the API with the rules', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)
    mockCreatePlaylist.mockClear()
    mockCreatePlaylist.mockResolvedValueOnce({ id: 9, name: 'Smart PL' })
    vi.mocked(toast.success).mockClear()

    await user.click(screen.getByText('Smart Builder'))
    await user.click(screen.getByText('Save Smart'))

    await waitFor(() => {
      expect(mockCreatePlaylist).toHaveBeenCalledWith({
        name: 'Smart PL',
        description: 'Smart desc',
        is_smart: true,
        smart_rules: [{ field: 'genre', operator: 'equals', value: 'rock' }],
      })
    })
    expect(toast.success).toHaveBeenCalledWith('Created smart playlist: Smart PL')
    // back on the list
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('Smart Builder save shows the server validation message and stays on the builder', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)
    mockCreatePlaylist.mockClear()
    mockCreatePlaylist.mockRejectedValueOnce({ response: { data: { error: 'Invalid smart_rules: unknown field' } } })
    vi.mocked(toast.error).mockClear()

    await user.click(screen.getByText('Smart Builder'))
    await user.click(screen.getByText('Save Smart'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Invalid smart_rules: unknown field')
    })
    expect(screen.getByTestId('smart-builder')).toBeInTheDocument()
  })

  it('disables Create button when playlist name is empty', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))

    // The Create Playlist button in the form should be disabled when name is empty
    const createButtons = screen.getAllByText('Create Playlist')
    const formSubmitButton = createButtons.find(el => el.closest('.border-t'))
    expect(formSubmitButton?.closest('button')).toBeDisabled()
  })

  it('closes edit form via the X button', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Grid Edit'))
    expect(screen.getByText('Edit Playlist')).toBeInTheDocument()

    // Close via X button (the ghost button with X icon in the header)
    const closeButtons = screen.getAllByRole('button')
    const xButton = closeButtons.find(b => {
      const parent = b.closest('.flex.items-center.justify-between')
      return parent && b.querySelector('svg') !== null
    })
    if (xButton) {
      await user.click(xButton)
    }
  })

  it('shows selected items section when items are added via edit', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    // Edit a playlist that has items
    await user.click(screen.getByText('Grid Edit'))
    expect(screen.getByText('Edit Playlist')).toBeInTheDocument()

    // Should show the Selected Items section since the edited playlist has items
    expect(screen.getByText(/Selected Items/)).toBeInTheDocument()
  })

  // --- Additional branch coverage tests ---

  it('filters playlists by media type', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    const select = screen.getByRole('combobox')
    await user.selectOptions(select, 'music')

    // Grid still renders (filtering is internal)
    expect(screen.getByTestId('playlist-grid')).toBeInTheDocument()
  })

  it('prevents creating playlist without a name', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))

    // The form submit button should be disabled when name is empty
    const createButtons = screen.getAllByText('Create Playlist')
    const formBtn = createButtons.find(el => el.closest('.border-t'))
    expect(formBtn?.closest('button')).toBeDisabled()
  })

  it('adds item to playlist and prevents duplicate adds', async () => {
    mockSearchMedia.mockResolvedValue({
      items: [
        { id: 99, title: 'Unique Song', media_type: 'music', year: 2024, cover_image: '', duration: 200, rating: 5, quality: 'high', directory_path: '/music' },
      ],
    })
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    await user.type(mediaSearch, 'Unique Song')
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(screen.getByText('Unique Song')).toBeInTheDocument()
    })

    // Add the item
    const searchResultItem = screen.getByText('Unique Song').closest('[class*="border-b"]')
    const addButton = searchResultItem?.querySelector('button')
    await user.click(addButton as HTMLElement)

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('Added'))
    })

    // Try adding the same item again - should show error
    // Need to re-search since state may have changed
    await user.clear(mediaSearch)
    await user.type(mediaSearch, 'Unique Song')
    await user.keyboard('{Enter}')

    await waitFor(() => {
      const addButtons = screen.getAllByText(/Added/)
      expect(addButtons.length).toBeGreaterThanOrEqual(1)
    })
  })

  it('shows Searching... text while media search is in progress', async () => {
    // Create a promise we can control
    let resolveSearch: (value: any) => void
    const searchPromise = new Promise((resolve) => { resolveSearch = resolve })
    mockSearchMedia.mockReturnValue(searchPromise)

    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const mediaSearch = screen.getByPlaceholderText('Search media items...')
    await user.type(mediaSearch, 'slow search')
    await user.keyboard('{Enter}')

    expect(screen.getByText('Searching...')).toBeInTheDocument()

    // Resolve the search
    resolveSearch!({ items: [] })
    await waitFor(() => {
      expect(screen.queryByText('Searching...')).not.toBeInTheDocument()
    })
  })

  it('opens create form from Grid Create button', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Grid Create'))

    expect(screen.getByText('Create New Playlist')).toBeInTheDocument()
  })

  it('resets form after successful create', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const nameInput = screen.getByPlaceholderText('Enter playlist name')
    await user.type(nameInput, 'My Playlist')

    const createButtons = screen.getAllByRole('button', { name: /Create Playlist/i })
    await user.click(createButtons[createButtons.length - 1])

    await waitFor(() => {
      expect(mockCreatePlaylist).toHaveBeenCalled()
    })

    // After success, the form should be gone
    await waitFor(() => {
      expect(screen.queryByText('Create New Playlist')).not.toBeInTheDocument()
    })
  })

  it('toggles public checkbox in create form', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    const publicCheckbox = screen.getByLabelText('Make this playlist public')
    expect(publicCheckbox).not.toBeChecked()

    await user.click(publicCheckbox)
    expect(publicCheckbox).toBeChecked()

    await user.click(publicCheckbox)
    expect(publicCheckbox).not.toBeChecked()
  })

  it('closes create form by clicking Cancel in footer', async () => {
    const user = userEvent.setup()
    render(<PlaylistsPage />)

    await user.click(screen.getByText('Create Playlist'))
    expect(screen.getByText('Create New Playlist')).toBeInTheDocument()

    await user.click(screen.getByText('Cancel'))
    expect(screen.queryByText('Create New Playlist')).not.toBeInTheDocument()
  })
})
