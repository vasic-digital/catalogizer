import api from './api'
import { asList } from './apiShapes'

export interface RecommendedItem {
  id: number
  title: string
  media_type: string
  year?: number
  rating?: number
  similarity_score?: number
  reason?: string
}

export interface TrendingItem {
  id: number
  title: string
  media_type: string
  year?: number
  access_count: number
  trending_score: number
}

export interface PersonalizedRecommendation {
  id: number
  title: string
  media_type: string
  year?: number
  rating?: number
  recommendation_reason: string
}

interface LocalSimilar {
  media_id?: string | number
  media_metadata?: { id?: number; title?: string; media_type?: string; year?: number; rating?: number } | null
  similarity_score?: number
  similarity_reasons?: string[] | null
}

/** catalog-api answers {local_items, external_items, ...}; pages consume flat RecommendedItem rows (local, owned items only: they are navigable). */
function toRecommendedItem(raw: LocalSimilar): RecommendedItem | null {
  const meta = raw.media_metadata
  const id = Number(raw.media_id ?? meta?.id)
  if (!Number.isFinite(id) || !meta) return null
  const reasons = Array.isArray(raw.similarity_reasons) ? raw.similarity_reasons : []
  return {
    id,
    title: meta.title ?? '',
    media_type: meta.media_type ?? '',
    ...(meta.year != null ? { year: meta.year } : {}),
    ...(meta.rating != null ? { rating: meta.rating } : {}),
    ...(raw.similarity_score != null ? { similarity_score: raw.similarity_score } : {}),
    ...(reasons.length ? { reason: reasons.join(', ') } : {}),
  }
}

export const recommendationsApi = {
  getSimilar: (mediaId: number): Promise<RecommendedItem[]> =>
    api.get(`/recommendations/similar/${mediaId}`).then((res) =>
      asList<LocalSimilar>(res.data, 'local_items')
        .map(toRecommendedItem)
        .filter((x): x is RecommendedItem => x !== null),
    ),

  getTrending: (limit = 10): Promise<TrendingItem[]> =>
    api.get('/recommendations/trending', { params: { limit } }).then((res) => asList<TrendingItem>(res.data, 'items')),

  getPersonalized: (userId: number, limit = 10): Promise<PersonalizedRecommendation[]> =>
    api.get(`/recommendations/personalized/${userId}`, { params: { limit } }).then((res) => asList<PersonalizedRecommendation>(res.data, 'items')),
}
