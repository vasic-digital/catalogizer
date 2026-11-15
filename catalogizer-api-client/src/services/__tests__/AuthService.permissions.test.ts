import { describe, it, expect, vi } from 'vitest';
import { AuthService, permissionsGrant } from '../AuthService';

// Drives the real AuthService.hasPermission with the server vocabulary
// (catalog-api models.Permissions.HasPermission): exact, "*", prefix wildcard on
// a segment boundary; "system.admin" grants everything.
// The live GET /auth/permissions body (catalog-api GetPermissionsGin): an envelope
// {role, permissions, is_admin} with no `data` field, which HttpClient.extractData
// returns as-is.
const envelope = (perms: unknown) => ({ role: 'User', permissions: perms, is_admin: false });
const has = async (granted: unknown[], wanted: string): Promise<boolean> => {
  const http: any = { get: vi.fn().mockResolvedValueOnce(envelope(granted)) };
  return new AuthService(http).hasPermission(wanted);
};

describe('AuthService.hasPermission (server vocabulary)', () => {
  it('exact match grants', async () => expect(await has(['media.view'], 'media.view')).toBe(true));
  it('missing permission is denied', async () => expect(await has(['media.view'], 'media.edit')).toBe(false));
  it('"*" grants everything', async () => expect(await has(['*'], 'anything.at.all')).toBe(true));
  it('media.* grants media.view', async () => expect(await has(['media.*'], 'media.view')).toBe(true));
  it('admin:* grants admin:users on a colon boundary', async () => expect(await has(['admin:*'], 'admin:users')).toBe(true));
  it('media.* does not span mediax.view (segment boundary)', async () => expect(await has(['media.*'], 'mediax.view')).toBe(false));
  it('med* without boundary grants nothing', async () => expect(await has(['med*'], 'media.view')).toBe(false));
  it('bare "*"-free prefix is not a wildcard', async () => expect(await has(['media.'], 'media.view')).toBe(false));
  it('system.admin passes any permission', async () => expect(await has(['system.admin'], 'whatever.perm')).toBe(true));
  it('dead string admin:system no longer grants', async () => expect(await has(['admin:system'], 'whatever.perm')).toBe(false));
  it('non-string entries are ignored, not thrown on', async () => {
    expect(await has([null, 42, { a: 1 }, 'media.view'], 'media.view')).toBe(true);
    expect(await has([null, 42], 'media.view')).toBe(false);
  });

  it('substring (non-prefix) match does not grant: media.* vs x.media.view', async () =>
    expect(await has(['media.*'], 'x.media.view')).toBe(false));
  it('".*" does not match a.b', async () => expect(await has(['.*'], 'a.b')).toBe(false));

  it('getPermissions unwraps the live envelope to the string array', async () => {
    const http: any = { get: vi.fn().mockResolvedValueOnce(envelope(['a', 'b'])) };
    expect(await new AuthService(http).getPermissions()).toEqual(['a', 'b']);
    expect(http.get).toHaveBeenCalledWith('/auth/permissions');
  });
  // SSEF4R-API-FU1: pin getPermissions itself (hasPermission is also guarded by permissionsGrant, so it cannot
  // distinguish a getPermissions that leaks a non-array through).
  it('getPermissions returns [] for every malformed body', async () => {
    for (const bad of [{ permissions: 'x' }, { permissions: null }, null, {}, undefined, 'media.view', 7, { permissions: { 0: 'a' } }]) {
      const http: any = { get: vi.fn().mockResolvedValueOnce(bad) };
      const got = await new AuthService(http).getPermissions();
      expect(Array.isArray(got)).toBe(true);
      expect(got).toEqual([]);
    }
  });
  it('fails closed (false, no throw) on a malformed container', async () => {
    for (const bad of [null, undefined, 'media.view', 7, { permissions: 'media.view' }, { permissions: null }, {}]) {
      const http: any = { get: vi.fn().mockResolvedValueOnce(bad) };
      expect(await new AuthService(http).hasPermission('media.view')).toBe(false);
    }
  });

  it('permissionsGrant fails closed on a non-array container (string would iterate chars)', () => {
    for (const bad of ['*', 'media.view', 7, {}, null, undefined]) {
      expect(permissionsGrant(bad as any, 'media.view')).toBe(false);
    }
  });
});
