# CONTINUATION — Catalogizer

> Live work-state document per Constitution **§12.10 (CONTINUATION)** and **§11.4.131**.
> A fresh session that receives only the prompt `continue` MUST read this file first, then
> `docs/FOLLOW_UPS_NEXT_PHASE.md`, then resume the §11.4.126 loop with the steps in "RESUME ORDER".

**Revision:** 13
**Last modified:** 2026-11-05 (queue replaced by the workable-items register)
**Exports:** `CONTINUATION.html` / `CONTINUATION.pdf` are STALE (Rev 9). Regenerate from this file (owed, §11.4.65); this `.md` is authoritative.

## 1. State at stop

* `main` = this doc commit on top of `6ffc4593` (D7 class-wide contrast merge), on all six upstreams (origin, github-milos, gitlab-milos, gitlab-vasic, gitflic-vasic, gitverse-vasic). No `wip/*` branch remains upstream: `wip/s3-protocol` is LANDED.
* Release target: first tag `v2.4.0` (pre-release, SemVer), only after live validation. Everything landed is **PROVISIONAL-DONE** (§11.4.290) until its live rerun passes N>=3 with machine evidence.
* No agent runs at the time of writing; re-dispatch per section 5.
* Constitution pin 92a9ecf8 (v73.2). Periodic fetch of the constitution submodule still owed.
* **Synology drives were powered OFF by the operator on 2026-11-05 19:09 CET.** Synology SMB/FTP/NFS live tests = SKIP `hardware_not_present` until the operator says otherwise (never substitute a double).
* Binding rules: memory `feedback_release_completion_mandate`, `feedback_review_budget_policy` (max 5 review rounds, minors -> follow-ups; security-adjacent minors still block), `feedback_all_work_on_main`, `project_decisions_2026_11_03`, `project_decisions_2026_11_05`, `feedback_pkill_self_match`. Never force-push; never print credentials (Yandex WebDAV creds only in scratchpad `yandex_webdav.env`, Synology in `syn_test.env`, stack in `catqa3/qa.env`/`mon.env`, all 0600 outside git). No sudo, rootless podman only; do not touch stacks catqa1/catqa2 or shared aardvark-dns.

## 2. NOT ON MAIN

| Branch | Where | Item | State |
|---|---|---|---|
| `feat/sonarqube-qa` HEAD `734a5ee6` (local only, not pushed) | scratch clone `/home/milosvasic/.cache/claude-tmp/sonarqa.FG8Tji/repo` | SonarQube on-demand QA service (minimal tokens) | BLOCKED: the pinned sonarqube/scanner images are gone locally and the Docker Hub unauthenticated pull rate limit was hit (needs login, a mirror, or waiting). Evidence `results run1/run2` in that tree is the OLD first run. Tracked as `CAT-QA-SONAR-1` (register). Owed: README F7 fixes, a fresh run once images are available, Opus review (max 5 rounds), then land. |

## 3. Landed (all PROVISIONAL-DONE until live rerun)

Since Rev 11 (`109acde2`): D7 remaining contrast `c78c654a`; HelixQA stack-3 API smoke wrapper `fc1192fe` (UI vision leg not implemented, honest SKIP); Tauri SSRF hardening `4ccead68` (TAURI-FU-5); S3 storage protocol `83a21e38` (unit-class); S3 live evidence on real Garage `0a4fb76a` (N=3 live); Tauri backend-enforced confirmation on server URL change `865287f4` (needs native boot); D7 class-wide dark-gray contrast `6ffc4593`. Earlier: UI-D1/D2/D3/D5/D6/D7/D8, Android QA boot (`docs/qa/android-boot-20261105`), Tauri container build + QA boot tooling (`109acde2`), SECQ-1/3/4/5/6, WD-F1, NEW-1, stack-3 verify pack. Follow-ups are in `docs/FOLLOW_UPS_NEXT_PHASE.md` (the authoritative pending list).

## 4. Stack 3 (containers do NOT survive reboot)

