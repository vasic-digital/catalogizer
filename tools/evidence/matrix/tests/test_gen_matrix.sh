#!/usr/bin/env bash
# test_gen_matrix.sh - T195 (RED first) / T197 (GREEN). Oracle for tools/evidence/matrix/gen_matrix.py (docs/05 13.1 - 13.3).
# Independent oracles: (1) hand-derived golden counts (this file's expectations are written from the fixture rows with tr/grep, never from the
# generator); (2) the 4.4 table of docs/05 tallied directly from the document text and compared with the generator fed the same table;
# (3) a paired mutation: copies of the generator with ONE guard removed must make the matching leg FAIL.
# Run through `scripts/test-in-container.sh tooling unit -- bash tools/evidence/matrix/tests/test_gen_matrix.sh` (IMG-TESTUTIL has python3 + PyYAML).
# Usage: test_gen_matrix.sh [--no-mutations]    Env: GEN (generator under test), MUTATION_RECORD (file receiving one line per mutation)
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../../.." && pwd)"
GEN="${GEN:-$HERE/../gen_matrix.py}"
PASSES=0; FAILS=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
T="$(mktemp -d "${TMPDIR:-/tmp}/genmatrix-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -f "$GEN" ] || { bad "generator not found at $GEN"; echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"; exit 1; }
TS="2026-10-06T00:00:00Z"
TYPES="unit integration e2e full_automation security ddos scaling chaos stress performance benchmarking ui ux challenges helixqa"

# mkmap FILE name:'15 states' ... : write an applicability file from compact rows (test-side helper, NOT the generator)
mkmap() {
  local f="$1"; shift
  { echo "schema: applicability/1"; echo "types: [${TYPES// /, }]"; echo "components:"
    for spec in "$@"; do
      local name="${spec%%:*}" row="${spec#*:}" i=0
      echo "  $name:"; echo "    group: test"; echo "    name: $name"; echo "    path: x/"; echo "    cells:"
      for t in $TYPES; do
        i=$((i+1)); local s; s="$(echo "$row" | awk -v n=$i '{print $n}')"
        case "$s" in na) echo "      $t: {state: \"n/a\", reason: \"fixture reason for $t\"}";; *) echo "      $t: {state: \"$s\", reason: \"fixture reading for $t\"}";; esac
      done
    done
    echo "cross_cutting: {}"; } >"$f"
}
tally() { # tally 'row'... -> "P ~ A na ?"
  printf '%s\n' "$@" | tr ' ' '\n' | awk '{c[$1]++} END{printf "%d %d %d %d %d", c["P"], c["~"], c["A"], c["na"], c["?"]}'
}
declared() { python3 -I - "$1" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))["counts"]["declared"]
print(d["P"], d["~"], d["A"], d["n/a"])
PY
}

R1="P ~ A A ~ na na ~ P A ~ na na A ~"
R2="P P ~ A A ~ na A na A A A A ~ ~"
mkmap "$T/good.yaml" "alpha:$R1" "beta:$R2"
read -r eP eT eA eN e0 <<<"$(tally "$R1" "$R2")"
OUT="$T/out-good"; mkdir -p "$OUT"
python3 -I "$GEN" --applicability "$T/good.yaml" --out "$OUT" --timestamp "$TS" >"$T/good.stdout" 2>"$T/good.stderr"; rc=$?
check "golden-good: exit 0" "$rc" 0
check "golden-good: the declared counts equal the hand tally of the rows (P ~ A n/a)" "$(declared "$OUT/coverage-matrix.json")" "$eP $eT $eA $eN"
check "golden-good: two components x 15 types = 30 cells" "$(python3 -I -c "import json;print(json.load(open('$OUT/coverage-matrix.json'))['counts']['cells'])")" 30
check "golden-good: no cell is '?'" "$(grep -c '"?"' "$OUT/coverage-matrix.json")" 0
tail -c1 "$OUT/coverage-matrix.md" | od -An -c | grep -q '\\n' && ok "golden-good: the md ends with one newline" || bad "golden-good: the md has no final newline"
if grep -nE '[[:space:]]+$' "$OUT/coverage-matrix.md" >/dev/null; then bad "golden-good: the md has trailing whitespace"; else ok "golden-good: the md has no trailing whitespace"; fi
grep -q '^| Revision |' "$OUT/coverage-matrix.md" && grep -q '^| Last modified |' "$OUT/coverage-matrix.md" && ok "golden-good: the md carries the 11.4.44 revision header" || bad "golden-good: revision header missing"
# determinism: a second run over the same input is byte-identical
OUT2="$T/out-good2"; mkdir -p "$OUT2"; python3 -I "$GEN" --applicability "$T/good.yaml" --out "$OUT2" --timestamp "$TS" >/dev/null 2>&1
check "golden-good: two runs are byte-identical (md)" "$(sha256sum <"$OUT/coverage-matrix.md")" "$(sha256sum <"$OUT2/coverage-matrix.md")"
check "golden-good: two runs are byte-identical (json)" "$(sha256sum <"$OUT/coverage-matrix.json")" "$(sha256sum <"$OUT2/coverage-matrix.json")"

# one '?' cell: exit non-zero naming the cell, and NO output written
R3="P ~ A A ~ na na ~ ? A ~ na na A ~"
mkmap "$T/q1.yaml" "alpha:$R3" "beta:$R2"
OUTQ="$T/out-q"; mkdir -p "$OUTQ"
python3 -I "$GEN" --applicability "$T/q1.yaml" --out "$OUTQ" --timestamp "$TS" >"$T/q.stdout" 2>"$T/q.stderr"; rc=$?
[ "$rc" != 0 ] && ok "one '?' cell: the generator exits non-zero (got $rc)" || bad "one '?' cell: the generator exited 0"
grep -q "could not classify" "$T/q.stderr" && ok "one '?' cell: the refusal says the cell could not be classified" || bad "one '?' cell: the refusal does not name the '?' rule: $(cat "$T/q.stderr")"
grep -q 'alpha' "$T/q.stderr" && grep -q 'stress' "$T/q.stderr" && ok "one '?' cell: stderr names the component and the type" || bad "one '?' cell: stderr does not name alpha/stress: $(cat "$T/q.stderr")"
[ -z "$(ls "$OUTQ")" ] && ok "one '?' cell: no matrix file was written" || bad "one '?' cell: output exists: $(ls "$OUTQ")"

