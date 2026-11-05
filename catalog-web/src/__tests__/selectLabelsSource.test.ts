import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

// Every <Select> call site must be named at the source: aria-label, aria-labelledby, or an id
// paired with <label htmlFor> in the same file, or be wrapped by a <label> (CAT-UI-D5D6).
// LIMIT: this is a static scan. It cannot detect an id that is repeated at run time because the
// call site sits inside a loop (rules.map ...); such cases need a rendering test with 2+ items
// (see SmartPlaylistBuilder.test.tsx 'per-rule control ids').
function walk(dir: string, out: string[] = []): string[] {
  for (const e of readdirSync(dir)) {
    const p = join(dir, e)
    if (statSync(p).isDirectory()) { if (e !== '__tests__' && e !== 'node_modules') walk(p, out) }
    else if (p.endsWith('.tsx')) out.push(p)
  }
  return out
}

describe('every <Select> call site is named', () => {
  const files = walk(join(__dirname, '..')).filter(f => !f.endsWith(join('ui', 'Select.tsx')))
  const offenders: string[] = []
  let seen = 0
  for (const f of files) {
    const src = readFileSync(f, 'utf8')
    const re = /<Select(?=[\s>])([\s\S]*?)(?<!=)>/g
    let m: RegExpExecArray | null
    while ((m = re.exec(src))) {
      seen++
      const tag = m[1]
      const before = src.slice(Math.max(0, m.index - 400), m.index)
      const wrapped = /<label(?![^>]*htmlFor)[^>]*>(?:(?!<\/label>)[\s\S])*$/.test(before)
      const id = /\sid="([^"]+)"/.exec(tag)?.[1]
      const ok = /aria-label=|aria-labelledby=/.test(tag) || wrapped || (id && src.includes(`htmlFor="${id}"`))
      if (!ok) offenders.push(`${f.split('src/')[1]}:${src.slice(0, m.index).split('\n').length}`)
    }
  }
  it('found call sites (control needle)', () => { expect(seen).toBeGreaterThan(30) })
  it('has no unnamed call site', () => { expect(offenders).toEqual([]) })
})
