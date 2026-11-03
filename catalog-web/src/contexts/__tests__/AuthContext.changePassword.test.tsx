import React from 'react'
import { render, screen, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from '../AuthContext'
import { authApi } from '@/lib/api'

vi.mock('@/lib/api', async () => ({
  authApi: { getAuthStatus: vi.fn(), getPermissions: vi.fn(), login: vi.fn(), logout: vi.fn(), register: vi.fn(), updateProfile: vi.fn(), changePassword: vi.fn() },
}))
vi.mock('react-hot-toast', () => ({ default: { success: vi.fn(), error: vi.fn() } }))

let ctx: ReturnType<typeof useAuth>
function Probe() { ctx = useAuth(); return <div>{ctx.isLoading ? 'LOADING' : ctx.isAuthenticated ? 'IN' : 'OUT'}</div> }

const authedStatus = { authenticated: true, permissions: ['*'], user: { id: 1, username: 'u', role_id: 1, role: { id: 1, name: 'Admin' } } }

function mount() {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><AuthProvider><Probe /></AuthProvider></QueryClientProvider>)
}

// Server contract: POST /auth/change-password ends ALL of the user's sessions, so
// after success the client must be signed out locally, not left holding a dead token.
it('a successful password change signs the user out locally and clears the stored token', async () => {
  localStorage.setItem('auth_token', 'old-token')
  vi.mocked(authApi.getAuthStatus).mockResolvedValue(authedStatus as never)
  vi.mocked(authApi.getPermissions).mockResolvedValue({ permissions: ['*'] } as never)
  vi.mocked(authApi.changePassword).mockResolvedValue(undefined as never)
  mount()
  await waitFor(() => expect(screen.getByText('IN')).toBeInTheDocument())
  localStorage.setItem('user', '{"id":1}')
  vi.mocked(authApi.getAuthStatus).mockResolvedValue({ authenticated: false } as never)
  await act(async () => { await ctx.changePassword({ current_password: 'a', new_password: 'b' } as never) })
  await waitFor(() => expect(screen.getByText('OUT')).toBeInTheDocument())
  expect(ctx.user).toBeNull()
  expect(ctx.permissions).toEqual([])
  expect(localStorage.getItem('user')).toBeNull()
  expect(localStorage.getItem('auth_token')).toBeNull()
  expect(authApi.logout).not.toHaveBeenCalled()
})

it('a FAILED password change keeps the session', async () => {
  localStorage.setItem('auth_token', 'old-token')
  vi.mocked(authApi.getAuthStatus).mockResolvedValue(authedStatus as never)
  vi.mocked(authApi.getPermissions).mockResolvedValue({ permissions: ['*'] } as never)
  vi.mocked(authApi.changePassword).mockRejectedValue({ response: { data: { error: 'wrong current password' } } })
  mount()
  await waitFor(() => expect(screen.getByText('IN')).toBeInTheDocument())
  await act(async () => { try { await ctx.changePassword({ current_password: 'x', new_password: 'y' } as never) } catch { /* expected */ } })
  expect(screen.getByText('IN')).toBeInTheDocument()
  expect(localStorage.getItem('auth_token')).toBe('old-token')
})

// The local state reset must not depend on the auth-status refetch: even when the
// status endpoint still (wrongly) reports a session, change-password clears user.
it('clears user and permissions immediately, independent of the auth-status refetch', async () => {
  vi.mocked(authApi.getAuthStatus).mockResolvedValue(authedStatus as never)
  vi.mocked(authApi.getPermissions).mockResolvedValue({ permissions: ['*'] } as never)
  vi.mocked(authApi.changePassword).mockResolvedValue(undefined as never)
  mount()
  await waitFor(() => expect(screen.getByText('IN')).toBeInTheDocument())
  vi.mocked(authApi.getAuthStatus).mockImplementation(() => new Promise(() => {}) as never)
  await act(async () => { await ctx.changePassword({ current_password: 'a', new_password: 'b' } as never) })
  expect(ctx.user).toBeNull()
})
