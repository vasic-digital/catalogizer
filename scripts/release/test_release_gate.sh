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

HA="$(printf 'a%.0s' {1..64})"; HB="$(printf 'b%.0s' {1..64})"
setup() { D="$(mktemp -d "$ROOT/c.XXXXXX")"
  printf 'product=opi5\ntarget_id=SER123\nconnected=yes\nread_from=target\nkind=physical\nbuild_sha256=%s\n' "$HB" >"$D/hw"
  printf 'product=opi5\nsuite=full\ncomplete=yes\ntests_total=3\ntests_reported=3\nverdicts_sha256=%s\n' "$HA" >"$D/live"
  printf 'schema=image_no_worse/1\nverdict=NO-WORSE-PROVEN\nreasons=\nnew_verdicts_sha256=%s\nboard=SER123\nsuite=full\nimage_new=system.img 1.1 %s\nbaseline_notpass=1\nnew_total=3\nnew_notpass_on_baseline_set=0\nlost=\ntargets_missing=\ndiff=t1 FAIL PASS\n' "$HA" "$HB" >"$D/cmp"; }
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
setup; sed -i "s/$HA/$HB/" "$D/cmp"; expect "comparison for another run" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/NO-WORSE-PROVEN/REGRESSED/' "$D/cmp"; expect "regressed" 22 REFUSED-REGRESSED
setup; sed -i 's/NO-WORSE-PROVEN/MAYBE/' "$D/cmp"; run >/dev/null 2>&1; [ $? = 2 ] && ok || bad "unknown verdict must exit 2"
setup; "$GATE" --bogus >/dev/null 2>&1; [ $? = 2 ] && ok || bad "unknown option must exit 2"
setup; "$GATE" --product "" --hardware-record "$D/hw" --live-run-record "$D/live" --comparison-record "$D/cmp" --out "$D/g" >/dev/null 2>&1; [ $? = 2 ] && ok || bad "empty product must exit 2"
setup; rm "$D/cmp"; if run >/dev/null 2>&1; then bad "blocked exited 0"; else ok; fi
setup; echo "state=RELEASABLE" >"$D/gate"; rm "$D/live"; run >/dev/null 2>&1; grep -qx 'state=BLOCKED-NO-HARDWARE' "$D/gate" && ok || bad "stale gate record kept"

# ---- review round 1 ----
setup; sed -i '/^kind=/d' "$D/hw"; expect "kind missing (I-4)" 20 BLOCKED-NO-HARDWARE
for kd in emulator host; do setup; sed -i "s/kind=physical/kind=$kd/" "$D/hw"; expect "kind=$kd" 20 BLOCKED-NO-HARDWARE; done
setup; sed -i 's/^target_id=.*/target_id=/' "$D/hw"; expect "empty target_id (R7)" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/tests_total=3/tests_total=0/; s/tests_reported=3/tests_reported=0/' "$D/live"; expect "tests_total=0 (R6)" 20 BLOCKED-NO-HARDWARE
setup; sed -i '/^verdicts_sha256=/d' "$D/live"; expect "live verdicts_sha256 absent (I-3)" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/^verdicts_sha256=.*/verdicts_sha256=/' "$D/live"; sed -i 's/^new_verdicts_sha256=.*/new_verdicts_sha256=/' "$D/cmp"; expect "both verdict shas empty (I-3)" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/^verdicts_sha256=.*/verdicts_sha256=abc/' "$D/live"; sed -i 's/^new_verdicts_sha256=.*/new_verdicts_sha256=abc/' "$D/cmp"; expect "non-hex equal shas (I-3)" 20 BLOCKED-NO-HARDWARE
setup; sed -i '/^new_verdicts_sha256=/d' "$D/cmp"; expect "comparison sha absent" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^new_verdicts_sha256=.*/new_verdicts_sha256=/' "$D/cmp"; expect "comparison sha empty" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^board=.*/board=OTHER/' "$D/cmp"; expect "comparison board != target_id (I-4)" 21 BLOCKED-NO-BASELINE
setup; sed -i '/^board=/d' "$D/cmp"; expect "comparison board absent" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^suite=.*/suite=smoke/' "$D/cmp"; expect "suite mismatch (I-4)" 21 BLOCKED-NO-BASELINE
setup; sed -i '/^image_new=/d' "$D/cmp"; expect "comparison without image_new" 21 BLOCKED-NO-BASELINE
setup; sed -i "s/^build_sha256=.*/build_sha256=$HA/" "$D/hw"; expect "target build != compared new image (I-4)" 21 BLOCKED-NO-BASELINE
setup; sed -i '/^build_sha256=/d' "$D/hw"; expect "no build fingerprint from target" 20 BLOCKED-NO-HARDWARE
setup; echo "build_sha256=$HA" >>"$D/hw"; expect "extra target artifact not in compared image" 21 BLOCKED-NO-BASELINE
setup; expect "golden-good" 0 RELEASABLE
grep -qx "build_identity=system.img 1.1 $HB" "$D/gate" && ok || bad "gate record lacks build identity"
# I-5: BLIND paths rewrite the record
setup; echo "state=RELEASABLE" >"$D/gate"; sed -i 's/NO-WORSE-PROVEN/MAYBE/' "$D/cmp"; run >/dev/null 2>&1
grep -qx 'state=BLIND' "$D/gate" && ok || bad "unknown verdict left stale RELEASABLE"
setup; echo "state=RELEASABLE" >"$D/gate"; "$GATE" --bogus --out "$D/gate" >/dev/null 2>&1
grep -qx 'state=BLIND' "$D/gate" && ok || bad "usage error left stale RELEASABLE"
setup; if "$GATE" --product opi5 --hardware-record "$D/hw" --live-run-record "$D/live" --comparison-record "$D/cmp" --out "$D/nodir/g" >/dev/null 2>&1; then bad "unwritable --out exited 0"; else ok; fi

