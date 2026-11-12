export interface Favorite {
  id: string
  user_id: number
  media_id: number
  media_item: {
    id: number
    title: string
    media_type: string
    year?: number
    cover_image?: string
    duration?: number
    rating?: number
    quality?: string
  }
  created_at: string
  updated_at: string
}

export interface FavoriteToggleRequest {
  media_id: number
  is_favorite: boolean
  /** Closed-set entity type of media_id; defaults to the legacy 'movie' only when omitted. */
  entity_type?: string
}

export interface FavoritesResponse {
  items: Favorite[]
  total: number
  limit: number
  offset: number
}

export interface FavoriteStats {
  total_count: number
  media_type_breakdown: {
    movie: number
    tv_show: number
    music: number
    game: number
    documentary: number
    anime: number
    concert: number
    other: number
  }
  recent_additions: Favorite[]
}