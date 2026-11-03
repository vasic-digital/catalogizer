import React from 'react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { FavoriteToggle } from '@/components/favorites/FavoriteToggle'
import { ProgressBadge } from '@/components/media/ProgressBadge'
import type { MediaItem } from '@/types/media'
import type { UiPlaybackProgress } from '@/types/playback'
import {
  Film,
  Music,
  Gamepad2,
  Monitor,
  BookOpen,
  Star,
  Calendar,
  HardDrive,
  Clock,
  ExternalLink,
  Download,
  Eye,
  Play
} from 'lucide-react'
import { motion } from 'framer-motion'
import { formatDate, truncateText } from '@/lib/utils'

interface MediaCardProps {
  media: MediaItem
  onView?: (media: MediaItem) => void
  onPlay?: (media: MediaItem) => void
  onDownload?: (media: MediaItem) => void
  progress?: UiPlaybackProgress | null
  onOpenHistory?: (media: MediaItem) => void
  className?: string
}

const getMediaIcon = (mediaType: string) => {
  switch (mediaType.toLowerCase()) {
    case 'movie':
    case 'tv_show':
    case 'documentary':
    case 'anime':
      return <Film className="h-5 w-5" />
    case 'music':
    case 'audiobook':
    case 'podcast':
      return <Music className="h-5 w-5" />
    case 'game':
      return <Gamepad2 className="h-5 w-5" />
    case 'software':
      return <Monitor className="h-5 w-5" />
    case 'ebook':
    case 'training':
      return <BookOpen className="h-5 w-5" />
    default:
      return <Film className="h-5 w-5" />
  }
}

const getQualityColor = (quality?: string) => {
  if (!quality) return 'bg-gray-100 text-gray-800'

  switch (quality.toLowerCase()) {
    case '4k':
    case 'hdr':
    case 'dolby_vision':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
    case '1080p':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
    case '720p':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
  }
}

const formatFileSize = (bytes?: number) => {
  if (!bytes) return 'Unknown'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unitIndex = 0

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }

  return `${size.toFixed(1)} ${units[unitIndex]}`
}

const formatDuration = (minutes?: number) => {
  if (!minutes) return null

  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60

  if (hours > 0) {
    return `${hours}h ${mins}m`
  }
  return `${mins}m`
}

/**
 * MediaCard displays a single media item as a card with poster, metadata,
 * quality badge, and action buttons for viewing, playing, and downloading.
 */
export const MediaCard: React.FC<MediaCardProps> = React.memo(({
  media,
  onView,
  onPlay,
  onDownload,
  progress,
  onOpenHistory,
  className = ''
}) => {
  return (
    <motion.div
      whileHover={{ y: -4 }}
      transition={{ duration: 0.2 }}
      className={className}
      data-testid={`media-item-${media.id}`}
    >
      <Card className="h-full overflow-hidden hover:shadow-xl transition-all duration-300 group">
        {/* Cover Image */}
        <div className="relative aspect-[3/4] bg-gradient-to-br from-gray-100 to-gray-200 dark:from-gray-700 dark:to-gray-800">
          {media.cover_image ? (
            <img
              src={media.cover_image}
              alt={media.title}
              className="w-full h-full object-cover"
              loading="lazy"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center text-gray-400 dark:text-gray-500">
              {getMediaIcon(media.media_type ?? '')}
            </div>
          )}

          {/* Overlay */}
          <div className="absolute inset-0 bg-black/0 group-hover:bg-black/20 transition-all duration-300 flex items-center justify-center opacity-0 group-hover:opacity-100">
            <div className="flex space-x-2">
              {onPlay && (
                <Button
                  size="sm"
                  variant="glass"
                  onClick={(e) => {
                    e.stopPropagation()
                    onPlay(media)
                  }}
                  data-testid={`play-button-${media.id}`}
                >
                  <Play className="h-4 w-4" />
                </Button>
              )}
              {onView && (
                <Button
                  size="sm"
                  variant="glass"
                  onClick={(e) => {
                    e.stopPropagation()
                    onView(media)
                  }}
                  data-testid={`view-button-${media.id}`}
                >
                  <Eye className="h-4 w-4" />
                </Button>
              )}
              {onDownload && (
                <Button
                  size="sm"
                  variant="glass"
                  onClick={(e) => {
                    e.stopPropagation()
                    onDownload(media)
                  }}
                  data-testid={`download-button-${media.id}`}
                >
                  <Download className="h-4 w-4" />
                </Button>
              )}
            </div>
          </div>

          {/* Quality Badge */}
          {media.quality && (
            <div className="absolute top-2 right-2">
              <span className={`px-2 py-1 rounded-full text-xs font-medium ${getQualityColor(media.quality)}`}>
                {media.quality.toUpperCase()}
              </span>
            </div>
          )}

          {/* Favorite Toggle */}
          <FavoriteToggle
            mediaId={media.id}
            mediaItem={media}
            variant="card"
            size="md"
          />

          {/* Media Type Badge */}
          <div className="absolute top-2 left-2">
            <span className="px-2 py-1 rounded-full text-xs font-medium bg-black/50 text-white backdrop-blur-sm">
              {media.media_type?.replace('_', ' ').toUpperCase()}
            </span>
          </div>

          {/* Reproduction progress overlay (opens HistoryDrawer on click) */}
          {progress && (
            <ProgressBadge
              progress={progress}
              onOpenHistory={() => onOpenHistory?.(media)}
            />
          )}
        </div>

        <CardContent className="p-4">
          {/* Title */}
          <h3 className="font-semibold text-gray-900 dark:text-white mb-2 line-clamp-2 break-words">
            {media.title}
          </h3>

          {/* Year and Rating */}
          <div className="flex items-center justify-between mb-3">
            {media.year && (
              <div className="flex items-center text-sm text-gray-600 dark:text-gray-400">
                <Calendar className="h-4 w-4 mr-1" />
                {media.year}
              </div>
            )}
            {media.rating && (
              <div className="flex items-center text-sm text-yellow-600">
                <Star className="h-4 w-4 mr-1 fill-current" />
                {media.rating.toFixed(1)}
              </div>
            )}
          </div>

          {/* Description */}
          {media.description && (
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-3 line-clamp-2">
              {truncateText(media.description, 100)}
            </p>
          )}

          {/* File Info */}
          <div className="space-y-2 text-xs text-gray-500 dark:text-gray-400">
            {media.file_size && (
              <div className="flex items-center">
                <HardDrive className="h-3 w-3 mr-1" />
                {formatFileSize(media.file_size)}
              </div>
            )}

            {media.duration && (
              <div className="flex items-center">
                <Clock className="h-3 w-3 mr-1" />
                {formatDuration(media.duration)}
              </div>
            )}
          </div>

          {/* Versions */}
          {media.versions && media.versions.length > 1 && (
            <div className="mt-3">
              <span className="text-xs text-gray-500 dark:text-gray-400">
                {media.versions.length} versions available
              </span>
            </div>
          )}

          {/* External Links */}
          {media.external_metadata && media.external_metadata.length > 0 && (
            <div className="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
              <div className="flex items-center space-x-2">
                <ExternalLink className="h-3 w-3 text-gray-400" />
                <span className="text-xs text-gray-500 dark:text-gray-400">
                  {media.external_metadata.length} external source{media.external_metadata.length > 1 ? 's' : ''}
                </span>
              </div>
            </div>
          )}

          {/* Last Updated */}
          <div className="mt-3 text-xs text-gray-400">
            Updated {media.updated_at ? formatDate(media.updated_at) : 'Unknown'}
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
})

MediaCard.displayName = 'MediaCard'