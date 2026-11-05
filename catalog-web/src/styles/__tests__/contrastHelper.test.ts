// Golden fixtures for the contrast helper itself (an unvalidated oracle is a bluff).
import { backgroundOf, foregroundOf, contrastOf, ratio } from '@/test/contrast'

function tree(parentCls: string, childCls: string) {
  const p = document.createElement('div'); p.className = parentCls
  const c = document.createElement('span'); c.className = childCls
  p.appendChild(c)
  return c
}

describe('contrast helper goldens', () => {
  it('dark: variant is used in dark and ignored in light', () => {
    const c = tree('bg-white dark:bg-gray-900', 'text-gray-900 dark:text-gray-100')
    expect(foregroundOf(c, 'light')).toEqual([17, 24, 39]) // gray-900
    expect(foregroundOf(c, 'dark')).toEqual([243, 244, 246]) // gray-100
    expect(backgroundOf(c, 'dark')).toEqual([17, 24, 39])
  })

  it('only the dark: class differs: base passes, dark fails, and the helper sees it', () => {
    const c = tree('bg-white dark:bg-gray-800', 'text-gray-500 dark:text-gray-700')
    expect(contrastOf(c, 'light')).toBeGreaterThan(4.5)
    expect(contrastOf(c, 'dark')).toBeLessThan(4.5)
  })

  it('text alpha blends over the background', () => {
    const c = tree('bg-white', 'text-black/50')
    expect(foregroundOf(c, 'light')).toEqual([128, 128, 128])
  })

  it('background alpha blends over the next ancestor instead of being skipped', () => {
    const outer = document.createElement('div'); outer.className = 'bg-black'
    const mid = document.createElement('div'); mid.className = 'bg-white/50'
    const c = document.createElement('span')
    outer.appendChild(mid); mid.appendChild(c)
    expect(backgroundOf(c, 'light')).toEqual([128, 128, 128])
  })

  it('font-size / alignment text-* classes are not treated as colours', () => {
    const c = tree('bg-white', 'text-xs text-center text-gray-900')
    expect(foregroundOf(c, 'light')).toEqual([17, 24, 39])
  })

  it('unresolvable colour classes throw instead of silently falling to an ancestor', () => {
    expect(() => foregroundOf(tree('bg-white', 'text-foreground'), 'light')).toThrow(/cannot resolve/)
    expect(() => foregroundOf(tree('bg-white', 'text-[#999]'), 'light')).toThrow(/cannot resolve/)
    expect(() => backgroundOf(tree('bg-background', 'x'), 'light')).toThrow(/cannot resolve/)
  })

  it('ratio is symmetric and 1:1 for identical colours', () => {
    expect(ratio([10, 20, 30], [10, 20, 30])).toBeCloseTo(1, 5)
    expect(ratio([0, 0, 0], [255, 255, 255])).toBeCloseTo(ratio([255, 255, 255], [0, 0, 0]), 8)
  })
})
