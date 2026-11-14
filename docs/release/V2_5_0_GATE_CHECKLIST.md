# v2.5.0 production release gate checklist

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-11-14 |
| Based on | `main` `25cb9e40c` (clone read only, no runs executed); `docs/CONTINUATION.md` Rev 21 |
| Status | DRAFT, independently reviewed 2026-11-14: NO-GO round 1; revised. Every state below is "as of the repo text now", not a fresh measurement |
| Sources | `docs/CONTINUATION.md` (Rev 21; its section 7a is not re-checked here), `docs/FOLLOW_UPS_NEXT_PHASE.md`, `docs/WORKABLE_ITEMS_REGISTER.md`, `docs/CHANGELOG.md`, `docs/requests/background_queue.md` (BG-OPQ-2026-11-13b), `scripts/release/*`, `docs/qa/*` |

Rules: owner state is one of OPEN / PROVISIONAL / DONE / UNCONFIRMED. Nothing here is DONE: no gate has a machine record in the repo yet. A gate passes only with its evidence file (path + sha256) and the command's own verdict line; INCONCLUSIVE, BLIND and SKIPPED never count as pass (11.4.293, 11.4.298). Tokens are named by variable only, never printed. This is a checklist, not evidence.

Release class: v2.5.0 is a PRODUCTION release (operator decision 2026-11-13, BG-OPQ-2026-11-13b). Every gate below (G1 to G12) is required for v2.5.0. No rc exemption is recorded; any rc carve-out needs a recorded operator decision (UNCONFIRMED). Operator decisions found in the repo: "Vision QA REQUIRED for v2.5.0 production" (`docs/requests/background_queue.md` L64) and "v2.5.0 gates = retest+live run, Vision QA, SEC-WS-1 F3 + live /ws, Sonar zero blocker/critical" (L97); neither text records an rc exemption.

Follow-ups accepted under 11.4.276(G.2) (GO-with-follow-ups) ride the release as tracked items and are listed in the changelog; they do not replace any gate.

## Gates

