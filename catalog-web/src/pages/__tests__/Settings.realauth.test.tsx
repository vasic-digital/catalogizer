import React from 'react'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Settings } from '../Settings'
import { AuthProvider } from '@/contexts/AuthContext'
import { authApi } from '@/lib/api'
import { syncApi } from '@/lib/syncApi'

// SSE-F4-R1/R2/R3/R4: these tests use the REAL AuthProvider (only the network
// edge, authApi, is mocked) so the role -> isAdmin -> Sync-tab chain is exercised
// end to end instead of mocking useAuth.
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

vi.mock('framer-motion', async () => {
  const MockMotionDiv = ({ children, initial: _i, animate: _a, transition: _t, ...props }: any) => (
    <div {...props}>{children}</div>
  )
  return { motion: { div: MockMotionDiv } }
})

vi.mock('react-hot-toast', async () => ({ default: { success: vi.fn(), error: vi.fn() } }))

vi.mock('@/lib/scansApi', async () => ({
  scansApi: {
    listScans: vi.fn().mockResolvedValue([]),
    queueScan: vi.fn().mockResolvedValue({}),
    getScanStatus: vi.fn().mockResolvedValue({}),
  },
}))
vi.mock('@/lib/smbApi', async () => ({
  smbApi: {
    discover: vi.fn().mockResolvedValue([]),
    testConnection: vi.fn().mockResolvedValue({ success: true, message: 'OK' }),
    browse: vi.fn().mockResolvedValue([]),
  },
}))
vi.mock('@/lib/syncApi', async () => ({
  syncApi: {
    listEndpoints: vi.fn().mockResolvedValue([]),
    createEndpoint: vi.fn().mockResolvedValue({}),
    startSync: vi.fn().mockResolvedValue({}),
    deleteEndpoint: vi.fn().mockResolvedValue(undefined),
    getSessions: vi.fn().mockResolvedValue([]),
    getStatistics: vi.fn().mockResolvedValue({
      total_syncs: 10,
      successful_syncs: 8,
      failed_syncs: 2,
      total_items_synced: 500,
    }),
  },
}))
vi.mock('@/lib/mediaApi', async () => ({
  mediaApi: { getStorageRoots: vi.fn().mockResolvedValue([]) },
}))

const ADMIN = { id: 1, username: 'root', role: { name: 'Admin' }, role_id: 1 }
const USER = { id: 2, username: 'bob', role: { name: 'User' }, role_id: 2 }

// The Admin role is seeded with '*'; isAdmin follows the server rule on these permissions (SSEF4R-M1).
const status = (user: object, permissions: string[] = []) => ({ authenticated: true, user, permissions })

function mount() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Settings />
      </AuthProvider>
    </QueryClientProvider>
  )
  return { queryClient, ...utils }
}

const syncCalls = () =>
  vi.mocked(syncApi.listEndpoints).mock.calls.length +
  vi.mocked(syncApi.getStatistics).mock.calls.length +
  vi.mocked(syncApi.getSessions).mock.calls.length

describe('Settings with the real AuthProvider', () => {
  beforeEach(() => {
    vi.mocked(syncApi.listEndpoints).mockClear()
    vi.mocked(syncApi.getStatistics).mockClear()
    vi.mocked(syncApi.getSessions).mockClear()
    vi.mocked(authApi.getPermissions).mockResolvedValue({ permissions: [] } as any)
  })

  it('R1: non-admin (role_id 2): no Sync tab, no /sync/* request, Scans/SMB intact', async () => {
    vi.mocked(authApi.getAuthStatus).mockResolvedValue(status(USER) as any)
    mount()
    await screen.findByText('Settings')
    await screen.findByText('No scans recorded yet.')
    await new Promise((r) => setTimeout(r, 50))
    expect(screen.queryByRole('button', { name: /^Sync$/ })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Scans/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /SMB Discovery/ })).toBeInTheDocument()
    expect(syncCalls()).toBe(0)
  })

  it('R1: admin (role_id 1) via the real provider sees the Sync tab and its data', async () => {
    vi.mocked(authApi.getAuthStatus).mockResolvedValue(status(ADMIN, ['*']) as any)
    mount()
    const syncTab = await screen.findByRole('button', { name: /^Sync$/ })
    await userEvent.setup().click(syncTab)
    await screen.findByText('Total Syncs')
    expect(syncCalls()).toBeGreaterThan(0)
  })

  it('R2/R4: admin on the Sync tab who loses admin lands on Scans, not a blank panel, and no further /sync/* query', async () => {
    vi.mocked(authApi.getAuthStatus).mockResolvedValue(status(ADMIN, ['*']) as any)
    const { queryClient } = mount()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /^Sync$/ }))
    await screen.findByText('Sync Endpoints')

    // The role is revoked: the real provider re-derives isAdmin from the new status.
    await act(async () => {
      queryClient.setQueryData(['auth-status'], status(USER))
    })
    await waitFor(() => expect(screen.queryByRole('button', { name: /^Sync$/ })).not.toBeInTheDocument())

    // Observable output: the sync panel is gone AND a real panel is shown instead.
    expect(screen.queryByText('Sync Endpoints')).not.toBeInTheDocument()
    expect(screen.queryByText('Total Syncs')).not.toBeInTheDocument()
    expect(await screen.findByText('Scan History')).toBeInTheDocument()
  })

  it('R2: non-admin rendered output contains no Sync panel text (NOTE: with the R4 fallback an ungated TabsContent is an equivalent mutant; the R4 test above is the one that kills a removed fallback)', async () => {
    vi.mocked(authApi.getAuthStatus).mockResolvedValue(status(USER) as any)
    mount()
    await screen.findByText('Scan History')
    expect(screen.queryByText('Sync Endpoints')).not.toBeInTheDocument()
    expect(screen.queryByText('Total Syncs')).not.toBeInTheDocument()
  })

  it('non-admin who selects SMB Discovery stays on it (fallback only affects the Sync tab)', async () => {
    vi.mocked(authApi.getAuthStatus).mockResolvedValue(status(USER) as any)
    mount()
    await userEvent.setup().click(await screen.findByRole('button', { name: /SMB Discovery/ }))
    expect(await screen.findByText('Network Discovery')).toBeInTheDocument()
    expect(screen.queryByText('Scan History')).not.toBeInTheDocument()
  })

  it('R3: a role NAMED Admin whose permissions do not grant system.admin is not shown the Sync tab (isAdmin follows the server rule on /auth/status permissions)', async () => {
    vi.mocked(authApi.getAuthStatus).mockResolvedValue(
      status({ id: 9, username: 'impostor', role: { name: 'Admin' }, role_id: 2 }) as any
    )
    mount()
    await screen.findByText('Scan History')
    await new Promise((r) => setTimeout(r, 50))
    expect(screen.queryByRole('button', { name: /^Sync$/ })).not.toBeInTheDocument()
    expect(syncCalls()).toBe(0)
  })
})
