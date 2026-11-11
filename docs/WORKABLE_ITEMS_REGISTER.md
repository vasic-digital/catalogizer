# Workable items register — Catalogizer

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-05 |
| Status | active |
| Source of truth for | every pending / upcoming / left work item (ids, Type, Status, blocker, acceptance) |
| Companion docs | `docs/CONTINUATION.md` (state + resume order), `docs/FOLLOW_UPS_NEXT_PHASE.md` (per-finding detail) |

## How items are recorded

* Store: this file (index and new items) plus the detail bullets in `docs/FOLLOW_UPS_NEXT_PHASE.md` (grep the id). There is no SQLite workable-items DB in this repo; the constitution engine (`constitution/scripts/workable-items`) is not wired here. Wiring it is item `CAT-WI-ENGINE-1` below.
* Ticket key `CAT` (existing style `CAT-D9-SUSPEND`, `CAT-ST`, `CAT-CSRF-LAN`). Ids of older follow-ups (`SECQ-2`, `D9-F1`, ...) are kept unchanged; they are listed in Part A with a pointer. No new key prefix is introduced.
* Type: closed set Bug / Feature / Task (§11.4.16). Status: Queued, In progress, Blocked (operator), PROVISIONAL-DONE (§11.4.290, carries a `Verification:` line naming the pending live job), Skipped (honest SKIP with reason code). Bugs also carry `Reproduction:` (§11.4.292); UNREPRODUCED is stated, never hidden.

## Part A — existing ids (already recorded in FOLLOW_UPS_NEXT_PHASE.md; not duplicated)

| Id | Type | Status | Where |
|---|---|---|---|
| SECQ-2 / WD-F1 / WD-F1a | Bug | Queued, UNREPRODUCED (possible data loss, cause UNCONFIRMED) | FOLLOW_UPS "Security queue fix", "WD-F1 update" |
| SECQ-4b, SECQ-7, SECQ-8 | Task | Queued | FOLLOW_UPS "Security queue fix", "SECQ-4b landed" |
| SUB-H1..H7 | Task | Queued (test-instrumentation) | FOLLOW_UPS subtitle sections |
| ST-R4-1..4, ST-NFS, ST-D* | Task | Queued; ST-NFS Blocked (operator: NFS export) | FOLLOW_UPS storage sections |
| LW3-1 | Bug | Queued, UNREPRODUCED intermittent | FOLLOW_UPS "Live QA round 3" |
| LE3-B, LE3-D | Task | Queued | FOLLOW_UPS live-evidence sections |
| WL13-R* | Task | Queued | FOLLOW_UPS "WL13 review follow-ups" |
| MON-G4, MON-G5, MON-PROV, QA-MON-FU-3, QA-MON-FU-4 | Task | Queued | FOLLOW_UPS monitoring sections (QA-MON-FU-2 is named in CONTINUATION only; recorded in Part B as `CAT-QA-MON-FU-2`) |
| D8-F1, D9-F1, CAT-D9-SUSPEND | Bug | Queued (CAT-D9-SUSPEND important, security, pre-existing) | FOLLOW_UPS "D9 admin role change" |
| D4-3 (FTS5/tsvector) | Feature | Queued | FOLLOW_UPS "D4 document full-text search" |
| UI-D4 (UI-D4-FU-1..3) | Bug | Queued, UNREPRODUCED | FOLLOW_UPS "Stack-3 browser pass follow-ups" |
| UI-FAV-1 | Bug | Queued (important, pre-existing) | same |
| MP-FLAKE-1, WEB-DEPS-1 | Task | Queued | FOLLOW_UPS "UI-D7/D8 landed" |
| AND-FU-1..7 | Task | Queued | FOLLOW_UPS "Android QA boot landed" |
| S3-FU-1..3, S3-R2-*, S3-LIVE-FU-1/2 | Task | Queued; S3 live PROVISIONAL | FOLLOW_UPS S3 sections |
| TAURI-FU-1..7, TAURI-F6/F7/F9/F10, TAURI-F14..F16 | Task / Bug | Queued; confirm-on-URL-change PROVISIONAL until native boot | FOLLOW_UPS Tauri sections |
| D11-R2-2..5 | Task | Queued | FOLLOW_UPS "D11/D7 review round 1" |
| D7-R2-F1..F4, D7-F3, D7-R3-FU1, D7-R3-M1, D7-R3-M3, D7-LIVE | Task / Bug | Queued; D7 landed PROVISIONAL | FOLLOW_UPS D7 sections |
| HELIXQA-FU-1/2, HQA-R3-1..4 | Task | Queued | FOLLOW_UPS "HelixQA stack3 wrapper" |
| LV3-1..5 | Task | Queued (LV3-1 is a SKIP for plain users) | FOLLOW_UPS "Live verification of stack 3" |
| AZ2-3, AZ2-5, AZ2-10, AZ2-R2/R5/R6 | Task | Queued | FOLLOW_UPS "Authz round 2" |
| D1 (90 owed script guides), CAT-021-F4 | Task | Queued | FOLLOW_UPS secrets/doc sections |

