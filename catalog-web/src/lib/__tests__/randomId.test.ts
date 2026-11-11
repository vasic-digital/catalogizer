import { describe, it, expect, vi, afterEach } from 'vitest';
import { randomId } from '../randomId';

describe('randomId', () => {
  afterEach(() => vi.restoreAllMocks());

  it('returns 9 lowercase-hex characters', () => {
    expect(randomId()).toMatch(/^[0-9a-f]{9}$/);
  });

  it('is unique across many calls', () => {
    const ids = new Set(Array.from({ length: 1000 }, () => randomId()));
    expect(ids.size).toBe(1000);
  });

  it('does not use Math.random', () => {
    const spy = vi.spyOn(Math, 'random');
    randomId();
    expect(spy).not.toHaveBeenCalled();
  });

  it('falls back to getRandomValues when randomUUID is absent', () => {
    const real = globalThis.crypto;
    const getRandomValues = vi.fn((a: Uint8Array) => { a.fill(0xab); return a; });
    vi.stubGlobal('crypto', { getRandomValues });
    try {
      expect(randomId()).toBe('abababab' + 'a');
      expect(getRandomValues).toHaveBeenCalled();
    } finally {
      vi.stubGlobal('crypto', real);
    }
  });
});
