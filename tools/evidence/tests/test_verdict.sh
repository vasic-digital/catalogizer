#!/usr/bin/env bash
# T054 - failing-first test of tools/evidence/verdict, the verdict deriver (docs/06 s4.2, s10, s13.1, s13.4).
# The 18 scenario cases of docs/06 s13.1 are ported into verdict_cases/ (scenario.sh builds each ledger with the REAL recorder,
# expected.tsv holds the expected verdict of each case from docs/06 s13.4): `good` and the honest recurrence `second_cycle` derive
# PASS, the other 16 derive FAIL with the failing check the document names. Anti-vacuity: while the deriver is absent no check passes.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
VERDICT=$root/tools/evidence/verdict; export EVREC=$root/tools/evidence/evrec FORGE=$here/forge.py
S=$(mktemp -d); trap 'rm -rf "$S"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$VERDICT" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: $VERDICT absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
# check CASE DIR MODE VERDICT FALSE_CHECKS EXTRA : derive and compare against the expected row
check() {
  local c=$1 d=$2 mode=$3 want=$4 falses=$5 extra=$6 out rc flag=""
  [ "$mode" = chain-only ] && flag=--chain-only
  out=$("$VERDICT" CAT-001 --ledger "$d/ledger.jsonl" $flag 2>"$d/err"); rc=$?
  local wrc=1; [ "$want" = PASS ] && wrc=0
  [ "$rc" = "$wrc" ] || { bad "case $c: exit $rc, want $wrc ($(head -c 150 "$d/err"))"; return; }
  python3 - "$out" "$want" "$falses" "$extra" "$c" <<'PY' && ok "case $c derives $want${falses:+ ($falses false)}" || bad "case $c: wrong checks: $out"
import json, sys
o = json.loads(sys.argv[1]); want, falses, extra, case = sys.argv[2:6]
assert o["verdict"] == want, ("verdict", o["verdict"])
CHK = ["red_ok", "green_ok", "green_identical", "same_test", "red_before_green", "fingerprints_differ", "fingerprints_new"]
if want == "PASS":
    assert all(o[k] is True for k in CHK), ("a check is false on a PASS", o)
for k in (falses.split(",") if falses != "-" else []):
    assert o[k] is False, ("%s must be false" % k, o)
if want == "FAIL":
    assert any(o[k] is False for k in CHK), "FAIL with every check true"
for kv in (extra.split(",") if extra != "-" else []):
    pass
# extra assertions: split on commas that are not inside [...]
import re
for kv in re.findall(r"([a-z_]+)=(\[[^\]]*\]|[^,]+)", extra if extra != "-" else ""):
    assert o[kv[0]] == json.loads(kv[1]), (kv[0], o[kv[0]], kv[1])
PY
}
if [ -x "$EVREC" ]; then
  while IFS=$'\t' read -r c want falses extra mode; do
    case $c in ''|'#'*) continue ;; esac
    d=$S/case-$c; bash "$here/verdict_cases/scenario.sh" "$c" "$d" >"$S/sc.out" 2>&1 || { bad "case $c: scenario failed: $(tail -2 "$S/sc.out" | tr '\n' ' ')"; continue; }
    check "$c" "$d" "$mode" "$want" "$falses" "$extra"
  done <"$here/verdict_cases/expected.tsv"
else
  for c in good blind_red exit127 exit126 signal other_argv other_target dash_argv typo_red green_dup_iter green_first reopen_no_new second_cycle launder reopen_pass reopen_after_fail green_on_reopen_fp green_old_fp; do bad "case $c: the recorder tools/evidence/evrec is absent"; done
fi
cnt=$(grep -vc '^#\|^$' "$here/verdict_cases/expected.tsv"); [ "$cnt" = 18 ] && ok "expected.tsv holds the 18 scenario cases" || bad "expected.tsv holds $cnt cases, want 18"
# --- layered defence: the strict mode refuses a ledger the recorder could not have written; a broken chain is unverifiable
d=$S/case-blind_red; if [ -f "$d/ledger.jsonl" ]; then
  "$VERDICT" CAT-001 --ledger "$d/ledger.jsonl" >"$S/o" 2>"$S/e"; r=$?; [ "$r" = 3 ] && grep -q 'reason=schema_invalid' "$S/e" && ok "strict mode refuses a forged RED that passed (exit 3 schema_invalid)" || bad "strict mode on a forged ledger: exit $r"
fi
d=$S/case-good; if [ -f "$d/ledger.jsonl" ]; then
  cp "$d/ledger.jsonl" "$S/del.jsonl"; sed -i '2d' "$S/del.jsonl"
  "$VERDICT" CAT-001 --ledger "$S/del.jsonl" >"$S/o" 2>"$S/e"; r=$?; [ "$r" = 3 ] && grep -q 'reason=chain_failure' "$S/e" && ok "a ledger with a deleted entry is unverifiable (exit 3 chain_failure), no verdict" || bad "deleted entry: exit $r"
  "$VERDICT" CAT-999 --ledger "$d/ledger.jsonl" >"$S/o" 2>"$S/e"; r=$?; [ "$r" = 1 ] && grep -q '"verdict": *"FAIL"' "$S/o" && ok "an item with no entries derives FAIL" || bad "unknown item: exit $r"
  "$VERDICT" >"$S/o" 2>"$S/e"; r=$?; [ "$r" = 64 ] && ok "no argument is a usage error (64)" || bad "no argument: exit $r"
  "$VERDICT" CAT-001 --nope >"$S/o" 2>"$S/e"; r=$?; [ "$r" = 64 ] && ok "an unknown flag is a usage error (64)" || bad "unknown flag: exit $r"
  "$VERDICT" CAT-001 --ledger "$S/none.jsonl" >"$S/o" 2>"$S/e"; r=$?; [ "$r" = 3 ] && ok "an absent ledger is unverifiable (3)" || bad "absent ledger: exit $r"
  # the derivation reads the ledger only: the same ledger twice gives byte-identical output (deterministic, 11.4.50)
  a=$("$VERDICT" CAT-001 --ledger "$d/ledger.jsonl" 2>/dev/null); b=$("$VERDICT" CAT-001 --ledger "$d/ledger.jsonl" 2>/dev/null); [ -n "$a" ] && [ "$a" = "$b" ] && ok "deterministic: two derivations are byte-identical" || bad "derivation not deterministic"
fi
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