## Part B — items registered by this revision (previously only in prose or missing)

Each: Status Queued unless stated. Acceptance always requires machine evidence from a live run (§11.4.289) with N>=3 identical results where it is a verification item.

### Verification (PROVISIONAL until the job passes)

| Id | Type | Status | Description | Acceptance | Blocker |
|---|---|---|---|---|---|
| CAT-QA-SONAR-1 | Feature | PROVISIONAL-DONE | SonarQube on-demand QA service landed on `main`; run3 scan evidence in `docs/qa/sonarqube-20261105/run3/` (gate OK; api 699, web 404 issues); sub-item CAT-QA-SONAR-1b recorded one live start through the Containers backend from the main checkout (log not committed). **Verification:** provisional — job CAT-QA-SONAR-1 — checks live `stack3/up.sh` and `sonarqube/up.sh` through the Containers backend, scan re-run after the triage fixes — layer runtime — target QA host — artifact `main` — due before any v2.4.1 tag. | Scan re-run with evidence; the Containers-backend start captured under `docs/qa/`. | None blocking; the earlier Docker Hub rate-limit blocker no longer applies to run3 (pinned images were present). |
| CAT-QA-SONAR-2 | Task | Queued | Triage of SonarQube run3 (api 699 = 697 smells + 2 bugs; web 404 = 384 smells / 15 bugs / 5 vulnerabilities): fix order web S2245 vulnerabilities, web bugs, then S3735/S3776, api S1186; api S1192/S3776 by baseline ratchet (see `docs/FOLLOW_UPS_NEXT_PHASE.md`). | Sonar re-run shows the vulnerabilities and bugs gone; ratchet baseline recorded. | None. |
| CAT-QA-SONAR-3 | Task | PROVISIONAL-DONE | The 16 `typescript:S2699` assertion-free web tests: fix landed in `765d6544` and `6b3d706e` (merged `c80e35dd`) and `23b842d0` (direct on `main`); open follow-ups are test gaps listed under CAT-QA-SONAR-3 in the follow-ups file. **Verification:** provisional — job Sonar re-run — checks S2699 count 0 — layer artifact — target SonarQube run4 — artifact `main` — due before any v2.4.1 tag. | S2699 count 0 on re-run; follow-ups closed. | Sonar re-run owed. |
| CAT-QA-SONAR-4 | Task | PROVISIONAL-DONE | Fix the 5 web `typescript:S2245` vulnerabilities and 15 web BUGs from SonarQube run3 (branch `fix/web-sonar-vulns-bugs`, landed in merge `f9932aec`, Opus round-2 GO-with-follow-ups). **Verification:** provisional - job: Sonar re-run of catalogizer-web - checks: the 20 findings no longer reported - layer: static analysis on main - target: SonarQube instance (stack3/sonarqube, containers backend) - artifact: main tip at re-run - due: next release prep (verification OVERDUE until run). | Re-run SonarQube; follow-ups FU-A (flaky MediaPlayer resume test), FU-B/FU-C (cosmetic), FU-D (randomId is not a security token). | Sonar re-run not yet run. |
| CAT-QA-SONAR-5 | Task | PROVISIONAL-DONE | The 2 API `go:S3923` bugs: fix landed in `79547302` (merge `b304304b`, Opus round-1 GO-with-follow-ups); the follow-up is an operator decision on an ignored wizard template (§11.4.122 / §11.4.66). **Verification:** provisional — job Sonar re-run — checks go:S3923 count 0 — layer artifact — target SonarQube run4 — artifact `main` — due before any v2.4.1 tag. | S3923 count 0 on re-run; operator decision recorded. | Sonar re-run owed; operator decision open. |
| CAT-UI-AXE-LIVE-1 | Task | PROVISIONAL-DONE | Live axe rerun of D1-D8 (`docs/qa/ui-d5d6-20261105/axe_live.cjs`, `ui-d7d8-20261105/d7_axe.cjs` + `d8_repro.cjs`, UI-D1 repro, D2/D3 pages) on a rebuilt web image, N>=3. Verification: provisional — job CAT-UI-AXE-LIVE-1 — checks axe/d8/D1-D3 — layer runtime-UI — target stack 3 web — due on image rebuild. | N>=3 identical sha256 per check; promotes D1..D8 to VERIFIED; contrary evidence reopens. | Web image must be rebuilt from `main`; stack 3 containers are down. |
| CAT-STACK3-REBUILD-1 | Task | Queued | Rebuild web and API images from `main` (API: S3 protocol is in `catalog-api`), redeploy `docs/qa/stack3/up.sh <tag>` and monitoring `up.sh`; verify the URL map on the LAN URLs; rotate QA JWT_SECRET / METRICS_SCRAPE_TOKEN at recreate. | All services healthy on LAN URLs; new image tag recorded. | Conductor-run (containers do not survive reboot); host load. |
| CAT-PROM-LAN-1 | Task | Queued (decision made, NOT yet applied) | Expose Prometheus LAN-only at `http://192.168.1.115:39090` behind the scrape token; PostgreSQL 127.0.0.1:35434 and Redis 127.0.0.1:36381 stay loopback. | LAN URL answers with token, anonymous refused; PG/Redis not reachable from LAN. | Needs CAT-STACK3-REBUILD-1. |
| CAT-PPROF-LIVE-1 | Task | Queued | Restart the stack-3 API with `HELIX_PPROF_ENABLED=true`; confirm `/debug/pprof/*` 401/403 for non-admin and the 60 s cap; run cover-art SSRF-F2..F5; measure `TRUSTED_PROXIES` from a second LAN host. | Live evidence for each gate (currently UNCONFIRMED). | CAT-STACK3-REBUILD-1; second LAN host (open decision). |
| CAT-DETERMINISM-LEDGER-1 | Task | Queued | N>=3 determinism ledger: per live check, identical result across 3 runs on the same artifact fingerprint and board/target (§11.4.50). | Ledger file under `docs/qa/` listing every check, N, hashes. | Depends on the live reruns above. |

