# scripts/release

Release-preparation gates (roadmap item REL-2.4.1-GATE).

## check_release_consistency.sh

```
scripts/release/check_release_consistency.sh [--repo DIR] [--version X.Y.Z] [--prev-tag TAG]
                                             [--section X.Y.Z] [--check version|changelog|all]
                                             [--waivers FILE] [--evidence-prefix PREFIX]...
scripts/release/check_release_consistency.sh --selftest
```

Exit codes: `0` all OK, `1` any FAIL, `2` usage error or BLIND (the instrument could
not evaluate: section not found, no VERIFIED subsection, zero paths extracted,
changelog missing, control needle failed, a control character in a VERIFIED path or a nested
evidence directory name, a waiver file that differs from HEAD). BLIND is never a pass. `PASS-WITH-WAIVERS`
exits `0` like `PASS` but means the result relies on at least one declared, unexpired evidence gap
(`waived>0`); scripts that must not accept waivers test the VERDICT word, not only the exit code. If a FAIL exists the
exit code is 1 even when something else was also BLIND. The last output line is always
machine-readable: `VERDICT: PASS|PASS-WITH-WAIVERS|FAIL|BLIND <counters>` (`-h` prints the usage and `VERDICT: HELP`).
A version helper that crashes (non-zero exit), is silent, or yields fewer field rows than it
planned is BLIND, never PASS. `--version ""` is a usage error.

### Check 1: version consistency

Expected version = `--version`, else `versions.json` `global` major.minor.patch.
Every product-version field must equal it:

| File | Field |
|---|---|
| `versions.json` | `global` major.minor.patch |
| `docs/api/openapi.yaml` | `info.version` (indent-2 key directly under top-level `info:`; exactly one must exist) |
| `catalog-web`, `catalogizer-desktop`, `installer-wizard` `package.json` | `version` |
| their `package-lock.json` | top `version` and `packages[""].version` |
| `catalogizer-desktop/src-tauri`, `installer-wizard/src-tauri` `tauri.conf.json` | `version` |
| their `Cargo.toml` | `[package] version` |
| `catalogizer-android/app`, `catalogizer-androidtv/app` `build.gradle.kts` | `versionName` |

If both `build.gradle.kts` and `build.gradle` exist for an app that is a FAIL (ambiguous).
Parsing uses `python3 -I` with `json`/`tomllib`; dependency versions, other nested
`version` keys and history are never matched. `catalogizer-api-client` is a library with
its own version and is intentionally out of scope. `versionCode` of both Android apps is
printed and must be an integer; with `--prev-tag TAG` it must be strictly greater than
the value at that tag (a file absent at the tag counts as new).

### Check 2: changelog evidence paths

Section = `--section` (e.g. `2.4.0`), default the topmost numbered `## [x.y.z...]`
heading of `docs/CHANGELOG.md` (`[Unreleased]` is skipped). In its `### VERIFIED`
subsections (every `### VERIFIED...` heading of the section is read, each until the next heading) every backticked token that looks like a repo path
(contains `/`, no spaces/globs/URLs, first directory is a tracked top-level directory or
one of the known names `docs scripts challenges tests tools specs catalog* installer-wizard ...`;
a trailing `:12` or `:12-20` is stripped) must exist in the working tree and be
git-tracked; each failure is a `MISS`. Every backticked token that contains a `/` but is not accepted as a repo path (spaces, globs,
URLs, `~`, absolute, or a first segment that is not a repo directory) is listed and counted
as `skipped` (never failed). Tokens without a `/` are not counted.

For every extracted path, the nearest git-tracked manifest (`SHA256SUMS`, `SHA256SUMS.txt` or `MANIFEST*.sha256`) at or above it
(never the repo root) is verified with `sha256sum --strict -c`: malformed manifest lines are a `SHAFAIL`; a manifest
entry with an absolute path or a `..` segment is a `SHAFAIL` and that manifest is not run; a mismatch or a missing non-ignored file is `SHAFAIL`;
a listed file that is gitignored (for example PNG screenshots) and absent is
`UNVERIFIABLE-FROM-CLONE`, counted in `unverifiable=` and not a failure.

#### Evidence must sit under a complete manifest (RC-3, RC-3b)

* **RC-3.** A VERIFIED path under an evidence prefix (`--evidence-prefix`, repeatable, default
  `docs/qa/`) with no manifest at or above it is `NOMANIFEST` and the verdict is FAIL
  (`no_manifest=`): an integrity claim with nothing to check it against. Paths outside the
  evidence prefixes (for example `scripts/tool.sh`) are the allowed non-evidence tokens and need
  no manifest. Giving `--evidence-prefix` replaces the default. The decision is made on the real
  in-repo location: `//` and `/./` segments are collapsed and symlinks resolved first, so
  `docs/./qa/x`, `docs//qa/x` and a symlink alias into `docs/qa/` are still evidence, and the
  prefix directory itself (`docs/qa/`) is evidence too. A manifest that is untracked or
  gitignored does not exist for a clone and does not count.
* **RC-3b.** Every manifest in use must list every regular file of the tree it covers
  (tracked files plus untracked, non-gitignored files; symlinks and gitignored files are not
  required; the manifest files `SHA256SUMS`, `SHA256SUMS.txt` and `MANIFEST*.sha256` themselves are excluded). A
  file the manifest does not list is `UNLISTED` and FAILs (`unlisted=`), so deleting an entry
  and tampering with the file no longer passes. Files in a subdirectory that has its own
  manifest are the responsibility of that nested manifest, which is queued and fully checked
  (syntax, checksums, completeness) too.

