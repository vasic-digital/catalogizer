import { describe, it, expect } from 'vitest'
describe('adder', () => {
  it('adds', () => { expect(2 + 3).toBe(5) })
  it('seeded failure', () => { expect(2 - 3).toBe(5) })
  it.skip('skipped', () => {})
})
