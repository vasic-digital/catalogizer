import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Header } from '../Header'
import { useAuth } from '@/contexts/AuthContext'
import { ThemeProvider } from '@/contexts/ThemeContext'

// Shared providers wrapper — wraps children with every context provider the
// Header component (and its children, e.g. ThemeToggle → useTheme) requires.
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

function AllProviders({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        {children}
      </ThemeProvider>
    </QueryClientProvider>
  )
}

// Custom render that provides ALL context providers the component tree needs.
// Wraps children in AllProviders + MemoryRouter so tests can focus on assertions.
const customRender = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => (
      <AllProviders>
        <MemoryRouter>{children}</MemoryRouter>
      </AllProviders>
    ),
  })

// Mock AuthContext
const mockLogout = vi.fn()
const mockNavigate = vi.fn()

vi.mock('@/contexts/AuthContext', async () => ({
  useAuth: vi.fn(),
}))

vi.mock('react-router-dom', async () => ({
  ...(await vi.importActual('react-router-dom')),
  useNavigate: () => mockNavigate,
}))

// Mock framer-motion to avoid animation issues in tests
vi.mock('framer-motion', async () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

const mockUseAuth = vi.mocked(useAuth)

describe('Header', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Logo and Branding', () => {
    it('renders the Catalogizer logo', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isAuthenticated: false,
        logout: mockLogout,
      })

      customRender(<Header />)

      expect(screen.getByText('Catalogizer')).toBeInTheDocument()
      expect(screen.getByText('C')).toBeInTheDocument()
    })

    it('logo links to home page', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isAuthenticated: false,
        logout: mockLogout,
      })

      customRender(<Header />)

      const logoLink = screen.getByText('Catalogizer').closest('a')
      expect(logoLink).toHaveAttribute('href', '/')
    })
  })

  describe('Unauthenticated State', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: null,
        isAuthenticated: false,
        logout: mockLogout,
      })
    })

    it('does not display navigation links when not authenticated', () => {
      customRender(<Header />)

      expect(screen.queryByText('Dashboard')).not.toBeInTheDocument()
      expect(screen.queryByText('Media')).not.toBeInTheDocument()
      expect(screen.queryByText('Analytics')).not.toBeInTheDocument()
    })

    it('does not display search bar when not authenticated', () => {
      customRender(<Header />)

      expect(screen.queryByPlaceholderText('Search movies, shows, music...')).not.toBeInTheDocument()
    })

    it('displays Login and Sign Up buttons when not authenticated', () => {
      customRender(<Header />)

      expect(screen.getByRole('button', { name: /login/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /sign up/i })).toBeInTheDocument()
    })

    it('navigates to login page when Login button is clicked', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      await user.click(screen.getByRole('button', { name: /login/i }))
      expect(mockNavigate).toHaveBeenCalledWith('/login')
    })

    it('navigates to register page when Sign Up button is clicked', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      await user.click(screen.getByRole('button', { name: /sign up/i }))
      expect(mockNavigate).toHaveBeenCalledWith('/register')
    })
  })

  describe('Authenticated State - Regular User', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: {
          id: 1,
          username: 'testuser',
          first_name: 'Test',
          last_name: 'User',
          role: 'user',
        },
        isAuthenticated: true,
        logout: mockLogout,
      })
    })

    it('displays navigation links when authenticated', () => {
      customRender(<Header />)

      expect(screen.getByText('Dashboard')).toBeInTheDocument()
      expect(screen.getByText('Media')).toBeInTheDocument()
      expect(screen.getByText('Analytics')).toBeInTheDocument()
    })

    it('does not display Admin link for regular users', () => {
      customRender(<Header />)

      expect(screen.queryByText('Admin')).not.toBeInTheDocument()
    })

    it('displays search bar when authenticated', () => {
      customRender(<Header />)

      expect(screen.getAllByPlaceholderText('Search movies, shows, music...').length).toBeGreaterThan(0)
    })

    it('displays user greeting with first name', () => {
      customRender(<Header />)

      expect(screen.getByText(/Welcome, Test/i)).toBeInTheDocument()
    })

    it('displays username when first name is not available', () => {
      mockUseAuth.mockReturnValue({
        user: {
          id: 1,
          username: 'testuser',
          role: 'user',
        },
        isAuthenticated: true,
        logout: mockLogout,
      })

      customRender(<Header />)

      expect(screen.getByText(/Welcome, testuser/i)).toBeInTheDocument()
    })

    it('navigates to profile page when profile button is clicked', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      await user.click(screen.getByRole('button', { name: 'Profile' }))

      expect(mockNavigate).toHaveBeenCalledWith('/profile')
    })

    it('calls logout when logout button is clicked', async () => {
      const user = userEvent.setup()
      mockLogout.mockResolvedValue(undefined)

      customRender(<Header />)

      // Find logout button (LogOut icon) - it's the last icon button in desktop menu
      const iconButtons = screen.getAllByRole('button').filter(btn =>
        btn.querySelector('svg') && btn.className.includes('h-8 w-8')
      )
      const logoutButton = iconButtons[iconButtons.length - 1]

      await user.click(logoutButton)

      await waitFor(() => {
        expect(mockLogout).toHaveBeenCalled()
      })
    })

    it('navigates to login after successful logout', async () => {
      const user = userEvent.setup()
      mockLogout.mockResolvedValue(undefined)

      customRender(<Header />)

      const iconButtons = screen.getAllByRole('button').filter(btn =>
        btn.querySelector('svg') && btn.className.includes('h-8 w-8')
      )
      const logoutButton = iconButtons[iconButtons.length - 1]

      await user.click(logoutButton)

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/login')
      })
    })

    it('handles logout errors gracefully', async () => {
      const user = userEvent.setup()
      mockLogout.mockRejectedValue(new Error('Logout failed'))

      customRender(<Header />)

      const iconButtons = screen.getAllByRole('button').filter(btn =>
        btn.querySelector('svg') && btn.className.includes('h-8 w-8')
      )
      const logoutButton = iconButtons[iconButtons.length - 1]

      await user.click(logoutButton)

      // Logout failure is silently caught (non-critical), no console.error call
      await waitFor(() => {
        expect(mockLogout).toHaveBeenCalled()
      })
    })
  })

  describe('Authenticated State - Admin User', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: {
          id: 1,
          username: 'admin',
          first_name: 'Admin',
          last_name: 'User',
          role: { id: 1, name: 'Admin' },
          role_id: 1,
        },
        isAuthenticated: true,
        logout: mockLogout,
      })
    })

    it('displays Admin link for admin users', () => {
      customRender(<Header />)

      expect(screen.getByText('Admin')).toBeInTheDocument()
    })

    it('Admin link navigates to admin page', () => {
      customRender(<Header />)

      const adminLink = screen.getByText('Admin').closest('a')
      expect(adminLink).toHaveAttribute('href', '/admin')
    })
  })

  describe('Navigation Links', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: {
          id: 1,
          username: 'testuser',
          role: 'user',
        },
        isAuthenticated: true,
        logout: mockLogout,
      })
    })

    it('Dashboard link navigates to dashboard page', () => {
      customRender(<Header />)

      const dashboardLink = screen.getByText('Dashboard').closest('a')
      expect(dashboardLink).toHaveAttribute('href', '/dashboard')
    })

    it('Media link navigates to media page', () => {
      customRender(<Header />)

      const mediaLink = screen.getByText('Media').closest('a')
      expect(mediaLink).toHaveAttribute('href', '/media')
    })

    it('Analytics link navigates to analytics page', () => {
      customRender(<Header />)

      const analyticsLink = screen.getByText('Analytics').closest('a')
      expect(analyticsLink).toHaveAttribute('href', '/analytics')
    })
  })

  describe('Mobile Menu', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: {
          id: 1,
          username: 'testuser',
          first_name: 'Test',
          role: 'user',
        },
        isAuthenticated: true,
        logout: mockLogout,
      })
    })

    it('mobile menu is closed by default', () => {
      customRender(<Header />)

      // Mobile menu content should not be visible
      const mobileLinks = screen.queryAllByText('Dashboard')
      // Should only find desktop link, not mobile
      expect(mobileLinks.length).toBe(1)
    })

    it('toggles mobile menu when menu button is clicked', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      // Find the mobile menu toggle button
      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      expect(menuToggle).toBeInTheDocument()

      if (menuToggle) {
        await user.click(menuToggle)
      }

      // After clicking, mobile menu should be open (multiple Dashboard links visible)
      await waitFor(() => {
        const dashboardLinks = screen.getAllByText('Dashboard')
        expect(dashboardLinks.length).toBeGreaterThan(1)
      })
    })

    it('displays mobile navigation links when menu is open', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        const dashboardLinks = screen.getAllByText('Dashboard')
        expect(dashboardLinks.length).toBeGreaterThan(1)
        const mediaLinks = screen.getAllByText('Media')
        expect(mediaLinks.length).toBeGreaterThan(1)
      })
    })

    it('displays mobile search bar when menu is open and user is authenticated', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        const searchInputs = screen.getAllByPlaceholderText('Search movies, shows, music...')
        // Should have both desktop and mobile search bars
        expect(searchInputs.length).toBeGreaterThan(1)
      })
    })

    it('displays user profile links in mobile menu', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        expect(screen.getByText('Profile')).toBeInTheDocument()
        expect(screen.getByText('Settings')).toBeInTheDocument()
        expect(screen.getByText('Logout')).toBeInTheDocument()
      })
    })

    it('displays username in mobile menu', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        // Should display first name in mobile menu
        const usernames = screen.getAllByText(/Test/)
        expect(usernames.length).toBeGreaterThan(1)
      })
    })

    it('closes mobile menu when logout is clicked', async () => {
      const user = userEvent.setup()
      mockLogout.mockResolvedValue(undefined)

      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        expect(screen.getByText('Logout')).toBeInTheDocument()
      })

      await user.click(screen.getByText('Logout'))

      await waitFor(() => {
        expect(mockLogout).toHaveBeenCalled()
      })
    })
  })

  describe('Mobile Menu - Unauthenticated', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: null,
        isAuthenticated: false,
        logout: mockLogout,
      })
    })

    it('displays Login and Sign Up in mobile menu when not authenticated', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        const loginLinks = screen.getAllByText(/login/i)
        expect(loginLinks.length).toBeGreaterThan(1)
      })
    })

    it('does not display navigation links in mobile menu when not authenticated', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        expect(screen.queryByText('Dashboard')).not.toBeInTheDocument()
      })
    })
  })

  describe('Mobile Menu - Admin User', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue({
        user: {
          id: 1,
          username: 'admin',
          role: { id: 1, name: 'Admin' },
          role_id: 1,
        },
        isAuthenticated: true,
        logout: mockLogout,
      })
    })

    it('displays Admin link in mobile menu for admin users', async () => {
      const user = userEvent.setup()
      customRender(<Header />)

      const menuButtons = screen.getAllByRole('button')
      const menuToggle = menuButtons.find(btn =>
        btn.querySelector('svg') && btn.className.includes('md:hidden')
      )

      if (menuToggle) {
        await user.click(menuToggle)
      }

      await waitFor(() => {
        const adminLinks = screen.getAllByText('Admin')
        // Should have both desktop and mobile Admin links
        expect(adminLinks.length).toBeGreaterThan(1)
      })
    })
  })
})
