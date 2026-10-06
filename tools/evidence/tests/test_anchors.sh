#!/usr/bin/env bash
# T055 - failing-first test of the anchors (docs/06 s7, s8, s13.3 anchor rows, s17 step 4; constitution 11.4.268 B/C/D).
# RED is run against the chain-only verifier of T050 (it answers exit 3 anchor_not_compared for any anchor and has no `evrec anchor`).
# Rows: delete-and-recompute is caught ONLY by the anchor (exit 2), tail truncation likewise (exit 2), an unreadable anchor is
# UNVERIFIED (exit 3), a lagging anchor over an intact prefix is valid growth (exit 0); the strength probe downgrades to `policy`
# when no remote rejects a rewrite; the anchor leg of `evrec rerecord` replaces the local anchors after the common prefix by one
# anchor over the re-chained head and lists it in the seq map. Test-only forgeries come from tests/forge.py.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify; FORGE=$here/forge.py
S=$(mktemp -d); trap 'chmod -R u+rwx "$S" 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchors.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run CAT-001 PROBE "$1" shell_script x -- true >/dev/null 2>&1; }
vrc() { "$VERIFY" >"$S/vo" 2>"$S/ve"; echo $?; }
want() { # want NAME RC [PAT] : verify exits RC and (stdout+stderr) matches PAT
  local r; r=$(vrc); if [ "$r" != "$2" ]; then bad "$1 (exit $r, want $2: $(head -c 160 "$S/vo" | tr '\n' ' ') $(head -c 100 "$S/ve" | tr '\n' ' '))"; return; fi
  if [ -n "${3:-}" ] && ! cat "$S/vo" "$S/ve" | grep -Eq -- "$3"; then bad "$1 (exit ok, output lacks /$3/: $(head -c 160 "$S/vo"))"; return; fi
  ok "$1"; }
four() { fresh; for i in 1 2 3 4; do rec "$i"; done; }
hold() { mv "$EV_ANCHOR" "$S/held.anchor" 2>/dev/null || bad "setup: there is no anchor to hold (the anchor writer produced none)"; }

