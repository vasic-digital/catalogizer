import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '../ProtectedRoute'
import { authApi } from '@/lib/api'

// SSEF4R-FD-F2: the admin gate used by /identities (App.tsx: <ProtectedRoute requireAdmin>),
// real AuthProvider + real ProtectedRoute, only the HTTP edge replaced (unit-class evidence).
vi.mock('@/lib/api', async () => ({
  authApi: { getAuthStatus: vi.fn(), getPermissions: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn(), updateProfile: vi.fn(), changePassword: vi.fn() },
}))
vi.mock('react-hot-toast', async () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('react-router-dom', async () => ({
  ...(await vi.importActual('react-router-dom')),
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate-to">{to}</div>,
}))
const api = vi.mocked(authApi)
const user = { id: 5, username: 'u', email: 'o@x', first_name: 'O', last_name: 'P', role_id: 2, role: { id: 2, name: 'User' }, is_active: true }

const gate = async (perms: string[]) => {
  api.getAuthStatus.mockResolvedValue({ authenticated: true, user, permissions: perms } as never)
  api.getPermissions.mockResolvedValue({ permissions: perms } as never)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AuthProvider>
          <ProtectedRoute requireAdmin><div>Identity Page</div></ProtectedRoute>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('requireAdmin gate (/identities, SSEF4R-FD-F2)', () => {
  it('media.view holder is redirected to /dashboard', async () => {
    await gate(['media.view'])
    expect((await screen.findByTestId('navigate-to')).textContent).toBe('/dashboard')
    expect(screen.queryByText('Identity Page')).not.toBeInTheDocument()
  })
  it('system admin (*) sees the page', async () => {
    await gate(['*'])
    expect(await screen.findByText('Identity Page')).toBeInTheDocument()
  })
})