| Id | Gate | Proof (evidence file / command) | State now | Blocker items |
|---|---|---|---|---|
| G1 | 11.4.40 full-suite retest on the tag candidate fingerprint, the same candidate fingerprint as G2 to G5; a SKIPPED component blocks (Go, web, desktop, wizard, cargo, Android as applicable) | Retest record under `docs/qa/retest-<date>/` with `SHA256SUMS`; candidate commit sha recorded in it. Existing `docs/qa/retest-20261112/RETEST_main_563617b1.md` is partial (Go ~60 pkgs, web 2585 pass / 14 fail; desktop, wizard, cargo not run) and is for another commit, so it does not count | OPEN (OVERDUE per CONTINUATION Rev 21) | CAT-PREEXIST-1..10 (all PROVISIONAL-DONE, 9b Queued; failures also present on v2.4.1); CAT-TEST-PREEXIST-1 |
| G2 | 11.4.298 per-product live run on connected physical hardware, compared with baseline; verdict NO-WORSE-PROVEN; first baseline = live run of v2.4.2 (note per 11.4.293(B): if v2.4.3 is flashed first, v2.4.3 becomes the previous image and the baseline must be its run instead) | `scripts/release/release_gate.sh --product ID --hardware-record F --live-run-record F --comparison-record F --out R` must end `GATE-STATE: RELEASABLE` (exit 0); comparison from `scripts/release/image_no_worse.sh`. Tests: `scripts/release/test_release_gate.sh`, `test_image_no_worse.sh`. Per product: BLOCKED-NO-HARDWARE (20) / BLOCKED-NO-BASELINE (21) / REFUSED-REGRESSED (22) all block | OPEN. Tooling exists in repo; no hardware record, live run or v2.4.2 baseline record found in the repo (UNCONFIRMED that none exists elsewhere). Products to cover: UNCONFIRMED list (repo has Android `catalogizer-android`, Android TV `catalogizer-androidtv`, web, desktop, wizard; which are "deployable builds on hardware" is not stated) | BLOCKER: a recorded declaration of the 11.4.298 product list and deployable-build kinds is required before G2 can be evaluated; each product without connected hardware is BLOCKED-NO-HARDWARE. Also needs the baseline run first. Synology live legs stay SKIP `hardware_not_present` (CAT-SYN-LIVE-1) and do not count as pass |
| G3 | Vision/UI QA with the smallest fitting llmctl profile; Vision QA REQUIRED for v2.5.0 production (no rc exemption recorded, see the rules above); Vision banks on EVERY shipped UI surface (web, Android, Android TV, desktop, wizard) per 11.4.278(A)/(B), catalogue 100% of non-waived entries read-verified, expected-content spec committed before the run, un-drivable surfaces recorded `topology_unsupported` / `operator_attended` with a tracked item, model identity per 11.4.278(L) in every verdict | `llmctl require-validated <profile>` PASS under `LLMCTL_VALIDATE_STRICT=1` (11.4.277), then HelixQA Vision bank runs on every shipped UI surface (web, Android, Android TV, desktop, wizard) against the candidate build, per-frame evidence (PNG + sha256, spec, verdict, model identity) under `docs/qa/`; an un-drivable surface carries its recorded reason code and tracked item | OPEN. No Vision validation PASS exists; the llmctl `vision` start gate hung ~80 s (root cause UNCONFIRMED) per CONTINUATION Rev 21. Plan: stop the decide model via `llmctl stop`, then `llmctl start vision`, no overrides, host load permitting | CAT-HELIXQA-VISION-1 (Queued); HELIXQA-FU-1/2 |
| G4 | SEC-WS-1 F3 closed (connected /ws socket re-checked after upgrade: revoked session or suspended user stops receiving) AND a live /ws client run (real catalog-web build and Android through the production nginx, token query + Origin) | Test that fails before / passes after the fix; live run record under `docs/qa/` carrying the 11.4.289(D) target fields (artifact fingerprint from the target, endpoint identity reached, answering-process identity); httptest evidence is UNIT-CLASS and does not satisfy the live part (cf. `docs/qa/sec-ws-1/`, `sec-ws-1r3/`) | OPEN. SEC-WS-1 auth landed PROVISIONAL (httptest level only; `docs/qa/sec-ws-1r3/{red,green}.txt` exist). F3 is listed as follow-up; live proof is SEC-WS-1-F1 (owed). Neither is in `WORKABLE_ITEMS_REGISTER.md` by id (UNCONFIRMED whether registered elsewhere) | SEC-WS-1-F3, SEC-WS-1-F1, SEC-WS-1-F2 (ticket instead of JWT in URL, minor), SEC-WS-1-F4, F5, SEC-WS-1R3FU-1..3 |
| G5 | SonarQube scan of the tag: zero blocker/critical | Run record under `docs/qa/sonarqube-<date>/runN/` with `SHA256SUMS`, quality-gate status and blocker/critical counts | OPEN, FAILING on the last scan: run5 (`563617b1`) catalog-api BLOCKER 0 / CRITICAL 648 (go:S1192 338, go:S3776 310; all CODE_SMELL); catalog-web BLOCKER 0 / CRITICAL 0 (source `docs/qa/sonarqube-20261111/run5/*.issue_facets.json`, numbers re-read in this revision). No scan of a v2.5.0 candidate. OPERATOR DECISION 2026-11-14 (interactive, recorded in project memory project_decisions_2026_11_14): the v2.5.0 Sonar gate covers BUG / VULNERABILITY / security hotspots only; code smells are tracked separately as CAT-QA-SONAR-CRIT-1 (api has 0 BUG, 0 VULNERABILITY on run5; a fresh scan of the candidate is still required) | CAT-QA-SONAR-CRIT-1 (new, to be registered: the 648 api CRITICAL findings, or an operator decision narrowing the scope), CAT-QA-SONAR-1 (PROVISIONAL-DONE), CAT-QA-SONAR-7; Sonar images need docker login (operator, 0600 file, waiting) |
| G6 | Release checker CAT-REL-CHECK BLOCKING for v2.5.0 (advisory for v2.4.3) | `scripts/release/check_release_consistency.sh --version 2.5.0 [--prev-tag v2.4.3]` last line exactly `VERDICT: PASS`; PASS-WITH-WAIVERS does NOT satisfy this gate unless the operator records acceptance of the named waivers in `docs/qa/EVIDENCE_WAIVERS.txt`; tests `scripts/release/test_check_release_consistency.sh`, mutations `mutation_check_release_consistency.sh` | PROVISIONAL. CAT-REL-CHECK-3 (HEAD-snapshot checker) landed on `main` in merge `33ee44041` with Opus round 4 GO-with-follow-ups (`docs/FOLLOW_UPS_NEXT_PHASE.md` L2491; CAT-REL-CHECK-1/-2 are its predecessors). Advisory for v2.4.3, blocking for v2.5.0. Open: its round-4 follow-ups (e.g. F4-1), the full 236-mutant set not run end to end (`docs/CHANGELOG.md` L74), GNU `sha256sum` path unexercised, RC-3 status UNCONFIRMED. | CAT-REL-CHECK-3 follow-ups (F4-1 ...), full mutant run, RC-3 |
| G7 | 11.4.185 manual QA confirmation | Operator/QA confirmation record (not machine-producible) | OPEN (OVERDUE per CONTINUATION Rev 21). UNCONFIRMED where the record should live; repo has `docs/qa/QA_TESTING_GUIDE.md`, `CATALOGIZER_QA.md` | none by id |
| G8 | Changelog `[2.5.0]` with separate VERIFIED / PROVISIONAL / REOPENED / OVERDUE sections (11.4.290) | `docs/CHANGELOG.md` section; checker G6 verifies every backticked VERIFIED path exists, is tracked and matches its `SHA256SUMS` | OPEN, depends on G12 (no `[2.5.0]` section yet; `[2.4.2]` is the model, `[2.4.3]` heading at `docs/CHANGELOG.md:10`, `[2.4.2]` at `:164`; the `[2.5.0]` PROVISIONAL section must be empty unless the operator re-classes the release as pre-release) | none |
| G9 | All open important-or-higher items listed with a state (section below) | This file's table, regenerated at tag time from `FOLLOW_UPS_NEXT_PHASE.md` + register | OPEN, list below is a first cut, not verified | see section |
| G10 | Release hosts pushed with fast-forward parity: GitHub + GitLab, Codeberg, m3w.org, GitFlic, GitVerse | `git ls-remote <remote> refs/heads/main` and the tag equal on every remote; push SERIALLY (sleep 4), no force. Local remotes seen in the main checkout: `github`, `gitlab`, `codeberg`, `m3w`, `gitflic-vasic`, `gitverse-vasic` (+ org/milos variants). Release PAGES on Codeberg and m3w need an API token by variable NAME in a 0600 file | OPEN. Codeberg/m3w release-page token: not found as of 2026-11-12 (operator to supply). GitFlic/GitVerse release-page mechanism: UNCONFIRMED | none by id |
| G11 | Version consistency 2.5.0 in every product file (Android `versionCode` strictly above v2.4.3's Android 9 / TV 11) | G6 check 1 (versions.json, openapi, package.json + lock, tauri.conf, Cargo.toml, build.gradle.kts) | OPEN | none |
| G12 | Every item contained in v2.5.0 is VERIFIED-DONE (11.4.290): reproduction `reproduced-live`, or `reproduced-host-fixture` on a host-only surface, and root cause found with evidence (11.4.292, 11.4.294); PROVISIONAL / UNREPRODUCED / OVERDUE content is verified or moved out of scope | Per-item records (reproduction + investigation state, evidence path + sha256); `[2.5.0]` PROVISIONAL changelog section empty (see G8) | OPEN: per `docs/CONTINUATION.md` Rev 21 L16, 328 commits / 101 Roadmap-Item ids landed since Rev 20, all PROVISIONAL, plus the `[2.4.3]` PROVISIONAL list in `docs/CHANGELOG.md`; per-item VERIFIED count not measured | every PROVISIONAL-DONE item in the list below |

## Open important-or-higher items (first cut)

Source: `docs/FOLLOW_UPS_NEXT_PHASE.md` (line numbers at `main` `10f270b7e`) and the register. State is what the text says; where the text says nothing newer, UNCONFIRMED. Items marked FIXED/PROVISIONAL in the follow-ups file still need the G1 retest to count.

| Item | Text says | State |
|---|---|---|
| SEC-WS-1 (+F1..F6, R3) | /ws auth, important candidate (L2094) | PROVISIONAL; F3 and live run OPEN (G4) |
| CAT-D9-SUSPEND | important, security (L1279) | PROVISIONAL-DONE, merge `811da03f`; follow-ups D9S-F5/F6/F7, M1/M4/M5 open |
| SECQ-1 (= W5) | WebDAV storage-root client has no outbound guard (L1392) | Operator decided: apply netguard (branch fix/opdec-b); landed? UNCONFIRMED |
| SEC-DIAL-1 | SMB storage roots dial without vetting (L2369) | UNCONFIRMED |
| CAT-LOGIN-LOCK-1 / -2 | no per-account lockout (L2370) | -2 PROVISIONAL-DONE, landed `3a33fb05` (Opus round 5 GO-with-follow-ups) |
| CAT-CHPW-RL-2 | change-password limiter successor (PKGC-FU4-F2, L2345) | PROVISIONAL-DONE, landed `2c799a9f` (Opus round 5 GO-with-follow-ups) |
| CAT-SMB-COPY-1 | SmbClient.CopyFile truncates destination (L2426) | OPEN, unconfirmed live |
| CAT-FS-SYMLINK-1 | symlink confinement | PROVISIONAL, landed `52afef317`; follow-ups open |
| CAT-JOB-404-1 / CAT-SYNC-404-1 | non-owner job id 404 | PROVISIONAL, landed `e16b75d0b`; CAT-SYNC-404-1 registered, OPEN |
| LOGSHARE-TOK-1-F1 | v28 migration renumber order (L2387) | UNCONFIRMED |
| AZ2-R1 | resolvers follow redirects, SSRF (L1111) | UNCONFIRMED (cover-art SSRF live test CAT-QA-COVERART-1 landed PROVISIONAL) |
| PKGA-FU2 | `blockedExact` lacks `fd20:ce::254` (L2330) | UNCONFIRMED |
| P2 / P2-t / N2 | missing permission checks on storage/scan routes, analytics user_id forgery, collections without owner (L759, L972, L1027) | UNCONFIRMED (N2 "closed by" a later branch per text) |
| F1 ffmpeg hardening | ffmpeg/convert on user files, no -nostdin/-protocol_whitelist (L910) | UNCONFIRMED; operator chose static-ffmpeg image slim 2026-11-13 |
| UI-FAV-1 | favourites link wrong entity | Queued (register L33) |
| D7-R2-F1 | dark-mode contrast class (L1531) | Queued |
| TAURI-F6 | native DialogConfirmer never shown on a display (L1564) | CAT-TAURI-NATIVE-1 PROVISIONAL-DONE; display check UNCONFIRMED |
| F3 (L402) | client calls with no route in `main.go` | UNCONFIRMED |
| CAT-TRUSTEDPROXIES-PROD-1 / -AUTO-1 | important (register L135-136) | Ready for testing (PROVISIONAL) |
| CAT-PREEXIST-1..10 | pre-existing test failures | PROVISIONAL-DONE except 9b Queued (G1 blockers) |
| RC-3 | checker passes VERIFIED path with no manifest (L1828) | UNCONFIRMED whether CAT-REL-CHECK-3 closed it |
| E1, E2, E4, E5 (L297-310), D8-F1, D9-F1, F2/F5 test-instrumentation (L807-808, L408) | important test-instrumentation | UNCONFIRMED |

## Not release-blocking by decision, kept visible

CAT-KEYROT-WAIVER-1 (operator-accepted open risk); Synology live tests SKIP `hardware_not_present` (never counted as pass); Jev stays SHADOW; API image size 761 MB accepted (static-ffmpeg slim planned); LLMCTL_API_KEY rotation (operator).

## Tag-time procedure (order)

1. Land everything on `main` fast-forward (11.4.288); `git fetch --all`, parity check.
2. G12 (all contained items VERIFIED-DONE or moved out) and the Sonar scope decision (G5), before anything else is built or run. Then G11 + G8, then G6 on the candidate commit.
3. G1 retest, G5 scan, G4 live /ws, G3 Vision, G2 live run + comparison + gate record, all on the same candidate fingerprint.
4. G7 manual QA confirmation, then tag and push serially to all G10 remotes; verify parity.
