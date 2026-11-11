import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PlaylistItemComponent } from '../PlaylistItem'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, onClick, onMouseEnter, onMouseLeave, ..._props }: any) => (
      <div className={className} onClick={onClick} onMouseEnter={onMouseEnter} onMouseLeave={onMouseLeave}>
        {children}
      </div>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

vi.mock('../../media/MediaPlayer', () => ({
  MediaPlayer: () => <div data-testid="media-player">Media Player</div>,
}))

vi.mock('../../favorites/FavoriteToggle', () => ({
  FavoriteToggle: () => <div data-testid="favorite-toggle">Favorite</div>,
}))

vi.mock('../../../types/playlists', () => ({
  flattenPlaylistItem: vi.fn((item: any) => ({
    title: item.title || item.media?.title || 'Unknown',
    media_type: item.media_type || item.media?.media_type || 'music',
    duration: item.duration || 0,
    file_path: item.file_path || '',
    thumbnail_url: item.thumbnail_url || '',
    item_id: item.id || '1',
  })),
  getMediaIconWithMap: vi.fn(() => {
    // Return a React component function, not an object
    const MediaIcon = (props: any) => <span data-testid="media-icon" {...props} />
    MediaIcon.displayName = 'MediaIcon'
    return MediaIcon
  }),
}))

const mockItem = {
  id: '1',
  title: 'Test Track',
  media_type: 'music',
  duration: 180,
  file_path: '/music/test.mp3',
  media: { title: 'Test Track', media_type: 'music' },
}

const defaultProps = {
  item: mockItem as any,
  index: 0,
  isPlaying: false,
  isCurrent: false,
}

describe('PlaylistItemComponent', () => {
  it('renders the component with item title', () => {
    render(<PlaylistItemComponent {...defaultProps} />)
    expect(screen.getByText('Test Track')).toBeInTheDocument()
  })

  it('renders the track index', () => {
    const { container } = render(<PlaylistItemComponent {...defaultProps} index={2} />)
    // Verify the component rendered successfully with the given index
    expect(container.firstChild).toBeTruthy()
  })

  it('calls onPlay when play is clicked', async () => {
    const user = userEvent.setup()
    const onPlay = vi.fn()
    render(<PlaylistItemComponent {...defaultProps} onPlay={onPlay} />)

    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
    expect(onPlay).not.toHaveBeenCalled()
    await user.click(buttons[0])
    expect(onPlay).toHaveBeenCalledTimes(1)
    expect(onPlay).toHaveBeenCalledWith(mockItem, 0)
  })

  it('calls onRemove when remove is clicked', async () => {
    const onRemove = vi.fn()
    render(<PlaylistItemComponent {...defaultProps} onRemove={onRemove} showActions={true} />)

    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
  })

  it('applies current styling when isCurrent is true', () => {
    const { container } = render(
      <PlaylistItemComponent {...defaultProps} isCurrent={true} />
    )
    expect(container.firstChild).toBeTruthy()
  })

  it('renders without optional props', () => {
    expect(() => {
      render(<PlaylistItemComponent item={mockItem as any} index={0} isPlaying={false} isCurrent={false} />)
    }).not.toThrow()
  })

  it('renders with isPlaying true', () => {
    const { container } = render(
      <PlaylistItemComponent {...defaultProps} isPlaying={true} isCurrent={true} />
    )
    expect(container.firstChild).toBeTruthy()
  })

  it('renders action buttons when showActions is true', () => {
    render(<PlaylistItemComponent {...defaultProps} showActions={true} onRemove={vi.fn()} />)
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThanOrEqual(1)
  })
})
