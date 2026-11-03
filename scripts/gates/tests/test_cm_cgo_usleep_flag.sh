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
good=$(mk good); printf '#!/bin/bash\n. "$(dirname "$0")/lib/cgo_flags.sh" || exit 1\ngo build ./...\n' > "$good/scripts/b.sh"
printf 'scripts/distribute.sh\texempt:fixture\nscripts/b.sh\tcovered\n' > "$good/scope.tsv"; git -C "$good" add -A
bad1=$(mk bad1); printf '#!/bin/bash\ngo build ./...\n' > "$bad1/scripts/b.sh"; printf 'scripts/distribute.sh\texempt:fixture\nscripts/b.sh\tcovered\n' > "$bad1/scope.tsv"; git -C "$bad1" add -A
bad2=$(mk bad2); printf '#!/bin/bash\ngo vet ./...\n' > "$bad2/scripts/new.sh"; printf 'scripts/distribute.sh\texempt:fixture\n' > "$bad2/scope.tsv"; git -C "$bad2" add -A
run() { bash "$GATE" --root "$1" --scope "$1/scope.tsv" >"$T/o" 2>&1; echo $?; }
eq "golden-good fixture PASSes" "$(run "$good")" 0
eq "golden-bad: covered file without define/source FAILs" "$(run "$bad1")" 1
eq "golden-bad: unclassified go-line file FAILs" "$(run "$bad2")" 1
nn=$(mk nn); printf 'scripts/x\texempt:y\n' > "$nn/scope.tsv"; rm "$nn/scripts/distribute.sh"; git -C "$nn" add -A
eq "missing positive control needle is BLIND (exit 2)" "$(run "$nn")" 2

# F1: only UNCOMMENTED executable lines count (each case is a covered file the gate must reject)
SRCLN='. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo missing >&2; exit 1; }'
covcase() { # covcase name file-body
  d=$(mk "c_$1"); printf '%s\n' "$2" > "$d/scripts/b.sh"; printf 'scripts/distribute.sh\texempt:fixture\nscripts/b.sh\tcovered\n' > "$d/scope.tsv"; git -C "$d" add -A; echo "$d"; }
eq "covered with a real sourcing line PASSes" "$(run "$(covcase ok $'#!/bin/bash\n'"$SRCLN"$'\ngo build ./...')")" 0
eq "F1 mutation: sourcing line commented out FAILs" "$(run "$(covcase m1 $'#!/bin/bash\n# '"$SRCLN"$'\ngo build ./...')")" 1
eq "F1 mutation: sourcing line replaced by an echo naming the lib FAILs" "$(run "$(covcase m2 $'#!/bin/bash\necho "skip lib/cgo_flags.sh"\ngo build ./...')")" 1
eq "F1 mutation: lib name only in a comment, go line without define FAILs" "$(run "$(covcase m3 $'#!/bin/bash\n# uses lib/cgo_flags.sh\nCGO_CPPFLAGS= go build ./...')")" 1
eq "F1 mutation: define only in a comment FAILs" "$(run "$(covcase m4 $'#!/bin/bash\ngo build ./... # CGO_CPPFLAGS=-DHAVE_USLEEP=1')")" 1
eq "F1 mutation: define only in an echo FAILs" "$(run "$(covcase m5 $'#!/bin/bash\necho CGO_CPPFLAGS=-DHAVE_USLEEP=1\ngo build ./...')")" 1
eq "F4: sourcing line without a loud failure branch FAILs" "$(run "$(covcase m6 $'#!/bin/bash\n. "$(dirname "$0")/lib/cgo_flags.sh"\ngo build ./...')")" 1
eq "inline define on an executable line PASSes" "$(run "$(covcase i1 $'#!/bin/bash\nCGO_CPPFLAGS=-DHAVE_USLEEP=1 go build ./...')")" 0
# F2: sweep widening (each unclassified file must be FOUND, i.e. FAIL as unclassified)
sweep() { # sweep name filename body
  d=$(mk "s_$1"); printf '%s\n' "$3" > "$d/$2"; chmod +x "$d/$2"; printf 'scripts/distribute.sh\texempt:fixture\n' > "$d/scope.tsv"; git -C "$d" add -A; echo "$d"; }
eq "F2: extension-less script with a shebang is swept" "$(run "$(sweep e1 scripts/runit $'#!/usr/bin/env bash\ngo test ./...')")" 1
eq "F2: two spaces between go and subcommand" "$(run "$(sweep e2 scripts/x.sh $'#!/bin/bash\ngo  build ./...')")" 1
eq "F2: absolute /go path prefix" "$(run "$(sweep e3 scripts/x.sh $'#!/bin/bash\n/usr/local/go/bin/go build ./...')")" 1
eq "F2: go after && on the same line" "$(run "$(sweep e4 scripts/x.sh $'#!/bin/bash\ncd a && go vet ./...')")" 1
eq "F2: go after ; on the same line" "$(run "$(sweep e5 scripts/x.sh $'#!/bin/bash\ncd a; go run main.go')")" 1
eq "F2: line continuation" "$(run "$(sweep e6 scripts/x.sh $'#!/bin/bash\ngo \\\n  build ./...')")" 1
eq "F2: .bash extension" "$(run "$(sweep e7 scripts/x.bash $'#!/bin/bash\ngo build ./...')")" 1
eq "F2: go generate" "$(run "$(sweep e8 scripts/x.sh $'#!/bin/bash\ngo generate ./...')")" 1
eq "F2: build.Dockerfile" "$(run "$(sweep e9 build.Dockerfile $'FROM x\nRUN go build ./...')")" 1
eq "F2: \$GO variable invocation" "$(run "$(sweep e10 scripts/x.sh $'#!/bin/bash\n"$GO" test ./...')")" 1
eq "F2: docker-compose command" "$(run "$(sweep e11 docker-compose.dev.yml $'services:\n  a:\n    command: go run main.go')")" 1
eq "F2: .air.toml cmd" "$(run "$(sweep e12 .air.toml $'cmd = "go build -o x ."')")" 1
eq "F2 control: a comment-only mention is NOT a holder" "$(run "$(sweep e13 scripts/x.sh $'#!/bin/bash\n# go build is slow')")" 0
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
