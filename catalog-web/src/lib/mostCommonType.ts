/**
 * Returns the human-readable key with the highest count, or 'None' when the
 * breakdown is empty (a bare reduce() without an initial value throws there).
 */
export function mostCommonType(breakdown: Record<string, number>): string {
  const entries = Object.entries(breakdown);
  if (entries.length === 0) return 'None';
  const [type] = entries.reduce((a, b) => (a[1] > b[1] ? a : b), entries[0]);
  return type.replace('_', ' ');
}
