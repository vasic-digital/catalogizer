import React from 'react'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Settings } from '../Settings'

// Mock framer-motion
vi.mock('framer-motion', async () => {
  const MockMotionDiv = ({ children, ...props }: any) => <div {...props}>{children}</div>
  MockMotionDiv.displayName = 'MockMotionDiv'
  return {
    motion: {
      div: MockMotionDiv,
    },
  }
})

// Mock lucide-react icons
vi.mock('lucide-react', async () => {
  const icon = (name: string) => {
    const Component = (props: any) => <svg data-testid={`icon-${name.toLowerCase()}`} {...props} />
    Component.displayName = name
    return Component
  }
  return {
    Scan: icon('scan'),
    Network: icon('network'),
    RefreshCw: icon('refreshcw'),
    Plus: icon('plus'),
    Play: icon('play'),
    Trash2: icon('trash2'),
    CheckCircle: icon('checkcircle'),
    XCircle: icon('xcircle'),
    Clock: icon('clock'),
    Server: icon('server'),
    FolderSearch: icon('foldersearch'),
    Wifi: icon('wifi'),
  }
})

// Mock react-hot-toast
vi.mock('react-hot-toast', async () => ({
  default: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock API modules
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
  mediaApi: {
    getStorageRoots: vi.fn().mockResolvedValue([]),
  },
}))

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

describe('Settings', () => {
  describe('Rendering', () => {
    it('renders the Settings page heading', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('Settings')).toBeInTheDocument()
    })

    it('renders the description text', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(
        screen.getByText('Manage scans, network discovery, and synchronization')
      ).toBeInTheDocument()
    })
  })

  describe('Tabs', () => {
    it('renders Scans tab', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('Scans')).toBeInTheDocument()
    })

    it('renders SMB Discovery tab', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('SMB Discovery')).toBeInTheDocument()
    })

    it('renders Sync tab', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('Sync')).toBeInTheDocument()
    })
  })

  describe('Scans Section (default tab)', () => {
    it('renders Start New Scan card', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('Start New Scan')).toBeInTheDocument()
    })

    it('renders Storage Root select', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByLabelText('Storage Root')).toBeInTheDocument()
    })

    it('renders Start Scan button', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('Start Scan')).toBeInTheDocument()
    })

    it('renders Scan History card', () => {
      render(<Settings />, { wrapper: createWrapper() })

      expect(screen.getByText('Scan History')).toBeInTheDocument()
    })
  })

  describe('Layout', () => {
    it('renders within a max-width container', () => {
      const { container } = render(<Settings />, { wrapper: createWrapper() })

      const wrapper = container.querySelector('.max-w-7xl')
      expect(wrapper).toBeInTheDocument()
    })
  })

  describe('SMB Discovery Section', () => {
    it('renders SMB Discovery tab and can switch to it', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('SMB Discovery'))
      expect(screen.getByText('Network Discovery')).toBeInTheDocument()
      // "Test Connection" appears as both a heading and a button
      const testConnElements = screen.getAllByText('Test Connection')
      expect(testConnElements.length).toBeGreaterThanOrEqual(1)
    })

    it('renders Discover Shares button', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('SMB Discovery'))
      expect(screen.getByText('Discover Shares')).toBeInTheDocument()
    })

    it('renders SMB connection test form fields', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('SMB Discovery'))
      expect(screen.getByPlaceholderText('192.168.1.100')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('media')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('guest')).toBeInTheDocument()
    })

    it('renders Test Connection button (initially disabled)', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('SMB Discovery'))
      // Find the button element specifically (not the card heading)
      const testBtns = screen.getAllByText('Test Connection')
      const testButton = testBtns.find(el => el.closest('button') && !el.closest('[class*="CardTitle"]'))
      expect(testButton?.closest('button')).toBeDisabled()
    })

    it('enables Test Connection button when host and share are filled', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('SMB Discovery'))
      await user.type(screen.getByPlaceholderText('192.168.1.100'), '10.0.0.1')
      await user.type(screen.getByPlaceholderText('media'), 'videos')
      const testBtns = screen.getAllByText('Test Connection')
      const testButton = testBtns.find(el => el.closest('button') && !el.closest('[class*="CardTitle"]'))
      expect(testButton?.closest('button')).not.toBeDisabled()
    })

    it('shows scan description text', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('SMB Discovery'))
      expect(screen.getByText('Scan your network for available SMB shares.')).toBeInTheDocument()
    })
  })

  describe('Sync Section', () => {
    it('renders Sync tab and can switch to it', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      expect(screen.getByText('Sync Endpoints')).toBeInTheDocument()
    })

    it('renders Add Endpoint button', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      expect(screen.getByText('Add Endpoint')).toBeInTheDocument()
    })

    it('shows sync statistics cards', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      await screen.findByText('Total Syncs')
      expect(screen.getByText('Successful')).toBeInTheDocument()
      expect(screen.getByText('Failed')).toBeInTheDocument()
      expect(screen.getByText('Items Synced')).toBeInTheDocument()
    })

    it('shows empty endpoints message when none configured', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      expect(await screen.findByText('No sync endpoints configured.')).toBeInTheDocument()
    })

    it('shows endpoint creation form when Add Endpoint is clicked', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      await user.click(screen.getByText('Add Endpoint'))
      expect(screen.getByPlaceholderText('My Sync Target')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('https://remote-catalogizer.example.com')).toBeInTheDocument()
    })

    it('shows Create and Cancel buttons in endpoint form', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      await user.click(screen.getByText('Add Endpoint'))
      expect(screen.getByText('Create')).toBeInTheDocument()
      expect(screen.getByText('Cancel')).toBeInTheDocument()
    })

    it('Create button disabled when name and url are empty', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      await user.click(screen.getByText('Add Endpoint'))
      const createBtn = screen.getByText('Create')
      expect(createBtn.closest('button')).toBeDisabled()
    })

    it('hides form when Cancel is clicked', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      await user.click(screen.getByText('Add Endpoint'))
      expect(screen.getByPlaceholderText('My Sync Target')).toBeInTheDocument()
      await user.click(screen.getByText('Cancel'))
      expect(screen.queryByPlaceholderText('My Sync Target')).not.toBeInTheDocument()
    })

    it('renders sync stats values from mock data', async () => {
      const user = (await import('@testing-library/user-event')).default.setup()
      render(<Settings />, { wrapper: createWrapper() })
      await user.click(screen.getByText('Sync'))
      await screen.findByText('10')
      expect(screen.getByText('8')).toBeInTheDocument()
      expect(screen.getByText('2')).toBeInTheDocument()
      expect(screen.getByText('500')).toBeInTheDocument()
    })
  })

  describe('Scans Section Interactions', () => {
    it('renders empty scan history message', async () => {
      render(<Settings />, { wrapper: createWrapper() })
      expect(await screen.findByText('No scans recorded yet.')).toBeInTheDocument()
    })

    it('Start Scan button is disabled when no root selected', () => {
      render(<Settings />, { wrapper: createWrapper() })
      const startBtn = screen.getByText('Start Scan')
      expect(startBtn.closest('button')).toBeDisabled()
    })

    it('renders default option in storage root select', () => {
      render(<Settings />, { wrapper: createWrapper() })
      const select = screen.getByLabelText('Storage Root')
      expect(select).toHaveValue('0')
    })
  })
})
