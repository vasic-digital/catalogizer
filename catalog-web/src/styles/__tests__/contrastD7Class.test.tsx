// UI-D7-FU class-wide (D7-R2-F1): static class strings with text-gray-500/600 and NO dark:text-* variant on a
// dark surface (3.04:1 / 1.94:1 on gray-800). Every component below is rendered for real and EVERY text-bearing
// element is walked with the real Tailwind theme and the REAL ancestor background (a light panel without a
// dark:bg-* variant stays light in dark mode, so adding a dark:text-* class there would be the regression).
import React from 'react'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { failingElements, type Theme } from '@/test/contrast'
import ExternalIntegrations from '@/components/collections/ExternalIntegrations'
import AdvancedSearch from '@/components/collections/AdvancedSearch'
import CollectionAutomation from '@/components/collections/CollectionAutomation'
import { UploadManager } from '@/components/upload/UploadManager'
import { FormatConverter } from '@/components/conversion/FormatConverter'
import { EntityHero, ChildrenList, FilesList, DuplicatesList } from '@/components/entity/EntityDetailView'
import { AIUserBehaviorAnalytics } from '@/components/ai/AIAnalytics'
import { AIContentCategorizer } from '@/components/ai/AIComponents'
import { AIMetadataExtractor } from '@/components/ai/AIMetadata'
import { Settings } from '@/pages/Settings'

vi.mock('framer-motion', () => {
  const M = ({ children, ...p }: any) => { const { initial, animate, exit, transition, whileHover, whileTap, layout, ...rest } = p; return <div {...rest}>{children}</div> }
  const B = ({ children, ...p }: any) => { const { initial, animate, exit, transition, whileHover, whileTap, layout, ...rest } = p; return <button {...rest}>{children}</button> }
  return { motion: { div: M, button: B }, AnimatePresence: ({ children }: any) => <>{children}</> }
})
vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn() }, default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/components/favorites/FavoriteToggle', () => ({ FavoriteToggle: () => null }))
vi.mock('@/lib/identitiesApi', () => ({
  identitiesApi: {
    list: vi.fn(() => Promise.resolve([])), listHosts: vi.fn(() => Promise.resolve([])), listBindings: vi.fn(() => Promise.resolve([])),
    listShares: vi.fn(() => Promise.resolve([{ id: 1, host_id: 1, host_ip: '10.0.0.5', host_hostname: 'nas', protocol: 'smb', share_name: 'media', port: 445, first_seen: '2026-01-01T00:00:00Z', last_seen: '2026-01-01T00:00:00Z', enumeration_evidence_path: null }])),
    scanNetwork: vi.fn(), probeShare: vi.fn(),
  },
}))

vi.mock('@/lib/syncApi', () => ({
  syncApi: {
    listEndpoints: vi.fn(() => Promise.resolve([])),
    getStatistics: vi.fn(() => Promise.resolve({ total_syncs: 5, successful_syncs: 4, failed_syncs: 1, total_items_synced: 99 })),
    getSessions: vi.fn(() => Promise.resolve([{ id: 1, endpoint_id: 7, status: 'completed', items_synced: 3, started_at: '2026-01-01T00:00:00Z' }])),
  },
}))
vi.mock('@/lib/smbApi', () => ({ smbApi: { discover: vi.fn(() => Promise.resolve([{ name: 'media', host: '10.0.0.5', path: '/m' }])), testConnection: vi.fn() } }))
vi.mock('@/lib/mediaApi', () => ({ mediaApi: { getStorageRoots: vi.fn(() => Promise.resolve([])) } }))
vi.mock('@/lib/scansApi', () => ({ scansApi: { listScans: vi.fn(() => Promise.resolve([])), queueScan: vi.fn() } }))

const THEMES: Theme[] = ['light', 'dark']
const fail = (c: Element, t: Theme) => failingElements(c, t).join('\n')
const wrap = (ui: React.ReactElement) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  return render(<QueryClientProvider client={qc}><MemoryRouter>{ui}</MemoryRouter></QueryClientProvider>)
}

