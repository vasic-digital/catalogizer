# Changelog

All notable changes to the Catalogizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.5.0] - 2026-11-17 (pre-release 2.5.0-rc.1 candidate, not production-ready)

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Prepared on branch `rc1ver` from `main` `630520890` (487 commits since the `v2.4.3` tag, 49 of them without a `Roadmap-Item` line; `git rev-list --count v2.4.3..630520890`; the 2.5.0 prep commit and the merge of main sit on top of that range). Not tagged, not published. The intended tag is `v2.5.0-rc.1`, but see the gate section: no tag may be cut yet. Version files carry `2.5.0` (not `2.5.0-rc.1`) because `scripts/release/check_release_consistency.sh` compares every field with one `X.Y.Z` taken from `versions.json` global (major.minor.patch integers, which cannot hold a pre-release suffix); the pre-release identity is carried by the tag and by this heading. Bumped to 2.5.0: `versions.json` (via `bump_version minor` of `Build/lib/version.sh`), `docs/api/openapi.yaml`, `catalog-web`, `catalogizer-desktop` and `installer-wizard` (package.json, package-lock.json, tauri.conf.json, Cargo.toml), Android `versionCode` 10 (was 9), Android TV `versionCode` 12 (was 11), both `versionName` 2.5.0, the two Android `CLAUDE.md` version lines, the installer-wizard AppImage file name in `challenges/scripts/desktop_appimage_launch_challenge.sh`, and the version mentions in `README.md` (components line), `docs/CONTRIBUTING.md` and `docs/DEPLOYMENT_GUIDE.md`. Not touched: `Cargo.lock` files, `build_number` in `versions.json` (still 25), the README release-status banner (it describes the published 2.4.3).

### What changed for users (summary)
- **Per-user favorites.** A favorite is now yours alone: `PUT /api/v1/media/:id/favorite` stores a per-user row (new table `user_favorites`, schema migration v30) instead of a global flag that every user could see and overwrite. Old global flags are not migrated (they never recorded an owner) and are ignored; users re-mark their favorites. The request body field is `favorite`.
- **New read route.** `GET /api/v1/entities/:id/user-metadata` returns only the caller's own user metadata (a missing or foreign row is one 404).
- **Sync destinations.** Creating or updating a sync destination on a private network without confirmation now answers **400** (was 409; the error code `private_network_confirmation_required` is unchanged), the same as storage roots. The `/sync/*` GET routes stay **403 admin-required** for non-admins (decision: keep 403, the earlier 404 change stays reverted).
- **Security and correctness:** authentication, WebSocket, egress, error-leak and redaction hardening carried from 2.4.3 onward, last-admin protection on SQLite and PostgreSQL, duplicate email/name answers 409 instead of 500, Retry-After on 429, SMB `CopyFile` never truncates the destination before the source is read, NFS READDIR fallback. Details in the lists below.
- **Upgrade:** back up the database first; there is no downgrade. See `docs/UPGRADE_NOTES.md`, section "Upgrading from 2.4.3 to 2.5.0".

### Release gate (constitution 11.4.298, stated first)
Gate state per product, each **`BLOCKED-NO-HARDWARE`** (11.4.298(D): a product without a live-run record is `BLOCKED-NO-HARDWARE`):
- `api`: `BLOCKED-NO-HARDWARE`
- `web`: `BLOCKED-NO-HARDWARE` (the v2.4.2 baseline in `docs/qa/baseline-v2.4.2-20261114/` does not change this: there is no rc.1 live run to compare against it, so no 11.4.293 comparison exists)
- `desktop`: `BLOCKED-NO-HARDWARE`
- `wizard`: `BLOCKED-NO-HARDWARE`
- `Android`: `BLOCKED-NO-HARDWARE`
- `Android TV`: `BLOCKED-NO-HARDWARE`

**Under 11.4.298(E) no tag of any class, `v2.5.0-rc.1` included, may be cut until a live run exists for every product the tag covers.** Nothing here may be deployed. The 11.4.40 full-suite retest of the candidate, the 11.4.185 manual QA and the live UI QA have NOT been run. This is a labelled pre-release candidate and must not be called production-ready.

### State summary by label (11.4.290(G))
V = 0 items verified in this candidate. P = 240 distinct `Roadmap-Item` ids, derived from the `Roadmap-Item:` trailer lines of `git log v2.4.3..630520890` (`%B` bodies, `grep '^Roadmap-Item:'`, parentheticals dropped, split on commas and whitespace, `..` ranges expanded, `sort -u`; 487 commits, 49 carry no `Roadmap-Item` line), minus `CAT-SYNC-404-1` (decided, see below). V counts verified items and P counts items. R = 0 reopened items identified in this range (UNCONFIRMED: not cross-checked against the tracker database). O = 2 overdue verification jobs, named below. Nothing below is claimed fixed or verified.

### VERIFIED
- None. The `docs/qa/img-slim/` evidence set (CAT-IMG-SLIM-1 minimal-ffmpeg image) is carried evidence: its `SHA256SUMS.txt` hashes verify with `sha256sum -c`, it was not re-run on this candidate, and it verifies no behaviour of this candidate.

### REOPENED
- None identified in this range (UNCONFIRMED, see above).

### OVERDUE
- `VJ243-21` (documentation tooling and README reachability; `docs/PROVISIONAL_VERIFICATION_PLAN.md` section 6) was due 2026-11-13 and has not been run: **verification OVERDUE**.
- `VJ243-22` (research and records review; same plan) was due 2026-11-13 and has not been run: **verification OVERDUE**.
- Other `[2.4.3]` jobs carried from earlier sections remain open; see `[2.4.3]` below.

### DECIDED (not provisional)
- `CAT-SYNC-404-1`: operator decision, the `/sync/*` routes keep **403** for non-admins; the earlier 404 change stays reverted. Closed as a documented contract (docs only; `docs/api/SYNC_API.md`, `docs/api/openapi.yaml`).

### Follow-ups noted for this section
- The `[2.4.3]` section below still carries the gate label `BLOCKED-NO-BASELINE`, which does not match the per-product `BLOCKED-NO-HARDWARE` state of 11.4.298(D); left as published and to be corrected separately.

