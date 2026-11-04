# catalog-web LIVE browser specs (constitution 11.4.289)

Real browser, real login, real catalog-api + nginx proxy. **No** `page.route`, `route.fulfill`, mock helpers or fixture servers anywhere in this directory.
The only deliberate fault injection is a real TCP forwarder (`startForwarder` in `support.ts`) that forwards real bytes and can be severed.

| Spec | Covers |
|---|---|
| `login.live.spec.ts` | valid login (real 200, token accepted by `/auth/me`), wrong password (real 401, visible error, no session), unknown user |
| `registration.live.spec.ts` | client validation; register -> pending (403 `account_pending_approval`) -> admin approves via real API -> login works (REGISTRATION_MODE=approval) |
| `catalog.live.spec.ts` | storage root + scan seeded through the real API from the generated media corpus; Media list/search and Browse counts equal the real API. Honest SKIP when the target refuses local storage roots (no `CATALOGIZER_LOCAL_ROOT_BASES`) |
| `playback.live.spec.ts` | corpus files streamed through the real `/stream/:id` (sha256 equal to the manifest, Range 206/416), stream-ticket path (mint, 60 s ticket valid only for its media id and only on the stream route), and the web player: `<video>` `currentTime` advances for an mp4, the same player plays an mp3, media requests carry a ticket and never the session JWT |
| `management.live.spec.ts` | favourite from a Media card (stored for the clicked item, listed on the Favorites page), manual collection CRUD, smart collection rule evaluated by the server (+ invalid rules 400), foreign-user isolation, admin Users tab Approve (UI), lock/unlock/role change effect on login, conversion page + a real mp3 to wav job |
| `pages.live.spec.ts` | Collections / Favorites / Admin render without the error boundary |
| `websocket.live.spec.ts` | no chip pre-login; one live `/ws` after login and chip hidden; outage via forwarder shows the chip, recovery opens a NEW socket and hides it |
| `session.live.spec.ts` | logout (real POST, local clear, old token revoked), tampered JWT, session survives reload and deep link |

Run: `ADMIN_USERNAME=... ADMIN_PASSWORD=... LIVE_CORPUS_MANIFEST=<corpus manifest.json> LIVE_EXPECTED_WEB_IMAGE_DIGEST=sha256:... e2e/live/run_live.sh <chromium-desktop|chromium-390|firefox-desktop|firefox-390> [runs]`
(credentials only from the environment, `requireEnv`; defaults point at QA stack 3: web `:33020`, API `:38082`, containers `catqa3-web`/`catqa3-api`; override with `PLAYWRIGHT_BASE_URL`, `LIVE_API_URL`, `LIVE_WEB_CONTAINER`, `LIVE_API_CONTAINER`).
`run_live.sh` runs the suite N times (default 3), then `make_evidence.mjs` writes one `catalogizer.live-evidence/1` record per spec and project
(artifact fingerprint read from the target: `/health` body + container image digests; verify with `scripts/test-infra/verify_live_evidence.sh --require-live`).
`LIVE_RED=1` makes the login-error selector deliberately wrong to capture the RED polarity.
**Evidence recorder dependency.** The recorder (`tests/live-evidence/`, `docs/qa/live-evidence.schema.json`, `docs/qa/live-evidence.doubles.txt`, `scripts/test-infra/verify_live_evidence.sh`) lives on branch `test-live-evidence-recorder`, not in this branch; it is referenced by path only. To verify the committed records, check out those four paths from that branch (`git checkout test-live-evidence-recorder -- tests/live-evidence docs/qa/live-evidence.schema.json docs/qa/live-evidence.doubles.txt scripts/test-infra/verify_live_evidence.sh`) and run `scripts/test-infra/verify_live_evidence.sh --require-live <evidence dir>`.
The committed evidence under `docs/qa/web-live-20261103/` is trimmed to the JSON records, run logs and a few screenshots; the full artifact set (traces, all screenshots) is kept outside the repository with a sha256 list (`FULL_ARTIFACTS.sha256`).
Needs Playwright 1.61.x for the installed browser builds (`npm i --no-save @playwright/test@1.61.0`).

## Committed evidence (docs/qa/web-live-20261103/stack3)

Trimmed: JSON evidence records (all `class=LIVE` under `scripts/test-infra/verify_live_evidence.sh --require-live`), Playwright `report.json`, run logs, and a small screenshot set in `screenshots/`. The full set (traces, videos, all screenshots) is kept outside the repository; `FULL_ARTIFACTS.sha256` lists the sha256 of every file of that full set. Target: stack 3, API image `catqa3-api:main-71552b16`, web image digest `sha256:cc9d4093...`. Results: 4 projects x 3 runs, 33 tests each (incl. the long-playback leg), all exit=0, image main-71552b16.
