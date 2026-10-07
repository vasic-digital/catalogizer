#!/usr/bin/env bash
# test_dispatch_c_e2e.sh - round c end-to-end proofs with REAL containers (scripts/containers/run_pinned.sh: rootless podman, IMG-GO, digest-pinned from
# build/containers/images.lock.yaml; the disk gate runs first, memory is bounded at run time to the 60% ceiling), the real remote/emit.sh and a real git checkout.
# Host exception (declared, as test_dispatch_e2e.sh): no build host qualifies yet (T006a BLOCKED-ON ODG-07), so the "build host" is this host through
# DISPATCH_TRANSPORT=local DISPATCH_ALLOW_LOCAL=1; the snapshot, the closure, the tree cache, the container, the cgroup sampling, the events and the callback are the real ones.
# Cases: r1 (u) a Go module of a git root with a `replace` into submodules/ builds in IMG-GO because the submodule's tree was shipped into its path, and the
#   snapshot-addressed tree cache sends nothing the second time; r2 (T115, quiet build) a build that prints NOTHING for several seconds and burns CPU is not HUNG
#   with --progress log+cpu (the container's real cgroup CPU counter), its completed event carries a real peak_rss_bytes read from the container's cgroup, and the SAME
#   build with --progress log ends build_progress_flat (control).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
B=${BUILD_DIR:-$here/..}
DSP=${DISPATCH_SCRIPT:-$B/dispatch.sh}
pass=0; fail=0
ok()  { pass=$((pass+1)); echo "ok   $1"; }
bad() { fail=$((fail+1)); echo "FAIL $1${2:+ -- $2}"; }
chk() { local w=$1; shift; if "$@"; then ok "$w"; else bad "$w"; fi; }
for t in podman jq python3 git; do command -v "$t" >/dev/null 2>&1 || { echo "FAIL dependency missing: $t"; echo "RESULT pass=0 fail=1"; exit 1; }; done
T=$(mktemp -d "${TMPDIR:-/tmp}/tdcee.XXXXXX"); trap 'rm -rf "$T"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
G() { git -c protocol.file.allow=always -c user.name=t -c user.email=t@t "$@"; }
mkdir -p "$T/state" "$T/up/lib" "$T/src"
( cd "$T/up/lib" && G init -q -b main . && printf 'module example.com/lib\n\ngo 1.25\n' > go.mod && printf 'package lib\n\nconst Name = "from the shipped submodule"\n' > lib.go && G add -A && G commit -qm lib )
( cd "$T/src" && G init -q -b main . && mkdir -p svc submodules && printf 'module svc\n\ngo 1.25\n\nrequire example.com/lib v0.0.0\n\nreplace example.com/lib => ../submodules/lib\n' > svc/go.mod \
    && printf 'package main\n\nimport (\n\t"fmt"\n\t"example.com/lib"\n)\n\nfunc main() { fmt.Println(lib.Name) }\n' > svc/main.go && printf '/.audit/\n' > .gitignore \
    && G submodule add -q "$T/up/lib" submodules/lib && G add -A && G commit -qm root )
export DISPATCH_BUILDS_ROOT="$T/builds" DISPATCH_STATE_DIR="$T/state" DISPATCH_TRANSPORT=local DISPATCH_ALLOW_LOCAL=1 DISPATCH_DISK_OUT="$T/disk" DISPATCH_JOBS=2 \
       DISPATCH_HUB=0 DISPATCH_LONGOPS_DIR="$T/longops"
