# v2.5.0-rc.1 preparation audit (gates G1, G6, G8, G10, G11)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-17 (file name carries the audit date 2026-11-16 as requested) |
| Based on | `main` `62ee38f00`, scratch clone branch `gatechk5`; read-only audit, no product code changed |
| Status | DRAFT, PROVISIONAL, not independently reviewed |
| Gates satisfied | **0 of 5** rc.1 gates (G1, G6, G8, G10, G11). No gate has a captured pass record; nothing below is marked satisfied |
| Source of the rc.1 gate set | `docs/requests/background_queue.md` L116 (sweep 7: rc.1 may ship as a labelled pre-release with relaxed gates; G1, G6, G8, G10, G11 required; G3, G4, G7, G12 owed and listed) |
| Commands run | `grep`, `git ls-remote` (read-only, 15 remotes), `scripts/release/check_release_consistency.sh --version 2.5.0 --prev-tag v2.4.3` (read-only, exit 1), `git log`/`rev-list`. No build, test, scan or live run |

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

Open question: whether rc.1 uses `--version 2.5.0-rc.1` or `2.5.0` is UNCONFIRMED. The checker usage is `--version X.Y.Z` (`scripts/release/check_release_consistency.sh:34`) and I did not test an rc suffix.

Command: `scripts/release/check_release_consistency.sh --version 2.5.0 --prev-tag v2.4.3` (G6 check 1 is the same run).

## G6 - release checker (PROVISIONAL tool, gate OPEN)

- Last line today: `VERDICT: FAIL section=- version_ok=0 version_fail=19 changelog_paths=0 ...`, exit 1. Also prints `BLIND: section not found: 2.5.0`.
- A pass needs the last line exactly `VERDICT: PASS` (checklist G6); PASS-WITH-WAIVERS exits 0 but does not satisfy the gate.

## G8 - CHANGELOG 2.5.0 section (OPEN, missing)

- `docs/CHANGELOG.md` has `## [Unreleased]` at L8 (empty) and `## [2.4.3]` at L10 as the topmost release section; no `## [2.5.0]` heading exists (`grep -n -E '^## \[' docs/CHANGELOG.md`, first hit after Unreleased is 2.4.3).
- The checker requires a `### VERIFIED` subsection in the section (`check_release_consistency.sh:792`, error text at L802); every backticked repo path there must exist, be tracked and match a `SHA256SUMS`. The required separate sections are VERIFIED / PROVISIONAL / REOPENED / OVERDUE (11.4.290(G)).
- Today's VERIFIED set is empty: 0 commit subjects since v2.4.3 say VERIFIED (`git log --format=%s v2.4.3..HEAD | grep -c VERIFIED` = 0).

## G1 - full-suite retest on the candidate (OPEN)

- Only prior retest directory: `docs/qa/retest-20261112/` (`RETEST_main_563617b1.md`, `SHA256SUMS`), a different fingerprint; no retest of the rc.1 candidate exists. Needs `docs/qa/retest-<date>/` with `SHA256SUMS`, same candidate fingerprint, no SKIPPED component.

## G10 - release hosts parity (OPEN)

- Measured 2026-11-17, `git ls-remote <r> refs/heads/main 'refs/tags/v2.5*'` on the 15 remotes (`codeberg`, `gitflic-vasic`, `gitflicvasicdigital`, `github`, `github-milos`, `githubvasicdigital`, `gitlab`, `gitlab-milos`, `gitlab-vasic`, `gitlabvasicdigital`, `gitverse-vasic`, `gitversevasicdigital`, `m3w`, `origin`, `upstream`): all answered `main` = `62ee38f008ef0a8077efb33b6e441a6879251ff3`; no `v2.5*` tag line returned. Parity holds for main only; there is no rc tag to compare.
- Release tool CON-412: constitution `scripts/forge/forge.sh` (constitution checkout `1dfbb4d19`; not initialised in the scratch clone, read in the main checkout). Registered at `constitution/docs/Issues.md:4810`; guide `constitution/docs/guides/multiforge_release_guide.md` (L28 `release-ensure`, L46-47 dry-run example); header usage `constitution/scripts/forge/forge.sh:8-14`; `--dry-run` = GETs only, prints what it would do (`forge.sh:29`, flag parse `:89`).
- Dry-run command per host (tokens by NAME from a 0600 file, never printed; the tag must exist on the forge and point at SHA, else `RESULT: UNVERIFIED-COMMIT` exit 5 in a dry-run too, guide L143, `forge.sh:23`): `constitution/scripts/forge/forge.sh release-ensure --remote <name> --tag <rc-tag> --sha <40-hex> --notes-file <file> --prerelease --dry-run`. Hosts: github, gitlab, codeberg, m3w, gitflic, gitverse. Exit codes: 2 USAGE, 3 GAP, 4 API, 5 UNVERIFIED-COMMIT (guide L31).
- Token validity per host and GitFlic/GitVerse write paths: UNCONFIRMED; write paths are fixture-proven only per the guide's own status line (PROVISIONAL).
- Push serially (sleep 4), fast-forward only, never force (checklist G10).

## Verdict

0 of 5 rc.1 gates satisfied. Missing: version bumps in 14+ product files and both versionCodes (G11), a `[2.5.0]` CHANGELOG section with VERIFIED/PROVISIONAL/REOPENED/OVERDUE (G8, then G6 can pass), a retest on the candidate (G1), an rc tag and forge dry-run plus pushes (G10).
