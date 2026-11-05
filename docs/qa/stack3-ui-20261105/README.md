# Stack 3 live browser verification (UI), 2026-11-05

Roadmap-Item: CAT-QA-STACK3-UI. Image main-5c480072 (QA stack 3). Real stack, real browser (headless Chromium via playwright-core), no mocks (section 11.4.289). Web UI via nginx front door http://192.168.1.115:33080 (footer v1.1.0), website http://192.168.1.115:33021, user qa_admin (credential read from the 0600 env file, never printed; evidence redacted). Read-only walk: nothing created or modified. axe-core run per page from a local node_modules copy.

Evidence: `screenshots/` (PNG viewport), `results-app.json`, `results-site.json`, `results-mini.json`, `results-entity.json` (raw per-page console / network / axe / overflow data), `MANIFEST.sha256`.

## Headline

- FAIL: /conversion crashes into the error boundary (TypeError on `.name` of undefined) on every load.
- FAIL: /identities calls GET /api/v1/identities which returns 404 (twice per load); page shows only its shell.
- FAIL: /entity/35 (id taken from the Media page cards): GET /entities/35, /recommendations/similar/35, /entities/35/duplicates all 404, page sits on an infinite spinner (no error state).
- FAIL: 360px width, /admin: horizontal overflow (scrollWidth 618 > 360).
- FAIL: WebSocket /ws handshake 101 and subscribe OK, then closed 4 times in about 25s idle (reconnect churn); header shows a red "Disconnected" badge on most pages.
- A11y: critical axe findings on /media (78 buttons without accessible name), /collections, /playlists, /favorites (select-name), /ai (1 button-name); serious color-contrast on every app page.
- VERIFIED (closes the PROVISIONAL item): GET /api/v1/entities/types returns 200 from the Browse page UI call (and 200 via authenticated fetch, body key `types`).

## 1. App pages (viewport 1440x900)

| page | renders real data | status | console errors (excl. COOP note) | network 4xx/5xx | h-overflow | axe violations (rules, nodes) | label |
|---|---|---|---|---|---|---|---|
| login | yes | VERIFIED | 0 | 0 | false | 3 rules (landmark-one-main:1, page-has-heading-one:1, region:8) | VERIFIED |
| dashboard | yes (stats, welcome qa_admin) | VERIFIED | 0 | 0 | false | 4 rules (color-contrast:2, heading-order:1, region:1, svg-img-alt:6) | VERIFIED |
| media | yes (25 movie cards; per-card favorites/check + progress calls) | VERIFIED | 0 | 0 | false | 4 rules (button-name:78, color-contrast:34, heading-order:1, region:1) | VERIFIED (renders; a11y critical noted) |
| browse | yes (20 entities by type) | VERIFIED | 0 | 0 | false | 3 rules (color-contrast:1, heading-order:1, region:1) | VERIFIED |
| collections | yes (2 items) | VERIFIED | 0 | 0 | false | 5 rules (button-name:2, color-contrast:1, heading-order:1, region:1, select-name:2) | VERIFIED (renders; a11y critical noted) |
| favorites | yes | VERIFIED | 0 | 0 | false | 4 rules (color-contrast:1, heading-order:1, region:1, select-name:2) | VERIFIED (renders; a11y critical noted) |
| playlists | yes | VERIFIED | 0 | 0 | false | 5 rules (button-name:2, color-contrast:1, landmark-unique:1, region:1, select-name:4) | VERIFIED (renders; a11y critical noted) |
| subtitles | yes (languages+providers 200) | VERIFIED | 0 | 0 | false | 2 rules (color-contrast:1, region:1) | VERIFIED |
| analytics | yes (Total Media Items 47) | VERIFIED | 0 | 0 | false | 4 rules (color-contrast:1, heading-order:1, region:1, svg-img-alt:6) | VERIFIED |
| conversion | NO - error boundary | FAIL | 2 | 0 | false | 3 rules (color-contrast:1, page-has-heading-one:1, region:1) | FAIL |
| admin | yes (users/storage/backups/system-info 200) | VERIFIED | 0 | 0 | false | 3 rules (color-contrast:1, heading-order:1, region:1) | VERIFIED |
| settings | yes (scans, storage roots) | VERIFIED | 0 | 0 | false | 3 rules (color-contrast:1, heading-order:1, region:1) | VERIFIED |
| ai | yes (AI dashboard shell) | VERIFIED | 0 | 0 | false | 5 rules (button-name:1, color-contrast:3, heading-order:1, landmark-unique:1, region:1) | VERIFIED (renders; a11y critical noted) |
| identities | shell only, API 404 | FAIL | 2 | 2 | false | 3 rules (color-contrast:1, heading-order:1, region:1) | FAIL |
| search | navigates to /browse?search=a; list shows type counts, filter not visibly applied (UNCONFIRMED) | VERIFIED | 0 | 0 | false | 3 rules (color-contrast:1, heading-order:1, region:1) | VERIFIED |

COOP note: every page logs one console error "Cross-Origin-Opener-Policy header has been ignored, because the URL's origin was untrustworthy", because the QA stack is plain HTTP on a LAN IP. Environmental, LOW, expected over HTTP.

Keyboard: login tab order is username, password, show-password, remember-me, forgot password, create-account link, create-account button (logical). Dashboard tab order: brand link, nav links, More, search, theme toggle (logical). No skip-link was observed.

