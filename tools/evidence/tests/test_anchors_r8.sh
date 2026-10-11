#!/usr/bin/env bash
# WP-05 fix round 8 - failing-first tests for the independent review WP-05 NO-GO (evidence/reviews/WP-05.json, findings R1 to R8; T055, T054, T048a).
#   R1  verify exits 3 (UNVERIFIED anchor_absent), never 0, when anchors.jsonl is missing or empty; `--chain-only` is the explicit opt-in and
#       never says "OK"/"verified"
#   R2  an end-of-session anchor path: `evrec end-session` and `evrec run ... --anchor-after`
#   R3  an anchor is `policy` for every remote the producer can reconfigure or that does not exist (the tool stores the anchor in a local file
#       only, so it can never prove `mechanism`); `--strength mechanism` is refused
#   R4  verify rejects every `mechanism` row (hand-written, over a real or a nonexistent remote)
#   R5  the verdict deriver applies the same anchor comparison as verify
#   R7  the five reviewer mutants that survived (REV-1, REV-3, REV-4, REV-5, REV-6): a test per mutant
#   R8  contracts/evidence-record.schema.json test_state_gos: the stale wording is gone and an item must be a safe relative .json path;
#       the WP-05 verdict file validates against contracts/review-verdict.schema.json
# R6 (harness baseline) is tests/test_mutation_harness.sh. Constitution 11.4.224 (test first), 11.4.201 (a null needs a control), 11.4.113.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify; VERDICT=$root/tools/evidence/verdict; FORGE=$here/forge.py
F=specs/001-full-project-audit-remediation
S=$(mktemp -d); trap 'chmod -R u+rwx "$S" 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchors.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run CAT-001 PROBE "$1" shell_script x -- true >/dev/null 2>&1; }
vrc() { "$VERIFY" "$@" >"$S/vo" 2>"$S/ve"; echo $?; }
want() { # want NAME RC PAT [verify args...] : verify exits RC and (stdout+stderr) matches PAT
  local nm=$1 rc=$2 pat=$3; shift 3; local r; r=$(vrc "$@")
  if [ "$r" != "$rc" ]; then bad "$nm (exit $r, want $rc: $(head -c 160 "$S/vo" | tr '\n' ' ') $(head -c 160 "$S/ve" | tr '\n' ' '))"; return; fi
  if [ -n "$pat" ] && ! cat "$S/vo" "$S/ve" | grep -Eq -- "$pat"; then bad "$nm (exit ok, output lacks /$pat/: $(head -c 200 "$S/vo")$(head -c 200 "$S/ve"))"; return; fi
  ok "$nm"; }
four() { fresh; for i in 1 2 3 4; do rec "$i"; done; }

# ============================================================================ R1: no anchor is never "verified"
four
want "R1: a ledger with NO anchors file is UNVERIFIED, exit 3 reason=anchor_absent" 3 'anchor_absent'
grep -Eq '^OK ' "$S/vo" && bad "R1: the output of a no-anchor verify still starts with OK" || ok "R1: a no-anchor verify never prints an OK line"
: >"$EV_ANCHOR"; want "R1: an EMPTY anchors file is UNVERIFIED, exit 3 anchor_absent" 3 'anchor_absent'
rm -f "$EV_ANCHOR"
python3 "$FORGE" truncate "$EV_LEDGER" 3
want "R1: a truncated tail with no anchors file is UNVERIFIED (exit 3), not verified" 3 'anchor_absent'
four; python3 "$FORGE" delete "$EV_LEDGER" 2 renumber
want "R1: delete-and-recompute with no anchors file is UNVERIFIED (exit 3), not verified" 3 'anchor_absent'
four; want "R1: --chain-only on an unanchored ledger exits 0 and says CHAIN-ONLY" 0 'CHAIN-ONLY' --chain-only
grep -Eq '^OK |verified' "$S/vo" && ! grep -Eiq 'not verified|NOT anchor-verified|unverified' "$S/vo" && bad "R1: --chain-only output claims verified: $(cat "$S/vo")" || ok "R1: --chain-only output never claims the ledger is verified"
"$EVREC" anchor >/dev/null 2>&1; cp "$EV_LEDGER" "$S/orig.jsonl"
python3 "$FORGE" delete "$EV_LEDGER" 2 renumber
want "R1: --chain-only does NOT switch off an anchor that is present (delete-and-recompute still exit 2)" 2 'anchored count=4' --chain-only
cp "$S/orig.jsonl" "$EV_LEDGER"
want "R1: ledger plus agreeing anchor still verifies, exit 0 (golden-false: the new rule does not refuse a good ledger)" 0 'anchor agrees'

