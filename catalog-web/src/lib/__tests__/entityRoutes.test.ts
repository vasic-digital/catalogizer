import { entityPath, parseEntityId } from '../entityRoutes'

describe('entityPath', () => {
  it('builds view and autoplay routes', () => {
    expect(entityPath(7)).toBe('/entity/7')
    expect(entityPath('7')).toBe('/entity/7')
    expect(entityPath(7, { autoplay: true })).toBe('/entity/7?autoplay=true')
    expect(entityPath(Number.MAX_SAFE_INTEGER)).toBe(`/entity/${Number.MAX_SAFE_INTEGER}`)
  })
  it.each([0, -1, Number.NaN, Infinity, 1.5, 1e21, 2 ** 53, Number.MAX_SAFE_INTEGER + 2])('rejects number %s', (v) => {
    expect(entityPath(v)).toBeNull()
  })
  it.each(['', '0', '007', '0x10', '1e3', '1e+21', '-3', '1.5', ' 5', '5 ', '9007199254740993', 'abc'])('rejects string %j', (v) => {
    expect(entityPath(v)).toBeNull()
  })
  it('rejects non-number/string at runtime', () => {
    expect(entityPath(true as any)).toBeNull()
    expect(entityPath([5] as any)).toBeNull()
  })
})

describe('parseEntityId', () => {
  it('accepts canonical positive safe integers', () => {
    expect(parseEntityId(42)).toBe(42)
    expect(parseEntityId('42')).toBe(42)
  })
  it.each([undefined, null, 0, '0', true, [5], '0x10', '1e3', 1.5, 1e21, '9007199254740993', {}])('rejects %j', (v) => {
    expect(parseEntityId(v)).toBeNull()
  })
})
