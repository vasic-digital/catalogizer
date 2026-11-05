#!/usr/bin/env bash
# Sandboxed test of down.sh --purge guard. Uses a mktemp root as fake purge root; never touches the real HOME. podman calls are shimmed away.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
DOWN="${DOWN:-$HERE/down.sh}"   # mutation runs point DOWN at a mutated COPY inside the sandbox
SB="$(mktemp -d /home/milosvasic/.cache/claude-tmp/downpurge.XXXXXX)"; : "${SB:?}"
trap 'rm -rf -- "${SB:?}"' EXIT
mkdir -p "$SB/bin" "$SB/root/catqa-s3live" "$SB/root/catqa-s3live-x" "$SB/root/other" "$SB/outside"
printf '#!/bin/sh\nexit 0\n' > "$SB/bin/podman"; chmod +x "$SB/bin/podman"
touch "$SB/root/catqa-s3live/f" "$SB/root/other/keep" "$SB/outside/keep"
fail=0
run() { PATH="$SB/bin:$PATH" S3_PURGE_ROOT="$SB/root" S3_STATE="$1" bash "$DOWN" --purge >/dev/null 2>&1; echo $?; }
chk() { [ -e "$1" ] || { echo "FAIL: $1 was deleted ($2)"; fail=1; }; }
rc=$(run ""); [ "$rc" = 2 ] || { echo "FAIL empty rc=$rc"; fail=1; }
rc=$(run "$SB/root/catqa-s3live-x/../../outside"); [ "$rc" = 2 ] || { echo "FAIL dotdot-escape rc=$rc"; fail=1; }
rc=$(run "$SB/root/catqa-s3live-x/../other"); [ "$rc" = 2 ] || { echo "FAIL dotdot-sibling rc=$rc"; fail=1; }
rc=$(run "$SB/root"); [ "$rc" = 2 ] || { echo "FAIL root itself rc=$rc"; fail=1; }
rc=$(run "/"); [ "$rc" = 2 ] || { echo "FAIL slash rc=$rc"; fail=1; }
ln -s "$SB/outside" "$SB/root/catqa-s3live-link"
rc=$(run "$SB/root/catqa-s3live-link"); [ "$rc" = 2 ] || { echo "FAIL symlink rc=$rc"; fail=1; }
# layer-independence: a '..' path that RESOLVES to a legitimate target must still be refused by the '..' segment check alone
mkdir -p "$SB/root/catqa-s3live"; touch "$SB/root/catqa-s3live/f"
rc=$(run "$SB/root/other/../catqa-s3live"); [ "$rc" = 2 ] && [ -e "$SB/root/catqa-s3live/f" ] || { echo "FAIL dotdot-resolving-legit rc=$rc"; fail=1; }
# a '..'-free but nested path (dirname layer alone) and a '..'-free symlink (readlink layer alone)
mkdir -p "$SB/root/catqa-s3live/sub/catqa-s3live-n"; touch "$SB/root/catqa-s3live/sub/catqa-s3live-n/f"
rc=$(run "$SB/root/catqa-s3live/sub/catqa-s3live-n"); [ "$rc" = 2 ] && [ -e "$SB/root/catqa-s3live/sub/catqa-s3live-n/f" ] || { echo "FAIL nested rc=$rc"; fail=1; }
chk "$SB/outside/keep" symlink-target
chk "$SB/root/other/keep" dotdot; chk "$SB/outside/keep" escape; chk "$SB/root" root
rc=$(run "$SB/root/catqa-s3live"); [ "$rc" = 0 ] && [ ! -e "$SB/root/catqa-s3live" ] || { echo "FAIL legit purge rc=$rc"; fail=1; }
[ $fail = 0 ] && echo "PASS test_down_purge" || { echo "FAILED"; exit 1; }
