import { reportsApi } from '../reportsApi'
import apiDefault from '../api'

vi.mock('../api', async () => {
  const mockApi = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  }
  return {
    __esModule: true,
    default: mockApi,
    api: mockApi,
  }
})

const mockApi = vi.mocked(apiDefault)

describe('reportsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getUsageReport', () => {
    it('decodes the base64 report envelope; unmeasured fields are null, not invented', async () => {
      const doc = { start_date: '2026-10-01T00:00:00Z', end_date: '2026-10-31T00:00:00Z', total_users: 3, active_users: 2, total_media_accesses: 7, popular_content: [{ media_id: 9, access_count: 4 }] }
      mockApi.get.mockResolvedValue({ data: { type: 'user_analytics', format: 'json', content: Buffer.from(JSON.stringify(doc)).toString('base64'), generated_at: '2026-11-03T00:00:00Z', parameters: {} } })

      const result = await reportsApi.getUsageReport()

      expect(mockApi.get).toHaveBeenCalledWith('/reports/usage', { params: undefined })
      expect(result).toMatchObject({ total_users: 3, active_users: 2, total_media_accessed: 7, total_downloads: null, storage_growth_bytes: null })
      expect(result.top_media).toEqual([{ id: 9, title: 'Media #9', access_count: 4 }])
      expect(result.user_activity).toEqual([])
    })

    it('calls GET /reports/usage with date range params', async () => {
      const params = { from: '2024-01-01', to: '2024-01-31' }
      mockApi.get.mockResolvedValue({ data: {} })

      await reportsApi.getUsageReport(params)

      expect(mockApi.get).toHaveBeenCalledWith('/reports/usage', { params })
    })

    it('propagates errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Report failed'))

      await expect(reportsApi.getUsageReport()).rejects.toThrow('Report failed')
    })
  })

  describe('getPerformanceReport', () => {
    it('decodes performance_metrics; missing metrics are null so the page can render a dash', async () => {
      const doc = { performance_metrics: { response_time: 45, throughput: 10, error_rate: 0.02 } }
      mockApi.get.mockResolvedValue({ data: { type: 'system_overview', format: 'json', content: Buffer.from(JSON.stringify(doc)).toString('base64'), generated_at: '2026-11-03T00:00:00Z', parameters: {} } })

      const result = await reportsApi.getPerformanceReport()

      expect(mockApi.get).toHaveBeenCalledWith('/reports/performance', { params: undefined })
      expect(result).toMatchObject({ avg_response_time_ms: 45, error_rate: 0.02, total_requests: null, p95_response_time_ms: null, uptime_percentage: null })
      expect(result.slowest_endpoints).toEqual([])
    })

    it('calls GET /reports/performance with date range params', async () => {
      const params = { from: '2024-01-01', to: '2024-01-31' }
      mockApi.get.mockResolvedValue({ data: {} })

      await reportsApi.getPerformanceReport(params)

      expect(mockApi.get).toHaveBeenCalledWith('/reports/performance', { params })
    })

    it('propagates errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Server error'))

      await expect(reportsApi.getPerformanceReport()).rejects.toThrow('Server error')
    })
  })
})
