#!/usr/bin/env bash
# test_track_coverage.sh - T198 (RED first, against the old scripts/track-coverage.sh which swallows a failing package: doc05 7.4 A1). Oracle for the rewritten collector
# scripts/coverage/track-coverage.sh and for scripts/coverage/gocov_merge.py. NOTHING is compiled. The producers the collector depends on are run for REAL wherever they can be:
#  - scripts/build/dispatch.sh `status` (the REAL script, over build directories laid out the way dispatch.sh lays them out: submit.json, terminal/state.json, artifacts/, events.jsonl);
#  - tools/evidence/evrec (the REAL recorder, into a scratch ledger);
#  - scripts/test-in-container.sh + run_go.sh + run_pinned.sh (the REAL lane, `go list` in IMG-GO, SKIPped by name where podman or the image is absent);
#  - the fence gate, the git snapshot of the source and the Go module reader.
# Only `submit`, `group-seal`, `argv-digest` and `snapshot` of the dispatcher, the run half of TIC (it maps /src and /out to scratch directories and runs a "binary" that is a bash script
# writing a Go cover profile of the fixture's choosing) and the default recorder are shims. The split-lane contract asserted here:
#  (1) submit hands ONE build group per run to the dispatcher, one member per (lane, package) that belongs to the lane, the compile half
#      `sh -c 'cd /src/<app> && go test [-tags integration] -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test ./<rel>'`;
#  (2) collect takes each package's result only from the callback record of its build (`status`), and NEVER waits (the shim logs every dispatcher call: no `wait`);
#  (3) a brought-back binary is COPIED, the build tree and the copy are verified against the digest of the completed event, and only the copy runs in the run half;
#  (4) the per-package profiles are merged (counts summed per block) and the result goes through the REAL recorder;
#  (5) exit non-zero NAMING the package on: build_failed, infra_failed (its reason detail kept), a missing binary, a tampered build tree, a failing package, a panicking package,
#      an empty profile, a non-terminal member, a non-completed terminal kind; a reused --out, an unsealed or incomplete state and a skewed snapshot are refused up front.
# Usage: test_track_coverage.sh [--no-mutations]    Env: COLLECTOR (script under test), MUTATION_RECORD
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REPO="$(cd "$HERE/../../.." && pwd)"
COLLECTOR="${COLLECTOR:-$REPO/scripts/coverage/track-coverage.sh}"
GOCOV="$(dirname "$COLLECTOR")/gocov_merge.py"
PASSES=0; FAILS=0; SKIPS=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
skip() { SKIPS=$((SKIPS+1)); echo "SKIP: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required"; exit 2; }; done
T="$(mktemp -d "${TMPDIR:-/tmp}/trackcov-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -f "$COLLECTOR" ] || { bad "the collector $COLLECTOR does not exist"; echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=$SKIPS"; exit 1; }

SHIM="$T/shim"; mkdir -p "$SHIM"
cat >"$SHIM/dispatch.sh" <<'S'
#!/usr/bin/env bash
# the submit half of the dispatcher (shim); `status` is delegated to the REAL dispatch.sh by dispatch-wrap.sh. Logs every call.
echo "dispatch $*" >>"${SHIM_LOG:?}"
case "${1:-}" in
  snapshot) cat "$SHIM_STATE/snap" 2>/dev/null || printf '%064d\n' 1;;
  argv-digest) shift; printf '%s' "$*" | sha256sum | cut -d' ' -f1;;
  submit) [ -z "${SHIM_READS_STDIN:-}" ] || cat >/dev/null
          [ -z "${SHIM_SLOW:-}" ] || { n0=$(grep -c '^submit-id' "$SHIM_STATE/ids" 2>/dev/null || echo 0); [ "$n0" -lt 1 ] || sleep "$SHIM_SLOW"; }
          if [ -n "${SHIM_FAIL_PKG:-}" ] && printf '%s' "$*" | grep -q -- "/out/[a-z]*/$SHIM_FAIL_PKG.test"; then echo "submit refused" >&2; exit 20; fi
          n=$(grep -c '^submit-id' "$SHIM_STATE/ids" 2>/dev/null || echo 0); id="b-$(printf '%024d' $((n+1)))"; echo "submit-id $id" >>"$SHIM_STATE/ids"
          # remember which package this id is for: the argv after -- carries the -o /out/<lane>/<name>.test
          printf '%s\n' "$*" | grep -o '/out/[A-Za-z0-9_./-]*\.test' | head -1 >"$SHIM_STATE/pkg-$id"; echo "$id";;
  group-seal) echo sealed;;
  wait) echo "FOREGROUND WAIT" >>"$SHIM_LOG"; exit 3;;
  *) exit 0;;
esac
S
cat >"$SHIM/dispatch-wrap.sh" <<'S'
#!/usr/bin/env bash
# TC_DISPATCH: `status` is the REAL scripts/build/dispatch.sh reading the fixture builds root; everything else is the shim
case "${1:-}" in
  status) echo "dispatch $*" >>"${SHIM_LOG:?}"; DISPATCH_BUILDS_ROOT="$SHIM_BUILDS" exec bash "$REAL_DISPATCH" "$@";;
  *) exec bash "$SHIM_DIR/dispatch.sh" "$@";;
