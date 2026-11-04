# Web test classification (11.4.289)

Files: 43  |  REAL 3  MOCKED 32  MIXED 5  other 3
Tests (playwright specs): 455  |  real 30  mocked 425

| File | Class | Tests (real/mocked) | Domains | Evidence (file:line) |
|---|---|---|---|---|
| catalog-web/e2e/accessibility.spec.ts | MOCKED | 0/23 | collections,login | L106:mock-helper:setupAllMocks, L118:mock-helper:setupAllMocks, L12:mock-helper:setupAllMocks |
| catalog-web/e2e/auth.spec.ts | MOCKED | 0/23 | collections,login,registration | L117:route-intercept, L118:route.fulfill, L125:route-intercept |
| catalog-web/e2e/browse.spec.ts | MOCKED | 0/18 | - | L100:route-intercept, L101:route.fulfill, L119:route-intercept |
| catalog-web/e2e/collections.spec.ts | MOCKED | 0/20 | collections | L100:route.fulfill, L113:route.fulfill, L122:route.fulfill |
| catalog-web/e2e/favorites.spec.ts | MOCKED | 0/16 | - | L109:route-intercept, L111:route.fulfill, L117:route.fulfill |
| catalog-web/e2e/real-backend-pages.spec.ts | MIXED | 5/1 | collections,login | L61:route-intercept, L62:route.fulfill |
| catalog-web/e2e/responsive.spec.ts | MOCKED | 0/22 | collections,login | L108:mock-helper, L119:mock-helper:setupAllMocks, L12:mock-helper:setupAllMocks |
| catalog-web/e2e/search.spec.ts | MOCKED | 0/16 | collections | L107:route-intercept, L114:route.fulfill, L127:route-intercept |
| catalog-web/e2e/tests/accessibility-expanded.spec.ts | MOCKED | 0/20 | collections,login | L111:mock-helper, L139:mock-helper, L13:mock-helper |
| catalog-web/e2e/tests/accessibility.spec.ts | MOCKED | 0/6 | login,registration | L50:mock-helper, L8:mock-helper |
| catalog-web/e2e/tests/auth-flows.spec.ts | MOCKED | 0/20 | login,registration | L115:route-intercept, L116:route.fulfill, L122:mock-helper |
| catalog-web/e2e/tests/auth.spec.ts | MOCKED | 0/13 | login,registration | L35:route-intercept, L36:route.fulfill, L6:mock-helper |
| catalog-web/e2e/tests/backup-management.spec.ts | MOCKED | 0/4 | admin | L10:mock-helper, L30:route-intercept, L31:route.fulfill |
| catalog-web/e2e/tests/browsing-challenge.spec.ts | REAL | 5/0 | - | - |
| catalog-web/e2e/tests/collections-management.spec.ts | MOCKED | 0/16 | collections | L100:route.fulfill, L108:route-intercept, L109:route.fulfill |
| catalog-web/e2e/tests/collections.spec.ts | MOCKED | 0/2 | collections | L8:mock-helper, L9:mock-helper |
| catalog-web/e2e/tests/critical-flows.spec.ts | MOCKED | 0/11 | collections,login | L12:mock-helper, L135:mock-helper, L139:route-intercept |
| catalog-web/e2e/tests/dashboard.spec.ts | MOCKED | 0/3 | - | L8:mock-helper, L9:mock-helper |
| catalog-web/e2e/tests/error-handling.spec.ts | MIXED | 7/9 | admin,collections,login | L107:mock-helper, L108:mock-helper, L111:route-intercept |
| catalog-web/e2e/tests/favorites.spec.ts | MOCKED | 0/14 | - | L10:mock-helper, L40:route-intercept, L43:route.fulfill |
| catalog-web/e2e/tests/media-browser.spec.ts | MOCKED | 0/2 | login | L8:mock-helper, L9:mock-helper |
| catalog-web/e2e/tests/media-browsing.spec.ts | MOCKED | 0/17 | - | L12:route-intercept, L13:route.fulfill, L170:route-intercept |
| catalog-web/e2e/tests/media-playback.spec.ts | MOCKED | 0/6 | playback | L10:mock-helper, L11:mock-helper, L14:route-intercept |
| catalog-web/e2e/tests/media-quality.spec.ts | MOCKED | 0/7 | - | L10:mock-helper, L11:mock-helper, L14:route-intercept |
| catalog-web/e2e/tests/navigation.spec.ts | MIXED | 1/16 | admin,collections,login,registration | L105:mock-helper:setupAllPageMocks, L10:mock-helper, L13:route-intercept |
| catalog-web/e2e/tests/password-change.spec.ts | MOCKED | 0/5 | - | L10:mock-helper, L13:route-intercept, L27:route.fulfill |
| catalog-web/e2e/tests/performance.spec.ts | MOCKED | 0/14 | login | L22:mock-helper, L239:mock-helper, L240:mock-helper |
| catalog-web/e2e/tests/playlists.spec.ts | MOCKED | 0/7 | - | L101:route-intercept, L102:route.fulfill, L10:mock-helper |
| catalog-web/e2e/tests/protected-routes.spec.ts | MIXED | 7/2 | collections,login,registration | L53:mock-helper, L54:mock-helper, L74:mock-helper |
| catalog-web/e2e/tests/recent-popular.spec.ts | MOCKED | 0/8 | - | L10:mock-helper, L11:mock-helper, L14:route-intercept |
| catalog-web/e2e/tests/registration.spec.ts | MOCKED | 0/13 | login,registration | L145:route-intercept, L146:route.fulfill, L6:mock-helper |
| catalog-web/e2e/tests/responsive-layout.spec.ts | MOCKED | 0/20 | collections,login | L237:mock-helper, L238:mock-helper, L250:mock-helper |
| catalog-web/e2e/tests/search.spec.ts | MOCKED | 0/8 | collections | L105:route-intercept, L107:route.fulfill, L10:mock-helper |
| catalog-web/e2e/tests/settings-admin.spec.ts | MOCKED | 0/15 | admin | L10:route-intercept, L113:route-intercept, L115:route.fulfill |
| catalog-web/e2e/tests/visual-regression-dark.spec.ts | MOCKED | 0/3 | login | L28:mock-helper, L60:mock-helper |
| catalog-web/e2e/tests/visual-regression.spec.ts | MOCKED | 0/17 | login | L118:mock-helper, L145:mock-helper, L196:mock-helper |
| catalog-web/e2e/tests/websocket-realtime.spec.ts | MOCKED | 0/8 | collections | L108:mock-helper, L109:mock-helper, L111:route-intercept |
| catalogizer-desktop/e2e/theme.spec.ts | REAL | 5/0 | - | - |
| tests/responsive-audit/measure_d1_d2.mjs | REAL | 0/0 | - | - |
| tests/responsive-audit/needle.mjs | SELFTEST | 0/0 | - | - |
| tests/responsive-audit/oracle.mjs | HELPER | 0/0 | - | - |
| tests/responsive-audit/run.mjs | MIXED | 0/0 | - | L53:init-script-double |
| tests/responsive-audit/serve.mjs | HELPER | 0/0 | - | - |

