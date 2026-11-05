# CONTINUATION — Catalogizer

> Live work-state document per Constitution **§12.10 (CONTINUATION)** and **§11.4.131**.
> A fresh session that receives only the prompt `continue` MUST read this file first, then
> `docs/FOLLOW_UPS_NEXT_PHASE.md`, then resume the §11.4.126 loop with the steps in "RESUME ORDER".

**Revision:** 10
**Last modified:** 2026-11-05T10:15:00Z (11:15 CET; written right before an operator-requested host reboot)
**Exports:** `CONTINUATION.html` / `CONTINUATION.pdf` are STALE (Rev 9, June). Regenerate from this file (owed, §11.4.65); this `.md` is authoritative.

## 1. State at stop

* `main` = `3293e280` on all six upstreams (origin, github-milos, gitlab-milos, gitlab-vasic, gitflic-vasic, gitverse-vasic) before this doc commit. Release target: first tag `v2.4.0` (pre-release, SemVer), only after live validation.
* Everything below the line "NOT ON MAIN" is work in progress, preserved as `wip/*` branches pushed to all six upstreams (vehicles per §11.4.288(B); merge ff-only to `main` after review GO, then delete the `wip/*` branch on every upstream) and also as scratch clones on disk under `/home/milosvasic/.cache/claude-tmp/` (not tmpfs, survives reboot).
* All subagents were stopped for the reboot. Their reviews in flight were lost (re-dispatch, see below). No agent is running after reboot.
* Constitution pin 92a9ecf8 (v73.2). Periodic fetch of the constitution submodule is still owed.
* Binding rules: see memory `feedback_release_completion_mandate`, `feedback_review_budget_policy` (max 5 review rounds, minors -> follow-ups), `feedback_all_work_on_main`, `project_decisions_2026_11_03`. Never force-push; never print credentials (Yandex WebDAV creds only in scratchpad `yandex_webdav.env`, Synology in `syn_test.env`, stack in `catqa3/qa.env`/`mon.env`, all 0600, outside git).

## 2. NOT ON MAIN — branches awaiting review/landing

| `wip/` branch (all upstreams) | Head | Scratch clone | Item | State |
|---|---|---|---|---|
| `wip/secq-3-5-6` | 29abe617 | `secq.EUCL/c` | SECQ-3 pprof sampling cap (all `seconds` routes), SECQ-4 smart-playlist cap race (in-process lock), SECQ-5 GET /smb/discover,/smb/test -> 410 + POST callers, SECQ-6 logout characterization, SECQ-4b registered | Opus round 1 NO-GO, author fixed round 2, **re-review owed** (round 2/5). Also check: `DELETE /api/v1/subtitles/{id}` contract violation — verify it pre-exists on main. |
| `wip/secq1-webdav-ssrf` | 8954028d | `secq1-i0dyet/repo` | SECQ-1 WebDAV storage-root SSRF guard (netguard operator.go; RFC1918/CGNAT/public allowed, loopback/link-local/metadata/mapped forms blocked; strict opt-in `CATALOGIZER_OPERATOR_DENY_PRIVATE=1`) | Opus round 1 NO-GO (proxy bypass etc.), author fixed F1-F9, **re-review owed**; judge residual "direct conn to a previously-used proxy address not destination-checked". |
| `wip/wdf1-webdav-hardening` | e5c33cb5 | `wdf1.w4zBXG/c` | WD-F1/SECQ-2 live reproduction (UNREPRODUCED, 12 forced resets, 0 lost acked writes; WD-F1a same-size CORRUPT anomaly tracked) + defensive WriteFile read-back | Opus round 1 NO-GO, author fixed F1-F7, **re-review owed**. SECQ-2 stays open/UNREPRODUCED. |
| `wip/cat-new1-cover-thumbnails` | 4269dcee | `cat-new1.jsZlPK/repo` | NEW-1: generate-thumbnails silent no-op (scan order, doubled path, missing cache dir, SQLite upsert) | Reproduced live, 4 root causes FACT, **Opus review owed (never completed)**. go vet + full ./internal/services not yet run. Live re-check needs stack 3 API image rebuilt from the fixed head. |
| `wip/qa-stack3-verify-20261105` | 5f5f2795 | `stk3verify.c4XWW8/repo` | Stack 3 rebuild + live re-verify pack, METRICS_SCRAPE_TOKEN, TRUSTED_PROXIES, pprof pass-through | Opus review NO-GO (F1-F7: TRUSTED_PROXIES doc/caveat, authz claim scope, vacuous S5 check, missing test asserts, chown check, evidence nits). Author was mid-fix; HEAD commit is a **WIP snapshot (not review-clean)**. Resume the author on this branch with the review findings (below), then re-review. |

Review findings still to be applied to `wip/qa-stack3-verify-20261105` (the author was given them, may not have finished): F1 TRUSTED_PROXIES host/32 is a single-tenant-QA trade-off (same-host clients can forge X-Forwarded-For; remote-LAN source preservation UNCONFIRMED -> measure from a second LAN host, tracked); F2 relabel authz as vertical-VERIFIED / horizontal-PROVISIONAL, ideally run a live user-A-vs-B check; F3 control for S5 conversion-job check + tighten 401 acceptances; F4 add asserts (default excludes 127.0.0.0/8 and ::1; seed 31 refused/32 accepted; monitoring up.sh feeds token on stdin never `-e`); F5 guard idempotency test with `[ -n "$t2" ]`; F6 check `chown` + revoke/delete stale `/tokens/previous.tok`; F7 evidence nits, independent random test-user password, document token visible in `podman inspect catqa3-api`.

## 3. Verified vs provisional (from the stack 3 live pack, build 015896fd)

