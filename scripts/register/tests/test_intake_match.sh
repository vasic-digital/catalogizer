#!/usr/bin/env bash
# test_intake_match.sh - T169 (WP-20): calibration of the engine matcher `workable-items intake-match` BEFORE any duplicate clustering
# of the 1,778-ticket corpus (T172) and before match_basis='import_1to1' may be dropped (doc04 sections 8 and 13.2, constitution 11.4.214(4)).
# Real SQLite, the real engine binary, scratch databases only (never docs/workable_items.db). Dry-run decisions only: the matcher is
# trusted on the corpus only when BOTH controls pass, three times in a row on freshly built databases.
#   (a0) control   the matcher refuses a call without --config (exit 1, no decision file): the instrument can refuse
#   (G)  golden    each golden duplicate (same defect re-reported, optionally re-titled) resolves SAME_DEFECT to the right item
#   (S)  scope     the same text under another scope, or an empty scope, stays DISTINCT (the scope is part of the key)
#   (N)  negative  the primary negative control (two of the eight HELIX-001-* tickets, same scope, genuinely different defects) is
#                  DISTINCT; the full leave-one-out over all eight tickets never yields SAME_DEFECT (a false merge loses a defect) and
#                  equals the pinned verdict vector of the fixture (a change of the matcher or its thresholds must be recalibrated on purpose)
#   (R)  repeats   the three repetitions give byte-identical result vectors
#   (P)  probe     `intake-match --apply` on a register database is REFUSED by the identity trigger (the matcher's mint calls `add`
#                  without --id) and writes nothing: a register mint after a verdict goes through reg_ids + `add --id` (doc04 12.3)
# Output: the calibration record (JSON) at $CALIBRATION_OUT (default: $T_SCR/intake-match-calibration.json; evidence runs set it).
# Env: WI (engine), INTAKE_FIXTURE (default tests/intake_match_fixture.json), CALIBRATION_OUT, EV (evidence root).
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
source "$(dirname "${BASH_SOURCE[0]}")/wp20_ident.sh"
EV=${EV:-$ROOT/specs/001-full-project-audit-remediation/evidence}
FIXTURE=${INTAKE_FIXTURE:-$REG_DIR/tests/intake_match_fixture.json}
OUTJ=${CALIBRATION_OUT:-$T_SCR/intake-match-calibration.json}
ident_header_wp20 T169
# ---- step 0: the engine binary (the T064 verdict lists its sha256 when that verdict is present)
WISHA=$(sha256sum "$WI" | cut -d' ' -f1); VERD="$EV/wp06/register-binaries.json"
if [ -f "$VERD" ]; then grep -q "$WISHA" "$VERD" && ok "engine binary sha256 $WISHA is the recorded verdict" || bad "engine_binary_mismatch: sha256 $WISHA not in $VERD"
else echo "# $VERD absent: engine identity check UNCONFIRMED (sha256 $WISHA recorded in the calibration record)"; fi
# ---- the fixture is the specification of the calibration
if [ ! -f "$FIXTURE" ]; then bad "(fixture) calibration fixture absent: $FIXTURE (RED: no golden/negative-control data yet)"; echo "RESULT: pass=$PASS fail=$FAIL"; exit 1; fi
# ---- (a0) no --config: refused, no decision written
printf '%s' '{"title":"t","scope":"s","description":"d","intake_path":"manual-qa"}' > "$T_SCR/rep0.json"
out0=$("$WI" intake-match --report "$T_SCR/rep0.json" --out "$T_SCR/out0.json" 2>&1); rc0=$?
if [ $rc0 -eq 1 ] && [ ! -e "$T_SCR/out0.json" ] && printf '%s' "$out0" | grep -q -- '--config'; then ok "(a0) intake-match without --config exits 1 and writes no decision"; else bad "(a0) rc=$rc0 out=[$out0] decision_written=$([ -e "$T_SCR/out0.json" ] && echo yes || echo no)"; fi
# ---- (G)(S)(N)(R)(P): the driver builds fresh databases, three times, and compares
WI="$WI" SQLITE3="$SQLITE3" REG_DIR="$REG_DIR" T_SCR="$T_SCR" FIXTURE="$FIXTURE" OUTJ="$OUTJ" WISHA="$WISHA" python3 -I - <<'PY'
import hashlib, json, os, subprocess, sys
WI, REG, SCR, FX, OUTJ = os.environ["WI"], os.environ["REG_DIR"], os.environ["T_SCR"], os.environ["FIXTURE"], os.environ["OUTJ"]
fx = json.load(open(FX, encoding="utf-8"))
SCOPE = fx["scope"]; T = fx["tickets"]
MARK = "**Affected scope / file-scope manifest:**"
fail = 0
def check(label, cond, info=""):
    global fail
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info))); fail += 0 if cond else 1
def sh(args, **kw):
    return subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, **kw)
