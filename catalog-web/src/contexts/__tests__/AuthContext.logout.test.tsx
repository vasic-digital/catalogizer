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

it.each([['ok', true], ['api-fails', false]])('logout (%s) ends logged out, not loading forever', async (_n, ok) => {
  let authed = true
  vi.mocked(authApi.getAuthStatus).mockImplementation(async () => (authed
    ? { authenticated: true, permissions: ['*'], user: { id: 1, username: 'u', role_id: 1, role: { id: 1, name: 'Admin' } } }
    : { authenticated: false }) as never)
  vi.mocked(authApi.getPermissions).mockResolvedValue({ permissions: ['*'] } as never)
  vi.mocked(authApi.logout).mockImplementation(async () => { authed = false; if (!ok) throw new Error('x') })
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><AuthProvider><Probe /></AuthProvider></QueryClientProvider>)
  await waitFor(() => expect(screen.getByText('IN')).toBeInTheDocument())
  await act(async () => { try { await ctx.logout() } catch { /* api failure path */ } })
  await waitFor(() => expect(screen.getByText('OUT')).toBeInTheDocument())
})
