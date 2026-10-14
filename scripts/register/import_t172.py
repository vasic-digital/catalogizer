#!/usr/bin/env python3
"""import_t172.py - the engine of scripts/register/import_t172.sh (T172, WP-20; docs/12 section 11.3 and 11.4, QF-10; doc04 sections 8 and 9).

doc12 WP-Q5 ticket specifics inside the import. Two stages over the issue_file entries (docs/issues/*.md) that Stage 0 (enumerate_sources.py) put into
reg_source_entries; every ticket text is read from the FROZEN SNAPSHOT named by --freeze-json through the one reader of import_tickets.py (never the live
tree, never an evidence file the ticket names), after scripts/register/check_freeze_snapshot.sh (T161) has passed. Neither stage inserts a reg_source_entries
row: the sorted triple export (locator of the source, locator of the entry, entry sha256) is taken before and after and a difference is refused
`t172_entry_set_changed` naming each triple.

  --stage qa        BEFORE import.sh (T168): the closed-class tickets whose resolution says the QA tool, not the product, was wrong ("QA infrastructure
                    failure ... screenshot timing", wrong screen, black frame, wrong ADB command or package) are grouped by the STATED CAUSE into ONE `Bug`
                    item per cause (component `helixqa` or `qa-scripts`, status Queued, mint_basis import, category bug); every ticket of the cause is mapped
                    to that item in reg_source_map (the lowest locator `primary`, the others `duplicate_of`, match_basis manual_operator, exactly one
                    severity_governs row), so T168 skips those entries (an entry already in reg_source_map is skipped) and imports no per-ticket item for
                    them. A QA ticket that T168 already imported as its own item is refused `qa_entry_individually_imported` (a mapping is corrected, never
                    deleted: reg_source_map_no_delete) and nothing is written. The component must already be a reg_components row (`component_not_registered`).
  --stage families  AFTER import.sh: (1) platform inference for the tickets whose `platform:` is empty, from the path strings of the ticket text (screen
                    field and body: screenshot, frame and logcat names, URLs); a declared platform wins; two evidence classes are UNKNOWN (conflict), no
                    evidence is UNKNOWN, never guessed; the inferred value is appended to the item description as `[DEFAULT - adjustable]` and recorded in
                    platform_inference.json. (2) duplicate families by (normalised title slug, platform class, screen; an empty screen is no scope and is never linked): the members of a family get one
                    reg_recurrence_links row each (intake_path import, verdict UNDECIDED = mint-with-link, the autonomous default of doc04 section 8, because a
                    wrongly merged defect is lost; the head is the lowest locator); members that share only the slug are recorded as distinct (families.json).
Usage: import_t172.py --stage qa|families --db DB --freeze-json F --engine WI --out-dir D [--category-map F] [--backup-record F] [--progress-log F]
         [--report F] [--busy-timeout-ms N] [--dry-run]
Outputs in --out-dir: qa_causes.json (qa) | families.json and platform_inference.json (families) | entryset-before.tsv, entryset-after.tsv, entryset.json (both).
Exit: 0 ok | 2 usage | 20 refusal (`import_t172: REFUSED reason=<code> ...`) | 21 a write failed and the run is resumable | 1 engine or invariant failure.
Idempotent and resumable (marker `register-import-t172:qa:<cause>` in reg_ids.minted_by); a second run writes nothing.
Test hooks (refused unless IMPORT_TEST_MODE=1): IMPORT_FAULT=<stage>:<n> (after_mint after_ext after_add before_map) as import_tickets.py.
"""
import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
import re
import sqlite3
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.dont_write_bytecode = True


def _load(name, fname):
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, fname))
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


IT = _load("import_tickets", "import_tickets.py")
PL = _load("import_plan", "import_plan.py")
Refusal = IT.Refusal
ACTOR = "register-import-t172"
CLOSED_CLASS = IT.CLOSED_CLASS
SEV_RANK = PL.SEV_RANK
ENTRYSET_SQL = "SELECT s.locator, e.locator, e.entry_sha256 FROM reg_source_entries e JOIN reg_sources s USING(source_id) ORDER BY 1, 2, 3"

