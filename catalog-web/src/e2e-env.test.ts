import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { requireEnv } from '../e2e/fixtures/env';

// Literal credential defaults: `|| 'x'` / `?? 'x'` after a password env read,
// or a `password: 'literal'` object property in fixtures.
const LITERAL_PASSWORD_DEFAULT =
  /(PASSWORD[^\n]*(\|\||\?\?)\s*['"`][^'"`]+['"`])|(\bpassword\s*:\s*['"`][^'"`]+['"`])/i;

function listTs(dir: string): string[] {
  return readdirSync(dir).flatMap((n) => {
    const p = join(dir, n);
    if (n === 'node_modules') return [];
    return statSync(p).isDirectory() ? listTs(p) : p.endsWith('.ts') ? [p] : [];
  });
}

describe('requireEnv', () => {
  it('throws naming the variable (not a value) when unset', () => {
    expect(() => requireEnv('ADMIN_PASSWORD', {})).toThrow(/ADMIN_PASSWORD/);
  });
  it('throws when set to the empty string', () => {
    expect(() => requireEnv('ADMIN_PASSWORD', { ADMIN_PASSWORD: '' })).toThrow(/ADMIN_PASSWORD/);
  });
  it('returns the provided value when set', () => {
    expect(requireEnv('ADMIN_PASSWORD', { ADMIN_PASSWORD: 'from-env' })).toBe('from-env');
  });
});

describe('e2e tree has no literal credential defaults', () => {
  it('control needle: the scanner pattern detects a known literal default', () => {
    expect(LITERAL_PASSWORD_DEFAULT.test("const p = process.env.ADMIN_PASSWORD || 'x1';")).toBe(true);
    expect(LITERAL_PASSWORD_DEFAULT.test("const p = requireEnv('ADMIN_PASSWORD');")).toBe(false);
  });
  it('no e2e file under ./e2e carries a literal password default', () => {
    const files = listTs(join(__dirname, '..', 'e2e'));
    expect(files.length).toBeGreaterThan(10);
    const hits = files.flatMap((f) =>
      readFileSync(f, 'utf8')
        .split('\n')
        .map((l, i) => ({ f, i: i + 1, l }))
        .filter(({ l }) => LITERAL_PASSWORD_DEFAULT.test(l) && !/fill\(/.test(l))
        .map(({ f, i }) => `${f.replace(__dirname + '/../', '')}:${i}`),
    );
    expect(hits).toEqual([]);
  });
});
