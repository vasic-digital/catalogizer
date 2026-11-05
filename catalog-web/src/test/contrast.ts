// Real WCAG contrast computed from the REAL Tailwind theme (tailwind.config.js, via resolveConfig),
// the class names actually rendered by a component, and the nearest ancestor background.
import resolveConfig from 'tailwindcss/resolveConfig'
// @ts-expect-error tailwind.config.js is plain JS
import tailwindConfig from '../../tailwind.config.js'

export type Theme = 'light' | 'dark'

const theme = (resolveConfig(tailwindConfig as never) as { theme: { colors: Record<string, unknown> } }).theme

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

function pick(el: Element, prefix: 'text' | 'bg', t: Theme): Resolved | null {
  const classes = Array.from(el.classList)
  const find = (pfx: string) => {
    for (const c of classes) {
      if (!c.startsWith(pfx)) continue
      const r = resolveToken(c.slice(pfx.length))
      if (r) return r
    }
    return null
  }
  return (t === 'dark' ? find(`dark:${prefix}-`) : null) ?? find(`${prefix}-`)
}

/** Effective background colour of el: nearest ancestor-or-self with a bg-* colour class, else the page default. */
export function backgroundOf(el: Element, t: Theme): RGB {
  for (let n: Element | null = el; n; n = n.parentElement) {
    const r = pick(n, 'bg', t)
    if (r && r.alpha === 1) return r.rgb
  }
  return t === 'dark' ? resolveToken('gray-900')!.rgb : resolveToken('white')!.rgb
}

/** Effective text colour (alpha blended on the background) of el: nearest ancestor-or-self with a text-* colour class. */
export function foregroundOf(el: Element, t: Theme): RGB {
  const bg = backgroundOf(el, t)
  for (let n: Element | null = el; n; n = n.parentElement) {
    const r = pick(n, 'text', t)
    if (r) return r.rgb.map((c, i) => Math.round(c * r.alpha + bg[i] * (1 - r.alpha))) as RGB
  }
  return t === 'dark' ? resolveToken('white')!.rgb : resolveToken('gray-900')!.rgb
}

export function contrastOf(el: Element, t: Theme): number {
  return ratio(foregroundOf(el, t), backgroundOf(el, t))
}

export const WCAG_AA_NORMAL = 4.5
