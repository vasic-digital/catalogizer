#!/usr/bin/env bash
# Test for scripts/container-local.sh (local rootless-podman build/test runner).
# Asserts via --dry-run: only podman, no docker/sudo/ssh, non-root user, no root-writable host mounts.
set -u
S="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/container-local.sh"
fail=0; n=0
ck() { n=$((n+1)); if eval "$2"; then echo "ok $n - $1"; else echo "not ok $n - $1"; fail=$((fail+1)); fi; }
[ -x "$S" ] || { echo "not ok 0 - $S missing or not executable"; exit 1; }
for m in api-build api-test web-build web-test; do
  out=$("$S" --dry-run "$m" 2>&1); rc=$?
  ck "$m dry-run exits 0" "[ $rc -eq 0 ]"
  ck "$m uses podman run" "grep -q '^podman run' <<<\"\$out\""
  ck "$m is non-root (keep-id)" "grep -q -- '--userns=keep-id' <<<\"\$out\""
  ck "$m has memory limit" "grep -q -- '--memory' <<<\"\$out\""
  ck "$m has no docker/sudo/ssh/thinker" "! grep -E -q '(^|[ /])(docker|sudo|ssh|rsync|scp)( |$)|thinker' <<<\"\$out\""
done
out=$("$S" --dry-run api-test 2>&1)
ck "api-test runs with --network none (hermetic)" "grep -q -- '--network none' <<<\"\$out\""
"$S" bogus >/dev/null 2>&1; ck "unknown mode exits non-zero" "[ $? -ne 0 ]"
echo "1..$n failed=$fail"; [ $fail -eq 0 ]
