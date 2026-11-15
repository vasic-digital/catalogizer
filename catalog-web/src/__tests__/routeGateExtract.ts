// SSEF4R-FD-m1: gate extraction + completeness check for the route-gate test.
// The extractor only understands double-quoted, gate-first formatting where
// path="..." element={ <ProtectedRoute requiredPermission="..." may be separated
// by whitespace including newlines (the regex's \s*); single quotes, a different
// attribute order or props between the tag and requiredPermission are not captured;
// the completeness check makes any other formatting fail loudly instead of being
// silently ignored.
export const GATE_RE =
  /path="([^"]+)"\s*element=\{\s*<ProtectedRoute requiredPermission="([^"]+)"/g

export function extractGates(src: string): Array<readonly [string, string]> {
  return [...src.matchAll(GATE_RE)].map((m) => [m[1], m[2]] as const)
}

function offsetsOfOccurrences(src: string): number[] {
  return [...src.matchAll(/requiredPermission/g)].map((m) => m.index as number)
}

function capturedOffsets(src: string): Set<number> {
  const out = new Set<number>()
  for (const m of src.matchAll(GATE_RE)) {
    out.add((m.index as number) + m[0].indexOf('requiredPermission'))
  }
  return out
}

export interface Uncaptured {
  line: number
  text: string
}

// Every occurrence of requiredPermission that the extractor did NOT capture.
export function findUncapturedOccurrences(src: string): Uncaptured[] {
  const captured = capturedOffsets(src)
  return offsetsOfOccurrences(src)
    .filter((o) => !captured.has(o))
    .map((o) => {
      const line = src.slice(0, o).split('\n').length
      return { line, text: src.split('\n')[line - 1].trim() }
    })
}

// Throws with the uncaptured occurrences listed.
export function assertAllGatesCaptured(src: string): void {
  const missed = findUncapturedOccurrences(src)
  if (missed.length > 0) {
    throw new Error(
      `route-gate extractor missed ${missed.length} requiredPermission occurrence(s): ` +
        missed.map((u) => `line ${u.line}: ${u.text}`).join(' | ')
    )
  }
}
