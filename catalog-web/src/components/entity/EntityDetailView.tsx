import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button, buttonVariants } from '@/components/ui/Button'
import {
  Star, Clock, Globe, Calendar, Folder, FileText, Copy, Heart,
  RefreshCw, ChevronRight, Play, Download, Film,
} from 'lucide-react'
import { motion } from 'framer-motion'
import { cn } from '@/lib/utils'
import { TYPE_ICONS } from './TypeSelector'
import type { MediaEntityDetail, MediaEntity, EntityFile } from '@/types/media'

/**
 * EntityHero renders the hero section for an entity detail page with title,
 * metadata badges, description, and favorite/refresh actions.
 */
export function EntityHero({
  entity,
  files,
  duplicateCount,
  onFavorite,
  onRefresh,
  refreshPending,
  canRefresh = true,
}: {
  entity: MediaEntityDetail
  files: EntityFile[]
  duplicateCount: number
  onFavorite: () => void
  onRefresh: () => void
  refreshPending: boolean
  /** Metadata refresh triggers outbound provider calls and is admin-only on the API; hide it for non-admins. */
  canRefresh?: boolean
}) {
  const Icon = TYPE_ICONS[entity.media_type] || Film

  return (
    <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
      <Card>
        <CardContent className="p-6">
          <div className="flex flex-col sm:flex-row items-start gap-4 sm:gap-6">
            <div className="w-20 h-20 rounded-xl bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center flex-shrink-0">
              <Icon className="h-10 w-10 text-white" />
            </div>
            <div className="flex-1 min-w-0">
              <h1 className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white break-words">
                {entity.title}
              </h1>
              {entity.original_title && entity.original_title !== entity.title && (
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  {entity.original_title}
                </p>
              )}

              <div className="flex flex-wrap items-center gap-4 mt-3 text-sm text-gray-600 dark:text-gray-400">
                <span className="capitalize px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded">
                  {entity.media_type.replace(/_/g, ' ')}
                </span>
                {entity.year && (
                  <span className="flex items-center gap-1">
                    <Calendar className="h-4 w-4" /> {entity.year}
                  </span>
                )}
                {entity.rating != null && (
                  <span className="flex items-center gap-1">
                    <Star className="h-4 w-4 text-yellow-500" /> {entity.rating.toFixed(1)}
                  </span>
                )}
                {entity.runtime != null && (
                  <span className="flex items-center gap-1">
                    <Clock className="h-4 w-4" /> {entity.runtime} min
                  </span>
                )}
                {entity.language && (
                  <span className="flex items-center gap-1">
                    <Globe className="h-4 w-4" /> {entity.language}
                  </span>
                )}
              </div>

              {entity.genre && entity.genre.length > 0 && (
                <div className="flex flex-wrap gap-2 mt-3">
                  {entity.genre.map((g) => (
                    <span key={g} className="px-3 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-800 rounded-full text-gray-700 dark:text-gray-300">
                      {g}
                    </span>
                  ))}
                </div>
              )}

              {entity.director && (
                <p className="text-sm text-gray-600 dark:text-gray-400 mt-2">
                  Directed by <span className="font-medium">{entity.director}</span>
                </p>
              )}

              {entity.description && (
                <p className="text-gray-700 dark:text-gray-300 mt-4 leading-relaxed">
                  {entity.description}
                </p>
              )}
            </div>

            <div className="flex flex-row flex-wrap sm:flex-col gap-2 flex-shrink-0">
              <Button variant="outline" size="sm" onClick={onFavorite}>
                <Heart className="h-4 w-4 mr-1" /> Favorite
              </Button>
              {canRefresh && (
                <Button variant="outline" size="sm" onClick={onRefresh} disabled={refreshPending}>
                  <RefreshCw className={`h-4 w-4 mr-1 ${refreshPending ? 'animate-spin' : ''}`} /> Refresh
                </Button>
              )}
              {files.length > 0 && (
                <>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={async () => {
                      try {
                        const res = await fetch(`/api/v1/entities/${entity.id}/stream`)
                        if (!res.ok) throw new Error(`HTTP ${res.status}`)
                        const data = await res.json()
                        if (data.stream_url) {
                          window.open(data.stream_url, '_blank')
                        }
                      } catch {
                        // Fall back to download link
                        window.open(`/api/v1/entities/${entity.id}/download`, '_blank')
                      }
                    }}
                  >
                    <Play className="h-4 w-4 mr-1" /> Play
                  </Button>
                  <a
                    href={`/api/v1/entities/${entity.id}/download`}
                    className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
                  >
                    <Download className="h-4 w-4 mr-1" /> Download
                  </a>
                </>
              )}
            </div>
          </div>

          <div className="flex gap-6 mt-6 pt-4 border-t border-gray-200 dark:border-gray-700">
            <div className="text-center">
              <div className="text-lg font-semibold text-gray-900 dark:text-white">{entity.file_count}</div>
              <div className="text-xs text-gray-500">Files</div>
            </div>
            <div className="text-center">
              <div className="text-lg font-semibold text-gray-900 dark:text-white">{entity.children_count}</div>
              <div className="text-xs text-gray-500">Children</div>
            </div>
            {duplicateCount > 0 && (
              <div className="text-center">
                <div className="text-lg font-semibold text-orange-600">{duplicateCount}</div>
                <div className="text-xs text-gray-500">Duplicates</div>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

/**
 * ChildrenList displays child entities (seasons, episodes, albums, songs)
 * in a navigable list within an entity hierarchy.
 */
export function ChildrenList({
  items,
  mediaType,
  onChildClick,
}: {
  items: MediaEntity[]
  mediaType: string
  onChildClick: (id: number) => void
}) {
  if (items.length === 0) return null

  const label =
    mediaType === 'tv_show' ? 'Seasons'
    : mediaType === 'tv_season' ? 'Episodes'
    : mediaType === 'music_album' ? 'Tracks'
    : 'Children'

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Folder className="h-5 w-5" />
          {label}
          <span className="text-sm font-normal text-gray-500">({items.length})</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {items.map((child) => (
            <button
              key={child.id}
              onClick={() => onChildClick(child.id)}
              className="w-full flex items-center gap-3 p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors text-left"
            >
              <div className="w-8 h-8 rounded bg-gray-100 dark:bg-gray-700 flex items-center justify-center text-sm font-medium text-gray-600 dark:text-gray-400">
                {child.season_number || child.episode_number || child.track_number || '#'}
              </div>
              <div className="flex-1 min-w-0">
                <span className="font-medium text-gray-900 dark:text-white truncate block">
                  {child.title}
                </span>
              </div>
              {child.year && <span className="text-sm text-gray-500">{child.year}</span>}
              <ChevronRight className="h-4 w-4 text-gray-400" />
            </button>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

/**
 * FilesList renders a table of files associated with a media entity,
 * showing path, size, and format details.
 */
export function FilesList({ files }: { files: EntityFile[] }) {
  if (files.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <FileText className="h-5 w-5" />
          Files
          <span className="text-sm font-normal text-gray-500">({files.length})</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {files.map((file) => (
            <div key={file.id} className="flex items-center gap-3 p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
              <FileText className="h-5 w-5 text-gray-400" />
              <div className="flex-1 min-w-0">
                <span className="text-sm text-gray-900 dark:text-white">File #{file.file_id}</span>
                {file.quality_info && <span className="ml-2 text-xs text-gray-500">{file.quality_info}</span>}
              </div>
              {file.language && (
                <span className="text-xs px-2 py-0.5 bg-gray-200 dark:bg-gray-700 rounded">{file.language}</span>
              )}
              {file.is_primary && (
                <span className="text-xs px-2 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded">Primary</span>
              )}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

/**
 * DuplicatesList displays potential duplicate entities detected for the
 * current media item, with navigation to each duplicate.
 */
export function DuplicatesList({
  duplicates,
  onDuplicateClick,
}: {
  duplicates: MediaEntity[]
  onDuplicateClick: (id: number) => void
}) {
  if (duplicates.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-orange-600">
          <Copy className="h-5 w-5" />
          Potential Duplicates
          <span className="text-sm font-normal text-gray-500">({duplicates.length})</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-2">
          {duplicates.map((dup) => (
            <button
              key={dup.id}
              onClick={() => onDuplicateClick(dup.id)}
              className="w-full flex items-center gap-3 p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors text-left"
            >
              <div className="flex-1 min-w-0">
                <span className="font-medium text-gray-900 dark:text-white">{dup.title}</span>
                {dup.year && <span className="text-sm text-gray-500 ml-2">({dup.year})</span>}
              </div>
              <span className="text-xs text-gray-500 capitalize">{dup.status}</span>
              <ChevronRight className="h-4 w-4 text-gray-400" />
            </button>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
