#!/usr/bin/env bash
# WP-05 fix round 9 - failing-first tests for the independent re-review WP-05-r2 (NO-GO, one blocking finding):
#   F1 (blocking) reviewer mutant RR2-4: `evrec run --anchor-after` swallowed the anchor refusal and exited 0. The failure path (entry recorded, anchor refused)
#      had no test. The refusal is reached deterministically: the command under measurement writes an anchors file that contradicts the chain AFTER the
#      pre-run check and BEFORE the entry is appended.
#   F2 `verdict --chain-only` on an unanchored ledger never reports PASS: a PASS derived without an anchor is CHAIN-ONLY-UNVERIFIED, exit 4.
#   F3 `evrec run` refuses (fail closed) BEFORE running the command and before appending when the newest anchor contradicts the ledger.
#   F5 `run_mutations.py --check` labels an applicable edit APPLIES, never CAUGHT.
# Constitution 11.4.224 (test first), 11.4.194(6)(d), 11.4.201, 11.4.273.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify; VERDICT=$root/tools/evidence/verdict; FORGE=$here/forge.py
S=$(mktemp -d); trap 'chmod -R u+rwx "$S" 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchors.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run CAT-001 PROBE "$1" shell_script x -- true >/dev/null 2>&1; }
four() { fresh; for i in 1 2 3 4; do rec "$i"; done; }

# ---- F1 / RR2-4: the anchor refusal after a recorded entry is NOT swallowed
fresh
cat >"$S/plant.sh" <<P
#!/usr/bin/env bash
printf '{"at":"2026-10-08T00:00:00Z","count":1,"head":"%s","schema":"ev-anchor/1","strength":"policy"}\n' "$(printf 'f%.0s' $(seq 64))" >"$S/w/anchors.jsonl"
P
chmod +x "$S/plant.sh"
"$EVREC" run CAT-001 PROBE 1 shell_script x --anchor-after -- "$S/plant.sh" >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 2 ] && grep -q 'recorded seq=1' "$S/o" && grep -q 'reason=anchor_disagrees' "$S/e" && grep -q 'WAS recorded' "$S/e"; } \
  && ok "F1/RR2-4: --anchor-after with an anchor the chain contradicts exits 2 anchor_disagrees and says the entry WAS recorded" || bad "F1/RR2-4: exit $r out=$(head -c 120 "$S/o") err=$(head -c 200 "$S/e")"
[ "$(wc -l <"$EV_ANCHOR")" = 1 ] && ok "F1/RR2-4: no anchor row was appended by the refused anchor (the file holds only the planted row)" || bad "F1/RR2-4: anchors file changed: $(wc -l <"$EV_ANCHOR") lines"
"$VERIFY" >/dev/null 2>&1; [ "$?" = 2 ] && ok "F1/RR2-4: verify agrees with the refusal (exit 2)" || bad "F1/RR2-4: verify did not exit 2"
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x --anchor-after -- true >/dev/null 2>&1; r=$?
[ "$r" = 0 ] && ok "F1 control: --anchor-after on a healthy ledger still exits 0" || bad "F1 control: exit $r"

# ---- F3: evrec run fails closed before appending onto a ledger its anchor contradicts
four; "$EVREC" end-session >/dev/null 2>&1; cp "$EV_ANCHOR" "$S/keep.anchor"
python3 "$FORGE" delete "$EV_LEDGER" 2 renumber; cp "$EV_LEDGER" "$S/forged.jsonl"; rm -f "$S/marker"
"$EVREC" run CAT-001 PROBE 9 shell_script x -- bash -c "touch '$S/marker'" >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 2 ] && grep -q 'reason=anchor_disagrees' "$S/e" && [ ! -e "$S/marker" ] && cmp -s "$EV_LEDGER" "$S/forged.jsonl" && cmp -s "$EV_ANCHOR" "$S/keep.anchor"; } \
  && ok "F3: run onto a delete-and-recomputed ledger refuses (2 anchor_disagrees) before the command runs; ledger and anchors byte-unchanged" || bad "F3: exit $r marker=$(ls "$S/marker" 2>/dev/null) err=$(head -c 200 "$S/e")"