# ============================================================================ R2: end-of-session anchor
four
"$EVREC" end-session >"$S/o" 2>"$S/e"; r=$?
[ "$r" = 0 ] && grep -q 'anchored count=4' "$S/o" && ok "R2: evrec end-session anchors the ledger (exit 0)" || bad "R2: end-session exit $r: $(head -c 200 "$S/o") $(head -c 200 "$S/e")"
want "R2: after end-session the ledger verifies against its anchor (exit 0)" 0 'anchor agrees'
before=$(sha256sum "$EV_ANCHOR" | cut -c1-64)
"$EVREC" end-session >"$S/o" 2>"$S/e"; r=$?; after=$(sha256sum "$EV_ANCHOR" | cut -c1-64)
{ [ "$r" = 0 ] && grep -q 'anchor unchanged' "$S/o" && [ "$before" = "$after" ]; } && ok "R2: a second end-session on an unchanged ledger is a no-op (anchor file byte-identical)" || bad "R2: second end-session exit $r"
"$EVREC" end-session --strength mechanism >"$S/o" 2>"$S/e"; r=$?
[ "$r" = 64 ] && ok "R2: end-session takes no arguments (usage error 64)" || bad "R2: end-session with an argument exit $r"
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x --anchor-after -- true >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 0 ] && [ "$(wc -l <"$EV_ANCHOR" 2>/dev/null)" = 1 ] && grep -q '"count":1' "$EV_ANCHOR"; } && ok "R2: evrec run --anchor-after anchors right after recording (1 row, count 1)" || bad "R2: run --anchor-after exit $r anchors=$(cat "$EV_ANCHOR" 2>/dev/null | head -c 120)"
"$EVREC" run CAT-001 PROBE 2 shell_script x --anchor-after -- true >/dev/null 2>&1
{ [ "$(wc -l <"$EV_ANCHOR")" = 2 ] && tail -1 "$EV_ANCHOR" | grep -q '"count":2'; } && ok "R2: a second --anchor-after run appends the second anchor row (count 2)" || bad "R2: second --anchor-after: $(cat "$EV_ANCHOR" | head -c 300)"
want "R2: a ledger written with --anchor-after verifies against the last row (exit 0)" 0 'anchor agrees: 2 row'
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x -- true >/dev/null 2>&1; "$EVREC" run CAT-001 PROBE 2 shell_script x -- true >/dev/null 2>&1
sed '1d' "$EV_LEDGER" >"$S/bad.jsonl"; cp "$S/bad.jsonl" "$EV_LEDGER"; rm -f "$EV_ANCHOR"
"$EVREC" end-session >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" != 0 ] && [ ! -e "$EV_ANCHOR" ] && grep -q 'reason=chain_failure' "$S/e"; } && ok "R2: end-session on a ledger whose chain fails writes nothing and fails (exit $r, chain_failure)" || bad "R2: end-session anchored a broken chain (exit $r)"

