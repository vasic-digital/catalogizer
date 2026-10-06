#!/usr/bin/env bash
# test_disk_headroom.sh - T001 (disk free-space headroom gate, constitution section 12.9 as consumer DATA).
# Drives scripts/containers/disk_headroom.sh through its real invocation path with `podman` and `df` shims
# first on PATH (the two oracles of free space), a scratch repository root, a scratch commit-turn grant and
# a stub host entry point (CPA_HOST_ENTRY). Paired mutations (copies of the script with one behaviour
# removed) must each make the turn-freeze fixtures FAIL; results go to the mutation record.
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUT="$HERE/../disk_headroom.sh"
CONF_REAL="$HERE/../disk_headroom.conf"
# The mutation record is repo evidence: written only when DISK_HEADROOM_MUTATION_RECORD is passed explicitly (default: scratch dir).
FAILS=0; PASSES=0
ok()   { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad()  { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required by this test"; exit 2; }

T="$(mktemp -d "${TMPDIR:-/tmp}/dh-test.XXXXXX")"
trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$T"' EXIT
MUT_RECORD="${DISK_HEADROOM_MUTATION_RECORD:-$T/disk-headroom-mutation.txt}"
SHIMS="$T/shims"; mkdir -p "$SHIMS"
cat >"$SHIMS/podman" <<'SH'
#!/usr/bin/env bash
# shim: only `podman info --format ...GraphRoot...` is answered; SHIM_PODMAN_SLEEP makes it hang (W3, exec so no orphaned child)
[ -n "${SHIM_PODMAN_SLEEP:-}" ] && exec sleep "$SHIM_PODMAN_SLEEP"
case "$*" in *GraphRoot*) echo "$SHIM_GRAPHROOT"; exit 0;; esac
exit 1
SH
cat >"$SHIMS/df" <<'SH'
#!/usr/bin/env bash
# STRICT shim (review r2 N1): answers only `df -B1 --output=avail <path>`; any other argument shape (for example
# --output=size, which would read total size instead of free space) gets garbage and a failing status.
if [ "$#" -ne 3 ] || [ "$1" != "-B1" ] || [ "$2" != "--output=avail" ]; then echo "df shim: unexpected arguments: $*" >&2; echo garbage; exit 1; fi
last="$3"
[ -n "${SHIM_DF_SLEEP:-}" ] && exec sleep "$SHIM_DF_SLEEP"
echo "Avail"
if [ "$last" = "$SHIM_GRAPHROOT" ]; then
  printf '%s\n' "$SHIM_DF_GRAPH"
  # M2: a valid-looking number followed by a failure, by a hang, or by a TERM-ignoring hang (graphroot call only): none of them may be believed
  [ -n "${SHIM_DF_FAIL_AFTER:-}" ] && exit 1
  [ -n "${SHIM_DF_HANG_AFTER:-}" ] && exec sleep "$SHIM_DF_HANG_AFTER"
  if [ -n "${SHIM_DF_IGNORE_TERM:-}" ]; then trap '' TERM; end=$((SECONDS + SHIM_DF_IGNORE_TERM)); while [ "$SECONDS" -lt "$end" ]; do sleep 1; done; fi
else printf '%s\n' "$SHIM_DF_REPO"; fi
SH
chmod +x "$SHIMS/podman" "$SHIMS/df"

mkconf() { printf 'min_free_bytes=%s\n' "$2" >"$1"; }
GRAPH="$T/graphroot"; mkdir -p "$GRAPH"
MIN=1000

# run_dh <script> <repo_root> <ev_dir> [extra env assignments...] -- <args...>; sets RC OUT ERR
run_dh() {
  local script="$1" root="$2" ev="$3"; shift 3
  local envs=()
  while [ "$1" != "--" ]; do envs+=("$1"); shift; done; shift
  RC=0
  env -u EVREC_TURN_RUN_ID PATH="$SHIMS:$PATH" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$root" EV="$ev" \
      DISK_HEADROOM_CONF="${CONF:-$T/conf}" "${envs[@]}" timeout 20 bash "$script" "$@" >"$T/o.txt" 2>"$T/e.txt" || RC=$?
  OUT="$(cat "$T/o.txt")"; ERR="$(cat "$T/e.txt")"
}

# =================== threshold / refusal logic (no grant file present) ===================
ROOT="$T/root0"; mkdir -p "$ROOT"; CONF="$T/conf"; mkconf "$CONF" "$MIN"
EV0="$T/ev0"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1000 --op-id pass1
check "above threshold: exit 0" "$RC" "0"
J="$EV0/disk/pass1.json"
check "above threshold: record written to EV/disk/<op_id>.json" "$([ -s "$J" ] && echo yes || echo no)" "yes"
check "record carries before value" "$(jq -r '[.filesystems[].free_before]|min' "$J" 2>/dev/null)" "5000"
check "record carries after value (free minus need)" "$(jq -r '[.filesystems[].free_after]|min' "$J" 2>/dev/null)" "4000"
check "record verdict pass" "$(jq -r '.verdict' "$J" 2>/dev/null)" "pass"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=2000 SHIM_DF_REPO=2000 -- --need 1000 --op-id edge_eq
check "free-need exactly at threshold: passes" "$RC" "0"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=1999 SHIM_DF_REPO=2000 -- --need 1000 --op-id edge_lt
check "free-need one byte below threshold (graphroot fs): refused non-zero" "$([ "$RC" -ne 0 ] && echo nz || echo zero)" "nz"
check "refusal reason disk_below_headroom on stderr" "$(echo "$ERR" | grep -c 'reason=disk_below_headroom')" "1"
check "refusal record carries reason" "$(jq -r '.reason' "$EV0/disk/edge_lt.json" 2>/dev/null)" "disk_below_headroom"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=1500 -- --need 1000 --op-id repo_low
check "repository filesystem below threshold: refused" "$(echo "$ERR" | grep -c 'reason=disk_below_headroom')" "1"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=garbage SHIM_DF_REPO=5000 -- --need 1000 --op-id unread
check "unreadable free-space value: refused non-zero" "$([ "$RC" -ne 0 ] && echo nz || echo zero)" "nz"
check "unreadable free-space: named reason, never treated as enough" "$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --op-id noneed
check "missing --need: refused as usage error (exit 2)" "$RC" "2"
CONF="$T/conf_pct"; printf 'min_free_bytes=10%%\n' >"$CONF"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id pct
check "percentage value in conf: refused as configuration error (exit 2)" "$RC" "2"
check "percentage refusal names reason" "$(echo "$ERR" | grep -c 'reason=config_percentage_refused')" "1"
CONF="$T/conf_none"; : >"$CONF"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id nokey
check "conf without min_free_bytes: refused (exit 2)" "$RC" "2"
CONF="$T/conf"
OD="$T/outdir"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_OUT_DIR="$OD" -- --need 1 --op-id viaoutdir
check "DISK_HEADROOM_OUT_DIR: record written there" "$([ -s "$OD/viaoutdir.json" ] && echo yes || echo no)" "yes"
check "DISK_HEADROOM_OUT_DIR: nothing written under EV/disk for that op" "$([ -e "$EV0/disk/viaoutdir.json" ] && echo yes || echo no)" "no"
# the committed conf: min_free_bytes is an absolute positive integer, never a percentage
mfb="$(grep -E '^min_free_bytes=' "$CONF_REAL" 2>/dev/null | head -1 | cut -d= -f2)"
case "$mfb" in ''|*[!0-9]*) bad "committed conf min_free_bytes is an absolute byte count (got '$mfb')";; *) ok "committed conf min_free_bytes is an absolute byte count ($mfb)";; esac
check "committed conf documents formula, inputs and measurement dates" "$(grep -c -E '^# (formula|input|measured)' "$CONF_REAL" 2>/dev/null | awk '{print ($1>=3)?"yes":"no"}')" "yes"


# =================== review T001 findings F1-F3, F7, F9: validated base-10 numbers, no fail-open ===================
GOOD=(SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000)
pass_has_two_fs() { jq -e '(.filesystems|length)==2 and ([.filesystems[].readable]|all)' "$1" >/dev/null 2>&1 && echo yes || echo no; }
check "pass record read exactly 2 readable filesystems" "$(pass_has_two_fs "$EV0/disk/pass1.json")" "yes"
no_pass() { # $1 label, $2 expected rc, $3 record path that must NOT say pass
  check "$1: exit $2" "$RC" "$2"
  check "$1: stdout never says pass" "$(echo "$OUT" | grep -c '^disk_headroom: pass')" "0"
  local v=""; [ -e "$3" ] && v="$(jq -r .verdict "$3" 2>/dev/null)"
  check "$1: no pass verdict recorded" "$([ "$v" = pass ] && echo 1 || echo 0)" "0"
}
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 08 --op-id f1a
no_pass "F1 --need 08 (invalid octal, leading zero)" 2 "$EV0/disk/f1a.json"
check "F1 --need 08: reason usage named" "$(echo "$ERR" | grep -c 'reason=usage')" "1"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=1099 SHIM_DF_REPO=1099 -- --need 0100 --op-id f2b
no_pass "F2b --need 0100 (octal misread would pass)" 2 "$EV0/disk/f2b.json"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 18446744073709551616 --op-id f2a
no_pass "F2a --need 2^64 (wraps to 0)" 2 "$EV0/disk/f2a.json"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 99999999999999999999 --op-id f2a20
no_pass "F2 --need 20 digits" 2 "$EV0/disk/f2a20.json"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 9223372036854775808 --op-id f2int
no_pass "F2 --need INT64_MAX+1" 2 "$EV0/disk/f2int.json"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 00 --op-id f2z
no_pass "F2 --need 00 (leading zeros refused)" 2 "$EV0/disk/f2z.json"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 0 --op-id need0
check "--need 0 is a valid base-10 value: passes" "$RC" "0"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 9223372036854775807 --op-id needmax
check "--need INT64_MAX: refused below headroom (rc 1), not a wrap" "$RC" "1"
CONF="$T/conf20"; mkconf "$CONF" 99999999999999999999
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=10 SHIM_DF_REPO=10 -- --need 0 --op-id f2c
no_pass "F2c conf min_free_bytes 20 digits (10 bytes free)" 2 "$EV0/disk/f2c.json"
check "F2c: reason config_not_integer" "$(echo "$ERR" | grep -c 'reason=config_not_integer')" "1"
CONF="$T/conf0100"; mkconf "$CONF" 0100
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 0 --op-id f2d
no_pass "F2d conf min_free_bytes with leading zero" 2 "$EV0/disk/f2d.json"
CONF="$T/confdup"; printf 'min_free_bytes=1\nmin_free_bytes=2\n' >"$CONF"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 0 --op-id f9dup
no_pass "F9 duplicate conf key refused" 2 "$EV0/disk/f9dup.json"
check "F9 duplicate conf key: reason named" "$(echo "$ERR" | grep -c 'reason=config_duplicate_key')" "1"
CONF="$T/conf"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=99999999999999999999 SHIM_DF_REPO=5000 -- --need 0 --op-id f2df
no_pass "F2 df value 20 digits is unreadable" 1 "$EV0/disk/f2df.json"
check "F2 df 20 digits: reason disk_free_unreadable" "$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=0500 SHIM_DF_REPO=5000 -- --need 0 --op-id f2dz
no_pass "F2 df value with leading zero is unreadable" 1 "$EV0/disk/f2dz.json"
# F3: option as last argument must refuse, never hang (run_dh wraps timeout 20; rc 124 = hang)
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need
check "F3 '--need' as last argument: refused usage (exit 2), no hang" "$RC" "2"
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 1 --op-id
check "F3 '--op-id' as last argument: refused usage (exit 2), no hang" "$RC" "2"
# F9: repeated --op-id; reason independent of filesystem order; empty graphroot
run_dh "$SUT" "$ROOT" "$EV0" "${GOOD[@]}" -- --need 1 --op-id a --op-id b
check "F9 repeated --op-id refused usage (exit 2)" "$RC" "2"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=10 SHIM_DF_REPO=garbage -- --need 1 --op-id ord1
check "F9 first fs below, second unreadable: reason disk_free_unreadable" "$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1"
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=garbage SHIM_DF_REPO=10 -- --need 1 --op-id ord2
check "F9 first fs unreadable, second below: reason disk_free_unreadable" "$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1"
GRAPH_SAVE="$GRAPH"; GRAPH=""
run_dh "$SUT" "$ROOT" "$EV0" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id nograph
check "podman reports no graphroot: refused (never pass on one filesystem)" "$([ "$RC" -eq 1 ] && echo refused || echo "rc=$RC")" "refused"
check "podman reports no graphroot: record is not pass" "$(jq -r .verdict "$EV0/disk/nograph.json" 2>/dev/null)" "refused"
GRAPH="$GRAPH_SAVE"

