import React from 'react'
import { render, screen, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WebSocketProvider } from '../WebSocketContext'
import { ConnectionStatus } from '@/components/ui/ConnectionStatus'

// Unit-class fake of the transport library ONLY (its constructor ignores its args: the ticket-in-URL check lives in
// src/lib/__tests__/wsTicketClient.test.ts): the real useWebSocket hook,
// provider and ConnectionStatus run together. Live proof: e2e/real-backend-pages.spec.ts.
const states = { value: 'disconnected' as string, connects: 0 }
vi.mock('@vasic-digital/websocket-client', () => ({
  WebSocketClient: class {
    on() { return undefined }
    sendJSON() { return undefined }
    dispose() { return undefined }
    connect() { states.connects += 1; states.value = 'connected' }
    getState() { return states.value }
  },
}))
// SEC-WS-1 F2: the provider's client mints a one-shot ticket (POST /ws/ticket) before it opens the base client; without
// this mock the real api.post is called, fails, and the base connect() is never reached (connects stays 0).
const post = vi.fn()
vi.mock('@/lib/api', () => ({ api: { post: (...a: any[]) => post(...a) } }))
vi.mock('@/contexts/AuthContext', () => ({
  useAuth: () => ({ isAuthenticated: true, user: { id: 1 } }),
}))
vi.mock('framer-motion', () => ({
  motion: { div: ({ children, ...p }: any) => <div {...p}>{children}</div> },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

const renderProvider = () => render(
  <QueryClientProvider client={new QueryClient()}>
    <WebSocketProvider>
      <ConnectionStatus />
    </WebSocketProvider>
  </QueryClientProvider>
)

describe('ConnectionStatus with the real WebSocketProvider', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    states.value = 'disconnected'
    states.connects = 0
    post.mockReset()
    post.mockResolvedValue({ data: { ticket: 'tk-1' } })
  })
  afterEach(() => {
    vi.useRealTimers()
    localStorage.removeItem('auth_token')
  })

  it('shows no "Disconnected" banner once the provider has opened the socket', async () => {
    // useWebSocket().connect() returns early without a stored session token (AuthContext stores it on login)
    localStorage.setItem('auth_token', 'test-session-token')
    renderProvider()
    await act(async () => { vi.advanceTimersByTime(2500) })
    expect(post).toHaveBeenCalledWith('/ws/ticket', {})
    expect(states.connects).toBe(1)
    expect(screen.queryByText(/disconnected/i)).not.toBeInTheDocument()
    // the mint resolves in the last flush, so one more poll tick is needed before the banner can reflect 'open'
    await act(async () => { vi.advanceTimersByTime(1000) })
    expect(screen.queryByTestId('connection-status')).not.toBeInTheDocument()
  })

  it('without a stored auth_token nothing mints a ticket and nothing connects', async () => {
    renderProvider()
    await act(async () => { vi.advanceTimersByTime(2500) })
    expect(post).not.toHaveBeenCalled()
    expect(states.connects).toBe(0)
  })
})
