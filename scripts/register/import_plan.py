#!/usr/bin/env python3
"""import_plan.py - the engine of scripts/register/import_plan.sh (T170 plan-document seeds, T171 innovation entries; WP-20; doc04 sections 8, 9.3, 12.2; docs/21 sections 9.1, 9.3, 9.5).

Imports the Stage 0 entries of the two plan-document sources into the findings register with the SAME single-writer mechanics as the T168 ticket importer
(scripts/register/import_tickets.py, whose Importer base class this file extends: freeze check, pre-op backup record, free-space precondition, single-flight lock,
`reg_ids` mint, `reg_item_ext` BEFORE `$WI add --id`, real engine binary, `reg_source_map`, final validate / foreign_key_check / integrity_check):

  --class seeds       S-26 "plan-document seeds" (docs/21 section 9.1). 254 entries (rev 8). One item per seed, except that each docs/21 section 9.3 family becomes ONE
                      item: its first member is the head (`primary`), every other member is `duplicate_of` the head (`manual_operator`: the folding is the plan's own
                      table). A member marked `(part)` is only partly folded and stays its own item. A seed whose id an ANOTHER source already imported (a
                      `reg_source_entries.legacy_id` equal to the seed id or to `docNN:id`, mapped to an item) is linked to that item, never minted twice (11.4.214).
                      Items are Type Task [DEFAULT - adjustable], status Queued, category = the interim category of category_map.yaml (the final category is BLOCKED-ON
                      ODG-17, applied by T223), severity from the source document's own label (IC-01 normalisation, the raw label kept in `severity_source`; an unrated seed
                      is defaulted medium [DEFAULT - adjustable]). They are CANDIDATES until a detector produces machine evidence (FR-007). No `reg_findings` row is written.
                      Bound: items <= 254 - folded members + families (docs/21: at most 198 on rev 8); a larger result is refused and nothing is written.
  --class innovation  S-27 "innovation entries" (docs/21 section 9.5). 34 entries (28 PROPOSAL, 6 PROPOSAL parts). One Feature item per entry, status Queued, category `gap`,
                      mint_basis import; the description quotes the docs/18 row with its Touches, its evidence label under the docs/18 section 2.1 mapping (EMERGING -> PLAUSIBLE,
                      secondary -> `single_source`, `n/a` with the repository citation), its ranked game changer and falsifiable experiment where one exists, a **Sources** block and
                      the line `**Disposition (ODG-39):** pending`. No `reg_findings` row (a proposal is not a finding, FR-008, SC-003); nothing is implemented or closed here.
Reads the plan documents ONLY from the frozen snapshot named by --freeze-json (entry check scripts/register/check_freeze_snapshot.sh first, `freeze_snapshot_moved` stops it).
`reg_item_ext.component_id` stays NULL (T168: the component an imported item names lives in its description only).

Usage: import_plan.py --class seeds|innovation --db DB --freeze-json F --engine WI [--category-map F] [--backup-record F] [--progress-log F] [--report F]
         [--reconciliation F] [--busy-timeout-ms N] [--dry-run]
Exit: 0 ok | 2 usage | 20 refusal (`import_plan: REFUSED reason=<code> ...`, nothing written) | 21 a write failed and the run is resumable | 1 engine or invariant failure.
Idempotent and resumable exactly as the ticket importer (marker `register-import-plan:e<entry_id>` in reg_ids.minted_by).
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
EN = IT.load_enumerator()
Refusal = IT.Refusal
ACTOR = "register-import-plan"
SEED_SRC = "plan-document seeds (docs/21 section 9.1)"
INNO_SRC = "innovation entries (docs/21 section 9.5)"
SEV_RANK = {"cosmetic": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}
WORD = {"critical": "critical", "high": "high", "medium": "medium", "med": "medium", "low": "low", "info": "cosmetic", "cosmetic": "cosmetic"}


def norm_severity(label):
    """-> (severity, how, defaulted). A compound label (`Low-Medium`, `Med-High`) takes the HIGHER word; `if confirmed` and `UNCONFIRMED` stay in the text of `how`."""
    if not label:
        return "medium", "no severity in the source row; defaulted medium [DEFAULT - adjustable]", True, False
    low = label.lower()
    words = re.findall(r"\b(critical|high|medium|med|low|info|cosmetic)\b", low.split("(")[0])
    if not words or low.startswith("unknown"):
        return "medium", "source label %r names no severity word; defaulted medium [DEFAULT - adjustable]" % label[:60], True, False
    first_two = [WORD[w] for w in words[:2]] if re.match(r"^\W*[a-z]+[ -]+(critical|high|medium|med|low)\b", low) else [WORD[words[0]]]
    sev = max(first_two, key=lambda x: SEV_RANK[x])
    cand = " (candidate: %s)" % ("if confirmed" if "if confirmed" in low else "unconfirmed") if ("if confirmed" in low or "unconfirmed" in low) else ""
    return sev, "source label %r normalised to %s%s" % (label[:60], sev, cand), False, bool(cand)


def evidence_label(raw):
    """docs/18 evidence label -> (register value, single_source flag) under the docs/18 section 2.1 mapping"""
    r = (raw or "").strip()
    if not r or r.lower().startswith("n/a"):
        return "n/a", False
    first = re.match(r"^(PROVEN|EMERGING|SPECULATIVE|UNCONFIRMED)\b", r)
    val = {"PROVEN": "PROVEN", "EMERGING": "PLAUSIBLE", "SPECULATIVE": "SPECULATIVE", "UNCONFIRMED": "UNCONFIRMED"}[first.group(1)] if first else "UNCONFIRMED"
    return val, "secondary" in r.lower()


def cut_text(s, n):
    s = re.sub(r"\s+", " ", (s or "")).strip()
    return s if len(s.encode("utf-8")) <= n else IT.cut_utf8(s.encode("utf-8"), n).decode("utf-8").rstrip() + " ..."


def assemble(body, sources, tail, cap=IT.DESC_CAP):
    """description <= cap bytes: body (cut on a UTF-8 boundary and marked) + the Sources block + the fixed tail (never cut)"""
    block = "**Sources**\n" + "\n".join(sources) + (("\n\n" + tail) if tail else "")

    def mk(b, note):
        return ((b + "\n\n") if b else "") + block + (("\n" + note) if note else "")
    whole = mk(body.strip(), "")
    if len(whole.encode("utf-8")) <= cap:
        return whole, False
    raw = body.strip().encode("utf-8")
    n = min(len(raw), cap)
    while n >= 0:
        kept = IT.cut_utf8(raw, n).decode("utf-8").rstrip()
        d = mk(kept, "- Note: the text above is cut at %d of %d bytes; the full text is in the plan document named above." % (len(kept.encode("utf-8")), len(raw)))
        over = len(d.encode("utf-8")) - cap
        if over <= 0:
            return d, True
        n -= over
    raise Refusal("sources_block_too_large", "the Sources block alone exceeds %d bytes" % cap)


class PlanImporter(IT.Importer):
    def __init__(self, a):
        IT.Importer.__init__(self, a)
        self.stats.update({"groups": 0, "families": 0, "linked_existing": 0, "folded_members": 0, "located": 0, "unlocated": 0})
        self.rec = {}

    # ------------------------------------------------------------------ the plan, from the frozen snapshot (one parser: the enumerator's)
    def snapshot_seeds(self, fj):
        snap = EN.Snapshot(fj["snapshot"])
        if EN.DOC21 not in snap.files:
            raise Refusal("plan_document_missing", "%s is not in the frozen snapshot" % EN.DOC21)
        lines = EN.lines_of(snap, EN.DOC21)
        try:
            rows = EN.parse_seed_table(lines)
            fams = EN.parse_family_table(lines, None)
            inno = EN.parse_innovation_table(lines)
        except EN.Refusal as e:
            raise Refusal(e.reason, e.detail)
        files = EN.doc_files(snap.files)
        seeds = EN.seed_entries(lambda q: EN.lines_of(snap, q), files, rows)
        return snap, lines, rows, fams, inno, files, seeds

    def verify_entries(self, c, snap, srcloc, expected):
        """the register's entries of this source must equal what the snapshot yields NOW (locator and sha); else the import is refused with nothing written"""
        got = {r[0]: (r[1], r[2]) for r in c.execute(
            "SELECT e.locator, e.entry_sha256, e.entry_id FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=?", (srcloc,))}
        missing = sorted(set(expected) - set(got))
        extra = sorted(set(got) - set(expected))
        changed = sorted(l for l in set(got) & set(expected) if got[l][0] != expected[l])
        if missing or extra or changed:
            raise Refusal("entries_out_of_date", "source %r: %d expected entries missing from the register (%s), %d register entries the snapshot does not yield (%s), %d changed (%s); "
                          "re-run enumerate_sources.py over this snapshot - nothing written" % (srcloc, len(missing), missing[:3], len(extra), extra[:3], len(changed), changed[:3]))
        return {l: got[l][1] for l in got}      # locator -> entry_id

    # ------------------------------------------------------------------ one group (a head item and the entries it carries)
    def mint_marker(self, c, marker):
        row = c.execute("SELECT atm_id FROM reg_ids WHERE minted_by=? AND mint_basis='import'", (marker,)).fetchone()
        if row:
            self.stats["resumed"] += 1
            return row[0]
        c.execute("INSERT INTO reg_ids(minted_by,mint_basis) VALUES (?, 'import')", (marker,))
        self.stats["minted"] += 1
        return c.execute("SELECT atm_id FROM reg_ids WHERE seq=last_insert_rowid()").fetchone()[0]

    def process_group(self, c, g):
        """g: {head_entry, typ, sev, sev_src, cat, title, desc, members: [(entry_id, relation, match_basis, severity_governs)]}"""
        atm = self.mint_marker(c, "%s:e%d" % (ACTOR, g["head_entry"]))
        self.hook("after_mint")
        if not c.execute("SELECT 1 FROM reg_item_ext WHERE atm_id=?", (atm,)).fetchone():
            c.execute("INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,severity_source,custody_basis,reverify_required,legacy_status) VALUES (?,?,?,?,?,?,?,?)",
                      (atm, g["cat"], "source", g["sev"], g["sev_src"], "machine_evidence", 0, None))
        self.hook("after_ext")
        if not c.execute("SELECT 1 FROM items WHERE atm_id=?", (atm,)).fetchone():
            rc, out = self.engine(["add", g["typ"], g["sev"], "--db", self.a.db, "--id", atm, "--prefix", "CAT", "--title", g["title"], "--description", g["desc"], "--created-by", ACTOR])
            if rc != 0:
                self.engine_fail("add %s" % atm, out)
            self.stats["items_added"] += 1
        self.hook("after_add")
        self.stats["queued"] += 1
        self.hook("after_close")
        self.hook("before_map")
        for (eid, rel, basis, gov) in g["members"]:
            if not c.execute("SELECT 1 FROM reg_source_map WHERE entry_id=?", (eid,)).fetchone():
                c.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) VALUES (?,?,?,?,?,?)", (eid, atm, rel, basis, gov, ACTOR))
                if rel == "duplicate_of":
                    self.stats["folded_members"] += 1
        return atm

    def link_existing(self, c, eid, atm, basis):
        if not c.execute("SELECT 1 FROM reg_source_map WHERE entry_id=?", (eid,)).fetchone():
            c.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) VALUES (?,?,'duplicate_of',?,0,?)", (eid, atm, basis, ACTOR))
            self.stats["linked_existing"] += 1

    # ------------------------------------------------------------------ seeds (T170)
    def plan_seeds(self, c, fj):
        snap, lines, rows, fams, inno, files, seeds = self.snapshot_seeds(fj)
        d18, d18c = {}, {}
        if files.get("doc18"):
            d18 = self.doc18_routing(EN.lines_of(snap, files["doc18"]))
            d18c = EN.parse_doc18_candidates(EN.lines_of(snap, files["doc18"]))
        expected = {}
        for sd in seeds:
            loc = "%s#seed:%s:%s" % (EN.DOC21, sd["doc"], sd["id"])
            basis = "%s\0%s\0%s\0%s" % (sd["doc"], sd["id"], lines[sd["row_line"] - 1], sd["src_text"] or "")
            expected[loc] = EN.sha_text(basis)
        emap = self.verify_entries(c, snap, SEED_SRC, expected)          # locator -> entry_id
        by_key = {(s["doc"], s["id"]): s for s in seeds}
        for s in seeds:
            s["entry_id"] = emap["%s#seed:%s:%s" % (EN.DOC21, s["doc"], s["id"])]
            s["locator"] = "%s#seed:%s:%s" % (EN.DOC21, s["doc"], s["id"])
            s["sha"] = expected[s["locator"]]
        # families: a member that is only partly folded stays its own item; every family member must be a seed
        fam_of, fam_list = {}, []
        for f in fams:
            ms = []
            for (doc, i, part) in f["members"]:
                if (doc, i) not in by_key:
                    raise Refusal("family_member_not_a_seed", "docs/21 line %d: family %r names %s %s which is not a section 9.1 seed" % (f["line"], f["family"][:40], doc, i))
                if part:
                    continue
                if (doc, i) in fam_of:
                    raise Refusal("family_member_twice", "docs/21: %s %s is a member of two families" % (doc, i))
                ms.append((doc, i))
            for k in ms:
                fam_of[k] = len(fam_list)
            fam_list.append({"name": f["family"], "line": f["line"], "members": ms, "partly": [(d, i) for (d, i, p) in f["members"] if p]})
        folded = sum(len(f["members"]) - 1 for f in fam_list if f["members"])
        bound = len(seeds) - folded
        # existing items imported by ANOTHER source under the same legacy id (11.4.214)
        existing = {}
        for lid, atm in c.execute("SELECT e.legacy_id, m.atm_id FROM reg_source_entries e JOIN reg_sources s USING(source_id) JOIN reg_source_map m USING(entry_id) "
                                  "WHERE s.locator NOT IN (?,?) AND e.legacy_id IS NOT NULL ORDER BY e.entry_id", (SEED_SRC, INNO_SRC)):
            existing.setdefault(lid, atm)
        groups, links = [], []
        done_fam = set()
        order = [(s["doc"], s["id"]) for s in seeds]
        for key in order:
            s = by_key[key]
            ex = existing.get(key[1]) or existing.get("%s:%s" % key)
            fi = fam_of.get(key)
            if fi is not None:
                if fi in done_fam:
                    continue
                done_fam.add(fi)
                fam = fam_list[fi]
                mem = [by_key[k] for k in fam["members"]]
                exm = next((existing.get(k[1]) or existing.get("%s:%s" % k) for k in fam["members"] if existing.get(k[1]) or existing.get("%s:%s" % k)), None)
                if exm:
                    for m in mem:
                        links.append((m["entry_id"], exm, "ticket"))
                    continue
                groups.append(self.seed_group(mem, fam, d18, d18c))
            else:
                if ex:
                    links.append((s["entry_id"], ex, "ticket"))
                    continue
                groups.append(self.seed_group([s], None, d18, d18c))
        self.rec = {"schema": "seed-reconciliation/1", "class": "S-26", "entries": len(seeds), "rated": sum(1 for s in seeds if s["rated"]), "unrated": sum(1 for s in seeds if not s["rated"]),
                    "families": len(fam_list), "family_members": sum(len(f["members"]) + len(f["partly"]) for f in fam_list), "fully_folded_members": sum(len(f["members"]) for f in fam_list),
                    "partly_folded_members_kept_separate": [("%s:%s" % p) for f in fam_list for p in f["partly"]], "folded_non_head_members": folded,
                    "items_upper_bound": bound, "items_planned": len(groups), "linked_to_items_of_other_sources": len(links), "located_in_source_document": sum(1 for s in seeds if s["located"]),
                    "unlocated_seed_ids": [("%s:%s" % (s["doc"], s["id"])) for s in seeds if not s["located"]],
                    "doc21_bound_rev8": 198}
        if len(groups) > bound:
            raise Refusal("item_bound_exceeded", "%d items planned, the docs/21 bound is %d (%d seeds - %d folded) - nothing written" % (len(groups), bound, len(seeds), folded))
        self.links = links
        return groups, seeds, fam_list

    def doc18_routing(self, lines18):
        """docs/18 section 1.1 routing table -> {candidate id: {"kind", "label", "destination"}}"""
        sec = EN.md_section(lines18, r"^### 1\.1\s")
        out = {}
        if sec:
            for n, c in EN.table_rows(lines18, *sec):
                if len(c) >= 4 and re.match(r"^T\d+-[A-F]$", c[0]):
                    out[c[0]] = {"kind": c[1], "label": c[2], "destination": c[3], "line": n}
        return out

    def seed_group(self, mem, fam, d18, d18c):
        head = mem[0]
        # severity: the HIGHEST of the members; the member that gives it governs (exactly one severity_governs=1 row per item, doc04 9.4)
        sevs = [(norm_severity(m["severity"]), m) for m in mem]
        best = max(range(len(sevs)), key=lambda i: (SEV_RANK[sevs[i][0][0]], not sevs[i][0][3], -i))      # highest word; at a tie a firm label beats a candidate one; then the first listed
        (sev, how, defaulted, _cand), gm = sevs[best]
        if defaulted:
            self.stats["severity_defaulted"] += 1
        title = cut_text("[%s %s] %s" % (head["doc"], head["id"], head["title"]), 140)
        body = ["Seed %s %s of docs/21 section 9.1 (%s): a CANDIDATE, not a confirmed defect, until a detector or audit package produces machine evidence (FR-007)." % (head["doc"], head["id"], cut_text(head["app"], 70))]
        if head["located"]:
            body.append("Source text (%s:L%d): %s" % (head["src_path"], head["src_line"], cut_text(head["src_text"], 650)))
            self.stats["located"] += 1
        else:
            body.append("Source text NOT located in %s by the leading-id rule (honest gap); the docs/21 row says: %s" % (head["doc"], cut_text("%s (%s)" % (head["app"], head["source"]), 200)))
            self.stats["unlocated"] += 1
        if head["doc"] == "doc18":
            key = head["id"].replace("(R-ADJ)", "")
            r = d18.get(key)
            c = d18c.get(key)
            if r:
                body.append("Routing (docs/18 section 1.1): %s. The plan document and fix work package named here decide the fix and its test." % cut_text(r["destination"], 300))
            if c:
                body.append("docs/18 candidate (%s): %s" % (key, cut_text(c["cells"].get("Candidate", ""), 300)))
        if fam:
            others = ["%s %s" % m for m in fam["members"][1:]]
            body.append("Family (docs/21 section 9.3) %s: this item carries the members %s; each is linked duplicate_of. Raw severities: %s." % (
                cut_text(fam["name"], 80), ", ".join(others) or "none", "; ".join("%s %s=%s" % (m["doc"], m["id"], m["severity"] or "unrated") for m in mem)))
        sources = ["- File: %s" % mem[0]["locator"], "- sha256: %s" % mem[0]["sha"],
                   "- Plan document: %s%s" % (head["src_path"] or EN.DOC21, (":L%d" % head["src_line"]) if head["src_line"] else " (seed row docs/21:L%d)" % head["row_line"]),
                   "- Seed id: %s %s%s" % (head["doc"], head["id"], " [family head of %d]" % len(mem) if fam else ""),
                   "- Raw severity: %s (%s)" % (gm["severity"] or "none", how),
                   "- Type: Task [DEFAULT - adjustable]; category: interim, not final (BLOCKED-ON ODG-17)"]
        desc, cut = assemble("\n".join(body), sources, "")
        members = []
        for i, m in enumerate(mem):
            members.append((m["entry_id"], "primary" if i == 0 else "duplicate_of", "manual_operator" if (fam and len(mem) > 1) else "import_1to1", 1 if m is gm else 0))
        if not any(x[3] for x in members):
            members[0] = members[0][:3] + (1,)
        if fam:
            self.stats["families"] += 1
        return {"head_entry": head["entry_id"], "typ": "Task", "sev": sev, "sev_src": "seed %s %s: %s" % (gm["doc"], gm["id"], how), "cat": self.interim, "title": title, "desc": desc, "members": members,
                "seed_ids": ["%s:%s" % (m["doc"], m["id"]) for m in mem], "cut": cut}

    # ------------------------------------------------------------------ innovation entries (T171)
    def plan_innovation(self, c, fj):
        snap, lines, rows, fams, inno, files, seeds = self.snapshot_seeds(fj)
        if not files.get("doc18"):
            raise Refusal("plan_document_missing", "docs/18 is not in the frozen snapshot")
        l18 = EN.lines_of(snap, files["doc18"])
        cand = EN.parse_doc18_candidates(l18)
        routing = self.doc18_routing(l18)
        exps = self.doc18_experiments(l18)
        ranks = self.doc18_ranks(l18)
        expected = {}
        for e in inno:
            cc = cand.get(e["id"])
            basis = "%s\0%s\0%s\0%s" % (e["id"], e["kind"], lines[e["line"] - 1], json.dumps(cc["cells"], sort_keys=True) if cc else "")
            expected["%s#innovation:%s%s" % (EN.DOC21, e["id"], ":PROPOSAL-part" if e["kind"] == "PROPOSAL part" else "")] = EN.sha_text(basis)
        emap = self.verify_entries(c, snap, INNO_SRC, expected)
        groups = []
        for e in inno:
            loc = "%s#innovation:%s%s" % (EN.DOC21, e["id"], ":PROPOSAL-part" if e["kind"] == "PROPOSAL part" else "")
            cc = cand.get(e["id"])
            if not cc:
                raise Refusal("candidate_row_missing", "docs/18 has no candidate row for %s" % e["id"])
            cells = cc["cells"]
            label_raw = cells.get("Evidence label (finding)", "")
            val, single = evidence_label(label_raw)
            rk = re.search(r"rank\s+(\d+)", e["rank"])
            body = ["Innovation %s of docs/21 section 9.5 (%s): a PROPOSAL for a LATER feature, not a finding, defect or task of feature 001; it is not implemented here (docs/18 section 1)." % (e["id"], e["kind"]),
                    "docs/18 row (line %d): %s" % (cc["line"], cut_text(cells.get("Candidate", ""), 520)),
                    "Touches: %s; effort %s, impact %s, risk %s." % (cut_text(cells.get("Touches", ""), 80), cells.get("Effort", "?"), cells.get("Impact", "?"), cells.get("Risk", "?")),
                    "Evidence label: %s -> register value %s%s%s (docs/18 section 2.1)." % (cut_text(label_raw, 120), val, " + single_source" if single else "",
                                                                                            " (repository citation: the docs/18 row itself)" if val == "n/a" else "")]
            if rk and rk.group(1) in ranks:
                body.append("Ranked game changer (docs/18 section 13, rank %s): %s." % (rk.group(1), cut_text(ranks[rk.group(1)], 150)))
                if rk.group(1) in exps:
                    body.append("Falsifiable experiment (docs/18 section 13.1, %s): %s" % (exps[rk.group(1)]["name"], cut_text(exps[rk.group(1)]["text"], 260)))
            sources = ["- File: %s" % loc, "- sha256: %s" % expected[loc], "- docs/18 row: %s:L%d" % (EN.DOC18, cc["line"]), "- Source entry: S-27 %s" % loc,
                       "- docs/21 section 9.5 entry: %s (line %d)" % (e["id"], e["line"]), "- Type: Feature; status Queued; category gap (closed-set value closest to a proposal)"]
            desc, cut = assemble("\n".join(body), sources, "**Disposition (ODG-39):** pending")
            title = cut_text("[doc18 %s] %s" % (e["id"], cells.get("Candidate", e["text"])), 140)
            groups.append({"head_entry": emap[loc], "typ": "Feature", "sev": "low", "sev_src": "Feature item (a proposal, not a defect): severity not applicable, recorded low [DEFAULT - adjustable]",
                           "cat": "gap", "title": title, "desc": desc, "members": [(emap[loc], "primary", "import_1to1", 1)], "doc18_id": e["id"], "kind": e["kind"],
                           "label": val, "single_source": single, "rank": rk.group(1) if rk else None, "cut": cut})
        # reconciliation of the 47 candidates: R-ADJ only through S-26, PROPOSAL only through S-27, mixed through both
        s26 = {re.sub(r"\(R-ADJ\)$", "", s["id"]) for s in seeds if s["doc"] == "doc18"}
        s27 = {e["id"] for e in inno}
        cov = {"S-26 only": [], "S-27 only": [], "both": [], "neither": []}
        wrong = []
        for cid, cc in sorted(cand.items()):
            a, b = cid in s26, cid in s27
            cov["both" if a and b else "S-26 only" if a else "S-27 only" if b else "neither"].append(cid)
            want = {"R-ADJ": (True, False), "PROPOSAL": (False, True), "mixed": (True, True)}[cc["tag_class"]] if cc["tag_class"] != "none" else None
            if want != (a, b):
                wrong.append({"id": cid, "tag_class": cc["tag_class"], "in_S26": a, "in_S27": b})
        self.rec = {"schema": "innovation-reconciliation/1", "class": "S-27", "candidates_read": len(cand), "entries": len(inno),
                    "tag_classes": {k: sum(1 for v in cand.values() if v["tag_class"] == k) for k in ("PROPOSAL", "R-ADJ", "mixed", "none")},
                    "coverage_counts": {k: len(v) for k, v in cov.items()}, "coverage": cov, "coverage_mismatches": wrong,
                    "feature_items_planned": len(groups), "by_kind": {"PROPOSAL": sum(1 for g in groups if g["kind"] == "PROPOSAL"), "PROPOSAL part": sum(1 for g in groups if g["kind"] == "PROPOSAL part")},
                    "by_evidence_label": {k: sum(1 for g in groups if g["label"] == k) for k in sorted({g["label"] for g in groups})}, "single_source": sum(1 for g in groups if g["single_source"]),
                    "with_ranked_game_changer": sum(1 for g in groups if g["rank"]), "no_reg_findings_row": True}
        return groups, inno, cand

    def doc18_ranks(self, l18):
        sec = EN.md_section(l18, r"^## 13\.\s")
        out = {}
        if sec:
            for n, c in EN.table_rows(l18, *sec):
                if len(c) >= 3 and re.match(r"^\d+$", c[0]):
                    out[c[0]] = c[1]
        return out

    def doc18_experiments(self, l18):
        out = {}
        sec = EN.md_section(l18, r"^### 13\.1\s")
        if sec:
            a, b = sec
            i = a
            while i < b:
                m = re.match(r"^\*\*(E\d+) - ([^*]*?)\(rank (\d+)\)\.\*\*\s*$", l18[i])
                if m:
                    j = i + 1
                    parts = []
                    while j < b and l18[j].strip() and not l18[j].startswith("**E"):
                        if l18[j].startswith("- Hypothesis") or l18[j].startswith("- Pass"):
                            parts.append(l18[j].lstrip("- ").strip())
                        j += 1
                    out[m.group(3)] = {"name": m.group(1) + " " + m.group(2).strip(), "text": " ".join(parts)}
                i += 1
        return out

    # ------------------------------------------------------------------ run
    def run(self):
        a = self.a
        if not os.path.isfile(a.db):
            raise Refusal("db_missing", "no database at %s (apply_ext.sh creates a fresh engine DB)" % a.db)
        if not (os.path.isfile(a.engine) and os.access(a.engine, os.X_OK)):
            raise Refusal("engine_missing", "engine binary %s is not an executable file" % a.engine)
        if a.klass not in ("seeds", "innovation"):
            raise Refusal("class_unsupported", "--class seeds|innovation")
        fj = self.read_freeze()
        self.check_snapshot(fj)
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
        self.links = []
        groups, _a, _b = (self.plan_seeds if a.klass == "seeds" else self.plan_innovation)(c, fj)[:3]
        todo = [g for g in groups if not all(c.execute("SELECT 1 FROM reg_source_map WHERE entry_id=?", (m[0],)).fetchone() for m in g["members"])]
        self.stats["entries_selected"] = sum(len(g["members"]) for g in groups) + len(self.links)
        self.stats["already_mapped"] = len(groups) - len(todo)
        if a.dry_run:
            IT.log("dry run: %d items planned, %d already complete, %d links to existing items" % (len(groups), len(groups) - len(todo), len(self.links)))
            return self.finish(c, dry=True, planned=groups)
        plog = open(a.progress_log, "a", encoding="utf-8") if a.progress_log else None
        try:
            for g in todo:
                self.processed += 1
                try:
                    self.process_group(c, g)
                except sqlite3.Error as e:
                    cl = IT.classify_db_error(str(e))
                    raise Refusal(cl or "db_error", "entry %d: %s" % (g["head_entry"], e), 21 if cl else 1)
                self.stats["groups"] += 1
                if plog and self.processed % 25 == 0:
                    plog.write("progress %d/%d\n" % (self.processed, len(todo)))
                    plog.flush()
            for (eid, atm, basis) in self.links:
                self.link_existing(c, eid, atm, basis)
            if plog:
                plog.write("done %d/%d\n" % (self.processed, len(todo)))
        finally:
            if plog:
                plog.close()
        return self.finish(c, dry=False, planned=groups)

    def finish(self, c, dry, planned):
        a = self.a
        srcloc = SEED_SRC if a.klass == "seeds" else INNO_SRC
        unm = c.execute("SELECT count(*) FROM v_unmapped_entries v JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=?", (srcloc,)).fetchone()[0]
        res = {"schema": "import-plan-run/1", "class": a.klass, "dry_run": dry, "stats": self.stats, "unmapped_in_source": unm}
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
            if unm:
                raise Refusal("incomplete", "%d entries of %s are still unmapped (v_unmapped_entries)" % (unm, srcloc), 1)
            self.rec["items_in_register"] = c.execute("SELECT count(DISTINCT m.atm_id) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=? AND m.mapped_by=?", (srcloc, ACTOR)).fetchone()[0]
            self.rec["unmapped_entries"] = unm
            self.rec["map_rows"] = {r[0]: r[1] for r in c.execute("SELECT m.relation, count(*) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=? GROUP BY 1", (srcloc,))}
            self.rec["by_type"] = {r[0]: r[1] for r in c.execute("SELECT i.type, count(*) FROM items i WHERE i.atm_id IN (SELECT m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=? AND m.relation='primary') GROUP BY 1", (srcloc,))}
            self.rec["by_status"] = {r[0]: r[1] for r in c.execute("SELECT i.status, count(*) FROM items i WHERE i.atm_id IN (SELECT m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=? AND m.relation='primary') GROUP BY 1", (srcloc,))}
            self.rec["findings_rows_written"] = c.execute("SELECT count(*) FROM reg_findings").fetchone()[0]
            self.rec["component_id_not_null"] = c.execute("SELECT count(*) FROM reg_item_ext WHERE component_id IS NOT NULL").fetchone()[0]
        if a.reconciliation:
            with open(a.reconciliation, "w", encoding="utf-8", newline="\n") as f:
                json.dump(self.rec, f, sort_keys=True, indent=1)
                f.write("\n")
        if a.report:
            with open(a.report, "w", encoding="utf-8", newline="\n") as f:
                json.dump(res, f, sort_keys=True, indent=1)
                f.write("\n")
        IT.log("ok %s %s" % (a.klass, json.dumps(self.stats, sort_keys=True)))
        return 0


