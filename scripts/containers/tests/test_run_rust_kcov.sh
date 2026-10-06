#!/usr/bin/env bash
# test_run_rust_kcov.sh - T200a (RED first). Oracle for the wrappers scripts/containers/run_rust.sh (IMG-RUST, class `compile`: a local start is refused with
# 20 compile_class_local) and run_kcov.sh (IMG-KCOV, class `interpreter`, local), and for the coverage lane rows of scripts/containers/lanes.tsv.
# Same two independent oracles as test_runners.sh: a run_pinned.sh SHIM that logs argv, RUNP_* limits and the registry state at call time, and the REAL
# long-op registry read back with jq. Lane needles go through the REAL scripts/test-in-container.sh with a shim wrapper directory (TIC_TEST_MODE=1).
# Paired mutations: copies of the containers directory (RUNNER_SUT_DIR / TIC copy) with ONE expression changed must each make this body FAIL.
# Usage: test_run_rust_kcov.sh [--no-mutations]     Env: RUNNER_SUT_DIR (containers dir under test), MUTATION_RECORD
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUTDIR="${RUNNER_SUT_DIR:-$HERE/..}"
TICDIR="${TIC_SUT_DIR:-$REPO/scripts}"
PASSES=0; FAILS=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
for w in run_rust run_kcov; do [ -f "$SUTDIR/$w.sh" ] || bad "$w.sh not found at $SUTDIR (T200a: the wrapper does not exist yet)"; done
[ "$FAILS" = 0 ] || { echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"; exit 1; }
T="$(mktemp -d "${TMPDIR:-/tmp}/rustkcov-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
D1="sha256:$(printf 'a%.0s' $(seq 64))"
SHIMS="$T/shims"; mkdir -p "$SHIMS"
cat >"$SHIMS/run_pinned.sh" <<'SH'
#!/usr/bin/env bash
prev=""; for a in "$@"; do [ "$prev" = --out ] && mkdir -p -- "$a"; prev=$a; done
{
  echo "---CALL---"
  printf 'ARG:%s\n' "$@"
  printf 'ENV:RUNP_MEMORY=%s\nENV:RUNP_CPUS=%s\n' "${RUNP_MEMORY-}" "${RUNP_CPUS-}"
  printf 'REG:%s\n' "$(for f in "${LONGOPS_DIR:-/nonexistent}"/ops/*.json; do [ -e "$f" ] && jq -r '.op_id + "=" + .state' "$f"; done 2>/dev/null | tr '\n' ' ')"
} >>"${SHIM_LOG:?}"
case " $* " in *cpa-probe*|*" --version "*) [ -z "${SHIM_PROBE_OUT:-}" ] || cat "$SHIM_PROBE_OUT"; exit 0;; esac
echo "main-run-stdout"; exit "${SHIM_RC:-0}"
SH
cat >"$SHIMS/sweep.sh" <<'SH'
#!/usr/bin/env bash
printf 'SWEEP:%s\n' "$*" >>"${SWEEP_LOG:?}"; echo "AM-P1 clean"; exit 0
SH
cat >"$SHIMS/podman" <<'SH'
#!/usr/bin/env bash
{ echo "---PODMAN---"; printf '%s\n' "$@"; } >>"${PODMAN_LOG:?}"
case "$*" in "image inspect"*) echo "${SHIM_INSPECT_DIGEST:-}"; exit 0;; esac
exit 0
SH
chmod +x "$SHIMS"/*
LOCKF="$T/lock.yaml"
cat >"$LOCKF" <<LK
schema: 1
images:
- id: IMG-KCOV
  reference: localhost/example-kcov
  tag_intent: abc
  digest: "$D1"
- id: IMG-RUST
  reference: localhost/example-rust
  tag_intent: abc
  digest: "$D1"
- id: IMG-TESTUTIL
  reference: localhost/example-testutil
  tag_intent: abc
  digest: "$D1"
LK
LOCKNORUST="$T/lock-norust.yaml"; grep -v 'IMG-RUST' "$LOCKF" | awk 'BEGIN{skip=0} {print}' >/dev/null
python3 -I - "$LOCKF" "$LOCKNORUST" <<'PY'
import sys,yaml
d=yaml.safe_load(open(sys.argv[1])); d["images"]=[i for i in d["images"] if i["id"]!="IMG-RUST"]
yaml.safe_dump(d,open(sys.argv[2],"w"))
PY
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
PROBE_OK="$T/probe_ok.txt"; printf 'version=tool 1.0\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n' >"$PROBE_OK"
newreg() { rm -rf "$T/reg"; mkdir -p "$T/reg/repo/.audit"; export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1; }
newreg; CK="$T/checkout"; mkdir -p "$CK"
export RUNNER_TEST_MODE=1 RUNNER_RUNP="$SHIMS/run_pinned.sh" RUNNER_SWEEP="$SHIMS/sweep.sh" RUNP_LOCK="$LOCKF" RUNNER_HEARTBEAT_S=1
export ENVELOPE_TEST_MODE=1 ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000
export SHIM_LOG="$T/run.log" SWEEP_LOG="$T/sweep.log" PODMAN_LOG="$T/podman.log" SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1"
export PATH="$SHIMS:$PATH"
resetlogs() { : >"$SHIM_LOG"; : >"$SWEEP_LOG"; : >"$PODMAN_LOG"; unset SHIM_RC RUNNER_REMOTE_CALL; export RUNP_LOCK="$LOCKF"; }
wr() { local w=$1; shift; ( cd "$CK" && bash "$SUTDIR/$w.sh" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
ncalls() { grep -c '^---CALL---$' "$SHIM_LOG"; }
call() { awk -v n="$1" '/^---CALL---$/{c++; next} c==n{print}' "$SHIM_LOG"; }
callarg() { call "$1" | grep -qxF -- "ARG:$2"; }
nops() { ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | wc -l | tr -d ' '; }

# ---------------- run_kcov: the T120 runner contract, class interpreter (local) ----------------
resetlogs; newreg; OUT="$T/out-kcov"
wr run_kcov --out "$OUT" -- echo hello
check "run_kcov: success exit 0" "$RC" 0
check "run_kcov: two containers go through run_pinned.sh (probe, then run)" "$(ncalls)" 2
callarg 1 IMG-KCOV && callarg 2 IMG-KCOV && ok "run_kcov: both calls name IMG-KCOV" || bad "run_kcov: IMG-KCOV missing from the shim calls"
[ -n "$(call 2 | sed -n 's/^ENV:RUNP_MEMORY=//p')" ] && ok "run_kcov: the envelope memory limit is handed to run_pinned.sh" || bad "run_kcov: no RUNP_MEMORY"
check "run_kcov: the long operation ends complete" "$(jq -r .state "$LONGOPS_DIR"/ops/*.json)" complete
check "run_kcov: a toolchain record is written" "$(jq -r .schema "$OUT/toolchain.json" 2>/dev/null)" "toolchain-record/1"
wr run_kcov; check "run_kcov: no arguments is a usage error" "$RC" 2
resetlogs; newreg; SHIM_RC=3 wr run_kcov --out "$T/out-kcov2" -- false
check "run_kcov: the exit code of the run is passed through" "$RC" 3
resetlogs; newreg; RUNP_LOCK="$T/lock-missing.yaml" wr run_kcov --out "$T/out-kcov3" -- true
[ "$RC" = 1 ] && grep -q 'REFUSED reason=lock_unreadable' "$T/stderr" && ok "run_kcov: an unreadable lock is refused" || bad "run_kcov: lock refusal missing (rc $RC)"
resetlogs; newreg; wr run_kcov --image IMG-TESTUTIL --out "$T/out-kcov4" -- true
[ "$RC" = 2 ] && ok "run_kcov: --image is not an option (a single-image wrapper), usage error" || bad "run_kcov: --image accepted (rc $RC)"

# ---------------- run_rust: class compile, a local start is refused with 20 compile_class_local ----------------
resetlogs; newreg
wr run_rust --out "$T/out-rust0" -- cargo --version
check "run_rust: a local start is refused with exit 20" "$RC" 20
grep -q 'REFUSED reason=compile_class_local' "$T/stderr" && ok "run_rust: stderr names compile_class_local" || bad "run_rust: reason missing: $(cat "$T/stderr")"
check "run_rust: a refused local start runs no container (control needle: the shim was never reached)" "$(ncalls)" 0
check "run_rust: a refused local start registers no long operation" "$(nops)" 0
[ ! -s "$PODMAN_LOG" ] && ok "run_rust: a refused local start touches podman not at all" || bad "run_rust: podman was called: $(cat "$PODMAN_LOG")"
resetlogs; newreg
RUNNER_REMOTE_CALL=1 wr run_rust --out "$T/out-rust1" -- cargo --version
check "run_rust: a call composed by the remote emitter (RUNNER_REMOTE_CALL=1) runs: exit 0" "$RC" 0
check "run_rust: two containers (probe, run)" "$(ncalls)" 2
callarg 1 IMG-RUST && callarg 2 IMG-RUST && ok "run_rust: both calls name IMG-RUST" || bad "run_rust: IMG-RUST missing"
check "run_rust: the long operation ends complete" "$(jq -r .state "$LONGOPS_DIR"/ops/*.json)" complete
resetlogs; newreg
RUNNER_REMOTE_CALL=1 RUNP_LOCK="$LOCKNORUST" wr run_rust --out "$T/out-rust2" -- cargo --version
[ "$RC" = 1 ] && grep -q 'REFUSED reason=image_not_in_lock' "$T/stderr" && grep -q 'T143' "$T/stderr" && ok "run_rust: IMG-RUST not in the lock is refused naming T143 (BLOCKED, never another image)" || bad "run_rust: not-in-lock refusal missing (rc $RC): $(cat "$T/stderr")"
check "run_rust: that refusal runs no container" "$(ncalls)" 0
resetlogs; newreg; RUNNER_REMOTE_CALL=0 wr run_rust --out "$T/out-rust3" -- true
check "run_rust: RUNNER_REMOTE_CALL=0 is not a remote call: refused 20" "$RC" 20
resetlogs; newreg; RUNNER_REMOTE_CALL=yes wr run_rust --out "$T/out-rust4" -- true
check "run_rust: only RUNNER_REMOTE_CALL=1 counts as the emitter: refused 20" "$RC" 20
resetlogs; RUNNER_REMOTE_CALL=1 wr run_rust; check "run_rust: no arguments is a usage error even for a remote call" "$RC" 2

# ---------------- the lane rows, through the REAL test-in-container.sh with a shim wrapper directory ----------------
WD="$T/wrappers"; mkdir -p "$WD"
for w in run_kcov run_rust run_testutil run_node run_go run_docs run_scan run_playwright run_qa; do
  printf '#!/usr/bin/env bash\necho "WRAPPER %s $*" >>"${TIC_LOG:?}"\nexit 0\n' "$w" >"$WD/$w.sh"
done
export TIC_TEST_MODE=1 TIC_WRAPPER_DIR="$WD" TIC_LOG="$T/tic.log"
tic() { ( cd "$REPO" && bash "$TICDIR/test-in-container.sh" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
: >"$TIC_LOG"; tic build-scripts unit -- true
check "lane: TIC build-scripts unit -- true exits 0" "$RC" 0
grep -q '^WRAPPER run_kcov ' "$TIC_LOG" && ok "lane needle: build-scripts unit reaches the run_kcov wrapper (never run_testutil, never a bare podman)" || bad "lane needle: build-scripts unit did not reach run_kcov: $(cat "$TIC_LOG") $(cat "$T/stderr")"
: >"$TIC_LOG"; tic installer-wizard rust -- true
check "lane: TIC installer-wizard rust -- true exits 0 on the shim" "$RC" 0
grep -q '^WRAPPER run_rust ' "$TIC_LOG" && ok "lane needle: installer-wizard rust reaches the run_rust wrapper" || bad "lane needle: installer-wizard rust did not reach run_rust: $(cat "$TIC_LOG") $(cat "$T/stderr")"
: >"$TIC_LOG"; tic catalogizer-desktop rust -- true
grep -q '^WRAPPER run_rust ' "$TIC_LOG" && ok "lane needle: catalogizer-desktop rust reaches the run_rust wrapper" || bad "lane needle: catalogizer-desktop rust did not reach run_rust"
: >"$TIC_LOG"; tic not-an-app unit -- true
[ "$RC" = 1 ] && grep -q 'REFUSED reason=unknown_app' "$T/stderr" && [ ! -s "$TIC_LOG" ] && ok "lane: an app key not in the table is still refused (unknown_app), nothing ran" || bad "lane: unknown app not refused (rc $RC)"
: >"$TIC_LOG"; tic build-scripts rust -- true
[ "$RC" = 1 ] && grep -q 'REFUSED reason=no_lane_row' "$T/stderr" && [ ! -s "$TIC_LOG" ] && ok "lane: (build-scripts, rust) has no row and is refused" || bad "lane: build-scripts rust not refused (rc $RC)"
: >"$TIC_LOG"; tic catalogizer-desktop unit -- true
[ "$RC" = 1 ] && [ ! -s "$TIC_LOG" ] && ok "lane: (catalogizer-desktop, unit) stays refused: its remote row waits for the T121a site column (BLOCKED)" || bad "lane: catalogizer-desktop unit ran or exited $RC"
# every row of the table: a wrapper that is a reviewed one, exactly three columns (TIC itself validates; here the table is also checked as data)
python3 -I - "$REPO/scripts/containers/lanes.tsv" <<'PY' && ok "lane table: the added coverage rows are exactly the reviewed ones" || bad "lane table: unexpected coverage rows"
import sys
rows=[l.rstrip("\n").split("\t") for l in open(sys.argv[1]) if l.strip() and not l.startswith("#")]
d={(r[0],r[1]):r[2] for r in rows}
assert d.get(("build-scripts","unit"))=="run_kcov", d
assert d.get(("installer-wizard","rust"))=="run_rust", d
assert d.get(("catalogizer-desktop","rust"))=="run_rust", d
assert all(len(r)==3 for r in rows)
assert not any(w=="run_node" and a not in ("catalog-web",) for (a,l),w in d.items()), "a remote node row must wait for the T121a site column"
PY

# ---------------- paired mutations ----------------
if [ "${1:-}" != --no-mutations ] && [ -z "${RUNNER_SUT_DIR:-}" ] && [ -z "${TIC_SUT_DIR:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  mutcp() { # mutcp NAME FILE(relative to repo scripts/) OLD NEW [kind: containers|tic|lanes]
    local name="$1" rel="$2" old="$3" new="$4" kind="${5:-containers}" dst
    rm -rf "$T/mut-$name"; mkdir -p "$T/mut-$name"
    if [ "$kind" = containers ]; then mkdir -p "$T/mut-$name/scripts"; cp -a "$HERE/.." "$T/mut-$name/scripts/containers"; ln -s "$REPO/scripts/longops" "$T/mut-$name/scripts/longops"; ln -s "$REPO/scripts/anti-mess" "$T/mut-$name/scripts/anti-mess"; dst="$T/mut-$name/scripts/containers/$rel"
    elif [ "$kind" = lanes ]; then cp -a "$REPO/scripts" "$T/mut-$name/scripts"; dst="$T/mut-$name/scripts/containers/lanes.tsv"
    else cp -a "$REPO/scripts" "$T/mut-$name/scripts"; dst="$T/mut-$name/scripts/$rel"; fi
    python3 -I - "$dst" "$old" "$new" <<'PY' || { bad "mutation $name: anchor not unique/absent"; return; }
import sys
s=open(sys.argv[1]).read(); old,new=sys.argv[2].replace("\\t","\t"),sys.argv[3].replace("\\n","\n").replace("\\t","\t")
if s.count(old)!=1: sys.exit(1)
open(sys.argv[1],"w").write(s.replace(old,new))
PY
    local env=(RUNNER_SUT_DIR="$T/mut-$name/scripts/containers")
    [ "$kind" = containers ] || env=(TIC_SUT_DIR="$T/mut-$name/scripts")
    if env "${env[@]}" bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-$name.out" 2>&1; then bad "mutation $name SURVIVED"; echo "SURVIVED $name" >>"$REC"
    else ok "mutation $name caught ($(grep -c '^FAIL:' "$T/mut-$name.out") failing legs)"; echo "CAUGHT $name: $(grep '^FAIL:' "$T/mut-$name.out" | head -2 | cut -c1-120 | tr '\n' '|')" >>"$REC"; fi
  }
  mutcp rust-local-allowed run_rust.sh '[ "${RUNNER_REMOTE_CALL:-}" != 1 ]' 'false'
  mutcp rust-image-swapped run_rust.sh 'RUNNER_IMAGES="IMG-RUST"' 'RUNNER_IMAGES="IMG-TESTUTIL"'
  mutcp kcov-image-swapped run_kcov.sh 'RUNNER_IMAGES="IMG-KCOV"' 'RUNNER_IMAGES="IMG-TESTUTIL"'
  mutcp lane-kcov-to-testutil lanes.tsv 'build-scripts\tunit\trun_kcov' 'build-scripts\tunit\trun_testutil' lanes
  mutcp lane-rust-local lanes.tsv 'installer-wizard\trust\trun_rust' 'installer-wizard\trust\trun_testutil' lanes
  mutcp lane-desktop-node-row lanes.tsv 'catalogizer-desktop\trust\trun_rust' 'catalogizer-desktop\trust\trun_rust\ncatalogizer-desktop\tunit\trun_node' lanes
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"
[ "$FAILS" = 0 ]
