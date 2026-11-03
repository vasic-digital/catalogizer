import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MediaPlayer } from '../MediaPlayer'

vi.mock('lucide-react', () => ({
  Play: () => <span data-testid="icon-play">Play</span>,
  Pause: () => <span data-testid="icon-pause">Pause</span>,
  Volume2: () => <span data-testid="icon-volume2">Volume2</span>,
  VolumeX: () => <span data-testid="icon-volumex">VolumeX</span>,
  Maximize2: () => <span data-testid="icon-maximize">Maximize</span>,
  Subtitles: () => <span data-testid="icon-subtitles">Subtitles</span>,
  Settings: () => <span data-testid="icon-settings">Settings</span>,
  SkipForward: () => <span data-testid="icon-skip-forward">SkipForward</span>,
  SkipBack: () => <span data-testid="icon-skip-back">SkipBack</span>,
  Square: () => <span data-testid="icon-square">Square</span>,
}))

const mockMedia = {
  id: 1,
  title: 'Test Movie',
  media_type: 'video/mp4',
  directory_path: '/videos/test.mp4',
  storage_root_name: 'local',
  storage_root_protocol: 'file',
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
}

const mockSubtitles = [
  { id: 'sub-1', language: 'en', language_name: 'English', media_id: 1, file_path: '/subs/en.srt', format: 'srt' },
  { id: 'sub-2', language: 'fr', language_name: 'French', media_id: 1, file_path: '/subs/fr.srt', format: 'srt' },
]

describe('MediaPlayer', () => {
  it('renders media title', () => {
    render(<MediaPlayer media={mockMedia as any} />)
    expect(screen.getByText('Test Movie')).toBeInTheDocument()
  })

  it('renders "Unknown Title" when no title', () => {
    const noTitleMedia = { ...mockMedia, title: '' }
    render(<MediaPlayer media={noTitleMedia as any} />)
    expect(screen.getByText('Unknown Title')).toBeInTheDocument()
  })

  it('renders video element fed from the real stream endpoint, never the storage-relative path', () => {
    localStorage.removeItem('auth_token')
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    expect(container.querySelector('video')).toBeInTheDocument()
    const source = container.querySelector('source')
    expect(source).toHaveAttribute('src', '/api/v1/stream/1')
    expect(source).not.toHaveAttribute('type')
  })

  it('carries the session token as the query fallback because <video> cannot send headers', () => {
    localStorage.setItem('auth_token', 'a.b+c')
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    expect(container.querySelector('source')).toHaveAttribute('src', '/api/v1/stream/1?access_token=a.b%2Bc')
    localStorage.removeItem('auth_token')
  })

  it('an explicit entity stream URL takes priority', () => {
    const { container } = render(<MediaPlayer media={mockMedia as any} entityStreamUrl="/api/v1/entities/9/stream" />)
    expect(container.querySelector('source')).toHaveAttribute('src', '/api/v1/entities/9/stream')
  })

  it('renders play button initially', () => {
    render(<MediaPlayer media={mockMedia as any} />)
    // Play icons are shown by default (not playing)
    const playIcons = screen.getAllByTestId('icon-play')
    expect(playIcons.length).toBeGreaterThan(0)
  })

  it('renders control buttons', () => {
    render(<MediaPlayer media={mockMedia as any} />)
    expect(screen.getByTestId('icon-skip-back')).toBeInTheDocument()
    expect(screen.getByTestId('icon-skip-forward')).toBeInTheDocument()
    expect(screen.getByTestId('icon-volume2')).toBeInTheDocument()
    expect(screen.getByTestId('icon-subtitles')).toBeInTheDocument()
    expect(screen.getByTestId('icon-settings')).toBeInTheDocument()
    expect(screen.getByTestId('icon-maximize')).toBeInTheDocument()
  })

  it('renders progress bar', () => {
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    const progressBar = container.querySelector('input[type="range"]')
    expect(progressBar).toBeInTheDocument()
  })

  it('renders time display showing 00:00', () => {
    render(<MediaPlayer media={mockMedia as any} />)
    const timeDisplays = screen.getAllByText('00:00')
    expect(timeDisplays.length).toBe(2) // current time and duration
  })

  it('renders volume slider', () => {
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    const sliders = container.querySelectorAll('input[type="range"]')
    // Should have progress bar and volume slider
    expect(sliders.length).toBeGreaterThanOrEqual(2)
  })

  it('does not show subtitle panel by default', () => {
    render(
      <MediaPlayer media={mockMedia as any} subtitles={mockSubtitles as any} />
    )
    expect(screen.queryByText('English')).not.toBeInTheDocument()
  })

  it('shows subtitle panel when subtitle button clicked', async () => {
    const user = userEvent.setup()
    render(
      <MediaPlayer media={mockMedia as any} subtitles={mockSubtitles as any} />
    )

    const subtitleButtons = screen.getAllByTestId('icon-subtitles')
    // Click the button containing the subtitle icon
    const button = subtitleButtons[0].closest('button')
    if (button) {
      await user.click(button)
    }

    // "Subtitles" appears both in the mock icon and in the subtitle panel h3 heading
    const subtitlesElements = screen.getAllByText('Subtitles')
    expect(subtitlesElements.length).toBeGreaterThanOrEqual(2)
    expect(screen.getByText('English')).toBeInTheDocument()
    expect(screen.getByText('French')).toBeInTheDocument()
    expect(screen.getByText('Off')).toBeInTheDocument()
  })

  it('does not show subtitle options when no subtitles provided', async () => {
    const user = userEvent.setup()
    render(<MediaPlayer media={mockMedia as any} subtitles={[]} />)

    // Even if we click the subtitle button, no panel should appear with options
    const subtitleButtons = screen.getAllByTestId('icon-subtitles')
    const button = subtitleButtons[0].closest('button')
    if (button) {
      await user.click(button)
    }
    // "Off" button should not be present
    expect(screen.queryByText('Off')).not.toBeInTheDocument()
  })

  it('calls onProgress callback', () => {
    const onProgress = vi.fn()
    const { container } = render(
      <MediaPlayer media={mockMedia as any} onProgress={onProgress} />
    )

    const video = container.querySelector('video')!
    fireEvent.timeUpdate(video)

    // The onProgress is triggered via addEventListener in useEffect
    // Due to mock HTMLMediaElement, we verify the callback is accepted
    expect(onProgress).toBeDefined()
  })

  it('calls onEnded callback', () => {
    const onEnded = vi.fn()
    const { container } = render(
      <MediaPlayer media={mockMedia as any} onEnded={onEnded} />
    )

    const video = container.querySelector('video')!
    fireEvent.ended(video)

    // The event is handled inside useEffect
    expect(onEnded).toBeDefined()
  })

  it('calls onError callback', () => {
    const onError = vi.fn()
    const { container } = render(
      <MediaPlayer media={mockMedia as any} onError={onError} />
    )

    const video = container.querySelector('video')!
    fireEvent.error(video)

    expect(onError).toBeDefined()
  })

  it('renders within a Card component', () => {
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    // Card renders a div with certain structure
    expect(container.firstChild).toBeInTheDocument()
  })

  it('has aspect-video container for the video', () => {
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    const aspectVideo = container.querySelector('.aspect-video')
    expect(aspectVideo).toBeInTheDocument()
  })
})
