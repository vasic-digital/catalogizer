# UI D7 class-wide dark-mode contrast (D7-R2-F1..F4) - 2026-11-05

Status: PROVISIONAL (unit-class, jsdom + real Tailwind theme; round 2 after reviewer NO-GO). No live-stack axe re-run was done here.
is owed to the conductor after the next stack rebuild.

## Defect
Static class strings with `text-gray-500/600` (also `text-gray-900/700`, `text-orange-600`, `text-blue-600`, status pairs)
and no `dark:text-*` variant on a dark surface (Card = `dark:bg-gray-800`, page = `gray-900`). The fix is decided by the REAL
ancestor background, not by blindly adding `dark:` classes: a panel that is `bg-white` with no `dark:bg-*` stays light in dark
mode (adding `dark:text-gray-400` there would give 2.54:1 - the helper flags that case; golden in `contrastHelper.test.ts`).

## Method
- `src/test/contrast.ts`: (F3) throws on two different colour classes of the same prefix+variant on one element; `bg-transparent`/`text-current` inherit;
  new exported whole-element walker `failingElements` (F2) (skips svg, WCAG 1.4.3-exempt `[disabled]`, bg-black/ overlays; large text 3:1).
- `contrastD7Shared.test.tsx` and `contrastD7Remaining.test.tsx` use the shared walker; new `contrastD7Class.test.tsx` renders
  ExternalIntegrations, AdvancedSearch (all tabs), CollectionAutomation, UploadManager, FormatConverter, EntityDetailView pieces, AI panels, Settings (3 tabs).
- (F4) CollectionsManager view toggles got `aria-label` "Grid view"/"List view"; the test finds them by role+name, not by button index.

## Before / after (WCAG ratios, from RED_prefix_source.log -> GREEN)
| element | before (dark) | after (dark) |
|---|---|---|
| MediaDistributionChart "No media data available" (gray-500 on gray-800/900) | 3.04 | 5.78 (gray-400/800) |
| CollectionsManager "4 items" / "1m" (gray-500) | 3.04 | 5.78 |
| CollectionsManager description (gray-600) | 1.94 | 9.96 (gray-300/800) |
| CollectionsManager title h3 (gray-900) | 1.21 | 14.68 (white/800) |
| EntityDetailView Files/Children/Duplicates labels (gray-500) | 3.04 | 5.78 |
| UploadManager hint (gray-500) | 3.04 | 5.78/6.99 |
| FormatConverter empty text (gray-600) | 1.94 | 9.96/12.04 |
| ExternalIntegrations stats/labels (white card, no dark bg) | text 1.00-3.67 | card gets `dark:bg-gray-800`, labels 5.78 |
| CollectionRealTime error badge (red-600 on red-100, LIGHT mode) | 3.95 | 5.30 (red-700); green 3.00->4.57, yellow 2.74->6.38 |
| EntityDetail "0 files available" / "(2003)" / "ID: 603" | 3.04 | 5.78 |
| EntityDetailView orange duplicates count (orange-600) | 3.56 light / 4.12 dark | 5.18 (orange-700) / 6.49 (orange-400) |
| AdvancedSearch active tab (blue-600 on gray-800) | 2.84 | 5.77 (blue-400) |
Light-mode ratios are unchanged except the three status-colour fixes above (all >= 4.5). Full list of measured RED elements: `RED_measured_ratios.txt`.

## Sites
- 12 production files edited for the class (+ MediaFilters/Playlists/EntityDetail single sites): ~138 insertions of `dark:` classes
  (57 `dark:text-gray-400`, 38 `dark:text-gray-300`, 21 `dark:text-white`, 3 `dark:text-blue-400`, 2 `dark:text-orange-400`, status pairs, 14 surface `dark:bg-*`).
- Sites deliberately NOT changed (real ancestor is light or the element is non-text/exempt): AI*.tsx reason lines and MediaDistributionChart tooltip (bg-white panels,
  gray-500 on white = 4.83), CollectionAutomation disabled "Create Rule" (WCAG-exempt), ActivityFeed badge (gray-600 on gray-100), icons only (Dashboard, EntityCard,
  DiscoveredShares, CollectionPreview, Playlists:404; svg, non-text 3:1 holds: >= 3.04), hover-only/icon-only (Header, PerformanceOptimizer:422, FavoriteToggle).