four; "$EVREC" end-session >/dev/null 2>&1; python3 "$FORGE" set "$EV_LEDGER" 2 duration_ms 999; cp "$EV_LEDGER" "$S/edited.jsonl"
"$EVREC" run CAT-001 PROBE 9 shell_script x -- true >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 2 ] && grep -q 'reason=anchor_disagrees' "$S/e" && cmp -s "$EV_LEDGER" "$S/edited.jsonl"; } && ok "F3 head: run onto an edited-and-recomputed ledger of the SAME length refuses (2) and appends nothing" || bad "F3 head: exit $r $(head -c 200 "$S/e")"
four; "$EVREC" end-session >/dev/null 2>&1; python3 "$FORGE" truncate "$EV_LEDGER" 3; cp "$EV_LEDGER" "$S/trunc.jsonl"
"$EVREC" run CAT-001 PROBE 9 shell_script x -- true >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 2 ] && cmp -s "$EV_LEDGER" "$S/trunc.jsonl"; } && ok "F3: run onto a tail-truncated ledger refuses (2) and appends nothing" || bad "F3 truncation: exit $r"
four; "$EVREC" end-session >/dev/null 2>&1; rm -f "$EV_ANCHOR"; mkdir "$EV_ANCHOR"; cp "$EV_LEDGER" "$S/same.jsonl"
"$EVREC" run CAT-001 PROBE 9 shell_script x -- true >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 3 ] && cmp -s "$EV_LEDGER" "$S/same.jsonl"; } && ok "F3: an unreadable anchor refuses the run (3 UNVERIFIED) and appends nothing" || bad "F3 unreadable anchor: exit $r"
four; "$EVREC" end-session >/dev/null 2>&1; rec 5; rec 6
"$EVREC" run CAT-001 PROBE 7 shell_script x -- true >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 0 ] && [ "$(wc -l <"$EV_LEDGER")" = 7 ]; } && ok "F3 golden-false: a LAGGING anchor over an intact prefix does not block a run (7 entries)" || bad "F3 control: lagging anchor blocked a run (exit $r) $(head -c 200 "$S/e")"
four; "$EVREC" run CAT-001 PROBE 5 shell_script x -- true >/dev/null 2>&1; r=$?
{ [ "$r" = 0 ] && [ "$(wc -l <"$EV_LEDGER")" = 5 ]; } && ok "F3 golden-false: a ledger with NO anchor file is not blocked (nothing to contradict)" || bad "F3 control: unanchored run exit $r"

# ---- F2: verdict --chain-only never reports PASS without an anchor
mkscen() { EVREC=$EVREC FORGE=$FORGE bash "$here/verdict_cases/scenario.sh" "$1" "$2" >"$S/sc.out" 2>&1 || { bad "F2: scenario $1 failed"; return 1; }; }
unset EV_LEDGER EV_ANCHOR EV_BLOBS
vd() { "$VERDICT" CAT-001 --ledger "$1/ledger.jsonl" "${@:2}" >"$S/do" 2>"$S/de"; echo $?; }
if mkscen good "$S/g"; then
  rm -f "$S/g/anchors.jsonl"; python3 "$FORGE" set "$S/g/ledger.jsonl" 3 duration_ms 999        # edit + recompute, no anchor: the reviewer's repro
  r=$(vd "$S/g" --chain-only)
  { [ "$r" = 4 ] && grep -q '"verdict": "CHAIN-ONLY-UNVERIFIED"' "$S/do" && grep -q '"derived_verdict": "PASS"' "$S/do" && ! grep -q '"verdict": "PASS"' "$S/do"; } \
    && ok "F2: verdict --chain-only on an unanchored recomputed ledger is CHAIN-ONLY-UNVERIFIED, exit 4, never PASS" || bad "F2: exit $r $(head -c 300 "$S/do")"
  cp -r "$S/g" "$S/g2"; mkscen good "$S/g3" && r=$(vd "$S/g3" --chain-only)
  { [ "$r" = 0 ] && grep -q '"verdict": "PASS"' "$S/do"; } && ok "F2 golden-false: --chain-only with a good PRESENT anchor still derives PASS (exit 0)" || bad "F2 control: exit $r $(head -c 200 "$S/do")"
fi
if mkscen blind_red "$S/br"; then
  r=$(vd "$S/br" --chain-only); { [ "$r" = 1 ] && grep -q '"verdict": "FAIL"' "$S/do"; } && ok "F2 golden-false: a chain-only FAIL stays FAIL, exit 1 (the golden-bad fixtures)" || bad "F2 FAIL stays FAIL: exit $r"
fi

# ---- F5: --check is not a kill result
python3 -B "$here/run_mutations.py" --check V9-anchor-note-dropped >"$S/chk" 2>&1
{ grep -q 'APPLIES' "$S/chk" && ! grep -q 'CAUGHT' "$S/chk"; } && ok "F5: run_mutations.py --check labels an applicable edit APPLIES, never CAUGHT" || bad "F5: --check output: $(head -c 300 "$S/chk")"

echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
