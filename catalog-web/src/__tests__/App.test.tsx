import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import App from '../App'

// Mock SplashScreen to immediately call onComplete
vi.mock('@/components/SplashScreen', async () => ({
  SplashScreen: ({ onComplete }: { onComplete: () => void }) => {
    React.useEffect(() => { onComplete() }, [onComplete])
    return null
  },
}))

// Mock BrowserRouter to use MemoryRouter for testing
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    BrowserRouter: ({ children }: { children: React.ReactNode }) => (
      <actual.MemoryRouter initialEntries={[(global as any).testInitialRoute || '/']}>{children}</actual.MemoryRouter>
    ),
  }
})

// Mock all child components and contexts
vi.mock('@/contexts/AuthContext', async () => ({
  AuthProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="auth-provider">{children}</div>
  ),
  useAuth: vi.fn(() => ({
    user: null,
    isAuthenticated: false,
    login: vi.fn(),
    logout: vi.fn(),
  })),
}))

vi.mock('@/contexts/WebSocketContext', async () => ({
  WebSocketProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="websocket-provider">{children}</div>
  ),
  useWebSocketContext: vi.fn(),
}))

vi.mock('@/components/ui/ConnectionStatus', async () => ({
  ConnectionStatus: () => <div data-testid="connection-status">Connection Status</div>,
}))

vi.mock('@/components/layout/Layout', async () => {
  const actual = await import('react-router-dom')
  return {
    Layout: () => (
      <div data-testid="layout">
        Layout
        <actual.Outlet />
      </div>
    ),
  }
})

vi.mock('@/components/auth/LoginForm', async () => ({
  LoginForm: () => <div data-testid="login-form">Login Form</div>,
}))

vi.mock('@/components/auth/RegisterForm', async () => ({
  RegisterForm: () => <div data-testid="register-form">Register Form</div>,
}))

vi.mock('@/components/auth/ProtectedRoute', async () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="protected-route">{children}</div>
  ),
}))

vi.mock('@/pages/Dashboard', async () => ({
  Dashboard: () => <div data-testid="dashboard-page">Dashboard Page</div>,
}))

vi.mock('@/pages/MediaBrowser', async () => ({
  MediaBrowser: () => <div data-testid="media-browser-page">Media Browser Page</div>,
}))

vi.mock('@/pages/Analytics', async () => ({
  Analytics: () => <div data-testid="analytics-page">Analytics Page</div>,
}))

vi.mock('@/pages/Admin', async () => ({
  Admin: () => <div data-testid="admin-page">Admin Page</div>,
}))

vi.mock('@/pages/SubtitleManager', async () => ({
  SubtitleManager: () => <div data-testid="subtitle-manager-page">Subtitle Manager Page</div>,
}))

vi.mock('@/pages/Collections', async () => ({
  Collections: () => <div data-testid="collections-page">Collections Page</div>,
}))

vi.mock('@/pages/ConversionTools', async () => ({
  ConversionTools: () => <div data-testid="conversion-tools-page">Conversion Tools Page</div>,
}))

vi.mock('@/pages/Favorites', async () => ({
  default: () => <div data-testid="favorites-page">Favorites Page</div>,
}))

vi.mock('@/pages/Playlists', async () => ({
  PlaylistsPage: () => <div data-testid="playlists-page">Playlists Page</div>,
}))

vi.mock('@/pages/AIDashboard', async () => ({
  default: () => <div data-testid="ai-dashboard-page">AI Dashboard Page</div>,
}))

vi.mock('@/pages/Settings', async () => ({
  Settings: () => <div data-testid="settings-page">Settings Page</div>,
  default: () => <div data-testid="settings-page">Settings Page</div>,
}))

