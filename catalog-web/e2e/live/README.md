# catalog-web LIVE browser specs (constitution 11.4.289)

Real browser, real login, real catalog-api + nginx proxy. **No** `page.route`, `route.fulfill`, mock helpers or fixture servers anywhere in this directory.
The only deliberate fault injection is a real TCP forwarder (`startForwarder` in `support.ts`) that forwards real bytes and can be severed.

| Spec | Covers |
|---|---|
| `login.live.spec.ts` | valid login (real 200, token accepted by `/auth/me`), wrong password (real 401, visible error, no session), unknown user |
| `registration.live.spec.ts` | client validation; register -> pending (403 `account_pending_approval`) -> admin approves via real API -> login works (REGISTRATION_MODE=approval) |
| `catalog.live.spec.ts` | storage root + scan seeded through the real API from the generated media corpus; Media list/search and Browse counts equal the real API. Honest SKIP when the target refuses local storage roots (no `CATALOGIZER_LOCAL_ROOT_BASES`) |
| `pages.live.spec.ts` | Collections / Favorites / Admin render without the error boundary |
| `websocket.live.spec.ts` | no chip pre-login; one live `/ws` after login and chip hidden; outage via forwarder shows the chip, recovery opens a NEW socket and hides it |
| `session.live.spec.ts` | logout (real POST, local clear, old token revoked), tampered JWT, session survives reload and deep link |

Run: `ADMIN_USERNAME=... ADMIN_PASSWORD=... LIVE_CORPUS_MANIFEST=<corpus manifest.json> LIVE_EXPECTED_WEB_IMAGE_DIGEST=sha256:... e2e/live/run_live.sh <chromium-desktop|chromium-390|firefox-desktop|firefox-390> [runs]`
(credentials only from the environment, `requireEnv`; defaults point at QA stack 3: web `:33020`, API `:38082`, containers `catqa3-web`/`catqa3-api`; override with `PLAYWRIGHT_BASE_URL`, `LIVE_API_URL`, `LIVE_WEB_CONTAINER`, `LIVE_API_CONTAINER`).
`run_live.sh` runs the suite N times (default 3), then `make_evidence.mjs` writes one `catalogizer.live-evidence/1` record per spec and project
(artifact fingerprint read from the target: `/health` body + container image digests; verify with `scripts/test-infra/verify_live_evidence.sh --require-live`).
`LIVE_RED=1` makes the login-error selector deliberately wrong to capture the RED polarity.
Needs Playwright 1.61.x for the installed browser builds (`npm i --no-save @playwright/test@1.61.0`).
