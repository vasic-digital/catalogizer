import { useState } from 'react'
import { Star, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { useFavoriteStatus, useFavorites } from '@/hooks/useFavorites'
import { cn } from '@/lib/utils'
import type { MediaItem } from '@/types/media'

interface FavoriteToggleProps {
  mediaId: number
  mediaItem?: MediaItem
  size?: 'sm' | 'md' | 'lg'
  variant?: 'button' | 'icon' | 'card'
  className?: string
  showLabel?: boolean
  disabled?: boolean
}

const ICON_SIZE_CLASSES = {
  sm: 'w-3 h-3',
  md: 'w-4 h-4',
  lg: 'w-5 h-5'
}

/** Spinner while loading, otherwise the (optionally hover-previewed) favorite star. */
const ToggleIcon: React.FC<{
  isLoading: boolean
  isFavorite: boolean
  isHovered: boolean
  size: 'sm' | 'md' | 'lg'
  spaced?: boolean
}> = ({ isLoading, isFavorite, isHovered, size, spaced = false }) => {
  if (isLoading) {
    return <Loader2 className={cn('animate-spin', spaced && 'mr-2', ICON_SIZE_CLASSES[size])} />
  }
  return (
    <Star
      className={cn(
        spaced && 'mr-2',
        ICON_SIZE_CLASSES[size],
        isFavorite ? 'fill-current' : '',
        isHovered && !isFavorite && 'fill-current opacity-50'
      )}
    />
  )
}

interface VariantProps {
  size: 'sm' | 'md' | 'lg'
  className?: string
  showLabel: boolean
  disabled: boolean
  isFavorite: boolean
  isLoading: boolean
  isHovered: boolean
  handleToggle: (e: React.MouseEvent) => void
  handleMouseEnter: () => void
  handleMouseLeave: () => void
}

const IconVariant: React.FC<VariantProps> = ({ size, className, disabled, isFavorite, isLoading, isHovered, handleToggle, handleMouseEnter, handleMouseLeave }) => {
  return (
    <button
      className={cn(
        'relative inline-flex items-center justify-center p-1 rounded-full transition-all duration-200',
        'hover:bg-gray-100 dark:hover:bg-gray-800',
        'focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500',
        isFavorite && 'text-yellow-500',
        !isFavorite && 'text-gray-400 hover:text-yellow-500',
        isLoading && 'opacity-50 cursor-not-allowed',
        disabled && 'opacity-50 cursor-not-allowed',
        className
      )}
      onClick={handleToggle}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      disabled={isLoading || disabled}
      title={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
    >
      <ToggleIcon isLoading={isLoading} isFavorite={isFavorite} isHovered={isHovered} size={size} />
    </button>
  )
}

const CardVariant: React.FC<VariantProps> = ({ size, className, disabled, isFavorite, isLoading, isHovered, handleToggle, handleMouseEnter, handleMouseLeave }) => {
  return (
    <div
      className={cn(
        'absolute top-2 right-2 z-10 opacity-0 transition-opacity duration-200',
        'group-hover:opacity-100',
        className
      )}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <button
        className={cn(
          'relative inline-flex items-center justify-center p-2 rounded-full transition-all duration-200',
          'bg-white/90 dark:bg-black/90 backdrop-blur-sm',
          'hover:bg-white dark:hover:bg-black',
          'shadow-md',
          isFavorite && 'text-yellow-500',
          !isFavorite && 'text-gray-600 hover:text-yellow-500',
          isLoading && 'opacity-50 cursor-not-allowed',
          disabled && 'opacity-50 cursor-not-allowed'
        )}
        onClick={handleToggle}
        disabled={isLoading || disabled}
        title={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
      >
        <ToggleIcon isLoading={isLoading} isFavorite={isFavorite} isHovered={isHovered} size={size} />
      </button>
    </div>
  )
}

const ButtonVariant: React.FC<VariantProps> = ({ size, className, showLabel, disabled, isFavorite, isLoading, isHovered, handleToggle, handleMouseEnter, handleMouseLeave }) => {
  return (
    <Button
      variant={isFavorite ? 'primary' : 'outline'}
      size={size}
      onClick={handleToggle}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      disabled={isLoading || disabled}
      className={cn(
        'transition-all duration-200',
        isFavorite && 'bg-yellow-50 border-yellow-300 text-yellow-700 hover:bg-yellow-100 dark:bg-yellow-900/20 dark:border-yellow-600 dark:text-yellow-300',
        !isFavorite && 'hover:border-yellow-300 hover:text-yellow-600',
        className
      )}
    >
      <ToggleIcon isLoading={isLoading} isFavorite={isFavorite} isHovered={isHovered} size={size} spaced />
      {showLabel && (
        isLoading ? 'Loading...' : (isFavorite ? 'Remove from Favorites' : 'Add to Favorites')
      )}
    </Button>
  )
}

/**
 * FavoriteToggle renders a star button that toggles the favorite status
 * of a media item with optimistic UI updates.
 */
export const FavoriteToggle: React.FC<FavoriteToggleProps> = ({
  mediaId,
  mediaItem: _mediaItem,
  size = 'md',
  variant = 'button',
  className,
  showLabel = false,
  disabled = false
}) => {
  const [isHovered, setIsHovered] = useState(false)
  const { data: favoriteStatus, isLoading: statusLoading } = useFavoriteStatus(mediaId)
  const { toggleFavorite, isToggling } = useFavorites()
  
  const isFavorite = favoriteStatus?.is_favorite || false
  const isLoading = statusLoading || isToggling
  
  const handleToggle = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!isLoading && !disabled) {
      toggleFavorite(mediaId, isFavorite)
    }
  }

  const handleMouseEnter = () => setIsHovered(true)
  const handleMouseLeave = () => setIsHovered(false)

  const variantProps: VariantProps = {
    size, className, showLabel, disabled, isFavorite, isLoading, isHovered,
    handleToggle, handleMouseEnter, handleMouseLeave
  }

  if (variant === 'icon') return <IconVariant {...variantProps} />
  if (variant === 'card') return <CardVariant {...variantProps} />
  return <ButtonVariant {...variantProps} />
}