### Feature / coverage legs

| Id | Type | Status | Description | Acceptance | Blocker |
|---|---|---|---|---|---|
| CAT-HELIXQA-VISION-1 | Feature | Queued | HelixQA UI leg with a validated Vision model (§11.4.278): web Vision bank + CDP wiring on stack 3 (extends HELIXQA-FU-1/2). Stop/start `llmctl-onnx@decide-nli` only via `llmctl stop`/`start`, never `enable`, CPU-capped. | Per-frame verdicts with model identity and validation record; screen catalogue coverage. | Host capacity (`llmctl capacity`: does_not_fit while decide-nli runs uncapped) — operator decision. |
| CAT-TAURI-NATIVE-1 | Task | PROVISIONAL-DONE | Native boot of Tauri confirm-on-URL-change (TAURI-F6) and SSRF changes: `TAURI_BUILD_JOBS=2 scripts/tauri/build.sh <out>`, `QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop`; recording + OCR that the dialog appears, Cancel/close return false. Verification: provisional — job CAT-TAURI-NATIVE-1 — layer native-UI — target desktop session — due before v2.4.0. | Recording + OCR evidence N>=3; mutant R5 killed by the live check. | Desktop display session. |
| CAT-TAURI-INSTALLER-1 | Feature | Queued | Desktop installer wizard (build, run, verify install/uninstall on a clean target, §11.4.38). | Installable artifact evidence on a clean target. | Depends on CAT-TAURI-NATIVE-1 tooling. |
| CAT-ANDROID-LOGIN-1 | Task | Queued | Android real login via a secret-safe path (no credentials typed blind, §11.4.193) and LAN display check, mobile and TV. | OCR/uiautomator evidence pre and post login. | Emulator/device availability; secret-safe path. |
| CAT-S3-LIVE-2 | Task | Queued | Second S3 implementation run (AWS S3 or MinIO/Ceph RGW) to kill survivors M2 (1001-key batch) and MC (`+` in CopySource); virtual-host addressing, TLS, >5 GiB objects. Plus S3-FU-1..3 UI/docs/`SUPPORTED_PROTOCOLS`. | Suite green on second server with evidence. | Second S3 server (operator). |
| CAT-RESPONSIVE-A11Y-MATRIX-1 | Task | Queued | Responsive/a11y live matrix across devices and browsers (Chromium/Firefox/WebKit x breakpoints x light/dark) for web, website and admin. | Matrix evidence with layout oracle (§11.4.190). | Rebuilt web image. |
| CAT-PERF-CHAOS-1 | Task | Queued | Performance, security and chaos live suites for the release (extends S1 RSS growth UNCONFIRMED, scaling/chaos review follow-ups). | Evidence per suite on stack 3. | Rebuilt images, host headroom. |
| CAT-WL13-PLAYER-1 | Feature | Queued | WL13 player follow-ups plus subtitles panel (see WL13-R*). | Live player run with subtitles; review GO. | none |
| CAT-PG-LASTADMIN-1 | Bug | Queued | PostgreSQL last-admin race (concurrent demotion/lock; related D9-F1). | RED reproduction on PostgreSQL, fix, GREEN. | Reproduce first (§11.4.292). |
| CAT-D5-BURSTY-AUDIO-1 | Task | Queued | D5 bursty-audio fixture for the subtitle/audio pipeline. | Fixture + live test. | none |
| CAT-QA-MON-FU-2 | Task | Queued | QA monitoring follow-up 2 (named in CONTINUATION queue; detail to be written from the monitoring review when worked). | Defined on pickup. | none |
| CAT-STACK-RETIRE-1 | Task | Queued | Retire QA stacks catqa1/catqa2 (docs note only; do not touch them in the meantime). | Docs note landed. | Operator go-ahead. |