def build(name, skip=None):
    db = os.path.join(SCR, name)
    for p in (db, db + "-wal", db + "-shm"):
        if os.path.exists(p): os.remove(p)
    r = sh([os.path.join(REG, "apply_ext.sh"), "--db", db]); assert r.returncode == 0, r.stdout
    ids = {}
    for i, t in enumerate(T):
        if i == skip: continue
        # the register path: mint in reg_ids, then `add --id` (doc04 12.3); the scope convention is the matcher's own marker
        sh([os.environ["SQLITE3"], db, "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t169','manual')"])
        atm = sh([os.environ["SQLITE3"], db, "SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1"]).stdout.strip()
        r = sh([WI, "add", "Bug", "High", "--db", db, "--id", atm, "--prefix", "CAT", "--title", t["title"], "--description", t["text"] + "\n\n" + MARK + "\n" + SCOPE])
        assert r.returncode == 0, r.stdout
        ids[i] = atm
    cfg = db + ".cfg.yaml"; open(cfg, "w").write("db: " + db + "\n")
    return db, cfg, ids
def match(cfg, title, desc, scope, tag):
    rep, out = os.path.join(SCR, tag + ".rep.json"), os.path.join(SCR, tag + ".out.json")
    if os.path.exists(out): os.remove(out)
    json.dump({"title": title, "scope": scope, "description": desc, "intake_path": "manual-qa"}, open(rep, "w"))
    r = sh([WI, "intake-match", "--config", cfg, "--report", rep, "--out", out])
    if r.returncode != 0 or not os.path.exists(out): return {"verdict": "ERROR(rc=%d)" % r.returncode, "original": None}
    o = json.load(open(out)); return {"verdict": o["verdict"], "original": o.get("original_item_id")}
def one_run(rep):
    res = {"golden": {}, "scope": {}, "leave_one_out": [], "primary_negative": None, "needle": None}
    db, cfg, ids = build("full%d.db" % rep)
    for g in fx["golden"]:
        t = T[g["ticket"]]; title = t["title"] if g.get("title_from") == "ticket" else g["title"]
        r = match(cfg, title, t["text"] + g["description_suffix"], SCOPE, "g%d%s" % (rep, g["id"])); r["want_item"] = ids[g["ticket"]]; res["golden"][g["id"]] = r
    for s in fx["scope_controls"]:
        t = T[s["ticket"]]; res["scope"][s["id"]] = match(cfg, t["title"], t["text"], s["scope"], "s%d%s" % (rep, s["id"]))
    # control needle: an identical report of a registered ticket MUST be seen (the instrument can see through this exact path)
    res["needle"] = match(cfg, T[0]["title"], T[0]["text"], SCOPE, "needle%d" % rep); res["needle"]["want_item"] = ids[0]
    for k in range(len(T)):
        db2, cfg2, ids2 = build("loo%d_%d.db" % (rep, k), skip=k)
        res["leave_one_out"].append(match(cfg2, T[k]["title"], T[k]["text"], SCOPE, "loo%d_%d" % (rep, k)))
    pn = fx["primary_negative_control"]
    # the register holds ONLY register_ticket: a single-item register
    db3 = os.path.join(SCR, "pn%d.db" % rep); cfg3 = db3 + ".cfg.yaml"; open(cfg3, "w").write("db: " + db3 + "\n")
    for p in (db3, db3 + "-wal", db3 + "-shm"):
        if os.path.exists(p): os.remove(p)
    assert sh([os.path.join(REG, "apply_ext.sh"), "--db", db3]).returncode == 0
    sh([os.environ["SQLITE3"], db3, "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t169','manual')"])
    atm = sh([os.environ["SQLITE3"], db3, "SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1"]).stdout.strip()
    tt = T[pn["register_ticket"]]
    assert sh([WI, "add", "Bug", "High", "--db", db3, "--id", atm, "--prefix", "CAT", "--title", tt["title"], "--description", tt["text"] + "\n\n" + MARK + "\n" + SCOPE]).returncode == 0
    rt = T[pn["report_ticket"]]
    res["primary_negative"] = match(cfg3, rt["title"], rt["text"], SCOPE, "pnr%d" % rep)
    return res
runs = [one_run(i) for i in range(3)]
canon = [json.dumps(r, sort_keys=True) for r in runs]
r0 = runs[0]
for g in fx["golden"]:
    x = r0["golden"][g["id"]]
    check("(G) golden %s: %s" % (g["id"], g["about"]), x["verdict"] == g["expect"] and x["original"] == x["want_item"], x)
for s in fx["scope_controls"]:
    x = r0["scope"][s["id"]]
    check("(S) scope control %s: %s" % (s["id"], s["about"]), x["verdict"] == s["expect"], x)
