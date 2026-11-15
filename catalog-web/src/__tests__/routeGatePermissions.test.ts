import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { assertAllGatesCaptured, extractGates, findUncapturedOccurrences } from './routeGateExtract'

// SSEF4R-FD / review F1: every route gate string in the REAL App.tsx must be a
// permission constant the server defines (catalog-api/models/user.go), and each
// route keeps its recorded gate. Both files are read from the repository at test
// time, nothing is copied, so a revert or typo of any gate fails here.
const repo = resolve(__dirname, '../../..')
const app = readFileSync(resolve(repo, 'catalog-web/src/App.tsx'), 'utf8')
const goModels = readFileSync(resolve(repo, 'catalog-api/models/user.go'), 'utf8')

const serverPermissions = new Set(
  [...goModels.matchAll(/\bPermission\w+\s*=\s*"([^"]+)"/g)].map((m) => m[1])
)

const gates = extractGates(app)

// F3 (SSEF4R-FD-F3): /conversion is gated conversion.view, the minimum the page loads with.
// F2 (SSEF4R-FD-F2): /identities is admin-only (requireAdmin), asserted below.
const expected: Record<string, string> = {
  '/media': 'media.view',
  '/analytics': 'analytics.view',
  '/subtitles': 'media.edit',
  '/collections': 'media.view',
  '/conversion': 'conversion.view',
  '/browse': 'media.view',
  '/entity/:id': 'media.view',
}

describe('App.tsx route gates use the server permission vocabulary', () => {
  it('finds all seven permission-gated routes (control needle)', () => {
    expect(serverPermissions.has('media.view')).toBe(true)
    expect(serverPermissions.has('read:media')).toBe(false)
    expect(Object.fromEntries(gates)).toEqual(expected)
  })
  it.each(Object.entries(expected))('route %s is gated by %s', (path, perm) => {
    expect(Object.fromEntries(gates)[path]).toBe(perm)
  })
  it.each(gates)('gate of %s (%s) is a constant defined in catalog-api/models/user.go', (_path, perm) => {
    expect(serverPermissions.has(perm)).toBe(true)
  })
})

describe('/identities is admin-only (SSEF4R-FD-F2)', () => {
  it('route uses requireAdmin and no requiredPermission', () => {
    const i = app.indexOf('path="/identities"')
    expect(i).toBeGreaterThan(-1)
    const block = app.slice(i, app.indexOf('</Route>', i) === -1 ? i + 400 : i + 400)
    expect(block).toMatch(/<ProtectedRoute requireAdmin>/)
    expect(block.split('IdentityManagerPage')[0]).not.toMatch(/requiredPermission/)
  })
})

// SSEF4R-FD-m1: completeness. Every requiredPermission occurrence in the real
// App.tsx must have been captured by the extractor; a gated route written in any
// other syntax would otherwise be silently skipped.
describe('route-gate extractor completeness (SSEF4R-FD-m1)', () => {
  it('real App.tsx: every requiredPermission occurrence is captured (control needle)', () => {
    const occurrences = [...app.matchAll(/requiredPermission/g)].length
    expect(occurrences).toBe(7) // control: the instrument can see the real tree
    expect(gates.length).toBe(occurrences)
    expect(findUncapturedOccurrences(app)).toEqual([])
    expect(() => assertAllGatesCaptured(app)).not.toThrow()
  })

  const goldenBad: Array<[string, string]> = [
    ['single quotes', `<Route path='/x' element={<ProtectedRoute requiredPermission='media.view'><X /></ProtectedRoute>} />`],
    ['multi-line', `<Route path="/x" element={\n<ProtectedRoute\n requiredPermission="media.view">\n<X /></ProtectedRoute>} />`],
    ['gate-last', `<Route element={<ProtectedRoute requiredPermission="media.view"><X /></ProtectedRoute>} path="/x" />`],
  ]
  it.each(goldenBad)('golden-bad (%s): extractor misses it and the check fires', (_n, src) => {
    expect(extractGates(src)).toEqual([])
    expect(findUncapturedOccurrences(src)).toHaveLength(1)
    expect(() => assertAllGatesCaptured(src)).toThrow(/missed 1 requiredPermission.*line \d+/)
  })

  it('golden-bad mixed with a good gate: only the uncaptured one is reported', () => {
    const good = `<Route path="/a" element={<ProtectedRoute requiredPermission="media.view"><A /></ProtectedRoute>} />`
    const bad = `<Route path='/b' element={<ProtectedRoute requiredPermission='media.edit'><B /></ProtectedRoute>} />`
    const missed = findUncapturedOccurrences(`${good}\n${bad}`)
    expect(missed).toHaveLength(1)
    expect(missed[0].line).toBe(2)
    expect(extractGates(`${good}\n${bad}`)).toEqual([['/a', 'media.view']])
  })
})
