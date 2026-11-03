import { asList, decodeReportEnvelope } from '../apiShapes'

describe('apiShapes.asList', () => {
  it('accepts a bare array', () => expect(asList([1, 2])).toEqual([1, 2]))
  it('unwraps named keys, items and data', () => {
    expect(asList({ scans: [1] }, 'scans')).toEqual([1])
    expect(asList({ items: [2], total: 1 })).toEqual([2])
    expect(asList({ data: [3], success: true })).toEqual([3])
  })
  it('turns null / missing / wrong types into []', () => {
    for (const v of [null, undefined, {}, { data: null }, { items: 'x' }, 7, 'x']) expect(asList(v, 'scans')).toEqual([])
  })
})

describe('apiShapes.decodeReportEnvelope', () => {
  it('decodes a base64 JSON content field', () => {
    const content = Buffer.from(JSON.stringify({ a: 1 })).toString('base64')
    expect(decodeReportEnvelope({ content })).toEqual({ a: 1 })
  })
  it('returns null for malformed envelopes instead of throwing', () => {
    expect(decodeReportEnvelope({ content: '!!!' })).toBeNull()
    expect(decodeReportEnvelope({})).toBeNull()
    expect(decodeReportEnvelope(null as never)).toBeNull()
  })
})
