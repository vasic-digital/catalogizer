#!/usr/bin/env bash
# test_dispatch_e2e.sh - T005a/T005b end-to-end proof: a tiny REAL containerized build through the dispatcher, the real emitter and
# scripts/containers/run_pinned.sh (rootless podman, IMG-GO, digest-pinned image from build/containers/images.lock.yaml).
# Host exception (declared, as the dispatcher's local transport requires): no build host qualifies yet (T006a is BLOCKED-ON ODG-07), so
# the "build host" is this host through DISPATCH_TRANSPORT=local DISPATCH_ALLOW_LOCAL=1; the container, the limits, the disk gate, the
# events, the digest and the callback are the real ones.
# Cases: e1 a succeeding Go build (artifact digest = the event's, build once on a second submit); e2 a failing build (build_failed, callback
# once, never a pass); e3 a real container cancelled by label (no container left); e4 the container ran rootless, digest-pinned, limited.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
DSP=${DISPATCH_SCRIPT:-$here/../dispatch.sh}
pass=0; fail=0
ok()  { pass=$((pass+1)); echo "ok   $1"; }
bad() { fail=$((fail+1)); echo "FAIL $1${2:+ -- $2}"; }
chk() { local w=$1; shift; if "$@"; then ok "$w"; else bad "$w"; fi; }
for t in podman jq python3; do command -v "$t" >/dev/null 2>&1 || { echo "FAIL dependency missing: $t"; echo "RESULT pass=0 fail=1"; exit 1; }; done
T=$(mktemp -d /tmp/tdspe.XXXXXX); trap 'rm -rf "$T"' EXIT
mkdir -p "$T/state" "$T/ok" "$T/bad" "$T/sleep"
printf 'package main\nimport "fmt"\nfunc main() { fmt.Println("hello from a containerized build") }\n' > "$T/ok/main.go"
printf 'module tiny\n\ngo 1.25\n' | tee "$T/ok/go.mod" "$T/bad/go.mod" "$T/sleep/go.mod" >/dev/null
printf 'package main\nfunc main() { undefined_symbol() }\n' > "$T/bad/main.go"
printf 'package main\nfunc main() {}\n' > "$T/sleep/main.go"
export DISPATCH_BUILDS_ROOT="$T/builds" DISPATCH_STATE_DIR="$T/state" DISPATCH_TRANSPORT=local DISPATCH_ALLOW_LOCAL=1 DISPATCH_DISK_OUT="$T/disk" DISPATCH_JOBS=2
sub() { local dir=$1 it=$2; shift 2; bash "$DSP" submit --purpose "build:tiny:go:$(bash "$DSP" snapshot "$dir"):$(bash "$DSP" argv-digest "$@"):primary:$it" --callback record-only \
          --image IMG-GO --src "$dir" --heartbeat 1 --no-progress-budget 120 --wallclock-cap 600 -- "$@"; }
bd() { echo "$T/builds/$1"; }
BUILD=(sh -c 'cd /src && go build -o /out/artifacts/hello .')

