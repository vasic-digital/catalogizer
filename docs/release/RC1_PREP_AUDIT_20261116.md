# v2.5.0-rc.1 preparation audit (gates G1, G6, G8, G10, G11, plus the 11.4.298 live run)

| Field | Value |
|---|---|
| Revision | 2 (review round 1: NO-GO fixed, I-1 and m-1..n-5) |
| Created | 2026-11-17; the file name carries 2026-11-16, the date the audit was requested, not the date it was written |
| Based on | `main` `b0066508a` merged into scratch clone branch `gatechk5` (re-measured 2026-11-17; Rev 1 was `62ee38f00`); read-only audit, no product code changed |
| Status | DRAFT, PROVISIONAL, not independently reviewed |
| Gates satisfied | **0 of 5** sweep-7 rc.1 gates (G1, G6, G8, G10, G11), and the constitutional 11.4.298 live-run precondition is also unmet (section below). No gate has a captured pass record; nothing below is marked satisfied |
| Source of the rc.1 gate set | `docs/requests/background_queue.md` L116 (sweep 7: rc.1 may ship as a labelled pre-release with relaxed gates; G1, G6, G8, G10, G11 required; G3, G4, G7, G12 owed and listed) |
| Commands run | `grep`, `git ls-remote` (read-only, 15 remotes), `scripts/release/check_release_consistency.sh --version 2.5.0 --prev-tag v2.4.3` and the same with `--version 2.5.0-rc.1` (read-only, both exit 1), `git log`/`rev-list`/`remote -v`. No build, test, scan or live run |

## Constitutional precondition for any rc.1 tag: 11.4.298 per-product live run (OPEN, NOT in the sweep-7 gate set)

- Sweep 7 (`docs/requests/background_queue.md` L116) named G1, G6, G8, G10, G11 as the rc.1 gates. That list is NOT sufficient for a tag: 11.4.298(E) (`constitution/Constitution.md:13078`, refinement note at `:12831`) says a build without a completed live run on connected hardware of its product gets no tag of any class. The only thing a missing baseline allows is a labelled dev-class tag of a `BLOCKED-NO-BASELINE` build, and that tag still needs the live run to have been done and recorded (11.4.293(F) as amended by 11.4.298(E)).
- So the per-product live run (checklist G2) is a constitutional precondition of the rc.1 tag. Today all six products lack a live-run record: `web` is `BLOCKED-NO-BASELINE` (`docs/qa/baseline-v2.4.2-20261114/gate_web.rec`, baseline of v2.4.2 only), and api, desktop, wizard, Android, Android TV are `BLOCKED-NO-HARDWARE` (their `gate_*.rec`). No product is `RELEASABLE`; no live-run record exists for any candidate.
- UNCONFIRMED, operator question: whether the rc.1 tag may ship with G2 (and G5 Sonar, G9 open-items list) relaxed. Sweep 7 text lists G3, G4, G7, G12 as owed for rc.1 and is silent on G2, G5 and G9; the constitution does not allow a G2 relaxation on its own text. A recorded operator decision, or a recorded waiver under 11.4.271, would be needed; none is in the repo.

## G11 - version consistency (OPEN)

Checker run (`VERDICT: FAIL ... version_ok=0 version_fail=19`): every product file still reads 2.4.3.

