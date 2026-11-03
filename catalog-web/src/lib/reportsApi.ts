import api from './api'
import { asList, decodeReportEnvelope } from './apiShapes'

/**
 * Report documents as the UI consumes them. Fields the backend does not (yet)
 * measure are `null`, never invented: pages render "—" for them.
 */
export interface UsageReport {
  period: string
  total_users: number | null
  active_users: number | null
  total_media_accessed: number | null
  total_downloads: number | null
  storage_growth_bytes: number | null
  top_media: { id: number; title: string; access_count: number }[]
  user_activity: { user_id: number; username: string; actions: number }[]
}

export interface PerformanceReport {
  period: string
  avg_response_time_ms: number | null
  p95_response_time_ms: number | null
  p99_response_time_ms: number | null
  total_requests: number | null
  error_rate: number | null
  uptime_percentage: number | null
  slowest_endpoints: { path: string; avg_ms: number; count: number }[]
}

type Params = { from?: string; to?: string }

const numOrNull = (v: unknown): number | null =>
  typeof v === 'number' && Number.isFinite(v) ? v : null

function period(doc: Record<string, unknown> | null, env: Record<string, unknown>): string {
  const params = (env.parameters ?? {}) as Record<string, unknown>
  const start = doc?.start_date ?? params.start_date
  const end = doc?.end_date ?? params.end_date
  const day = (v: unknown) => (typeof v === 'string' ? v.slice(0, 10) : '')
  return start && end ? `${day(start)} – ${day(end)}` : ''
}

// GET /reports/usage and /reports/performance answer a generated-report
// envelope `{type, format:'json', content:'<base64 JSON>', generated_at,
// parameters}` (user_analytics / system_overview documents), not the flat
// objects the pages render. Decode the envelope and map what it really carries.
function toUsageReport(payload: unknown): UsageReport {
  const env = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>
  const doc = decodeReportEnvelope<Record<string, unknown>>(env)
  const popular = asList<Record<string, unknown>>(doc?.popular_content)
  return {
    period: period(doc, env),
    total_users: numOrNull(doc?.total_users),
    active_users: numOrNull(doc?.active_users),
    total_media_accessed: numOrNull(doc?.total_media_accesses),
    total_downloads: null,
    storage_growth_bytes: null,
    top_media: popular.map((m) => ({
      id: Number(m.media_id ?? m.id ?? 0),
      title: typeof m.title === 'string' && m.title ? m.title : `Media #${Number(m.media_id ?? m.id ?? 0)}`,
      access_count: Number(m.access_count ?? 0),
    })),
    user_activity: [],
  }
}

function toPerformanceReport(payload: unknown): PerformanceReport {
  const env = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>
  const doc = decodeReportEnvelope<Record<string, unknown>>(env)
  const perf = (doc?.performance_metrics && typeof doc.performance_metrics === 'object'
    ? doc.performance_metrics
    : {}) as Record<string, unknown>
  return {
    period: period(doc, env),
    // reporting_service.go reports `response_time` as the P95 (snap.P95Seconds*1000),
    // not an average: map it truthfully and leave the average unknown.
    avg_response_time_ms: null,
    p95_response_time_ms: numOrNull(perf.response_time),
    p99_response_time_ms: null,
    total_requests: null,
    error_rate: numOrNull(perf.error_rate),
    uptime_percentage: null,
    slowest_endpoints: [],
  }
}

export const reportsApi = {
  getUsageReport: (params?: Params): Promise<UsageReport> =>
    api.get('/reports/usage', { params }).then((res) => toUsageReport(res.data)),

  getPerformanceReport: (params?: Params): Promise<PerformanceReport> =>
    api.get('/reports/performance', { params }).then((res) => toPerformanceReport(res.data)),
}
