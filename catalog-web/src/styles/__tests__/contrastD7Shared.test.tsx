// UI-D7-FU F7: same defect class as the D7 list, found in other components by review:
// white-on-green-600 buttons (3.29:1) and small text-gray-400 (2.53:1 light / dark variants below AA).
import React from 'react'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { contrastOf, WCAG_AA_NORMAL, type Theme } from '@/test/contrast'
import { SubtitleManager } from '@/pages/SubtitleManager'
import { CollectionRealTime } from '@/components/collections/CollectionRealTime'
import { CollectionsManager } from '@/components/collections/CollectionsManager'
import { MediaDistributionChart } from '@/components/dashboard/MediaDistributionChart'
import { DiscoveredShares } from '@/components/identity/DiscoveredShares'
import { EntityDetail } from '@/pages/EntityDetail'

vi.mock('framer-motion', () => ({
  motion: { div: ({ children, className, onClick }: any) => <div className={className} onClick={onClick}>{children}</div> },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))
vi.mock('react-hot-toast', () => ({ toast: vi.fn(), default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('recharts', () => {
  const D = ({ children }: any) => <div>{children}</div>
  return { PieChart: D, Pie: D, Cell: D, ResponsiveContainer: D, Legend: D, Tooltip: D }
})
vi.mock('@/contexts/AuthContext', () => ({ useAuth: () => ({ isAdmin: false }) }))
vi.mock('@/components/subtitles/SubtitleSyncModal', () => ({ SubtitleSyncModal: () => <div /> }))
vi.mock('@/components/subtitles/SubtitleUploadModal', () => ({ SubtitleUploadModal: () => <div /> }))
vi.mock('@/lib/mediaApi', () => ({
  mediaApi: { searchMedia: vi.fn(() => Promise.resolve({ items: [] })) },
  entityApi: {
    getEntity: vi.fn(() => Promise.resolve({ id: 1, title: 'The Matrix', media_type: 'movie', media_type_id: 1, year: 1999, genre: [], status: 'movie', file_count: 0, children_count: 0, external_metadata: [], first_detected: '2024-01-01T00:00:00Z', last_updated: '2024-01-02T00:00:00Z' })),
    getEntityChildren: vi.fn(() => Promise.resolve({ items: [], total: 0, limit: 24, offset: 0 })),
    getEntityFiles: vi.fn(() => Promise.resolve({ files: [], total: 0 })),
    getEntityDuplicates: vi.fn(() => Promise.resolve({ duplicates: [], total: 0 })),
  },
}))
vi.mock('@/lib/recommendationsApi', () => ({
  recommendationsApi: { getSimilar: vi.fn(() => Promise.resolve([{ id: 2, title: 'Reloaded', media_type: 'movie', year: 2003, similarity_score: 0.8, reason: 'Same director' }])) },
}))
vi.mock('@/lib/downloadApi', () => ({ downloadApi: {} }))
vi.mock('@/lib/analyticsApi', () => ({ analyticsApi: { trackEvent: vi.fn(() => Promise.resolve()), trackAccess: vi.fn(() => Promise.resolve({ ok: true })) } }))
vi.mock('@/components/entity/EntityDetailView', () => ({
  EntityHero: () => <div />, ChildrenList: () => <div />, FilesList: () => <div />, DuplicatesList: () => <div />,
}))
vi.mock('@/lib/subtitleApi', () => ({
  subtitleApi: {
    searchSubtitles: vi.fn(() => Promise.resolve({ results: [{ id: 's1', title: 'Matrix', language: 'en', language_name: 'English', provider: 'p', download_url: 'u', hearing_impaired: false, foreign_parts_only: false, machine_translated: false }], total: 1, limit: 20, offset: 0, query_time_ms: 1 })),
    downloadSubtitle: vi.fn(), getMediaSubtitles: vi.fn(), verifySync: vi.fn(), translateSubtitle: vi.fn(),
    getSupportedLanguages: vi.fn(() => Promise.resolve([])), getSupportedProviders: vi.fn(() => Promise.resolve([])),
    deleteSubtitle: vi.fn(), updateSubtitle: vi.fn(), uploadSubtitle: vi.fn(),
  },
}))
vi.mock('@/lib/identitiesApi', () => ({
  identitiesApi: {
    list: vi.fn(() => Promise.resolve([])), scanNetwork: vi.fn(), probeShare: vi.fn(), listBindings: vi.fn(),
    listHosts: vi.fn(() => Promise.resolve([{ id: 1, ip: '10.0.0.5', hostname: 'nas', first_seen: '2026-01-01T00:00:00Z', last_seen: '2026-01-01T00:00:00Z', reachable: true, ifaces_json: null }])),
    listShares: vi.fn(() => Promise.resolve([
      { id: 1, host_id: 1, host_ip: '10.0.0.5', host_hostname: 'nas', protocol: 'smb', share_name: 'media', port: 445, first_seen: '2026-01-01T00:00:00Z', last_seen: '2026-01-01T00:00:00Z', enumeration_evidence_path: null },
      { id: 2, host_id: 1, host_ip: '10.0.0.5', host_hostname: 'nas', protocol: 'nfs', share_name: 'backup', port: 2049, first_seen: '2026-01-01T00:00:00Z', last_seen: '2026-01-02T00:00:00Z', enumeration_evidence_path: null },
    ])),
  },
}))

const THEMES: Theme[] = ['light', 'dark']

/** Contrast of every element matching each text matcher (targeted: only the defects listed in review F7). */
function failing(_root: Element, theme: Theme, matchers: (string | RegExp)[]) {
  const bad: string[] = []
  for (const m of matchers) {
    const els = screen.getAllByText(m)
    for (const el of els) {
      const r = contrastOf(el, theme)
      if (r < WCAG_AA_NORMAL) bad.push(`${el.tagName.toLowerCase()}.${el.getAttribute('class')} "${(el.textContent ?? '').trim().slice(0, 30)}" ${r.toFixed(2)}`)
    }
  }
  return bad.join('\n')
}

const wrap = (ui: React.ReactElement) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  return render(<QueryClientProvider client={qc}><MemoryRouter>{ui}</MemoryRouter></QueryClientProvider>)
}

const COLLECTIONS = [{ id: '1', name: 'Movie Favorites', description: 'd', mediaCount: 4, duration: 60, isSmart: true, createdAt: '2024-01-01T00:00:00Z', updatedAt: '2024-01-15T00:00:00Z' }]

describe.each(THEMES)('UI-D7-FU shared components, %s theme (WCAG AA 4.5:1)', (theme) => {
  it('SubtitleManager: result Download button meets AA', async () => {
    const { container } = wrap(<SubtitleManager />)
    fireEvent.change(screen.getByPlaceholderText(/Search subtitles by title/), { target: { value: 'Matrix' } })
    fireEvent.click(screen.getByText('Search'))
    await screen.findByText('Download')
    expect(failing(container, theme, ['Download'])).toBe('')
  })

  it('CollectionRealTime: Connect button meets AA', () => {
    const c = { id: '1', name: 'T', description: '', is_smart: false, smart_rules: [], item_count: 5, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z', is_public: false, primary_media_type: 'music', owner_id: 'u' }
    const { container } = render(<CollectionRealTime collection={c as any} />)
    expect(failing(container, theme, [/^Connect$/])).toBe('')
  })

  it('CollectionsManager: "Updated" text in grid and list view meets AA', () => {
    const props = { collections: COLLECTIONS, onCreateCollection: vi.fn(), onUpdateCollection: vi.fn(), onDeleteCollection: vi.fn(), onPlayCollection: vi.fn() }
    const { container } = render(<CollectionsManager {...props} />)
    expect(failing(container, theme, [/^Updated /])).toBe('')
    fireEvent.click(container.querySelectorAll('button')[1]) // list-view toggle
    expect(failing(container, theme, [/^Updated /])).toBe('')
  })

  it('MediaDistributionChart: empty-state hint meets AA', () => {
    const { container } = render(<MediaDistributionChart />)
    expect(failing(container, theme, ['Media will appear here once scanned'])).toBe('')
  })

  it('DiscoveredShares: last-seen and filtered hint meet AA', async () => {
    const { container } = wrap(<DiscoveredShares />)
    await screen.findByText('media')
    expect(failing(container, theme, [/^seen /])).toBe('')
    fireEvent.change(screen.getByPlaceholderText('Search host...'), { target: { value: '10.0' } })
    fireEvent.change(container.querySelector('select')!, { target: { value: 'smb' } })
    expect(failing(container, theme, [/filtered from/])).toBe('')
  })

  it('EntityDetail: similar-items reason line meets AA', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    const { container } = render(
      <QueryClientProvider client={qc}><MemoryRouter initialEntries={['/entity/1']}><Routes><Route path="/entity/:id" element={<EntityDetail />} /></Routes></MemoryRouter></QueryClientProvider>,
    )
    await screen.findByText('Same director')
    expect(failing(container, theme, ['Same director'])).toBe('')
  })
})
