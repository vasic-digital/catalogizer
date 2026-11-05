# CONTINUATION — Catalogizer

> Live work-state document per Constitution **§12.10 (CONTINUATION)** and **§11.4.131**.
> A fresh session that receives only the prompt `continue` MUST read this file first, then
> `docs/FOLLOW_UPS_NEXT_PHASE.md`, then resume the §11.4.126 loop with the steps in "RESUME ORDER".

**Revision:** 11
**Last modified:** 2026-11-05T13:30:00Z (14:30 CET; written right before an operator-requested host reboot)
**Exports:** `CONTINUATION.html` / `CONTINUATION.pdf` are STALE (Rev 9). Regenerate from this file (owed, §11.4.65); this `.md` is authoritative.

## 1. State at stop

* `main` = this doc commit on top of `109acde2` (Tauri QA boot merge), on all six upstreams (origin, github-milos, gitlab-milos, gitlab-vasic, gitflic-vasic, gitverse-vasic). All earlier `wip/*` branches (secq, webdav, cat-new1, stack3-verify) are LANDED; only `wip/s3-protocol` remains upstream.
* Release target: first tag `v2.4.0` (pre-release, SemVer), only after live validation. Everything landed this session is **PROVISIONAL-DONE** (§11.4.290) until its live rerun passes N>=3 with machine evidence.
* All subagents were stopped for the reboot (S3 author, live-rerun verifier, D7 author). No agent runs after reboot; re-dispatch (below).
* Constitution pin 92a9ecf8 (v73.2). Periodic fetch of the constitution submodule still owed.
* Binding rules: memory `feedback_release_completion_mandate`, `feedback_review_budget_policy` (max 5 review rounds, minors -> follow-ups; security-adjacent minors still block), `feedback_all_work_on_main`, `project_decisions_2026_11_03`, `feedback_pkill_self_match`. Never force-push; never print credentials (Yandex WebDAV creds only in scratchpad `yandex_webdav.env`, Synology in `syn_test.env`, stack in `catqa3/qa.env`/`mon.env`, all 0600 outside git). No sudo, rootless podman only; do not touch stacks catqa1/catqa2 or shared aardvark-dns.

## 2. NOT ON MAIN

| Branch | Where | Item | State |
|---|---|---|---|
| `wip/s3-protocol` (all 6 upstreams, head b783255c) | scratch clone `/home/milosvasic/.cache/claude-tmp/s3proto.y6Du6d/repo` (branch feat/s3-protocol) | S3 storage protocol (aws-sdk-go-v2; catalog-api/filesystem/s3_client.go, s3_register.go, s3_unit_test.go, settings.go; handlers/scan_handler.go loopback/metadata pre-check) | Commit 5990c988 got Opus review round 1 = NO-GO. Author was applying fixes F1 (per-page timeouts + batched DeleteObjects), F2 mutation-killing tests, F3 copy-source encoding, F4 odd keys, F5 seekable data-loss, F6 scrub, F7 SSRF pre-check generalised to s3, F9. HEAD b783255c is a **WIP snapshot, not review-clean, go vet/tests not confirmed**. Next: resume an author on that branch (finish F1-F9, run `go vet` + `go test ./filesystem ./handlers`), then Opus delta re-review (round 2/5), land on GO. Live S3 evidence PENDING (httptest double is unit-class only): needs a real S3 server (MinIO blocked earlier; try Garage/SeaweedFS/versitygw/Ceph rgw, pull ability unverified). Also: UI/docs for s3 roots, web SUPPORTED_PROTOCOLS, settings_contract.md keys region/addressing, SeekableClient, rename tracker. |

D7 remaining-contrast author (clone `d7b.PybOvV/repo`) had produced NOTHING (clean main). Re-dispatch fresh.

## 3. Landed this session (all PROVISIONAL until live rerun)