# ============================================================================ R3 / R4: mechanism is never recorded and never trusted
command -v git >/dev/null 2>&1 || echo "skip: git absent"
if command -v git >/dev/null 2>&1; then
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
  git init -q --bare -b main "$S/deny.git"; git -C "$S/deny.git" config receive.denyNonFastForwards true; git -C "$S/deny.git" config receive.denyDeletes true
  four; "$EVREC" anchor --probe-remote "$S/deny.git" >"$S/o" 2>"$S/e"; r=$?
  s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  { [ "$r" = 0 ] && [ "$s" = policy ]; } && ok "R3: a same-uid local bare repository that denies rewrites by config is still only policy (the reviewer's repro)" || bad "R3: strength=$s exit $r: $(cat "$S/o")"
  tail -1 "$EV_ANCHOR" | jq -e '.probe.producer_can_reconfigure==true and .probe.anchor_location=="local_file" and .probe.probe_kind=="config"' >/dev/null 2>&1 \
    && ok "R3: the probe record states producer_can_reconfigure=true and anchor_location=local_file" || bad "R3: probe record lacks the honest facts: $(tail -1 "$EV_ANCHOR" | head -c 400)"
  four; "$EVREC" anchor --strength mechanism --probe-remote "$S/deny.git" >"$S/o" 2>"$S/e"; r=$?
  { [ "$r" = 71 ] && grep -q 'strength_unproven' "$S/e" && [ ! -s "$EV_ANCHOR" ]; } && ok "R3: --strength mechanism over a reconfigurable remote is refused (71 strength_unproven, nothing written)" || bad "R3: mechanism claim exit $r"
  four; "$EVREC" anchor --strength mechanism --probe-remote "$S/nonexistent.git" >"$S/o" 2>"$S/e"; r=$?
  { [ "$r" = 71 ] && [ ! -s "$EV_ANCHOR" ]; } && ok "R3: --strength mechanism over a nonexistent remote is refused (71)" || bad "R3: nonexistent remote mechanism claim exit $r"
  # R4: a hand-written mechanism row is never accepted, whatever it names
  mkrow() { # mkrow REMOTE COUNT : a mechanism row over the ledger's own entry COUNT with config-probe-shaped evidence
    python3 - "$EV_LEDGER" "$EV_ANCHOR" "$1" "$2" <<'P'
import json, sys
led = [json.loads(l) for l in open(sys.argv[1])]
c = int(sys.argv[4])
a = {"schema": "ev-anchor/1", "head": led[c - 1]["entry_hash"], "count": c, "at": "2026-10-08T00:00:00Z", "strength": "mechanism",
     "probe": {"remote": sys.argv[3], "probe_kind": "config", "rewrite_rejected": True, "result": "rewrite_rejected_by_config"}}
open(sys.argv[2], "w").write(json.dumps(a, sort_keys=True, separators=(",", ":")) + "\n")
P
  }
  four; python3 "$FORGE" truncate "$EV_LEDGER" 3; mkrow /nonexistent 3
  want "R4: a hand-written mechanism row over a NONEXISTENT remote and a truncated ledger is UNVERIFIED (exit 3 strength_unproven), not 'agrees'" 3 'strength_unproven'
  four; mkrow "$S/deny.git" 4
  want "R4: a hand-written mechanism row over a real deny.git remote and an intact ledger is UNVERIFIED (exit 3 strength_unproven)" 3 'strength_unproven'
  four; mkrow /nonexistent 4
  want "R4: a hand-written mechanism row over an intact ledger is not accepted either (exit 3)" 3 'mechanism'
fi

# ============================================================================ R5: the verdict deriver honours the anchors
unset EV_LEDGER EV_ANCHOR EV_BLOBS      # the verdict is asked about explicit --ledger files; their anchors sit beside them
mkscen() { # mkscen CASE DIR : a verdict scenario built with the REAL recorder (and anchored by the scenario)
  EVREC=$EVREC FORGE=$FORGE bash "$here/verdict_cases/scenario.sh" "$1" "$2" >"$S/sc.out" 2>&1 || { bad "R5: scenario $1 failed: $(tail -2 "$S/sc.out" | tr '\n' ' ')"; return 1; }; }
