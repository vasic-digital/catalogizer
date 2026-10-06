# check_pins.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T17:40:00Z |
| Status | new, untracked at writing (tasks T104 and T105); independent review owed (constitution 11.4.142); not yet registered in `scripts/repo/validate_checks.tsv`, `scripts/repo/cpa_code.txt` or the CPA baseline (owed, see Honest limits) |
| Source | `scripts/containers/check_pins.sh`; tests `scripts/containers/tests/test_check_pins.sh` with the mutation table `scripts/containers/tests/check_pins_mutations.tsv` |

## Purpose

Fails when a compose file, a Dockerfile or Containerfile, or a shell script references an external image without a full
`@sha256:<64 lowercase hex>` digest, or installs software by piping a download into a shell. docs/16 section 7.1 step 3 and findings D-05
and D-07 name it; docs/20 W20-01 and W20-02 need it to reject mutable tags and `curl | sh`.

## Usage

```bash
scripts/containers/check_pins.sh [--root DIR] [--list] [--recurse] [--exclude-dir NAME]... [PATH...]
```

| Option | Meaning |
|---|---|
| `--root DIR` | root the reported paths are relative to (default: git top level of the cwd, else the cwd) |
| `PATH...` | files or directories under the root to scan; default every tracked file (`git ls-files`) or, outside git, every file |
| `--recurse` | with the default file set, also the files of every submodule checkout |
| `--list` | one TSV row per violation, `rule`, `path`, `line`, `reference`, no summary (the baseline form) |
| `--exclude-dir NAME` | skip directories of this name (`.git`, `node_modules`, `vendor`, `.audit` are always skipped) |

Output: `VIOLATION <rule> <path>:<line>: <reference or excerpt>` per finding, sorted by path, line, rule, then
`check_pins: N violations in M files scanned`. Exit 0 clean, 1 violations, 2 usage.

## Rules

| Rule | Fires on |
|---|---|
| `compose_image_unpinned` | `image:` in a compose file (name contains `compose`, `.yml`/`.yaml`) without a full digest; `${VAR:-default}` is judged by its default; a bare `${VAR}` and `localhost/` images are not judged |
| `from_unpinned` | `FROM` of a Dockerfile/Containerfile with an external image without a full digest (build-stage aliases, `scratch`, `localhost/` skipped) |
| `from_arg_unpinned` | `FROM ${ARG}` whose `ARG` default exists and has no digest (an `ARG` without a default is the builder's input) |
| `copy_from_unpinned` | `COPY --from=<external image>` without a digest (a stage alias or number is not an image) |
| `script_image_unpinned` | the image operand of `docker\|podman\|nerdctl run\|pull\|create`, or any registry-qualified reference with a tag, in a `.sh`/`.bash` file |
| `pipe_to_shell` | `curl\|wget\|fetch ... \| [sudo] sh\|bash`, `bash <(curl ...)`, `sh -c "$(curl ...)"` in a Dockerfile/Containerfile or script; the multi-line `for ... done \| bash -` form is one logical line, reported at its first physical line; a quoted install hint a script prints is flagged too |

Carriers do not fire (11.4.201): `#` comments (full-line and trailing) are removed first; Markdown, text and every other file type are not scanned.
In a script under a `tests` directory or named `test_*`/`mutate_*` the here-document bodies are fixture data and are not judged; in any other
script they are judged like code.

## Verification

`scripts/containers/tests/test_check_pins.sh` writes its fixtures to a temporary directory at run time (golden-bad per rule, golden-good, carriers,
a negative control file holding a carrier and a real violation, determinism, usage errors) and then re-runs the fixture body against 23 mutants
of the script (`check_pins_mutations.tsv`; each replaces one exact text, which must exist exactly once). Evidence: `specs/001-full-project-audit-remediation/evidence/wp11/t104-*`.

## Honest limits

- The scan reads text. It does not resolve a registry, does not expand variables other than a `${VAR:-default}` default, and does not prove that a
  digest exists or is signed (signature verification is T149's `IMG-SIGVERIFY`).
- Script image detection is a heuristic: the operand after the option list of `docker|podman run|pull|create`, and registry-qualified references with a
  tag. An image named only through a variable or a config file is not seen. A `FROM` line inside a script's here-document is not a `FROM` of a Dockerfile.
- The mutation `strip-line-comment` (skipping the full-line comment branch of the line joiner) is equivalent: `strip_comment` removes the same lines, so
  it is not in the table (UNCONFIRMED that no further equivalent mutants exist).
- Baseline: the main repository measure (91 rows at HEAD 7f5e9f4b plus uncommitted work) is `evidence/wp11/pins-baseline.txt`; the recursive measure over
  every submodule checkout is `evidence/wp11/pins-baseline-recursive.txt` and includes third-party submodules (the own-organisation filter is the
  CPA's, T040 rule W, UNCONFIRMED). The plan review's 45 compose lines plus 6 deploy lines were not reconciled row by row with the 50 rows measured here.
- Not done by this change (other agents' areas, owed): the registration row in `scripts/repo/validate_checks.tsv`, the line in
  `scripts/repo/cpa_code.txt`, the ratchet baseline `scripts/repo/validate_baselines/check_pins.tsv` and the review verdict `$EV/reviews/WP-11.json` (T105, T117).