### Release

| Id | Type | Status | Description | Acceptance | Blocker |
|---|---|---|---|---|---|
| CAT-REL-V240-1 | Task | PROVISIONAL-DONE | v2.4.0 pre-release PUBLISHED (tag `v2.4.0` on `031045eb`, per CONTINUATION Rev 18). Original steps: `versions.json` 2.3.0 -> 2.4.0, changelog with VERIFIED / PROVISIONAL / REOPENED / OVERDUE listed separately (§11.4.290), tag on the upstreams, §11.4.40 full-suite retest, §11.4.185 manual QA. **Verification:** provisional — job `docs/PROVISIONAL_VERIFICATION_PLAN.md` order, then §11.4.40 full-suite retest and §11.4.185 manual QA — checks the PROVISIONAL rows of `[2.4.0]` — layer runtime — target stack 3, test servers, devices — artifact v2.4.0 / `031045eb` — due before the v2.4.0 tag (passed, tag already published): verification OVERDUE. | Every `[2.4.0]` PROVISIONAL row promoted or reopened with evidence; retest and manual QA recorded. | Tag IS published (six upstreams per Rev 18); owed: GitFlic and GitVerse release objects (no CLI found, plan §4), plus all verification above. |
| CAT-REL-V241 | Task | In progress | v2.4.1 pre-release CANDIDATE, NOT tagged: product versions bumped to 2.4.1 on branch `release/v2.4.1-prep`, changelog `[2.4.1]` with VERIFIED / PROVISIONAL / REOPENED / OVERDUE listed separately (§11.4.290); open: PROVISIONAL verification plan, §11.4.40 full-suite retest, §11.4.185 manual QA, Opus review, merge to main, tag. Not PROVISIONAL-DONE yet: not landed on main and not tagged. | Tag exists on every upstream; changelog honest; no production-ready claim before retest and manual QA. | All verification items in `docs/PROVISIONAL_VERIFICATION_PLAN.md`; CAT-REL-V240-1 OVERDUE rows carried forward; tag form (pre-release vs production) is an open decision. |
| CAT-REL-PERM-1 | Task | Ready for testing (PROVISIONAL) | 2026-11-10 throwaway tag v0.0.0-permtest.1 (created, released, deleted, ls-remote clean on 4 repos): GitLab release create PASS on milos85vasic + vasic-digital; GitHub FAILS 403 with fine-grained GITHUB_TOKEN (lacks Contents: Read+write), PASSES with keyring gho_ token (run gh with GITHUB_TOKEN unset). Use that path at release time. | none |
| CAT-KEYROT-WAIVER-1 | Task | Open risk, operator-accepted | Key-rotation waiver recorded as an operator-accepted open risk. NO expiry is set or invented. | Operator sets an expiry or closes the risk. | Operator decision. |

