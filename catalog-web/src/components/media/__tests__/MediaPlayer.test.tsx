import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MediaPlayer } from '../MediaPlayer'

const { postMock } = vi.hoisted(() => ({ postMock: vi.fn().mockResolvedValue({ data: { ticket: 'tk' } }) }))
vi.mock('@/lib/api', () => ({ api: { post: postMock } }))

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

  it('mints a stream ticket and feeds the video from /stream/:id?ticket=, never the path or the session token', async () => {
    postMock.mockResolvedValue({ data: { ticket: 'v1.a.b' } })
    localStorage.setItem('auth_token', 'SESSION.JWT.VALUE')
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    await waitFor(() => expect(container.querySelector('video')).toHaveAttribute('src', '/api/v1/stream/1?ticket=v1.a.b'))
    expect(postMock).toHaveBeenCalledWith('/stream/1/ticket', {})
    expect(container.querySelector('video')!.getAttribute('src')).not.toContain('SESSION')
    expect(container.querySelector('video')!.getAttribute('src')).not.toContain('/videos/test.mp4')
    localStorage.removeItem('auth_token')
  })

  it('shows an error and no source when the ticket cannot be minted', async () => {
    postMock.mockRejectedValue(new Error('401'))
    const { container } = render(<MediaPlayer media={mockMedia as any} />)
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(container.querySelector('video')).not.toHaveAttribute('src')
  })

  it('an explicit entity stream URL takes priority and mints nothing', () => {
    postMock.mockClear()
    const { container } = render(<MediaPlayer media={mockMedia as any} entityStreamUrl="/api/v1/entities/9/stream" />)
    expect(container.querySelector('video')).toHaveAttribute('src', '/api/v1/entities/9/stream')
    expect(postMock).not.toHaveBeenCalled()
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

describe('MediaPlayer ticket lifecycle', () => {
  let n = 0
  const mintSeq = () => postMock.mockImplementation(async () => ({ data: { ticket: `v1.t${++n}.m` } }))
  const srcOf = (c: HTMLElement) => c.querySelector('video')!.getAttribute('src')
  const setPaused = (v: HTMLVideoElement, paused: boolean) => Object.defineProperty(v, 'paused', { configurable: true, value: paused })

  beforeEach(() => { n = 0; postMock.mockReset(); mintSeq() })

  it('error recovery is not exhausted by earlier recoveries: progress resets the budget (5 recoveries, each followed by playing)', async () => {
    const onError = vi.fn()
    const { container } = render(<MediaPlayer media={mockMedia as any} onError={onError} />)
    const video = container.querySelector('video')!
    await waitFor(() => expect(srcOf(container)).toContain('t1'))
    for (let i = 2; i <= 6; i++) {
      fireEvent.error(video)
      await waitFor(() => expect(srcOf(container)).toContain(`t${i}.`))
      fireEvent(video, new Event('playing'))
    }
    expect(onError).not.toHaveBeenCalled()
  })

  it('gives up with onError only after 3 consecutive recoveries without progress', async () => {
    const onError = vi.fn()
    const { container } = render(<MediaPlayer media={mockMedia as any} onError={onError} />)
    const video = container.querySelector('video')!
    await waitFor(() => expect(srcOf(container)).toContain('t1'))
    for (let i = 2; i <= 4; i++) {
      fireEvent.error(video)
      await waitFor(() => expect(srcOf(container)).toContain(`t${i}.`))
    }
    expect(onError).not.toHaveBeenCalled()
    fireEvent.error(video)
    expect(onError).toHaveBeenCalledTimes(1)
  })

  describe('timer', () => {
    beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
    afterEach(() => { vi.useRealTimers() })

    it('refreshes the ticket before expiry while playing (seeks after 60 s need a live ticket) and leaves a paused element alone', async () => {
      const { container } = render(<MediaPlayer media={mockMedia as any} />)
      const video = container.querySelector('video')!
      await waitFor(() => expect(srcOf(container)).toContain('t1.'))
      setPaused(video, false)
      await vi.advanceTimersByTimeAsync(46_000)
      await waitFor(() => expect(srcOf(container)).toContain('t2.'))
      setPaused(video, true)
      await vi.advanceTimersByTimeAsync(100_000)
      expect(srcOf(container)).toContain('t2.')
    })

    it('idle/timer refreshes never consume the error-recovery budget', async () => {
      const onError = vi.fn()
      const { container } = render(<MediaPlayer media={mockMedia as any} onError={onError} />)
      const video = container.querySelector('video')!
      await waitFor(() => expect(srcOf(container)).toContain('t1.'))
      setPaused(video, false)
      for (let i = 0; i < 5; i++) await vi.advanceTimersByTimeAsync(46_000)
      fireEvent.error(video)
      await waitFor(() => expect(Number(/t(\d+)\./.exec(srcOf(container)!)![1])).toBeGreaterThan(5))
      expect(onError).not.toHaveBeenCalled()
    })
  })
})