# --- the anchor writer
four; out=$("$EVREC" anchor 2>"$S/ae"); r=$?
[ "$r" = 0 ] && ok "anchor: evrec anchor exits 0" || bad "anchor: evrec anchor exit $r $(head -c 150 "$S/ae")"
row=$(tail -1 "$EV_ANCHOR" 2>/dev/null)
python3 - "$row" "$EV_LEDGER" <<'P' && ok "anchor row: schema ev-anchor/1, head = ledger head, count 4, UTC time, strength policy" || bad "anchor row malformed: $row"
import json, sys
a = json.loads(sys.argv[1]); led = [json.loads(l) for l in open(sys.argv[2])]
assert a["schema"] == "ev-anchor/1" and a["count"] == 4 and a["head"] == led[-1]["entry_hash"] and a["strength"] == "policy", a
assert a["at"].endswith("Z") and "probe" not in a, a
assert set(a) == {"schema", "head", "count", "at", "strength"}, set(a)
P
want "verify: ledger and anchor agree (exit 0)" 0 'anchor.*agree'
cp "$EV_LEDGER" "$S/orig.jsonl"; cp "$EV_ANCHOR" "$S/orig.anchor"
# --- 13.3 chain rows still hold with an anchor present
sed '2d' "$S/orig.jsonl" >"$EV_LEDGER"; want "tamper: delete entry 2 without repair exits 1 (chain)" 1 'chain'
cp "$S/orig.jsonl" "$EV_LEDGER"
# --- 13.3 anchor rows
python3 "$FORGE" delete "$EV_LEDGER" 2; hold
want "tamper: delete-and-recompute that keeps the old seq values is caught by the chain's contiguity check alone (exit 1, the documented bonus)" 1 'not contiguous'
cp "$S/orig.jsonl" "$EV_LEDGER"; python3 "$FORGE" delete "$EV_LEDGER" 2 renumber
want "tamper: delete-and-recompute (the careful forger renumbers) passes the chain alone (no anchor to compare: exit 0, stated)" 0 'no anchor'
mv "$S/held.anchor" "$EV_ANCHOR"
want "tamper: delete-and-recompute is caught by the anchor (exit 2)" 2 'anchored count=4'
cp "$S/orig.jsonl" "$EV_LEDGER"; python3 "$FORGE" delete "$EV_LEDGER" 3 renumber; mv "$EV_ANCHOR" "$S/held.anchor"; cp "$S/orig.anchor" "$EV_ANCHOR"
want "tamper: deleting a different entry (3) and recomputing is caught by the anchor too (exit 2)" 2 'anchor'
cp "$S/orig.jsonl" "$EV_LEDGER"
python3 "$FORGE" truncate "$EV_LEDGER" 3; hold
want "tamper: truncated tail passes the chain alone (exit 0)" 0 'no anchor'
mv "$S/held.anchor" "$EV_ANCHOR"
want "tamper: truncated tail is caught by the anchor (exit 2)" 2 'anchored count=4'
cp "$S/orig.jsonl" "$EV_LEDGER"
python3 "$FORGE" swap "$EV_LEDGER" 2 3 
want "tamper: two reordered entries with the chain recomputed but the seq values kept: caught by the chain (exit 1)" 1 'not contiguous'
python3 "$FORGE" renumber "$EV_LEDGER"
want "tamper: two reordered entries, chain recomputed AND renumbered, caught by the anchor (exit 2)" 2 'anchor'
cp "$S/orig.jsonl" "$EV_LEDGER"
python3 "$FORGE" set "$EV_LEDGER" 3 duration_ms 999
want "tamper: an edited entry, chain recomputed, caught by the anchor (exit 2)" 2 'anchor'
cp "$S/orig.jsonl" "$EV_LEDGER"
want "original restored exits 0" 0 'anchor.*agree'
# anchor unreadable: UNVERIFIED (exit 3), a different state from PASS and FAIL
rm -f "$EV_ANCHOR"; mkdir "$EV_ANCHOR"; want "anchor unreadable (a directory in its place) is UNVERIFIED, exit 3 anchor_unreadable" 3 'anchor_unreadable'; rmdir "$EV_ANCHOR"
cp "$S/orig.anchor" "$EV_ANCHOR"
if [ "$(id -u)" != 0 ]; then chmod 000 "$EV_ANCHOR"; want "anchor unreadable (mode 000) is UNVERIFIED, exit 3 anchor_unreadable" 3 'anchor_unreadable'; chmod 644 "$EV_ANCHOR"; else echo "skip: mode-000 anchor case (root)"; fi
printf '{"anchor":1}\n' >"$EV_ANCHOR"; want "a malformed anchor row is UNVERIFIED, exit 3 anchor_not_compared" 3 'anchor_not_compared'
printf 'not json at all\n' >"$EV_ANCHOR"; want "an anchor that is not JSON is UNVERIFIED, exit 3" 3 'anchor_not_compared|UNVERIFIED'
cp "$S/orig.anchor" "$EV_ANCHOR"
# a lagging anchor over an intact prefix is valid growth (the continuum negative control)
rec 5; rec 6; want "lagging anchor (ledger grew by 2 after it): exit 0, valid growth" 0 'anchor.*agree'
"$EVREC" anchor >/dev/null 2>&1; want "a fresh anchor over the grown ledger agrees" 0 'anchor.*agree'
python3 "$FORGE" delete "$EV_LEDGER" 2 renumber; want "after growth, delete-and-recompute is still caught against the NEWEST anchor (exit 2)" 2 'anchor'
# anchor rows are append-only history: an older anchor that disagrees with the chain is caught even when the newest agrees
four; "$EVREC" anchor >/dev/null 2>&1; rec 5; "$EVREC" anchor >/dev/null 2>&1; cp "$EV_LEDGER" "$S/five.jsonl"
python3 "$FORGE" set "$EV_LEDGER" 2 duration_ms 777; python3 "$FORGE" rechain "$EV_LEDGER"
cp "$EV_ANCHOR" "$S/two.anchor"; head -n1 "$S/two.anchor" >"$S/first.anchor"
python3 - "$EV_LEDGER" "$S/last.anchor" <<'P'
import json, sys
led = [json.loads(l) for l in open(sys.argv[1])]
a = {"schema": "ev-anchor/1", "head": led[-1]["entry_hash"], "count": 5, "at": "2026-10-06T00:00:00Z", "strength": "policy"}
open(sys.argv[2], "w").write(json.dumps(a, sort_keys=True, separators=(",", ":")) + "\n")
P
cat "$S/first.anchor" "$S/last.anchor" >"$EV_ANCHOR"; want "an old anchor row that the rewritten history contradicts is caught although the newest row agrees (exit 2)" 2 'anchor'
# anchor rows that contradict each other
four; "$EVREC" anchor >/dev/null 2>&1; python3 - "$EV_ANCHOR" <<'P'
import json, sys
a = json.loads(open(sys.argv[1]).read().splitlines()[0]); b = dict(a, head="f" * 64)
open(sys.argv[1], "a").write(json.dumps(b, sort_keys=True, separators=(",", ":")) + "\n")
P
want "two anchor rows with the same count and different heads are inconsistent (exit 2)" 2 'anchor_inconsistent'
# anchoring refuses a broken chain
four; sed '2d' "$EV_LEDGER" >"$S/bad.jsonl"; cp "$S/bad.jsonl" "$EV_LEDGER"; rm -f "$EV_ANCHOR"
"$EVREC" anchor >"$S/ao" 2>"$S/ae"; r=$?; { [ "$r" != 0 ] && [ ! -s "$EV_ANCHOR" ] && grep -q 'reason=chain_failure' "$S/ae"; } && ok "anchor: refuses to anchor a ledger whose chain fails (reason=chain_failure, nothing written)" || bad "anchor: anchored a broken chain (exit $r)"
# a forged history is never re-anchored: the existing anchor rows must agree with the chain before a new row is added
four; "$EVREC" anchor >/dev/null 2>&1; cp "$EV_ANCHOR" "$S/keep.anchor"; python3 "$FORGE" delete "$EV_LEDGER" 2 renumber
"$EVREC" anchor >"$S/ao" 2>"$S/ae"; r=$?; { [ "$r" = 2 ] && grep -q 'reason=anchor_disagrees' "$S/ae" && cmp -s "$EV_ANCHOR" "$S/keep.anchor"; } && ok "anchor: a forged chain is not re-anchored (exit 2 anchor_disagrees, the anchor file is byte-unchanged)" || bad "anchor: re-anchored a forged chain (exit $r)"
# anchoring an empty / absent ledger is refused
fresh; "$EVREC" anchor >"$S/ao" 2>"$S/ae"; r=$?; { [ "$r" != 0 ] && [ ! -e "$EV_ANCHOR" ] && grep -q 'reason=UNVERIFIED' "$S/ae"; } && ok "anchor: refuses an absent ledger (reason=UNVERIFIED)" || bad "anchor: exit $r on an absent ledger"
# --- strength: mechanism needs evidence of a rejected rewrite
four; "$EVREC" anchor --strength mechanism >"$S/ao" 2>"$S/ae"; r=$?; { [ "$r" != 0 ] && grep -q 'strength_unproven' "$S/ae"; } && ok "anchor: --strength mechanism without a probe is refused (strength_unproven)" || bad "anchor: mechanism claimed without evidence (exit $r)"
{ [ ! -s "$EV_ANCHOR" ] && grep -q 'strength_unproven' "$S/ae"; } && ok "anchor: nothing written for the refused claim" || bad "anchor: a row was written for a refused claim"
four; "$EVREC" anchor >/dev/null 2>&1; python3 - "$EV_ANCHOR" <<'P'
import json, sys
a = json.loads(open(sys.argv[1]).read().splitlines()[0]); a["strength"] = "mechanism"
open(sys.argv[1], "w").write(json.dumps(a, sort_keys=True, separators=(",", ":")) + "\n")
P
want "verify: a hand-written mechanism row with no probe evidence is UNVERIFIED (exit 3, strength_unproven)" 3 'strength_unproven'
# --- the strength probe: READ-ONLY inspection of a local bare repository's own configuration (probe_kind=config). Constitution 11.4.113 forbids
# --- every force or non-fast-forward push on any repository, so the probe pushes NOTHING: a PATH shim logs every git call and the test asserts none is a push.
command -v git >/dev/null 2>&1 || echo "skip: git absent, probe cases not run"
if command -v git >/dev/null 2>&1; then
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
  REALGIT=$(command -v git); mkdir -p "$S/shim"
  printf '#!/bin/sh\nprintf "%%s\\n" "$*" >>"%s/gitcalls"\nexec "%s" "$@"\n' "$S" "$REALGIT" >"$S/shim/git"; chmod +x "$S/shim/git"
  git init -q --bare -b main "$S/deny.git"; git -C "$S/deny.git" config receive.denyNonFastForwards true; git -C "$S/deny.git" config receive.denyDeletes true
  git init -q --bare -b main "$S/onlynff.git"; git -C "$S/onlynff.git" config receive.denyNonFastForwards true
  git init -q --bare -b main "$S/onlydel.git"; git -C "$S/onlydel.git" config receive.denyDeletes true
  git init -q --bare -b main "$S/allow.git"; git -C "$S/allow.git" config receive.denyNonFastForwards false; git -C "$S/allow.git" config receive.denyDeletes false
  git init -q --bare -b main "$S/hook.git"; git -C "$S/hook.git" config receive.denyNonFastForwards false
  printf '#!/bin/sh\nexit 1\n' >"$S/hook.git/hooks/pre-receive"; chmod +x "$S/hook.git/hooks/pre-receive"
  mkdir "$S/work-nonbare" && git -C "$S/work-nonbare" init -q
  probe() { : >"$S/gitcalls"; PATH="$S/shim:$PATH" "$EVREC" anchor "$@" >"$S/ao" 2>"$S/ae"; }
  nopush() { ! grep -Eq '(^| )push( |$)' "$S/gitcalls"; }
  four; probe --probe-remote "$S/deny.git"; r=$?
  s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  { [ "$r" = 0 ] && [ "$s" = mechanism ] && tail -1 "$EV_ANCHOR" | jq -e '.probe.rewrite_rejected==true and .probe.probe_kind=="config"' >/dev/null 2>&1; } && ok "probe: both receive.denyNonFastForwards and receive.denyDeletes true gives mechanism with probe_kind=config" || bad "probe(deny): exit $r strength=$s $(head -c 200 "$S/ae")"
  tail -1 "$EV_ANCHOR" | jq -e '.probe.config["receive.denyNonFastForwards"].effective==true and .probe.config["receive.denyDeletes"].effective==true and (.probe.config["receive.denyNonFastForwards"].values[0].value=="true") and (.probe.config["receive.denyNonFastForwards"].values[0].origin|test("deny.git/config"))' >/dev/null 2>&1 && ok "probe: the exact config values read and their origin (show-origin) are recorded in the row" || bad "probe(deny): config values not recorded: $(tail -1 "$EV_ANCHOR" | head -c 400)"
  nopush && grep -q 'config' "$S/gitcalls" && ok "probe: read-only: git was called only to read config, and no push of any kind was made (call log checked)" || bad "probe: a git push was attempted: $(cat "$S/gitcalls" | head -5 | tr '\n' ';')"
  [ -z "$(git -C "$S/deny.git" for-each-ref)" ] && ok "probe: nothing was pushed to the bare repository (no refs)" || bad "probe: the bare repository gained refs"
  want "verify: the probed mechanism row verifies (exit 0)" 0 'mechanism'
  four; probe --strength policy --probe-remote "$S/deny.git"; r=$?
  s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  { [ "$r" = 0 ] && [ "$s" = policy ] && tail -1 "$EV_ANCHOR" | jq -e '.probe.rewrite_rejected==true' >/dev/null 2>&1; } && ok "probe: an explicit --strength policy is honoured even when the config denies a rewrite (the evidence is still recorded)" || bad "probe(policy requested): exit $r strength=$s"
  for k in onlynff onlydel allow; do
    four; probe --probe-remote "$S/$k.git"; r=$?; s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
    { [ "$r" = 0 ] && [ "$s" = policy ] && tail -1 "$EV_ANCHOR" | jq -e '.probe.rewrite_rejected==false and .probe.probe_kind=="config"' >/dev/null 2>&1 && nopush; } && ok "probe($k): a remote whose config does not set BOTH denies stays policy (rewrite_rejected=false recorded, no push)" || bad "probe($k): exit $r strength=$s $(head -c 200 "$S/ae")"
  done
  four; probe --strength mechanism --probe-remote "$S/onlynff.git"; r=$?
  { [ "$r" = 71 ] && grep -q 'strength_unproven' "$S/ae" && [ ! -s "$EV_ANCHOR" ]; } && ok "probe: --strength mechanism over a half-configured remote is refused (strength_unproven, nothing written)" || bad "probe(mechanism, onlynff): exit $r"
  four; probe --probe-remote "$S/hook.git"; r=$?; s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  { [ "$r" = 0 ] && [ "$s" = policy ] && tail -1 "$EV_ANCHOR" | jq -e '.probe.rewrite_rejected==false and (.probe.hooks["pre-receive"]=="executable")' >/dev/null 2>&1; } && ok "probe: an executable pre-receive hook is recorded as evidence only, never as proof (policy, hooks.pre-receive=executable)" || bad "probe(hook): exit $r strength=$s $(tail -1 "$EV_ANCHOR" | head -c 300)"
  # the bare repository's own environment: a global config file that denies both counts (read through git config --show-origin)
  printf '[receive]\n\tdenyNonFastForwards = true\n\tdenyDeletes = true\n' >"$S/global.cfg"
  four; ( export GIT_CONFIG_GLOBAL="$S/global.cfg"; probe --probe-remote "$S/allow.git" ); s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  tail -1 "$EV_ANCHOR" | jq -e '.probe.config["receive.denyNonFastForwards"].values|map(.origin)|map(test("global.cfg"))|any' >/dev/null 2>&1 && ok "probe: the global config of the bare repository's environment is considered and its origin recorded (show-origin)" || bad "probe(global): origin not recorded: $(tail -1 "$EV_ANCHOR" | head -c 400)"
  four; probe --probe-remote "$S/work-nonbare"; r=$?; s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  { [ "$r" = 0 ] && [ "$s" = policy ] && tail -1 "$EV_ANCHOR" | jq -e '.probe.rewrite_rejected==null and .probe.result=="not_a_bare_repository"' >/dev/null 2>&1; } && ok "probe: a non-bare repository is not a server-side remote: policy, result=not_a_bare_repository" || bad "probe(non-bare): exit $r strength=$s"
  four; probe --probe-remote "https://example.invalid/x.git"; r=$?; { [ "$r" != 0 ] && grep -q 'probe_remote_not_local' "$S/ae" && nopush; } && ok "probe: a network remote is never probed (probe_remote_not_local)" || bad "probe(url): exit $r"
  four; probe --probe-remote "$S/does-not-exist.git"; r=$?
  s=$(tail -1 "$EV_ANCHOR" 2>/dev/null | jq -r .strength 2>/dev/null)
  { [ "$r" = 0 ] && [ "$s" = policy ]; } && ok "probe: an unreachable remote downgrades to policy (nothing was shown to reject a rewrite)" || bad "probe(unreachable): exit $r strength=$s"
  four; probe --probe-remote "$S/deny.git" --probe-scratch-ok; r=$?; { [ "$r" = 64 ] && grep -q 'usage_error' "$S/ae" && [ ! -s "$EV_ANCHOR" ]; } && ok "probe: the retired --probe-scratch-ok flag is a usage error (no scratch rewrite exists any more)" || bad "probe(retired flag): exit $r"
  # a mechanism row written by the retired force-based probe (no probe_kind) is no longer evidence
  four; "$EVREC" anchor >/dev/null 2>&1; python3 - "$EV_ANCHOR" <<'P'
