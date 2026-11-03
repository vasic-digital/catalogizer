import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { PageHeader } from '../PageHeader'

vi.mock('@/lib/utils', () => ({
  cn: (...classes: any[]) => classes.filter(Boolean).join(' '),
}))

const renderWithRouter = (ui: React.ReactElement) =>
  render(<MemoryRouter>{ui}</MemoryRouter>)

describe('PageHeader', () => {
  it('renders the title', () => {
    renderWithRouter(<PageHeader title="Dashboard" />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('renders as h1 heading', () => {
    renderWithRouter(<PageHeader title="My Page" />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('My Page')
  })

  it('renders subtitle when provided', () => {
    renderWithRouter(
      <PageHeader title="Settings" subtitle="Manage your preferences" />
    )
    expect(screen.getByText('Manage your preferences')).toBeInTheDocument()
  })

  it('does not render subtitle when not provided', () => {
    renderWithRouter(<PageHeader title="Settings" />)
    const paragraphs = document.querySelectorAll('p')
    // No p tag should exist for subtitle
    const hasSubtitle = Array.from(paragraphs).some(
      p => p.classList.contains('text-gray-600')
    )
    expect(hasSubtitle).toBe(false)
  })

  it('renders icon when provided', () => {
    renderWithRouter(
      <PageHeader
        title="Music"
        icon={<span data-testid="music-icon">Icon</span>}
      />
    )
    expect(screen.getByTestId('music-icon')).toBeInTheDocument()
  })

  it('does not render icon container when icon is not provided', () => {
    const { container } = renderWithRouter(<PageHeader title="No Icon" />)
    const iconContainer = container.querySelector('.bg-blue-100')
    expect(iconContainer).toBeNull()
  })

  it('renders actions when provided', () => {
    renderWithRouter(
      <PageHeader
        title="Collections"
        actions={<button data-testid="action-btn">Create</button>}
      />
    )
    expect(screen.getByTestId('action-btn')).toBeInTheDocument()
  })

  it('does not render actions container when actions is not provided', () => {
    const { container } = renderWithRouter(<PageHeader title="No Actions" />)
    // The actions wrapper has mt-4 sm:mt-0 classes
    const actionsDiv = container.querySelector('.sm\\:mt-0')
    expect(actionsDiv).toBeNull()
  })

  it('renders breadcrumbs when provided', () => {
    renderWithRouter(
      <PageHeader
        title="Detail"
        breadcrumbs={[
          { label: 'Home', href: '/' },
          { label: 'Media', href: '/media' },
          { label: 'Detail' },
        ]}
      />
    )

    expect(screen.getByText('Home')).toBeInTheDocument()
    expect(screen.getByText('Media')).toBeInTheDocument()
    // "Detail" appears both in the breadcrumb (span) and h1 title
    const detailElements = screen.getAllByText('Detail')
    expect(detailElements.length).toBe(2)
  })

  it('renders breadcrumb links as router links', () => {
    renderWithRouter(
      <PageHeader
        title="Detail"
        breadcrumbs={[
          { label: 'Home', href: '/' },
          { label: 'Detail' },
        ]}
      />
    )

    const link = screen.getByText('Home')
    expect(link.tagName).toBe('A')
    expect(link).toHaveAttribute('href', '/')
  })

  it('renders last breadcrumb without link when no href', () => {
    renderWithRouter(
      <PageHeader
        title="Detail"
        breadcrumbs={[
          { label: 'Home', href: '/' },
          { label: 'Current Page' },
        ]}
      />
    )

    const currentPage = screen.getByText('Current Page')
    expect(currentPage.tagName).toBe('SPAN')
    expect(currentPage).toHaveClass('font-medium')
  })

  it('renders breadcrumb separators between items', () => {
    renderWithRouter(
      <PageHeader
        title="Detail"
        breadcrumbs={[
          { label: 'Home', href: '/' },
          { label: 'Media', href: '/media' },
          { label: 'Detail' },
        ]}
      />
    )

    const separators = screen.getAllByText('/')
    expect(separators.length).toBe(2)
  })

  it('does not render breadcrumbs when empty array provided', () => {
    const { container } = renderWithRouter(
      <PageHeader title="No Breadcrumbs" breadcrumbs={[]} />
    )
    const nav = container.querySelector('nav')
    expect(nav).toBeNull()
  })

  it('does not render breadcrumbs when not provided', () => {
    const { container } = renderWithRouter(
      <PageHeader title="No Breadcrumbs" />
    )
    const nav = container.querySelector('nav')
    expect(nav).toBeNull()
  })

  it('applies custom className', () => {
    const { container } = renderWithRouter(
      <PageHeader title="Custom" className="my-custom-class" />
    )
    expect(container.firstChild).toHaveClass('my-custom-class')
  })

  it('always has mb-6 class', () => {
    const { container } = renderWithRouter(
      <PageHeader title="Margin" />
    )
    expect(container.firstChild).toHaveClass('mb-6')
  })
})

describe('PageHeader - breadcrumb touch target', () => {
  it('breadcrumb links are touch-sized (not bare inline text)', () => {
    renderWithRouter(<PageHeader title="Playlists" breadcrumbs={[{ label: 'Home', href: '/' }, { label: 'Playlists' }]} />)
    expect(screen.getByRole('link', { name: 'Home' }).className).toContain('touch-target')
  })
})
