import { api } from './api'
import type { 
  Favorite, 
  FavoriteToggleRequest, 
  FavoritesResponse, 
  FavoriteStats 
} from '@/types/favorites'

export const favoritesApi = {
  // Get user's favorites
  async getFavorites(params?: {
    limit?: number
    offset?: number
    media_type?: string
    sort_by?: 'created_at' | 'title' | 'rating' | 'year'
    sort_order?: 'asc' | 'desc'
  }): Promise<FavoritesResponse> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.append('limit', params.limit.toString())
    if (params?.offset) searchParams.append('offset', params.offset.toString())
    if (params?.media_type) searchParams.append('media_type', params.media_type)
    if (params?.sort_by) searchParams.append('sort_by', params.sort_by)
    if (params?.sort_order) searchParams.append('sort_order', params.sort_order)

    const response = await api.get(`/favorites?${searchParams}`)
    return response.data
  },

  // Get favorite statistics
  async getFavoriteStats(): Promise<FavoriteStats> {
    const response = await api.get('/favorites/stats')
    return response.data
  },

  // Toggle favorite status
  async toggleFavorite(request: FavoriteToggleRequest): Promise<{ is_favorite: boolean }> {
    const response = await api.post('/favorites/toggle', request)
    return response.data
  },

  // Check if media is in favorites
  async checkFavorite(mediaId: number): Promise<{ is_favorite: boolean }> {
    const response = await api.get(`/favorites/check/${mediaId}`)
    return response.data
  },

  // Add to favorites
  async addToFavorites(mediaId: number): Promise<Favorite> {
    const response = await api.post('/favorites', { media_id: mediaId })
    return response.data
  },

  // Remove from favorites
  async removeFromFavorites(mediaId: number): Promise<void> {
    await api.delete(`/favorites/${mediaId}`)
  },

  // Bulk operations
  async bulkAddToFavorites(mediaIds: number[]): Promise<{ added: number; failed: number }> {
    const response = await api.post('/favorites/bulk', { media_ids: mediaIds })
    return response.data
  },

  async bulkRemoveFromFavorites(mediaIds: number[]): Promise<{ removed: number; failed: number }> {
    const response = await api.delete('/favorites/bulk', { data: { media_ids: mediaIds } })
    return response.data
  }
}