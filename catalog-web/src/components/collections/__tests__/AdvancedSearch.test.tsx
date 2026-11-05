import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AdvancedSearch from '../AdvancedSearch'
import { toast } from 'react-hot-toast'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
    button: ({ children, ...props }: any) => <button {...props}>{children}</button>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe('AdvancedSearch', () => {
  it('renders heading and description', () => {
    render(<AdvancedSearch />)
    expect(screen.getByText('Advanced Search')).toBeInTheDocument()
    expect(
      screen.getByText('Build complex search queries with multiple rules and conditions')
    ).toBeInTheDocument()
  })

  it('renders tab navigation with builder, presets, saved tabs', () => {
    render(<AdvancedSearch />)
    expect(screen.getByText('Builder')).toBeInTheDocument()
    expect(screen.getByText('Presets')).toBeInTheDocument()
    expect(screen.getByText('Saved')).toBeInTheDocument()
  })

  it('shows empty state when no rules are defined', () => {
    render(<AdvancedSearch />)
    expect(screen.getByText('No search rules defined')).toBeInTheDocument()
    expect(screen.getByText('Add rules to build your search query')).toBeInTheDocument()
  })

  it('adds a rule when Add Rule is clicked', async () => {
    const user = userEvent.setup()
    render(<AdvancedSearch />)

    await user.click(screen.getByText('Add First Rule'))

    expect(screen.queryByText('No search rules defined')).not.toBeInTheDocument()
  })

  it('shows active rules count', () => {
    render(<AdvancedSearch />)
    expect(screen.getByText('0 active rules')).toBeInTheDocument()
  })

  it('renders Search button disabled when no rules', () => {
    render(<AdvancedSearch />)
    const searchButton = screen.getByText('Search').closest('button')
    expect(searchButton).toBeDisabled()
  })

  it('renders Save Search button disabled when no rules', () => {
    render(<AdvancedSearch />)
    const saveButton = screen.getByText('Save Search').closest('button')
    expect(saveButton).toBeDisabled()
  })

  it('switches to presets tab', async () => {
    const user = userEvent.setup()
    render(<AdvancedSearch />)

    await user.click(screen.getByText('Presets'))

    expect(screen.getByText('Search Presets')).toBeInTheDocument()
    expect(screen.getByText('Quick start with pre-configured search patterns')).toBeInTheDocument()
    expect(screen.getByText('HD Movies')).toBeInTheDocument()
    expect(screen.getByText('Large Files')).toBeInTheDocument()
  })

  it('switches to saved tab and shows empty state', async () => {
    const user = userEvent.setup()
    render(<AdvancedSearch />)

    await user.click(screen.getByText('Saved'))

    expect(screen.getByText('Saved Searches')).toBeInTheDocument()
    expect(screen.getByText('No saved searches yet')).toBeInTheDocument()
  })

  it('renders search settings section', async () => {
    const _user = userEvent.setup()
    render(<AdvancedSearch />)

    expect(screen.getByText('Search Settings')).toBeInTheDocument()
    expect(screen.getByText('Sort By')).toBeInTheDocument()
  })

  describe('CAT-001: no success claim without an action', () => {
    beforeEach(() => { vi.mocked(toast.success).mockClear(); vi.mocked(toast.error).mockClear() })

    it('the Search button stays disabled with a reason and never toasts "Searching"', async () => {
      const user = userEvent.setup()
      render(<AdvancedSearch />)
      await user.click(screen.getByText('Add First Rule'))
      const btn = screen.getByText('Search').closest('button')!
      expect(btn).toBeDisabled()
      expect(btn.getAttribute('title') || '').toMatch(/not available/i)
      await user.click(btn)
      expect(toast.success).not.toHaveBeenCalledWith(expect.stringMatching(/Searching with/))
    })

    it('Save Search says the search is kept for this session only', async () => {
      const user = userEvent.setup()
      render(<AdvancedSearch />)
      await user.click(screen.getByText('Add First Rule'))
      await user.click(screen.getByText('Save Search').closest('button')!)
      expect(toast.success).not.toHaveBeenCalledWith('Search saved successfully')
      expect(toast.success).toHaveBeenCalledWith(expect.stringMatching(/this session only/i))
    })
  })
})

describe('AdvancedSearch accessible names (CAT-UI-D5D6)', () => {
  it('names rule selects and associates the Sort By / Order / Items per page labels', async () => {
    const user = userEvent.setup()
    render(<AdvancedSearch />)
    await user.click(screen.getByText('Add First Rule'))
    expect(screen.getByRole('combobox', { name: 'Rule field' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Rule operator' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Sort By' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Order' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Items per page' })).toBeInTheDocument()
  })
})
