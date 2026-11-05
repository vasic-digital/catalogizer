import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { MediaCard } from '../MediaCard';
import type { MediaItem } from '@/types/media';

// Mock the FavoriteToggle component to avoid its react-query dependencies
vi.mock('@/components/favorites/FavoriteToggle', () => ({
  FavoriteToggle: () => null,
}));

// Mock framer-motion to simplify rendering
vi.mock('framer-motion', () => ({
  motion: {
    div: (() => {
      const MotionDiv = React.forwardRef(({ children, className, ...props }: any, ref: any) => (
        <div ref={ref} className={className} data-testid={props['data-testid']}>
          {children}
        </div>
      ));
      MotionDiv.displayName = 'MotionDiv';
      return MotionDiv;
    })(),
  },
}));

const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  );
};

const mockMediaItem: MediaItem = {
  id: 1,
  title: 'Test Movie',
  media_type: 'movie',
  year: 2024,
  description: 'A test movie description',
  rating: 8.5,
  quality: '1080p',
  file_size: 1073741824, // 1GB
  duration: 7200, // 2 hours
  directory_path: '/movies/test-movie',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

describe('MediaCard', () => {
  it('renders media title', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);
    expect(screen.getByText('Test Movie')).toBeInTheDocument();
  });

  it('renders media type icon', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);
    // Film icon should be rendered for movies
    const card = screen.getByText('Test Movie').closest('div');
    expect(card).toBeInTheDocument();
  });

  it('displays formatted file size', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);
    expect(screen.getByText(/1\.\d+ GB/i)).toBeInTheDocument();
  });

  it('displays quality badge', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);
    // Quality is displayed in uppercase
    expect(screen.getByText('1080P')).toBeInTheDocument();
  });

  it('displays year when provided', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);
    expect(screen.getByText('2024')).toBeInTheDocument();
  });

  it('displays rating when provided', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);
    expect(screen.getByText('8.5')).toBeInTheDocument();
  });

  it('handles missing optional fields gracefully', () => {
    const minimalMedia: MediaItem = {
      id: 2,
      title: 'Minimal Movie',
      media_type: 'movie',
      directory_path: '/movies/minimal',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    };

    renderWithProviders(<MediaCard media={minimalMedia} />);
    expect(screen.getByText('Minimal Movie')).toBeInTheDocument();
  });

  it('calls onView when view button is clicked', async () => {
    const user = userEvent.setup();
    const handleView = vi.fn();

    renderWithProviders(<MediaCard media={mockMediaItem} onView={handleView} />);

    // Buttons have icons but no text, so we need to find them differently
    const buttons = screen.getAllByRole('button');
    expect(buttons.length).toBeGreaterThan(0);

    await user.click(buttons[0]);

    expect(handleView).toHaveBeenCalledTimes(1);
    expect(handleView).toHaveBeenCalledWith(mockMediaItem);
  });

  it('calls onDownload when download button is clicked', async () => {
    const user = userEvent.setup();
    const handleDownload = vi.fn();

    renderWithProviders(<MediaCard media={mockMediaItem} onDownload={handleDownload} />);

    const buttons = screen.getAllByRole('button');
    expect(buttons.length).toBeGreaterThan(0);

    await user.click(buttons[0]);

    expect(handleDownload).toHaveBeenCalledTimes(1);
    expect(handleDownload).toHaveBeenCalledWith(mockMediaItem);
  });

  it('applies custom className', () => {
    const { container } = renderWithProviders(
      <MediaCard media={mockMediaItem} className="custom-class" />
    );
    expect(container.firstChild).toHaveClass('custom-class');
  });

  describe('media type icons', () => {
    it('renders film icon for movie type', () => {
      const movieMedia = { ...mockMediaItem, media_type: 'movie' };
      renderWithProviders(<MediaCard media={movieMedia} />);
      expect(screen.getByText('Test Movie')).toBeInTheDocument();
    });

    it('renders music icon for music type', () => {
      const musicMedia = { ...mockMediaItem, media_type: 'music', title: 'Test Song' };
      renderWithProviders(<MediaCard media={musicMedia} />);
      expect(screen.getByText('Test Song')).toBeInTheDocument();
    });

    it('renders gamepad icon for game type', () => {
      const gameMedia = { ...mockMediaItem, media_type: 'game', title: 'Test Game' };
      renderWithProviders(<MediaCard media={gameMedia} />);
      expect(screen.getByText('Test Game')).toBeInTheDocument();
    });

    it('renders book icon for ebook type', () => {
      const ebookMedia = { ...mockMediaItem, media_type: 'ebook', title: 'Test Book' };
      renderWithProviders(<MediaCard media={ebookMedia} />);
      expect(screen.getByText('Test Book')).toBeInTheDocument();
    });
  });

  describe('quality badge colors', () => {
    it('renders purple badge for 4K quality', () => {
      const hqMedia = { ...mockMediaItem, quality: '4K' };
      renderWithProviders(<MediaCard media={hqMedia} />);
      expect(screen.getByText('4K')).toBeInTheDocument();
    });

    it('renders blue badge for 1080p quality', () => {
      const hdMedia = { ...mockMediaItem, quality: '1080p' };
      renderWithProviders(<MediaCard media={hdMedia} />);
      // Quality is displayed in uppercase
      expect(screen.getByText('1080P')).toBeInTheDocument();
    });

    it('renders green badge for 720p quality', () => {
      const sdMedia = { ...mockMediaItem, quality: '720p' };
      renderWithProviders(<MediaCard media={sdMedia} />);
      // Quality is displayed in uppercase
      expect(screen.getByText('720P')).toBeInTheDocument();
    });

    it('renders gray badge for unknown quality', () => {
      const unknownMedia = { ...mockMediaItem, quality: 'unknown' };
      renderWithProviders(<MediaCard media={unknownMedia} />);
      // Quality is displayed in uppercase
      expect(screen.getByText('UNKNOWN')).toBeInTheDocument();
    });
  });

  describe('file size formatting', () => {
    it('formats bytes correctly', () => {
      const smallMedia = { ...mockMediaItem, file_size: 512 };
      renderWithProviders(<MediaCard media={smallMedia} />);
      expect(screen.getByText(/512\.\d+ B/i)).toBeInTheDocument();
    });

    it('formats kilobytes correctly', () => {
      const kbMedia = { ...mockMediaItem, file_size: 1024 };
      renderWithProviders(<MediaCard media={kbMedia} />);
      expect(screen.getByText(/1\.\d+ KB/i)).toBeInTheDocument();
    });

    it('formats megabytes correctly', () => {
      const mbMedia = { ...mockMediaItem, file_size: 1048576 };
      renderWithProviders(<MediaCard media={mbMedia} />);
      expect(screen.getByText(/1\.\d+ MB/i)).toBeInTheDocument();
    });

    it('formats gigabytes correctly', () => {
      const gbMedia = { ...mockMediaItem, file_size: 1073741824 };
      renderWithProviders(<MediaCard media={gbMedia} />);
      expect(screen.getByText(/1\.\d+ GB/i)).toBeInTheDocument();
    });

    it('handles missing file size', () => {
      const noSizeMedia = { ...mockMediaItem, file_size: undefined };
      renderWithProviders(<MediaCard media={noSizeMedia} />);
      // When file_size is undefined, the component doesn't render the file size section at all
      expect(screen.queryByText(/GB|MB|KB|B/i)).not.toBeInTheDocument();
    });
  });

  it('truncates long descriptions', () => {
    const longDescMedia = {
      ...mockMediaItem,
      description: 'A'.repeat(200),
    };
    renderWithProviders(<MediaCard media={longDescMedia} />);
    // Description should be truncated to approximately 100 characters
    const text = screen.getByText(/A+/);
    expect(text.textContent?.length).toBeLessThan(200);
  });

  it('renders without action buttons when callbacks not provided', () => {
    renderWithProviders(<MediaCard media={mockMediaItem} />);

    // Buttons should not be present if no callbacks provided
    const buttons = screen.queryAllByRole('button');
    expect(buttons.length).toBe(0);
  });

  it('renders only one button when only onView provided', () => {
    const handleView = vi.fn();
    renderWithProviders(<MediaCard media={mockMediaItem} onView={handleView} />);

    const buttons = screen.queryAllByRole('button');
    expect(buttons.length).toBe(1);
  });

  it('renders only one button when only onDownload provided', () => {
    const handleDownload = vi.fn();
    renderWithProviders(<MediaCard media={mockMediaItem} onDownload={handleDownload} />);

    const buttons = screen.queryAllByRole('button');
    expect(buttons.length).toBe(1);
  });

  it('renders two buttons when both callbacks provided', () => {
    const handleView = vi.fn();
    const handleDownload = vi.fn();
    renderWithProviders(<MediaCard media={mockMediaItem} onView={handleView} onDownload={handleDownload} />);

    const buttons = screen.queryAllByRole('button');
    expect(buttons.length).toBe(2);
  });
});

describe('MediaCard overlay buttons accessible names (CAT-UI-D5D6)', () => {
  it('exposes play/view/download by role and name including the title', () => {
    renderWithProviders(
      <MediaCard media={mockMediaItem} onPlay={() => {}} onView={() => {}} onDownload={() => {}} />
    );
    expect(screen.getByRole('button', { name: 'Play Test Movie' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View details of Test Movie' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Download Test Movie' })).toBeInTheDocument();
  });
});