## Evidence
RED (fixed source reverted via reverse patch, tests kept): `RED_prefix_source.log` = 14 failed / 33 passed of 47 across contrastD7Class/Shared/Remaining/Helper.
GREEN: `GREEN_affected.log` (53 files, 865 tests, styles + collections/upload/conversion/entity/dashboard/ai + pages), `GREEN_final.log` (17 files, 327 tests incl. media + EntityDetail), `tsc.log` rc=0.
Reviewer-style mutations (`mutations_summary.txt`, `mut_*.log`): 13 of 14 killed. M10 (walker skips every element) SURVIVED the component tests (vacuity), so walker goldens were added to
`contrastHelper.test.ts`; re-run as M10b it is killed (3 failed), as is M13 (threshold 0). Additional manual mutation: EntityDetail "ID:" dark class reverted -> `contrastD7Shared` fails (3.04).
Full `vitest run` on the scratch clone: 2485 passed / 14 failed in 8 files (Avatar, EmptyState, LoadingSpinner, AuthContext.integration, WebSocketContext x2, module-registry, websocket)
- none touch the edited files; causes seen: unresolved `@vasic-digital/*` imports (submodules not initialised in the scratch clone) and localStorage assertions. UNCONFIRMED: not re-run against pristine main.
Digests: `SHA256SUMS`.

## Round 2 (reviewer NO-GO F1/F2 + minors)
Correction of round-1 claims: the round-1 `contrastD7Class` ExternalIntegrations test mocked `fetch` as `ok:false` (no cards / expanded panel / modal) and the
UploadManager test never queued a file, so "whole panel" / "with a queued file" overstated coverage. The new `contrastD7Populated.test.tsx` (reviewer probe adopted, with
assertions) renders: ExternalIntegrations cards (connected + pending), expanded panel, add modal; UploadManager drag-over zone and queued file; FormatConverter selected preset.
RED on the round-1 source: `RED_populated.log` (6 failed): queued/drag-over 1.35/2.33, pending badge 2.43, connected badge 2.18 (light AND dark), hint 4.44 on blue-50 (light).
Fixes: light surfaces got dark surfaces (UploadManager row `dark:bg-gray-800`, drag-over `dark:bg-blue-900/20`; ExternalIntegrations expanded panel `dark:bg-gray-900` with inner boxes
`dark:bg-gray-800` and code chips `dark:bg-gray-700`; FormatConverter selected preset `dark:bg-blue-900/20`; status badges: green/red/blue-500 -> -700 + `dark:*-400` on `dark:bg-*-900/20`,
default gray `gray-600/gray-300` on `bg-gray-50 dark:bg-gray-800`). Light-mode changes (all improvements, listed honestly): status badge text 500->700 (2.18 -> >=4.5), drag-over hint
gray-500 -> gray-600 on blue-50 (4.44 -> 6.0), EntityDetailView duplicates count orange-600 -> orange-700 (3.56 -> 5.18), CollectionRealTime badges (round 1).
PerformanceOptimizer:422 is a text "x" button (not an icon): gray-500 -> gray-400 (hover gray-200) on its dark overlay (3.67 -> ~6.7). Not render-tested (UNCONFIRMED by render).
Walker minors: overlay skip anchored `(^|\s)bg-black/` plus `bg-black bg-opacity-N`; goldens added killing R5 (skip elements with own bg-*), R6 (`text-lg` counted large), R7 (unanchored overlay).
Mutations re-run (`mutations_summary.txt`): M1-M13, P1-P6 (new surfaces/badge), R5-R7 - all killed (each >=1 failing test); M10/M13 killed via helper goldens.
GREEN: `GREEN_affected2.log` 61 files / 1070 tests passed (styles, collections, upload, conversion, entity, dashboard, ai, media, pages), `tsc2.log` rc=0.
Follow-up registered (not fixed): D7-R3-FU1 FormatConverter job error/success boxes (`bg-red-50`/`bg-green-50`, text -700) are light islands in dark mode (readable, 6+:1, but visually inconsistent): add `dark:bg-*-900/20 dark:text-*-300`.
