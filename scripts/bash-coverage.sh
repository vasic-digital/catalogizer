#!/usr/bin/env bash
# bash-coverage.sh - T199. Bash line-coverage harness (docs/05 7.1, constitution 11.4.224 E): a PS4 line trace. Every bash process the command starts (children
# included) is made to trace itself through BASH_ENV: a PS4 that carries the length-prefixed FULL path, the line, the process id, the $PWD and the sha256 of the file, and
# `set -x` with the trace sent to its own file descriptor (BASH_XTRACEFD), so a script's own `exec 2>/dev/null` cannot blind it. The executed lines of the targets divided by
# their executable lines is the figure.
# Usage: bash-coverage.sh --src-root DIR --target FILE [--target FILE ...] [--exclusions FILE] [--tools FILE] [--items-file FILE] [--out DIR] -- <command word>...
#   --src-root DIR     the tree the targets are relative to
#   --target FILE      a script to measure, relative to --src-root (repeatable); a traced line is credited to a target by path (the same file, or a copy that keeps the
#                      target's relative layout AND whose sha256 taken in the run equals the target's sha256 before it), never by base name alone; two --target arguments
#                      naming one file are refused; a target is hashed before the run and again at the report, and a change in between is refused
#   --exclusions FILE  a coverage-exclusions/1 fence (coverage/exclusions/<app>.yaml): it is COPIED once into the run directory, the copy must pass
#                      scripts/coverage/check_exclusions.sh FIRST (against the repository this script lives in, the tools record --tools and this run's targets), and only that
#                      copy is applied: the files it lists are left out of the figure and named in the record with their class and the sha256 of the copy
#   --tools FILE       coverage/exclusions/tools.yaml (default: the one of this repository); --items-file FILE a register export for first-party tracked items
#   --out DIR          a NEW or EMPTY directory (a non-empty one is refused: a stale record or a late writer of an earlier run could be read as this run's); receives
#                      bash-coverage.json (schema bash-coverage/1), trace.txt and command.stdout.txt (named so that `.gitignore` does not drop the evidence)
# The command runs in its OWN process group. INT and TERM sent to this script are forwarded to that group, the script waits for it, and the run is then recorded `interrupted`
# (exit 130 or 143) with NO figure. When the command returns, tracing is switched off (the run-scoped gate file is removed) and the group must be empty: a survivor (a
# background child still running) is signalled, reaped, and the run is refused `trace_writers_outlived_command` (a late writer would make the figure depend on timing).
# Exit: the command's own exit status when it fails (the record says command_failed, a failing suite is never swallowed); 0 otherwise; 2 usage; 3 refusal
#   (reason on stderr: target_missing, duplicate_target, exclusions_gate_failed, trace_empty - the trace saw no line at all, so the instrument was blind and a 0 percent would be
#   a lie, 11.4.201 - classifier_disagreement, target_changed_during_run, out_not_empty, out_unsafe_path, trace_writers_outlived_command); 130 / 143 interrupted.
#   HONEST LIMITS written into every record: line, not branch; `set +x` regions and `sh` (dash) children are not traced; the executable-line rule is a documented
#   heuristic (scripts/coverage/bashcov.py). Contract: docs/scripts/bash-coverage.md.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/.." && pwd)"
GATE="$HERE/coverage/check_exclusions.sh"
SRC=""; EXCL=""; OUT=""; TOOLS="$REPO/coverage/exclusions/tools.yaml"; ITEMS=""; TARGETS=()
refuse() { echo "bash-coverage: REFUSED reason=$1 ${2:-}" >&2; exit 3; }
usage() { echo "bash-coverage: usage: $1" >&2; echo "bash-coverage: bash-coverage.sh --src-root DIR --target FILE... [--exclusions FILE] [--tools FILE] [--items-file FILE] [--out DIR] -- <command>..." >&2; exit 2; }
while [ $# -gt 0 ]; do case "$1" in
  --) shift; break;;
  --src-root) [ $# -ge 2 ] || usage "--src-root needs a value"; SRC="$2"; shift 2;;
  --target) [ $# -ge 2 ] || usage "--target needs a value"; TARGETS+=("$2"); shift 2;;
  --exclusions) [ $# -ge 2 ] || usage "--exclusions needs a value"; EXCL="$2"; shift 2;;
  --tools) [ $# -ge 2 ] || usage "--tools needs a value"; TOOLS="$2"; shift 2;;
  --items-file) [ $# -ge 2 ] || usage "--items-file needs a value"; ITEMS="$2"; shift 2;;
  --out) [ $# -ge 2 ] || usage "--out needs a value"; OUT="$2"; shift 2;;
  *) usage "unknown argument '$1'";;