bd() { echo "$T/builds/$1"; }
sub() { # dir iteration lane budget extra-args... -- argv...
  local dir=$1 it=$2 lane=$3 bud=$4; shift 4
  local ex=(); while [ $# -gt 0 ] && [ "$1" != -- ]; do ex+=("$1"); shift; done; shift
  bash "$DSP" submit --purpose "build:tiny:$lane:$(bash "$DSP" snapshot "$dir"):$(bash "$DSP" argv-digest "$@"):primary:$it" --callback record-only --image IMG-GO --src "$dir" \
       --heartbeat 1 --no-progress-budget "$bud" --wallclock-cap 600 ${ex[@]+"${ex[@]}"} -- "$@"; }

# ---- r1: git snapshot, closure and tree cache through a real containerized Go build
BUILD=(sh -c 'cd /src/svc && go build -o /out/artifacts/hello .')
id=$(sub "$T/src" 1 go 120 --component svc -- "${BUILD[@]}"); bash "$DSP" wait "$id" 400 >/dev/null; wrc=$?
chk "r1 the callback is done and the build completed/succeeded" test "$wrc" = 0 -a "$(jq -r '.kind + "/" + .exit_class' "$(bd "$id")/terminal/state.json")" = completed/succeeded
chk "r1 (u) the binary prints the submodule's constant: the submodule tree was shipped into submodules/lib and the replace resolved in the container" test "$("$(bd "$id")/artifacts/hello" 2>&1)" = "from the shipped submodule"
chk "r1 the closure of component svc is the root and submodules/lib, recorded" test "$(jq -r .closure_repos "$(bd "$id")/submit.json")" = ".,submodules/lib"
n1=$(jq -r .transfer.objects "$(bd "$id")/submit.json")
id2=$(sub "$T/src" 2 go 120 --component svc -- "${BUILD[@]}"); bash "$DSP" wait "$id2" 400 >/dev/null
chk "r1 (w) the second submit of the unchanged checkout sends no object (first sent $n1) and completes" test "$n1" -gt 3 -a "$(jq -r .transfer.objects "$(bd "$id2")/submit.json")" = 0 -a "$(jq -r .exit_class "$(bd "$id2")/terminal/state.json")" = succeeded
for _ in $(seq 1 60); do [ "$(jq -r .state "$T/longops/ops/$id2.json" 2>/dev/null)" = complete ] && break; sleep 0.2; done   # test-side bounded wait: the op ends just after the callback
chk "r1 the registry op of each build ended complete" test "$(jq -r .state "$T/longops/ops/$id.json")" = complete -a "$(jq -r .state "$T/longops/ops/$id2.json")" = complete

# ---- r2: a quiet build that works is not read as flat; real cgroup CPU counter and memory peak
QUIET=(sh -c 'end=$(( $(date +%s) + 30 )); while [ "$(date +%s)" -lt "$end" ]; do :; done; echo done > /out/artifacts/q.txt')
mkdir -p "$T/src/web" 2>/dev/null; printf 'x\n' > "$T/src/web/index.js"; G -C "$T/src" add -A >/dev/null 2>&1; G -C "$T/src" commit -qm web >/dev/null 2>&1
id=$(sub "$T/src" 4 quiet 12 --component web --progress log+cpu -- "${QUIET[@]}"); bash "$DSP" wait "$id" 300 >/dev/null
chk "r2 a quiet build (nothing printed) burning CPU past its 12 s no-progress budget (a 30 s burn; the budget leaves room for a slow container start on a loaded host) is NOT hung with --progress log+cpu: completed/succeeded" test "$(jq -r '.kind + "/" + .exit_class' "$(bd "$id")/terminal/state.json")" = completed/succeeded
pk=$(jq -r 'select(.kind=="completed") | .peak_rss_bytes // empty' "$(bd "$id")/events.jsonl" | head -1)
chk "r2 (T115) the completed event carries a real peak_rss_bytes read from the container's cgroup (a positive integer: $pk)" test "${pk:-0}" -gt 0
echo "r2 heartbeat progress: $(jq -c 'select(.kind=="heartbeat") | .progress_offset' "$(bd "$id")/events.jsonl" | tr '\n' ' ')"
id=$(sub "$T/src" 5 quiet 12 --component web --progress log -- "${QUIET[@]}"); bash "$DSP" wait "$id" 300 >/dev/null
chk "r2 control: the SAME quiet build with --progress log ends build_progress_flat (the failure the option exists for)" test "$(jq -r .kind "$(bd "$id")/terminal/state.json")" = blocked-unavailable -a "$(jq -r .digest "$(bd "$id")/terminal/state.json")" = build_progress_flat
op="dispatch-$id"; for _ in $(seq 1 40); do [ -z "$(podman ps -q --filter "label=catalogizer.op_id=$op")" ] && break; sleep 0.5; done
chk "r2 the HUNG build's container was reaped by its label" test -z "$(podman ps -aq --filter "label=catalogizer.op_id=$op")"

echo "RESULT pass=$pass fail=$fail"
[ "$fail" = 0 ]