UI-D1 (conversion download by job id), UI-D2/D3 (entity detail), UI-D5/D6 (a11y names, admin 360px overflow), UI-D7/D8 (contrast, header search ?q=), Android QA boot (JDK17 container build, emulators; evidence `docs/qa/android-boot-20261105`), Tauri container build + QA boot tooling + desktop login fixes (`109acde2`, GO-with-follow-ups round 2/5; follow-ups TAURI-FU-2..7 in FOLLOW_UPS). Earlier waves: SECQ-1/3/4/5/6, WD-F1 hardening, NEW-1 thumbnails, stack-3 verify pack.

## 4. Stack 3 (containers do NOT survive reboot)

Images built and tagged: `localhost/catqa3-api:main-1db4760f`, `localhost/catqa3-web:main-1db4760f` (include the D1-D8 UI fixes; later commits only touched Android/Tauri tooling and docs, so rebuild only if API/web code changes after 1db4760f). Was deployed with `docs/qa/stack3/up.sh main-1db4760f` (all healthy), then `GRAFANA_BIND=192.168.1.115 PGADMIN_BIND=192.168.1.115 docs/qa/stack3/monitoring/up.sh` exited 6 (Grafana/pgAdmin check at 127.0.0.1 failed = LAN-bound; earlier identical run later showed LAN endpoints healthy — verify with the LAN URLs, not loopback).
URL map (host 192.168.1.115): API :38082 (/health), web :33020, website :33021, nginx front door :33080 (web, /api, /ws) and :33081 (website), Grafana http://192.168.1.115:33030, pgAdmin http://192.168.1.115:33050, Prometheus 127.0.0.1:39090, PostgreSQL 127.0.0.1:35434, Redis 127.0.0.1:36381. Logins: qa_admin (password only in `/home/milosvasic/.cache/claude-tmp/catqa3/qa.env`, login response field `session_token`); monitoring logins in `mon.env`. Run `monitoring/verify.py` at most once a minute. HELIX_PPROF_ENABLED is NOT set on the API (pprof gate UNCONFIRMED until restart with it true). Rotate QA JWT_SECRET/METRICS_SCRAPE_TOKEN at next recreate (a browser agent printed them in its own session).

## 5. RESUME ORDER (what "continue" means)

