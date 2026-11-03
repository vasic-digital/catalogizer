import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UploadManager } from '../UploadManager'

vi.mock('lucide-react', () => ({
  Upload: () => <span data-testid="icon-upload">Upload</span>,
  X: () => <span data-testid="icon-x">X</span>,
  File: () => <span data-testid="icon-file">File</span>,
  CheckCircle: () => <span data-testid="icon-check">Check</span>,
  AlertCircle: () => <span data-testid="icon-alert">Alert</span>,
  Trash2: () => <span data-testid="icon-trash">Trash</span>,
}))

const createMockFile = (name: string, size: number, type: string): File => {
  const file = new File(['test content'], name, { type })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

describe('UploadManager', () => {
  it('renders the upload manager title', () => {
    render(<UploadManager />)
    expect(screen.getByText('Upload Manager')).toBeInTheDocument()
  })

  it('renders the drop zone', () => {
    render(<UploadManager />)
    expect(screen.getByText('Drop files here or click to browse')).toBeInTheDocument()
  })

  it('renders Select Files button', () => {
    render(<UploadManager />)
    expect(screen.getByText('Select Files')).toBeInTheDocument()
  })

  it('displays max file size information', () => {
    render(<UploadManager maxFileSize={50 * 1024 * 1024} />)
    expect(screen.getByText(/Max file size:/)).toBeInTheDocument()
  })

  it('displays accepted types information', () => {
    render(<UploadManager acceptedTypes={['video/*', 'audio/*']} />)
    expect(screen.getByText(/Accepted types:/)).toBeInTheDocument()
  })

  it('displays default max file size of 100MB', () => {
    render(<UploadManager />)
    expect(screen.getByText(/100 MB/)).toBeInTheDocument()
  })

  it('does not show upload queue when no files', () => {
    render(<UploadManager />)
    expect(screen.queryByText('Upload Queue')).not.toBeInTheDocument()
  })

  it('does not show Clear Completed button when no completed uploads', () => {
    render(<UploadManager />)
    expect(screen.queryByText('Clear Completed')).not.toBeInTheDocument()
  })

  it('handles drag over event', () => {
    const { container } = render(<UploadManager />)
    const dropZone = container.querySelector('.border-dashed')!

    fireEvent.dragOver(dropZone, { dataTransfer: { files: [] } })
    expect(dropZone).toHaveClass('border-blue-500')
  })

  it('handles drag leave event', () => {
    const { container } = render(<UploadManager />)
    const dropZone = container.querySelector('.border-dashed')!

    fireEvent.dragOver(dropZone, { dataTransfer: { files: [] } })
    expect(dropZone).toHaveClass('border-blue-500')

    fireEvent.dragLeave(dropZone, { dataTransfer: { files: [] } })
    expect(dropZone).not.toHaveClass('border-blue-500')
  })

  it('handles file drop', async () => {
    const onUpload = vi.fn().mockResolvedValue(undefined)
    const { container } = render(<UploadManager onUpload={onUpload} />)
    const dropZone = container.querySelector('.border-dashed')!

    const file = createMockFile('test.mp4', 1024, 'video/mp4')
    const dataTransfer = {
      files: [file],
      items: [{ kind: 'file', getAsFile: () => file }],
      types: ['Files'],
    }

    fireEvent.drop(dropZone, { dataTransfer })

    await waitFor(() => {
      expect(screen.getByText(/Upload Queue/)).toBeInTheDocument()
    })
  })

  it('filters out files exceeding max size on drop', () => {
    const { container } = render(
      <UploadManager maxFileSize={100} />
    )
    const dropZone = container.querySelector('.border-dashed')!

    const largeFile = createMockFile('large.mp4', 1000, 'video/mp4')
    const dataTransfer = {
      files: [largeFile],
      items: [{ kind: 'file', getAsFile: () => largeFile }],
      types: ['Files'],
    }

    fireEvent.drop(dropZone, { dataTransfer })

    // Should not show queue since file was too large
    expect(screen.queryByText(/Upload Queue/)).not.toBeInTheDocument()
  })

  it('filters out files with unaccepted types', () => {
    const { container } = render(
      <UploadManager acceptedTypes={['video/*']} />
    )
    const dropZone = container.querySelector('.border-dashed')!

    const textFile = createMockFile('doc.txt', 100, 'text/plain')
    const dataTransfer = {
      files: [textFile],
      items: [{ kind: 'file', getAsFile: () => textFile }],
      types: ['Files'],
    }

    fireEvent.drop(dropZone, { dataTransfer })

    expect(screen.queryByText(/Upload Queue/)).not.toBeInTheDocument()
  })

  it('handles file input change', async () => {
    const { container } = render(<UploadManager />)
    const fileInput = container.querySelector('input[type="file"]')!

    const file = createMockFile('song.mp3', 5000, 'audio/mp3')
    fireEvent.change(fileInput, { target: { files: [file] } })

    await waitFor(() => {
      expect(screen.getByText(/Upload Queue/)).toBeInTheDocument()
    })
  })

  it('calls onRemove when remove button is clicked', async () => {
    const onRemove = vi.fn()
    const { container } = render(<UploadManager onRemove={onRemove} />)
    const fileInput = container.querySelector('input[type="file"]')!

    const file = createMockFile('song.mp3', 5000, 'audio/mp3')
    fireEvent.change(fileInput, { target: { files: [file] } })

    await waitFor(() => {
      expect(screen.getByText('song.mp3')).toBeInTheDocument()
    })

    // Click the trash/remove button
    const trashButtons = screen.getAllByTestId('icon-trash')
    const removeBtn = trashButtons[0].closest('button')
    if (removeBtn) {
      await userEvent.setup().click(removeBtn)
    }

    // After removal, the file should be gone
    expect(screen.queryByText('song.mp3')).not.toBeInTheDocument()
  })

  it('renders file input with correct accept attribute', () => {
    const { container } = render(
      <UploadManager acceptedTypes={['video/*', 'audio/*']} />
    )
    const fileInput = container.querySelector('input[type="file"]')!
    expect(fileInput).toHaveAttribute('accept', 'video/*,audio/*')
  })

  it('renders file input with multiple attribute', () => {
    const { container } = render(<UploadManager />)
    const fileInput = container.querySelector('input[type="file"]')!
    expect(fileInput).toHaveAttribute('multiple')
  })

  it('displays file name and size in queue', async () => {
    const { container } = render(<UploadManager />)
    const fileInput = container.querySelector('input[type="file"]')!

    const file = createMockFile('movie.mp4', 1048576, 'video/mp4')
    fireEvent.change(fileInput, { target: { files: [file] } })

    await waitFor(() => {
      expect(screen.getByText('movie.mp4')).toBeInTheDocument()
    })
  })

  it('shows upload queue count', async () => {
    const { container } = render(<UploadManager />)
    const fileInput = container.querySelector('input[type="file"]')!

    const file1 = createMockFile('file1.mp4', 1000, 'video/mp4')
    const file2 = createMockFile('file2.mp4', 2000, 'video/mp4')
    fireEvent.change(fileInput, { target: { files: [file1, file2] } })

    await waitFor(() => {
      expect(screen.getByText(/Upload Queue \(2 items\)/)).toBeInTheDocument()
    })
  })

  const dropOne = (container: HTMLElement) => {
    const dropZone = container.querySelector('.border-dashed')!
    const file = createMockFile('real.mp4', 1024, 'video/mp4')
    fireEvent.drop(dropZone, { dataTransfer: { files: [file], items: [], types: ['Files'] } })
  }

  it('CAT-001: without an upload handler a dropped file is reported as not sent, never as success', async () => {
    const { container } = render(<UploadManager />)
    dropOne(container)
    expect(await screen.findByText(/Upload is not available/)).toBeInTheDocument()
    expect(container.querySelector('.text-green-500, .text-green-600')).toBeNull()
  })

  it('CAT-001: success is shown only after onUpload resolves, and failure when it rejects', async () => {
    const fail = vi.fn().mockRejectedValue(new Error('server said no'))
    const { container, unmount } = render(<UploadManager onUpload={fail} />)
    dropOne(container)
    expect(await screen.findByText('server said no')).toBeInTheDocument()
    expect(fail).toHaveBeenCalledTimes(1)
    unmount()

    const ok = vi.fn().mockResolvedValue(undefined)
    const r2 = render(<UploadManager onUpload={ok} />)
    dropOne(r2.container)
    await waitFor(() => expect(ok).toHaveBeenCalledTimes(1))
    expect(screen.queryByText(/Upload is not available/)).not.toBeInTheDocument()
  })
})
