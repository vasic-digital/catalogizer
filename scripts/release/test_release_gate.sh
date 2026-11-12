#!/usr/bin/env bash
# Test suite for release_gate.sh (constitution 11.4.298 per-product gate record).
# CLASS: gate-logic unit tests (11.4.289(C)(2)); record files are fixture INPUT data, not evidence
# that any hardware exists or any build is releasable. GATE=<path> runs against another copy.
# shellcheck disable=SC2015  # A && ok || bad is intentional (ok never fails)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE="${GATE:-$HERE/release_gate.sh}"
BASE="${NOWORSE_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/nowc-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/gtest.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/gtest.*) ;; *) echo "odd ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT
PASS=0; FAIL=0
ok() { PASS=$((PASS+1)); }
bad() { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }

setup() { D="$(mktemp -d "$ROOT/c.XXXXXX")"
  printf 'product=opi5\ntarget_id=SER123\nconnected=yes\nread_from=target\n' >"$D/hw"
  printf 'product=opi5\nsuite=full\ncomplete=yes\ntests_total=3\ntests_reported=3\nverdicts_sha256=abc\n' >"$D/live"
  printf 'verdict=NO-WORSE-PROVEN\nnew_verdicts_sha256=abc\n' >"$D/cmp"; }
run() { "$GATE" --product opi5 --hardware-record "$D/hw" --live-run-record "$D/live" --comparison-record "$D/cmp" --out "$D/gate" "$@"; }
expect() { local label="$1" rc="$2" state="$3"; shift 3; local out got
  out="$(run "$@" 2>&1)"; got=$?
  [ "$got" = "$rc" ] || { bad "$label: rc=$got want $rc ($out)"; return; }
  printf '%s\n' "$out" | grep -qF -- "GATE-STATE: $state" || { bad "$label: lacks state $state ($out)"; return; }
  grep -qx "state=$state" "$D/gate" || { bad "$label: gate record lacks state"; return; }
  ok; }

setup; expect "golden-good RELEASABLE" 0 RELEASABLE
grep -q '^product=opi5$' "$D/gate" && grep -q '^hardware_target_id=SER123$' "$D/gate" && ok || bad "gate record fields"
setup; rm "$D/live";   expect "no live-run record" 20 BLOCKED-NO-HARDWARE
grep -qx 'reason=no live-run record' "$D/gate" && ok || bad "no live-run record: reason not recorded"
setup; rm "$D/hw";     expect "no hardware record" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/connected=yes/connected=no/' "$D/hw"; expect "not connected" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/read_from=target/read_from=emulator/' "$D/hw"; expect "emulator identity" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/product=opi5/product=other/' "$D/hw"; expect "other product hardware" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/product=opi5/product=other/' "$D/live"; expect "other product live run" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/complete=yes/complete=no/' "$D/live"; expect "incomplete live run" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/tests_reported=3/tests_reported=2/' "$D/live"; expect "reported < total" 20 BLOCKED-NO-HARDWARE
setup; rm "$D/cmp";    expect "comparison missing" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/NO-WORSE-PROVEN/INCOMPARABLE-BASELINE-MISSING/' "$D/cmp"; expect "incomparable" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/abc/zzz/' "$D/cmp"; expect "comparison for another run" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/NO-WORSE-PROVEN/REGRESSED/' "$D/cmp"; expect "regressed" 22 REFUSED-REGRESSED
setup; sed -i 's/NO-WORSE-PROVEN/MAYBE/' "$D/cmp"; run >/dev/null 2>&1; [ $? = 2 ] && ok || bad "unknown verdict must exit 2"
setup; "$GATE" --bogus >/dev/null 2>&1; [ $? = 2 ] && ok || bad "unknown option must exit 2"
setup; "$GATE" --product "" --hardware-record "$D/hw" --live-run-record "$D/live" --comparison-record "$D/cmp" --out "$D/g" >/dev/null 2>&1; [ $? = 2 ] && ok || bad "empty product must exit 2"
setup; rm "$D/cmp"; if run >/dev/null 2>&1; then bad "blocked exited 0"; else ok; fi
setup; echo "state=RELEASABLE" >"$D/gate"; rm "$D/live"; run >/dev/null 2>&1; grep -qx 'state=BLOCKED-NO-HARDWARE' "$D/gate" && ok || bad "stale gate record kept"

echo "release_gate tests: pass=$PASS fail=$FAIL"
[ "$FAIL" = 0 ]
