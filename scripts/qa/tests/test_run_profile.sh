#!/usr/bin/env bash
# test_run_profile.sh - T217 (RED first: scripts/qa/run_profile.sh is absent). Oracle for the QA wrapper scripts/qa/run_profile.sh (doc12 8.1).
# `helixqa run` is SHIMMED (a bash script driven by SHIM_MODE that writes a conduit.events.jsonl like helixqa autonomous does); the probes of the
# fixture manifest are STUBS (a probes dir of shell scripts) except in the one leg that runs the real scripts/qa/probes against a closed port.
# The shim is the only fake; the wrapper, the adapter (T215), the canonical-result tool and the real probes are the real files under test.
# Legs: resolve profile from the manifest; refuse unknown profile / unset image digest / below the floor; probes first (a blocked probe stops the run:
#   the runner is never started); runner exit status captured in a variable (a crashing runner is not hidden by post-processing, QF-05);
#   verdict fail -> exit 1; blocked -> exit 4 (never pass, gate CM-QA-BLOCKED-NOT-PASS); runner crash -> exit 7; SKIP in the deterministic lane -> exit 5 (invalid);
#   three runs with one canonical hash -> exit 0, a different verdict in run 2 -> exit 6; absent conduit stream -> exit 5 (never fabricated).
# Paired mutations (each makes the body FAIL): runner status through a pipe, blocked treated as pass, floor check removed, probes not first,
#   hash comparison removed, deterministic lane switched to exploratory.
# Usage: bash scripts/qa/tests/test_run_profile.sh    (RUN_PROFILE_NO_MUTATIONS=1 skips the mutation legs). Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QA="$(cd "$HERE/.." && pwd)"
SUT="${RUN_PROFILE_SUT:-$QA/run_profile.sh}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
[ -f "$SUT" ] || { echo "FAIL: $SUT not found (T217 not implemented)"; echo "RESULT pass=0 fail=1"; exit 1; }
for d in python3 jq; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d required"; exit 2; }; done
T="$(mktemp -d "${TMPDIR:-/tmp}/run-profile-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT

# ---- fixtures: a bank dir with two banks, a manifest, a shim helixqa, stub probes ----
BD="$T/banks"; mkdir -p "$BD"
cp "$HERE/fixtures/banks/good.yaml" "$BD/bank-a.yaml"
python3 -I - "$BD" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1] + "/bank-a.yaml"))
d["name"] = "bank-b"; d["test_cases"] = [d["test_cases"][0]]; d["test_cases"][0]["id"] = "good-login-b"
yaml.safe_dump(d, open(sys.argv[1] + "/bank-b.yaml", "w"))
PY
MAN="$T/MANIFEST.yaml"
cat >"$MAN" <<'EOF'
schema: qa-manifest/1
image: {id: IMG-QA, digest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", status: PINNED}
profiles: [api, android]
profile_probes:
  api:
    - {probe: service, args: ["--url", "http://127.0.0.1:1/health"]}
  android:
    - {probe: device, args: ["--serial", "emulator-5554"]}
banks:
  - {file: bank-a.yaml, platforms: [api, android], profiles: [api], case_floor: 2}
  - {file: bank-b.yaml, platforms: [api], profiles: [api], case_floor: 1}
EOF
HQ="$T/helixqa"
cat >"$HQ" <<'SH'
#!/usr/bin/env bash
# shim helixqa: `helixqa run --banks FILES --platform P --output DIR`; SHIM_MODE: pass fail blocked skip nondet noconduit crash
out=""; while [ $# -gt 0 ]; do [ "$1" = --output ] && out="$2"; shift; done
echo "$out" >>"${SHIM_CALLS:?}"
n=$(wc -l <"$SHIM_CALLS")
mkdir -p "$out"
ev() { printf '{"seq":%s,"type":"%s","session":"s"%s}\n' "$1" "$2" "$3" >>"$out/conduit.events.jsonl"; }
: >"$out/conduit.events.jsonl"
[ "${SHIM_MODE:-pass}" = noconduit ] && { echo "no stream"; exit 0; }
step() { ev "$1" challenge_step ',"challenge":"good-login","step":"0","verdict":"PASS","fields":{"assertion_digest":"d1"}'; }
step 1
case "${SHIM_MODE:-pass}" in
  pass)    ev 2 challenge_verdict ',"challenge":"good-login","verdict":"PASS"'; ev 3 challenge_verdict ',"challenge":"good-login-b","verdict":"PASS"';;
  fail)    ev 2 challenge_verdict ',"challenge":"good-login","verdict":"FAIL"'; ev 3 challenge_verdict ',"challenge":"good-login-b","verdict":"PASS"';;
  blocked) ev 2 challenge_verdict ',"challenge":"good-login","verdict":"OPERATOR-BLOCKED","reason":"service_unreachable"'; ev 3 challenge_verdict ',"challenge":"good-login-b","verdict":"PASS"';;
  skip)    ev 2 challenge_verdict ',"challenge":"good-login","verdict":"SKIP","reason":"x"'; ev 3 challenge_verdict ',"challenge":"good-login-b","verdict":"PASS"';;
  nondet)  v=PASS; [ "$n" = 2 ] && v=FAIL; ev 2 challenge_verdict ",\"challenge\":\"good-login\",\"verdict\":\"$v\""; ev 3 challenge_verdict ',"challenge":"good-login-b","verdict":"PASS"';;
  crash)   echo "panic: runner crashed" >&2; exit 3;;
