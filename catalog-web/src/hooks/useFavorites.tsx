import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { favoritesApi } from '@/lib/favoritesApi'
import type { Favorite, FavoriteToggleRequest } from '@/types/favorites'
import toast from 'react-hot-toast'

/**
 * useFavorites manages favorite media items with list fetching, stats,
 * and an optimistic toggle mutation.
 *
 * @param params - Optional pagination, filtering, and sorting parameters
 * @returns Favorites list, stats, toggle action, and loading/error states
 */
export const useFavorites = (params?: {
  limit?: number
  offset?: number
  media_type?: string
  sort_by?: 'created_at' | 'title' | 'rating' | 'year'
  sort_order?: 'asc' | 'desc'
}) => {
  const queryClient = useQueryClient()

  const {
    data: favoritesData,
    isLoading: isLoadingFavorites,
    error: favoritesError,
    refetch: refetchFavorites
  } = useQuery({
    queryKey: ['favorites', params],
    queryFn: () => favoritesApi.getFavorites(params),
    staleTime: 1000 * 60 * 5, // 5 minutes
    cacheTime: 1000 * 60 * 10, // 10 minutes
  })

  const {
    data: favoriteStats,
    isLoading: isLoadingStats,
    error: statsError,
    refetch: refetchStats
  } = useQuery({
    queryKey: ['favorites-stats'],
    queryFn: () => favoritesApi.getFavoriteStats(),
    staleTime: 1000 * 60 * 2, // 2 minutes
    cacheTime: 1000 * 60 * 5, // 5 minutes
  })

  const toggleFavoriteMutation = useMutation({
    mutationFn: (request: FavoriteToggleRequest) => favoritesApi.toggleFavorite(request),
    onMutate: async (request) => {
      // Cancel any ongoing refetches
      await queryClient.cancelQueries(['favorites'])
      
      // Snapshot previous value
      const previousFavorites = queryClient.getQueryData(['favorites'])
      
      // Optimistically update
      queryClient.setQueryData(['favorites'], (old: { items?: Favorite[] } | undefined) => {
        if (!old?.items) return old
        
        if (request.is_favorite) {
          // Add to favorites: the full media item is not available here, so leave the cache as it is.
          // CAT-001 (R5): the success toast is shown in onSuccess, after the server accepted the add.
          return old
        } else {
          // Remove from favorites
          return {
            ...old,
            items: old.items.filter((fav: Favorite) =>
              !(fav.media_id === request.media_id && (!request.entity_type || fav.media_item.media_type === request.entity_type)))
          }
        }
      })
      
      return { previousFavorites }
    },
    onSuccess: (_data, request) => {
      if (request.is_favorite) toast.success('Added to favorites')
    },
    onError: (error, variables, context) => {
      // Revert optimistic update
      queryClient.setQueryData(['favorites'], context?.previousFavorites)
      toast.error('Failed to update favorite status')
    },
    onSettled: () => {
      // Refetch to ensure server state
      queryClient.invalidateQueries(['favorites'])
      queryClient.invalidateQueries(['favorites-stats'])
      queryClient.invalidateQueries(['favorite-status'])
    }
  })

  const bulkAddMutation = useMutation({
    mutationFn: (mediaIds: number[]) => favoritesApi.bulkAddToFavorites(mediaIds),
    onSuccess: (data) => {
      toast.success(`Added ${data.added} items to favorites`)
      queryClient.invalidateQueries(['favorites'])
      queryClient.invalidateQueries(['favorites-stats'])
    },
    onError: () => {
      toast.error('Failed to add items to favorites')
    }
  })

  const bulkRemoveMutation = useMutation({
    mutationFn: (mediaIds: number[]) => favoritesApi.bulkRemoveFromFavorites(mediaIds),
    onSuccess: (data) => {
      toast.success(`Removed ${data.removed} items from favorites`)
      queryClient.invalidateQueries(['favorites'])
      queryClient.invalidateQueries(['favorites-stats'])
    },
    onError: () => {
      toast.error('Failed to remove items from favorites')
    }
  })

  const checkFavoriteStatus = (mediaId: number, entityType?: string): boolean => {
    if (!favoritesData?.items) return false
    return favoritesData.items.some(fav => fav.media_id === mediaId && (!entityType || fav.media_item.media_type === entityType))
  }

  const toggleFavorite = (mediaId: number, currentStatus?: boolean, entityType?: string) => {
    const isFavorite = currentStatus ?? checkFavoriteStatus(mediaId, entityType)
    toggleFavoriteMutation.mutate({
      media_id: mediaId,
      is_favorite: !isFavorite,
      ...(entityType && { entity_type: entityType })
    })
  }

  const bulkAddToFavorites = (mediaIds: number[]) => {
    bulkAddMutation.mutate(mediaIds)
  }

  const bulkRemoveFromFavorites = (mediaIds: number[]) => {
    bulkRemoveMutation.mutate(mediaIds)
  }

  return {
    favorites: favoritesData?.items || [],
    total: favoritesData?.total || 0,
    isLoading: isLoadingFavorites || isLoadingStats,
    error: favoritesError || statsError,
    stats: favoriteStats,
    refetchFavorites,
    refetchStats,
    toggleFavorite,
    checkFavoriteStatus,
    bulkAddToFavorites,
    bulkRemoveFromFavorites,
    isToggling: toggleFavoriteMutation.isLoading
  }
}

/**
 * useFavoriteStatus checks whether a single media item is favorited.
 *
 * @param mediaId - The ID of the media item to check
 * @returns React Query result with the favorite status
 */
export const useFavoriteStatus = (mediaId: number, entityType?: string) => {
  return useQuery({
    queryKey: ['favorite-status', entityType ?? null, mediaId],
    queryFn: () => (entityType ? favoritesApi.checkFavorite(mediaId, entityType) : favoritesApi.checkFavorite(mediaId)),
    staleTime: 1000 * 60 * 5, // 5 minutes
    enabled: !!mediaId
  })
}