# scripts/release

Release-preparation gates (roadmap item REL-2.4.1-GATE).

## check_release_consistency.sh

```
scripts/release/check_release_consistency.sh [--repo DIR] [--version X.Y.Z] [--prev-tag TAG]
                                             [--section X.Y.Z] [--check version|changelog|all]
scripts/release/check_release_consistency.sh --selftest
```

Exit codes: `0` all OK, `1` any FAIL, `2` usage error or BLIND (the instrument could
not evaluate: section not found, no VERIFIED subsection, zero paths extracted,
changelog missing, control needle failed). BLIND is never a pass. If a FAIL exists the
exit code is 1 even when something else was also BLIND. The last output line is always
machine-readable: `VERDICT: PASS|FAIL|BLIND <counters>`.

### Check 1: version consistency

Expected version = `--version`, else `versions.json` `global` major.minor.patch.
Every product-version field must equal it:

| File | Field |
|---|---|
| `versions.json` | `global` major.minor.patch |
| `docs/api/openapi.yaml` | `info.version` (indent-2 key directly under top-level `info:`) |
| `catalog-web`, `catalogizer-desktop`, `installer-wizard` `package.json` | `version` |
| their `package-lock.json` | top `version` and `packages[""].version` |
| `catalogizer-desktop/src-tauri`, `installer-wizard/src-tauri` `tauri.conf.json` | `version` |
| their `Cargo.toml` | `[package] version` |
| `catalogizer-android/app`, `catalogizer-androidtv/app` `build.gradle.kts` | `versionName` |

Parsing uses `python3 -I` with `json`/`tomllib`; dependency versions, other nested
`version` keys and history are never matched. `catalogizer-api-client` is a library with
its own version and is intentionally out of scope. `versionCode` of both Android apps is
printed and must be an integer; with `--prev-tag TAG` it must be strictly greater than
the value at that tag (a file absent at the tag counts as new).

### Check 2: changelog evidence paths

Section = `--section` (e.g. `2.4.0`), default the topmost numbered `## [x.y.z...]`
heading of `docs/CHANGELOG.md` (`[Unreleased]` is skipped). In its `### VERIFIED`
subsection (until the next heading) every backticked token that looks like a repo path
(contains `/`, no spaces/globs/URLs, first directory is a tracked top-level directory or
one of the known names `docs scripts challenges tests tools specs catalog* installer-wizard ...`;
a trailing `:12` or `:12-20` is stripped) must exist in the working tree and be
git-tracked; each failure is a `MISS`. Tokens that have a `/` but a first segment that is
not a repo directory are listed as `skipped` (counted, not failed).

For every extracted path, the nearest `SHA256SUMS` at or above it (never the repo root)
is verified with `sha256sum -c`: a mismatch or a missing non-ignored file is `SHAFAIL`;
a listed file that is gitignored (for example PNG screenshots) and absent is
`UNVERIFIABLE-FROM-CLONE`, counted in `unverifiable=` and not a failure.

### Instrument verification (constitution 11.4.273, 11.4.224)

* `--selftest` builds a fixture repo, requires PASS on the good fixture and FAIL on a
  planted wrong version and on a planted fake path.
* The changelog check runs a control needle first (a known tracked path must be seen, a
  fabricated path must not) and goes BLIND otherwise.
* `test_check_release_consistency.sh` (written first, shown RED against a stub that
  always printed PASS): golden-good, one stale-version mutation per file type, versionCode
  cases, evidence-path cases, SHA256SUMS cases, usage cases and negative controls
  (dependency versions, nested version keys, other sections must not matter).
  `CHECKER=<path>` runs the suite against another copy.
* `mutation_check_release_consistency.sh` applies one mutation at a time to a copy of the
  checker and requires the suite to fail (mutant killed); it exits non-zero if any
  survives.
* `release_fixture.sh` is the shared fixture builder (fixture input data only).

Temporary files go under `$RELCHK_TMP` (default `~/.cache/relchk-tmp`).

### Limitations

* Component list and file locations are hard-coded to the current layout.
* Only `VERIFIED`-subsection lines are inspected; paths without a backtick are not found.
* Existence and tracking prove a path is present, not that its content supports the claim.
* The control needle and the selftest verdict logic are not themselves mutation-tested.
