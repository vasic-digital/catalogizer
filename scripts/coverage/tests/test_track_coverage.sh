#!/usr/bin/env bash
# test_track_coverage.sh - T198 (RED first, against the old scripts/track-coverage.sh which swallows a failing package: doc05 7.4 A1). Oracle for the rewritten
# collector scripts/coverage/track-coverage.sh. NOTHING is compiled: scripts/build/dispatch.sh, scripts/build/verify_artifact.sh, scripts/test-in-container.sh and the
# recorder are SHIMS that replay recorded callback records (T089a): `status` JSON per build, brought-back "binaries" (a bash script that writes a Go cover profile of the
# fixture's choosing, or fails, or panics) and a recorded verify decision. The split-lane contract asserted here:
#  (1) submit hands ONE build group per run to the dispatcher, one member per (lane, package), the compile half `go test -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test`;
#  (2) collect takes each package's result only from the callback record of its build (`status`), and NEVER waits (the shim logs every dispatcher call: no `wait`);
#  (3) a brought-back binary runs only after verify_artifact.sh accepted it for its build; the profile is written by `-test.coverprofile` from the package directory;
#  (4) the per-package profiles are merged (counts summed per block) and the result goes through the recorder;
#  (5) exit non-zero NAMING the package on: build_failed, infra_failed (its reason detail kept), a missing brought-back binary, a binary refused by verify_artifact.sh,
#      a failing package, a panicking package, an empty profile; golden case: two packages merging to the hand-computed percent.
# Usage: test_track_coverage.sh [--no-mutations]    Env: COLLECTOR (script under test), MUTATION_RECORD
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REPO="$(cd "$HERE/../../.." && pwd)"
COLLECTOR="${COLLECTOR:-$REPO/scripts/coverage/track-coverage.sh}"
PASSES=0; FAILS=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required"; exit 2; }; done
T="$(mktemp -d "${TMPDIR:-/tmp}/trackcov-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -f "$COLLECTOR" ] || { bad "the collector $COLLECTOR does not exist"; echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"; exit 1; }
# the old script has no submit/collect modes: the contract below fails against it (the RED of T198)

SHIM="$T/shim"; mkdir -p "$SHIM"
cat >"$SHIM/dispatch.sh" <<'S'
#!/usr/bin/env bash
# replays recorded callback records; logs every call
echo "dispatch $*" >>"${SHIM_LOG:?}"
case "${1:-}" in
  snapshot) printf '%064d\n' 1;;
  argv-digest) shift; printf '%s' "$*" | sha256sum | cut -d' ' -f1;;
  submit) n=$(grep -c '^submit-id' "$SHIM_STATE/ids" 2>/dev/null || echo 0); id="b-$(printf '%024d' $((n+1)))"; echo "submit-id $id" >>"$SHIM_STATE/ids"
          # remember which package this id is for: the argv after -- carries the -o /out/<lane>/<name>.test
          printf '%s\n' "$*" | grep -o '/out/[A-Za-z0-9_./-]*\.test' | head -1 >"$SHIM_STATE/pkg-$id"; echo "$id";;
  group-seal) echo sealed;;
  status) id="$2"; f="$SHIM_REC/$(cat "$SHIM_STATE/pkg-$id" 2>/dev/null | sed 's#^/out/##; s#\.test$##; s#/#__#g').json"; [ -f "$f" ] && cat "$f" || { echo "no record" >&2; exit 20; };;
  wait) echo "FOREGROUND WAIT" >>"$SHIM_LOG"; exit 3;;
  *) exit 0;;
