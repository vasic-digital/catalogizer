import React from 'react'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import type { MediaSearchRequest } from '@/types/media'
import { MEDIA_TYPES, QUALITY_LEVELS } from '@/types/media'
import { Filter, X, Search } from 'lucide-react'

interface MediaFiltersProps {
  filters: MediaSearchRequest
  onFiltersChange: (filters: MediaSearchRequest) => void
  onReset: () => void
  className?: string
}

/**
 * MediaFilters provides filter controls for media search including type, quality,
 * and text search with active-filter indicators and a reset option.
 */
export const MediaFilters: React.FC<MediaFiltersProps> = ({
  filters,
  onFiltersChange,
  onReset,
  className = ''
}) => {
  const updateFilter = (key: keyof MediaSearchRequest, value: string | number | string[] | undefined) => {
    onFiltersChange({
      ...filters,
      [key]: value,
    })
  }

  const clearFilter = (key: keyof MediaSearchRequest) => {
    const newFilters = { ...filters }
    delete newFilters[key]
    onFiltersChange(newFilters)
  }

  const hasActiveFilters = Object.keys(filters).some(
    key => key !== 'limit' && key !== 'offset' && filters[key as keyof MediaSearchRequest] !== undefined
  )

  return (
    <Card className={className}>
      <CardHeader className="pb-4">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center text-lg">
            <Filter className="h-5 w-5 mr-2" />
            Filters
          </CardTitle>
          {hasActiveFilters && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onReset}
              className="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
            >
              <X className="h-4 w-4 mr-1" />
              Clear all
            </Button>
          )}
        </div>
      </CardHeader>

      <CardContent className="space-y-6">
        {/* Search Query */}
        <div>
          <Input
            label="Search"
            type="text"
            placeholder="Search media titles..."
            value={filters.query || ''}
            onChange={(e) => updateFilter('query', e.target.value || undefined)}
            icon={<Search className="h-4 w-4" />}
          />
        </div>

        {/* Media Type */}
        <div>
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 block">
            Media Type
          </label>
          <div className="flex flex-wrap gap-2">
            {MEDIA_TYPES.map((type) => (
              <Button
                key={type}
                variant={filters.media_type === type ? 'default' : 'outline'}
                size="sm"
                onClick={() =>
                  filters.media_type === type
                    ? clearFilter('media_type')
                    : updateFilter('media_type', type)
                }
                className="text-xs"
              >
                {type.replace('_', ' ')}
                {filters.media_type === type && (
                  <X className="h-3 w-3 ml-1" />
                )}
              </Button>
            ))}
          </div>
        </div>

        {/* Quality */}
        <div>
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 block">
            Quality
          </label>
          <div className="flex flex-wrap gap-2">
            {QUALITY_LEVELS.map((quality) => (
              <Button
                key={quality}
                variant={filters.quality?.includes(quality) ? 'default' : 'outline'}
                size="sm"
                onClick={() =>
                  filters.quality?.includes(quality)
                    ? clearFilter('quality')
                    : updateFilter('quality', [quality])
                }
                className="text-xs"
              >
                {quality.toUpperCase()}
                {filters.quality?.includes(quality) && (
                  <X className="h-3 w-3 ml-1" />
                )}
              </Button>
            ))}
          </div>
        </div>

        {/* Year Range */}
        <div>
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 block">
            Year Range
          </label>
          <div className="grid grid-cols-2 gap-3">
            <Input
              type="number"
              placeholder="From"
              value={filters.year_min || ''}
              onChange={(e) =>
                updateFilter('year_min', e.target.value ? Number(e.target.value) : undefined)
              }
              min="1900"
              max={new Date().getFullYear()}
            />
            <Input
              type="number"
              placeholder="To"
              value={filters.year_max || ''}
              onChange={(e) =>
                updateFilter('year_max', e.target.value ? Number(e.target.value) : undefined)
              }
              min="1900"
              max={new Date().getFullYear()}
            />
          </div>
        </div>

        {/* Rating */}
        <div>
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 block">
            Minimum Rating
          </label>
          <Input
            type="number"
            placeholder="0.0"
            value={filters.rating_min || ''}
            onChange={(e) =>
              updateFilter('rating_min', e.target.value ? Number(e.target.value) : undefined)
            }
            min="0"
            max="10"
            step="0.1"
          />
        </div>

        {/* Sort Options */}
        <div>
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 block">
            Sort By
          </label>
          <div className="grid grid-cols-2 gap-3">
            <select
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
              value={filters.sort_by || 'updated_at'}
              onChange={(e) => updateFilter('sort_by', e.target.value)}
            >
              <option value="updated_at">Last Updated</option>
              <option value="created_at">Date Added</option>
              <option value="title">Title</option>
              <option value="year">Year</option>
              <option value="rating">Rating</option>
              <option value="file_size">File Size</option>
            </select>

            <select
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
              value={filters.sort_order || 'desc'}
              onChange={(e) => updateFilter('sort_order', e.target.value as 'asc' | 'desc')}
            >
              <option value="desc">Descending</option>
              <option value="asc">Ascending</option>
            </select>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}