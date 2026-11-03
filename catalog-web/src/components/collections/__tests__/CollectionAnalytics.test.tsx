import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CollectionAnalytics } from '../CollectionAnalytics'

vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className }: any) => <div className={className}>{children}</div>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

const items = [
  { id: 'a', title: 'Alpha', media_type: 'video', size: 2048, rating: 8, duration: 3600 },
  { id: 'b', title: 'Beta', media_type: 'music', size: 1024, rating: 6, duration: 60 },
  { id: 'c', title: 'Gamma', media_type: 'video', size: 0 },
]

vi.mock('../../../hooks/useCollections', () => ({
  useCollection: () => ({ collectionItems: { items }, isLoading: false }),
}))

const collection = { id: '1', name: 'Mine' } as any

describe('CollectionAnalytics (CAT-001: no fabricated analytics)', () => {
  it('shows only figures derived from the real items', () => {
    render(<CollectionAnalytics collection={collection} />)
    expect(screen.getByText('3')).toBeInTheDocument() // total items
    expect(screen.getByText('3 KB')).toBeInTheDocument() // 2048 + 1024 bytes
    expect(screen.getByText('7.0')).toBeInTheDocument() // mean of the two rated items
    expect(screen.getByText('Alpha')).toBeInTheDocument()
  })

  it('renders identical output twice (nothing random)', () => {
    const first = render(<CollectionAnalytics collection={collection} />).container.innerHTML
    const second = render(<CollectionAnalytics collection={collection} />).container.innerHTML
    expect(second).toBe(first)
  })

  it('shows Unavailable for plays, downloads, shares and views instead of numbers or trend percentages', () => {
    render(<CollectionAnalytics collection={collection} />)
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThanOrEqual(4)
    expect(screen.queryByText(/vs last period/)).not.toBeInTheDocument()
  })

  it.each(['Activity', 'Sharing', 'Engagement'])('the %s tab says the statistics are not recorded', async (tab) => {
    const user = userEvent.setup()
    render(<CollectionAnalytics collection={collection} />)
    await user.click(screen.getByText(tab))
    expect(screen.getByTestId('usage-unavailable')).toBeInTheDocument()
  })
})
