import { parseEntityId } from './entityRoutes'

/** A media row as the UI holds it: either a catalog entity ref or a FILE row (files.id) that may carry an entity link. */
export type EntityLinked = { entity_id?: unknown; entity_type?: unknown }

export interface FavoriteTarget {
  id: number
  type: string
}

/**
 * The (entity_type, entity_id) a favourite must be stored under. `id` of a file
 * row (/media/search) is a files.id and is NEVER a valid entity id: only the
 * explicit entity_id + entity_type link counts. Returns null when the row has
 * no linked entity, so callers disable the toggle instead of writing a favourite
 * for the wrong entity (CAT-UI-FAV-1b).
 */
export function resolveFavoriteTarget(item: object): FavoriteTarget | null {
  const linked = item as EntityLinked
  const id = parseEntityId(linked.entity_id)
  const type = typeof linked.entity_type === 'string' ? linked.entity_type.trim() : ''
  if (id === null || type === '') return null
  return { id, type }
}