# =================== review r2 N1/N2/N11: core suite, also run against mutants ===================
core_suite() { # $1 = script under test, $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS
  local R="$T/core-$label"; rm -rf "$R"; mkdir -p "$R"
  local E="$R/ev"
  # N1: with the strict df shim a pass proves the script asks df for AVAILABLE space (-B1 --output=avail <path>)
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1000 --op-id n1
  check "[$label] N1 strict df shim (-B1 --output=avail): pass, exit 0" "$RC" "0"
  # N2: a df line with two tokens must not merge into one number; one token with stray whitespace/CR is accepted
  run_dh "$S" "$R" "$E" "SHIM_DF_GRAPH=5000 4000" SHIM_DF_REPO=5000 -- --need 1 --op-id n2a
  check "[$label] N2 df value '5000 4000': refused rc 1" "$RC" "1"
  check "[$label] N2 df value '5000 4000': reason disk_free_unreadable" "$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1"
  check "[$label] N2 df value '5000 4000': no pass record" "$(jq -r .verdict "$E/disk/n2a.json" 2>/dev/null)" "refused"
  run_dh "$S" "$R" "$E" "SHIM_DF_GRAPH= 5000 " "SHIM_DF_REPO=5000"$'\r' -- --need 1 --op-id n2b
  check "[$label] N2 single token with spaces and CR: accepted" "$RC" "0"
  # N11: --op-id length cap with a named refusal
  local id128 id129 id300
  id128="$(printf 'a%.0s' $(seq 1 128))"; id129="${id128}a"; id300="$(printf 'b%.0s' $(seq 1 300))"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id "$id128"
  check "[$label] N11 op-id of 128 characters: accepted" "$RC" "0"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id "$id129"
  check "[$label] N11 op-id of 129 characters: refused rc 2" "$RC" "2"
  check "[$label] N11 op-id of 129 characters: reason op_id_too_long" "$(echo "$ERR" | grep -c 'reason=op_id_too_long')" "1"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id "$id300"
  check "[$label] N11 op-id of 300 characters: refused rc 2 with the named reason" "$RC/$(echo "$ERR" | grep -c 'reason=op_id_too_long')" "2/1"
  check "[$label] N11 over-long op-id: nothing written for it" "$(ls "$E/disk" | grep -c "$id129")" "0"
  # I2 (round 3): op-id charset ^[A-Za-z0-9._-]{1,128}$, no '/', no leading '.', no explicit empty value: nothing is written ANYWHERE
  local bad_id
  for bad_id in "a/b" "/../../../escaped_c" "../escaped_d" "../../escaped_e" ".." "." ".hidden_i2"; do
    run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id "$bad_id"
    check "[$label] I2 op-id '$bad_id': refused rc 2 reason usage" "$RC/$(echo "$ERR" | grep -c 'reason=usage')" "2/1"
  done
  check "[$label] I2 refused op-ids: nothing named escaped_* written anywhere under the test tree" "$(find "$T" -name 'escaped_*' 2>/dev/null | wc -l | tr -d ' ')" "0"
  check "[$label] I2 refused op-ids: no hidden record in the record dir" "$(ls -A "$E/disk" | grep -c -E '^\.\.?json$|hidden_i2')" "0"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id ""
  check "[$label] I2 explicit empty --op-id: refused rc 2 reason usage (not silently replaced by a generated name)" "$RC/$(echo "$ERR" | grep -c 'reason=usage')" "2/1"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id "A.b_c-9"
  check "[$label] I2 legal op-id 'A.b_c-9' (dots, underscores, dashes inside): accepted" "$RC/$([ -s "$E/disk/A.b_c-9.json" ] && echo yes || echo no)" "0/yes"
  # N11: unique temporary name (mktemp): a stale <op>.json.tmp directory must not break the write; nothing is left behind
  mkdir -p "$E/disk/n11t.json.tmp"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id n11t
  check "[$label] N11 stale <op>.json.tmp present: still writes the record, exit 0" "$RC/$([ -s "$E/disk/n11t.json" ] && echo yes || echo no)" "0/yes"
  rmdir "$E/disk/n11t.json.tmp"
  run_dh "$S" "$R" "$E" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id n11u
  check "[$label] N11 after a write only <op>.json exists for that op (no temp residue)" "$(ls -A "$E/disk" | grep -c 'n11u')" "1"
  check "[$label] N11 no dot-temp residue anywhere in the record dir" "$(ls -A "$E/disk" | grep -c '^\.')" "0"
  # N11: 12 concurrent runs with one op-id leave exactly one valid record and no residue
  local i; for i in $(seq 1 12); do
    env PATH="$SHIMS:$PATH" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$R" EV="$E" DISK_HEADROOM_CONF="$T/conf" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 \
      timeout 20 bash "$S" --need 1 --op-id n11c >/dev/null 2>&1 &
  done; wait
  check "[$label] N11 concurrent same-op-id runs: one valid record" "$(jq -r .verdict "$E/disk/n11c.json" 2>/dev/null)/$(ls -A "$E/disk" | grep -c 'n11c')" "pass/1"
  TS_FAILS=$((FAILS-before))
}

# =================== W3: every external call is bounded (liveness, 11.4.232(C)) ===================
bound_suite() { # $1 = script under test, $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS
  local R="$T/bound-$label"; rm -rf "$R"; mkdir -p "$R"
  local t0 t1
  t0=$(date +%s)
  run_dh "$S" "$R" "$R/ev" SHIM_PODMAN_SLEEP=40 SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_CMD_TIMEOUT=2 -- --need 1 --op-id w3p
  t1=$(date +%s)
  check "[$label] W3 hung podman info: refused disk_free_unreadable rc 1 (not a kill rc 124)" "$RC/$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1/1"
  check "[$label] W3 hung podman info: gate returned within 15s" "$([ $((t1-t0)) -lt 15 ] && echo yes || echo no)" "yes"
  check "[$label] W3 hung podman info: record is not pass" "$(jq -r .verdict "$R/ev/disk/w3p.json" 2>/dev/null)" "refused"
  t0=$(date +%s)
  run_dh "$S" "$R" "$R/ev" SHIM_DF_SLEEP=40 SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_CMD_TIMEOUT=2 -- --need 1 --op-id w3d
  t1=$(date +%s)
  check "[$label] W3 hung df: refused disk_free_unreadable rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1/1"
  check "[$label] W3 hung df: gate returned within 15s" "$([ $((t1-t0)) -lt 15 ] && echo yes || echo no)" "yes"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_REAPER_TIMEOUT=0 -- --need 1 --op-id w3z
  check "[$label] W3 DISK_HEADROOM_REAPER_TIMEOUT=0 (GNU: no timeout) refused rc 2 with a named reason" "$RC/$(echo "$ERR" | grep -c 'reason=config_reaper_timeout')" "2/1"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_REAPER_TIMEOUT=abc -- --need 1 --op-id w3a
  check "[$label] W3 DISK_HEADROOM_REAPER_TIMEOUT=abc refused rc 2" "$RC/$(echo "$ERR" | grep -c 'reason=config_reaper_timeout')" "2/1"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_CMD_TIMEOUT=0 -- --need 1 --op-id w3c
  check "[$label] W3 DISK_HEADROOM_CMD_TIMEOUT=0 refused rc 2" "$RC/$(echo "$ERR" | grep -c 'reason=config_cmd_timeout')" "2/1"
  check "[$label] W3 refused timeout configs wrote no record" "$(ls "$R/ev/disk" 2>/dev/null | grep -c -E 'w3[zac]')" "0"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_REAPER_TIMEOUT=7 DISK_HEADROOM_CMD_TIMEOUT=9 -- --need 1 --op-id w3ok
  check "[$label] W3 valid positive timeouts: still passes" "$RC" "0"
  # M2 (round 3): bounded_out keeps NOTHING from a probe that failed or was stopped, even when it already printed a valid number
  run_dh "$S" "$R" "$R/ev" SHIM_DF_FAIL_AFTER=1 SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id m2f
  check "[$label] M2 df prints 5000 then exits 1: refused disk_free_unreadable rc 1 (output of a failed probe is not believed)" "$RC/$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1/1"
  check "[$label] M2 failed probe: record is not pass" "$(jq -r .verdict "$R/ev/disk/m2f.json" 2>/dev/null)" "refused"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_HANG_AFTER=40 DISK_HEADROOM_CMD_TIMEOUT=2 SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id m2h
  check "[$label] M2 df prints 5000 then hangs past the bound: refused disk_free_unreadable rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1/1"
  # the probe ignores TERM: only the KILL escalation (timeout -k 2) ends it; without -k the gate would wait for the probe's own end
  t0=$(date +%s)
  run_dh "$S" "$R" "$R/ev" SHIM_DF_IGNORE_TERM=30 DISK_HEADROOM_CMD_TIMEOUT=2 SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id m2k
  t1=$(date +%s)
  check "[$label] M2 TERM-ignoring df: refused disk_free_unreadable rc 1 (not rc 124 from the harness bound)" "$RC/$(echo "$ERR" | grep -c 'reason=disk_free_unreadable')" "1/1"
  check "[$label] M2 TERM-ignoring df: gate returned within 15s (KILL escalation, not the probe's own 30s)" "$([ $((t1-t0)) -lt 15 ] && echo yes || echo no)" "yes"
  TS_FAILS=$((FAILS-before))
}