esac
# the runner's own exit status: nonzero only on crash (a failing verdict is the verdict's business, not the process status)
exit 0
SH
chmod +x "$HQ"
PD="$T/probes"; mkdir -p "$PD"
for p in service device; do
  cat >"$PD/$p.py" <<'PY'
#!/usr/bin/env python3
import json, os, sys
name = os.path.basename(sys.argv[0])[:-3]
if os.environ.get("STUB_PROBE_" + name.upper(), "present") == "present":
    print(json.dumps({"probe": name, "status": "present", "evidence": {}})); sys.exit(0)
print(json.dumps({"probe": name, "status": "blocked", "reason": "service_unreachable" if name == "service" else "device_absent", "evidence": {"stub": True}})); sys.exit(1)
PY
done
export SHIM_CALLS="$T/calls.txt"
run() { # run <profile> [extra args]: sets RC, OUT
  OUT="$T/out-$RANDOM"; : >"$SHIM_CALLS"; local prof="$1"; shift
  bash "$SUT" --profile "$prof" --manifest "$MAN" --banks-dir "$BD" --out "$OUT" --helixqa "$HQ" --probes-dir "$PD" --fingerprint "sha256:$(printf 'f%.0s' $(seq 64))" "$@" >"$T/stdout" 2>"$T/stderr"; RC=$?
}
runs() { wc -l <"$SHIM_CALLS" | tr -d ' '; }
refused() { grep -q "REFUSED reason=$1" "$T/stderr"; }

# ---- usage and refusals ----
bash "$SUT" >/dev/null 2>&1; check "no arguments: usage error" "$?" 2
run nosuch; check "unknown profile: refused (exit 3)" "$RC" 3; refused profile_unknown && ok "reason profile_unknown" || bad "reason: $(cat "$T/stderr")"
check "unknown profile: the runner never started" "$(runs)" 0
sed 's/digest: "sha256:e*"/digest: null/' "$MAN" >"$T/m2.yaml"; MAN_KEEP="$MAN"; MAN="$T/m2.yaml"
run api; check "manifest image digest unset: refused (exit 3)" "$RC" 3; refused manifest_image_unset && ok "reason manifest_image_unset" || bad "reason: $(cat "$T/stderr")"
MAN="$MAN_KEEP"
sed 's/case_floor: 2/case_floor: 3/' "$MAN" >"$T/m3.yaml"; MAN="$T/m3.yaml"
run api; check "bank below its case floor: refused (exit 3)" "$RC" 3; refused below_floor && ok "reason below_floor" || bad "reason: $(cat "$T/stderr")"
check "below the floor: the runner never started" "$(runs)" 0
MAN="$MAN_KEEP"

# ---- probes first ----
export STUB_PROBE_SERVICE=blocked
run api; check "blocked probe: exit 4 (never pass)" "$RC" 4
check "blocked probe: the runner is never started" "$(runs)" 0
[ -s "$OUT/probes.jsonl" ] && jq -e 'select(.status=="blocked" and .reason=="service_unreachable")' "$OUT/probes.jsonl" >/dev/null && ok "probe result written first, with its closed-set reason" || bad "probes.jsonl: $(cat "$OUT/probes.jsonl" 2>&1)"
unset STUB_PROBE_SERVICE
# real probes against a closed port: blocked with the real reason, never a pass
OUT="$T/out-real"; : >"$SHIM_CALLS"
bash "$SUT" --profile api --manifest "$MAN" --banks-dir "$BD" --out "$OUT" --helixqa "$HQ" --probes-dir "$QA/probes" --fingerprint "sha256:$(printf 'f%.0s' $(seq 64))" >"$T/stdout" 2>"$T/stderr"; RC=$?
check "real service probe on a closed port: exit 4" "$RC" 4
jq -e 'select(.reason=="service_unreachable")' "$OUT/probes.jsonl" >/dev/null 2>&1 && ok "real probe reports service_unreachable" || bad "real probes.jsonl: $(cat "$OUT/probes.jsonl" 2>&1)"

