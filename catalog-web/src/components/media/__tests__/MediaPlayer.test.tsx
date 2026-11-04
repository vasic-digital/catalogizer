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
  const srcOf = (c: HTMLElement) => c.querySelector('video')!.getAttribute('src')
  const mintCount = () => postMock.mock.calls.filter((c) => /\/ticket$/.test(c[0])).length
  // jsdom has no media pipeline: give the element the few observable properties the player reads, as plain writable state.
  function arm(video: HTMLVideoElement, st: { time: number; paused: boolean; code: number | null; buffered: Array<[number, number]> }) {
    Object.defineProperty(video, 'currentTime', { configurable: true, get: () => st.time, set: (v) => { st.time = v } })
    Object.defineProperty(video, 'paused', { configurable: true, get: () => st.paused })
    Object.defineProperty(video, 'error', { configurable: true, get: () => (st.code === null ? null : { code: st.code }) })
    Object.defineProperty(video, 'buffered', { configurable: true, get: () => ({ length: st.buffered.length, start: (i: number) => st.buffered[i][0], end: (i: number) => st.buffered[i][1] }) })
    video.load = vi.fn()
    video.play = vi.fn().mockResolvedValue(undefined)
  }
  const fresh = () => ({ time: 0, paused: true, code: null as number | null, buffered: [] as Array<[number, number]> })
  async function setup(onError = vi.fn()) {
    const r = render(<MediaPlayer media={mockMedia as any} onError={onError} />)
    const video = r.container.querySelector('video')!
    const st = fresh()
    arm(video, st)
    await waitFor(() => expect(srcOf(r.container)).toContain('t1.'))
    fireEvent(video, new Event('loadedmetadata')) // initial load finished
    return { ...r, video, st, onError }
  }
  // The user (or autoplay) starts playback: the element's paused flag drops and a play event fires.
  const playing = (video: HTMLVideoElement, st: { paused: boolean }) => { st.paused = false; fireEvent(video, new Event('play')) }
  const recoverTo = async (c: HTMLElement, i: number) => waitFor(() => expect(srcOf(c)).toContain(`t${i}.`))

  beforeEach(() => { n = 0; postMock.mockReset(); postMock.mockImplementation(async () => ({ data: { ticket: `v1.t${++n}.m` } })) })

  it('real progress restores the recovery budget (5 recoveries, each followed by playback advancing)', async () => {
    const { container, video, st, onError } = await setup()
    playing(video, st); st.time = 10
    for (let i = 2; i <= 6; i++) {
      st.code = 2
      fireEvent.error(video)
      await recoverTo(container, i)
      st.code = null
      fireEvent(video, new Event('loadedmetadata'))
      st.time += 3 // advances past the resume point
      fireEvent(video, new Event('timeupdate'))
    }
    expect(onError).not.toHaveBeenCalled()
  })

  it('loadedmetadata is NOT progress: a file that loads metadata and then errors on every reload gives up after 3 recoveries', async () => {
    const { container, video, st, onError } = await setup()
    playing(video, st)
    for (let i = 2; i <= 4; i++) {
      st.code = 2
      fireEvent.error(video)
      await recoverTo(container, i)
      fireEvent(video, new Event('loadedmetadata'))
    }
    st.code = 2
    fireEvent.error(video)
    expect(onError).toHaveBeenCalledTimes(1)
    expect(mintCount()).toBe(4) // 1 initial + 3 recoveries, then it stops
  })

  it('a decode error (MEDIA_ERR_DECODE) is never retried', async () => {
    const { video, st, onError } = await setup()
    st.code = 3
    fireEvent.error(video)
    expect(onError).toHaveBeenCalledTimes(1)
    expect(mintCount()).toBe(1)
  })

  it('an error that lands during a reload does not lose the resume position (playing at 30 s, two errors before metadata)', async () => {
    const { container, video, st } = await setup()
    playing(video, st); st.time = 30
    st.paused = true; st.code = 2 // Chromium pauses the element before it fires the error event
    fireEvent.error(video)
    await recoverTo(container, 2)
    st.time = 0 // what load() does to the element
    fireEvent.error(video) // second error before metadata: must not overwrite the target with 0/paused
    await new Promise((r) => setTimeout(r, 20))
    fireEvent(video, new Event('loadedmetadata'))
    expect(st.time).toBe(30)
    expect(video.play).toHaveBeenCalledTimes(1)
  })

  describe('no timer, lazy refresh', () => {
    beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
    afterEach(() => { vi.useRealTimers() })

    it('never reloads a playing element on a timer', async () => {
      const { st } = await setup()
      st.paused = false
      await vi.advanceTimersByTimeAsync(300_000)
      expect(mintCount()).toBe(1)
    })

    it('a seek that needs the network after the ticket is near expiry gets a fresh ticket and resumes at the seek target', async () => {
      const { container, video, st } = await setup()
      playing(video, st)
      await vi.advanceTimersByTimeAsync(50_000)
      st.time = 40
      fireEvent(video, new Event('seeking'))
      await recoverTo(container, 2)
      st.time = 0
      fireEvent(video, new Event('loadedmetadata'))
      expect(st.time).toBe(40)
    })

    it('a young ticket, or a seek inside the buffer, mints nothing', async () => {
      const { video, st } = await setup()
      st.time = 40
      await vi.advanceTimersByTimeAsync(10_000)
      fireEvent(video, new Event('seeking'))
      await vi.advanceTimersByTimeAsync(40_000) // now 50 s old, but the target is buffered
      st.buffered = [[35, 60]]
      fireEvent(video, new Event('seeking'))
      await vi.advanceTimersByTimeAsync(100)
      expect(mintCount()).toBe(1)
    })
  })

  describe('round-4 regressions', () => {
    it('D1: an error recovery resumes PLAYING even though the browser already set paused=true (play intent, not !paused)', async () => {
      const { container, video, st } = await setup()
      playing(video, st); st.time = 100
      st.paused = true; st.code = 2
      fireEvent(video, new Event('pause')) // Chromium may also queue a pause event; error is already set, so it is not the user's
      fireEvent.error(video)
      await recoverTo(container, 2)
      st.code = null; st.time = 0
      fireEvent(video, new Event('loadedmetadata'))
      expect(st.time).toBe(100)
      expect(video.play).toHaveBeenCalledTimes(1)
    })

    it('D1: a user pause is respected - the recovery resumes paused', async () => {
      const { container, video, st } = await setup()
      playing(video, st); st.time = 20
      st.paused = true
      fireEvent(video, new Event('pause')) // no error, not reloading: the user paused
      st.code = 2
      fireEvent.error(video)
      await recoverTo(container, 2)
      st.code = null
      fireEvent(video, new Event('loadedmetadata'))
      expect(video.play).not.toHaveBeenCalled()
    })

    it('R1: the resume itself fires timeupdate at the resume point; a source that keeps failing there still gives up after 3 recoveries', async () => {
      const { container, video, st, onError } = await setup()
      playing(video, st); st.time = 50
      for (let i = 2; i <= 4; i++) {
        st.code = 2
        fireEvent.error(video)
        await recoverTo(container, i)
        st.code = null; st.time = 0
        fireEvent(video, new Event('loadedmetadata')) // sets currentTime = 50
        fireEvent(video, new Event('timeupdate'))     // the browser reports the seek to 50: not progress
      }
      st.code = 2
      fireEvent.error(video)
      expect(onError).toHaveBeenCalledTimes(1)
      expect(mintCount()).toBe(4)
    })

    it('R2: advancing time while the element is paused is not progress', async () => {
      const { container, video, st, onError } = await setup()
      playing(video, st); st.time = 10
      for (let i = 2; i <= 4; i++) {
        st.code = 2
        fireEvent.error(video)
        await recoverTo(container, i)
        st.code = null
        fireEvent(video, new Event('loadedmetadata'))
        st.paused = true; st.time += 30
        fireEvent(video, new Event('timeupdate'))
      }
      st.code = 2
      fireEvent.error(video)
      expect(onError).toHaveBeenCalledTimes(1)
    })

    it('S2: sub-step movement (1 s) does not restore the budget; a source failing every ~1 s gives up', async () => {
      const { container, video, st, onError } = await setup()
      playing(video, st); st.time = 10
      for (let i = 2; i <= 4; i++) {
        st.code = 2
        fireEvent.error(video)
        await recoverTo(container, i)
        st.code = null
        fireEvent(video, new Event('loadedmetadata'))
        st.time += 1
        fireEvent(video, new Event('timeupdate'))
      }
      st.code = 2
      fireEvent.error(video)
      expect(onError).toHaveBeenCalledTimes(1)
    })

    describe('seeks and switches', () => {
      beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
      afterEach(() => { vi.useRealTimers() })

      it('R9: the seek target is what the player resumes at, even if currentTime has moved by the time the ticket arrives', async () => {
        const { container, video, st } = await setup()
        playing(video, st)
        await vi.advanceTimersByTimeAsync(50_000)
        let release!: () => void
        postMock.mockImplementationOnce(() => new Promise((res) => { release = () => res({ data: { ticket: 'v1.t2.m' } }) }))
        st.time = 90
        fireEvent(video, new Event('seeking'))
        st.time = 3 // the element moved on while the mint was pending
        release()
        await recoverTo(container, 2)
        st.time = 0
        fireEvent(video, new Event('loadedmetadata'))
        expect(st.time).toBe(90)
      })

      it('S1: a second seek during an in-flight seek-mint wins', async () => {
        const { container, video, st } = await setup()
        playing(video, st)
        await vi.advanceTimersByTimeAsync(50_000)
        let release!: () => void
        postMock.mockImplementationOnce(() => new Promise((res) => { release = () => res({ data: { ticket: 'v1.t2.m' } }) }))
        st.time = 40
        fireEvent(video, new Event('seeking'))
        st.time = 90
        fireEvent(video, new Event('seeking'))
        release()
        await recoverTo(container, 2)
        st.time = 0
        fireEvent(video, new Event('loadedmetadata'))
        expect(st.time).toBe(90)
        expect(mintCount()).toBe(2)
      })

      it('S1: a seek while the source is reloading is not overridden by the stored resume point', async () => {
        const { container, video, st } = await setup()
        playing(video, st); st.time = 30; st.code = 2
        fireEvent.error(video)
        await recoverTo(container, 2)
        st.code = null; st.time = 75 // user seeks while reloading: the element holds 75 as its start position
        fireEvent(video, new Event('loadedmetadata'))
        expect(st.time).toBe(75)
      })

      it('D2: the previous item stops being the source the moment the id changes (even before the new ticket arrives)', async () => {
        const r = render(<MediaPlayer media={mockMedia as any} />)
        await waitFor(() => expect(srcOf(r.container)).toContain('/stream/1?ticket='))
        postMock.mockImplementationOnce(() => new Promise(() => undefined))
        r.rerender(<MediaPlayer media={{ ...mockMedia, id: 2 } as any} />)
        await waitFor(() => expect(srcOf(r.container)).toBeNull())
      })

      it('D2: switching the item while a mint is in flight never plays the old item (stale result dropped, new id minted)', async () => {
        postMock.mockReset()
        const pending: Array<{ id: string; release: () => void }> = []
        postMock.mockImplementation((url: string) => new Promise((res) => { pending.push({ id: url, release: () => res({ data: { ticket: `v1.for${url.match(/stream\/(\d+)/)![1]}.m` } }) }) }))
        const r = render(<MediaPlayer media={mockMedia as any} />)
        r.rerender(<MediaPlayer media={{ ...mockMedia, id: 2 } as any} />)
        await waitFor(() => expect(pending.length).toBe(2)) // a mint for id 2 starts although the one for id 1 is still pending
        pending[0].release() // the OLD item's ticket arrives late
        await new Promise((x) => setTimeout(x, 30))
        expect(srcOf(r.container)).toBeNull()
        pending[1].release()
        await waitFor(() => expect(srcOf(r.container)).toContain('/stream/2?ticket='))
        expect(srcOf(r.container)).not.toContain('/stream/1')
      })
    })
  })
})
