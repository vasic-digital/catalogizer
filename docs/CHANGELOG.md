# Changelog

All notable changes to the Catalogizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security (TRUSTED_PROXIES)
- **`TRUSTED_PROXIES=auto` is deprecated and refused in release mode** (CAT-TRUSTEDPROXIES-AUTO-1): it trusts all RFC-1918/ULA ranges, so a client inside one of them could send `X-Forwarded-For: 9.9.9.9` and have the proxy append its own address, making `ClientIP()` 9.9.9.9 (login rate-limit bypass; gin skips trusted entries from the right and has no "trust only the direct hop" mode). With `GIN_MODE=release`, `auto` now fails unless `TRUSTED_PROXIES_ALLOW_AUTO=true`; a startup warning is logged whenever `auto` is used. A rejected or invalid `TRUSTED_PROXIES` now **fails closed** (no proxy trusted): previously a refused `auto` left gin's trust-everything default in place, and an invalid CIDR list left whatever gin had applied before the error (possibly a partial list). **Behaviour change:** deployments relying on `auto` in release mode must list explicit proxy CIDRs (or set the acknowledgement). `catalog-api/.env.example` no longer enables `auto`.
- **Production compose files now set `TRUSTED_PROXIES`** (CAT-TRUSTEDPROXIES-PROD-1): `docker-compose.yml` (nginx, `production` profile) and `deployment/docker-compose.yml` (`web`) pin the proxy to a fixed address on a fixed-subnet network (`172.28.0.10` / `172.29.0.10`) and the API trusts exactly that `/32`; previously every client behind nginx shared one login rate-limit bucket. Dynamic containers are allocated only from the upper half of each subnet (`ip_range` `172.28.0.128/25` / `172.29.0.128/25`), so the pinned address cannot be taken by another container. Overrides: `CATALOGIZER_SUBNET`, `CATALOGIZER_IP_RANGE`, `NGINX_IP` (root file) and `DOCKER_SUBNET`, `DOCKER_IP_RANGE`, `WEB_IP` (deployment file); when you change the address also set `TRUSTED_PROXIES`. A subnet that overlaps a host network fails `up` with a "Pool overlaps"/address-in-use error: pick another subnet via those variables.
- **Upgrade / migration (existing installs):** compose reuses an existing network of the same name and ignores new ipam settings, so the pinned address would fail or be ignored. Before the first `up` of this version run `compose down`: for the root file the network is project-scoped (`<project>_catalogizer-network`) and `compose down` removes it (do not remove a bare `catalogizer-network`, it belongs to another project). The `deployment` file's default network name changed from the fixed `catalogizer-network` to `catalogizer-frontdoor-net` (override `DOCKER_NETWORK`) so it does not collide with an older network; remove the old `catalogizer-network` only if no other stack uses it.
- `deployment/docker-compose.yml` `web` now sets `API_UPSTREAM: catalogizer-server:8080`. The web image's nginx template (`catalog-web/nginx.conf.template`, rendered by the official nginx entrypoint, `NGINX_ENVSUBST_FILTER=^API_`) proxies to `API_UPSTREAM`, whose default `host.containers.internal:8080` goes via the host and would hide the pinned web address from the API. Its `/ws` location now forwards `X-Real-IP`/`X-Forwarded-For` too, and `config/nginx.conf` `/health` does the same (a test scans every `proxy_pass` location).
- A startup warning is logged when `TRUSTED_PROXIES` contains `0.0.0.0/0` or `::/0`.
- Verification: `ip_range` reaching network creation was confirmed with a throwaway podman-compose project (pinned `.10` outside the range, a dynamic container got `.129`). Not verified live: the real compose stacks were only rendered (`podman-compose config`), not started; that the published `catalogizer/web` image is built from `catalog-web/Dockerfile` is assumed.
- Tests pin every `auto` range with both edges, the forgery hazard, release-mode refusal, fail-closed behaviour, and the compose-to-policy link (CAT-TRUSTEDPROXIES-FU-1).

### Governance
- **Spec Kit constitution 2.5.0** (2026-10-21): single-source flat layout (owner mandate: no nested submodule chains); canon pin moved `4bcb512` → `a3d80ce` (canon branch `feat/hub-engines-single-source` merged and pushed to canon `main`) and then to its follow-up `5f8503b` (pushed to canon `main`); `llmctl` `f42bad6` (cleaned, `CM-LLMCTL-SUBMODULE-NONRECURSIVE` PASS), root `helix_qa` `ad8b12b`; `claude_toolkit` `1f6dad3` (cleaned: nests only the third-party Go toolchain, ships `helix-deps.yaml`; at `a3d80ce` it was `223c702` with both gates RED); `CM-HUB-ENGINE-NO-OWNORG-NESTING`, `CM-HELIX-DEPS-MANIFEST-PRESENT` and `scripts/verify_helix_deps_manifests.sh` PASS (pass 54 / fail 0); index scope re-rendered from DATA (`derive_scope.sh`, `scope_to_lumen_json.py`, `scope_render.py`, `scope_drift_check.sh --regen`), drift check passes; root `helix-deps.yaml` refs re-synced to the staged gitlinks; `scripts/safe_submodule_init.sh` warns about leftover directories of dropped gitlinks; safe init through `scripts/safe_submodule_init.sh`; 27 flat tool submodules; Known Conflicts items 21(d), 25 to 27 and 29 updated (27 and 29 RESOLVED), items 30 to 32 added. Earlier in the day, **2.4.0**: canon pin moved `94b8836` → `4bcb512`; new section "Constitution Hub Engines" binding the vendored `llmctl`, `claude_toolkit` and `open_design` (rules HE-1 to HE-8, appendix Part 3); Known Conflicts items 21(c)/(d) corrected and items 25 to 29 added (29: code-index scope data) (canon's `CM-LLMCTL-SUBMODULE-NONRECURSIVE` gate fails at the new pin; `claude_toolkit` and `open_design` are not in canon's manifest). See `docs/GOVERNANCE.md`.
- Documentation: safe, non-recursive submodule initialisation through `constitution/` in `README.md`, `GETTING_STARTED.md`, `docs/DEVELOPER_GUIDE.md` and `docs/SUBMODULE_DEPENDENCIES.md`; new `docs/GOVERNANCE.md`.