# =================== commit-turn freeze ===================
# scratch repository root with a grant + a stub commit_turn_check.sh reached only via a stub host entry
turn_suite() { # $1 = script under test; $2 = label; sets TS_FAILS (assertion failures inside this suite)
  local S="$1" label="$2" before=$FAILS
  local R="$T/turn-$label"; rm -rf "$R"; mkdir -p "$R/.audit" "$R/scripts/release"
  local CT_LOG="$R.ct.log" CPA_LOG="$R.cpa.log"
  cat >"$R/scripts/release/commit_turn_check.sh" <<'STUB'
#!/usr/bin/env bash
# stub: logs each call; for --reap removes the grant only when the holder pid is dead or its cmdline differs
echo "$*" >>"$CT_LOG"
[ "${1:-}" = "--reap" ] || exit 2
g="$CT_ROOT/.audit/commit_turn.json"
[ -r "$g" ] || exit 0
pid="$(jq -r .pid "$g")"; rec="$(jq -r .cmdline "$g")"
if ! kill -0 "$pid" 2>/dev/null; then rm -f "$g"; exit 0; fi
cur="$(tr '\0' ' ' </proc/"$pid"/cmdline 2>/dev/null)"
[ "$cur" != "$rec" ] && rm -f "$g"
exit 0
STUB
  cat >"$R.cpa-host" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$CPA_LOG"
# dR2: like the real host entry, the reap script path is RELATIVE, so it only resolves when the caller's cwd is the repository root
if [ "$1 $2 $3" = "--exec-approved scripts/release/commit_turn_check.sh --reap" ]; then
  [ "$(pwd -P)" = "$(cd "$CT_ROOT" && pwd -P)" ] || { echo "cpa-host stub: cwd $(pwd -P) is not the repository root" >>"$CPA_LOG"; exit 126; }
  [ -f "scripts/release/commit_turn_check.sh" ] || exit 127
  exec bash "scripts/release/commit_turn_check.sh" --reap
fi
exit 2
STUB
  chmod +x "$R.cpa-host"
  # a live holder: a child of the test with a recorded cmdline
  sleep 600 & local HP=$!
  local HCMD; HCMD="$(tr '\0' ' ' </proc/$HP/cmdline)"
  local common=(SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 CT_ROOT="$R" CT_LOG="$CT_LOG" CPA_LOG="$CPA_LOG" CPA_HOST_ENTRY="$R.cpa-host")
  grant() { jq -nc --arg r "$1" --argjson p "$2" --arg c "$3" --arg s "$4" '{run_id:$r,paths_from_sha256:"x",pid:$p,cmdline:$c,started_at:$s}' >"$R/.audit/commit_turn.json"; }
  snap() { (cd "$R" && find . -type f -print0 | sort -z | xargs -0 sha256sum 2>/dev/null | sha256sum); }
  : >"$CT_LOG"; : >"$CPA_LOG"

  # (1) grant held by another run, holder live with recorded cmdline, started_at a year ago
  grant other-run "$HP" "$HCMD" "2025-10-01T00:00:00Z"
  local s0; s0="$(snap)"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t1
  check "[$label] live grant held by other run: refused non-zero" "$([ "$RC" -ne 0 ] && echo nz || echo zero)" "nz"
  check "[$label] refusal reason commit_turn_held" "$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1"
  check "[$label] nothing written under EV/disk" "$([ -e "$R/ev/disk/t1.json" ] && echo yes || echo no)" "no"
  check "[$label] scratch tree byte-unchanged" "$(snap)" "$s0"
  check "[$label] reaper called exactly once" "$(wc -l <"$CT_LOG" | tr -d ' ')" "1"
  check "[$label] every reap call went through --exec-approved (host entry log == reaper log)" "$(wc -l <"$CPA_LOG" | tr -d ' ')" "$(wc -l <"$CT_LOG" | tr -d ' ')"
  # (2) same grant, DISK_HEADROOM_OUT_DIR set: writes there and passes, grant not consulted
  : >"$CT_LOG"; : >"$CPA_LOG"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me DISK_HEADROOM_OUT_DIR="$R/.audit/out/x/disk" -- --need 1 --op-id t2
  check "[$label] OUT_DIR set: passes during a held turn" "$RC" "0"
  check "[$label] OUT_DIR set: record written there" "$([ -s "$R/.audit/out/x/disk/t2.json" ] && echo yes || echo no)" "yes"
  check "[$label] OUT_DIR set: reaper not called" "$(wc -l <"$CT_LOG" | tr -d ' ')" "0"
  # (3) caller's own run id equals the grant: writes
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=other-run -- --need 1 --op-id t3
  check "[$label] own run_id: writes EV/disk" "$([ -s "$R/ev/disk/t3.json" ] && echo yes || echo no)" "yes"
  # (4) no grant: writes
  rm -f "$R/.audit/commit_turn.json"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t4
  check "[$label] no grant file: writes EV/disk" "$([ -s "$R/ev/disk/t4.json" ] && echo yes || echo no)" "yes"
  # (3b) dR1: a caller with EVREC_TURN_RUN_ID UNSET (every build step that is not the turn holder) is never the holder
  : >"$CT_LOG"; : >"$CPA_LOG"
  grant other-run "$HP" "$HCMD" "2025-10-01T00:00:00Z"
  local s1; s1="$(snap)"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" -- --need 1 --op-id t3b
  check "[$label] dR1 EVREC_TURN_RUN_ID unset + held grant: refused commit_turn_held rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1/1"
  check "[$label] dR1 EVREC_TURN_RUN_ID unset + held grant: nothing written" "$([ -e "$R/ev/disk/t3b.json" ] && echo yes || echo no)" "no"
  check "[$label] dR1 EVREC_TURN_RUN_ID unset + held grant: tree byte-unchanged" "$(snap)" "$s1"
  # (3c) dR4: the freeze applies to REFUSAL records too: disk below headroom + held grant: commit_turn_held, no record at all
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=10 SHIM_DF_REPO=10 CT_ROOT="$R" CT_LOG="$CT_LOG" CPA_LOG="$CPA_LOG" CPA_HOST_ENTRY="$R.cpa-host" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t3c
  check "[$label] dR4 below headroom + held grant: refused commit_turn_held (not disk_below_headroom) rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$(echo "$ERR" | grep -c 'reason=disk_below_headroom')" "1/1/0"
  check "[$label] dR4 below headroom + held grant: no refusal record written" "$([ -e "$R/ev/disk/t3c.json" ] && echo yes || echo no)" "no"
  check "[$label] dR4 below headroom + held grant: tree byte-unchanged" "$(snap)" "$s1"
  # (3d) I2 (round 3): a grant whose run_id is empty, null, a number or missing names NO holder: it is unreadable for freeze purposes and the
  # gate refuses commit_turn_held whether EVREC_TURN_RUN_ID is unset, empty or equal to the (stringified) value; nothing is written
  local rj rjn=0 evenv
  for rj in '{"run_id":""}' '{"run_id":null}' '{"run_id":5}' '{"pid":1}' '{"run_id":true}'; do
    for evenv in UNSET EMPTY SAME; do
      rjn=$((rjn + 1)); printf '%s' "$rj" | jq -c '. + {pid:1,cmdline:"x"}' >"$R/.audit/commit_turn.json"
      local ea=(); case "$evenv" in EMPTY) ea=(EVREC_TURN_RUN_ID=);; SAME) ea=(EVREC_TURN_RUN_ID="$(printf '%s' "$rj" | jq -r '.run_id // empty')");; esac
      run_dh "$S" "$R" "$R/ev" "${common[@]}" CPA_HOST_ENTRY="$R.absent-host" ${ea[@]+"${ea[@]}"} -- --need 1 --op-id "t3d$rjn"
      check "[$label] I2 grant $rj, EVREC_TURN_RUN_ID $evenv: refused commit_turn_held rc 1, nothing written" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$([ -e "$R/ev/disk/t3d$rjn.json" ] && echo yes || echo no)" "1/1/no"
    done
  done
  rm -f "$R/.audit/commit_turn.json"
  # (5) holder exited: one reap clears the grant, then it writes
  sleep 0.01 & local DP=$!; wait "$DP" 2>/dev/null
  : >"$CT_LOG"; : >"$CPA_LOG"
  grant dead-run "$DP" "sleep 0.01 " "2026-10-01T00:00:00Z"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t5
  check "[$label] dead holder: reaped then written" "$([ -s "$R/ev/disk/t5.json" ] && echo yes || echo no)" "yes"
  check "[$label] dead holder: exit 0" "$RC" "0"
  check "[$label] dead holder: exactly one reap, via --exec-approved" "$(wc -l <"$CPA_LOG" | tr -d ' ')/$(wc -l <"$CT_LOG" | tr -d ' ')" "1/1"
  check "[$label] dead holder: --exec-approved line is the exact reap form" "$(head -1 "$CPA_LOG")" "--exec-approved scripts/release/commit_turn_check.sh --reap"
  # (6) no reaper script present (host entry fails): refusal stands
  grant other-run "$HP" "$HCMD" "2025-10-01T00:00:00Z"
  mv "$R/scripts/release/commit_turn_check.sh" "$R/ctc.off"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t6
  check "[$label] no reaper script: refusal stands" "$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1"
  mv "$R/ctc.off" "$R/scripts/release/commit_turn_check.sh"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" CPA_HOST_ENTRY="$R.absent-host" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t6b
  check "[$label] absent cpa-host counts as failing reaper: refusal stands" "$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1"
  check "[$label] absent cpa-host: nothing written" "$([ -e "$R/ev/disk/t6b.json" ] && echo yes || echo no)" "no"
  # (7) unreadable grant (mode 000) and non-JSON body: refused commit_turn_held, nothing written
  chmod 000 "$R/.audit/commit_turn.json"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t7
  check "[$label] unreadable grant: refused commit_turn_held" "$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1"
  check "[$label] unreadable grant: nothing written" "$([ -e "$R/ev/disk/t7.json" ] && echo yes || echo no)" "no"
  chmod 600 "$R/.audit/commit_turn.json"; printf 'not json {{{' >"$R/.audit/commit_turn.json"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t7b
  check "[$label] non-JSON grant: refused commit_turn_held" "$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1"
  check "[$label] non-JSON grant: nothing written" "$([ -e "$R/ev/disk/t7b.json" ] && echo yes || echo no)" "no"
  # (8) review r2 N3/D4: the reaper "succeeds" but leaves the grant unreadable or corrupt: still held, refused, nothing written
  cat >"$R.cpa-corrupt" <<'STUB'
#!/usr/bin/env bash
printf 'not json {{{' >"$CT_ROOT/.audit/commit_turn.json"; exit 0
STUB
  cat >"$R.cpa-unreadable" <<'STUB'
