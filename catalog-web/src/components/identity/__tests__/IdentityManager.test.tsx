/**
 * Smoke tests for the Identity Manager + Discovered Shares page.
 *
 * Verifies each component renders without crashing. Axe accessibility
 * scanning is not wired (the mock needs too many icon imports); WCAG
 * contrast is tested separately in identity_contrast.test.ts.
 */

import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Mock lucide-react icons used by identity components
vi.mock('lucide-react', async () => {
  const icon = ({ className }: { className?: string }) => (
    <svg className={className} data-testid="mock-icon" />
  )
  return {
    KeyRound: icon,
    Plus: icon,
    Trash2: icon,
    Pencil: icon,
    Check: icon,
    X: icon,
    Lock: icon,
    User: icon,
    Server: icon,
    Terminal: icon,
    Globe: icon,
    Fingerprint: icon,
    ChevronDown: icon,
    ArrowUpDown: icon,
    Network: icon,
    Scan: icon,
    FolderOpen: icon,
    Wifi: icon,
    RefreshCw: icon,
    Search: icon,
    Radio: icon,
    HardDrive: icon,
    Monitor: icon,
    ShieldCheck: icon,
    ShieldOff: icon,
    ShieldAlert: icon,
    HelpCircle: icon,
  }
})

// Mock the identitiesApi module
vi.mock('@/lib/identitiesApi', () => ({
  identitiesApi: {
    list: vi.fn().mockResolvedValue([]),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    scanNetwork: vi.fn(),
    listHosts: vi.fn().mockResolvedValue([]),
    listShares: vi.fn().mockResolvedValue([]),
    probeShare: vi.fn(),
    listBindings: vi.fn(),
  },
}))

import { IdentityManager } from '@/components/identity/IdentityManager'
import { DiscoveredShares } from '@/components/identity/DiscoveredShares'

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  Wrapper.displayName = 'TestWrapper'
  return Wrapper
}

describe('IdentityManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the Identities card header', () => {
    render(<IdentityManager />, { wrapper: createWrapper() })
    expect(screen.getByText('Identities')).toBeTruthy()
  })

  it('renders an Add Identity button', () => {
    render(<IdentityManager />, { wrapper: createWrapper() })
    expect(screen.getByText('Add Identity')).toBeTruthy()
  })

  it('shows empty state when no identities exist', async () => {
    const { identitiesApi } = await import('@/lib/identitiesApi')
    vi.mocked(identitiesApi.list).mockResolvedValue([])

    render(<IdentityManager />, { wrapper: createWrapper() })

    // Wait for the empty state text to appear after the query finishes
    const emptyText = await screen.findByText(
      /No identities configured/i
    )
    expect(emptyText).toBeTruthy()
  })
})

describe('DiscoveredShares', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the Discovered Shares card header', () => {
    render(<DiscoveredShares />, { wrapper: createWrapper() })
    expect(screen.getByText('Discovered Shares')).toBeTruthy()
  })

  it('renders a Scan Network button', () => {
    render(<DiscoveredShares />, { wrapper: createWrapper() })
    expect(screen.getByText('Scan Network')).toBeTruthy()
  })

  it('shows empty state when no shares exist', async () => {
    const { identitiesApi } = await import('@/lib/identitiesApi')
    vi.mocked(identitiesApi.listHosts).mockResolvedValue([])
    vi.mocked(identitiesApi.listShares).mockResolvedValue([])

    render(<DiscoveredShares />, { wrapper: createWrapper() })

    const emptyText = await screen.findByText(/No shares discovered/i)
    expect(emptyText).toBeTruthy()
  })


describe('DiscoveredShares keyboard operability (S1082)', () => {
  it.each([['{Enter}'], [' ']])('toggles a host group via key %j', async (key) => {
    const { identitiesApi } = await import('@/lib/identitiesApi')
    vi.mocked(identitiesApi.listHosts).mockResolvedValue([
      { id: 1, ip: '10.0.0.5', hostname: 'nas', first_seen: 'a', last_seen: 'b', reachable: true, ifaces_json: null },
    ] as never)
    vi.mocked(identitiesApi.listShares).mockResolvedValue([
      { id: 9, host_id: 1, host_ip: '10.0.0.5', host_hostname: 'nas', protocol: 'smb', share_name: 'media', port: 445, first_seen: 'a', last_seen: 'b', enumeration_evidence_path: null },
    ] as never)
    const user = userEvent.setup()
    render(<DiscoveredShares />, { wrapper: createWrapper() })
    const header = (await screen.findByText('nas')).closest('[role="button"]') as HTMLElement
    expect(header).toHaveAttribute('aria-expanded', 'true')
    header.focus()
    await user.keyboard(key)
    expect(header).toHaveAttribute('aria-expanded', 'false')
  })
})
})
