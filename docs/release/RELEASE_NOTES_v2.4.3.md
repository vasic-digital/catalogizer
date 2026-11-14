# Catalogizer v2.4.3 release notes

**Pre-release, NOT production-ready.** Release gate state: `BLOCKED-NO-BASELINE` (no live run on connected hardware and no v2.4.2 baseline run exist; the full-suite retest, manual QA and live UI QA have not been run). Everything below labelled PROVISIONAL landed on `main` and counts as done for planning but is not verified. Source of truth: [`docs/CHANGELOG.md`](../CHANGELOG.md) section `[2.4.3]`; this file is the same content in release-notes form for GitHub/GitLab release pages. Credentials and private hosts are not part of these notes.

Upgrade from v2.4.2: read "Behaviour changes and upgrade notes" first; the changelog also keeps the `[Unreleased]` items carried into this release (outbound address classes, `TRUSTED_PROXIES`, FTP `CopyFile`, constitution 2.5.0) under `Carried from the former [Unreleased] section`.

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Prepared on branch `release/v2.4.3-prep` from `main` `b43f5370` (328 commits since the `v2.4.2` tag, `26f2e1df`; `git log v2.4.2..b43f5370`). Not tagged, not published. Versioning is SemVer. Version files bumped to 2.4.3: `versions.json` (global 2.4.3), `docs/api/openapi.yaml`, `catalog-web`, `catalogizer-desktop` and `installer-wizard` (package.json, package-lock.json, tauri.conf.json, Cargo.toml), Android `versionName` 2.4.3 / `versionCode` 9 (was 8), Android TV `versionName` 2.4.3 / `versionCode` 11 (was 10), the two Android `CLAUDE.md` version lines, the installer-wizard AppImage file name in `challenges/scripts/desktop_appimage_launch_challenge.sh`, and the version mentions in `README.md`, `docs/CONTRIBUTING.md` and `docs/DEPLOYMENT_GUIDE.md`.

### Release gate (constitution 11.4.298, stated first)
- **Gate state: `BLOCKED-NO-BASELINE`.** No 11.4.298 live run of the whole suite on connected target hardware has been done for this build, and no baseline run of v2.4.2 exists, so no 11.4.293 comparison (`NO-WORSE-PROVEN`) is possible. The live baseline run (flash/run v2.4.2, run the suite, then the same on v2.4.3) is **owed**. This is a labelled pre-release; it must not be deployed to users or called production-ready. The comparator and gate tools (`image_no_worse`, `release_gate`, CAT-WP03-MECH) are gate-logic unit class and are not wired into tagging or deploy.
- **The full-suite retest of the tag (11.4.40), the 11.4.185 manual QA and the live UI QA have NOT been run on this candidate.** The release-consistency checker is **advisory** for v2.4.3 (blocking for v2.5.0, see `docs/release/V2_5_0_GATE_CHECKLIST.md`, an unreviewed draft).

### State summary by label (11.4.290(G))
V verified: 3 evidence sets (evidence exists and sha-verifies; their content is PROVISIONAL, see below) + P provisional: every item in the PROVISIONAL list (47 distinct `Roadmap-Item` ids in the commit trailers, plus the sub-ids named in the lines) + 0 reopened + 0 overdue-new (the carried OVERDUE jobs below remain).

### VERIFIED (committed machine evidence, checked while preparing this entry)
Checked on this branch with `sha256sum -c` (0 non-OK lines). "VERIFIED" here means the evidence set exists and its hashes verify, not that the behaviour is verified.
- `docs/qa/anton-smb-n3-20261112/` (Anton SMB N=3 live run; `SHA256SUMS` verifies). No negative control: the run proves reachability at N=3 only.
- `docs/qa/img-slim/` (CAT-IMG-SLIM-1 minimal-ffmpeg image evidence; `SHA256SUMS.txt` verifies). Container class; the full `catalog-api/Dockerfile` build including the Go stage was not exercised.
- `docs/qa/secq2-repro-20261112/` (CAT-SECQ2-WD-F1 live WebDAV reproduction: not reproduced, 6/6 ok; `SHA256SUMS` verifies).
- Carried from 2.4.2 and earlier, not re-run: the VERIFIED rows of `[2.4.2]`, `[2.4.1]`, `[2.4.0]`.
- Other evidence directories added since v2.4.2 (`docs/qa/login-lock-2-round*`, `chpw-rl-2-round*`, `sec-ws-1*`, `favr3-fu*`, `stack3`) carry no checkable hash manifest and are treated as PROVISIONAL.

