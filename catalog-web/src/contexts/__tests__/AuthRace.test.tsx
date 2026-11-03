import React from 'react'
import { render, screen, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from '../AuthContext'
import { authApi } from '@/lib/api'

vi.mock('@/lib/api', async () => ({
  authApi: {
    getAuthStatus: vi.fn(),
    getPermissions: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    updateProfile: vi.fn(),
    changePassword: vi.fn(),
  },
}))
vi.mock('react-hot-toast', async () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))

const m = vi.mocked(authApi)
const user = { id: 1, username: 'u', email: 'e', role_id: 2, is_active: true, created_at: '', updated_at: '' } as any
const renders: Array<{ loading: boolean; auth: boolean }> = []
let logoutFn: (() => Promise<void>) | null = null

function Probe() {
  const a = useAuth()
  renders.push({ loading: a.isLoading, auth: a.isAuthenticated })
  logoutFn = a.logout
  return <div data-testid="s">{a.isLoading ? 'loading' : a.isAuthenticated ? 'in' : 'out'}</div>
}

function mount() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={qc}><AuthProvider><Probe /></AuthProvider></QueryClientProvider>)
}

describe("AuthContext isLoading race (ProtectedRoute must not see loading=false with user=null)", () => {
  beforeEach(() => { renders.length = 0; vi.clearAllMocks() })

  it('never renders loading=false && unauthenticated for a valid session', async () => {
    m.getAuthStatus.mockResolvedValue({ authenticated: true, user, permissions: [] } as any)
    m.getPermissions.mockResolvedValue({ permissions: [] } as any)
    mount()
    await waitFor(() => expect(screen.getByTestId('s').textContent).toBe('in'))
    const bad = renders.filter(r => !r.loading && !r.auth)
    expect(bad).toEqual([])
  })

  it('logout does not hang in loading', async () => {
    m.getAuthStatus.mockResolvedValue({ authenticated: true, user, permissions: [] } as any)
    m.getPermissions.mockResolvedValue({ permissions: [] } as any)
    m.logout.mockResolvedValue(undefined as any)
    mount()
    await waitFor(() => expect(screen.getByTestId('s').textContent).toBe('in'))
    m.getAuthStatus.mockResolvedValue({ authenticated: false } as any)
    await act(async () => { await logoutFn!() })
    await waitFor(() => expect(screen.getByTestId('s').textContent).toBe('out'), { timeout: 3000 })
  })

  it('unauthenticated status resolves to out, not loading', async () => {
    m.getAuthStatus.mockResolvedValue({ authenticated: false } as any)
    mount()
    await waitFor(() => expect(screen.getByTestId('s').textContent).toBe('out'))
  })
})
