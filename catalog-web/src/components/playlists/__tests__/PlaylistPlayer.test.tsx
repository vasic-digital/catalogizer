import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PlaylistPlayer } from '../PlaylistPlayer'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, ..._props }: any) => <div className={className}>{children}</div>,
    button: ({ children, className, onClick, ..._props }: any) => (
      <button className={className} onClick={onClick}>{children}</button>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

// Icons carry a data-icon marker (the real lucide icons drop their identifying
// class when a className is passed), so controls can be located by icon.
vi.mock('lucide-react', () => {
  const names = ['GripVertical','Play','Pause','SkipForward','SkipBack','Volume2','Maximize2','MoreHorizontal','Shuffle','Repeat','List','Share','BarChart3','X','ChevronLeft','ChevronRight','Music','FileText']
  return Object.fromEntries(names.map(n => [n, ({ className }: { className?: string }) => <svg data-icon={n} className={className} />]))
})

vi.mock('../../media/MediaPlayer', () => ({
  MediaPlayer: () => <div data-testid="media-player">Media Player</div>,
}))

vi.mock('../../favorites/FavoriteToggle', () => ({
  FavoriteToggle: () => <div data-testid="favorite-toggle">Favorite</div>,
}))

vi.mock('../../../lib/playlistsApi', () => ({
  playlistsApi: {
    updatePlayHistory: vi.fn(),
  },
}))

vi.mock('../PlaylistAnalytics', () => ({
  PlaylistAnalytics: () => <div data-testid="playlist-analytics">Analytics</div>,
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock types/playlists to handle flattenPlaylistItem
vi.mock('../../../types/playlists', () => ({
  flattenPlaylistItem: vi.fn((item: any) => ({
    title: item.title || item.media_item?.title || 'Unknown',
    media_type: item.media_type || item.media_item?.media_type || 'music',
    duration: item.duration || 0,
    file_path: item.file_path || '',
    thumbnail_url: item.thumbnail_url || '',
    item_id: item.id || '1',
    artist: item.artist || '',
    album: item.album || '',
  })),
  getMediaIconWithMap: vi.fn(() => {
    const MediaIcon = (props: any) => <span data-testid="media-icon" {...props} />
    MediaIcon.displayName = 'MediaIcon'
    return MediaIcon
  }),
}))

const mockPlaylist = {
  id: '1',
  name: 'Test Playlist',
  description: 'A test playlist',
  is_public: false,
  primary_media_type: 'music',
  items: [],
  item_count: 2,
}

const mockItems = [
  {
    id: '1',
    title: 'Track 1',
    media_type: 'music',
    duration: 180,
    file_path: '/music/track1.mp3',
    media_item: { id: '1', title: 'Track 1', media_type: 'music' },
  },
  {
    id: '2',
    title: 'Track 2',
    media_type: 'music',
    duration: 240,
    file_path: '/music/track2.mp3',
    media_item: { id: '2', title: 'Track 2', media_type: 'music' },
  },
]

// Controls have no accessible names, so locate them by their icon.
const buttonWithIcon = (icon: string): HTMLButtonElement => {
  const svg = document.querySelector(`button svg[data-icon="${icon}"]`)
  expect(svg).not.toBeNull()
  return svg!.closest('button') as HTMLButtonElement
}

describe('PlaylistPlayer', () => {
  it('renders playlist name', () => {
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    // "Test Playlist" appears in header and player display
    const playlistNames = screen.getAllByText('Test Playlist')
    expect(playlistNames.length).toBeGreaterThanOrEqual(1)
  })

  it('renders player controls', () => {
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    // Should have play/pause and skip controls
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('shows current track info', () => {
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    // "Track 1" appears in now playing section and track list
    const trackElements = screen.getAllByText('Track 1')
    expect(trackElements.length).toBeGreaterThanOrEqual(1)
  })

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <PlaylistPlayer
        playlist={mockPlaylist as any}
        items={mockItems as any}
        onClose={onClose}
      />
    )

    expect(onClose).not.toHaveBeenCalled()
    await user.click(buttonWithIcon('X'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('displays track list', () => {
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    // Track names appear in track list (may appear multiple times due to now-playing display)
    const track1Elements = screen.getAllByText('Track 1')
    expect(track1Elements.length).toBeGreaterThanOrEqual(1)
    const track2Elements = screen.getAllByText('Track 2')
    expect(track2Elements.length).toBeGreaterThanOrEqual(1)
  })

  // --- Additional branch coverage tests ---

  it('renders with empty items array without crashing', () => {
    const { container } = render(<PlaylistPlayer playlist={mockPlaylist as any} items={[]} />)
    // Should render without crashing even with no items
    expect(container.firstChild).toBeTruthy()
  })

  it('handles play/pause toggle', async () => {
    const user = userEvent.setup()
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)

    const playBtn = screen.getAllByRole('button').find(btn => btn.classList.contains('rounded-full'))!
    expect(playBtn).toBeDefined()
    expect(playBtn.querySelector('svg[data-icon="Play"]')).not.toBeNull()
    await user.click(playBtn)
    expect(playBtn.querySelector('svg[data-icon="Pause"]')).not.toBeNull()
    expect(playBtn.querySelector('svg[data-icon="Play"]')).toBeNull()
    await user.click(playBtn)
    expect(playBtn.querySelector('svg[data-icon="Play"]')).not.toBeNull()
    expect(playBtn.querySelector('svg[data-icon="Pause"]')).toBeNull()
  })

  it('handles next track navigation', async () => {
    const user = userEvent.setup()
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)

    expect(screen.getByText('1 of 2')).toBeInTheDocument()
    const nextBtn = buttonWithIcon('SkipForward')
    expect(nextBtn).not.toBeDisabled()
    await user.click(nextBtn)
    expect(screen.getByText('2 of 2')).toBeInTheDocument()
    // Last track: next is now disabled
    expect(nextBtn).toBeDisabled()
  })

  it('previous button goes back and is disabled at the first track', async () => {
    const user = userEvent.setup()
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    const prevBtn = buttonWithIcon('SkipBack')
    expect(screen.getByText('1 of 2')).toBeInTheDocument()
    expect(prevBtn).toBeDisabled()
    await user.click(buttonWithIcon('SkipForward'))
    expect(screen.getByText('2 of 2')).toBeInTheDocument()
    expect(prevBtn).not.toBeDisabled()
    await user.click(prevBtn)
    expect(screen.getByText('1 of 2')).toBeInTheDocument()
    expect(prevBtn).toBeDisabled()
  })

  it('repeat-all wraps next from last to first and previous from first to last', async () => {
    const user = userEvent.setup()
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    await user.click(buttonWithIcon('Repeat')) // off -> all
    const nextBtn = buttonWithIcon('SkipForward')
    const prevBtn = buttonWithIcon('SkipBack')
    expect(prevBtn).not.toBeDisabled()
    await user.click(prevBtn) // first -> last
    expect(screen.getByText('2 of 2')).toBeInTheDocument()
    expect(nextBtn).not.toBeDisabled()
    await user.click(nextBtn) // last -> first
    expect(screen.getByText('1 of 2')).toBeInTheDocument()
  })

  it('handles shuffle toggle', async () => {
    const onShuffle = vi.fn()
    const user = userEvent.setup()
    render(
      <PlaylistPlayer
        playlist={mockPlaylist as any}
        items={mockItems as any}
        onShuffle={onShuffle}
      />
    )

    const shuffleBtn = buttonWithIcon('Shuffle')
    expect(shuffleBtn.className).not.toContain('text-blue-600')
    await user.click(shuffleBtn)
    expect(onShuffle).toHaveBeenCalledTimes(1)
    expect(shuffleBtn.className).toContain('text-blue-600')
    await user.click(shuffleBtn)
    expect(onShuffle).toHaveBeenCalledTimes(2)
    expect(shuffleBtn.className).not.toContain('text-blue-600')
  })

  it('handles repeat mode cycling', async () => {
    const onRepeat = vi.fn()
    const user = userEvent.setup()
    render(
      <PlaylistPlayer
        playlist={mockPlaylist as any}
        items={mockItems as any}
        onRepeat={onRepeat}
      />
    )

    const repeatBtn = buttonWithIcon('Repeat')
    const icon = () => repeatBtn.querySelector('svg')!.getAttribute('class') ?? ''
    expect(repeatBtn.className).not.toContain('text-blue-600')
    await user.click(repeatBtn) // off -> all
    expect(onRepeat).toHaveBeenCalledTimes(1)
    expect(repeatBtn.className).toContain('text-blue-600')
    expect(icon()).not.toContain('scale-x-[-1]')
    await user.click(repeatBtn) // all -> one
    expect(onRepeat).toHaveBeenCalledTimes(2)
    expect(icon()).toContain('scale-x-[-1]')
    await user.click(repeatBtn) // one -> off
    expect(onRepeat).toHaveBeenCalledTimes(3)
    expect(repeatBtn.className).not.toContain('text-blue-600')
  })

  it('renders with initialIndex prop', () => {
    render(
      <PlaylistPlayer
        playlist={mockPlaylist as any}
        items={mockItems as any}
        initialIndex={1}
      />
    )
    // Track 2 should be the current track
    const track2Elements = screen.getAllByText('Track 2')
    expect(track2Elements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders with className prop', () => {
    const { container } = render(
      <PlaylistPlayer
        playlist={mockPlaylist as any}
        items={mockItems as any}
        className="custom-player-class"
      />
    )
    expect(container.firstChild).toBeTruthy()
  })

  it('shows progress info for current position', () => {
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    // Should show "1 of 2" somewhere
    expect(screen.getByText('1 of 2')).toBeInTheDocument()
  })

  it('displays remaining time information', () => {
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={mockItems as any} />)
    // Should show remaining time
    const remainingText = screen.getByText(/remaining/)
    expect(remainingText).toBeInTheDocument()
  })

  it('renders with single item playlist', () => {
    const singleItem = [mockItems[0]]
    render(<PlaylistPlayer playlist={mockPlaylist as any} items={singleItem as any} />)

    const track1Elements = screen.getAllByText('Track 1')
    expect(track1Elements.length).toBeGreaterThanOrEqual(1)
  })
})
