import { describe, it, expect } from 'vitest';
import { mostCommonType } from '../mostCommonType';

describe('mostCommonType', () => {
  it('returns the highest-count key with underscores spaced', () => {
    expect(mostCommonType({ movie: 1, tv_show: 5, music: 2 })).toBe('tv show');
  });
  it('does not throw on an empty breakdown', () => {
    expect(mostCommonType({})).toBe('None');
  });
});
