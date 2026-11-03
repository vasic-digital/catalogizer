import type { SmartCollection } from '../types/collections'

export interface CollectionFilterOptions {
  /** 'smart' | 'manual' narrow by is_smart; any other tab (e.g. 'all') does not filter. */
  tab: string
  /** 'all' or a primary_media_type value. */
  mediaType: string
  search: string
  sortBy: string
}

/** Filter + sort (ascending) the collections list shown on the Collections page. */
export function filterCollections(collections: SmartCollection[], opts: CollectionFilterOptions): SmartCollection[] {
  const q = opts.search.trim().toLowerCase()
  const list = collections.filter((c) => {
    if (opts.tab === 'smart' && !c.is_smart) return false
    if (opts.tab === 'manual' && c.is_smart) return false
    if (opts.mediaType !== 'all' && c.primary_media_type !== opts.mediaType) return false
    if (q && !`${c.name} ${c.description ?? ''}`.toLowerCase().includes(q)) return false
    return true
  })
  const key = opts.sortBy as keyof SmartCollection
  return [...list].sort((a, b) => {
    const av = a[key] as unknown
    const bv = b[key] as unknown
    if (typeof av === 'number' && typeof bv === 'number') return av - bv
    return String(av ?? '').localeCompare(String(bv ?? ''))
  })
}
