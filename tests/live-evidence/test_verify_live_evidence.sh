#!/usr/bin/env bash
# Tests of scripts/test-infra/verify_live_evidence.sh (constitution 11.4.289(D)). Gate-logic UNIT test (11.4.289(C)(2)): fixtures prove the verifier decides correctly.
# VERIFIER=<path> runs the suite against another copy (used by the paired mutation test). Control needles per 11.4.273.
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
VERIFIER=${VERIFIER:-$ROOT/scripts/test-infra/verify_live_evidence.sh}
FX=$HERE/fixtures
# test register = the project default + the hash that fixture bad_double_by_hash stands for
TREG=$(mktemp); trap 'rm -f "${TREG:?}"' EXIT
{ cat "$ROOT/docs/qa/live-evidence.doubles.txt"; printf 'sha256:%s\n' "$(printf b | sha256sum | cut -d' ' -f1)"; } > "$TREG"
export LIVE_EVIDENCE_DOUBLES=$TREG
pass=0; fail=0
ok(){ pass=$((pass+1)); }
no(){ fail=$((fail+1)); echo "FAIL: $1" >&2; }
# expect <fixture> <exit> <class> [reason-substring]
expect(){
  local f=$1 want=$2 cls=$3 sub=${4:-} out rc
  out=$("$VERIFIER" "$FX/$f.json" 2>&1); rc=$?
  [ "$rc" = "$want" ] || { no "$f exit=$rc want=$want out=$out"; return; }
  case "$out" in *"class=$cls"*) ;; *) no "$f class want=$cls out=$out"; return;; esac
  if [ -n "$sub" ]; then case "$out" in *"$sub"*) ;; *) no "$f reason want=$sub out=$out"; return;; esac; fi
  ok
}
[ -x "$VERIFIER" ] || { echo "FAIL: verifier missing or not executable: $VERIFIER" >&2; exit 1; }
expect golden_good 0 LIVE
expect golden_false_carrier 0 LIVE          # real service on loopback, test-started, under a scratch dir: must NOT be flagged
expect golden_false_container 0 LIVE
expect declared_unit 0 UNIT
expect skip_good 0 SKIP credentials_absent
expect bad_double_by_hash 1 UNIT registered_double
expect bad_double_by_name 1 UNIT registered_double
expect bad_double_by_cmdline 1 UNIT registered_double
expect bad_mismatch 1 UNIT identity_mismatch
expect bad_image_mismatch 1 UNIT identity_mismatch
expect bad_missing_answering 1 UNIT answering_identity_missing
expect bad_missing_fingerprint 1 UNIT artifact_fingerprint_missing
expect bad_skip_reason 2 INVALID skip_reason
expect bad_skip_no_reason 2 INVALID skip_reason
expect invalid_not_identical 2 INVALID runs
expect invalid_json 2 INVALID
expect bad_no_endpoint 1 UNIT endpoint_identity_missing
expect bad_no_expected 1 UNIT expected_identity_missing
expect bad_schema 2 INVALID schema
expect bad_class 2 INVALID class
expect bad_verdict 2 INVALID verdict
expect bad_empty_signature 2 INVALID runs_not_identical
expect bad_socket_no_hash 1 UNIT answering_identity_missing
expect bad_socket_no_pid 1 UNIT answering_identity_missing
expect golden_socket 0 LIVE
expect bad_fp_mismatch 1 UNIT fingerprint_mismatch
expect bad_no_cmdline 1 UNIT endpoint_identity_incomplete
expect bad_runs_below_3 1 UNIT runs_below_3
expect bad_port_string 2 INVALID port_not_integer
expect bad_timestamp 2 INVALID timestamp_not_rfc3339
expect bad_resolve_error 1 UNIT listener_unresolved_or_ambiguous
expect bad_skip_no_runs 2 INVALID runs_missing
expect golden_interp_script 0 LIVE
expect bad_interp_no_script 1 UNIT interpreter_script_missing
expect bad_interp_unbound 1 UNIT interpreter_script_unbound
expect bad_multi_value 2 INVALID not_exactly_one_json_value
# --require-live turns a declared UNIT record into a failure
"$VERIFIER" --require-live "$FX/declared_unit.json" >/dev/null 2>&1; [ $? = 1 ] && ok || no "require-live on UNIT must exit 1"
# --require-live: a SKIP is never live proof
"$VERIFIER" --require-live "$FX/skip_good.json" >/dev/null 2>&1; [ $? = 1 ] && ok || no "require-live on SKIP must exit 1"
"$VERIFIER" --require-live "$FX/golden_good.json" >/dev/null 2>&1; [ $? = 0 ] && ok || no "require-live on LIVE must exit 0"
# directory mode: worst exit wins, every file reported
out=$("$VERIFIER" "$FX" 2>&1); rc=$?
[ "$rc" = 2 ] && ok || no "dir mode exit=$rc want 2"
n=$(printf '%s\n' "$out" | grep -c 'class='); [ "$n" = "$(ls "$FX"/*.json | wc -l)" ] && ok || no "dir mode reported $n files"
# no input / unknown option = usage error 2
"$VERIFIER" >/dev/null 2>&1; [ $? = 2 ] && ok || no "no args must exit 2"
# control needle: the verifier can see a known-present marker and not a fabricated one (11.4.273)
"$VERIFIER" "$FX/bad_double_by_hash.json" 2>&1 | grep -q registered_double && ok || no "needle+ missing"
"$VERIFIER" "$FX/golden_good.json" 2>&1 | grep -q registered_double && no "needle- false hit" || ok
# custom register
r=$(mktemp); printf 'sha256:%s\n' "$(printf a | sha256sum | cut -d' ' -f1)" > "$r"
LIVE_EVIDENCE_DOUBLES=$r "$VERIFIER" "$FX/golden_good.json" 2>&1 | grep -q registered_double && ok || no "custom register not honoured"
rm -f "${r:?}"
echo "verify_live_evidence tests: pass=$pass fail=$fail"
[ "$fail" = 0 ]
