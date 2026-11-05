// UI-D8 (CAT-UI-D8): the header search must drive the Browse page's entity query. Real Header + real EntityBrowser +
// real router (no mocked navigate); only the HTTP layer (entityApi) is replaced. Live RED: docs/qa/ui-d7d8-20261105/.
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Header } from '../Header'
import { EntityBrowser } from '@/pages/EntityBrowser'
import { entityApi } from '@/lib/mediaApi'
import { useAuth } from '@/contexts/AuthContext'
import { ThemeProvider } from '@/contexts/ThemeContext'

vi.mock('@/contexts/AuthContext', async () => ({ useAuth: vi.fn() }))
vi.mock('@/components/ui/ConnectionStatus', async () => ({ ConnectionStatus: () => <div /> }))
vi.mock('@/lib/mediaApi', () => ({
  entityApi: {
    getEntityTypes: vi.fn().mockResolvedValue({ types: [{ id: 1, name: 'movie', description: 'Movies', count: 10 }] }),
    getEntityStats: vi.fn().mockResolvedValue({ total_entities: 10, by_type: { movie: 10 } }),
    getEntities: vi.fn().mockResolvedValue({ items: [], total: 0, limit: 24, offset: 0 }),
    browseByType: vi.fn().mockResolvedValue({ items: [], total: 0, limit: 24, offset: 0, type: 'movie' }),
  },
}))

const TRICKY = 'a&b #c/+'

function mountApp() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <ThemeProvider>
        <MemoryRouter initialEntries={['/dashboard']}>
          <Header />
          <Routes>
            <Route path="/dashboard" element={<div>dash</div>} />
            <Route path="/browse" element={<EntityBrowser />} />
          </Routes>
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>,
  )
}

describe('Header search drives Browse (UI-D8)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useAuth).mockReturnValue({
      user: { id: 1, username: 'qa', role: 'admin', permissions: [] },
      isAuthenticated: true,
      logout: vi.fn(),
    } as never)
  })

  it('submitting a term in the header search queries entities with that term', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <ThemeProvider>
          <MemoryRouter initialEntries={['/dashboard']}>
            <Header />
            <Routes>
              <Route path="/dashboard" element={<div>dash</div>} />
              <Route path="/browse" element={<EntityBrowser />} />
            </Routes>
          </MemoryRouter>
        </ThemeProvider>
      </QueryClientProvider>,
    )
    const user = userEvent.setup()
    await user.type(document.getElementById('header-search') as HTMLInputElement, 'matrix{Enter}')

    await waitFor(() => expect(entityApi.getEntities).toHaveBeenCalled())
    expect(entityApi.getEntities).toHaveBeenCalledWith(expect.objectContaining({ query: 'matrix' }))
    // filtered state: the unfiltered type selector is not what the user is left looking at
    expect(screen.queryByText(/across all types/i)).not.toBeInTheDocument()
  })

  it('desktop: a term with & # / + arrives intact and without stray params', async () => {
    mountApp()
    const user = userEvent.setup()
    await user.type(document.getElementById('header-search') as HTMLInputElement, `${TRICKY}{Enter}`)
    await waitFor(() => expect(entityApi.getEntities).toHaveBeenCalled())
    expect(entityApi.getEntities).toHaveBeenLastCalledWith(expect.objectContaining({ query: TRICKY, offset: 0 }))
    expect(entityApi.browseByType).not.toHaveBeenCalled() // no stray `type` param produced by an unescaped &
  })

  it('mobile menu: Enter in the mobile input queries entities, closes the menu, keeps the term intact', async () => {
    mountApp()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Open menu' }))
    await user.type(document.getElementById('mobile-search') as HTMLInputElement, `${TRICKY}{Enter}`)
    await waitFor(() => expect(entityApi.getEntities).toHaveBeenCalled())
    expect(entityApi.getEntities).toHaveBeenLastCalledWith(expect.objectContaining({ query: TRICKY, offset: 0 }))
    expect(entityApi.browseByType).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Open menu' })).toHaveAttribute('aria-expanded', 'false')
  })
})
