import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '@/contexts/ThemeContext'
import { AuthProvider } from '@/contexts/AuthContext'
import { Header } from '../Header'
import { authApi } from '@/lib/api'

// SSEF4R-FD-F3: nav entries follow the route gate. REAL AuthProvider + REAL Header;
// only the HTTP edge is replaced (unit-class evidence).
vi.mock('@/lib/api', async () => ({
  authApi: { getAuthStatus: vi.fn(), getPermissions: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn(), updateProfile: vi.fn(), changePassword: vi.fn() },
}))
vi.mock('react-hot-toast', async () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/components/ui/ConnectionStatus', async () => ({ ConnectionStatus: () => <div /> }))
const api = vi.mocked(authApi)
const user = { id: 2, username: 'bob', email: 'b@x', first_name: 'B', last_name: 'O', role_id: 2, role: { id: 2, name: 'User' }, is_active: true }

async function mount(perms: string[]) {
  api.getAuthStatus.mockResolvedValue({ authenticated: true, user, permissions: perms } as never)
  api.getPermissions.mockResolvedValue({ permissions: perms } as never)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <ThemeProvider>
        <MemoryRouter>
          <AuthProvider>
            <Header />
          </AuthProvider>
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>
  )
  await screen.findAllByText('Dashboard')
  await waitFor(() => {
    expect(qc.getQueryState(['auth-status'])?.status).toBe('success')
    expect(qc.getQueryState(['permissions'])?.status).toBe('success')
  })
}

describe('Header nav gating with the real AuthProvider (SSEF4R-FD-F2/F3)', () => {
  it('default User role (media.view, media.download): no Convert, no Identities, no Admin', async () => {
    await mount(['media.view', 'media.download'])
    expect(screen.getAllByText('Media').length).toBeGreaterThan(0) // control: the nav renders
    expect(screen.queryByText('Convert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Identit/)).not.toBeInTheDocument()
    expect(screen.queryByText('Admin')).not.toBeInTheDocument()
  })
  it('conversion.view shows Convert', async () => {
    await mount(['media.view', 'conversion.view'])
    expect(screen.getAllByText('Convert').length).toBeGreaterThan(0)
  })
  it('media.convert (the old, server-unused permission) does not show Convert', async () => {
    await mount(['media.view', 'media.convert'])
    expect(screen.queryByText('Convert')).not.toBeInTheDocument()
  })
})