### PROVISIONAL (landed on `main`; counts as done for planning, NOT verified)
Each item landed after an Opus review that returned GO-with-follow-ups (minor-class follow-ups tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`). Evidence class is mostly unit / httptest / SQLite (some PostgreSQL 15/16, container) level; live end-to-end runs are owed. Pending job per row: the full-suite retest, live UI QA and the live hardware baseline listed in `docs/PROVISIONAL_VERIFICATION_PLAN.md`.

**Security: authentication, sessions, rate limiting**
- CAT-D9-SUSPEND (+ D9S-R3-1..4, D9SR3-F1/F2/F3/F4): admin suspension is a permanent lock and revokes sessions; NULL-safe `is_locked` reads; login/refresh non-loginable semantics; unknown/legacy-user login timing.
- CAT-LOGIN-LOCK-1, CAT-LOGIN-LOCK-2: login back-off redesigned; in-flight attempt cap, every evaluated attempt charged, owner success never writes the account scope; PostgreSQL-verified (both images).
- CAT-CHPW-RL-1, CAT-CHPW-RL-1-FU, CAT-CHPW-RL-2, PKGC-FU4-F2: change-password per-session failure budget and per-user ceiling, session revoke on change, AST wiring guards for the `/api/v1/auth` routes (unit class, in-memory counters per process).
- OPA-1..OPA-5, PKGC-FU4, CAT-OPDEC-A: Redis rate-limit per-limiter outage breaker with half-open single probe, per-tier Redis keys (auth 600 / default 2000 per minute), outage after startup falls back to the in-memory limiter; role 2 compared like every other role.
- CAT-LOGSHARE-TOK-1: log-share tokens are 32 random bytes, hashed at rest, shown once, uniform 404 on every failure; session-id hardening.
- CAT-JOB-404-1: a job id owned by another user returns the same 404 as a missing id; permission lookup runs before the job lookup.
- SEC-WS-1, SEC-WS-1-F3, SEC-WS-1 F4/F5/F6, SEC-WS-1R3, SEC-WS-1R3FU-1/2: `/ws` behind authentication with an Origin policy, post-upgrade auth recheck, audience-filtered broadcasts, nginx confs forward `$http_host` (httptest level; live client/nginx run owed).
**Security: egress, error leakage, confinement**
- CAT-SECQ1-A, CAT-SECQ1-B (W3), S1A3-1..4, OBF-3, PKGA-FU2, PKGA-FU2-F2: outbound address-class pinning; the cloud-metadata singleton addresses (including `fd20:ce::254`) are always refused, and SMB host vetting shares the netguard reserved ranges.
- SEC-DIAL-1, SEC-DIAL-1-FU: SMB/FTP/NFS storage hosts are vetted at create and pinned at dial; dial errors scrubbed.
- CAT-PXF23, CAT-PXF23-R, CAT-PXF23-R5, CAT-PXF23-R5B, PXF23-R5a/F4, CAT-R5-FU, CAT-R5FU-FU, CAT-WP02-ERRLEAK, SVC-PREEXIST-1: internal `err.Error()` text no longer reaches clients (fixed class text, request id).
- PX-F1..F23 (PX-F13/F17/F20-F23: unified log/panic redaction, custom Recovery, typed leak scanner, KnownSecrets), HANDLERS-RED-1: log redaction hardening.
- CAT-FS-SYMLINK-1: symlink confinement for local and NFS filesystem clients with a single confined copy path.
- CAT-SEC-CONTRIB-1: `.mailmap` maps the placeholder agent identity to the owner.
- CAT-RETEST-STALE-1: NFS export refuses `,` followed by `:` (second darwin server location); two stale tests fixed.
**Storage connectors**
- CAT-SMB-SUBDIR-1: Anton SMB N=3 live logs added (no negative control).
- CAT-SECQ2-WD-F1: live WebDAV reproduction, not reproduced (VERIFIED evidence, PROVISIONAL conclusion).
**Installation wizard**
- OBF-1, OBF-2, OBF-4, OBF-FU, CAT-OPB-FU, CAT-OPB-FU2, CAT-QA-SONAR-5: merge-never-overwrite of the config, symlink-resolved confinement (dangling links refused), probe temp files, `CONFIG_PATH` (wizard directory) vs `CATALOG_CONFIG_PATH` (app config file) docs, declared `file_create` template loaded.
**Web**
- CAT-UI-FAV-1, CAT-UI-FAV-1b, FAVR3-1/2, FAVR3R3FU: favourites open the right entity (`entity_id`/`entity_type`), optimistic update on live keys with rollback; live Playwright run owed.
- CAT-QA-SONAR-7-FU2, CAT-OPDEC-B: Enter auto-repeat guard, no block content in template buttons, unused `onActivateKey` removed.
- CAT-PREEXIST-7, CAT-PREEXIST-8, CAT-PREEXIST-9, PW-F1..F10 (CAT-PWF9): vitest alias fallback for dist-less `@vasic-digital/*` packages, jsdom Storage on Node 26.
**Release tooling**
- CAT-REL-CHECK-1, CAT-REL-CHECK-2, CAT-REL-CHECK-3: HEAD-snapshot release-consistency checker (502 tests, 236 mutants), waivers; advisory for v2.4.3.
- CAT-WP03, CAT-WP03-MECH (WP03R3-F1..F7): `image_no_worse` comparator and `release_gate` scripts (11.4.293 / 11.4.298), gate-logic unit class, not wired into tagging or deploy.
- V250-GATES: v2.5.0 gate checklist, an UNREVIEWED draft (independent review owed).
- CAT-CONST-BUMP-1, CAT-UPSTREAMS-1: constitution bump (`helixconstitution-v73.3-397-g02c10935`), Codeberg upstream recipe.
**Image, QA tooling, tests, docs**
- CAT-IMG-SLIM-1: API image 746 MB to 307 MB via minimal ffmpeg (container class).
- CAT-PREEXIST-1..6, CAT-PREEXIST-10, CAT-PXF23 follow-ups: stale or flaky pre-existing test failures fixed (host-adaptive stress floor, nil-slice allowlist, sqlmock expectations, openapi-contract 404).
- CAT-WP10-TRIAGE, WP-10, WP10R4: follow-up triage document and generator (`docs/FOLLOW_UPS_TRIAGE_2026-11-12.md`).
- CONT-DENY-STRUCT (Containers `aa99d46`), BG-NASEMU-2026-11-13 (NAS emulator research and NE-3 seed, Containers `24ba32c`), CAT-DEVEMU (device-emulation plan), BG-OPQ-2026-11-13b (operator decisions record), CAT-QA-SONAR / CAT-SONAR-REGAUTH (Sonar registry-auth follow-ups), DOCS-ORPHANS-2026-11-13 (orphan reachability and PDF renderer), CAT-DOCS-README (DeepWiki badge).

### REOPENED
- None recorded between `v2.4.2` and `main` `b43f5370`: no live rerun FAILed on a previously closed item. This is the absence of recorded failures, not proof that nothing regressed (the full-suite retest of the tag has not been run).

### OVERDUE
- Carried, still overdue: the `[2.4.2]` and `[2.4.1]` verification jobs (full-suite retest of a tag, SonarQube run6 on a tag, 11.4.185 manual QA, live UI QA with Vision, Anton `Public/Testing` SMB runs, cover-art SSRF N>=3 production-client leg). No new job has an elapsed due date recorded.

### Known failing, unconfirmed or not landed (from the 11.4.40 partial retest of `main` `3a33fb05`, CAT-RETEST-STALE-1, in `docs/FOLLOW_UPS_NEXT_PHASE.md`)
- Retest (scratch copy, PROVISIONAL): go build/vet rc 0; go test 5935 pass / 20 honest skip / 3 fail (two new stale tests fixed on `main`; the third was scratch pollution); catalog-web type-check pass, vitest 2796 pass.
- Pre-existing lint error: catalog-web lint 1 error + 84 warnings (`ExternalIntegrations.test.tsx:100` unnecessary semicolon, same line in v2.4.2).
- The `catalog-api/tests/automation` module fails with connection refused instead of an honest SKIP (same on v2.4.2).
- The catalog-web production build is UNCONFIRMED (offline npm cache lacks `@vasic-digital/*`; ENOTCACHED). `gofmt` lists 42 files (same on v2.4.2). `-race` (Alt-Svc Use race F-F) not run.
- Release checker: the known 4 findings (sha_fail=1, no_manifest=1, unlisted=1, unused_waivers=3 as counted by the checker) are unchanged and advisory for v2.4.3.
- **CAT-NFS-EMU-1 was NOT landed** (R_max NO-GO on `feat/nfs-emulator`); successor **CAT-NFS-EMU-2** is open. Likewise not landed or still open: CAT-PREEXIST-9b (web load failures, UNREPRODUCED), D8-F1, D9-F1, SEC-WS-1 follow-ups, RS-1..RS-4 (minor, from CAT-RETEST-STALE-1). Items owed before v2.5.0 are in `docs/release/V2_5_0_GATE_CHECKLIST.md`.
- Synology SMB/FTP/NFS live tests remain SKIPPED (drives powered off).

### Behaviour changes and upgrade notes (read before upgrading)
- **Job ids:** a conversion job owned by another user now returns 404, identical to a missing id (was distinguishable from a missing id).
- **Symlink confinement:** local and NFS filesystem roots refuse symlinks that escape the root. A per-root option `allow_symlink_escape` (default off) exists; it is not reachable from the DB row or config in this release (documented in `docs/security/PATH_CONFINEMENT.md`; UNCONFIRMED how an operator sets it).
- **Metadata addresses:** the cloud-metadata singletons (incl. `fd20:ce::254`) are always refused for storage roots and SMB hosts, with no allow-list. Storage hosts (SMB/FTP/NFS) are vetted at create and pinned at dial: an existing root pointing at a now-refused address will fail to connect.
- **NFS export:** an export string with `,` followed by `:` is refused (a second server location on darwin `mount_nfs`).
- **Redis rate limits:** per-tier keys (`auth` 600 / default 2000 per minute); an outage after startup falls back to the in-memory limiter; login/register stay on the in-memory 30/min limiter.
- **Login throttle:** a cap of in-flight password evaluations per account; every evaluated failed attempt is charged, also on an already blocked account; a successful owner login no longer writes the account throttle scope (throttle state is stored in the database; PostgreSQL-verified).
- **Change password:** failed current-password attempts are limited per session and per user (5 per 15 min); a password change revokes sessions (see CAT-CHPW-RL-1); the change-password counters are in memory per process.
- **Suspension:** an admin suspension is a permanent lock and revokes sessions (CAT-D9-SUSPEND).
- **Log shares:** tokens are new 32-byte random, hashed at rest, shown once; every failure returns the same 404; a JWT is required to create. Check the v28/v29 migration order notes in `docs/FOLLOW_UPS_NEXT_PHASE.md`; whether pre-existing share links keep working is UNCONFIRMED.
- **WebSocket:** `/ws` requires authentication and an Origin policy (host:port and scheme); deployments using the shipped nginx configs must pick up the updated confs (they forward `$http_host`).
- **Errors:** responses no longer include internal error text (fixed class text and a request id).
- **Roles:** role 2 is compared against the caller like any other role.
- **Image:** the API image is 307 MB (was 746 MB) with a minimal ffmpeg; **conversions to ogg and m4a now drop embedded cover art**.
- **Wizard:** the config is merged, never overwritten; `CONFIG_PATH` is the wizard directory and `CATALOG_CONFIG_PATH` the app config file.
- Behaviour changes already listed under `Security (outbound address classes)` and `TRUSTED_PROXIES` below (including the compose network upgrade step) apply to this release.