esac
S
cat >"$SHIM/tic.sh" <<'S'
#!/usr/bin/env bash
# the run-half shim: executes the command after -- in the shim "container" (the host, in a scratch dir tree), mapping /src and /out
echo "tic $*" >>"${SHIM_LOG:?}"
OUT=""; while [ $# -gt 0 ]; do case "$1" in --out) OUT="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
# the command after -- is `sh -c '<string>'`
[ "${1:-}" = sh ] && [ "${2:-}" = -c ] || { echo "tic shim: expected sh -c, got: $*" >&2; exit 64; }
cmd="$3"; cmd="${cmd//\/out\//$OUT/}"; cmd="${cmd//\/src\//$SHIM_SRC/}"
bash -c "$cmd"
S
cat >"$SHIM/recorder.sh" <<'S'
#!/usr/bin/env bash
echo "recorder $*" >>"${SHIM_LOG:?}"
while [ $# -gt 0 ] && [ "$1" != "--" ]; do shift; done; shift; exec "$@"
S
chmod +x "$SHIM"/*.sh
export TC_TEST_MODE=1 TC_DISPATCH="$SHIM/dispatch-wrap.sh" TC_TIC="$SHIM/tic.sh" TC_RECORDER="$SHIM/recorder.sh" SHIM_DIR="$SHIM" REAL_DISPATCH="$REPO/scripts/build/dispatch.sh"
unset TC_VERIFY

# ---- the recorded world: module `catalogizer`, packages alpha and beta (unit lane), statement blocks hand-counted ----
#   alpha profile: 4 blocks, 10 statements; beta profile: the same block of alpha.go plus 2 own blocks.
SRC="$T/src"; mkdir -p "$SRC/catalog-api/alpha" "$SRC/catalog-api/beta"
printf 'catalogizer/alpha\tcatalog-api/alpha\ncatalogizer/beta\tcatalog-api/beta\n' >"$T/packages.tsv"
mkbin() { # mkbin FILE profile-lines...: a "binary" that writes the profile (mode atomic) to the -test.coverprofile path
  local f="$1"; shift; { echo '#!/usr/bin/env bash'; echo 'for a in "$@"; do case "$a" in -test.coverprofile=*) P="${a#-test.coverprofile=}";; esac; done'
    echo "printf \"mode: ${MODE:-atomic}\\n\" >\"\$P\""; for l in "$@"; do echo "printf '%s\\n' '$l' >>\"\$P\""; done; echo 'echo "=== RUN   TestX"; echo "--- PASS: TestX (0.00s)"; echo PASS'; } >"$f"; chmod +x "$f"
}
newworld() { # newworld NAME: fresh state, records, artifacts and builds root
  W="$T/$1"; rm -rf "$W"; mkdir -p "$W/state" "$W/rec" "$W/art" "$W/out" "$W/builds"
  export SHIM_STATE="$W/state" SHIM_REC="$W/rec" SHIM_LOG="$W/log" SHIM_SRC="$SRC" SHIM_BUILDS="$W/builds" DISPATCH_BUILDS_ROOT="$W/builds"; : >"$SHIM_LOG"
  unset SHIM_READS_STDIN SHIM_SLOW SHIM_FAIL_PKG
}
rec() { # rec unit__alpha succeeded [kind reason]: the terminal record the dispatcher keeps for that member's build (terminal/state.json)
  local name="$1" cls="$2" kind="${3:-completed}" reason="${4:-}"
  jq -n --arg c "$cls" --arg k "$kind" --arg r "$reason" '{kind:$k, exit_class:$c, digest:$r}' >"$W/rec/$name.term"
}
tree_manifest() { ( cd "$1" 2>/dev/null && LC_ALL=C find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sed 's#  \./#  #' | sha256sum | cut -d' ' -f1 ); }
stage_builds() { # one build directory per member, laid out as dispatch.sh lays it out; the artifact tree is brought back from $W/art
  local lane name dir bid d f
  while IFS=$'\t' read -r lane name dir bid; do
    f="$W/rec/${lane}__${name}.term"; [ -f "$f" ] || continue
    d="$W/builds/$bid"; mkdir -p "$d/consumed" "$d/terminal"; printf '{"purpose":"p","host":"h","run_id":"r1"}\n' >"$d/submit.json"
    [ -z "${NO_TERMINAL:-}" ] || { rmdir "$d/terminal"; continue; }
    cp "$f" "$d/terminal/state.json"; printf 'done' >"$d/terminal/callback.state"
    if [ -f "$W/art/${lane}__${name}.test" ]; then mkdir -p "$d/artifacts/$lane"; cp -p "$W/art/${lane}__${name}.test" "$d/artifacts/$lane/$name.test"; fi
    mkdir -p "$d/artifacts"; printf '{"kind":"completed","artifact_manifest_sha256":"%s"}\n' "$(tree_manifest "$d/artifacts")" >"$d/events.jsonl"
    [ -z "${TAMPER:-}" ] || [ "$TAMPER" != "$lane/$name" ] || echo "tampered after the digest was recorded" >>"$d/artifacts/$lane/$name.test"
  done <"$W/state/members.tsv"
}
A_BLK='catalogizer/alpha/a.go:1.1,2.1 3 1'
A_BLK2='catalogizer/alpha/a.go:3.1,4.1 2 0'
A_BLK3='catalogizer/alpha/a.go:5.1,6.1 3 4'
A_BLK4='catalogizer/alpha/a.go:7.1,8.1 2 0'
B_OWN1='catalogizer/beta/b.go:1.1,2.1 4 2'
B_OWN2='catalogizer/beta/b.go:3.1,4.1 1 0'
goodworld() { newworld "$1"; mkbin "$W/art/unit__alpha.test" "$A_BLK" "$A_BLK2" "$A_BLK3" "$A_BLK4"
  # beta's binary also instruments alpha (-coverpkg=./...): it ran alpha's first block 1 more time and alpha's second block once
  mkbin "$W/art/unit__beta.test" 'catalogizer/alpha/a.go:1.1,2.1 3 1' 'catalogizer/alpha/a.go:3.1,4.1 2 1' "$B_OWN1" "$B_OWN2"
  rec unit__alpha succeeded; rec unit__beta succeeded; }
submit() { bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" --group tcg-test >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?; }
collect() { stage_builds; bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >"$W/collect.out" 2>"$W/collect.err"; COL_RC=$?; }

# ================= golden: two packages merge =================
goodworld g1; submit
check "submit: exit 0" "$SRC_RC" 0
check "submit: ONE build group per run, two members (one per package)" "$(grep -c '^dispatch submit ' "$SHIM_LOG")" 2
check "submit: every member names the same group" "$(grep '^dispatch submit ' "$SHIM_LOG" | grep -c -- '--group tcg-test')" 2
grep '^dispatch submit ' "$SHIM_LOG" | head -1 | grep -q -F 'go test -c -cover -covermode=atomic -coverpkg=./... -o /out/unit/alpha.test ./alpha' \
  && ok "submit: the compile half is go test -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test" || bad "submit: compile half argv wrong: $(grep '^dispatch submit ' "$SHIM_LOG" | head -1)"
grep '^dispatch submit ' "$SHIM_LOG" | head -1 | grep -q -F 'sh -c cd /src/catalog-api && go test' && ok "K6.3: the compile half runs INSIDE the module (cd /src/catalog-api): the repository root has no go.mod" || bad "K6.3: no cd into the module: $(grep '^dispatch submit ' "$SHIM_LOG" | head -1)"
grep -q 'dispatch group-seal tcg-test' "$SHIM_LOG" && ok "submit: the group is sealed after the last member" || bad "submit: the group was not sealed"
grep -q 'FOREGROUND WAIT' "$SHIM_LOG" && bad "submit: it waited in the foreground" || ok "submit: it does not wait (it returns after sealing)"
grep -q '^dispatch status' "$SHIM_LOG" && bad "submit: it read a result" || ok "submit: it reads no result"
check "K9.1: the state record is sealed, complete and carries a nonce" "$(jq -r '(.sealed|tostring) + ":" + (.expected_members|tostring) + ":" + (.submitted|tostring) + ":" + ((.nonce|length) > 0 | tostring)' "$W/state/state.json")" "true:2:2:true"
collect
check "collect: exit 0 on the golden case" "$COL_RC" 0
grep -q 'FOREGROUND WAIT' "$SHIM_LOG" && bad "collect: it waited in the foreground" || ok "collect: it never waits (results come from the callback records)"
check "K6.2 collect: one status read per member (the REAL dispatch.sh status)" "$(grep -c '^dispatch status' "$SHIM_LOG")" 2
[ -x "$W/out/run/unit/alpha.test" ] && ok "K8.3: the run half executed the COPY in the run directory" || bad "K8.3: no copy under the run directory"
grep -q "/out/unit/alpha.test -test.coverprofile=/out/unit/alpha.cover" "$SHIM_LOG" && ok "collect: the profile is written by -test.coverprofile from the package directory" || bad "collect: run half argv: $(grep '^tic ' "$SHIM_LOG" | head -1)"
M="$W/out/merged.cover"; [ -f "$M" ] && ok "collect: the merged profile exists" || bad "collect: no merged profile"
check "collect: merged profile header" "$(head -1 "$M" 2>/dev/null)" "mode: atomic"
check "collect: merged blocks (7: six distinct plus the header line)" "$(grep -c ':' "$M" 2>/dev/null)" 7
check "collect: the block both binaries instrument is counted once with the SUMMED count" "$(grep 'a.go:1.1,2.1' "$M" | awk '{print $3}')" 2
# hand-computed merge: alpha block1 (3 stmts) 1+1=2 covered; block2 (2) 0+1=1 covered; block3 (3) 4 covered; block4 (2) 0 not covered; beta b1 (4) covered; b2 (1) not
#   statements: 3+2+3+2+4+1 = 15; covered: 3+2+3+4 = 12  -> 80.00 %
check "collect: statements total (hand count 15)" "$(jq -r .statements "$W/out/result.json")" 15
check "collect: statements covered (hand count 12)" "$(jq -r .covered "$W/out/result.json")" 12
check "collect: percent" "$(jq -r .percent "$W/out/result.json")" "80.00"
check "collect: result status" "$(jq -r .status "$W/out/result.json")" ok
check "collect: per-package results are kept" "$(jq -r '.packages | map(.package) | sort | join(" ")' "$W/out/result.json")" "alpha beta"
grep -q '^recorder ' "$SHIM_LOG" && ok "collect: the result is written through the recorder" || bad "collect: the recorder was not used"
check "collect: the baseline record has the schema coverage-baseline/1" "$(jq -r .schema "$W/out/baseline.json" 2>/dev/null)" "coverage-baseline/1"
check "collect: the baseline names the instrument" "$(jq -r .instrument "$W/out/baseline.json" 2>/dev/null)" "go-cover-atomic-split"
check "K12.4: the baseline names the snapshot that was measured (the submit's), and no commit for a non-git source" "$(jq -r '(.source_snapshot|length|tostring) + ":" + (.source_commit|tostring)' "$W/out/baseline.json")" "64:null"
# GREEN x3 from three replays over the same records
P3=""; for i in 1 2 3; do goodworld "g3-$i"; submit; collect; P3="$P3 $(jq -r .percent "$W/out/result.json")"; done
check "GREEN x3: three replays give the identical percent" "$P3" " 80.00 80.00 80.00"

# ================= K6.1: the REAL recorder (evrec), into a scratch ledger =================
goodworld rr; submit; stage_builds
TC_RECORDER="$REPO/tools/evidence/evrec" EV_LEDGER="$W/ledger.jsonl" bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >"$W/collect.out" 2>"$W/collect.err"; COL_RC=$?
check "K6.1: with the REAL evrec the collector exits 0 (it ended with 'the recorder did not record the result' for every GREEN run: five refusals)" "$COL_RC" 0
check "K6.1: the ledger holds one entry: item CAT-123, BASELINE, pass, evidence class runtime, target class go_binary" "$(jq -r '[.item, .polarity, .verdict, .evidence_class, .target_class] | join(",")' "$W/ledger.jsonl" 2>/dev/null | tail -1)" "CAT-123,BASELINE,pass,runtime,go_binary"
check "K6.1: the entry's target fingerprint is the sha256 of result.json" "$(jq -r .target_fingerprint "$W/ledger.jsonl" 2>/dev/null | tail -1)" "$(sha256sum <"$W/out/result.json" | cut -d' ' -f1)"
mkdir -p "$W/ctl"; echo '{}' >"$W/ctl/result.json"
EV_LEDGER="$W/ctl/ledger.jsonl" "$REPO/tools/evidence/evrec" run T201 GREEN 1 runtime coverage-baseline -- python3 -I "$GOCOV" blocks /dev/null >/dev/null 2>"$W/ctl/err"; r_old=$?
[ "$r_old" != 0 ] && ok "K6.1 control: the argv the old collector passed (item T201, GREEN, class runtime, a ref that does not resolve) is refused by the real evrec (exit $r_old): the leg can fail" || bad "K6.1 control: the old argv was accepted"
bash "$COLLECTOR" collect --state "$W/state" --out "$T/o-noitem" >/dev/null 2>"$W/err"; [ "$?" = 2 ] && grep -q -- '--item is required' "$W/err" && ok "K6.1: --item is mandatory (usage error 2)" || bad "K6.1: collect without --item: $(cat "$W/err")"
bash "$COLLECTOR" collect --state "$W/state" --out "$T/o-badtem" --item T201 >/dev/null 2>"$W/err"; [ "$?" = 2 ] && grep -q 'not a register item id' "$W/err" && ok "K6.1: --item T201 (a task id, not a register item) is refused (usage error 2)" || bad "K6.1: bad item accepted: $(cat "$W/err")"

# ================= K6.2: layout and fields of the real dispatcher =================
goodworld ly; submit; stage_builds
bid1="$(cut -f4 "$W/state/members.tsv" | head -1)"
DISPATCH_BUILDS_ROOT="$W/builds" bash "$REPO/scripts/build/dispatch.sh" status "$bid1" >"$W/status.json" 2>/dev/null
check "K6.2: the REAL dispatch.sh status carries no artifact_dir or artifact_name (the old collector read both and ended with binary_missing)" "$(jq -r '[has("artifact_dir"), has("artifact_name")] | any | tostring' "$W/status.json")" false
check "K6.2: it reports the terminal as kind / exit_class / reason" "$(jq -r '.terminal | keys | join(",")' "$W/status.json")" "exit_class,kind,reason"
[ ! -e "$REPO/scripts/build/verify_artifact.sh" ] && ok "K6.2: scripts/build/verify_artifact.sh does not exist (so the collector must not call it: it verifies against the completed event's digest)" || bad "K6.2: verify_artifact.sh now exists: the collector should use it"
bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >/dev/null 2>"$W/collect.err"; check "K6.2: the artifact is found at <builds root>/<build id>/artifacts/<lane>/<pkg>.test (exit 0)" "$?" 0

# ================= each failure names the package and exits non-zero =================
failcase() { # failcase NAME PATTERN setup-fn
  local name="$1" pat="$2" fn="$3"; goodworld "f-$name"; submit; $fn; collect
  [ "$COL_RC" != 0 ] && ok "$name: exit non-zero ($COL_RC)" || bad "$name: exit 0 (a failing package was swallowed)"
  grep -q "$pat" "$W/collect.err" && ok "$name: stderr names the package and the reason ($pat)" || bad "$name: stderr lacks '$pat': $(cat "$W/collect.err")"
  [ "$(jq -r .status "$W/out/result.json" 2>/dev/null)" = failed ] && ok "$name: the result record says failed" || bad "$name: result status is not failed"
  [ ! -e "$W/out/baseline.json" ] && ok "$name: no baseline was recorded" || bad "$name: a baseline exists for a failed run"
}
f_build() { rec unit__beta build_failed completed "go: undefined: Foo"; }
failcase build_failed 'beta (unit): build_failed' f_build
f_infra() { rec unit__beta infra_failed completed "snapshot_mismatch"; }
failcase infra_failed 'beta (unit): infra_failed (snapshot_mismatch)' f_infra
f_missing() { rm -f "$W/art/unit__beta.test"; }
failcase binary_missing 'beta (unit): binary_missing' f_missing
f_tamper() { export TAMPER=unit/beta; }
failcase verify_refused 'beta (unit): verify_refused' f_tamper; unset TAMPER
f_failing() { { echo '#!/usr/bin/env bash'; echo 'echo "--- FAIL: TestBeta (0.00s)"; echo FAIL'; echo 'exit 1'; } >"$W/art/unit__beta.test"; }
failcase package_failed 'beta (unit): package_failed' f_failing
f_panic() { { echo '#!/usr/bin/env bash'; echo 'echo "panic: runtime error: index out of range"; echo "goroutine 1 [running]:"'; echo 'exit 2'; } >"$W/art/unit__beta.test"; }
failcase package_panicked 'beta (unit): package_panicked' f_panic
f_panic0() { { echo '#!/usr/bin/env bash'; echo 'for a in "$@"; do case "$a" in -test.coverprofile=*) printf "mode: atomic\n" >"${a#-test.coverprofile=}";; esac; done; echo "panic: boom"; exit 0'; } >"$W/art/unit__beta.test"; }
failcase panic_with_exit_zero 'beta (unit): package_panicked' f_panic0
f_empty() { { echo '#!/usr/bin/env bash'; echo 'for a in "$@"; do case "$a" in -test.coverprofile=*) printf "mode: atomic\n" >"${a#-test.coverprofile=}";; esac; done; echo PASS'; } >"$W/art/unit__beta.test"; }
failcase profile_empty 'beta (unit): profile_empty' f_empty
f_nocallback() { rm -f "$W/rec/unit__beta.term"; }
failcase callback_record_missing 'beta (unit): callback_record_missing' f_nocallback
f_cancel() { rec unit__beta cancelled completed ""; }
failcase cancelled 'beta (unit): cancelled' f_cancel
f_timedout() { rec unit__beta succeeded timed_out ""; }
failcase timed_out_with_succeeded_class 'beta (unit): timed_out' f_timedout
f_notterm() { export NO_TERMINAL=1; }
failcase not_terminal 'beta (unit): not_terminal (state=' f_notterm; unset NO_TERMINAL
# one failing package does not hide the other: alpha's result is still computed and the failing package is the only one named
goodworld one; submit; f_failing; collect
grep -q 'alpha' "$W/collect.err" && bad "one failing package: the healthy package alpha is named as a failure" || ok "one failing package: only the failing package is named"
check "one failing package: the healthy package alpha still ran" "$(grep -c '^tic ' "$SHIM_LOG")" 2
# K8.3: an artifact that cannot be copied is a named failure, never the previous run's binary
goodworld cp; submit; stage_builds; chmod 000 "$W/builds"/*/artifacts/unit/beta.test 2>/dev/null
bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >"$W/collect.out" 2>"$W/collect.err"; COL_RC=$?; chmod 644 "$W/builds"/*/artifacts/unit/beta.test 2>/dev/null
if [ "$(id -u)" = 0 ]; then skip "K8.3: running as root, an unreadable file is still readable"; else
  [ "$COL_RC" != 0 ] && grep -q 'beta (unit): copy_failed' "$W/collect.err" && ok "K8.3: an unreadable artifact is copy_failed for that package (the copy status is checked)" || bad "K8.3: unreadable artifact (rc $COL_RC): $(cat "$W/collect.err")"; fi
# K8.4 / K8.5: a reused --out is refused (a stale result.json, profile or log could be recorded as this run's)
goodworld ro; submit; collect; first_pct="$(jq -r .percent "$W/out/result.json")"
bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >"$W/collect2.out" 2>"$W/collect2.err"; COL2=$?
[ "$COL2" != 0 ] && grep -q 'out_not_empty' "$W/collect2.err" && ok "K8.4: collect into a --out that already holds a run is refused (out_not_empty)" || bad "K8.4: reused --out accepted (rc $COL2): $(cat "$W/collect2.err")"
check "K8.4: and the first run's result is untouched" "$(jq -r .percent "$W/out/result.json")" "$first_pct"
# K8.4: the merge fails (an atomic profile and a count profile): the status is failed and NO baseline is written (CM20)
goodworld mg; MODE=count mkbin "$W/art/unit__beta.test" 'catalogizer/beta/b.go:1.1,2.1 4 2'; submit; collect
[ "$COL_RC" != 0 ] && grep -q 'merge: profiles could not be merged' "$W/collect.err" && ok "CM20: atomic + count profiles: the merge failure is an error and the run fails" || bad "CM20: merge failure swallowed (rc $COL_RC): $(cat "$W/collect.err")"
[ ! -e "$W/out/baseline.json" ] && [ "$(jq -r .status "$W/out/result.json" 2>/dev/null)" = failed ] && ok "CM20: result.json says failed and no baseline was written" || bad "CM20: status $(jq -r .status "$W/out/result.json" 2>/dev/null), baseline $([ -e "$W/out/baseline.json" ] && echo present || echo absent)"
# K9.1: an unsealed state is refused by collect
goodworld us; SHIM_FAIL_PKG=beta submit
[ "$SRC_RC" != 0 ] && grep -q 'member_submit_failed' "$W/submit.err" && ok "K9.1: a failed member submission fails the submit (member_submit_failed)" || bad "K9.1: submit rc $SRC_RC: $(cat "$W/submit.err")"
check "K9.1: the state is NOT sealed" "$(jq -r .sealed "$W/state/state.json")" false
check "K9.1: members.tsv holds alpha only" "$(cut -f2 "$W/state/members.tsv" | tr '\n' ' ')" "alpha "
collect; [ "$COL_RC" != 0 ] && grep -q 'group_not_sealed' "$W/collect.err" && ok "K9.1: collect over the unsealed state is refused (group_not_sealed), not recorded as a baseline over alpha alone" || bad "K9.1: unsealed state collected (rc $COL_RC): $(cat "$W/collect.err")"
[ ! -e "$W/out/baseline.json" ] && ok "K9.1: no baseline" || bad "K9.1: a baseline exists"
goodworld sc; submit; jq '.expected_members = 3' "$W/state/state.json" >"$W/state/x" && mv "$W/state/x" "$W/state/state.json"; collect
[ "$COL_RC" != 0 ] && grep -q 'state_incomplete' "$W/collect.err" && ok "K9.1: a state that expects 3 members and holds 2 is refused (state_incomplete)" || bad "K9.1: incomplete state collected (rc $COL_RC): $(cat "$W/collect.err")"
rm -f "$W/state/state.json"; collect; [ "$COL_RC" != 0 ] && grep -q 'state_incomplete' "$W/collect.err" && ok "K9.1: a missing state.json is refused (state_incomplete)" || bad "K9.1: missing state accepted"
# K9.1: submit interrupted with INT leaves an unsealed, interrupted state
goodworld ii; set -m; ( SHIM_SLOW=2 bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" --group tcg-int >"$W/submit.out" 2>"$W/submit.err"; echo $? >"$W/submit.rc" ) &
SUBPID=$!; set +m; sleep 1.2; CP="$(pgrep -P "$SUBPID" | head -1)"; [ -z "$CP" ] || kill -INT "$CP" 2>/dev/null; wait "$SUBPID" 2>/dev/null
check "K9.1: a submit interrupted by INT exits 130" "$(cat "$W/submit.rc" 2>/dev/null)" 130
check "K9.1: and leaves sealed=false interrupted=true" "$(jq -r '(.sealed|tostring) + ":" + (.interrupted|tostring)' "$W/state/state.json" 2>/dev/null)" "false:true"
collect; [ "$COL_RC" != 0 ] && grep -q 'group_not_sealed' "$W/collect.err" && ok "K9.1: and collect refuses it" || bad "K9.1: interrupted state collected (rc $COL_RC)"
# K9.1: a state directory that already holds a submitted group is refused by submit
goodworld sr; submit; submit; [ "$SRC_RC" != 0 ] && grep -q 'state_not_empty' "$W/submit.err" && ok "K9.1: a second submit into the same --state is refused (state_not_empty)" || bad "K9.1: state reused (rc $SRC_RC): $(cat "$W/submit.err")"
# K6.4 b: the dispatcher call gets /dev/null as stdin: with a stdin that never closes (a held-open fifo) a call that reads it would hang for ever
mkfifo "$T/hold.fifo"; exec 7<>"$T/hold.fifo"
goodworld sg; SHIM_READS_STDIN=1 timeout 30 bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" --group tcg-sg <&7 >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
exec 7<&-
check "K6.4: a dispatcher call that reads an OPEN stdin does not hang the submit (exit 0, not the timeout's 124)" "$SRC_RC" 0
# K9.2: a members.tsv whose last line has no newline: `read` drops that member silently, so the reconcile must refuse (member_unaccounted)
goodworld nl; submit; printf '%s' "$(cat "$W/state/members.tsv")" >"$W/state/members.tsv"; collect
[ "$COL_RC" != 0 ] && grep -q 'member_unaccounted' "$W/collect.err" && ok "K9.2: a member the loop dropped (no trailing newline) is refused as member_unaccounted, never collected as a subset" || bad "K9.2: dropped member not detected (rc $COL_RC): $(cat "$W/collect.err")"
# a failed collection never reaches the recorder and writes no baseline
goodworld fb; rec unit__beta build_failed completed "compile error"; submit; collect
[ "$COL_RC" != 0 ] && ! grep -q '^recorder ' "$SHIM_LOG" && [ ! -e "$W/out/baseline.json" ] && ok "status: a collection with a failed package never calls the recorder and writes no baseline" || bad "status: failed collection reached the recorder (rc $COL_RC): $(grep recorder "$SHIM_LOG")"
# the build tags no lane compiles and the packages without tests travel from gaps.json into result.json and the baseline
goodworld gp; submit; printf '{"uncompiled_build_tags":{"e2e_binary":3},"packages_without_tests":["catalogizer/nolib"]}\n' >"$W/state/gaps.json"; collect
check "gaps: result.json carries the uncompiled build tags from gaps.json" "$(jq -c .uncompiled_build_tags "$W/out/result.json")" '{"e2e_binary":3}'
check "gaps: and the baseline carries them (a reader of the baseline sees what the figure does not cover)" "$(jq -c '[.uncompiled_build_tags, .packages_without_tests]' "$W/out/baseline.json")" '[{"e2e_binary":3},["catalogizer/nolib"]]'
# a summary the figure cannot be computed from (no module path) fails the collection and says so
goodworld sm; submit; TC_GOMOD="$T/no-such-go.mod"; export TC_GOMOD; collect; unset TC_GOMOD
[ "$COL_RC" != 0 ] && grep -q 'summary: the figure could not be computed' "$W/collect.err" && ok "summary: a figure that cannot be computed (module_unknown) fails the collection and names the summary step" || bad "summary: failure not reported (rc $COL_RC): $(cat "$W/collect.err")"
# the real recorder keeps the test source of the baseline entry (the BASELINE polarity does not demand it, so only the fingerprint shows it)
goodworld ts; submit; stage_builds; TC_RECORDER="$REPO/tools/evidence/evrec" EV_LEDGER="$W/ledger.jsonl" bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >/dev/null 2>&1
mkdir -p "$W/ctl2"; EV_LEDGER="$W/ctl2/ledger.jsonl" "$REPO/tools/evidence/evrec" run CAT-123 BASELINE 1 go_binary "$W/out/result.json" --evidence-class runtime --oracle invariant --oracle-independent --test-source "$GOCOV" -- python3 -I "$GOCOV" blocks /dev/null >/dev/null 2>&1
EV_LEDGER="$W/ctl2/ledger-nosrc.jsonl" "$REPO/tools/evidence/evrec" run CAT-123 BASELINE 1 go_binary "$W/out/result.json" --evidence-class runtime --oracle invariant --oracle-independent -- python3 -I "$GOCOV" blocks /dev/null >/dev/null 2>&1
check "recorder: the collector's entry has the test_fingerprint of (python + gocov_merge.py), the control with the source declared" "$(jq -r .test_fingerprint "$W/ledger.jsonl" | tail -1)" "$(jq -r .test_fingerprint "$W/ctl2/ledger.jsonl" | tail -1)"
check "recorder control: the same entry WITHOUT the declared source has another test_fingerprint (the leg can fail)" "$([ "$(jq -r .test_fingerprint "$W/ledger.jsonl" | tail -1)" != "$(jq -r .test_fingerprint "$W/ctl2/ledger-nosrc.jsonl" | tail -1)" ] && echo differ)" differ
# K6.4: a dispatcher call that reads stdin must not eat the loop's package list
printf 'catalogizer/alpha\tcatalog-api/alpha\ncatalogizer/beta\tcatalog-api/beta\ncatalogizer/gamma\tcatalog-api/gamma\ncatalogizer/delta\tcatalog-api/delta\n' >"$T/packages4.tsv"
goodworld si; SHIM_READS_STDIN=1 bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages4.tsv" --state "$W/state" --group tcg-stdin >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
check "K6.4: with a dispatcher call that reads stdin all 4 packages are still submitted (was 1 of 4 with exit 0)" "$(grep -c . "$W/state/members.tsv")/$SRC_RC" "4/0"
# K9.2 / reconcile: a package list line the loop cannot place is not silently dropped: expected equals submitted
check "K9.1: expected_members equals the packages x lane memberships (4)" "$(jq -r .expected_members "$W/state/state.json")" 4
# K6.3 c: relative --out / --state from another directory
goodworld rl; ( cd "$W" && bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state rel-state --group tcg-rel >"$W/submit.out" 2>"$W/submit.err" ); r1=$?
check "K6.3 c: submit with a RELATIVE --state exits 0 (the state is made absolute)" "$r1" 0
[ -s "$W/rel-state/members.tsv" ] && ok "K6.3 c: and the state landed in <cwd>/rel-state" || bad "K6.3 c: no relative state at $W/rel-state"
rm -rf "$W/state"; cp -r "$W/rel-state" "$W/state"; collect_rel() { stage_builds; ( cd "$W" && bash "$COLLECTOR" collect --state state --out rel-out --item CAT-123 >"$W/collect.out" 2>"$W/collect.err" ); COL_RC=$?; }
rec unit__alpha succeeded; rec unit__beta succeeded; mkbin "$W/art/unit__alpha.test" "$A_BLK"; mkbin "$W/art/unit__beta.test" "$B_OWN1"; collect_rel
check "K6.3 c: collect with a RELATIVE --out and --state exits 0 (the runner refuses a relative --out: out_dir_not_absolute)" "$COL_RC" 0
[ "$COL_RC" = 0 ] || sed 's/^/  collect.err: /' "$W/collect.err" | head -8
# K6.3 b: the collector works from any directory (it moves to the repository root, the one tree of the lane)
goodworld cw; submit; stage_builds; ( cd "$T" && bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item CAT-123 >"$W/collect.out" 2>"$W/collect.err" ); check "K6.3 b: collect started from an unrelated directory works" "$?" 0
# K12.4: a git source names ITS commit (not the commit of the directory the collector was started in)
goodworld gs; ( cd "$SRC" && git init -q . 2>/dev/null && git -c user.email=t@t -c user.name=t add -A 2>/dev/null; git -c user.email=t@t -c user.name=t commit -q --allow-empty -m s 2>/dev/null ); submit; collect
check "K12.4: the baseline's source_commit is the commit of the SOURCE tree" "$(jq -r .source_commit "$W/out/baseline.json")" "$(git -C "$SRC" rev-parse HEAD)"
check "K12.4: and it is not the commit of the repository the collector lives in" "$([ "$(jq -r .source_commit "$W/out/baseline.json")" != "$(git -C "$REPO" rev-parse HEAD)" ] && echo different)" different
rm -rf "$SRC/.git"
# K12.4 / snapshot skew: the source changed between submit and collect
goodworld sk; submit; printf '%064d\n' 7 >"$W/state/snap"; collect
[ "$COL_RC" != 0 ] && grep -q 'snapshot_skew' "$W/collect.err" && ok "K12.4: a source that changed since submit is refused (snapshot_skew): binaries from one tree must not run against another" || bad "K12.4: skewed snapshot collected (rc $COL_RC): $(cat "$W/collect.err")"
# lane membership (K6.3 d): the integration lane holds what the real gating says
LS="$T/lsrc"; rm -rf "$LS"; mkdir -p "$LS/catalog-api/alpha" "$LS/catalog-api/tagged" "$LS/catalog-api/tests/integration/x" "$LS/catalog-api/e2e"
printf 'package alpha\n' >"$LS/catalog-api/alpha/a_test.go"; printf '//go:build integration\n\npackage tagged\n' >"$LS/catalog-api/tagged/t_test.go"; printf 'package x\n' >"$LS/catalog-api/tests/integration/x/x_test.go"
printf '//go:build e2e_binary\n\npackage e2e\n' >"$LS/catalog-api/e2e/e_test.go"; printf '//go:build linux\n\npackage alpha\n' >"$LS/catalog-api/alpha/l_test.go"
printf 'catalogizer/alpha\tcatalog-api/alpha\ncatalogizer/tagged\tcatalog-api/tagged\ncatalogizer/tests/integration/x\tcatalog-api/tests/integration/x\ncatalogizer/e2e\tcatalog-api/e2e\n' >"$T/packages-l.tsv"
printf 'catalogizer/alpha\t/src/catalog-api/alpha\ncatalogizer/tagged\t/src/catalog-api/tagged\ncatalogizer/tests/integration/x\t/src/catalog-api/tests/integration/x\ncatalogizer/e2e\t/src/catalog-api/e2e\ncatalogizer/nolib\t/src/catalog-api/nolib\n' >"$T/all-l.tsv"
goodworld ln; mkdir -p "$W/state"; cp "$T/all-l.tsv" "$W/state/all_packages.tsv"
bash "$COLLECTOR" submit --app catalog-api --src "$LS" --lanes "unit integration" --packages "$T/packages-l.tsv" --state "$W/state" --group tcg-l >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
check "K6.3 d: submit with two lanes exits 0" "$SRC_RC" 0
check "K6.3 d: lane membership: unit holds alpha and e2e, integration holds tagged and tests/integration/x (4 members, no package twice)" "$(awk -F'\t' '{print $1":"$2}' "$W/state/members.tsv" | sort | tr '\n' ' ')" "integration:tagged integration:tests__integration__x unit:alpha unit:e2e "
grep '^dispatch submit ' "$SHIM_LOG" | grep -F -- '-o /out/integration/tagged.test' | grep -q -F -- 'go test -tags integration -c' && ok "K6.3 d: the package whose test files carry //go:build integration is compiled with -tags integration" || bad "K6.3 d: -tags integration missing for tagged: $(grep '^dispatch submit ' "$SHIM_LOG" | grep tagged)"
grep '^dispatch submit ' "$SHIM_LOG" | grep -F -- '-o /out/integration/tests__integration__x.test' | grep -q -F -- '-tags integration' && bad "K6.3 d: a directory-gated integration package got -tags integration (no file carries the tag)" || ok "K6.3 d: a package gated by directory only gets no -tags integration"
check "K6.3 d: the build tags no lane compiles are listed as a gap (e2e_binary yes; integration, linux no)" "$(jq -r '.uncompiled_build_tags | keys | join(",")' "$W/state/gaps.json")" "e2e_binary"
check "K6.3 d: and the packages with no test are listed (nolib)" "$(jq -r '.packages_without_tests | join(",")' "$W/state/gaps.json")" "catalogizer/nolib"

# ================= I6 / K2.6: the fence is APPLIED, SNAPSHOTTED and gated =================
printf 'catalogizer/alpha\t/src/catalog-api/alpha\ncatalogizer/beta\t/src/catalog-api/beta\n' >"$T/packages-abs.tsv"
goodworld ia; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages-abs.tsv" --state "$W/state" --group tcg-abs >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
check "I10: submit with absolute go-list directories exits 0" "$SRC_RC" 0
grep '^dispatch submit ' "$SHIM_LOG" | head -1 | grep -q -F -- '-o /out/unit/alpha.test ./alpha' && ok "I10: the compile target is ./alpha, never ./src/catalog-api/alpha" || bad "I10: compile target wrong: $(grep '^dispatch submit ' "$SHIM_LOG" | head -1)"
check "I10: members.tsv records the checkout-relative directory (the run half's cd /src/<dir>)" "$(cut -f3 "$W/state/members.tsv" | head -1)" "catalog-api/alpha"
printf 'catalogizer/alpha\t/tmp/elsewhere/alpha\n' >"$T/packages-out.tsv"
goodworld ib; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages-out.tsv" --state "$W/state" --group tcg-out >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
[ "$SRC_RC" != 0 ] && grep -q 'package_dir_outside_source' "$W/submit.err" && ok "I10: an absolute directory outside /src is refused (package_dir_outside_source)" || bad "I10: outside-/src directory accepted ($SRC_RC): $(cat "$W/submit.err")"
check "I10: that refusal submitted no build" "$(grep -c '^dispatch submit ' "$SHIM_LOG")" 0
goodworld ex
mkbin "$W/art/unit__alpha.test" "$A_BLK" "$A_BLK2" "$A_BLK3" "$A_BLK4" 'catalogizer/tests/mocks/ftp/s.go:1.1,2.1 7 0'
submit; collect
check "I6: exit 0 with a fenced file in the profile" "$COL_RC" 0
check "I6: statements exclude the fenced file's 7 (hand count 15, was 22 before the fence was applied)" "$(jq -r .statements "$W/out/result.json")" 15
check "I6: the percent is the unfenced 80.00" "$(jq -r .percent "$W/out/result.json")" "80.00"
check "I6: the record names the excluded file" "$(jq -r '.excluded.files | join(",")' "$W/out/result.json")" "tests/mocks/ftp/s.go"
check "I6: the record counts the excluded statements" "$(jq -r '.excluded.statements' "$W/out/result.json")" 7
check "I6: the baseline record carries the exclusion" "$(jq -r '.excluded.statements' "$W/out/baseline.json" 2>/dev/null)" 7
check "K8.1: the baseline records the sha256 of the fence that was APPLIED (not its path)" "$(jq -r '.scope.exclusion_list | (if type=="object" then .sha256 else . end) | tostring | length' "$W/out/baseline.json")" 64
check "K8.1: and it is the sha256 of the snapshot taken into the run directory" "$(jq -r '.scope.exclusion_list | (if type=="object" then .sha256 else . end)' "$W/out/baseline.json")" "$(sha256sum <"$W/out/fence/catalog-api.yaml" | cut -d' ' -f1)"
# K2.6: no fence file for the application: a recorded `none`, not a crash
mkdir -p "$T/nofence"; goodworld nf; submit; TC_FENCE_DIR="$T/nofence"; export TC_FENCE_DIR; collect; unset TC_FENCE_DIR
check "K2.6: an application with no fence file collects (exit 0) and says fence: none" "$COL_RC/$(jq -r .fence "$W/out/result.json")" "0/none"
check "K2.6: and the baseline's exclusion_list is none, with no path that does not exist" "$(jq -r .scope.exclusion_list "$W/out/baseline.json")" "none"
# I6: a fence the T200 gate refuses blocks the figure (the gate runs before the fence is applied)
mkdir -p "$T/badfence"; cat >"$T/badfence/catalog-api.yaml" <<'Y'
schema: coverage-exclusions/1
application: catalog-api
exclusions:
  - path: "poison/first_party_without_item.go"
    class: first-party
    justification: "a first-party entry written without a tracked item, the case the gate must refuse"
Y
goodworld bf; submit; TC_FENCE_DIR="$T/badfence"; export TC_FENCE_DIR; collect; unset TC_FENCE_DIR
[ "$COL_RC" != 0 ] && grep -q 'exclusion fence refused by the T200 gate' "$W/collect.err" && ok "I6: a fence the gate refuses fails the collection (exit $COL_RC) and says why" || bad "I6: refused fence was accepted (rc $COL_RC): $(cat "$W/collect.err")"
[ "$(jq -r .status "$W/out/result.json" 2>/dev/null)" = failed ] && ok "I6: and result.json says failed (no baseline over an unchecked scope)" || bad "I6: result status is not failed after a refused fence"

# ================= gocov_merge.py, run for real =================
GM="$T/gm"; mkdir -p "$GM"
printf 'mode: atomic\ncatalogizer/p/a.go:1.1,2.1 3 1\ncatalogizer/p/a.go:3.1,4.1 2 0\n' >"$GM/p1.cover"
printf 'mode: atomic\ncatalogizer/p/a.go:1.1,2.1 3 2\ncatalogizer/p/a.go:3.1,4.1 2 5\n' >"$GM/p2.cover"
python3 -I "$GOCOV" merge "$GM/p1.cover" --out "$GM/m.cover" "$GM/p2.cover" >"$GM/m.out" 2>"$GM/m.err"; r=$?
check "K11.5: merge p1 --out F p2 sums BOTH profiles (a[2:o] dropped p1): block 1 counted 3, block 2 counted 5" "$r:$(awk 'NR>1{print $3}' "$GM/m.cover" 2>/dev/null | tr '\n' ',')" "0:3,5,"
printf 'mode: count\ncatalogizer/p/a.go:1.1,2.1 3 1\n' >"$GM/pc.cover"
python3 -I "$GOCOV" merge --out "$GM/m2.cover" "$GM/p1.cover" "$GM/pc.cover" >"$GM/m2.out" 2>"$GM/m2.err"; r=$?
[ "$r" = 1 ] && grep -q 'mode count differs from atomic' "$GM/m2.err" && ok "CM21: atomic + count profiles: merge exits 1 'mode count differs from atomic'" || bad "CM21: mode mix merged (rc $r): $(cat "$GM/m2.err")"
printf 'mode: atomic\ncatalogizer/p/a.go:1.1,2.1 5 1\n' >"$GM/p5.cover"
python3 -I "$GOCOV" merge --out "$GM/m3.cover" "$GM/p1.cover" "$GM/p5.cover" >"$GM/m3.out" 2>"$GM/m3.err"; r=$?
[ "$r" = 1 ] && grep -q 'has two statement counts' "$GM/m3.err" && ok "CM22: one block with 3 and with 5 statements: merge exits 1 'has two statement counts'" || bad "CM22: statement mismatch merged (rc $r): $(cat "$GM/m3.err")"
python3 -I "$GOCOV" merge --out >/dev/null 2>"$GM/e"; check "K11.1: merge --out with no value is a usage error (2)" "$?" 2
printf 'schema: coverage-exclusions/1\napplication: catalog-api\nexclusions:\n  - path: "p/a.go"\n    class: generated-code\n    justification: "x"\n' >"$GM/fence.yaml"
python3 -I "$GOCOV" summary --profile "$GM/p1.cover" --exclusions "$GM/fence.yaml" >/dev/null 2>"$GM/e"; r=$?
[ "$r" = 3 ] && grep -q 'module_unknown' "$GM/e" && ok "K2.5: --exclusions with no module is REFUSED (3 module_unknown); the old code applied nothing and printed a figure" || bad "K2.5: no module (rc $r): $(cat "$GM/e")"
for form in 'module catalogizer' 'module "catalogizer"' 'module catalogizer // the api' 'module catalogizer\r'; do
  printf '%b\ngo 1.25\n' "$form" >"$GM/go.mod"
  python3 -I "$GOCOV" summary --profile "$GM/p1.cover" --go-mod "$GM/go.mod" --exclusions "$GM/fence.yaml" >"$GM/s.json" 2>"$GM/e"; r=$?
  check "K2.5: go.mod form '$form': the module is read and the fence applied (excluded statements 5, figure 0)" "$r/$(jq -r '(.excluded.statements|tostring) + "/" + (.statements|tostring)' "$GM/s.json" 2>/dev/null)" "0/5/0"
done
python3 -I "$GOCOV" summary --profile "$GM/p1.cover" --go-mod "$GM/go.mod" --exclusions "$GM/does-not-exist.yaml" >/dev/null 2>"$GM/e"; r=$?
[ "$r" = 3 ] && grep -q 'fence_missing' "$GM/e" && ok "K2.6: --exclusions naming a fence that does not exist is REFUSED (3 fence_missing), not a FileNotFoundError and a 0-byte summary" || bad "K2.6: missing fence (rc $r): $(cat "$GM/e")"
printf 'schema: coverage-exclusions/1\napplication: catalog-api\nexclusions:\n  - class: generated-code\n' >"$GM/fence-nopath.yaml"
python3 -I "$GOCOV" summary --profile "$GM/p1.cover" --go-mod "$GM/go.mod" --exclusions "$GM/fence-nopath.yaml" >/dev/null 2>"$GM/e"; r=$?
[ "$r" = 3 ] && grep -q 'fence_malformed' "$GM/e" && ok "K2.6: a fence entry without `path` is REFUSED (3 fence_malformed), not a KeyError" || bad "K2.6: no-path entry (rc $r): $(cat "$GM/e")"
mkdir -p "$GM/root/p"; echo x >"$GM/root/p/a.go"; echo y >"$GM/root/p/unlinked.go"
printf 'schema: coverage-exclusions/1\napplication: catalog-api\nexclusions:\n  - path: "p/unlinked.go"\n    class: generated-code\n    justification: "x"\n' >"$GM/fence-unl.yaml"
python3 -I "$GOCOV" summary --profile "$GM/p1.cover" --go-mod "$GM/go.mod" --exclusions "$GM/fence-unl.yaml" --root "$GM/root" >"$GM/s.json" 2>"$GM/e"
check "K2.5: a fence entry that names tracked Go files but no block of the profile is RECORDED (it excludes nothing from this figure)" "$(jq -r '.fence_entries_without_blocks | map(.path) | join(",")' "$GM/s.json")" "p/unlinked.go"
# baseline refusals
jq -n '{status:"ok",nonce:"N1",percent:"50.00",statements:2,covered:1,packages:[],fence:"none"}' >"$GM/res.json"
python3 -I "$GOCOV" baseline --result "$GM/res.json" --out "$GM/b.json" --app catalog-api --runs 1 --commit abc --snapshot "" --nonce OTHER >/dev/null 2>"$GM/e"; r=$?
[ "$r" = 3 ] && grep -q 'result_not_this_run' "$GM/e" && ok "K8.4: a result.json whose nonce is not this run's is never turned into a baseline (3 result_not_this_run)" || bad "K8.4: foreign result accepted (rc $r): $(cat "$GM/e")"
python3 -I "$GOCOV" baseline --result "$GM/res.json" --out "$GM/b.json" --app catalog-api --runs 1 --commit "" --snapshot "" --nonce N1 >/dev/null 2>"$GM/e"; r=$?
[ "$r" = 3 ] && grep -q 'provenance_missing' "$GM/e" && ok "K12.4: a baseline that names neither a commit nor a snapshot is refused (3 provenance_missing)" || bad "K12.4: empty provenance accepted (rc $r)"
python3 -I "$GOCOV" baseline --result "$GM/res.json" --out "$GM/b.json" --app catalog-api --runs 1 --commit abc --snapshot "" --nonce N1 >/dev/null 2>"$GM/e"; check "K12.4 control: a commit alone is enough (exit 0)" "$?" 0
jq '.status="failed"' "$GM/res.json" >"$GM/res2.json"; python3 -I "$GOCOV" baseline --result "$GM/res2.json" --out "$GM/b2.json" --app catalog-api --runs 1 --commit abc --snapshot s --nonce N1 >/dev/null 2>"$GM/e"; r=$?
[ "$r" = 3 ] && grep -q 'result_not_ok' "$GM/e" && ok "K8.4: a failed result is never turned into a baseline (3 result_not_ok)" || bad "K8.4: failed result accepted (rc $r)"
# ================= K6.3 a: the REAL lane: go list inside the module, in IMG-GO =================
if command -v podman >/dev/null 2>&1 && [ -z "${TRACKCOV_NO_REAL_LANE:-}" ]; then
  newworld real; RL="$T/reallane"; mkdir -p "$RL/state"
  ( cd "$REPO" && env -u TC_TIC bash "$COLLECTOR" submit --app catalog-api --src "$REPO" --lanes "unit integration" --state "$RL/state" --group tcg-real ) >"$RL/submit.out" 2>"$RL/submit.err"; r=$?
  if [ "$r" = 0 ]; then
    n_unit="$(awk -F'\t' '$1=="unit"' "$RL/state/members.tsv" | wc -l | tr -d ' ')"; n_int="$(awk -F'\t' '$1=="integration"' "$RL/state/members.tsv" | wc -l | tr -d ' ')"
    [ "$n_unit" -gt 5 ] && ok "K6.3 a: the REAL go list through TIC from the module directory lists packages ($n_unit unit members; at the repository root it fails: 'directory prefix . does not contain main module')" || bad "K6.3 a: unit members $n_unit: $(cat "$RL/submit.err" | head -3)"
    [ "$n_int" -ge 1 ] && ok "K6.3 d: the REAL package list yields integration members from tests/integration ($n_int)" || bad "K6.3 d: no integration member"
    check "K6.3: every real member directory is checkout-relative (no /src prefix, none absolute)" "$(cut -f3 "$RL/state/members.tsv" | grep -c '^/')" 0
    check "K6.3 d: no package is a member of both lanes" "$(cut -f2 "$RL/state/members.tsv" | sort | uniq -d | wc -l | tr -d ' ')" 0
    jq -e '.uncompiled_build_tags | has("e2e_binary") and (has("integration") | not)' "$RL/state/gaps.json" >/dev/null 2>&1 && ok "K6.3 d: the real tree's build tags no lane compiles are listed (e2e_binary yes, integration no)" || bad "K6.3 d: gaps.json: $(jq -c .uncompiled_build_tags "$RL/state/gaps.json" 2>&1 | head -c 300)"
  else
    if grep -qE 'image_not_present|image_not_in_lock|REFUSED|podman' "$RL/submit.err"; then skip "K6.3 a: the real lane did not run here ($(head -c 160 "$RL/submit.err" | tr '\n' ' '))"; else bad "K6.3 a: the real-lane submit failed (rc $r): $(head -c 400 "$RL/submit.err")"; fi
  fi
else skip "K6.3 a: podman is not installed or TRACKCOV_NO_REAL_LANE is set: the real go list leg did not run"; fi

# ================= refusals =================
newworld r1; bash "$COLLECTOR" collect --state "$W/state" --out "$W/out2" --item CAT-123 >/dev/null 2>"$W/e"; RCX=$?
[ "$RCX" != 0 ] && ok "collect without a submitted group is refused (rc $RCX)" || bad "collect without state exited 0"
bash "$COLLECTOR" >/dev/null 2>&1; check "no mode is a usage error" "$?" 2
bash "$COLLECTOR" bogus >/dev/null 2>&1; check "an unknown mode is a usage error" "$?" 2
bash "$COLLECTOR" submit --app >/dev/null 2>"$W/e"; r=$?; [ "$r" = 2 ] && grep -q 'needs a value' "$W/e" && ok "K11.1: an option with no value is a usage error (2), not an unbound variable (1)" || bad "K11.1: submit --app with no value: rc $r: $(cat "$W/e")"
newworld r2; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/does-not-exist" --state "$W/state" >/dev/null 2>&1
[ "$?" != 0 ] && ok "submit with an unreadable package list is refused" || bad "submit accepted a missing package list"
# the test hooks are honoured only in test mode
( unset TC_TEST_MODE; goodworld r3; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" >/dev/null 2>"$T/r3e"; echo $? >"$T/r3rc" )
[ "$(cat "$T/r3rc")" != 0 ] && grep -q 'test_hook_outside_test_mode' "$T/r3e" && ok "TC_DISPATCH and the other hooks are refused outside test mode" || bad "hooks honoured outside test mode"
( unset TC_TEST_MODE TC_DISPATCH TC_TIC TC_RECORDER; goodworld r4; TC_FENCE_DIR="$T/badfence" bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" >/dev/null 2>"$T/r4e"; echo $? >"$T/r4rc" )
[ "$(cat "$T/r4rc")" != 0 ] && grep -q 'test_hook_outside_test_mode' "$T/r4e" && ok "TC_FENCE_DIR alone is refused outside test mode (a caller cannot point the figure at a fence of its choosing)" || bad "TC_FENCE_DIR honoured outside test mode"

# ================= paired mutation =================
if [ "${1:-}" != --no-mutations ] && [ -z "${COLLECTOR_MUTANT:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  copy_sut() { # the mutant is a MIRROR of the repository layout (ROOT = two levels above the collector), so the fence legs run against a real fence and a real go.mod
    local d="$1"; rm -rf "$d"; mkdir -p "$d/scripts/coverage" "$d/coverage" "$d/catalog-api"
    cp "$REPO"/scripts/coverage/*.py "$REPO"/scripts/coverage/*.sh "$d/scripts/coverage/"; cp -r "$REPO/coverage/exclusions" "$d/coverage/"; cp "$REPO/catalog-api/go.mod" "$d/catalog-api/"; }
  mut() { # mut NAME FILE OLD NEW      FILE is track-coverage.sh or gocov_merge.py
    local name="$1" f="$2" old="$3" new="$4" d="$T/mut-$1"; copy_sut "$d"
    python3 -I - "$d/scripts/coverage/$f" "$old" "$new" <<'PY' || { bad "mutation $name: anchor not unique"; return; }
import sys
s=open(sys.argv[1]).read()
if s.count(sys.argv[2])!=1: sys.exit(1)
open(sys.argv[1],"w").write(s.replace(sys.argv[2],sys.argv[3]))
PY
    if COLLECTOR="$d/scripts/coverage/track-coverage.sh" COLLECTOR_MUTANT=1 TRACKCOV_NO_REAL_LANE=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-$name.out" 2>&1; then bad "mutation $name SURVIVED"; echo "SURVIVED $name" >>"$REC"
    else ok "mutation $name caught ($(grep -c '^FAIL:' "$T/mut-$name.out") failing legs)"; echo "CAUGHT $name: $(grep '^FAIL:' "$T/mut-$name.out" | head -2 | cut -c1-110 | tr '\n' '|')" >>"$REC"; fi
  }
  # SANDBOX CONTROL (review round 1): an UNMUTATED mirror must pass this body, or every CAUGHT below could be a broken sandbox
  d="$T/mut-ctl"; copy_sut "$d"
  if COLLECTOR="$d/scripts/coverage/track-coverage.sh" COLLECTOR_MUTANT=1 TRACKCOV_NO_REAL_LANE=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-ctl.out" 2>&1; then ok "mutation sandbox control: an UNMUTATED mirror passes this body"; echo "CONTROL PASS" >>"$REC"
  else bad "mutation sandbox control FAILED ($(grep -c '^FAIL:' "$T/mut-ctl.out") failing legs): every CAUGHT below is suspect"; echo "CONTROL FAIL" >>"$REC"; fi
  mut run-half-status-dropped track-coverage.sh 'RUN_RC=$?; } >"$RUNLOG" 2>&1' 'RUN_RC=0; } >"$RUNLOG" 2>&1'
  mut verify-skipped track-coverage.sh 'if [ -z "$WANT" ] || [ "$WANT" != "$GOT" ] || [ "$COPYSHA" != "$LISTED" ]; then   # MUT:verify' 'if false; then   # MUT:verify'
  mut foreground-wait track-coverage.sh 'STATUS_JSON="$(bash "$DISPATCH" status "$BID" 2>/dev/null </dev/null)"' 'bash "$DISPATCH" wait "$BID" 5 >/dev/null 2>&1; STATUS_JSON="$(bash "$DISPATCH" status "$BID" 2>/dev/null </dev/null)"'
  mut panic-unchecked track-coverage.sh 'if grep -q "^panic:" "$RUNLOG"; then' 'if false; then'
  mut empty-profile-ok track-coverage.sh 'if [ "$PROFILE_BLOCKS" -eq 0 ]; then err' 'if false; then err'
  mut infra-detail-dropped track-coverage.sh 'infra_failed ($DETAIL)' 'infra_failed'
  mut dir-abs-unhandled track-coverage.sh 'case "$DIR" in /src/*) DIR="${DIR#/src/}";; /*) refuse package_dir_outside_source "$IMPORT: $DIR is absolute and not under /src";; esac   # MUT:dir_abs' ':'
  mut fence-not-applied track-coverage.sh '${FENCE_ARGS[@]+"${FENCE_ARGS[@]}"}' ''
  mut fence-gate-skipped track-coverage.sh 'FOUT="$(bash "$HERE/check_exclusions.sh" "$OUT/fence/$APP.yaml"' 'FOUT="$(true "$OUT/fence/$APP.yaml"'
  mut CM20_merge_failure_ignored track-coverage.sh '|| err "merge: profiles could not be merged (' '|| : "merge: profiles could not be merged ('
  mut CM23_kind_not_checked track-coverage.sh 'if [ "$KIND" != completed ]; then err' 'if false; then err'
  mut recorder-argv-restored track-coverage.sh 'BASELINE 1 go_binary "$OUT/result.json" --evidence-class runtime --oracle invariant --oracle-independent' 'GREEN 1 runtime coverage-baseline'
  mut drop-test-source track-coverage.sh ' --test-source "$HERE/gocov_merge.py" --' ' --'
  mut artifact-fields-read-again track-coverage.sh 'AROOT="$BUILDS_ROOT/$BID/artifacts"; ART="$AROOT/$LANE/$NAME.test"' 'AROOT="$(jq -r .artifact_dir <<<"$STATUS_JSON")"; ART="$AROOT/$LANE/$NAME.test"'
  mut cd-app-dropped track-coverage.sh 'CMDSTR="cd /src/$APP && go test' 'CMDSTR="go test'
  mut lane-membership-dropped track-coverage.sh 'case "$LANE:$MEMBER_LANE" in unit:unit|integration:integration|integration:integration-tagged) ;; *) continue;; esac   # MUT:lane_membership' ':'
  mut stdin-guard-dropped track-coverage.sh '"${ARGV[@]}" 2>"$STATE/submit-$LANE-$NAME.err" </dev/null)"' '"${ARGV[@]}" 2>"$STATE/submit-$LANE-$NAME.err")"'
  mut read-fd-dropped track-coverage.sh 'while IFS=$'"'"'\t'"'"' read -r -u 9 IMPORT DIR || [ -n "${IMPORT:-}" ]; do' 'while IFS=$'"'"'\t'"'"' read -r IMPORT DIR || [ -n "${IMPORT:-}" ]; do'
  mut seal-gate-dropped track-coverage.sh 'if [ "$FAILS" = 0 ] && [ "$SUBMITTED" = "$EXPECTED" ] && bash "$DISPATCH" group-seal "$GROUP" >/dev/null 2>&1 </dev/null; then SEALED=true; fi   # MUT:seal_gate' 'SEALED=true'
  mut sealed-check-dropped track-coverage.sh '[ "$(jq -r .sealed "$STATE/state.json")" = true ] || refuse group_not_sealed' ':; true || refuse group_not_sealed'
  mut state-complete-dropped track-coverage.sh '[ "$EXPECTED_N" = "$MEMBERS_N" ] || refuse state_incomplete' ':; true || refuse state_incomplete'
  mut out-not-empty-dropped track-coverage.sh '[ ! -e "$OUT" ] || [ -z "$(ls -A "$OUT" 2>/dev/null)" ] || refuse out_not_empty' ':; true || refuse out_not_empty'
  mut snapshot-skew-dropped track-coverage.sh '[ "$SNAPNOW" = "$SNAP" ] || refuse snapshot_skew' ':; true || refuse snapshot_skew'
  mut state-not-empty-dropped track-coverage.sh '[ ! -s "$STATE/members.tsv" ] && [ ! -e "$STATE/state.json" ] || refuse state_not_empty' ':; true || refuse state_not_empty'
  mut not-terminal-dropped track-coverage.sh 'if [ "$(jq -r '"'"'.terminal == null'"'"' <<<"$STATUS_JSON")" = true ]; then' 'if false; then'
  mut copy-status-dropped track-coverage.sh '|| { err "$PKG ($LANE): copy_failed ($(head -c 160 "$OUT/run/$LANE-$NAME.cp.err" | tr '"'"'\n'"'"' '"'"' '"'"'))"; continue; }   # MUT:copy_status' '|| :   # MUT:copy_status'
  mut reconcile-dropped track-coverage.sh '[ $((PROFILE_COUNT + ERROR_COUNT)) = "$MEMBERS_N" ] || err' ':; true || err'
  mut summary-status-dropped track-coverage.sh '|| err "summary: the figure could not be computed (' '|| : "summary: the figure could not be computed ('
  mut status-before-result track-coverage.sh 'STATUS=failed
if jq -e --arg n "$NONCE" '"'"'.nonce == $n and .status == "ok"'"'"' "$OUT/result.json" >/dev/null 2>&1 && [ "$ERROR_COUNT" = 0 ]; then STATUS=ok; fi   # MUT:status_after_result' 'STATUS=ok'
  mut interrupted-state-dropped track-coverage.sh "trap 'on_sig INT 130' INT; trap 'on_sig TERM 143' TERM" ':'
  mut abspath-dropped track-coverage.sh '*) printf '"'"'%s/%s'"'"' "$CALLER_PWD" "$1";; esac; }   # MUT:abspath' '*) printf '"'"'%s'"'"' "$1";; esac; }   # MUT:abspath'
  mut item-not-validated track-coverage.sh '[[ "$ITEM" =~ ^(CAT-[0-9]{3,}|FND-[0-9]{4,}|RUN-[0-9]+|AUD-[0-9A-Za-z-]+)$ ]] || usage' ':; true || usage'
  mut gaps-dropped track-coverage.sh 's["uncompiled_build_tags"] = gaps.get("uncompiled_build_tags")' 's["uncompiled_build_tags"] = None'
  # gocov_merge.py (K14.5: the collector's python half has paired mutations too)
  mut m-G_merge_argv_first_profile_dropped gocov_merge.py 'out = a[o + 1]; paths = a[1:o] + a[o + 2:]   # MUT:merge_argv' 'out = a[o + 1]; paths = a[2:o] + a[o + 2:]   # MUT:merge_argv'
  mut CM21_mode_mix_merged gocov_merge.py 'elif m != mode: raise ValueError("%s: mode %s differs from %s" % (p, m, mode))' 'elif False: pass'
  mut CM22_statement_mismatch_merged gocov_merge.py 'if out[k][0] != s: raise ValueError("block %s has two statement counts" % k)' 'if False: pass'
  mut module-unknown-applied-silently gocov_merge.py 'if not module:   # MUT:module_unknown' 'if False:   # MUT:module_unknown'
  mut fence-missing-crash gocov_merge.py 'refuse("fence_missing",' 'sys.stderr.write("x\n"); refuse("fence_unreadable_but_continue",'
  mut fence-malformed-accepted gocov_merge.py 'if not isinstance(e_, dict) or not isinstance(e_.get("path"), str):' 'if False:'
  mut nonce-not-checked gocov_merge.py 'if res.get("nonce") != opt(a, "--nonce"):   # MUT:nonce' 'if False:   # MUT:nonce'
  mut provenance-not-checked gocov_merge.py 'if not commit and not snap:   # MUT:provenance' 'if False:   # MUT:provenance'
  mut result-status-not-checked gocov_merge.py 'if res.get("status") != "ok":' 'if False:'
  mut go-mod-comment-not-parsed gocov_merge.py '|([^\s/][^\s]*))\s*(?://.*)?$' '|([^\s/][^\s]*))\s*$'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=$SKIPS"
[ "$FAILS" = 0 ]
