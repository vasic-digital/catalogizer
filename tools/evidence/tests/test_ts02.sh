#!/usr/bin/env bash
# T052 - failing-first test of docs/05 TS-02: the per-script `ab_pass_with_evidence` copies are replaced by ONE definition that records
# an ev/1 entry (tools/evidence/lib/ab_pass_with_evidence.sh) before it prints PASS.
# Parts: (1) the structural census (tools/evidence/census_ab_pass.py): definitions versus carriers, a control needle copy planted in a
# scratch tree must be found, decoys (comment, heredoc, string, doc) must not count as definitions; (2) the repository census:
# zero definitions under scripts/testing/full_automation, exactly one repo-wide, every suite sources it; (3) behaviour: the legacy
# copy (fixtures/ab_pass_legacy.sh) is the golden-bad (PASS without an entry), the shared definition the golden-good; (4) a copy
# restored into a scratch tree is caught (the paired mutation, run by capture_wp05b.sh into ts02-mutation.txt).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
CENSUS=$root/tools/evidence/census_ab_pass.py; LIB=$root/tools/evidence/lib/ab_pass_with_evidence.sh; EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify
FA=scripts/testing/full_automation
S=$(mktemp -d); trap 'rm -rf "$S"' EXIT; fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -f "$CENSUS" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: census tool absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
census() { python3 -I "$CENSUS" --root "$1" "${@:2}" 2>"$S/cerr"; }
jqc() { jq -r "$1" 2>/dev/null; }

# --- (1) census: control needle and decoys in a scratch tree
T=$S/tree; mkdir -p "$T/$FA" "$T/docs/scripts" "$T/.specify/memory"
cat >"$T/$FA/needle.sh" <<'N'
#!/usr/bin/env bash
function ab_pass_with_evidence {
  echo PASS
}
N
cat >"$T/$FA/spaced.sh" <<'N'
#!/usr/bin/env bash
  ab_pass_with_evidence ( )
  {
    echo PASS
  }
N
cat >"$T/$FA/decoys.sh" <<'N'
#!/usr/bin/env bash
# ab_pass_with_evidence() { this is a comment, not a definition
echo "ab_pass_with_evidence() { quoted in a string }"
cat <<'H'
ab_pass_with_evidence() { inside a heredoc }
H
ab_pass_with_evidence "a call, not a definition" /dev/null
N
printf 'The helper ab_pass_with_evidence() { is described here }\n' >"$T/docs/scripts/guide.md"
printf 'ab_pass_with_evidence\n' >"$T/.specify/memory/appendix.md"
out=$(census "$T"); rcc=$?
if [ "$rcc" = 0 ] && [ -n "$out" ]; then
  [ "$(printf '%s' "$out" | jqc '[.definitions[]|.path]|sort|join(",")')" = "$FA/needle.sh,$FA/spaced.sh" ] && ok "census: the control needle (function keyword form) and the spaced form are found, the decoys are not" || bad "census: definitions = $(printf '%s' "$out" | jqc '[.definitions[]|.path]|join(",")')"
  [ "$(printf '%s' "$out" | jqc '[.carriers[]|.path]|sort|join(",")')" = ".specify/memory/appendix.md,docs/scripts/guide.md,$FA/decoys.sh" ] && ok "census: carriers (mentions only) are listed apart from definitions" || bad "census: carriers = $(printf '%s' "$out" | jqc '[.carriers[]|.path]|join(",")')"
  [ "$(printf '%s' "$out" | jqc '.needle_found')" = true ] && ok "census: it reports that its own needle probe was found (the instrument can see)" || bad "census: no needle self-check"
  [ "$(printf '%s' "$out" | jqc '[.calls[]|.path]|join(",")')" = "$FA/decoys.sh" ] && ok "census: a statement-position call is a call, not a definition" || bad "census: calls = $(printf '%s' "$out" | jqc '[.calls[]|.path]|join(",")')"