### Blocked by hardware

| Id | Type | Status | Description | Acceptance | Blocker |
|---|---|---|---|---|---|
| CAT-SYN-LIVE-1 | Task | Skipped (`hardware_not_present`) | Synology SMB/FTP/NFS live tests and idle-deadline/cut-recovery live tests (extends ST-NFS). Drives powered OFF by the operator 2026-11-05 19:09 CET; never substitute a double. | Live evidence once drives are on and NFS export enabled. | Operator powers the NAS back on. |

### Governance / housekeeping

| Id | Type | Status | Description | Blocker |
|---|---|---|---|---|
| CAT-WI-ENGINE-1 | Task | Queued | Wire the constitution workable-items engine (SQLite SSoT) and migrate this register into it; includes the `atm_id` -> `cat_id` rename. | Engine key-grammar validation owed upstream. |
| CAT-CONST-FETCH-1 | Task | Queued | Periodic fetch of the constitution submodule (pin 92a9ecf8, v73.2). | none |
| CAT-LLMS-VERIFIER-1 | Task | Queued | `llms_verifier` submodule decision/wiring. | Operator decision. |
| CAT-WAIVER-GATE-DESIGN-1 | Task | Queued | Waiver gate consumer-data design and trust manifest. | Operator decision. |
| CAT-LLMCTL-PINS-1 | Task | Queued | llmctl pin bumps. | none |
| CAT-TEST-PREEXIST-1 | Bug | Queued | Pre-existing root-package OpenAPI test failure; services `TestCAT002` is host-memory dependent (make hermetic or quarantine, §11.4.248). | Reproduce first. |
| CAT-DOCS-EXPORT-1 | Task | Queued | Regenerate `CONTINUATION.html/.pdf` (stale at Rev 9) and the 90 owed script guides (D1). | none |

### Added Revision 2 (2026-11-06)