VERIFIED live (3 rounds): QA-MON-FU-1 static scrape token, /metrics auth, pprof admin-only, vertical authz on named routes, CSRF-R2, P2-g log redaction, monitoring add-on (45/45), D1-D11 regression (54 PASS/1 SKIP).
PROVISIONAL: SSRF-F1..F5 (unit/real-listener only; cover-art product route blocked by NEW-1), AZ2-3, R4-1 (needs WebDAV), TRUSTED_PROXIES at nginx (same-host forgery measured; remote unmeasured). Contrary evidence reopens items; none reopened so far.

## 4. RESUME ORDER (what "continue" means)

1. `cd /home/milosvasic/Projects/catalogizer && git fetch --all --prune --recurse-submodules=no`; confirm six-upstream parity of `main`; `git status --short --ignore-submodules=all` clean. Check disk (`df -h /` was 99%, ~20 GB free) and load (< ~12) before dispatching.
2. Bring stack 3 back (containers do not survive reboot): `docs/qa/stack3/up.sh` (secrets in `/home/milosvasic/.cache/claude-tmp/catqa3/qa.env`), then `docs/qa/stack3/monitoring/up.sh`. Do NOT touch stacks catqa1/catqa2 or the shared aardvark-dns. Rootless podman only, no sudo. URL map host 192.168.1.115: API :38082, web :33020, website :33021, nginx front door :33080/:33081, Postgres 127.0.0.1:35434, Redis 127.0.0.1:36381, Grafana 127.0.0.1:33030, pgAdmin 127.0.0.1:33050, Prometheus 127.0.0.1:39090 (monitoring logins in `mon.env`). Run `monitoring/verify.py` at most once a minute (API login rate limit).
3. Re-dispatch (Opus reviewers on fresh instances; authors Sonnet in scratch clones, never push): re-review the four branches in section 2; resume the stack3 author. Max 5 rounds/branch; minor non-security -> follow-ups. After GO: land each via `git fetch <clone> <branch>`; verify submodule diff = 0; `git merge --no-ff`; build/vet/targeted tests; commit with trailers `Roadmap-Item:` + `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`; push ff to all six upstreams; delete the `wip/*` branch on every upstream and the scratch clone only after confirmation.
4. After NEW-1 lands: rebuild stack 3 API image from main, re-run live suites incl. cover-art SSRF-F2..F5 through the product path -> promote PROVISIONAL items.
5. Remaining release scope (memory `feedback_release_completion_mandate`): WL13 player follow-ups; live Playwright on rebuilt stack 3 (closes provisional GET /entities/types); responsive/a11y/visual; desktop (Tauri) + installer wizard; Android/Android TV; HelixQA with Vision; performance/security/chaos; N>=3 determinism ledger; SonarQube (operator decision pending); S3-compatible alternative; refresh stacks 1/2 to current main; LV3-1..5; D5 bursty-audio fixture; Synology idle-deadline/cut-recovery live; PG last-admin race; key-rotation waiver as operator-accepted open risk (no expiry; conflicts with §11.4.271, never invent a date); `versions.json` 2.3.0 -> 2.4.0 + changelog (VERIFIED / PROVISIONAL / REOPENED / OVERDUE listed separately); test GITHUB_TOKEN / GitLab release permission with a real create at release time; tag `v2.4.0` on all upstreams only after live validation.
6. Queue: SECQ-4b (CollectionHandler.CreateCollection same count-then-insert race; RED plan in FOLLOW_UPS), WD-F1a, SUB-H1..H7, ST-R4-1..4, LW3-1, WL13-R*, MON-G4/G5/PROV, LE3-B/D, ST-NFS, D8-F1, CAT-D9-SUSPEND, D9-F1, D4-3 FTS5/tsvector, D11-R2-2..5, root-package OpenAPI test failure (TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions, pre-existing), services TestCAT002_Iso_LimitsAreHostAdaptiveAndBounded (host-memory dependent, pre-existing), QA-MON-FU-2, llms_verifier submodule, periodic constitution fetch, 90 owed script guides, SIGINT/SIGTERM slow shutdown.

## 5. Disk prune (operator-approved steps 1-5, partially executed)

Operator approved: PDF + junk, merged scratch clones, unused catqa3 image tags, old scratchpads/helix-install-test/old session scratchpad clones. Script: scratchpad `run_prune.sh` (log `prune.log`), guarded by in-use + clone_ok checks. It was STOPPED by the reboot after removing the 9.7 GB PDF, cat-sqlc4, fixweb*, gap, dbf, cat001, ftp-fix, cat002 (drift.PxZZQr was KEPT by the guard). Remaining: other clones, rev-le.YICleC (orphan servers already killed), 7 catqa3 image tags (`podman rmi localhost/catqa3-{api:main-4c79b7e3,8e8cb9b1,b4d86546,e0cb1b36; web:main-8e8cb9b1,b4d86546,e0cb1b36}`), old session scratchpads, junk dirs. Re-run the script (idempotent, guarded). Go caches and other projects' session dirs (vasic 194 GB, constitution 54 GB) were NOT approved; never touch. Protected: the five scratch clones in section 2, `fst.L10uPf`, `cat-build.fuhReo`, `atm1217rep.8jTn`, `anchor291.GCm2XY`, `amend230F.VarFNo`, `canon-v72`, `cat-recover`, `notes-backup-pre-merge.bundle`. Footgun learned: never `pkill -f <script name>` from the shell that mentions it (self-match).

## 6. Open operator questions

* SonarQube inclusion; tag form (default `v2.4.0` unless told otherwise); whether `undetermined`-style questions on key-rotation waiver need a different gate treatment.
* SECQ-1 was answered "yes if safe" (implemented with LAN allowed by default).
