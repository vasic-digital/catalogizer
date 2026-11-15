import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider } from '@/contexts/AuthContext'
import { ProtectedRoute } from '../ProtectedRoute'
import { authApi } from '@/lib/api'

// SSEF4R-FD: REAL AuthProvider + REAL ProtectedRoute; only the HTTP layer is
// replaced (unit-class evidence). The permissions come in the server vocabulary
// (catalog-api/models/user.go: "media.view", "media.*").
vi.mock('@/lib/api', async () => ({
  authApi: { getAuthStatus: vi.fn(), getPermissions: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn(), updateProfile: vi.fn(), changePassword: vi.fn() },
}))
vi.mock('react-hot-toast', async () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('react-router-dom', async () => ({
  ...(await vi.importActual('react-router-dom')),
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate-to">{to}</div>,
}))
const api = vi.mocked(authApi)

const user = { id: 5, username: 'ops', email: 'o@x', first_name: 'O', last_name: 'P', role_id: 9, role: { id: 9, name: 'Curator' }, is_active: true }

const renderGate = async (perms: string[], required: string) => {
  api.getAuthStatus.mockResolvedValue({ authenticated: true, user, permissions: perms } as never)
  api.getPermissions.mockResolvedValue({ permissions: perms } as never)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AuthProvider>
          <ProtectedRoute requiredPermission={required}><div>Protected Content</div></ProtectedRoute>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('route gate vocabulary matches the server permission vocabulary', () => {
  it('admits a non-admin custom role holding media.view on the /media route gate (read:media)', async () => {
    await renderGate(['media.view'], 'read:media')
    expect(await screen.findByText('Protected Content')).toBeInTheDocument()
  })
  it('admits a custom role holding the media.* wildcard on the /media route gate', async () => {
    await renderGate(['media.*'], 'read:media')
    expect(await screen.findByText('Protected Content')).toBeInTheDocument()
  })
  it('still refuses a role without any media permission', async () => {
    await renderGate(['share.view'], 'read:media')
    expect(await screen.findByTestId('navigate-to')).toHaveTextContent('/dashboard')
  })
})
