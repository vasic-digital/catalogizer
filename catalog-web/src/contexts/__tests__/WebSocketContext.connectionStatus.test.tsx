import React from 'react'
import { render, screen, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WebSocketProvider } from '../WebSocketContext'
import { ConnectionStatus } from '@/components/ui/ConnectionStatus'

// Unit-class fake of the transport library ONLY: the real useWebSocket hook,
// provider and ConnectionStatus run together. Live proof: e2e/real-backend-pages.spec.ts.
const states = { value: 'disconnected' as string, connects: 0 }
vi.mock('@vasic-digital/websocket-client', () => ({
  WebSocketClient: class {
    on() {}
    sendJSON() {}
    dispose() {}
    connect() { states.connects += 1; states.value = 'connected' }
    getState() { return states.value }
  },
}))
vi.mock('@/contexts/AuthContext', () => ({
  useAuth: () => ({ isAuthenticated: true, user: { id: 1 } }),
}))
vi.mock('framer-motion', () => ({
  motion: { div: ({ children, ...p }: any) => <div {...p}>{children}</div> },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

describe('ConnectionStatus with the real WebSocketProvider', () => {
  it('shows no "Disconnected" banner once the provider has opened the socket', async () => {
    vi.useFakeTimers()
    states.value = 'disconnected'
    states.connects = 0
    render(
      <QueryClientProvider client={new QueryClient()}>
        <WebSocketProvider>
          <ConnectionStatus />
        </WebSocketProvider>
      </QueryClientProvider>
    )
    await act(async () => { vi.advanceTimersByTime(2500) })
    expect(states.connects).toBe(1)
    expect(screen.queryByText(/disconnected/i)).not.toBeInTheDocument()
    vi.useRealTimers()
  })
})
