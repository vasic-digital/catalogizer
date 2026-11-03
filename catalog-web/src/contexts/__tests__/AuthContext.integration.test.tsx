import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from '../AuthContext'
import type { User, LoginResponse, AuthStatus } from '@/types/auth'
import { authApi } from '@/lib/api'
import toast from 'react-hot-toast'

// Mock the API module
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

// Mock react-hot-toast
vi.mock('react-hot-toast', async () => ({
  __esModule: true,
  default: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockAuthApi = vi.mocked(authApi)
const mockToast = vi.mocked(toast)

// --- Test helper components ---

const mockUser: User = {
  id: 1,
  username: 'testuser',
  email: 'test@example.com',
  first_name: 'Test',
  last_name: 'User',
  role_id: 2,
  role: { id: 2, name: 'user', description: 'Regular user', permissions: [], is_system: false, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockAdminUser: User = {
  ...mockUser,
  id: 2,
  username: 'admin',
  role_id: 1,
  role: { id: 1, name: 'Admin', description: 'Administrator', permissions: ['admin:system'], is_system: true, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
}

function AuthStateDisplay() {
  const {
    user,
    isAuthenticated,
    isLoading,
    permissions,
    isAdmin,
    hasPermission,
    canAccess,
  } = useAuth()

  if (isLoading) return <div data-testid="loading">Loading...</div>

  return (
    <div>
      <div data-testid="authenticated">{String(isAuthenticated)}</div>
      <div data-testid="is-admin">{String(isAdmin)}</div>
      <div data-testid="user-info">{user ? user.username : 'none'}</div>
      <div data-testid="user-role">{user ? user.role?.name : 'none'}</div>
      <div data-testid="permissions">{permissions.join(',')}</div>
      <div data-testid="has-read-media">{String(hasPermission('read:media'))}</div>
      <div data-testid="can-read-media">{String(canAccess('media', 'read'))}</div>
    </div>
  )
}

function LoginTrigger() {
  const { login } = useAuth()
  const [error, setError] = React.useState<string | null>(null)

  const handleLogin = async () => {
    try {
      await login({ username: 'testuser', password: 'password123' })
    } catch (e: any) {
      setError(e.message || 'Login failed')
    }
  }

  return (
    <div>
      <button data-testid="login-btn" onClick={handleLogin}>Login</button>
      {error && <div data-testid="login-error">{error}</div>}
    </div>
  )
}

function RegisterTrigger() {
  const { register } = useAuth()
  const [error, setError] = React.useState<string | null>(null)

  const handleRegister = async () => {
    try {
      await register({
        username: 'newuser',
        email: 'new@example.com',
        password: 'password123',
        first_name: 'New',
        last_name: 'User',
      })
    } catch (e: any) {
      setError(e.message || 'Register failed')
    }
  }

  return (
    <div>
      <button data-testid="register-btn" onClick={handleRegister}>Register</button>
      {error && <div data-testid="register-error">{error}</div>}
    </div>
  )
}

function LogoutTrigger() {
  const { logout } = useAuth()

  const handleLogout = async () => {
    try {
      await logout()
    } catch (error) {
      console.error("Error:", error);
      // handled by mutation
    }
  }

  return <button data-testid="logout-btn" onClick={handleLogout}>Logout</button>
}

function ProfileUpdateTrigger() {
  const { updateProfile } = useAuth()

  const handleUpdate = async () => {
    try {
      await updateProfile({ first_name: 'Updated', last_name: 'Name' })
    } catch (error) {
      console.error("Error:", error);
      // handled by mutation
    }
  }

  return <button data-testid="update-profile-btn" onClick={handleUpdate}>Update Profile</button>
}

function PasswordChangeTrigger() {
  const { changePassword } = useAuth()

  const handleChange = async () => {
    try {
      await changePassword({ current_password: 'old', new_password: 'new123456' })
    } catch (error) {
      console.error("Error:", error);
      // handled by mutation
    }
  }

  return <button data-testid="change-password-btn" onClick={handleChange}>Change Password</button>
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, cacheTime: 0 },
      mutations: { retry: false },
    },
  })
}

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{ui}</AuthProvider>
    </QueryClientProvider>
  )
}

// --- Tests ---

let setItemSpy: vi.SpyInstance
let removeItemSpy: vi.SpyInstance