### Landed since the first prep draft (all PROVISIONAL, from `main` `630520890`)
- `CAT-HAUTHZ-F2`: per-user favorites (`user_favorites`, migration v30; `PUT /api/v1/media/:id/favorite`, `GET /media/:id` returns the caller's state only; review follow-ups: per-item, cascade and populated-v29 upgrade tests, 401 before body parse, openapi 400/401).
- `CAT-HAUTHZ-O1`: caller-scoped user-metadata read, new route `GET /api/v1/entities/:id/user-metadata`; the legacy unrouted handler now filters by `user_id`.
- `CAT-CPN1-F2` (commit message id `CAT-CPN1-F2-SYNC-400`): sync private-network confirmation answers 400, client-visible change from 409.
- `SMBCOPYR` follow-ups F1-F7 (cleanup context, temp location, remove-failure cleanup, backslash same-path, live temp cleanup) on `CAT-SMB-COPY-1`.
- `CPN1R2` follow-ups (openapi partial-500 fields, blocked-IP-first test, `ErrInvalidSMBInput` to 400, dead field dropped, doc fix) on `CAT-CPN-1`.
- Sync stays 403 admin-required (`CAT-SYNC-404-1`, above).

### PROVISIONAL (every item below; commits since v2.4.3 grouped by `Roadmap-Item`, latest commit subject shown; PROVISIONAL means implemented and wired, verification pending)
- `ADMD1-F1` (2 commits; latest 68396c78c): Merge admd1f1-fu: structural admin-403 classifier (no after-quote negation; ambiguity resolves to gated) (ADMD1-F1; Opus GO-with-follow-ups round 5; PROVISIONAL)
- `ADMD1-m1` (3 commits; latest 5ba34385f): Merge ADMD1-m1..m4: ADMIN_GUIDE/openapi corrections (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1F1-S1` (2 commits; latest b8e00e929): test(admd1f1-s1): route-table cross-check, 12 more admin ops marked (85), loader hardening (Opus round 1 F1-F5)
- `ADMD1R-m1` (3 commits; latest ffafd85f4): Merge ADMD1R-m1/m2/n3-n6: PG duplicate role name 409, session-revocation docs (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1R-m2` (2 commits; latest ffafd85f4): Merge ADMD1R-m1/m2/n3-n6: PG duplicate role name 409, session-revocation docs (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1R-n3` (2 commits; latest ffafd85f4): Merge ADMD1R-m1/m2/n3-n6: PG duplicate role name 409, session-revocation docs (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1R-n4` (2 commits; latest ffafd85f4): Merge ADMD1R-m1/m2/n3-n6: PG duplicate role name 409, session-revocation docs (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1R-n5` (2 commits; latest ffafd85f4): Merge ADMD1R-m1/m2/n3-n6: PG duplicate role name 409, session-revocation docs (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1R-n6` (2 commits; latest ffafd85f4): Merge ADMD1R-m1/m2/n3-n6: PG duplicate role name 409, session-revocation docs (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RR-ma` (2 commits; latest ae1d2ae45): Merge ADMD1RR-md/ma: shared dialect-neutral unique-violation classifier, user update 409 on PostgreSQL (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RR-md` (3 commits; latest ae1d2ae45): Merge ADMD1RR-md/ma: shared dialect-neutral unique-violation classifier, user update 409 on PostgreSQL (Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRR-I1` (2 commits; latest 66666daf0): Merge admd1rrr-i1: duplicate-email/name 409 on admin update, register, storage-root (ADMD1RRR-I1; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRR-m1` (1 commit; latest ad7885a65): fix(admin,auth,storage): duplicate email on admin user update, racing registration and racing storage-root create answer 409 on SQLite and PostgreSQL, not 500 (ADMD1RRR-I1/m1/m2) (PROVISIONAL)
- `ADMD1RRR-m2` (1 commit; latest ad7885a65): fix(admin,auth,storage): duplicate email on admin user update, racing registration and racing storage-root create answer 409 on SQLite and PostgreSQL, not 500 (ADMD1RRR-I1/m1/m2) (PROVISIONAL)
- `ADMD1RRRR` (3 commits; latest 867eb185f): Merge admd1rrrr-fu: storage-root 409 existing_id/hint, non-unique negative controls, PG race leg, openapi 409 wording (ADMD1RRRR-m1..m4,n1,n2; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRR` (3 commits; latest fa6dc5f93): Merge admd1rrrrr-fu: deterministic insert-path 409 test seam, StorageRootConflict schema, no-id hint (ADMD1RRRRR-m1,n1,n2,n3; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRRR` (3 commits; latest 7e87aab95): Merge admd1rrrrrr-fu: provable pre-check order, failed-winner-lookup handler test, oneOf 409 schemas, admin-update wording (ADMD1RRRRRR-m1,m2,n1,n2; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRRRR` (3 commits; latest fce3c0671): Merge admd1rrrrrrr-fu: after-conflict hook assertion, real 409 bodies vs openapi oneOf contract test, not-required-code + enum, winner id assert (ADMD1RRRRRRR-m1,m2,n1,n2; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRRRRR` (3 commits; latest f3b4ed96c): Merge admd1rrrrrrrr-fu: oneOf exclusivity control, unknown-keyword guard, duplicate 403 merge (ADMD1RRRRRRRR-m1,n1,n2; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRRRRRR` (3 commits; latest 248e3dcf5): Merge admd1rrrrrrrrr-fu: admin403 classifier, sync 403 body pins, validator guard extension (ADMD1RRRRRRRRR-m1,m2,n1,n2; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRRRRRRR` (4 commits; latest 155348976): Merge admd1rrrrrrrrrr-fu: pin test drives real SyncHandler and live adminCheck, int32 boundaries, classifier ignores negated quotes (ADMD1RRRRRRRRRR-m1,n1..n5; Opus GO-with-follow-ups; PROVISIONAL)
- `ADMD1RRRRRRRRRRR` (5 commits; latest 4cc6817c6): test(admd1f1-fu): drop after-quote negation entirely (ambiguity resolves gated), fallback strips only body-marked quotations (ADMD1RRRRRRRRRRR round 5)
- `ADMINDOC-1` (3 commits; latest 3f2a544a1): Merge ADMINDOC-1: ADMIN_GUIDE lock/corrupt-role docs (Opus GO-with-follow-ups; PROVISIONAL)
- `BG-OPQ-2026-11-14b` (2 commits; latest 07255d30d): docs: record sweep 4 operator answers
- `BG-OPQ-2026-11-15` (1 commit; latest b6202942b): docs: record BG-OPQ-2026-11-15 operator decision sweep
- `BG-OPQ-2026-11-16` (2 commits; latest 4288c6a82): docs: record BG-OPQ-2026-11-16 operator answers
- `BG-OPQ-2026-11-16b` (2 commits; latest 1391b0ff9): docs: record operator answers of decision sweep 7 (BG-OPQ-2026-11-16b)
- `CAT-CONSTITUTION-BUMP` (1 commit; latest 8e74497b7): chore: bump constitution to e3f156617 (PR #7 merged; operator approved 2026-11-16)
- `CAT-CONSTITUTION-BUMP-2` (1 commit; latest c9ce429c8): chore: bump constitution e3f156617 -> 9f9f808b (PR #8..#11: label-sync hooks, F9/F9b/L8, N-04 port, F3a registry-login mutant)
- `CAT-CONSTITUTION-BUMP-3` (1 commit; latest 95248de32): chore: bump constitution 9f9f808b -> cc51c320 (CON-423 secscan deadlock fix, tracker follow-ups)
- `CAT-CONSTITUTION-BUMP-4` (1 commit; latest 1569638d2): chore: bump constitution cc51c320 -> 93465196 (PR #13: constitution-cmd mutation test sharded)
- `CAT-CONSTITUTION-BUMP-5` (1 commit; latest 82d05777d): chore: bump constitution 93465196 -> cb7008c4 (CON-425 history-scan replace-ref fix, CON-435..442 follow-ups)
- `CAT-CONSTITUTION-BUMP-6` (1 commit; latest 5a2be8541): chore: bump constitution cb7008c4 -> 1dfbb4d1 (CON-435 gitleaks fix, drift-seam landing, CON-443..478 follow-ups)
- `CAT-CPN-1` (6 commits; latest a807a7b30): fix+test: CPN-1 review follow-ups m1-m4, n1 (PROVISIONAL)
- `CAT-GENERIC-HOSTS-1` (7 commits; latest 9b4651f58): docs: keep disclosure note free of the vendor name (PROVISIONAL)
- `CAT-HAUTHZ-1` (10 commits; latest 45a1adc6e): CAT-HAUTHZ-1 follow-ups 3: evidence (RED/GREEN x3, mutants M3-M8), SHA256SUMS regenerated (PROVISIONAL)
- `CAT-HAUTHZ-F2` (2 commits; latest 3b1bae3c1): CAT-HAUTHZ-F2 review follow-ups: per-item, cascade and populated-v29 upgrade tests, 401 before body parse, openapi 400/401, docs corrections (PROVISIONAL)
- `CAT-HAUTHZ-O1` (1 commit; latest 7d0271c66): CAT-HAUTHZ-O1: caller-scoped user-metadata read, routed (PROVISIONAL)
- `CAT-KEYROT-WAIVER-1` (2 commits; latest 632c3d0b3): Merge CAT-KEYROT-WAIVER-1 permanent-risk waiver kind (Opus GO-with-follow-ups; PROVISIONAL)
- `CAT-MW-TIERTEST-1` (3 commits; latest 6546bc75e): Merge cat-mw-tiertest-1-rlw-fu2 (tier test, Retry-After on RateLimiter and pprof 429; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `CAT-NFS-EMU-2` (3 commits; latest b96303dbb): Merge CAT-NFS-EMU-2: live unfs3 emulator leg for pkg/nfs3 (Opus GO-with-follow-ups; PROVISIONAL, EMULATOR)
- `CAT-NFS3-FU2` (1 commit; latest 0b0beb752): test: NFS factory leg sends all float64 ports; command grep + CRLF/fraction/col-0 fixtures (NFS3FU2-M1/M2/N1/N2)
- `CAT-PG-LASTADMIN-1` (6 commits; latest 0422accac): docs: register CAT-PG-LASTADMIN-1 review follow-ups
- `CAT-QA-BASELINE-V242` (2 commits; latest 9e624f09f): Merge qa/baseline-v2.4.2 (first live baseline of v2.4.2 for product web; Opus round 2 GO-with-follow-ups)
- `CAT-QA-SONAR-CRIT-1` (1 commit; latest 7eb68ec26): docs: record operator Sonar gate scope (G5 = bugs/vulns/hotspots) and register CAT-QA-SONAR-CRIT-1
- `CAT-REL-2.5.0-GATES` (9 commits; latest c42e0e216): docs: register gate checklist Rev 4 review follow-ups
- `CAT-SECQ1-C` (5 commits; latest 9343681d9): Merge CAT-SECQ1-C W4 step-up limiter + approved class on live WebDAV path (Opus GO-with-follow-ups; PROVISIONAL)
- `CAT-SMB-COPY-1` (6 commits; latest 2c3c117d8): fix(smb): CAT-SMB-COPY-1 review follow-ups m1-m5 (clean same-path guard, fresh-ctx temp cleanup, refuse read-only dest, copyFS seam + mutant-killing tests) PROVISIONAL
- `CAT-SSE-FU2R` (1 commit; latest 8d0440abf): test: pin RequireAdmin (false,err) 503 body + positive control (SSEFU2R-m1/m2)
- `CAT-SYN-LIVE-1` (1 commit; latest 2846f4bef): docs(qa): live real-NAS evidence 2026-11-15 (PROVISIONAL)
- `CHPW-RL-1-F1` (1 commit; latest 19fdf45ef): test(auth): drive real limiter tiers through the real router; extract newAuthRateTiers (PKGC-FU4-F1, CHPW-RL-1-F1/F2)
- `CHPW-RL-1-F2` (1 commit; latest 19fdf45ef): test(auth): drive real limiter tiers through the real router; extract newAuthRateTiers (PKGC-FU4-F1, CHPW-RL-1-F1/F2)
- `ELFU-m1` (4 commits; latest fa2e7b058): Merge ELFU-m1: errleak pins exact services path, closes bare-ScrubErrorFor bypass (Opus round-2 GO; PROVISIONAL)
- `ELFUR-m1` (2 commits; latest 0662d0459): Merge PGR3R-m1/m2/n3 + ELFUR-m1: escaped cgo fixture, unquote-error-as-cgo, tight errleak name heuristics (Opus GO-with-follow-ups; PROVISIONAL)
- `ENV-F1` (3 commits; latest 5cf3e9ffa): Merge ENV-F1: SMB tests no longer depend on host-own or real LAN addresses (Opus GO-with-follow-ups; PROVISIONAL)
- `ENVR-i4` (1 commit; latest 97504de96): docs: register ENV-F1 review follow-ups
- `ENVR-m1` (3 commits; latest afd5524c0): Merge PGR3RR-m1/m2/n3 + ENVR-m1: real unicode-escape assertion, e.Err pin, SMB fixture cleanup (Opus GO-with-follow-ups; PROVISIONAL)
- `ENVR-n2` (1 commit; latest 97504de96): docs: register ENV-F1 review follow-ups
- `ENVR-n3` (1 commit; latest 97504de96): docs: register ENV-F1 review follow-ups
- `ERRLEAK-FU-A` (3 commits; latest a3ea474e3): Merge ERRLEAK-FU: narrowed GetStorageRootStatus pin, TestConnection redaction test, S6 fixture (Opus GO-with-follow-ups; PROVISIONAL)
- `ERRLEAK-FU-B` (2 commits; latest a3ea474e3): Merge ERRLEAK-FU: narrowed GetStorageRootStatus pin, TestConnection redaction test, S6 fixture (Opus GO-with-follow-ups; PROVISIONAL)
- `ERRLEAK-FU-C` (3 commits; latest 2ab31e583): Merge ERRLEAK-FU-C/D: errleak checker callee identity (Opus GO-with-follow-ups; PROVISIONAL)
- `ERRLEAK-FU-D` (3 commits; latest 2ab31e583): Merge ERRLEAK-FU-C/D: errleak checker callee identity (Opus GO-with-follow-ups; PROVISIONAL)
- `ERRLEAK-FU-E` (2 commits; latest a3ea474e3): Merge ERRLEAK-FU: narrowed GetStorageRootStatus pin, TestConnection redaction test, S6 fixture (Opus GO-with-follow-ups; PROVISIONAL)
- `EXAMPLES-F7-FALLOUT-1` (3 commits; latest 7acba1140): Merge EXAMPLES-F7-FALLOUT-1: example sqlmock for tx-wrapped Delete (Opus GO; PROVISIONAL)
- `F2F3-m1` (3 commits; latest 44ec831b6): Merge F2F3-m1..m3: fail-closed conversion gates + nav/route tie test (Opus GO-with-follow-ups; PROVISIONAL)
- `FU-TESTS-A` (3 commits; latest ea4a53757): Merge FU-TESTS-A test strengthening (Opus GO-with-follow-ups; PROVISIONAL)
- `FU-TESTS-B` (2 commits; latest 2c93ed933): Merge FU-TESTS-B CORS case, nearNow time pin, web state waits (Opus GO; PROVISIONAL)
- `FU10` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `FU11` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `FU2-1` (2 commits; latest 4b9350efb): qa: correct PW-F4 conclusion (plugin-react adds dedupe), commit variant reproducer, tighten FU2-1 controls
- `FU7` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `FU8` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `FU9` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `FUTB-F1` (3 commits; latest 11c25039b): Merge FUTB-FU: tighter nearNow bound, state-based web wait (Opus GO; PROVISIONAL)
- `FUTB-F2` (2 commits; latest 11c25039b): Merge FUTB-FU: tighter nearNow bound, state-based web wait (Opus GO; PROVISIONAL)
- `FUTB-N3` (1 commit; latest d218a55b4): test: WEB-TEST-BUNDLE-1 route-gate golden cases, header wording, FUTB-N3 timer advance
- `GATECHK5` (1 commit; latest e6e89cf18): docs(release): gate checklist Rev 5 + rc.1 audit Rev 2 (11.4.298 live-run precondition, re-measured at b0066508a; review I-1, m-1..m-5, n-1..n-5; PROVISIONAL)
- `HANDLERS-RED-1-F3` (1 commit; latest aeaea77f6): logredact: route-param secrets in any path segment and decoded %2F tokens no longer reach the path log field
- `HYG-F3` (3 commits; latest 934fa382a): Merge hyg-f3-f8 (access-log route-param redaction HYG-F3, error-text HYG-F8; Opus round 3 GO-with-follow-ups; PROVISIONAL)
- `HYG-F4` (4 commits; latest 290c73189): Merge HYG-F4 + SSEFU2 tests (Opus GO-with-follow-ups; PROVISIONAL)
- `HYG-F6` (1 commit; latest 638f49d8d): fix(logredact): ReplaceKnown also removes the %q-quoted form of a secret (HYG-F6)
- `HYG-F8` (1 commit; latest 4de55eb77): test(handlers): PXF17R1 redaction tests exercise the production ErrorTextKnown path (HYG-F8)
- `HYG-R1-F1` (1 commit; latest dbc97863b): fix(middleware): canonical per-segment path for all log sinks; sink and buildRouter tests (HYG-R1-F1, F2, F3)
- `HYG-R1-F2` (1 commit; latest dbc97863b): fix(middleware): canonical per-segment path for all log sinks; sink and buildRouter tests (HYG-R1-F1, F2, F3)
- `HYG-R1-F3` (1 commit; latest dbc97863b): fix(middleware): canonical per-segment path for all log sinks; sink and buildRouter tests (HYG-R1-F1, F2, F3)
- `HYG-R1-F4` (1 commit; latest b0a5bb1b8): fix(logredact): ReplaceKnown also removes the %+q (QuoteToASCII) form (HYG-R1-F4)
- `HYG-R2-F1` (1 commit; latest a88d53fa2): fix(router): pin AccessLogRouteParams in the global middleware chain lists (HYG-R2-F1)
- `HYG-R3-F1` (7 commits; latest 608843b1b): Merge HYG-R3-F1/F2 (Opus round 5 GO-with-follow-ups; PROVISIONAL)
- `HYG-R3-F2` (5 commits; latest eb875875d): fix(logredact,middleware): %2F key shapes in the gin formatter; punctuation-only token (HYG-R3-F1 review round 4)
- `HYGF4-F1` (4 commits; latest 28fbbe538): Merge HYGF4-F1 logredact invalid-escape fail-closed fix (Opus GO-with-follow-ups; PROVISIONAL)
- `HYGF4B` (4 commits; latest ad6803ec5): Merge HYGF4B logredact cap/key-word/alloc fixes (Opus GO-with-follow-ups; PROVISIONAL)
- `HYGF4C` (3 commits; latest 70d3afefd): Merge HYGF4C logredact slot-refill fix (Opus GO-with-follow-ups; PROVISIONAL)
- `HYGF4D` (3 commits; latest 106f4bc38): Merge HYGF4D logredact determinism + aliasing/case tests (Opus GO-with-follow-ups; PROVISIONAL)
- `MAIN-ERRLEAK` (4 commits; latest a05689cf5): Merge MAIN-ERRLEAK: errleak sweep green on main (Opus GO-with-follow-ups; PROVISIONAL)
- `MAIN-EXAMPLES` (1 commit; latest 019278272): test(examples): DeleteUser_ProtectedLastAdmin binds the real 6-arg guarded DELETE (time cut-off matcher)
- `MAIN-POOLGUARD` (1 commit; latest f09fa5d25): docs: register MAIN-EXAMPLES landing and MAIN-POOLGUARD hold
- `MAIN-POOLGUARD-R3` (5 commits; latest f920383a2): Merge MAIN-POOLGUARD-R3: go/types poolguard successor (Opus round-3 GO-with-follow-ups; PROVISIONAL)
- `NFS3-READDIR` (3 commits; latest 37138396d): Merge nfs3-readdir: plain READDIR fallback in pkg/nfs3 + pin filesystem 6d89c22 (NFS3-READDIR; Opus GO-with-follow-ups; EMULATOR+unit evidence; PROVISIONAL)
- `NFS3-WIRE-1` (11 commits; latest b9f6cd4a3): Merge nfs3w-fu2: port range contract, factory contract test, emulator log check hardening (NFS3-WIRE-1 follow-ups; Opus GO-with-follow-ups; PROVISIONAL)
- `NFS3RD` (1 commit; latest b2b09e8b2): chore: pin filesystem 0a13554 (NFS3RD follow-up tests, FallbackMaxEntries; Opus GO-with-follow-ups; unit evidence; PROVISIONAL); register NFS3RDFU follow-ups
- `NFS3RDFU` (1 commit; latest b4902c4e6): chore: pin filesystem 50b5e75 (NFS3RDFU attempted-entry cap, negative cap rejected; Opus GO-with-follow-ups; unit evidence; PROVISIONAL); register NFS3RDFUFU follow-ups
- `NFS3RDFUFU` (1 commit; latest 7ae234105): chore: bump submodules/filesystem 50b5e75 -> 7b58084 (NFS3RDFUFU-m1..n3; Opus GO-with-follow-ups; PROVISIONAL); register NFS3RDFUFUFU follow-up
- `NFS3W-FU2` (1 commit; latest 5b978c4fb): NFS3W-FU2 follow-ups: port-order/float64 JSON port tests, withNFS3 helper, strict emulator PASS-line shape
- `NFSE2-follow-ups` (3 commits; latest 53c08e927): Merge NFSE2-follow-ups: IMG-INFRA-NFS lock entry, checker map, nfs_build note (Opus GO-with-follow-ups; PROVISIONAL)
- `OBF-FU-F1` (1 commit; latest f7bf38a03): wizard: relative CONFIG_PATH no longer refuses an absolute in-config symlink target
- `PGLA-M1` (3 commits; latest db8957b0d): Merge PGLA-M1 + TIERPGLA-F3: in-tx re-read live test, rate-limit header comment (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLA-M1-F1` (3 commits; latest 82dec7a41): Merge test-hygiene bundle: PG role leak, F6 -count, R5/N3/N4, api-client getPermissions cases (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLA-M1-N1` (1 commit; latest 88566cb50): test(PGLA-M1): drop LOGIN role after database drop with checked errors, pin full advisory key, fail on unparsable port
- `PGLA-M1-N2` (1 commit; latest 88566cb50): test(PGLA-M1): drop LOGIN role after database drop with checked errors, pin full advisory key, fail on unparsable port
- `PGLA-X1` (3 commits; latest daf2117e4): Merge PGLA-X1 UpdateRole last-admin guard, settings stale-read fix, lock-before-read (Opus GO-with-follow-ups; live PG16 interleave 0 violations; PROVISIONAL)
- `PGLAR-m1` (1 commit; latest 0803bf211): docs: register CAT-PG-LASTADMIN-1 review follow-ups
- `PGLAR-m2` (1 commit; latest 0803bf211): docs: register CAT-PG-LASTADMIN-1 review follow-ups
- `PGLAR-n3` (1 commit; latest 0803bf211): docs: register CAT-PG-LASTADMIN-1 review follow-ups
- `PGLAR-n4` (1 commit; latest 0803bf211): docs: register CAT-PG-LASTADMIN-1 review follow-ups
- `PGLAST-F2` (1 commit; latest 3f2a544a1): Merge ADMINDOC-1: ADMIN_GUIDE lock/corrupt-role docs (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAX1-F6` (5 commits; latest 9df927d2c): Merge PGLAX1-F6: SQLite UpdateRole strip serialised by write-first tx (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAX1-F6-FU1` (2 commits; latest 82dec7a41): Merge test-hygiene bundle: PG role leak, F6 -count, R5/N3/N4, api-client getPermissions cases (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAX1-F6-FU2` (1 commit; latest 7c1d0d859): test(PGLAX1-F6): run with -count>1 (unique driver names) and pin admin-set read through tx with a one-connection pool
- `PGLAX1-F7` (4 commits; latest 4fd58b5ac): Merge PGLAX1-F7: SQLite user-write guards serialised by write-first tx (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAX1-FU` (8 commits; latest d8b21ec3c): Merge PGLAX1-FU last-admin follow-ups: fail-closed currentlyAdmin, 409 on role strip, live PG deactivate variants (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAXFU-F1` (3 commits; latest 1b22faffa): Merge PGLAXFU tests: PG-path fail-closed, role handler error mapping, deterministic F4 control (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAXFU-F2` (2 commits; latest 1b22faffa): Merge PGLAXFU tests: PG-path fail-closed, role handler error mapping, deterministic F4 control (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAXFU-F3` (1 commit; latest 3f2a544a1): Merge ADMINDOC-1: ADMIN_GUIDE lock/corrupt-role docs (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAXFU-N2` (2 commits; latest 1b22faffa): Merge PGLAXFU tests: PG-path fail-closed, role handler error mapping, deterministic F4 control (Opus GO-with-follow-ups; PROVISIONAL)
- `PGLAXFU-N3` (1 commit; latest 29ec1b574): test(PGLAXFU): fail-closed when admin-role set unreadable (sqlmock), assert live error text, document env
- `PGLAXFU-N4` (1 commit; latest 29ec1b574): test(PGLAXFU): fail-closed when admin-role set unreadable (sqlmock), assert live error text, document env
- `PGLAXFU-R5` (2 commits; latest 82dec7a41): Merge test-hygiene bundle: PG role leak, F6 -count, R5/N3/N4, api-client getPermissions cases (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3-m1` (3 commits; latest 3946b8686): Merge PGR3-m1/m2: poolguard raw-string cgo import + cgo-last corpus (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3R-m1` (2 commits; latest 0662d0459): Merge PGR3R-m1/m2/n3 + ELFUR-m1: escaped cgo fixture, unquote-error-as-cgo, tight errleak name heuristics (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3R-m2` (2 commits; latest 0662d0459): Merge PGR3R-m1/m2/n3 + ELFUR-m1: escaped cgo fixture, unquote-error-as-cgo, tight errleak name heuristics (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3R-n3` (2 commits; latest 0662d0459): Merge PGR3R-m1/m2/n3 + ELFUR-m1: escaped cgo fixture, unquote-error-as-cgo, tight errleak name heuristics (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3RR-m1` (3 commits; latest afd5524c0): Merge PGR3RR-m1/m2/n3 + ENVR-m1: real unicode-escape assertion, e.Err pin, SMB fixture cleanup (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3RR-m2` (3 commits; latest afd5524c0): Merge PGR3RR-m1/m2/n3 + ENVR-m1: real unicode-escape assertion, e.Err pin, SMB fixture cleanup (Opus GO-with-follow-ups; PROVISIONAL)
- `PGR3RR-n3` (3 commits; latest afd5524c0): Merge PGR3RR-m1/m2/n3 + ENVR-m1: real unicode-escape assertion, e.Err pin, SMB fixture cleanup (Opus GO-with-follow-ups; PROVISIONAL)
- `PGRRR-n1` (1 commit; latest de1355928): docs: register PGR3RR/ENVR review follow-ups
- `PGRRR-n2` (1 commit; latest de1355928): docs: register PGR3RR/ENVR review follow-ups
- `PKGB-FU1` (4 commits; latest 0d9e74220): Merge fix/red-hyg (PKGB-FU1/FU2, HANDLERS-RED-1-F3, OBF-FU-F1, PXR-2, PXF23-F1; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `PKGB-FU2` (1 commit; latest f5a4ea82e): main: access-log formatter removes the request's known secrets from error text
- `PKGC-FU4-F1` (3 commits; latest e3b2d4a2d): Merge test/rl-wiring (real-limiter wiring tests for auth routes; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `PW-F4` (4 commits; latest 46c834e50): Merge PW-F4 + FU2-1 web verification evidence (Opus round 2 GO; PROVISIONAL)
- `PXF18-FU1` (3 commits; latest 5a199fe36): Merge docs/pxf18-fu (PXF18-FU1..5,7..11 corrections, PXF23-F2 sweep; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `PXF18-FU2` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `PXF18-FU3` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `PXF18-FU4` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `PXF18-FU5` (1 commit; latest 54b600ed3): docs(qa): PX-F18 enumeration corrections PXF18-FU1..FU5, FU7..FU11 (FU6 not touched); html/pdf twins regenerated
- `PXF18-FU6` (4 commits; latest 4903ed47b): Merge fix/pxf18-fu6-challenge-results-admin (challenges routes admin-only; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `PXF18-FU6-F1` (1 commit; latest 7f07f3c61): docs(api): add 403 admin-required to GET challenges routes (PXF18-FU6-F1)
- `PXF18-FU6-F2` (1 commit; latest aca660155): docs(api): whole challenges group is admin-only (PXF18-FU6-F2)
- `PXF18-FU6-F3` (1 commit; latest fc2545f7b): docs: provenance note for hand-edited challenge rows in ROUTE_AUTHZ_TABLE (PXF18-FU6-F3)
- `PXF18DOC-F1` (1 commit; latest 09a10bc5e): test(handlers): pin M6/M7/M8 gaps and bool-fail false positive in PXF23-F2 sweep (PXF18DOC-F1, PXF18DOC-F2)
- `PXF18DOC-F2` (1 commit; latest 20f09605c): docs(qa): err_error_class corrections PXF18DOC-F1..F6 (twins re-rendered)
- `PXF18DOC-F3` (1 commit; latest 20f09605c): docs(qa): err_error_class corrections PXF18DOC-F1..F6 (twins re-rendered)
- `PXF18DOC-F4` (1 commit; latest 20f09605c): docs(qa): err_error_class corrections PXF18DOC-F1..F6 (twins re-rendered)
- `PXF18DOC-F5` (1 commit; latest 20f09605c): docs(qa): err_error_class corrections PXF18DOC-F1..F6 (twins re-rendered)
- `PXF18DOC-F6` (1 commit; latest 20f09605c): docs(qa): err_error_class corrections PXF18DOC-F1..F6 (twins re-rendered)
- `PXF23-F1` (2 commits; latest 3287ffc2c): monitoring: test that the logged health-check error is redacted (F7)
- `PXF23-F2` (1 commit; latest c0508d772): test(handlers): PXF23-F2 source sweep sees an error value passed bare to a client sink, whatever it is called
- `PXR-2` (2 commits; latest d776a2257): docs: PXR-2 README names only the real residuals (F5)
- `RLW-FU1` (3 commits; latest e8d07b080): Merge batch B (PXF18-FU6-F1..F3, PXF18DOC-F1..F6, RLW-FU1; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `RLW-FU2` (3 commits; latest 4275fad98): Merge rlw-fu2 (Retry-After on 429; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `RLW-FU2-F1` (1 commit; latest 719db3b8b): fix(catalog-api): Retry-After on RateLimiter/pprof 429s; exact fixed-window test; real-path login-tier test
- `RLW-FU2-F3` (1 commit; latest 719db3b8b): fix(catalog-api): Retry-After on RateLimiter/pprof 429s; exact fixed-window test; real-path login-tier test
- `RLW-FU2-TIER` (3 commits; latest e2a65d5cb): Merge RLW-FU2-TIER rate-limit body/CORS/clock fixes (Opus GO-with-follow-ups; PROVISIONAL)
- `SEC-WS-1` (2 commits; latest 141b49b25): Merge WSF2 follow-ups (F1 web failOpen, F3 per-user keying tests, F4 purge head index, N1/N2; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `SEC-WS-1-F2` (7 commits; latest 6b67141ae): Merge SEC-WS-1 F2 api-client + Android ticket clients (709e5ae67, 08eb2d0f2; same landing unit; PROVISIONAL)
- `SECQ1C-FU` (3 commits; latest 429b46d5d): Merge SECQ1C-FU create-time WebDAV class test + step-up/limiter/SSE tests (Opus GO-with-follow-ups; PROVISIONAL)
- `SSE-F1` (3 commits; latest e5ac577e0): Merge SSE-F1..F3 (fail-closed admin tests, openapi 503 on sync GETs, authz doc corrections; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `SSE-F2` (2 commits; latest e5ac577e0): Merge SSE-F1..F3 (fail-closed admin tests, openapi 503 on sync GETs, authz doc corrections; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `SSE-F3` (2 commits; latest e5ac577e0): Merge SSE-F1..F3 (fail-closed admin tests, openapi 503 on sync GETs, authz doc corrections; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `SSE-F4` (3 commits; latest a45ca2f48): Merge SSE-F4 (hide Sync tab and disable sync queries for non-admins; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `SSE-F4-R` (5 commits; latest 5552dda01): Merge SSE-F4-R Settings real-auth tests + AuthContext/tab fallback (Opus GO-with-follow-ups; PROVISIONAL)
- `SSE-OPQ-1` (7 commits; latest 87124fde1): Merge SSE-OPQ-1 test follow-ups F1/F2/F3/F5 (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-API` (2 commits; latest 25beebbb2): Merge SSEF4R-API: api-client permission matcher + envelope unwrap (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-API-FU1` (2 commits; latest 82dec7a41): Merge test-hygiene bundle: PG role leak, F6 -count, R5/N3/N4, api-client getPermissions cases (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-FD` (7 commits; latest 79fc662c2): Merge SSEF4R-FD: web route gates use server permission vocabulary (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-FD-F2` (3 commits; latest 7827dd00f): Merge SSEF4R-FD-F2/F3: /identities requireAdmin, /conversion conversion.view/create/manage (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-FD-F3` (3 commits; latest 7827dd00f): Merge SSEF4R-FD-F2/F3: /identities requireAdmin, /conversion conversion.view/create/manage (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-FD-m1` (3 commits; latest 4806f4046): Merge SSEF4R-FD-m1: route-gate completeness check (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEF4R-FD-m1-FU1` (1 commit; latest d218a55b4): test: WEB-TEST-BUNDLE-1 route-gate golden cases, header wording, FUTB-N3 timer advance
- `SSEF4R-FD-m1-FU3` (1 commit; latest d218a55b4): test: WEB-TEST-BUNDLE-1 route-gate golden cases, header wording, FUTB-N3 timer advance
- `SSEF4R-FU` (5 commits; latest 855ee46d0): Merge SSEF4R-FU UI isAdmin from /auth/status permissions (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFU-F1` (4 commits; latest 004eb302c): docs(api): SSEFU-F4/F5 consistent 503 wording on sync GETs, $ref Error for sync 500s, success on Error schema (PROVISIONAL)
- `SSEFU-F2` (4 commits; latest 004eb302c): docs(api): SSEFU-F4/F5 consistent 503 wording on sync GETs, $ref Error for sync 500s, success on Error schema (PROVISIONAL)
- `SSEFU-F3` (4 commits; latest 004eb302c): docs(api): SSEFU-F4/F5 consistent 503 wording on sync GETs, $ref Error for sync 500s, success on Error schema (PROVISIONAL)
- `SSEFU-F4` (5 commits; latest 05460c33e): Merge SSEFU-F4/F5: consistent sync-GET 503 wording, Error schema refs (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFU-F5` (5 commits; latest 05460c33e): Merge SSEFU-F4/F5: consistent sync-GET 503 wording, Error schema refs (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFU2-F1` (2 commits; latest 290c73189): Merge HYG-F4 + SSEFU2 tests (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFU2-F2` (2 commits; latest 290c73189): Merge HYG-F4 + SSEFU2 tests (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFU2-F3` (3 commits; latest 4d936f547): SSEFU2-F3: pin exact 503 body of RequireAdmin check failure (JSONEq)
- `SSEFUR-i5` (1 commit; latest 9d73a17d5): docs: register SSEFU-F4/F5 review follow-ups
- `SSEFUR-m1` (3 commits; latest a22d9fa0b): Merge SSEFUR-m1/m2/n3/n4: sync docs 503/403 alignment, admin-only Auth lines, 500 example (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFUR-m2` (3 commits; latest a22d9fa0b): Merge SSEFUR-m1/m2/n3/n4: sync docs 503/403 alignment, admin-only Auth lines, 500 example (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFUR-n3` (3 commits; latest a22d9fa0b): Merge SSEFUR-m1/m2/n3/n4: sync docs 503/403 alignment, admin-only Auth lines, 500 example (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFUR-n4` (3 commits; latest a22d9fa0b): Merge SSEFUR-m1/m2/n3/n4: sync docs 503/403 alignment, admin-only Auth lines, 500 example (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFURR-m1` (4 commits; latest 84f602053): Merge SSEFURR-m1/n2: openapi 503 wording aligned to real gate strings (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFURR-n2` (3 commits; latest 84f602053): Merge SSEFURR-m1/n2: openapi 503 wording aligned to real gate strings (Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFURRR` (1 commit; latest 9625a5015): Merge ssefurrr-docs: openapi 503 precision for gated operations (SSEFURRR-m1/m2/m3; Opus GO-with-follow-ups; PROVISIONAL)
- `SSEFURRR-m1` (1 commit; latest 492f3e7ba): docs(api): precise 503 gate wording for 30 gated operations; Error.error text/plain wording (SSEFURRR-m1/m2/m3)
- `SSEFURRR-m2` (1 commit; latest 492f3e7ba): docs(api): precise 503 gate wording for 30 gated operations; Error.error text/plain wording (SSEFURRR-m1/m2/m3)
- `SSEFURRR-m3` (1 commit; latest 492f3e7ba): docs(api): precise 503 gate wording for 30 gated operations; Error.error text/plain wording (SSEFURRR-m1/m2/m3)
- `SSEFURRRR` (1 commit; latest 6af5ce16f): docs: register SSEFURRRR review follow-ups
- `STATS-SCANS-ERR-1` (1 commit; latest cada4debc): fix(catalog-api): admin-only /sync/* GETs; fixed error text for non-admin /stats/scans
- `SYNC-ADMIN-1` (3 commits; latest 0ed6495e0): Merge fix/stats-scans-err-sync-admin (STATS-SCANS-ERR-1, SYNC-ADMIN-1; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `NAS-LIVE-1` (2 commits; latest 2635b3e2e): Merge live NAS evidence 2026-11-15 (Opus GO-with-follow-ups; PROVISIONAL; roadmap id recorded in history under the vendor-specific label)
- `TEST-HYGIENE-BUNDLE-2` (3 commits; latest aff954c69): Merge TEST-HYGIENE-BUNDLE-2: PG test gating, single-connection F7 test, Delete rollback pin (Opus GO-with-follow-ups; PROVISIONAL)
- `TEST-HYGIENE-BUNDLE-3` (3 commits; latest 7fed296bd): Merge TEST-HYGIENE-BUNDLE-3: deadlock guards and tightened SQL pins (Opus GO-with-follow-ups; PROVISIONAL)
- `TH2-MINOR-1` (3 commits; latest 9827ba2a2): Merge TH2-MINOR-1: sole-admin LockAccount single-connection test (Opus GO-with-follow-ups; PROVISIONAL)
- `TH3-follow-ups` (3 commits; latest 8250ae116): Merge TH3/WTB1 follow-ups: runWithin value hand-off, anchored SQL pins, multi-gate extractor case (Opus GO-with-follow-ups; PROVISIONAL)
- `TIERFU-PGLA-TESTS` (3 commits; latest aedab3d3d): Merge TIERFU-PGLA-TESTS CORS expose + Retry-After + LockAccount classification tests (Opus GO-with-follow-ups; PROVISIONAL)
- `TIERPGLA-F3` (1 commit; latest 28720f5e6): docs(cors): TIERPGLA-F3 state that X-RateLimit-Algorithm has no production emitter today
- `TIERTEST-RLW-F1` (3 commits; latest 872f6ba88): Merge tiertest-rlw-f1-f4 (flake fixes, pprof stale window, injectable bucket clock; Opus round 1 GO-with-follow-ups; PROVISIONAL)
- `TIERTEST-RLW-F2` (1 commit; latest a150a542e): fix(catalog-api/middleware): de-flake tier/RLW tests, pprof guard stale-window read (TIERTEST-RLW-F1..F4)
- `TIERTEST-RLW-F3` (1 commit; latest a150a542e): fix(catalog-api/middleware): de-flake tier/RLW tests, pprof guard stale-window read (TIERTEST-RLW-F1..F4)
- `TIERTEST-RLW-F4` (1 commit; latest a150a542e): fix(catalog-api/middleware): de-flake tier/RLW tests, pprof guard stale-window read (TIERTEST-RLW-F1..F4)
- `V250-GATES` (1 commit; latest bbe6c2bed): docs(release): v2.5.0 gate checklist rev 3 against main ae1d2ae45 (re-derived G1..G12; 0 of 12 satisfied)
- `V2_5_0_GATE_CHECKLIST` (4 commits; latest bc58945b6): Merge gate checklist Rev 3 refresh against main ae1d2ae45 (0 of 12 gates satisfied; Opus GO after round 2; PROVISIONAL)
- `WBF1-m2` (3 commits; latest 5c11ba839): Merge WBF1-m3/m2: MediaPlayer useLayoutEffect ticket fix + deterministic test + web submodule guard (Opus GO r3; PROVISIONAL)
- `WBF1-m3` (5 commits; latest 5c11ba839): Merge WBF1-m3/m2: MediaPlayer useLayoutEffect ticket fix + deterministic test + web submodule guard (Opus GO r3; PROVISIONAL)
- `WEB-BASE-FAIL-1` (1 commit; latest 784c576eb): docs: register WEB-BASE-FAIL-1 investigation result
- `WEB-TEST-BUNDLE-1` (3 commits; latest f8407c04a): Merge WEB-TEST-BUNDLE-1: route-gate golden cases + Settings fake-timer test (Opus GO-with-follow-ups; PROVISIONAL)
- `WSF2-B` (4 commits; latest 7976e1180): docs: register WSF2BR review follow-ups
- `WSF2-R2-F1` (1 commit; latest e67d73985): fix(ws): SEC-WS-1 F2 review follow-ups (WSF2-R2-F1, F3, F4, N1, N2) PROVISIONAL
- `WSF2-R2-F2` (3 commits; latest 5531c0721): Merge WSF2-R2-F2 Android listener fix (Opus GO-with-follow-ups; PROVISIONAL)
- `WSF2-R2-F3` (1 commit; latest e67d73985): fix(ws): SEC-WS-1 F2 review follow-ups (WSF2-R2-F1, F3, F4, N1, N2) PROVISIONAL
- `WSF2-R2-F4` (1 commit; latest e67d73985): fix(ws): SEC-WS-1 F2 review follow-ups (WSF2-R2-F1, F3, F4, N1, N2) PROVISIONAL
- `WSF2-R2-N1` (1 commit; latest e67d73985): fix(ws): SEC-WS-1 F2 review follow-ups (WSF2-R2-F1, F3, F4, N1, N2) PROVISIONAL
- `WSF2-R2-N2` (1 commit; latest e67d73985): fix(ws): SEC-WS-1 F2 review follow-ups (WSF2-R2-F1, F3, F4, N1, N2) PROVISIONAL
- `WSF2B2-F1` (4 commits; latest af8e37c2f): docs: register WSF2B2R review follow-ups
- `WSF2B2-F2` (2 commits; latest 532c08944): Merge WSF2B2-F1/F2: Android WS attempt cancellation + minter timeout handling (Opus GO-with-follow-ups; PROVISIONAL)
- `WSF2B2R` (1 commit; latest 5fd99a487): Merge wsf2b2r-fu: Android WS cancelled-attempt silence fix + tests, Bearer test flake fix (WSF2B2R-m1..m4, WSF2BR-m3; Opus GO-with-follow-ups; host JVM unit only; PROVISIONAL)
- `WSF2B2R-m1` (1 commit; latest ee3bd63fa): fix(android): WSF2B2R follow-ups m1-m4 (cancelled mint attempt neither publishes Error nor reconnects; Bearer-header test no longer flaky)
- `WSF2B2R-m2` (1 commit; latest ee3bd63fa): fix(android): WSF2B2R follow-ups m1-m4 (cancelled mint attempt neither publishes Error nor reconnects; Bearer-header test no longer flaky)
- `WSF2B2R-m3` (1 commit; latest ee3bd63fa): fix(android): WSF2B2R follow-ups m1-m4 (cancelled mint attempt neither publishes Error nor reconnects; Bearer-header test no longer flaky)
- `WSF2B2R-m4` (1 commit; latest ee3bd63fa): fix(android): WSF2B2R follow-ups m1-m4 (cancelled mint attempt neither publishes Error nor reconnects; Bearer-header test no longer flaky)
- `WSF2B2RR` (4 commits; latest 0691c756b): Merge wsf2b2rr-fu: stale 401 and cancelled-attempt failures publish no Error, 5 lifecycle tests (WSF2B2RR-m1,m2; Opus GO-with-follow-ups; PROVISIONAL)
- `WSF2B2RRR` (3 commits; latest c7fa65dea): Merge wsf2b2rrr-fu: sequence-recording Android lifecycle tests (WSF2B2RRR-f1,n1; Opus GO-with-follow-ups; PROVISIONAL)
- `WSF2BR-m3` (1 commit; latest ee3bd63fa): fix(android): WSF2B2R follow-ups m1-m4 (cancelled mint attempt neither publishes Error nor reconnects; Bearer-header test no longer flaky)
- `WSFU-I1` (4 commits; latest e0c7e6797): Merge WSFU-I1 (repair stale WebSocketContext.connectionStatus test; Opus GO-with-follow-ups, follow-up commit implements reviewer fix; PROVISIONAL)
- `WTB1-follow-ups` (1 commit; latest a11079c6c): test(web): pin per-occurrence route-gate reporting, reword timer comment (WTB1-m1/n1; PROVISIONAL)

## [2.4.3] - 2026-11-14 (pre-release, not production-ready)

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Prepared on branch `release/v2.4.3-prep` from `main` `b43f5370` (328 commits since the `v2.4.2` tag; the annotated tag object is `26f2e1df` and its commit is `fc8dcf27e`; `git log v2.4.2..b43f5370`). Not tagged, not published. Versioning is SemVer. Version files bumped to 2.4.3: `versions.json` (global 2.4.3), `docs/api/openapi.yaml`, `catalog-web`, `catalogizer-desktop` and `installer-wizard` (package.json, package-lock.json, tauri.conf.json, Cargo.toml), Android `versionName` 2.4.3 / `versionCode` 9 (was 8), Android TV `versionName` 2.4.3 / `versionCode` 11 (was 10), the two Android `CLAUDE.md` version lines, the installer-wizard AppImage file name in `challenges/scripts/desktop_appimage_launch_challenge.sh`, and the version mentions in `README.md`, `docs/CONTRIBUTING.md` and `docs/DEPLOYMENT_GUIDE.md`.

### Release gate (constitution 11.4.298, stated first)
- **Gate state: `BLOCKED-NO-BASELINE`.** No 11.4.298 live run of the whole suite on connected target hardware has been done for this build, and no baseline run of v2.4.2 exists, so no 11.4.293 comparison (`NO-WORSE-PROVEN`) is possible. The live baseline run (flash/run v2.4.2, run the suite, then the same on v2.4.3) is **owed**. This is a labelled pre-release; it must not be deployed to users or called production-ready. The comparator and gate tools (`image_no_worse`, `release_gate`, CAT-WP03-MECH) are gate-logic unit class and are not wired into tagging or deploy.
- **The full-suite retest of the tag (11.4.40), the 11.4.185 manual QA and the live UI QA have NOT been run on this candidate.** The release-consistency checker is **advisory** for v2.4.3 (blocking for v2.5.0, see `docs/release/V2_5_0_GATE_CHECKLIST.md`, an unreviewed draft).

### State summary by label (11.4.290(G))
Counts (11.4.290(4)): **V counts evidence sets, P counts items, so they are not comparable.** V = 3 evidence sets whose hashes verify (1 checker-cited, 2 hand-verified; the behaviour they back is PROVISIONAL). P = 101 distinct `Roadmap-Item` ids, derived with `git log v2.4.2..b43f5370 --format=%B | grep '^Roadmap-Item:'` (287 lines), split on commas, trimmed, then `sort -u | wc -l` (a trailer-block method undercounts, it reads only the final block); of the 328 commits in the range (75 merges), 286 carry a `Roadmap-Item:` line (one has two) and 42 carry none (merges and some docs commits; the items they merge are counted through their own commits). Sub-ids named in the lines below (PX-F1..F23, PW-F1..F10 and so on) belong to those ids or to the groups that contain them and are not counted separately. R = 0 reopened, O = 0 new overdue (the carried OVERDUE jobs below remain).

### VERIFIED (committed machine evidence, checked while preparing this entry)
Checked on this branch with `sha256sum -c` (0 non-OK lines). "VERIFIED" here means the evidence set exists and its hashes verify, not that the behaviour is verified.
- `docs/qa/img-slim/` (CAT-IMG-SLIM-1 minimal-ffmpeg image evidence; `SHA256SUMS.txt` verifies). Container class; the full `catalog-api/Dockerfile` build including the Go stage was not exercised.
- Carried from 2.4.2 and earlier, not re-run: the VERIFIED rows of `[2.4.2]`, `[2.4.1]`, `[2.4.0]`. Caveat carried with them: the cover-art SSRF live-test evidence set (the coverart results/final directory under docs/qa) has a `SHA256SUMS.txt` that FAILs `sha256sum -c`; this is the checker's known `sha_fail=1` under `--section 2.4.2` (see the checker bullet below).
- Other evidence directories added since v2.4.2 (`docs/qa/login-lock-2-round*`, `chpw-rl-2-round*`, `sec-ws-1*`, `favr3-fu*`, `stack3`) carry no checkable hash manifest and are treated as PROVISIONAL.

#### Hand-verified evidence sets (not checker-cited)

Checked with `sha256sum -c` (0 non-OK lines) by hand. They are not cited as backticked paths because the checker would list their unhashed README.html and README.pdf twins as UNLISTED. The behaviour they back stays PROVISIONAL.

- docs/qa/anton-smb-n3-20261112/run1.log, run2.log, run3.log and README.md there (Anton SMB N=3 live run; SHA256SUMS verifies). No negative control: the run proves reachability at N=3 only.
- docs/qa/secq2-repro-20261112/rep6.json and rep6.log.txt (CAT-SECQ2-WD-F1 live WebDAV reproduction: not reproduced, 6/6 ok; SHA256SUMS verifies).

### PROVISIONAL (landed on `main`; counts as done for planning, NOT verified)
Each item landed after an Opus review that returned GO-with-follow-ups (minor-class follow-ups tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`). Evidence class is mostly unit / httptest / SQLite (some PostgreSQL 15/16, container) level; live end-to-end runs are owed. **Verification jobs (11.4.290(1)):** `docs/PROVISIONAL_VERIFICATION_PLAN.md` section 6 schedules one job per group below, `VJ243-01..VJ243-23`, each with its target, the existing command (or `command: owed (no automated check yet)`, registered as gap `G243-n`), the evidence class needed for VERIFIED and a due date. Group to job: authentication and sessions VJ243-01..06; `/ws` VJ243-07; egress and storage hosts VJ243-08, 11, 12; error leakage and redaction VJ243-09; confinement VJ243-10; wizard VJ243-13; web VJ243-14; release tooling VJ243-15..18; image VJ243-19; stale tests VJ243-20; docs VJ243-21; research and records VJ243-22; whole release VJ243-23. **Due dates are proposed by the release preparer; the operator may adjust them:** host jobs 2026-11-21, runtime and live jobs 2026-11-30 or before `v2.5.0-rc.1`, whichever is first. No job has been run.

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

- CAT-REL-CHECK-1, CAT-REL-CHECK-2, CAT-REL-CHECK-3: HEAD-snapshot release-consistency checker (502 tests; 236 mutants defined, but the full 236-mutant set has not been run end to end, per `docs/FOLLOW_UPS_NEXT_PHASE.md`), waivers; advisory for v2.4.3.
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
- Release checker: `scripts/release/check_release_consistency.sh --repo . --version 2.4.3 --prev-tag v2.4.2` (section `[2.4.3]`) returns PASS with only `unused_waivers=3` (version_ok=19, sha_ok=64, sha_fail=0, unlisted=0). The "known 4 findings" (sha_fail=1, no_manifest=1, unlisted=1, unused_waivers=3) come from checking the `[2.4.2]` section (`--section 2.4.2`, the cover-art evidence set) and are unchanged. The checker is advisory for v2.4.3 and blocking for v2.5.0.
- `scripts/anti-bluff-scan.sh` reports 4 `ASSERT_TAUTOLOGY` findings, all in third-party/hub code, none in first-party code (carried from the retest, unchanged).
- **CAT-NFS-EMU-1 was NOT landed** (R_max NO-GO on `feat/nfs-emulator`); successor **CAT-NFS-EMU-2** is open. Likewise not landed or still open: CAT-PREEXIST-9b (web load failures, UNREPRODUCED), D8-F1, D9-F1, SEC-WS-1 follow-ups, RS-1..RS-4 (minor, from CAT-RETEST-STALE-1). Items owed before v2.5.0 are in `docs/release/V2_5_0_GATE_CHECKLIST.md`.
- NAS SMB/FTP/NFS live tests remain SKIPPED (drives powered off).

### Behaviour changes and upgrade notes (read before upgrading)
- **Job ids:** a conversion job owned by another user now returns 404, identical to a missing id (was distinguishable from a missing id).
- **Symlink confinement:** local and NFS filesystem roots refuse symlinks that escape the root. A per-root option `allow_symlink_escape` (default off) exists; it is not reachable from the DB row or config in this release (documented in `docs/security/PATH_CONFINEMENT.md`; UNCONFIRMED how an operator sets it).
- **Metadata addresses:** the cloud-metadata singletons (incl. `fd20:ce::254`) are always refused for storage roots and SMB hosts, with no allow-list. Storage hosts (SMB/FTP/NFS) are vetted at create and pinned at dial: an existing root pointing at a now-refused address will fail to connect.
- **NFS export:** an export string with `,` followed by `:` is refused (a second server location on darwin `mount_nfs`).
- **Redis rate limits:** per-tier keys (`auth` 600 / default 2000 per minute); an outage after startup falls back to the in-memory limiter; login/register stay on the in-memory 30/min limiter.
- **Login throttle:** a cap of in-flight password evaluations per account; every evaluated failed attempt is charged, also on an already blocked account; a successful owner login no longer writes the account throttle scope (throttle state is stored in the database; PostgreSQL-verified).
- **Change password:** failed current-password attempts are limited per session and per user (5 per 15 min); a password change revokes sessions (see CAT-CHPW-RL-1); the change-password counters are in memory per process.
- **Suspension:** an admin suspension is a permanent lock and revokes sessions (CAT-D9-SUSPEND).
- **Log shares:** tokens are new 32-byte random, hashed at rest, shown once; every failure returns the same 404; a JWT is required to create. Migration v28 retires existing share links (see the migrations bullet).
- **WebSocket:** `/ws` requires authentication and an Origin policy (host:port and scheme); deployments using the shipped nginx configs must pick up the updated confs (they forward `$http_host`).
- **Errors:** responses no longer include internal error text (fixed class text and a request id).
- **Login fails closed:** when the login throttle state cannot be read or written, login answers **503** `sign-in temporarily unavailable` (`ErrLoginThrottleUnavailable`, `handlers/auth_handler.go`) instead of letting the attempt through. While an account or address is throttled, login answers **429** (`too many failed sign-in attempts, try again later`).
- **Unlock of a suspended account:** the user unlock call answers **409** for a suspended account; only an administrator status change to active lifts a suspension (`handlers/user_handler.go`).
- **Schema migrations run on upgrade:** v27 (`approved_class` columns on `storage_roots` and `sync_endpoints`, CAT-SECQ1-A), v28 (retires every legacy plaintext log-share token: the share row is deactivated and its token overwritten, so **existing log-share links stop working and the creator must re-share**, CAT-LOGSHARE-TOK-1) and v29 (creates the `login_throttle` table). **Rollback:** the in-process migration runner has only `Up` steps (`catalog-api/database/migrations.go`, `Migration{Version, Name, Up}`), so a downgrade is NOT supported; back the database up before upgrading and restore it to go back. Whether a v2.4.2 binary tolerates the v27/v29 schema (extra columns and table) is UNCONFIRMED, and v28 cannot be undone.
- **Roles:** role 2 is compared against the caller like any other role.
- **Image:** the API image is 307 MB (was 746 MB) with a minimal ffmpeg; **conversions to ogg and m4a now drop embedded cover art**.
- **Wizard:** the config is merged, never overwritten; `CONFIG_PATH` is the wizard directory and `CATALOG_CONFIG_PATH` the app config file.
- The behaviour change under `Security (outbound address classes, CAT-SECQ1-A)` below is new in this release. The `TRUSTED_PROXIES` changes (including the compose network upgrade step) are not new: they shipped in `v2.4.0`.

### New in 2.4.3, from the former `[Unreleased]` section (checked with `git grep` on the earlier tags)

### Security (outbound address classes, CAT-SECQ1-A; PROVISIONAL)
- **Behaviour change: storage roots (WebDAV, S3) can no longer reach the server's own interface addresses** (for example its LAN address) unless allow-listed (`CATALOGIZER_OUTBOUND_ALLOW_CIDRS=<ip>/32`). Other LAN addresses keep working.
- Operator transports pin themselves to the address class of their first connection (a root on a public address cannot be redirected into the LAN, also inside gowebdav); a dial-time check re-validates the connected address; 168.63.129.16 is refused; invalid or over-broad (`/0`...`/7`, also IPv4-mapped) allow-list entries are logged and ignored unless `CATALOGIZER_OUTBOUND_ALLOW_BROAD=1`; non-ASCII and numeric host spellings are refused on all paths. Details: `docs/ENV_VARIABLES.md`, `docs/OUTBOUND_HTTP_CLIENTS.md`.

### Already shipped earlier (listed for context, NOT new in 2.4.3)

The items below were in the former `[Unreleased]` section but are already contained in earlier tags (checked with `git tag --contains` on their commits and `git grep` on `v2.4.0`, `v2.4.1`, `v2.4.2`): TRUSTED_PROXIES hardening (commit `8c1fb3339`, first tag `v2.4.0`; `TRUSTED_PROXIES_ALLOW_AUTO` is in `v2.4.0`'s `.env.example`), FTP `CopyFile` with the `.ftpcopy-<hex>.part` temp file and `renameGraceAfterCancel` (first tag `v2.4.0`), Spec Kit constitution 2.5.0 and `docs/GOVERNANCE.md` (in `v2.4.0`), and the old `Fixed` items such as the challenge framework, Android TV lint and the axios SSRF fix (first tag `v2.4.0`). They are kept verbatim for context and are not part of the v2.4.3 list or counts.

#### Security (TRUSTED_PROXIES)
- **`TRUSTED_PROXIES=auto` is deprecated and refused in release mode** (CAT-TRUSTEDPROXIES-AUTO-1): it trusts all RFC-1918/ULA ranges, so a client inside one of them could send `X-Forwarded-For: 9.9.9.9` and have the proxy append its own address, making `ClientIP()` 9.9.9.9 (login rate-limit bypass; gin skips trusted entries from the right and has no "trust only the direct hop" mode). With `GIN_MODE=release`, `auto` now fails unless `TRUSTED_PROXIES_ALLOW_AUTO=true`; a startup warning is logged whenever `auto` is used. A rejected or invalid `TRUSTED_PROXIES` now **fails closed** (no proxy trusted): previously a refused `auto` left gin's trust-everything default in place, and an invalid CIDR list left whatever gin had applied before the error (possibly a partial list). **Behaviour change:** deployments relying on `auto` in release mode must list explicit proxy CIDRs (or set the acknowledgement). `catalog-api/.env.example` no longer enables `auto`.
- **Production compose files now set `TRUSTED_PROXIES`** (CAT-TRUSTEDPROXIES-PROD-1): `docker-compose.yml` (nginx, `production` profile) and `deployment/docker-compose.yml` (`web`) pin the proxy to a fixed address on a fixed-subnet network (`172.28.0.10` / `172.29.0.10`) and the API trusts exactly that `/32`; previously every client behind nginx shared one login rate-limit bucket. Dynamic containers are allocated only from the upper half of each subnet (`ip_range` `172.28.0.128/25` / `172.29.0.128/25`), so the pinned address cannot be taken by another container. Overrides: `CATALOGIZER_SUBNET`, `CATALOGIZER_IP_RANGE`, `NGINX_IP` (root file) and `DOCKER_SUBNET`, `DOCKER_IP_RANGE`, `WEB_IP` (deployment file); when you change the address also set `TRUSTED_PROXIES`. A subnet that overlaps a host network fails `up` with a "Pool overlaps"/address-in-use error: pick another subnet via those variables.
- **Upgrade / migration (existing installs):** compose reuses an existing network of the same name and ignores new ipam settings, so the pinned address would fail or be ignored. Before the first `up` of this version run `compose down`: for the root file the network is project-scoped (`<project>_catalogizer-network`) and `compose down` removes it (do not remove a bare `catalogizer-network`, it belongs to another project). The `deployment` file's default network name changed from the fixed `catalogizer-network` to `catalogizer-frontdoor-net` (override `DOCKER_NETWORK`) so it does not collide with an older network; remove the old `catalogizer-network` only if no other stack uses it.
- `deployment/docker-compose.yml` `web` now sets `API_UPSTREAM: catalogizer-server:8080`. The web image's nginx template (`catalog-web/nginx.conf.template`, rendered by the official nginx entrypoint, `NGINX_ENVSUBST_FILTER=^API_`) proxies to `API_UPSTREAM`, whose default `host.containers.internal:8080` goes via the host and would hide the pinned web address from the API. Its `/ws` location now forwards `X-Real-IP`/`X-Forwarded-For` too, and `config/nginx.conf` `/health` does the same (a test scans every `proxy_pass` location).
- A startup warning is logged when `TRUSTED_PROXIES` contains `0.0.0.0/0` or `::/0`.
- Verification: `ip_range` reaching network creation was confirmed with a throwaway podman-compose project (pinned `.10` outside the range, a dynamic container got `.129`). Not verified live: the real compose stacks were only rendered (`podman-compose config`), not started; that the published `catalogizer/web` image is built from `catalog-web/Dockerfile` is assumed.
- Tests pin every `auto` range with both edges, the forgery hazard, release-mode refusal, fail-closed behaviour, and the compose-to-policy link (CAT-TRUSTEDPROXIES-FU-1).

#### Governance
- **Spec Kit constitution 2.5.0** (2026-10-21): single-source flat layout (owner mandate: no nested submodule chains); canon pin moved `4bcb512` → `a3d80ce` (canon branch `feat/hub-engines-single-source` merged and pushed to canon `main`) and then to its follow-up `5f8503b` (pushed to canon `main`); `llmctl` `f42bad6` (cleaned, `CM-LLMCTL-SUBMODULE-NONRECURSIVE` PASS), root `helix_qa` `ad8b12b`; `claude_toolkit` `1f6dad3` (cleaned: nests only the third-party Go toolchain, ships `helix-deps.yaml`; at `a3d80ce` it was `223c702` with both gates RED); `CM-HUB-ENGINE-NO-OWNORG-NESTING`, `CM-HELIX-DEPS-MANIFEST-PRESENT` and `scripts/verify_helix_deps_manifests.sh` PASS (pass 54 / fail 0); index scope re-rendered from DATA (`derive_scope.sh`, `scope_to_lumen_json.py`, `scope_render.py`, `scope_drift_check.sh --regen`), drift check passes; root `helix-deps.yaml` refs re-synced to the staged gitlinks; `scripts/safe_submodule_init.sh` warns about leftover directories of dropped gitlinks; safe init through `scripts/safe_submodule_init.sh`; 27 flat tool submodules; Known Conflicts items 21(d), 25 to 27 and 29 updated (27 and 29 RESOLVED), items 30 to 32 added. Earlier in the day, **2.4.0**: canon pin moved `94b8836` → `4bcb512`; new section "Constitution Hub Engines" binding the vendored `llmctl`, `claude_toolkit` and `open_design` (rules HE-1 to HE-8, appendix Part 3); Known Conflicts items 21(c)/(d) corrected and items 25 to 29 added (29: code-index scope data) (canon's `CM-LLMCTL-SUBMODULE-NONRECURSIVE` gate fails at the new pin; `claude_toolkit` and `open_design` are not in canon's manifest). See `docs/GOVERNANCE.md`.
- Documentation: safe, non-recursive submodule initialisation through `constitution/` in `README.md`, `GETTING_STARTED.md`, `docs/DEVELOPER_GUIDE.md` and `docs/SUBMODULE_DEPENDENCIES.md`; new `docs/GOVERNANCE.md`.

#### Changed (FTP CopyFile, filesystem package)
- **FTP `CopyFile` no longer deletes or overwrites the destination in place** (`catalog-api/filesystem/ftp_client.go`). The data is now stored under a temp name beside the destination, `.<base>.ftpcopy-<16 hex>.part` (base cut to 200 bytes on a rune boundary, so the whole name stays under NAME_MAX), and renamed onto the destination (RNFR/RNTO) only after the source transfer is confirmed. **Behaviour change:** a server that refuses rename onto an existing file (some proftpd/pure-ftpd configurations) now makes every copy onto an existing destination fail, with the destination left unchanged and the temp removed; previously the destination was overwritten in place (plain STOR), which such servers allowed. Servers with MoveFile-style rename semantics (possibly Windows/IIS FTP, UNCONFIRMED) may be affected as well. A failed or cancelled copy never touches a pre-existing destination.
- **Orphan reporting contract:** if the temp cannot be removed after a failure or cancellation, the returned error names it (`temporary file <path> may remain on the server`; typed `*orphanError`, reachable with `errors.As`, also on the cancel path) and a log line is written. Whether a failed cleanup DELE means "the temp never existed" is decided from the 550 reply text: only a text naming a missing file/directory (no such file, no such directory, not found, does not exist) counts, and any mention of permission, privilege, denial or read-only vetoes it (`550 No such privilege` and `550 File not found or permission denied` are reported as possible orphans). The heuristic is imperfect (servers word it differently, localised servers differ); an unrecognised or mixed wording is reported as a possible orphan rather than ignored, so a miss errs towards reporting.
- **Known limits:** once the transfer is confirmed the cancel watcher is disarmed, so a cancel during the final rename does not close the connection at once (a responsive server still commits). A rename that then gets no answer is cut off after `renameGraceAfterCancel` (5s after the cancel): the session is closed and the error says the rename "may or may not have been applied". A connection that dies during the rename is reported the same way, not as "left unchanged". The single-session spool fallback has no such cut-off. Evidence is unit-class against a fixture server; no live-NAS proof.
- **Scanner:** the catalogue scanner (`internal/services/generic_scanner.go`, FTP storage only) skips *files* named exactly `.<anything>.ftpcopy-<16 lowercase hex>.part` (in-flight or stale `CopyFile` temps; the pattern also matches names containing a newline); a user's own look-alike names such as `.x.part` are still catalogued. `FTPClient.ListDirectory` itself reports every entry, so callers that list, move and remove a directory (`moveDirectory` in `internal/services/protocol_handlers.go`) see a stale temp instead of failing the final remove with a confusing "not empty". Stale temps left by a crashed copy are not removed automatically.
- The single-session fallback spool directory is configurable (`FTPConfig.SpoolDir`, default the OS temp dir); free space is computed from `f_frsize` on linux and `f_bsize` on darwin/freebsd (build-tag split, so darwin builds); other platforms report free space as unknown. A negative `f_bavail` counts as 0.
- Evidence class: unit tests against a hand-written FTP responder; no live-NAS run yet (owed).

#### Fixed
- **Challenge framework**: `RunSequence` now performs Kahn topological sort on dependencies, fixing `RunByCategory` alphabetical-order failures
- **Challenge streaming**: `RunAll` and `RunByCategory` handlers support NDJSON streaming mode (`?stream=true`) to eliminate tens-of-MB buffer blocking
- **Challenge timeout**: per-challenge timeout reduced from 72h to 30min as a hard ceiling distinct from stale-progress detection
- **database-connectivity hang**: `DatabaseConnectivityChallenge` now implements progress reporting, enabling 5-min stale detection
- **Android TV lint**: 49 pre-existing lint errors fixed via `@SuppressLint` for legitimate `androidx.tvprovider` and Media3 API usage
- **Android TV package**: challenge config corrected from `com.vasic.catalogizer.tv` to `com.catalogizer.androidtv`
- **Security**: axios SSRF vulnerability patched in `Catalogizer-API-Client-TS`
- **PlaybackTracker**: added `ensureActive()` and re-throw `CancellationException` to prevent coroutine leaks
- **PlaybackTrackerTest**: stabilized by removing shared `TestScope`, eliminating `UncompletedCoroutinesError`

## [2.4.2] - pre-release (NOT production-ready)

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Prepared on branch `release/v2.4.2-prep`, merged up to `main` `1538ef19` (commits since the `v2.4.1` tag: `git log v2.4.1..HEAD`). Not tagged, not published. Versioning is SemVer. The `v2.4.1` annotated tag object is `a90896a3`; its commit is `04c6d422`.
Version files bumped to 2.4.2: `versions.json` (global 2.4.2), `docs/api/openapi.yaml`, `catalog-web`, `catalogizer-desktop` and `installer-wizard` (package.json, package-lock.json, tauri.conf.json, Cargo.toml), Android `versionName` 2.4.2 / `versionCode` 8 (was 7), Android TV `versionName` 2.4.2 / `versionCode` 10 (was 9), the two Android `CLAUDE.md` version lines and the installer-wizard AppImage file name in `challenges/scripts/desktop_appimage_launch_challenge.sh`. Checked with the new `scripts/release/check_release_consistency.sh --version 2.4.2 --prev-tag v2.4.1` (see the VERIFIED row).
**What this candidate is NOT:** the full-suite retest (11.4.40), the 11.4.185 manual QA and the live UI QA have NOT been run on it; nothing here may be called production-ready.

### Changed (user-visible)
- **Web `CollectionPreview`: Enter-to-toggle was restored by the SONAR-7 follow-up round** (FU-2, `6c063630`); the intermediate state in which Enter did not toggle the checkbox did not ship in this release. Space toggles as well. Template cards are native buttons, and the upload text is honest about what the control does. Other web controls changed from `role="button"` / `role="checkbox"` elements to native elements for Sonar S6819 (`AdvancedSearch`, `CollectionExport`, `CollectionTemplates`, `DiscoveredShares`, `HistoryDrawer`, `SubtitleManager`, `FavoriteToggle`); no live browser check of these has been run (FU-3: `HistoryDrawer` backdrop stacking needs a real browser).

### VERIFIED (committed machine evidence, checked while preparing this entry)
- SonarQube run4 measurement exists and verifies from the committed tree: `sha256sum -c SHA256SUMS` in `docs/qa/sonarqube-20261111/run4/` passes (0 failures). Scanned tag `v2.4.1` (`04c6d422`), so it measures the code BEFORE the SONAR-7 fixes below. Recorded result: all 60 targeted findings gone (22 CRITICAL, 2 api `go:S3923`, 5 web `typescript:S2245`, 15 web bugs, 16 web `typescript:S2699`); api 699 -> 687 open (648 CRITICAL `S1192`/`S3776` unchanged), web 404 -> 351 open; 13 NEW web code smells; quality gate api OK, **web ERROR** (13 new violations; the coverage condition is not meaningful, no coverage report was fed in). This verifies the recorded measurement only; it does not mean the gate is green.
- Cover-art SSRF live test (CAT-COVERART-LIVE-1 / CAT-QA-COVERART-1): `docs/qa/coverart/results/final/` holds baseline PASS (10 required tests), 9 mutants all KILLED (caveat: `results/final/SHA256SUMS.txt` FAILs `sha256sum -c` on `summary.txt`, follow-up E-1; the PASS rests on the hash-valid `baseline/` and `mutant-*` files only), `MUTATION-SUITE: PASS`, and a `-race` run, produced from committed tree `25e122e4` under a network namespace. Scope is stated in `docs/qa/coverart/README.md`: fixture servers are fixture INPUTS (11.4.289 C(3)); the 32 MiB cap and redirect legs run on `netguard.Client`, only two legs use the production `StrictClient`; one host, N=1. This is why the SSRF items stay PROVISIONAL below.
- Partial-retest evidence file exists and is sha-verified (`docs/qa/retest-20261112/`); its content is PROVISIONAL (below).
- Release-consistency checker `scripts/release/check_release_consistency.sh` exists (script + reviews; hardening PROVISIONAL) (CAT-REL-CHECK-1) with fixture suite and mutation runner; two Opus review rounds, GO-with-follow-ups (follow-ups open, below).
- SonarQube run5 evidence exists and is sha-verified (`docs/qa/sonarqube-20261111/run5/`, scanned `main` `563617b1`). Listed here only as "evidence exists"; its content is static analysis and is PROVISIONAL (below).
- Carried from 2.4.1, not re-run: the VERIFIED rows of `[2.4.1]` / `[2.4.0]`.

### PROVISIONAL (landed on `main`; counts as done for planning, NOT verified)
Pending job per row; machine evidence goes under `docs/qa/`; due before any non-pre-release tag.
- **Sonar run4 findings fixed on `main` after the scan** (CAT-QA-SONAR-7, merge `0a926439`, `60bc821d`; Opus GO-with-follow-ups): the 13 new web smells have a fix landed and run5 (below) reports them cleared with web gate OK. **Static Sonar clearance stays PROVISIONAL.** Pending job: SonarQube re-run on the v2.4.2 tag.
- **Cover-art SSRF through the product path** (F2 32 MiB cap, F4 redirect / private-address blocking): live test and evidence exist (VERIFIED row) but the evidence class is fixture-input, `netguard.Client` for most legs, one host, N=1. Pending job: N>=3 runs and a production-client cap/redirect leg.
- **Web Sonar fixes from run3 (S2245, bugs, S2699, S3735, S3776, api S1186, S3923)**: run4 measured them gone; the measurement is verified, the items are kept PROVISIONAL until a Sonar scan of the v2.4.2 code (the web code changed again after run4).
- **Release-checker F1-F5 hardening** (CAT-REL-CHECK-1, merge `9b484c65`, `d58123e0`, `019f12b6`; Opus round-2 GO-with-follow-ups): CRLF, symlink, manifest-escape and fenced-heading fixes. Reported by the review/test run: 137 tests, 76/76 mutants killed. PROVISIONAL: tooling-class evidence only; GNU coreutils `sha256sum` was NOT exercised (the host runs a different implementation), so the BSD/escape handling is unproven there.
- **Web follow-ups FU-1..FU-7** (CAT-QA-SONAR-7, merge `6278cdac`, `6c063630`; Opus GO-with-follow-ups): Enter-to-toggle restored, native button template cards, honest upload text, added tests. PROVISIONAL: jsdom only. FU-3 (HistoryDrawer paint order / backdrop stacking) needs a real browser check and is owed. Round-2 nits open: FU2-1 (stacking test is structural only), FU2-2 (Enter handler lacks `!e.repeat`), FU2-3 (template buttons still contain block elements), FU2-4 (`onActivateKey` has no production callers; investigate per 11.4.124 before any removal).
- **SonarQube run5 on `main` `563617b1`** (`docs/qa/sonarqube-20261111/run5/`): api gate OK, 687 open (no change from run4; 648 CRITICAL `S1192`/`S3776` remain); web gate OK (was ERROR), the 13 run4 smells cleared, web open 351 -> 338, 0 hotspots, no new issue. **Static analysis only: PROVISIONAL.** A closed finding means Sonar no longer reports it, not that the product works. This supersedes the "no re-scan exists" statements of the SONAR-7 row for the scanned commit; the v2.4.2 tag itself is still unscanned.
- **Partial retest of `main` `563617b1` vs `v2.4.1`** (evidence file `docs/qa/retest-20261112/RETEST_main_563617b1.md`, sha-verified via `SHA256SUMS`; PROVISIONAL, a summary log, not raw output): Go 60 packages run (some in batches, unscoped under `nice` after the systemd scope was SIGTERM'd, host load 10-18), web 2585 passed / 14 failed. **No regression among the 6 Go + 14 web failures**: each also fails on `v2.4.1` (pre-existing). Failing tests: Go: `config` TestDocsAdminPasswordExamplesAreAllRejected; `internal/nilslice_guard` TestNoUnnormalisedNilListReturns; root `catalogizer` TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions; `internal/tests/examples` TestUserRepositoryExample; `services` TestCAT002_Iso_LimitsAreHostAdaptiveAndBounded; `tests` TestConversionIntegration. Web: 7 `module-registry` tests, 7 `AuthContext.integration` tests, and load failures of the files Avatar, EmptyState, LoadingSpinner, WebSocketContext, WebSocketContext.connectionStatus, websocket (8 files, 14 tests). Separately, `tests/stress` TestCacheStress_HighThroughput/CacheWriteThroughput: load-dependent failure on main only (1 of 3 runs, 74 ops/s; it PASSED on `v2.4.1`); regression UNCONFIRMED, a `v2.4.1` run under the same load is owed. Not run: desktop, installer-wizard (no node_modules), cargo/Tauri (no toolchain), live stacks, NAS. This is **NOT** the 11.4.40 full-suite retest of the tag and does not satisfy it. Note: the frozen, sha-pinned evidence file's own wording "Regressions: NONE" / "not a regression" for the stress test predates the qualification above and is not edited. The pre-existing failures are registered as follow-ups `CAT-PREEXIST-1..` in `docs/FOLLOW_UPS_NEXT_PHASE.md`.
- **Release-consistency checker hardening N1-N5** (merge `9f4fecfa`, `c47d55e5`; superseded in scope by F1-F5 above): unit and mutation class tests only; the checker checks version fields and changelog evidence paths, not product behaviour.
- All `[2.4.1]` PROVISIONAL rows not listed above stay PROVISIONAL under their existing ids (SMB hardening SMB-FU-10/12/14, Anton `Public/Testing` SMB run at N=1, QA stack scripts via ctrctl, lifecycle follow-ups, Tauri desktop and wizard native boot, Android real login, UI D7 contrast, FTP `CopyFile`, and the others there).

### REOPENED
- None recorded between `v2.4.1` and the merged `main` tip `1538ef19`: no live rerun FAILed on a previously closed item. This is the absence of recorded failures, not proof that nothing regressed (the full-suite retest has not been run). Note: run4's web quality gate ERROR is a finding set (13 new smells), not a reopened closed item; those smells are tracked under CAT-QA-SONAR-7.

### OVERDUE
- The `[2.4.1]` verification jobs were due "next release" (register row CAT-REL-V241): full-suite retest (11.4.40), SonarQube re-run, 11.4.185 manual QA on the v2.4.1 tag, live UI QA, and Anton `Public/Testing` SMB runs 2 and 3 (N=1 < N>=3). None of these has been run on the tag, so these are **verification OVERDUE** and are re-run first. Also owed: manual QA (11.4.185); live UI QA with Vision (the llmctl `vision` start gate hung about 80 s on this host, root cause UNCONFIRMED, and no Vision validation PASS exists); stack rebuild from the tag; the full-suite retest of the tag (the partial retest above is not it); GNU `sha256sum` exercise of the checker; the FU2-1..4 nits; desktop / installer-wizard / cargo suites. The `[2.4.0]` rows not promoted are likewise still OVERDUE.

### Open follow-ups
Authoritative list: `docs/FOLLOW_UPS_NEXT_PHASE.md`. Newly registered in this window: CAT-QA-COVERART-1 (E-1 manifest after summary line and `race/`, E-2 stray `UNKNOWN` line, 4 surviving mutants), CAT-REL-CHECK-1 round-1 N1-N5 (landed) and round-2 F1-F5 (F1: backslash-prefixed BSD manifest lines bypass the escape check; fix soon), CAT-QA-SONAR-7 FU-1..7 (FU-1 missing checkbox click/Space test, FU-3 browser check of HistoryDrawer, FU-4 FavoriteToggle "Loading..." untested, FU-5..7 nits).

### Skipped live tests
- **NAS SMB / FTP / NFS live tests remain SKIPPED (`hardware_not_present`); the drives have been powered off since 2026-11-05.** NAS is NOT live-verified for this candidate; no test double was substituted.

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
- **SMB subdirectory roots on the Anton host Samba, share `Public`, sub `Testing`** (CAT-SMB-SUBDIR-1): one live run recorded on `main` `e5817850`, 2026-11-11T19:03:25Z, `TestLiveSMBSubdir` (4 outside-name probes) and `TestLiveSMBServiceSubdir` PASS. Scope: real host Samba, reached by host name from an env file; NOT a bare-IP proof, NOT NAS. Evidence: `docs/qa/smb-subdir-20261110/live-run-anton-20261111T190325Z/` (log committed in `652928dc`, `sha256sum -c go_test.log.sha256` passes (run from repo root); single run, N=1). Why it is not VERIFIED: N=1 < required N>=3. Pending job: two more runs of `docs/qa/smb-subdir-20261110/run.sh <0600 env>` with committed logs.
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
- **NAS SMB / FTP / NFS live tests remain SKIPPED (`hardware_not_present`); the drives have been powered off since 2026-11-05.** NAS is NOT live-verified for this candidate, no test double was substituted.

## [2.4.0] - pre-release (candidate, to be published as a labelled pre-release; PROVISIONAL items below are not verified)

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Candidate state, brought up to date with `main` `80d9b2d8` (previously computed at `17ec3c1f`; about 159 first-parent commits since 2026-10-28 plus the landings listed below; no git tag exists yet).
Version files: `versions.json` (global 2.4.0), `docs/api/openapi.yaml` (info.version 2.4.0). Components `catalog-web`, `catalogizer-desktop`, `installer-wizard` (package.json, tauri.conf.json, Cargo.toml) and the Android / Android TV `versionName` were already 2.4.0. Versioning is SemVer (operator decision 2026-11-03); the decimal-carry rule of 11.4.235(E.5) does not apply.

### VERIFIED (live evidence on QA stack 3, machine evidence under `docs/qa/`; each row ran on the image or commit named in that row, NOT on the release candidate; re-run on the candidate before a non-pre-release tag per 11.4.135)
- Stack 3 rebuilt from `main` `d9238b47`: 54 PASS / 0 FAIL / 1 SKIP, 3 identical rounds; Yandex WebDAV read-only suite 3/3. The NAS SMB/FTP read-only suites also ran 3/3 on `d9238b47` but are NOT counted as verified for this candidate: they predate the SMB-FU-2/SMB-FU-9 dial changes and the dead-SMB removal and NAS is powered off (see Skipped live tests); they are PROVISIONAL, pending job: re-run when NAS is back (`docs/qa/stack3-rebuild-live-20261105/`). Promoted to VERIFIED: conversion D1, D2, D10; PDF pages D3; document full-text search D4; D5; subtitle D6 and DELETE route; admin role change D9; D7, D8, D11a, D11b; storage-client live suites (SMB/FTP/WebDAV).
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
- Live gaps: LV3-1 non-privileged cross-user conversion isolation (SKIP); LV3-2 storage idle-deadline and cut-recovery against real NAS (loopback/unit only); LV3-3 D5 bursty-audio fixture; LV3-4 stale fixture rows; LV3-5 PostgreSQL multi-process last-admin race; STK3L2-FU-1 no delete API for storage roots; STK3L2-FU-2 `/smb/discover` returns 500 on timeout.
- UI: UI-FAV-1 favourites link the wrong entity (pre-existing, important); UI-D4 `/ws` idle closes (unreproduced) and login double-connect; UI-D1-FU-1/2 conversion cancel verb and request shape; UI-D2-FU-1 identity CRUD routes missing; UI-D3-FU-1/4 retries on 404.
- Security residuals: path-confinement TOCTOU (F4, `os.Root` sinks) and hard-link policy (operator decision); secrets scanner residuals; key-rotation waiver is an operator-accepted open risk with no expiry.
- Process: container-hygiene guard parser gaps; HelixQA web Vision bank and CDP wiring (needs host capacity decision); upstream `helix_qa` credential option; operator decisions pending listed in the follow-ups file (amber podman, `thinker.local`, `catalogizer/server` image).
- Not part of this release: SonarQube analysis (blocked on image access).

### Skipped live tests
- **NAS SMB / FTP / NFS live tests are SKIPPED (`hardware_not_present`): the NAS drives were powered off by the operator on 2026-11-05 19:09 CET. NAS is NOT live-verified for this candidate.** No test double was substituted; the earlier read-only NAS suites (3/3 on `d9238b47`) predate the power-off. NFS was already `topology_unsupported` (no exports).

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
