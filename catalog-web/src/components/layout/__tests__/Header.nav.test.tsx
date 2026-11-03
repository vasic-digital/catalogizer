import { render, screen, within } from '@testing-library/react'
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

const authed = (admin = false) =>
  mockUseAuth.mockReturnValue({
    user: { id: 1, username: 'u', first_name: 'U', last_name: 'X', role: admin ? { id: 1, name: 'Admin' } : 'user' },
    isAuthenticated: true,
    logout: mockLogout,
  } as never)

// Every destination in the desktop nav, in priority order (tier = first breakpoint where it is inline).
const ALL = ['Dashboard', 'Media', 'Browse', 'Favorites', 'Playlists', 'Analytics', 'Subtitles', 'Collections', 'Convert']

describe('Header navigation overflow ("More" menu) and mobile parity', () => {
  beforeEach(() => vi.clearAllMocks())

  it('exposes a More menu button (aria-haspopup=menu) when authenticated', () => {
    authed()
    customRender(<Header />)
    const more = screen.getByRole('button', { name: /more/i })
    expect(more).toHaveAttribute('aria-haspopup', 'menu')
    expect(more).toHaveAttribute('aria-expanded', 'false')
    // the More button disappears once every item is inline (>= 2xl)
    expect(more.parentElement!.className).toContain('2xl:hidden')
  })

  it('has no More menu when logged out', () => {
    mockUseAuth.mockReturnValue({ user: null, isAuthenticated: false, logout: mockLogout } as never)
    customRender(<Header />)
    expect(screen.queryByRole('button', { name: /more/i })).not.toBeInTheDocument()
  })

  it('keeps the highest-priority items always inline and demotes the rest by breakpoint', () => {
    authed()
    customRender(<Header />)
    const cls = (label: string) => screen.getByText(label).closest('a')!.parentElement!.className + ' ' + screen.getByText(label).closest('a')!.className
    expect(cls('Dashboard')).not.toMatch(/\bhidden\b/)
    expect(cls('Media')).not.toMatch(/\bhidden\b/)
    expect(cls('Browse')).toMatch(/\bhidden\b.*\blg:/)
    expect(cls('Favorites')).toMatch(/\bhidden\b.*\blg:/)
    expect(cls('Playlists')).toMatch(/\bhidden\b.*\bxl:/)
    expect(cls('Analytics')).toMatch(/\bhidden\b.*\bxl:/)
    expect(cls('Collections')).toMatch(/\bhidden\b.*\b2xl:/)
  })

  it('opens a menu listing exactly the demoted items, each hidden once it is inline, and closes on Escape', async () => {
    authed()
    const user = userEvent.setup()
    customRender(<Header />)
    const more = screen.getByRole('button', { name: /more/i })
    await user.click(more)
    expect(more).toHaveAttribute('aria-expanded', 'true')
    const menu = screen.getByRole('menu')
    const items = within(menu).getAllByRole('menuitem')
    const labels = items.map((i) => i.textContent?.trim())
    expect(labels).toEqual(['Browse', 'Favorites', 'Playlists', 'Analytics', 'Subtitles', 'Collections', 'Convert'])
    const byLabel = Object.fromEntries(items.map((i) => [i.textContent?.trim(), i.className]))
    expect(byLabel['Browse']).toContain('lg:hidden')
    expect(byLabel['Playlists']).toContain('xl:hidden')
    expect(byLabel['Collections']).toContain('2xl:hidden')
    expect(within(menu).queryByText('Dashboard')).not.toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(more).toHaveAttribute('aria-expanded', 'false')
  })

  it('includes Admin in the More menu only for admins', async () => {
    const user = userEvent.setup()
    authed(false)
    const r = customRender(<Header />)
    await user.click(screen.getByRole('button', { name: /more/i }))
    expect(within(screen.getByRole('menu')).queryByText('Admin')).not.toBeInTheDocument()
    r.unmount()
    authed(true)
    customRender(<Header />)
    await user.click(screen.getByRole('button', { name: /more/i }))
    expect(within(screen.getByRole('menu')).getByText('Admin')).toBeInTheDocument()
  })

  it('menu items are touch-sized links', async () => {
    authed()
    const user = userEvent.setup()
    customRender(<Header />)
    await user.click(screen.getByRole('button', { name: /more/i }))
    for (const i of screen.getAllByRole('menuitem')) expect(i.className).toContain('min-h-[44px]')
  })

  it('mobile menu offers every destination the desktop nav offers (Analytics was missing)', async () => {
    authed()
    const user = userEvent.setup()
    customRender(<Header />)
    const toggle = screen.getAllByRole('button').find((b) => b.className.includes('md:hidden'))!
    await user.click(toggle)
    for (const label of ALL) {
      const links = screen.getAllByText(label).map((n) => n.closest('a')).filter(Boolean)
      expect(links.length, label).toBeGreaterThanOrEqual(2) // desktop + mobile copy
    }
  })

  it('every inline nav link and the standalone logo link are touch-sized', () => {
    authed(true)
    customRender(<Header />)
    for (const label of ALL.concat('Admin')) {
      const cn = screen.getByText(label).closest('a')!.className
      expect(cn, label).toContain('touch-target')
      expect(cn, label).toContain('flex-shrink-0') // a scrolling flex strip must not squash links into each other
    }
  })

  // At 768-1023px the full search box leaves no room for the nav ("Media" ended up under "More"). From md to lg the
  // box collapses into a single touch-sized search link; the box returns at lg.
  it('search box is lg+ only and a compact search link covers md..lg', () => {
    authed()
    customRender(<Header />)
    const input = screen.getAllByPlaceholderText(/search movies/i)[0]
    expect(input.closest('.relative')!.parentElement!.className).toMatch(/\bhidden\b.*\blg:flex\b/)
    const compact = screen.getByRole('link', { name: 'Search' })
    expect(compact).toHaveAttribute('href', '/browse')
    expect(compact.className).toContain('lg:hidden')
    expect(compact.className).toContain('touch-target')
  })
})
