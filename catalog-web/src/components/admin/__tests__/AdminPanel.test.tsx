import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AdminPanel } from '../AdminPanel'

const defaultProps = {
  systemInfo: {
    version: '1.0.0',
    uptime: 86400,
    cpuUsage: 45,
    memoryUsage: 62,
    diskUsage: {
      total: 1073741824000,
      used: 536870912000,
      free: 536870912000,
    },
    activeConnections: 12,
    totalRequests: 15420,
  },
  users: [
    {
      id: '1',
      username: 'admin',
      email: 'admin@test.com',
      role: 'admin' as const,
      status: 'active' as const,
      lastLogin: '2024-01-01T00:00:00Z',
      createdAt: '2023-01-01T00:00:00Z',
    },
    {
      id: '2',
      username: 'user1',
      email: 'user1@test.com',
      role: 'user' as const,
      status: 'active' as const,
      createdAt: '2023-06-01T00:00:00Z',
    },
  ],
  storageInfo: [
    {
      path: '/media/movies',
      totalSpace: 1073741824000,
      usedSpace: 536870912000,
      availableSpace: 536870912000,
      mediaCount: 1250,
      lastScan: '2024-01-01T00:00:00Z',
    },
  ],
  backups: [
    {
      id: '1',
      filename: 'backup-20240101.tar.gz',
      size: 1073741824,
      createdAt: '2024-01-01T00:00:00Z',
      type: 'full' as const,
      status: 'completed' as const,
    },
    {
      id: '2',
      filename: 'backup-20240102.tar.gz',
      size: 268435456,
      createdAt: '2024-01-02T00:00:00Z',
      type: 'incremental' as const,
      status: 'in-progress' as const,
    },
  ],
}