#!/usr/bin/env bash
chmod 000 "$CT_ROOT/.audit/commit_turn.json"; exit 0
STUB
  chmod +x "$R.cpa-corrupt" "$R.cpa-unreadable"
  grant other-run "$HP" "$HCMD" "2025-10-01T00:00:00Z"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" CPA_HOST_ENTRY="$R.cpa-corrupt" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t8a
  check "[$label] N3 reaper leaves a non-JSON grant: refused commit_turn_held rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1/1"
  check "[$label] N3 reaper leaves a non-JSON grant: nothing written" "$([ -e "$R/ev/disk/t8a.json" ] && echo yes || echo no)" "no"
  chmod 600 "$R/.audit/commit_turn.json" 2>/dev/null; grant other-run "$HP" "$HCMD" "2025-10-01T00:00:00Z"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" CPA_HOST_ENTRY="$R.cpa-unreadable" EVREC_TURN_RUN_ID=me -- --need 1 --op-id t8b
  check "[$label] N3 reaper leaves a mode-000 grant: refused commit_turn_held rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1/1"
  check "[$label] N3 reaper leaves a mode-000 grant: nothing written" "$([ -e "$R/ev/disk/t8b.json" ] && echo yes || echo no)" "no"
  chmod 600 "$R/.audit/commit_turn.json" 2>/dev/null
  kill "$HP" 2>/dev/null
  TS_FAILS=$((FAILS-before))
}

# =================== signal safety: a gate killed from outside must not orphan its reaper (11.4.263 / 11.4.196(D)) ===================
# The reaper stub records its own pid and the pid of a grandchild it spawns, then sleeps. The test signals ONLY the gate pid it
# started itself, then checks through /proc that neither recorded pid still lives. Leftovers are killed by those recorded pids only.
alive() { # $1 pid: 0 when the process exists and is not a zombie
  [ -n "$1" ] && [ -r "/proc/$1/stat" ] || return 1
  local st; st="$(sed -E 's/^[0-9]+ \(.*\) ([A-Za-z]) .*/\1/' "/proc/$1/stat" 2>/dev/null)"
  [ "$st" != Z ] && [ -n "$st" ]
}
signal_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS sig
  for sig in TERM INT; do
    local R="$T/sig-$label-$sig"; rm -rf "$R"; mkdir -p "$R/.audit"
    jq -nc '{run_id:"other-run",pid:1,cmdline:"x",started_at:"2025-10-01T00:00:00Z"}' >"$R/.audit/commit_turn.json"
    cat >"$R/slow-host" <<'STUB'
#!/usr/bin/env bash
echo $$ >"$SIG_DIR/stub.pid"
sleep 300 & echo $! >"$SIG_DIR/grandchild.pid"
wait
STUB
    chmod +x "$R/slow-host"
    env -u EVREC_TURN_RUN_ID PATH="$SHIMS:$PATH" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$R" EV="$R/ev" DISK_HEADROOM_CONF="$T/conf" \
      SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 SIG_DIR="$R" CPA_HOST_ENTRY="$R/slow-host" DISK_HEADROOM_REAPER_TIMEOUT=60 \
      python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$S" --need 1 --op-id sig >"$R/out.txt" 2>"$R/err.txt" &   # exec keeps the pid; an async job starts with SIGINT ignored, which a trap cannot undo, so restore the defaults a real caller delivers into
    local GP=$! i=0
    while [ ! -s "$R/grandchild.pid" ] && [ "$i" -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
    local SP GC; SP="$(cat "$R/stub.pid" 2>/dev/null)"; GC="$(cat "$R/grandchild.pid" 2>/dev/null)"
    check "[$label] $sig: reaper stub is running before the signal" "$(alive "$SP" && alive "$GC" && echo yes || echo no)" "yes"
    kill -"$sig" "$GP" 2>/dev/null
    local rc=0; i=0
    while alive "$GP" && [ "$i" -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
    wait "$GP" 2>/dev/null; rc=$?
    local want=143; [ "$sig" = INT ] && want=130
    check "[$label] $sig: gate exit status is 128+signal ($want)" "$rc" "$want"
    sleep 1
    check "[$label] $sig: reaper stub no longer alive (not orphaned)" "$(alive "$SP" && echo alive || echo gone)" "gone"
    check "[$label] $sig: reaper grandchild no longer alive (not orphaned)" "$(alive "$GC" && echo alive || echo gone)" "gone"
    check "[$label] $sig: nothing written under EV/disk" "$([ -e "$R/ev/disk/sig.json" ] && echo yes || echo no)" "no"
    # cleanup of leftovers by the exact recorded pids only (our own stub's processes)
    [ -n "$GC" ] && alive "$GC" && kill -KILL "$GC" 2>/dev/null
    [ -n "$SP" ] && alive "$SP" && kill -KILL "$SP" 2>/dev/null
  done
  # an unsignalled run keeps its status: a reaper that exits 3 still ends in the commit_turn_held refusal, rc 1
  local R2="$T/sig-$label-plain"; rm -rf "$R2"; mkdir -p "$R2/.audit"
  jq -nc '{run_id:"other-run",pid:1,cmdline:"x",started_at:"2025-10-01T00:00:00Z"}' >"$R2/.audit/commit_turn.json"
  printf '#!/usr/bin/env bash\nexit 3\n' >"$R2/rc3-host"; chmod +x "$R2/rc3-host"
  run_dh "$S" "$R2" "$R2/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 CPA_HOST_ENTRY="$R2/rc3-host" -- --need 1 --op-id plain
  check "[$label] unsignalled failing reaper: refusal status preserved (rc 1, commit_turn_held)" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1/1"
  TS_FAILS=$((FAILS-before))
}


# =================== signal safety of the bounded probes (podman info / df): a gate killed from outside must not orphan them ===================
# Shims record their own pid and a grandchild's pid, then hang. The test signals only the gate pid it started, then proves through
# /proc that the `timeout` between gate and shim, the shim and the grandchild are all gone. Leftovers are killed by recorded pids only.
mk_hang_shims() { # $1 = dir; $2 = which tool hangs (podman|df); $3 = "ign" to make `timeout` ignore TERM/INT/HUP
  local D="$1"; mkdir -p "$D/shims"
  cat >"$D/shims/hang" <<'STUB'
#!/usr/bin/env bash
echo $$ >"$SIG_DIR/stub.pid"
sleep 30 & echo $! >"$SIG_DIR/grandchild.pid"
wait
STUB
  cat >"$D/shims/podman" <<'STUB'
#!/usr/bin/env bash
if [ "${SHIM_HANG:-}" = podman ]; then exec "$(dirname "$0")/hang"; fi
case "$*" in *GraphRoot*) echo "$SHIM_GRAPHROOT"; exit 0;; esac
exit 1
STUB
  cat >"$D/shims/df" <<'STUB'
#!/usr/bin/env bash
[ "${SHIM_HANG:-}" = df ] && exec "$(dirname "$0")/hang"
echo Avail; echo 5000
STUB
  if [ "${3:-}" = ign ]; then
    cat >"$D/shims/timeout" <<'STUB'
#!/usr/bin/env bash
# stands in for a `timeout` that ignores TERM/INT/HUP (only KILL stops it); argv0 is rewritten so the cmdline starts with "timeout "
shift 3
exec -a timeout bash -c 'trap "" TERM INT HUP; "$@" & wait $!' timeout "$@"
STUB
  fi
  chmod +x "$D/shims/"*
}
ppid_of() { local s; IFS= read -r s <"/proc/$1/stat" 2>/dev/null || return 1; s="${s##*) }"; set -- $s; echo "$2"; }
# sig_one <script> <label> <dir> <hang tool> <signal> <ign|-> [probe]: runs one signalled gate; sets SO_RC SO_SP SO_GC SO_TP
sig_one() {
  local S="$1" label="$2" R="$3" tool="$4" sig="$5" ign="$6" i=0
  rm -rf "$R"; mkdir -p "$R/tmp" "$R/.audit"; mk_hang_shims "$R" "$tool" "$ign"
  local host=""
  if [ "$tool" = reaper ]; then   # the grant is held by another run, so the gate calls the (hanging) host entry point as its reaper
    jq -nc '{run_id:"other-run",pid:1,cmdline:"x",started_at:"2025-10-01T00:00:00Z"}' >"$R/.audit/commit_turn.json"; host="$R/shims/hang"
  fi
  env -u EVREC_TURN_RUN_ID PATH="$R/shims:$PATH" TMPDIR="$R/tmp" SHIM_HANG="$tool" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$R" EV="$R/ev" \
    DISK_HEADROOM_CONF="$T/conf" SIG_DIR="$R" DISK_HEADROOM_CMD_TIMEOUT=60 PROBE_CMD="$([ "$tool" = reaper ] && echo "" || echo "$tool")" \
    ${host:+CPA_HOST_ENTRY="$host"} DISK_HEADROOM_REAPER_TIMEOUT=60 \
    python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$S" --need 1 --op-id bsig >"$R/out.txt" 2>"$R/err.txt" &
  local GP=$!
  while [ ! -s "$R/grandchild.pid" ] && [ "$i" -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
  SO_SP="$(cat "$R/stub.pid" 2>/dev/null)"; SO_GC="$(cat "$R/grandchild.pid" 2>/dev/null)"; SO_TP="$(ppid_of "${SO_SP:-0}" 2>/dev/null)"
  [ "${PROBE:-0}" = 1 ] || kill -"$sig" "$GP" 2>/dev/null   # PROBE=1: the script signals itself inside the fork window
  i=0; while alive "$GP" && [ "$i" -lt 200 ]; do sleep 0.1; i=$((i + 1)); done
  if alive "$GP"; then kill -KILL "$GP" 2>/dev/null; SO_RC=still_running; else SO_RC=0; fi   # the shims hang for 300s: a gate still up after 20s did not react to the signal
  local w=0; wait "$GP" 2>/dev/null; w=$?; [ "$SO_RC" = still_running ] || SO_RC=$w
  sleep 1
}
sig_cleanup() { local p; for p in "$SO_GC" "$SO_SP" "$SO_TP"; do [ -n "$p" ] && [ "$p" != "$$" ] && alive "$p" && kill -KILL "$p" 2>/dev/null; done; return 0; }
sig_asserts() { # $1 label $2 name
  check "[$1] $2: shim was running before the signal" "$([ -n "$SO_SP" ] && [ -n "$SO_GC" ] && [ -n "$SO_TP" ] && echo yes || echo no)" "yes"
  check "[$1] $2: gate exit status is 128+signal ($WANT)" "$SO_RC" "$WANT"
  check "[$1] $2: timeout wrapper no longer alive" "$(alive "$SO_TP" && echo alive || echo gone)" "gone"
  check "[$1] $2: shim no longer alive (not orphaned)" "$(alive "$SO_SP" && echo alive || echo gone)" "gone"
  check "[$1] $2: shim grandchild no longer alive (not orphaned)" "$(alive "$SO_GC" && echo alive || echo gone)" "gone"
  check "[$1] $2: nothing written under EV/disk" "$([ -e "$RR_/ev/disk/bsig.json" ] && echo yes || echo no)" "no"
  check "[$1] $2: no output temp file left in TMPDIR" "$(ls -A "$RR_/tmp" | grep -c .)" "0"
  sig_cleanup
}
bound_signal_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS tool sig
  for tool in podman df; do
    for sig in TERM INT HUP; do
      RR_="$T/bsig-$label-$tool-$sig"; WANT=143; [ "$sig" = INT ] && WANT=130; [ "$sig" = HUP ] && WANT=129
      sig_one "$S" "$label" "$RR_" "$tool" "$sig" -
      sig_asserts "$label" "$tool $sig"
    done
  done
  TS_FAILS=$((FAILS-before))
}
# the `timeout` wrapper ignores TERM: the KILL fallback must stop it AND the chain it started (verified own pids only)
kill_chain_suite() {
  local S="$1" label="$2" before=$FAILS tool
  for tool in podman df; do
    RR_="$T/ksig-$label-$tool"; WANT=143
    sig_one "$S" "$label" "$RR_" "$tool" TERM ign
    sig_asserts "$label" "ignoring timeout, $tool hangs, TERM"
  done
  TS_FAILS=$((FAILS-before))
}

