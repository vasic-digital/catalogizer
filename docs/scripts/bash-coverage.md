# bash-coverage.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:57:44Z |
| Status | new in the working tree (T199), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed; review round 1 (WF11, 2026-10-06 UTC): fixes I3, I4, i1, I5 (harness side) applied; independent re-review owed (constitution 11.4.142) |
| Source | `scripts/bash-coverage.sh`, `scripts/coverage/bashcov.py`; tests `scripts/coverage/tests/test_bash_coverage.sh` and `scripts/coverage/tests/fixtures/` |

## Purpose
The bash line-coverage harness of docs/05 7.1 (the instrument for A9 and A13, constitution 11.4.224 E). Every bash process the command starts is made to trace itself through `BASH_ENV`: `PS4='+COV:${BASH_SOURCE}:${LINENO}:'` (the full path since round 1 of the WF11 review), `set -x`, the trace sent to its own descriptor (`BASH_XTRACEFD`) so a script's `exec 2>/dev/null` cannot blind it. Executed lines of the targets over executable lines is the figure.

## Usage
`bash-coverage.sh --src-root DIR --target FILE... [--exclusions FILE] [--out DIR] -- <command>...`. Run through `scripts/test-in-container.sh build-scripts unit -- bash scripts/bash-coverage.sh ...` (IMG-KCOV, local interpreter lane). Output: `bash-coverage.json` (`bash-coverage/1`: per target executable, executed, percent, uncovered lines, uncovered functions, `traced_non_executable`), `trace.log`.

## Exit statuses
The command's own status when it fails (record `command_failed`); 0 otherwise; 2 usage; 3 refusal: `target_missing`, `duplicate_target` (two `--target` arguments name one file), `exclusions_gate_failed` (the T200 gate runs first), `trace_empty` (no traced line at all: the instrument was blind, never a 0 percent).

## Executable lines
A documented heuristic (`bashcov.py` docstring): blank lines, comments, here-document bodies, continuation lines, structural keywords (`fi`, `done`, `esac`, `else`, `then`, `do`, braces, `;;`), function definition lines and bare case labels are not counted. `traced_non_executable` makes a classifier error visible.

## Stated limits (written into every record)
Line, not branch (an if/else line counts when either side ran: the founding measured case of 11.4.224 C); `set +x` regions, traps and `sh` (dash) children are not traced; a percentage is a minimum on a proxy, necessary and never sufficient. kcov (in IMG-KCOV) is the cross-check; the test compares which lines of a fixture both instruments report never executed, and SKIPs by name where kcov is absent.

## Review round 1 (WF11, 2026-10-06 UTC)
- **I3 (attribution)**: the PS4 is now `+COV:${BASH_SOURCE}:${LINENO}:` (the FULL path). A traced line is credited to a target by PATH: the same file (absolute path, or a relative path that resolves from the harness's start directory to the target's realpath), or a COPY elsewhere that keeps the target's relative layout (`.../Build/lib/common.sh`, the way `tests/test_build_system.sh` copies `Build/`). A root-level target is credited only when direct. A same-basename file that is not the target earns nothing and is NAMED in `attribution.foreign_same_basename` (an undercount that is visible, never an overcount). Two `--target` arguments naming one file are refused (`duplicate_target`); two targets that only share a base name are legal.
- **I4**: `--out`, `--exclusions` and `--src-root` are made absolute before anything uses them: a relative `--out` plus a child that `cd`s used to lose every later trace (70.00 percent measured as 0.00 percent, exit 0).
- **i1**: a continuation of a pipeline or an and/or list (`cmd | \` followed by `next`) is executable: the tracer reports each element's own line. A multi-line array assignment (`NAME=(` ... `)`) counts ONE line, its closing line (bash traces it once there; Build/lib/hash.sh 31-42 was counted as 12). The tracer's line for a command that spans lines depends on the bash version (measured: 5.3.9 on the host reports the FIRST line of `echo \` + `"arg"`, 5.2.15 in IMG-KCOV the LAST), so the classifier returns an alias map and the report folds a traced continuation or array line onto the command's counted line (proven on synthetic traces of both styles). `status` is `classifier_disagreement` when a traced line is still classed non-executable.
- **I5**: the fence gate runs with `--root` (the src root), so the class claim of each entry is checked against the files it names.

## Review round 5 (WP-23 fix round, 2026-10-07 UTC): a structural round on the classifier and the harness
The round-4 findings kept recurring in the same classes (what counts as an executable line; what the harness lets a late or foreign writer do), so this round replaced the point fixes with a tokenizer and a run contract. Convergence assessment: `specs/001-full-project-audit-remediation/evidence/wp23/fix-r5-convergence-assessment.md`.
- **Classifier (`bashcov.py`)**: a real bash tokenizer (quotes, `$()`, `$(())`, backticks, `${}`, `[[ ]]`, arrays `=(`, here-documents, process substitution, `case` patterns inside `$()`). Every multi-line command is ONE group: the first line is the counted line and the others alias to it, so a trace at any line of the group credits it once. A traced line that is still classed non-executable gives status `classifier_disagreement`, `totals.percent` null and exit 3 (never a figure built on a known classifier error). Trap records at line 1 of a sourced file are ignored unless the command word is on line 1.
- **Trace record**: `+COV US len US path US LINENO US pid US PWD US sha256 US command` (US = 0x1f, the path is length-prefixed so a path holding `:` or a newline cannot forge a field). A copy of a target is credited only when its sha256 taken IN the run equals the target's sha256 before the run (`--pre-sha`); a modified copy earns nothing.
- **Run contract**: the command runs in its own process group; INT/TERM are forwarded to that group and the run is recorded `interrupted` (130/143) with no figure; the run-scoped gate file that switches tracing on is removed when the command returns; survivors of the group are signalled and reaped and the run is refused `trace_writers_outlived_command`. `--out` must be new or empty (`out_not_empty`) and must not contain `$`, a backtick or a backslash (`out_unsafe_path`: bash expands `BASH_ENV`). Targets are hashed before and after (`target_changed_during_run`). The fence is snapshotted into the run directory, gated and applied as that one file; its sha256 is recorded.
- **Default `--out`**: `/out/bash-coverage` when `/out` is writable, otherwise `$PWD/.audit/out/bash-coverage` (in the harness lane `/src` is read-only).
- **New options**: `--tools FILE` (default `coverage/exclusions/tools.yaml`), `--items-file FILE` (register export for first-party tracked items).
- **Tests**: `test_bash_coverage.sh` is rewritten around the real harness and eight new fixtures (`cov_multiline.sh`, `cov_arrays.sh`, `cov_arith.sh`, `cov_hdoc2.sh`, `cov_procsub.sh`, `lib_trap.sh`, `cov_trap.sh`, `cov_case.sh`); mutations are tagged `# MUT:` in `bashcov.py` and adopted from the round-4 reviewer set.
