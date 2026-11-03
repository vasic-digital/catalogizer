import api from './api'
import { asList, unwrapData } from './apiShapes'

export interface SyncEndpoint {
  id: number
  name: string
  url: string
  api_key?: string
  enabled: boolean
  last_sync_at?: string
  created_at: string
  updated_at: string
}

export interface CreateEndpointRequest {
  name: string
  url: string
  api_key?: string
  enabled?: boolean
}

export interface UpdateEndpointRequest {
  name?: string
  url?: string
  api_key?: string
  enabled?: boolean
}

export interface SyncSession {
  id: number
  endpoint_id: number
  status: 'running' | 'completed' | 'failed'
  started_at: string
  completed_at?: string
  items_synced: number
  items_failed: number
  error_message?: string
}

export interface SyncStatistics {
  total_syncs: number
  successful_syncs: number
  failed_syncs: number
  total_items_synced: number
  last_sync_at?: string
}

function num(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0
}

function normalizeEndpoint(raw: Record<string, unknown>): SyncEndpoint {
  return {
    ...raw,
    enabled: typeof raw.enabled === 'boolean' ? raw.enabled : raw.status === 'active',
  } as unknown as SyncEndpoint
}

function normalizeSession(raw: Record<string, unknown>): SyncSession {
  return {
    ...raw,
    items_synced: num(raw.items_synced ?? raw.synced_files),
    items_failed: num(raw.items_failed ?? raw.failed_files),
  } as unknown as SyncSession
}

// Wire shape: `{total_sessions, total_files_synced, total_files_failed, by_status:{…}, success_rate}`.
function normalizeStatistics(raw: unknown): SyncStatistics {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const byStatus = (r.by_status && typeof r.by_status === 'object' ? r.by_status : {}) as Record<string, unknown>
  return {
    total_syncs: num(r.total_syncs ?? r.total_sessions),
    successful_syncs: num(r.successful_syncs ?? byStatus.completed),
    failed_syncs: num(r.failed_syncs ?? byStatus.failed),
    total_items_synced: num(r.total_items_synced ?? r.total_files_synced),
    last_sync_at: r.last_sync_at as string | undefined,
  }
}

export const syncApi = {
  // Wire shape: `{success, data:[…]}` with backend names (`status`, not `enabled`).
  listEndpoints: (): Promise<SyncEndpoint[]> =>
    api.get('/sync/endpoints').then((res) =>
      asList<Record<string, unknown>>(res.data, 'data').map(normalizeEndpoint)
    ),

  getEndpoint: (id: number): Promise<SyncEndpoint> =>
    api.get(`/sync/endpoints/${id}`).then((res) => res.data),

  createEndpoint: (body: CreateEndpointRequest): Promise<SyncEndpoint> =>
    api.post('/sync/endpoints', body).then((res) => res.data),

  updateEndpoint: (id: number, body: UpdateEndpointRequest): Promise<SyncEndpoint> =>
    api.put(`/sync/endpoints/${id}`, body).then((res) => res.data),

  deleteEndpoint: (id: number): Promise<void> =>
    api.delete(`/sync/endpoints/${id}`).then(() => {/* no content */}),

  startSync: (id: number): Promise<SyncSession> =>
    api.post(`/sync/endpoints/${id}/sync`).then((res) => res.data),

  getSessions: (): Promise<SyncSession[]> =>
    api.get('/sync/sessions').then((res) =>
      asList<Record<string, unknown>>(res.data, 'data').map(normalizeSession)
    ),

  getSession: (id: number): Promise<SyncSession> =>
    api.get(`/sync/sessions/${id}`).then((res) => res.data),

  getStatistics: (): Promise<SyncStatistics> =>
    api.get('/sync/statistics').then((res) => normalizeStatistics(unwrapData(res.data))),

  scheduleSync: (body: { endpoint_id: number; cron_expression: string }): Promise<{ message: string }> =>
    api.post('/sync/schedules', body).then((res) => res.data),

  cleanup: (body?: { older_than_days?: number }): Promise<{ message: string }> =>
    api.post('/sync/cleanup', body).then((res) => res.data),
}
