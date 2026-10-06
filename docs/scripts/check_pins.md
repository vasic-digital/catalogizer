# check_pins.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T00:50:00Z |
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
`check_pins: N violations in M files scanned`. Exit 0 clean, 1 violations, 2 usage, **3 internal error** (an uncaught exception: nothing is printed on stdout, a traceback and
`check_pins: internal error: ...` go to stderr). A caller that compares rows with a baseline must treat 3 as a failure of the check, never as zero rows (WF10 p1 F3).

## Rules

| Rule | Fires on |
|---|---|
| `compose_image_unpinned` | `image:` in a compose file (name contains `compose`, `.yml`/`.yaml`) without a full digest; the value may be on the next line; `${VAR:-default}` is judged by its default; a bare `${VAR}` or `$VAR`, `localhost/` images, and an UNQUALIFIED image (no registry host) of a service that also has a `build:` key (the tag the build produces, a local image) are not judged; a registry-qualified image next to `build:` is judged |
| `from_unpinned` | `FROM` of a Dockerfile/Containerfile with an external image without a full digest (build-stage aliases, `scratch`, `localhost/` skipped) |
| `from_arg_unpinned` | `FROM ${ARG}` / `${ARG:-default}` whose default exists and has no digest (an `ARG` without a default is the builder's input). Only `ARG` lines before the first `FROM` are global; a stage-local `ARG X` after a `FROM` neither hides nor replaces the global default; several `NAME=value` on one `ARG` line are all read |
| `copy_from_unpinned` | `COPY --from=` / `ADD --from=` / `RUN --mount=...,from=` naming an external image without a digest (a stage alias, a stage number, `localhost/` or a `$` value is not judged) |
| `script_image_unpinned` | the image operand of `docker\|podman\|nerdctl run\|pull\|create` (also `buildah from\|pull`), including a bare name (implicit `:latest`), judged in EVERY simple command of the line (`&&`, `;`, `\|`, `$(...)`, quoted `"$(...)"`, `sh -c '...'`, `ssh host "..."`, `eval`), after wrappers (`sudo [-u x]`, `env`, `timeout 30`, `nice -n`, `VAR=x`), global options (`podman --log-level error run`) and `image`/`container`; plus any registry-qualified reference with a tag in a `.sh`/`.bash` file (`docker://` transport prefix removed first). `localhost/` operands and `$VAR` operands are not judged. Option values are skipped by a complete table of the `run`/`create`/`pull` value options (`--add-host`, `--dns`, `--log-opt`, `--name`, ...); an unknown option is a boolean, except that a following `key=value`, port, address or `host:ip` token is its value |
| `pipe_to_shell` | `curl\|wget\|fetch ... \| [sudo] sh\|bash\|zsh\|dash\|ash\|ksh` (also by `/bin/` or `/usr/bin/` path) or `\| python3\|perl\|ruby -` (stdin), `bash <(curl ...)`, `source <(curl ...)`, `. <(curl ...)`, `sh -c "$(curl ...)"`, `eval "$(curl ...)"` in a Dockerfile/Containerfile or script; a pipe continued onto the next line by a trailing `\|` (no backslash) or by a backslash, and the multi-line `for ... done \| bash -` form, are one logical line reported at its first physical line; a quoted install hint a script prints is flagged too. `\| python3 -m json.tool`, `\| python3 -c`, `\| sha256sum`, `\| ssh` are not shell installs |

Carriers do not fire (11.4.201): `#` comments (full-line and trailing) are removed first; Markdown, text and every other file type are not scanned.
In a script under a `tests` directory or named `test_*`/`mutate_*` the here-document bodies are fixture data and are not judged (a `<<` inside a quoted string or an arithmetic `$((a<<b))` does not open one), the simple commands that only write or filter data (`printf`, `echo`, `sed`, `tee`, `grep`, `awk`, a bare `VAR=value`) are masked for the pipe and run rules, and a registry-qualified literal outside a real engine command is not judged; a real `docker|podman run|pull|create` in a test script is still judged. In any other script they are judged like code. A UTF-8 byte-order mark before the first line is ignored.

## Verification

`scripts/containers/tests/test_check_pins.sh` writes its fixtures to a temporary directory at run time (golden-bad per rule, golden-good, carriers,
a negative control file holding a carrier and a real violation, determinism, usage errors; WF10 p1 added the false-positive carriers, the false-negative inputs, the crash exit code and a git submodule fixture for `--recurse`) and then re-runs the fixture body against 73 mutants
of the script (`check_pins_mutations.tsv`: the 21 original rows that still apply (22 were re-pointed or re-listed; 2 moved anchors re-listed as `dynamic-digest-skip-removed` and `value-option-skip-removed`), the independent reviewer's 12 survivors R1-R12, and N-rows for the code added by the fix; each replaces one exact text, which must exist exactly once, a literal backslash is written doubled). Evidence: `specs/001-full-project-audit-remediation/evidence/wp11/t104-*` (first version) and `wf10fix-p1-check-pins-*` (this fix).

## Honest limits

- The scan reads text. It does not resolve a registry, does not expand variables other than a `${VAR:-default}` default, and does not prove that a
  digest exists or is signed (signature verification is T149's `IMG-SIGVERIFY`).
- Script image detection is a heuristic: the operand after the option list of `docker|podman run|pull|create`, and registry-qualified references with a
  tag. An image named only through a variable or a config file is not seen. A `FROM` line inside a script's here-document is not a `FROM` of a Dockerfile.
- Not resolved by the scan (UNCONFIRMED whether they occur): a script without a `.sh`/`.bash` extension, a `Makefile` command, an image named only through a variable or inside `${IMG:-default}` of a `run` operand, a value option unknown to the table whose value looks like an image.
- The mutation `strip-line-comment` (skipping the full-line comment branch of the line joiner) is equivalent: `strip_comment` removes the same lines, so
  it is not in the table (UNCONFIRMED that no further equivalent mutants exist).
- Baseline: the main repository measure (91 rows at HEAD 7f5e9f4b plus uncommitted work) is `evidence/wp11/pins-baseline.txt`; the recursive measure over
  every submodule checkout is `evidence/wp11/pins-baseline-recursive.txt` and includes third-party submodules (the own-organisation filter is the
  CPA's, T040 rule W, UNCONFIRMED). The plan review's 45 compose lines plus 6 deploy lines were not reconciled row by row with the 50 rows measured here.
- Not done by this change (other agents' areas, owed): the registration row in `scripts/repo/validate_checks.tsv`, the line in
  `scripts/repo/cpa_code.txt`, the ratchet baseline `scripts/repo/validate_baselines/check_pins.tsv` and the review verdict `$EV/reviews/WP-11.json` (T105, T117).