import json, sys
a = json.loads(open(sys.argv[1]).read().splitlines()[0]); a["strength"] = "mechanism"; a["probe"] = {"remote": "/x", "rewrite_rejected": True, "result": "rewrite_rejected"}
open(sys.argv[1], "w").write(json.dumps(a, sort_keys=True, separators=(",", ":")) + "\n")
P
  want "verify: a mechanism row whose probe is not probe_kind=config (the retired push probe) is UNVERIFIED strength_unproven" 3 'strength_unproven'
else for k in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16; do bad "probe case $k: git absent"; done; fi
# --- 11.4.113: no force push, no +ref push, no force-with-lease anywhere in the anchor tooling, its tests, its mutation runner or its guide
python3 - "$root" <<'P'
import re, sys, os
root = sys.argv[1]
FORCE = "-" + "-for" + "ce"                      # the pieces keep this scanner from matching itself
LEASE = "for" + "ce-with-" + "lease"
PUSH = "pu" + "sh"
files = ["tools/evidence/evanchor.py", "tools/evidence/tests/test_anchors.sh", "tools/evidence/tests/run_mutations_wp05b.py", "docs/scripts/evidence-recorder.md"]
bad = []
for f in files:
    p = os.path.join(root, f)
    if not os.path.exists(p):
        bad.append("%s: missing" % f); continue
    for n, line in enumerate(open(p, encoding="utf-8", errors="replace"), 1):
        s = line.strip()
        if s.startswith("#") or ("11.4.113" in line and f.endswith(".md")):   # a comment or a doc sentence quoting the ban
            continue
        if re.search(re.escape(FORCE) + r"\b|" + LEASE, line) or (re.search(r"\b" + PUSH + r"\b", line) and re.search(r"(?<![\w+])\+(refs/|HEAD|[A-Za-z0-9_./-]+:)", line)):
            bad.append("%s:%d: %s" % (f, n, s[:100]))
        if f.endswith(".py") and re.search(r"[\"']" + PUSH + r"[\"']", line):
            bad.append("%s:%d: a push is built in code: %s" % (f, n, s[:100]))
