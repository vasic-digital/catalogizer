// UI-D7-FU class-wide, populated states (reviewer round 1, F1/F2): integration cards, expanded panel, add modal,
// queued upload file, drag-over zone, selected conversion preset - the surfaces where a light bg-*-50 panel without a
// dark:bg-* variant turned a newly added dark:text-* class into a dark-mode regression (1.05-2.4:1).
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { failingElements, type Theme } from '@/test/contrast'
import ExternalIntegrations from '@/components/collections/ExternalIntegrations'
import { UploadManager } from '@/components/upload/UploadManager'
import { FormatConverter } from '@/components/conversion/FormatConverter'

vi.mock('framer-motion', () => {
  const M = ({ children, ...p }: any) => { const { initial, animate, exit, transition, whileHover, whileTap, layout, ...rest } = p; return <div {...rest}>{children}</div> }
  return { motion: { div: M, button: M }, AnimatePresence: ({ children }: any) => <>{children}</> }
})
// Icons keep a data-icon marker so the expand (Info) button can be found.
vi.mock('lucide-react', async (orig) => {
  const m: any = await orig(); const o: any = {}
  for (const k of Object.keys(m)) o[k] = /^[A-Z]/.test(k) ? (p: any) => <svg data-icon={k} className={p.className} /> : m[k]
  return o
})
vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn() }, default: { success: vi.fn(), error: vi.fn() } }))

const ITEM = {
  id: 'i1', name: 'Dropbox Sync', provider: 'dropbox', type: 'storage', status: 'connected', description: 'Sync files',
  config: { endpoint: 'https://x', webhook: 'https://w', apiKey: 'k' },
  syncSettings: { enabled: false, frequency: 'daily', direction: 'import', filters: [] },
  statistics: { totalSyncs: 3, successfulSyncs: 2, failedSyncs: 1, lastSyncStatus: 'success', itemsProcessed: 9, lastSyncDuration: 1200 },
}
const JOB: any = { id: 'j1', sourceFile: { path: '/a.mkv', name: 'a.mkv', format: 'mkv', size: 1048576 }, targetFormat: 'mp4', quality: 'high', status: 'completed', progress: 100, options: {}, startTime: '2024-01-01T00:00:00Z', endTime: '2024-01-01T00:01:00Z', outputFile: '/a.mp4' }

// D7-R4-2: tests assign global.fetch; restore it so the mock never leaks into sibling tests.
const realFetch = global.fetch
afterEach(() => { global.fetch = realFetch })

describe.each(['light', 'dark'] as Theme[])('UI-D7-FU populated states, %s theme', (theme) => {
  const fail = (c: Element) => failingElements(c, theme).join('\n')

  it('ExternalIntegrations: cards (connected + pending badge), expanded panel, add modal', async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ([ITEM, { ...ITEM, id: 'i2', name: 'Other', status: 'pending' }]) }) as any
    const { container } = render(<ExternalIntegrations />)
    await screen.findByText('Dropbox Sync')
    expect(screen.getByText('Other')).toBeInTheDocument()
    expect(fail(container), 'cards').toBe('')
    const info = container.querySelector('svg[data-icon="Info"]')!.closest('button')!
    fireEvent.click(info)
    await screen.findByText('Configuration')
    expect(fail(container), 'expanded').toBe('')
    fireEvent.click(screen.getAllByText('Add Integration')[0])
    await screen.findByText('Add External Integration')
    expect(fail(container), 'modal').toBe('')
  })

  it('UploadManager: queued file row and drag-over zone', async () => {
    const { container } = render(<UploadManager onUpload={vi.fn(() => new Promise(() => {}))} />)
    const zone = container.querySelector('.border-dashed')!
    fireEvent.dragOver(zone)
    expect(zone.className).toMatch(/bg-blue-50/)
    expect(fail(container), 'drag-over').toBe('')
    fireEvent.dragLeave(zone)
    fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [new File(['x'.repeat(10)], 'movie.mp4', { type: 'video/mp4' })] } })
    await screen.findByText('movie.mp4')
    expect(fail(container), 'queued').toBe('')
  })

  it('FormatConverter: selected quality preset (bg-blue-50 when selected) with the form open', () => {
    const { container } = render(<FormatConverter jobs={[JOB]} supportedFormats={['mp4', 'mkv']} />)
    fireEvent.click(screen.getAllByText('New Conversion')[0])
    const radios = Array.from(container.querySelectorAll('input[type=radio][name=quality]'))
    expect(radios.length).toBeGreaterThan(0)
    const checked = radios.find((r) => (r as HTMLInputElement).checked)!
    expect(checked.closest('label')!.className).toMatch(/bg-blue-50/)
    expect(fail(container)).toBe('')
  })
})

// D7-R4-FU-3: runs after the fetch-mocking tests above; fails if the afterEach restore is removed (reviewer mutation M7).
describe('D7-R4-FU-3 global.fetch does not leak out of the mocking tests', () => {
  it('fetch is the original again', () => {
    expect(global.fetch).toBe(realFetch)
  })
})