# the stated causes (docs/12 section 11.3): key -> (label, component, fix text); first matching rule wins (CAUSE_RULES order)
CAUSES = {
    "screenshot_timing": ("the capture is taken before the app reached the expected state (screenshot timing)", "helixqa"),
    "black_frame": ("the captured frame is black or blank (nothing rendered at capture time)", "helixqa"),
    "wrong_screen": ("the capture shows the wrong screen", "helixqa"),
    "wrong_adb": ("a wrong ADB command, key code or package name drives the test", "qa-scripts"),
    "other_qa_instrument": ("another statement that the QA infrastructure, not the product, was wrong [DEFAULT - adjustable]", "helixqa"),
}
CAUSE_RULES = IT.CAUSE_RULES            # the classifier lives in import_tickets.py (shared with the T168 order guard)
QA_STATEMENT = IT.QA_STATEMENT
FIX = ("Fix (docs/12 section 11.3): a deterministic capture wait that waits for a SEEN UI condition instead of `sleep:`, with a RED case. The product behaviours "
       "those tickets pointed at are re-tested by the new cases; a product defect that was dismissed as infrastructure noise surfaces as RED.")

# platform evidence (docs/12 section 11.4): closed rule table over the path strings of the ticket text; first column is the platform class
PLATFORM_RULES = (
    ("androidtv", re.compile(r"android[ _-]?tv", re.I)),
    ("android", re.compile(r"(?<![a-z0-9])android(?![a-z0-9])(?![ _-]?tv)", re.I)),
    ("web", re.compile(r"(?<![a-z0-9])web(?![a-z0-9])", re.I)),
    ("desktop", re.compile(r"(?<![a-z0-9])desktop(?![a-z0-9])", re.I)),
    ("installer", re.compile(r"(?<![a-z0-9])installer(?![a-z0-9])", re.I)),
    ("video-frame", re.compile(r"(?<![a-z0-9])frame_[0-9]+\.(?:png|jpg)", re.I)),
    ("api", re.compile(r"https?://(?:localhost|127\.0\.0\.1)(?::[0-9]+)?/api\b", re.I)),
)
PATH_TOKEN = re.compile(r"[A-Za-z0-9_./:()-]*[A-Za-z0-9_-]+\.(?:png|jpg|jpeg|gif|mp4|txt|log|json)\b|https?://[^\s)]+|logcat[A-Za-z0-9_.-]*", re.I)


def slug_of(title):
    return re.sub(r"[^a-z0-9]+", "-", (title or "").lower()).strip("-")


def screen_of(s):
    return re.sub(r"\s+", " ", (s or "").strip())


PLATFORM_CLASSES = {re.sub(r"[ _-]+", "", p): p for p, _ in PLATFORM_RULES}


def norm_declared(declared):
    """a declared platform spelled as a class (`Android TV`, `android_tv`, `Android-TV`) is that class (`androidtv`); any other declared value is kept as written"""
    return PLATFORM_CLASSES.get(re.sub(r"[ _-]+", "", declared.lower()), declared)


def infer_platform(fm, body):
    """-> (platform|None, basis, evidence list). declared wins; two evidence classes -> UNKNOWN conflict; none -> UNKNOWN."""
    declared = (fm.get("platform") or "").strip()
    if declared:
        return norm_declared(declared), "declared", []
    strings = []
    if (fm.get("screen") or "").strip():
        strings.append(("screen", fm["screen"].strip()))
    for m in PATH_TOKEN.finditer(body or ""):
        strings.append(("body", m.group(0)))
    found, ev = {}, []
    for src, s in strings:
        for plat, rx in PLATFORM_RULES:
            if rx.search(s):
                found.setdefault(plat, []).append("%s:%s" % (src, s[:80]))
                break      # one class per evidence string, in table order
    if len(found) == 1:
        plat = next(iter(found))
        return plat, "inferred", found[plat][:3]
    if len(found) > 1:
        return None, "UNKNOWN_conflict", ["%s=%s" % (p, "|".join(v[:2])) for p, v in sorted(found.items())]
    return None, "UNKNOWN_no_evidence", []


