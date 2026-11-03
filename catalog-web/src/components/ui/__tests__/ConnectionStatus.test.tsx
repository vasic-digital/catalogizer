import { render, screen, waitFor } from '@testing-library/react'
import { ConnectionStatus } from '../ConnectionStatus'
import { useWebSocketContext as useWebSocket } from '@/contexts/WebSocketContext'
import { useAuth } from '@/contexts/AuthContext'

// Mock the websocket hook
vi.mock('@/contexts/WebSocketContext', async () => ({
  useWebSocketContext: vi.fn(),
}))

// Article XI §11.5: ConnectionStatus is hidden pre-auth (added
// 2026-04-29 alongside the silent-rejection wrong-password fix
// in catalog-api). All existing tests in this file were written
// before that gate and expect the component to render based
// purely on websocket state. Default the auth mock to
// authenticated so the existing assertions hold; the gate itself
// is verified by the e2e/auth.spec.ts permanent regression and
// by an explicit "hidden when unauthenticated" subtest below.
vi.mock('@/contexts/AuthContext', async () => ({
  useAuth: vi.fn(),
}))

// Mock framer-motion to avoid animation issues in tests
vi.mock('framer-motion', async () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

const mockUseWebSocket = vi.mocked(useWebSocket)
const mockUseAuth = vi.mocked(useAuth)

describe('ConnectionStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    // Default to authenticated for the bulk of legacy assertions;
    // individual tests can override.
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: null,
      login: vi.fn(),
      logout: vi.fn(),
      register: vi.fn(),
      loading: false,
    } as any)
  })

  afterEach(() => {
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  describe('Connection States', () => {
    it('displays connecting status when connection state is connecting', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('connecting'),
      })

      render(<ConnectionStatus />)

      expect(screen.getByText('Connecting...')).toBeInTheDocument()
      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('does not display status when connection state is open', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('open'),
      })

      render(<ConnectionStatus />)

      expect(screen.queryByText('Connected')).not.toBeInTheDocument()
      expect(screen.queryByText('Connecting...')).not.toBeInTheDocument()
      expect(screen.queryByText('Disconnected')).not.toBeInTheDocument()
    })

    it('displays disconnecting status when connection state is closing', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('closing'),
      })

      render(<ConnectionStatus />)

      expect(screen.getByText('Disconnecting...')).toBeInTheDocument()
    })

    it('displays disconnected status when connection state is closed', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('closed'),
      })

      render(<ConnectionStatus />)

      expect(screen.getByText('Disconnected')).toBeInTheDocument()
    })
  })

  describe('Status Colors', () => {
    it('applies yellow background for connecting state', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('connecting'),
      })

      render(<ConnectionStatus />)

      const statusElement = screen.getByText('Connecting...').parentElement
      expect(statusElement).toHaveClass('bg-yellow-500')
    })

    it('applies red background for disconnected state', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('closed'),
      })

      render(<ConnectionStatus />)

      const statusElement = screen.getByText('Disconnected').parentElement
      expect(statusElement).toHaveClass('bg-red-500')
    })

    it('applies orange background for disconnecting state', () => {
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('closing'),
      })

      render(<ConnectionStatus />)

      const statusElement = screen.getByText('Disconnecting...').parentElement
      expect(statusElement).toHaveClass('bg-orange-500')
    })
  })

  describe('Dynamic State Changes', () => {
    it('updates status when connection state changes', async () => {
      const getConnectionState = vi.fn().mockReturnValue('connecting')
      mockUseWebSocket.mockReturnValue({
        getConnectionState,
      })

      const { rerender: _rerender } = render(<ConnectionStatus />)

      expect(screen.getByText('Connecting...')).toBeInTheDocument()

      // Change state to closed
      getConnectionState.mockReturnValue('closed')

      // Fast-forward time to trigger the interval
      vi.advanceTimersByTime(1000)

      // Wait for the state update
      await waitFor(() => {
        expect(screen.getByText('Disconnected')).toBeInTheDocument()
      })
    })

    it('hides status when connection becomes open', async () => {
      const getConnectionState = vi.fn().mockReturnValue('connecting')
      mockUseWebSocket.mockReturnValue({
        getConnectionState,
      })

      render(<ConnectionStatus />)

      expect(screen.getByText('Connecting...')).toBeInTheDocument()

      // Change state to open
      getConnectionState.mockReturnValue('open')

      // Fast-forward time to trigger the interval
      vi.advanceTimersByTime(1000)

      // Wait for the status to be hidden
      await waitFor(() => {
        expect(screen.queryByText('Connected')).not.toBeInTheDocument()
      })
    })
  })

  describe('Interval Updates', () => {
    it('checks connection state every second', () => {
      const getConnectionState = vi.fn().mockReturnValue('closed')
      mockUseWebSocket.mockReturnValue({
        getConnectionState,
      })

      render(<ConnectionStatus />)

      // Initial call
      expect(getConnectionState).toHaveBeenCalledTimes(1)

      // After 1 second
      vi.advanceTimersByTime(1000)
      expect(getConnectionState).toHaveBeenCalledTimes(2)

      // After 3 more seconds
      vi.advanceTimersByTime(3000)
      expect(getConnectionState).toHaveBeenCalledTimes(5)
    })

    it('cleans up interval on unmount', () => {
      const getConnectionState = vi.fn().mockReturnValue('closed')
      mockUseWebSocket.mockReturnValue({
        getConnectionState,
      })

      const { unmount } = render(<ConnectionStatus />)

      expect(getConnectionState).toHaveBeenCalledTimes(1)

      unmount()

      // Advance time after unmount
      vi.advanceTimersByTime(5000)

      // Should not call again after unmount
      expect(getConnectionState).toHaveBeenCalledTimes(1)
    })
  })

  describe('Visibility Logic', () => {
    it('shows status only when not connected', () => {
      const testCases = [
        { state: 'connecting', shouldShow: true },
        { state: 'open', shouldShow: false },
        { state: 'closing', shouldShow: true },
        { state: 'closed', shouldShow: true },
      ]

      testCases.forEach(({ state, shouldShow }) => {
        mockUseWebSocket.mockReturnValue({
          getConnectionState: vi.fn().mockReturnValue(state),
        })

        const { unmount } = render(<ConnectionStatus />)

        const statusTexts = ['Connecting...', 'Connected', 'Disconnecting...', 'Disconnected']
        const hasAnyStatus = statusTexts.some((text) => screen.queryByText(text) !== null)

        expect(hasAnyStatus).toBe(shouldShow)

        unmount()
      })
    })
  })

  describe('Auth Gate (Article XI §11.5)', () => {
    // Article XI §11.2.5 anti-bluff anchor: removing the
    // `if (!isAuthenticated) return null` early-return in
    // ConnectionStatus.tsx makes both these tests fail. The
    // regression here pairs with the e2e/auth.spec.ts permanent
    // regression that exercises the same gate against a real
    // browser at runtime.
    it('renders nothing when user is not authenticated, even with a connecting websocket', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        register: vi.fn(),
        loading: false,
      } as any)
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('connecting'),
      })

      const { container } = render(<ConnectionStatus />)
      expect(screen.queryByText('Connecting...')).not.toBeInTheDocument()
      expect(screen.queryByText('Disconnected')).not.toBeInTheDocument()
      expect(container.firstChild).toBeNull()
    })

    it('renders nothing when user is not authenticated, even with a closed websocket', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        register: vi.fn(),
        loading: false,
      } as any)
      mockUseWebSocket.mockReturnValue({
        getConnectionState: vi.fn().mockReturnValue('closed'),
      })

      const { container } = render(<ConnectionStatus />)
      expect(screen.queryByText('Disconnected')).not.toBeInTheDocument()
      expect(container.firstChild).toBeNull()
    })
  })
})

