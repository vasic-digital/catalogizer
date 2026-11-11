# SonarQube run4 - Catalogizer main tag v2.4.1 (commit 04c6d422b093a8beca5ae6ac0818b0a6ed41957a)

Purpose: measure whether the landed Sonar fixes (CAT-QA-SONAR-1/-3/-4/-6, PROVISIONAL) cleared findings. Baseline: `../../sonarqube-20261105/run3/` (main 0020ad3e).
Procedure: `up.sh` run from the MAIN checkout with `QA_STACK_BACKEND=containers`; `scan.sh` run from a scratch clone checked out at 04c6d422 (clean tree), `OUT=docs/qa/sonarqube-20261111`, `RUN_ID=run4`.
Backend line: `qa_stack: backend=containers ctrctl_sha256=91a0feda6cafc207489b26fdecd6aed4ef0bbc36e61d5df8237b05e2b593f93d`.
Same SonarQube instance/volumes as run3 (analysis history kept), so issue keys are stable and findings are tracked by key: no key of run4 is hidden, 0 api and 13 web issues are NEW. Stack taken down with `down.sh` afterwards (0 catsonar containers left). Scrub control needle PASS.

## Read this first: `reported_total` vs `exported`
`*.issues.json` `reported_total` (api 699, web 417) counts ALL issues incl. CLOSED/FIXED ones (the scanner's unfiltered `ps=1` query); `exported` (687 / 351) is the open set (`resolved=false`). The difference (12 / 66) is exactly the issues Sonar closed as FIXED. Closed ones are exported in `catalog-*.issues_closed.json` (added in this run, queried after the scan). Counts below are OPEN issues (= `issue_facets.json`).

## Counts (open) run3 -> run4
| project | total | smells | bugs | vulns | BLOCKER | CRITICAL | MAJOR | MINOR | hotspots |
|---|---|---|---|---|---|---|---|---|---|
| api | 699 -> 687 | 697 -> 687 | 2 -> 0 | 0 -> 0 | 0 -> 0 | 658 -> 648 | 30 -> 28 | 11 -> 11 | 0 |
| web | 404 -> 351 (66 closed, 13 new) | 384 -> 351 | 15 -> 0 | 5 -> 0 | 16 -> 0 | 12 -> 0 | 219 -> 206 | 157 -> 145 | 0 |

Per-rule run3 -> run4 and the full GONE/STILL lists: `comparison_vs_run3.md`.
Quality gate (Sonar way, new code only): api OK; web ERROR (new_violations 13 > 0, new_coverage 0.0 < 80; no coverage report fed in, so the coverage condition is not meaningful for this scan). The 13 new_violations are the 13 NEW findings below. run3 gate was OK for both.

## Verdict table
| target set | total | GONE | STILL PRESENT | evidence |
|---|---|---|---|---|
| 22 CRITICAL (api 10 go:S1186 + web 12: 8 S3776, 4 S3735) | 22 | 22 | 0 | api s3_client.go:324-343 (6), cover_art_service.go:617, pdf_page_render.go:73, conversion_proc_pdeathsig_other.go:9, encryption.go:524; web 12 in comparison_vs_run3.md |
| api go:S3923 bugs | 2 | 2 | 0 | webdav_client_test.go:594, configuration_wizard_service.go:1179 |
| web typescript:S2245 vulns | 5 | 5 | 0 | |
| web bugs | 15 | 15 | 0 | S1082x9, S6959x2, S8985x2, S3923, S9115 |
| web typescript:S2699 BLOCKER | 16 | 16 | 0 | |
| NEW findings | | | 13 (web only; api 0) | below |

Other closures beyond the targets: web S6848 8->0, S6847 1->0, S1874 5->1, S3358 33->32 (net). All status CLOSED/resolution FIXED.

## STILL PRESENT (not targeted by the fixes, unchanged)
api: 338 go:S1192, 310 go:S3776 (all 648 CRITICAL), 16 S4144, 10 S107, 6 S100 etc - unchanged. web: no BLOCKER/CRITICAL/BUG/VULN remain; 351 MAJOR/MINOR smells.

## NEW in run4 (web, introduced since run3; all code smells, lines as in the scanned tree)
- typescript:S6819 MAJOR: components/collections/CollectionTemplates.tsx:497, AdvancedSearch.tsx:488 and :532, CollectionExport.tsx:723, CollectionPreview.tsx:216, components/identity/DiscoveredShares.tsx:151, components/media/HistoryDrawer.tsx:66, pages/SubtitleManager.tsx:278 (8 new; 1 pre-existing S6819 stays) - all under `catalog-web/src/`
- typescript:S3358 MAJOR: components/favorites/FavoriteToggle.tsx:134
- typescript:S4323 MINOR: components/favorites/FavoriteToggle.tsx:11
- typescript:S7773 MINOR: lib/collectionRules.ts:358, :361, :384
Likely side effects of the test/refactor edits (S6819 = ARIA role on non-native element); that attribution is UNCONFIRMED, not investigated.

## Honest boundaries
Static analysis only; a closed finding means Sonar no longer reports it (code changed), not that the product works (11.4.289). The fixes remain PROVISIONAL-DONE until their own live/test evidence exists; this run shows static clearance only. "GONE" = key status CLOSED/FIXED on the same instance. Disk 98% used / 51 GB free, ES disk threshold disabled as in run3.
