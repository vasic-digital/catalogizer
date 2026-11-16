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

  // SSEF4R-FD-m1-FU1: golden cases that pin per-occurrence, offset-based reporting.
  // Each kills a reviewer mutant of findUncapturedOccurrences: M5 (count-based
  // subtraction), M3 (per-line subtraction), M6 (skip lines containing //).
  const good = (p: string, perm: string) =>
    `<Route path="${p}" element={<ProtectedRoute requiredPermission="${perm}"><X /></ProtectedRoute>} />`
  const bad = (p: string, perm: string) =>
    `<Route path='${p}' element={<ProtectedRoute requiredPermission='${perm}'><X /></ProtectedRoute>} />`

  it('bad gate BEFORE a good one: the bad one (line 1) is reported, not the good one', () => {
    const src = `${bad('/b', 'media.edit')}\n${good('/a', 'media.view')}`
    const missed = findUncapturedOccurrences(src)
    expect(missed).toHaveLength(1)
    expect(missed[0].line).toBe(1)
    expect(missed[0].text).toContain("requiredPermission='media.edit'")
    expect(extractGates(src)).toEqual([['/a', 'media.view']])
  })

  it('good and bad gate on the SAME line: the bad one is still reported', () => {
    const src = `${good('/a', 'media.view')} ${bad('/b', 'media.edit')}`
    const missed = findUncapturedOccurrences(src)
    expect(missed).toHaveLength(1)
    expect(missed[0].line).toBe(1)
    expect(extractGates(src)).toEqual([['/a', 'media.view']])
    expect(() => assertAllGatesCaptured(src)).toThrow(/missed 1 requiredPermission.*line 1/)
  })

  it('an occurrence on a // comment line is reported (not skipped)', () => {
    const src = `// TODO gate this with requiredPermission later\n${good('/a', 'media.view')}`
    const missed = findUncapturedOccurrences(src)
    expect(missed).toHaveLength(1)
    expect(missed[0].line).toBe(1)
    expect(missed[0].text).toContain('// TODO')
    expect(() => assertAllGatesCaptured(src)).toThrow(/line 1/)
  })

  // WTB1-m1: two uncaptured gates on lines 2 and 3 pin the per-occurrence text and the reported count
  // (kills a mutant that takes the text from the first line, and one that reports only the first occurrence).
  it('two bad gates on lines 2 and 3: both reported with their own text and the count is 2', () => {
    const src = `${good('/a', 'media.view')}\n${bad('/b', 'media.edit')}\n${bad('/c', 'media.delete')}`
    const missed = findUncapturedOccurrences(src)
    expect(missed).toHaveLength(2)
    expect(missed.map((m) => m.line)).toEqual([2, 3])
    expect(missed[0].text).toContain("requiredPermission='media.edit'")
    expect(missed[1].text).toContain("requiredPermission='media.delete'")
    expect(() => assertAllGatesCaptured(src)).toThrow(/missed 2 requiredPermission/)
  })
})

// F2F3-m3: the Header NAV_ITEMS gate must equal the App.tsx route gate, read from the real files.
describe('Header NAV_ITEMS gates equal App.tsx route gates (F2F3-m3)', () => {
  const header = readFileSync(resolve(repo, 'catalog-web/src/components/layout/Header.tsx'), 'utf8')
  const navGates = [...header.matchAll(/\{\s*to:\s*'([^']+)'[^}]*requiredPermission:\s*'([^']+)'[^}]*\}/g)].map(
    (m) => [m[1], m[2]] as [string, string]
  )
  it('finds the gated nav items (control needle)', () => {
    expect([...header.matchAll(/requiredPermission:/g)].length).toBe(navGates.length)
    expect(Object.fromEntries(navGates)).toHaveProperty('/conversion', 'conversion.view')
  })
  it.each(navGates)('nav item %s gate (%s) equals its App.tsx route gate', (to, perm) => {
    expect(Object.fromEntries(gates)[to]).toBe(perm)
  })
})