# ---- the runs ----
export SHIM_MODE=pass
run api; check "all pass: exit 0" "$RC" 0
check "three runs of the runner" "$(runs)" 3
HS=$(jq -r .canonical_hash "$OUT"/canonical-run-*.json 2>/dev/null | sort -u | wc -l | tr -d ' ')
check "one canonical hash across the three runs" "$HS" 1
[ -s "$OUT/summary.json" ] && check "summary: verdict counts" "$(jq -c '[.pass,.fail,.blocked]' "$OUT/summary.json")" "[2,0,0]" || bad "summary.json missing"
check "summary: runs and reconciliation" "$(jq -c '[.runs,.reconciled]' "$OUT/summary.json" 2>/dev/null)" "[3,true]"
export SHIM_MODE=fail
run api; check "a failing verdict: exit 1" "$RC" 1
export SHIM_MODE=blocked
run api; check "a blocked verdict: exit 4 (blocked is not pass, CM-QA-BLOCKED-NOT-PASS)" "$RC" 4
export SHIM_MODE=skip
run api; check "SKIP in the deterministic lane: exit 5 (invalid run)" "$RC" 5
export SHIM_MODE=nondet
run api; check "a different verdict in run 2: exit 6 (nondeterministic)" "$RC" 6
export SHIM_MODE=crash
run api; check "a crashing runner is reported by its own status, not hidden by post-processing (QF-05): exit 7" "$RC" 7
export SHIM_MODE=noconduit
run api; check "no conduit stream from the runner: exit 5, never fabricated" "$RC" 5
grep -qi 'conduit' "$T/stderr" && ok "the refusal names the missing conduit stream" || bad "stderr: $(cat "$T/stderr")"
unset SHIM_MODE

echo "RESULT pass=$PASSES fail=$FAILS"
if [ "${RUN_PROFILE_MUTANT:-0}" != 1 ] && [ "${RUN_PROFILE_NO_MUTATIONS:-0}" != 1 ]; then
  mut() { # mut <name> <replacement line>: a copy of the wrapper with the line tagged `# MUT:<name>` replaced; the test body must FAIL against it
    local name="$1" repl="$2" M="$T/mut-$1"; mkdir -p "$M"; cp -r "$QA"/. "$M"/ 2>/dev/null; rm -rf "$M/tests"
    python3 -I - "$M/run_profile.sh" "$name" "$repl" <<'PY' || { bad "mutation $name: anchor not found exactly once"; return; }
import sys
p, name, repl = sys.argv[1:4]
lines = open(p).read().split("\n")
idx = [i for i, l in enumerate(lines) if l.rstrip().endswith("# MUT:" + name)]
if len(idx) != 1:
    sys.exit(1)
lines[idx[0]] = repl + "   # MUT:" + name
open(p, "w").write("\n".join(lines))
PY
    mkdir -p "$M/tests"; cp -r "$HERE"/. "$M/tests/"
    RUN_PROFILE_SUT="$M/run_profile.sh" RUN_PROFILE_MUTANT=1 bash "$M/tests/test_run_profile.sh" >"$T/mut.$name.out" 2>&1; local mrc=$?
    if [ "$mrc" != 0 ]; then echo "MUTATION $name: test FAILED as required (rc=$mrc)"; else bad "mutation $name went undetected"; fi
  }
  mut runner-status '  "$HQ" run --banks "$BANKFILES" --platform "$PLATFORM" --output "$RUNDIR" 2>&1 | tee "$RUNDIR/stdout.txt" >/dev/null; RC=$?'
  mut blocked-as-pass 'true'
  mut below-floor 'true'
  mut probes-first 'true'
  mut hash-compare 'true'
  mut lane 'LANE=exploratory'
fi
[ "$FAILS" = 0 ] && exit 0 || exit 1