## Domain coverage (playback / login / registration / collections / admin)

- **playback**: tests/media-playback.spec.ts [MOCKED]
- **login**: accessibility.spec.ts [MOCKED], auth.spec.ts [MOCKED], real-backend-pages.spec.ts [MIXED], responsive.spec.ts [MOCKED], tests/accessibility-expanded.spec.ts [MOCKED], tests/accessibility.spec.ts [MOCKED], tests/auth-flows.spec.ts [MOCKED], tests/auth.spec.ts [MOCKED], tests/critical-flows.spec.ts [MOCKED], tests/error-handling.spec.ts [MIXED], tests/media-browser.spec.ts [MOCKED], tests/navigation.spec.ts [MIXED], tests/performance.spec.ts [MOCKED], tests/protected-routes.spec.ts [MIXED], tests/registration.spec.ts [MOCKED], tests/responsive-layout.spec.ts [MOCKED], tests/visual-regression-dark.spec.ts [MOCKED], tests/visual-regression.spec.ts [MOCKED]
- **registration**: auth.spec.ts [MOCKED], tests/accessibility.spec.ts [MOCKED], tests/auth-flows.spec.ts [MOCKED], tests/auth.spec.ts [MOCKED], tests/navigation.spec.ts [MIXED], tests/protected-routes.spec.ts [MIXED], tests/registration.spec.ts [MOCKED]
- **collections**: accessibility.spec.ts [MOCKED], auth.spec.ts [MOCKED], collections.spec.ts [MOCKED], real-backend-pages.spec.ts [MIXED], responsive.spec.ts [MOCKED], search.spec.ts [MOCKED], tests/accessibility-expanded.spec.ts [MOCKED], tests/collections-management.spec.ts [MOCKED], tests/collections.spec.ts [MOCKED], tests/critical-flows.spec.ts [MOCKED], tests/error-handling.spec.ts [MIXED], tests/navigation.spec.ts [MIXED], tests/protected-routes.spec.ts [MIXED], tests/responsive-layout.spec.ts [MOCKED], tests/search.spec.ts [MOCKED], tests/websocket-realtime.spec.ts [MOCKED]
- **admin**: tests/backup-management.spec.ts [MOCKED], tests/error-handling.spec.ts [MIXED], tests/navigation.spec.ts [MIXED], tests/settings-admin.spec.ts [MOCKED]

## Make-live plan per MOCKED/MIXED file

