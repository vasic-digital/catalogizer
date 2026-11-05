// Golden fixtures for the contrast helper itself (an unvalidated oracle is a bluff).
import { backgroundOf, foregroundOf, contrastOf, ratio, failingElements } from '@/test/contrast'

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

  it('conflicting same-prefix colour classes on one element throw (D7-R2-F3)', () => {
    expect(() => foregroundOf(tree('bg-white', 'text-gray-500 text-gray-900'), 'light')).toThrow(/conflicting/)
    expect(() => foregroundOf(tree('bg-white', 'text-gray-500 dark:text-gray-300 dark:text-gray-400'), 'dark')).toThrow(/conflicting/)
    expect(() => backgroundOf(tree('bg-white bg-gray-800', 'x'), 'light')).toThrow(/conflicting/)
    // same class twice, or one base + one dark variant, is NOT a conflict
    expect(foregroundOf(tree('bg-white', 'text-gray-900 text-gray-900'), 'light')).toEqual([17, 24, 39])
    expect(foregroundOf(tree('bg-white', 'text-gray-500 dark:text-gray-300'), 'light')).toEqual([107, 114, 128])
  })

  it('bg-transparent / text-current inherit from the ancestor instead of throwing', () => {
    expect(backgroundOf(tree('bg-white', 'bg-transparent'), 'light')).toEqual([255, 255, 255])
    expect(foregroundOf(tree('bg-white text-gray-900', 'text-current'), 'light')).toEqual([17, 24, 39])
  })

  it('ratio is symmetric and 1:1 for identical colours', () => {
    expect(ratio([10, 20, 30], [10, 20, 30])).toBeCloseTo(1, 5)
    expect(ratio([0, 0, 0], [255, 255, 255])).toBeCloseTo(ratio([255, 255, 255], [0, 0, 0]), 8)
  })

  describe('failingElements walker goldens (a walker that sees nothing would pass every component)', () => {
    const html = (inner: string) => { const d = document.createElement('div'); d.className = 'bg-white dark:bg-gray-800'; d.innerHTML = inner; return d }

    it('reports the exact measured defect and passes the fixed class string', () => {
      const bad = html('<p class="text-gray-500">No media data available</p>')
      expect(failingElements(bad, 'light')).toEqual([])
      const dark = failingElements(bad, 'dark')
      expect(dark).toHaveLength(1)
      expect(dark[0]).toMatch(/"No media data available" 3\.04$/)
      expect(failingElements(html('<p class="text-gray-500 dark:text-gray-400">ok</p>'), 'dark')).toEqual([])
    })

    it('a light panel without a dark bg stays light: dark:text-gray-400 on it is flagged (real ancestor, not blind dark class)', () => {
      const d = document.createElement('div'); d.className = 'bg-white'
      d.innerHTML = '<p class="text-gray-500 dark:text-gray-400">x</p>'
      expect(failingElements(d, 'dark')).toHaveLength(1)
      expect(failingElements(d, 'light')).toEqual([])
    })

    it('large text needs 3:1 only; svg content and disabled controls are exempt', () => {
      expect(failingElements(html('<p class="text-3xl text-gray-500">big</p>'), 'dark')).toEqual([])
      expect(failingElements(html('<svg><text class="text-gray-500">s</text></svg>'), 'dark')).toEqual([])
      expect(failingElements(html('<button disabled class="text-gray-500"><span>d</span>t</button>'), 'dark')).toEqual([])
      expect(failingElements(html('<button class="text-gray-500">t</button>'), 'dark')).toHaveLength(1)
    })
  })
})
