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

describe('api client CSRF retry', () => {
  it('retries a mutating request once with the token handed back by a csrf rejection', async () => {
    vi.resetModules();
    const { api } = await import('../api');
    const seen: Seen[] = [];
    api.defaults.adapter = async (config: InternalAxiosRequestConfig): Promise<AxiosResponse> => {
      const get = (config.headers as unknown as { get(n: string): string | undefined }).get.bind(config.headers);
      seen.push({ method: config.method, csrf: get('X-CSRF-Token') });
      if (get('X-CSRF-Token') !== 'fresh') {
        const err = Object.assign(new Error('csrf'), { config, isAxiosError: true, response: { status: 403, data: { error: 'csrf token mismatch' }, headers: { 'x-csrf-token': 'fresh' }, config, statusText: 'Forbidden' } });
        throw err;
      }
      return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config };
    };
    const res = await api.put('/admin/users/9', { status: 'active' });
    expect(res.data).toEqual({ ok: true });
    expect(seen.map((s) => s.csrf)).toEqual([undefined, 'fresh']);
  });

  it('does not retry forever and does not retry non-csrf 403s', async () => {
    vi.resetModules();
    const { api } = await import('../api');
    let calls = 0;
    api.defaults.adapter = async (config: InternalAxiosRequestConfig): Promise<AxiosResponse> => {
      calls++;
      throw Object.assign(new Error('forbidden'), { config, isAxiosError: true, response: { status: 403, data: { error: 'Insufficient permissions' }, headers: {}, config, statusText: 'Forbidden' } });
    };
    await expect(api.put('/users/9', {})).rejects.toBeTruthy();
    expect(calls).toBe(1);
    calls = 0;
    api.defaults.adapter = async (config: InternalAxiosRequestConfig): Promise<AxiosResponse> => {
      calls++;
      throw Object.assign(new Error('csrf'), { config, isAxiosError: true, response: { status: 403, data: { error: 'csrf token mismatch' }, headers: { 'x-csrf-token': 'x' }, config, statusText: 'Forbidden' } });
    };
    await expect(api.put('/admin/users/9', {})).rejects.toBeTruthy();
    expect(calls).toBe(2);
  });
});
