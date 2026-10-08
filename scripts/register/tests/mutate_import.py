#!/usr/bin/env python3
"""mutate_import.py - T168 mutation set of the importer (WP-20). Writes one mutant tree per id under <out>/<ID>/ (a byte copy of
scripts/register/{import.sh,import_tickets.py,category_map.yaml,enumerate_sources.py,snapshot_manifest.py,check_freeze_snapshot.sh}
with ONE edit of import_tickets.py) and prints the id list. Every edit must match exactly once (a stale anchor is an error, never a
silent no-op). Mutants are written by the task author (docs/04 13.2 mutation obligation); the control mutant C00 is comment-only and
MUST survive (it proves the harness does not call every change a kill); N00 is the unmutated tree and MUST pass.

Usage: mutate_import.py <out dir> [ID...]      mutate_import.py --list
"""
import os, shutil, sys

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
FILES = ("import.sh", "import_tickets.py", "category_map.yaml", "enumerate_sources.py", "snapshot_manifest.py", "check_freeze_snapshot.sh")

# (id, what the mutant breaks, anchor text that must occur exactly once, replacement)
MUTANTS = [
    ("N00", "NEGATIVE CONTROL: the unmutated tree must pass", None, None),
    ("C00", "CONTROL: a comment-only change must SURVIVE", "ACTOR = \"register-import\"", "ACTOR = \"register-import\"  # comment only"),
    ("M01", "skips the mandatory freeze-snapshot entry check", "        self.check_snapshot(fj)                         # the mandatory entry check (round-31 review I1): before any ticket is read\n", "        pass\n"),
    ("M02", "drops the legacy_import path: closed-class tickets get machine_evidence", "                vals = (\"legacy_import\", 1)", "                vals = (\"machine_evidence\", 0)"),
    ("M03", "closed-class set loses 'closed'", "CLOSED_CLASS = (\"resolved\", \"fixed\", \"closed\")", "CLOSED_CLASS = (\"resolved\", \"fixed\")"),
    ("M04", "treats wontfix as closed (FR-008: wontfix is not a permitted closure)", "CLOSED_CLASS = (\"resolved\", \"fixed\", \"closed\")", "CLOSED_CLASS = (\"resolved\", \"fixed\", \"closed\", \"wontfix\")"),
    ("M05", "description cap raised to 4096 bytes", "DESC_CAP = 2048 ", "DESC_CAP = 4096 "),
    ("M06", "cut on a byte offset, not a character boundary (replacement characters)", "return data[:n].decode(\"utf-8\", \"ignore\").encode(\"utf-8\")", "return data[:n].decode(\"utf-8\", \"replace\").encode(\"utf-8\")"),
    ("M07", "primary map row without severity_governs", "VALUES (?,?,'primary','import_1to1',1,?)", "VALUES (?,?,'primary','import_1to1',0,?)"),
    ("M08", "IC-01 S2 mapped to medium instead of high", "\"s2\": \"high\"", "\"s2\": \"medium\""),
    ("M09", "resume key ignored: every run mints a new id", "        row = c.execute(\"SELECT atm_id FROM reg_ids WHERE minted_by=? AND mint_basis='import'\", (marker,)).fetchone()", "        row = None"),
    ("M10", "entry sha256 comparison removed (a changed entry is imported)", "            if sha != esha:", "            if False:"),
    ("M11", "duplicate-locator rule off (a second listing mints a second item)", "                    if t[\"locator\"] in head_atm:", "                    if False:"),
    ("M12", "concurrency lock removed", "            fcntl.flock(dbf, fcntl.LOCK_EX | fcntl.LOCK_NB)", "            pass"),
    ("M13", "a locked database is not classified (exit 1 db_error instead of 21 db_locked)", "    if \"locked\" in t or \"busy\" in t:\n        return \"db_locked\"", "    if \"zzz-never\" in t:\n        return \"db_locked\""),
    ("M14", "a full disk is not classified", "    if \"disk is full\" in t or \"no space\" in t or \"file too large\" in t:\n        return \"disk_full\"\n    if \"i/o error\" in t or \"disk i/o\" in t:\n        return \"io_error\"", "    if \"zzz-never\" in t:\n        return \"disk_full\""),
    ("M15", "backup record check skipped", "        self.check_backup()\n", "        pass\n"),
    ("M16", "backup staleness (database changed after the backup) not checked", "        if h.hexdigest() != ssha:", "        if False:"),
    ("M17", "writes a component into component_id (the register holds no components)", "\"INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,severity_source,custody_basis,reverify_required,legacy_status) \"\n                      \"VALUES (?,?,?,?,?,?,?,?)\", (atm, self.interim,",
     "\"INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,severity_source,custody_basis,reverify_required,legacy_status,component_id) \"\n                      \"VALUES (?,?,?,?,?,?,?,?,'backend')\", (atm, self.interim,"),
    ("M18", "APPLIES the proposed category to reg_item_ext.category", "(atm, self.interim, \"source\"", "(atm, (self.cmap.get((t[\"fm\"].get(\"category\") or \"\").lower()) or self.interim), \"source\""),
    ("M19", "engine type mapping ignored: every item is a Task", "        typ = self.tmap.get(rc)\n", "        typ = None\n"),
    ("M20", "closes a Bug with the Task vocabulary", "CLOSE_STATUS = {\"Bug\": \"fixed\"", "CLOSE_STATUS = {\"Bug\": \"completed\""),
    ("M21", "an unmapped severity defaults to high", "    return \"medium\", \"unmapped raw value; defaulted medium [DEFAULT - adjustable]\", True", "    return \"high\", \"unmapped raw value; defaulted medium [DEFAULT - adjustable]\", True"),
    ("M22", "legacy_status not stored", "plan[\"rst\"] or None))", "None))"),
    ("M23", "entries processed in reverse order (legacy order lost)", "ORDER BY e.entry_id\" % \",\".join", "ORDER BY e.entry_id DESC\" % \",\".join"),
    ("M24", "register DDL presence not checked", "        self.check_ddl(c)\n", "        pass\n"),
    ("M25", "progress log never says done", "                plog.write(\"done %d/%d\\n\" % (self.processed, len(todo)))", "                plog.write(\"x\\n\")"),
    ("M26", "reads the ticket from the working directory, not the frozen snapshot", "        p = os.path.join(snap, loc)\n", "        p = os.path.join(os.getcwd(), loc)\n"),
    ("M27", "dry run writes", "        if a.dry_run:", "        if False:"),
    ("M28", "unsafe locators (..) accepted", "or \"..\" in loc.split(\"/\")", "or False"),
    ("M29", "crash hook honoured outside IMPORT_TEST_MODE", "            if os.environ.get(\"IMPORT_TEST_MODE\") != \"1\":\n                raise Refusal(\"test_hook_outside_test_mode\", \"IMPORT_FAULT is a test hook (IMPORT_TEST_MODE=1 required)\")\n", ""),
    ("M30", "extension row written after the item exists (legacy order inverted) - skips the ext write before add", "        if not c.execute(\"SELECT 1 FROM reg_item_ext WHERE atm_id=?\", (atm,)).fetchone():\n            if plan[\"closed\"]:", "        if False:\n            if plan[\"closed\"]:"),
    ("M31", "the Sources block drops the sha256 line", "lines = [\"- File: %s\" % t[\"locator\"], \"- sha256: %s\" % t[\"sha\"],", "lines = [\"- File: %s\" % t[\"locator\"],"),
    ("M32", "an engine failure is swallowed (add error ignored)", "            if rc != 0:\n                self.engine_fail(\"add %s\" % atm, out)\n            self.stats[\"items_added\"] += 1", "            self.stats[\"items_added\"] += 1"),
    ("M33", "the final validate/integrity gate is skipped", "            if rc != 0:\n                raise Refusal(\"validate_failed\", out.strip()[:300], 1)\n", ""),
    ("M34", "the free-space precondition is skipped", "        self.check_disk()\n", "        pass\n"),
]


def build(out, ids):
    src = open(os.path.join(REG, "import_tickets.py"), encoding="utf-8").read()
    made = []
    for mid, what, old, new in MUTANTS:
        if ids and mid not in ids:
            continue
        d = os.path.join(out, mid)
        shutil.rmtree(d, ignore_errors=True)
        os.makedirs(d)
        for f in FILES:
            shutil.copy2(os.path.join(REG, f), os.path.join(d, f))
        if old is not None:
            n = src.count(old)
            if n != 1:
                sys.exit("mutate_import: mutant %s anchor occurs %d times (want exactly 1): %r" % (mid, n, old[:80]))
            open(os.path.join(d, "import_tickets.py"), "w", encoding="utf-8").write(src.replace(old, new))
        open(os.path.join(d, "WHAT"), "w").write(what + "\n")
        made.append(mid)
    return made


if __name__ == "__main__":
    a = sys.argv[1:]
    if a[:1] == ["--list"]:
        for mid, what, _o, _n in MUTANTS:
            print("%s\t%s" % (mid, what))
    elif len(a) >= 1:
        print("\n".join(build(a[0], a[1:])))
    else:
        sys.exit(__doc__)