print("\n".join(bad)); sys.exit(1 if bad else 0)
P
r=$?; [ "$r" = 0 ] && ok "no-force-push scan: evanchor.py, test_anchors.sh, run_mutations_wp05b.py and the guide contain no forced, lease-guarded, plus-ref or built push (11.4.113)" || bad "no-force-push scan found usage (rc=$r), see lines above"
# --- the anchor leg of `evrec rerecord`: shared prefix of 2, then the two sides diverge; remote anchored at 3, local anchored at 4 (after the prefix)
fresh; mkdir -p "$S/rr"
for i in 1 2; do rec "$i"; done; cp "$EV_LEDGER" "$S/rr/base.jsonl"; cp -r "$EV_BLOBS" "$S/rr/blobs-base"
rec 3; cp "$EV_LEDGER" "$S/rr/remote.jsonl"; "$EVREC" anchor >/dev/null 2>&1; cp "$EV_ANCHOR" "$S/rr/remote.anchors"
cp "$S/rr/base.jsonl" "$EV_LEDGER"; rm -f "$EV_ANCHOR"; "$EVREC" anchor >/dev/null 2>&1; cp "$EV_ANCHOR" "$S/rr/prefix.anchors"
"$EVREC" run CAT-002 PROBE 1 shell_script x -- true >/dev/null 2>&1; "$EVREC" run CAT-002 PROBE 2 shell_script x -- true >/dev/null 2>&1
cp "$EV_LEDGER" "$S/rr/local.jsonl"; "$EVREC" anchor >/dev/null 2>&1; cp "$EV_ANCHOR" "$S/rr/local.anchors"
rm -rf "$S/rr/out"; "$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out" --onto-anchors "$S/rr/remote.anchors" --local-anchors "$S/rr/local.anchors" >"$S/ro" 2>"$S/re"; r=$?
[ "$r" = 0 ] && ok "rerecord anchor leg: exits 0" || bad "rerecord anchor leg: exit $r $(head -c 200 "$S/re")"
if [ -s "$S/rr/out/anchors.jsonl" ]; then
  python3 - "$S/rr/out" "$S/rr/remote.anchors" <<'P' && ok "rerecord anchor leg: the remote anchors are kept verbatim, the local anchors after the prefix are replaced by ONE anchor over the re-chained head" || bad "rerecord anchor leg: wrong anchors"