# ---- review round 3 ----
HC="$(printf 'c%.0s' {1..64})"
setup; sed -i '/^schema=/d' "$D/cmp"; expect "comparison schema absent (I-6)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^schema=.*/schema=image_no_worse\/2/' "$D/cmp"; expect "comparison schema wrong (I-6)" 21 BLOCKED-NO-BASELINE
setup; sed -i '/^new_total=/d' "$D/cmp"; expect "comparison new_total absent (I-6)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^new_total=.*/new_total=2/' "$D/cmp"; expect "comparison new_total != tests_total: truncated (I-6)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^new_total=.*/new_total=x/' "$D/cmp"; expect "comparison new_total non-integer (I-6)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^image_new=.*/& extra/' "$D/cmp"; expect "image_new extra field (I-6)" 21 BLOCKED-NO-BASELINE
setup; echo "image_new=system.img 1.1 $HB" >>"$D/cmp"; expect "duplicate image_new line (M-d)" 21 BLOCKED-NO-BASELINE
setup; echo "image_new=system.img 1.1 $HC" >>"$D/cmp"; echo "build_sha256=$HC" >>"$D/hw"; expect "same file with two shas (M-d)" 21 BLOCKED-NO-BASELINE
setup; sed -i '/^lost=/d' "$D/cmp"; expect "NO-WORSE record lacks lost= (M-a)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^lost=.*/lost=t9(FAIL)/' "$D/cmp"; expect "NO-WORSE record with lost (M-a)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^reasons=.*/reasons=baseline-pass-lost/' "$D/cmp"; expect "NO-WORSE record with reasons (M-a)" 21 BLOCKED-NO-BASELINE
setup; sed -i 's/^new_notpass_on_baseline_set=.*/new_notpass_on_baseline_set=5/' "$D/cmp"; expect "NO-WORSE with more not-pass (M-a)" 21 BLOCKED-NO-BASELINE
setup; echo "diff=t2 PASS FAIL" >>"$D/cmp"; expect "NO-WORSE with PASS-lost diff (M-a)" 21 BLOCKED-NO-BASELINE
# r2: legitimate multi-artifact build (target order differs from compared order) must pass
setup; printf 'build_sha256=%s\n' "$HC" >>"$D/hw"; echo "image_new=boot.img 1.1 $HC" >>"$D/cmp"; expect "multi-artifact build (r2)" 0 RELEASABLE
setup; sed -i "/^build_sha256=/d" "$D/hw"; printf 'build_sha256=%s\nbuild_sha256=%s\n' "$HC" "$HB" >>"$D/hw"; echo "image_new=boot.img 1.1 $HC" >>"$D/cmp"; expect "multi-artifact, target reordered (r2)" 0 RELEASABLE
# r3: CR in a record value
setup; sed -i 's/^kind=physical/kind=physical\r/' "$D/hw"; expect "CR in hardware value (r3)" 20 BLOCKED-NO-HARDWARE
setup; sed -i 's/^board=SER123/board=SER123\r/' "$D/cmp"; expect "CR in comparison board (r3)" 21 BLOCKED-NO-BASELINE
# M-c: --out may not be an input
for f in hw live cmp; do setup; cp "$D/$f" "$D/$f.orig"
  "$GATE" --product opi5 --hardware-record "$D/hw" --live-run-record "$D/live" --comparison-record "$D/cmp" --out "$D/$f" >/dev/null 2>&1; rc=$?
  { [ "$rc" = 2 ] && cmp -s "$D/$f" "$D/$f.orig"; } && ok || bad "--out == $f must exit 2 and leave the input intact (rc=$rc)"; done
setup; ln -s "$D/cmp" "$D/lnk"; cp "$D/cmp" "$D/cmp.orig"
"$GATE" --product opi5 --hardware-record "$D/hw" --live-run-record "$D/live" --comparison-record "$D/cmp" --out "$D/lnk" >/dev/null 2>&1; rc=$?
{ [ "$rc" = 2 ] && cmp -s "$D/cmp" "$D/cmp.orig"; } && ok || bad "--out symlink to an input must exit 2 (rc=$rc)"

echo "release_gate tests: pass=$PASS fail=$FAIL"
[ "$FAIL" = 0 ]