**Stack 3 containers are DOWN.** Existing images `localhost/catqa3-api:main-1db4760f`, `localhost/catqa3-web:main-1db4760f` predate the D7 fixes: the **web image MUST be rebuilt from current `main`** (D7 contrast fixes) before the live axe rerun (N>=3); the API image only if API code changed (S3 protocol is in `catalog-api`, so rebuild it too for S3 coverage). Was deployed with `docs/qa/stack3/up.sh main-1db4760f` (all healthy), then `GRAFANA_BIND=192.168.1.115 PGADMIN_BIND=192.168.1.115 docs/qa/stack3/monitoring/up.sh` (needs LAN binds, add the Prometheus LAN bind per the 2026-11-05 decision) previously exited 6 (Grafana/pgAdmin check at 127.0.0.1 failed = LAN-bound; earlier identical run later showed LAN endpoints healthy — verify with the LAN URLs, not loopback).
URL map (host 192.168.1.115): API :38082 (/health), web :33020, website :33021, nginx front door :33080 (web, /api, /ws) and :33081 (website), Grafana http://192.168.1.115:33030, pgAdmin http://192.168.1.115:33050, Prometheus (currently 127.0.0.1:39090; operator decision 2026-11-05: expose LAN-only at http://192.168.1.115:39090 behind the scrape token, NOT YET APPLIED; PostgreSQL 127.0.0.1:35434 and Redis 127.0.0.1:36381 STAY loopback). Logins: qa_admin (password only in `/home/milosvasic/.cache/claude-tmp/catqa3/qa.env`, login response field `session_token`); monitoring logins in `mon.env`. Run `monitoring/verify.py` at most once a minute. HELIX_PPROF_ENABLED is NOT set on the API (pprof gate UNCONFIRMED until restart with it true). Rotate QA JWT_SECRET/METRICS_SCRAPE_TOKEN at next recreate (a browser agent printed them in its own session).

## 5. RESUME ORDER (what "continue" means)

1. `cd /home/milosvasic/Projects/catalogizer && git fetch --all --prune --recurse-submodules=no`; confirm six-upstream parity of `main`; `git status --short --ignore-submodules=all` clean. Check disk and load (<~12 before dispatching; hold dispatch while >20).
2. Rebuild the web (and API) images from `main`, then `docs/qa/stack3/up.sh <new-tag>` and `docs/qa/stack3/monitoring/up.sh` with LAN binds. Verify the URL map with curl on the LAN URLs.
3. Live re-verify (dispatch a Sonnet verifier; evidence to `docs/qa/ui-live-rerun-20261105/`, screenshots to `/home/milosvasic/.cache/claude-tmp/evidence-archive/`): `node docs/qa/ui-d5d6-20261105/axe_live.cjs` (env NODE_MODULES_DIR=/home/milosvasic/.cache/claude-tmp/ui-d1.nNlTmm/pw/node_modules, CHROMIUM_PATH), `docs/qa/ui-d7d8-20261105/d7_axe.cjs` + `d8_repro.cjs`, UI-D1 repro `/home/milosvasic/.cache/claude-tmp/ui-d1.nNlTmm/pw/repro.js`, D2/D3 pages. N>=3 identical runs -> promote PROVISIONAL to VERIFIED; contrary evidence reopens. Restart API with HELIX_PPROF_ENABLED=true for pprof 401/403 + 60 s cap; cover-art SSRF-F2..F5; TRUSTED_PROXIES from a second LAN host.
4. Open work items (Sonnet authors in scratch clones via `git clone --no-hardlinks`, then Opus reviewers, max 5 rounds): (a) SonarQube (section 2; unblock image pull first); (b) Vision/HelixQA UI leg: stop `llmctl-onnx@decide-nli` ONLY via `llmctl stop`/`start` (never `enable`), CPU-capped, do not harm other sessions; (c) Tauri native boot of the confirm-on-URL-change and SSRF changes (`TAURI_BUILD_JOBS=2 scripts/tauri/build.sh <out>`, `QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop`); (d) Android real login via secret-safe path; (e) S3 UI/docs/web SUPPORTED_PROTOCOLS follow-ups (S3-FU); (f) throwaway pre-release tag test to prove GitHub/GitLab release permission (then delete the test tag everywhere).
5. Landing procedure: `git fetch -q --recurse-submodules=no <clone> <branch>`; `git diff --raw HEAD...FETCH_HEAD -- submodules constitution | wc -l` must be 0; `git merge --no-ff --no-commit FETCH_HEAD`; append follow-ups to docs/FOLLOW_UPS_NEXT_PHASE.md; commit with `Roadmap-Item:` + `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`; push SERIALLY to six upstreams with `sleep 4` (parallel push fails "failed to lock"); verify `git ls-remote <r> refs/heads/main` parity; delete any wip branch on every upstream after landing.
6. Remaining release scope (memory `feedback_release_completion_mandate`): WL13 player follow-ups + subtitles panel; responsive/a11y live matrix across devices/browsers; desktop+installer wizard; Android/Android TV; HelixQA with Vision; performance/security/chaos; N>=3 determinism ledger; retire stacks catqa1/catqa2 (docs note only); LV3-1..5; D5 bursty-audio fixture; Synology idle-deadline/cut-recovery live and Synology SMB/FTP/NFS live tests (SKIP `hardware_not_present` while the drives are OFF); PostgreSQL last-admin race; key-rotation waiver = operator-accepted open risk, no expiry, never invent a date; `versions.json` 2.3.0 -> 2.4.0 + changelog (VERIFIED / PROVISIONAL / REOPENED / OVERDUE separately); test GITHUB_TOKEN / GitLab release permission with a real create at release time; tag `v2.4.0` on all upstreams only after live validation.
7. Queue: every open item is a registered workable item in `docs/WORKABLE_ITEMS_REGISTER.md` (Part A = existing follow-up ids with pointers into `docs/FOLLOW_UPS_NEXT_PHASE.md`; Part B = items registered at Rev 13). Do not keep a parallel list here.

