/**
 * Transport-level fake for the shared axios instance, serving responses that
 * were CAPTURED from a running catalog-api (see fixtures/realApi/*.json).
 *
 * Unlike `vi.mock('@/lib/xxxApi')` (which hands a page the shape its author
 * *wished* the backend returned), this replaces only the HTTP adapter, so the
 * real api modules, normalisation code and pages run against the real wire
 * shapes. Contract-unit class evidence only: live evidence comes from the
 * Playwright run against a booted stack.
 */
import api from '@/lib/api'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'

import mediaStats from './fixtures/realApi/media_stats.json'
import mediaRecent from './fixtures/realApi/media_recent.json'
import mediaPopular from './fixtures/realApi/media_popular.json'
import statsOverall from './fixtures/realApi/stats_overall.json'
import statsFiletypes from './fixtures/realApi/stats_filetypes.json'
import statsDuplicates from './fixtures/realApi/stats_duplicates.json'
import statsGrowth from './fixtures/realApi/stats_growth.json'
import analyticsSystem from './fixtures/realApi/analytics_system.json'
import collections from './fixtures/realApi/collections.json'
import adminSystemInfo from './fixtures/realApi/admin_system_info.json'
import adminUsers from './fixtures/realApi/admin_users.json'
import adminStorage from './fixtures/realApi/admin_storage.json'
import adminBackups from './fixtures/realApi/admin_backups.json'
import reportsUsage from './fixtures/realApi/reports_usage.json'
import reportsPerformance from './fixtures/realApi/reports_performance.json'
import scans from './fixtures/realApi/scans.json'
import storageRoots from './fixtures/realApi/storage_roots.json'
import syncEndpoints from './fixtures/realApi/sync_endpoints.json'
import syncStatistics from './fixtures/realApi/sync_statistics.json'
import syncSessions from './fixtures/realApi/sync_sessions.json'
import favorites from './fixtures/realApi/favorites.json'

export const realApiFixtures: Record<string, unknown> = {
  '/media/stats': mediaStats,
  '/media/recent': mediaRecent,
  '/media/popular': mediaPopular,
  '/stats/overall': statsOverall,
  '/stats/filetypes': statsFiletypes,
  '/stats/duplicates': statsDuplicates,
  '/stats/growth': statsGrowth,
  '/analytics/system': analyticsSystem,
  '/collections': collections,
  '/admin/system-info': adminSystemInfo,
  '/admin/users': adminUsers,
  '/admin/storage': adminStorage,
  '/admin/backups': adminBackups,
  '/reports/usage': reportsUsage,
  '/reports/performance': reportsPerformance,
  '/scans': scans,
  '/storage/roots': storageRoots,
  '/sync/endpoints': syncEndpoints,
  '/sync/statistics': syncStatistics,
  '/sync/sessions': syncSessions,
  '/favorites': favorites,
}

export const requested: string[] = []

/** Install the adapter; `overrides` replaces/adds fixtures by path. Returns restore(). */
export function installRealApi(overrides: Record<string, unknown> = {}): () => void {
  const table = { ...realApiFixtures, ...overrides }
  const previous = api.defaults.adapter
  requested.length = 0
  const adapter: AxiosAdapter = async (config: InternalAxiosRequestConfig) => {
    const path = (config.url || '').split('?')[0]
    requested.push(path)
    if (!(path in table)) {
      // Mirror the real backend for routes it does not serve.
      const err = new Error(`Request failed with status code 404 (${path})`) as Error & {
        response?: unknown
        isAxiosError?: boolean
      }
      err.isAxiosError = true
      err.response = { status: 404, data: '404 page not found', headers: {}, config, statusText: 'Not Found' }
      throw err
    }
    return {
      data: JSON.parse(JSON.stringify(table[path])),
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  api.defaults.adapter = adapter
  return () => {
    api.defaults.adapter = previous
  }
}
