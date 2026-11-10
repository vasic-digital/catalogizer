#!/usr/bin/env bash
# CAT-FRONTDOOR-LOCALMODE-1: sandbox test of the --interface (local second IP) mode of test_clientip_frontdoor.sh.
# Fake curl/ip/ssh on PATH inside a mktemp dir; no stack, no network, no credential.
# usage: test_frontdoor_interface.sh [--script <path>]   (override lets the RED run target a pre-change copy)  exit 0 = all pass
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; SCRIPT="$HERE/test_clientip_frontdoor.sh"; [ "${1:-}" = --script ] && SCRIPT="$2"
S="$(mktemp -d)"; : "${S:?}"; trap 'rm -rf -- "${S:?}"' EXIT
fail=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1 $2"; fail=1; }
mkdir -p "$S/bin"
cat > "$S/bin/curl" <<'FC'
#!/bin/sh
# fake front door: default-source bucket answers 401 for the control call then 429 (flooded); a call bound with --interface has its own bucket (401).
echo "$*" >> "$FAKE_DIR/curl.log"
case "$*" in *--interface*) printf '%s' "${FAKE_IFACE_CODE:-401}"; exit 0;; esac
n=$(($(cat "$FAKE_DIR/n" 2>/dev/null || echo 0)+1)); echo $n > "$FAKE_DIR/n"
[ "$n" -le 1 ] && printf 401 || printf 429
FC
cat > "$S/bin/ip" <<'FI'
#!/bin/sh
case "$*" in
  *"addr"*) printf '2: eth0    inet 10.0.0.5/24 brd 10.0.0.255 scope global eth0\n2: eth0    inet 10.0.0.6/24 scope global secondary eth0\n';;
  *"route get"*) echo "10.0.0.9 dev eth0 src ${FAKE_DEFAULT_SRC:-10.0.0.5} uid 1000";;
esac
FI
cat > "$S/bin/ssh" <<'FS'
#!/bin/sh
echo SSH-CALLED >> "$FAKE_DIR/ssh.log"; printf 401
FS
chmod +x "$S/bin/"*
run() { rm -f "$S/n" "$S/curl.log" "$S/ssh.log"; : > "$S/curl.log"; FAKE_DIR="$S" PATH="$S/bin:$PATH" FD_WARMUP=0 timeout 60 bash "$SCRIPT" "$@" > "$S/out" 2>&1; echo $? > "$S/rc"; }

run http://10.0.0.9:33080 --interface 10.0.0.6
[ "$(cat "$S/rc")" = 0 ] && ok I1.second_ip_leg_passes_exit0 || bad I1.second_ip_leg_passes_exit0 "rc=$(cat "$S/rc") $(tail -n 3 "$S/out")"
grep -q -- '--interface 10.0.0.6' "$S/curl.log" && ok I2.curl_bound_to_the_second_ip || bad I2.curl_bound_to_the_second_ip "no --interface call"
grep -q "^B(local): .*10.0.0.6.* 401" "$S/out" && ok I3.leg_B_reported_with_status || bad I3.leg_B_reported_with_status "$(cat "$S/out")"
[ ! -s "$S/ssh.log" ] && ok I4.no_ssh_in_local_mode || bad I4.no_ssh_in_local_mode "ssh called"
grep -q 'PASS (A and B)' "$S/out" && ok I5.verdict_names_B_verified || bad I5.verdict_names_B_verified ""

FAKE_IFACE_CODE=429 run http://10.0.0.9:33080 --interface 10.0.0.6
[ "$(cat "$S/rc")" = 1 ] && ok I6.second_ip_locked_out_fails || bad I6.second_ip_locked_out_fails "rc=$(cat "$S/rc")"

run http://10.0.0.9:33080 --interface 10.0.0.77
[ "$(cat "$S/rc")" = 1 ] && grep -q 'not assigned to this host' "$S/out" && ok I7.unassigned_ip_refused_loudly || bad I7.unassigned_ip_refused_loudly "rc=$(cat "$S/rc") $(tail -n 2 "$S/out")"

run http://10.0.0.9:33080 --interface 'not-an-ip;x'
[ "$(cat "$S/rc")" = 1 ] && ok I8.malformed_value_refused || bad I8.malformed_value_refused "rc=$(cat "$S/rc")"

FAKE_DEFAULT_SRC=10.0.0.6 run http://10.0.0.9:33080 --interface 10.0.0.6
grep -q 'SKIP.*same_as_default_source' "$S/out" && ! grep -q 'PASS (A and B)' "$S/out" && ok I9.same_bucket_skipped_not_passed || bad I9.same_bucket_skipped_not_passed "$(tail -n 3 "$S/out")"

run http://10.0.0.9:33080
grep -q 'B: SKIP second_host_not_supplied' "$S/out" && ok I10.legacy_no_arg_still_skips_B || bad I10.legacy_no_arg_still_skips_B "$(tail -n 3 "$S/out")"
run http://10.0.0.9:33080 someuser@otherhost
[ -s "$S/ssh.log" ] && ok I11.legacy_ssh_second_host_still_works || bad I11.legacy_ssh_second_host_still_works "ssh not called: $(tail -n 3 "$S/out")"
exit $fail
