import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

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

const gates = [...app.matchAll(/path="([^"]+)"\s*element=\{\s*<ProtectedRoute requiredPermission="([^"]+)"/g)].map(
  (m) => [m[1], m[2]] as const
)

// F2 (/identities) and F3 (/conversion) are registered follow-ups: their current
// values are pinned as-is, not endorsed.
const expected: Record<string, string> = {
  '/media': 'media.view',
  '/analytics': 'analytics.view',
  '/subtitles': 'media.edit',
  '/collections': 'media.view',
  '/conversion': 'media.convert',
  '/browse': 'media.view',
  '/entity/:id': 'media.view',
  '/identities': 'media.view',
}

describe('App.tsx route gates use the server permission vocabulary', () => {
  it('finds all eight gated routes (control needle)', () => {
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
