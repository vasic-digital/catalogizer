import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from '../AuthContext'
import { authApi } from '@/lib/api'

// SSEF4R-M1/N1: isAdmin follows the server rule (userRepo.IsAdmin ->
// Permissions.HasPermission("system.admin")) evaluated on the /auth/status
// permissions, never on role_id or the role name.
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

const mockAuthApi = vi.mocked(authApi)

function Probe() {
  const { isAdmin, user, isLoading } = useAuth()
  if (isLoading) return <div>Loading...</div>
  return (
    <div>
      <div data-testid="user">{user ? user.username : 'none'}</div>
      <div data-testid="admin">{isAdmin ? 'admin' : 'not-admin'}</div>
    </div>
  )
}

async function adminFor(user: Record<string, unknown> | null, permissions: string[], permissionsEndpoint?: string[]) {
  mockAuthApi.getAuthStatus.mockResolvedValue({ authenticated: !!user, user, permissions } as never)
  mockAuthApi.getPermissions.mockResolvedValue({ permissions: permissionsEndpoint ?? permissions } as never)
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <AuthProvider>
        <Probe />
      </AuthProvider>
    </QueryClientProvider>
  )
  await waitFor(() => expect(screen.getByTestId('user')).toBeInTheDocument())
  if (user) await waitFor(() => expect(screen.getByTestId('user')).toHaveTextContent(String(user.username)))
  return screen.getByTestId('admin').textContent
}

describe('AuthContext isAdmin follows the server rule', () => {
  beforeEach(() => vi.clearAllMocks())

  it.each([
    [['system.admin']],
    [['*']],
    [['system.*']],
    [['read:media', 'system.admin']],
  ])('custom admin role (role_id 7, name Ops) with %j is admin', async (perms) => {
    expect(await adminFor({ id: 1, username: 'ops', role_id: 7, role: { id: 7, name: 'Ops' } }, perms)).toBe('admin')
  })

  it('role_id 1 that lost its admin permissions is NOT admin', async () => {
    expect(await adminFor({ id: 1, username: 'a', role_id: 1, role: { id: 1, name: 'Admin' } }, ['read:media'])).toBe('not-admin')
  })

  it('role name Admin with no permissions is NOT admin (no role-name fallback)', async () => {
    expect(await adminFor({ id: 1, username: 'a', role: { id: 5, name: 'Admin' } }, [])).toBe('not-admin')
  })

  it.each([
    [['sys*']],
    [['media.*']],
    [['admin:system']],
    [['system.administrator']],
    [['system.']],
  ])('non-matching %j is NOT admin (server segment-boundary matching)', async (perms) => {
    expect(await adminFor({ id: 1, username: 'x', role_id: 1 }, perms)).toBe('not-admin')
  })

  it('an unauthenticated session is not admin', async () => {
    expect(await adminFor(null, ['*'])).toBe('not-admin')
  })

  it('uses the /auth/status permissions, not a differing /auth/permissions list', async () => {
    expect(await adminFor({ id: 1, username: 'x', role_id: 1 }, ['system.admin'], ['read:media'])).toBe('admin')
    vi.clearAllMocks()
  })
})
