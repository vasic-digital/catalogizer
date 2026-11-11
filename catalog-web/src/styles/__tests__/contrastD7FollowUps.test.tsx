// UI-D7-R3 follow-ups: FormatConverter job error/success boxes (light islands in dark mode),
// ExternalIntegrations status badges for every status in both themes, UploadManager queued row vs its Card.
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { backgroundOf, contrastOf, failingElements, luminance, ratio, type Theme } from '@/test/contrast'
import ExternalIntegrations from '@/components/collections/ExternalIntegrations'
import { UploadManager } from '@/components/upload/UploadManager'
import { FormatConverter } from '@/components/conversion/FormatConverter'

vi.mock('framer-motion', () => {
  const M = ({ children, ...p }: any) => { const { initial, animate, exit, transition, whileHover, whileTap, layout, ...rest } = p; return <div {...rest}>{children}</div> }
  return { motion: { div: M, button: M }, AnimatePresence: ({ children }: any) => <>{children}</> }
})
vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn() }, default: { success: vi.fn(), error: vi.fn() } }))

const JOB: any = { id: 'j1', sourceFile: { path: '/a.mkv', name: 'a.mkv', format: 'mkv', size: 1048576 }, targetFormat: 'mp4', quality: 'high', status: 'completed', progress: 100, options: {}, startTime: '2024-01-01T00:00:00Z', endTime: '2024-01-01T00:01:00Z', outputFile: '/a.mp4' }
// A dark-mode background brighter than this is a "light island" on the gray-900 page (gray-800 = 0.023).
const LIGHT_ISLAND_LUM = 0.1

// D7-R4-2: tests that assign global.fetch must restore it, never leak the mock into sibling tests.
const realFetch = global.fetch
afterEach(() => { global.fetch = realFetch })

describe.each(['light', 'dark'] as Theme[])('D7-R3-FU1 FormatConverter job boxes, %s theme', (theme) => {
  it.each([
    ['failed', { status: 'failed', error: 'ffmpeg exited 1' }, 'ffmpeg exited 1', 5.91],
    ['completed', { status: 'completed' }, /Conversion completed successfully/, 4.79],
  ] as const)('%s box', (_n, over, text, lightRatio) => {
    const { container } = render(<FormatConverter jobs={[{ ...JOB, ...over }]} supportedFormats={['mp4', 'mkv']} />)
    const box = screen.getByText(text as any)
    const r = contrastOf(box, theme)
    expect(r).toBeGreaterThanOrEqual(4.5)
    expect(failingElements(container, theme).join('\n')).toBe('')
    if (theme === 'dark') expect(luminance(backgroundOf(box, 'dark')), 'light island in dark').toBeLessThan(LIGHT_ISLAND_LUM)
    else expect(r).toBeCloseTo(lightRatio, 1) // light ratios unchanged
  })
})

describe.each(['light', 'dark'] as Theme[])('D7-R3-M1 ExternalIntegrations status badges, %s theme', (theme) => {
  it('one integration per status, badge contrast >= 4.5', async () => {
    const statuses = ['connected', 'connecting', 'pending', 'disconnected', 'error']
    const items = statuses.map((s) => ({ id: s, name: `Int-${s}`, provider: 'dropbox', type: 'storage', status: s, description: 'd', config: {}, syncSettings: { enabled: false, frequency: 'daily', direction: 'import', filters: [] }, statistics: { totalSyncs: 0, successfulSyncs: 0, failedSyncs: 0, lastSyncStatus: 'pending', itemsProcessed: 0, lastSyncDuration: 0 } }))
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => items }) as any
    const { container } = render(<ExternalIntegrations />)
    await screen.findByText('Int-connected')
    for (const s of statuses) {
      // badge = the rounded-full chip whose own text is the status word, inside this integration's card
      const card = screen.getByText(`Int-${s}`).closest('.rounded-lg')!
      const badge = Array.from(card.querySelectorAll('div.rounded-full')).find((e) => e.textContent?.trim() === s)
      expect(badge, `badge ${s}`).toBeTruthy()
      expect(contrastOf(badge!, theme), `badge ${s} contrast`).toBeGreaterThanOrEqual(4.5)
      // D7-R4-3: the pending chip must be distinguishable from the card it sits on in dark (card = dark:bg-gray-800)
      if (s === 'pending' && theme === 'dark') {
        expect(backgroundOf(badge!, theme), 'pending chip bg != card bg').not.toEqual(backgroundOf(card, theme))
        expect(ratio(backgroundOf(badge!, theme), backgroundOf(card, theme)), 'pending chip vs card').toBeGreaterThan(1.05)
      }
      // status icon (svg) must carry the badge colour via inheritance, never its own low-contrast colour
      const svg = badge!.querySelector('svg')
      if (svg) expect(svg.getAttribute('class') ?? '', `icon ${s}`).not.toMatch(/text-(blue|red|green|gray)-[345]00/)
    }
    expect(failingElements(container, theme, 4.5, { islands: true }).join('\n')).toBe('')
  })
})

describe('D7-R3-M3 UploadManager queued row vs Card, dark', () => {
  it('row background differs from the Card background and keeps >= 4.5', async () => {
    const { container } = render(<UploadManager onUpload={vi.fn(() => new Promise(() => {}))} />)
    fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [new File(['x'], 'movie.mp4', { type: 'video/mp4' })] } })
    const name = await screen.findByText('movie.mp4')
    const row = name.closest('.rounded-lg.p-3') as Element
    expect(row).toBeTruthy()
    for (const theme of ['light', 'dark'] as Theme[]) {
      const rowBg = backgroundOf(row, theme)
      const cardBg = backgroundOf(row.parentElement, theme)
      expect(rowBg, `row bg differs from card bg (${theme})`).not.toEqual(cardBg)
      expect(contrastOf(name, theme)).toBeGreaterThanOrEqual(4.5)
    }
    expect(ratio(backgroundOf(row, 'dark'), backgroundOf(row.parentElement, 'dark'))).toBeGreaterThan(1.05)
    expect(failingElements(container, 'dark').join('\n')).toBe('')
  })
})

describe.each(['light', 'dark'] as Theme[])('D7-R4-1 UploadManager FAILED row error text, %s theme', (theme) => {
  it('error text has >= 4.5 on the real row background', async () => {
    const { container } = render(<UploadManager onUpload={vi.fn(() => Promise.reject(new Error('Disk quota exceeded')))} />)
    fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [new File(['x'], 'bad.mp4', { type: 'video/mp4' })] } })
    const err = await screen.findByText('Disk quota exceeded')
    expect(contrastOf(err, theme)).toBeGreaterThanOrEqual(4.5)
    expect(failingElements(container, theme).join('\n')).toBe('')
    expect(failingElements(container, theme, 4.5, { islands: true }).join('\n')).toBe('')
  })
})

describe('D7-R4-2 global.fetch is restored after the mocking tests', () => {
  it('fetch is the original again', () => {
    expect(global.fetch).toBe(realFetch)
  })
})
