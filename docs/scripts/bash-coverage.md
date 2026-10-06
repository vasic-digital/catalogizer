# bash-coverage.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | new in the working tree (T199), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/bash-coverage.sh`, `scripts/coverage/bashcov.py`; tests `scripts/coverage/tests/test_bash_coverage.sh` and `scripts/coverage/tests/fixtures/` |

## Purpose
The bash line-coverage harness of docs/05 7.1 (the instrument for A9 and A13, constitution 11.4.224 E). Every bash process the command starts is made to trace itself through `BASH_ENV`: `PS4='+COV:${BASH_SOURCE##*/}:${LINENO}:'`, `set -x`, the trace sent to its own descriptor (`BASH_XTRACEFD`) so a script's `exec 2>/dev/null` cannot blind it. Executed lines of the targets over executable lines is the figure.

## Usage
`bash-coverage.sh --src-root DIR --target FILE... [--exclusions FILE] [--out DIR] -- <command>...`. Run through `scripts/test-in-container.sh build-scripts unit -- bash scripts/bash-coverage.sh ...` (IMG-KCOV, local interpreter lane). Output: `bash-coverage.json` (`bash-coverage/1`: per target executable, executed, percent, uncovered lines, uncovered functions, `traced_non_executable`), `trace.log`.

## Exit statuses
The command's own status when it fails (record `command_failed`); 0 otherwise; 2 usage; 3 refusal: `target_missing`, `basename_collision` (the PS4 names the base name only), `exclusions_gate_failed` (the T200 gate runs first), `trace_empty` (no traced line at all: the instrument was blind, never a 0 percent).

## Executable lines
A documented heuristic (`bashcov.py` docstring): blank lines, comments, here-document bodies, continuation lines, structural keywords (`fi`, `done`, `esac`, `else`, `then`, `do`, braces, `;;`), function definition lines and bare case labels are not counted. `traced_non_executable` makes a classifier error visible.

## Stated limits (written into every record)
Line, not branch (an if/else line counts when either side ran: the founding measured case of 11.4.224 C); `set +x` regions, traps and `sh` (dash) children are not traced; a percentage is a minimum on a proxy, necessary and never sufficient. kcov (in IMG-KCOV) is the cross-check; the test compares which lines of a fixture both instruments report never executed, and SKIPs by name where kcov is absent.
