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
  if [ -n "$sub" ]; then case "${out#* class=}" in *"$sub"*) ;; *) no "$f reason want=$sub out=$out"; return;; esac; fi
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
expect bad_proc_vs_image 1 UNIT expected_kind_mismatch
expect bad_container_vs_exe 1 UNIT expected_kind_mismatch
expect bad_socket_vs_image 1 UNIT expected_kind_mismatch
expect bad_double_by_name_case 1 UNIT registered_double
expect bad_fp_bare_mismatch 1 UNIT fingerprint_mismatch
expect golden_fp_bare_match 0 LIVE
expect bad_renamed_interp 1 UNIT interpreter_script_missing
expect bad_timestamp_range 2 INVALID timestamp_not_rfc3339
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
# empty / comment-only register is BLIND (exit 2), a one-rule register is not
e=$(mktemp); printf '# only a comment\n\n' > "$e"
LIVE_EVIDENCE_DOUBLES=$e "$VERIFIER" "$FX/golden_good.json" >/dev/null 2>&1; [ $? = 2 ] && ok || no "comment-only register must exit 2"
: > "$e"; LIVE_EVIDENCE_DOUBLES=$e "$VERIFIER" "$FX/golden_good.json" >/dev/null 2>&1; [ $? = 2 ] && ok || no "empty register must exit 2"
printf 'name:zzz\n' > "$e"; LIVE_EVIDENCE_DOUBLES=$e "$VERIFIER" "$FX/golden_good.json" >/dev/null 2>&1; [ $? = 0 ] && ok || no "one-rule register must work"
# '?' is a single-character wildcard in name: rules
printf 'name:catalog-ap?\n' > "$e"; LIVE_EVIDENCE_DOUBLES=$e "$VERIFIER" "$FX/golden_good.json" 2>&1 | grep -q registered_double && ok || no "? glob not honoured"
printf 'name:catalog-ap??\n' > "$e"; LIVE_EVIDENCE_DOUBLES=$e "$VERIFIER" "$FX/golden_good.json" 2>&1 | grep -q registered_double && no "?? matched a 1-char tail" || ok
rm -f "${e:?}"
# custom register
r=$(mktemp); printf 'sha256:%s\n' "$(printf a | sha256sum | cut -d' ' -f1)" > "$r"
LIVE_EVIDENCE_DOUBLES=$r "$VERIFIER" "$FX/golden_good.json" 2>&1 | grep -q registered_double && ok || no "custom register not honoured"
rm -f "${r:?}"
# class-level fixtures (round 3): one normaliser, every expected key bound, socket == process, sha256 rules cover exe/image/script
expect bad_double_by_cmd_case 1 UNIT registered_double      # cmd: rules are case-insensitive (--mode FAKE_SERVER)
expect bad_double_by_image_case 1 UNIT registered_double    # image: rules are applied and case-insensitive (WireMock/...)
expect bad_double_by_script_hash 1 UNIT registered_double   # a sha256: rule matches the SCRIPT hash too
expect golden_upper_hash 0 LIVE                             # SHA256:/upper case hex normalises to the same identity
expect bad_fp_upper_prefix 1 UNIT fingerprint_mismatch      # SHA256: prefix must not escape the fingerprint binding
expect bad_socket_interp_script 1 UNIT identity_mismatch    # a socket answer is bound to expected.script_sha256 like a process
expect bad_socket_no_cmdline 1 UNIT endpoint_identity_incomplete
expect bad_expected_script_native 1 UNIT identity_mismatch  # expected names a script, the answer carries none: never LIVE
b=$(mktemp); printf 'name:foo/bar\n' > "$b"; LIVE_EVIDENCE_DOUBLES=$b "$VERIFIER" "$FX/golden_good.json" >/dev/null 2>&1; [ $? = 2 ] && ok || no "a rule that cannot match its own probe must make the register BLIND (exit 2)"
rm -f "${b:?}"
# round-5: one hash normalisation then strict ^[0-9a-f]{64}$ everywhere; code-loading options; strict types
for f in bad_hash_double_prefix bad_hash_nul bad_hash_number bad_hash_array bad_hash_junk bad_hash_space; do expect $f 2 INVALID hash_malformed; done
expect bad_fp_fullwidth 2 INVALID fingerprint_malformed
expect bad_fp_space 2 INVALID fingerprint_malformed
expect bad_runs_identical_string 2 INVALID runs
expect bad_runs_identical_missing 2 INVALID runs
expect bad_no_port 1 UNIT endpoint_identity_missing
expect bad_codeload_node 1 UNIT interpreter_code_loading_option
expect golden_node_plain 0 LIVE
expect bad_exe_path_slash 2 INVALID exe_path_malformed
expect bad_expected_unknown_key 2 INVALID expected_unknown_key
expect bad_pid_zero 1 UNIT answering_identity_missing
# round-6: ALLOW-LIST of interpreter options (the table docs/qa/live-evidence.interpreter-options.json); strict types
expect bad_fp_number 2 INVALID fingerprint_malformed
expect bad_fp_blank 2 INVALID fingerprint_malformed
expect bad_exe_relative 2 INVALID exe_path_malformed
expect bad_exe_nul 2 INVALID exe_path_malformed
expect bad_cmdline_number 2 INVALID cmdline_malformed
expect bad_script_path_number 2 INVALID script_path_malformed
expect bad_runs_n_float 2 INVALID runs_not_identical_or_missing
expect bad_runs_sig_number 2 INVALID runs_not_identical_or_missing
expect bad_runs_sig_blank 2 INVALID runs_not_identical_or_missing
expect bad_script_not_in_cmdline 1 UNIT script_not_in_cmdline
expect bad_env_injected 1 UNIT interpreter_env_injection
expect bad_env_unrecorded 1 UNIT env_unrecorded
expect bad_relative_script 1 UNIT script_not_in_cmdline
expect bad_fp_0x 2 INVALID fingerprint_malformed
expect bad_fp_sha_dash 2 INVALID fingerprint_malformed
expect bad_fp_zero_width 2 INVALID fingerprint_malformed
expect bad_endpoint_host_number 1 UNIT endpoint_identity_missing
expect bad_sh_c 1 UNIT interpreter_code_loading_option
expect bad_fold_sh 1 UNIT interpreter_code_loading_option
expect bad_shell_plus_opt 1 UNIT interpreter_option_unknown
expect bad_python_W_dot 1 UNIT interpreter_option_unknown
for f in bad_node_eval_eq bad_node_env_file bad_node_env_file_space bad_node_e bad_java_patch_module bad_php_c_ini bad_python_c bad_python_m bad_option_after_benign; do expect $f 1 UNIT interpreter_code_loading_option; done
expect bad_unknown_option 1 UNIT interpreter_option_unknown
expect bad_bun_preload 1 UNIT interpreter_family_unsupported
expect bad_deno_preload 1 UNIT interpreter_family_unsupported
expect golden_node_benign_options 0 LIVE
expect golden_python_benign_options 0 LIVE
expect golden_script_args_not_options 0 LIVE       # tokens after the script are script arguments, not interpreter options
# the option table is part of the instrument: unreadable / malformed / empty table is refused (exit 2), never "everything allowed"
t=$(mktemp)
o=$(LIVE_EVIDENCE_INTERP_OPTIONS=/nonexistent "$VERIFIER" "$FX/golden_node_plain.json" 2>&1); rc=$?; [ $rc = 2 ] && case $o in *"table unreadable"*) true;; *) false;; esac && ok || no "missing option table must exit 2 (unreadable): $o"
printf '{}' > "$t"; o=$(LIVE_EVIDENCE_INTERP_OPTIONS=$t "$VERIFIER" "$FX/golden_node_plain.json" 2>&1); rc=$?; [ $rc = 2 ] && case $o in *"malformed/empty"*) true;; *) false;; esac && ok || no "empty option table must exit 2 (malformed): $o"
printf 'nope' > "$t"; o=$(LIVE_EVIDENCE_INTERP_OPTIONS=$t "$VERIFIER" "$FX/golden_node_plain.json" 2>&1); rc=$?; [ $rc = 2 ] && case $o in *"malformed/empty"*) true;; *) false;; esac && ok || no "unparseable option table must exit 2 (malformed): $o"
jq '.families.node.benign[0].val = "sideways"' "$ROOT/docs/qa/live-evidence.interpreter-options.json" > "$t"; o=$(LIVE_EVIDENCE_INTERP_OPTIONS=$t "$VERIFIER" "$FX/golden_node_plain.json" 2>&1); rc=$?; [ $rc = 2 ] && case $o in *"malformed/empty"*) true;; *) false;; esac && ok || no "table entry with unknown val must exit 2 (malformed): $o"
rm -f "${t:?}"
# synthetic option table: entry shapes the shipped table does not exercise (next-valued long option with a value regex) must be refused by the CODE
t=$(mktemp); r=$(mktemp)
jq '.families.node.benign += [{"opt":"--nx","val":"next","re":"^[a-z]+$","ex":"abc"}]' "$ROOT/docs/qa/live-evidence.interpreter-options.json" > "$t"
for c in "node --nx=abc /srv/app/server.js:0:LIVE" "node --nx=ABC /srv/app/server.js:1:UNIT" "node --nx abc /srv/app/server.js:0:LIVE" "node --nx ABC /srv/app/server.js:1:UNIT" "node --nx -abc /srv/app/server.js:1:UNIT" "node --nx:1:UNIT"; do
  cmd=${c%%:*}; rest=${c#*:}; want=${rest%%:*}; wc=${rest#*:}
  jq --arg c "$cmd" '.answering.cmdline = $c' "$FX/golden_node_plain.json" > "$r"
  o=$(LIVE_EVIDENCE_INTERP_OPTIONS=$t "$VERIFIER" "$r" 2>&1); rc=$?
  { [ $rc = "$want" ] && case $o in *"class=$wc"*) true;; *) false;; esac; } && ok || no "synthetic table: [$cmd] want exit=$want class=$wc got rc=$rc $o"
