/**
 * Typed API client for the Identity + Network-Share Discovery endpoints.
 *
 * Per DESIGN.md §2.5, endpoints live under `/api/v1`:
 *   - /identities           CRUD
 *   - /discovery/scan       kick-off network sweep
 *   - /discovery/hosts      enumerated hosts
 *   - /discovery/shares     enumerated shares
 *   - /discovery/shares/:id/probe  anon-first + identity-fallback
 *   - /discovery/bindings   share-identity remembered pairings
 *
 * Methods targeting not-yet-implemented backend endpoints are marked with
 * a **STUB** comment — they have the correct return type and route shape
 * but will reject with a descriptive error, NEVER fake a real response.
 *
 * §11.4.10 — no credential value is EVER logged or rendered client-side.
 * The `secret` field in IdentityRequest is sent to the API once; the
 * response never echoes it.
 */

import api from './api'
import type {
  Identity,
  IdentityRequest,
  DiscoveredHost,
  DiscoveredShare,
  ShareIdentityBinding,
  ProbeResult,
  ScanResult,
} from '@/types/identity'

/** Wire shape of GET /api/v1/smb/identities (handlers.ListIdentityInfo). */
interface SmbIdentityInfo {
  index: number
  kind: string
  username: string
  label: string
}

export const identitiesApi = {
  // --- Identity CRUD ---

  /**
   * catalog-api exposes no /identities CRUD route. The identities it knows are
   * loaded from the server environment and listed, without secrets, by the
   * admin-only GET /smb/identities ({index, kind, username, label}); map that
   * read-only view onto the Identity shape. Create/update/remove below still
   * target routes the backend does not provide.
   */
  list: (): Promise<Identity[]> =>
    api.get<SmbIdentityInfo[]>('/smb/identities').then((r) =>
      (r.data ?? []).map(
        (i): Identity => ({
          id: i.index,
          name: i.label,
          type: i.kind === 'guest' ? 'anonymous' : 'credentials',
          username: i.username || null,
          secret_ref: null,
          domain: null,
          key_path: null,
          enabled: true,
          priority: i.index,
        })
      )
    ),

  get: (id: number): Promise<Identity> =>
    api.get(`/identities/${id}`).then((r) => r.data),

  create: (body: IdentityRequest): Promise<Identity> =>
    api.post('/identities', body).then((r) => r.data),

  update: (id: number, body: Partial<IdentityRequest>): Promise<Identity> =>
    api.put(`/identities/${id}`, body).then((r) => r.data),

  remove: (id: number): Promise<void> =>
    api.delete(`/identities/${id}`).then(() => undefined),

  // --- Discovery ---

  /**
   * Kick off a network scan (host + share enumeration).
   * STUB — backend endpoint not yet implemented.
   */
  scanNetwork: (): Promise<ScanResult> =>
    api.post('/discovery/scan').then((r) => r.data),

  /**
   * List discovered hosts.
   * STUB — backend endpoint not yet implemented.
   */
  listHosts: (): Promise<DiscoveredHost[]> =>
    api.get('/discovery/hosts').then((r) => r.data),

  /**
   * List discovered shares (optionally filtered by host_id).
   * STUB — backend endpoint not yet implemented.
   */
  listShares: (hostId?: number): Promise<DiscoveredShare[]> => {
    const params = hostId ? { host_id: hostId } : undefined
    return api.get('/discovery/shares', { params }).then((r) => r.data)
  },

  /**
   * Probe a specific share: anonymous-first, then each enabled identity
   * by priority until one authenticates.
   * STUB — backend endpoint not yet implemented.
   */
  probeShare: (shareId: number): Promise<ProbeResult> =>
    api.post(`/discovery/shares/${shareId}/probe`).then((r) => r.data),

  /**
   * List all share-identity bindings.
   * STUB — backend endpoint not yet implemented.
   */
  listBindings: (shareId?: number): Promise<ShareIdentityBinding[]> => {
    const params = shareId ? { share_id: shareId } : undefined
    return api.get('/discovery/bindings', { params }).then((r) => r.data)
    // STUB
  },
}

export default identitiesApi