# the fifteen types in the contract order: a reordered list is refused; a P/~/A cell without the reading it came from is refused
sed '0,/^types: \[unit, integration,/s//types: [integration, unit,/' "$T/good.yaml" >"$T/types.yaml"
python3 -I "$GEN" --applicability "$T/types.yaml" --out "$T/o1t" --timestamp "$TS" >/dev/null 2>"$T/e1t"; rc=$?
[ "$rc" = 3 ] && grep -q 'types must be exactly' "$T/e1t" && ok "a reordered types list is refused (3): the map cannot redefine the fifteen types" || bad "reordered types accepted (rc $rc): $(cat "$T/e1t")"
sed '0,/unit: {state: "P", reason: "fixture reading for unit"}/s//unit: {state: "P", reason: ""}/' "$T/good.yaml" >"$T/noreadp.yaml"
grep -q 'state: "P", reason: ""' "$T/noreadp.yaml" || sed '0,/unit: {state: "\(.*\)", reason: "fixture reading for unit"}/s//unit: {state: "\1", reason: ""}/' "$T/good.yaml" >"$T/noreadp.yaml"
python3 -I "$GEN" --applicability "$T/noreadp.yaml" --out "$T/o2t" --timestamp "$TS" >/dev/null 2>"$T/e2t"; rc=$?
[ "$rc" = 3 ] && grep -q 'needs the reading it came from' "$T/e2t" && ok "a non-n/a cell without a reason is refused (3)" || bad "reasonless cell accepted (rc $rc): $(cat "$T/e2t")"
# n/a without a reason, a missing type, an unknown state, a wrong schema, a duplicate type list are each refused
sed 's/stress: {state: "n\/a", reason: "fixture reason for stress"}/stress: {state: "n\/a", reason: ""}/; s/ddos: {state: "n\/a", reason: "fixture reason for ddos"}/ddos: {state: "n\/a", reason: ""}/' "$T/good.yaml" >"$T/noreason.yaml"
python3 -I "$GEN" --applicability "$T/noreason.yaml" --out "$T/o1" --timestamp "$TS" >/dev/null 2>"$T/e1"; rc=$?
[ "$rc" != 0 ] && grep -q 'without a reason' "$T/e1" && ok "an n/a cell without a reason is refused (rc $rc)" || bad "n/a without reason accepted (rc $rc): $(cat "$T/e1")"
grep -v '^      ux:' "$T/good.yaml" >"$T/missing.yaml"
python3 -I "$GEN" --applicability "$T/missing.yaml" --out "$T/o2" --timestamp "$TS" >/dev/null 2>"$T/e2"; rc=$?
[ "$rc" != 0 ] && grep -q 'ux: missing type' "$T/e2" && ok "a component missing a type is refused naming it (rc $rc)" || bad "missing type accepted (rc $rc)"
sed 's/ddos: {state: "n\/a"/ddos: {state: "maybe"/' "$T/good.yaml" >"$T/unk.yaml"
python3 -I "$GEN" --applicability "$T/unk.yaml" --out "$T/o3" --timestamp "$TS" >/dev/null 2>"$T/e3"; rc=$?
[ "$rc" != 0 ] && ok "an unknown state is refused (rc $rc)" || bad "unknown state accepted"
sed 's/^schema: applicability\/1/schema: applicability\/9/' "$T/good.yaml" >"$T/schema.yaml"
python3 -I "$GEN" --applicability "$T/schema.yaml" --out "$T/o4" --timestamp "$TS" >/dev/null 2>&1; rc=$?
[ "$rc" != 0 ] && ok "a wrong schema is refused (rc $rc)" || bad "wrong schema accepted"
python3 -I "$GEN" --applicability "$T/does-not-exist.yaml" --out "$T/o5" --timestamp "$TS" >/dev/null 2>&1; rc=$?
[ "$rc" != 0 ] && ok "a missing input is refused (rc $rc)" || bad "missing input accepted"

# a hand-edited count is overwritten on regeneration
sed -i 's/"P": [0-9]*/"P": 999/' "$OUT/coverage-matrix.json"
python3 -I "$GEN" --applicability "$T/good.yaml" --out "$OUT" --timestamp "$TS" >/dev/null 2>&1
check "a hand-edited count is overwritten on regeneration" "$(declared "$OUT/coverage-matrix.json")" "$eP $eT $eA $eN"
printf '\nhand-edited footer\n' >>"$OUT/coverage-matrix.md"
python3 -I "$GEN" --applicability "$T/good.yaml" --out "$OUT" --timestamp "$TS" >/dev/null 2>&1
grep -q 'hand-edited footer' "$OUT/coverage-matrix.md" && bad "a hand edit of the md survived regeneration" || ok "a hand edit of the md does not survive regeneration"

# the doc05 4.4 table: tallied straight from the document text, then the generator is fed the same table
python3 -I - "$REPO/specs/001-full-project-audit-remediation/docs/05-test-strategy-and-coverage-matrix.md" "$T/doc44.yaml" "$T/doc44.tally" <<'PY'
import re,sys,collections
txt=open(sys.argv[1],encoding="utf-8").read()
m=re.search(r"### 4\.4 The matrix.*?\n(\| Type .*?)\n\n",txt,re.S)
rows=[r for r in m.group(1).split("\n") if r.startswith("|")]
hdr=[c.strip() for c in rows[0].strip("|").split("|")]
cols=[re.match(r"(A\d+)",h).group(1) for h in hdr[1:]]
types=["unit","integration","e2e","full_automation","security","ddos","scaling","chaos","stress","performance","benchmarking","ui","ux","challenges","helixqa"]
cnt=collections.Counter(); cell={}
for r in rows[2:]:
    cs=[c.strip() for c in r.strip("|").split("|")]
    n=int(cs[0].split()[0])
    for col,v in zip(cols,cs[1:]):
        v=v.split()[0]; cnt[v]+=1; cell[(col,types[n-1])]=v
