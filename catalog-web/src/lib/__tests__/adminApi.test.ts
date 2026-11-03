import { adminApi } from '../adminApi'
import { api } from '../api'

vi.mock('../api', () => {
  const mockApi = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  }
  return { __esModule: true, default: mockApi, api: mockApi }
})

const notFound = { response: { status: 404 } }
const mocked = api as unknown as Record<'get' | 'post' | 'put', ReturnType<typeof vi.fn>>

describe('adminApi (no fabricated fallbacks)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('passes server data through unchanged, including nulls', async () => {
    const payload = {
      version: '2.1.0',
      uptime: 5,
      cpuUsage: null,
      memoryUsage: 3,
      diskUsage: null,
      activeConnections: 0,
      totalRequests: null,
      unavailable: ['cpuUsage', 'diskUsage', 'totalRequests'],
    }
    mocked.get.mockResolvedValue({ data: payload })
    await expect(adminApi.getSystemInfo()).resolves.toEqual(payload)
  })

  it.each([
    ['getSystemInfo', () => adminApi.getSystemInfo()],
    ['getUsers', () => adminApi.getUsers()],
    ['getStorageInfo', () => adminApi.getStorageInfo()],
    ['getBackups', () => adminApi.getBackups()],
  ])('%s propagates a 404 instead of returning mock data', async (_n, call) => {
    mocked.get.mockRejectedValue(notFound)
    await expect(call()).rejects.toBe(notFound)
  })

  it.each([
    ['createBackup', () => adminApi.createBackup('full'), 'post'],
    ['restoreBackup', () => adminApi.restoreBackup('1'), 'post'],
    ['scanStorage', () => adminApi.scanStorage('/x'), 'post'],
    ['updateUser', () => adminApi.updateUser('1', {}), 'put'],
  ] as const)('%s does not report success when the API answers 404', async (_n, call, verb) => {
    mocked[verb].mockRejectedValue(notFound)
    await expect(call()).rejects.toBe(notFound)
  })

  it('sends the documented requests', async () => {
    mocked.post.mockResolvedValue({ data: {} })
    await adminApi.createBackup('incremental')
    expect(mocked.post).toHaveBeenCalledWith('/admin/backups', { type: 'incremental' })
  })
})