### Changed (FTP CopyFile, filesystem package)
- **FTP `CopyFile` no longer deletes or overwrites the destination in place** (`catalog-api/filesystem/ftp_client.go`). The data is now stored under a temp name beside the destination, `.<base>.ftpcopy-<16 hex>.part` (base cut to 200 bytes on a rune boundary, so the whole name stays under NAME_MAX), and renamed onto the destination (RNFR/RNTO) only after the source transfer is confirmed. **Behaviour change:** a server that refuses rename onto an existing file (some proftpd/pure-ftpd configurations) now makes every copy onto an existing destination fail, with the destination left unchanged and the temp removed; previously the destination was overwritten in place (plain STOR), which such servers allowed. Servers with MoveFile-style rename semantics (possibly Windows/IIS FTP, UNCONFIRMED) may be affected as well. A failed or cancelled copy never touches a pre-existing destination.
- **Orphan reporting contract:** if the temp cannot be removed after a failure or cancellation, the returned error names it (`temporary file <path> may remain on the server`; typed `*orphanError`, reachable with `errors.As`, also on the cancel path) and a log line is written. Whether a failed cleanup DELE means "the temp never existed" is decided from the 550 reply text: only a text naming a missing file/directory (no such file, no such directory, not found, does not exist) counts, and any mention of permission, privilege, denial or read-only vetoes it (`550 No such privilege` and `550 File not found or permission denied` are reported as possible orphans). The heuristic is imperfect (servers word it differently, localised servers differ); an unrecognised or mixed wording is reported as a possible orphan rather than ignored, so a miss errs towards reporting.
- **Known limits:** once the transfer is confirmed the cancel watcher is disarmed, so a cancel during the final rename does not close the connection at once (a responsive server still commits). A rename that then gets no answer is cut off after `renameGraceAfterCancel` (5s after the cancel): the session is closed and the error says the rename "may or may not have been applied". A connection that dies during the rename is reported the same way, not as "left unchanged". The single-session spool fallback has no such cut-off. Evidence is unit-class against a fixture server; no live-NAS proof.
- **Scanner:** the catalogue scanner (`internal/services/generic_scanner.go`, FTP storage only) skips *files* named exactly `.<anything>.ftpcopy-<16 lowercase hex>.part` (in-flight or stale `CopyFile` temps; the pattern also matches names containing a newline); a user's own look-alike names such as `.x.part` are still catalogued. `FTPClient.ListDirectory` itself reports every entry, so callers that list, move and remove a directory (`moveDirectory` in `internal/services/protocol_handlers.go`) see a stale temp instead of failing the final remove with a confusing "not empty". Stale temps left by a crashed copy are not removed automatically.
- The single-session fallback spool directory is configurable (`FTPConfig.SpoolDir`, default the OS temp dir); free space is computed from `f_frsize` on linux and `f_bsize` on darwin/freebsd (build-tag split, so darwin builds); other platforms report free space as unknown. A negative `f_bavail` counts as 0.
- Evidence class: unit tests against a hand-written FTP responder; no live-NAS run yet (owed).

### Fixed
- **Challenge framework**: `RunSequence` now performs Kahn topological sort on dependencies, fixing `RunByCategory` alphabetical-order failures
- **Challenge streaming**: `RunAll` and `RunByCategory` handlers support NDJSON streaming mode (`?stream=true`) to eliminate tens-of-MB buffer blocking
- **Challenge timeout**: per-challenge timeout reduced from 72h to 30min as a hard ceiling distinct from stale-progress detection
- **database-connectivity hang**: `DatabaseConnectivityChallenge` now implements progress reporting, enabling 5-min stale detection
- **Android TV lint**: 49 pre-existing lint errors fixed via `@SuppressLint` for legitimate `androidx.tvprovider` and Media3 API usage
- **Android TV package**: challenge config corrected from `com.vasic.catalogizer.tv` to `com.catalogizer.androidtv`
- **Security**: axios SSRF vulnerability patched in `Catalogizer-API-Client-TS`
- **PlaybackTracker**: added `ensureActive()` and re-throw `CancellationException` to prevent coroutine leaks
- **PlaybackTrackerTest**: stabilized by removing shared `TestScope`, eliminating `UncompletedCoroutinesError`

## [2.4.1] - 2026-11-11 - pre-release (NOT production-ready)

Note: the `[Unreleased]` section above predates 2.4.0 and was never folded into a dated section; it is not part of the 2.4.1 content below. Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Prepared on branch `release/v2.4.1-prep` from `main` `1397d5b9` (37 commits since the `v2.4.0` tag, measured with `git rev-list --count v2.4.0..1397d5b9`). The tag `v2.4.1` is cut from the `main` commit that carries this text (later merges to `main` before the tag, listed under the Sonar rows below, are included); the release is published as a labelled pre-release (`docs/PROVISIONAL_VERIFICATION_PLAN.md` section 4).
Version files bumped to 2.4.1: `versions.json` (global 2.4.1, `build_number` untouched, owned by the build tool), `docs/api/openapi.yaml`, `catalog-web`, `catalogizer-desktop` and `installer-wizard` (package.json, package-lock.json, tauri.conf.json, Cargo.toml), Android `versionName` 2.4.1 / `versionCode` 7 (was 6), Android TV `versionName` 2.4.1 / `versionCode` 9 (was 8), the two Android `CLAUDE.md` version lines, and the installer-wizard AppImage file name in `challenges/scripts/desktop_appimage_launch_challenge.sh`. A throwaway check (not a repo script; the repo has no version-consistency script) parsed the 12 machine-readable version values (versions.json, openapi, 3 package.json, 3 package-lock.json roots, 2 tauri.conf.json, 2 Cargo.toml) and found them equal; with the 2 `versionName` and 2 `versionCode` values, the 3 lock-file nested root entries and the 2 `CLAUDE.md` lines the bump touches 21 version fields in all, all now 2.4.1 (the `versionCode` values are 7 and 9). Versioning is SemVer.
**What this candidate is NOT:** the full-suite retest (11.4.40) and the 11.4.185 manual QA have NOT been run, and the PROVISIONAL rows below are not verified. Nothing here may be called production-ready.

