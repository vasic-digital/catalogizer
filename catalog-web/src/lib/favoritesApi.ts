import { api } from './api'
import { asList } from './apiShapes'
import type { 
  Favorite, 
  FavoriteToggleRequest, 
  FavoritesResponse, 
  FavoriteStats 
} from '@/types/favorites'

/**
 * Wire shape of GET /favorites: `{favorites:[{id,user_id,entity_type,entity_id,
 * created_at,…}], count, limit, offset}`. The backend stores entity references
 * only (no catalog join yet), so `media_item` carries the reference, not a
 * title.
 */
function toFavoritesResponse(
  payload: unknown,
  params?: { limit?: number; offset?: number }
): FavoritesResponse {
  const body = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>
  const rows = asList<Record<string, unknown>>(body, 'favorites')
  const items: Favorite[] = rows.map((r) => {
    const type = typeof r.entity_type === 'string' ? r.entity_type : 'other'
    const entityId = Number(r.entity_id ?? 0)
    return {
      id: String(r.id ?? ''),
      user_id: Number(r.user_id ?? 0),
      media_id: entityId,
      media_item: { id: entityId, title: `${type.replace(/_/g, ' ')} #${entityId}`, media_type: type },
      created_at: typeof r.created_at === 'string' ? r.created_at : '',
      updated_at: typeof r.updated_at === 'string' ? r.updated_at : typeof r.created_at === 'string' ? r.created_at : '',
    }
  })
  return {
    items,
    total: typeof body.count === 'number' ? body.count : items.length,
    limit: typeof body.limit === 'number' ? body.limit : params?.limit ?? items.length,
    offset: typeof body.offset === 'number' ? body.offset : params?.offset ?? 0,
  }
}

const DEFAULT_ENTITY = 'movie'

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
    return toFavoritesResponse(response.data, params)
  },

  // Get favorite statistics
  async getFavoriteStats(): Promise<FavoriteStats> {
    // The backend has no /favorites/stats route (404); derive the statistics
    // from the user's own favorites list instead of requesting a missing route.
    const response = await api.get('/favorites', { params: { limit: 200 } })
    const items = toFavoritesResponse(response.data).items
    const breakdown = { movie: 0, tv_show: 0, music: 0, game: 0, documentary: 0, anime: 0, concert: 0, other: 0 }
    for (const f of items) {
      const t = f.media_item.media_type
      if (t in breakdown) breakdown[t as keyof typeof breakdown] += 1
      else breakdown.other += 1
    }
    return { total_count: items.length, media_type_breakdown: breakdown, recent_additions: items.slice(0, 5) }
  },

  // The UI only knows media ids; the backend keys favorites by
  // (entity_type, entity_id) over a CLOSED entity-type set. UNCONFIRMED: which
  // entity_type a generic catalog media id maps to; 'movie' is used as the
  // default until the media item's type is threaded through (tracked gap).
  // Toggle favorite status
  async toggleFavorite(request: FavoriteToggleRequest): Promise<{ is_favorite: boolean }> {
    // POST /favorites/toggle does not exist on the backend: compose it.
    if (request.is_favorite) {
      await api.post('/favorites', { entity_type: DEFAULT_ENTITY, entity_id: request.media_id })
    } else {
      await api.delete(`/favorites/${DEFAULT_ENTITY}/${request.media_id}`)
    }
    return { is_favorite: request.is_favorite }
  },

  // Check if media is in favorites
  async checkFavorite(mediaId: number): Promise<{ is_favorite: boolean }> {
    const response = await api.get(`/favorites/check/${DEFAULT_ENTITY}/${mediaId}`)
    return { is_favorite: response.data?.is_favorite === true }
  },

  // Add to favorites
  async addToFavorites(mediaId: number): Promise<Favorite> {
    const response = await api.post('/favorites', { entity_type: DEFAULT_ENTITY, entity_id: mediaId })
    return toFavoritesResponse({ favorites: [response.data] }).items[0]
  },

  // Remove from favorites
  async removeFromFavorites(mediaId: number): Promise<void> {
    await api.delete(`/favorites/${DEFAULT_ENTITY}/${mediaId}`)
  },

  // Bulk operations: no /favorites/bulk route exists; fan out per id.
  async bulkAddToFavorites(mediaIds: number[]): Promise<{ added: number; failed: number }> {
    const results = await Promise.allSettled(mediaIds.map((id) => this.addToFavorites(id)))
    const added = results.filter((r) => r.status === 'fulfilled').length
    return { added, failed: results.length - added }
  },

  async bulkRemoveFromFavorites(mediaIds: number[]): Promise<{ removed: number; failed: number }> {
    const results = await Promise.allSettled(mediaIds.map((id) => this.removeFromFavorites(id)))
    const removed = results.filter((r) => r.status === 'fulfilled').length
    return { removed, failed: results.length - removed }
  }
}