// Responsive contract (11.4.190): a top-right fixed badge covers the page title on a phone. On small screens it sits
// at the bottom edge inside the gutters; from `sm` up it keeps the top-right position.
describe('ConnectionStatus - placement', () => {
  it('is bottom-anchored on phones and top-right from sm up, inside the viewport gutters', () => {
    vi.useFakeTimers()
    mockUseAuth.mockReturnValue({ isAuthenticated: true, user: null } as any)
    mockUseWebSocket.mockReturnValue({ getConnectionState: vi.fn().mockReturnValue('closed') })
    render(<ConnectionStatus />)
    const wrapper = screen.getByText('Disconnected').closest('.fixed') as HTMLElement
    // bottom-10 (2.5rem) clears the fixed version chip in Layout (bottom-2, ~1rem tall) so the two never overlap on a phone
    expect(wrapper.className).toMatch(/(^|\s)bottom-10(\s|$)/)
    expect(wrapper.className).not.toMatch(/(^|\s)bottom-4(\s|$)/)
    expect(wrapper.className).toContain('left-4')
    expect(wrapper.className).toContain('right-4')
    expect(wrapper.className).toContain('sm:top-20')
    expect(wrapper.className).not.toMatch(/(^|\s)top-20(\s|$)/)
    vi.useRealTimers()
  })
})