out=["schema: applicability/1","types: ["+", ".join(types)+"]","components:"]
for col in cols:
    out+=["  %s:"%col,"    group: doc","    name: %s"%col,"    path: x/","    cells:"]
    for t in types:
        v=cell[(col,t)]
        out.append('      %s: {state: "%s", reason: "doc05 4.4"}'%(t,v))
out.append("cross_cutting: {}")
open(sys.argv[2],"w").write("\n".join(out)+"\n")
open(sys.argv[3],"w").write("%d %d %d %d %d\n"%(cnt["P"],cnt["~"],cnt["A"],cnt["n/a"],cnt["?"]))
PY
read -r dP dT dA dN dQ <"$T/doc44.tally"
[ "$((dP+dT+dA+dN+dQ))" = 165 ] && ok "the doc05 4.4 table has 11 columns x 15 rows = 165 cells (P=$dP ~=$dT A=$dA n/a=$dN ?=$dQ)" || bad "doc05 4.4 tally does not add up to 165: $dP $dT $dA $dN $dQ"
python3 -I "$GEN" --applicability "$T/doc44.yaml" --out "$T/o6" --timestamp "$TS" >/dev/null 2>"$T/e6"; rc=$?
if [ "$dQ" -gt 0 ]; then
  [ "$rc" != 0 ] && ok "the doc05 4.4 table as written (it still holds $dQ '?' cells) is refused by the generator" || bad "the doc table with '?' cells was accepted"
  n="$(grep -c '?' "$T/e6")"; [ "$n" -ge 1 ] && ok "the refusal lists the '?' cells" || bad "no '?' cell listed"
  # replace each '?' with `A` (the applicable-and-unverified reading the TS-00 rule gives an unknown) to compare the counts
  sed 's/{state: "?"/{state: "A"/' "$T/doc44.yaml" >"$T/doc44a.yaml"
  python3 -I "$GEN" --applicability "$T/doc44a.yaml" --out "$T/o7" --timestamp "$TS" >/dev/null 2>&1
  check "doc05 4.4 with '?' resolved to A: declared counts equal the document tally" "$(declared "$T/o7/coverage-matrix.json")" "$dP $dT $((dA+dQ)) $dN"
else
  check "doc05 4.4: declared counts equal the document tally" "$(declared "$T/o6/coverage-matrix.json")" "$dP $dT $dA $dN"
fi

