#!/usr/bin/env bash
# test-in-container.sh - T121 (TIC). The lane dispatcher: every build and test lane of the project is started through this one command, which
# looks the (app, lane) up in scripts/containers/lanes.tsv and hands the command to the wrapper that row names (run_go.sh, run_node.sh, run_docs.sh,
# run_scan.sh, run_playwright.sh, run_testutil.sh, later run_qa.sh, run_rust.sh, run_kcov.sh), with the dynamic envelope limits composed in
# (scripts/containers/envelope.sh: --memory and --cpus are always passed). There is NO bare-host fallback: an unknown app, an unknown lane, an
# (app, lane) without a row, a row whose wrapper does not exist yet - each is REFUSED with a non-zero exit and the command never runs.
#
# Usage:  test-in-container.sh [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#                              [--no-progress-s N] [--wall-s N] <app> <lane> -- <command word>...
#   The options are handed to the wrapper unchanged. App keys: catalog-api, catalog-web, qa (reserved, no rows until T212), docs, tooling.
#   Lanes: unit, contract, integration, e2e, api, docs, render, tooling.
# Exits:  the wrapper's exit code on a run; 1 REFUSED (`test-in-container: REFUSED reason=<code>` on stderr: unknown_app, unknown_lane, no_lane_row,
#   lane_table_unreadable, lane_table_malformed, lane_table_duplicate, wrapper_missing, envelope_refused, test_hook_outside_test_mode); 2 usage.
# Test hooks (honoured ONLY with TIC_TEST_MODE=1, else REFUSED test_hook_outside_test_mode): TIC_WRAPPER_DIR (directory holding the run_*.sh shims),
#   TIC_LANES (lane table file). The envelope hooks of envelope.sh (ENVELOPE_*) are separate and have their own gate.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CDIR="$HERE/containers"
refuse() { echo "test-in-container: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
usage()  { echo "test-in-container: usage: $1" >&2; echo "test-in-container: test-in-container.sh [wrapper options] <app> <lane> -- <cmd>..." >&2; exit 2; }
valid_int() { case "$1" in ''|*[!0-9]*) return 1;; 0) return 0;; 0*) return 1;; esac; [ "${#1}" -le 18 ]; }

APPS="catalog-api catalog-web qa docs tooling build-scripts catalogizer-desktop installer-wizard"
LANES="unit contract integration e2e api docs render tooling rust"
KNOWN_WRAPPERS="run_go run_node run_docs run_scan run_playwright run_testutil run_qa run_rust run_kcov"
APP=""; LANE=""; PASS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --) shift; break;;
    --out|--rw|--need|--purpose|--op-id|--no-progress-s|--wall-s) [ $# -ge 2 ] || usage "$1 requires a value"; PASS+=("$1" "$2"); shift 2;;
    --network=none) PASS+=("$1"); shift;;
    --memory|--cpus) usage "$1 is composed from the envelope by TIC and is not an option of it";;
    -*) usage "unknown option '$1'";;
    *) if [ -z "$APP" ]; then APP="$1"; elif [ -z "$LANE" ]; then LANE="$1"; else usage "unexpected argument '$1' before --"; fi; shift;;
  esac
