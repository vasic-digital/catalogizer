import api from './api'
import { asList } from './apiShapes'

export interface MediaAccessEvent {
  media_id: number
  action: 'view' | 'play' | 'download'
  duration_ms?: number
}

export interface TrackEventRequest {
  event_type: string
  event_data?: Record<string, unknown>
}

export interface UserAnalytics {
  user_id: number
  total_views: number
  total_plays: number
  total_downloads: number
  favorite_types: Record<string, number>
  recent_activity: { date: string; count: number }[]
}

export interface SystemAnalytics {
  total_events: number
  active_users_24h: number
  popular_media: { media_id: number; title: string; access_count: number }[]
  events_by_type: Record<string, number>
}

export interface MediaAnalytics {
  media_id: number
  view_count: number
  play_count: number
  download_count: number
  unique_users: number
}

function normalizeSystemAnalytics(raw: unknown): SystemAnalytics {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const n = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : 0)
  const top = asList<Record<string, unknown>>(r.popular_media ?? r.top_accessed_media)
  return {
    total_events: n(r.total_events),
    active_users_24h: n(r.active_users_24h ?? r.active_users),
    popular_media: top.map((m) => ({
      media_id: n(m.media_id),
      title: typeof m.title === 'string' && m.title ? m.title : `Media #${n(m.media_id)}`,
      access_count: n(m.access_count),
    })),
    events_by_type: (r.events_by_type && typeof r.events_by_type === 'object' ? r.events_by_type : {}) as Record<string, number>,
  }
}

export const analyticsApi = {
  trackAccess: (body: MediaAccessEvent): Promise<{ message: string }> =>
    api.post('/analytics/access', body).then((res) => res.data),

  trackEvent: (body: TrackEventRequest): Promise<{ message: string }> =>
    api.post('/analytics/event', body).then((res) => res.data),

  getUserAnalytics: (userId: number): Promise<UserAnalytics> =>
    api.get(`/analytics/user/${userId}`).then((res) => res.data),

  // Wire shape: `{total_events, active_users, top_accessed_media:[{media_id,access_count}]|null, …}`.
  getSystemAnalytics: (): Promise<SystemAnalytics> =>
    api.get('/analytics/system').then((res) => normalizeSystemAnalytics(res.data)),

  getMediaAnalytics: (mediaId: number): Promise<MediaAnalytics> =>
    api.get(`/analytics/media/${mediaId}`).then((res) => res.data),

  createReport: (body: { report_type: string; date_range?: { from: string; to: string } }): Promise<{ report_id: string; message: string }> =>
    api.post('/analytics/reports', body).then((res) => res.data),
}
