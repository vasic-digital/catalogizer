#!/usr/bin/env bash
# run_profile.sh - T217 (doc12 8.1). The ONE QA execution wrapper: resolves a run profile from challenges/helixqa-banks/MANIFEST.yaml, refuses below the floor,
# runs the availability probes FIRST, runs `helixqa run` three times with the conduit stream, adapts the events to the ledger (conduit_to_ledger.py, T215),
# computes the canonical result hash per run (canonical_result.py, doc12 8.3) and exits non-zero on any fail, blocked, invalid or nondeterministic outcome.
# It runs INSIDE the QA image (IMG-QA) from the read-only source mount; it is not baked into the image. It never pipes the runner through a command that
# replaces its exit status (QF-05): the status is captured in a variable first, then the output is post-processed.
# Usage: run_profile.sh --profile P [--manifest F] [--banks-dir D] [--out DIR] [--helixqa CMD] [--probes-dir D] [--runs N] [--fingerprint VALUE]
#   --profile      api | web | desktop | android | androidtv | installer (a profile listed by the manifest)
#   --manifest     default challenges/helixqa-banks/MANIFEST.yaml of this checkout;  --banks-dir default: the manifest's directory
#   --out          default <checkout>/.audit/out/qa-<profile>-<utc timestamp>;  --helixqa default `helixqa`;  --probes-dir default scripts/qa/probes
#   --runs         default 3 (the canonical-hash repetition of doc12 8.3);  --fingerprint the target fingerprint READ FROM THE TARGET at run time (11.4.115 F);
#                  without it the build id the `service` probe read from the target is used; neither: the run is invalid
# Outputs under --out: probes.jsonl (written first), run-<i>/ (runner output + conduit.events.jsonl), ledger-<i>.jsonl, canonical-run-<i>.json, summary.json.
# Exits: 0 every verdict pass and one canonical hash over all runs; 1 at least one `fail` verdict; 2 usage; 3 REFUSED before running (profile_unknown,
#   manifest_image_unset, below_floor, bank_missing, ...: `run_profile: REFUSED reason=<code>` on stderr); 4 blocked (a probe or a verdict: blocked is never pass);
#   5 invalid run (SKIP in the deterministic lane, no conduit stream, missing evidence/fingerprint, reconciliation failure); 6 nondeterministic (the canonical
#   hashes of the runs differ); 7 the runner itself crashed (non-zero status of `helixqa run`).
# Honest boundary: `helixqa run` does not itself emit the conduit stream in the pinned submodule (the writer is wired in `helixqa autonomous` only,
# cmd/helixqa/main.go:884-905 vs cmdRun 119-262, read statically); a run that produces no stream is exit 5, never a fabricated verdict.
set -u
QA="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$QA/../.." && pwd)"
PROFILE=""; MANIFEST="$REPO/challenges/helixqa-banks/MANIFEST.yaml"; BANKS_DIR=""; OUT=""; HQ="helixqa"; PROBES_DIR="$QA/probes"; RUNS=3; FP=""
refuse() { echo "run_profile: REFUSED reason=$1 ${2:-}" >&2; exit 3; }
usage() { echo "run_profile: usage: $1" >&2; echo "run_profile: run_profile.sh --profile P [--manifest F] [--banks-dir D] [--out DIR] [--helixqa CMD] [--probes-dir D] [--runs N] [--fingerprint V]" >&2; exit 2; }
while [ $# -gt 0 ]; do
  case "$1" in
    --profile) [ $# -ge 2 ] || usage "--profile requires a value"; PROFILE="$2"; shift 2;;
    --manifest) [ $# -ge 2 ] || usage "--manifest requires a value"; MANIFEST="$2"; shift 2;;
    --banks-dir) [ $# -ge 2 ] || usage "--banks-dir requires a value"; BANKS_DIR="$2"; shift 2;;
    --out) [ $# -ge 2 ] || usage "--out requires a value"; OUT="$2"; shift 2;;
    --helixqa) [ $# -ge 2 ] || usage "--helixqa requires a value"; HQ="$2"; shift 2;;
    --probes-dir) [ $# -ge 2 ] || usage "--probes-dir requires a value"; PROBES_DIR="$2"; shift 2;;
    --runs) [ $# -ge 2 ] || usage "--runs requires a value"; RUNS="$2"; shift 2;;
    --fingerprint) [ $# -ge 2 ] || usage "--fingerprint requires a value"; FP="$2"; shift 2;;
    *) usage "unknown option '$1'";;
  esac