### VERIFIED (committed machine evidence; this window)
- SonarQube run3 measurements exist and verify from the committed tree: `sha256sum -c SHA256SUMS` in `docs/qa/sonarqube-20261105/run3/` passes (checked while preparing this entry); quality gate OK for both projects, catalogizer-api 699 issues (697 code smells, 2 bugs `go:S3923`, 0 hotspots), catalogizer-web 404 issues (384 code smells / 15 bugs / 5 vulnerabilities, 0 hotspots; recomputed from `catalog-web.issue_facets.json`). The 5 vulnerabilities are `typescript:S2245` MAJOR (insecure PRNG: `test-setup.ts:132,135`, `AdvancedSearch.tsx:199`, `SmartCollectionBuilder.tsx:100`, `UploadManager.tsx:50`); the 15 bugs are S1082 x9, S6959 x2, S8985 x2, S3923 x1, S9115 x1; 16 BLOCKER `typescript:S2699` are tests without assertions. Status after the scan: a fix for the 16 S2699 tests has landed on `main` (PROVISIONAL, see the Sonar row below; not claimed cleared); a fix for the 2 API `go:S3923` bugs has also landed on `main` (PROVISIONAL, Sonar re-run owed, follow-up CAT-QA-SONAR-5); a fix for the 5 web vulnerabilities and the 15 web bugs has landed on `main` in merge `f9932aec` (CAT-QA-SONAR-4, Opus round-2 GO-with-follow-ups; PROVISIONAL, Sonar re-run owed); the 22 CRITICAL findings (go:S1186 x10, typescript:S3735 x4, typescript:S3776 x8) have a fix landed in merge `17aaa10a` (CAT-QA-SONAR-6 follow-ups; PROVISIONAL, Sonar re-run owed). Still not addressed: the remaining S3776/S1192 smells (api baseline ratchet). This verifies the recorded measurement only. The scan item itself stays PROVISIONAL (below), and a gate of OK does not mean zero issues.
- Carried from 2.4.0, unchanged and not re-run for this candidate: the VERIFIED rows of `[2.4.0]` (stack 3 suites, pprof gate, UI rerun, SMB share/sub-directory live run on the `cattsrv-smb` test server, `docs/qa/smb-subdir-20261110/live-run-20261111T121104Z/`, checksums verify). Re-run on the candidate before a non-pre-release tag (11.4.135).

### PROVISIONAL (landed on `main`, review GO-with-follow-ups or shim/unit-class evidence; counts as done for planning, NOT verified)
Pending job per row; machine evidence goes under `docs/qa/`; due before any `v2.4.1` tag. Verifier commands are in `docs/PROVISIONAL_VERIFICATION_PLAN.md`.
- **SMB hardening SMB-FU-10 / 12 / 14** (reserved share names and NTFS alternate-stream / device-name path checks, per-host configured ports bound to vetted IPs at dial time, locked test globals; Opus round 5 GO-with-follow-ups; follow-up fixes F1, F2, F4 in `15a5735c`: no false malformed warning for port 445/0, credential-like host redacted in the warning). Evidence is unit-class only (hand-written fakes, no live SMB run through `dialVetted`). Pending job: live `TestConnection`/`BrowseShare` against a bare-IP SMB target (test server :1445 with `SMB_LIVE_PORT` and `CATALOG_SMB_ALLOWED_PORTS`), N>=3.
- **SMB subdirectory roots on the Anton host Samba, share `Public`, sub `Testing`** (CAT-SMB-SUBDIR-1): one live run recorded on `main` `e5817850`, 2026-11-11T19:03:25Z, `TestLiveSMBSubdir` (4 outside-name probes) and `TestLiveSMBServiceSubdir` PASS. Scope: real host Samba, reached by host name from an env file; NOT a bare-IP proof, NOT Synology. Evidence: `docs/qa/smb-subdir-20261110/live-run-anton-20261111T190325Z/` (log committed in `652928dc`, `sha256sum -c go_test.log.sha256` passes (run from repo root); single run, N=1). Why it is not VERIFIED: N=1 < required N>=3. Pending job: two more runs of `docs/qa/smb-subdir-20261110/run.sh <0600 env>` with committed logs.
- **QA stack scripts migrated to the Containers submodule via ctrctl** (STACK-CTR-FU-6..13: `stack3/up.sh` and `sonarqube/up.sh` start containers through `ctr_up` / `qa_stack_start`; static raw-start guard, create-failure and golden-create-argv tests, name guard before `podman create`, backend cache integrity, `errexit` and podman rc propagation fixes; Opus rounds GO-with-follow-ups). Evidence is shim-verified. Live: one SonarQube start through the Containers backend from the main checkout was observed 2026-11-11 (`qa_stack: backend=containers ctrctl_sha256=b6f69c8e...`, `catsonar-db` healthy, SonarQube UP, `down.sh` left 0 `catsonar` containers; CAT-QA-SONAR-1b) but its log was not committed (scratch), so it is PROVISIONAL. The `stack3` (`catqa3`) `up.sh` has not been run live through this path. Pending job: a live `stack3/up.sh <tag>` and `sonarqube/up.sh` run with the `backend=` line and `podman ps` captured under `docs/qa/`.
- **QA stack lifecycle follow-ups LIFE-FU-5..18** (health-inspect hang bounded, `unknown` state instead of false HEALTHY, start pre-flight for missing `timeout`, ctrctl exit 69 mapped to 1, errexit-safe return codes in `podman_env.sh`, local `IFS`, last-state in the timeout message, tests plus killed mutants). Shim-verified; a live lifecycle run on this tip is owed.
- **SonarQube run3 scan** (CAT-QA-SONAR-1 / -2): scan evidence committed; the run's own note records `backend=podman DEVIATION` because it ran from a scratch clone without the `containers` submodule checked out (a scratch-clone artefact, not a host limit). PROVISIONAL-DONE for the scan; issue triage (CAT-QA-SONAR-2) is OPEN; fix status: the 5 web S2245 vulnerabilities and 15 web bugs have a fix landed (merge `f9932aec`), the 2 API go:S3923 bugs have a fix landed (`79547302` via merge `b304304b`), and the web S3735/S3776 and api S1186 CRITICAL findings have a fix landed (merge `17aaa10a`); all PROVISIONAL, a Sonar re-run is owed to confirm clearance; remaining: api S1192/S3776 (648 issues) by baseline ratchet. Open follow-ups: CAT-QA-SONAR-3, -5 and -6 (see `docs/FOLLOW_UPS_NEXT_PHASE.md`). **S2699 fix:** a fix landed for the 16 assertion-free web tests in `765d6544` and `6b3d706e` (merged in `c80e35dd`) and `23b842d0` (direct follow-up commit on `main`) (Opus GO-with-follow-ups); PROVISIONAL, a Sonar re-run is owed to confirm clearance (follow-ups CAT-QA-SONAR-3).
- (LIFE-FU-19 is OPEN and appears only under open follow-ups.)
- **QA tooling and plans**: `docs/PROVISIONAL_VERIFICATION_PLAN.md` (the verification order and the v2.4.1 release checklist), CONTINUATION Rev 18 and Rev 19, follow-up registrations (STACK-CTR-FU-14, LIFE-FU-19). Documentation only.
- Every PROVISIONAL row of `[2.4.0]` that was not re-run in this window stays PROVISIONAL under its existing id (Tauri desktop and wizard native GUI boot, Android real login, UI D7 contrast, SMB discovery confinement and dial of the vetted IP, FTP `CopyFile`, cover-art SSRF through the product path, and the others listed there).

