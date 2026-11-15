import React from 'react'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { ProtectedRoute } from '../ProtectedRoute'
import { useAuth } from '@/contexts/AuthContext'

// Mock the AuthContext
vi.mock('@/contexts/AuthContext', async () => ({
  useAuth: vi.fn(),
}))

const mockUseAuth = vi.mocked(useAuth)

function renderApp(
  initialRoute: string,
  routeElement: React.ReactElement,
  extraRoutes?: React.ReactElement[]
) {
  return render(
    <MemoryRouter initialEntries={[initialRoute]}>
      <Routes>
        <Route path="/login" element={<div data-testid="login-page">Login Page</div>} />
        <Route path="/dashboard" element={<div data-testid="dashboard">Dashboard</div>} />
        {routeElement}
        {extraRoutes}
      </Routes>
    </MemoryRouter>
  )
}

describe('ProtectedRoute Integration Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Loading State', () => {
    it('displays loading spinner while auth status is being checked', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: true,
        user: null,
        hasPermission: vi.fn(),
      })

      renderApp(
        '/protected',
        <Route
          path="/protected"
          element={
            <ProtectedRoute>
              <div data-testid="protected-content">Protected</div>
            </ProtectedRoute>
          }
        />
      )

      const spinner = document.querySelector('.animate-spin')
      expect(spinner).toBeInTheDocument()
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
      expect(screen.queryByTestId('login-page')).not.toBeInTheDocument()
    })

    it('does not render children while loading', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: true,
        user: { id: 1, username: 'test', role: 'admin' },
        hasPermission: vi.fn().mockReturnValue(true),
      })

      renderApp(
        '/protected',
        <Route
          path="/protected"
          element={
            <ProtectedRoute requireAdmin requiredPermission="admin:all">
              <div data-testid="protected-content">Protected</div>
            </ProtectedRoute>
          }
        />
      )

      expect(document.querySelector('.animate-spin')).toBeInTheDocument()
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    })
  })

  describe('Unauthenticated Redirect', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        hasPermission: vi.fn(),
      })
    })

    it('redirects to /login when not authenticated', () => {
      renderApp(
        '/protected',
        <Route
          path="/protected"
          element={
            <ProtectedRoute>
              <div data-testid="protected-content">Protected</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('login-page')).toBeInTheDocument()
      expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    })

    it('redirects to /login from deeply nested protected routes', () => {
      renderApp(
        '/admin/settings/advanced',
        <Route
          path="/admin/settings/advanced"
          element={
            <ProtectedRoute>
              <div data-testid="advanced-settings">Advanced Settings</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('login-page')).toBeInTheDocument()
      expect(screen.queryByTestId('advanced-settings')).not.toBeInTheDocument()
    })

    it('redirects to /login even when requireAdmin is also specified', () => {
      renderApp(
        '/admin-panel',
        <Route
          path="/admin-panel"
          element={
            <ProtectedRoute requireAdmin>
              <div data-testid="admin-panel">Admin Panel</div>
            </ProtectedRoute>
          }
        />
      )

      // Auth check comes first, so it should go to login, not dashboard
      expect(screen.getByTestId('login-page')).toBeInTheDocument()
      expect(screen.queryByTestId('dashboard')).not.toBeInTheDocument()
    })
  })

  describe('Authenticated User Access', () => {
    it('renders protected content for authenticated regular user', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn(),
      })

      renderApp(
        '/protected',
        <Route
          path="/protected"
          element={
            <ProtectedRoute>
              <div data-testid="protected-content">Protected Content</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
      expect(screen.queryByTestId('login-page')).not.toBeInTheDocument()
    })

    it('renders protected content for authenticated admin user', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'admin', role: 'admin' },
        hasPermission: vi.fn(),
      })

      renderApp(
        '/protected',
        <Route
          path="/protected"
          element={
            <ProtectedRoute>
              <div data-testid="protected-content">Protected Content</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    })
  })

  describe('Admin Route Protection', () => {
    it('allows admin users to access admin routes', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'admin', role: { id: 1, name: 'Admin' }, role_id: 1 },
        isAdmin: true,
        hasPermission: vi.fn(),
      })

      renderApp(
        '/admin-panel',
        <Route
          path="/admin-panel"
          element={
            <ProtectedRoute requireAdmin>
              <div data-testid="admin-panel">Admin Panel</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('admin-panel')).toBeInTheDocument()
    })

    it('redirects non-admin to /dashboard on admin routes', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn(),
      })

      renderApp(
        '/admin-panel',
        <Route
          path="/admin-panel"
          element={
            <ProtectedRoute requireAdmin>
              <div data-testid="admin-panel">Admin Panel</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      expect(screen.queryByTestId('admin-panel')).not.toBeInTheDocument()
    })
  })

  describe('Role-Based Route Protection', () => {
    it('allows user with matching role', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'editor1', role: { id: 3, name: 'editor' }, role_id: 3 },
        hasPermission: vi.fn(),
      })

      renderApp(
        '/editor',
        <Route
          path="/editor"
          element={
            <ProtectedRoute requiredRole="editor">
              <div data-testid="editor-page">Editor Page</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('editor-page')).toBeInTheDocument()
    })

    it('redirects user without matching role to /dashboard', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'viewer', role: 'viewer' },
        hasPermission: vi.fn(),
      })

      renderApp(
        '/editor',
        <Route
          path="/editor"
          element={
            <ProtectedRoute requiredRole="editor">
              <div data-testid="editor-page">Editor Page</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      expect(screen.queryByTestId('editor-page')).not.toBeInTheDocument()
    })
  })

  describe('Permission-Based Route Protection', () => {
    it('allows user with required permission', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn().mockReturnValue(true),
      })

      renderApp(
        '/upload',
        <Route
          path="/upload"
          element={
            <ProtectedRoute requiredPermission="write:media">
              <div data-testid="upload-page">Upload Page</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('upload-page')).toBeInTheDocument()
    })

    it('calls hasPermission with the correct permission string', () => {
      const mockHasPermission = vi.fn().mockReturnValue(true)
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: mockHasPermission,
      })

      renderApp(
        '/upload',
        <Route
          path="/upload"
          element={
            <ProtectedRoute requiredPermission="write:media">
              <div data-testid="upload-page">Upload Page</div>
            </ProtectedRoute>
          }
        />
      )

      expect(mockHasPermission).toHaveBeenCalledWith('write:media')
    })

    it('redirects user without required permission to /dashboard', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn().mockReturnValue(false),
      })

      renderApp(
        '/upload',
        <Route
          path="/upload"
          element={
            <ProtectedRoute requiredPermission="write:media">
              <div data-testid="upload-page">Upload Page</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      expect(screen.queryByTestId('upload-page')).not.toBeInTheDocument()
    })
  })

  describe('Combined Protection Rules', () => {
    it('checks authentication before admin requirement', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        hasPermission: vi.fn(),
      })

      renderApp(
        '/admin-settings',
        <Route
          path="/admin-settings"
          element={
            <ProtectedRoute requireAdmin requiredPermission="admin:settings">
              <div data-testid="admin-settings">Admin Settings</div>
            </ProtectedRoute>
          }
        />
      )

      // Should redirect to /login (not /dashboard) because auth check comes first
      expect(screen.getByTestId('login-page')).toBeInTheDocument()
      expect(screen.queryByTestId('dashboard')).not.toBeInTheDocument()
    })

    it('checks admin requirement before permission requirement', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn().mockReturnValue(true),
      })

      renderApp(
        '/admin-settings',
        <Route
          path="/admin-settings"
          element={
            <ProtectedRoute requireAdmin requiredPermission="admin:settings">
              <div data-testid="admin-settings">Admin Settings</div>
            </ProtectedRoute>
          }
        />
      )

      // Should redirect to /dashboard because admin check fails before permission check
      expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      expect(screen.queryByTestId('admin-settings')).not.toBeInTheDocument()
    })

    it('checks role requirement before permission requirement', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'viewer' },
        hasPermission: vi.fn().mockReturnValue(true),
      })

      renderApp(
        '/editor-tools',
        <Route
          path="/editor-tools"
          element={
            <ProtectedRoute requiredRole="editor" requiredPermission="use:tools">
              <div data-testid="editor-tools">Editor Tools</div>
            </ProtectedRoute>
          }
        />
      )

      // Should redirect to /dashboard because role check fails before permission check
      expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      expect(screen.queryByTestId('editor-tools')).not.toBeInTheDocument()
    })

    it('grants access when all combined conditions are met', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'admin', role: { id: 1, name: 'Admin' }, role_id: 1 },
        isAdmin: true,
        hasPermission: vi.fn().mockReturnValue(true),
      })

      renderApp(
        '/admin-settings',
        <Route
          path="/admin-settings"
          element={
            <ProtectedRoute requireAdmin requiredPermission="admin:settings">
              <div data-testid="admin-settings">Admin Settings</div>
            </ProtectedRoute>
          }
        />
      )

      expect(screen.getByTestId('admin-settings')).toBeInTheDocument()
    })
  })

  describe('Multiple Protected Routes', () => {
    it('each route enforces its own protection rules independently', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: 1, username: 'testuser', role: 'user' },
        hasPermission: vi.fn().mockReturnValue(false),
      })

      // Test that one route redirects while showing the correct dashboard
      renderApp(
        '/admin-only',
        <Route
          path="/admin-only"
          element={
            <ProtectedRoute requireAdmin>
              <div data-testid="admin-content">Admin Only</div>
            </ProtectedRoute>
          }
        />,
        [
          <Route
            key="public"
            path="/public-protected"
            element={
              <ProtectedRoute>
                <div data-testid="public-content">Public Protected</div>
              </ProtectedRoute>
            }
          />,
        ]
      )

      expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      expect(screen.queryByTestId('admin-content')).not.toBeInTheDocument()
    })
  })
})
