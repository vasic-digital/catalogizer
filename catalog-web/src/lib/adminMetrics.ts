/** Helpers that keep "not measured" distinct from 0 in the admin views. */

export const UNAVAILABLE_LABEL = 'Unavailable';

/** "42%" for a measured percentage, "Unavailable" for null/undefined/NaN. */
export function formatPercentOrUnavailable(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? `${value}%` : UNAVAILABLE_LABEL;
}

/** used/total as a 0-100 percentage, or null when either figure is missing or total is not positive. */
export function usagePercent(used: number | null | undefined, total: number | null | undefined): number | null {
  if (typeof used !== 'number' || typeof total !== 'number' || !Number.isFinite(used) || !Number.isFinite(total) || total <= 0) {
    return null;
  }
  return Math.min(100, Math.max(0, (used / total) * 100));
}

/** Locale-formatted integer, or "Unavailable" for null/undefined. */
export function formatCountOrUnavailable(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString() : UNAVAILABLE_LABEL;
}