1. `cd /home/milosvasic/Projects/catalogizer && git fetch --all --prune --recurse-submodules=no`; confirm six-upstream parity of `main`; `git status --short --ignore-submodules=all` clean. Check disk (`df -h /` 98%, ~37 GB free) and load (<~12 before dispatching; hold dispatch while >20).
2. Bring stack 3 up: `docs/qa/stack3/up.sh main-1db4760f`, then monitoring/up.sh with the LAN binds above. Verify map with curl on the LAN URLs.
3. Live re-verify (dispatch a Sonnet verifier; evidence to `docs/qa/ui-live-rerun-20261105/`, screenshots to `/home/milosvasic/.cache/claude-tmp/evidence-archive/`): `node docs/qa/ui-d5d6-20261105/axe_live.cjs` (env NODE_MODULES_DIR=/home/milosvasic/.cache/claude-tmp/ui-d1.nNlTmm/pw/node_modules, CHROMIUM_PATH), `docs/qa/ui-d7d8-20261105/d7_axe.cjs` + `d8_repro.cjs` (env NM, CHROME), UI-D1 repro `/home/milosvasic/.cache/claude-tmp/ui-d1.nNlTmm/pw/repro.js`, D2/D3 pages. N>=3 identical runs -> promote PROVISIONAL to VERIFIED; contrary evidence reopens. Restart API with HELIX_PPROF_ENABLED=true for pprof 401/403 + 60 s cap; cover-art SSRF-F2..F5 via product path; TRUSTED_PROXIES from a second LAN host.
4. Re-dispatch authors (Sonnet, scratch clone via `git clone --no-hardlinks`, rules at scratchpad `author_rules.md`, never push) then Opus reviewers (max 5 rounds): (a) S3 finish + review (section 2); (b) D7 remaining contrast: /ai badges+tabs, /admin dark (23 nodes), /media card text, /analytics dark labels (Analytics.tsx ~381-412), RED-first with `catalog-web/src/test/contrast.ts`; (c) HelixQA wiring against stack 3 (vision provider needed; do not edit the submodule); (d) SonarQube (docker-compose.security.yml; operator decision pending); (e) S3 test-server live evidence; (f) Tauri: rebuild desktop with login fix, build wizard (`TAURI_BUILD_JOBS=2 scripts/tauri/build.sh <out>` once load <20), boot `QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop` (noVNC :6080), fix `starts_with` SSRF in main.rs (TAURI-FU-5, security-adjacent, RED-first); (g) Android real login via secret-safe path + LAN display access.
5. Landing procedure: `git fetch -q --recurse-submodules=no <clone> <branch>`; `git diff --raw HEAD...FETCH_HEAD -- submodules constitution | wc -l` must be 0; `git merge --no-ff --no-commit FETCH_HEAD`; append follow-ups to docs/FOLLOW_UPS_NEXT_PHASE.md; commit with `Roadmap-Item:` + `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`; push SERIALLY to six upstreams with `sleep 4` (parallel push fails "failed to lock"); verify `git ls-remote <r> refs/heads/main` parity; delete the wip branch on every upstream after landing.
6. Remaining release scope (memory `feedback_release_completion_mandate`): WL13 player follow-ups + subtitles panel; responsive/a11y live matrix across devices/browsers; desktop+installer wizard; Android/Android TV; HelixQA with Vision; performance/security/chaos; N>=3 determinism ledger; S3 alternative; retire stacks catqa1/catqa2 (docs note only); LV3-1..5; D5 bursty-audio fixture; Synology idle-deadline/cut-recovery live; PostgreSQL last-admin race; key-rotation waiver = operator-accepted open risk, no expiry, never invent a date; `versions.json` 2.3.0 -> 2.4.0 + changelog (VERIFIED / PROVISIONAL / REOPENED / OVERDUE separately); test GITHUB_TOKEN / GitLab release permission with a real create at release time; tag `v2.4.0` on all upstreams only after live validation.
7. Queue: SECQ-2 (WD-F1/WD-F1a, UNREPRODUCED), SECQ-4b, SECQ-7, SECQ-8, SUB-H1..H7, ST-R4-1..4, LW3-1, WL13-R*, MON-G4/G5/PROV, QA-MON-FU-2/3/4, LE3-B/D, ST-NFS, D8-F1, CAT-D9-SUSPEND, D9-F1, D4-3 FTS5/tsvector, UI-D4 (UNREPRODUCED), UI-FAV-1, MP-FLAKE-1, WEB-DEPS-1, AND-FU-1..7, S3-FU-1..3, TAURI-FU-1..7, D11-R2-2..5, root-package OpenAPI test failure (pre-existing), services TestCAT002 (host-memory dependent), llms_verifier submodule, waiver gate consumer-data design, trust manifest, atm_id->cat_id rename, llmctl pin bumps, periodic constitution fetch, 90 owed script guides.

## 6. Disk (98%, ~37 GB free)

Operator-approved prune steps (PDF+junk, merged scratch clones, unused catqa3 image tags, old scratchpads) were partly executed; script scratchpad `run_prune.sh` is idempotent and guarded — re-run it. NOT approved: go-build/modcache, Android SDK system images (9.8 GB), podman reclaimable images/volumes, other projects' dirs; ask before pruning those. Never `podman system prune`. Scratch under `/home/milosvasic/.cache/claude-tmp/` (survives reboot): s3proto.y6Du6d (KEEP, wip), tauri.yV0oud (landed; deletable with guarded path after confirming main has it), andqa.Lo6bOU (Android, ~7 GB), d7b.PybOvV (empty, deletable), reviewer roots `rvroot`.

## 7. Open operator decisions

SonarQube inclusion; tag form (default `v2.4.0`); pruning Android/Tauri/podman artifacts and go caches; exposing Prometheus/PostgreSQL/Redis to LAN (currently no); horizontal-authz / second-LAN-host measurement.
