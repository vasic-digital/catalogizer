// Golden fixtures for the contrast helper itself (an unvalidated oracle is a bluff).
import { backgroundOf, foregroundOf, contrastOf, ratio, failingElements, islandsOf } from '@/test/contrast'

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

    it('an element with its own bg-* is still measured (badge text-red-600 on bg-red-100 = 3.95)', () => {
      const d = document.createElement('div'); d.className = 'bg-white'
      d.innerHTML = '<span class="bg-red-100 text-red-600">Error</span><span class="bg-red-100 text-red-700">Error</span>'
      const out = failingElements(d, 'light')
      expect(out).toHaveLength(1)
      expect(out[0]).toMatch(/text-red-600.* 3\.95$/)
    })

    it('text-lg is NOT large text (needs 4.5, not 3); text-2xl is', () => {
      const lg = failingElements(html('<p class="text-lg text-gray-500">x</p>'), 'dark')
      expect(lg).toHaveLength(1) // 3.04 < 4.5
      expect(failingElements(html('<p class="text-2xl text-gray-500">x</p>'), 'dark')).toEqual([])
      expect(failingElements(html('<p class="text-xl font-bold text-gray-500">x</p>'), 'dark')).toEqual([])
      expect(failingElements(html('<p class="text-xl text-gray-500">x</p>'), 'dark')).toHaveLength(1)
    })

    it('overlay skip is anchored: only a real bg-black/N or bg-black+bg-opacity-N skips, variant classes do not', () => {
      expect(failingElements(html('<p class="text-gray-500 bg-black/50">x</p>'), 'dark')).toEqual([])
      expect(failingElements(html('<p class="text-gray-500 bg-black bg-opacity-50">x</p>'), 'dark')).toEqual([])
      expect(failingElements(html('<p class="text-gray-500 hover:bg-black/85">x</p>'), 'dark')).toHaveLength(1)
      expect(failingElements(html('<p class="text-gray-500 group-hover:bg-black/85">x</p>'), 'dark')).toHaveLength(1)
    })

    it('large text needs 3:1 only; svg content and disabled controls are exempt', () => {
      expect(failingElements(html('<p class="text-3xl text-gray-500">big</p>'), 'dark')).toEqual([])
      expect(failingElements(html('<svg><text class="text-gray-500">s</text></svg>'), 'dark')).toEqual([])
      expect(failingElements(html('<button disabled class="text-gray-500"><span>d</span>t</button>'), 'dark')).toEqual([])
      expect(failingElements(html('<button class="text-gray-500">t</button>'), 'dark')).toHaveLength(1)
    })
  })
})

describe('D7-R4-4 light-island detector goldens', () => {
  const mk = (cls: string) => { const r = document.createElement('div'); const c = document.createElement('div'); c.className = cls; c.textContent = 'x'; r.appendChild(c); return r }
  it.each(['bg-white', 'bg-gray-50', 'bg-gray-100', 'bg-red-50', 'bg-blue-100'])('positive: %s without dark:bg is an island in dark', (cls) => {
    expect(islandsOf(mk(cls), 'dark')).toHaveLength(1)
  })
  it.each(['bg-white dark:bg-gray-800', 'bg-gray-50 dark:bg-gray-700/50', 'bg-gray-900', 'bg-white/10', 'bg-red-600', 'text-gray-900'])('negative: %s is not an island', (cls) => {
    expect(islandsOf(mk(cls), 'dark')).toHaveLength(0)
  })
  const mkEmpty = (cls: string) => { const r = document.createElement('div'); const c = document.createElement('div'); c.className = cls; r.appendChild(c); return r }
  it('negative: a small, empty, white toggle knob is not a surface', () => {
    expect(islandsOf(mkEmpty('inline-block h-4 w-4 rounded-full bg-white'), 'dark')).toHaveLength(0)
  })
  // D7-R4-FU-1: the knob exemption must not swallow real islands (reviewer mutation M4).
  it.each([
    ['large round chip', 'rounded-full w-16 h-16 bg-gray-100', mkEmpty],
    ['text badge with only a height class', 'rounded-full h-6 px-2 bg-blue-100', mk],
    ['small round NON-white chip', 'rounded-full w-4 h-4 bg-gray-100', mkEmpty],
    ['round white with only one size class', 'rounded-full w-4 bg-white', mkEmpty],
    ['small white circle that holds text', 'rounded-full w-4 h-4 bg-white', mk],
    ['large white round surface (W7)', 'rounded-full w-16 h-16 bg-white', mkEmpty],
    ['white round chip with only 2 of 4 size classes in range (W8)', 'rounded-full w-4 h-16 bg-white', mkEmpty],
  ] as const)('positive: %s is still an island', (_n, cls, make) => {
    expect(islandsOf(make(cls), 'dark')).toHaveLength(1)
  })
  // D7-R4-FU-2: -200 shades, light dark:bg, high dark:bg-opacity-* do not hide an island.
  it.each(['bg-gray-200', 'bg-blue-200', 'bg-white dark:bg-white', 'bg-gray-100 dark:bg-gray-100', 'bg-white dark:bg-opacity-90'])('positive: %s is an island in dark', (cls) => {
    expect(islandsOf(mk(cls), 'dark')).toHaveLength(1)
  })
  // Tailwind 3: bg-opacity-* is a modifier of the bg colour, so `bg-white dark:bg-opacity-10` renders white at 10% in dark = not an island.
  it('negative: bg-white dark:bg-opacity-10 renders as 10% white in dark and is not a bright island', () => {
    expect(islandsOf(mk('bg-white dark:bg-opacity-10'), 'dark')).toHaveLength(0)
  })
  it.each(['bg-gray-300', 'bg-gray-200 dark:bg-gray-700', 'bg-white dark:bg-gray-800 dark:bg-opacity-90'])('negative: %s is not an island', (cls) => {
    expect(islandsOf(mk(cls), 'dark')).toHaveLength(0)
  })
  it('opt-in via failingElements: reported only with islands:true', () => {
    const r = mk('bg-white text-gray-900')
    expect(failingElements(r, 'dark')).toHaveLength(0)
    expect(failingElements(r, 'dark', 4.5, { islands: true })).toHaveLength(1)
  })
  it('light theme never reports islands', () => {
    expect(islandsOf(mk('bg-white'), 'light')).toHaveLength(0)
  })
})