### REOPENED
- None recorded in this window: no live rerun FAILed on a previously closed item between `v2.4.0` and `1397d5b9`. This is the absence of recorded failures, not proof that nothing regressed (the full-suite retest has not been run).

### OVERDUE
- The `[2.4.0]` verification jobs carried a due date of "before the v2.4.0 tag"; that tag was published with the PROVISIONAL rows unverified, so every `[2.4.0]` PROVISIONAL row not promoted above is **verification OVERDUE** and is re-run first (order in `docs/PROVISIONAL_VERIFICATION_PLAN.md`). The rows added by this section have no due date yet other than "before any v2.4.1 tag".

### Open follow-ups
Authoritative list: `docs/FOLLOW_UPS_NEXT_PHASE.md`. New open items in this window: STACK-CTR-FU-1..5 and FU-14 (remaining raw podman calls, monitoring and s3 starts not migrated, detector gaps), LIFE-FU-19 (test gaps, OPEN), CAT-QA-SONAR-2 (triage), SMB-FU-9b, SMB-FU-13 and the rest of the SMB list.

### Skipped live tests
- **Synology SMB / FTP / NFS live tests remain SKIPPED (`hardware_not_present`); the drives have been powered off since 2026-11-05.** Synology is NOT live-verified for this candidate, no test double was substituted.

## [2.4.0] - pre-release (candidate, to be published as a labelled pre-release; PROVISIONAL items below are not verified)

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Candidate state, brought up to date with `main` `80d9b2d8` (previously computed at `17ec3c1f`; about 159 first-parent commits since 2026-10-28 plus the landings listed below; no git tag exists yet).
Version files: `versions.json` (global 2.4.0), `docs/api/openapi.yaml` (info.version 2.4.0). Components `catalog-web`, `catalogizer-desktop`, `installer-wizard` (package.json, tauri.conf.json, Cargo.toml) and the Android / Android TV `versionName` were already 2.4.0. Versioning is SemVer (operator decision 2026-11-03); the decimal-carry rule of 11.4.235(E.5) does not apply.

### VERIFIED (live evidence on QA stack 3, machine evidence under `docs/qa/`; each row ran on the image or commit named in that row, NOT on the release candidate; re-run on the candidate before a non-pre-release tag per 11.4.135)
- Stack 3 rebuilt from `main` `d9238b47`: 54 PASS / 0 FAIL / 1 SKIP, 3 identical rounds; Yandex WebDAV read-only suite 3/3. The Synology SMB/FTP read-only suites also ran 3/3 on `d9238b47` but are NOT counted as verified for this candidate: they predate the SMB-FU-2/SMB-FU-9 dial changes and the dead-SMB removal and Synology is powered off (see Skipped live tests); they are PROVISIONAL, pending job: re-run when Synology is back (`docs/qa/stack3-rebuild-live-20261105/`). Promoted to VERIFIED: conversion D1, D2, D10; PDF pages D3; document full-text search D4; D5; subtitle D6 and DELETE route; admin role change D9; D7, D8, D11a, D11b; storage-client live suites (SMB/FTP/WebDAV).
- NEW-1 cover thumbnails (batch endpoint): generated 2 then 0, `cover_art` rows on PostgreSQL (`docs/qa/new1-cover-thumbnails/green_main5c48.txt`).
- On image `main-5c480072` (`docs/qa/stack3-live-20261105/`): SECQ-1 WebDAV storage-root SSRF guard, SECQ-5 (GET 410, 401/403), SECQ-4/4b smart-collection cap (51 creates at 16 concurrency gave exactly 50 x 201 + 1 x 409, 3 rounds), subtitle DELETE non-owner 404, subtitle contract.
- Live pprof gate (SECQ-3 / CAT-QA-PPROF-1, 2026-11-11, stack 3 `catqa3-api` image `main-70d10721`): flag on gave 3 identical runs of 34 checks, 34 PASS / 0 FAIL (anonymous 401 on all six endpoints, scrape token 401, non-admin 403, admin 200, bad `seconds` 400, concurrent sampler 429, 60 s cap bounded); RED baseline with the flag off recorded (404); flag restored off afterwards. One host only (`docs/qa/live-pprof-20261111/`). This supersedes the earlier "SECQ-3 pprof is UNCONFIRMED live" note for that target.
- SMB share/sub-directory roots (CAT-SMB-SUBDIR-1): live run on the `cattsrv-smb` test server (share `corpus`, sub `corpus/nested`), 3/3 identical PASS, zeroed-sha negative control FAILs as required (`docs/qa/smb-subdir-20261110/live-run-20261111T121104Z/`). Caveat: the original Anton `Public/Testing` target was not run when this row was written (one Anton run was recorded afterwards, PROVISIONAL, see `[2.4.1]`).
- Live browser pass (`docs/qa/stack3-ui-20261105/`): 11 app pages render real data, website 31 internal pages OK, theme toggle. That run also recorded 5 FAILs (conversion crash, identities 404, entity spinner, 360px overflow, ws churn); all but UI-D4 were fixed and re-run in the rerun below.
- Live UI rerun on `main-1db4760f` (`docs/qa/ui-live-rerun-20261105/`, N=3 identical): D5/D6 axe, /admin at 360px, D8 search, /conversion, /identities, /entity/:id PASS.

