import { render, screen } from '@testing-library/react'
import { DashboardStats, StatCard } from '../DashboardStats'
import { Database } from 'lucide-react'

describe('StatCard', () => {
  it('renders title and value', () => {
    render(<StatCard title="Test Stat" value="42" icon={Database} />)
    expect(screen.getByText('Test Stat')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('renders with numeric value', () => {
    render(<StatCard title="Count" value={100} icon={Database} />)
    expect(screen.getByText('100')).toBeInTheDocument()
  })

  it('shows loading skeleton when loading is true', () => {
    render(<StatCard title="Loading Stat" value="42" icon={Database} loading />)
    expect(screen.getByText('Loading Stat')).toBeInTheDocument()
    // Value should not be visible, but loading skeleton should be present
    expect(screen.queryByText('42')).not.toBeInTheDocument()
  })

  it('renders positive trend indicator', () => {
    render(
      <StatCard
        title="Trend"
        value="42"
        icon={Database}
        trend={{ value: 12.5, isPositive: true }}
      />
    )
    expect(screen.getByText('12.5%')).toBeInTheDocument()
  })

  it('renders negative trend indicator', () => {
    render(
      <StatCard
        title="Trend"
        value="42"
        icon={Database}
        trend={{ value: 5.3, isPositive: false }}
      />
    )
    expect(screen.getByText('5.3%')).toBeInTheDocument()
  })

  it('renders description text', () => {
    render(
      <StatCard title="Stat" value="42" icon={Database} description="vs last month" />
    )
    expect(screen.getByText('vs last month')).toBeInTheDocument()
  })
})

describe('DashboardStats', () => {
  const mediaStats = {
    total_items: 1500,
    total_size: 500000000000,
    recent_additions: 25,
    by_type: { movie: 500, tv_show: 300, music: 700 },
    by_quality: { '1080p': 800, '720p': 500, '4K': 200 },
  }

  const userStats = {
    active_users: 5,
    total_users: 20,
    sessions_today: 35,
    avg_session_duration: 90,
  }

  it('renders all stat cards', () => {
    render(<DashboardStats mediaStats={mediaStats} userStats={userStats} />)

    expect(screen.getByText('Total Media')).toBeInTheDocument()
    expect(screen.getByText('Storage Used')).toBeInTheDocument()
    expect(screen.getByText('Recent Additions')).toBeInTheDocument()
    expect(screen.getByText('Quality Score')).toBeInTheDocument()
    expect(screen.getByText('Active Users')).toBeInTheDocument()
    expect(screen.getByText('Total Users')).toBeInTheDocument()
    expect(screen.getByText('Sessions Today')).toBeInTheDocument()
    expect(screen.getByText('Avg Session')).toBeInTheDocument()
  })

  it('displays formatted media stats', () => {
    render(<DashboardStats mediaStats={mediaStats} userStats={userStats} />)
    expect(screen.getByText('1,500')).toBeInTheDocument()
    expect(screen.getByText('25')).toBeInTheDocument()
  })

  it('displays formatted user stats', () => {
    render(<DashboardStats mediaStats={mediaStats} userStats={userStats} />)
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.getByText('20')).toBeInTheDocument()
    expect(screen.getByText('35')).toBeInTheDocument()
  })

  it('displays formatted session duration', () => {
    render(<DashboardStats mediaStats={mediaStats} userStats={userStats} />)
    expect(screen.getByText('1h 30m')).toBeInTheDocument()
  })

  it('shows 0 values when stats are undefined', () => {
    render(<DashboardStats />)
    expect(screen.getByText('Total Media')).toBeInTheDocument()
    expect(screen.getAllByText('0').length).toBeGreaterThan(0)
  })

  it('shows loading state', () => {
    render(<DashboardStats loading />)
    expect(screen.getByText('Total Media')).toBeInTheDocument()
    // In loading state, values should not be shown
  })

  // Regression for TICKET-008-web-dashboard-error-after-login:
  // Dashboard threw "Something went wrong" when the backend returned a
  // partial MediaStats payload missing total_items / total_size / etc.
  // The bug was unsafe `.toLocaleString()` calls on possibly-undefined
  // numeric fields. This test feeds partial payloads and asserts the
  // component renders without throwing.
  it('renders cleanly with partial mediaStats missing total_items', () => {
    const partial = {
      by_type: { movie: 5 },
      by_quality: { '1080p': 5 },
    } as Parameters<typeof DashboardStats>[0]['mediaStats']
    expect(() =>
      render(<DashboardStats mediaStats={partial} userStats={userStats} />),
    ).not.toThrow()
    // And it shows the 0 fallback instead of crashing
    expect(screen.getByText('Total Media')).toBeInTheDocument()
  })

  it('renders cleanly with partial userStats missing active_users', () => {
    const partial = {
      total_users: 10,
    } as Parameters<typeof DashboardStats>[0]['userStats']
    expect(() =>
      render(<DashboardStats mediaStats={mediaStats} userStats={partial} />),
    ).not.toThrow()
    expect(screen.getByText('Active Users')).toBeInTheDocument()
  })

  it('renders cleanly with fully empty mediaStats + userStats objects', () => {
    expect(() =>
      render(
        <DashboardStats
          mediaStats={{} as NonNullable<Parameters<typeof DashboardStats>[0]['mediaStats']>}
          userStats={{} as NonNullable<Parameters<typeof DashboardStats>[0]['userStats']>}
        />,
      ),
    ).not.toThrow()
    // Multiple "0" fallbacks are expected
    expect(screen.getAllByText('0').length).toBeGreaterThan(0)
  })
})

describe('DashboardStats without user statistics (CAT-001)', () => {
  it('shows Unavailable rather than zeros when no userStats are supplied', () => {
    render(<DashboardStats mediaStats={{ total_items: 3 } as any} />)
    expect(screen.getAllByText('Unavailable').length).toBe(4)
  })
})