done
[ -n "$PROFILE" ] || usage "--profile is required"
case "$RUNS" in ''|*[!0-9]*|0) usage "--runs must be a positive integer";; esac
[ -n "$BANKS_DIR" ] || BANKS_DIR="$(dirname "$MANIFEST")"
[ -n "$OUT" ] || OUT="$REPO/.audit/out/qa-$PROFILE-$(date -u +%Y%m%dT%H%M%SZ)"

# ---- 1. resolve the profile from the manifest ----
RES="$(python3 -I "$QA/manifest_resolve.py" --manifest "$MANIFEST" --profile "$PROFILE" --banks-dir "$BANKS_DIR" 2>&1)"; rrc=$?
if [ "$rrc" != 0 ]; then
  code="$(printf '%s\n' "$RES" | awk -F'\t' '$1=="ERR"{print $2; exit}')"; detail="$(printf '%s\n' "$RES" | awk -F'\t' '$1=="ERR"{print $3; exit}')"
  refuse "${code:-manifest_unreadable}" "${detail:-$(printf '%s' "$RES" | head -c 200)}"
fi
IMG_DIGEST="$(printf '%s\n' "$RES" | awk -F'\t' '$1=="IMAGE"{print $3}')"
[ -n "$IMG_DIGEST" ] || refuse manifest_image_unset "the manifest records no IMG-QA digest (T211 not delivered): a run on an unrecorded image is never started"
PLATFORM="$(printf '%s\n' "$RES" | awk -F'\t' '$1=="PLATFORM"{print $2}')"
BANKFILES=""
while IFS=$'\t' read -r kind file floor held; do
  [ "$kind" = BANK ] || continue
  [ "$held" -ge "$floor" ] || refuse below_floor "$file holds $held case(s), its manifest floor is $floor"   # MUT:below-floor
  BANKFILES="${BANKFILES:+$BANKFILES,}$BANKS_DIR/$file"
done <<<"$RES"
[ -n "$BANKFILES" ] || refuse no_bank_for_profile "$PROFILE"
mkdir -p "$OUT" || refuse out_not_writable "$OUT"

# ---- 2. probes first (doc12 10.3): a blocked dependency means the runner is never started ----
run_probes() {
  local blocked=0 kind name rest args
  : >"$OUT/probes.jsonl"
  while IFS=$'\t' read -r kind name rest; do
    case "$kind" in
      PROBEENV) printf '{"probe":"%s","status":"blocked","reason":"credential_absent","evidence":{"env":"%s","set":false,"needed_for":"probe arguments"}}\n' "$name" "$rest" >>"$OUT/probes.jsonl"; blocked=1;;
      PROBE)
        mapfile -t PARGS < <(python3 -I -c 'import json,sys; [print(a) for a in json.loads(sys.argv[1])]' "$rest")
        if [ -f "$PROBES_DIR/$name.py" ]; then
          python3 -I "$PROBES_DIR/$name.py" "${PARGS[@]}" >>"$OUT/probes.jsonl" 2>"$OUT/probe-$name.err"; prc=$?
          [ "$prc" = 0 ] || blocked=1
        else
          printf '{"probe":"%s","status":"blocked","reason":"host_resource_unavailable","evidence":{"error":"probe script absent"}}\n' "$name" >>"$OUT/probes.jsonl"; blocked=1
        fi;;
    esac
  done <<<"$RES"
  [ "$blocked" = 0 ]
}
run_probes || exit 4   # MUT:probes-first
# the target fingerprint: given, or the build id the service probe read from the target
if [ -z "$FP" ]; then
  FP="$(python3 -I -c 'import json,sys
