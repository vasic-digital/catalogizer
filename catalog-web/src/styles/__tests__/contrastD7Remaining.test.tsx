// UI-D7-FU: the remaining WCAG AA colour-contrast defects axe reported on the live stack
// (docs/qa/ui-d7d8-20261105/d7_live_red.txt): /ai badges+tabs, /admin dark, /media card text, /analytics dark labels.
// Real Tailwind theme + the class names the REAL components render; every text-bearing element is checked.
import React from 'react'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { contrastOf, WCAG_AA_NORMAL, type Theme } from '@/test/contrast'
import { AdminPanel } from '@/components/admin/AdminPanel'
import { Admin } from '@/pages/Admin'
import AIDashboard from '@/pages/AIDashboard'
import { MediaCard } from '@/components/media/MediaCard'
import { Analytics } from '@/pages/Analytics'
import type { MediaItem } from '@/types/media'

vi.mock('react-hot-toast', () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))
vi.mock('framer-motion', async () => {
  const M = ({ children, ...p }: any) => <div {...p}>{children}</div>
  return { motion: { div: M } }
})
vi.mock('@/components/favorites/FavoriteToggle', () => ({ FavoriteToggle: () => null }))
vi.mock('recharts', async () => {
  const D = ({ children }: any) => <div>{children}</div>
  return { ResponsiveContainer: D, PieChart: D, Pie: D, Cell: () => <div />, BarChart: D, Bar: D, LineChart: D, Line: D, AreaChart: D, Area: D, CartesianGrid: D, XAxis: D, YAxis: D, Tooltip: D }
})
vi.mock('@/components/ai/AIComponents', () => ({ AICollectionSuggestions: () => <div />, AINaturalSearch: () => <div />, AIContentCategorizer: () => <div />, AIService: {} }))
vi.mock('@/components/ai/AIAnalytics', () => ({ AIUserBehaviorAnalytics: () => <div />, AIPredictions: () => <div />, AISmartOrganization: () => <div />, AIAnalyticsService: {} }))
vi.mock('@/components/ai/AIMetadata', () => ({ AIMetadataExtractor: () => <div />, AIAutomationRules: () => <div />, AIContentQualityAnalyzer: () => <div />, AIMetadataService: {} }))
vi.mock('@/lib/adminApi', () => ({
  adminApi: {
    getSystemInfo: vi.fn(() => Promise.resolve({ version: '1.0.0', uptime: 86400, cpuUsage: 45, memoryUsage: 62, diskUsage: { total: 100, used: 50, free: 50 }, activeConnections: 12, totalRequests: 15420 })),
    getUsers: vi.fn(() => Promise.resolve([])), getStorageInfo: vi.fn(() => Promise.resolve([])), getBackups: vi.fn(() => Promise.resolve([])),
  },
}))
vi.mock('@/lib/reportsApi', () => ({
  reportsApi: {
    getUsageReport: vi.fn(() => Promise.resolve({ active_users: 3, total_media_accessed: 9, total_downloads: 2, storage_growth_bytes: 1024 ** 3, top_media: [{ id: 1, title: 'Top One', access_count: 4 }] })),
    getPerformanceReport: vi.fn(() => Promise.resolve({ p95_response_time_ms: 12, total_requests: 100, error_rate: 1, avg_response_time_ms: 5, uptime_percentage: 99.5, slowest_endpoints: [{ path: '/api/x', avg_ms: 30, count: 4 }] })),
  },
}))
vi.mock('@/lib/mediaApi', async () => ({
  mediaApi: {
    getMediaStats: vi.fn(() => Promise.resolve({ total_items: 3, total_size: 1, recent_additions: 1, by_type: {}, by_quality: {} })),
    getRecentMedia: vi.fn(() => Promise.resolve([])),
  },
}))
vi.mock('@/lib/statsApi', async () => ({
  statsApi: {
    getGrowthTrends: vi.fn(() => Promise.resolve([{ date: '2026-03-01', files_added: 10, bytes_added: 1000 }])),
    getFileTypeStats: vi.fn(() => Promise.resolve([{ extension: '.mp4', count: 100, total_size: 5 }])),
    getDuplicateStats: vi.fn(() => Promise.resolve({ total_duplicate_groups: 5, total_duplicate_files: 12, wasted_space: 500000 })),
    getOverallStats: vi.fn(() => Promise.resolve({ total_files: 1000, total_dirs: 50, total_size: 100 })),
    getScanHistory: vi.fn(() => Promise.resolve([])),
  },
}))
vi.mock('@/lib/analyticsApi', async () => ({
  analyticsApi: {
    getSystemAnalytics: vi.fn(() => Promise.resolve({ total_events: 42, active_users_24h: 3, popular_media: [{ media_id: 1, title: 'Pop', access_count: 7 }], events_by_type: {} })),
  },
}))