# --gate (docs/05 13.3) over a REAL evrec ledger (K10): the ledger is written by tools/evidence/evrec, the status of a cell is DERIVED by tools/evidence/evverdict
EVT="$REPO/tools/evidence"; OR="--oracle invariant --oracle-independent --test-source t.sh"
evsetup() { # evsetup NAME : a fresh hermetic ledger directory and the env evrec reads
  EVW="$T/ev-$1"; rm -rf "$EVW"; mkdir -p "$EVW/repo/.audit" "$EVW/home" "$EVW/cwd"; printf 'grep -q ok target.txt\n' >"$EVW/cwd/t.sh"; : >"$EVW/evrec.log"
  export EV_LEDGER="$EVW/ledger.jsonl" EV_ANCHOR="$EVW/anchor.jsonl" EV_BLOBS="$EVW/blobs" EVREC_REPO_ROOT="$EVW/repo" HOME="$EVW/home" CPA_HOST_ENTRY=/bin/false
}
evr() { ( cd "$EVW/cwd" && "$EVT/evrec" run "$@" ) >>"$EVW/evrec.log" 2>&1; }
mkitem() { # mkitem ITEM CLASS [greens=3] [mutation=yes] : RED (target bad) , GREENs (target ok), one caught MUTATION (target bad again)
  local item="$1" cls="$2" g="${3:-3}" mu="${4:-yes}" i
  echo bad >"$EVW/cwd/target.txt"; evr "$item" RED 1 go_binary target.txt $OR --evidence-class "$cls" -- bash t.sh
  echo ok >"$EVW/cwd/target.txt"
  for i in $(seq 1 "$g"); do evr "$item" GREEN "$i" go_binary target.txt $OR --evidence-class "$cls" -- bash t.sh; done
  if [ "$mu" = yes ]; then echo bad >"$EVW/cwd/target.txt"; evr "$item" MUTATION 1 go_binary target.txt $OR --evidence-class "$cls" --mutation-json '{"operator":"x","location":"y","author":"reviewer","result":"caught"}' -- bash t.sh; echo ok >"$EVW/cwd/target.txt"; fi
}
cellitems() { printf '%s\n' "$1" >"$EVW/ci.json"; }
rungate() { # rungate NAME MAP [extra gen args] : GR = exit status, stderr in $T/NAME.err
  python3 -I "$GEN" --applicability "$2" --ledger "$EV_LEDGER" --cell-items "$EVW/ci.json" --out "$T/$1" --timestamp "$TS" --gate --map-unbound "${@:3}" >"$T/$1.out" 2>"$T/$1.err"; GR=$?
}
cellstatus() { python3 -I -c "import json,sys;d=json.load(open(sys.argv[1]));print(','.join(sorted(c['status'] for c in d['cells'] if c['declared']!='n/a')))" "$1/coverage-matrix.json"; }
mkmap "$T/g.yaml" "alpha:P na na na na na na na na na na na na na na"
evsetup ok; mkitem CAT-001 source; cellitems '{"alpha|unit":"CAT-001"}'
check "K10 fixture: the real recorder wrote RED + 3 GREEN + MUTATION (5 chained entries)" "$(wc -l <"$EV_LEDGER" | tr -d ' ')" 5
check "K10 fixture: the verdict deriver says PASS for the item (the ledger is the real one)" "$("$EVT/verdict" CAT-001 --ledger "$EV_LEDGER" --blobs "$EV_BLOBS" 2>/dev/null | python3 -I -c "import json,sys;print(json.load(sys.stdin)['verdict'])")" PASS
rungate g1 "$T/g.yaml"
check "gate golden-false: a complete REAL ledger passes (exit 0)" "$GR" 0
check "gate golden-false: the cell is present in the json" "$(cellstatus "$T/g1")" present
# a ledger with entries for ANOTHER item only: the cell is absent and the gate fails naming it
evsetup other; mkitem CAT-002 source; cellitems '{"alpha|unit":"CAT-001"}'
rungate g2 "$T/g.yaml"
[ "$GR" = 1 ] && grep -q 'alpha / unit' "$T/g2.err" && grep -q 'no ledger entry for CAT-001' "$T/g2.err" && ok "gate: a cell whose item has no ledger entry is absent and the gate fails naming it (rc 1)" || bad "gate did not fail on the absent item (rc $GR): $(cat "$T/g2.err")"
# forged: a deleted line breaks the chain: the whole ledger is refused (exit 3), no cell is judged
evsetup forge; mkitem CAT-001 source; cellitems '{"alpha|unit":"CAT-001"}'; sed -i '2d' "$EV_LEDGER"
rungate g3 "$T/g.yaml"
[ "$GR" = 3 ] && grep -q 'ledger_chain_invalid' "$T/g3.err" && ok "gate RM-A: a ledger with a deleted line is refused (3 ledger_chain_invalid), never judged" || bad "gate accepted a forged ledger (rc $GR): $(cat "$T/g3.err")"
evsetup forge2; mkitem CAT-001 source; cellitems '{"alpha|unit":"CAT-001"}'; python3 -I - "$EV_LEDGER" <<'PY'
import sys
L=open(sys.argv[1]).read().split("\n"); L[1],L[2]=L[2],L[1]; open(sys.argv[1],"w").write("\n".join(L))
PY
rungate g3b "$T/g.yaml"
[ "$GR" = 3 ] && ok "gate RM-A: two reordered lines are refused (rc 3)" || bad "gate accepted a reordered ledger (rc $GR)"
# two GREENs are not three: the DERIVED verdict is not PASS
evsetup two; mkitem CAT-001 source 2; cellitems '{"alpha|unit":"CAT-001"}'
rungate g4 "$T/g.yaml"
[ "$GR" = 1 ] && grep -q 'verdict derived from the ledger is FAIL' "$T/g4.err" && ok "gate: two GREEN entries (not three) leave the cell partial: the derived verdict is not PASS (rc 1)" || bad "gate accepted two GREEN entries (rc $GR): $(cat "$T/g4.err")"
# no caught mutation
evsetup nomut; mkitem CAT-001 source 3 no; cellitems '{"alpha|unit":"CAT-001"}'
rungate g5 "$T/g.yaml"
[ "$GR" = 1 ] && grep -q 'no caught mutation' "$T/g5.err" && ok "gate: no caught MUTATION entry is not present (rc 1)" || bad "gate accepted a record without a caught mutation (rc $GR): $(cat "$T/g5.err")"
# a MUTATION entry that did not get caught (the test still passed on the mutated target) is not a caught mutation
evsetup surv; cellitems '{"alpha|unit":"CAT-001"}'; mkitem CAT-001 source 3 no
echo ok >"$EVW/cwd/target.txt"; evr CAT-001 MUTATION 1 go_binary target.txt $OR --evidence-class source --mutation-json '{"operator":"x","location":"y","author":"reviewer","result":"survived"}' -- bash t.sh
rungate sv "$T/g.yaml"
[ "$GR" = 1 ] && grep -q 'no caught mutation' "$T/sv.err" && ok "gate: a MUTATION entry whose result is survived is not a caught mutation (rc 1)" || bad "gate accepted a survived mutation (rc $GR): $(cat "$T/sv.err") / $(tail -2 "$EVW/evrec.log")"
# RM1: an evidence class BELOW the type's need is not present (e2e needs runtime, ui needs user_visible); the exact and a stronger class are present
mkmap "$T/cls.yaml" "alpha:P na P na na na na na na na na P na na na"
evsetup low; mkitem CAT-001 source; mkitem CAT-002 source; mkitem CAT-003 runtime
cellitems '{"alpha|unit":"CAT-001","alpha|e2e":"CAT-002","alpha|ui":"CAT-003"}'
rungate c1 "$T/cls.yaml"
check "RM1: source evidence for e2e and runtime evidence for ui are below the required class: the gate fails" "$GR" 1
grep -q 'evidence class source is below runtime' "$T/c1.err" && grep -q 'evidence class runtime is below user_visible' "$T/c1.err" && ok "RM1: both cells are named with their class gap" || bad "RM1: class gap not named: $(cat "$T/c1.err")"
check "RM1: e2e and ui are partial, unit present, in the json" "$(cellstatus "$T/c1")" "partial,partial,present"
evsetup high; mkitem CAT-001 user_visible; mkitem CAT-002 runtime; mkitem CAT-003 user_visible
cellitems '{"alpha|unit":"CAT-001","alpha|e2e":"CAT-002","alpha|ui":"CAT-003"}'
rungate c2 "$T/cls.yaml"
check "RM1 golden-false: runtime for e2e, user_visible for ui and a STRONGER class for unit pass the gate" "$GR" 0
# RM2: a PASS item REOPENed by a failing run of the same test is no longer present (the ledger is judged by its LAST cycle)
evsetup reopen; mkitem CAT-001 source; cellitems '{"alpha|unit":"CAT-001"}'; echo bad >"$EVW/cwd/target.txt"; evr CAT-001 REOPEN 1 go_binary target.txt $OR --evidence-class source -- bash t.sh
rungate r1 "$T/g.yaml"
check "RM2: a cutting REOPEN removes present: the gate fails" "$GR" 1
check "RM2: the cell is partial in the json" "$(cellstatus "$T/r1")" partial
# RM3: a latest entry that says blocked yields the status `blocked` (not partial), and the gate still fails
evsetup blk; cellitems '{"alpha|unit":"CAT-001"}'; echo bad >"$EVW/cwd/target.txt"; evr CAT-001 RED 1 go_binary target.txt $OR --evidence-class source -- bash t.sh
python3 -I - "$EVT" <<'PY' >>"$EVW/evrec.log" 2>&1
import sys, json, os
sys.path.insert(0, sys.argv[1]); import evcore
led = [json.loads(l) for l in open(os.environ["EV_LEDGER"])]
body = {k: v for k, v in led[-1].items() if k not in ("seq", "prev_hash", "entry_hash")}
body.update(polarity="RED", iteration=2, verdict="blocked", exit_status=7, blocked_reason="service_unreachable", blocked_detail="fixture: registry host not reachable", counts_as="not_passing")
for k in ("stdout_sha256", "stderr_sha256", "stdout_digest", "stderr_digest", "mutation"):
    pass