# a `timeout` that ignores TERM and has no child at all (wedged): only the KILL fallback can stop it
wedged_suite() {
  local S="$1" label="$2" before=$FAILS R="$T/wedge-$2" i=0
  rm -rf "$R"; mkdir -p "$R/tmp" "$R/shims"
  cat >"$R/shims/timeout" <<'STUB'
#!/usr/bin/env bash
exec -a timeout bash -c 'trap "" TERM INT HUP; echo $$ >"$SIG_DIR/stub.pid"; mkfifo "$SIG_DIR/ff"; exec 3<>"$SIG_DIR/ff"; while :; do read -r -t 1 -u 3 || :; done' timeout "$@"
STUB
  chmod +x "$R/shims/timeout"
  env -u EVREC_TURN_RUN_ID PATH="$R/shims:$PATH" TMPDIR="$R/tmp" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$R" EV="$R/ev" \
    DISK_HEADROOM_CONF="$T/conf" SIG_DIR="$R" DISK_HEADROOM_CMD_TIMEOUT=60 \
    python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$S" --need 1 --op-id wedge >"$R/out.txt" 2>"$R/err.txt" &
  local GP=$! TP
  while [ ! -s "$R/stub.pid" ] && [ "$i" -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
  TP="$(cat "$R/stub.pid" 2>/dev/null)"
  check "[$label] wedged timeout: running before the signal, a child of the gate" "$(alive "$TP" && [ "$(ppid_of "$TP")" = "$GP" ] && echo yes || echo no)" "yes"
  kill -TERM "$GP" 2>/dev/null
  i=0; while alive "$GP" && [ "$i" -lt 150 ]; do sleep 0.1; i=$((i + 1)); done
  local rc=0; if alive "$GP"; then kill -KILL "$GP" 2>/dev/null; rc=still_running; fi
  local w=0; wait "$GP" 2>/dev/null; w=$?; [ "$rc" = still_running ] || rc=$w
  check "[$label] wedged timeout: gate exit status is 143" "$rc" "143"
  check "[$label] wedged timeout: stopped by the KILL fallback" "$(alive "$TP" && echo alive || echo gone)" "gone"
  [ -n "$TP" ] && alive "$TP" && kill -KILL "$TP" 2>/dev/null
  TS_FAILS=$((FAILS-before))
}
# fork-to-$! window: a copy of the script that signals ITSELF right after the background launch, before its pid is stored in a variable
window_suite() {
  local S="$1" label="$2" before=$FAILS P="$T/probe-$2.sh" tool
  sed -E '/# MUT:pid-assign$/i\  [ "${1:-}" = "${PROBE_CMD:-}" ] \&\& kill -TERM $$' "$S" >"$P"
  if cmp -s "$P" "$S"; then bad "[$label] fork-window probe changed nothing (marker missing in script)"; fi
  for tool in podman df reaper; do
    RR_="$T/wsig-$label-$tool"; WANT=143
    PROBE=1 sig_one "$P" "$label" "$RR_" "$tool" TERM -
    sig_asserts "$label" "fork window, $tool hangs"
  done
  TS_FAILS=$((FAILS-before))
}


# M5 (round 3): a missing tool is refused with its own named reason (rc 2), never reported as an unreadable free-space or a bad config key
dep_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS tool R="$T/dep-$2" BASHB TOUTB
  BASHB="$(command -v bash)"; TOUTB="$(command -v timeout)"
  rm -rf "$R"; mkdir -p "$R/ev"
  for tool in timeout grep mktemp; do
    local B="$R/bin-$tool"; mkdir -p "$B"
    local t; for t in jq dirname cut tr date mkdir chmod mv rm sleep cat grep timeout mktemp df podman env; do
      [ "$t" = "$tool" ] && continue
      if [ "$t" = podman ] || [ "$t" = df ]; then ln -s "$SHIMS/$t" "$B/$t"; else ln -s "$(command -v "$t")" "$B/$t"; fi
    done
    RC=0
    env -i PATH="$B" HOME="$HOME" SHIM_GRAPHROOT="$GRAPH" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 DISK_HEADROOM_REPO_ROOT="$R" EV="$R/ev" DISK_HEADROOM_CONF="$T/conf" \
      "$TOUTB" 20 "$BASHB" "$S" --need 1 --op-id dep >"$R/o.txt" 2>"$R/e.txt" || RC=$?
    check "[$label] M5 '$tool' missing: refused rc 2 with reason=dependency_missing naming it" "$RC/$(grep -c "reason=dependency_missing $tool" "$R/e.txt")" "2/1"
    check "[$label] M5 '$tool' missing: nothing written" "$([ -e "$R/ev/disk/dep.json" ] && echo yes || echo no)" "no"
  done
  TS_FAILS=$((FAILS-before))
}

# M1 (round 3): a TERM that lands while the record is being written must not leave the temp file in the (tracked) evidence directory
write_signal_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS R="$T/wsig2-$2" i=0 JQ
  JQ="$(command -v jq)"; rm -rf "$R"; mkdir -p "$R/shims" "$R/ev"
  printf '#!/usr/bin/env bash\ncase "$*" in *disk-headroom/1*) : >"$SIG_DIR/jq.started"; sleep 3;; esac\nexec "%s" "$@"\n' "$JQ" >"$R/shims/jq"; chmod +x "$R/shims/jq"
  env -u EVREC_TURN_RUN_ID PATH="$R/shims:$SHIMS:$PATH" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$R" EV="$R/ev" DISK_HEADROOM_CONF="$T/conf" \
    SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 SIG_DIR="$R" \
    python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$S" --need 1 --op-id wsig >"$R/out.txt" 2>"$R/err.txt" &
  local GP=$!
  while [ ! -e "$R/jq.started" ] && [ "$i" -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
  check "[$label] M1 record write in progress before the signal" "$([ -e "$R/jq.started" ] && echo yes || echo no)" "yes"
  kill -TERM "$GP" 2>/dev/null
  i=0; while alive "$GP" && [ "$i" -lt 150 ]; do sleep 0.1; i=$((i + 1)); done
  local rc=0; if alive "$GP"; then kill -KILL "$GP" 2>/dev/null; rc=still_running; fi
  local w=0; wait "$GP" 2>/dev/null; w=$?; [ "$rc" = still_running ] || rc=$w
  sleep 1
  check "[$label] M1 TERM during the record write: gate exit status 143" "$rc" "143"
  check "[$label] M1 TERM during the record write: no temp residue in the record dir" "$(ls -A "$R/ev/disk" 2>/dev/null | grep -c '^\.')" "0"
  check "[$label] M1 TERM during the record write: no partial record" "$(ls -A "$R/ev/disk" 2>/dev/null | grep -c 'wsig')" "0"
  TS_FAILS=$((FAILS-before))
}