### PROVISIONAL (landed on `main`, review GO-with-follow-ups; counts as done for planning, NOT verified)
Pending job for every row: the verifier named in the row runs on the rebuilt stack or device, N=3, machine evidence under `docs/qa/`; due before the `v2.4.0` tag. Command is the one recorded in `docs/CONTINUATION.md` section 3 unless stated.
- UI contrast D7 (class-wide dark-gray surfaces, rounds 1-2) and D7 remaining-contrast: target rebuilt web image on stack 3; command `node docs/qa/ui-d5d6-20261105/axe_live.cjs` (D7 check: `d7_axe.cjs`), expecting 0 violations in light and dark on /ai, /media, /analytics, /admin.
- UI-D1/D2/D3 (conversion page live shape, identities route, entity 404 handling), UI-D5/D6 accessible names, UI-D7/D8: listed here as PROVISIONAL (the live rerun above passed N=3 for D1, D2, D3, D5, D6, D8; but its own README does not mark them VERIFIED-DONE; D7 pending as above).
- SMB discovery confinement (SMB-FU-2, landed 2026-11-10/11, Opus GO-with-follow-ups): `TestConnection`/`BrowseShare` confined to the share/sub root, shared host vetting (bracket/colon forms, single resolve, all IPs, extended blocklist incl. 6to4/SIIT/NAT64/broadcast), port allowlist `CATALOG_SMB_ALLOWED_PORTS`, admin-share filter, generic errors. Evidence is unit-class; no live SMB run of the vetting paths.
- SMB dial of the vetted IP (SMB-FU-9, Opus GO-with-follow-ups): the dial uses the vetted address, never re-resolves the name; one shared connect deadline, at most 4 IPs; Tailscale/CGNAT `100.64.0.0/10` targets allowed (operator decision 2026-11-11). Unit-class tests only. SMB over a bare IP relies on reading the go-smb2 source (its `Dialer.Dial` takes an established connection); there is NO live SMB run through `dialVetted`.
- QA stack lifecycle (CAT-QA-STACK-LIFECYCLE-1, Opus GO-with-follow-ups on both rounds): root cause of the podman start failure `crun: sd-bus call: Operation not permitted` in harness shells is `,guid=<hex>` in `DBUS_SESSION_BUS_ADDRESS`; `docs/qa/lib/podman_env.sh` strips only `guid=`; start/stop wrappers go through the Containers submodule (`docs/qa/lib/ctrctl`, backend `containers`, podman fallback logged as a deviation); shim-based gate-logic tests (39 passed, 21 mutants killed) plus a probe on a throwaway container (`docs/qa/stack3/evidence-lifecycle-20261111/`). Not yet run against the real `catqa3-api`; `up.sh`/`down.sh`/`build.sh` still call podman directly. Follow-ups LIFE-FU-5..9 open.
- Dead SMB code removal (CAT-SMB-SUBDIR-1 / SMB-FU-1, 2, 15; Opus round 3 GO-with-follow-ups after two NO-GO rounds): removed the root `handlers` download/copy handlers, `CopyToSMB`/`ListSMBPath`/`GetSMBHosts`, the unwired `SMBHandler`, the `internal/smb` package and the `smb` client; linker symbol check on the base binary showed none of it reachable; the path-confinement guard tests were ported to `internal/pathsafe` with symlink-escape cases. Generated `catalogizer-desktop/src-tauri/gen/schemas/capabilities.json` untracked. Unit-class evidence; follow-ups SMB-FU-16, 19a..c open.
- Tauri guard hardening: wizard capability guard (`scripts/tauri/test_wizard_capabilities*.sh`, paired mutants, pre-flight cwd test) after review rounds 1-4, generated `gen/schemas/capabilities.json` untracked and replaced by tracked `capabilities/default.json`; static guard only, Opus GO-with-follow-ups.
- CAT-TAURI-NATIVE-1 (Tauri native boot, wizard dialog ACL fix): PROVISIONAL. No live GUI run is claimed here; the green live wizard run is owed and the binary was rebuilt since the earlier driver evidence. Pending job: native boot of desktop and wizard (`scripts/tauri/run-qa.sh`), N=3, recording + OCR; target desktop display session; due before the `v2.4.0` tag.
- Other landed since `17ec3c1f` with unit/tooling-class evidence only: TRUSTED_PROXIES hardening, Prometheus basic auth (CAT-PROM-AUTH-1), dynamic LAN IP (CAT-LANIP-DYN-1), `verify.py` LAN-bind and pgAdmin checks, on-demand SonarQube QA service, rootless live test servers, loopback trust guard, D7-R4 contrast follow-ups.
- Tauri desktop: QA boot tooling and login fixes, SSRF hardening (TAURI-FU-5), backend-enforced confirmation on server URL change: target native Tauri boot of the desktop app (container build exists; native boot not run); command per `docs/qa/` Tauri boot notes.
- Android and Android TV QA boot (JDK 17 build, emulators; evidence `docs/qa/android-boot-20261105/`): target emulator/device live run of the full banks.
- S3 storage protocol (aws-sdk-go-v2): unit-class evidence plus live N=3 on a real Garage v1.0.1 server only; target other S3 implementations.
- HelixQA stack 3 API smoke wrapper: API leg only; the UI Vision leg is not implemented (HELIXQA-FU-1).
- Authz and security series (SSRF resolvers, authz round 2, route permissions, CSRF LAN toggle, pprof/metrics gating SECQ-3/6, WebDAV hardening WD-F1, pathguard confinement series): unit and package-live evidence; SECQ-3 pprof is live-verified (see VERIFIED); NEW-1 SSRF-F2..F5 through the cover-art product path not yet live.
- Remaining landed series with review GO-with-follow-ups and no separate live rerun recorded: login/validator and password hashing, CAT-001 honest UI/API (R1-R6), CAT-002 isolated PDF text extraction, CAT-021 default credentials removal, smart collections, registration approval and converter tools, converter hardening, FTP `CopyFile` temp-name semantics (no live-NAS run), SQLite busy handler, Redis rate limiter, nil-slice `[]` serialisation, pool-deadlock class fix, live scaling/stress/chaos suite, docs link repair (111 links), runner scripts honest failure reporting.

