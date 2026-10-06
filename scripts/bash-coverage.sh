#!/usr/bin/env bash
# bash-coverage.sh - T199. Bash line-coverage harness (docs/05 7.1, constitution 11.4.224 E): a PS4 line trace. Every bash process the command starts (children
# included) is made to trace itself through BASH_ENV: `PS4='+COV:${BASH_SOURCE##*/}:${LINENO}:'` and `set -x` with the trace sent to its own file descriptor
# (BASH_XTRACEFD), so a script's own `exec 2>/dev/null` cannot blind it. The executed lines of the targets divided by their executable lines is the figure.
# Usage: bash-coverage.sh --src-root DIR --target FILE [--target FILE ...] [--exclusions FILE] [--out DIR] -- <command word>...
#   --src-root DIR     the tree the targets are relative to
#   --target FILE      a script to measure, relative to --src-root (repeatable); two targets with one base name are refused (the PS4 carries the base name only)
#   --exclusions FILE  a coverage-exclusions/1 fence (coverage/exclusions/<app>.yaml): it must pass scripts/coverage/check_exclusions.sh FIRST, then the files it
#                      lists are left out of the figure and named in the record with their class
#   --out DIR          receives bash-coverage.json (schema bash-coverage/1) and trace.log
# Exit: the command's own exit status when it fails (the record says command_failed, a failing suite is never swallowed); 0 otherwise; 2 usage; 3 refusal
#   (reason on stderr: target_missing, basename_collision, exclusions_gate_failed, trace_empty - the trace saw no line at all, so the instrument was blind and
#   a 0 percent would be a lie, 11.4.201). HONEST LIMITS written into every record: line, not branch; `set +x` regions, traps and `sh` (dash) children are
#   not traced; the executable-line rule is a documented heuristic (scripts/coverage/bashcov.py). Contract: docs/scripts/bash-coverage.md.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE="$HERE/coverage/check_exclusions.sh"
SRC=""; EXCL=""; OUT=""; TARGETS=()
refuse() { echo "bash-coverage: REFUSED reason=$1 ${2:-}" >&2; exit 3; }
usage() { echo "bash-coverage: usage: $1" >&2; echo "bash-coverage: bash-coverage.sh --src-root DIR --target FILE... [--exclusions FILE] [--out DIR] -- <command>..." >&2; exit 2; }
while [ $# -gt 0 ]; do case "$1" in
  --) shift; break;;
  --src-root) [ $# -ge 2 ] || usage "--src-root needs a value"; SRC="$2"; shift 2;;
  --target) [ $# -ge 2 ] || usage "--target needs a value"; TARGETS+=("$2"); shift 2;;
  --exclusions) [ $# -ge 2 ] || usage "--exclusions needs a value"; EXCL="$2"; shift 2;;
  --out) [ $# -ge 2 ] || usage "--out needs a value"; OUT="$2"; shift 2;;
  *) usage "unknown argument '$1'";;
esac; done
[ -n "$SRC" ] && [ -d "$SRC" ] || usage "--src-root DIR (an existing directory) is required"
[ "${#TARGETS[@]}" -ge 1 ] || usage "at least one --target is required"
[ $# -ge 1 ] || usage "the command is missing after --"
[ -n "$OUT" ] || OUT="$PWD/.audit/out/bash-coverage-$(date -u +%Y%m%dT%H%M%S)-$$"
if [ -n "$EXCL" ]; then
  [ -f "$EXCL" ] || refuse exclusions_gate_failed "fence file $EXCL not found"
  GOUT="$(bash "$GATE" "$EXCL" 2>&1)" || refuse exclusions_gate_failed "the T200 gate refused $EXCL: $(printf '%s' "$GOUT" | tr '\n' ' ' | cut -c1-400)"   # MUT:gate
fi
mkdir -p "$OUT" || refuse out_uncreatable "$OUT"
TRACE="$OUT/trace.log"; : >"$TRACE"
BENV="$OUT/.cov-bashenv.sh"
cat >"$BENV" <<'BE'
# written by bash-coverage.sh: every non-interactive bash that inherits BASH_ENV traces itself to its own descriptor
PS4='+COV:${BASH_SOURCE##*/}:${LINENO}:'
exec {COV_FD}>>"$COV_TRACE_FILE"
BASH_XTRACEFD=$COV_FD
set -x
BE
# the command runs with BASH_ENV only for the run (the caller's environment is not changed)
COV_TRACE_FILE="$TRACE" BASH_ENV="$BENV" "$@" >"$OUT/command.out" 2>"$OUT/command.err" </dev/null; CMD_RC=$?
cat "$OUT/command.out"; cat "$OUT/command.err" >&2
python3 -I "$HERE/coverage/bashcov.py" report --src-root "$SRC" --trace "$TRACE" --out "$OUT/bash-coverage.json" ${EXCL:+--exclusions "$EXCL"} \
  --command "$*" --command-rc "$CMD_RC" --bash-version "$BASH_VERSION" -- "${TARGETS[@]}"; PYRC=$?
[ "$PYRC" = 0 ] || exit "$PYRC"
[ "$CMD_RC" = 0 ] || exit "$CMD_RC"   # MUT:rc
exit 0