| Product file | Current | Citation | Needed |
|---|---|---|---|
| Android `versionCode` | 9 | `catalogizer-android/app/build.gradle.kts:20` | integer > 9 (checker: "9 is not > 9 at v2.4.3") |
| Android `versionName` | "2.4.3" | `catalogizer-android/app/build.gradle.kts:21` | the release version |
| Android TV `versionCode` | 11 | `catalogizer-androidtv/app/build.gradle.kts:32` | integer > 11 |
| Android TV `versionName` | "2.4.3" | `catalogizer-androidtv/app/build.gradle.kts:33` | the release version |
| `versions.json` global | 2.4.3 (major 2, minor 4, patch 3, build 25) | `versions.json:4-7` | 2.5.0 |
| OpenAPI `info.version` | 2.4.3 | `docs/api/openapi.yaml:15` | 2.5.0 |
| web | 2.4.3 | `catalog-web/package.json:3` (+ lock, 2 fields) | 2.5.0 |
| desktop | 2.4.3 | `catalogizer-desktop/package.json:4` (+ lock 2), `catalogizer-desktop/src-tauri/tauri.conf.json:3`, `catalogizer-desktop/src-tauri/Cargo.toml:3` | 2.5.0 |
| wizard | 2.4.3 | `installer-wizard/package.json:4` (+ lock 2), `installer-wizard/src-tauri/tauri.conf.json:3`, `installer-wizard/src-tauri/Cargo.toml:3` | 2.5.0 |
| Go API | `Version = "dev"` (ldflags at build) | `catalog-api/main.go:73` | no file edit; the checker has no Go version check, build-time injection UNCONFIRMED here |
| `catalogizer-api-client`, `Website` | 1.0.0 | `catalogizer-api-client/package.json:3`, `Website/package.json:3` | out of the checker's scope (own version) |

Open question: whether rc.1 uses `--version 2.5.0-rc.1` or `2.5.0` is UNCONFIRMED. The checker usage line is `--version X.Y.Z` (`scripts/release/check_release_consistency.sh:30`). Tested read-only 2026-11-17: `--version 2.5.0-rc.1` is accepted syntactically (no usage error), prints `BLIND: section not found: 2.5.0-rc.1`, ends `VERDICT: FAIL ... version_ok=0 version_fail=19`, exit 1 (same counts as `2.5.0`; the failures read `expected '2.5.0-rc.1'`). Whether a CHANGELOG section named `[2.5.0-rc.1]` would satisfy it was not tested.

Command: `scripts/release/check_release_consistency.sh --version 2.5.0 --prev-tag v2.4.3` (G6 check 1 is the same run).

## G6 - release checker (PROVISIONAL tool, gate OPEN)

- Last line today: `VERDICT: FAIL section=- version_ok=0 version_fail=19 changelog_paths=0 ...`, exit 1. Also prints `BLIND: section not found: 2.5.0`.
- Exit codes (`check_release_consistency.sh:1349-1359`): 0 for `VERDICT: PASS` and for `PASS-WITH-WAIVERS`, 1 FAIL, 2 BLIND. A pass needs the last line exactly `VERDICT: PASS` (checklist G6); `PASS-WITH-WAIVERS` exits 0, relies on declared evidence gaps, and does NOT satisfy the gate unless the operator accepts the waivers.

## G8 - CHANGELOG 2.5.0 section (OPEN, missing)

- `docs/CHANGELOG.md` has `## [Unreleased]` at L8 (empty) and `## [2.4.3]` at L10 as the topmost release section; no `## [2.5.0]` heading exists (`grep -n -E '^## \[' docs/CHANGELOG.md`, first hit after Unreleased is 2.4.3).
- The checker requires a `### VERIFIED` subsection in the section (`check_release_consistency.sh:792`, error text at L802); every backticked repo path there must exist, be tracked and match a `SHA256SUMS`. The required separate sections are VERIFIED / PROVISIONAL / REOPENED / OVERDUE (11.4.290(G)).
- Today's VERIFIED set is empty: 0 commit subjects since v2.4.3 say VERIFIED (`git log --format=%s v2.4.3..b0066508a | grep -c VERIFIED` = 0). Positive control for the same instrument over all history: `git log --format=%s | grep -c VERIFIED` = 9, so the grep can see the word; negative control `ZZQQ` = 0. Those 9 predate v2.4.3.

## G1 - full-suite retest on the candidate (OPEN)

- Only prior retest directory: `docs/qa/retest-20261112/` (`RETEST_main_563617b1.md`, `SHA256SUMS`), a different fingerprint; no retest of the rc.1 candidate exists. Needs `docs/qa/retest-<date>/` with `SHA256SUMS`, same candidate fingerprint, no SKIPPED component.

## G10 - release hosts parity (OPEN)