### REOPENED
- None recorded since the last review (still true at `532c2ec8`: no live-rerun FAIL after the landings above). Any live-rerun FAIL reopens its item and is listed here at tag time (UI-D7 is the one live check that failed on the last rerun and has since been re-fixed, see PROVISIONAL).

### OVERDUE
- None: no verification job has a due date that has passed (no due dates were set before this candidate). Every PROVISIONAL row becomes overdue if it is still unverified when the tag is attempted; such a release would carry the label `verification OVERDUE`.

### Open follow-ups (not fixed; tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`, which is the authoritative list)
- SMB follow-ups open (SMB-FU-10/12/14 are in Opus review at tag time and ship in a later release): SMB-FU-9b (admin-configured by-name dials: storage roots, `SMBService.getConnection`, `smb/client.go`, `filesystem/smb_client.go` can still be rebound), SMB-FU-13 (`print$` shown in listing), SMB-FU-14 (no-`t.Parallel()` lint for global resolver tests), SMB-FU-10 (lexical-only path checks on plain shares), SMB-FU-11 (residual timing/leak distinctions), SMB-FU-12 (port allowlist vs configured storage-root ports). SMB-FU-15 (unwired `SMBHandler` returns `err.Error()`) is to be removed in a separate change.
- Live gaps: LV3-1 non-privileged cross-user conversion isolation (SKIP); LV3-2 storage idle-deadline and cut-recovery against real Synology (loopback/unit only); LV3-3 D5 bursty-audio fixture; LV3-4 stale fixture rows; LV3-5 PostgreSQL multi-process last-admin race; STK3L2-FU-1 no delete API for storage roots; STK3L2-FU-2 `/smb/discover` returns 500 on timeout.
- UI: UI-FAV-1 favourites link the wrong entity (pre-existing, important); UI-D4 `/ws` idle closes (unreproduced) and login double-connect; UI-D1-FU-1/2 conversion cancel verb and request shape; UI-D2-FU-1 identity CRUD routes missing; UI-D3-FU-1/4 retries on 404.
- Security residuals: path-confinement TOCTOU (F4, `os.Root` sinks) and hard-link policy (operator decision); secrets scanner residuals; key-rotation waiver is an operator-accepted open risk with no expiry.
- Process: container-hygiene guard parser gaps; HelixQA web Vision bank and CDP wiring (needs host capacity decision); upstream `helix_qa` credential option; operator decisions pending listed in the follow-ups file (amber podman, `thinker.local`, `catalogizer/server` image).
- Not part of this release: SonarQube analysis (blocked on image access).

### Skipped live tests
- **Synology SMB / FTP / NFS live tests are SKIPPED (`hardware_not_present`): the Synology drives were powered off by the operator on 2026-11-05 19:09 CET. Synology is NOT live-verified for this candidate.** No test double was substituted; the earlier read-only Synology suites (3/3 on `d9238b47`) predate the power-off. NFS was already `topology_unsupported` (no exports).

## [2.4.0-dev.20260414] - 2026-04-14 (historical heading, never tagged; folded into 2.4.0)

### Fixed
- **Goroutine leak**: `WorkerPool.SubmitAsync()` now tracks spawned goroutines via WaitGroup (catalog-api)
- **Goroutine leak**: `Throttler` run goroutine now tracked and awaited on Stop() (catalog-api)
- **Deadlock risk**: `Debouncer.Flush()` executes pending function outside the lock to prevent reentrant deadlock (catalog-api)
- **Race condition**: WebSocket handler uses reservation pattern to prevent over-admission under concurrent connections (catalog-api)
- **Resource leak**: SMBChangeWatcher.Stop() drains all debounce timers before closing stop channel (catalog-api)
- **Context propagation**: CacheService cleanup now cancels on service shutdown instead of using detached Background context (catalog-api)
- **Memory leak**: WebSocketContext no longer causes reconnection storms from stale dependency array (catalog-web)
- **Duplicate state update**: AuthContext consolidated from two permission effects into one (catalog-web)
- **Timer leak**: UploadManager clears pending timer before scheduling new one (catalog-web)
- **Sync on logout**: SyncManager.cancelSync() now cancels in-flight manual sync via Job tracking and ensureActive() (catalogizer-android)
- **Indefinite suspension**: MediaRepository.getRecentMedia/getPopularMedia use withTimeoutOrNull(5s) on Flow.first() (catalogizer-android)
- **DataStore contention**: AuthRepository shares a single DataStore flow via shareIn() instead of 7 independent collectors (catalogizer-android)
- **Room DB lifecycle**: CatalogizerDatabase.closeDatabase() companion method for proper cleanup (catalogizer-android)
- **Version alignment**: All component package.json/build.gradle versions aligned to 2.3.0 (all components)

### Added
- **Episode navigation**: Android TV VLCPlayerActivity.playNextEpisode/playPreviousEpisode now fetch real episodes via /api/v1/entities/:id/children API (catalogizer-androidtv)
- **React lazy loading**: Sub-component lazy imports for MediaPlayer, MediaDetailModal, UploadManager, Dashboard charts, 9 AI components (catalog-web)
- **Custom Semgrep rules**: `.semgrep.yml` with 17 security rules for Go SQL injection, XSS, command injection, secrets detection
- **Hadolint**: Dockerfile linting service added to docker-compose.security.yml
- **govulncheck integration**: Added to scripts/security-scan.sh for Go vulnerability scanning
- **Android SDK 35**: Dockerfile.builder now installs both SDK 34 and 35 platforms
- **AGENTS.md**: Created for 6 components (catalog-web, android, androidtv, desktop, api-client, wizard)
- **SQL Migration Reference**: Complete 1323-line docs/database/SQL_MIGRATION_REFERENCE.md covering all 15 migrations
- **Performance Tuning Guide**: 905-line docs/guides/PERFORMANCE_TUNING.md with concrete values from source code
- **Video courses**: 6 new course scripts (35 modules, ~24 hours content) for all core components
- **Architecture diagrams**: 4 new Mermaid diagrams (security-scanning-pipeline, lazy-init-flow, semaphore-control, test-coverage-matrix)
- **HelixQA banks**: Security-comprehensive (30 entries), DDoS-ratelimit-comprehensive (20 entries), benchmarking-baselines (15 entries), fixes-validation (29 entries)
- **Go benchmarks**: Benchmark tests for WorkerPool, Throttler, JWT, TitleParser, RateLimiter
- **Rust tests**: Tests for all installer-wizard and catalogizer-desktop Tauri backend modules
- **Test coverage**: Fixed 6 skipped catalog-web tests, completed API client tests, expanded TS submodule coverage