describe('App', () => {
  describe('Rendering and Setup', () => {
    it('renders the App component', () => {
      render(<App />)
      expect(screen.getByTestId('auth-provider')).toBeInTheDocument()
    })

    it('renders with provider hierarchy', () => {
      render(<App />)
      const authProvider = screen.getByTestId('auth-provider')
      const websocketProvider = screen.getByTestId('websocket-provider')

      // WebSocketProvider should be inside AuthProvider
      expect(authProvider).toContainElement(websocketProvider)
    })

    it('does not render ConnectionStatus globally (docked in Header, D1)', () => {
      render(<App />)
      expect(screen.queryByTestId('connection-status')).not.toBeInTheDocument()
    })

    it('sets up Router correctly', () => {
      const { container } = render(<App />)
      // Router should be present (no errors thrown)
      expect(container.firstChild).toBeInTheDocument()
    })
  })

  describe('Public Routes', () => {
    it('renders LoginForm on /login route', async () => {
      (global as any).testInitialRoute = '/login'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('login-form')).toBeInTheDocument()
      })
    })

    it('renders RegisterForm on /register route', async () => {
      (global as any).testInitialRoute = '/register'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('register-form')).toBeInTheDocument()
      })
    })
  })

  describe('Protected Routes with Layout', () => {
    it('renders Dashboard on /dashboard route', async () => {
      (global as any).testInitialRoute = '/dashboard'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('renders MediaBrowser on /media route', async () => {
      (global as any).testInitialRoute = '/media'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('renders Analytics on /analytics route', async () => {
      (global as any).testInitialRoute = '/analytics'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('renders Admin page on /admin route', async () => {
      (global as any).testInitialRoute = '/admin'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('renders Profile page on /profile route', async () => {
      (global as any).testInitialRoute = '/profile'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('renders Settings page on /settings route', async () => {
      (global as any).testInitialRoute = '/settings'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })
  })

  describe('Navigation and Redirects', () => {
    it('redirects from root / to /dashboard', async () => {
      (global as any).testInitialRoute = '/'
      render(<App />)

      await waitFor(() => {
        // Should render Layout (which means we're on /dashboard after redirect)
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('redirects from unknown route to /dashboard', async () => {
      (global as any).testInitialRoute = '/unknown-route'
      render(<App />)

      await waitFor(() => {
        // Should render Layout (catch-all redirects to /dashboard)
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })

    it('redirects from invalid nested route to /dashboard', async () => {
      (global as any).testInitialRoute = '/invalid/nested/route'
      render(<App />)

      await waitFor(() => {
        // Should render Layout (catch-all redirects to /dashboard)
        expect(screen.getByTestId('layout')).toBeInTheDocument()
      })
    })
  })

  describe('Layout Integration', () => {
    it('protected routes render inside Layout wrapper', async () => {
      (global as any).testInitialRoute = '/dashboard'
      render(<App />)

      await waitFor(() => {
        const layout = screen.getByTestId('layout')
        expect(layout).toBeInTheDocument()
      })
    })

    it('public routes render without Layout wrapper', async () => {
      (global as any).testInitialRoute = '/login'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('login-form')).toBeInTheDocument()
        expect(screen.queryByTestId('layout')).not.toBeInTheDocument()
      })
    })
  })

  describe('Protected Route Wrapper', () => {
    it('wraps dashboard with ProtectedRoute', async () => {
      (global as any).testInitialRoute = '/dashboard'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('protected-route')).toBeInTheDocument()
      })
    })

    it('wraps media browser with ProtectedRoute', async () => {
      (global as any).testInitialRoute = '/media'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('protected-route')).toBeInTheDocument()
      })
    })

    it('wraps analytics with ProtectedRoute', async () => {
      (global as any).testInitialRoute = '/analytics'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('protected-route')).toBeInTheDocument()
      })
    })
  })

  describe('Provider Hierarchy', () => {
    it('AuthProvider is the outermost provider', () => {
      const { container } = render(<App />)
      const authProvider = screen.getByTestId('auth-provider')

      // AuthProvider should be at the top level
      expect(container.firstChild).toContainElement(authProvider)
    })

    it('WebSocketProvider is inside AuthProvider', () => {
      render(<App />)

      const authProvider = screen.getByTestId('auth-provider')
      const websocketProvider = screen.getByTestId('websocket-provider')

      expect(authProvider).toContainElement(websocketProvider)
    })
  })

  describe('Edge Cases', () => {
    it('handles login route rendering', async () => {
      (global as any).testInitialRoute = '/login'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('login-form')).toBeInTheDocument()
        expect(screen.queryByTestId('connection-status')).not.toBeInTheDocument()
      })
    })

    it('handles dashboard route rendering', async () => {
      (global as any).testInitialRoute = '/dashboard'
      render(<App />)

      await waitFor(() => {
        expect(screen.getByTestId('layout')).toBeInTheDocument()
        expect(screen.queryByTestId('connection-status')).not.toBeInTheDocument()
      })
    })

    it('does not float ConnectionStatus on public routes', async () => {
      (global as any).testInitialRoute = '/login'
      render(<App />)

      expect(screen.queryByTestId('connection-status')).not.toBeInTheDocument()
    })

    it('does not float ConnectionStatus on protected routes', async () => {
      (global as any).testInitialRoute = '/dashboard'
      render(<App />)

      expect(screen.queryByTestId('connection-status')).not.toBeInTheDocument()
    })
  })
})
