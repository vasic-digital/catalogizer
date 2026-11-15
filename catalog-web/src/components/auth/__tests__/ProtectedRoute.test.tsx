import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ProtectedRoute } from '../ProtectedRoute'
import { useAuth } from '@/contexts/AuthContext'

// Mock the AuthContext
vi.mock('@/contexts/AuthContext', async () => ({
  useAuth: vi.fn(),
}))

// Mock Navigate component
vi.mock('react-router-dom', async () => ({
  ...(await vi.importActual('react-router-dom')),
  Navigate: ({ to }: { to: string }) => <div data-testid="navigate-to">{to}</div>,
}))

const mockUseAuth = vi.mocked(useAuth)

describe('ProtectedRoute', () => {
  const TestChild = () => <div>Protected Content</div>

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Loading State', () => {
    it('displays loading spinner when auth is loading', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: true,
        user: null,
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute>
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      // Check for loading spinner
      const spinner = document.querySelector('.animate-spin')
      expect(spinner).toBeInTheDocument()
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
    })
  })

  describe('Unauthenticated Access', () => {
    it('redirects to login when user is not authenticated', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute>
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByTestId('navigate-to')).toHaveTextContent('/login')
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
    })
  })

  describe('Authenticated Access', () => {
    it('renders children when user is authenticated', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute>
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })

  describe('Admin Access Control', () => {
    it('allows access when user is admin and requireAdmin is true', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'admin', role: { id: 1, name: 'Admin' }, role_id: 1 },
        isAdmin: true,
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requireAdmin={true}>
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })

    it('redirects to dashboard when user is not admin but requireAdmin is true', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: { id: 2, name: 'user' }, role_id: 2 },
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requireAdmin={true}>
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByTestId('navigate-to')).toHaveTextContent('/dashboard')
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
    })
  })

  describe('Role-Based Access Control', () => {
    it('allows access when user has required role', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: { id: 3, name: 'editor' }, role_id: 3 },
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requiredRole="editor">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })

    it('redirects to dashboard when user does not have required role', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: { id: 4, name: 'viewer' }, role_id: 4 },
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requiredRole="editor">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByTestId('navigate-to')).toHaveTextContent('/dashboard')
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
    })
  })

  describe('Permission-Based Access Control', () => {
    it('allows access when user has required permission', () => {
      const mockHasPermission = vi.fn().mockReturnValue(true)
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: mockHasPermission,
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requiredPermission="read:media">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(mockHasPermission).toHaveBeenCalledWith('read:media')
      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })

    it('redirects to dashboard when user does not have required permission', () => {
      const mockHasPermission = vi.fn().mockReturnValue(false)
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: mockHasPermission,
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requiredPermission="admin:settings">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(mockHasPermission).toHaveBeenCalledWith('admin:settings')
      expect(screen.getByTestId('navigate-to')).toHaveTextContent('/dashboard')
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
    })
  })

  describe('Complex Access Scenarios', () => {
    it('checks authentication first, then admin, then role, then permission', () => {
      // Authentication check should happen first
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        hasPermission: vi.fn(),
      })

      const { rerender } = render(
        <MemoryRouter>
          <ProtectedRoute requireAdmin={true} requiredRole="editor" requiredPermission="write:media">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      // Should redirect to login (authentication check)
      expect(screen.getByTestId('navigate-to')).toHaveTextContent('/login')

      // Now authenticated but not admin
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn().mockReturnValue(true),
      })

      rerender(
        <MemoryRouter>
          <ProtectedRoute requireAdmin={true} requiredRole="editor" requiredPermission="write:media">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      // Should redirect to dashboard (admin check)
      expect(screen.getByTestId('navigate-to')).toHaveTextContent('/dashboard')
    })

    it('allows access when all conditions are met', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'admin', role: { id: 1, name: 'Admin' }, role_id: 1 },
        isAdmin: true,
        hasPermission: vi.fn().mockReturnValue(true),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute requireAdmin={true} requiredPermission="admin:all">
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })

  describe('No Access Restrictions', () => {
    it('only checks authentication when no restrictions are provided', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn(),
      })

      render(
        <MemoryRouter>
          <ProtectedRoute>
            <TestChild />
          </ProtectedRoute>
        </MemoryRouter>
      )

      expect(screen.getByText('Protected Content')).toBeInTheDocument()
    })
  })
})
