/**
 * Roadmap-Item CAT-UI-D2D3 (UI-D3): a 404 on /entities/:id must reach a
 * terminal not-found state promptly (no endless spinner), and a non-404
 * failure must reach a distinct error state with a retry control.
 *
 * Error shape captured LIVE from stack 3 (GET /api/v1/entities/35, HTTP 404):
 *   {"success":false,"error":"Entity not found","details":"media item not found"}
 * The QueryClient mirrors the production defaults of src/main.tsx
 * (retry up to 3 times for anything but 401/403), which is what made the
 * page spin for ~7s of backoff on a plain 404.
 */
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { EntityDetail } from '../EntityDetail'

const notFound = {
  response: {
    status: 404,
    data: { success: false, error: 'Entity not found', details: 'media item not found' },
  },
}
const serverError = { response: { status: 500, data: { success: false, error: 'boom' } } }
const calls = vi.hoisted(() => ({ getEntity: 0 }))
const failure = vi.hoisted(() => ({ err: null as unknown }))

vi.mock('@/lib/mediaApi', () => ({
  entityApi: {
    getEntity: vi.fn(() => {
      calls.getEntity++
      return Promise.reject(failure.err)
    }),
    getEntityChildren: vi.fn().mockResolvedValue({ items: [], total: 0 }),
    getEntityFiles: vi.fn(() => Promise.reject(failure.err)),
    getEntityDuplicates: vi.fn(() => Promise.reject(failure.err)),
    refreshEntityMetadata: vi.fn(),
    updateUserMetadata: vi.fn(),
  },
}))
vi.mock('@/lib/recommendationsApi', () => ({
  recommendationsApi: { getSimilar: vi.fn().mockRejectedValue({ response: { status: 404 } }) },
}))
vi.mock('@/lib/downloadApi', () => ({ downloadApi: { downloadEntity: vi.fn() } }))
vi.mock('@/lib/analyticsApi', () => ({ analyticsApi: { trackAccess: vi.fn().mockResolvedValue({}) } }))
vi.mock('@/contexts/AuthContext', () => ({ useAuth: () => ({ isAdmin: false }) }))
vi.mock('react-hot-toast', () => ({ default: { success: vi.fn(), error: vi.fn() } }))

function renderPage(id = '35') {
  // Production-like defaults (src/main.tsx), minus the long backoff delay of
  // the real client so the test stays fast while keeping the retry count.
  const qc = new QueryClient({
    defaultOptions: {
      queries: {
        retry: (n: number, error: unknown) => {
          const s = (error as { response?: { status?: number } })?.response?.status
          return s === 401 || s === 403 ? false : n < 3
        },
        retryDelay: 50,
      },
    },
  })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/entity/${id}`]}>
        <Routes>
          <Route path="/entity/:id" element={<EntityDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('EntityDetail terminal error states', () => {
  beforeEach(() => {
    calls.getEntity = 0
  })

  it('shows not-found immediately on a 404 and does not retry it', async () => {
    failure.err = notFound
    renderPage()
    expect(await screen.findByText('Entity not found', {}, { timeout: 1000 })).toBeInTheDocument()
    expect(screen.queryByTestId('entity-loading')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /back to browse/i })).toBeInTheDocument()
    expect(calls.getEntity).toBe(1)
  })

  it('shows a distinct error state with a retry control on a 500', async () => {
    failure.err = serverError
    renderPage()
    const alert = await screen.findByRole('alert', {}, { timeout: 2000 })
    expect(alert).toHaveTextContent(/failed to load/i)
    expect(screen.queryByText('Entity not found')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it.each(['abc', '0', '-5'])('shows not-found for invalid id %s without any request', async (bad) => {
    failure.err = notFound
    renderPage(bad)
    expect(await screen.findByText('Entity not found', {}, { timeout: 1000 })).toBeInTheDocument()
    expect(screen.queryByTestId('entity-loading')).not.toBeInTheDocument()
    expect(calls.getEntity).toBe(0)
  })

  it.each([400, 403])('does not retry a %i (all 4xx are terminal)', async (status) => {
    failure.err = { response: { status, data: { success: false, error: 'x' } } }
    renderPage()
    await screen.findByText(/not found|access|invalid/i, {}, { timeout: 1000 })
    await new Promise((r) => setTimeout(r, 300))
    expect(calls.getEntity).toBe(1)
  })

  it('shows a distinct access-denied message for 401/403', async () => {
    failure.err = { response: { status: 403, data: { success: false, error: 'forbidden' } } }
    renderPage()
    expect(await screen.findByText(/do not have access/i, {}, { timeout: 1000 })).toBeInTheDocument()
    expect(screen.queryByText('Entity not found')).not.toBeInTheDocument()
  })
})