vd() { "$VERDICT" CAT-001 --ledger "$1/ledger.jsonl" "${@:2}" >"$S/do" 2>"$S/de"; echo $?; }
if mkscen good "$S/g"; then
  r=$(vd "$S/g"); { [ "$r" = 0 ] && grep -q '"verdict": "PASS"' "$S/do"; } && ok "R5: golden-false: an anchored good ledger derives PASS, exit 0" || bad "R5: anchored good ledger exit $r: $(head -c 200 "$S/de")"
  grep -q '"anchor"' "$S/do" && ok "R5: the derivation names the anchor result it relied on" || bad "R5: the derivation output carries no anchor field: $(cat "$S/do")"
  cp -r "$S/g" "$S/g2"; python3 "$FORGE" set "$S/g2/ledger.jsonl" 3 duration_ms 999
  v=$("$VERIFY" --ledger "$S/g2/ledger.jsonl" >/dev/null 2>"$S/ve2"; echo $?)
  r=$(vd "$S/g2"); { [ "$v" = 2 ] && [ "$r" = 3 ] && grep -q 'anchor_disagrees' "$S/de"; } && ok "R5: an anchored ledger edited with the chain recomputed: verify exits 2 and verdict exits 3 anchor_disagrees (no verdict)" || bad "R5: edited anchored ledger: verify=$v verdict=$r $(head -c 200 "$S/de") $(head -c 100 "$S/do")"
  cp -r "$S/g" "$S/g3"; rm -f "$S/g3/anchors.jsonl"
  r=$(vd "$S/g3"); { [ "$r" = 3 ] && grep -q 'anchor_absent' "$S/de"; } && ok "R5: a ledger with no anchors derives no verdict (exit 3 anchor_absent)" || bad "R5: unanchored ledger: verdict exit $r"
  r=$(vd "$S/g3" --chain-only); { [ "$r" = 4 ] && grep -q '"anchor": "not compared' "$S/do"; } && ok "R5: --chain-only is the explicit opt-in, the output says the anchor was not compared, and a derived PASS is CHAIN-ONLY-UNVERIFIED exit 4 (round 9 F2)" || bad "R5: --chain-only unanchored: exit $r $(head -c 200 "$S/do")"
  r=$(vd "$S/g2" --chain-only); [ "$r" = 3 ] && ok "R5: --chain-only does not switch off a PRESENT anchor that disagrees (exit 3)" || bad "R5: --chain-only on a forged anchored ledger exit $r"
fi
if mkscen green_first "$S/gf"; then   # the reviewer's second case: reorder so RED precedes GREEN, chain recomputed and renumbered
  python3 "$FORGE" swap "$S/gf/ledger.jsonl" 1 4; python3 "$FORGE" renumber "$S/gf/ledger.jsonl"
  v=$("$VERIFY" --ledger "$S/gf/ledger.jsonl" >/dev/null 2>&1; echo $?); r=$(vd "$S/gf")
  { [ "$v" = 2 ] && [ "$r" = 3 ]; } && ok "R5: a reordered-and-recomputed ledger that flips the verdict's checks is refused (verify 2, verdict 3)" || bad "R5: reordered ledger: verify=$v verdict=$r $(head -c 200 "$S/do")"
fi