### Removed
- **Dead middleware**: Deleted advanced_rate_limiter.go, enhanced_rate_limiter.go (never used)
- **Deprecated stub**: Removed NewStubHandler() function
- **Dead hooks**: Deleted usePlayerState, useLazyImage, usePlaylistReorder, useVirtualScroll (unused)
- **Mock APIs**: Deleted mockCollectionsApi.ts, removed conversionApi.ts mock fallbacks
- **Unused config**: Removed CertFile/KeyFile from ServerConfig (self-signed certs generated at runtime)

### Changed
- **SonarQube**: Moved to profile-based activation (no longer auto-starts consuming resources)
- **.env.example**: AI/Vision env vars clearly labeled as HelixQA-only, SNYK_TOKEN added
- **Go submodules**: LLM modules bumped from Go 1.24.x to 1.25
- **useDebounce**: Wired into MediaBrowser search replacing inline debounce utility

## [2.3.0] - 2026-04-04

### Added

#### Media Entity System (Phases 1-10)
- Complete media entity aggregation pipeline: scanner post-scan hook triggers title parsing, MediaItem creation, hierarchy building, and duplicate detection
- 11 media types seeded in `media_types` table: movie, tv_show, tv_season, tv_episode, music_artist, music_album, song, game, software, book, comic
- Entity hierarchy with parent_id self-reference: TV Show -> seasons -> episodes, Music Artist -> albums -> songs
- Entity API (`/api/v1/entities`) with 17 endpoints: list, get, children, files, metadata, duplicates, stream, download, install-info, browse by type, entity types, entity stats, duplicate groups, user metadata, metadata refresh
- Title parser (`internal/services/title_parser.go`) with regex-based extraction for movie, TV, music, game, and software naming conventions
- AggregationService (`internal/services/aggregation_service.go`) for post-scan entity creation with automatic hierarchy building
- MediaItemRepository, MediaFileRepository, ExternalMetadataRepository, UserMetadataRepository, DirectoryAnalysisRepository for entity data access
- Entity browser frontend components in `catalog-web`: browse page, entity detail page, type-based navigation
- User metadata system: user ratings, watch status, favorites, personal notes, and tags per entity

#### Asset Management System
- Asset management submodule (`digital.vasic.assets`) integrated for cover art, thumbnails, and media artwork
- Lazy asset resolution pipeline: request -> background resolution -> WebSocket notification -> client refresh
- Three asset resolvers chained: CachedFileResolver, ExternalMetadataResolver, LocalScanResolver
- Default placeholder provider via `defaults.NewEmbeddedProvider()` for assets pending resolution
- Asset serving endpoint (`GET /api/v1/assets/:id`) with `X-Asset-Status` header (ready/pending)
- Asset event bridge: asset resolution events broadcast to WebSocket clients for real-time UI updates

#### Challenge-Based Testing Framework (CH-001 to CH-020)
- `digital.vasic.challenges` Go submodule for structured test scenario definition and execution
- 20 challenges covering infrastructure (CH-001 to CH-003), browsing (CH-004 to CH-005), assets (CH-006 to CH-007), auth (CH-008), scanning/storage (CH-009 to CH-015), and entity system (CH-016 to CH-020)
- Challenge REST API (`/api/v1/challenges`) with list, get, run single, run all, run by category, and get results endpoints
- Progress-based liveness detection with 5-minute stale threshold
- 117 assertions across all 20 challenges, all passing

#### Container Discovery Submodule
- `digital.vasic.containers` Go submodule for TCP-based service discovery
- Dynamic port binding: API server finds available port at startup, writes to `.service-port` file
- Frontend dev server reads `.service-port` for automatic API proxy configuration

#### Submodule Architecture
- 10 actively integrated Go submodules via `replace` directives in `go.mod`: Challenges, Assets, Containers, Concurrency, Config, Filesystem, Auth, Cache, Entities, EventBus
- 9 TypeScript/React submodules linked via `file:../` in `catalog-web/package.json`
- ADR-002 documenting submodule integration decisions and exclusion rationale for 10 unused Go submodules

#### HTTP/3 (QUIC) Support
- HTTP/3 server on UDP port 8443 via `quic-go/http3`
- HTTPS/HTTP2 server on TCP port 8443 as fallback
- Self-signed TLS certificate generation at startup for development
- Alt-Svc header advertising HTTP/3 availability to clients
- Brotli compression middleware (`andybalholm/brotli`) for all API responses

#### Advanced Statistics
- 9 statistics endpoints under `/api/v1/stats`: overall, per-storage-root, file types, size distribution, duplicates, duplicate groups, access patterns, growth trends, scan history
- Media browse endpoints (`/api/v1/media/search`, `/api/v1/media/stats`) backed by real database queries

#### Frontend Component Library
- React 18 with TypeScript, Vite, Tailwind CSS, React Query, Zustand
- Entity browser with type-based navigation, grid/list views, and entity detail pages
- Collection manager UI with create, edit, delete operations
- Dashboard analytics page with statistics visualization
- Media player components with subtitle support
- Path aliases: `@/components`, `@/hooks`, `@/lib`, `@/types`, `@/services`, `@/store`, `@/pages`, `@/assets`
- Build output split into vendor chunks: vendor (react), router, ui, charts, utils

#### Documentation (Phase 6)
- OpenAPI 3.0 specification (`docs/api/openapi.yaml`) covering all `/api/v1/*` endpoints with Bearer JWT auth, request/response schemas, and proper HTTP status codes
- ADR-001: Dual Database Dialect (SQLite + PostgreSQL abstraction)
- ADR-002: Submodule Architecture and Integration Decisions
- ADR-003: HTTP/3 (QUIC) with Brotli Compression Requirement
- ADR-004: Challenge-Based Testing Framework
- ADR-005: Container-Only Build and Runtime Policy
- ADR-006: Zero Warning / Zero Error Policy
- Comprehensive CHANGELOG entries for all post-1.0.0 work