import json, sys
d = sys.argv[1]
led = [json.loads(l) for l in open(d + "/ledger.jsonl")]
rows = [json.loads(l) for l in open(d + "/anchors.jsonl")]
rem = [json.loads(l) for l in open(sys.argv[2])]
assert rows[:len(rem)] == rem, "remote anchors not kept verbatim"
extra = rows[len(rem):]
assert len(extra) == 1, extra
assert extra[0]["count"] == len(led) and extra[0]["head"] == led[-1]["entry_hash"], (extra, len(led))
sm = json.load(open(d + "/ledger-seq-map.json"))
an = [e for e in sm if e.get("kind") == "anchor"]
assert len(an) == 1 and an[0]["new_count"] == len(led) and an[0]["digest"] == led[-1]["entry_hash"], sm
assert any("old" in e and e.get("kind") != "anchor" for e in sm), "ledger entries must still be listed"
P
  mkdir -p "$S/rr/chk"; cp "$S/rr/out/ledger.jsonl" "$S/rr/chk/ledger.jsonl"; cp "$S/rr/out/anchors.jsonl" "$S/rr/chk/anchors.jsonl"; mkdir -p "$S/rr/chk/blobs"
  cp -r "$S/rr/blobs-base/." "$S/rr/chk/blobs/" 2>/dev/null; cp -r "$EV_BLOBS/." "$S/rr/chk/blobs/" 2>/dev/null
  env -u EV_ANCHOR -u EV_BLOBS "$VERIFY" --ledger "$S/rr/chk/ledger.jsonl" >"$S/vo" 2>"$S/ve"; r=$?; [ "$r" = 0 ] && grep -q 'anchor.*agree' "$S/vo" && ok "rerecord anchor leg: verify accepts the output ledger against the output anchors (exit 0)" || bad "rerecord anchor leg: verify exit $r $(head -c 200 "$S/vo") $(head -c 100 "$S/ve")"
  printf '[\n]\n' >/dev/null
  printf 'ledger#3 and ledger#4 are cited here\n' >"$S/rr/doc.md"; printf '%s\n' "$S/rr/doc.md" >"$S/rr/list"
  "$EVREC" remap-refs --map "$S/rr/out/ledger-seq-map.json" --files-from "$S/rr/list" >"$S/mo" 2>"$S/me"; r=$?; [ "$r" = 0 ] && ok "remap-refs ignores the anchor entry of the seq map and still remaps ledger entries" || bad "remap-refs on a map with an anchor entry: exit $r $(head -c 200 "$S/me")"
