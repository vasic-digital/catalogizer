import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { favoritesApi } from '@/lib/favoritesApi'
import type { Favorite, FavoriteToggleRequest } from '@/types/favorites'
import toast from 'react-hot-toast'

type FavoritesList = { items?: Favorite[]; total?: number }
type RemovedRows = { key: readonly unknown[]; rows: { fav: Favorite; index: number }[] }

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
      
      // FAVR3-1: the live list key is ['favorites', params]; act on every cached list by prefix.
      // FAVR3-8: record, per cached key, exactly the rows THIS request removed (with their index) so a
      // failure re-inserts only those rows; a whole-list snapshot restore would resurrect rows removed
      // by another in-flight toggle.
      const removed: RemovedRows[] = []
      if (!request.is_favorite) {
        // Remove from favorites. (Adding leaves the cache as it is: the full media item is not available
        // here; CAT-001 (R5): the success toast is shown in onSuccess, after the server accepted the add.)
        for (const [key, old] of queryClient.getQueriesData<FavoritesList>(['favorites'])) {
          if (!old?.items) continue
          const rows: RemovedRows['rows'] = []
          const items = old.items.filter((fav: Favorite, index: number) => {
            const match = fav.media_id === request.media_id &&
              (!request.entity_type || fav.media_item.media_type === request.entity_type)
            if (match) rows.push({ fav, index })
            return !match
          })
          if (rows.length === 0) continue
          removed.push({ key, rows })
          queryClient.setQueryData<FavoritesList>(key, {
            ...old,
            items,
            // FAVR3-9: keep the cached total in step with the removed rows
            ...(typeof old.total === 'number' && { total: Math.max(0, old.total - rows.length) })
          })
        }
      }

      return { removed }
    },
    onSuccess: (_data, request) => {
      if (request.is_favorite) toast.success('Added to favorites')
    },
    onError: (error, variables, context) => {
      // Revert optimistic update: re-insert only the rows this request removed, per key, at their
      // original index; rows another in-flight toggle removed stay removed (FAVR3-8).
      context?.removed?.forEach(({ key, rows }) => {
        queryClient.setQueryData<FavoritesList>(key, (current) => {
          if (!current?.items) return current
          const items = [...current.items]
          let restored = 0
          for (const { fav, index } of rows) {
            if (items.some((f) => f.id === fav.id)) continue
            items.splice(Math.min(index, items.length), 0, fav)
            restored++
          }
          if (restored === 0) return current
          return {
            ...current,
            items,
            ...(typeof current.total === 'number' && { total: current.total + restored })
          }
        })
      })
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