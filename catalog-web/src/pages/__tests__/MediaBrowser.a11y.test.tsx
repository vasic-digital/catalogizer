import React from 'react'
import { render, screen } from '@testing-library/react'
import axe from 'axe-core'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MediaBrowser } from '../MediaBrowser'
import { mediaApi } from '@/lib/mediaApi'

// Only the network layer is replaced; real Button/Card/Input/MediaGrid are rendered so the
// real accessible names are what the assertions see (CAT-UI-D5D6).
vi.mock('@/lib/mediaApi', async () => ({
  mediaApi: {
    searchMedia: vi.fn(),
    getMediaStats: vi.fn(),
    downloadMedia: vi.fn(),
    getRecentMedia: vi.fn(),
    getPopularMedia: vi.fn(),
  },
}))
vi.mock('@/hooks/useDebounce', async () => ({ useDebounce: vi.fn((v: unknown) => v) }))

const renderPage = () =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MediaBrowser />
    </QueryClientProvider>
  )

describe('MediaBrowser icon-only controls have accessible names (CAT-UI-D5D6)', () => {
  beforeEach(() => {
    vi.mocked(mediaApi.searchMedia).mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 } as any)
    vi.mocked(mediaApi.getMediaStats).mockResolvedValue({ total_items: 100, total_size: 1024, by_type: { movie: 100 }, by_quality: {}, recent_additions: 1 } as any)
  })

  it.each([
    ['grid-view-button', /^grid view$/i],
    ['list-view-button', /^list view$/i],
    ['upload-button', /upload/i],
    ['refresh-button', /refresh/i],
  ])('%s is reachable by role+name', (testId, name) => {
    renderPage()
    expect(screen.getByRole('button', { name })).toBe(screen.getByTestId(testId))
  })

  it('names the pagination buttons when there is more than one page', async () => {
    vi.mocked(mediaApi.searchMedia).mockResolvedValue({ items: [], total: 100, limit: 20, offset: 0 } as any)
    renderPage()
    const prev = await screen.findByRole('button', { name: 'Previous page' })
    const next = screen.getByRole('button', { name: 'Next page' })
    expect(prev).toBe(screen.getByTestId('prev-page-button'))
    expect(next).toBe(screen.getByTestId('next-page-button'))
  })

  it('has no axe button-name violation', async () => {
    const { container } = renderPage()
    const r = await axe.run(container, { runOnly: { type: 'rule', values: ['button-name'] } })
    expect(r.violations.map(v => `${v.id}:${v.nodes.length}`)).toEqual([])
  })
})
