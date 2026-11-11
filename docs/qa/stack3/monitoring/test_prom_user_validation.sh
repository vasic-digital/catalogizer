#!/usr/bin/env bash
# PROM-R2-4: up.sh must validate PROM_AUTH_USER BEFORE it is used in sed/podman/YAML. Runs the REAL up.sh in a mktemp sandbox
# (QA_DIR under it, fake podman/curl on PATH that only record calls; no container, port or credential is touched).
# Hostile value (e.g. `a/b;c`) -> exit 2, "invalid PROM_AUTH_USER", and NO podman call. Valid value -> gets past validation (exit 3 at the fake missing image).
# usage: test_prom_user_validation.sh [--script <up.sh>]   exit 0 = all pass
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; SCRIPT="$HERE/up.sh"; [ "${1:-}" = --script ] && SCRIPT="$2"
S="$(mktemp -d)"; : "${S:?}"; trap 'rm -rf -- "${S:?}"' EXIT
fail=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1 $2"; fail=1; }
mkdir -p "$S/bin" "$S/qa"
printf '#!/bin/sh\necho "$*" >> "%s/podman.calls"\nexit 1\n' "$S" > "$S/bin/podman"; chmod +x "$S/bin/podman"
printf '#!/bin/sh\necho "$*" >> "%s/curl.calls"\nexit 1\n' "$S" > "$S/bin/curl"; chmod +x "$S/bin/curl"
: > "$S/qa/qa.env"
run() { # <mon.env user line or empty> <env PROM_AUTH_USER or empty> ; sets RC and $S/err
  : > "$S/podman.calls"; : > "$S/curl.calls"
  printf 'GF_SECURITY_ADMIN_USER=g\nPROM_AUTH_PASSWORD=Pm-0123456789abcdef0123456789abcdef-Zz9\n%s\n' "$1" > "$S/qa/mon.env"; chmod 600 "$S/qa/mon.env"
  if [ -n "$2" ]; then PROM_AUTH_USER="$2" PATH="$S/bin:$PATH" QA_DIR="$S/qa" bash "$SCRIPT" > "$S/out" 2> "$S/err"; RC=$?
  else env -u PROM_AUTH_USER PATH="$S/bin:$PATH" QA_DIR="$S/qa" bash "$SCRIPT" > "$S/out" 2> "$S/err"; RC=$?; fi
}
for v in 'a/b;c' 'a|b' 'a&b' 'a\b' 'A' '1abc' 'a b' '' 'a$(x)' "a'b"; do
  run "PROM_AUTH_USER=$(printf '%q' "$v")" ""
  if [ "$RC" = 2 ] && grep -q 'invalid PROM_AUTH_USER' "$S/err" && [ ! -s "$S/podman.calls" ] && [ ! -s "$S/curl.calls" ]; then ok "mon.env hostile [$v]"
  else bad "mon.env hostile [$v]" "rc=$RC podman_calls=$(wc -l < "$S/podman.calls") err=$(head -c 120 "$S/err")"; fi
  # same value from the environment (mon.env has none)
  run "" "$v"
  if [ -z "$v" ]; then continue; fi   # empty env var = unset: lib.sh default applies, not a hostile case
  if [ "$RC" = 2 ] && grep -q 'invalid PROM_AUTH_USER' "$S/err" && [ ! -s "$S/podman.calls" ]; then ok "env hostile [$v]"
  else bad "env hostile [$v]" "rc=$RC podman_calls=$(wc -l < "$S/podman.calls")"; fi
done
for v in qa_prom a prom-user_2; do   # golden-good: passes validation, then stops at the fake missing image (exit 3)
  run "PROM_AUTH_USER=$v" ""
  if [ "$RC" = 3 ] && ! grep -q 'invalid PROM_AUTH_USER' "$S/err" && [ -s "$S/podman.calls" ]; then ok "valid [$v] passes validation"
  else bad "valid [$v] passes validation" "rc=$RC err=$(head -c 120 "$S/err")"; fi
done
run "" ""; if [ "$RC" = 3 ] && ! grep -q 'invalid PROM_AUTH_USER' "$S/err"; then ok "default user passes validation"; else bad "default user passes validation" "rc=$RC"; fi
exit $fail
