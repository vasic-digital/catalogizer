#!/usr/bin/env bash
# test_cm_cgo_usleep_flag.sh - executes scripts/lib/cgo_flags.sh and scripts/gates/cm_cgo_usleep_flag.sh for real:
# append-safe/idempotent define, golden-good/golden-bad fixture repos, BLIND on missing control needle, real-repo PASS,
# every sourcing line resolves to the real lib, and a paired mutation of the gate (GATE env: mutated copy must FAIL golden-bad).
set -u
SRC=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
GATE=${GATE:-$SRC/scripts/gates/cm_cgo_usleep_flag.sh}
T=$(mktemp -d) || exit 2; trap 'rm -rf "${T:?}"' EXIT
pass=0; fail=0
chk() { if [ "$2" = ok ]; then pass=$((pass+1)); echo "PASS: $1"; else fail=$((fail+1)); echo "FAIL: $1"; fi; }
eq() { [ "$2" = "$3" ] && chk "$1" ok || chk "$1 (got [$2] want [$3])" no; }
# lib
eq "lib defines when unset" "$(unset CGO_CPPFLAGS; . "$SRC/scripts/lib/cgo_flags.sh"; echo "$CGO_CPPFLAGS")" "-DHAVE_USLEEP=1"
eq "lib preserves a caller CGO_CPPFLAGS" "$(CGO_CPPFLAGS='-I/x'; . "$SRC/scripts/lib/cgo_flags.sh"; echo "$CGO_CPPFLAGS")" "-I/x -DHAVE_USLEEP=1"
eq "lib is idempotent" "$(unset CGO_CPPFLAGS; . "$SRC/scripts/lib/cgo_flags.sh"; . "$SRC/scripts/lib/cgo_flags.sh"; echo "$CGO_CPPFLAGS")" "-DHAVE_USLEEP=1"
eq "lib leaves CGO_CFLAGS alone (go default -O2 -g kept)" "$(unset CGO_CFLAGS; . "$SRC/scripts/lib/cgo_flags.sh"; echo "${CGO_CFLAGS-unset}")" "unset"
# fixture repos
mk() { d=$T/$1; mkdir -p "$d/scripts/lib"; git -C "$d" init -q; cp "$SRC/scripts/lib/cgo_flags.sh" "$d/scripts/lib/"; printf '#!/bin/sh\ngo test ./...\n' > "$d/scripts/distribute.sh"; echo "$d"; }
good=$(mk good); printf '#!/bin/bash\n. "$(dirname "$0")/lib/cgo_flags.sh"\ngo build ./...\n' > "$good/scripts/b.sh"
printf 'scripts/distribute.sh\texempt:fixture\nscripts/b.sh\tcovered\n' > "$good/scope.tsv"; git -C "$good" add -A
bad1=$(mk bad1); printf '#!/bin/bash\ngo build ./...\n' > "$bad1/scripts/b.sh"; printf 'scripts/distribute.sh\texempt:fixture\nscripts/b.sh\tcovered\n' > "$bad1/scope.tsv"; git -C "$bad1" add -A
bad2=$(mk bad2); printf '#!/bin/bash\ngo vet ./...\n' > "$bad2/scripts/new.sh"; printf 'scripts/distribute.sh\texempt:fixture\n' > "$bad2/scope.tsv"; git -C "$bad2" add -A
run() { bash "$GATE" --root "$1" --scope "$1/scope.tsv" >"$T/o" 2>&1; echo $?; }
eq "golden-good fixture PASSes" "$(run "$good")" 0
eq "golden-bad: covered file without define/source FAILs" "$(run "$bad1")" 1
eq "golden-bad: unclassified go-line file FAILs" "$(run "$bad2")" 1
nn=$(mk nn); printf 'scripts/x\texempt:y\n' > "$nn/scope.tsv"; rm "$nn/scripts/distribute.sh"; git -C "$nn" add -A
eq "missing positive control needle is BLIND (exit 2)" "$(run "$nn")" 2
# real repo
bash "$SRC/scripts/gates/cm_cgo_usleep_flag.sh" >"$T/real" 2>&1; eq "real repo PASSes" "$?" 0
# every sourcing line resolves
miss=0
while IFS=$'\t' read -r f c; do
  [ "$c" = covered ] || continue; [ -f "$SRC/$f" ] || continue
  rel=$(grep -o '/[^"]*lib/cgo_flags\.sh"' "$SRC/$f" | head -1 | sed 's/"$//')
  [ -n "$rel" ] || continue
  [ -f "$SRC/$(dirname "$f")$rel" ] || { echo "unresolved: $f -> $rel"; miss=1; }
done < "$SRC/scripts/lib/cgo_flag_scope.tsv"
eq "every sourcing line resolves to scripts/lib/cgo_flags.sh" "$miss" 0
echo "RESULT pass=$pass fail=$fail"; [ "$fail" = 0 ]
