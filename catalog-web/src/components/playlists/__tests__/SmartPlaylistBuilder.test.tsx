import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, ..._props }: any) => <div className={className}>{children}</div>,
    button: ({ children, className, onClick, ..._props }: any) => (
      <button className={className} onClick={onClick}>{children}</button>
    ),
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('../../../types/playlists', () => ({
  SmartPlaylistRule: {},
  PlaylistType: {},
}))

describe('SmartPlaylistBuilder', () => {
  let SmartPlaylistBuilder: any
  const defaultProps = {
    onSave: vi.fn(),
    onCancel: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  beforeAll(async () => {
    const mod = await import('../SmartPlaylistBuilder')
    SmartPlaylistBuilder = mod.SmartPlaylistBuilder || mod.default
  })

  it('renders the component with name input', () => {
    if (!SmartPlaylistBuilder) return
    render(<SmartPlaylistBuilder {...defaultProps} />)
    expect(screen.getByPlaceholderText('Enter playlist name...')).toBeInTheDocument()
  })

  it('renders Save and Cancel buttons', () => {
    if (!SmartPlaylistBuilder) return
    render(<SmartPlaylistBuilder {...defaultProps} />)
    expect(screen.getByText('Save Playlist')).toBeInTheDocument()
    const cancelElements = screen.getAllByText(/Cancel/)
    expect(cancelElements.length).toBeGreaterThanOrEqual(1)
  })

  it('calls onCancel when Cancel is clicked', async () => {
    if (!SmartPlaylistBuilder) return
    const user = userEvent.setup()
    render(<SmartPlaylistBuilder {...defaultProps} />)

    const cancelElements = screen.getAllByText(/Cancel/)
    await user.click(cancelElements[0])

    expect(defaultProps.onCancel).toHaveBeenCalled()
  })

  it('renders Add Rule button', () => {
    if (!SmartPlaylistBuilder) return
    render(<SmartPlaylistBuilder {...defaultProps} />)
    expect(screen.getByText('Add Rule')).toBeInTheDocument()
  })

  it('allows typing a playlist name', async () => {
    if (!SmartPlaylistBuilder) return
    const user = userEvent.setup()
    render(<SmartPlaylistBuilder {...defaultProps} />)
    const input = screen.getByPlaceholderText('Enter playlist name...')
    await user.type(input, 'My Smart Playlist')
    expect(input).toHaveValue('My Smart Playlist')
  })

  it('renders description input', () => {
    if (!SmartPlaylistBuilder) return
    render(<SmartPlaylistBuilder {...defaultProps} />)
    const descriptionInput = screen.getByPlaceholderText(/describe your smart playlist/i)
    expect(descriptionInput).toBeInTheDocument()
  })

  it('adds a rule when Add Rule is clicked', async () => {
    if (!SmartPlaylistBuilder) return
    const user = userEvent.setup()
    render(<SmartPlaylistBuilder {...defaultProps} />)
    await user.click(screen.getByText('Add Rule'))
    // After adding a rule, there should be rule field selectors
    const selects = document.querySelectorAll('select')
    expect(selects.length).toBeGreaterThan(0)
  })

  it('renders the Smart Playlist Builder heading', () => {
    if (!SmartPlaylistBuilder) return
    render(<SmartPlaylistBuilder {...defaultProps} />)
    expect(screen.getByText('Smart Playlist Builder')).toBeInTheDocument()
  })

  it('Save is enabled and passes name, description and the rules to onSave', async () => {
    const onSave = vi.fn()
    const user = userEvent.setup()
    render(<SmartPlaylistBuilder {...defaultProps} onSave={onSave} initialData={{ name: 'N', description: 'D', rules: [{ field: 'genre', operator: 'equals', value: 'rock' }] }} />)
    const btn = screen.getByText('Save Playlist').closest('button')!
    expect(btn).toBeEnabled()
    await user.click(btn)
    expect(onSave).toHaveBeenCalledWith('N', 'D', [{ field: 'genre', operator: 'equals', value: 'rock' }])
  })
})

describe('SmartPlaylistBuilder per-rule control ids (CAT-UI-D5D6)', () => {
  it('gives every rule uniquely-id\'d, correctly named Field/Condition selects', async () => {
    const { SmartPlaylistBuilder } = await import('../SmartPlaylistBuilder')
    const user = userEvent.setup()
    render(<SmartPlaylistBuilder onSave={vi.fn()} onCancel={vi.fn()} />)
    await user.click(screen.getByText('Add Rule'))
    await user.click(screen.getByText('Add Rule'))
    const fields = screen.getAllByRole('combobox', { name: 'Field' })
    const conds = screen.getAllByRole('combobox', { name: 'Condition' })
    expect(fields.length).toBeGreaterThanOrEqual(2)
    expect(conds.length).toBe(fields.length)
    const ids = Array.from(document.querySelectorAll('[id]')).map(e => e.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