- Measured 2026-11-17 at `62ee38f00` (Rev 1; not repeated after the merge, main has since moved to `b0066508a`), `git ls-remote <r> refs/heads/main 'refs/tags/v2.5*'` on the 15 remotes (`codeberg`, `gitflic-vasic`, `gitflicvasicdigital`, `github`, `github-milos`, `githubvasicdigital`, `gitlab`, `gitlab-milos`, `gitlab-vasic`, `gitlabvasicdigital`, `gitverse-vasic`, `gitversevasicdigital`, `m3w`, `origin`, `upstream`): all answered `main` = `62ee38f008ef0a8077efb33b6e441a6879251ff3`; no `v2.5*` tag line returned. Parity holds for main only; there is no rc tag to compare.
- Release tool CON-412: constitution `scripts/forge/forge.sh` (constitution checkout `1dfbb4d19`; not initialised in the scratch clone, read in the main checkout). Registered at `constitution/docs/Issues.md:4810`; guide `constitution/docs/guides/multiforge_release_guide.md` (L28 `release-ensure`, L46-47 dry-run example); header usage `constitution/scripts/forge/forge.sh:8-14`; `--dry-run` = GETs only, prints what it would do (`forge.sh:29`, flag parse `:89`).
- Dry-run command per host. Tokens are read by variable NAME from a 0600 file and never printed. Even in a dry-run the tag must already exist on the forge and point at the given SHA; otherwise the run ends `RESULT: UNVERIFIED-COMMIT`, exit 5 (guide L143, `forge.sh:23`), so the tag has to be pushed before the dry-run can pass: `constitution/scripts/forge/forge.sh release-ensure --remote <name> --tag <rc-tag> --sha <40-hex> --notes-file <file> --prerelease --dry-run`. Hosts: github, gitlab, codeberg, m3w, gitflic, gitverse. Real remote names in the main checkout (`git remote -v`, 2026-11-17): `github`, `gitlab`, `codeberg`, `m3w`, `gitflic-vasic` (alias `gitflicvasicdigital`, same URL `git@gitflic.ru:vasic-digital/catalogizer.git`), `gitverse-vasic` (alias `gitversevasicdigital`, same URL `ssh://git@gitverse.ru:2222/vasic-digital/Catalogizer.git`); `--remote` takes these names, not the short host words. Exit codes: 2 USAGE, 3 GAP, 4 API, 5 UNVERIFIED-COMMIT (guide L31).
- Token validity per host and GitFlic/GitVerse write paths: UNCONFIRMED; write paths are fixture-proven only per the guide's own status line (PROVISIONAL).
- Push serially (sleep 4), fast-forward only, never force (checklist G10).

## Verdict

0 of 5 sweep-7 rc.1 gates satisfied, and the 11.4.298 live run is also missing. Missing: the G2 per-product live run with a gate record for each of the six products (constitutional precondition of any tag; today web `BLOCKED-NO-BASELINE`, five `BLOCKED-NO-HARDWARE`), version bumps in 14+ product files and both versionCodes (G11; the rc1ver bump is still pending, not landed), a `[2.5.0]` CHANGELOG section with VERIFIED/PROVISIONAL/REOPENED/OVERDUE (G8, then G6 can pass), a retest on the candidate (G1), an rc tag and forge dry-run plus pushes (G10). Operator question: may rc.1 ship with G2, G5 and G9 relaxed (UNCONFIRMED; constitution text says no for G2).

## Operator exception for rc.1 (recorded 2026-11-17, PROVISIONAL)

Operator decisions 2026-11-17 (relayed by the conductor, not verbatim; identifier UNCONFIRMED): rc.1 may ship as a LABELLED PRE-RELEASE with G2, G5 and G9 relaxed. This is a recorded operator exception to 11.4.298(E) for rc.1 ONLY. v2.5.0 keeps G1..G12 and the per-product live run. Emulator runs are allowed as NON-GATE dry-run pre-checks only and never count as gate evidence (see `docs/release/RC1_DRYRUN_PLAN_20261117.md`). The gate record per product stays honest: web `BLOCKED-NO-BASELINE`, api, desktop, wizard, Android, Android TV `BLOCKED-NO-HARDWARE`; the exception does not change any record. The rc.1 tag MUST carry the pre-release label and cite this exception (this section plus the operator decision id once it is known) in its notes. Answers the "Operator question" in the Verdict above: yes for rc.1, no for v2.5.0.
