import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import { AuthProvider } from '../AuthContext'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { authApi } from '@/lib/api'

// Unit-class (authApi faked at the module edge); live proof is the reload
// scenario in e2e/real-backend-pages.spec.ts.
vi.mock('@/lib/api', async () => ({
  authApi: { getAuthStatus: vi.fn(), getPermissions: vi.fn(), login: vi.fn(), logout: vi.fn(), register: vi.fn(), updateProfile: vi.fn(), changePassword: vi.fn() },
}))
vi.mock('react-hot-toast', () => ({ default: { success: vi.fn(), error: vi.fn() } }))

const visited: string[] = []
function Spy() {
  visited.push(useLocation().pathname)
  return null
}

describe('hard reload of a protected page with a valid session', () => {
  it('never bounces to /login while the session is being restored', async () => {
    visited.length = 0
    vi.mocked(authApi.getAuthStatus).mockResolvedValue({
      authenticated: true,
      permissions: ['*'],
      user: { id: 1, username: 'u', role_id: 1, role: { id: 1, name: 'Admin' } },
    } as never)
    vi.mocked(authApi.getPermissions).mockResolvedValue({ permissions: ['*'] } as never)

    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <AuthProvider>
          <MemoryRouter initialEntries={['/collections']}>
            <Spy />
            <Routes>
              <Route path="/login" element={<div>LOGIN</div>} />
              <Route path="/collections" element={<ProtectedRoute><div>COLLECTIONS PAGE</div></ProtectedRoute>} />
            </Routes>
          </MemoryRouter>
        </AuthProvider>
      </QueryClientProvider>
    )

    await waitFor(() => expect(screen.getByText('COLLECTIONS PAGE')).toBeInTheDocument())
    expect(visited).not.toContain('/login')
    expect(screen.queryByText('LOGIN')).not.toBeInTheDocument()
  })
})