# I1 (round 3): a signal in the fork-to-exec window. strace delays ONLY the execve of `timeout` (a stand-in for a descheduled child), so the
# background child is observably still the gate's own image when the signal lands. It must still be stopped: after the delay it must NOT exec.
children_of() { # $1 pid: its children, from the kernel's own list (fast), else by scanning /proc
  local d p
  if [ -r "/proc/$1/task/$1/children" ]; then tr ' ' '\n' <"/proc/$1/task/$1/children" 2>/dev/null | grep -v '^$'; return 0; fi
  for d in /proc/[0-9]*; do p="${d#/proc/}"; [ "$(ppid_of "$p" 2>/dev/null)" = "$1" ] && echo "$p"; done; return 0
}
cmdline_of() { tr '\0' ' ' 2>/dev/null <"/proc/$1/cmdline"; }
preexec_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS tool TO
  TS_FAILS=0
  if ! command -v strace >/dev/null 2>&1 || ! strace -qq -o /dev/null -e trace=execve true >/dev/null 2>&1; then
    ok "[$label] I1 fork-to-exec fixture SKIPPED with reason: strace is not installed or not allowed to trace here (UNCONFIRMED on this host)"; return 0
  fi
  TO="$(command -v timeout)"
  for tool in podman reaper; do
    local R="$T/pre-$label-$tool" i=0 CP="" GP="" SP
    rm -rf "$R"; mkdir -p "$R/tmp" "$R/.audit"; mk_hang_shims "$R" podman
    # Which exec is held: the probes run `timeout` from the gate's cwd, the reaper subshell runs it after `cd "$ROOT_DIR"`. For the reaper a RELATIVE
    # PATH entry (rtbin, present only below $ROOT_DIR) makes that exec, and only that one, name a different file ($R/rtbin/timeout, a wrapper), which
    # is the one strace is told to hold.
    local host="" held="$TO" rtp=""
    if [ "$tool" = reaper ]; then
      mkdir -p "$R/rtbin"; printf '#!/usr/bin/env bash\nexec "%s" "$@"\n' "$TO" >"$R/rtbin/timeout"; chmod +x "$R/rtbin/timeout"; held="$R/rtbin/timeout"; rtp="rtbin:"
      jq -nc '{run_id:"other-run",pid:1,cmdline:"x",started_at:"2025-10-01T00:00:00Z"}' >"$R/.audit/commit_turn.json"; host="$R/shims/hang"; fi
    env -u EVREC_TURN_RUN_ID PATH="${rtp}$R/shims:$PATH" TMPDIR="$R/tmp" SHIM_HANG="$([ "$tool" = reaper ] && echo none || echo podman)" SHIM_GRAPHROOT="$GRAPH" DISK_HEADROOM_REPO_ROOT="$R" EV="$R/ev" \
      DISK_HEADROOM_CONF="$T/conf" SIG_DIR="$R" DISK_HEADROOM_CMD_TIMEOUT=60 ${host:+CPA_HOST_ENTRY="$host"} DISK_HEADROOM_REAPER_TIMEOUT=60 \
      strace -f -qq -o /dev/null -P "$held" -e trace=execve -e inject=execve:delay_enter=10000000 \
      bash "$S" --need 1 --op-id pre >"$R/out.txt" 2>"$R/err.txt" &
    SP=$!
    while [ -z "$GP" ] && [ "$i" -lt 100 ]; do GP="$(children_of "$SP" | head -1)"; sleep 0.1; i=$((i + 1)); done
    # the pre-exec child: a child of the gate whose command line is still the gate's own, seen at two instants 0.7 s apart (a command
    # substitution's fork is gone within milliseconds, the delayed one is held for 4 s)
    # gcmd is taken only once the child has exec'd the gate (its command line starts with `bash <script under test> `): under load strace's child still shows
    # strace's own image for a while, and a read taken then matches no pre-exec child (a false red). Bounded, 10 s; if never seen, gcmd stays
    # the last read and the observation below fails honestly.
    local gcmd p1=""; i=0
    while [ "$i" -lt 100 ]; do gcmd="$(cmdline_of "$GP")"; case "$gcmd" in "bash $S "*) break ;; esac; sleep 0.1; i=$((i + 1)); done
    i=0
    while [ -z "$CP" ] && [ "$i" -lt 100 ]; do
      p1="$(for c in $(children_of "$GP"); do [ "$(cmdline_of "$c")" = "$gcmd" ] && echo "$c"; done | head -1)"
      sleep 0.4
      [ -n "$p1" ] && [ "$(cmdline_of "$p1")" = "$gcmd" ] && CP="$p1"
      i=$((i + 1))
    done
    check "[$label] I1 $tool: a pre-exec child (still the gate's own image) is observed" "$([ -n "$CP" ] && echo yes || echo no)" "yes"
    local t0 t1 lat; t0=$(date +%s%N)
    kill -TERM "$GP" 2>/dev/null
    i=0; while alive "$GP" && [ "$i" -lt 300 ]; do sleep 0.05; i=$((i + 1)); done
    t1=$(date +%s%N); lat=$(( (t1 - t0) / 1000000 ))
    local gate_up=no; alive "$GP" && gate_up=yes
    sleep 12   # longer than the 10 s exec delay: a surviving child would now have exec'd `timeout` and started the hang shim
    # a pre-exec child is KILLed at once; falling back to the TERM-then-wait path would hold the gate for the 3 s TERM wait
    check "[$label] I1 $tool: the gate stops promptly after the signal (under 2500 ms; the TERM-wait path takes at least 3000 ms)" "$([ "$lat" -lt 2500 ] && echo yes || echo "no (${lat} ms)")" "yes"
    check "[$label] I1 $tool: gate stopped by the signal (exited, said so on stderr)" "$gate_up/$(grep -c 'interrupted by SIGTERM' "$R/err.txt")" "no/1"
    check "[$label] I1 $tool: the pre-exec child is gone (not orphaned)" "$([ -n "$CP" ] && alive "$CP" && echo alive || echo gone)" "gone"
    check "[$label] I1 $tool: the delayed child never exec'd (hang shim never started)" "$([ -e "$R/stub.pid" ] && echo started || echo never)" "never"
    check "[$label] I1 $tool: nothing written under EV/disk" "$([ -e "$R/ev/disk/pre.json" ] && echo yes || echo no)" "no"
    local q; for q in "$(cat "$R/grandchild.pid" 2>/dev/null)" "$(cat "$R/stub.pid" 2>/dev/null)" "$CP"; do [ -n "$q" ] && alive "$q" && kill -KILL "$q" 2>/dev/null; done
    wait "$SP" 2>/dev/null
  done
  TS_FAILS=$((FAILS-before))
}

# own-ness guards, unit level: the helper functions are extracted from the script under test and run with a stub `kill` that logs, so
# the test proves what would be signalled without signalling anything. Targets are this test's own children only.
own_suite() { # $1 script under test; $2 label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS F="$T/own-$2.fn" R="$T/own-$2"; mkdir -p "$R"
  printf '#!/usr/bin/env bash\nexec -a timeout "$LOOPBASH" "$0"\n' >"$R/execloop.sh"
  sed -n '/^proc_fields() {/,/^stop_chain() {/p' "$S" | sed '$d' >"$F"
  [ -s "$F" ] || bad "[$label] own-ness helpers not found in the script under test"
  local out
  # run as its own bash process: the helpers compare a parent pid with $$, which must be the real parent of the test's children
  {
    echo 'F="$1"; R="$2"'
    cat <<'UNIT'
SNAP=()
# shellcheck disable=SC1090
. "$F"
kill() { echo "kill $*" >>"$R/kill.log"; }
: >"$R/kill.log"
r() { "$@" && echo yes || echo no; }
echo "pid1 $(r own_child 1)"; echo "empty $(r own_child '')"; echo "alpha $(r own_child abc)"; echo "zero $(r own_child 0)"
timeout 30 sleep 30 & OWN=$!
sleep 30 & PLAIN=$!
bash -c 'exec -a timeout sleep 30 & echo $!' >"$R/foreign.pid"; FOREIGN="$(cat "$R/foreign.pid")"
timeout 30 bash -c 'sleep 30 & wait' & TREE=$!
( sleep 30; true ) & PRE=$!   # a forked subshell has not exec'd yet: its command line is still this shell's own (the fork-to-exec window, I1)
bash -c 'sleep 30 & echo $!' >"$R/foreign2.pid"; FOREIGN2="$(cat "$R/foreign2.pid")"
sleep 0.5
echo "own-timeout-child $(r own_child "$OWN")"
echo "own-nontimeout-child $(r own_child "$PLAIN")"
echo "foreign-timeout-named $(r own_child "$FOREIGN")"
echo "preexec-own-image-child $(r own_child "$PRE")"
SAVE_SELF="$SELF_CMD"; SELF_CMD="sleep 30 "; echo "foreign-same-image-not-our-child $(r own_child "$FOREIGN2")"; SELF_CMD="$SAVE_SELF"
snapshot_descendants "$TREE"; echo "tree-descendants ${#SNAP[@]}"; LEFT=("${SNAP[@]}"); snapshot_descendants "$OWN"; LEFT+=("${SNAP[@]}")
# R1 (round 4): a child INSIDE execve reads an EMPTY cmdline. A shell function `tr` stands in for the kernel window: it answers empty for the cmdline
# of ONE pid ($EMPTYPID) and delegates everything else, so the fixture is deterministic and works against any version of the script.
tr() { local f; f="$(readlink /proc/self/fd/0 2>/dev/null)"; if [ "$f" = "/proc/${EMPTYPID:-0}/cmdline" ]; then cat >/dev/null; return 0; fi; command tr "$@"; }
EMPTYPID="$OWN"; echo "empty-cmdline-own-child $(r own_child "$OWN")"
EMPTYPID="$FOREIGN2"; echo "empty-cmdline-foreign-not-ours $(r own_child "$FOREIGN2")"
EMPTYPID="$OWN"; SAVE_START="$SELF_START"; SELF_START=99999999999999; echo "empty-cmdline-older-than-self-not-ours $(r own_child "$OWN")"; SELF_START="$SAVE_START"
EMPTYPID="$OWN"; SAVE_START="$SELF_START"; SELF_START=""; echo "empty-cmdline-no-self-start-not-ours $(r own_child "$OWN")"; SELF_START="$SAVE_START"
EMPTYPID="$OWN"; echo "empty-cmdline-is-preexec $(r pre_exec_child "$OWN")"
EMPTYPID=0; echo "timeout-child-is-not-preexec $(r pre_exec_child "$OWN")"
unset -f tr
# R1 (round 4): a live own child that re-execs ITSELF in a loop (argv0 "timeout") spends a few percent of its life inside execve; own_child must NEVER
# answer "not ours" for it, whichever instant the read lands on
export LOOPBASH; LOOPBASH="$(command -v bash)"   # NOT $BASH: bash derives that from argv[0], which the loop sets to "timeout"
( exec -a timeout "$LOOPBASH" "$R/execloop.sh" ) & LOOP=$!
sleep 0.5
NOTS=0; EMPTIES=0; i=0
while [ "$i" -lt 600 ]; do own_child "$LOOP" || NOTS=$((NOTS + 1)); [ -z "$(command tr '\0' ' ' 2>/dev/null </proc/$LOOP/cmdline)" ] && EMPTIES=$((EMPTIES + 1)); i=$((i + 1)); done
echo "execloop-not-ours $NOTS"; echo "execloop-empty-reads-observed $EMPTIES"
builtin kill -KILL "$LOOP" 2>/dev/null
SNAP=("$OWN:1"); snap_signal TERM; echo "wrong-starttime-signals $(grep -c . "$R/kill.log")"
proc_fields 1; P1START="$PF_START"
SNAP=("1:$P1START"); snap_signal TERM; echo "pid1-signals $(grep -c . "$R/kill.log")"
SNAP=("0:1"); snap_signal TERM
proc_fields "$OWN"; SNAP=("$OWN:$PF_START"); snap_signal TERM; echo "verified-signals $(grep -c . "$R/kill.log")"
for p in $OWN $PLAIN $FOREIGN $FOREIGN2 $PRE $TREE; do builtin kill -KILL "$p" 2>/dev/null; done
for e in "${LEFT[@]}"; do builtin kill -KILL "${e%%:*}" 2>/dev/null; done
  
UNIT
  } >"$R/unit.sh"
  out="$(bash "$R/unit.sh" "$F" "$R" 2>&1)"
  chk() { check "[$label] own-ness: $1" "$(printf '%s\n' "$out" | sed -n "s/^$1 //p")" "$2"; }
  chk pid1 no; chk empty no; chk alpha no; chk zero no
  chk own-timeout-child yes; chk own-nontimeout-child no; chk foreign-timeout-named no
  chk preexec-own-image-child yes; chk foreign-same-image-not-our-child no
  chk tree-descendants 2
  chk empty-cmdline-own-child yes; chk empty-cmdline-foreign-not-ours no; chk empty-cmdline-older-than-self-not-ours no
  chk empty-cmdline-no-self-start-not-ours no; chk empty-cmdline-is-preexec yes; chk timeout-child-is-not-preexec no
  chk execloop-not-ours 0
  chk wrong-starttime-signals 0; chk pid1-signals 0; chk verified-signals 1
  TS_FAILS=$((FAILS-before))
}


# =================== round 4 (WF3-REVIEW R2, R3, R4, R8) ===================
# R2 + R3: the commit-turn freeze holds whenever the grant cannot be PROVEN absent, and one decision drives both the freeze and the record path
freeze_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS R="$T/frz-$2" HP HCMD
  chmod -R u+rwx "${R:?}" 2>/dev/null; rm -rf "${R:?}"; mkdir -p "$R/.audit"
  sleep 600 & HP=$!; HCMD="$(tr '\0' ' ' </proc/$HP/cmdline)"
  local common=(SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 CPA_HOST_ENTRY="$R.absent-host")
  local g='{"run_id":"other-run","paths_from_sha256":"x","pid":1,"cmdline":"x","started_at":"2025-10-01T00:00:00Z"}'
  # control: a readable .audit without a grant: no grant is PROVEN, the record is written (golden-FALSE: the freeze must not refuse a free turn)
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r2ctl
  check "[$label] R2 control: readable .audit without a grant: writes, exit 0" "$RC/$([ -s "$R/ev/disk/r2ctl.json" ] && echo yes || echo no)" "0/yes"
  # control: no .audit at all: provably no grant, writes
  rmdir "$R/.audit"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r2ctl2
  check "[$label] R2 control: no .audit directory at all: writes, exit 0" "$RC/$([ -s "$R/ev/disk/r2ctl2.json" ] && echo yes || echo no)" "0/yes"
  mkdir -p "$R/.audit"
  # (a) a held grant behind an UNTRAVERSABLE .audit: stat fails with EACCES, `test -e` is false, yet the turn is held
  printf '%s' "$g" >"$R/.audit/commit_turn.json"; chmod 000 "$R/.audit"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r2a
  chmod 700 "$R/.audit"
  check "[$label] R2 untraversable .audit (mode 000): refused commit_turn_held rc 1, nothing written" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$([ -e "$R/ev/disk/r2a.json" ] && echo yes || echo no)" "1/1/no"
  # (b) the grant path is a DANGLING symlink
  rm -f "$R/.audit/commit_turn.json"; ln -s "$R/.audit/nowhere" "$R/.audit/commit_turn.json"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r2b
  check "[$label] R2 grant is a dangling symlink: refused commit_turn_held rc 1, nothing written" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$([ -e "$R/ev/disk/r2b.json" ] && echo yes || echo no)" "1/1/no"
  rm -f "$R/.audit/commit_turn.json"
  # (c) .audit is a regular file (ENOTDIR): the grant cannot be reached
  rmdir "$R/.audit"; : >"$R/.audit"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r2c
  check "[$label] R2 .audit is a regular file: refused commit_turn_held rc 1, nothing written" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$([ -e "$R/ev/disk/r2c.json" ] && echo yes || echo no)" "1/1/no"
  rm -f "$R/.audit"; mkdir -p "$R/.audit"
  # (d) .audit is a dangling symlink
  rmdir "$R/.audit"; ln -s "$R/nowhere-dir" "$R/.audit"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r2d
  check "[$label] R2 .audit is a dangling symlink: refused commit_turn_held rc 1, nothing written" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$([ -e "$R/ev/disk/r2d.json" ] && echo yes || echo no)" "1/1/no"
  rm -f "$R/.audit"; mkdir -p "$R/.audit"
  # R3: a set-but-EMPTY DISK_HEADROOM_OUT_DIR is "not in use" for the freeze AND for the record path: a held turn refuses, nothing is written anywhere
  jq -nc --arg c "$HCMD" --argjson p "$HP" '{run_id:"other-run",paths_from_sha256:"x",pid:$p,cmdline:$c,started_at:"2025-10-01T00:00:00Z"}' >"$R/.audit/commit_turn.json"
  local sn0; sn0="$(cd "$R" && find . -type f -print0 | sort -z | xargs -0 sha256sum 2>/dev/null | sha256sum)"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" DISK_HEADROOM_OUT_DIR= EVREC_TURN_RUN_ID=me -- --need 1 --op-id r3a
  check "[$label] R3 empty DISK_HEADROOM_OUT_DIR + held grant: refused commit_turn_held rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1/1"
  check "[$label] R3 empty DISK_HEADROOM_OUT_DIR + held grant: nothing written anywhere (tree byte-unchanged)" "$([ -e "$R/ev/disk/r3a.json" ] && echo yes || echo no)/$([ "$(cd "$R" && find . -type f -print0 | sort -z | xargs -0 sha256sum 2>/dev/null | sha256sum)" = "$sn0" ] && echo same || echo changed)" "no/same"
  run_dh "$S" "$R" "$R/ev" "${common[@]}" DISK_HEADROOM_OUT_DIR= -- --need 1 --op-id r3b
  check "[$label] R3 empty DISK_HEADROOM_OUT_DIR, EVREC_TURN_RUN_ID unset + held grant: refused commit_turn_held rc 1" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')/$([ -e "$R/ev/disk/r3b.json" ] && echo yes || echo no)" "1/1/no"
  # golden-FALSE: a non-empty OUT_DIR still bypasses the freeze (CPA's own record path), and the record lands there
  run_dh "$S" "$R" "$R/ev" "${common[@]}" DISK_HEADROOM_OUT_DIR="$R/.audit/out/disk" EVREC_TURN_RUN_ID=me -- --need 1 --op-id r3c
  check "[$label] R3 non-empty DISK_HEADROOM_OUT_DIR + held grant: writes there, exit 0" "$RC/$([ -s "$R/.audit/out/disk/r3c.json" ] && echo yes || echo no)" "0/yes"
  kill "$HP" 2>/dev/null
  TS_FAILS=$((FAILS-before))
}