## 2. Responsive (360x800 / 768x1024 / 1440x900)

| page | 360 overflow | 768 overflow | 1440 overflow | notes |
|---|---|---|---|---|
| login | no | no | no |  |
| dashboard | no | no | no |  |
| media | no | no | no |  |
| browse | no | no | no |  |
| admin | YES 618>360 | no | no | 360: stat-card row right edge 618px |
| settings | no | no | no |  |

At 360 and 768 the top nav collapses to a More menu; the overflow probe found no clipped or overflowing elements except the /admin stat cards. Overlap or clipping beyond overflow was judged from screenshots only (UNCONFIRMED at pixel level).

## 3. Accessibility (axe-core)

Aggregate node counts per rule across all app page runs (all sizes): landmark-one-main (moderate) 3; page-has-heading-one (moderate) 4; region (moderate) 48; color-contrast (serious) 123; heading-order (moderate) 21; svg-img-alt (serious) 24; button-name (critical) 239; select-name (critical) 8; landmark-unique (moderate) 2.

Dark theme: dashboard color-contrast:serious:14, heading-order:moderate:1, region:moderate:1, svg-img-alt:serious:6; media button-name:critical:78, color-contrast:serious:11, heading-order:moderate:1, region:moderate:1.

Key defects: /media 78 icon buttons with no accessible name (critical); unlabeled select elements on /collections, /favorites, /playlists (critical); /ai one unnamed button; 6 svg-img-alt on dashboard/analytics charts; color-contrast on all pages; heading-order everywhere; missing main landmark / h1 on login and conversion.

## 4. Website (http://192.168.1.115:33021, VitePress)

31 internal pages crawled; all 31 internal URLs return 200 (0 broken); console errors 0 and network failures 0 at all three sizes; no horizontal overflow at 360/768/1440. External links found in docs: https://github.com/vasic-digital/Catalogizer, http://localhost:8080/, http://localhost:9090/, http://localhost:3001/, https://rustup.rs/, https://tauri.app/v1/guides/getting-started/prerequisites (not fetched; localhost entries are documentation examples).

Axe: color-contrast (serious) on download, support, getting-started, platforms, guides, developer docs (up to 53 nodes on /developer/testing.html), 1 on home; landmark-one-main and region on home. No critical findings. Theme toggle (VitePress switch): {"before":"false|rgb(255, 255, 255)","after":"true|rgb(27, 27, 31)"} -> VERIFIED. Per-page rows (url, size, overflow, axe) are in results-site.json.

## 5. Theme toggle (app)

Present (button "Switch to dark theme"): {"before":"|rgb(255, 255, 255)","after":"dark|rgb(2, 8, 23)"}, persisted after navigation (html class "dark"), localStorage key catalog-web-theme. VERIFIED. Dark axe results are in section 3.

## 6. Other observations

- Fonts load from fonts.googleapis.com / gstatic (external dependency; offline installs fall back).
- /media issues about 25 favorites/check and 25 progress requests (N+1) per load, all 200 (LOW, performance).
- Subtitles page renders (languages and providers 200). The subtitle panel inside the player was NOT reached: the Movie id tested (35) 404s, so UNCONFIRMED. Entities 3 (Music Album) and 4 (Comic) render with Files, Play, Download, Favorite: VERIFIED.
- Full API inventory per size is in results-app.json (api_desktop / api_tablet / api_mobile; 124 / 90 / 90 calls); the only 4xx are the /identities 404s. The entity-page 404s are in results-entity.json.

## Defects (not fixed, per scope)

| # | severity | defect | repro |
|---|---|---|---|
| D1 | HIGH | /conversion crashes: TypeError reading `name` of undefined in the ConversionTools chunk; ErrorBoundary shown | log in, open /conversion |
| D2 | MEDIUM | /identities: GET /api/v1/identities 404 (x2); page non-functional | log in, open /identities |
| D3 | MEDIUM | Media page lists an id that 404s (/entity/35: entities/35, similar/35, duplicates 404) and the entity page spins forever with no error state; cause (media-item id vs entity id) UNCONFIRMED | open /media, then /entity/35 |
| D4 | MEDIUM | WebSocket /ws closes and reconnects roughly every 6-10s while idle; "Disconnected" badge shown | log in, watch header badge; results-mini.json wsClosed=4 in 25s |
| D5 | MEDIUM | /admin overflows horizontally at 360px (stat card row 618px wide) | viewport 360x800, open /admin |
| D6 | MEDIUM | a11y (axe critical): 78 unnamed buttons on /media; unlabeled selects on /collections, /favorites, /playlists | axe run |
| D7 | LOW | color-contrast (serious) on all app and most website pages; svg-img-alt x6 on dashboard/analytics | axe run |
| D8 | LOW | header search goes to /browse?search=a but results show unfiltered type counts (UNCONFIRMED whether the filter applies) | type "a" in header search, Enter |

## Evidence-storage note (added by conductor, 2026-11-05)
The 62 screenshots are NOT tracked in git: repository policy ignores `docs/qa/**/*.png` (.gitignore). Their hashes are in MANIFEST.screenshots.sha256 (verify with `sha256sum -c` from the archive directory); the files are kept at `/home/milosvasic/.cache/claude-tmp/evidence-archive/stack3-ui-20261105/screenshots/` on the QA host (outside the repo). MANIFEST.sha256 covers the tracked files only.
