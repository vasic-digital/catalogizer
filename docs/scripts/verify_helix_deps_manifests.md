# verify_helix_deps_manifests.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-21 |
| Last modified | 2026-10-21T00:00:00Z |
| Status | tracked; independent review owed (constitution 11.4.142) |
| Source | `scripts/verify_helix_deps_manifests.sh` (thin wrapper) |
| Canonical engine | `constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh` (+ `helix_deps_manifest.py`) |
| Test | `scripts/tests/test_verify_helix_deps_manifests.sh` |

## Purpose

Enforces constitution 11.4.31: every own-org (`vasic-digital`, `HelixDevelopment`) repository carries a well-formed `helix-deps.yaml` at its root, and the root project's own manifest ([`helix-deps.yaml`](../../helix-deps.yaml)) exists.

The verification logic is NOT implemented in this repository any more (single source of truth, 11.4.28(C) / 11.4.177). `scripts/verify_helix_deps_manifests.sh` is a wrapper that resolves the project root from its own location, locates the canonical engine at `constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh` and runs `exec bash <canonical> --root "<project root>" "$@"`. Schema rules, walk, verdict semantics and the full option list (`--own-orgs`, `--validate-file`, `--scaffold`, `--selftest`, `--help`) are those of the canonical engine; see its header and `constitution/docs/manuals/dependency_manifest_manual.md`.

## Usage

```bash
scripts/verify_helix_deps_manifests.sh                       # walk this project
scripts/verify_helix_deps_manifests.sh --own-orgs a,b        # extra args are forwarded verbatim
scripts/verify_helix_deps_manifests.sh --validate-file path/to/helix-deps.yaml
```

## Delegation and failure behaviour

| Situation | Behaviour |
|---|---|
| canonical engine present | `exec` into it; its stdout/stderr and exit code are the wrapper's |
| canonical engine missing (constitution not checked out, or too old) | message on stderr naming the expected path and the `git submodule update --init constitution` fix (never `--recursive` through `constitution/`), exit 2 (BLIND); never 0 |

## Output

One line per repository (`STATUS<TAB>path<TAB>role<TAB>detail`), then `VERDICT: PASS|FAIL|BLIND (pass=.. fail=.. blind=.. skip_third_party=.. known=..)`. Parse the VERDICT line. `SKIP(third_party)` is never counted as a pass.

## Exit codes (canonical, passed through unchanged)

| Code | Meaning |
|---|---|
| 0 | PASS: every own-org repository passes |
| 1 | FAIL: at least one own-org repository fails (BLIND lines are still printed) |
| 2 | BLIND: nothing failed but something could not be verified, the instrument failed its own control needles, or (wrapper) the canonical engine is missing |
| 64 | usage error |

The previous in-repo engine used 0/1/2/3/4 (2 usage, 3 BLIND, 4 needle failure); callers that tested for 3 or 4 must now test for 2.

## Test

`scripts/tests/test_verify_helix_deps_manifests.sh` copies the real wrapper into a throw-away project tree whose `constitution/` holds a stub canonical engine that records its argv and exits with a chosen code. It asserts: delegation, `--root <project root>` first, extra arguments forwarded, exit codes 0/1/2/64 passed through, missing canon gives exit 2 with an actionable message naming the path. It then runs four mutants of the wrapper (exit code swallowed, `--root` dropped, arguments dropped, missing canon exits 0); the suite must fail on each (15 checks in total). `LIVE=1` additionally runs the wrapper against the real constitution engine and requires a `VERDICT:` line. The engine's own behaviour is tested in the constitution (`tests/test_verify_helix_deps_manifests.sh` there).