const ENTITY: any = { id: 1, title: 'The Matrix', media_type: 'movie', media_type_id: 1, year: 1999, description: 'Neo learns.', genre: ['Sci-Fi'], status: 'detected', rating: 8.7, file_count: 1, children_count: 2, external_metadata: [], first_detected: '2024-01-01T00:00:00Z', last_updated: '2024-01-02T00:00:00Z' }
const CHILD: any = { id: 2, title: 'Reloaded', media_type: 'movie', media_type_id: 1, year: 2003, status: 'detected', first_detected: '2024-01-01T00:00:00Z', last_updated: '2024-01-02T00:00:00Z' }
const FILE: any = { id: 1, media_item_id: 1, file_id: 1, quality_info: '1080p', language: 'en', is_primary: true, created_at: '2024-01-01T00:00:00Z' }
const JOB: any = { id: 'j1', sourceFile: { path: '/a.mkv', name: 'a.mkv', format: 'mkv', size: 1048576 }, targetFormat: 'mp4', quality: 'high', status: 'completed', progress: 100, options: {}, startTime: '2024-01-01T00:00:00Z', endTime: '2024-01-01T00:01:00Z', outputFile: '/a.mp4' }

describe.each(THEMES)('UI-D7-FU class-wide gray text, %s theme (WCAG AA 4.5:1)', (theme) => {
  it('ExternalIntegrations: whole panel incl. integration cards', async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: false, json: async () => ([]) }) as any
    const { container } = render(<ExternalIntegrations />)
    await screen.findByText('External Integrations')
    expect(fail(container, theme)).toBe('')
  })

  it('AdvancedSearch: every tab', () => {
    const { container } = render(<AdvancedSearch />)
    expect(fail(container, theme)).toBe('')
    for (const name of ['Presets', 'Saved']) {
      fireEvent.click(screen.getByRole('button', { name }))
      expect(fail(container, theme), name).toBe('')
    }
  })

  it('CollectionAutomation', () => {
    const { container } = render(<CollectionAutomation />)
    expect(fail(container, theme)).toBe('')
  })

  it('UploadManager: empty and with a queued file', () => {
    const { container } = render(<UploadManager />)
    expect(fail(container, theme)).toBe('')
  })

  it('FormatConverter: empty list and a completed job', () => {
    const { container, rerender } = render(<FormatConverter jobs={[]} supportedFormats={['mp4', 'mkv']} />)
    expect(fail(container, theme)).toBe('')
    rerender(<FormatConverter jobs={[JOB]} supportedFormats={['mp4', 'mkv']} />)
    expect(fail(container, theme)).toBe('')
  })

  it('EntityDetailView: hero, children, files, duplicates', () => {
    const { container } = render(
      <MemoryRouter>
        <EntityHero entity={ENTITY} files={[FILE]} duplicateCount={1} onFavorite={() => {}} onRefresh={() => {}} refreshPending={false} />
        <ChildrenList items={[CHILD]} mediaType="movie" onChildClick={() => {}} />
        <FilesList files={[FILE]} />
        <DuplicatesList duplicates={[CHILD]} onDuplicateClick={() => {}} />
      </MemoryRouter>,
    )
    expect(fail(container, theme)).toBe('')
  })

  it('AI unavailable panels', () => {
    const { container } = render(
      <div>
        <AIUserBehaviorAnalytics userId="u" onActionImplement={() => {}} />
        <AIContentCategorizer />
        <AIMetadataExtractor />
      </div>,
    )
    expect(fail(container, theme)).toBe('')
  })

  it('Settings page: scans, SMB discovery result and sync statistics/sessions', async () => {
    const { container } = wrap(<Settings />)
    await screen.findByText('Settings')
    expect(fail(container, theme)).toBe('')
    fireEvent.click(screen.getByText('SMB Discovery'))
    fireEvent.click(await screen.findByText('Discover Shares'))
    await screen.findByText('media')
    expect(fail(container, theme)).toBe('')
    fireEvent.click(screen.getByText('Sync'))
    await screen.findByText('Total Syncs')
    await screen.findByText(/items synced/)
    expect(fail(container, theme)).toBe('')
  })
})