describe('AdminPanel', () => {
  it('renders Admin Panel heading', () => {
    render(<AdminPanel {...defaultProps} />)
    expect(screen.getByText('Admin Panel')).toBeInTheDocument()
  })

  it('renders navigation tabs', () => {
    render(<AdminPanel {...defaultProps} />)
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Users')).toBeInTheDocument()
    expect(screen.getByText('Storage')).toBeInTheDocument()
    expect(screen.getByText('Backups')).toBeInTheDocument()
  })

  it('shows overview tab by default', () => {
    render(<AdminPanel {...defaultProps} />)
    expect(screen.getByText('1.0.0')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('15,420')).toBeInTheDocument()
  })

  it('displays system version', () => {
    render(<AdminPanel {...defaultProps} />)
    expect(screen.getByText('1.0.0')).toBeInTheDocument()
  })

  it('displays formatted uptime', () => {
    render(<AdminPanel {...defaultProps} />)
    expect(screen.getByText('1d 0h 0m')).toBeInTheDocument()
  })

  it('shows CPU and memory usage', () => {
    render(<AdminPanel {...defaultProps} />)
    expect(screen.getByText('45%')).toBeInTheDocument()
    expect(screen.getByText('62%')).toBeInTheDocument()
  })

  it('switches to users tab on click', async () => {
    const user = userEvent.setup()
    render(<AdminPanel {...defaultProps} />)

    await user.click(screen.getByText('Users'))

    expect(screen.getByText('User Management')).toBeInTheDocument()
    expect(screen.getByText('admin')).toBeInTheDocument()
    expect(screen.getByText('user1')).toBeInTheDocument()
  })

  it('shows user emails in users tab', async () => {
    const user = userEvent.setup()
    render(<AdminPanel {...defaultProps} />)

    await user.click(screen.getByText('Users'))

    expect(screen.getByText('admin@test.com')).toBeInTheDocument()
    expect(screen.getByText('user1@test.com')).toBeInTheDocument()
  })

  it('switches to storage tab on click', async () => {
    const user = userEvent.setup()
    render(<AdminPanel {...defaultProps} />)

    await user.click(screen.getByText('Storage'))

    expect(screen.getByText('/media/movies')).toBeInTheDocument()
    expect(screen.getByText('1,250')).toBeInTheDocument()
  })

  it('switches to backups tab on click', async () => {
    const user = userEvent.setup()
    render(<AdminPanel {...defaultProps} />)

    await user.click(screen.getByText('Backups'))

    expect(screen.getByText('Backup Management')).toBeInTheDocument()
    expect(screen.getByText('backup-20240101.tar.gz')).toBeInTheDocument()
  })

  it('calls onCreateBackup when full backup button is clicked', async () => {
    const user = userEvent.setup()
    const onCreateBackup = vi.fn()
    render(<AdminPanel {...defaultProps} onCreateBackup={onCreateBackup} />)

    await user.click(screen.getByText('Create Full Backup'))

    expect(onCreateBackup).toHaveBeenCalledWith('full')
  })

  it('calls onCreateBackup for incremental backup', async () => {
    const user = userEvent.setup()
    const onCreateBackup = vi.fn()
    render(<AdminPanel {...defaultProps} onCreateBackup={onCreateBackup} />)

    await user.click(screen.getByText('Create Incremental Backup'))

    expect(onCreateBackup).toHaveBeenCalledWith('incremental')
  })

  it('calls onScanStorage when scan button is clicked in storage tab', async () => {
    const user = userEvent.setup()
    const onScanStorage = vi.fn()
    render(<AdminPanel {...defaultProps} onScanStorage={onScanStorage} />)

    await user.click(screen.getByText('Storage'))
    await user.click(screen.getByText('Scan'))

    expect(onScanStorage).toHaveBeenCalledWith('/media/movies')
  })

  it('disables restore button for in-progress backups', async () => {
    const user = userEvent.setup()
    render(<AdminPanel {...defaultProps} />)

    await user.click(screen.getByText('Backups'))

    const restoreButtons = screen.getAllByText('Restore')
    // Second backup is in-progress, so its restore button should be disabled
    expect(restoreButtons[1].closest('button')).toBeDisabled()
  })

  describe('unmeasured figures render as Unavailable (CAT-001)', () => {
    it('does not crash and shows Unavailable for null CPU, disk and request count', () => {
      const props = {
        ...defaultProps,
        systemInfo: {
          ...defaultProps.systemInfo,
          cpuUsage: null,
          diskUsage: null,
          totalRequests: null,
          unavailable: ['cpuUsage', 'diskUsage', 'totalRequests'],
        },
      }
      render(<AdminPanel {...props} />)
      expect(screen.getAllByText('Unavailable').length).toBeGreaterThanOrEqual(3)
      expect(screen.queryByText('0%')).not.toBeInTheDocument()
    })

    it('shows Unavailable (not 0%, 0 or 0d 0h 0m) for null memory, uptime and connections (review F4)', () => {
      const props = {
        ...defaultProps,
        systemInfo: {
          ...defaultProps.systemInfo,
          cpuUsage: null,
          memoryUsage: null,
          uptime: null,
          activeConnections: null,
          diskUsage: null,
          totalRequests: null,
        },
      }
      render(<AdminPanel {...props} />)
      expect(screen.queryByText('0%')).not.toBeInTheDocument()
      expect(screen.queryByText('0d 0h 0m')).not.toBeInTheDocument()
      expect(screen.queryByText('0')).not.toBeInTheDocument()
      expect(screen.getAllByText('Unavailable').length).toBeGreaterThanOrEqual(6)
    })

    it('shows Unavailable for null storage capacity instead of 0 B', async () => {
      const user = userEvent.setup()
      const props = {
        ...defaultProps,
        storageInfo: [{ ...defaultProps.storageInfo[0], totalSpace: null, usedSpace: null, availableSpace: null }],
      }
      render(<AdminPanel {...props} />)
      await user.click(screen.getByText('Storage'))
      expect(screen.getAllByText('Unavailable').length).toBeGreaterThanOrEqual(3)
    })
  })
})

// Responsive contract (11.4.190): four 44px-high tab triggers do not fit a 360px screen, so the strip must scroll
// instead of overflowing the document, and the triggers must neither shrink nor wrap.
describe('AdminPanel - responsive tab strip', () => {
  it('scrolls horizontally and keeps triggers intact', () => {
    render(<AdminPanel {...defaultProps} />)
    const nav = screen.getByRole('navigation', { name: 'Admin sections' })
    expect(nav.className).toContain('overflow-x-auto')
    const triggers = Array.from(nav.querySelectorAll('button'))
    expect(triggers.map((b) => b.textContent?.trim())).toEqual(['Overview', 'Users', 'Storage', 'Backups'])
    for (const b of triggers) {
      expect(b.className).toContain('flex-shrink-0')
      expect(b.className).toContain('whitespace-nowrap')
    }
  })
})