def main(argv):
    ap = argparse.ArgumentParser(prog="import_plan.sh", add_help=True)
    ap.add_argument("--class", dest="klass", required=True)
    ap.add_argument("--db", required=True)
    ap.add_argument("--freeze-json", dest="freeze_json", required=True)
    ap.add_argument("--engine", required=True)
    ap.add_argument("--category-map", dest="category_map", default=os.path.join(HERE, "category_map.yaml"))
    ap.add_argument("--backup-record", dest="backup_record")
    ap.add_argument("--progress-log", dest="progress_log")
    ap.add_argument("--report")
    ap.add_argument("--reconciliation")
    ap.add_argument("--busy-timeout-ms", dest="busy_timeout_ms", type=int, default=5000)
    ap.add_argument("--dry-run", dest="dry_run", action="store_true")
    try:
        a = ap.parse_args(argv)
    except SystemExit as e:
        return 2 if e.code else 0
    a.kinds, a.category_out = "report_doc", None
    try:
        return PlanImporter(a).run()
    except Refusal as r:
        sys.stderr.write("import_plan: REFUSED reason=%s %s\n" % (r.reason, r.detail))
        return r.code
    except sqlite3.Error as e:
        cl = IT.classify_db_error(str(e))
        sys.stderr.write("import_plan: REFUSED reason=%s %s\n" % (cl or "db_error", e))
        return 21 if cl else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
