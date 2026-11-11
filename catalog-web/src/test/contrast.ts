// Real WCAG contrast computed from the REAL Tailwind theme (tailwind.config.js, via resolveConfig),
// the class names actually rendered by a component, and the nearest ancestor background.
import resolveConfig from 'tailwindcss/resolveConfig'
// @ts-expect-error tailwind.config.js is plain JS
import tailwindConfig from '../../tailwind.config.js'

export type Theme = 'light' | 'dark'

const theme = (resolveConfig(tailwindConfig as never) as unknown as { theme: { colors: Record<string, unknown> } }).theme

function flatten(colors: Record<string, unknown>, prefix = '', out: Record<string, string> = {}) {
  for (const [k, v] of Object.entries(colors)) {
    const name = k === 'DEFAULT' ? prefix : prefix ? `${prefix}-${k}` : k
    if (typeof v === 'string') out[name] = v
    else if (v && typeof v === 'object') flatten(v as Record<string, unknown>, name, out)
  }
  return out
}
const PALETTE = flatten(theme.colors)

type RGB = [number, number, number]
function hexToRgb(hex: string): RGB | null {
  const full = /^#([0-9a-f]{3})$/i.test(hex) ? '#' + hex.slice(1).replace(/./g, (c) => c + c) : hex
  const m = /^#([0-9a-f]{6})$/i.exec(full)
  if (!m) return null
  const n = parseInt(m[1], 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}
function lin(c: number) {
  const s = c / 255
  return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4)
}
export function luminance([r, g, b]: RGB) {
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}
export function ratio(a: RGB, b: RGB) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

interface Resolved { rgb: RGB; alpha: number }
function resolveToken(tok: string): Resolved | null {
  const [name, op] = tok.split('/')
  const hex = PALETTE[name]
  const rgb = hex ? hexToRgb(hex) : null
  if (!rgb) return null
  return { rgb, alpha: op ? Number(op) / 100 : 1 }
}

const NON_COLOR_TEXT = /^(xs|sm|base|lg|[2-9]?xl|center|left|right|justify|start|end|ellipsis|clip|wrap|nowrap|balance|pretty|opacity-\d+|current|inherit)$/
const NON_COLOR_BG = /^(transparent|current|inherit|none|cover|contain|center|top|bottom|left|right|repeat.*|no-repeat|fixed|local|scroll|clip-.*|origin-.*|blend-.*|opacity-\d+|gradient-.*)$/

/** Resolve one class for one prefix: null when irrelevant; throws when it is a colour class we cannot resolve. */
function classColor(c: string, pfx: string, prefix: 'text' | 'bg'): Resolved | null {
  if (!c.startsWith(pfx)) return null
  const tok = c.slice(pfx.length)
  if (!tok || (prefix === 'text' ? NON_COLOR_TEXT : NON_COLOR_BG).test(tok)) return null
  const r = resolveToken(tok)
  if (r) return r
  // A colour class we cannot resolve to a hex (hsl(var(--x)) token, arbitrary value) must never silently
  // fall through to an ancestor: that would hide a real contrast problem.
  const name = tok.split('/')[0]
  if (name.startsWith('[') || name in PALETTE) {
    throw new Error(`contrast helper cannot resolve colour class "${c}"`)
  }
  return null
}

function findColor(classes: string[], pfx: string, prefix: 'text' | 'bg'): Resolved | null {
  // D7-R2-F3: fail closed. Two different colour classes of the same prefix+variant on one element
  // (e.g. "text-gray-500 text-gray-900") make the winner depend on CSS source order, which a class
  // list cannot tell us; silently picking the first would hide a real contrast problem.
  let found: { cls: string; res: Resolved } | null = null
  for (const c of classes) {
    const r = classColor(c, pfx, prefix)
    if (!r) continue
    if (found && found.cls !== c) {
      throw new Error(`contrast helper: conflicting colour classes "${found.cls}" and "${c}" on one element`)
    }
    found = found ?? { cls: c, res: r }
  }
  return found ? found.res : null
}

function pick(el: Element, prefix: 'text' | 'bg', t: Theme): Resolved | null {
  const classes = Array.from(el.classList)
  return (t === 'dark' ? findColor(classes, `dark:${prefix}-`, prefix) : null) ?? findColor(classes, `${prefix}-`, prefix)
}

function blend(fg: RGB, alpha: number, under: RGB): RGB {
  return fg.map((c, i) => Math.round(c * alpha + under[i] * (1 - alpha))) as RGB
}

/** Effective background colour of el: nearest ancestor-or-self with a bg-* colour class (alpha blended over what is behind it), else the page default. */
export function backgroundOf(el: Element | null, t: Theme): RGB {
  for (let n: Element | null = el; n; n = n.parentElement) {
    const r = pick(n, 'bg', t)
    if (r) return r.alpha === 1 ? r.rgb : blend(r.rgb, r.alpha, backgroundOf(n.parentElement, t))
  }
  return t === 'dark' ? resolveToken('gray-900')!.rgb : resolveToken('white')!.rgb
}

/** Effective text colour (alpha blended on the background) of el: nearest ancestor-or-self with a text-* colour class. */
export function foregroundOf(el: Element, t: Theme): RGB {
  const bg = backgroundOf(el, t)
  for (let n: Element | null = el; n; n = n.parentElement) {
    const r = pick(n, 'text', t)
    if (r) return blend(r.rgb, r.alpha, bg)
  }
  return t === 'dark' ? resolveToken('white')!.rgb : resolveToken('gray-900')!.rgb
}