# R8: whitespace INSIDE the threshold value is refused, not merged; leading/trailing whitespace (tabs, CR of a CRLF file) is trimmed
conf_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS R="$T/cnf-$2" SAVE_CONF="${CONF:-}" v n=0
  rm -rf "${R:?}"; mkdir -p "$R"
  for v in '100 0' '1 000' '10 # comment' '1	000'; do
    n=$((n + 1)); CONF="$R/c$n"; printf 'min_free_bytes=%s\n' "$v" >"$CONF"
    run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 0 --op-id "r8bad$n"
    check "[$label] R8 min_free_bytes='$v': refused rc 2 reason config_not_integer, nothing written" "$RC/$(echo "$ERR" | grep -c 'reason=config_not_integer')/$([ -e "$R/ev/disk/r8bad$n.json" ] && echo yes || echo no)" "2/1/no"
  done
  CONF="$R/ok1"; printf 'min_free_bytes =\t1000 \r\n' >"$CONF"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 0 --op-id r8ok1
  check "[$label] R8 value padded with spaces, a tab and a CR: trimmed, accepted, min_free_bytes recorded 1000" "$RC/$(jq -r '.min_free_bytes' "$R/ev/disk/r8ok1.json" 2>/dev/null)" "0/1000"
  CONF="$R/ok2"; printf 'min_free_bytes=1000\n' >"$CONF"
  run_dh "$S" "$R" "$R/ev" SHIM_DF_GRAPH=1500 SHIM_DF_REPO=5000 -- --need 600 --op-id r8ok2
  check "[$label] R8 plain value 1000 still gates: 900 left after the need is refused disk_below_headroom" "$RC/$(echo "$ERR" | grep -c 'reason=disk_below_headroom')" "1/1"
  CONF="$SAVE_CONF"
  TS_FAILS=$((FAILS-before))
}

# R4: the bounded probes' output lives in a private mktemp -d directory; a FIFO planted at the OLD predictable names is never opened
privtmp_suite() { # $1 = script under test; $2 = label; sets TS_FAILS
  local S="$1" label="$2" before=$FAILS R="$T/ptm-$2"
  rm -rf "${R:?}"; mkdir -p "$R/tmp"
  cat >"$R/planted.sh" <<'WRAP'
#!/usr/bin/env bash
# plants FIFOs at the predictable names an earlier version used ($TMPDIR/disk_headroom.out.<pid>.<n>), then becomes the gate (same pid)
for n in 1 2 3 4 5 6; do mkfifo "$TMPDIR/disk_headroom.out.$$.$n"; done
exec bash "$DH_REAL" "$@"
WRAP
  run_dh "$R/planted.sh" "$R" "$R/ev" TMPDIR="$R/tmp" DH_REAL="$S" DISK_HEADROOM_CMD_TIMEOUT=3 SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id r4a
  check "[$label] R4 FIFOs planted at the old predictable names: the gate passes with the REAL values (never reads the FIFO)" "$RC/$(jq -r '[.filesystems[].free_before]|min' "$R/ev/disk/r4a.json" 2>/dev/null)/$(jq -r .verdict "$R/ev/disk/r4a.json" 2>/dev/null)" "0/5000/pass"
  check "[$label] R4 after the run the only entries left in TMPDIR are the planted FIFOs (private dir removed)" "$(ls -A "$R/tmp" | grep -vc '^disk_headroom\.out\.')" "0"
  rm -rf "${R:?}/tmp"; mkdir -p "$R/tmp"
  run_dh "$S" "$R" "$R/ev" TMPDIR="$R/tmp" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 -- --need 1 --op-id r4b
  check "[$label] R4 normal run: passes and leaves nothing in TMPDIR" "$RC/$(ls -A "$R/tmp" | wc -l | tr -d ' ')" "0/0"
  TS_FAILS=$((FAILS-before))
}

if [ "${DH_ONLY_R4:-0}" = 1 ]; then   # fast loop for the round-4 fixtures: only the suites above plus the own-ness units
  QUIET=0 freeze_suite "$SUT" real; QUIET=0 conf_suite "$SUT" real; QUIET=0 privtmp_suite "$SUT" real; QUIET=0 own_suite "$SUT" real
  echo "disk_headroom r4 tests: $PASSES passed, $FAILS failed"; [ "$FAILS" -eq 0 ]; exit $?
fi

QUIET=0 turn_suite "$SUT" real
REAL_TURN_FAILS=$TS_FAILS
QUIET=0 core_suite "$SUT" real
REAL_CORE_FAILS=$TS_FAILS
QUIET=0 bound_suite "$SUT" real
REAL_BOUND_FAILS=$TS_FAILS
QUIET=0 signal_suite "$SUT" real
REAL_SIGNAL_FAILS=$TS_FAILS
QUIET=0 bound_signal_suite "$SUT" real
REAL_BSIG_FAILS=$TS_FAILS
QUIET=0 kill_chain_suite "$SUT" real
REAL_KCHAIN_FAILS=$TS_FAILS
QUIET=0 window_suite "$SUT" real
REAL_WINDOW_FAILS=$TS_FAILS
QUIET=0 wedged_suite "$SUT" real
REAL_WEDGE_FAILS=$TS_FAILS
QUIET=0 own_suite "$SUT" real
REAL_OWN_FAILS=$TS_FAILS
QUIET=0 dep_suite "$SUT" real
REAL_DEP_FAILS=$TS_FAILS
QUIET=0 write_signal_suite "$SUT" real
REAL_WSIG_FAILS=$TS_FAILS
QUIET=0 preexec_suite "$SUT" real
REAL_PRE_FAILS=$TS_FAILS
QUIET=0 freeze_suite "$SUT" real
REAL_FREEZE_FAILS=$TS_FAILS
QUIET=0 conf_suite "$SUT" real
REAL_CONF_FAILS=$TS_FAILS
QUIET=0 privtmp_suite "$SUT" real
REAL_PTM_FAILS=$TS_FAILS

