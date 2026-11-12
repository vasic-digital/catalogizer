# SonarQube run5 - Catalogizer main 563617b1fe04ec9310adb8b1795264caf955f53b (PROVISIONAL)

PROVISIONAL: static analysis only. A closed finding means Sonar no longer reports it, not that the product works (11.4.289). Same procedure and same SonarQube instance/volumes as run4 (`../run4/README.md`), so issue keys are stable. Scanned from a clean `git clone --no-hardlinks` checkout of main (no submodules initialised). `up.sh` with `QA_STACK_BACKEND=containers`, `scan.sh` with `SQ_LOAD_TIMEOUT=3600` under `systemd-run --user --scope -p MemoryMax=4G -p MemorySwapMax=0`, `nice -n 10`, `GOMAXPROCS=2`, then `down.sh` (0 catsonar containers left). Disk 98% used / 42 GB free; no guard was overridden. Scrub control needle PASS (19 files).

## Counts (open) run4 -> run5
| project | gate run4 -> run5 | open | smells | bugs | vulns | BLOCKER/CRITICAL | MAJOR | MINOR | hotspots |
|---|---|---|---|---|---|---|---|---|---|
| api | OK -> OK | 687 -> 687 | 687 | 0 | 0 | 0 / 648 | 28 | 11 | 0 |
| web | ERROR -> OK | 351 -> 338 | 338 | 0 | 0 | 0 / 0 | 197 | 141 | 0 |

Key-level diff vs run4: api 0 gone / 0 new / 687 still; web 13 gone / 0 new / 338 still. (`reported_total` api 699, web 417 includes closed issues; open set = `exported`.)
Web gate conditions: new_violations 0 (was 13), new_coverage 0.0 (OK in this run; no coverage report is fed in), new_duplicated_lines_density 0.0.

## The 13 web smells from run4: CLEARED (all 13 keys no longer open)
S6819 x8 (AdvancedSearch.tsx x2, CollectionExport, CollectionPreview, CollectionTemplates, DiscoveredShares, HistoryDrawer, SubtitleManager), S3358 x1 + S4323 x1 (FavoriteToggle.tsx), S7773 x3 (lib/collectionRules.ts). No new issue appeared in either project.

## Not changed
api: 648 CRITICAL (338 go:S1192, 310 go:S3776) and the rest unchanged. web: 338 MAJOR/MINOR smells remain, no BLOCKER/CRITICAL/BUG/VULNERABILITY.
