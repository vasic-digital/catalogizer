/**
 * Non-guessable id helper backed by the Web Crypto API.
 * Replaces Math.random()-based ids (SonarQube typescript:S2245).
 */
export function randomId(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === 'function') {
    return c.randomUUID().replace(/-/g, '').slice(0, 9);
  }
  const bytes = new Uint8Array(6);
  if (c && typeof c.getRandomValues === 'function') {
    c.getRandomValues(bytes);
  } else {
    throw new Error('Web Crypto API is not available');
  }
  return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('').slice(0, 9);
}