#### Declaring known evidence gaps (waivers)

A known-bad or missing manifest is never silently ignored. A tracked file
`docs/qa/EVIDENCE_WAIVERS.txt` (or `--waivers FILE`, repo-relative) declares the gap, one
line each, `#` comments allowed:

```
no-manifest|<evidence path (resolved, repo-relative, no trailing slash)>|<item>|<expires>|<reason>
unlisted|<manifest dir>|<file relative to it>|<item>|<expires>|<reason>
bad-entry|<manifest dir>|<manifest entry>|<item>|<expires>|<reason>     # a manifest entry whose file is missing
```

Fields are trimmed. Following the waiver schema of constitution 11.4.271 every waiver carries a
tracked item id (`<ALPHA>-<ID>`, for example `CAT-REL-CHECK-1`: who owns the debt), an expiry date
`YYYY-MM-DD` (valid through that day, `RELCHK_TODAY` overrides today for tests) and a non-blank
reason. A missing or malformed item or date, an expired waiver, a blank reason, an unknown kind, an
empty path field or a duplicate is `BADWAIVER` and FAILs (`bad_waivers=`); the gap it covered then
FAILs as well. The waiver file must be git-tracked and identical to HEAD, otherwise the run is BLIND
(an uncommitted waiver is not a reviewed waiver). An explicit `--waivers` file that is absent is BLIND.

Scope: a waiver is path-scoped, not section-scoped. It applies wherever the named path is cited in
the checked changelog section, and only to that exact path/file/entry, never to a directory subtree.
Each applied waiver is printed as `WAIVED <what>: <reason> [item X, expires D]` and counted in
`waived=`; any applied waiver turns `PASS` into `PASS-WITH-WAIVERS`. A waiver that matches nothing in
the checked section is printed as `UNUSED-WAIVER <key>` and counted in `unused_waivers=`; it does not
fail and does not change the verdict word (it may belong to another changelog section).

A `bad-entry` waiver covers only a manifest entry whose file is missing (for example listed but never
committed). A checksum mismatch is tampering or corruption and is never waivable: it is always
`SHAFAIL`, and a `bad-entry` waiver naming it is refused as `BADWAIVER` (counted in `bad_waivers=`).
The only cure for a mismatch is to regenerate the evidence and its manifest. Example (real tree):
the cover-art run `docs/qa/coverart/results/final/` has a `summary.txt` whose checksum no longer
matches (finding E-1); it stays a `SHAFAIL` until that evidence is regenerated.

Committed state only: a clone sees HEAD, so the run goes BLIND (exit 2) when `docs/CHANGELOG.md`, any
version file that was read, any existing cited path, or any tracked file under a manifest tree is not in
HEAD or differs from it in the index or working tree (staged-but-uncommitted counts). Untracked files are
not part of this check (they are judged by the tracked/UNLISTED rules). Commit first, then run the check.

### Instrument verification (constitution 11.4.273, 11.4.224)

* `--selftest` builds a fixture repo, requires PASS on the good fixture and FAIL on a
  planted wrong version and on a planted fake path.
* The changelog check runs a control needle first (a plain-named tracked path must be seen by
  git, the changelog must exist, a fabricated path must not be seen) and goes BLIND otherwise.
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
* The selftest verdict logic is not mutation-tested. The `--strict` flag and the
  'improperly formatted' text check are redundant layers; the mutant removes both together.
* Helper-output integrity (N1): bash pins the expected 19 version rows by label (exactly one
  PLAN line equal to 19, every expected label exactly once, no foreign labels, INFO only on
  versionCode rows) and requires the changelog helper to end with a matching END trailer
  (one SECTION, path/skip counts equal). A helper that is edited to emit a complete,
  correct-looking but wrong row set (a forged OK with the right label) is still not
  detected: bash cannot re-derive the field values.
* SHA256SUMS: GNU and BSD-tag (`SHA256 (name) = hash`) lines both get the directory-escape
  check. Any non-blank line that is not one of those two unescaped forms (including
  backslash-escaped GNU or BSD lines and comment lines) is a `SHAFAIL ... unsupported
  manifest line`: the checker cannot judge it, and some `sha256sum` implementations
  honour the escape. Each entry is also resolved with `readlink -m`; an entry (for example
  an in-directory symlink) that resolves outside the repository is a `SHAFAIL`. A CR anywhere
  but the single trailing position of a line (for example `SHA256 (a\r) = H`) is refused.
* A fenced code block (``` or ~~~) in the CHANGELOG never supplies a section or
  subsection heading. `versionCode = 7;` and `versionCode = 7 /* c */` parse as literals.
* Under `--prev-tag`, an `INFO` row for a versionCode is BLIND unless the `--prev-tag`
  row itself FAILed (unknown tag).
* A tracked path that resolves (symlink) outside the repository is a `MISS`.
* `versionCode`/`versionName` must be a bare literal (optional trailing `//` comment);
  `versionNameSuffix` is not mistaken for a second `versionName`. Expressions such as
  `7 - 2` are a FAIL. Any number of consecutive `/* */` block comments (and a final `//`
  comment) may follow the literal (RC2-1).
* A duplicated `## [x.y.z]` heading for the selected section is BLIND.
* The selected version-code monotonicity is still checked only against `--prev-tag`.
