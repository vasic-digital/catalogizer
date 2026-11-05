# UI D7 class-wide dark-mode contrast (D7-R2-F1..F4) - 2026-11-05

Status: PROVISIONAL (unit-class, jsdom + real Tailwind theme). No live-stack axe re-run was done here; the live re-check
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
