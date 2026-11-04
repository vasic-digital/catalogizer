import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'
import { EntityHero, ChildrenList, FilesList, DuplicatesList } from '../EntityDetailView'
import type { MediaEntityDetail, MediaEntity, EntityFile } from '@/types/media'

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
}))

// Mock lucide-react icons
vi.mock('lucide-react', () => ({
  Star: (props: any) => <svg data-testid="icon-star" {...props} />,
  Clock: (props: any) => <svg data-testid="icon-clock" {...props} />,
  Globe: (props: any) => <svg data-testid="icon-globe" {...props} />,
  Calendar: (props: any) => <svg data-testid="icon-calendar" {...props} />,
  Folder: (props: any) => <svg data-testid="icon-folder" {...props} />,
  FileText: (props: any) => <svg data-testid="icon-filetext" {...props} />,
  Copy: (props: any) => <svg data-testid="icon-copy" {...props} />,
  Heart: (props: any) => <svg data-testid="icon-heart" {...props} />,
  RefreshCw: (props: any) => <svg data-testid="icon-refresh" {...props} />,
  ChevronRight: (props: any) => <svg data-testid="icon-chevron-right" {...props} />,
  Play: (props: any) => <svg data-testid="icon-play" {...props} />,
  Download: (props: any) => <svg data-testid="icon-download" {...props} />,
  Film: (props: any) => <svg data-testid="icon-film" {...props} />,
  Tv: (props: any) => <svg data-testid="icon-tv" {...props} />,
  Music: (props: any) => <svg data-testid="icon-music" {...props} />,
  Gamepad2: (props: any) => <svg data-testid="icon-gamepad" {...props} />,
  Monitor: (props: any) => <svg data-testid="icon-monitor" {...props} />,
  BookOpen: (props: any) => <svg data-testid="icon-bookopen" {...props} />,
  Book: (props: any) => <svg data-testid="icon-book" {...props} />,
}))

vi.mock('@/lib/utils', () => ({
  cn: (...args: any[]) => args.filter(Boolean).join(' '),
}))

const mockEntityDetail: MediaEntityDetail = {
  id: 1,
  media_type_id: 1,
  media_type: 'movie',
  title: 'Inception',
  original_title: 'Inception',
  year: 2010,
  description: 'A thief who steals corporate secrets through dream-sharing technology.',
  genre: ['Action', 'Sci-Fi', 'Thriller'],
  director: 'Christopher Nolan',
  rating: 8.8,
  runtime: 148,
  language: 'English',
  status: 'movie',
  file_count: 3,
  children_count: 0,
  external_metadata: [],
  first_detected: '2024-01-01T00:00:00Z',
  last_updated: '2024-01-01T00:00:00Z',
}

const mockFiles: EntityFile[] = [
  {
    id: 1,
    media_item_id: 1,
    file_id: 101,
    quality_info: '1080p Blu-ray',
    language: 'English',
    is_primary: true,
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    media_item_id: 1,
    file_id: 102,
    quality_info: '720p WebRip',
    language: 'Spanish',
    is_primary: false,
    created_at: '2024-01-02T00:00:00Z',
  },
]

const mockChildren: MediaEntity[] = [
  {
    id: 10,
    media_type_id: 3,
    title: 'Season 1',
    season_number: 1,
    year: 2020,
    status: 'tv_season',
    first_detected: '2024-01-01T00:00:00Z',
    last_updated: '2024-01-01T00:00:00Z',
  },
  {
    id: 11,
    media_type_id: 3,
    title: 'Season 2',
    season_number: 2,
    year: 2021,
    status: 'tv_season',
    first_detected: '2024-01-01T00:00:00Z',
    last_updated: '2024-01-01T00:00:00Z',
  },
]

const mockDuplicates: MediaEntity[] = [
  {
    id: 20,
    media_type_id: 1,
    title: 'Inception (Duplicate)',
    year: 2010,
    status: 'movie',
    first_detected: '2024-01-01T00:00:00Z',
    last_updated: '2024-01-01T00:00:00Z',
  },
]

// ---- EntityHero tests ----