done
rm -f "${t:?}" "${r:?}"
"$HERE/test_verifier_binding.sh" "$VERIFIER" >/dev/null 2>&1 && ok || no "verifier binding cases (script binding, java positional, synthetic table) must pass"
# a sha256: register rule also matches the container IMAGE DIGEST (golden_false_container is LIVE under the default register)
r=$(mktemp); printf 'sha256:%s\n' "$(jq -r .answering.image_digest "$FX/golden_false_container.json" | sed 's/^sha256://')" > "$r"
LIVE_EVIDENCE_DOUBLES=$r "$VERIFIER" "$FX/golden_false_container.json" 2>&1 | grep -q registered_double && ok || no "sha256 register rule must match an image digest"
# exact (no '*') image: rule matches the image reference exactly
printf 'image:%s\n' "$(jq -r .answering.image "$FX/bad_double_by_image_case.json" | tr A-Z a-z)" > "$r"
LIVE_EVIDENCE_DOUBLES=$r "$VERIFIER" "$FX/bad_double_by_image_case.json" 2>&1 | grep -q registered_double && ok || no "exact image: rule must match"
# a malformed sha256: rule makes the register BLIND
printf 'sha256:nothex\n' > "$r"; LIVE_EVIDENCE_DOUBLES=$r "$VERIFIER" "$FX/golden_good.json" >/dev/null 2>&1; [ $? = 2 ] && ok || no "malformed sha256 rule must be BLIND (exit 2)"
rm -f "${r:?}"
# table-driven matrix: kind x {interpreter,native,renamed} x every expected key
bash "$HERE/test_identity_matrix.sh" "$VERIFIER" "$TREG" && ok || no "identity matrix"
echo "verify_live_evidence tests: pass=$pass fail=$fail"
[ "$fail" = 0 ]
