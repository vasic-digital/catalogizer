import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ProtectedRoute } from '../ProtectedRoute'
import { useAuth } from '@/contexts/AuthContext'

// SSEF4R-M2: requireAdmin consults useAuth().isAdmin only.
vi.mock('@/contexts/AuthContext', async () => ({ useAuth: vi.fn() }))
vi.mock('react-router-dom', async () => ({
  ...(await vi.importActual('react-router-dom')),
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate-to">{to}</div>,
}))
const mockUseAuth = vi.mocked(useAuth)

const base = { isAuthenticated: true, isLoading: false, hasPermission: vi.fn() }
const renderRoute = () =>
  render(
    <MemoryRouter>
      <ProtectedRoute requireAdmin>
        <div>Protected Content</div>
      </ProtectedRoute>
    </MemoryRouter>
  )

describe('ProtectedRoute requireAdmin uses useAuth().isAdmin', () => {
  it('admits a custom admin role (role_id 7, name Ops) when isAdmin is true', () => {
    mockUseAuth.mockReturnValue({ ...base, isAdmin: true, user: { id: 1, role_id: 7, role: { id: 7, name: 'Ops' } } } as never)
    renderRoute()
    expect(screen.getByText('Protected Content')).toBeInTheDocument()
  })

  it('rejects role_id 1 / name Admin when isAdmin is false', () => {
    mockUseAuth.mockReturnValue({ ...base, isAdmin: false, user: { id: 1, role_id: 1, role: { id: 1, name: 'Admin' } } } as never)
    renderRoute()
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
    expect(screen.getByTestId('navigate-to')).toHaveTextContent('/dashboard')
  })
})
