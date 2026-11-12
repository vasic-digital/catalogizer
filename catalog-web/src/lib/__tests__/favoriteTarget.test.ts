import { resolveFavoriteTarget } from '../favoriteTarget'

describe('resolveFavoriteTarget', () => {
  it('uses the linked entity, never the row id (a files.id)', () => {
    expect(resolveFavoriteTarget({ id: 5, entity_id: 900, entity_type: 'song' })).toEqual({ id: 900, type: 'song' })
  })
  it('accepts a numeric-string entity id', () => {
    expect(resolveFavoriteTarget({ entity_id: '12', entity_type: 'movie' })).toEqual({ id: 12, type: 'movie' })
  })
  it.each([
    [{ id: 5 }],
    [{ id: 5, entity_id: 900 }],
    [{ id: 5, entity_type: 'song' }],
    [{ id: 5, entity_id: 0, entity_type: 'song' }],
    [{ id: 5, entity_id: '0x10', entity_type: 'song' }],
    [{ id: 5, entity_id: 3, entity_type: '  ' }],
    [{ id: 5, entity_id: 3, entity_type: 7 }],
  ])('returns null for an unlinked row %j', (row) => {
    expect(resolveFavoriteTarget(row)).toBeNull()
  })
})