| Group | Item ids (see register) |
|---|---|
| Verification (PROVISIONAL until live) | CAT-UI-AXE-LIVE-1, CAT-STACK3-REBUILD-1, CAT-PROM-LAN-1, CAT-PPROF-LIVE-1, CAT-DETERMINISM-LEDGER-1, CAT-TAURI-NATIVE-1 |
| Blocked | CAT-QA-SONAR-1 (Docker Hub rate limit), CAT-SYN-LIVE-1 + ST-NFS (drives OFF, SKIP hardware_not_present), CAT-HELIXQA-VISION-1 (host capacity) |
| Features / coverage | CAT-TAURI-INSTALLER-1, CAT-ANDROID-LOGIN-1, CAT-S3-LIVE-2, CAT-RESPONSIVE-A11Y-MATRIX-1, CAT-PERF-CHAOS-1, CAT-WL13-PLAYER-1, CAT-D5-BURSTY-AUDIO-1, CAT-PG-LASTADMIN-1, CAT-QA-MON-FU-2, CAT-STACK-RETIRE-1 |
| Release | CAT-REL-V240-1, CAT-REL-PERM-1, CAT-KEYROT-WAIVER-1 (operator-accepted open risk, no expiry) |
| Governance | CAT-WI-ENGINE-1, CAT-CONST-FETCH-1, CAT-LLMS-VERIFIER-1, CAT-WAIVER-GATE-DESIGN-1, CAT-LLMCTL-PINS-1, CAT-TEST-PREEXIST-1, CAT-DOCS-EXPORT-1 |
| Existing follow-up ids | SECQ-2/4b/7/8, SUB-H1..H7, ST-R4-1..4, LW3-1, WL13-R*, MON-G4/G5/PROV, QA-MON-FU-3/4, LE3-B/D, D8-F1, CAT-D9-SUSPEND, D9-F1, D4-3, UI-D4, UI-FAV-1, MP-FLAKE-1, WEB-DEPS-1, AND-FU-1..7, S3-FU-1..3, TAURI-FU-1..7, D11-R2-2..5, D7-R3-FU1/M1/M3, LV3-1..5 |

## 6. Disk

Operator decision 2026-11-05: prune APPROVED for Go build/module caches, Android SDK system images, and NAMED unused podman images/volumes. NEVER a blanket prune (`podman system prune` is forbidden); other projects' dirs and anything not named stay untouched. Re-run the idempotent guarded scratchpad `run_prune.sh` for the earlier approved set. Scratch under `/home/milosvasic/.cache/claude-tmp/` (survives reboot): `sonarqa.FG8Tji` (KEEP, unlanded SonarQube work), `s3proto.y6Du6d` and `tauri.yV0oud` (landed; deletable with guarded `${D:?}` paths after confirming main has them), `andqa.Lo6bOU` (Android, ~7 GB), `d7b.PybOvV` (empty), reviewer roots `rvroot`.

## 7. Open operator decisions

Decided 2026-11-05: SonarQube = on-demand QA service with minimal tokens; prune scope (section 6); Prometheus LAN-only; live S3 on Garage; throwaway pre-release tag test; Tauri confirm on URL change (done). Still open: tag form (default `v2.4.0`); horizontal-authz / second-LAN-host measurement; when the Synology drives are powered back on; Docker Hub access method for the SonarQube images (login, mirror, or wait).