print(evcore.append_entry(body)["seq"])
PY
rungate b1 "$T/g.yaml"
check "RM3: a blocked latest entry still fails the gate (blocked is not present)" "$GR" 1
check "RM3: the cell status is blocked in the json" "$(cellstatus "$T/b1")" blocked
grep -q 'the latest entry says blocked' "$T/b1.err" && ok "RM3: the gate output says blocked" || bad "RM3: blocked reason missing: $(cat "$T/b1.err") / $(tail -3 "$EVW/evrec.log")"
# candidate fingerprint: the GREEN entries must be for THIS build
evsetup cand; mkitem CAT-001 source; cellitems '{"alpha|unit":"CAT-001"}'
FP="$(python3 -I -c "import json,os;print([json.loads(l) for l in open(os.environ['EV_LEDGER']) if json.loads(l)['polarity']=='GREEN'][0]['target_fingerprint'])")"
rungate f1 "$T/g.yaml" --candidate-fingerprint "$FP"; check "candidate: GREEN entries for exactly this fingerprint pass (golden-false)" "$GR" 0
rungate f2 "$T/g.yaml" --candidate-fingerprint "$(printf '%064d' 5)"; check "candidate: a record for another build is not evidence for this one (rc 1)" "$GR" 1
grep -q 'another artifact than the candidate' "$T/f2.err" && ok "candidate: the refusal names the candidate rule" || bad "candidate: reason missing: $(cat "$T/f2.err")"
# input guards
python3 -I "$GEN" --applicability "$T/g.yaml" --out "$T/og6" --timestamp "$TS" --gate --map-unbound >"$T/g6.out" 2>"$T/g6.err"; rc=$?
check "gate B1: --gate WITHOUT --ledger is a usage refusal (exit 2), never a vacuous pass" "$rc" 2
grep -q 'requires --ledger' "$T/g6.err" && ok "gate B1: the refusal names the missing ledger" || bad "gate B1: refusal text missing: $(cat "$T/g6.err")"
[ ! -e "$T/og6/coverage-matrix.json" ] && ok "gate B1: a refused gate wrote no matrix" || bad "gate B1: output written despite the refusal"
python3 -I "$GEN" --applicability "$T/g.yaml" --out "$T/og6b" --timestamp "$TS" >/dev/null 2>&1; check "gate B1 control: the same map without --gate and without a ledger still renders (exit 0)" "$?" 0
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$EV_LEDGER" --cell-items "$EVW/ci.json" --out "$T/og9" --timestamp "$TS" --gate >/dev/null 2>"$T/g9.err"; rc=$?
[ "$rc" = 2 ] && grep -q 'map-unbound' "$T/g9.err" && ok "K11.1: --gate without --repo or --map-unbound is refused (2): the map is not tied to the repository" || bad "K11.1: unbound gate accepted (rc $rc): $(cat "$T/g9.err")"
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$EV_LEDGER" --cell-items "$EVW/ci.json" --out "$T/og10" --timestamp "$TS" --gate --repo "$REPO" >/dev/null 2>"$T/g10.err"; rc=$?
[ "$rc" = 3 ] && grep -q 'map_not_bound' "$T/g10.err" && ok "K11.1: a one-component map that is not what the repository derives is refused with --repo (3 map_not_bound)" || bad "K11.1: unbound map accepted with --repo (rc $rc): $(head -c 300 "$T/g10.err")"
cellitems '{"alpah|unit":"CAT-001"}'; rungate g7 "$T/g.yaml"
[ "$GR" = 3 ] && grep -q 'cell_items_unmatched' "$T/g7.err" && ok "gate: a --cell-items key naming no component of the map is refused (3 cell_items_unmatched)" || bad "gate: stray cell-items key accepted (rc $GR)"
cellitems '{"alpha|unit":"not an item"}'; rungate g7b "$T/g.yaml"
[ "$GR" = 3 ] && grep -q 'cell_items_invalid' "$T/g7b.err" && ok "gate: a --cell-items value that is no register item id is refused (3 cell_items_invalid)" || bad "gate: invalid item id accepted (rc $GR)"
cellitems '{"alpha|unit":"CAT-001"}'
mkmap "$T/allna.yaml" "alpha:na na na na na na na na na na na na na na na"
rungate g8 "$T/allna.yaml"
[ "$GR" = 3 ] && grep -q 'gate_vacuous' "$T/g8.err" && ok "gate: a map with no applicable cell is refused (3 gate_vacuous)" || bad "gate: vacuous map accepted (rc $GR)"
python3 -I "$GEN" --applicability "$T/g.yaml" --out "$T/ots" --timestamp "2026-10-06 00:00:00" >/dev/null 2>&1; check "K11.2: a timestamp with a space is refused (2): it would corrupt the 11.4.44 header" "$?" 2
cp "$T/g.yaml" "$T/dup.yaml"; sed -i '0,/^  alpha:/s//  alpha:\n    group: x/' "$T/dup.yaml"; python3 -I "$GEN" --applicability "$T/dup.yaml" --out "$T/odup" --timestamp "$TS" >/dev/null 2>"$T/dup.err"; rc=$?
[ "$rc" = 3 ] && grep -q 'duplicate key' "$T/dup.err" && ok "K11.3: a duplicate YAML key is refused (3), never silently the last one" || bad "K11.3: duplicate key accepted (rc $rc): $(cat "$T/dup.err")"
printf 'schema: applicability/1\ntypes: [unit]\ncomponents: [1, 2\n' >"$T/broken.yaml"; python3 -I "$GEN" --applicability "$T/broken.yaml" --out "$T/obr" --timestamp "$TS" >/dev/null 2>&1; rc=$?
[ "$rc" = 3 ] && ok "K11.1: an unparsable map is exit 3 (never exit 1, which means the gate failed)" || bad "K11.1: unparsable map gave rc $rc"
printf '[]\n' >"$T/ci-bad.json"; evsetup ci; mkitem CAT-001 source; python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$EV_LEDGER" --cell-items "$T/ci-bad.json" --out "$T/ocb" --timestamp "$TS" --gate --map-unbound >/dev/null 2>&1; rc=$?
[ "$rc" = 3 ] && ok "K11.1: a --cell-items file that is not a JSON object is exit 3" || bad "K11.1: bad cell-items gave rc $rc"
# the inputs are recorded in the json (K11.4): a reader can tell which ledger and map a figure came from
check "inputs: the json records the sha256 of the applicability map and the ledger and map_bound=false" "$(python3 -I -c "import json,sys;i=json.load(open(sys.argv[1]))['inputs'];print(len(i['applicability_sha256']),len(i['ledger_sha256']),i['map_bound'])" "$T/g1/coverage-matrix.json")" "64 64 False"

