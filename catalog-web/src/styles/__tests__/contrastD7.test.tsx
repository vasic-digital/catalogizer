// UI-D7 (CAT-UI-D7D8): the app-page colour-contrast defects axe reported on the live stack, asserted with the
// REAL Tailwind theme + the class names the REAL components render (no mocked classes).
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { contrastOf, ratio, WCAG_AA_NORMAL, type Theme } from '@/test/contrast'
import { Layout } from '@/components/layout/Layout'
import { ActivityFeed } from '@/components/dashboard/ActivityFeed'
import { Dashboard } from '@/pages/Dashboard'

vi.mock('@/components/layout/Header', async () => ({ Header: () => <div /> }))
vi.mock('@/contexts/AuthContext', async () => ({ useAuth: vi.fn(() => ({ user: { username: 'qa' } })) }))
vi.mock('react-hot-toast', async () => ({ default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/components/dashboard/DashboardStats', async () => ({ DashboardStats: () => <div /> }))
vi.mock('@/components/dashboard/MediaDistributionChart', async () => ({ MediaDistributionChart: () => <div /> }))
vi.mock('@/lib/adminApi', async () => ({ adminApi: { getSystemInfo: vi.fn().mockResolvedValue({}) } }))
vi.mock('@/lib/mediaApi', async () => ({
  mediaApi: { getMediaStats: vi.fn().mockResolvedValue({ total_items: 3, total_size: 1, recent_additions: 1, by_type: {}, by_quality: {} }) },
  entityApi: { getEntityStats: vi.fn().mockResolvedValue({ total_entities: 5, by_type: { movie: 3, tv_show: 2 } }) },
}))
vi.mock('@/lib/statsApi', async () => ({
  statsApi: {
    getOverallStats: vi.fn().mockResolvedValue({ total_directories: 4, total_files: 9, total_size: 2 * 1024 ** 3 }),
    getDuplicateStats: vi.fn().mockResolvedValue({ total_duplicate_groups: 2 }),
    getGrowthTrends: vi.fn().mockResolvedValue([{ files_added: 3 }]),
    getScanHistory: vi.fn().mockResolvedValue([{ id: 1, storage_root_name: 'root', files_found: 7, status: 'completed' }, { id: 2, storage_root_name: 'bad', files_found: 3, status: 'failed' }]),
  },
}))

const THEMES: Theme[] = ['light', 'dark']

describe('contrast helper is a real WCAG computation', () => {
  it('black on white is 21:1 and #767676 on white is 4.54:1', () => {
    expect(ratio([0, 0, 0], [255, 255, 255])).toBeCloseTo(21, 1)
    expect(ratio([0x76, 0x76, 0x76], [255, 255, 255])).toBeCloseTo(4.54, 1)
  })
})

const ROW = {
  id: 'a1', type: 'media_played' as const, title: 't', user: 'bob', timestamp: new Date(), metadata: { title: 'Matrix Row', quality: 'HD', duration: '2h' },
}

describe.each(THEMES)('UI-D7 contrast, %s theme (WCAG AA 4.5:1)', (theme) => {
  it('Layout version label (.fixed) meets AA', () => {
    render(<MemoryRouter><Layout /></MemoryRouter>)
    expect(contrastOf(screen.getByText('v1.1.0'), theme)).toBeGreaterThanOrEqual(WCAG_AA_NORMAL)
  })

  it('ActivityFeed unavailable-state hint meets AA', () => {
    render(<ActivityFeed />)
    for (const p of Array.from(screen.getByTestId('activity-unavailable').querySelectorAll('p'))) {
      expect(contrastOf(p, theme), p.textContent ?? '').toBeGreaterThanOrEqual(WCAG_AA_NORMAL)
    }
  })

  it('ActivityFeed row title, user and metadata meet AA', () => {
    render(<ActivityFeed activities={[ROW]} />)
    for (const el of [screen.getByText(/Matrix Row/), screen.getByText('bob'), screen.getByText(/HD/)]) {
      expect(contrastOf(el, theme), el.textContent ?? '').toBeGreaterThanOrEqual(WCAG_AA_NORMAL)
    }
  })

  it('Dashboard stat labels and scan rows meet AA', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(<QueryClientProvider client={qc}><MemoryRouter><Dashboard /></MemoryRouter></QueryClientProvider>)
    const labels = ['movie', 'tv show', 'Directories', 'Total Files', 'Total Size', 'Duplicate Groups', 'Files This Week', '7 files', 'completed', 'failed']
    for (const text of labels) {
      const el = await waitFor(() => screen.getByText(text))
      const r = contrastOf(el, theme)
      expect(r, `${text} (${theme}) ratio ${r.toFixed(2)}`).toBeGreaterThanOrEqual(WCAG_AA_NORMAL)
    }
  })
})
