import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FavoriteToggle } from '../FavoriteToggle'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Mock the hooks
vi.mock('@/hooks/useFavorites', () => ({
  useFavoriteStatus: vi.fn((_mediaId: number) => ({
    data: { is_favorite: false },
    isLoading: false,
  })),
  useFavorites: vi.fn(() => ({
    toggleFavorite: vi.fn(),
    isToggling: false,
  })),
}))

vi.mock('@/lib/utils', () => ({
  cn: (...classes: any[]) => classes.filter(Boolean).join(' '),
}))

import { useFavoriteStatus, useFavorites } from '@/hooks/useFavorites'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

const renderWithProvider = (ui: React.ReactElement) => {
  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  )
}

describe('FavoriteToggle', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useFavoriteStatus).mockReturnValue({
      data: { is_favorite: false },
      isLoading: false,
    } as any)
    vi.mocked(useFavorites).mockReturnValue({
      toggleFavorite: vi.fn(),
      isToggling: false,
    } as any)
  })

  it('renders with button variant by default', () => {
    renderWithProvider(<FavoriteToggle mediaId={1} />)
    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()
  })

  it('renders with icon variant', () => {
    renderWithProvider(<FavoriteToggle mediaId={1} variant="icon" />)
    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()
    expect(button).toHaveAttribute('title', 'Add to favorites')
  })

  it('shows "Remove from favorites" title when already favorited', () => {
    vi.mocked(useFavoriteStatus).mockReturnValue({
      data: { is_favorite: true },
      isLoading: false,
    } as any)

    renderWithProvider(<FavoriteToggle mediaId={1} variant="icon" />)
    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('title', 'Remove from favorites')
  })

  it('calls toggleFavorite when clicked', async () => {
    const user = userEvent.setup()
    const toggleFavorite = vi.fn()
    vi.mocked(useFavorites).mockReturnValue({
      toggleFavorite,
      isToggling: false,
    } as any)

    renderWithProvider(<FavoriteToggle mediaId={42} variant="icon" />)
    await user.click(screen.getByRole('button'))

    expect(toggleFavorite).toHaveBeenCalledWith(42, false)
  })

  it('shows label text when showLabel is true', () => {
    renderWithProvider(<FavoriteToggle mediaId={1} showLabel />)
    expect(screen.getByText('Add to Favorites')).toBeInTheDocument()
  })

  it('shows "Remove from Favorites" label when already favorited', () => {
    vi.mocked(useFavoriteStatus).mockReturnValue({
      data: { is_favorite: true },
      isLoading: false,
    } as any)

    renderWithProvider(<FavoriteToggle mediaId={1} showLabel />)
    expect(screen.getByText('Remove from Favorites')).toBeInTheDocument()
  })

  it('disables button when loading', () => {
    vi.mocked(useFavoriteStatus).mockReturnValue({
      data: null,
      isLoading: true,
    } as any)

    renderWithProvider(<FavoriteToggle mediaId={1} variant="icon" />)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('shows the "Loading..." label in the button variant while loading', () => {
    vi.mocked(useFavoriteStatus).mockReturnValue({
      data: null,
      isLoading: true,
    } as any)

    renderWithProvider(<FavoriteToggle mediaId={1} showLabel />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByText('Add to Favorites')).not.toBeInTheDocument()
  })

  it('disables button when disabled prop is true', () => {
    renderWithProvider(<FavoriteToggle mediaId={1} variant="icon" disabled />)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('does not call toggleFavorite when disabled', async () => {
    const user = userEvent.setup()
    const toggleFavorite = vi.fn()
    vi.mocked(useFavorites).mockReturnValue({
      toggleFavorite,
      isToggling: false,
    } as any)

    renderWithProvider(<FavoriteToggle mediaId={1} variant="icon" disabled />)
    await user.click(screen.getByRole('button'))

    expect(toggleFavorite).not.toHaveBeenCalled()
  })

  it('renders card variant', () => {
    renderWithProvider(<FavoriteToggle mediaId={1} variant="card" />)
    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()
  })
})