# minting (spec US3 acceptance 4, docs/05 13.3): one absent and one partial cell yield exactly two items, a second run none; the register enforces the idempotency key
mkmap "$T/m.yaml" "alpha:A ~ P na na na na na na na na na na na na"
cat >"$T/mint.sh" <<'M'
#!/usr/bin/env bash
# fixture mint command: `CMD component type state key` ; the register contract is "same key -> same item"
n=$(wc -l <"$MINT_LOG" 2>/dev/null || echo 0); echo "$1 $2 $3 $4" >>"$MINT_LOG"
k="$(grep -F -- "$4" "$MINT_LOG" | wc -l)"; case "$1/$2" in alpha/unit) echo CAT-901;; alpha/integration) echo CAT-902;; *) echo CAT-9$((n+10));; esac
M
chmod +x "$T/mint.sh"; : >"$T/mint.log"; export MINT_LOG="$T/mint.log"
OUTM="$T/out-m"; mkdir -p "$OUTM"; MLED="$T/mint-ledger-1.json"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTM" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$MLED" >/dev/null 2>"$T/m1.err"; rc=$?
check "mint: first run exits 0" "$rc" 0
check "mint: one absent and one partial cell yield exactly two items" "$(wc -l <"$T/mint.log" | tr -d ' ')" 2
grep -q '^alpha unit A mint:alpha|unit$' "$T/mint.log" && grep -q '^alpha integration ~ mint:alpha|integration$' "$T/mint.log" && ok "mint: the calls name the cells and carry the idempotency key (alpha unit absent, alpha integration partial)" || bad "mint calls wrong: $(cat "$T/mint.log")"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTM" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$MLED" >/dev/null 2>&1
check "mint: a second run mints none (11.4.214)" "$(wc -l <"$T/mint.log" | tr -d ' ')" 2
python3 -I - "$MLED" <<'PY' && ok "mint: the ledger links both cells to their item, state minted" || bad "mint ledger malformed"
import json,sys
d=json.load(open(sys.argv[1])); assert len(d["items"])==2, d
assert all(v["state"]=="minted" and v["item"].startswith("CAT-9") for v in d["items"].values())
PY
# the default ledger sits NEXT TO THE MAP, so a different --out does not reset it (K11.4)
mkdir -p "$T/mapdir"; cp "$T/m.yaml" "$T/mapdir/applicability.yaml"; : >"$T/mint.log"
python3 -I "$GEN" --applicability "$T/mapdir/applicability.yaml" --out "$T/o-a" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" >/dev/null 2>&1
python3 -I "$GEN" --applicability "$T/mapdir/applicability.yaml" --out "$T/o-b" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" >/dev/null 2>&1
check "mint K11.4: two runs with DIFFERENT --out mint 2 items in total (the default ledger is next to the map)" "$(wc -l <"$T/mint.log" | tr -d ' ')" 2
[ -s "$T/mapdir/mint-ledger.json" ] && [ ! -e "$T/o-a/mint-ledger.json" ] && ok "mint K11.4: the ledger is beside the map, not under --out" || bad "mint K11.4: ledger location wrong"
# concurrent runs: the exclusive lock held for the whole run makes the second read the first's ledger
: >"$T/mint.log"; rm -f "$T/mint-ledger-c.json"
for i in 1 2 3; do python3 -I "$GEN" --applicability "$T/m.yaml" --out "$T/o-c$i" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$T/mint-ledger-c.json" >/dev/null 2>&1 & done; wait
check "mint: three CONCURRENT runs mint 2 items in total (lock held for the whole run)" "$(wc -l <"$T/mint.log" | tr -d ' ')" 2
# crash: the first cell is minted, the second mint command fails -> a pending row; the retry reuses the SAME key and does not mint the first again
cat >"$T/mintcrash.sh" <<'M'
#!/usr/bin/env bash
echo "$1 $2 $3 $4" >>"$MINT_LOG"; [ "$1/$2" = alpha/unit ] && { echo CAT-901; exit 0; }; echo "boom" >&2; exit 9
M
chmod +x "$T/mintcrash.sh"; : >"$T/mint.log"; rm -f "$T/mint-ledger-x.json"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$T/o-x" --timestamp "$TS" --mint --mint-cmd "$T/mintcrash.sh" --mint-ledger "$T/mint-ledger-x.json" >/dev/null 2>"$T/x.err"; rc=$?
[ "$rc" = 4 ] && grep -q 'boom' "$T/x.err" && ok "mint: a failing mint command fails the run (4) and reports its stderr" || bad "mint: failing command gave rc $rc: $(cat "$T/x.err")"
check "mint: the ledger holds one minted and one PENDING row after the crash" "$(python3 -I -c "import json,sys;print(sorted(v['state'] for v in json.load(open(sys.argv[1]))['items'].values()))" "$T/mint-ledger-x.json")" "['minted', 'pending']"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$T/o-x" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$T/mint-ledger-x.json" >/dev/null 2>&1
check "mint: the retry calls the register ONCE more, for the pending cell, with the same key" "$(tail -n +3 "$T/mint.log" | tr '\n' '|')" "alpha integration ~ mint:alpha|integration|"
check "mint: both cells are minted after the retry" "$(python3 -I -c "import json,sys;print(sorted(v['state'] for v in json.load(open(sys.argv[1]))['items'].values()))" "$T/mint-ledger-x.json")" "['minted', 'minted']"
# refusals of what the register printed
for bad_out in "ATM-FX1" "CAT-901 extra"; do
  cat >"$T/mintbad.sh" <<M