check("(N) control needle: an identical report of a registered ticket is SEEN (SAME_DEFECT with that item)", r0["needle"]["verdict"] == "SAME_DEFECT" and r0["needle"]["original"] == r0["needle"]["want_item"], r0["needle"])
pn = fx["primary_negative_control"]
check("(N) primary negative control: ticket %d vs register holding ticket %d stays %s" % (pn["report_ticket"], pn["register_ticket"], pn["expect"]), r0["primary_negative"]["verdict"] == pn["expect"], r0["primary_negative"])
loo = [x["verdict"] for x in r0["leave_one_out"]]
check("(N) leave-one-out over the eight same-id tickets never yields SAME_DEFECT (no false merge)", "SAME_DEFECT" not in loo and all(v in ("DISTINCT", "UNDECIDED") for v in loo), loo)
check("(N) leave-one-out equals the pinned verdict vector (matcher calibration unchanged)", loo == fx["leave_one_out_expected"], "got %s want %s" % (loo, fx["leave_one_out_expected"]))
check("(R) the three repetitions on freshly built databases give identical result vectors", canon[0] == canon[1] == canon[2], [hashlib.sha256(c.encode()).hexdigest()[:12] for c in canon])
# (P) --apply on a register DB: refused by the identity trigger, nothing written
db, cfg, ids = build("apply.db")
before = sh([os.environ["SQLITE3"], db, "SELECT count(*) FROM items"]).stdout.strip()
rep = os.path.join(SCR, "apply.rep.json"); out = os.path.join(SCR, "apply.out.json")
json.dump({"title": "A brand new unrelated report about audio crackle", "scope": "x.png", "description": "audio crackles for a second when playback starts", "intake_path": "manual-qa"}, open(rep, "w"))
r = sh([WI, "intake-match", "--config", cfg, "--report", rep, "--out", out, "--apply"])
after = sh([os.environ["SQLITE3"], db, "SELECT count(*) FROM items"]).stdout.strip()
check("(P) intake-match --apply on a register DB is refused by the identity trigger and writes no item (finding: mint goes through reg_ids + add --id)",
      r.returncode != 0 and "no reg_ids row" in r.stdout and before == after, "rc=%d before=%s after=%s out=%s" % (r.returncode, before, after, r.stdout.strip()[:160]))
passed = fail == 0
rec = {"schema": "intake-match-calibration/1", "task": "T169", "engine_sha256": os.environ["WISHA"],
       "fixture_sha256": hashlib.sha256(open(FX, "rb").read()).hexdigest(), "repetitions": 3, "stable_across_repetitions": canon[0] == canon[1] == canon[2],
       "thresholds": {"same_defect_percent": 50, "candidate_percent": 20, "source": "intake_match.go constants, derived by their author on two fixtures; this record is the corpus-shaped check"},
       "golden": r0["golden"], "scope_controls": r0["scope"], "control_needle": r0["needle"], "primary_negative_control": r0["primary_negative"],
       "leave_one_out": loo, "leave_one_out_expected": fx["leave_one_out_expected"], "false_merges": sum(1 for v in loo if v == "SAME_DEFECT"),
       "strict_distinct_count": sum(1 for v in loo if v == "DISTINCT"), "undecided_count": sum(1 for v in loo if v == "UNDECIDED"),
       "calibration_pass": passed,
       "limits": ["measured on the eight HELIX-001-* tickets only (one id, eight different problems): the whole 1,778-ticket corpus is NOT measured",
                  "a re-titled duplicate with a re-worded text scores below the merge threshold (UNDECIDED, mint-with-link), so the matcher merges re-reports of the same text, not paraphrases",
                  "two tickets that share most of a sentence (insecure-password-input and the password-strength ticket) are UNDECIDED, never merged",
                  "intake-match --apply cannot mint on a register database (identity trigger): the verdict is used dry-run, the mint is reg_ids + add --id"],
       "decision": "match_basis stays import_1to1 for the bulk import; T172 may use SAME_DEFECT verdicts as duplicate_of links only for re-reports of the same text and must treat UNDECIDED as mint-with-link" if passed else "calibration FAILED: the matcher is NOT trusted on the corpus; every entry stays its own head (import_1to1)"}
json.dump(rec, open(OUTJ, "w"), indent=1, sort_keys=True); open(OUTJ, "a").write("\n")
print("# calibration record: %s (pass=%s, false_merges=%d, strict_distinct=%d, undecided=%d)" % (OUTJ, passed, rec["false_merges"], rec["strict_distinct_count"], rec["undecided_count"]))
sys.exit(1 if fail else 0)
PY
[ $? -eq 0 ] || FAIL=$((FAIL+1))
echo "RESULT: pass=$PASS fail=$FAIL"
[ "$FAIL" -eq 0 ]
