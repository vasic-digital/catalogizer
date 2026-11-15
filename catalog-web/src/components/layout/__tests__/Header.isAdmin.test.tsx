import React from 'react'
import { screen } from '@testing-library/react'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '@/contexts/ThemeContext'
import { Header } from '../Header'
import { useAuth } from '@/contexts/AuthContext'

// SSEF4R-M2: the Admin nav entry follows useAuth().isAdmin only.
vi.mock('@/contexts/AuthContext', async () => ({ useAuth: vi.fn() }))
vi.mock('@/components/ui/ConnectionStatus', async () => ({ ConnectionStatus: () => <div /> }))
const mockUseAuth = vi.mocked(useAuth)
const logout = vi.fn()
const customRender = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <ThemeProvider>
          <MemoryRouter>{children}</MemoryRouter>
        </ThemeProvider>
      </QueryClientProvider>
    ),
  })

describe('Header Admin entry uses useAuth().isAdmin', () => {
  it('shows Admin for a custom admin role (role name Ops) when isAdmin is true', () => {
    mockUseAuth.mockReturnValue({ user: { id: 1, username: 'o', first_name: 'O', last_name: 'P', role: { id: 7, name: 'Ops' }, role_id: 7 }, isAuthenticated: true, hasPermission: () => true, isAdmin: true, logout } as never)
    customRender(<Header />)
    expect(screen.getAllByText('Admin').length).toBeGreaterThan(0)
  })

  it('hides Admin for role name Admin when isAdmin is false', () => {
    mockUseAuth.mockReturnValue({ user: { id: 1, username: 'o', first_name: 'O', last_name: 'P', role: { id: 1, name: 'Admin' }, role_id: 1 }, isAuthenticated: true, hasPermission: () => true, isAdmin: false, logout } as never)
    customRender(<Header />)
    expect(screen.queryByText('Admin')).not.toBeInTheDocument()
  })
})
