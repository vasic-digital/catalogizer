import { filterCollections } from '../collectionFilter'
import type { SmartCollection } from '../../types/collections'

const mk = (o: Partial<SmartCollection>): SmartCollection =>
  ({ id: 'x', name: 'n', is_smart: false, primary_media_type: 'mixed', item_count: 0, ...o }) as SmartCollection

const data = [
  mk({ id: '1', name: 'Zeta Smart', is_smart: true, primary_media_type: 'video', item_count: 3 }),
  mk({ id: '2', name: 'Alpha Manual', is_smart: false, primary_media_type: 'music', item_count: 10, description: 'Weekend mix' }),
  mk({ id: '3', name: 'Beta Manual', is_smart: false, primary_media_type: 'video', item_count: 1 }),
]
const base = { tab: 'all', mediaType: 'all', search: '', sortBy: 'name' }
const ids = (l: SmartCollection[]) => l.map((c) => c.id)

describe('filterCollections', () => {
  it('all tab keeps everything, sorted ascending by name', () => {
    expect(ids(filterCollections(data, base))).toEqual(['2', '3', '1'])
  })
  it('smart tab keeps only smart collections', () => {
    expect(ids(filterCollections(data, { ...base, tab: 'smart' }))).toEqual(['1'])
  })
  it('manual tab keeps only non-smart collections', () => {
    expect(ids(filterCollections(data, { ...base, tab: 'manual' }))).toEqual(['2', '3'])
  })
  it('media type narrows by primary_media_type', () => {
    expect(ids(filterCollections(data, { ...base, mediaType: 'video' }))).toEqual(['3', '1'])
  })
  it('search matches name or description, case-insensitively, trimmed', () => {
    expect(ids(filterCollections(data, { ...base, search: '  WEEKEND ' }))).toEqual(['2'])
    expect(ids(filterCollections(data, { ...base, search: 'beta' }))).toEqual(['3'])
    expect(filterCollections(data, { ...base, search: 'nomatch' })).toEqual([])
  })
  it('filters combine', () => {
    expect(ids(filterCollections(data, { ...base, tab: 'manual', mediaType: 'video' }))).toEqual(['3'])
  })
  it('numeric sort is numeric, not lexicographic', () => {
    expect(ids(filterCollections(data, { ...base, sortBy: 'item_count' }))).toEqual(['3', '1', '2'])
  })
  it('does not mutate its input', () => {
    const copy = [...data]
    filterCollections(data, base)
    expect(data).toEqual(copy)
  })
})
