#!/usr/bin/env bash
# T016 test (TDD, RED first): scripts/audit/index_health.sh gate for proofs P1..P8 (docs/02 section 4.1, 4.2).
#
# Purpose   Drive scripts/audit/index_health.sh with FIXTURE inputs and assert that each bad fixture FAILS on the
#           proof it breaks (exit 1, `index-health/1` JSON with that proof id FAIL and top-level verdict FAIL) and that
#           the all-good fixture PASSES (exit 0, every proof PASS or SKIP-with-reason, verdict PASS).
# Usage     RUNP IMG-TESTUTIL -- bash scripts/audit/tests/test_index_health.sh   (fixtures parsed with jq and python3)
#           Env: IH=<path to the script under test> (default scripts/audit/index_health.sh; the mutation test of T021
#           points it at a mutated copy).
# Contract of the script under test (this test DEFINES it; T021 implements it):
#   index_health.sh --out FILE --cg-status FILE --cg-files FILE --tracked FILE --needle-pos PATH --needle-neg PATH
#                   [--lumen-status FILE | --lumen-bin FILE] [--lumen-files FILE] [--parity-tolerance-pct N]
#                   [--lumen-unindexable-ext CSV]
#     --cg-status   `codegraph status --json` output          --cg-files  `codegraph files --json` output (list of {path})
#     --tracked     newline list of in-scope tracked paths    --lumen-files newline list of paths Lumen indexed
#     --lumen-status  captured MCP index_status text ("Files: N | Indexed: N | Chunks: N | ... | Stale: yes|no") plus a
#                   "Captured: <iso time>" line; without it, --lumen-bin FILE drives a `lumen search` probe
#     exit 0 = PASS, 1 = FAIL (any proof FAIL), 2 = usage error. JSON: {"schema":"index-health/1","verdict":...,
#     "codegraph":{"proofs":[{"id","verdict",...}]},"lumen":{"proofs":[...]},"parity":{...}} where every proof row has
#     "id" in P1..P8 or "P-Lumen", "verdict" in PASS|FAIL|SKIP.
# Side effects  temp dir only. Needs jq and python3.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
IH="${IH:-scripts/audit/index_health.sh}"
T="$(mktemp -d "${TMPDIR:-/tmp}/ih_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }

# ---- fixtures ---------------------------------------------------------------------------------------------------
POS="catalog-api/cmd/server/main.go"; NEG="zz/does_not_exist.go"
printf '%s\n' "$POS" "catalog-api/handlers/scan.go" "submodules/auth/auth.go" > "$T/tracked.txt"
python3 - "$T" "$POS" <<'PY'
import json, sys
t, pos = sys.argv[1:3]
status = {"initialized": True, "version": "1.6.0", "fileCount": 3, "nodeCount": 10, "edgeCount": 20,
          "pendingChanges": {"added": 0, "modified": 0, "removed": 0}, "worktreeMismatch": None,
          "index": {"builtWithVersion": "1.6.0", "currentExtractionVersion": 25, "reindexRecommended": False,
                    "state": "complete", "pendingRefs": 0}}
json.dump(status, open(t + "/cg_status_good.json", "w"))
files = [{"path": p, "language": "go", "nodeCount": 1, "size": 1} for p in
         (pos, "catalog-api/handlers/scan.go", "submodules/auth/auth.go")]
json.dump(files, open(t + "/cg_files_good.json", "w"))
PY
jq '.index.pendingRefs=5' "$T/cg_status_good.json" > "$T/cg_status_pending.json"
jq 'del(.index.pendingRefs)' "$T/cg_status_good.json" > "$T/cg_status_nofield.json"
jq '.worktreeMismatch="/other/worktree"' "$T/cg_status_good.json" > "$T/cg_status_wt.json"
jq --arg n "$NEG" '. + [{"path":$n,"language":"go","nodeCount":0,"size":0}]' "$T/cg_files_good.json" > "$T/cg_files_negfound.json"
jq --arg p "$POS" 'map(select(.path != $p))' "$T/cg_files_good.json" > "$T/cg_files_posmissing.json"
cp "$T/cg_files_good.json" "$T/cg_files_parity.json"
cat > "$T/lumen_good.txt" <<'TXT'
Files: 3 | Indexed: 3 | Chunks: 30 | Vectors: 30 unique | Storage: int8 | Last indexed: 2026-10-05T10:00:00Z | Stale: no
Captured: 2026-10-05T10:05:00Z
TXT
sed 's/Stale: no/Stale: yes/' "$T/lumen_good.txt" > "$T/lumen_stale.txt"
# Lumen indexed set: the three CG files plus one .sh Lumen cannot chunk (removed by the unindexable-extension rule)
printf '%s\n' "$POS" "catalog-api/handlers/scan.go" "submodules/auth/auth.go" > "$T/lumen_files_good.txt"
# parity-bad: Lumen indexes 4 extra chunkable files CG does not (outside tolerance of 2 percent)
{ cat "$T/lumen_files_good.txt"; printf 'extra/a.go\nextra/b.go\nextra/c.go\nextra/d.go\n'; } > "$T/lumen_files_parity.txt"

# shellcheck disable=SC2054  # "sh,kt,txt" is ONE comma-separated option value, not a list of array words
BASE=( --cg-status "$T/cg_status_good.json" --cg-files "$T/cg_files_good.json" --tracked "$T/tracked.txt"
       --needle-pos "$POS" --needle-neg "$NEG" --lumen-status "$T/lumen_good.txt"
       --lumen-files "$T/lumen_files_good.txt" --parity-tolerance-pct 2 --lumen-unindexable-ext sh,kt,txt )

