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

# --gate (docs/05 13.3): an applicable cell that is not `present` in the ledger fails the gate naming the cell; a complete ledger passes (golden-false)
mkmap "$T/g.yaml" "alpha:P na na na na na na na na na na na na na na"
cat >"$T/ledger-ok.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"source"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/ledger-ok.json" --out "$T/og1" --timestamp "$TS" --gate >"$T/g1.out" 2>"$T/g1.err"; rc=$?
check "gate golden-false: a complete ledger passes (exit 0)" "$rc" 0
echo '[]' >"$T/ledger-empty.json"
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/ledger-empty.json" --out "$T/og2" --timestamp "$TS" --gate >"$T/g2.out" 2>"$T/g2.err"; rc=$?
[ "$rc" = 1 ] && grep -q 'alpha' "$T/g2.err" && grep -q 'unit' "$T/g2.err" && ok "gate: a removed ledger record flips the cell and the gate fails naming it (rc 1)" || bad "gate did not fail on the empty ledger (rc $rc): $(cat "$T/g2.err")"
cat >"$T/ledger-forged.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"PASS","runs":3,"identical_runs":2,"mutation_caught":true,"evidence_class":"source"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/ledger-forged.json" --out "$T/og3" --timestamp "$TS" --gate >/dev/null 2>"$T/g3.err"; rc=$?
[ "$rc" = 1 ] && ok "gate: a record whose runs are not three identical is not present (rc 1)" || bad "gate accepted a non-identical-run record (rc $rc)"
cat >"$T/ledger-nomut.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":false,"evidence_class":"source"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/ledger-nomut.json" --out "$T/og4" --timestamp "$TS" --gate >/dev/null 2>&1; rc=$?
[ "$rc" = 1 ] && ok "gate: a record with no caught mutation is not present (rc 1)" || bad "gate accepted a record without a caught mutation (rc $rc)"
cat >"$T/ledger-class.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"user-visible"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/ledger-class.json" --out "$T/og5" --timestamp "$TS" --gate >/dev/null 2>&1; rc=$?
check "gate: a stronger evidence class than the type requires still passes (golden-false)" "$rc" 0

# minting (spec US3 acceptance 4, docs/05 13.3 revision 19): one absent and one partial cell yield exactly two items, a second run none
mkmap "$T/m.yaml" "alpha:A ~ P na na na na na na na na na na na na"
cat >"$T/mint.sh" <<'M'
#!/usr/bin/env bash
# fixture mint command: records the call and prints an item id
n=$(wc -l <"$MINT_LOG" 2>/dev/null || echo 0); id="ATM-FX$((n+1))"; echo "$1 $2 $3" >>"$MINT_LOG"; echo "$id"
M
chmod +x "$T/mint.sh"; : >"$T/mint.log"
export MINT_LOG="$T/mint.log"
OUTM="$T/out-m"; mkdir -p "$OUTM"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTM" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$OUTM/mint-ledger.json" >/dev/null 2>"$T/m1.err"; rc=$?
check "mint: first run exits 0" "$rc" 0
check "mint: one absent and one partial cell yield exactly two items" "$(wc -l <"$T/mint.log")" 2
grep -q '^alpha unit A$' "$T/mint.log" && grep -q '^alpha integration ~$' "$T/mint.log" && ok "mint: the items name the cells (alpha unit absent, alpha integration partial)" || bad "mint calls wrong: $(cat "$T/mint.log")"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTM" --timestamp "$TS" --mint --mint-cmd "$T/mint.sh" --mint-ledger "$OUTM/mint-ledger.json" >/dev/null 2>&1
check "mint: a second run mints none (11.4.214)" "$(wc -l <"$T/mint.log")" 2
python3 -I - "$OUTM/mint-ledger.json" <<'PY' && ok "mint: the ledger links both cells to their item" || bad "mint ledger malformed"
import json,sys
d=json.load(open(sys.argv[1])); assert len(d["items"])==2, d
assert all(v["item"].startswith("ATM-FX") for v in d["items"].values())
PY
# without a mint command the baseline records the honest skip and writes no ledger rows
OUTN="$T/out-n"; mkdir -p "$OUTN"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTN" --timestamp "$TS" --mint --mint-ledger "$OUTN/mint-ledger.json" >"$T/n.out" 2>"$T/n.err"; rc=$?
[ "$rc" = 0 ] && grep -q 'mint_cmd_absent' "$T/n.out$(true)" "$T/n.err" 2>/dev/null && ok "mint: no mint command is an honest recorded skip (mint_cmd_absent), never a fabricated item" || bad "mint without a command: rc=$rc out=$(cat "$T/n.out") err=$(cat "$T/n.err")"
[ ! -s "$OUTN/mint-ledger.json" ] || ! grep -q 'ATM' "$OUTN/mint-ledger.json" && ok "mint: no item was recorded when nothing was minted" || bad "a ledger row exists without a mint"
# a failing mint command records nothing for that cell and the run fails
cat >"$T/mintfail.sh" <<'M'
#!/usr/bin/env bash
echo "ATM-FAILED"; exit 7
M
chmod +x "$T/mintfail.sh"; OUTF="$T/out-f"; mkdir -p "$OUTF"
python3 -I "$GEN" --applicability "$T/m.yaml" --out "$OUTF" --timestamp "$TS" --mint --mint-cmd "$T/mintfail.sh" --mint-ledger "$OUTF/mint-ledger.json" >/dev/null 2>&1; rc=$?
[ "$rc" != 0 ] && ok "mint: a failing mint command fails the run (rc $rc)" || bad "failing mint command exit 0"
grep -q 'ATM' "$OUTF/mint-ledger.json" 2>/dev/null && bad "a ledger row exists for a failed mint" || ok "mint: a failed mint leaves no ledger row"

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


