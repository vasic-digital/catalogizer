import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'
import { HistoryDrawer } from '../HistoryDrawer'
import type { UiPlaybackProgress, UiPlaybackSession } from '@/types/playback'

vi.mock('@/lib/playbackApi', () => ({
  getEntityProgress: vi.fn(),
  listEntityHistory: vi.fn(),
}))

vi.mock('lucide-react', () => ({
  X: ({ className }: { className?: string }) => (
    <svg data-testid="x-icon" className={className} />
  ),
}))

import { getEntityProgress, listEntityHistory } from '@/lib/playbackApi'

const mockGetEntityProgress = vi.mocked(getEntityProgress)
const mockListEntityHistory = vi.mocked(listEntityHistory)

const sampleProgress: UiPlaybackProgress = {
  mediaItemId: 42,
  positionUnit: 'seconds',
  durationTotal: 7200,
  lastPosition: 3600,
  lastSessionAmount: 1800,
  totalReproductions: 5,
  aggregateAmount: 9000,
  lastSessionEndedAt: '2026-04-10T15:30:00Z',
}

const sampleSessions: UiPlaybackSession[] = [
  {
    id: 1,
    positionUnit: 'seconds',
    startPosition: 0,
    endPosition: 1800,
    totalAmount: 1800,
    startedAt: '2026-04-10T14:00:00Z',
    endedAt: '2026-04-10T14:30:00Z',
    completed: true,
  },
  {
    id: 2,
    positionUnit: 'seconds',
    startPosition: 1800,
    endPosition: 3600,
    totalAmount: 1800,
    startedAt: '2026-04-10T15:00:00Z',
    endedAt: null,
    completed: false,
  },
]

const defaultProps = {
  mediaItemId: 42,
  mediaTitle: 'Test Movie',
  open: true,
  onClose: vi.fn(),
}

