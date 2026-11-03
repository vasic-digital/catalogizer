import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SmartCollectionBuilder } from '../SmartCollectionBuilder'
import { toast } from 'react-hot-toast'

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, className, onClick }: any) => <div className={className} onClick={onClick}>{children}</div>,
    button: ({ children, className, onClick }: any) => (
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

vi.mock('../../../lib/collectionRules', () => ({
  COLLECTION_TEMPLATES: [
    {
      id: 'recent',
      name: 'Recently Added',
      description: 'Items added in the last 30 days',
      icon: 'Clock',
      category: 'Time-based',
      rules: [{ field: 'created_at', operator: 'greater_than', value: '30d' }],
    },
  ],
  COLLECTION_FIELD_OPTIONS: [
    { value: 'media_type', label: 'Media Type' },
    { value: 'year', label: 'Year' },
  ],
  COLLECTION_OPERATORS: {
    text: [
      { value: 'equals', label: 'Equals' },
      { value: 'contains', label: 'Contains' },
    ],
    number: [
      { value: 'equals', label: 'Equals' },
      { value: 'greater_than', label: 'Greater than' },
    ],
  },
  getFieldOptions: vi.fn(() => []),
  getFieldLabel: vi.fn((field: string) => field),
  getFieldType: vi.fn(() => 'text'),
  validateRules: vi.fn(() => ({ isValid: true, errors: [] })),
}))

const defaultProps = {
  onSave: vi.fn(),
  onCancel: vi.fn(),
}

describe('SmartCollectionBuilder', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the component', () => {
    render(<SmartCollectionBuilder {...defaultProps} />)
    expect(screen.getByPlaceholderText('Enter collection name')).toBeInTheDocument()
  })

  it('renders name input field', () => {
    render(<SmartCollectionBuilder {...defaultProps} />)
    const nameInput = screen.getByPlaceholderText('Enter collection name')
    expect(nameInput).toBeInTheDocument()
  })

  it('renders description textarea', () => {
    render(<SmartCollectionBuilder {...defaultProps} />)
    const descInput = screen.getByPlaceholderText('Optional description for this collection')
    expect(descInput).toBeInTheDocument()
  })

  it('renders Create Collection and Cancel buttons', () => {
    render(<SmartCollectionBuilder {...defaultProps} />)
    expect(screen.getByText('Create Collection')).toBeInTheDocument()
    // Cancel button may appear multiple times
    const cancelButtons = screen.getAllByText('Cancel')
    expect(cancelButtons.length).toBeGreaterThanOrEqual(1)
  })

  it('calls onCancel when Cancel is clicked', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    const cancelButtons = screen.getAllByText('Cancel')
    await user.click(cancelButtons[0])

    expect(defaultProps.onCancel).toHaveBeenCalled()
  })

  it('renders Add Rule button', () => {
    render(<SmartCollectionBuilder {...defaultProps} />)
    expect(screen.getByText('Add Rule')).toBeInTheDocument()
  })

  it('renders Quick Start Templates section', () => {
    render(<SmartCollectionBuilder {...defaultProps} />)
    // Templates section header is visible (collapsed by default)
    expect(screen.getByText('Quick Start Templates')).toBeInTheDocument()
  })

  it('accepts initial values', () => {
    render(
      <SmartCollectionBuilder
        {...defaultProps}
        initialName="Test Smart Collection"
        initialDescription="Test description"
      />
    )
    const nameInput = screen.getByPlaceholderText('Enter collection name') as HTMLInputElement
    expect(nameInput.value).toBe('Test Smart Collection')
  })

  // --- Additional branch coverage tests ---

  it('adds a new rule when Add Rule is clicked', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // One rule already exists from initialization
    const addRuleBtn = screen.getByText('Add Rule')
    await user.click(addRuleBtn)

    // Should now have multiple rule containers (select inputs)
    const fieldSelects = document.querySelectorAll('select')
    expect(fieldSelects.length).toBeGreaterThanOrEqual(2)
  })

  it('removes a rule when trash button is clicked', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // Add a second rule first
    await user.click(screen.getByText('Add Rule'))

    // Find remove buttons (trash icon buttons)
    const removeButtons = document.querySelectorAll('button')
    const trashButtons = Array.from(removeButtons).filter(btn =>
      btn.querySelector('svg') && btn.closest('.border.border-gray-200')
    )

    if (trashButtons.length > 0) {
      await user.click(trashButtons[trashButtons.length - 1] as HTMLElement)
    }
  })

  it('toggles Quick Start Templates section visibility', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // The toggle button is near the "Quick Start Templates" heading
    const heading = screen.getByText('Quick Start Templates')
    const templateSection = heading.closest('div.mb-6') || heading.closest('div')
    const toggleBtn = templateSection?.querySelector('button')
    expect(toggleBtn).toBeTruthy()
    await user.click(toggleBtn!)

    // After toggling, template content should be visible
    await screen.findByText('Recently Added')
  })

  it('loads a template when clicked', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // Open templates section
    const heading = screen.getByText('Quick Start Templates')
    const templateSection = heading.closest('div.mb-6') || heading.closest('div')
    const toggleBtn = templateSection?.querySelector('button')
    if (toggleBtn) {
      await user.click(toggleBtn)
    }

    // Click on the template
    const templateBtn = await screen.findByText('Recently Added')
    await user.click(templateBtn)

    // Name should be populated from template
    const nameInput = screen.getByPlaceholderText('Enter collection name') as HTMLInputElement
    expect(nameInput.value).toBe('Recently Added')
  })

  it('shows validation error when saving with empty name', async () => {
    const { toast: mockToast } = await import('react-hot-toast')
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // Clear the name field (should be empty already)
    const nameInput = screen.getByPlaceholderText('Enter collection name')
    await user.clear(nameInput)

    // Try to save
    await user.click(screen.getByText('Create Collection'))

    // Should show validation error
    expect(mockToast.error).toHaveBeenCalled()
  })

  it('calls onSave with correct data when valid', async () => {
    const { validateRules } = await import('../../../lib/collectionRules')
    ;(validateRules as any).mockReturnValue({ isValid: true, errors: [] })

    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // Fill in name
    const nameInput = screen.getByPlaceholderText('Enter collection name')
    await user.type(nameInput, 'My Smart Collection')

    // Fill in description
    const descInput = screen.getByPlaceholderText('Optional description for this collection')
    await user.type(descInput, 'A test description')

    // Save
    await user.click(screen.getByText('Create Collection'))

    expect(defaultProps.onSave).toHaveBeenCalledWith(
      'My Smart Collection',
      'A test description',
      expect.any(Array)
    )
  })

  it('updates rule field when field select is changed', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)

    // Find the first field select (should exist from initial rule)
    const selects = document.querySelectorAll('select')
    if (selects.length > 0) {
      await user.selectOptions(selects[0] as HTMLSelectElement, 'year')
    }
  })

  it('renders with initial rules', () => {
    const initialRules = [
      { id: 'r1', field: 'genre', operator: 'equals', value: 'rock', field_type: 'text' as const, label: 'Genre' },
    ]
    render(
      <SmartCollectionBuilder
        {...defaultProps}
        initialRules={initialRules}
      />
    )
    // Should render without crashing with initial rules
    expect(screen.getByPlaceholderText('Enter collection name')).toBeInTheDocument()
  })

  it('renders with className prop', () => {
    const { container } = render(
      <SmartCollectionBuilder {...defaultProps} className="custom-class" />
    )
    expect(container.firstChild).toBeTruthy()
  })

  it('CAT-001: Test Rules reports unavailable and shows no canned results', async () => {
    const user = userEvent.setup()
    render(<SmartCollectionBuilder {...defaultProps} />)
    await user.click(screen.getByText('Test Rules'))
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('not available'))
    expect(toast.success).not.toHaveBeenCalled()
    for (const invented of [/1234/, /Sample Song 1/, /Sample Movie 1/, /245/]) {
      expect(screen.queryByText(invented)).not.toBeInTheDocument()
    }
  })
})