done
[ -n "$APP" ] && [ -n "$LANE" ] || usage "app and lane are required"
[ $# -ge 1 ] || usage "command missing after --"
CMD=("$@")

# ---- test hooks: only a declared test run may replace a real component ----
WDIR="$CDIR"; TABLE="$CDIR/lanes.tsv"
if { [ -n "${TIC_WRAPPER_DIR+x}" ] || [ -n "${TIC_LANES+x}" ]; } && [ "${TIC_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
  refuse test_hook_outside_test_mode "TIC_WRAPPER_DIR / TIC_LANES are test hooks and need TIC_TEST_MODE=1"
fi
[ -z "${TIC_WRAPPER_DIR:-}" ] || WDIR="$TIC_WRAPPER_DIR"
[ -z "${TIC_LANES:-}" ] || TABLE="$TIC_LANES"

# ---- the lane table: read, validated as a whole (a malformed or duplicated row refuses every lane) ----
[ -r "$TABLE" ] || refuse lane_table_unreadable "$TABLE"
ROWS=()
while IFS=$'\t' read -r a l w extra || [ -n "${a:-}" ]; do
  case "${a:-}" in ''|'#'*) continue;; esac
  { [ -n "${l:-}" ] && [ -n "${w:-}" ] && [ -z "${extra:-}" ]; } || refuse lane_table_malformed "row '$a' needs exactly three TAB separated columns: app lane wrapper"
  case " $APPS " in *" $a "*) ;; *) refuse lane_table_malformed "row app '$a' is not one of: $APPS";; esac
  case " $LANES " in *" $l "*) ;; *) refuse lane_table_malformed "row lane '$l' is not one of: $LANES";; esac
  case " $KNOWN_WRAPPERS " in *" $w "*) ;; *) refuse lane_table_malformed "row ($a $l) names '$w', which is not a reviewed wrapper: $KNOWN_WRAPPERS";; esac
  for r in "${ROWS[@]:-}"; do case "$r" in "$a	$l	"*) refuse lane_table_duplicate "($a $l) has two rows";; esac; done   # MUT:duplicate
  ROWS+=("$a	$l	$w")
done <"$TABLE"

# ---- lookup: never a fallback ----
case " $APPS " in *" $APP "*) ;; *) refuse unknown_app "'$APP' is not one of: $APPS";; esac   # MUT:unknown-app
case " $LANES " in *" $LANE "*) ;; *) refuse unknown_lane "'$LANE' is not one of: $LANES";; esac   # MUT:unknown-lane
WRAPPER=""
for r in "${ROWS[@]:-}"; do
  IFS=$'\t' read -r ra rl rw <<<"$r"
  if [ "$ra" = "$APP" ] && [ "$rl" = "$LANE" ]; then WRAPPER="$rw"; break; fi   # MUT:lookup
done
if [ -z "$WRAPPER" ]; then
  if [ "$APP" = qa ]; then refuse unknown_app "'qa' is reserved and has no rows until T212 adds the run_qa.sh rows; it is never sent to another image"; fi
  refuse no_lane_row "($APP $LANE) has no row in $TABLE; a lane exists only by a reviewed row, and there is no bare-host fallback"   # MUT:no-row
fi
[ -f "$WDIR/$WRAPPER.sh" ] || refuse wrapper_missing "($APP $LANE) names $WRAPPER.sh, which does not exist in $WDIR yet (BLOCKED until its task adds it); nothing was run"   # MUT:wrapper-missing

# ---- the envelope limits, composed into the call ----
ENVJ="$(bash "$CDIR/envelope.sh" --toolchain "${WRAPPER#run_}" --format json 2>&1)" || refuse envelope_refused "$ENVJ"
MEM="$(jq -r .memory_bytes <<<"$ENVJ" 2>/dev/null)"; CPUS="$(jq -r .cpus <<<"$ENVJ" 2>/dev/null)"
{ valid_int "$MEM" && valid_int "$CPUS" && [ "$MEM" -ge 1 ] && [ "$CPUS" -ge 1 ]; } || refuse envelope_refused "unparsable envelope: $ENVJ"
# the wrapper reads the envelope again moments later and (fix round r1, review F2/F16) no longer tolerates a request above its own reading, so TIC asks for LESS:
# 98% of its reading covers a 2% fall of MemAvailable between the two reads (a request below the wrapper's reading is always honoured)
MEM=$(( MEM - MEM / 50 ))

bash "$WDIR/$WRAPPER.sh" --memory "$MEM" --cpus "$CPUS" "${PASS[@]}" -- "${CMD[@]}"   # MUT:memory MUT:cpus
exit $?   # MUT:exit
