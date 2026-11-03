import { describe, it, expect } from 'vitest'
import css from '../../index.css?raw'

/**
 * Touch-target contract (constitution 11.4.190, WCAG 2.5.8 / Apple HIG 44px).
 * The size is enforced ONCE, centrally, for every interactive element on coarse-pointer devices, instead of
 * per instance. The real proof is the browser audit (tests/responsive-audit); this guards the contract source.
 * jsdom has no layout, so the rule text itself is what can be asserted here.
 */
function coarseBlock(): string {
  const start = css.indexOf('@media (pointer: coarse)')
  expect(start, 'index.css must contain a (pointer: coarse) block').toBeGreaterThan(-1)
  let depth = 0
  for (let i = css.indexOf('{', start); i < css.length; i++) {
    if (css[i] === '{') depth++
    if (css[i] === '}' && --depth === 0) return css.slice(start, i + 1)
  }
  throw new Error('unterminated coarse block')
}

describe('central coarse-pointer touch targets', () => {
  const block = () => coarseBlock()

  it.each(['button', 'select', 'textarea', "[role='tab']", "[role='button']", "[role='menuitem']", 'summary'])(
    'covers %s',
    (sel) => {
      expect(block()).toContain(sel)
    },
  )

  it('covers text-like inputs but not checkbox/radio/range/hidden (their label is the target)', () => {
    const b = block()
    expect(b).toMatch(/input:not\(\[type='checkbox'\]\)/)
    expect(b).toContain("[type='radio']")
    expect(b).toContain("[type='hidden']")
  })

  it('enforces a 44px minimum height and width at zero specificity so explicit utilities can still grow it', () => {
    const b = block()
    expect(b).toMatch(/min-height:\s*44px/)
    expect(b).toMatch(/min-width:\s*44px/)
    expect(b).toContain(':where(')
  })

  it('provides a .touch-target helper for links that are not inline prose', () => {
    expect(css).toMatch(/\.touch-target/)
  })
})
