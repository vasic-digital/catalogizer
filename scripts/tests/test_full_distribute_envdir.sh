#!/usr/bin/env bash
# Test: scripts/full-distribute.sh --start removes its per-host temp env dir
# even when secret generation fails (no leaked directory under TMPDIR).
# Runs the REAL script in a throwaway project tree; ssh/scp are PATH shims that
# only record being called (no network), and a failing `openssl` shim forces
# install_write_env to fail.
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "${tmp:?}"' EXIT
fail=0; ok() { echo "ok   - $1"; }; bad() { echo "FAIL - $1"; fail=1; }

mkdir -p "$tmp/p/scripts/lib" "$tmp/p/submodules/containers" "$tmp/bin" "$tmp/t"
cp "$ROOT/scripts/full-distribute.sh" "$tmp/p/scripts/"
cp "$ROOT/scripts/lib/install_env.sh" "$tmp/p/scripts/lib/"
: > "$tmp/p/docker-compose.dev.yml"
printf 'CONTAINERS_REMOTE_HOST_1_NAME=h1\nCONTAINERS_REMOTE_HOST_1_ADDRESS=127.0.0.1\nCONTAINERS_REMOTE_HOST_1_USER=u\n' > "$tmp/p/submodules/containers/.env"
for c in ssh scp; do printf '#!/bin/sh\necho "%s $*" >> "%s/calls"\nexit 0\n' "$c" "$tmp" > "$tmp/bin/$c"; chmod +x "$tmp/bin/$c"; done
printf '#!/bin/sh\nexit 1\n' > "$tmp/bin/openssl"; chmod +x "$tmp/bin/openssl"

# 1) generation failure: script must fail AND leave no temp dir behind.
( cd "$tmp/p" && PATH="$tmp/bin:$PATH" TMPDIR="$tmp/t" bash scripts/full-distribute.sh --start >/dev/null 2>&1 ); rc=$?
[ "$rc" -ne 0 ] && ok "generation failure aborts the script (rc=$rc)" || bad "script succeeded despite failing generator"
[ -z "$(ls -A "$tmp/t")" ] && ok "no leaked temp dir after failure" || bad "leaked: $(ls -A "$tmp/t")"
grep -q '^scp .*\.env' "$tmp/calls" 2>/dev/null && bad "an env file was sent despite failure" || ok "no env file sent on failure"

# 2) success path (working openssl): temp dir also cleaned, env file sent.
rm -f "$tmp/bin/openssl" "$tmp/calls"
( cd "$tmp/p" && PATH="$tmp/bin:$PATH" TMPDIR="$tmp/t" bash scripts/full-distribute.sh --start >/dev/null 2>&1 ); rc=$?
[ "$rc" -eq 0 ] && ok "success path rc=0" || bad "success path rc=$rc"
[ -z "$(ls -A "$tmp/t")" ] && ok "no leaked temp dir after success" || bad "leaked: $(ls -A "$tmp/t")"
grep -q '^scp -q .*\.env' "$tmp/calls" && ok "env file was sent" || bad "env file not sent"

[ "$fail" -eq 0 ] && echo PASS || echo FAILED
exit $fail
