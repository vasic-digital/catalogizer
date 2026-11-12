/**
 * Single builder for the entity detail route (App.tsx: `/entity/:id`).
 * Every entity type in the closed set (movie, tv_show, ... comic) is served
 * by that one route, so the id is the only variable part. Returns null for an
 * unusable id (0, negative, NaN, non-integer) so callers never navigate to a
 * bogus entity such as `/entity/0` or `/entity/NaN` (CAT-UI-FAV-1).
 */
export function entityPath(id: number, opts?: { autoplay?: boolean }): string | null {
  if (!Number.isInteger(id) || id <= 0) return null
  return opts?.autoplay ? `/entity/${id}?autoplay=true` : `/entity/${id}`
}
