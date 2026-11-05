import { useState, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Grid,
  List,
  Heart,
  SortAsc,
  SortDesc,
  Star,
  Calendar,
  Award
} from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { MediaGrid } from '@/components/media/MediaGrid'
import { FavoriteToggle } from './FavoriteToggle'
import { useFavorites } from '@/hooks/useFavorites'
import { mediaApi } from '@/lib/mediaApi'
import type { MediaItem } from '@/types/media'

interface FavoritesGridProps {
  className?: string
  showFilters?: boolean
  showStats?: boolean
  selectable?: boolean
  onSelectChange?: (selectedItems: MediaItem[]) => void
}

/**
 * FavoritesGrid displays the user's favorited media items in a filterable,
 * sortable grid with search and view mode toggling.
 */
export const FavoritesGrid: React.FC<FavoritesGridProps> = ({
  className,
  showFilters = true,
  showStats = true,
  selectable = false,
  onSelectChange: _onSelectChange
}) => {
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [searchQuery, setSearchQuery] = useState('')
  const [mediaTypeFilter, setMediaTypeFilter] = useState<string>('all')
  const [sortBy, setSortBy] = useState<'created_at' | 'title' | 'rating' | 'year'>('created_at')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [selectedItems, setSelectedItems] = useState<number[]>([])

  const navigate = useNavigate()

  const {
    favorites,
    total: _total,
    isLoading,
    error,
    stats,
    toggleFavorite: _toggleFavorite,
    bulkRemoveFromFavorites
  } = useFavorites({
    sort_by: sortBy,
    sort_order: sortOrder,
    ...(mediaTypeFilter !== 'all' && { media_type: mediaTypeFilter })
  })

  // Filter favorites based on search query
  const filteredFavorites = useMemo(() => {
    if (!searchQuery) return favorites
    
    return favorites.filter(favorite => 
      favorite.media_item.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      favorite.media_item.media_type.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (favorite.media_item.year?.toString() || '').includes(searchQuery)
    )
  }, [favorites, searchQuery])

  // Convert favorites to media items for MediaGrid
  const mediaItems: MediaItem[] = useMemo(() => {
    return filteredFavorites.map(favorite => ({
      id: favorite.media_item.id,
      title: favorite.media_item.title,
      media_type: favorite.media_item.media_type,
      year: favorite.media_item.year,
      cover_image: favorite.media_item.cover_image,
      duration: favorite.media_item.duration,
      rating: favorite.media_item.rating,
      quality: favorite.media_item.quality,
      directory_path: '', // Add appropriate path
      storage_root_name: '',
      storage_root_protocol: '',
      created_at: favorite.created_at,
      updated_at: favorite.updated_at
    }))
  }, [filteredFavorites])

  // Enhanced media item with favorite toggle
  const enhancedMediaItems = useMemo(() => {
    return mediaItems.map(item => ({
      ...item,
      actions: (
        <div className="flex items-center gap-2">
          <FavoriteToggle
            mediaId={item.id}
            mediaItem={item}
            variant="icon"
            size="md"
          />
        </div>
      )
    }))
  }, [mediaItems])

  const handleSelectItem = (mediaId: number, selected: boolean) => {
    if (!selectable) return
    
    setSelectedItems(prev => {
      if (selected) {
        return [...prev, mediaId]
      } else {
        return prev.filter(id => id !== mediaId)
      }
    })
  }

  const handleSelectAll = () => {
    if (!selectable) return
    
    if (selectedItems.length === mediaItems.length) {
      setSelectedItems([])
    } else {
      setSelectedItems(mediaItems.map(item => item.id))
    }
  }

  const handleBulkRemoveFromFavorites = () => {
    if (selectedItems.length === 0) return
    bulkRemoveFromFavorites(selectedItems)
    setSelectedItems([])
  }

  const handleMediaView = useCallback((media: MediaItem) => {
    navigate(`/entity/${media.id}`)
  }, [navigate])

  const handleMediaPlay = useCallback((media: MediaItem) => {
    navigate(`/entity/${media.id}?autoplay=true`)
  }, [navigate])

  const handleMediaDownload = useCallback((media: MediaItem) => {
    mediaApi.downloadMedia(media)
  }, [])

  if (error) {
    return (
      <Card className={className}>
        <CardContent className="p-6">
          <div className="text-center text-red-600">
            <Heart className="w-12 h-12 mx-auto mb-4 opacity-50" />
            <p>Failed to load favorites</p>
            <p className="text-sm opacity-75 mt-1">{String(error)}</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className={className}>
      {/* Stats Section */}
      {showStats && stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-2">
                <Heart className="w-5 h-5 text-red-500" />
                <div>
                  <p className="text-sm text-gray-600 dark:text-gray-400">Total</p>
                  <p className="text-xl font-semibold">{stats.total_count}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-2">
                <Star className="w-5 h-5 text-yellow-500" />
                <div>
                  <p className="text-sm text-gray-600 dark:text-gray-400">Movies</p>
                  <p className="text-xl font-semibold">{stats.media_type_breakdown.movie}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-2">
                <Award className="w-5 h-5 text-blue-500" />
                <div>
                  <p className="text-sm text-gray-600 dark:text-gray-400">TV Shows</p>
                  <p className="text-xl font-semibold">{stats.media_type_breakdown.tv_show}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-2">
                <Calendar className="w-5 h-5 text-green-500" />
                <div>
                  <p className="text-sm text-gray-600 dark:text-gray-400">Music</p>
                  <p className="text-xl font-semibold">{stats.media_type_breakdown.music}</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Filters Section */}
      {showFilters && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="text-lg">Filters & Search</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col lg:flex-row gap-4">
              {/* Search */}
              <div className="flex-1">
                <Input
                  placeholder="Search favorites..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full"
                />
              </div>
              
              {/* Media Type Filter */}
              <Select aria-label="Filter by media type" value={mediaTypeFilter} onValueChange={setMediaTypeFilter}>
                <option value="all">All Types</option>
                <option value="movie">Movies</option>
                <option value="tv_show">TV Shows</option>
                <option value="music">Music</option>
                <option value="game">Games</option>
                <option value="documentary">Documentaries</option>
                <option value="anime">Anime</option>
              </Select>
              
              {/* Sort Options */}
              <div className="flex gap-2">
                <Select aria-label="Sort favorites" value={sortBy} onValueChange={(value: string) => setSortBy(value as 'created_at' | 'title' | 'rating' | 'year')}>
                  <option value="created_at">Date Added</option>
                  <option value="title">Title</option>
                  <option value="rating">Rating</option>
                  <option value="year">Year</option>
                </Select>
                
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')}
                  title={`Sort ${sortOrder === 'asc' ? 'descending' : 'ascending'}`}
                >
                  {sortOrder === 'asc' ? <SortAsc className="w-4 h-4" /> : <SortDesc className="w-4 h-4" />}
                </Button>
              </div>
              
              {/* View Mode */}
              <div className="flex gap-2">
                <Button
                  variant={viewMode === 'grid' ? 'default' : 'outline'}
                  size="icon"
                  onClick={() => setViewMode('grid')}
                  title="Grid view"
                >
                  <Grid className="w-4 h-4" />
                </Button>
                <Button
                  variant={viewMode === 'list' ? 'default' : 'outline'}
                  size="icon"
                  onClick={() => setViewMode('list')}
                  title="List view"
                >
                  <List className="w-4 h-4" />
                </Button>
              </div>
            </div>
            
            {/* Bulk Actions */}
            {selectable && selectedItems.length > 0 && (
              <div className="mt-4 flex items-center justify-between p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                <span className="text-sm text-blue-700 dark:text-blue-300">
                  {selectedItems.length} items selected
                </span>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={() => setSelectedItems([])}>
                    Clear Selection
                  </Button>
                  <Button 
                    variant="destructive" 
                    onClick={handleBulkRemoveFromFavorites}
                  >
                    Remove from Favorites
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Favorites Grid/List */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>
            Favorites ({filteredFavorites.length})
          </CardTitle>
          {selectable && (
            <Button variant="outline" onClick={handleSelectAll}>
              {selectedItems.length === mediaItems.length ? 'Deselect All' : 'Select All'}
            </Button>
          )}
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
              {Array.from({ length: 12 }).map((_, index) => (
                <div key={index} className="animate-pulse">
                  <div className="bg-gray-200 dark:bg-gray-700 rounded-lg aspect-video mb-2"></div>
                  <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded mb-1"></div>
                  <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-3/4"></div>
                </div>
              ))}
            </div>
          ) : filteredFavorites.length === 0 ? (
            <div className="text-center py-12">
              <Heart className="w-16 h-16 mx-auto mb-4 text-gray-400" />
              <h3 className="text-lg font-semibold mb-2">
                {searchQuery ? 'No favorites found' : 'No favorites yet'}
              </h3>
              <p className="text-gray-600 dark:text-gray-400">
                {searchQuery 
                  ? 'Try adjusting your search or filters'
                  : 'Start adding items to your favorites to see them here'
                }
              </p>
            </div>
          ) : (
            <MediaGrid
              media={enhancedMediaItems}
              viewMode={viewMode}
              onMediaView={handleMediaView}
              onMediaPlay={handleMediaPlay}
              onMediaDownload={handleMediaDownload}
              onSelect={selectable ? handleSelectItem : undefined}
              selectedItems={selectedItems}
              showActions={true}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}