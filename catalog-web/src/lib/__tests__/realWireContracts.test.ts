/**
 * API-layer contract tests: the real api modules run over responses captured
 * from a live catalog-api (transport-level fake only), pinning the normalised
 * shapes the pages depend on. Paired with realApiShapes.test.tsx (page level).
 */
import { installRealApi } from '@/test/realApiTransport'
import { syncApi } from '../syncApi'
import { analyticsApi } from '../analyticsApi'
import { statsApi } from '../statsApi'

let restore: () => void
afterEach(() => restore?.())

describe('sync wire contract ({data:null|[...],success:true})', () => {
  it('null data lists become empty arrays', async () => {
    restore = installRealApi()
    expect(await syncApi.listEndpoints()).toEqual([])
    expect(await syncApi.getSessions()).toEqual([])
  })

  it('maps statistics (total_sessions / by_status / total_files_synced)', async () => {
    restore = installRealApi({
      '/sync/statistics': { success: true, data: { total_sessions: 4, by_status: { completed: 3, failed: 1 }, total_files_synced: 12, total_files_failed: 2 } },
    })
    const s = await syncApi.getStatistics()
    expect(s).toMatchObject({ total_syncs: 4 })
    expect(JSON.stringify(s)).toContain('12')
  })

  it('normalises an endpoint row (status → enabled)', async () => {
    restore = installRealApi({
      '/sync/endpoints': { success: true, data: [{ id: 1, name: 'e', status: 'active', url: 'x' }, { id: 2, name: 'f', status: 'paused', url: 'y' }] },
    })
    const eps = await syncApi.listEndpoints()
    expect(eps.map((e) => e.enabled)).toEqual([true, false])
  })
})

describe('analytics wire contract', () => {
  it('system analytics: null top_accessed_media and renamed fields do not leak undefined', async () => {
    restore = installRealApi()
    const a = await analyticsApi.getSystemAnalytics()
    expect(a.popular_media).toEqual([])
    expect(a.events_by_type).toEqual({})
    expect(typeof a.active_users_24h).toBe('number')
  })

  it('growth trends are derived from monthly_growth with cumulative totals', async () => {
    restore = installRealApi({
      '/stats/growth': { success: true, data: { monthly_growth: [{ month: '2026-10', files_added: 2, size_added: 10 }, { month: '2026-11', files_added: 3, size_added: 5 }] } },
    })
    const g = await statsApi.getGrowthTrends()
    expect(g).toHaveLength(2)
    expect(g[1]).toMatchObject({ cumulative_files: 5, cumulative_size: 15 })
  })
})