#!/usr/bin/env bash
echo '$bad_out'
M
  chmod +x "$T/mintbad.sh"; rm -f "$T/mint-ledger-b.json"
  python3 -I "$GEN" --applicability "$T/m.yaml" --out "$T/o-b2" --timestamp "$TS" --mint --mint-cmd "$T/mintbad.sh" --mint-ledger "$T/mint-ledger-b.json" >/dev/null 2>"$T/b2.err"; rc=$?
  [ "$rc" = 4 ] && grep -q 'not a register item id' "$T/b2.err" && ok "mint: an output that is not a register item id ('$bad_out') is refused (4)" || bad "mint: '$bad_out' accepted (rc $rc): $(cat "$T/b2.err")"
done
cat >"$T/mintdup.sh" <<'M'
#!/usr/bin/env bash
echo CAT-901
M
chmod +x "$T/mintdup.sh"; rm -f "$T/mint-ledger-d.json"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$T/o-d" --timestamp "$TS" --mint --mint-cmd "$T/mintdup.sh" --mint-ledger "$T/mint-ledger-d.json" >/dev/null 2>"$T/d.err"; rc=$?
[ "$rc" = 4 ] && grep -q 'already stands for another cell' "$T/d.err" && ok "mint: the same item id for two cells is refused (4): two cells cannot share one item" || bad "mint: duplicate id accepted (rc $rc): $(cat "$T/d.err")"
printf '{not json\n' >"$T/mint-ledger-m.json"; : >"$T/mint.log"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$T/o-m" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$T/mint-ledger-m.json" >/dev/null 2>"$T/mm.err"; rc=$?
[ "$rc" = 4 ] && [ ! -s "$T/mint.log" ] && ok "mint: a malformed mint ledger refuses (4) and mints nothing (a duplicate item would break 11.4.214)" || bad "mint: malformed ledger gave rc $rc, calls $(wc -l <"$T/mint.log")"
# without a mint command the baseline records the honest skip and writes no ledger rows
OUTN="$T/out-n"; mkdir -p "$OUTN"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTN" --timestamp "$TS" --mint --mint-ledger "$OUTN/mint-ledger.json" >"$T/n.out" 2>"$T/n.err"; rc=$?
[ "$rc" = 0 ] && grep -q 'mint_cmd_absent' "$T/n.out" && ok "mint: no mint command is an honest recorded skip (mint_cmd_absent), never a fabricated item" || bad "mint without a command: rc=$rc out=$(cat "$T/n.out") err=$(cat "$T/n.err")"
[ ! -e "$OUTN/mint-ledger.json" ] && ok "mint: no ledger was written when nothing was minted" || bad "a ledger exists without a mint"
mkmap "$T/dash.yaml" "-alpha:A na na na na na na na na na na na na na na"
python3 -I "$GEN" --applicability "$T/dash.yaml" --out "$T/odash" --timestamp "$TS" >/dev/null 2>"$T/dash.err"; rc=$?
[ "$rc" = 3 ] && grep -q "must not begin with '-'" "$T/dash.err" && ok "mint: a component id beginning with '-' is refused (3): it would reach the mint command as an option" || bad "dash id accepted (rc $rc)"

# the real applicability.yaml of T196 is accepted, has no '?' cell and covers every component x 15 types
REAL="$REPO/specs/001-full-project-audit-remediation/matrix/applicability.yaml"
if [ -f "$REAL" ]; then
  OUTR="$T/out-real"; mkdir -p "$OUTR"
  python3 -I "$GEN" --applicability "$REAL" --out "$OUTR" --timestamp "$TS" >/dev/null 2>"$T/r.err"; rc=$?
  check "T196 applicability.yaml: accepted (no '?' cell, every cell has a reason)" "$rc" 0
  python3 -I - "$OUTR/coverage-matrix.json" "$REAL" <<'PY' && ok "T196 applicability.yaml: cells = components x 15 and the per-state counts add up" || bad "T196 applicability.yaml: count mismatch"
import json,sys,yaml
m=json.load(open(sys.argv[1])); y=yaml.safe_load(open(sys.argv[2]))
n=len(y["components"]); c=m["counts"]
assert c["cells"]==n*15==c["declared"]["P"]+c["declared"]["~"]+c["declared"]["A"]+c["declared"]["n/a"], (n,c)
PY
else bad "T196 applicability.yaml not found at $REAL"; fi


# m3: the 11.4.44 header carries Created forward and raises Revision only when the body changed
H="$T/out-hdr"; mkdir -p "$H"
python3 -I "$GEN" --applicability "$T/good.yaml" --out "$H" --timestamp "2026-10-06T00:00:00Z" >/dev/null 2>&1
python3 -I "$GEN" --applicability "$T/good.yaml" --out "$H" --timestamp "2026-10-07T00:00:00Z" >/dev/null 2>&1
check "m3: Created is carried forward (not overwritten by the second run)" "$(sed -n 's/^| Created | \(.*\) |$/\1/p' "$H/coverage-matrix.md")" "2026-10-06T00:00:00Z"
check "m3: Last modified is the second run" "$(sed -n 's/^| Last modified | \(.*\) |$/\1/p' "$H/coverage-matrix.md")" "2026-10-07T00:00:00Z"
check "m3: Revision stays 1 while the body is unchanged" "$(sed -n 's/^| Revision | \(.*\) |$/\1/p' "$H/coverage-matrix.md")" 1
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$H" --timestamp "2026-10-08T00:00:00Z" >/dev/null 2>&1
check "m3: Revision rises to 2 when the body changed" "$(sed -n 's/^| Revision | \(.*\) |$/\1/p' "$H/coverage-matrix.md")" 2

