import { formatPercentOrUnavailable, formatCountOrUnavailable, usagePercent, UNAVAILABLE_LABEL } from '../adminMetrics'

describe('adminMetrics null handling', () => {
  it('shows Unavailable for null, never 0', () => {
    expect(formatPercentOrUnavailable(null)).toBe(UNAVAILABLE_LABEL)
    expect(formatCountOrUnavailable(null)).toBe(UNAVAILABLE_LABEL)
    expect(formatPercentOrUnavailable(undefined as unknown as null)).toBe(UNAVAILABLE_LABEL)
  })
  it('keeps a real zero distinct from unavailable', () => {
    expect(formatPercentOrUnavailable(0)).not.toBe(UNAVAILABLE_LABEL)
    expect(formatCountOrUnavailable(0)).not.toBe(UNAVAILABLE_LABEL)
  })
  it('usagePercent is null when inputs are missing or total is not positive', () => {
    expect(usagePercent(null, 10)).toBeNull()
    expect(usagePercent(5, null)).toBeNull()
    expect(usagePercent(5, 0)).toBeNull()
    expect(usagePercent(5, 10)).toBe(50)
  })
})
