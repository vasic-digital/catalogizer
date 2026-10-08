#!/bin/bash -p
# T041 helper: record_deferral.sh - writes one deferral row into a CPA run directory (never into the tracked tree).
#
# Purpose   S0/S4 of a CPA run (docs/16 section 12.2) record a deferred gate here; commit_recursive.sh later reads the
#           run's flag set as the `Deferred-Gates:` trailer value (--list).
# Usage     record_deferral.sh --run-dir <dir> --flag SKIP_LONG|SWEEP_ABSENT|LOCAL_ONLY|CHECK_PENDING_RELEASE|CHECKS_DEFERRED|GATES_NOT_BUILT|CHECKS_NOT_JUDGED --reason <text>
#                              [--awaits-review <verdict path>] [--commit <sha>]
#           record_deferral.sh --run-dir <dir> --list      prints the comma-joined flag set in closed-set order
# Row       <dir>/deferrals.tsv: utc_time <TAB> flag <TAB> reason <TAB> awaits_review <TAB> commit  (header line first)
# Exits     0 ok; 20 internal/usage error (unknown flag, empty reason, run dir in the tracked tree, failed write).
#           A failed write is 20, never a success without its row (a 14 without its row is forbidden, T042).
# Never     touches git state, the network, or any path outside <dir>.
set -u
# The last three are the flags of a gate that did NOT run (WF11 review F3): CHECK_PENDING_RELEASE a registry row the approved copy lacks,
# CHECKS_DEFERRED a registry row whose mode is `deferred`, GATES_NOT_BUILT a gate of scripts/repo/owed_gates.tsv (or an unreadable list), CHECKS_NOT_JUDGED a declared
# path that no S3 check judged (a symlink of a non-evidence class, a file the check its suffix selects left out; WF14 N4). Appended at the
# END of the closed set: the order of --list for the first three flags is unchanged.
CLOSED="SKIP_LONG SWEEP_ABSENT LOCAL_ONLY CHECK_PENDING_RELEASE CHECKS_DEFERRED GATES_NOT_BUILT CHECKS_NOT_JUDGED"
die() { echo "record_deferral: $1: $2" >&2; exit 20; }
RUN=""; FLAG=""; REASON=""; AWAIT=""; COMMIT=""; LIST=0; HAVE_REASON=0
while [ $# -gt 0 ]; do
  case "$1" in --run-dir|--flag|--reason|--awaits-review|--commit) [ $# -ge 2 ] || die usage "$1 needs a value" ;; esac
  case "$1" in
    --run-dir) RUN="$2"; shift 2 ;;
    --flag) FLAG="$2"; shift 2 ;;
    --reason) REASON="$2"; HAVE_REASON=1; shift 2 ;;
    --awaits-review) AWAIT="$2"; shift 2 ;;
    --commit) COMMIT="$2"; shift 2 ;;
    --list) LIST=1; shift ;;
    *) die usage "unknown argument '$1'" ;;
  esac
done
[ -n "$RUN" ] || die usage "--run-dir is required"
# the run directory must not be a tracked or un-ignored path of a git work tree (the row must never reach a commit)
if [ -d "$RUN" ] || mkdir -p "$RUN" 2>/dev/null; then :; else die run_dir_unwritable "$RUN"; fi
RUN="$(cd "$RUN" && pwd)" || die run_dir_unwritable "$RUN"
if top="$(git -C "$RUN" rev-parse --show-toplevel 2>/dev/null)"; then
  if [ -n "$(git -C "$top" ls-files -- "$RUN" 2>/dev/null)" ]; then die run_dir_in_tracked_tree "$RUN holds tracked files"; fi
  git -C "$top" check-ignore -q -- "$RUN/deferrals.tsv" || die run_dir_in_tracked_tree "$RUN is not ignored by git (a row there would be committed)"
fi
F="$RUN/deferrals.tsv"
if [ "$LIST" = 1 ]; then
  [ -f "$F" ] || exit 0
  out=""; for f in $CLOSED; do
    if awk -F'\t' -v f="$f" 'NR>1 && $2==f {found=1} END{exit !found}' "$F"; then out="${out:+$out,}$f"; fi
  done; echo "$out"; exit 0
fi
ok=0; for f in $CLOSED; do [ "$FLAG" = "$f" ] && ok=1; done
[ "$ok" = 1 ] || die flag_not_in_closed_set "'$FLAG' (closed set: $CLOSED)"
[ "$HAVE_REASON" = 1 ] && [ -n "$REASON" ] || die reason_required "a deferral row needs a reason"
case "$REASON$AWAIT$COMMIT" in *[[:cntrl:]]*) die field_invalid "a control character in a field (tab, newline, CR, ESC ...)" ;; esac     # lib_safe safe_line's rule: the row is a TSV line and is later shown in a terminal (ENC-1)
if [ -n "$COMMIT" ] && [ -z "$AWAIT" ]; then die awaits_review_required "a held commit names the verdict it waits for"; fi
if [ ! -f "$F" ]; then printf 'utc_time\tflag\treason\tawaits_review\tcommit\n' > "$F" || die write_failed "$F"; fi
row="$(date -u +%Y-%m-%dT%H:%M:%SZ)	$FLAG	$REASON	$AWAIT	$COMMIT"
printf '%s\n' "$row" >> "$F" || die write_failed "$F"
# read back: the row must really be the last line (a short write is an error, never a 14 without its row)
[ "$(tail -n 1 "$F")" = "$row" ] || die write_failed "read-back mismatch in $F"
exit 0
