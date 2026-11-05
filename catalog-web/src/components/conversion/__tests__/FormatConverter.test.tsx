import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FormatConverter } from '../FormatConverter'

const completedJob = {
  id: '1',
  sourceFile: { path: '/media/test.mkv', name: 'test.mkv', format: 'mkv', size: 1073741824 },
  targetFormat: 'mp4',
  quality: 'high' as const,
  status: 'completed' as const,
  progress: 100,
  startTime: '2024-01-01T10:00:00Z',
  endTime: '2024-01-01T10:15:00Z',
  outputFile: '/media/converted/test.mp4',
  options: { resolution: '1080p', bitrate: 5000, framerate: 30, audioCodec: 'aac', videoCodec: 'h264' },
}

const processingJob = {
  id: '2',
  sourceFile: { path: '/media/video.avi', name: 'video.avi', format: 'avi', size: 2147483648 },
  targetFormat: 'mp4',
  quality: 'medium' as const,
  status: 'processing' as const,
  progress: 65,
  startTime: '2024-01-01T11:00:00Z',
  options: { resolution: '720p', bitrate: 2500, framerate: 30, audioCodec: 'aac', videoCodec: 'h264' },
}

const failedJob = {
  id: '3',
  sourceFile: { path: '/media/bad.avi', name: 'bad.avi', format: 'avi', size: 500000 },
  targetFormat: 'mp4',
  quality: 'low' as const,
  status: 'failed' as const,
  progress: 0,
  error: 'Unsupported codec',
  options: { resolution: '480p', bitrate: 1000, framerate: 30, audioCodec: 'aac', videoCodec: 'h264' },
}

const supportedFormats = ['mp4', 'mkv', 'avi', 'mov', 'webm']

describe('FormatConverter', () => {
  it('renders the Format Converter heading', () => {
    render(<FormatConverter jobs={[]} supportedFormats={supportedFormats} />)
    expect(screen.getByText('Format Converter')).toBeInTheDocument()
  })

  it('renders New Conversion button', () => {
    render(<FormatConverter jobs={[]} supportedFormats={supportedFormats} />)
    expect(screen.getByText('New Conversion')).toBeInTheDocument()
  })

  it('shows empty state when no jobs', () => {
    render(<FormatConverter jobs={[]} supportedFormats={supportedFormats} />)
    expect(screen.getByText('No conversion jobs')).toBeInTheDocument()
    expect(screen.getByText('Start your first media format conversion')).toBeInTheDocument()
  })

  it('renders completed job with download button', () => {
    render(<FormatConverter jobs={[completedJob]} supportedFormats={supportedFormats} />)
    expect(screen.getByText('test.mkv')).toBeInTheDocument()
    expect(screen.getByText('Download')).toBeInTheDocument()
    expect(screen.getByText('Completed')).toBeInTheDocument()
  })

  it('shows success message for completed jobs', () => {
    render(<FormatConverter jobs={[completedJob]} supportedFormats={supportedFormats} />)
    expect(screen.getByText(/Conversion completed successfully/)).toBeInTheDocument()
  })

  it('renders processing job with progress and cancel button', () => {
    render(<FormatConverter jobs={[processingJob]} supportedFormats={supportedFormats} />)
    expect(screen.getByText('video.avi')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.getByText('Processing')).toBeInTheDocument()
  })

  it('renders failed job with retry button and error', () => {
    render(<FormatConverter jobs={[failedJob]} supportedFormats={supportedFormats} />)
    expect(screen.getByText('bad.avi')).toBeInTheDocument()
    expect(screen.getByText('Retry')).toBeInTheDocument()
    expect(screen.getByText('Unsupported codec')).toBeInTheDocument()
    expect(screen.getByText('Failed')).toBeInTheDocument()
  })

  it('calls onDownloadFile when download is clicked', async () => {
    const user = userEvent.setup()
    const onDownloadFile = vi.fn()
    render(
      <FormatConverter
        jobs={[completedJob]}
        supportedFormats={supportedFormats}
        onDownloadFile={onDownloadFile}
      />
    )

    await user.click(screen.getByText('Download'))
    expect(onDownloadFile).toHaveBeenCalledWith(completedJob.id)
  })

  it('calls onCancelConversion when cancel is clicked', async () => {
    const user = userEvent.setup()
    const onCancelConversion = vi.fn()
    render(
      <FormatConverter
        jobs={[processingJob]}
        supportedFormats={supportedFormats}
        onCancelConversion={onCancelConversion}
      />
    )

    await user.click(screen.getByText('Cancel'))
    expect(onCancelConversion).toHaveBeenCalledWith('2')
  })

  it('calls onRetryConversion when retry is clicked', async () => {
    const user = userEvent.setup()
    const onRetryConversion = vi.fn()
    render(
      <FormatConverter
        jobs={[failedJob]}
        supportedFormats={supportedFormats}
        onRetryConversion={onRetryConversion}
      />
    )

    await user.click(screen.getByText('Retry'))
    expect(onRetryConversion).toHaveBeenCalledWith('3')
  })

  it('opens create conversion modal on button click', async () => {
    const user = userEvent.setup()
    render(<FormatConverter jobs={[]} supportedFormats={supportedFormats} />)

    await user.click(screen.getByText('Create Conversion Job'))
    // "Create Conversion Job" appears as both button and modal title
    const createElements = screen.getAllByText('Create Conversion Job')
    expect(createElements.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Select File')).toBeInTheDocument()
    expect(screen.getByText('Target Format')).toBeInTheDocument()
    expect(screen.getByText('Quality Preset')).toBeInTheDocument()
  })

  it('shows format options for each supported format', async () => {
    const user = userEvent.setup()
    render(<FormatConverter jobs={[]} supportedFormats={supportedFormats} />)

    await user.click(screen.getByText('Create Conversion Job'))

    supportedFormats.forEach(format => {
      expect(screen.getByText(format.toUpperCase())).toBeInTheDocument()
    })
  })

  it('shows quality presets in the modal', async () => {
    const user = userEvent.setup()
    render(<FormatConverter jobs={[]} supportedFormats={supportedFormats} />)

    await user.click(screen.getByText('Create Conversion Job'))

    expect(screen.getByText('Low (Fast)')).toBeInTheDocument()
    expect(screen.getByText('Medium (Balanced)')).toBeInTheDocument()
    expect(screen.getByText('High (Quality)')).toBeInTheDocument()
    expect(screen.getByText('Ultra (Best)')).toBeInTheDocument()
  })

  it('displays file size correctly', () => {
    render(<FormatConverter jobs={[completedJob]} supportedFormats={supportedFormats} />)
    // 1073741824 bytes = 1 GB
    expect(screen.getByText('1 GB')).toBeInTheDocument()
  })

  it('displays multiple jobs', () => {
    render(
      <FormatConverter
        jobs={[completedJob, processingJob, failedJob]}
        supportedFormats={supportedFormats}
      />
    )
    expect(screen.getByText('test.mkv')).toBeInTheDocument()
    expect(screen.getByText('video.avi')).toBeInTheDocument()
    expect(screen.getByText('bad.avi')).toBeInTheDocument()
  })
})