esac; done
[ -n "$SRC" ] && [ -d "$SRC" ] || usage "--src-root DIR (an existing directory) is required"
[ "${#TARGETS[@]}" -ge 1 ] || usage "at least one --target is required"
[ $# -ge 1 ] || usage "the command is missing after --"
if [ -z "$OUT" ]; then   # K6.6: /src is mounted read-only in the harness's own lane, so the default is /out when that is writable
  if [ -d /out ] && [ -w /out ]; then OUT="/out/bash-coverage-$(date -u +%Y%m%dT%H%M%S)-$$"; else OUT="$PWD/.audit/out/bash-coverage-$(date -u +%Y%m%dT%H%M%S)-$$"; fi
fi
# review I4: --out and --exclusions are made ABSOLUTE before anything uses them: BASH_ENV and COV_TRACE_FILE are read by children that may `cd`, and a relative
# path would then name a different file (the measured failure: 70.00 percent became 0.00 percent with exit 0)
mkdir -p "$OUT" || refuse out_uncreatable "$OUT"
OUT="$(cd "$OUT" && pwd)"   # MUT:abs_out
# K6.5: bash EXPANDS the value of BASH_ENV (parameter, command and arithmetic expansion), so an --out path with `$`, a backtick or a backslash would run code in every traced bash
case "$OUT" in *'$'*|*'`'*|*'\'*|*$'\n'*) refuse out_unsafe_path "the --out path contains a character that bash expands in BASH_ENV";; esac   # MUT:unsafe_out
# K7.2 / K8.5: a directory that already holds files may hold a stale record or the trace of an orphan of an earlier run
[ -z "$(ls -A "$OUT" 2>/dev/null)" ] || refuse out_not_empty "$OUT already holds files (use a new directory)"   # MUT:out_not_empty
SRC="$(cd "$SRC" && pwd)"
TRACE="$OUT/trace.txt"; : >"$TRACE"
GATEFILE="$OUT/.cov-gate"; : >"$GATEFILE"
BENV="$OUT/.cov-bashenv.sh"
# K8.2: every target is hashed BEFORE the run (and again at the report); copy credit needs this hash
: >"$OUT/targets.sha256"
for T_ in "${TARGETS[@]}"; do
  NT_="$(python3 -I -c 'import posixpath,sys; print(posixpath.normpath(sys.argv[1]))' "$T_")"   # K3.4: `./x` and `a/../x` are the file the fence names
  case "$NT_" in /*|..|../*) refuse target_outside_src "$T_ is not under $SRC";; esac
  [ -f "$SRC/$T_" ] || refuse target_missing "$T_ (not a file under $SRC)"
  printf '%s  %s\n' "$(sha256sum <"$SRC/$T_" | cut -d' ' -f1)" "$NT_" >>"$OUT/targets.sha256"
done
if [ -n "$EXCL" ]; then
  [ -f "$EXCL" ] || refuse exclusions_gate_failed "fence file $EXCL not found"
  # K8.1: the fence is snapshotted ONCE; the snapshot is gated and the snapshot is applied, so a concurrent edit of the original cannot change what was gated
  mkdir -p "$OUT/fence"; SNAP="$OUT/fence/$(basename "$EXCL")"; cp "$EXCL" "$SNAP" || refuse exclusions_gate_failed "cannot snapshot $EXCL"
  printf '%s\n' "${TARGETS[@]}" >"$OUT/targets.txt"
  GOUT="$(bash "$GATE" "$SNAP" --root "$SRC" --repo "$REPO" --tools "$TOOLS" --measured "$OUT/targets.txt" --json "$OUT/exclusions-check.json" ${ITEMS:+--items-file "$ITEMS"} 2>&1)" || refuse exclusions_gate_failed "the T200 gate refused $EXCL: $(printf '%s' "$GOUT" | tr '\n' ' ' | cut -c1-400)"   # MUT:gate
  EXCL="$SNAP"
fi
cat >"$BENV" <<'BE'
# written by bash-coverage.sh: every non-interactive bash that inherits BASH_ENV traces itself to its own descriptor while the run-scoped gate file exists
[ -e "${COV_GATE:-/nonexistent}" ] || return 0
declare -A COV_H=()
PS4=$'+COV\037${#BASH_SOURCE}\037${BASH_SOURCE}\037${LINENO}\037$$\037${PWD}\037${COV_H[${BASH_SOURCE:-/dev/null}]:=$(sha256sum <"${BASH_SOURCE:-/dev/null}" 2>/dev/null)}\037'
exec {COV_FD}>>"$COV_TRACE_FILE"
BASH_XTRACEFD=$COV_FD
set -x
BE
# ---- the run: its own process group, signals forwarded, the group must be empty when the command returns ----
INTR=""; INTN=0; CPID=0
forward() { # forward SIGNAL: a validated group id only (11.4.263: never a group id <= 1, never this script's own group)
  [ "$CPID" -gt 1 ] 2>/dev/null || return 0
  [ "$CPID" != "$$" ] || return 0
  kill -s "$1" -- "-$CPID" 2>/dev/null
}
on_sig() { INTR="${INTR:-$1}"; INTN=$((INTN+1)); case "$INTN" in 1) forward "$1";; 2) forward TERM;; *) forward KILL;; esac; }
trap 'on_sig INT' INT; trap 'on_sig TERM' TERM
set -m
( COV_TRACE_FILE="$TRACE" COV_GATE="$GATEFILE" BASH_ENV="$BENV" exec "$@" ) >"$OUT/command.stdout.txt" 2>"$OUT/command.err" </dev/null &
CPID=$!
set +m
while :; do N0=$INTN; wait "$CPID" 2>/dev/null; CMD_RC=$?; [ "$INTN" != "$N0" ] || break; done   # MUT:wait_loop
rm -f "$GATEFILE"   # tracing is off from here: a bash started later does not trace
SIZE0="$(stat -c %s "$TRACE" 2>/dev/null || echo 0)"
SURV=0
if [ "$CPID" -gt 1 ] && kill -0 -- "-$CPID" 2>/dev/null; then   # MUT:survivors
  SURV=1; forward TERM
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do kill -0 -- "-$CPID" 2>/dev/null || break; sleep 0.1; done
  kill -0 -- "-$CPID" 2>/dev/null && { forward KILL; sleep 0.2; }
fi
SIZE1="$(stat -c %s "$TRACE" 2>/dev/null || echo 0)"
cat "$OUT/command.stdout.txt"; cat "$OUT/command.err" >&2
COMMON=(--out "$OUT/bash-coverage.json" --command "$*" --command-rc "$CMD_RC" --bash-version "$BASH_VERSION")
if [ -n "$INTR" ]; then   # MUT:interrupted
  python3 -I "$HERE/coverage/bashcov.py" refuse --status interrupted --reason "$INTR was received while the command ran; the group was signalled and waited for, and no figure is reported" "${COMMON[@]}" 2>&1 >/dev/null | tail -1 >&2
  [ "$INTR" = INT ] && exit 130; exit 143
fi
if [ "$SURV" = 1 ] || [ "$SIZE1" != "$SIZE0" ]; then   # MUT:late_writers
  python3 -I "$HERE/coverage/bashcov.py" refuse --status trace_writers_outlived_command --reason "a process of the command's group was still running when the command returned (or wrote trace lines afterwards): the figure would depend on timing" "${COMMON[@]}" 2>&1 >/dev/null | tail -1 >&2
  exit 3
fi
python3 -I "$HERE/coverage/bashcov.py" report --src-root "$SRC" --trace "$TRACE" --pre-sha "$OUT/targets.sha256" ${EXCL:+--exclusions "$EXCL"} \
  "${COMMON[@]}" -- "${TARGETS[@]}"; PYRC=$?
[ "$PYRC" = 0 ] || exit "$PYRC"
[ "$CMD_RC" = 0 ] || exit "$CMD_RC"   # MUT:rc
exit 0
