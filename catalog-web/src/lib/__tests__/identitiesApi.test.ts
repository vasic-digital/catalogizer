import { identitiesApi } from '../identitiesApi'
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

const mockIdentity = {
  id: 1,
  name: 'nas-admin',
  type: 'credentials',
  username: 'admin',
  secret_ref: 'sec_abc123',
  domain: null,
  key_path: null,
  enabled: true,
  priority: 10,
  created_at: '2026-06-26T00:00:00Z',
  updated_at: '2026-06-26T00:00:00Z',
}

describe('identitiesApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('list', () => {
    // Roadmap-Item CAT-UI-D2D3 (UI-D2). catalog-api has no /identities CRUD
    // route (live: GET /api/v1/identities -> 404 "404 page not found"); the only
    // identity listing is the admin-only, secret-free GET /api/v1/smb/identities
    // (handlers.ListIdentityInfo: index, kind, username, label), live body `[]`
    // on stack 3 when no identities are configured in the server environment.
    it('lists identities from GET /smb/identities (never the non-existent /identities)', async () => {
      mockApi.get.mockResolvedValue({
        data: [
          { index: 1, kind: 'credentials', username: 'nas-admin', label: 'nas-admin (SMB)' },
          { index: 2, kind: 'guest', username: '', label: 'guest' },
        ],
      })

      const result = await identitiesApi.list()

      expect(mockApi.get).toHaveBeenCalledTimes(1)
      expect(mockApi.get).toHaveBeenCalledWith('/smb/identities')
      expect(result).toEqual([
        expect.objectContaining({ id: 1, name: 'nas-admin (SMB)', type: 'credentials', username: 'nas-admin', secret_ref: null, enabled: true }),
        expect.objectContaining({ id: 2, name: 'guest', type: 'anonymous', username: null, secret_ref: null, enabled: true }),
      ])
      expect(JSON.stringify(result)).not.toMatch(/password|secret":"/i)
    })

    it('returns an empty list for the live empty response `[]`', async () => {
      mockApi.get.mockResolvedValue({ data: [] })
      await expect(identitiesApi.list()).resolves.toEqual([])
    })

    it('propagates errors', async () => {
      mockApi.get.mockRejectedValue(new Error('Fetch failed'))

      await expect(identitiesApi.list()).rejects.toThrow('Fetch failed')
    })
  })

  describe('get', () => {
    it('calls GET /identities/:id and returns identity', async () => {
      mockApi.get.mockResolvedValue({ data: mockIdentity })

      const result = await identitiesApi.get(1)

      expect(mockApi.get).toHaveBeenCalledWith('/identities/1')
      expect(result).toEqual(mockIdentity)
    })
  })

  describe('create', () => {
    it('calls POST /identities with request body', async () => {
      const body = {
        name: 'new-identity',
        type: 'credentials' as const,
        username: 'user',
        secret: 's3cret!',
        enabled: true,
        priority: 5,
      }
      const created = { ...mockIdentity, id: 2, name: 'new-identity' }
      mockApi.post.mockResolvedValue({ data: created })

      const result = await identitiesApi.create(body)

      expect(mockApi.post).toHaveBeenCalledWith('/identities', body)
      expect(result).toEqual(created)
    })

    it('propagates errors on create failure', async () => {
      mockApi.post.mockRejectedValue(new Error('Bad request'))

      await expect(
        identitiesApi.create({ name: 'x', type: 'api_token', secret: 'tok' })
      ).rejects.toThrow('Bad request')
    })
  })

  describe('update', () => {
    it('calls PUT /identities/:id with partial body', async () => {
      const update = { name: 'updated-name', priority: 20 }
      const updated = { ...mockIdentity, ...update }
      mockApi.put.mockResolvedValue({ data: updated })

      const result = await identitiesApi.update(1, update)

      expect(mockApi.put).toHaveBeenCalledWith('/identities/1', update)
      expect(result).toEqual(updated)
    })
  })

  describe('remove', () => {
    it('calls DELETE /identities/:id', async () => {
      mockApi.delete.mockResolvedValue({ data: {} })

      await identitiesApi.remove(1)

      expect(mockApi.delete).toHaveBeenCalledWith('/identities/1')
    })

    it('returns undefined on success', async () => {
      mockApi.delete.mockResolvedValue({ data: {} })

      const result = await identitiesApi.remove(1)

      expect(result).toBeUndefined()
    })
  })

  describe('scanNetwork (STUB)', () => {
    it('calls POST /discovery/scan', async () => {
      mockApi.post.mockResolvedValue({
        data: {
          hosts_found: 0,
          shares_found: 0,
          bindings_created: 0,
          scan_duration_ms: 0,
        },
      })

      await identitiesApi.scanNetwork()

      expect(mockApi.post).toHaveBeenCalledWith('/discovery/scan')
    })

    it('propagates 404 from not-yet-implemented endpoint', async () => {
      const error = { response: { status: 404 }, message: 'Not Found' }
      mockApi.post.mockRejectedValue(error)

      await expect(identitiesApi.scanNetwork()).rejects.toEqual(error)
    })
  })

  describe('listHosts (STUB)', () => {
    it('calls GET /discovery/hosts', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      const result = await identitiesApi.listHosts()

      expect(mockApi.get).toHaveBeenCalledWith('/discovery/hosts')
      expect(result).toEqual([])
    })
  })

  describe('listShares (STUB)', () => {
    it('calls GET /discovery/shares without filter', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      const result = await identitiesApi.listShares()

      expect(mockApi.get).toHaveBeenCalledWith('/discovery/shares', {
        params: undefined,
      })
      expect(result).toEqual([])
    })

    it('passes host_id filter when provided', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      await identitiesApi.listShares(42)

      expect(mockApi.get).toHaveBeenCalledWith('/discovery/shares', {
        params: { host_id: 42 },
      })
    })
  })

  describe('probeShare (STUB)', () => {
    it('calls POST /discovery/shares/:id/probe', async () => {
      const result = {
        share_id: 1,
        share_name: 'Data',
        protocol: 'smb',
        host: '192.168.0.108',
        anonymous_ok: false,
        bound_identity_id: 1,
        bound_identity_name: 'nas-admin',
        status: 'ok' as const,
        evidence_path: null,
      }
      mockApi.post.mockResolvedValue({ data: result })

      const res = await identitiesApi.probeShare(1)

      expect(mockApi.post).toHaveBeenCalledWith('/discovery/shares/1/probe')
      expect(res).toEqual(result)
    })
  })

  describe('listBindings (STUB)', () => {
    it('calls GET /discovery/bindings', async () => {
      mockApi.get.mockResolvedValue({ data: [] })

      const result = await identitiesApi.listBindings()

      expect(mockApi.get).toHaveBeenCalledWith('/discovery/bindings', {
        params: undefined,
      })
      expect(result).toEqual([])
    })
  })
})