else bad "census: tool failed on the scratch tree (exit $rcc) $(head -c 150 "$S/cerr")"; bad "census: carriers"; bad "census: needle self-check"; bad "census: calls"; fi
census "$S/nonexistent" >/dev/null; [ $? -ne 0 ] && grep -q 'reason=' "$S/cerr" && ok "census: an absent root is a named refusal, never an empty census" || bad "census: absent root accepted"
# --- (2) the repository census
rep=$(census "$root"); rr=$?
if [ "$rr" = 0 ] && [ -n "$rep" ]; then
  nfa=$(printf '%s' "$rep" | jq "[.definitions[]|select(.path|startswith(\"$FA/\"))]|length" 2>/dev/null)
  [ "$nfa" = 0 ] && ok "repo census: no per-script definition remains under $FA ($nfa)" || bad "repo census: $nfa per-script definitions under $FA"
  [ "$(printf '%s' "$rep" | jqc '[.definitions[]|select(.class=="shipping")|.path]|join(",")')" = "tools/evidence/lib/ab_pass_with_evidence.sh" ] && ok "repo census: the one shipping definition is tools/evidence/lib/ab_pass_with_evidence.sh (the legacy copy under fixtures/ is a fixture)" || bad "repo census: shipping definitions = $(printf '%s' "$rep" | jqc '[.definitions[]|select(.class=="shipping")|.path]|join(",")')"
  nsuite=$(ls "$root/$FA"/*.sh | wc -l); nsrc=$(printf '%s' "$rep" | jq "[.calls[]|select(.path|startswith(\"$FA/\"))]|length" 2>/dev/null)
  [ "$nsrc" = "$nsuite" ] && ok "repo census: every one of the $nsuite suites still calls the helper (nothing was dropped)" || bad "repo census: $nsrc of $nsuite suites call the helper"
  ns=$(grep -lE '^\. +"\$\(cd "\$\(dirname "\$\{BASH_SOURCE\[0\]\}"\)" && pwd\)/\.\./\.\./\.\./tools/evidence/lib/ab_pass_with_evidence\.sh"' "$root/$FA"/*.sh 2>/dev/null | wc -l)
  [ "$ns" = "$nsuite" ] && ok "every suite sources the shared definition by its own location" || bad "$ns of $nsuite suites source the shared definition"
  for f in "$root/$FA"/*.sh; do bash -n "$f" 2>/dev/null || bad "bash -n fails on $f"; done; ok "every suite still parses (bash -n)"
else bad "repo census: tool failed (exit $rr) $(head -c 150 "$S/cerr")"; bad "repo census: the one definition"; bad "repo census: suites still call"; bad "suites source the library"; bad "suites parse"; fi
# --- (3) behaviour: golden-bad (legacy), golden-good (shared), negative control (missing evidence), recorder unavailable
mkev() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchors.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; printf '{"status":"ok"}\n' >"$S/w/evidence.json"; : >"$S/w/empty.json"; }
drive() { # drive FILE_TO_SOURCE : source it, call the helper once on real evidence, print the counters
  bash -c '. "$1"; PASS_COUNT=0; FAIL_COUNT=0; SUMMARY_ROWS=""; ab_pass_with_evidence "desc one" "$2"; echo "rc=$? pass=$PASS_COUNT fail=$FAIL_COUNT"' _ "$1" "$S/w/evidence.json" 2>&1; }
mkev; legacy=$(drive "$here/fixtures/ab_pass_legacy.sh")
printf '%s\n' "$legacy" | grep -q '^PASS: desc one' && [ ! -s "$EV_LEDGER" ] && ok "golden-bad: the legacy copy prints PASS and writes NO ev/1 entry (the bluff the replacement removes)" || bad "golden-bad: legacy copy behaved unexpectedly: $legacy"
if [ -f "$LIB" ]; then
  mkev; gd=$(EVREC_ITEM=CAT-001 drive "$LIB")
  printf '%s\n' "$gd" | grep -q '^PASS: desc one \[evidence: ' && printf '%s\n' "$gd" | grep -q 'rc=0 pass=1 fail=0' && ok "golden-good: the shared definition prints the legacy PASS line and counts it" || bad "golden-good: $gd"
  [ "$(wc -l <"$EV_LEDGER" 2>/dev/null)" = 1 ] && "$VERIFY" >/dev/null 2>&1 && ok "golden-good: exactly one ev/1 entry was written and the ledger verifies" || bad "golden-good: ledger has $(wc -l <"$EV_LEDGER" 2>/dev/null) entries / does not verify"
  e=$(tail -1 "$EV_LEDGER" 2>/dev/null); want_fp=$(sha256sum "$S/w/evidence.json" | cut -d' ' -f1)
  { printf '%s' "$e" | jq -e --arg f "$want_fp" --arg p "$S/w/evidence.json" '.item=="CAT-001" and .polarity=="PROBE" and .exit_status==0 and .target_fingerprint==$f and (.argv|index($p)!=null)' >/dev/null 2>&1; } && ok "golden-good: the entry names the item, is exit 0, carries the sha256 of the evidence bytes and the evidence path in its argv" || bad "golden-good: entry = $e"
  mkev; neg=$(bash -c '. "$1"; PASS_COUNT=0; FAIL_COUNT=0; SUMMARY_ROWS=""; ab_pass_with_evidence "d" "$2"; echo "rc=$? pass=$PASS_COUNT fail=$FAIL_COUNT"' _ "$LIB" "$S/w/empty.json" 2>&1)
  printf '%s\n' "$neg" | grep -q '^FAIL: d \[evidence MISSING or empty: ' && printf '%s\n' "$neg" | grep -q 'rc=1 pass=0 fail=1' && [ ! -s "$EV_LEDGER" ] && ok "negative control: empty evidence prints the legacy FAIL line, returns 1, writes no entry" || bad "negative control: $neg"
  mkev; un=$(EVREC_BIN=$S/no-such-recorder bash -c '. "$1"; PASS_COUNT=0; FAIL_COUNT=0; SUMMARY_ROWS=""; ab_pass_with_evidence "d" "$2"; echo "rc=$? pass=$PASS_COUNT fail=$FAIL_COUNT"' _ "$LIB" "$S/w/evidence.json" 2>&1)
  printf '%s\n' "$un" | grep -q '^FAIL: ' && printf '%s\n' "$un" | grep -q 'rc=1 pass=0 fail=1' && ok "recorder unavailable: FAIL, never PASS (a PASS without an entry is the bluff)" || bad "recorder unavailable: $un"
  mkev; chmod 555 "$S/w"; rf=$(EVREC_ITEM=CAT-001 drive "$LIB"); chmod 755 "$S/w"
  if [ "$(id -u)" = 0 ]; then echo "skip: unwritable-ledger case (root)"; else printf '%s\n' "$rf" | grep -q '^FAIL: ' && printf '%s\n' "$rf" | grep -q 'rc=1 pass=0 fail=1' && ok "recorder refusing (ledger directory unwritable): FAIL, never PASS" || bad "recorder refusing: $rf"; fi
  mkev; d2=$(EVREC_ITEM=CAT-001 bash -c '. "$1"; PASS_COUNT=0; FAIL_COUNT=0; SUMMARY_ROWS=""; ab_pass_with_evidence "a" "$2"; ab_pass_with_evidence "b" "$2"; printf "%b" "$SUMMARY_ROWS"' _ "$LIB" "$S/w/evidence.json" 2>&1)
  [ "$(wc -l <"$EV_LEDGER")" = 2 ] && [ "$(jq -r .iteration "$EV_LEDGER" | tr '\n' ' ')" = "1 2 " ] && printf '%s' "$d2" | grep -q "^PASS	b	" && ok "two calls: two entries with iterations 1 and 2, SUMMARY_ROWS keeps the legacy tab format" || bad "two calls: $(wc -l <"$EV_LEDGER") entries, rows: $d2"
  mkev; (unset EVREC_ITEM; bash -c '. "$1"; PASS_COUNT=0; FAIL_COUNT=0; ab_pass_with_evidence "a" "$2" >/dev/null' _ "$LIB" "$S/w/evidence.json"); jq -e '.item|test("^RUN-[0-9]+$")' "$EV_LEDGER" >/dev/null 2>&1 && ok "without EVREC_ITEM the entry is filed under RUN-<pid> (a canonical run id)" || bad "default item: $(jq -r .item "$EV_LEDGER" 2>/dev/null)"
else for k in 1 2 3 4 5 6 7 8; do bad "behaviour case $k: the shared definition $LIB is absent"; done; fi
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
