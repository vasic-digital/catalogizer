import { render, screen, act } from '@testing-library/react'
import { CollectionRealTime } from '../CollectionRealTime'

vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, onClick }: any) => (
      <div className={className} onClick={onClick}>{children}</div>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))
vi.mock('react-hot-toast', () => ({ toast: vi.fn(), default: vi.fn() }))

const collection = {
  id: '1', name: 'Test Collection', description: '', is_smart: false, smart_rules: [],
  item_count: 5, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z',
  is_public: false, primary_media_type: 'music', owner_id: 'user1',
}

describe('CollectionRealTime (CAT-001: no simulated users or events)', () => {
  beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
  afterEach(() => { vi.useRealTimers() })

  it('says real-time is unavailable instead of faking a connection', () => {
    render(<CollectionRealTime collection={collection as any} />)
    expect(screen.getByTestId('realtime-unavailable')).toBeInTheDocument()
    expect(screen.queryByText('Connected')).not.toBeInTheDocument()
  })

  it('never shows invented users or events, even after time passes', () => {
    render(<CollectionRealTime collection={collection as any} />)
    act(() => { vi.advanceTimersByTime(60000) })
    for (const invented of [/John Doe/, /Jane Smith/, /Bob Wilson/, /Summer Mix 2024/, /Presentation Video/]) {
      expect(screen.queryByText(invented)).not.toBeInTheDocument()
    }
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThan(0)
  })

  // Characterization (CAT-QA-SONAR-2 stage 2): pins the rendered status/controls before the complexity refactor.
  it('renders the error status badge, four Unavailable stats, Connect and Enable controls', () => {
    render(<CollectionRealTime collection={collection as any} />)
    expect(screen.getByText('Error')).toBeInTheDocument()
    expect(screen.getAllByText('Unavailable')).toHaveLength(4)
    expect(screen.getByRole('button', { name: /^Connect$/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Enable' })).toBeInTheDocument()
    expect(screen.getByText('Online Users (0)')).toBeInTheDocument()
    expect(screen.queryByText(/Last connected/)).not.toBeInTheDocument()
  })
})
