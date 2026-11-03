/**
 * Helpers that normalise the response envelopes catalog-api actually emits.
 *
 * The backend is not uniform: depending on the handler a list arrives as a bare
 * array, as `{items,total}`, `{scans:[...]}`, `{roots:[...]}`, `{data:[...],
 * success:true}` (and `data:null` when empty on older builds). Pages iterate
 * the value they are handed, so every api module that returns a list MUST pass
 * the wire payload through `asList` instead of trusting `res.data` to be an
 * array (that assumption crashed /analytics, /collections and /settings).
 */

type Json = unknown

/** Return the first array found at `payload` or at payload[key] for key in `keys` (then `data`, `items`); otherwise []. */
export function asList<T = unknown>(payload: Json, ...keys: string[]): T[] {
  if (Array.isArray(payload)) return payload as T[]
  if (payload && typeof payload === 'object') {
    const obj = payload as Record<string, Json>
    for (const key of [...keys, 'items', 'data']) {
      const value = obj[key]
      if (Array.isArray(value)) return value as T[]
    }
  }
  return []
}

/** Return `payload.data` for `{data:…,success:true}` envelopes, otherwise the payload itself. */
export function unwrapData<T = unknown>(payload: Json): T {
  if (payload && typeof payload === 'object' && !Array.isArray(payload) && 'data' in (payload as object)) {
    return (payload as { data: T }).data
  }
  return payload as T
}

/**
 * Decode catalog-api's generated-report envelope
 * `{type, format:'json', content:'<base64 json>', generated_at, parameters}`
 * into the JSON document it carries. Returns null when the payload is not such
 * an envelope or cannot be decoded.
 */
export function decodeReportEnvelope<T = Record<string, unknown>>(payload: Json): T | null {
  if (!payload || typeof payload !== 'object') return null
  const env = payload as { format?: string; content?: unknown }
  if (typeof env.content !== 'string') return null
  try {
    const binary = atob(env.content)
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
    return JSON.parse(new TextDecoder().decode(bytes)) as T
  } catch {
    return null
  }
}