# ---- e1: success, artifact digest, build once
t0=$(date +%s)
id=$(sub "$T/ok" 1 "${BUILD[@]}"); echo "submitted $id"
bash "$DSP" wait "$id" 300 >/dev/null; wrc=$?; t1=$(date +%s)
chk "e1 wait returned done (callback done) in $((t1-t0)) s" test "$wrc" = 0
chk "e1 terminal completed / succeeded" test "$(jq -r '.kind + "/" + .exit_class' "$(bd "$id")/terminal/state.json")" = completed/succeeded
chk "e1 an executable artifact came back" test -x "$(bd "$id")/artifacts/hello"
want=$(jq -r 'select(.event == null and .kind == "completed") | .artifact_manifest_sha256' "$(bd "$id")/events.jsonl")
got=$(cd "$(bd "$id")/artifacts" && LC_ALL=C find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sed 's#  \./#  #' | sha256sum | cut -d' ' -f1)
chk "e1 content-addressed: the brought-back tree digest equals the digest in the signed completed event ($want)" test -n "$want" -a "$want" = "$got"
chk "e1 the binary prints the expected line when run (the artifact is the real build output)" test "$("$(bd "$id")/artifacts/hello" 2>&1)" = "hello from a containerized build"
chk "e1 callback applied exactly once" test "$(ls "$(bd "$id")/effects" | wc -l)" = 1
n1=$(wc -l < "$(bd "$id")/events.jsonl"); id2=$(sub "$T/ok" 1 "${BUILD[@]}" 2>/dev/null)
chk "e1 build once: the same purpose returns the same build and starts no second container" test "$id2" = "$id" -a "$(wc -l < "$(bd "$id")/events.jsonl")" = "$n1"
echo "e1 events:"; jq -c 'select(.event == null) | {seq,kind,progress_offset,stage,exit_class}' "$(bd "$id")/events.jsonl"

# ---- e2: a failing build is a build_failed, never a pass
id=$(sub "$T/bad" 1 "${BUILD[@]}"); bash "$DSP" wait "$id" 300 >/dev/null
chk "e2 a failing compile ends completed with exit_class build_failed" test "$(jq -r '.kind + "/" + .exit_class' "$(bd "$id")/terminal/state.json")" = completed/build_failed
chk "e2 callback once, no artifact kept" test "$(ls "$(bd "$id")/effects" | wc -l)" = 1 -a ! -d "$(bd "$id")/artifacts"
chk "e2 the log holds the compiler's real error text" grep -q 'undefined' "$(bd "$id")/remote/build.log"

# ---- e3: a real container is cancelled and reaped by label
SLEEP=(sh -c 'sleep 300')
id=$(sub "$T/sleep" 1 "${SLEEP[@]}"); op="dispatch-$id"
for _ in $(seq 1 60); do [ -n "$(podman ps -q --filter "label=catalogizer.op_id=$op")" ] && break; sleep 0.5; done   # test-side bounded wait for the container to exist
chk "e3 the container is running, labelled with the op id" test -n "$(podman ps -q --filter "label=catalogizer.op_id=$op")"
cid=$(podman ps -q --filter "label=catalogizer.op_id=$op" | head -1)
insp=$(podman inspect --format '{{.ImageName}}|{{.HostConfig.Memory}}|{{.HostConfig.PidsLimit}}|{{index .Config.Labels "project"}}|{{.Config.User}}|{{.EffectiveCaps}}' "$cid" 2>/dev/null)
echo "e3 container inspect: $insp"
chk "e4 digest-pinned image (a sha256 reference), memory and pids limits set, project label, non-root user, no effective capability" test "$(awk -F'|' '$1 ~ /@sha256:/ && $2 > 0 && $3 > 0 && $4 == "catalogizer" && $5 != "0" && $5 != "" && ($6 == "[]" || $6 == "<nil>") {print "yes"}' <<<"$insp")" = yes
chk "e4 rootless runtime: podman reports rootless" test "$(podman info --format '{{.Host.Security.Rootless}}')" = true
bash "$DSP" cancel "$id" >/dev/null 2>&1
bash "$DSP" wait "$id" 60 >/dev/null
for _ in $(seq 1 40); do [ -z "$(podman ps -q --filter "label=catalogizer.op_id=$op")" ] && break; sleep 0.5; done
chk "e3 cancelled: terminal kind cancelled, callback once" test "$(jq -r .kind "$(bd "$id")/terminal/state.json")" = cancelled -a "$(ls "$(bd "$id")/effects" | wc -l)" = 1
chk "e3 no container left (reaped by its label)" test -z "$(podman ps -aq --filter "label=catalogizer.op_id=$op")"

echo "RESULT pass=$pass fail=$fail"
[ "$fail" = 0 ]