### Changed
- Database migrations expanded to 8 versions (v7 = challenge/scan tables, v8 = media entity tables)
- Universal scanner enhanced with post-scan aggregation hook for automatic entity creation
- Config precedence clarified: env vars > `.env` > `config.json` > defaults
- Challenge runner timeout set to 72 hours with 5-minute stale threshold for stuck detection
- `config.json` `write_timeout` increased to 900 seconds to support long-running challenge RunAll operations
- Containerized build pipeline uses `--network host` for SSL reliability

### Deprecated

### Removed
- 21 unused Go/Android submodules cleaned up (Auth, Cache, Database, Discovery, Media, Middleware, Observability, RateLimiter, Security, Storage, Streaming, Watcher, and Android-specific modules)
- Deprecated `smb_roots` table references replaced by `storage_roots` multi-protocol table

### Fixed
- Database schema and scanner foreign key issues in entity tables
- Auth mock types, API key generation, and LoginResponse field mismatches in test suite
- All failing tests resolved: Go 35/35 packages pass, Frontend 102/102 files (1643 tests) pass
- All 20 challenges passing with 117/117 assertions

### Security
- JWT secret generation: cryptographically secure random secret generated at startup when not configured
- Password hashing uses bcrypt with per-user salt
- Rate limiting on auth endpoints (5 requests/minute) and general API (100 requests/minute)
- Input validation middleware with configurable rules
- Request ID middleware for audit trail correlation
- Redis-backed distributed rate limiting (optional, falls back to in-memory)

---

## [1.0.0] - 2026-02-02

### Added

#### Multi-Platform Support
- Go/Gin backend API (`catalog-api`) with Handler, Service, and Repository layers
- React/TypeScript web frontend (`catalog-web`) with React Query state management
- Tauri/Rust desktop application (`catalogizer-desktop`) with IPC commands/events
- Tauri-based installer wizard (`installer-wizard`) for guided setup
- Kotlin/Compose Android application (`catalogizer-android`) with MVVM architecture
- Kotlin/Compose Android TV application (`catalogizer-androidtv`) with lean-back UI
- TypeScript API client library (`catalogizer-api-client`)

#### Protocol Support
- SMB/CIFS network share scanning with circuit breaker and offline cache
- FTP file system support
- NFS network filesystem support
- WebDAV protocol support with sync and backup capabilities
- Local filesystem scanning

#### Media Detection and Management
- Automated media detection pipeline: detector, analyzer, and metadata providers
- TMDB, IMDB, and other metadata provider integrations
- Comprehensive media type classification (movies, TV shows, music, games, software, and 30+ additional types)
- Duplicate detection with configurable hash algorithms (MD5, SHA256, SHA1, BLAKE3, quick hash)
- Quality profile comparison and ranking (4K/UHD, 1080p, 720p, DVD, etc.)
- Media collection grouping for series, albums, and franchises
- File rename detection system
- Advanced AI features for media analysis (Phase 3.2.8)
- Performance optimization system (Phase 3.2.7)
- Advanced collection management system (Phase 3.2.5-3.2.6)

#### Media Player Features
- Subtitle track management with multi-language support
- Subtitle sync verification and offset adjustment
- Subtitle download tracking and caching
- Audio track management for multi-language audio
- Chapter support for video bookmarks
- Cover art fetching from MusicBrainz, Last.fm, Spotify, iTunes, Discogs
- Lyrics data with synced lyrics support
- Playback session tracking with resume capability
- Playlist management including smart playlists
- Translation cache for subtitles and lyrics
- Language preference management per content type

#### Real-Time Features
- WebSocket-based real-time event bus for live updates
- Real-time media change notifications
- File system monitoring and change tracking

#### Authentication and Authorization
- JWT-based authentication with session management
- Role-based access control (Admin, Moderator, User, Viewer)
- User permission management with custom permission grants
- Authentication audit logging
- Account lockout protection with failed login tracking
- Session management with device tracking

#### Media Conversion
- Format conversion job queue with priority support
- Support for video, audio, image, and document formats
- Batch conversion with progress tracking
- Quality settings per conversion job

#### Database and Storage
- SQLite database with automatic migrations on startup
- PostgreSQL support for production deployments
- 6-version migration system with tracking table
- Virtual paths for unified file access across protocols
- Scan history tracking with detailed statistics
- External API response caching

#### Administration and Deployment
- Docker and Docker Compose support for development and production
- Comprehensive configuration via environment variables, .env files, and config.json
- Installation wizard with step-by-step guided setup
- Backup and recovery documentation
- Scaling guide for production environments
- Production runbook for operations teams

#### Documentation
- Architecture documentation for all components (Go backend, React frontend, Android, Tauri IPC)
- API documentation with endpoint reference
- WebSocket events documentation
- Database schema documentation
- User guides for web, desktop, Android, and Android TV
- Deployment guide with Docker setup
- Monitoring guide with metrics
- Contributing guide with code standards
- Configuration guide
- Troubleshooting guide
- Security testing guide
- QA testing guide and execution reports
- Test strategy documentation

#### Testing
- Comprehensive test suite across all modules (~1000+ tests across phases)
- Table-driven tests for Go backend
- Benchmark tests for performance-critical paths
- Security scanning and remediation (Phase 6)
- Cross-cutting error handling tests
- WebSocket integration tests
- Auth flow tests
- Accessibility tests for frontend

### Changed
- Migrated storage model from SMB-only (`smb_roots`) to multi-protocol (`storage_roots`)
- Updated media items schema for Android TV compatibility (migration 006)
- Refactored subtitle foreign keys from `media_items` to `files` table reference
- Reorganized root directory structure: moved ~65 files into config/, scripts/, tests/, docs/ subdirectories

### Fixed
- Critical build blockers and security issues (Phase 0)
- Memory leaks and race conditions (Phase 0)
- Subtitle foreign key references pointing to wrong table (migration 015/v6)
- Disconnected features across all modules (Phase 2)

### Security
- Security hardening and stability fixes (Phase 1)
- Security scanning and remediation across all modules (Phase 6)
- JWT token management with secure session handling
- Password hashing with salt
- Input validation with Zod on frontend
- CORS and rate limiting middleware
- Freemium security setup documentation

---

## Release History

This changelog was created as part of the Phase 8 documentation completion effort. Prior development was tracked through auto-commits and phase-based commit messages. Future releases will follow this structured format.

[Unreleased]: https://github.com/catalogizer/catalogizer/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/catalogizer/catalogizer/releases/tag/v1.0.0