| Id | Type | Status | Description | Blocker |
|---|---|---|---|---|
| CAT-PROM-AUTH-1 | Task | In testing (PROVISIONAL) | Prometheus basic auth (landed 42933e6b, applied live; anonymous 401). N>=3 repeat in flight. | Live repeat. |
| CAT-PROM-AUTH-FU-1 | Task | Queued | PROM-R2-1..5 follow-ups in FOLLOW_UPS_NEXT_PHASE.md. | none |
| CAT-VERIFY-LANBIND-1 | Bug | Ready for testing | `monitoring/verify.py` LAN-NOT-reachable assertions contradict the LAN decision; make them conditional on *_BIND. | Reproduce first (3 FAILs observed). **Verification:** provisional — job: re-run `verify.py` live on a rebuilt stack 3 (stack was down when authored) — checks: lan.* reachable/auth/loopback rows PASS with expectation_source=env — layer: runtime — target: stack 3 monitoring pod — artifact: feat/followups-20261110 — due: next stack-3 bring-up. Unit-level only so far (test_lanbind.py). |
| CAT-D7-R4-1 | Bug | In progress | UploadManager failed-row error text contrast 3.60/3.29. | author in flight |
| CAT-D7-R4-2 | Task | In progress | Restore global.fetch in contrast tests. | author in flight |
| CAT-D7-R4-3 | Bug | In progress | ExternalIntegrations pending badge equals card bg in dark. | author in flight |
| CAT-D7-R4-4 | Task | In progress | Contrast helper: light-bg island detector. | author in flight |
| CAT-FRONTDOOR-RATELIMIT-1 | Bug | Ready for testing (PROVISIONAL) | Host-local clients share pasta alias 169.254.1.2 (by design). Substitute-source (extra IP 192.168.1.250 on this host, now removed) has its own bucket: VERIFIED 3/3 (docs/qa/second-host-20261106/). Real remote NIC path UNCONFIRMED (no second machine; = QA-MON-FU-4). Hardening for TRUSTED_PROXIES landed. | real second machine / phone-laptop on LAN |
| CAT-TRUSTEDPROXIES-PROD-1 | Bug | Ready for testing (PROVISIONAL) | F1 (important): production docker-compose.yml runs nginx in front of api with TRUSTED_PROXIES unset -> all clients share nginx IP -> one 30/min login bucket. Set explicit CIDR in deploy configs. | none |
| CAT-TRUSTEDPROXIES-AUTO-1 | Bug | Ready for testing (PROVISIONAL) | F2 (important): TRUSTED_PROXIES=auto (in .env.example) lets any RFC1918 client forge ClientIP via XFF 'x, <lan ip>' -> login limiter bypass. Fix/deprecate auto, document. | none |
| CAT-TRUSTEDPROXIES-FU-1 | Task | Queued | F3 pin auto ranges in tests; F4 capture API log line for ClientIP=169.254.1.2; F5 script: pre-flood 401 control, 'PASS (A only; B SKIP)' output, live-lockout warning; F6 quote $T, SC2034. | none |
| CAT-QA-THRW-CLEAN-1 | Task | Queued | Prune inactive throwaway users `qa_thrw_*` (no delete route). | none |
| CAT-SSRF-F2F5-LIVE-1 | Task | Queued | Live proof of cover-art ffmpeg/HLS/FIFO/size cap + dial-time/redirect SSRF needs seedable QA corpus with removable root. | corpus tooling |
| CAT-TAURI-WIZARD-ACL-1 | Bug | In progress | Installer wizard Save Configuration fails 'plugin:dialog|save not allowed by ACL' (no capabilities dir); fix on branch feat/tauri-native-qa 389f8abe, GREEN live runs owed. | review + GREEN runs |
| CAT-TAURI-FU-8 | Task | Queued | Desktop: Settings/Logout not sticky in sidebar; generic 'Login failed' no diagnostic; 'Invalid response format' in Storage Configuration (TAURI-FU-1 drift); poor contrast on Dark theme select; run-qa.sh container cannot reach host LAN IP (use host.containers.internal). | none |
| CAT-SMB-SUBDIR-1 | Task | Queued | SMB roots cannot be scoped to a subdirectory (storage_roots.path = share name); Anton Public/Testing corpus (docs/qa/anton-smb-20261106/) can only be added as the whole share `Public` (also indexes Misc/Incoming). Add subdir support or a dedicated `Testing` share (needs root smb.conf edit). | design |
| CAT-FRONTDOOR-LOCALMODE-1 | Task | Ready for testing | Extend test_clientip_frontdoor.sh with a local-second-IP (`--interface`) mode so leg B runs without ssh. | none **Verification:** provisional — job: run `test_clientip_frontdoor.sh URL --interface <2nd local IP>` live — checks: A and B(local) PASS — layer: runtime — target: stack 3 front door — artifact: feat/followups-20261110 — due: next stack-3 bring-up. Sandbox test only so far (test_frontdoor_interface.sh). |
| CAT-D7-ISLAND-1 | Bug | Queued | AdvancedSearch tab icon chips `bg-blue-100` (x5) are light islands in dark theme (found by `failingElements(..., {islands:true})`). | none |
| CAT-D7-ISLAND-2 | Bug | Queued | FormatConverter empty-state circle `bg-gray-100` and the three AI unavailable panels `bg-white rounded-lg shadow p-6` are light islands in dark theme. | none |
| CAT-D7-ISLAND-3 | Bug | Queued | AIDashboard icon chips `bg-indigo-100`/`bg-green-100`/`bg-yellow-100` are light islands in dark theme. | none |
| CAT-LANIP-DYN-1 | Bug | Ready for testing (PROVISIONAL) | Landed: docs/qa/lib/lan_ip.sh (QA_LAN_IP override, route-derived, validated); sourced by sonar/s3/helixqa/frontdoor/test-servers; FTPS cert regenerated atomically when SAN lacks current IP. Opus GO-with-follow-ups, security-adjacent minors fixed. **Verification:** provisional — job: run test-servers up.sh/verify.sh live (real cert swap) — checks: strict-TLS FTPS client with new SAN — layer: runtime — target: this host — due: next session. | none |

## Part C — counts at Revision 1

Registered in Part B: 29 new ids. Already present (Part A rows, each a pointer to one or more existing ids): 22 rows covering the ids named in the CONTINUATION queue. Duplicates created: 0.