else bad "rerecord anchor leg: no anchors.jsonl written"; bad "rerecord anchor leg: verify of the output"; bad "remap-refs with an anchor entry"; fi
rm -rf "$S/rr/out2"; "$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out2" >/dev/null 2>&1; [ ! -e "$S/rr/out2/anchors.jsonl" ] && ok "rerecord without the anchor options writes no anchors (the T049 behaviour is unchanged)" || bad "rerecord wrote anchors unasked"
printf '{"anchor":1}\n' >"$S/rr/badlocal.anchors"; rm -rf "$S/rr/out3"
"$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out3" --onto-anchors "$S/rr/remote.anchors" --local-anchors "$S/rr/badlocal.anchors" >/dev/null 2>&1; r=$?
{ [ "$r" != 0 ] && [ ! -e "$S/rr/out3/ledger.jsonl" ]; } && "$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out3" --onto-anchors "$S/rr/remote.anchors" --local-anchors "$S/rr/badlocal.anchors" 2>&1 >/dev/null | grep -q 'reason=side_unverifiable' && ok "rerecord: a local anchors file that does not verify against its ledger is refused, nothing written" || bad "rerecord accepted an unverifiable anchors file (exit $r)"
"$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out4" --onto-anchors "$S/rr/remote.anchors" >/dev/null 2>&1; r=$?; { [ "$r" = 64 ] && "$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out4" --onto-anchors "$S/rr/remote.anchors" 2>&1 >/dev/null | grep -q 'go together'; } && ok "rerecord: --onto-anchors without --local-anchors is a usage error (64, they go together)" || bad "rerecord: half an anchor leg: exit $r"
hermetic_repo; [ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: host entry reached"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
