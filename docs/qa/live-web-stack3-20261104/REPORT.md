# Live web specs, stack 3, round 3 (2026-11-04)
Target (LAN): web http://192.168.1.115:33020, API http://192.168.1.115:38082, website :33021. Specs + stack from main 4c79b7e3611fae4c400555ddf7f8db7a677f9ef7 (fresh clone /home/milosvasic/.cache/claude-tmp/r3w.RAwfvz/c, no commits). Playwright 1.61.0, workers=1, each run under systemd-run MemoryMax=4G. Host load 6-15 during runs.

## Step 1: rebuild + up
- build.sh 4c79b7e3 (SUBMODULE_SRC = live checkout) -> localhost/catqa3-{api,web}:main-4c79b7e3; BUILT_COMMIT = 4c79b7e3611f...; up.sh exit 0 (all 4 probes 200).
- /health (LAN IP): status healthy, version = build_number = 4c79b7e3611fae4c400555ddf7f8db7a677f9ef7 (equals built commit).
- Digests: api sha256:d14b46f9da9eeaeee0a6f51b47fcbd8a0d0125947de4130727f6bbae385e72bb (imageid 4ff31faff825...), web sha256:89e07bcf369604ef91ceb6aa2320d914a78881965779f0cedc599ad4cfdef07a (imageid bb1affdd75e1...). Full container ids: live-web-stack3-r3/machine-fields/target.txt.
- Probes: web :33020, API :38082 /health, website :33021 = 200 on both 127.0.0.1 and 192.168.1.115.

## Step 2: HTTP facts over the LAN URL (machine-fields/step2_probe.json, step2_csrf.json)
(a) NS-1 CLOSED. GET /entities/types: 200, `types` = array (len 11) normally. Empty-table condition created without data loss: copied media_types to a backup table, disabled FK triggers on media_items/detection_rules/media_types (superuser), emptied media_types (count 0), probed, restored rows, re-enabled triggers, dropped backup. Result with empty table: 200, `types` = array of length 0, raw body does not contain `"types":null`. Restore verified: md5 of media_types identical before/after (0bb722af...|11 rows), media_items still 17, triggers enabled. Handler code also initialises `result := []gin.H{}`.
(b) CSRF-1 CLOSED. Set-Cookie on GET /admin/users: name `csrf-dev`, attrs path, max-age, HttpOnly, NO Secure flag; X-CSRF-Token response header present. Registered a throwaway pending user (201, status pending); admin PUT /admin/users/:id {status:active} over plain http on the LAN URL: with bare token from X-CSRF-Token header + matching cookie -> 200 (status became active, approved user logs in 200); mismatched token -> 403; no CSRF header/cookie -> 403. Throwaway user deleted (204). (Note: the header must carry the bare token from the response header, not the signed cookie value; my first attempt using the cookie value got 403, that was my probe error, not a product defect.)

## Step 3: Playwright live suite (33 tests per run), LAN URLs
| project | runs | result |
|---|---|---|
| chromium-desktop | 3 | 33/33 pass x3, identical signature 845d50fe631f |
| firefox-desktop | 3 | run1: 32 pass, 1 fail; run2, run3: 33/33, signature 845d50fe631f |
| chromium-390 | 1 | 33/33 pass, signature 845d50fe631f |
| firefox-390 | 1 | 33/33 pass, signature 845d50fe631f |
Versus round 2: the Approve failure (management.live:112) is GONE in every one of 8 browser runs (CSRF fix effective on plain-http LAN); the mp3 flake (playback.live:104) did not recur (8/8 pass). 7 of 8 runs are fully green; 0 skipped (round 2 had a collateral skip).

## Failure classification
1. firefox-desktop run1: pages.live.spec:6 "Favorites page renders without the error boundary" timed out (120 s) in headerReady (support.ts:89): page snapshot shows the empty /login form, i.e. after uiAdminLogin (login POST 200 observed) the SPA was showing the login screen and no header link ever appeared. CLASSIFICATION: intermittent, root cause UNCONFIRMED (1 of 8 browser runs; not product-defect-proven, not spec-defect-proven). Evidence against a server fault: API log statuses in obs were all 200 for that test; no 429/401 recorded by the test's request fixture (browser-side requests are not in obs). Reproduction attempt: pages.live x3 tests, firefox-desktop, trace on, 6 consecutive runs = 18/18 pass, so not reproduced (diag-pages-firefox; trace zips deleted for size/secret hygiene). Leads, not conclusions: a client-side session drop / redirect to /login in Firefox after login (front-end 401 handler at lib/api.ts:75), or a race between login navigation and the header; needs a trace captured on a failing run (run with --trace retain-on-failure over repeated firefox runs) to decide. Not fixed.
2. No other failures. Evidence recorder (make_evidence) not on main: skipped as instructed.

## Defects
None confirmed product defect this round; one unconfirmed Firefox intermittent (above).

## Stack 3 URL/port table (podman pod catqa3, all Up ~50 min, pasta network)
| service | URL | note |
|---|---|---|
| catalog-web (nginx SPA, /api + /ws proxy) | http://192.168.1.115:33020 | 0.0.0.0 |
| catalog-api | http://192.168.1.115:38082 (/health) | 0.0.0.0 |
| Website (VitePress) | http://192.168.1.115:33021 | 0.0.0.0 |
| PostgreSQL 15 | 127.0.0.1:35434 | loopback only |
| Redis 7 | 127.0.0.1:36381 | loopback only |
podman ps --filter pod=catqa3: catqa3-postgres (healthy), catqa3-redis (healthy), catqa3-api (healthy), catqa3-web, catqa3-website, plus pod infra e3c495ce1bdb. Stacks catqa1/catqa2 and the shared aardvark-dns untouched.

## Bundle
/home/milosvasic/.cache/claude-tmp/live-web-stack3-r3/ (4.1 MB): runs/<project>/runN/{report.json,console.log,obs,artifacts (failure error-context.md)}, runs/runs.log, diag-pages-firefox, machine-fields (target.txt, step2_probe.json, step2_csrf.json, screenshot_manifest.txt = sha256+size of the 530 pass-run screenshots that were removed for size), driver scripts. Leak check: admin password, JWT secret and Postgres password strings grep over the bundle = 0 hits. Traces were deleted (not published).
