// Found by the LIVE admin spec: the admin group mints an X-CSRF-Token on GET and requires it back on every mutating call
// ("missing csrf header" -> 403), but the web client never echoed it, so every admin write from the browser (approve user, create
// backup, scan storage) failed. Unit test with an in-process axios adapter (unit-class evidence; the live spec is the proof).
import { describe, it, expect, beforeEach, vi } from 'vitest';
import type { InternalAxiosRequestConfig, AxiosResponse } from 'axios';

type Seen = { method?: string; csrf?: string };

async function install(seen: Seen[], tokenOnGet: string | null) {
  const { api } = await import('../api');
  api.defaults.adapter = async (config: InternalAxiosRequestConfig): Promise<AxiosResponse> => {
    seen.push({ method: config.method, csrf: (config.headers as unknown as { get(n: string): string | undefined }).get('X-CSRF-Token') });
    const headers: Record<string, string> = {};
    if (config.method === 'get' && tokenOnGet) headers['x-csrf-token'] = tokenOnGet;
    return { data: {}, status: 200, statusText: 'OK', headers, config };
  };
  return api;
}

describe('api client CSRF echo', () => {
  beforeEach(() => { localStorage.clear(); vi.resetModules(); });

  it('sends the token minted by an earlier GET on later mutating requests, and never on GET', async () => {
    const seen: Seen[] = [];
    const api = await install(seen, 'tok-123');
    await api.get('/admin/users');
    await api.put('/admin/users/9', { status: 'active' });
    await api.post('/admin/backups', { type: 'full' });
    await api.delete('/admin/x');
    expect(seen[0]).toEqual({ method: 'get', csrf: undefined });
    for (const s of seen.slice(1)) expect(s.csrf, `${s.method} carries the minted token`).toBe('tok-123');
  });

  it('sends no header when no token was ever minted', async () => {
    const seen: Seen[] = [];
    const api = await install(seen, null);
    await api.post('/auth/logout');
    expect(seen[0].csrf).toBeUndefined();
  });
});