for l in open(sys.argv[1]):
    d = json.loads(l)
    b = d.get("evidence", {}).get("build_id")
    if d.get("status") == "present" and b:
        print(b); break' "$OUT/probes.jsonl" 2>/dev/null)"
fi

# ---- 3. the runs ----
LANE=deterministic   # MUT:lane
RUN_ID="qa-$PROFILE-$(date -u +%Y%m%dT%H%M%SZ)-$$"
H1=""; HASH_DIFF=0; EXIT=0
for i in $(seq 1 "$RUNS"); do
  RUNDIR="$OUT/run-$i"; mkdir -p "$RUNDIR"
  "$HQ" run --banks "$BANKFILES" --platform "$PLATFORM" --output "$RUNDIR" >"$RUNDIR/stdout.txt" 2>"$RUNDIR/stderr.txt"; RC=$?   # MUT:runner-status
  if [ "$RC" != 0 ]; then echo "run_profile: the runner exited $RC in run $i (see $RUNDIR/stderr.txt)" >&2; exit 7; fi
  [ -s "$RUNDIR/conduit.events.jsonl" ] || { echo "run_profile: run $i produced no conduit stream ($RUNDIR/conduit.events.jsonl): no verdict can be adapted, never fabricated" >&2; exit 5; }
  FPARG=(); [ -z "$FP" ] || FPARG=(--target-fingerprint "$FP")
  python3 -I "$QA/conduit_to_ledger.py" --stream "$RUNDIR/conduit.events.jsonl" --ledger "$OUT/ledger-$i.jsonl" --run "$RUN_ID-$i" --lane "$LANE" "${FPARG[@]}" \
    --advisory-store "$OUT/advisory-$i.jsonl" --evidence-root "$RUNDIR" >"$OUT/adapt-$i.txt" 2>"$OUT/adapt-$i.err"; ARC=$?
  if [ "$ARC" != 0 ]; then echo "run_profile: run $i is INVALID: $(head -c 400 "$OUT/adapt-$i.err")" >&2; exit 5; fi
  H="$(python3 -I "$QA/canonical_result.py" --stream "$RUNDIR/conduit.events.jsonl" --out "$OUT/canonical-run-$i.json")" || { echo "run_profile: canonical result failed in run $i" >&2; exit 5; }
  if [ "$i" = 1 ]; then H1="$H"; fi
  [ "$H" = "$H1" ] || HASH_DIFF=1   # MUT:hash-compare
done

# ---- 4. summary and exit status ----
read -r N_PASS N_FAIL N_BLOCKED < <(python3 -I -c 'import json,sys
c = {"pass": 0, "fail": 0, "blocked": 0}
for l in open(sys.argv[1]):
    v = json.loads(l).get("verdict")
    if v in c: c[v] += 1
print(c["pass"], c["fail"], c["blocked"])' "$OUT/ledger-1.jsonl")
[ "$HASH_DIFF" = 0 ] || EXIT=6
if [ "$EXIT" = 0 ]; then
  [ "$N_FAIL" = 0 ] || EXIT=1
  [ "$N_BLOCKED" = 0 ] || [ "$EXIT" != 0 ] || EXIT=4   # MUT:blocked-as-pass
fi
python3 -I -c 'import json,sys
a = sys.argv
json.dump({"schema": "qa-run-summary/1", "profile": a[1], "runs": int(a[2]), "pass": int(a[3]), "fail": int(a[4]), "blocked": int(a[5]), "canonical_hash": a[6], "hash_stable": a[7] == "0",
           "reconciled": True, "target_fingerprint": a[8], "lane": a[9], "exit_code": int(a[11])}, open(a[10], "w"), indent=1, sort_keys=True)' \
  "$PROFILE" "$RUNS" "$N_PASS" "$N_FAIL" "$N_BLOCKED" "$H1" "$HASH_DIFF" "$FP" "$LANE" "$OUT/summary.json" "$EXIT"
echo "run_profile: profile=$PROFILE runs=$RUNS pass=$N_PASS fail=$N_FAIL blocked=$N_BLOCKED hash=$H1 hash_stable=$([ "$HASH_DIFF" = 0 ] && echo yes || echo NO) exit=$EXIT"
exit $EXIT