describe('AuthContext Integration Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setItemSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { /* noop */ })
    removeItemSpy = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { /* noop */ })
  })

  afterEach(() => {
    setItemSpy.mockRestore()
    removeItemSpy.mockRestore()
  })

  describe('Initial State and Auth Status Check', () => {
    it('shows loading state while checking auth status', () => {
      mockAuthApi.getAuthStatus.mockReturnValue(new Promise(() => { /* noop */ })) // never resolves

      renderWithProviders(<AuthStateDisplay />)

      expect(screen.getByTestId('loading')).toBeInTheDocument()
    })

    it('shows unauthenticated state when auth status returns not authenticated', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: false,
      } as AuthStatus)

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })
      expect(screen.getByTestId('user-info')).toHaveTextContent('none')
      expect(screen.getByTestId('permissions')).toHaveTextContent('')
    })

    it('shows authenticated state when auth status returns valid user', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media', 'write:media'],
      } as AuthStatus)
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media', 'write:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })
      expect(screen.getByTestId('user-info')).toHaveTextContent('testuser')
      expect(screen.getByTestId('user-role')).toHaveTextContent('user')
      expect(screen.getByTestId('is-admin')).toHaveTextContent('false')
    })

    it('shows admin state for admin users', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockAdminUser,
        permissions: ['admin:system'],
      } as AuthStatus)
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'admin',
        permissions: ['admin:system'],
        is_admin: true,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('is-admin')).toHaveTextContent('true')
      })
      expect(screen.getByTestId('user-role')).toHaveTextContent('Admin')
    })

    it('clears user state when auth status check fails with 401', async () => {
      const error401 = { response: { status: 401 } }
      mockAuthApi.getAuthStatus.mockRejectedValue(error401)

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })
      expect(screen.getByTestId('user-info')).toHaveTextContent('none')
    })
  })

  describe('Login Flow', () => {
    beforeEach(() => {
      mockAuthApi.getAuthStatus.mockResolvedValue({ authenticated: false })
    })

    it('successfully logs in and updates state', async () => {
      const user = userEvent.setup()
      const loginResponse: LoginResponse = {
        user: mockUser,
        session_token: 'jwt-token-123',
        refresh_token: 'refresh-token-456',
        expires_at: '2024-01-01T01:00:00Z',
      }
      mockAuthApi.login.mockResolvedValue(loginResponse)

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LoginTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })

      await user.click(screen.getByTestId('login-btn'))

      await waitFor(() => {
        expect(mockAuthApi.login).toHaveBeenCalledWith({
          username: 'testuser',
          password: 'password123',
        })
      })

      // Verify token stored in localStorage
      expect(setItemSpy).toHaveBeenCalledWith('auth_token', 'jwt-token-123')
      expect(setItemSpy).toHaveBeenCalledWith('user', JSON.stringify(mockUser))

      // Verify toast shown
      expect(mockToast.success).toHaveBeenCalledWith('Successfully logged in!')
    })

    it('handles login failure and shows error toast', async () => {
      const user = userEvent.setup()
      const loginError = {
        response: { data: { error: 'Invalid credentials' } },
      }
      mockAuthApi.login.mockRejectedValue(loginError)

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LoginTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })

      await user.click(screen.getByTestId('login-btn'))

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Invalid credentials')
      })

      // User should still not be authenticated
      expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
    })

    it('shows generic error message when no specific error is returned', async () => {
      const user = userEvent.setup()
      // Error with 'Network' in message triggers isNetworkError() duck-type check
      // in AuthContext, which shows the network-specific error toast.
      mockAuthApi.login.mockRejectedValue(new Error('Network error'))

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LoginTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })

      await user.click(screen.getByTestId('login-btn'))

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Network error: please check your connection')
      })
    })
  })

  describe('Registration Flow', () => {
    beforeEach(() => {
      mockAuthApi.getAuthStatus.mockResolvedValue({ authenticated: false })
    })

    it('successfully registers a new user', async () => {
      const user = userEvent.setup()
      mockAuthApi.register.mockResolvedValue(mockUser)

      renderWithProviders(<RegisterTrigger />)

      await user.click(screen.getByTestId('register-btn'))

      await waitFor(() => {
        expect(mockAuthApi.register).toHaveBeenCalledWith({
          username: 'newuser',
          email: 'new@example.com',
          password: 'password123',
          first_name: 'New',
          last_name: 'User',
        })
      })

      expect(mockToast.success).toHaveBeenCalledWith('Registration successful! Please log in.')
    })

    it('handles registration failure', async () => {
      const user = userEvent.setup()
      const registerError = {
        response: { data: { error: 'Username already taken' } },
      }
      mockAuthApi.register.mockRejectedValue(registerError)

      renderWithProviders(<RegisterTrigger />)

      await user.click(screen.getByTestId('register-btn'))

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Username already taken')
      })
    })

    it('shows generic error when no specific message returned on registration failure', async () => {
      const user = userEvent.setup()
      // Error with 'Network' in message triggers isNetworkError() duck-type check
      // in AuthContext, which shows the network-specific error toast.
      mockAuthApi.register.mockRejectedValue(new Error('Network error'))

      renderWithProviders(<RegisterTrigger />)

      await user.click(screen.getByTestId('register-btn'))

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Network error: please check your connection')
      })
    })
  })

  describe('Logout Flow', () => {
    it('successfully logs out and clears state', async () => {
      const user = userEvent.setup()
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media'],
        is_admin: false,
      })
      mockAuthApi.logout.mockResolvedValue(undefined)

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LogoutTrigger />
        </>
      )

      // Wait for authenticated state
      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })

      await user.click(screen.getByTestId('logout-btn'))

      await waitFor(() => {
        expect(removeItemSpy).toHaveBeenCalledWith('auth_token')
        expect(removeItemSpy).toHaveBeenCalledWith('user')
      })

      expect(mockToast.success).toHaveBeenCalledWith('Successfully logged out!')
    })

    it('clears state even when logout API call fails', async () => {
      const user = userEvent.setup()
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: [],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: [],
        is_admin: false,
      })
      mockAuthApi.logout.mockRejectedValue(new Error('Network error'))

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LogoutTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })

      await user.click(screen.getByTestId('logout-btn'))

      // Even on error, tokens should be cleared
      await waitFor(() => {
        expect(removeItemSpy).toHaveBeenCalledWith('auth_token')
        expect(removeItemSpy).toHaveBeenCalledWith('user')
      })
    })
  })

  describe('Profile Update Flow', () => {
    beforeEach(() => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: [],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: [],
        is_admin: false,
      })
    })

    it('successfully updates profile', async () => {
      const user = userEvent.setup()
      const updatedUser: User = {
        ...mockUser,
        first_name: 'Updated',
        last_name: 'Name',
      }
      mockAuthApi.updateProfile.mockResolvedValue(updatedUser)

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <ProfileUpdateTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })

      await user.click(screen.getByTestId('update-profile-btn'))

      await waitFor(() => {
        expect(mockAuthApi.updateProfile).toHaveBeenCalledWith({
          first_name: 'Updated',
          last_name: 'Name',
        })
      })

      expect(mockToast.success).toHaveBeenCalledWith('Profile updated successfully!')
    })

    it('handles profile update failure', async () => {
      const user = userEvent.setup()
      mockAuthApi.updateProfile.mockRejectedValue({
        response: { data: { error: 'Email already in use' } },
      })

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <ProfileUpdateTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })

      await user.click(screen.getByTestId('update-profile-btn'))

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Email already in use')
      })
    })
  })

  describe('Password Change Flow', () => {
    beforeEach(() => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: [],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: [],
        is_admin: false,
      })
    })

    it('successfully changes password', async () => {
      const user = userEvent.setup()
      mockAuthApi.changePassword.mockResolvedValue(undefined)

      renderWithProviders(<PasswordChangeTrigger />)

      await user.click(screen.getByTestId('change-password-btn'))

      await waitFor(() => {
        expect(mockAuthApi.changePassword).toHaveBeenCalledWith({
          current_password: 'old',
          new_password: 'new123456',
        })
      })

      expect(mockToast.success).toHaveBeenCalledWith('Password changed. Please log in again.')
    })

    it('handles password change failure', async () => {
      const user = userEvent.setup()
      mockAuthApi.changePassword.mockRejectedValue({
        response: { data: { error: 'Current password is incorrect' } },
      })

      renderWithProviders(<PasswordChangeTrigger />)

      await user.click(screen.getByTestId('change-password-btn'))

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Current password is incorrect')
      })
    })
  })

  describe('Permissions', () => {
    it('hasPermission returns true for admin users regardless of permission', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockAdminUser,
        permissions: [],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'admin',
        permissions: [],
        is_admin: true,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('has-read-media')).toHaveTextContent('true')
      })
    })

    it('hasPermission returns true when user has the specific permission', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('has-read-media')).toHaveTextContent('true')
      })
    })

    it('hasPermission returns false when user lacks the permission', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['write:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['write:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('has-read-media')).toHaveTextContent('false')
      })
    })

    it('canAccess constructs permission string from resource and action', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('can-read-media')).toHaveTextContent('true')
      })
    })

    it('canAccess returns true for users with admin:system permission', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['admin:system'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['admin:system'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('can-read-media')).toHaveTextContent('true')
      })
    })

    it('updates permissions when getPermissions returns data', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media', 'write:media', 'delete:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('permissions')).toHaveTextContent('read:media,write:media,delete:media')
      })
    })
  })

  describe('useAuth Hook Error', () => {
    it('throws error when useAuth is used outside AuthProvider', () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => { /* noop */ })

      expect(() => {
        const qc = createQueryClient()
        render(
          <QueryClientProvider client={qc}>
            <AuthStateDisplay />
          </QueryClientProvider>
        )
      }).toThrow('useAuth must be used within an AuthProvider')

      consoleSpy.mockRestore()
    })
  })

  describe('Token Management', () => {
    it('stores token in localStorage on successful login', async () => {
      const user = userEvent.setup()
      mockAuthApi.getAuthStatus.mockResolvedValue({ authenticated: false })
      mockAuthApi.login.mockResolvedValue({
        user: mockUser,
        session_token: 'new-jwt-token',
        refresh_token: 'new-refresh-token',
        expires_at: '2024-01-01T01:00:00Z',
      })

      renderWithProviders(<LoginTrigger />)

      await user.click(screen.getByTestId('login-btn'))

      await waitFor(() => {
        expect(setItemSpy).toHaveBeenCalledWith('auth_token', 'new-jwt-token')
      })
    })

    it('removes token from localStorage on logout', async () => {
      const user = userEvent.setup()
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: [],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: [],
        is_admin: false,
      })
      mockAuthApi.logout.mockResolvedValue(undefined)

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LogoutTrigger />
        </>
      )

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })

      await user.click(screen.getByTestId('logout-btn'))

      await waitFor(() => {
        expect(removeItemSpy).toHaveBeenCalledWith('auth_token')
        expect(removeItemSpy).toHaveBeenCalledWith('user')
      })
    })

    it('stores user data in localStorage on login', async () => {
      const user = userEvent.setup()
      mockAuthApi.getAuthStatus.mockResolvedValue({ authenticated: false })
      mockAuthApi.login.mockResolvedValue({
        user: mockUser,
        session_token: 'jwt-token',
        refresh_token: 'refresh-token',
        expires_at: '2024-01-01T01:00:00Z',
      })

      renderWithProviders(<LoginTrigger />)

      await user.click(screen.getByTestId('login-btn'))

      await waitFor(() => {
        expect(setItemSpy).toHaveBeenCalledWith('user', JSON.stringify(mockUser))
      })
    })
  })

  describe('Auth State Persistence', () => {
    it('checks auth status on mount to restore session', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      // Should call getAuthStatus on mount
      expect(mockAuthApi.getAuthStatus).toHaveBeenCalledTimes(1)

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
        expect(screen.getByTestId('user-info')).toHaveTextContent('testuser')
      })
    })

    it('fetches permissions only when user is authenticated', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })

      // getPermissions should NOT be called when not authenticated
      expect(mockAuthApi.getPermissions).not.toHaveBeenCalled()
    })

    it('fetches permissions when user becomes authenticated', async () => {
      mockAuthApi.getAuthStatus.mockResolvedValue({
        authenticated: true,
        user: mockUser,
        permissions: ['read:media'],
      })
      mockAuthApi.getPermissions.mockResolvedValue({
        role: 'user',
        permissions: ['read:media'],
        is_admin: false,
      })

      renderWithProviders(<AuthStateDisplay />)

      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('true')
      })

      // getPermissions should be called for authenticated users
      await waitFor(() => {
        expect(mockAuthApi.getPermissions).toHaveBeenCalled()
      })
    })
  })

  describe('Full Login-Logout Cycle', () => {
    it('completes a full login then logout cycle', async () => {
      const user = userEvent.setup()
      mockAuthApi.getAuthStatus.mockResolvedValue({ authenticated: false })
      mockAuthApi.login.mockResolvedValue({
        user: mockUser,
        session_token: 'jwt-token',
        refresh_token: 'refresh-token',
        expires_at: '2024-01-01T01:00:00Z',
      })
      mockAuthApi.logout.mockResolvedValue(undefined)

      renderWithProviders(
        <>
          <AuthStateDisplay />
          <LoginTrigger />
          <LogoutTrigger />
        </>
      )

      // Initially not authenticated
      await waitFor(() => {
        expect(screen.getByTestId('authenticated')).toHaveTextContent('false')
      })

      // Login
      await user.click(screen.getByTestId('login-btn'))

      await waitFor(() => {
        expect(setItemSpy).toHaveBeenCalledWith('auth_token', 'jwt-token')
      })

      expect(mockToast.success).toHaveBeenCalledWith('Successfully logged in!')

      // Logout
      await user.click(screen.getByTestId('logout-btn'))

      await waitFor(() => {
        expect(removeItemSpy).toHaveBeenCalledWith('auth_token')
        expect(removeItemSpy).toHaveBeenCalledWith('user')
      })

      expect(mockToast.success).toHaveBeenCalledWith('Successfully logged out!')
    })
  })
})
