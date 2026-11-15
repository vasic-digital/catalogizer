import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConversionTools } from '../ConversionTools'
import { AuthProvider } from '@/contexts/AuthContext'
import { authApi } from '@/lib/api'

// SSEF4R-FD-F3: the in-page conversion actions follow the server's handler
// permissions (conversion.create, conversion.manage). REAL AuthProvider + REAL
// ConversionTools + REAL FormatConverter; only the HTTP edge is replaced
// (unit-class evidence, not a live-stack proof).
vi.mock('@/lib/api', async () => ({
  authApi: { getAuthStatus: vi.fn(), getPermissions: vi.fn(), login: vi.fn(), register: vi.fn(), logout: vi.fn(), updateProfile: vi.fn(), changePassword: vi.fn() },
}))
vi.mock('react-hot-toast', async () => ({ __esModule: true, default: { success: vi.fn(), error: vi.fn() } }))

const jobs = [
  { id: 'f1', sourceFile: { path: '/m/a.mov', name: 'a.mov', format: 'mov', size: 1 }, targetFormat: 'webm', quality: 'low', status: 'failed', progress: 0, startTime: '2024-01-01T12:00:00Z', error: 'x', outputFile: '', options: {} },
  { id: 'p1', sourceFile: { path: '/m/b.mkv', name: 'b.mkv', format: 'mkv', size: 1 }, targetFormat: 'mp4', quality: 'high', status: 'processing', progress: 40, startTime: '2024-01-01T12:00:00Z', outputFile: '', options: {} },
]
vi.mock('@/lib/conversionApi', async () => ({
  conversionApi: {
    getConversionJobs: vi.fn(() => Promise.resolve(jobs)),
    startConversion: vi.fn(), cancelConversion: vi.fn(), retryConversion: vi.fn(), downloadFile: vi.fn(),
  },
}))

const api = vi.mocked(authApi)
const user = { id: 5, username: 'ops', email: 'o@x', first_name: 'O', last_name: 'P', role_id: 9, role: { id: 9, name: 'Curator' }, is_active: true }

async function mount(perms: string[]) {
  api.getAuthStatus.mockResolvedValue({ authenticated: true, user, permissions: perms } as never)
  api.getPermissions.mockResolvedValue({ permissions: perms } as never)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <ConversionTools />
      </AuthProvider>
    </QueryClientProvider>
  )
  await screen.findByText('a.mov') // jobs rendered
  await waitFor(() => expect(qc.getQueryState(['permissions'])?.status).toBe('success'))
}

describe('ConversionTools in-page gates with the real AuthProvider (SSEF4R-FD-F3)', () => {
  it('conversion.view only: no create, no Retry, no Cancel', async () => {
    await mount(['conversion.view'])
    expect(screen.queryByRole('button', { name: /New Conversion/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Retry/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Cancel$/ })).not.toBeInTheDocument()
  })
  it('conversion.create: create shown, manage actions still hidden', async () => {
    await mount(['conversion.view', 'conversion.create'])
    expect(screen.getByRole('button', { name: /New Conversion/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Retry/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Cancel$/ })).not.toBeInTheDocument()
  })
  it('conversion.manage: Retry and Cancel shown, create hidden', async () => {
    await mount(['conversion.view', 'conversion.manage'])
    expect(screen.getByRole('button', { name: /Retry/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Cancel$/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /New Conversion/ })).not.toBeInTheDocument()
  })
  it('conversion.* (prefix wildcard) shows everything', async () => {
    await mount(['conversion.*'])
    expect(screen.getByRole('button', { name: /New Conversion/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Retry/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Cancel$/ })).toBeInTheDocument()
  })
  it('media.* does NOT grant conversion actions (server wildcard rule)', async () => {
    await mount(['media.*'])
    expect(screen.queryByRole('button', { name: /New Conversion/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Retry/ })).not.toBeInTheDocument()
  })
})
