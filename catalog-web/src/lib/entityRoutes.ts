/**
 * Single builder for the entity detail route (App.tsx: `/entity/:id`).
 * Every entity type in the closed set (movie, tv_show, ... comic) is served
 * by that one route, so the id is the only variable part.
 *
 * Contract: `id` is a number or a canonical decimal string. It is accepted only
 * when it is a positive SAFE integer (no 0, negatives, NaN, fractions, 1e21,
 * values above 2^53-1, "0x10", "1e3", "007"). Anything else yields null so
 * callers can surface "no valid item id" instead of sending the user to a
 * bogus entity (EntityDetail would parseInt('1e+21') to 1).
 */
export function entityPath(id: number | string, opts?: { autoplay?: boolean }): string | null {
  let n: number
  if (typeof id === 'number') {
    n = id
  } else if (typeof id === 'string' && /^[1-9]\d*$/.test(id)) {
    n = Number(id)
  } else {
    return null
  }
  if (!Number.isSafeInteger(n) || n <= 0) return null
  return opts?.autoplay ? `/entity/${n}?autoplay=true` : `/entity/${n}`
}

/** Strict parse of a wire entity id; null when absent or not a canonical positive safe integer. */
export function parseEntityId(raw: unknown): number | null {
  if (typeof raw === 'number') return Number.isSafeInteger(raw) && raw > 0 ? raw : null
  if (typeof raw === 'string' && /^[1-9]\d*$/.test(raw)) {
    const n = Number(raw)
    return Number.isSafeInteger(n) ? n : null
  }
  return null
}
