// UI-D1 regression: /conversion crashed with "Cannot read properties of undefined (reading 'name')"
// because GET /conversion/jobs returns the flat catalog-api model (source_path, target_format, ...)
// while the page expected a nested sourceFile. These tests render the REAL ConversionTools,
// FormatConverter and conversionApi; only the HTTP client is replaced, answering with response
// bodies captured from the live QA stack (fixtures/conversion_jobs_live.json).
import userEvent from '@testing-library/user-event'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConversionTools } from '../ConversionTools'
import { FormatConverter } from '@/components/conversion/FormatConverter'
import { mapConversionJob } from '@/lib/conversionApi'
import liveJobs from './fixtures/conversion_jobs_live.json'

const mockGet = vi.fn()
const mockDownload = vi.fn()
vi.mock('@/lib/api', () => ({
  api: { get: (...a: any[]) => (String(a[0]).endsWith('/download') ? mockDownload(...a) : mockGet(...a)), post: vi.fn(), delete: vi.fn() },
  default: { get: (...a: any[]) => mockGet(...a) },
}))

const renderPage = () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <ConversionTools />
    </QueryClientProvider>,
  )
}

describe('ConversionTools with the live API response shape (UI-D1)', () => {
  beforeEach(() => mockGet.mockReset())

  it('renders every live job without throwing', async () => {
    mockGet.mockResolvedValue({ data: liveJobs })
    renderPage()
    await waitFor(() => expect(screen.getAllByText('Plain Tone.wav').length).toBe(liveJobs.length))
    expect(screen.getAllByText(/WAV → MP3/).length).toBeGreaterThan(0)
  })

  it('maps flat API fields into the nested job the UI renders', () => {
    const j = mapConversionJob(liveJobs.find((x: any) => x.status === 'failed'))
    expect(j.sourceFile.name).toBe('Plain Tone.wav')
    expect(j.sourceFile.format).toBe('wav')
    expect(j.targetFormat).toBe('mp3')
    expect(j.status).toBe('failed')
    expect(j.options).toEqual({})
    expect(typeof j.id).toBe('string')
  })

  it.each([
    ['empty array', []],
    ['null body', null],
    ['object without jobs', {}],
    ['envelope with jobs', { jobs: [] }],
  ])('does not crash on %s', async (_n, body) => {
    mockGet.mockResolvedValue({ data: body })
    renderPage()
    await waitFor(() => expect(screen.getByText('No conversion jobs')).toBeInTheDocument())
  })

  it('does not crash on a job missing every optional part', async () => {
    mockGet.mockResolvedValue({ data: [{ id: 7 }, null, { id: 8, status: 'running' }] })
    renderPage()
    await waitFor(() => expect(screen.getAllByText('unknown').length).toBeGreaterThan(0))
  })

  it('maps API statuses onto UI statuses', () => {
    expect(mapConversionJob({ id: 1, status: 'running' }).status).toBe('processing')
    expect(mapConversionJob({ id: 1, status: 'completed' }).status).toBe('completed')
      })

  it('FormatConverter itself tolerates a job without sourceFile/options', () => {
    render(<FormatConverter jobs={[{ id: 'x', status: 'pending', quality: 'low', targetFormat: 'mp3', progress: 0 } as any]} supportedFormats={['mp3']} />)
    expect(screen.getByText('unknown')).toBeInTheDocument()
  })

  it('renders error_message for failed live jobs', async () => {
    mockGet.mockResolvedValue({ data: liveJobs })
    renderPage()
    const failed = (liveJobs as any[]).filter(j => j.status === 'failed')
    expect(failed.length).toBeGreaterThan(0)
    const norm = (t: string) => t.replace(/\s+/g, ' ').trim()
    await waitFor(() => expect(screen.getAllByText((_c, el) => !!el && el.children.length === 0 && norm(el.textContent ?? '') === norm(failed[0].error_message)).length).toBeGreaterThan(0))
  })

  it('Download goes by job id, only for completed jobs (live shape)', async () => {
    mockGet.mockResolvedValue({ data: liveJobs })
    mockDownload.mockClear()
    renderPage()
    await waitFor(() => expect(screen.getAllByText('Plain Tone.wav').length).toBe(liveJobs.length))
    const completed = (liveJobs as any[]).filter(j => j.status === 'completed').length
    const buttons = screen.getAllByRole('button', { name: /download/i })
    expect(buttons.length).toBe(completed)
    await userEvent.click(buttons[0])
    const firstCompleted = (liveJobs as any[]).find(j => j.status === 'completed')
    await waitFor(() => expect(mockDownload).toHaveBeenCalledWith(`/conversion/jobs/${firstCompleted.id}/download`, expect.anything()))
  })

  it('unknown future status shows neutral state without Cancel', async () => {
    expect(mapConversionJob({ id: 1, status: 'weird' }).status).toBe('unknown')
    mockGet.mockResolvedValue({ data: [{ id: 9, status: 'weird', source_path: '/a/b.wav' }] })
    renderPage()
    await waitFor(() => expect(screen.getByText('b.wav')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: /cancel/i })).toBeNull()
    expect(screen.getByText('Unknown')).toBeInTheDocument()
  })

  it('jobs with missing ids get unique React keys (no duplicate-key warning)', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    mockGet.mockResolvedValue({ data: [{ source_path: '/a/x.wav' }, { id: null, source_path: '/a/y.wav' }] })
    renderPage()
    await waitFor(() => expect(screen.getByText('y.wav')).toBeInTheDocument())
    expect(spy.mock.calls.some(c => String(c[0]).includes('same key'))).toBe(false)
    spy.mockRestore()
  })
})
