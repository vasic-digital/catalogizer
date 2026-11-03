import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'react-hot-toast'

// Mock framer-motion
vi.mock('framer-motion', () => {
  const MockDiv = ({ children, ...props }: any) => <div {...props}>{children}</div>
  MockDiv.displayName = 'MockDiv'
  const MockButton = ({ children, ...props }: any) => <button {...props}>{children}</button>
  MockButton.displayName = 'MockButton'
  return {
    motion: { div: MockDiv, button: MockButton },
    AnimatePresence: ({ children }: any) => <>{children}</>,
  }
})

// Mock react-hot-toast
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

// Mock lucide-react icons
vi.mock('lucide-react', () => {
  const icon = (name: string) => {
    const Comp = (props: any) => <svg data-testid={`icon-${name}`} {...props} />
    Comp.displayName = name
    return Comp
  }
  return {
    Globe: icon('Globe'), Cloud: icon('Cloud'), Plus: icon('Plus'),
    Trash2: icon('Trash2'), Edit: icon('Edit'), CheckCircle: icon('CheckCircle'),
    AlertCircle: icon('AlertCircle'), XCircle: icon('XCircle'), Clock: icon('Clock'),
    RefreshCw: icon('RefreshCw'), Link: icon('Link'), Key: icon('Key'),
    Zap: icon('Zap'), Database: icon('Database'), FolderSync: icon('FolderSync'),
    Share2: icon('Share2'), Info: icon('Info'), ExternalLink: icon('ExternalLink'),
    TestTube: icon('TestTube'), Activity: icon('Activity'),
  }
})

// Mock fetch for integration loading
global.fetch = vi.fn().mockResolvedValue({
  ok: false,
  json: async () => ([]),
})

import ExternalIntegrations from '../ExternalIntegrations'

describe('ExternalIntegrations', () => {
  it('renders the heading', () => {
    render(<ExternalIntegrations />)
    expect(screen.getByText('External Integrations')).toBeInTheDocument()
  })

  it('renders the subtitle', () => {
    render(<ExternalIntegrations />)
    expect(screen.getByText('Connect with external services to extend functionality')).toBeInTheDocument()
  })

  it('shows Add Integration button', () => {
    render(<ExternalIntegrations />)
    expect(screen.getByText('Add Integration')).toBeInTheDocument()
  })

  it('renders stats cards', () => {
    render(<ExternalIntegrations />)
    expect(screen.getByText('Total Integrations')).toBeInTheDocument()
  })

  it('renders search input', () => {
    render(<ExternalIntegrations />)
    expect(screen.getByPlaceholderText('Search integrations...')).toBeInTheDocument()
  })

  it('renders filter buttons', () => {
    render(<ExternalIntegrations />)
    expect(screen.getAllByText('All').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Connected').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Disconnected').length).toBeGreaterThan(0)
  })

  it('shows empty state when no integrations', async () => {
    render(<ExternalIntegrations />)
    expect(await screen.findByText('No integrations found')).toBeInTheDocument()
  })

  it('renders without crashing', () => {
    const { container } = render(<ExternalIntegrations />)
    expect(container.firstChild).toBeTruthy()
  })

  describe('CAT-001: no success claims the server did not make', () => {
    const item = { id: 'i1', name: 'My Plex', provider: 'Plex', type: 'sharing', status: 'connected', description: 'd', enabled: true }
    const mockFetch = (postStatus: number) => {
      ;(global.fetch as any) = vi.fn(async (url: string, init?: RequestInit) => {
        const method = init?.method || 'GET'
        if (method === 'GET') return { ok: true, status: 200, json: async () => [item] }
        return { ok: false, status: postStatus, json: async () => ({}) }
      })
    }

    beforeEach(() => { vi.mocked(toast.success).mockClear(); vi.mocked(toast.error).mockClear() })

    it('a 404 connection test is an error toast, not "completed"', async () => {
      mockFetch(404)
      const user = userEvent.setup()
      render(<ExternalIntegrations />)
      await screen.findByText('My Plex')
      await user.click(screen.getByTestId('icon-TestTube').closest('button')!)
      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('not tested')))
      expect(toast.success).not.toHaveBeenCalled()
    })

    it('a failed sync is not counted or reported as success', async () => {
      mockFetch(404)
      const user = userEvent.setup()
      render(<ExternalIntegrations />)
      await screen.findByText('My Plex')
      await user.click(screen.getByTestId('icon-RefreshCw').closest('button')!)
      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Nothing was synced')))
      expect(toast.success).not.toHaveBeenCalledWith(expect.stringMatching(/Sync completed/))
    })
  })
})