### catalog-web/e2e/accessibility.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/entities/stats, **/api/v1/entities/types, **/api/v1/favorites**, **/api/v1/favorites/stats**, **/api/v1/media/stats**); assert on real responses.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/auth.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/init-status, **/api/v1/auth/login, **/api/v1/auth/status); assert on real responses.
- Mocked error statuses ['401', '500']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/browse.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/entities/*, **/api/v1/entities/*/children, **/api/v1/entities/*/duplicates, **/api/v1/entities/*/files, **/api/v1/entities/*/metadata**, **/api/v1/entities/*/user-metadata); assert on real responses.
- Mocked error statuses ['404']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/collections.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/collections**, **/api/v1/collections/*/duplicate, **/api/v1/collections/*/export**, **/api/v1/collections/*/items**, **/api/v1/collections/*/share, **/api/v1/collections/bulk**); assert on real responses.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/favorites.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/entities/*/user-metadata, **/api/v1/favorites**, **/api/v1/favorites/stats**); assert on real responses.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/real-backend-pages.spec.ts (MIXED)
- Delete page.route/route.fulfill interception (urls: **/api/v1/media/search*); assert on real responses.
- Visual snapshots were taken against mock data; regenerate baselines against the seeded live corpus (deterministic seed required) and record the artifact fingerprint.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- WebSocket behaviour: connect to the real /ws endpoint and trigger a real server event (scan/convert job) instead of a scripted frame.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/responsive.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/entities/stats, **/api/v1/entities/types, **/api/v1/favorites**, **/api/v1/favorites/stats**, **/api/v1/media/stats**); assert on real responses.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/search.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/collections**, **/api/v1/entities/stats, **/api/v1/entities/types, **/api/v1/entities?**, **/api/v1/media, **/api/v1/media/search**); assert on real responses.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/accessibility-expanded.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/login); assert on real responses.
- Mocked error statuses ['401']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/accessibility.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/auth-flows.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/login); assert on real responses.
- Mocked error statuses ['401', '500']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/auth.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/login); assert on real responses.
- Mocked error statuses ['401']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/backup-management.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/**, **/api/v1/admin/backup, **/api/v1/admin/backup/restore, **/api/v1/admin/backups**); assert on real responses.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/collections-management.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/collections**, **/api/v1/collections/*, **/api/v1/collections/*/duplicate, **/api/v1/collections/*/export**, **/api/v1/collections/*/share, **/api/v1/collections/bulk**); assert on real responses.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/collections.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/critical-flows.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/**); assert on real responses.
- Mocked error statuses ['401']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/dashboard.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/error-handling.spec.ts (MIXED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/login, **/api/v1/media/999, **/api/v1/media/search**, **/api/v1/media/stats**, **/api/v1/stats**); assert on real responses.
- Mocked error statuses ['401', '403', '404', '500']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/favorites.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/favorites**, **/api/v1/favorites/stats**); assert on real responses.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/media-browser.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/media-browsing.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/media/search**, **/api/v1/media/stats**); assert on real responses.
- Mocked error statuses ['500']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/media-playback.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/media/search**, **/api/v1/media/stats**, **/api/v1/playback/position**); assert on real responses.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/media-quality.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/**, **/api/v1/media/*/quality, **/api/v1/media/quality**); assert on real responses.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/navigation.spec.ts (MIXED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/**, **/api/v1/analytics**, **/api/v1/collections**, **/api/v1/conversion**, **/api/v1/favorites**, **/api/v1/favorites/stats**); assert on real responses.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/password-change.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/**, **/api/v1/auth/change-password); assert on real responses.
- Mocked error statuses ['401']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/performance.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/playlists.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/**, **/api/v1/playlists**); assert on real responses.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/protected-routes.spec.ts (MIXED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/recent-popular.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/**, **/api/v1/favorites**, **/api/v1/media/popular**, **/api/v1/media/recent**); assert on real responses.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/registration.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/register); assert on real responses.
- Mocked error statuses ['409']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/responsive-layout.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/search.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/collections**, **/api/v1/media, **/api/v1/media/search**, **/api/v1/media/stats**, **/api/v1/search/history**); assert on real responses.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/settings-admin.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/admin/backups**, **/api/v1/admin/storage**, **/api/v1/admin/system**, **/api/v1/admin/users**, **/api/v1/settings**); assert on real responses.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/visual-regression-dark.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Visual snapshots were taken against mock data; regenerate baselines against the seeded live corpus (deterministic seed required) and record the artifact fingerprint.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/visual-regression.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/auth/login); assert on real responses.
- Mocked error statuses ['401']: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Visual snapshots were taken against mock data; regenerate baselines against the seeded live corpus (deterministic seed required) and record the artifact fingerprint.
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### catalog-web/e2e/tests/websocket-realtime.spec.ts (MOCKED)
- Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.
- Delete page.route/route.fulfill interception (urls: **/api/v1/collections**, **/api/v1/favorites**, **/api/v1/favorites/stats**, **/api/v1/media/stats**); assert on real responses.
- Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).
- Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).
- WebSocket behaviour: connect to the real /ws endpoint and trigger a real server event (scan/convert job) instead of a scripted frame.
- Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.
- Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).

### tests/responsive-audit/run.mjs (MIXED)
- Replace the emulated Tauri IPC bridge (window.__TAURI_INTERNALS__) with the real Tauri window (built desktop artifact under Xvfb) for the desktop target; web/installer targets are already live.

