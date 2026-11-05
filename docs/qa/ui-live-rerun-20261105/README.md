# UI live re-run, stack 3 image main-1db4760f (2026-11-05)
Target: web http://192.168.1.115:33020 (D1 via :33080), API :38082, qa_admin. Headless Chromium shell 1228, Playwright 1.64.1, axe-core 4.10.2
(axe-core installed into a private mktemp dir, source untouched). Each check run 3x, sequentially, `systemd-run --user --scope -p MemoryMax=3G -p MemorySwapMax=0`. Credentials read by name from the 0600 env file, outputs scrubbed.
Raw: `<check>_run{1,2,3}.json/.err`. Determinism = sha256 of normalised result identical across 3 runs (all five: identical). Screenshots (D2/D3) + MANIFEST.sha256 in the archive dir.

| Check | RED reference | After | Verdict |
|---|---|---|---|
| UI-D5/D6 axe (button-name, select-name, svg-img-alt on /media /collections /favorites /playlists /ai /dashboard /analytics) | `ui-d5d6-20261105/red-live-axe.json` (78+2+2+...) | all 7 lists empty | PASS (N=3 identical) |
| UI-D5/D6 /admin @360px | scrollWidth 618 vs 360 | sw=bw=cw=360 | PASS (N=3) |
| UI-D7 axe color-contrast + svg-img-alt, light+dark, 9 pages | `ui-d7d8-20261105/d7_live_red.json` | Layout version label, Dashboard (light+dark) and ActivityFeed: 0 violations; svg-img-alt: 0. Remaining violations: see below | D7-scope PASS; whole-check FAIL (known out-of-scope pages) |
| UI-D8 header search | navigated to `/browse?search=`, no entity query | `/browse?q=zzznomatchzzz`, h1 `Search: "zzznomatchzzz"`, `GET /api/v1/entities?query=zzznomatchzzz`, "No entities found" | PASS (N=3) |
| UI-D1 /conversion | `repro.js` previously crashed | page renders, `/conversion` 200, `/conversion/jobs` 200, no pageerror (only a COOP console warning on plain-HTTP origin) | PASS (N=3) |
| UI-D2 /identities | `GET /identities` 404 | UI calls `/smb/identities` (200, `[]`); page renders "No identities configured", no 4xx | PASS (N=3) |
| UI-D3 /entity/:id | no not-found state | `/entity/35` (API 404): "Entity not found / Back to Browse" at once; `/entity/3` renders | PASS (N=3) |

## Remaining violations (d7_run*.json, identical in all 3 runs)
Known, listed as not fixed in `ui-d7d8-20261105/README.md` (owned by AIDashboard/AdminPanel/MediaCard/Analytics):
- light /ai: 2 (`.bg-green-600` 3.29:1, `.bg-yellow-600` 2.93:1)
- light /media: 24 (MediaCard `.mt-3.text-gray-400.text-xs` 2.53:1)
- dark /analytics: 5 (`text-gray-500 text-xs` on gray-800 3.03:1)
- dark /admin: 23 (tabs 3.43-3.66:1, card text 1.94:1, `.break-all` 1.2:1)
- dark /ai: 8 (tab labels 2.82-3.66:1)
All other page/theme combos (dashboard, browse, subtitles, settings, identities, light analytics/admin, dark media) had zero.

## Caveats
- /admin@360: one tab `BUTTON` has right=405 (inside the horizontally scrolling tab bar); document scrollWidth is 360, so no page overflow.
- UI-D3: on a not-found entity the secondary calls `/entities/35/duplicates` and `/recommendations/similar/35` are still retried (3x each, 404) in the background; the main state is correct.
- /media cards link to no `/entity/` route (0 links), unchanged id-space split noted in D2/D3 README.
- axe is automated; it does not cover everything. Labelled PASS (N>=3 identical, machine evidence) per check above, not VERIFIED-DONE of the items.
- Files uncommitted; `d2d3_live.cjs` is a new script written for this re-run (no prior live D2/D3 script existed).