describe('EntityHero', () => {
  const defaultProps = {
    entity: mockEntityDetail,
    files: mockFiles,
    duplicateCount: 1,
    onFavorite: vi.fn(),
    onRefresh: vi.fn(),
    refreshPending: false,
  }

  it('renders the entity title', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('Inception')).toBeInTheDocument()
  })

  it('renders the year with calendar icon', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('2010')).toBeInTheDocument()
    expect(screen.getByTestId('icon-calendar')).toBeInTheDocument()
  })

  it('renders the rating with star icon', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('8.8')).toBeInTheDocument()
    expect(screen.getByTestId('icon-star')).toBeInTheDocument()
  })

  it('renders the runtime', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('148 min')).toBeInTheDocument()
  })

  it('renders the language', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('English')).toBeInTheDocument()
  })

  it('renders genre badges', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('Action')).toBeInTheDocument()
    expect(screen.getByText('Sci-Fi')).toBeInTheDocument()
    expect(screen.getByText('Thriller')).toBeInTheDocument()
  })

  it('renders the director', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('Christopher Nolan')).toBeInTheDocument()
    expect(screen.getByText(/Directed by/)).toBeInTheDocument()
  })

  it('renders the description', () => {
    render(<EntityHero {...defaultProps} />)
    expect(
      screen.getByText('A thief who steals corporate secrets through dream-sharing technology.')
    ).toBeInTheDocument()
  })

  it('renders the media type badge', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('movie')).toBeInTheDocument()
  })

  it('renders Favorite button and calls onFavorite', async () => {
    const user = userEvent.setup()
    const handleFavorite = vi.fn()
    render(<EntityHero {...defaultProps} onFavorite={handleFavorite} />)

    const favoriteButton = screen.getByText('Favorite').closest('button')!
    await user.click(favoriteButton)
    expect(handleFavorite).toHaveBeenCalledTimes(1)
  })

  it('renders Refresh button and calls onRefresh', async () => {
    const user = userEvent.setup()
    const handleRefresh = vi.fn()
    render(<EntityHero {...defaultProps} canRefresh onRefresh={handleRefresh} />)

    const refreshButton = screen.getByText('Refresh').closest('button')!
    await user.click(refreshButton)
    expect(handleRefresh).toHaveBeenCalledTimes(1)
  })

  it('hides the Refresh button by default (canRefresh defaults to false)', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.queryByText('Refresh')).not.toBeInTheDocument()
  })

  it('hides the Refresh button when canRefresh is false (non-admin)', () => {
    render(<EntityHero {...defaultProps} canRefresh={false} />)
    expect(screen.queryByText('Refresh')).not.toBeInTheDocument()
    expect(screen.getByText('Favorite')).toBeInTheDocument()
  })

  it('disables Refresh button when refreshPending is true', () => {
    render(<EntityHero {...defaultProps} canRefresh refreshPending={true} />)
    const refreshButton = screen.getByText('Refresh').closest('button')!
    expect(refreshButton).toBeDisabled()
  })

  it('renders Play and Download links when files exist', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('Play')).toBeInTheDocument()
    expect(screen.getByText('Download')).toBeInTheDocument()
  })

  it('does not render Play and Download links when no files', () => {
    render(<EntityHero {...defaultProps} files={[]} />)
    expect(screen.queryByText('Play')).not.toBeInTheDocument()
    expect(screen.queryByText('Download')).not.toBeInTheDocument()
  })

  it('renders file count stat', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('Files')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('renders children count stat', () => {
    render(<EntityHero {...defaultProps} />)
    expect(screen.getByText('Children')).toBeInTheDocument()
    expect(screen.getByText('0')).toBeInTheDocument()
  })

  it('renders duplicate count when greater than 0', () => {
    render(<EntityHero {...defaultProps} duplicateCount={2} />)
    expect(screen.getByText('Duplicates')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('does not render duplicate stat when count is 0', () => {
    render(<EntityHero {...defaultProps} duplicateCount={0} />)
    expect(screen.queryByText('Duplicates')).not.toBeInTheDocument()
  })

  it('does not render original title when same as title', () => {
    render(<EntityHero {...defaultProps} />)
    // original_title === title, so it should not be shown separately
    const allInception = screen.getAllByText('Inception')
    expect(allInception).toHaveLength(1)
  })

  it('renders original title when different from title', () => {
    const entityWithOriginal = {
      ...mockEntityDetail,
      title: 'Inception (English)',
      original_title: 'Inception',
    }
    render(<EntityHero {...defaultProps} entity={entityWithOriginal} />)
    expect(screen.getByText('Inception (English)')).toBeInTheDocument()
    expect(screen.getByText('Inception')).toBeInTheDocument()
  })

  it('does not render year when absent', () => {
    const noYearEntity = { ...mockEntityDetail, year: undefined }
    render(<EntityHero {...defaultProps} entity={noYearEntity} />)
    expect(screen.queryByTestId('icon-calendar')).not.toBeInTheDocument()
  })

  it('does not render rating when null', () => {
    const noRatingEntity = { ...mockEntityDetail, rating: undefined }
    render(<EntityHero {...defaultProps} entity={noRatingEntity} />)
    expect(screen.queryByText('8.8')).not.toBeInTheDocument()
  })

  it('does not render runtime when null', () => {
    const noRuntimeEntity = { ...mockEntityDetail, runtime: undefined }
    render(<EntityHero {...defaultProps} entity={noRuntimeEntity} />)
    expect(screen.queryByText(/min/)).not.toBeInTheDocument()
  })

  it('does not render director when absent', () => {
    const noDirectorEntity = { ...mockEntityDetail, director: undefined }
    render(<EntityHero {...defaultProps} entity={noDirectorEntity} />)
    expect(screen.queryByText(/Directed by/)).not.toBeInTheDocument()
  })

  it('does not render description when absent', () => {
    const noDescEntity = { ...mockEntityDetail, description: undefined }
    render(<EntityHero {...defaultProps} entity={noDescEntity} />)
    expect(
      screen.queryByText('A thief who steals corporate secrets through dream-sharing technology.')
    ).not.toBeInTheDocument()
  })
})

// ---- ChildrenList tests ----

describe('ChildrenList', () => {
  it('renders children with Seasons label for tv_show', () => {
    render(
      <ChildrenList items={mockChildren} mediaType="tv_show" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('Seasons')).toBeInTheDocument()
    expect(screen.getByText('Season 1')).toBeInTheDocument()
    expect(screen.getByText('Season 2')).toBeInTheDocument()
  })

  it('renders children with Episodes label for tv_season', () => {
    const episodes: MediaEntity[] = [
      {
        id: 100,
        media_type_id: 4,
        title: 'Pilot',
        episode_number: 1,
        status: 'tv_episode',
        first_detected: '2024-01-01T00:00:00Z',
        last_updated: '2024-01-01T00:00:00Z',
      },
    ]
    render(
      <ChildrenList items={episodes} mediaType="tv_season" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('Episodes')).toBeInTheDocument()
    expect(screen.getByText('Pilot')).toBeInTheDocument()
  })

  it('renders children with Tracks label for music_album', () => {
    const tracks: MediaEntity[] = [
      {
        id: 200,
        media_type_id: 7,
        title: 'Track 1',
        track_number: 1,
        status: 'song',
        first_detected: '2024-01-01T00:00:00Z',
        last_updated: '2024-01-01T00:00:00Z',
      },
    ]
    render(
      <ChildrenList items={tracks} mediaType="music_album" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('Tracks')).toBeInTheDocument()
    expect(screen.getByText('Track 1')).toBeInTheDocument()
  })

  it('renders children with Children label for other types', () => {
    const children: MediaEntity[] = [
      {
        id: 300,
        media_type_id: 1,
        title: 'Child Item',
        status: 'movie',
        first_detected: '2024-01-01T00:00:00Z',
        last_updated: '2024-01-01T00:00:00Z',
      },
    ]
    render(
      <ChildrenList items={children} mediaType="game" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('Children')).toBeInTheDocument()
  })

  it('shows the count in parentheses', () => {
    render(
      <ChildrenList items={mockChildren} mediaType="tv_show" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('(2)')).toBeInTheDocument()
  })

  it('returns null when children array is empty', () => {
    const { container } = render(
      <ChildrenList items={[]} mediaType="tv_show" onChildClick={vi.fn()} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('calls onChildClick with the child id', async () => {
    const user = userEvent.setup()
    const handleChildClick = vi.fn()
    render(
      <ChildrenList items={mockChildren} mediaType="tv_show" onChildClick={handleChildClick} />
    )

    await user.click(screen.getByText('Season 1'))
    expect(handleChildClick).toHaveBeenCalledWith(10)

    await user.click(screen.getByText('Season 2'))
    expect(handleChildClick).toHaveBeenCalledWith(11)
  })

  it('shows season number for seasons', () => {
    render(
      <ChildrenList items={mockChildren} mediaType="tv_show" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('shows year when present on a child', () => {
    render(
      <ChildrenList items={mockChildren} mediaType="tv_show" onChildClick={vi.fn()} />
    )
    expect(screen.getByText('2020')).toBeInTheDocument()
    expect(screen.getByText('2021')).toBeInTheDocument()
  })
})

// ---- FilesList tests ----

describe('FilesList', () => {
  it('renders files with file IDs', () => {
    render(<FilesList files={mockFiles} />)
    expect(screen.getByText('File #101')).toBeInTheDocument()
    expect(screen.getByText('File #102')).toBeInTheDocument()
  })

  it('renders quality info', () => {
    render(<FilesList files={mockFiles} />)
    expect(screen.getByText('1080p Blu-ray')).toBeInTheDocument()
    expect(screen.getByText('720p WebRip')).toBeInTheDocument()
  })

  it('renders language badge', () => {
    render(<FilesList files={mockFiles} />)
    expect(screen.getByText('English')).toBeInTheDocument()
    expect(screen.getByText('Spanish')).toBeInTheDocument()
  })

  it('renders Primary badge for primary files', () => {
    render(<FilesList files={mockFiles} />)
    expect(screen.getByText('Primary')).toBeInTheDocument()
  })

  it('does not show Primary badge for non-primary files', () => {
    render(<FilesList files={mockFiles} />)
    // Only one Primary badge should exist
    const primaryBadges = screen.getAllByText('Primary')
    expect(primaryBadges).toHaveLength(1)
  })

  it('shows the file count in parentheses', () => {
    render(<FilesList files={mockFiles} />)
    expect(screen.getByText('(2)')).toBeInTheDocument()
  })

  it('shows "Files" heading', () => {
    render(<FilesList files={mockFiles} />)
    expect(screen.getByText('Files')).toBeInTheDocument()
  })

  it('returns null when files array is empty', () => {
    const { container } = render(<FilesList files={[]} />)
    expect(container.innerHTML).toBe('')
  })

  it('handles files without quality info', () => {
    const noQualityFiles: EntityFile[] = [
      {
        id: 5,
        media_item_id: 1,
        file_id: 500,
        is_primary: false,
        created_at: '2024-01-01T00:00:00Z',
      },
    ]
    render(<FilesList files={noQualityFiles} />)
    expect(screen.getByText('File #500')).toBeInTheDocument()
  })

  it('handles files without language', () => {
    const noLangFiles: EntityFile[] = [
      {
        id: 6,
        media_item_id: 1,
        file_id: 600,
        quality_info: '4K HDR',
        is_primary: true,
        created_at: '2024-01-01T00:00:00Z',
      },
    ]
    render(<FilesList files={noLangFiles} />)
    expect(screen.getByText('File #600')).toBeInTheDocument()
    expect(screen.getByText('4K HDR')).toBeInTheDocument()
    expect(screen.queryByText('English')).not.toBeInTheDocument()
  })
})

// ---- DuplicatesList tests ----

describe('DuplicatesList', () => {
  it('renders duplicates with title', () => {
    render(<DuplicatesList duplicates={mockDuplicates} onDuplicateClick={vi.fn()} />)
    expect(screen.getByText('Inception (Duplicate)')).toBeInTheDocument()
  })

  it('renders "Potential Duplicates" heading', () => {
    render(<DuplicatesList duplicates={mockDuplicates} onDuplicateClick={vi.fn()} />)
    expect(screen.getByText('Potential Duplicates')).toBeInTheDocument()
  })

  it('shows the duplicate count in parentheses', () => {
    render(<DuplicatesList duplicates={mockDuplicates} onDuplicateClick={vi.fn()} />)
    expect(screen.getByText('(1)')).toBeInTheDocument()
  })

  it('shows year in parentheses for duplicates', () => {
    render(<DuplicatesList duplicates={mockDuplicates} onDuplicateClick={vi.fn()} />)
    expect(screen.getByText('(2010)')).toBeInTheDocument()
  })

  it('shows the status of duplicates', () => {
    render(<DuplicatesList duplicates={mockDuplicates} onDuplicateClick={vi.fn()} />)
    expect(screen.getByText('movie')).toBeInTheDocument()
  })

  it('calls onDuplicateClick with the duplicate id', async () => {
    const user = userEvent.setup()
    const handleDuplicateClick = vi.fn()
    render(<DuplicatesList duplicates={mockDuplicates} onDuplicateClick={handleDuplicateClick} />)

    await user.click(screen.getByText('Inception (Duplicate)'))
    expect(handleDuplicateClick).toHaveBeenCalledWith(20)
  })

  it('returns null when duplicates array is empty', () => {
    const { container } = render(
      <DuplicatesList duplicates={[]} onDuplicateClick={vi.fn()} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders multiple duplicates', () => {
    const multipleDups: MediaEntity[] = [
      {
        id: 20,
        media_type_id: 1,
        title: 'Duplicate 1',
        year: 2010,
        status: 'movie',
        first_detected: '2024-01-01T00:00:00Z',
        last_updated: '2024-01-01T00:00:00Z',
      },
      {
        id: 21,
        media_type_id: 1,
        title: 'Duplicate 2',
        year: 2011,
        status: 'movie',
        first_detected: '2024-01-01T00:00:00Z',
        last_updated: '2024-01-01T00:00:00Z',
      },
    ]
    render(<DuplicatesList duplicates={multipleDups} onDuplicateClick={vi.fn()} />)
    expect(screen.getByText('Duplicate 1')).toBeInTheDocument()
    expect(screen.getByText('Duplicate 2')).toBeInTheDocument()
    expect(screen.getByText('(2)')).toBeInTheDocument()
  })
})