esac
S
cat >"$SHIM/verify_artifact.sh" <<'S'
#!/usr/bin/env bash
echo "verify $*" >>"${SHIM_LOG:?}"
f=""; while [ $# -gt 0 ]; do case "$1" in --file) f="$2"; shift 2;; *) shift;; esac; done
case "$(basename "$f")" in *refused*) echo "verify_artifact: REFUSED reason=digest_mismatch" >&2; exit 20;; esac; exit 0
S
cat >"$SHIM/tic.sh" <<'S'
#!/usr/bin/env bash
# the run-half shim: executes the command after -- in the shim "container" (the host, in a scratch dir tree), mapping /src and /out
echo "tic $*" >>"${SHIM_LOG:?}"
OUT=""; while [ $# -gt 0 ]; do case "$1" in --out) OUT="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
cmd="$*"; cmd="${cmd//\/out\//$OUT/}"; cmd="${cmd//\/src\//$SHIM_SRC/}"
bash -c "$cmd"
S
cat >"$SHIM/recorder.sh" <<'S'
#!/usr/bin/env bash
echo "recorder $*" >>"${SHIM_LOG:?}"
while [ $# -gt 0 ] && [ "$1" != "--" ]; do shift; done; shift; exec "$@"
S
chmod +x "$SHIM"/*.sh
export TC_TEST_MODE=1 TC_DISPATCH="$SHIM/dispatch.sh" TC_VERIFY="$SHIM/verify_artifact.sh" TC_TIC="$SHIM/tic.sh" TC_RECORDER="$SHIM/recorder.sh"

# ---- the recorded world: module `catalogizer`, packages alpha and beta (unit lane), statement blocks hand-counted ----
#   alpha profile: 4 blocks, 10 statements; beta profile: the same block of alpha.go plus 2 own blocks.
SRC="$T/src"; mkdir -p "$SRC/catalog-api/alpha" "$SRC/catalog-api/beta"
printf 'catalogizer/alpha\tcatalog-api/alpha\ncatalogizer/beta\tcatalog-api/beta\n' >"$T/packages.tsv"
mkbin() { # mkbin FILE profile-lines...: a "binary" that writes the profile to the -test.coverprofile path
  local f="$1"; shift; { echo '#!/usr/bin/env bash'; echo 'for a in "$@"; do case "$a" in -test.coverprofile=*) P="${a#-test.coverprofile=}";; esac; done'
    echo 'printf "mode: atomic\n" >"$P"'; for l in "$@"; do echo "printf '%s\\n' '$l' >>\"\$P\""; done; echo 'echo "=== RUN   TestX"; echo "--- PASS: TestX (0.00s)"; echo PASS'; } >"$f"; chmod +x "$f"
}
newworld() { # newworld NAME: fresh state, records and artifact dir
  W="$T/$1"; rm -rf "$W"; mkdir -p "$W/state" "$W/rec" "$W/art" "$W/out"; export SHIM_STATE="$W/state" SHIM_REC="$W/rec" SHIM_LOG="$W/log" SHIM_SRC="$SRC"; : >"$SHIM_LOG"
}
rec() { # rec unit__alpha succeeded [exit_class detail]: a recorded status JSON
  local name="$1" cls="$2" kind="${3:-completed}" reason="${4:-}"
  jq -n --arg c "$cls" --arg k "$kind" --arg r "$reason" --arg a "$W/art" --arg n "$name" \
     '{state:"terminal", terminal:{kind:$k, exit_class:$c, reason:$r}, callback_state:"done", artifact_dir:$a, manifest:($a+"/"+$n+".manifest.json"), artifact_name:($n+".test")}' >"$W/rec/$name.json"
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
collect() { bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" --item T201 >"$W/collect.out" 2>"$W/collect.err"; COL_RC=$?; }

# ================= golden: two packages merge =================
goodworld g1; submit
check "submit: exit 0" "$SRC_RC" 0
check "submit: ONE build group per run, two members (one per package)" "$(grep -c '^dispatch submit ' "$SHIM_LOG")" 2
check "submit: every member names the same group" "$(grep '^dispatch submit ' "$SHIM_LOG" | grep -c -- '--group tcg-test')" 2
grep '^dispatch submit ' "$SHIM_LOG" | head -1 | grep -q -- 'go test -c -cover -covermode=atomic -coverpkg=./... -o /out/unit/alpha.test' \
  && ok "submit: the compile half is go test -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test" || bad "submit: compile half argv wrong: $(grep '^dispatch submit ' "$SHIM_LOG" | head -1)"
grep -q 'dispatch group-seal tcg-test' "$SHIM_LOG" && ok "submit: the group is sealed after the last member" || bad "submit: the group was not sealed"
grep -q 'FOREGROUND WAIT' "$SHIM_LOG" && bad "submit: it waited in the foreground" || ok "submit: it does not wait (it returns after sealing)"
grep -q '^dispatch status' "$SHIM_LOG" && bad "submit: it read a result" || ok "submit: it reads no result"
# the shim's package mapping uses the -o path: lane/name, with the lane subdirectory
# collect
collect
check "collect: exit 0 on the golden case" "$COL_RC" 0
grep -q 'FOREGROUND WAIT' "$SHIM_LOG" && bad "collect: it waited in the foreground" || ok "collect: it never waits (results come from the callback records)"
check "collect: one status read per member" "$(grep -c '^dispatch status' "$SHIM_LOG")" 2
check "collect: every binary is verified before it runs (2 verify calls)" "$(grep -c '^verify ' "$SHIM_LOG")" 2
# order: the verify call of a package precedes its run
awk '/^verify /{v++} /^tic /{ if (v<1) bad=1 } END{exit bad}' "$SHIM_LOG" && ok "collect: no binary ran before its verification" || bad "collect: a run preceded the first verification"
# hand-computed merge: alpha block1 (3 stmts) 1+1=2 covered; block2 (2) 0+1=1 covered; block3 (3) 4 covered; block4 (2) 0 not covered; beta b1 (4) covered; b2 (1) not
#   statements: 3+2+3+2+4+1 = 15; covered: 3+2+3+4 = 12  -> 80.00 %
M="$W/out/merged.cover"; [ -f "$M" ] && ok "collect: the merged profile exists" || bad "collect: no merged profile"
check "collect: merged profile header" "$(head -1 "$M" 2>/dev/null)" "mode: atomic"
check "collect: merged blocks (7: six distinct plus the header line)" "$(grep -c ':' "$M" 2>/dev/null)" 7
check "collect: the block both binaries instrument is counted once with the SUMMED count" "$(grep 'a.go:1.1,2.1' "$M" | awk '{print $3}')" 2
check "collect: statements total (hand count 15)" "$(jq -r .statements "$W/out/result.json")" 15
check "collect: statements covered (hand count 12)" "$(jq -r .covered "$W/out/result.json")" 12
check "collect: percent" "$(jq -r .percent "$W/out/result.json")" "80.00"
check "collect: result status" "$(jq -r .status "$W/out/result.json")" ok
check "collect: per-package results are kept" "$(jq -r '.packages | map(.package) | sort | join(" ")' "$W/out/result.json")" "alpha beta"
grep -q '^recorder ' "$SHIM_LOG" && ok "collect: the result is written through the recorder" || bad "collect: the recorder was not used"
check "collect: the baseline record has the schema coverage-baseline/1" "$(jq -r .schema "$W/out/baseline.json" 2>/dev/null)" "coverage-baseline/1"
check "collect: the baseline names the instrument" "$(jq -r .instrument "$W/out/baseline.json" 2>/dev/null)" "go-cover-atomic-split"
# GREEN x3 from three replays over the same records
P3=""; for i in 1 2 3; do goodworld "g3-$i"; submit; collect; P3="$P3 $(jq -r .percent "$W/out/result.json")"; done
check "GREEN x3: three replays give the identical percent" "$P3" " 80.00 80.00 80.00"

# ================= review round 1 (WF11 I10, I6) =================
# I10: `go list {{.Dir}}` is ABSOLUTE; an absolute directory under /src is normalised to the checkout-relative path, one outside /src is refused
printf 'catalogizer/alpha\t/src/catalog-api/alpha\ncatalogizer/beta\t/src/catalog-api/beta\n' >"$T/packages-abs.tsv"
goodworld ia; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages-abs.tsv" --state "$W/state" --group tcg-abs >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
check "I10: submit with absolute go-list directories exits 0" "$SRC_RC" 0
grep '^dispatch submit ' "$SHIM_LOG" | head -1 | grep -q -- '-o /out/unit/alpha.test ./alpha$' && ok "I10: the compile target is ./alpha, never ./src/catalog-api/alpha" || bad "I10: compile target wrong: $(grep '^dispatch submit ' "$SHIM_LOG" | head -1)"
check "I10: members.tsv records the checkout-relative directory (the run half's cd /src/<dir>)" "$(cut -f3 "$W/state/members.tsv" | head -1)" "catalog-api/alpha"
printf 'catalogizer/alpha\t/tmp/elsewhere/alpha\n' >"$T/packages-out.tsv"
goodworld ib; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages-out.tsv" --state "$W/state" --group tcg-out >"$W/submit.out" 2>"$W/submit.err"; SRC_RC=$?
[ "$SRC_RC" != 0 ] && grep -q 'package_dir_outside_source' "$W/submit.err" && ok "I10: an absolute directory outside /src is refused (package_dir_outside_source)" || bad "I10: outside-/src directory accepted ($SRC_RC): $(cat "$W/submit.err")"
check "I10: that refusal submitted no build" "$(grep -c '^dispatch submit ' "$SHIM_LOG")" 0
# I6: the fence is APPLIED to the figure: blocks of a file the fence names (catalog-api tests/mocks/**) are dropped and counted
goodworld ex
mkbin "$W/art/unit__alpha.test" "$A_BLK" "$A_BLK2" "$A_BLK3" "$A_BLK4" 'catalogizer/tests/mocks/ftp/s.go:1.1,2.1 7 0'
submit; collect
check "I6: exit 0 with a fenced file in the profile" "$COL_RC" 0
check "I6: statements exclude the fenced file's 7 (hand count 15, was 22 before the fence was applied)" "$(jq -r .statements "$W/out/result.json")" 15
check "I6: the percent is the unfenced 80.00" "$(jq -r .percent "$W/out/result.json")" "80.00"
check "I6: the record names the excluded file" "$(jq -r '.excluded.files | join(",")' "$W/out/result.json")" "tests/mocks/ftp/s.go"
check "I6: the record counts the excluded statements" "$(jq -r '.excluded.statements' "$W/out/result.json")" 7
check "I6: the baseline record carries the exclusion" "$(jq -r '.excluded.statements' "$W/out/baseline.json" 2>/dev/null)" 7

# I6: a fence the T200 gate refuses blocks the figure (the gate runs before the fence is applied)
mkdir -p "$T/badfence"; cat >"$T/badfence/catalog-api.yaml" <<'Y'
schema: coverage-exclusions/1
application: catalog-api
exclusions:
  - path: "poison/first_party_without_item.go"
    class: first-party
    justification: "a first-party entry written without a tracked item, the case the gate must refuse"
Y
goodworld bf; submit; TC_FENCE_DIR="$T/badfence" collect
[ "$COL_RC" != 0 ] && grep -q 'exclusion fence refused by the T200 gate' "$W/collect.err" && ok "I6: a fence the gate refuses fails the collection (exit $COL_RC) and says why" || bad "I6: refused fence was accepted (rc $COL_RC): $(cat "$W/collect.err")"
[ "$(jq -r .status "$W/out/result.json" 2>/dev/null)" = failed ] && ok "I6: and result.json says failed (no baseline over an unchecked scope)" || bad "I6: result status is not failed after a refused fence"

# ================= each failure names the package and exits non-zero =================
failcase() { # failcase NAME PATTERN setup-fn
  local name="$1" pat="$2" fn="$3"; goodworld "f-$name"; submit; $fn; collect
  [ "$COL_RC" != 0 ] && ok "$name: exit non-zero ($COL_RC)" || bad "$name: exit 0 (a failing package was swallowed)"
  grep -q "$pat" "$W/collect.err" && ok "$name: stderr names the package and the reason ($pat)" || bad "$name: stderr lacks '$pat': $(cat "$W/collect.err")"
  [ "$(jq -r .status "$W/out/result.json" 2>/dev/null)" = failed ] && ok "$name: the result record says failed" || bad "$name: result status is not failed"
}
f_build() { rec unit__beta build_failed completed "go: undefined: Foo"; }
failcase build_failed 'beta (unit): build_failed' f_build
f_infra() { rec unit__beta infra_failed completed "snapshot_mismatch"; }
failcase infra_failed 'beta (unit): infra_failed (snapshot_mismatch)' f_infra
f_missing() { rm -f "$W/art/unit__beta.test"; }
failcase binary_missing 'beta (unit): binary_missing' f_missing
f_refused() { mv "$W/art/unit__beta.test" "$W/art/unit__beta_refused.test"; jq '.artifact_name="unit__beta_refused.test"' "$W/rec/unit__beta.json" >"$W/rec/x" && mv "$W/rec/x" "$W/rec/unit__beta.json"; }
failcase verify_refused 'beta (unit): verify_refused' f_refused
f_failing() { { echo '#!/usr/bin/env bash'; echo 'echo "--- FAIL: TestBeta (0.00s)"; echo FAIL'; echo 'exit 1'; } >"$W/art/unit__beta.test"; }
failcase package_failed 'beta (unit): package_failed' f_failing
f_panic() { { echo '#!/usr/bin/env bash'; echo 'echo "panic: runtime error: index out of range"; echo "goroutine 1 [running]:"'; echo 'exit 2'; } >"$W/art/unit__beta.test"; }
failcase package_panicked 'beta (unit): package_panicked' f_panic
f_panic0() { { echo '#!/usr/bin/env bash'; echo 'for a in "$@"; do case "$a" in -test.coverprofile=*) printf "mode: atomic\n" >"${a#-test.coverprofile=}";; esac; done; echo "panic: boom"; exit 0'; } >"$W/art/unit__beta.test"; }
failcase panic_with_exit_zero 'beta (unit): package_panicked' f_panic0
f_empty() { { echo '#!/usr/bin/env bash'; echo 'for a in "$@"; do case "$a" in -test.coverprofile=*) printf "mode: atomic\n" >"${a#-test.coverprofile=}";; esac; done; echo PASS'; } >"$W/art/unit__beta.test"; }
failcase profile_empty 'beta (unit): profile_empty' f_empty
f_nocallback() { rm -f "$W/rec/unit__beta.json"; }
failcase callback_record_missing 'beta (unit): callback_record_missing' f_nocallback
f_cancel() { rec unit__beta cancelled completed ""; }
failcase cancelled 'beta (unit): cancelled' f_cancel
# one failing package does not hide the other: alpha's result is still computed and the failing package is the only one named
goodworld one; submit; f_failing; collect
grep -q 'alpha' "$W/collect.err" && bad "one failing package: the healthy package alpha is named as a failure" || ok "one failing package: only the failing package is named"
check "one failing package: the healthy package alpha still ran" "$(grep -c '^tic ' "$SHIM_LOG")" 2

# ================= refusals =================
newworld r1; bash "$COLLECTOR" collect --state "$W/state" --out "$W/out" >/dev/null 2>"$W/e"; RCX=$?
[ "$RCX" != 0 ] && ok "collect without a submitted group is refused (rc $RCX)" || bad "collect without state exited 0"
bash "$COLLECTOR" >/dev/null 2>&1; check "no mode is a usage error" "$?" 2
bash "$COLLECTOR" bogus >/dev/null 2>&1; check "an unknown mode is a usage error" "$?" 2
newworld r2; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/does-not-exist" --state "$W/state" >/dev/null 2>&1
[ "$?" != 0 ] && ok "submit with an unreadable package list is refused" || bad "submit accepted a missing package list"
# the test hooks are honoured only in test mode
( unset TC_TEST_MODE; goodworld r3; bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" >/dev/null 2>"$T/r3e"; echo $? >"$T/r3rc" )
[ "$(cat "$T/r3rc")" != 0 ] && grep -q 'test_hook_outside_test_mode' "$T/r3e" && ok "TC_DISPATCH and the other hooks are refused outside test mode" || bad "hooks honoured outside test mode"
( unset TC_TEST_MODE TC_DISPATCH TC_VERIFY TC_TIC TC_RECORDER; goodworld r4; TC_FENCE_DIR="$T/badfence" bash "$COLLECTOR" submit --app catalog-api --src "$SRC" --lanes unit --packages "$T/packages.tsv" --state "$W/state" >/dev/null 2>"$T/r4e"; echo $? >"$T/r4rc" )
[ "$(cat "$T/r4rc")" != 0 ] && grep -q 'test_hook_outside_test_mode' "$T/r4e" && ok "TC_FENCE_DIR alone is refused outside test mode (a caller cannot point the figure at a fence of its choosing)" || bad "TC_FENCE_DIR honoured outside test mode"

# ================= paired mutation =================
if [ "${1:-}" != --no-mutations ] && [ -z "${COLLECTOR_MUTANT:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  mut() { # mut NAME OLD NEW
    # the mutant is a MIRROR of the repository layout (ROOT = two levels above the collector), so the fence legs run against a real fence and a real go.mod
    local name="$1" old="$2" new="$3" d="$T/mut-$1"; rm -rf "$d"; mkdir -p "$d/scripts/coverage" "$d/coverage" "$d/catalog-api"
    cp "$REPO/scripts/coverage/track-coverage.sh" "$REPO/scripts/coverage/gocov_merge.py" "$REPO/scripts/coverage/fence_lib.py" "$REPO/scripts/coverage/check_exclusions.sh" "$REPO/scripts/coverage/check_exclusions.py" "$d/scripts/coverage/"
    cp -r "$REPO/coverage/exclusions" "$d/coverage/"; cp "$REPO/catalog-api/go.mod" "$d/catalog-api/"
    python3 -I - "$d/scripts/coverage/track-coverage.sh" "$old" "$new" <<'PY' || { bad "mutation $name: anchor not unique"; return; }
import sys
s=open(sys.argv[1]).read()
if s.count(sys.argv[2])!=1: sys.exit(1)
open(sys.argv[1],"w").write(s.replace(sys.argv[2],sys.argv[3]))
PY
    if COLLECTOR="$d/scripts/coverage/track-coverage.sh" COLLECTOR_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-$name.out" 2>&1; then bad "mutation $name SURVIVED"; echo "SURVIVED $name" >>"$REC"
    else ok "mutation $name caught ($(grep -c '^FAIL:' "$T/mut-$name.out") failing legs)"; echo "CAUGHT $name: $(grep '^FAIL:' "$T/mut-$name.out" | head -2 | cut -c1-110 | tr '\n' '|')" >>"$REC"; fi
  }
  # SANDBOX CONTROL (review round 1): an UNMUTATED mirror must pass this body, or every CAUGHT below could be a broken sandbox
  d="$T/mut-ctl"; rm -rf "$d"; mkdir -p "$d/scripts/coverage" "$d/coverage" "$d/catalog-api"
  cp "$REPO/scripts/coverage/track-coverage.sh" "$REPO/scripts/coverage/gocov_merge.py" "$REPO/scripts/coverage/fence_lib.py" "$REPO/scripts/coverage/check_exclusions.sh" "$REPO/scripts/coverage/check_exclusions.py" "$d/scripts/coverage/"
  cp -r "$REPO/coverage/exclusions" "$d/coverage/"; cp "$REPO/catalog-api/go.mod" "$d/catalog-api/"
  if COLLECTOR="$d/scripts/coverage/track-coverage.sh" COLLECTOR_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-ctl.out" 2>&1; then ok "mutation sandbox control: an UNMUTATED mirror passes this body"; echo "CONTROL PASS" >>"$REC"
  else bad "mutation sandbox control FAILED ($(grep -c '^FAIL:' "$T/mut-ctl.out") failing legs): every CAUGHT below is suspect"; echo "CONTROL FAIL" >>"$REC"; fi
  mut run-half-or-true '"$BIN_RUN" "${RUNARGS[@]}"; RUN_RC=$?' '"$BIN_RUN" "${RUNARGS[@]}" || true; RUN_RC=0'
  mut verify-skipped 'bash "$VERIFY" --build-id "$BID" --file "$ART"' 'true'
  mut foreground-wait 'STATUS_JSON="$(bash "$DISPATCH" status "$BID" 2>/dev/null)"' 'bash "$DISPATCH" wait "$BID" 5 >/dev/null 2>&1; STATUS_JSON="$(bash "$DISPATCH" status "$BID" 2>/dev/null)"'
  mut panic-unchecked 'if grep -q "^panic:" "$RUNLOG"; then' 'if false; then'
  mut empty-profile-ok 'if [ "$PROFILE_BLOCKS" -eq 0 ]; then' 'if false; then'
  mut infra-detail-dropped 'infra_failed ($DETAIL)' 'infra_failed'
  # review round 1: the absolute go-list directory and the applied fence
  mut dir-abs-unhandled 'case "$DIR" in /src/*) DIR="${DIR#/src/}";; /*) refuse package_dir_outside_source "$IMPORT: $DIR is absolute and not under /src";; esac   # MUT:dir_abs' ':'
  mut fence-not-applied '${FENCE:+--exclusions "$FENCE"}' ''
  mut fence-gate-skipped 'FOUT="$(bash "$HERE/check_exclusions.sh" "$FENCE" --root "$ROOT/$APP" 2>&1)" ||' 'FOUT="$(true 2>&1)" ||'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"
[ "$FAILS" = 0 ]
