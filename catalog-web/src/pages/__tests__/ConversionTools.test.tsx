import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConversionTools } from '../ConversionTools'
import toast from 'react-hot-toast'

// Page-level tests that do not exercise gating: the user holds every conversion permission (gating has its own real-auth test).
vi.mock('@/contexts/AuthContext', () => ({ useAuth: () => ({ hasPermission: () => true }) }))

const mockGetConversionJobs = vi.fn()
const mockStartConversion = vi.fn()
const mockCancelConversion = vi.fn()
const mockRetryConversion = vi.fn()
const mockDownloadFile = vi.fn()

// Mock conversionApi
vi.mock('@/lib/conversionApi', () => ({
  conversionApi: {
    getConversionJobs: (...args: any[]) => mockGetConversionJobs(...args),
    startConversion: (...args: any[]) => mockStartConversion(...args),
    cancelConversion: (...args: any[]) => mockCancelConversion(...args),
    retryConversion: (...args: any[]) => mockRetryConversion(...args),
    downloadFile: (...args: any[]) => mockDownloadFile(...args),
  },
}))

vi.mock('react-hot-toast', () => ({
  __esModule: true,
  default: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock FormatConverter to expose its callback props
vi.mock('@/components/conversion/FormatConverter', () => ({
  FormatConverter: ({ jobs, supportedFormats, onStartConversion, onCancelConversion, onRetryConversion, onDownloadFile }: any) => (
    <div data-testid="format-converter">
      <div data-testid="jobs-count">{jobs.length} jobs</div>
      <div data-testid="formats-count">{supportedFormats.length} formats</div>
      {jobs.map((job: any) => (
        <div key={job.id} data-testid={`job-${job.id}`}>
          <span>{job.sourceFile.name}</span>
          <span data-testid={`job-status-${job.id}`}>{job.status}</span>
          {job.status === 'completed' && (
            <button onClick={() => onDownloadFile(job.outputFile)}>Download</button>
          )}
          {job.status === 'processing' && (
            <button onClick={() => onCancelConversion(job.id)}>Cancel</button>
          )}
          {job.status === 'failed' && (
            <button onClick={() => onRetryConversion(job.id)}>Retry</button>
          )}
        </div>
      ))}
      <button onClick={() => onStartConversion({ sourceFile: '/test.mkv', targetFormat: 'mp4', quality: 'high' })}>
        New Conversion
      </button>
    </div>
  ),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  })
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  Wrapper.displayName = 'TestWrapper'
  return Wrapper
}

const completedJob = {
  id: '1',
  sourceFile: { path: '/media/test.mkv', name: 'test.mkv', format: 'mkv', size: 1073741824 },
  targetFormat: 'mp4',
  quality: 'high',
  status: 'completed',
  progress: 100,
  startTime: '2024-01-01T10:00:00Z',
  endTime: '2024-01-01T10:15:00Z',
  outputFile: '/media/converted/test.mp4',
  options: { resolution: '1080p', bitrate: 5000, framerate: 30, audioCodec: 'aac', videoCodec: 'h264' },
}

const processingJob = {
  id: '2',
  sourceFile: { path: '/media/video.avi', name: 'video.avi', format: 'avi', size: 524288000 },
  targetFormat: 'mp4',
  quality: 'medium',
  status: 'processing',
  progress: 45,
  startTime: '2024-01-01T11:00:00Z',
  outputFile: '',
  options: { resolution: '720p', bitrate: 3000, framerate: 24, audioCodec: 'aac', videoCodec: 'h264' },
}

const failedJob = {
  id: '3',
  sourceFile: { path: '/media/broken.mov', name: 'broken.mov', format: 'mov', size: 200000000 },
  targetFormat: 'webm',
  quality: 'low',
  status: 'failed',
  progress: 0,
  startTime: '2024-01-01T12:00:00Z',
  error: 'Codec not supported',
  outputFile: '',
  options: { resolution: '480p', bitrate: 1000, framerate: 24, audioCodec: 'opus', videoCodec: 'vp9' },
}

describe('ConversionTools Page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetConversionJobs.mockResolvedValue([completedJob])
    mockStartConversion.mockResolvedValue({ id: '4', status: 'pending', sourceFile: { name: 'new.mkv' } })
    mockCancelConversion.mockResolvedValue(undefined)
    mockRetryConversion.mockResolvedValue(undefined)
    mockDownloadFile.mockResolvedValue(undefined)
  })

  it('renders page heading', async () => {
    render(<ConversionTools />, { wrapper: createWrapper() })
    expect(screen.getByText('Format Converter')).toBeInTheDocument()
  })

  it('renders page description', () => {
    render(<ConversionTools />, { wrapper: createWrapper() })
    expect(
      screen.getByText(/Convert media files to different formats/)
    ).toBeInTheDocument()
  })

  it('renders FormatConverter component', () => {
    render(<ConversionTools />, { wrapper: createWrapper() })
    expect(screen.getByTestId('format-converter')).toBeInTheDocument()
  })

  it('loads and displays conversion jobs', async () => {
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('test.mkv')).toBeInTheDocument()
    })
  })

  it('wraps content in max-width container', () => {
    const { container } = render(<ConversionTools />, { wrapper: createWrapper() })
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper).toHaveClass('max-w-7xl')
  })

  it('displays completed job status', async () => {
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByTestId('job-status-1')).toHaveTextContent('completed')
    })
  })

  it('shows download button for completed jobs', async () => {
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Download')).toBeInTheDocument()
    })
  })

  // --- New tests for increased coverage ---

  it('passes supported formats to FormatConverter', async () => {
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByTestId('formats-count')).toHaveTextContent('8 formats')
    })
  })

  it('starts a new conversion successfully', async () => {
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('New Conversion')).toBeInTheDocument()
    })

    await user.click(screen.getByText('New Conversion'))

    await waitFor(() => {
      expect(mockStartConversion).toHaveBeenCalledWith({
        sourceFile: '/test.mkv',
        targetFormat: 'mp4',
        quality: 'high',
      })
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Conversion started successfully')
    })
  })

  it('shows error toast when starting conversion fails', async () => {
    mockStartConversion.mockRejectedValue(new Error('Conversion failed'))
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('New Conversion')).toBeInTheDocument()
    })

    await user.click(screen.getByText('New Conversion'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to start conversion: Conversion failed')
    })
  })

  it('shows generic error message for non-Error rejection', async () => {
    mockStartConversion.mockRejectedValue('string error')
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('New Conversion')).toBeInTheDocument()
    })

    await user.click(screen.getByText('New Conversion'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to start conversion: Unknown error')
    })
  })

  it('cancels a processing conversion successfully', async () => {
    mockGetConversionJobs.mockResolvedValue([processingJob])
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Cancel')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Cancel'))

    await waitFor(() => {
      expect(mockCancelConversion).toHaveBeenCalledWith('2')
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Conversion cancelled')
    })
  })

  it('shows error toast when cancel conversion fails', async () => {
    mockGetConversionJobs.mockResolvedValue([processingJob])
    mockCancelConversion.mockRejectedValue(new Error('Cannot cancel'))
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Cancel')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Cancel'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to cancel conversion: Cannot cancel')
    })
  })

  it('retries a failed conversion successfully', async () => {
    mockGetConversionJobs.mockResolvedValue([failedJob])
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Retry')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(mockRetryConversion).toHaveBeenCalledWith('3')
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Conversion retry initiated')
    })
  })

  it('shows error toast when retry conversion fails', async () => {
    mockGetConversionJobs.mockResolvedValue([failedJob])
    mockRetryConversion.mockRejectedValue(new Error('Retry failed'))
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Retry')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to retry conversion: Retry failed')
    })
  })

  it('downloads a completed file successfully', async () => {
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Download')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Download'))

    await waitFor(() => {
      expect(mockDownloadFile).toHaveBeenCalledWith('/media/converted/test.mp4')
    })

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('File downloaded successfully')
    })
  })

  it('shows error toast when download fails', async () => {
    mockDownloadFile.mockRejectedValue(new Error('Download failed'))
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('Download')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Download'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Failed to download file: Download failed')
    })
  })

  it('displays multiple jobs when available', async () => {
    mockGetConversionJobs.mockResolvedValue([completedJob, processingJob, failedJob])
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByTestId('jobs-count')).toHaveTextContent('3 jobs')
    })

    expect(screen.getByText('test.mkv')).toBeInTheDocument()
    expect(screen.getByText('video.avi')).toBeInTheDocument()
    expect(screen.getByText('broken.mov')).toBeInTheDocument()
  })

  it('displays zero jobs initially before data loads', () => {
    mockGetConversionJobs.mockReturnValue(new Promise(() => { /* noop */ })) // Never resolves
    render(<ConversionTools />, { wrapper: createWrapper() })

    expect(screen.getByTestId('jobs-count')).toHaveTextContent('0 jobs')
  })

  it('updates jobs list when new conversion is started', async () => {
    mockGetConversionJobs.mockResolvedValue([completedJob])
    const newJob = {
      id: '4',
      sourceFile: { path: '/new.mkv', name: 'new.mkv', format: 'mkv', size: 500000 },
      targetFormat: 'mp4',
      quality: 'high',
      status: 'pending',
      progress: 0,
      outputFile: '',
      options: {},
    }
    mockStartConversion.mockResolvedValue(newJob)
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByTestId('jobs-count')).toHaveTextContent('1 jobs')
    })

    await user.click(screen.getByText('New Conversion'))

    await waitFor(() => {
      expect(screen.getByTestId('jobs-count')).toHaveTextContent('2 jobs')
    })
  })

  it('updates job status to cancelled after cancel', async () => {
    mockGetConversionJobs.mockResolvedValue([processingJob])
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByTestId('job-status-2')).toHaveTextContent('processing')
    })

    await user.click(screen.getByText('Cancel'))

    await waitFor(() => {
      expect(screen.getByTestId('job-status-2')).toHaveTextContent('cancelled')
    })
  })

  it('updates job status to pending after retry', async () => {
    mockGetConversionJobs.mockResolvedValue([failedJob])
    const user = userEvent.setup()
    render(<ConversionTools />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByTestId('job-status-3')).toHaveTextContent('failed')
    })

    await user.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(screen.getByTestId('job-status-3')).toHaveTextContent('pending')
    })
  })
})