// FU20-3: boundary characterization for the alpha cutoff (exactly 50) and fractional size classes.
describe('FU20-3 island detector boundaries', () => {
  const mk = (cls: string, text = 'x') => { const r = document.createElement('div'); const c = document.createElement('div'); c.className = cls; c.textContent = text; r.appendChild(c); return r }
  it.each([
    ['bg-white/50', 1], ['bg-white/51', 1], ['bg-white/100', 1],
    ['bg-white/49', 0], ['bg-white/0', 0],
  ] as const)('own bg alpha cutoff: %s -> %i island(s) (>=50 counts, <50 does not)', (cls, n) => {
    expect(islandsOf(mk(cls), 'dark')).toHaveLength(n)
  })
  it.each([
    ['bg-white dark:bg-white/50', 1], ['bg-white dark:bg-white/99', 1],
    ['bg-white dark:bg-white/49', 0],
    ['bg-white dark:bg-opacity-50', 1], ['bg-white dark:bg-opacity-49', 0],
  ] as const)('dark:bg alpha cutoff: %s -> %i island(s)', (cls, n) => {
    expect(islandsOf(mk(cls), 'dark')).toHaveLength(n)
  })
  const mkEmpty = (cls: string) => { const r = document.createElement('div'); const c = document.createElement('div'); c.className = cls; r.appendChild(c); return r }
  it('whole-number knob sizes 1 and 6 are exempt, 0 and 7 are not', () => {
    expect(islandsOf(mkEmpty('rounded-full bg-white w-1 h-1'), 'dark')).toHaveLength(0)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-6 h-6'), 'dark')).toHaveLength(0)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-7 h-7'), 'dark')).toHaveLength(1)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-0 h-0'), 'dark')).toHaveLength(1)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-4 h-0'), 'dark')).toHaveLength(1)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-0 h-4'), 'dark')).toHaveLength(1)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-4 h-7'), 'dark')).toHaveLength(1)
  })
  // Current behaviour, pinned: the knob rule only accepts whole numbers 1..6, so a fractional knob (h-3.5 w-3.5,
  // common in Tailwind toggles) is reported as an island. Changing that is a product decision, not a refactor.
  it('fractional knob sizes (w-3.5 h-3.5, w-2.5) are NOT exempt: reported as islands', () => {
    expect(islandsOf(mkEmpty('rounded-full bg-white w-3.5 h-3.5'), 'dark')).toHaveLength(1)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-4 h-2.5'), 'dark')).toHaveLength(1)
    expect(islandsOf(mkEmpty('rounded-full bg-white w-2.5 h-4'), 'dark')).toHaveLength(1)
  })
})