# ============================================================================ R7: the five surviving reviewer mutants
# REV-1: anchor rows may go backward
four; "$EVREC" anchor >/dev/null 2>&1; rec 5; "$EVREC" anchor >/dev/null 2>&1
python3 - "$EV_LEDGER" "$EV_ANCHOR" <<'P'
import json, sys
led = [json.loads(l) for l in open(sys.argv[1])]
a = {"schema": "ev-anchor/1", "head": led[2]["entry_hash"], "count": 3, "at": "2026-10-08T00:00:00Z", "strength": "policy"}
open(sys.argv[2], "a").write(json.dumps(a, sort_keys=True, separators=(",", ":")) + "\n")
P
want "REV-1: an anchor row with a count below the previous row's (each row agrees with the chain) is anchor_inconsistent, exit 2" 2 'anchor_inconsistent'
# REV-6: a count 0 anchor row
four; "$EVREC" anchor >/dev/null 2>&1
python3 - "$EV_LEDGER" "$EV_ANCHOR" <<'P'
import json, sys
led = [json.loads(l) for l in open(sys.argv[1])]
a = {"schema": "ev-anchor/1", "head": led[-1]["entry_hash"], "count": 0, "at": "2026-10-08T00:00:00Z", "strength": "policy"}
open(sys.argv[2], "w").write(json.dumps(a, sort_keys=True, separators=(",", ":")) + "\n")
P
want "REV-6: an anchor row with count 0 (it would index entry -1) is not a valid row, exit 3 anchor_not_compared" 3 'anchor_not_compared'
# REV-5: the anchor writer holds the ledger lock
four
python3 - "$root/tools/evidence" "$EV_LEDGER" >"$S/holder.out" 2>&1 <<'P' &
import sys, time
sys.path.insert(0, sys.argv[1]); sys.dont_write_bytecode = True
import evcore
l = evcore.LedgerLock(sys.argv[2], 5); l.acquire(); print("held", flush=True); time.sleep(9); l.release()
P
hp=$!; for _ in $(seq 1 50); do grep -q held "$S/holder.out" 2>/dev/null && break; sleep 0.1; done
EVREC_LOCK_TIMEOUT=1 "$EVREC" anchor >"$S/o" 2>"$S/e"; r=$?
{ [ "$r" = 75 ] && grep -q 'lock_held' "$S/e" && [ ! -e "$EV_ANCHOR" ]; } && ok "REV-5: evrec anchor waits for the ledger lock and refuses (75 lock_held) while another writer holds it, writing nothing" || bad "REV-5: anchor while the lock is held: exit $r anchors=$(ls "$EV_ANCHOR" 2>/dev/null) $(head -c 160 "$S/e")"
kill "$hp" 2>/dev/null; wait "$hp" 2>/dev/null; rm -f "$EV_LEDGER.lock"
# REV-3: the anchor leg of rerecord keeps a local anchor exactly at the common prefix
fresh; mkdir -p "$S/rr"
rec 1; "$EVREC" anchor >/dev/null 2>&1; cp "$EV_ANCHOR" "$S/rr/rem.pre"               # the remote side: anchored at 1 ...
rec 2; cp "$EV_LEDGER" "$S/rr/base.jsonl"; cp "$S/rr/base.jsonl" "$S/rr/lbase.jsonl"
rec 3; cp "$EV_LEDGER" "$S/rr/remote.jsonl"; cp "$S/rr/rem.pre" "$S/rr/remote.anchors"   # ... and grown to 3 (a lagging anchor)
cp "$S/rr/base.jsonl" "$EV_LEDGER"; rm -f "$EV_ANCHOR"; "$EVREC" anchor >/dev/null 2>&1   # the local side: anchored AT the common prefix (count 2)
"$EVREC" run CAT-002 PROBE 1 shell_script x -- true >/dev/null 2>&1; "$EVREC" run CAT-002 PROBE 2 shell_script x -- true >/dev/null 2>&1
"$EVREC" anchor >/dev/null 2>&1; cp "$EV_LEDGER" "$S/rr/local.jsonl"; cp "$EV_ANCHOR" "$S/rr/local.anchors"
rm -rf "$S/rr/out"; "$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out" --onto-anchors "$S/rr/remote.anchors" --local-anchors "$S/rr/local.anchors" >"$S/ro" 2>"$S/re"; r=$?
if [ "$r" = 0 ] && [ -s "$S/rr/out/anchors.jsonl" ]; then
  python3 - "$S/rr/out/anchors.jsonl" <<'P' && ok "REV-3: a local anchor exactly at the common prefix (count 2) survives the re-record next to the remote's count 1 and the re-chained head" || bad "REV-3: anchor at the common prefix dropped: $(cut -c1-120 "$S/rr/out/anchors.jsonl" | tr '\n' '|')"
import json, sys
rows = [json.loads(l) for l in open(sys.argv[1])]
assert [r["count"] for r in rows][:2] == [1, 2], [r["count"] for r in rows]
P
else bad "REV-3: rerecord exit $r $(head -c 200 "$S/re")"; fi
# REV-4: same_test compares the working directory
fresh; mkdir -p "$S/cwa" "$S/cwb"; printf '#!/usr/bin/env bash\n[ "$(cat "%s/state")" = 5 ]\n' "$S" >"$S/chk.sh"; chmod +x "$S/chk.sh"
runin() { # runin DIR POL ITER STATE : record the same argv from DIR against the state file
  printf '%s\n' "$4" >"$S/state"; ( cd "$1" && "$EVREC" run CAT-001 "$2" "$3" shell_script "$S/state" --oracle specified --oracle-independent --evidence-class runtime -- "$S/chk.sh" >/dev/null 2>&1 ); }