# ---- review round 1 (WF11 B1, RM1-RM4): the gate needs evidence; every cell rule has a leg that can fail ----
python3 -I "$GEN" --applicability "$T/g.yaml" --out "$T/og6" --timestamp "$TS" --gate >"$T/g6.out" 2>"$T/g6.err"; rc=$?
check "gate B1: --gate WITHOUT --ledger is a usage refusal (exit 2), never a vacuous pass" "$rc" 2
grep -q 'requires --ledger' "$T/g6.err" && ok "gate B1: the refusal names the missing ledger" || bad "gate B1: refusal text missing: $(cat "$T/g6.err")"
[ ! -e "$T/og6/coverage-matrix.json" ] && ok "gate B1: a refused gate wrote no matrix" || bad "gate B1: output written despite the refusal"
python3 -I "$GEN" --applicability "$T/g.yaml" --out "$T/og6b" --timestamp "$TS" >/dev/null 2>&1; rc=$?
check "gate B1 control: the same map without --gate and without a ledger still renders (exit 0)" "$rc" 0
cat >"$T/ledger-typo.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"source"},
 {"component":"alpah","type":"unit","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"source"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/ledger-typo.json" --out "$T/og7" --timestamp "$TS" --gate >/dev/null 2>"$T/g7.err"; rc=$?
[ "$rc" = 3 ] && grep -q 'ledger_record_unmatched' "$T/g7.err" && ok "gate: a record naming no component of the map is refused (3 ledger_record_unmatched)" || bad "gate: stray record accepted (rc $rc)"
mkmap "$T/allna.yaml" "alpha:na na na na na na na na na na na na na na na"
python3 -I "$GEN" --applicability "$T/allna.yaml" --ledger "$T/ledger-empty.json" --out "$T/og8" --timestamp "$TS" --gate >/dev/null 2>"$T/g8.err"; rc=$?
[ "$rc" = 3 ] && grep -q 'gate_vacuous' "$T/g8.err" && ok "gate: a map with no applicable cell is refused (3 gate_vacuous)" || bad "gate: vacuous map accepted (rc $rc)"
# RM1: an evidence class BELOW the type's need is not present (e2e needs runtime, ui needs user-visible) - and the exact class is present
mkmap "$T/cls.yaml" "alpha:na na P na na na na na na na na P na na na"
cat >"$T/lc-low.json" <<'J'
[{"component":"alpha","type":"e2e","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"source"},
 {"component":"alpha","type":"ui","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"runtime"}]
J
python3 -I "$GEN" --applicability "$T/cls.yaml" --ledger "$T/lc-low.json" --out "$T/oc1" --timestamp "$TS" --gate >/dev/null 2>"$T/c1.err"; rc=$?
check "RM1: source evidence for e2e and runtime evidence for ui are below the required class: the gate fails" "$rc" 1
grep -q 'evidence class source is below runtime' "$T/c1.err" && grep -q 'evidence class runtime is below user-visible' "$T/c1.err" && ok "RM1: both cells are named with their class gap" || bad "RM1: class gap not named: $(cat "$T/c1.err")"
check "RM1: e2e and ui are partial in the json" "$(python3 -I -c "import json;d=json.load(open('$T/oc1/coverage-matrix.json'));print(sorted(c['status'] for c in d['cells'] if c['declared']!='n/a'))")" "['partial', 'partial']"
cat >"$T/lc-ok.json" <<'J'
[{"component":"alpha","type":"e2e","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"runtime"},
 {"component":"alpha","type":"ui","verdict":"PASS","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"user-visible"}]
J
python3 -I "$GEN" --applicability "$T/cls.yaml" --ledger "$T/lc-ok.json" --out "$T/oc2" --timestamp "$TS" --gate >/dev/null 2>&1; rc=$?
check "RM1 golden-false: runtime for e2e and user-visible for ui pass the gate" "$rc" 0
# RM2: a FAIL verdict is not present even with three identical runs, a caught mutation and a sufficient class
cat >"$T/lv-fail.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"FAIL","runs":3,"identical_runs":3,"mutation_caught":true,"evidence_class":"user-visible"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/lv-fail.json" --out "$T/ov1" --timestamp "$TS" --gate >/dev/null 2>"$T/v1.err"; rc=$?
check "RM2: a FAIL verdict never counts as present: the gate fails" "$rc" 1
grep -q 'verdict FAIL is not PASS' "$T/v1.err" && ok "RM2: the verdict is named" || bad "RM2: verdict reason missing: $(cat "$T/v1.err")"
check "RM2: the cell is partial in the json" "$(python3 -I -c "import json;d=json.load(open('$T/ov1/coverage-matrix.json'));print([c['status'] for c in d['cells'] if c['declared']!='n/a'])")" "['partial']"
# RM3: a record that says blocked yields the status `blocked` (not partial), and the gate still fails
cat >"$T/lb.json" <<'J'
[{"component":"alpha","type":"unit","verdict":"blocked","runs":0,"identical_runs":0,"mutation_caught":false,"evidence_class":"source","blocked":"image not built"}]
J
python3 -I "$GEN" --applicability "$T/g.yaml" --ledger "$T/lb.json" --out "$T/ob1" --timestamp "$TS" --gate >/dev/null 2>"$T/b1.err"; rc=$?
check "RM3: a blocked cell still fails the gate (blocked is not present)" "$rc" 1
check "RM3: the cell status is blocked in the json" "$(python3 -I -c "import json;d=json.load(open('$T/ob1/coverage-matrix.json'));print([c['status'] for c in d['cells'] if c['declared']!='n/a'])")" "['blocked']"
grep -q 'a record says blocked' "$T/b1.err" && ok "RM3: the gate output says blocked" || bad "RM3: blocked reason missing: $(cat "$T/b1.err")"
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
    local name="$1" old="$2" new="$3" cp="$T/mut-$1.py"
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
  mut qcheck 'if st == "?"' 'if False'
  mut noreason 'if st == "n/a" and not reason' 'if False'
  mut missingtype 'if t not in cells:' 'if False:'
  mut gate_runs 'rec.get("identical_runs") == rec.get("runs") == 3' 'True'
  mut gate_mut 'rec.get("mutation_caught") is True' 'True'
  mut mint_always 'if key in ledger["items"]:' 'if False:'
  mut mint_fail 'if rc != 0:' 'if False:'
  mut overwrite 'os.replace(tmp, path)' 'None if os.path.exists(path) else os.replace(tmp, path)'
  # review round 1: RM1-RM4 adopted verbatim (the reviewer's four gen_matrix mutations) plus the gate-input guards and the header carry-forward
  mut RM1_class_check 'if CLASS_RANK.get(rec.get("evidence_class"), 0) < need:' 'if False:'
  mut RM2_verdict_check 'if rec.get("verdict") != "PASS":' 'if False:'
  mut RM3_blocked_branch 'if any(r.get("blocked") or r.get("verdict") == "blocked" for r in mine):' 'if False:'
  mut RM4_gate_exit 'if a["gate"] and gate_fail:' 'if a["gate"] and gate_fail and False:'
  mut gate_needs_ledger 'if a["gate"] and not a.get("ledger"):' 'if False:'
  mut gate_unmatched 'if stray:' 'if False:'
  mut gate_vacuous 'if not any(cells[t][0] != "n/a" for _cid, cells in comps for t in TYPES):' 'if False:'
  mut header_created 'created = prev[1]' 'created = ts'
  mut header_revision 'revision = prev[0] if pbody == nbody else prev[0] + 1' 'revision = prev[0]'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"
[ "$FAILS" = 0 ]