describe('HistoryDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetEntityProgress.mockResolvedValue(sampleProgress)
    mockListEntityHistory.mockResolvedValue(sampleSessions)
  })

  it('renders nothing when open is false', () => {
    const { container } = render(
      <HistoryDrawer {...defaultProps} open={false} />,
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders the drawer title and media title when open', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    expect(screen.getByText('Reproduction History')).toBeInTheDocument()
    expect(screen.getByText('Test Movie')).toBeInTheDocument()
  })

  it('has correct dialog accessibility attributes', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAttribute(
      'aria-labelledby',
      'history-drawer-title',
    )
  })

  it('renders close button with aria-label', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    const closeBtn = screen.getByTestId('history-drawer-close')
    expect(closeBtn).toHaveAttribute('aria-label', 'Close history drawer')
  })

  it('calls onClose when close button is clicked', async () => {
    const onClose = vi.fn()
    render(<HistoryDrawer {...defaultProps} onClose={onClose} />)
    const user = userEvent.setup()
    await user.click(screen.getByTestId('history-drawer-close'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when clicking the backdrop', async () => {
    const onClose = vi.fn()
    render(<HistoryDrawer {...defaultProps} onClose={onClose} />)
    const user = userEvent.setup()
    await user.click(screen.getByTestId('history-drawer-backdrop'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not call onClose when clicking inside the dialog content', async () => {
    const onClose = vi.fn()
    render(<HistoryDrawer {...defaultProps} onClose={onClose} />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('dialog'))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('calls onClose when Escape key is pressed', async () => {
    const onClose = vi.fn()
    render(<HistoryDrawer {...defaultProps} onClose={onClose} />)
    const user = userEvent.setup()
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('fetches progress and history when opened', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(mockGetEntityProgress).toHaveBeenCalledWith(42)
      expect(mockListEntityHistory).toHaveBeenCalledWith(42, 100)
    })
  })

  it('does not fetch when mediaItemId is null', () => {
    render(<HistoryDrawer {...defaultProps} mediaItemId={null} />)
    expect(mockGetEntityProgress).not.toHaveBeenCalled()
    expect(mockListEntityHistory).not.toHaveBeenCalled()
  })

  it('shows loading text before data arrives', () => {
    mockGetEntityProgress.mockReturnValue(new Promise(() => { /* never resolves */ }))
    mockListEntityHistory.mockReturnValue(new Promise(() => { /* never resolves */ }))
    render(<HistoryDrawer {...defaultProps} />)
    expect(screen.getByText(/Loading/)).toBeInTheDocument()
  })

  it('renders session rows after data loads', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByTestId('history-session-1')).toBeInTheDocument()
      expect(screen.getByTestId('history-session-2')).toBeInTheDocument()
    })
  })

  it('shows completed and in-progress status labels', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByText('Completed')).toBeInTheDocument()
      expect(screen.getByText('In progress')).toBeInTheDocument()
    })
  })

  it('shows "Never reproduced" when progress is null', async () => {
    mockGetEntityProgress.mockResolvedValue(null)
    mockListEntityHistory.mockResolvedValue([])
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByText('Never reproduced')).toBeInTheDocument()
    })
  })

  it('shows empty session message when no sessions exist', async () => {
    mockListEntityHistory.mockResolvedValue([])
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(
        screen.getByText('No sessions yet. Press Play to record your first.'),
      ).toBeInTheDocument()
    })
  })

  it('displays summary rows when progress data is loaded', async () => {
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByText('Total duration')).toBeInTheDocument()
      expect(screen.getByText('Current position')).toBeInTheDocument()
      expect(screen.getByText('Last session')).toBeInTheDocument()
      expect(screen.getByText('Times played')).toBeInTheDocument()
      expect(screen.getByText('Total reproduced')).toBeInTheDocument()
    })
  })

  it('handles API errors gracefully by showing loaded state', async () => {
    mockGetEntityProgress.mockRejectedValue(new Error('Network error'))
    mockListEntityHistory.mockRejectedValue(new Error('Network error'))
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(
        screen.getByText('No sessions yet. Press Play to record your first.'),
      ).toBeInTheDocument()
    })
  })

  it('cancels pending requests on unmount', async () => {
    let resolveProgress: (v: UiPlaybackProgress | null) => void = (_v) => { /* noop default */ }
    mockGetEntityProgress.mockReturnValue(
      new Promise((resolve) => {
        resolveProgress = resolve
      }),
    )
    mockListEntityHistory.mockResolvedValue([])

    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { unmount, container } = render(<HistoryDrawer {...defaultProps} />)
    expect(mockGetEntityProgress).toHaveBeenCalledWith(42)
    unmount()

    resolveProgress(sampleProgress)
    await act(async () => { await Promise.resolve() })
    // The late resolution must not render anything nor trigger a React error
    // (the cancelled flag prevents setState after unmount).
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByText('Total reproduced')).not.toBeInTheDocument()
    expect(errorSpy).not.toHaveBeenCalled()
    errorSpy.mockRestore()

    // A superseded request must not overwrite newer state: while the request for
    // item 42 is pending, switch to item 43 (no sessions), then let 42 resolve late.
    let resolveStale: (v: UiPlaybackSession[]) => void = (_v) => { /* noop default */ }
    mockGetEntityProgress.mockResolvedValue(null)
    mockListEntityHistory.mockReset()
    mockListEntityHistory.mockImplementation((id: number) =>
      id === 42
        ? new Promise((resolve) => { resolveStale = resolve })
        : Promise.resolve([]),
    )
    const view = render(<HistoryDrawer {...defaultProps} />)
    view.rerender(<HistoryDrawer {...defaultProps} mediaItemId={43} />)
    await screen.findByText('No sessions yet. Press Play to record your first.')
    await act(async () => { resolveStale(sampleSessions) })
    expect(screen.getByText('No sessions yet. Press Play to record your first.')).toBeInTheDocument()
  })

  it('re-fetches when mediaItemId changes', async () => {
    const { rerender } = render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(mockGetEntityProgress).toHaveBeenCalledWith(42)
    })

    vi.clearAllMocks()
    mockGetEntityProgress.mockResolvedValue(sampleProgress)
    mockListEntityHistory.mockResolvedValue(sampleSessions)

    rerender(<HistoryDrawer {...defaultProps} mediaItemId={99} />)
    await waitFor(() => {
      expect(mockGetEntityProgress).toHaveBeenCalledWith(99)
      expect(mockListEntityHistory).toHaveBeenCalledWith(99, 100)
    })
  })

  it('formats session timestamps correctly', async () => {
    mockListEntityHistory.mockResolvedValue([
      {
        id: 10,
        positionUnit: 'seconds',
        startPosition: 0,
        endPosition: 600,
        totalAmount: 600,
        startedAt: '2026-01-15T09:05:00Z',
        endedAt: '2026-01-15T09:15:00Z',
        completed: true,
      },
    ])
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByTestId('history-session-10')).toBeInTheDocument()
    })
    // The formatInstant function produces "YYYY-MM-DD HH:MM" format
    const sessionEl = screen.getByTestId('history-session-10')
    expect(sessionEl.textContent).toMatch(/2026-01-15/)
  })

  it('handles sessions with null startedAt gracefully', async () => {
    mockListEntityHistory.mockResolvedValue([
      {
        id: 20,
        positionUnit: 'seconds',
        startPosition: 0,
        endPosition: 300,
        totalAmount: 300,
        startedAt: '',
        endedAt: null,
        completed: false,
      },
    ])
    render(<HistoryDrawer {...defaultProps} />)
    await waitFor(() => {
      const sessionEl = screen.getByTestId('history-session-20')
      expect(sessionEl).toBeInTheDocument()
      // Empty startedAt falls back to '-'
      expect(sessionEl.textContent).toContain('-')
    })
  })

  it('removes Escape keydown listener when closed', async () => {
    const onClose = vi.fn()
    const { rerender } = render(
      <HistoryDrawer {...defaultProps} onClose={onClose} />,
    )
    rerender(<HistoryDrawer {...defaultProps} onClose={onClose} open={false} />)
    const user = userEvent.setup()
    await user.keyboard('{Escape}')
    expect(onClose).not.toHaveBeenCalled()
  })
})