# run NAME  [override args...]  -> sets RC and OUTJ; later duplicate options override earlier ones only by position, so
# the helper rebuilds the argument list from BASE with the named option replaced.
run() {  # run <name> <opt=value>...   (opt without leading dashes; value replaces BASE's value; "opt=-" removes the option)
  local name="$1"; shift
  local -a args=("${BASE[@]}") ; local kv k v i
  for kv in "$@"; do
    k="--${kv%%=*}"; v="${kv#*=}"
    for ((i=0;i<${#args[@]};i++)); do
      if [ "${args[i]}" = "$k" ]; then
        if [ "$v" = "-" ]; then unset 'args[i]' 'args[i+1]'; else args[i+1]="$v"; fi
        args=("${args[@]}"); break
      fi
    done
    case "$kv" in lumen-bin=*) args+=("$k" "$v") ;; esac
  done
  OUTJ="$T/$name.json"; rm -f "$OUTJ"
  "$IH" --out "$OUTJ" "${args[@]}" >"$T/$name.stdout" 2>"$T/$name.stderr"; RC=$?
}
proof_verdict() {  # proof_verdict <json> <id>  -> PASS|FAIL|SKIP|MISSING over both indexes' proofs
  jq -r --arg id "$2" '([.codegraph.proofs[]?, .lumen.proofs[]?, .parity?] | map(select(.id? == $id)) | .[0].verdict) // "MISSING"' "$1" 2>/dev/null || echo "MISSING"
}
expect_fail() {  # expect_fail <case name> <proof id> : RC==1, JSON verdict FAIL, that proof FAIL
  local name="$1" id="$2" pv tv
  pv="$(proof_verdict "$OUTJ" "$id")"; tv="$(jq -r '.verdict // "MISSING"' "$OUTJ" 2>/dev/null || echo MISSING)"
  if [ "$RC" -eq 1 ] && [ "$tv" = FAIL ] && [ "$pv" = FAIL ]; then ok "$name (rc=1, $id=FAIL, verdict=FAIL)"
  else bad "$name: want rc=1 verdict=FAIL $id=FAIL, got rc=$RC verdict=$tv $id=$pv"; fi
}

[ -x "$IH" ] || echo "NOTE: $IH is absent or not executable (RED state: every case below must FAIL)"

# 1 golden-good: PASS
run good
tv="$(jq -r '.verdict // "MISSING"' "$OUTJ" 2>/dev/null || echo MISSING)"
bad_proofs="$(jq -r '[.codegraph.proofs[]?, .lumen.proofs[]?] | map(select(.verdict=="FAIL") | .id) | join(",")' "$OUTJ" 2>/dev/null || echo "?")"
if [ "$RC" -eq 0 ] && [ "$tv" = PASS ] && [ -z "$bad_proofs" ]; then ok "all-good fixture PASS (rc=0, verdict=PASS, no FAIL proof)"
else bad "all-good fixture: want rc=0 verdict=PASS, got rc=$RC verdict=$tv failing=[$bad_proofs]"; fi
# schema marker and per-proof evidence fields (the gate must record command, raw sha256 and needles)
sch="$(jq -r '.schema // "MISSING"' "$OUTJ" 2>/dev/null || echo MISSING)"
[ "$sch" = "index-health/1" ] && ok "output carries schema index-health/1" || bad "schema: want index-health/1, got $sch"
ev="$(jq -r '[.codegraph.proofs[]? | select(.id=="P2") | (.needle_pos != null and .needle_neg != null)] | all' "$OUTJ" 2>/dev/null || echo false)"
[ "$ev" = true ] && ok "P2 row records both control needles" || bad "P2 row must record needle_pos and needle_neg"

# 2 pendingRefs > 0 -> P1 FAIL
run pending cg-status="$T/cg_status_pending.json";  expect_fail "pendingRefs>0" P1
# 3 required field absent -> P1 FAIL (absence is FAIL, never read as zero)
run nofield cg-status="$T/cg_status_nofield.json";  expect_fail "required field index.pendingRefs absent" P1
# 4 worktreeMismatch non-null -> P1 FAIL
run wt cg-status="$T/cg_status_wt.json";            expect_fail "worktreeMismatch non-null" P1
# 5 negative needle found in the index -> P2 FAIL (the instrument cannot be trusted)
run negfound cg-files="$T/cg_files_negfound.json";  expect_fail "negative needle zz/does_not_exist.go found" P2
# 6 positive needle missing -> P2 FAIL
run posmissing cg-files="$T/cg_files_posmissing.json"; expect_fail "positive needle missing from the index" P2
# 7 Lumen status Stale: yes -> P7 FAIL
run stale lumen-status="$T/lumen_stale.txt";         expect_fail "Lumen status Stale: yes" P7
# 8 no Lumen status file and no probe result (no --lumen-status, no --lumen-bin) -> P-Lumen FAIL
run nostatus lumen-status=-;                         expect_fail "no Lumen status file and no probe" P-Lumen
# 9 --lumen-bin path that does not exist (and no status file) -> P-Lumen FAIL
run nobin lumen-status=- lumen-bin="$T/no/such/lumen-bin"; expect_fail "--lumen-bin path does not exist" P-Lumen
# 10 scope parity outside tolerance -> P8 FAIL
run parity lumen-files="$T/lumen_files_parity.txt";  expect_fail "scope parity outside tolerance" P8
# 11 usage error: no --out -> exit 2 (a gate that cannot say where to write must not guess)
"$IH" "${BASE[@]}" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 2 ] && ok "missing --out is a usage error (rc=2)" || bad "missing --out: want rc=2, got rc=$rc"

echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