# F7: a hung host entry point must not hang the gate; the reaper call is bounded and counts as failing
RR="$T/reaper-hang"; mkdir -p "$RR/.audit"
jq -nc '{run_id:"other-run",pid:1,cmdline:"x",started_at:"2025-10-01T00:00:00Z"}' >"$RR/.audit/commit_turn.json"
printf '#!/usr/bin/env bash\nexec sleep 25\n' >"$RR/hang-host"; chmod +x "$RR/hang-host"
t0=$(date +%s)
run_dh "$SUT" "$RR" "$RR/ev" SHIM_DF_GRAPH=5000 SHIM_DF_REPO=5000 CPA_HOST_ENTRY="$RR/hang-host" DISK_HEADROOM_REAPER_TIMEOUT=2 EVREC_TURN_RUN_ID=me -- --need 1 --op-id hang
t1=$(date +%s)
check "F7 hung cpa-host: refused commit_turn_held (not a timeout kill rc=124)" "$RC/$(echo "$ERR" | grep -c 'reason=commit_turn_held')" "1/1"
check "F7 hung cpa-host: gate returned within 15s" "$([ $((t1-t0)) -lt 15 ] && echo yes || echo no)" "yes"
check "F7 hung cpa-host: nothing written" "$([ -e "$RR/ev/disk/hang.json" ] && echo yes || echo no)" "no"

# =================== paired mutations (each must make the turn suite FAIL) ===================
mkdir -p "$(dirname "$MUT_RECORD")"
{ echo "disk_headroom paired mutations $(date -u +%Y-%m-%dT%H:%M:%SZ)"; echo "script under test: $SUT"; } >"$MUT_RECORD"
mutate() { # $1 name, $2 sed expression applied to marker line(s), $3 suite function (default turn_suite)
  local f="$T/mut-$1.sh"
  sed -E "$2" "$SUT" >"$f" 2>/dev/null || { bad "mutation $1 not applied (script under test unreadable)"; echo "mutation $1: NOT APPLIED" >>"$MUT_RECORD"; return; }
  if cmp -s "$f" "$SUT"; then bad "mutation $1 changed nothing (marker missing in script)"; echo "mutation $1: NOT APPLIED (marker missing)" >>"$MUT_RECORD"; return; fi
  local keepf=$FAILS keepp=$PASSES
  QUIET=1 "${3:-turn_suite}" "$f" "mut-$1" >"$T/mut-$1.out" 2>&1
  local caught=$TS_FAILS
  FAILS=$keepf; PASSES=$keepp   # the mutant's failures are the expected result, not test failures
  if [ "$caught" -gt 0 ]; then ok "mutation $1 observed failing ($caught assertion(s) RED)"; echo "mutation $1: CAUGHT, $caught assertion(s) failed: $(grep '^FAIL' "$T/mut-$1.out" | head -3 | tr '\n' ';')" >>"$MUT_RECORD"
  else bad "mutation $1 survived (suite stayed green)"; echo "mutation $1: SURVIVED" >>"$MUT_RECORD"; fi
}
mutate ignore-grant       '/# MUT:ignore-grant$/c\  :'
mutate unreadable-as-none '/# MUT:unreadable-as-none$/c\  [ -r "$GRANT" ] || { GRUN=""; return 2; }'
mutate no-reread          '/# MUT:no-reread$/c\  st=2'
mutate direct-reap        '/# MUT:direct-reap$/c\  (cd "$ROOT_DIR" \&\& bash scripts/release/commit_turn_check.sh --reap) >/dev/null 2>\&1 || true'
mutate reread-unreadable-passes '/# MUT:no-reread$/a\    [ "$st" -eq 1 ] \&\& return 0'
mutate df-size-not-avail   's/--output=avail/--output=size/' core_suite
mutate df-multi-token-merged '/# MUT:single-field$/c\  out="${out//[[:space:]]/}"' core_suite
mutate opid-no-length-cap  '/# MUT:opid-cap$/c\  :' core_suite
mutate fixed-tmp-name      '/# MUT:tmpname$/c\TMP="$OUTDIR/$OP_ID.json.tmp"' core_suite
mutate freeze-unset-caller-is-holder 's/\$\{EVREC_TURN_RUN_ID:-\}/${EVREC_TURN_RUN_ID:-$GRUN}/g'
mutate reaper-cwd-dropped  's/\(cd "\$ROOT_DIR" && /(/'
mutate freeze-only-on-pass '/# MUT:ignore-grant$/c\if [ "$VERDICT" = pass ]; then turn_freeze_check || exit 1; fi'
mutate podman-unbounded    '/# MUT:podman-timeout$/c\BOUND_OUT="$(podman info --format '"'"'{{.Store.GraphRoot}}'"'"' 2>/dev/null)"' bound_suite
mutate df-unbounded        '/# MUT:df-timeout$/c\  BOUND_OUT="$(df -B1 --output=avail "$1" 2>/dev/null)"' bound_suite
mutate podman-foreground-orphans '/# MUT:podman-timeout$/c\BOUND_OUT="$(podman info --format '"'"'{{.Store.GraphRoot}}'"'"' 2>/dev/null)"' bound_signal_suite
mutate df-foreground-orphans     '/# MUT:df-timeout$/c\  BOUND_OUT="$(df -B1 --output=avail "$1" 2>/dev/null)"' bound_signal_suite
mutate no-signal-trap-bounded '/# MUT:signal-trap$/c\:' bound_signal_suite
mutate bang-fallback-dropped  '/# MUT:bang-fallback$/c\  :' window_suite
mutate kill-fallback-dropped  '/# MUT:kill-fallback$/c\  :' wedged_suite
mutate kill-chain-dropped     '/# MUT:kill-chain$/c\  :' kill_chain_suite
mutate own-ppid-dropped       '/# MUT:own-ppid$/c\  :' own_suite
mutate own-cmdline-dropped    '/# MUT:own-cmdline$/c\  return 0' own_suite
mutate snap-descend-dropped   '/# MUT:snap-descend$/c\      :' own_suite
mutate snap-pid-guard-dropped '/# MUT:snap-pid-guard$/c\    :' own_suite
mutate snap-start-guard-dropped '/# MUT:snap-start-guard$/c\    proc_fields "$p" \&\& [ "$PF_STATE" != Z ] \&\& kill -"$1" "$p" 2>/dev/null' own_suite
mutate no-signal-trap      '/# MUT:signal-trap$/c\    :' signal_suite
mutate timeout-zero-allowed 's/valid_pos_int\(\) \{ valid_int "\$1" && \[ "\$1" != 0 \]; \}/valid_pos_int() { valid_int "$1"; }/' bound_suite
mutate preexec-image-dropped   '/# MUT:own-preexec$/c\  :' own_suite
mutate preexec-image-dropped-it '/# MUT:own-preexec$/c\  :' preexec_suite
mutate preexec-kill-dropped    '/# MUT:preexec-kill$/c\  :' preexec_suite
mutate bound-keeps-failed-output '/# MUT:bound-rc$/c\  if :; then' bound_suite
mutate bound-no-kill-after     '/# MUT:kill-after$/c\  timeout "$CMD_TO" "$@" >"$OUT_F" 2>/dev/null \&' bound_suite
mutate opid-charset-dropped    '/# MUT:opid-charset$/c\  :' core_suite
mutate opid-leading-dot-dropped '/# MUT:opid-dot$/c\  :' core_suite
mutate opid-empty-defaulted    '/# MUT:opid-empty$/c\[ -n "$OP_ID" ] || OP_ID="op-$(date -u +%Y%m%dT%H%M%SZ)-$$"' core_suite
mutate grant-runid-nonempty-dropped 's/ and length>0//'
mutate grant-runid-type-dropped 's/type=="string" and //'
mutate preflight-dropped       '/# MUT:preflight$/c\  :' dep_suite
mutate write-trap-dropped      '/# MUT:write-trap$/c\trap - TERM INT HUP' write_signal_suite
# round 4 (WF3-REVIEW R1-R4, R8): each reviewer mutant that survived round 3, plus the new behaviours
mutate own-empty-dropped          '/# MUT:own-empty$/c\  :' own_suite
mutate own-empty-start-dropped    '/# MUT:own-empty$/c\  [ -z "$cmd" ] \&\& return 0' own_suite
mutate preexec-empty-dropped      '/# MUT:preexec-empty$/c\  :' own_suite
mutate grant-enoent-as-none       '/# MUT:enoent$/c\  [ -e "$GRANT" ] || return 2' freeze_suite
mutate freeze-outdir-set-semantics '/# MUT:outdir-decision$/c\OUT_DIR_IN_USE=0; [ -n "${DISK_HEADROOM_OUT_DIR+x}" ] \&\& OUT_DIR_IN_USE=1' freeze_suite
mutate bounded-predictable-name   '/# MUT:outname$/c\  OUT_N=$((OUT_N + 1)); OUT_F="${TMPDIR:-/tmp}/disk_headroom.out.$$.$OUT_N"' privtmp_suite
mutate conf-whitespace-merged     '/# MUT:conf-trim$/c\RAW="$(printf '"'"'%s\\n'"'"' "$KEYLINES" | cut -d= -f2- | tr -d '"'"'[:space:]'"'"')"' conf_suite
check "real script: bound suite has zero failures" "$REAL_BOUND_FAILS" "0"
check "real script: dependency-preflight suite has zero failures" "$REAL_DEP_FAILS" "0"
check "real script: record-write signal suite has zero failures" "$REAL_WSIG_FAILS" "0"
check "real script: fork-to-exec (pre-exec child) suite has zero failures" "$REAL_PRE_FAILS" "0"
check "real script: turn-freeze suite has zero failures" "$REAL_TURN_FAILS" "0"
check "real script: signal suite has zero failures" "$REAL_SIGNAL_FAILS" "0"
check "real script: bounded-probe signal suite has zero failures" "$REAL_BSIG_FAILS" "0"
check "real script: ignoring-timeout KILL-chain suite has zero failures" "$REAL_KCHAIN_FAILS" "0"
check "real script: fork-window suite has zero failures" "$REAL_WINDOW_FAILS" "0"
check "real script: wedged-timeout KILL suite has zero failures" "$REAL_WEDGE_FAILS" "0"
check "real script: own-ness guard suite has zero failures" "$REAL_OWN_FAILS" "0"
check "real script: core suite has zero failures" "$REAL_CORE_FAILS" "0"
check "real script: freeze/grant-reachability suite has zero failures" "$REAL_FREEZE_FAILS" "0"
check "real script: config-value suite has zero failures" "$REAL_CONF_FAILS" "0"
check "real script: private-tmp suite has zero failures" "$REAL_PTM_FAILS" "0"

echo "disk_headroom tests: $PASSES passed, $FAILS failed"
[ "$FAILS" -eq 0 ]