const THEMES: Theme[] = ['light', 'dark']

/** Every element that directly holds visible text, with its WCAG ratio; returns those below `min`. */
function failing(root: Element, theme: Theme, min = WCAG_AA_NORMAL) {
  const bad: string[] = []
  const w = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT)
  for (let n = w.currentNode as Element | null; n; n = w.nextNode() as Element | null) {
    if (n.closest('svg')) continue
    const own = Array.from(n.childNodes).some((c) => c.nodeType === 3 && (c.textContent ?? '').trim() !== '')
    if (!own) continue
    // Text over a translucent black overlay sits on user imagery: not computable from classes, skip it.
    if (/\bbg-black\//.test(n.getAttribute('class') ?? '')) continue
    // WCAG "large text" (>=24px, or >=18.66px bold) needs 3:1.
    const cls = n.getAttribute('class') ?? ''
    const large = /\btext-(2xl|3xl|4xl|5xl)\b/.test(cls) || (/\btext-xl\b/.test(cls) && /\bfont-bold\b/.test(cls))
    const r = contrastOf(n, theme)
    if (r < (large ? 3 : min)) bad.push(`${n.tagName.toLowerCase()}.${n.getAttribute('class')} "${(n.textContent ?? '').trim().slice(0, 30)}" ${r.toFixed(2)}`)
  }
  return bad
}

const wrap = (ui: React.ReactElement) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

const MEDIA: MediaItem = {
  id: 1, title: 'Test Movie', media_type: 'movie', year: 2024, description: 'A test movie description', rating: 8.5, quality: '1080p',
  file_size: 1073741824, duration: 7200, directory_path: '/movies/test-movie', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z',
}

describe.each(THEMES)('UI-D7-FU contrast, %s theme (WCAG AA 4.5:1)', (theme) => {
  it('/ai: tabs and every section (after lazy load) meet AA', async () => {
    const { container } = render(<AIDashboard />)
    expect(failing(container, theme).join('\n')).toBe('')
    const sections: [string, string][] = [
      ['AI Suggestions', 'AI-Powered Suggestions'], ['Natural Search', 'Natural Language Search'],
      ['Analytics', 'AI Analytics'], ['Metadata', 'AI Metadata Services'], ['Automation', 'AI Automation'],
    ]
    for (const [tab, heading] of sections) {
      fireEvent.click(screen.getAllByText(tab)[0])
      await screen.findByText(heading)
      await waitFor(() => expect(container.querySelector('.animate-spin')).toBeNull())
      expect(failing(container, theme).join('\n'), tab).toBe('')
    }
  })

  it('/admin: panel (all tabs) meets AA', () => {
    const props = {
      systemInfo: { version: '1.0.0', uptime: 86400, cpuUsage: 45, memoryUsage: 62, diskUsage: { total: 100, used: 50, free: 50 }, activeConnections: 12, totalRequests: 15420 },
      users: [{ id: '1', username: 'admin', email: 'a@t.com', role: 'admin' as const, status: 'active' as const, lastLogin: '2024-01-01T00:00:00Z', createdAt: '2023-01-01T00:00:00Z' }],
      storageInfo: [{ path: '/m', totalSpace: 100, usedSpace: 50, availableSpace: 50, mediaCount: 3, lastScan: '2024-01-01T00:00:00Z' }],
      backups: [{ id: '1', filename: 'b.tar.gz', size: 1, createdAt: '2024-01-01T00:00:00Z', type: 'full' as const, status: 'completed' as const }],
    }
    const { container } = render(<AdminPanel {...props} />)
    expect(failing(container, theme).join("\n")).toBe("")
    for (const t of ['Users', 'Storage', 'Backups']) {
      fireEvent.click(screen.getByText(t))
      expect(failing(container, theme).join("\n"), t).toBe("")
    }
  })

  it('/admin page: usage and performance reports meet AA', async () => {
    const { container } = wrap(<Admin />)
    await screen.findByText('Usage Report')
    await screen.findByText('Performance Report')
    expect(failing(container, theme).join("\n")).toBe("")
  })

  it('/media: MediaCard text meets AA', () => {
    const { container } = wrap(<MediaCard media={MEDIA} />)
    expect(failing(container, theme).join("\n")).toBe("")
  })

  it('/analytics: stat labels meet AA', async () => {
    const { container } = wrap(<Analytics />)
    await waitFor(() => expect(screen.getByText('Total Events')).toBeInTheDocument())
    expect(failing(container, theme).join("\n")).toBe("")
  })
})