class T172(IT.Importer):
    def __init__(self, a):
        IT.Importer.__init__(self, a)
        self.stats.update({"groups": 0, "mapped": 0, "families": 0, "links": 0, "platform_inferred": 0, "platform_unknown": 0, "platform_declared": 0,
                           "description_updates": 0})
        self.out = {}

    # ------------------------------------------------------------------ the entry-set invariant
    def entryset(self, c):
        return [tuple(r) for r in c.execute(ENTRYSET_SQL).fetchall()]

    def write_tsv(self, name, rows):
        with open(os.path.join(self.a.out_dir, name), "w", encoding="utf-8", newline="\n") as f:
            for r in rows:
                f.write("\t".join(r) + "\n")

    def check_entryset(self, c, before):
        after = self.entryset(c)
        self.write_tsv("entryset-after.tsv", after)
        bs, as_ = set(before), set(after)
        added, removed = sorted(as_ - bs), sorted(bs - as_)
        verdict = {"schema": "t172-entryset/1", "stage": self.a.stage, "freeze": self.ident, "fixture": self.ident["fixture"], "before_rows": len(before), "after_rows": len(after), "added": [list(x) for x in added],
                   "removed": [list(x) for x in removed], "identical": before == after,
                   "before_sha256": hashlib.sha256(("\n".join("\t".join(r) for r in before) + "\n").encode()).hexdigest(),
                   "after_sha256": hashlib.sha256(("\n".join("\t".join(r) for r in after) + "\n").encode()).hexdigest()}
        with open(os.path.join(self.a.out_dir, "entryset.json"), "w", encoding="utf-8", newline="\n") as f:
            json.dump(verdict, f, sort_keys=True, indent=1)
            f.write("\n")
        if before != after:
            raise Refusal("t172_entry_set_changed", "reg_source_entries changed: %d added (%s), %d removed (%s)" % (
                len(added), "; ".join("|".join(x) for x in added[:5]), len(removed), "; ".join("|".join(x) for x in removed[:5])), 20)
        return verdict

    def freeze_identity(self, fj):
        """what the records below were produced from (11.4.115(F)): the freeze's head, listing hash, manifest hash; `fixture` is true unless the head is a full git sha"""
        head = fj.get("head")
        return {"head": head, "frozen_at": fj.get("frozen_at"), "listing_sha256": fj.get("listing_sha256"),
                "manifest_sha256": hashlib.sha256(open(fj["manifest"], "rb").read()).hexdigest(),
                "fixture": not (isinstance(head, str) and re.fullmatch(r"[0-9a-f]{40}", head) is not None)}

    # ------------------------------------------------------------------ helpers
    def mint_marker(self, c, marker):
        row = c.execute("SELECT atm_id FROM reg_ids WHERE minted_by=? AND mint_basis='import'", (marker,)).fetchone()
        if row:
            self.stats["resumed"] += 1
            return row[0]
        c.execute("INSERT INTO reg_ids(minted_by,mint_basis) VALUES (?, 'import')", (marker,))
        self.stats["minted"] += 1
        return c.execute("SELECT atm_id FROM reg_ids WHERE seq=last_insert_rowid()").fetchone()[0]

    # ------------------------------------------------------------------ stage qa
    def classify_qa(self, t):
        return IT.qa_cause_of(t["raw_status"], t["fm"], t["body"])

    def stage_qa(self, c, tickets):
        by_cause, others = {}, 0
        for t in sorted(tickets, key=lambda x: x["locator"]):
            k = self.classify_qa(t)
            if k is None:
                others += 1
                continue
            by_cause.setdefault(k, []).append(t)
        # refusals before any write
        bad = []
        for k, ts in by_cause.items():
            marker = "%s:qa:%s" % (ACTOR, k)
            cause_atm = c.execute("SELECT atm_id FROM reg_ids WHERE minted_by=? AND mint_basis='import'", (marker,)).fetchone()
            for t in ts:
                r = c.execute("SELECT atm_id FROM reg_source_map WHERE entry_id=?", (t["entry_id"],)).fetchone()
                if r and (not cause_atm or r[0] != cause_atm[0]):
                    bad.append(t["locator"])
        if bad:
            raise Refusal("qa_entry_individually_imported", "%d QA-instrument ticket(s) already mapped to an item that is not their cause item (import.sh ran first): %s - "
                          "nothing written. This state cannot be repaired by this tool: an id is never retracted and a reg_source_map row is never deleted. Recovery: restore the "
                          "pre-import backup named by the --backup-record of the import (scripts/register/backup_db.sh), then run --stage qa to completion, then import.sh; "
                          "a register that cannot be restored needs an owner decision (docs/scripts/import_t172.md, section Recovery)" % (len(bad), ", ".join(bad[:5])))
        for k, ts in sorted(by_cause.items()):
            comp = CAUSES[k][1]
            if not c.execute("SELECT 1 FROM reg_components WHERE component_id=?", (comp,)).fetchone():
                raise Refusal("component_not_registered", "cause %s names component %r, which is not a reg_components row - nothing written" % (k, comp))
        causes_rec = {}
        for k, ts in sorted(by_cause.items()):
            self.processed += 1
            atm = self.process_cause(c, k, ts)
            members = c.execute("SELECT count(*) FROM reg_source_map WHERE atm_id=?", (atm,)).fetchone()[0]
            causes_rec[k] = {"atm_id": atm, "component": CAUSES[k][1], "tickets_in_snapshot": len(ts), "tickets_mapped": members,
                             "locators": [t["locator"] for t in ts], "severity": c.execute("SELECT severity FROM reg_item_ext WHERE atm_id=?", (atm,)).fetchone()[0]}
            self.stats["groups"] += 1
        self.out = {"schema": "t172-qa-causes/1", "tickets_read": len(tickets), "qa_instrument_tickets": sum(len(v) for v in by_cause.values()),
                    "not_qa_instrument": others, "causes": causes_rec,
                    "cause_rules": [k for k, _ in CAUSE_RULES] + ["other_qa_instrument"], "no_reg_source_entries_row_inserted": True,
                    "freeze": self.ident, "fixture": self.ident["fixture"]}

    def process_cause(self, c, k, ts):
        label, comp = CAUSES[k]
        sevs = [(IT.sev_of(t["raw_severity"]), t) for t in ts]
        best = max(range(len(sevs)), key=lambda i: (SEV_RANK[sevs[i][0][0]], -i))
        (sev, how, defaulted), gm = sevs[best]
        if defaulted:
            self.stats["severity_defaulted"] += 1
        marker = "%s:qa:%s" % (ACTOR, k)
        atm = self.mint_marker(c, marker)
        self.hook("after_mint")
        if not c.execute("SELECT 1 FROM reg_item_ext WHERE atm_id=?", (atm,)).fetchone():
            c.execute("INSERT INTO reg_item_ext(atm_id,category,defect_layer,component_id,severity,severity_source,custody_basis,reverify_required,legacy_status) "
                      "VALUES (?,?,?,?,?,?,?,?,?)", (atm, "bug", "source", comp, sev, "QA-instrument cause %s: %s (ticket %s)" % (k, how, gm["locator"]), "machine_evidence", 0, None))
        self.hook("after_ext")
        if not c.execute("SELECT 1 FROM items WHERE atm_id=?", (atm,)).fetchone():
            body = ["QA-instrument defect (docs/12 section 11.3, QF-10), stated cause `%s`: %s." % (k, label),
                    "Legacy tickets closed with the statement that the QA tool, not the product, was wrong. That statement is an analysis, not evidence "
                    "(11.4.201 (7)(c), 11.4.238): a QA tool that mis-captures screens is itself the defect, and it is registered here against component %s." % comp,
                    FIX]
            sources = ["- Cause: %s" % k, "- Component: %s (docs/12 section 11.3)" % comp,
                       "- Tickets: every ticket of this cause is a source entry mapped to this item in reg_source_map (relation primary for the first locator, duplicate_of for the others); "
                       "list them with SELECT e.locator FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE m.atm_id='%s'" % atm,
                       "- Stated resolution (first ticket %s): %s" % (ts[0]["locator"], PL.cut_text(ts[0]["fm"].get("resolution", ""), 240)),
                       "- Type: Bug; status Queued; category bug (closed-set value) [DEFAULT - adjustable]"]
            desc, _cut = PL.assemble("\n".join(body), sources, "")
            title = PL.cut_text("[QA-instrument] %s" % label, 140)
            rc, out = self.engine(["add", "Bug", sev, "--db", self.a.db, "--id", atm, "--prefix", "CAT", "--title", title, "--description", desc, "--created-by", ACTOR])
            if rc != 0:
                self.engine_fail("add %s" % atm, out)
            self.stats["items_added"] += 1
        self.hook("after_add")
        self.hook("before_map")
        has_primary = c.execute("SELECT 1 FROM reg_source_map WHERE atm_id=? AND relation='primary'", (atm,)).fetchone() is not None
        has_gov = c.execute("SELECT 1 FROM reg_source_map WHERE atm_id=? AND severity_governs=1", (atm,)).fetchone() is not None
        for i, t in enumerate(ts):
            if c.execute("SELECT 1 FROM reg_source_map WHERE entry_id=?", (t["entry_id"],)).fetchone():
                continue
            rel = "primary" if not has_primary else "duplicate_of"
            gov = 1 if (not has_gov and t is gm) else 0
            c.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) VALUES (?,?,?,'manual_operator',?,?)", (t["entry_id"], atm, rel, gov, ACTOR))
            has_primary = has_primary or rel == "primary"
            has_gov = has_gov or gov == 1
            self.stats["mapped"] += 1
        if not has_gov:      # a later run added tickets only: the governing row must exist exactly once
            first = c.execute("SELECT entry_id FROM reg_source_map WHERE atm_id=? ORDER BY relation='primary' DESC, entry_id LIMIT 1", (atm,)).fetchone()[0]
            c.execute("UPDATE reg_source_map SET severity_governs=1 WHERE entry_id=?", (first,))
        return atm

    # ------------------------------------------------------------------ stage families
    def stage_families(self, c, tickets):
        unm = [t["locator"] for t in tickets if not t["mapped"]]
        if unm:
            raise Refusal("families_need_import", "%d ticket entries are not mapped yet (run import.sh first): %s - nothing written" % (len(unm), ", ".join(unm[:5])))
        rows = []
        qa_marker = ACTOR + ":qa:"
        for t in sorted(tickets, key=lambda x: x["locator"]):
            atm, minted_by = c.execute("SELECT m.atm_id, r.minted_by FROM reg_source_map m JOIN reg_ids r USING(atm_id) WHERE m.entry_id=?", (t["entry_id"],)).fetchone()
            if minted_by.startswith(qa_marker):
                continue                      # the QA-instrument tickets live in their cause item
            plat, basis, ev = infer_platform(t["fm"], t["body"])
            rows.append({"t": t, "atm": atm, "plat": plat, "basis": basis, "ev": ev,
                         "slug": slug_of(t["title"]), "screen": screen_of(t["fm"].get("screen"))})
        # platform inference record + description line
        pi = []
        for r in rows:
            t = r["t"]
            pi.append({"locator": t["locator"], "entry_id": t["entry_id"], "atm_id": r["atm"], "declared": (t["fm"].get("platform") or "").strip() or None,
                       "platform": r["plat"] or "UNKNOWN", "basis": r["basis"], "evidence": r["ev"], "marker": "[DEFAULT - adjustable]" if r["basis"] == "inferred" else None})
            if r["basis"] == "declared":
                self.stats["platform_declared"] += 1
            elif r["basis"] == "inferred":
                self.stats["platform_inferred"] += 1
                self.write_inference(c, r)
            else:
                self.stats["platform_unknown"] += 1
        # families
        fam = {}
        for r in rows:
            fam.setdefault((r["slug"], r["plat"] or "UNKNOWN", r["screen"]), []).append(r)
        by_slug = {}
        for k, ms in fam.items():
            by_slug.setdefault(k[0], []).append(k)
        families, distinct = [], []
        no_scope = 0
        for k in sorted(fam):
            ms = fam[k]
            if len(ms) < 2 or not k[0]:
                continue
            if not k[2]:
                no_scope += len(ms)      # an empty scope stays DISTINCT (doc04 section 8 exact scope, T169 (S)): no link on a title alone
                continue
            head = ms[0]
            links = []
            for m in ms[1:]:
                links.append(self.link(c, head, m))
            families.append({"slug": k[0], "platform_class": k[1], "screen": k[2], "head": head["t"]["locator"], "head_atm_id": head["atm"],
                             "members": [{"locator": m["t"]["locator"], "atm_id": m["atm"], "verdict": "UNDECIDED", "link_id": l} for m, l in zip(ms[1:], links)],
                             "verdict_basis": "doc04 section 8 autonomous default: mint-with-link (UNDECIDED), never SAME_DEFECT without the calibrated matcher and the owner"})
            self.stats["families"] += 1
        for slug, keys in sorted(by_slug.items()):
            if slug and len([k for k in keys if k[2]]) > 1:
                distinct.append({"slug": slug, "keys": [{"platform_class": k[1], "screen": k[2], "members": [x["t"]["locator"] for x in fam[k]]} for k in sorted(keys)],
                                 "verdict": "DISTINCT (same slug, different platform class or screen: one case per member, never merged)"})
        self.out = {"schema": "t172-families/1", "tickets_read": len(tickets), "tickets_in_families_stage": len(rows), "families": families, "distinct_same_slug": distinct,
                    "freeze": self.ident, "fixture": self.ident["fixture"],
                    "key": "(normalised title slug, platform class, screen)", "members_linked": sum(len(f["members"]) for f in families),
                    "members_without_screen_not_linked": no_scope}
        self.out_pi = {"schema": "t172-platform-inference/1", "freeze": self.ident, "fixture": self.ident["fixture"], "rows": pi,
                       "counts": {"declared": self.stats["platform_declared"], "inferred": self.stats["platform_inferred"], "unknown": self.stats["platform_unknown"]},
                       "rule": "docs/12 section 11.4: inference is a draft marked [DEFAULT - adjustable]; an unresolvable platform stays UNKNOWN, never guessed",
                       "rules": [p for p, _ in PLATFORM_RULES]}

    def link(self, c, head, m):
        row = c.execute("SELECT link_id FROM reg_recurrence_links WHERE reported_entry_id=? AND intake_path='import' AND new_atm_id=?", (m["t"]["entry_id"], m["atm"])).fetchone()
        if row:
            return row[0]
        hl = c.execute("SELECT IFNULL(max(log_id),0) FROM reg_status_log WHERE atm_id=?", (head["atm"],)).fetchone()[0]
        c.execute("INSERT INTO reg_recurrence_links(head_atm_id,reported_entry_id,intake_path,verdict,match_basis,new_atm_id,reopened,head_log_id,decided_by) "
                  "VALUES (?,?,'import','UNDECIDED','normalised(subject,scope)',?,0,?,?)", (head["atm"], m["t"]["entry_id"], m["atm"], hl, ACTOR))
        self.stats["links"] += 1
        return c.execute("SELECT last_insert_rowid()").fetchone()[0]

    def write_inference(self, c, r):
        line = "- Platform (inferred from %s, [DEFAULT - adjustable]): %s" % ("; ".join(r["ev"][:2]), r["plat"])
        d, st = c.execute("SELECT description, status FROM items WHERE atm_id=?", (r["atm"],)).fetchone()
        loc = "Fixed" if "Fixed" in (st or "") else "Issues"
        if "- Platform (inferred" in d:
            return
        block = d.rstrip() + "\n" + line
        if len(block.encode("utf-8")) > IT.DESC_CAP:
            return          # the line is in platform_inference.json; the description is full (honest: recorded there only)
        rc, out = self.engine(["update", "--id", r["atm"], "--db", self.a.db, "--description", block, "--location", loc])
        if rc != 0:
            self.engine_fail("update %s" % r["atm"], out)
        self.stats["description_updates"] += 1

    # ------------------------------------------------------------------ run
    def run(self):
        a = self.a
        if not os.path.isfile(a.db):
            raise Refusal("db_missing", "no database at %s (apply_ext.sh creates a fresh engine DB)" % a.db)
        if not (os.path.isfile(a.engine) and os.access(a.engine, os.X_OK)):
            raise Refusal("engine_missing", "engine binary %s is not an executable file" % a.engine)
        if a.stage not in ("qa", "families"):
            raise Refusal("stage_unsupported", "--stage qa|families")
        if not a.out_dir:
            raise Refusal("out_dir_required", "--out-dir DIR names where the records are written")
        os.makedirs(a.out_dir, exist_ok=True)
        fj = self.read_freeze()
        self.check_snapshot(fj)
        self.ident = self.freeze_identity(fj)
        self.load_map()
        dbf = os.open(a.db, os.O_RDONLY)
        try:
            fcntl.flock(dbf, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            raise Refusal("import_already_running", "another importer holds %s" % a.db)
        self.check_backup()
        self.check_disk()
        c = self.connect()
        self.check_ddl(c)
        if not {"reg_recurrence_links", "reg_components", "reg_status_log"} <= {r[0] for r in c.execute("SELECT name FROM sqlite_master")}:
            raise Refusal("register_ddl_absent", "reg_recurrence_links, reg_components or reg_status_log missing (apply_ext.sh not applied)")
        before = self.entryset(c)
        self.write_tsv("entryset-before.tsv", before)
        tickets = self.read_tickets(c, fj["snapshot"], ["issue_file"])
        self.stats["entries_selected"] = len(tickets)
        if a.dry_run:
            IT.log("dry run: %d ticket entries read" % len(tickets))
            return self.finish(c, before, dry=True)
        try:
            (self.stage_qa if a.stage == "qa" else self.stage_families)(c, tickets)
        except sqlite3.Error as e:
            cl = IT.classify_db_error(str(e))
            raise Refusal(cl or "db_error", "%s" % e, 21 if cl else 1)
        return self.finish(c, before, dry=False)

    def finish(self, c, before, dry):
        a = self.a
        verdict = self.check_entryset(c, before)
        res = {"schema": "import-t172-run/1", "stage": a.stage, "dry_run": dry, "stats": self.stats, "entryset_identical": verdict["identical"]}
        if not dry:
            rc, out = self.engine(["validate", "--db", a.db])
            res["engine_validate"] = {"rc": rc, "text": out.strip()[:300]}
            if rc != 0:
                raise Refusal("validate_failed", out.strip()[:300], 1)
            fk = c.execute("PRAGMA foreign_key_check").fetchall()
            ic = c.execute("PRAGMA integrity_check").fetchone()[0]
            res["foreign_key_check_rows"], res["integrity_check"] = len(fk), ic
            if fk or ic != "ok":
                raise Refusal("integrity_failed", "foreign_key_check rows=%d integrity_check=%s" % (len(fk), ic), 1)
            if a.stage == "qa":
                self.write_json("qa_causes.json", self.out)
            else:
                self.write_json("families.json", self.out)
                self.write_json("platform_inference.json", self.out_pi)
        if a.report:
            with open(a.report, "w", encoding="utf-8", newline="\n") as f:
                json.dump(res, f, sort_keys=True, indent=1)
                f.write("\n")
        IT.log("ok %s %s" % (a.stage, json.dumps(self.stats, sort_keys=True)))
        return 0

    def write_json(self, name, obj):
        with open(os.path.join(self.a.out_dir, name), "w", encoding="utf-8", newline="\n") as f:
            json.dump(obj, f, sort_keys=True, indent=1)
            f.write("\n")


def main(argv):
    ap = argparse.ArgumentParser(prog="import_t172.sh", add_help=True)
    ap.add_argument("--stage", required=True)
    ap.add_argument("--db", required=True)
    ap.add_argument("--freeze-json", dest="freeze_json", required=True)
    ap.add_argument("--engine", required=True)
    ap.add_argument("--out-dir", dest="out_dir")
    ap.add_argument("--category-map", dest="category_map", default=os.path.join(HERE, "category_map.yaml"))
    ap.add_argument("--backup-record", dest="backup_record")
    ap.add_argument("--progress-log", dest="progress_log")
    ap.add_argument("--report")
    ap.add_argument("--busy-timeout-ms", dest="busy_timeout_ms", type=int, default=5000)
    ap.add_argument("--dry-run", dest="dry_run", action="store_true")
    try:
        a = ap.parse_args(argv)
    except SystemExit as e:
        return 2 if e.code else 0
    a.kinds, a.category_out = "issue_file", None
    try:
        return T172(a).run()
    except Refusal as r:
        sys.stderr.write("import_t172: REFUSED reason=%s %s\n" % (r.reason, r.detail))
        return r.code
    except sqlite3.Error as e:
        cl = IT.classify_db_error(str(e))
        sys.stderr.write("import_t172: REFUSED reason=%s %s\n" % (cl or "db_error", e))
        return 21 if cl else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
