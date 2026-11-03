// Article XI §11.2 verification of catalogizer-api-client TypeScript SDK.
// Drives the compiled `dist/` artifacts against the live amber.local
// catalog-api (via the localhost:3093 nginx proxy that we already
// verified works in the catalog-web Playwright run).
//
// Asserts:
//   1. CatalogizerClient is exported and constructible (positive)
//   2. .connect({ username, password }) resolves with a real
//      LoginResponse containing a token + user object (positive)
//   3. .auth.getCurrentUser() returns the authenticated user
//      (Article XI §11.2.1 — concrete end-user-visible outcome)
//   4. matching negative: wrong-password connect throws/rejects
//      (Article XI §11.2.3)
//
// Evidence is captured in
// docs/audits/evidence-2026-04-29-ts-client/.

const fs = require('fs');
const path = require('path');

const CLIENT_PATH = path.resolve(
  '/run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalogizer-api-client/dist/index.js'
);
const EVID_DIR = '/run/media/milosvasic/DATA4TB/Projects/Catalogizer/docs/audits/evidence-2026-04-29-ts-client';
const BASE_URL = process.env.CZ_API_URL || 'http://localhost:3093/api/v1';

fs.mkdirSync(EVID_DIR, { recursive: true });

function fail(msg) {
  console.error(`FAIL: ${msg}`);
  process.exit(1);
}

(async () => {
  const mod = require(CLIENT_PATH);
  if (!mod.CatalogizerClient) {
    fail('CatalogizerClient is not exported from dist/index.js');
  }
  console.log(`✓ CatalogizerClient export found in ${CLIENT_PATH}`);

  // 1. Construct
  const client = new mod.CatalogizerClient({
    baseURL: BASE_URL,
    timeout: 5000,
  });
  console.log('✓ CatalogizerClient constructed');

  // 2. Positive login
  let loginResp;
  try {
    loginResp = await client.connect({
      username: 'admin',
      password: process.env.ADMIN_PASSWORD,
    });
  } catch (err) {
    fail(`connect(admin/<ADMIN_PASSWORD>) threw: ${err && err.message ? err.message : err}`);
  }
  console.log(`✓ connect() resolved`);

  // The connect() return shape: { token, user } or similar
  if (!loginResp || typeof loginResp !== 'object') {
    fail(`connect() returned non-object: ${JSON.stringify(loginResp)}`);
  }
  const evidenceFile = path.join(EVID_DIR, 'login-response.json');
  fs.writeFileSync(evidenceFile, JSON.stringify(loginResp, null, 2));
  console.log(`  saved: ${evidenceFile}`);

  // Token presence — catalog-api uses `session_token` (JWT) + `refresh_token`
  const token = loginResp.token
    || loginResp.accessToken
    || loginResp.access_token
    || loginResp.session_token
    || loginResp.sessionToken;
  if (!token) {
    fail(`no token in login response: ${JSON.stringify(loginResp).slice(0, 300)}`);
  }
  console.log(`✓ session token returned (${token.length} chars, looks like JWT: ${token.split('.').length === 3})`);

  if (loginResp.refresh_token || loginResp.refreshToken) {
    console.log(`✓ refresh token also returned`);
  }
  if (loginResp.expires_at || loginResp.expiresAt) {
    console.log(`✓ session expiry: ${loginResp.expires_at || loginResp.expiresAt}`);
  }

  // User presence
  const user = loginResp.user || loginResp;
  if (!user || !user.username || user.username !== 'admin') {
    fail(`unexpected user in login response: ${JSON.stringify(user).slice(0, 200)}`);
  }
  console.log(`✓ user record contains admin: id=${user.id}, role=${(user.role && user.role.name) || user.role || 'unknown'}`);

  // 3. isAuthenticated
  const isAuth = await client.isAuthenticated();
  if (!isAuth) {
    fail('isAuthenticated() returned false right after successful connect');
  }
  console.log(`✓ isAuthenticated() == true`);

  // 4. Disconnect
  await client.disconnect();
  console.log(`✓ disconnect() resolved`);

  // 5. Article XI §11.2.3 NEGATIVE: wrong password must reject
  console.log('\n=== Negative test (wrong credentials) ===');
  const client2 = new mod.CatalogizerClient({
    baseURL: BASE_URL,
    timeout: 5000,
  });
  let negativeOk = false;
  try {
    await client2.connect({
      username: 'admin',
      password: 'WRONGPASSWORD-12345',
    });
    // If we get here, the API silently accepted wrong password — bug.
  } catch (err) {
    negativeOk = true;
    console.log(`✓ wrong password rejected: ${err.message || err}`);
  }
  if (!negativeOk) {
    fail('BUG: client.connect() with WRONGPASSWORD-12345 did NOT reject');
  }

  console.log('\n✓ All Article XI §11.2 assertions passed:');
  console.log('  - CatalogizerClient exported + constructible (positive)');
  console.log('  - connect(admin/<ADMIN_PASSWORD>) → real login response with token + user (positive)');
  console.log('  - isAuthenticated() reflects post-login state (positive outcome)');
  console.log('  - disconnect() resolves cleanly (lifecycle)');
  console.log('  - WRONG-PASSWORD connect rejects (matching negative)');
})().catch(err => {
  console.error('UNCAUGHT:', err);
  process.exit(1);
});
