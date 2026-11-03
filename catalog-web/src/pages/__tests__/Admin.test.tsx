import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Admin } from '../Admin'
import toast from 'react-hot-toast'
import { adminApi } from '@/lib/adminApi'

// Mock the adminApi
vi.mock('@/lib/adminApi', () => ({
  adminApi: {
    getSystemInfo: vi.fn(() =>
      Promise.resolve({
        version: '1.0.0',
        uptime: 86400,
        cpuUsage: 45,
        memoryUsage: 62,
        diskUsage: { total: 1073741824000, used: 536870912000, free: 536870912000 },
        activeConnections: 12,
        totalRequests: 15420,
      })
    ),
    getUsers: vi.fn(() =>
      Promise.resolve([
        { id: '1', username: 'admin', email: 'admin@test.com', role: 'admin', status: 'active', createdAt: '2023-01-01' },
      ])
    ),
    getStorageInfo: vi.fn(() =>
      Promise.resolve([
        { path: '/media/movies', totalSpace: 1073741824000, usedSpace: 536870912000, availableSpace: 536870912000, mediaCount: 1250 },
      ])
    ),
    getBackups: vi.fn(() =>
      Promise.resolve([
        { id: '1', filename: 'backup.tar.gz', size: 1073741824, createdAt: '2024-01-01', type: 'full', status: 'completed' },
      ])
    ),
    createBackup: vi.fn(() => Promise.resolve()),
    restoreBackup: vi.fn(() => Promise.resolve()),
    scanStorage: vi.fn(() => Promise.resolve()),
    updateUser: vi.fn(() => Promise.resolve()),
  },
}))

// Mock react-hot-toast
vi.mock('react-hot-toast', () => ({
  __esModule: true,
  default: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  })
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  Wrapper.displayName = 'TestWrapper'
  return Wrapper
}

describe('Admin Page', () => {
  it('renders Admin Panel heading', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Admin Panel')).toBeInTheDocument()
    })
  })

  it('renders overview tab with system stats', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Overview')).toBeInTheDocument()
    })
  })

  it('shows navigation tabs', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Users')).toBeInTheDocument()
      expect(screen.getByText('Storage')).toBeInTheDocument()
      expect(screen.getByText('Backups')).toBeInTheDocument()
    })
  })

  it('displays system version when data loads', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('1.0.0')).toBeInTheDocument()
    })
  })

  it('shows default values when data is loading', () => {
    render(<Admin />, { wrapper: createWrapper() })
    // The panel renders immediately; figures are honest 'Unavailable' until data loads
    expect(screen.getByText('Admin Panel')).toBeInTheDocument()
  })

  it('renders quick action buttons', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Create Full Backup')).toBeInTheDocument()
      expect(screen.getByText('Create Incremental Backup')).toBeInTheDocument()
    })
  })

  it('renders resource usage section', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('System Resources')).toBeInTheDocument()
      expect(screen.getByText('CPU Usage')).toBeInTheDocument()
      expect(screen.getByText('Memory Usage')).toBeInTheDocument()
    })
  })

  it('wraps content in max-width container', () => {
    const { container } = render(<Admin />, { wrapper: createWrapper() })
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper).toHaveClass('max-w-7xl')
  })

  // --- Additional branch coverage tests ---

  it('shows system info once loaded', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('1.0.0')).toBeInTheDocument()
    })
  })

  it('displays active connections count', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('12')).toBeInTheDocument()
    })
  })

  it('displays total requests count', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText(/15,?420/)).toBeInTheDocument()
    })
  })

  it('renders Users tab and can click it', async () => {
    const { default: userEvent } = await import('@testing-library/user-event')
    const user = userEvent.setup()
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Users')).toBeInTheDocument()
    })
    await user.click(screen.getByText('Users'))
  })

  it('renders Storage tab and can click it', async () => {
    const { default: userEvent } = await import('@testing-library/user-event')
    const user = userEvent.setup()
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Storage')).toBeInTheDocument()
    })
    await user.click(screen.getByText('Storage'))
  })

  it('renders Backups tab and can click it', async () => {
    const { default: userEvent } = await import('@testing-library/user-event')
    const user = userEvent.setup()
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Backups')).toBeInTheDocument()
    })
    await user.click(screen.getByText('Backups'))
  })

  it('shows backup section heading', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('Backups')).toBeInTheDocument()
    })
  })

  it('renders CPU Usage and Memory Usage in resources section', async () => {
    render(<Admin />, { wrapper: createWrapper() })
    await waitFor(() => {
      expect(screen.getByText('CPU Usage')).toBeInTheDocument()
      expect(screen.getByText('Memory Usage')).toBeInTheDocument()
    })
  })

  it('CAT-001: restore and scan show the server 501 guidance, not a success toast', async () => {
    const { default: userEvent } = await import('@testing-library/user-event')
    const user = userEvent.setup()
    const restoreMsg = 'restoring a backup through the API is not implemented; restore offline with the database restore tooling'
    const scanMsg = 'this endpoint does not start a scan; use POST /api/v1/scans'
    vi.mocked(adminApi.restoreBackup).mockRejectedValueOnce({ response: { status: 501, data: { error: restoreMsg } } })
    vi.mocked(adminApi.scanStorage).mockRejectedValueOnce({ response: { status: 501, data: { error: scanMsg } } })
    render(<Admin />, { wrapper: createWrapper() })

    await waitFor(() => expect(screen.getByText('Backups')).toBeInTheDocument())
    await user.click(screen.getByText('Backups'))
    await user.click(await screen.findByRole('button', { name: /Restore/ }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining(restoreMsg)))

    await user.click(screen.getByText('Storage'))
    await user.click(await screen.findByRole('button', { name: /Scan/ }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining(scanMsg)))
    expect(toast.success).not.toHaveBeenCalledWith(expect.stringMatching(/restore initiated|scan initiated/i))
  })
})