# ---- paired mutations ----
if [ "${1:-}" != --no-mutations ] && [ -z "${GEN_MUTANT:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  mut() { # mut NAME 'python expr: old' 'new'
    local name="$1" old="$2" new="$3" md="$T/mutev-$1" cp
    # the mutant lives in a symlink farm of tools/evidence so its imports (evcore, evverdict, derive_applicability) resolve to the REAL modules
    # (a mutant copied alone could not import them: the guards that need the imports would then fail for the wrong reason and every mutation of them would look caught)
    rm -rf "$md"; mkdir -p "$md/matrix"; for f in "$REPO"/tools/evidence/*.py; do ln -s "$f" "$md/$(basename "$f")"; done
    ln -s "$REPO/tools/evidence/matrix/derive_applicability.py" "$md/matrix/derive_applicability.py"; cp="$md/matrix/gen_matrix.py"
    python3 -I - "$GEN" "$cp" "$old" "$new" <<'PY'
import sys
s=open(sys.argv[1]).read(); old,new=sys.argv[3],sys.argv[4]
if s.count(old)!=1: sys.exit("mutation anchor count %d != 1 for %r"%(s.count(old),old))
open(sys.argv[2],"w").write(s.replace(old,new))
PY
    [ $? = 0 ] || { bad "mutation $name: could not apply"; return; }
    if GEN="$cp" GEN_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-$name.out" 2>&1; then bad "mutation $name SURVIVED (the test passed with the guard removed)"; echo "SURVIVED $name" >>"$REC"
    else ok "mutation $name caught ($(grep -c '^FAIL:' "$T/mut-$name.out") failing legs)"; echo "CAUGHT $name: $(grep '^FAIL:' "$T/mut-$name.out" | head -2 | cut -c1-110 | tr '\n' '|')" >>"$REC"; fi
  }
  # SANDBOX CONTROL: an UNMUTATED copy in the same symlink farm must pass this body, or every CAUGHT below could be a broken sandbox
  md="$T/mutev-ctl"; mkdir -p "$md/matrix"; for f in "$REPO"/tools/evidence/*.py; do ln -s "$f" "$md/$(basename "$f")"; done; ln -s "$REPO/tools/evidence/matrix/derive_applicability.py" "$md/matrix/derive_applicability.py"; cp "$GEN" "$md/matrix/gen_matrix.py"
  if GEN="$md/matrix/gen_matrix.py" GEN_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-ctl.out" 2>&1; then ok "mutation sandbox control: an UNMUTATED mirror passes this body"; echo "CONTROL PASS" >>"$REC"
  else bad "mutation sandbox control FAILED ($(grep -c '^FAIL:' "$T/mut-ctl.out") failing legs): every CAUGHT below is suspect"; echo "CONTROL FAIL" >>"$REC"; fi
  mut qcheck 'if st == "?":' 'if False:'
  mut noreason 'elif st == "n/a" and not reason:' 'elif False:'
  mut missingtype 'if t not in cells:' 'if False:'
  mut types_check 'or list(amap.get("types")) != TYPES:   # MUT:types_check' 'or False:   # MUT:types_check'
  mut reason_needed 'elif st != "n/a" and not reason:   # MUT:reason_needed' 'elif False:'
  mut strict_yaml 'Strict.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, construct_mapping)   # MUT:strict_yaml' 'pass'
  mut toplevel_handler 'except Exception as e:   # MUT:toplevel_handler' 'except ZeroDivisionError as e:'
  mut timestamp_check 'if a.get("timestamp") and not TS_RE.match(a["timestamp"]):   # MUT:timestamp_check' 'if False:'
  mut overwrite 'os.replace(tmp, path)' 'None if os.path.exists(path) else os.replace(tmp, path)'
  # ledger-derived cell status (K10): the status is DERIVED by evverdict over a chain-walked REAL ledger
  mut chain_walk 'entries, _head = evcore.chain_walk(path)' 'entries = [json.loads(l) for l in open(path)]; _head = None'
  mut RM1_class_check 'if low:   # MUT:class_check' 'if False:'
  mut RM2_verdict_check 'if d["verdict"] != "PASS":' 'if False:'
  mut RM3_blocked_branch 'if last.get("verdict") == "blocked":   # MUT:blocked_branch' 'if False:'
  mut RM4_gate_exit 'if a["gate"] and gate_fail:' 'if a["gate"] and gate_fail and False:'
  mut mutation_caught 'e.get("mutation", {}).get("result") == "caught"' 'True'
  mut candidate_check 'if ctx.get("candidate") and any(' 'if False and any('
  mut gate_needs_ledger 'if a["gate"] and not (a.get("ledger") and a.get("cell_items")):   # MUT:gate_needs_ledger' 'if False:'
  mut gate_needs_binding 'if a["gate"] and not (a.get("repo") or a["map_unbound"]):   # MUT:gate_needs_binding' 'if False:'
  mut gate_unmatched 'if stray:   # MUT:cell_items_unmatched' 'if False:'
  mut gate_vacuous 'if not any(cells[t][0] != "n/a" for _cid, cells in comps for t in TYPES):   # MUT:gate_vacuous' 'if False:'
  mut map_binding 'if diff:   # MUT:map_binding' 'if False:'
  # minting (K11.4): lock, intent row, distinct ids, id shape, corrupt ledger, failing command
  mut mint_lock 'fcntl.flock(lf, fcntl.LOCK_EX)   # MUT:mint_lock' 'pass'
  mut mint_always 'if cur and cur.get("state") == "minted":' 'if False:'
  mut mint_intent 'ledger["items"][key] = {"state": "pending", "declared": st, "component": cid, "type": t}   # MUT:mint_intent' 'pass'
  mut mint_fail 'if p.returncode != 0:' 'if False:'
  mut mint_shape 'if not ITEM_RE.match(item):' 'if False:'
  mut mint_distinct 'if item in known_ids:' 'if False:'
  mut mint_corrupt 'die(4, "the mint ledger %s is unreadable or malformed; refusing to mint (a duplicate item would break 11.4.214)" % led_path)   # MUT:mint_corrupt' 'pass'
  mut mint_key 'p = subprocess.run([a["mint_cmd"], cid, t, st, "mint:" + key]' 'p = subprocess.run([a["mint_cmd"], cid, t, st, "mint"]'
  mut header_created 'created = prev[1]' 'created = ts'
  mut header_revision 'revision = prev[0] if pbody == nbody else prev[0] + 1' 'revision = prev[0]'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"
[ "$FAILS" = 0 ]
