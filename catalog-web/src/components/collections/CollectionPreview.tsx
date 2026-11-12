import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Play, Pause, Volume2, Eye, MoreHorizontal, Music, Film, Image, FileText, Clock, Star, X } from 'lucide-react'
import { SmartCollection } from '../../types/collections'
import { useCollection } from '../../hooks/useCollections'
import { Button } from '../ui/Button'

interface CollectionItem {
  id: string
  title: string
  artist?: string
  album?: string
  duration?: number
  media_type: string
  file_size: number
  date_added: string
  thumbnail_url?: string
  is_playing?: boolean
  is_favorite?: boolean
  rating?: number
}

interface CollectionPreviewProps {
  collection: SmartCollection
  isLoading?: boolean
  onClose: () => void
  onPlayItem?: (itemId: string) => void
  onPreviewItem?: (itemId: string) => void
  onViewAllItems?: () => void
}

const MEDIA_ICONS = {
  music: Music,
  video: Film,
  image: Image,
  document: FileText
}

const formatDuration = (seconds?: number) => {
  if (!seconds) return '--:--'
  const minutes = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${minutes}:${secs.toString().padStart(2, '0')}`
}

const formatFileSize = (bytes: number) => {
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0
  
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }
  
  return `${size.toFixed(1)} ${units[unitIndex]}`
}

const renderStars = (rating?: number) => {
  if (!rating) return null
  
  return (
    <div className="flex items-center gap-1">
      {[1, 2, 3, 4, 5].map((star) => (
        <Star
          key={star}
          className={`w-3 h-3 ${
            star <= rating 
              ? 'fill-yellow-400 text-yellow-400' 
              : 'text-gray-300'
          }`}
        />
      ))}
    </div>
  )
}

/**
 * CollectionPreview displays a modal with the items inside a smart collection,
 * including playback, preview, and file size details.
 */
export const CollectionPreview: React.FC<CollectionPreviewProps> = ({
  collection,
  isLoading = false,
  onClose,
  onPlayItem,
  onPreviewItem,
  onViewAllItems
}) => {
  const [hoveredItem, setHoveredItem] = useState<string | null>(null)
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set())

  // Fetch collection items using the hook
  const { collectionItems, isLoadingItems } = useCollection(collection.id)

  const handleItemSelect = (itemId: string, event: React.SyntheticEvent) => {
    event.stopPropagation()
    const newSelected = new Set(selectedItems)
    if (newSelected.has(itemId)) {
      newSelected.delete(itemId)
    } else {
      newSelected.add(itemId)
    }
    setSelectedItems(newSelected)
  }

  const handlePlayItem = (itemId: string, event: React.MouseEvent) => {
    event.stopPropagation()
    // CAT-001: the parent's handler does the work; no success is announced on its behalf.
    onPlayItem?.(itemId)
  }

  const handlePreviewItem = (itemId: string, event: React.MouseEvent) => {
    event.stopPropagation()
    onPreviewItem?.(itemId)
  }

  if (isLoading || isLoadingItems) {
    return (
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
        <motion.div
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.9 }}
          className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-4xl max-h-[80vh] overflow-hidden"
        >
          <div className="p-6">
            <div className="animate-pulse">
              <div className="h-6 bg-gray-200 rounded w-1/3 mb-4"></div>
              <div className="space-y-3">
                {[1, 2, 3, 4, 5].map((i) => (
                  <div key={i} className="flex items-center gap-3">
                    <div className="w-12 h-12 bg-gray-200 rounded"></div>
                    <div className="flex-1">
                      <div className="h-4 bg-gray-200 rounded w-3/4 mb-2"></div>
                      <div className="h-3 bg-gray-200 rounded w-1/2"></div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </motion.div>
      </div>
    )
  }

  const items = collectionItems?.items || []
  const totalItems = collectionItems?.total || 0

  return (
    <AnimatePresence>
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
        <motion.div
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.9 }}
          className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-4xl max-h-[80vh] overflow-hidden"
        >
          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                {collection.name}
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {totalItems} items total
              </p>
            </div>
            
            <div className="flex items-center gap-2">
              {selectedItems.size > 0 && (
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  {selectedItems.size} selected
                </span>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={onClose}
              >
                <X className="w-4 h-4" />
              </Button>
            </div>
          </div>

          {/* Content */}
          <div className="p-6 overflow-y-auto max-h-[calc(80vh-120px)]">
            <div className="space-y-2">
              <AnimatePresence>
                {items.map((item: CollectionItem, index: number) => {
                  const Icon = MEDIA_ICONS[item.media_type as keyof typeof MEDIA_ICONS]
                  const isHovered = hoveredItem === item.id
                  const isSelected = selectedItems.has(item.id)
                  
                  return (
                    <motion.div
                      key={item.id}
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -20 }}
                      transition={{ duration: 0.2, delay: index * 0.05 }}
                      className={`
                        group relative flex items-center gap-3 p-3 rounded-lg cursor-pointer
                        ${isSelected 
                          ? 'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700' 
                          : 'hover:bg-gray-50 dark:hover:bg-gray-800'
                        }
                      `}
                      onMouseEnter={() => setHoveredItem(item.id)}
                      onMouseLeave={() => setHoveredItem(null)}
                      onClick={() => handlePreviewItem(item.id, { stopPropagation: () => { /* noop */ } } as React.MouseEvent)}
                    >
                      {/* Selection Checkbox */}
                      <input
                        type="checkbox"
                        className="absolute left-2 top-1/2 -translate-y-1/2 w-4 h-4 cursor-pointer accent-blue-500"
                        checked={isSelected}
                        onChange={() => { /* handled in onClick to keep stopPropagation semantics */ }}
                        onClick={(e) => handleItemSelect(item.id, e)}
                        onKeyDown={(e) => {
                          // Native checkboxes toggle on Space only; keep the previous Enter-to-toggle behaviour.
                          if (e.key === 'Enter') {
                            e.preventDefault()
                            handleItemSelect(item.id, e)
                          }
                        }}
                        aria-label="Select item"
                      />

                      {/* Thumbnail/Icon */}
                      <div className="relative ml-6">
                        {item.thumbnail_url ? (
                          <img
                            src={item.thumbnail_url}
                            alt={item.title}
                            className="w-12 h-12 rounded object-cover"
                          />
                        ) : (
                          <div className="w-12 h-12 bg-gray-100 dark:bg-gray-800 rounded flex items-center justify-center">
                            <Icon className="w-6 h-6 text-gray-400" />
                          </div>
                        )}
                        
                        {/* Play/Pause Overlay */}
                        {isHovered && (
                          <motion.div
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            className="absolute inset-0 bg-black/40 rounded flex items-center justify-center"
                            onClick={(e) => handlePlayItem(item.id, e)}
                          >
                            {item.is_playing ? (
                              <Pause className="w-4 h-4 text-white" />
                            ) : (
                              <Play className="w-4 h-4 text-white" />
                            )}
                          </motion.div>
                        )}

                        {/* Favorite Badge */}
                        {item.is_favorite && (
                          <div className="absolute -top-1 -right-1">
                            <Star className="w-3 h-3 fill-yellow-400 text-yellow-400" />
                          </div>
                        )}
                      </div>

                      {/* Item Info */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <h4 className="font-medium text-gray-900 dark:text-white truncate">
                            {item.title}
                          </h4>
                          {item.rating && renderStars(item.rating)}
                        </div>
                        
                        <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                          {item.artist && (
                            <span className="truncate">{item.artist}</span>
                          )}
                          {item.album && (
                            <span className="truncate">{item.album}</span>
                          )}
                          <span className="flex items-center gap-1">
                            <Clock className="w-3 h-3" />
                            {formatDuration(item.duration)}
                          </span>
                          <span className="flex items-center gap-1">
                            <Volume2 className="w-3 h-3" />
                            {formatFileSize(item.file_size)}
                          </span>
                        </div>
                      </div>

                      {/* Actions */}
                      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                          title="More options"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <MoreHorizontal className="w-4 h-4 text-gray-500" />
                        </button>
                      </div>
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>

            {/* Empty State */}
            {items.length === 0 && (
              <div className="text-center py-12">
                <div className="w-16 h-16 bg-gray-100 dark:bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-4">
                  <Eye className="w-8 h-8 text-gray-400" />
                </div>
                <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
                  No items found
                </h3>
                <p className="text-gray-500 dark:text-gray-400">
                  This collection doesn&apos;t have any items yet.
                </p>
              </div>
            )}

            {/* Load More Indicator */}
            {items.length < totalItems && items.length > 0 && (
              <div className="text-center py-4">
                <Button
                  variant="ghost"
                  onClick={onViewAllItems}
                >
                  View All Items
                </Button>
              </div>
            )}
          </div>
        </motion.div>
      </div>
    </AnimatePresence>
  )
}

export default CollectionPreview