export function contrastOf(el: Element, t: Theme): number {
  return ratio(foregroundOf(el, t), backgroundOf(el, t))
}

export const WCAG_AA_NORMAL = 4.5

function isExemptTextElement(n: Element): boolean {
  if (n.closest('svg')) return true
  // WCAG 1.4.3: text of inactive (disabled) user-interface components is exempt from the contrast requirement.
  if (n.closest('[disabled]')) return true
  const own = Array.from(n.childNodes).some((c) => c.nodeType === 3 && (c.textContent ?? '').trim() !== '')
  if (!own) return true
  const cls = n.getAttribute('class') ?? ''
  return /(^|\s)bg-black\//.test(cls) || (/(^|\s)bg-black(\s|$)/.test(cls) && /(^|\s)bg-opacity-\d+(\s|$)/.test(cls))
}

function isLargeText(cls: string): boolean {
  return /\btext-(2xl|3xl|4xl|5xl)\b/.test(cls) || (/\btext-xl\b/.test(cls) && /\bfont-bold\b/.test(cls))
}

/**
 * Whole-element walker (D7-R2-F2): every element that directly holds visible text, with its WCAG ratio;
 * returns descriptions of those below the AA threshold (4.5 normal, 3 for large text).
 * Text over a translucent black overlay sits on user imagery and is not computable from classes: skipped.
 */
export function failingElements(root: Element, theme: Theme, min = WCAG_AA_NORMAL, opts: { islands?: boolean } = {}): string[] {
  const bad: string[] = []
  const w = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT)
  for (let n = w.currentNode as Element | null; n; n = w.nextNode() as Element | null) {
    if (isExemptTextElement(n)) continue
    const cls = n.getAttribute('class') ?? ''
    const r = contrastOf(n, theme)
    if (r < (isLargeText(cls) ? 3 : min)) bad.push(`${n.tagName.toLowerCase()}.${cls} "${(n.textContent ?? '').trim().slice(0, 30)}" ${r.toFixed(2)}`)
  }
  // D7-R4-4: opt-in (existing suites predate it and contain known islands, listed in FOLLOW_UPS): also report light islands.
  if (opts.islands) bad.push(...islandsOf(root, theme))
  return bad
}

// D7-R4-FU-2: 50, 100 and 200 shades count as light backgrounds; 300 and darker are deliberately out of scope.
const LIGHT_BG_NAME = /^(white|[a-z]+-(50|100|200))$/

/** True when the element carries a dark:bg-* class that is a real dark background. */
function hasRealDarkBg(classes: string[]): boolean {
  // D7-R4-FU-2: a dark:bg-* class only counts when it is a real dark background: a dark:bg-* that is itself
  // a light colour (dark:bg-white) or a >=50% alpha modifier (dark:bg-opacity-*) leaves the island bright in dark.
  return classes.some((c) => {
    if (!c.startsWith('dark:bg-')) return false
    const tok = c.slice('dark:bg-'.length)
    // Tailwind 3: dark:bg-opacity-N is the alpha of the bg colour in dark: below 50% the island is faded away, from 50% up it stays bright.
    const om = /^opacity-(\d+)$/.exec(tok)
    if (om) return Number(om[1]) < 50
    const [name, op] = tok.split('/')
    const r = resolveToken(name)
    return !(r && LIGHT_BG_NAME.test(name) && (!op || Number(op) >= 50))
  })
}

/** D7-R4-FU-1: a toggle knob is a small (w-1..6 AND h-1..6), EMPTY, plain bg-white circle on a coloured track. */
function isToggleKnob(n: Element, classes: string[]): boolean {
  return (
    classes.includes('rounded-full') && classes.includes('bg-white') &&
    classes.some((c) => /^w-[1-6]$/.test(c)) && classes.some((c) => /^h-[1-6]$/.test(c)) &&
    (n.textContent ?? '').trim() === ''
  )
}

/** The first bg-* class that makes the element a light island, if any. */
function lightIslandClass(classes: string[]): string | null {
  for (const c of classes) {
    if (!c.startsWith('bg-')) continue
    const [name, op] = c.slice(3).split('/')
    if (!LIGHT_BG_NAME.test(name)) continue
    if (op && Number(op) < 50) continue
    if (resolveToken(name) && luminance(resolveToken(name)!.rgb) > 0.5) return c
  }
  return null
}

/**
 * D7-R4-4 light-island detector: in dark mode, an element whose OWN bg-* class is a light background
 * (bg-white, bg-*-50, bg-*-100; opaque enough to matter, alpha >= 50%) and that has NO dark:bg-* class.
 * backgroundOf() falls back to that light base colour in dark, so such an island passes the text-contrast
 * check against itself while sitting as a bright box on the dark page. Light theme never has islands.
 */
export function islandsOf(root: Element, theme: Theme): string[] {
  if (theme !== 'dark') return []
  const found: string[] = []
  const all = [root, ...Array.from(root.querySelectorAll('*'))]
  for (const n of all) {
    if (n.closest('svg')) continue
    const classes = Array.from(n.classList)
    if (hasRealDarkBg(classes)) continue
    // Anything that is not a plain toggle knob (other colour, one size class, any text) is a real surface.
    if (isToggleKnob(n, classes)) continue
    const c = lightIslandClass(classes)
    if (c) found.push(`${n.tagName.toLowerCase()}.${n.getAttribute('class')} (light island: ${c})`)
  }
  return found
}