cwdcase() { # cwdcase GREEN3DIR
  fresh; runin "$S/cwa" RED 1 2; runin "$S/cwa" GREEN 1 5; runin "$S/cwa" GREEN 2 5; runin "$1" GREEN 3 5; "$EVREC" end-session >/dev/null 2>&1
  "$VERDICT" CAT-001 >"$S/cdo" 2>"$S/cde"; echo $?; }
r=$(cwdcase "$S/cwa"); { [ "$r" = 0 ] && grep -q '"same_test": true' "$S/cdo"; } && ok "REV-4 control: RED and three GREEN from one directory derive PASS (same_test true)" || bad "REV-4 control: exit $r $(head -c 300 "$S/cdo") $(head -c 200 "$S/cde")"
r=$(cwdcase "$S/cwb"); { [ "$r" = 1 ] && grep -q '"same_test": false' "$S/cdo"; } && ok "REV-4: one GREEN run from another directory makes same_test false and the verdict FAIL" || bad "REV-4: other-cwd GREEN: exit $r $(head -c 300 "$S/cdo")"

# ============================================================================ R8: contract wording and constraint, verdict file shape
python3 - "$root/$F/contracts/evidence-record.schema.json" <<'P' && ok "R8: test_state_gos description no longer says the name stays UNCONFIRMED until T048a lands" || bad "R8: the stale UNCONFIRMED sentence is still in the test_state_gos description"
import json, sys
d = json.load(open(sys.argv[1]))["properties"]["test_state_gos"]["description"]
assert "UNCONFIRMED until T048a lands" not in d, d[-200:]
assert "T048a" in d
P
schema_check() { # schema_check NAME valid|refused JSONLIST
  local got; got=$(python3 - "$root/$F/contracts/evidence-record.schema.json" "$3" <<'P'
import json, sys, jsonschema
s = json.load(open(sys.argv[1])); s["$id"] = "urn:ev:1" if ":" not in s.get("$id", "x:") else s["$id"]
rec = {"schema": "ev/1", "seq": 1, "item": "CAT-001", "polarity": "PROBE", "iteration": 1, "started_at": "2026-10-05T00:00:00Z", "cwd": "/tmp", "argv": ["true"],
       "exit_status": 0, "verdict": "pass", "duration_ms": 3, "stdout_sha256": "a" * 64, "stderr_sha256": "a" * 64, "target_fingerprint": "a" * 64,
       "prev_hash": "0" * 64, "entry_hash": "a" * 64, "pre_release": True, "test_state_gos": json.loads(sys.argv[2])}
try: jsonschema.Draft202012Validator(s).validate(rec); print("valid")
except jsonschema.ValidationError: print("refused")
P
)
  [ "$got" = "$2" ] && ok "R8: test_state_gos $1 ($2)" || bad "R8: test_state_gos $1: wanted $2 got ${got:-error}"; }
schema_check "a repository-relative verdict file path" valid '["specs/001-full-project-audit-remediation/evidence/reviews/WP-05.json"]'
schema_check "a short relative verdict file name" valid '["evidence/wp05/go-1.json"]'
schema_check "a path-traversal entry" refused '["../../etc/passwd"]'
schema_check "an absolute path" refused '["/etc/passwd"]'
schema_check "a non-json name" refused '["evidence/reviews/WP-05"]'
schema_check "an embedded parent segment" refused '["evidence/../../x.json"]'
python3 - "$root/$F/evidence/reviews/WP-05.json" "$root/$F/contracts/review-verdict.schema.json" <<'P' && ok "R8: the WP-05 independent verdict file validates against contracts/review-verdict.schema.json (finding_layer in source/artifact/runtime, the 11.4.235(D) class kept as finding_class)" || bad "R8: the WP-05 verdict file does not validate against review-verdict.schema.json"
import json, sys, jsonschema
d = json.load(open(sys.argv[1])); s = json.load(open(sys.argv[2]))
errs = sorted("%s: %s" % ("/".join(map(str, e.absolute_path)), e.message[:120]) for e in jsonschema.Draft202012Validator(s).iter_errors(d))
for f in d["findings"]:
    assert f["finding_layer"] in ("source", "artifact", "runtime") and f["finding_class"] in ("source-defect", "test-instrumentation", "process-doc"), f["id"]
if errs:
    print("\n".join(errs)); sys.exit(1)
P

echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
