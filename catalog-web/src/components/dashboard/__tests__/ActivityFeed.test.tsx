import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActivityFeed } from '../ActivityFeed'

// Mock date-fns to get deterministic output
vi.mock('date-fns', () => ({
  formatDistanceToNow: vi.fn(() => '5 minutes ago'),
}))

const activities = [
  { id: '1', type: 'media_played' as const, title: 'Real Movie.mp4', user: 'alice', timestamp: new Date(), metadata: { title: 'Real Movie.mp4', quality: '1080p', duration: '1h' } },
  { id: '2', type: 'media_uploaded' as const, title: 'Real Upload.mkv', user: 'bob', timestamp: new Date(), metadata: { title: 'Real Upload.mkv', size: '1GB', format: 'MKV' } },
  { id: '3', type: 'system_event' as const, title: 'Scan', user: 'System', timestamp: new Date(), metadata: { message: 'Scan done' } },
]

describe('ActivityFeed (CAT-001: no sample activity)', () => {
  it('without supplied data shows an unavailable state and no invented people or titles', () => {
    render(<ActivityFeed />)
    expect(screen.getByText('Recent Activity')).toBeInTheDocument()
    expect(screen.getByTestId('activity-unavailable')).toBeInTheDocument()
    for (const invented of ['John Doe', 'Jane Smith', 'Mike Johnson', 'Sample Movie', 'New Episode']) {
      expect(screen.queryByText(new RegExp(invented))).not.toBeInTheDocument()
    }
  })

  it('renders the supplied activities', () => {
    render(<ActivityFeed activities={activities} />)
    expect(screen.getByText(/Real Movie/)).toBeInTheDocument()
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getAllByText('5 minutes ago').length).toBe(3)
  })

  it('an empty supplied list is the honest empty state, not the unavailable state', () => {
    render(<ActivityFeed activities={[]} />)
    expect(screen.getByText(/No recent activity/)).toBeInTheDocument()
    expect(screen.queryByTestId('activity-unavailable')).not.toBeInTheDocument()
  })

  it('shows filter buttons when showFilters is true and hides them when false', () => {
    const { unmount } = render(<ActivityFeed activities={activities} showFilters />)
    expect(screen.getByText('Playing')).toBeInTheDocument()
    expect(screen.getByText('Uploads')).toBeInTheDocument()
    unmount()
    render(<ActivityFeed activities={activities} showFilters={false} />)
    expect(screen.queryByText('Playing')).not.toBeInTheDocument()
  })

  it('filters activities when a filter is clicked', async () => {
    const user = userEvent.setup()
    render(<ActivityFeed activities={activities} showFilters />)
    await user.click(screen.getByText('Uploads'))
    expect(screen.getByText(/Real Upload/)).toBeInTheDocument()
    expect(screen.queryByText(/Real Movie/)).not.toBeInTheDocument()
  })

  it('respects the limit prop', () => {
    render(<ActivityFeed activities={activities} limit={1} showFilters={false} />)
    expect(screen.getByText(/Real Movie/)).toBeInTheDocument()
    expect(screen.queryByText(/Real Upload/)).not.toBeInTheDocument()
  })
})
