#!/usr/bin/env python3
"""import_tickets.py - the engine of scripts/register/import.sh (T168, WP-20; doc04 sections 8, 9.3, 9.4, 12.2).

Stage 1/2 importer of the findings register: for every `issue_file` source entry that Stage 0 (enumerate_sources.py) put into
reg_source_entries it mints a CAT id (reg_ids, mint_basis='import'), writes the reg_item_ext row BEFORE `$WI add --id` (the
legacy_import trigger accepts that order only), creates the item with the real engine binary, closes closed-class legacy tickets on
the legacy_import path (reverify_required=1), and maps the entry (reg_source_map, relation primary, match_basis import_1to1).
Everything is read from the FROZEN SNAPSHOT named by --freeze-json (never the live tree); the entry check
scripts/register/check_freeze_snapshot.sh runs first and its refusal stops the import with nothing written.

Usage: import_tickets.py --db <register db> --freeze-json <freeze.json> --engine <workable-items binary>
         [--kinds issue_file] [--category-map F] [--backup-record F] [--progress-log F] [--category-out F] [--report F]
         [--busy-timeout-ms N] [--by NAME] [--dry-run]
Exit: 0 ok | 2 usage | 20 refusal (`import: REFUSED reason=<code> ...` on stderr, nothing written) | 21 a write failed and the run is
resumable (db_locked, disk_full, io_error; every finished entry is kept, a re-run continues) | 1 engine or invariant failure.
Idempotent: an entry already in reg_source_map is skipped; a crash between the steps of one entry is resumed from the minted id
(reg_ids.minted_by = 'register-import:e<entry_id>'), never re-minted. A second run over a finished import writes nothing.
Test hooks (refused unless IMPORT_TEST_MODE=1): IMPORT_FAULT=<stage>:<n> (stages after_mint after_ext after_add after_close
before_map; exits 137 right after that stage of the n-th processed entry), IMPORT_MIN_FREE_BYTES (free-space precondition),
IMPORT_PAUSE=<n>:<file> (after the n-th processed entry create <file>.paused and wait, at most 30 s, for <file>.go).
"""
import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
import re
import sqlite3
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
DESC_CAP = 2048                       # bytes, Sources block included (T168; the register bound of T040b)
CLOSED_CLASS = ("resolved", "fixed", "closed")
SEVERITIES = ("critical", "high", "medium", "low", "cosmetic")
S_SCALE = {"s1": "critical", "s2": "high", "s3": "medium", "s4": "low", "s5": "cosmetic"}     # IC-01 (docs/21 section 7)
SUPPORTED_KINDS = ("issue_file",)
ACTOR = "register-import"
CLOSE_STATUS = {"Bug": "fixed", "Task": "completed", "Feature": "implemented"}     # the engine's per-Type closure vocabulary (11.4.33)
MIN_FREE_DEFAULT = 32 * 1024 * 1024


class Refusal(Exception):
    def __init__(self, reason, detail="", code=20):
        Exception.__init__(self, reason)
        self.reason, self.detail, self.code = reason, detail, code


def log(msg):
    sys.stderr.write("import: %s\n" % msg)


def load_enumerator():
    """the front-matter reader of the Stage 0 enumerator is the one reader (one parser, no second dialect)"""
    spec = importlib.util.spec_from_file_location("enumerate_sources", os.path.join(HERE, "enumerate_sources.py"))
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def classify_db_error(text):
    t = text.lower()
    if "locked" in t or "busy" in t:
        return "db_locked"
    if "disk is full" in t or "no space" in t or "file too large" in t:
        return "disk_full"
    if "i/o error" in t or "disk i/o" in t:
        return "io_error"
    return None


# ------------------------------------------------------------------------------------------------ description (pure functions)
def cut_utf8(data, n):
    """the first n bytes of data cut on a UTF-8 character boundary"""
    return data[:n].decode("utf-8", "ignore").encode("utf-8")


def build_description(body, source_lines):
    """body: the ticket text (str); source_lines: the lines of the Sources block after the file/sha lines.
    Returns (description str, cut bool). At most DESC_CAP bytes, the Sources block included; when the text is cut the block says so
    and gives the kept and total byte counts; the text is cut on a UTF-8 character boundary."""
    text = body.strip()
    raw = text.encode("utf-8")

    def assemble(kept, note):
        block = "**Sources**\n" + "\n".join(source_lines) + (("\n" + note) if note else "")
        return ((kept + "\n\n") if kept else "") + block

    whole = assemble(text, "")
    if len(whole.encode("utf-8")) <= DESC_CAP:
        return whole, False
    n = min(len(raw), DESC_CAP)
    while n >= 0:
        kept = cut_utf8(raw, n).decode("utf-8").rstrip()
        note = "- Note: the text above is cut at %d of %d bytes; the full text is in the file named above." % (len(kept.encode("utf-8")), len(raw))
        d = assemble(kept, note)
        over = len(d.encode("utf-8")) - DESC_CAP
        if over <= 0:
            return d, True
        n -= over
    raise Refusal("sources_block_too_large", "the Sources block alone exceeds %d bytes" % DESC_CAP)


def sev_of(raw):
    r = (raw or "").strip().lower()
    if r in SEVERITIES:
        return r, "IC-01 identity", False
    if r in S_SCALE:
        return S_SCALE[r], "IC-01 %s" % r.upper(), False
    return "medium", "unmapped raw value; defaulted medium [DEFAULT - adjustable]", True


# ------------------------------------------------------------------------------------------------ the importer
class Importer:
    def __init__(self, a):
        self.a = a
        self.fault = None
        f = os.environ.get("IMPORT_FAULT")
        if f is not None:
            if os.environ.get("IMPORT_TEST_MODE") != "1":
                raise Refusal("test_hook_outside_test_mode", "IMPORT_FAULT is a test hook (IMPORT_TEST_MODE=1 required)")
            st, _, n = f.partition(":")
            self.fault = (st, int(n or "1"))
        self.pause = None
        if os.environ.get("IMPORT_PAUSE") is not None:
            if os.environ.get("IMPORT_TEST_MODE") != "1":
                raise Refusal("test_hook_outside_test_mode", "IMPORT_PAUSE is a test hook (IMPORT_TEST_MODE=1 required)")
            n, _, f = os.environ["IMPORT_PAUSE"].partition(":")
            self.pause = (int(n), f)
        self.min_free = MIN_FREE_DEFAULT
        if os.environ.get("IMPORT_MIN_FREE_BYTES") is not None:
            if os.environ.get("IMPORT_TEST_MODE") != "1":
                raise Refusal("test_hook_outside_test_mode", "IMPORT_MIN_FREE_BYTES is a test hook (IMPORT_TEST_MODE=1 required)")
            self.min_free = int(os.environ["IMPORT_MIN_FREE_BYTES"])
        self.processed = 0
        self.stats = {"entries_selected": 0, "already_mapped": 0, "processed": 0, "minted": 0, "resumed": 0, "items_added": 0,
                      "closed_legacy": 0, "queued": 0, "duplicates_linked": 0, "severity_defaulted": 0, "type_defaulted": 0,
                      "category_unmapped": 0, "entry_changed_after_import": 0}
        self.cat_rows = []

    # -- fault injection
    def hook(self, stage):
        if self.fault and self.fault[0] == stage and self.processed == self.fault[1]:
            sys.stderr.write("import: FAULT %s at entry #%d (test hook)\n" % self.fault)
            os._exit(137)

    # -- preflight
    def read_freeze(self):
        p = self.a.freeze_json
        try:
            with open(p, encoding="utf-8") as f:
                fj = json.load(f)
        except FileNotFoundError:
            raise Refusal("freeze_missing", "no freeze json at %s (T161 not run, or the wrong path)" % p)
        except (OSError, ValueError) as e:
            raise Refusal("freeze_invalid", "%s: %s" % (p, type(e).__name__))
        if not isinstance(fj, dict) or not isinstance(fj.get("snapshot"), str) or not isinstance(fj.get("manifest"), str):
            raise Refusal("freeze_invalid", "%s lacks string keys 'snapshot' and 'manifest'" % p)
        if not os.path.isdir(fj["snapshot"]):
            raise Refusal("freeze_missing", "snapshot directory %s does not exist" % fj["snapshot"])
        if not os.path.isfile(fj["manifest"]):
            raise Refusal("freeze_missing", "manifest %s does not exist" % fj["manifest"])
        return fj

    def check_snapshot(self, fj):
        chk = os.path.join(HERE, "check_freeze_snapshot.sh")
        r = subprocess.run(["bash", chk, fj["snapshot"], fj["manifest"]], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if r.returncode != 0:
            err = r.stderr.decode("utf-8", "replace")
            m = re.search(r"reason=(\w+)", err)
            raise Refusal(m.group(1) if m else "freeze_check_failed", err.strip()[:600])

    def check_backup(self):
        db = self.a.db
        if os.path.basename(db) != "workable_items.db":
            return
        rec = self.a.backup_record
        if not rec:
            raise Refusal("backup_record_required", "the real register needs a pre-op backup record (scripts/register/backup_db.sh --record F, doc04 12.2); pass --backup-record F")
        try:
            with open(rec, encoding="utf-8") as f:
                r = json.load(f)
            bp, ssha = r["backup_path"], r["source_sha256"]
        except (OSError, ValueError, KeyError, TypeError):
            raise Refusal("backup_record_invalid", "%s is not a backup record (backup_path, source_sha256)" % rec)
        if r.get("integrity") not in (None, "ok") or r.get("restore_probe") not in (None, "equal"):
            raise Refusal("backup_record_invalid", "%s reports a failed check" % rec)
        if not os.path.isfile(os.path.join(os.path.dirname(os.path.abspath(db)), os.path.basename(bp))) and not os.path.isfile(bp):
            raise Refusal("backup_record_invalid", "the backup file %s named by the record does not exist" % bp)
        h = hashlib.sha256()
        with open(db, "rb") as f:
            for c in iter(lambda: f.read(1 << 20), b""):
                h.update(c)
        if h.hexdigest() != ssha:
            raise Refusal("backup_stale", "sha256 of %s is %s, the record says %s: the register changed after the backup" % (db, h.hexdigest(), ssha))

    def check_disk(self):
        d = os.path.dirname(os.path.abspath(self.a.db)) or "."
        st = os.statvfs(d)
        free = st.f_bavail * st.f_frsize
        if free < self.min_free:
            raise Refusal("disk_headroom", "%d bytes free under %s, %d required (nothing written)" % (free, d, self.min_free))

    # -- database
    def connect(self):
        c = sqlite3.connect(self.a.db, timeout=self.a.busy_timeout_ms / 1000.0, isolation_level=None)
        c.execute("PRAGMA foreign_keys=ON")
        c.execute("PRAGMA busy_timeout=%d" % self.a.busy_timeout_ms)
        return c

    def check_ddl(self, c):
        names = {r[0] for r in c.execute("SELECT name FROM sqlite_master")}
        need = {"reg_ids", "reg_item_ext", "reg_sources", "reg_source_entries", "reg_source_map", "items", "v_unmapped_entries"}
        if need - names:
            raise Refusal("register_ddl_absent", "missing %s (scripts/register/apply_ext.sh not applied to this database)" % ",".join(sorted(need - names)))

    # -- category map
    def load_map(self):
        import yaml
        try:
            with open(self.a.category_map, encoding="utf-8") as f:
                m = yaml.safe_load(f)
            assert isinstance(m["categories"], dict) and isinstance(m["types"], dict) and isinstance(m["interim_category"], str)
        except (OSError, ValueError, KeyError, TypeError, AssertionError):
            raise Refusal("category_map_invalid", "%s" % self.a.category_map)
        self.cmap = {str(k).lower(): v for k, v in m["categories"].items()}
        self.tmap = {str(k).lower(): v for k, v in m["types"].items()}
        self.interim = m["interim_category"]
        if self.interim not in ("bug", "error", "gap", "misalignment", "shortcoming", "weak_spot", "danger_zone", "documentation", "dependency", "test_gap", "governance"):
            raise Refusal("category_map_invalid", "interim_category %r is not a register category" % self.interim)

    # -- ticket reading
    def safe_path(self, snap, loc):
        if not loc or loc.startswith("/") or "\x00" in loc or ".." in loc.split("/"):
            return None
        p = os.path.join(snap, loc)
        real = os.path.realpath(p)
        if not real.startswith(os.path.realpath(snap) + os.sep):
            return None
        return p

    def read_tickets(self, c, snap, kinds):
        q = ("SELECT e.entry_id, s.kind, e.locator, e.legacy_id, e.title, e.raw_status, e.raw_severity, e.entry_sha256, "
             "(SELECT count(*) FROM reg_source_map m WHERE m.entry_id=e.entry_id) "
             "FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind IN (%s) ORDER BY e.entry_id" % ",".join("?" * len(kinds)))
        rows = c.execute(q, list(kinds)).fetchall()
        en = load_enumerator()
        out, problems = [], {}
        for (eid, kind, loc, lid, title, rst, rsev, esha, mapped) in rows:
            p = self.safe_path(snap, loc)
            if p is None:
                problems.setdefault("locator_unsafe", []).append(loc)
                continue
            try:
                with open(p, "rb") as f:
                    data = f.read()
            except OSError:
                problems.setdefault("ticket_missing", []).append(loc)
                continue
            sha = hashlib.sha256(data).hexdigest()
            if sha != esha:
                if mapped:
                    self.stats["entry_changed_after_import"] += 1
                else:
                    problems.setdefault("entry_changed", []).append("%s (register %s, snapshot %s)" % (loc, esha[:12], sha[:12]))
                    continue
            try:
                text = data.decode("utf-8")
            except UnicodeDecodeError:
                problems.setdefault("ticket_not_utf8", []).append(loc)
                continue
            fm, body, _errs = en.parse_frontmatter(text)
            htitle = en.first_heading(body)
            body = self.strip_heading(body)
            raw_cat = (fm.get("category") or "").strip()
            prop = self.cmap.get(raw_cat.lower())
            if raw_cat and prop is None:
                self.stats["category_unmapped"] += 1
            self.cat_rows.append((loc, lid or "", raw_cat, prop or ""))
            out.append({"entry_id": eid, "locator": loc, "legacy_id": lid, "title": title or htitle, "raw_status": rst, "raw_severity": rsev,
                        "sha": sha, "body": body, "fm": fm, "mapped": bool(mapped), "path": p})
        if problems:
            order = ("locator_unsafe", "ticket_missing", "entry_changed", "ticket_not_utf8")
            first = next(k for k in order if k in problems)
            detail = "; ".join("%s: %d (%s)" % (k, len(v), ", ".join(v[:5]) + (" ..." if len(v) > 5 else "")) for k, v in sorted(problems.items()))
            raise Refusal(first, detail + " - nothing written")
        return out

    @staticmethod
    def strip_heading(body):
        lines = body.split("\n")
        i = 0
        while i < len(lines) and not lines[i].strip():
            i += 1
        if i < len(lines) and re.match(r"^#{1,6}\s", lines[i]):
            i += 1
        return "\n".join(lines[i:])

    # -- one entry
    def plan_item(self, t):
        fm = t["fm"]
        raw_cat = (fm.get("category") or "").strip()
        rc = raw_cat.lower()
        proposed = self.cmap.get(rc)
        typ = self.tmap.get(rc)
        type_defaulted = typ is None
        if type_defaulted:
            typ = "Task"
            self.stats["type_defaulted"] += 1
        sev, sev_how, sev_def = sev_of(t["raw_severity"])
        if sev_def:
            self.stats["severity_defaulted"] += 1
        rst = (t["raw_status"] or "").strip().lower()
        closed = rst in CLOSED_CLASS
        title = re.sub(r"\s+", " ", (t["title"] or "").strip()) or os.path.splitext(os.path.basename(t["locator"]))[0]
        lines = ["- File: %s" % t["locator"], "- sha256: %s" % t["sha"],
                 "- Legacy id: %s (raw status: %s, raw severity: %s, raw category: %s)" % (t["legacy_id"] or "none", t["raw_status"] or "none", t["raw_severity"] or "none", raw_cat or "none"),
                 "- Category proposed by category_map.yaml, NOT applied (BLOCKED-ON ODG-17): %s" % (proposed or ("unmapped" if rc else "none")),
                 "- Type: %s%s" % (typ, " [DEFAULT - adjustable]" if type_defaulted else " (category_map.yaml)")]
        extra = [k for k in ("platform", "screen") if fm.get(k)]
        if extra:
            lines.append("- " + ", ".join("%s: %s" % (k, fm[k]) for k in extra))
        desc, cut = build_description(t["body"], lines)
        return {"title": title, "type": typ, "sev": sev, "sev_src": "%s: raw '%s' of %s" % (sev_how, t["raw_severity"] or "", t["locator"]),
                "closed": closed, "rst": rst, "desc": desc, "cut": cut}

    def engine(self, args):
        r = subprocess.run([self.a.engine] + args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        return r.returncode, r.stdout.decode("utf-8", "replace")

    def engine_fail(self, what, out):
        cl = classify_db_error(out)
        if cl:
            raise Refusal(cl, "%s: %s" % (what, out.strip()[:300]), 21)
        raise Refusal("engine_failed", "%s: %s" % (what, out.strip()[:300]), 1)

    def process_head(self, c, t):
        plan = self.plan_item(t)
        eid = t["entry_id"]
        marker = "%s:e%d" % (ACTOR, eid)
        row = c.execute("SELECT atm_id FROM reg_ids WHERE minted_by=? AND mint_basis='import'", (marker,)).fetchone()
        if row:
            atm = row[0]
            self.stats["resumed"] += 1
        else:
            c.execute("INSERT INTO reg_ids(minted_by,mint_basis) VALUES (?, 'import')", (marker,))
            atm = c.execute("SELECT atm_id FROM reg_ids WHERE seq=last_insert_rowid()").fetchone()[0]
            self.stats["minted"] += 1
        self.hook("after_mint")
        if not c.execute("SELECT 1 FROM reg_item_ext WHERE atm_id=?", (atm,)).fetchone():
            if plan["closed"]:
                vals = ("legacy_import", 1)
            else:
                vals = ("machine_evidence", 0)
            c.execute("INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,severity_source,custody_basis,reverify_required,legacy_status) "
                      "VALUES (?,?,?,?,?,?,?,?)", (atm, self.interim, "source", plan["sev"], plan["sev_src"], vals[0], vals[1], plan["rst"] or None))
        self.hook("after_ext")
        if not c.execute("SELECT 1 FROM items WHERE atm_id=?", (atm,)).fetchone():
            rc, out = self.engine(["add", plan["type"], plan["sev"], "--db", self.a.db, "--id", atm, "--prefix", "CAT", "--title", plan["title"],
                                   "--description", plan["desc"], "--created-by", ACTOR])
            if rc != 0:
                self.engine_fail("add %s" % atm, out)
            self.stats["items_added"] += 1
        self.hook("after_add")
        st = c.execute("SELECT status FROM items WHERE atm_id=?", (atm,)).fetchone()[0]
        if plan["closed"] and st == "Queued":
            rc, out = self.engine(["close", atm, "--db", self.a.db, "--status", CLOSE_STATUS[plan["type"]], "--evidence", t["path"]])
            if rc != 0:
                self.engine_fail("close %s" % atm, out)
            self.stats["closed_legacy"] += 1
        elif not plan["closed"]:
            self.stats["queued"] += 1
        self.hook("after_close")
        self.hook("before_map")
        c.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) VALUES (?,?,'primary','import_1to1',1,?)",
                  (eid, atm, ACTOR))
        return atm

    def run(self):
        a = self.a
        if not os.path.isfile(a.db):
            raise Refusal("db_missing", "no database at %s (apply_ext.sh creates a fresh engine DB)" % a.db)
        if not (os.path.isfile(a.engine) and os.access(a.engine, os.X_OK)):
            raise Refusal("engine_missing", "engine binary %s is not an executable file" % a.engine)
        kinds = [k for k in a.kinds.split(",") if k]
        bad = [k for k in kinds if k not in SUPPORTED_KINDS]
        if not kinds or bad:
            raise Refusal("kind_unsupported", "this importer reads %s; asked for %s" % (",".join(SUPPORTED_KINDS), ",".join(bad) or "nothing"))
        fj = self.read_freeze()
        self.check_snapshot(fj)                         # the mandatory entry check (round-31 review I1): before any ticket is read
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
        tickets = self.read_tickets(c, fj["snapshot"], kinds)
        self.stats["entries_selected"] = len(tickets)
        todo = []
        for t in tickets:
            if t["mapped"]:
                self.stats["already_mapped"] += 1
                continue
            todo.append(t)
        if a.dry_run:
            log("dry run: %d entries selected, %d already mapped, %d to import" % (len(tickets), self.stats["already_mapped"], len(todo)))
            return self.finish(c, kinds, dry=True)
        plog = open(a.progress_log, "a", encoding="utf-8") if a.progress_log else None
        try:
            head_atm = {}
            for t in tickets:       # heads already mapped: remember their item for the duplicate rule
                if t["mapped"]:
                    r = c.execute("SELECT atm_id FROM reg_source_map WHERE entry_id=?", (t["entry_id"],)).fetchone()
                    head_atm.setdefault(t["locator"], r[0])
            for t in todo:
                self.processed += 1
                try:
                    if t["locator"] in head_atm:        # the same ticket listed by a second source: linked, never minted twice (11.4.214)
                        c.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) VALUES (?,?,'duplicate_of','import_1to1',0,?)",
                                  (t["entry_id"], head_atm[t["locator"]], ACTOR))
                        self.stats["duplicates_linked"] += 1
                    else:
                        head_atm[t["locator"]] = self.process_head(c, t)
                except sqlite3.Error as e:
                    cl = classify_db_error(str(e))
                    raise Refusal(cl or "db_error", "entry %d (%s): %s" % (t["entry_id"], t["locator"], e), 21 if cl else 1)
                self.stats["processed"] += 1
                if self.pause and self.processed == self.pause[0]:
                    open(self.pause[1] + ".paused", "w").close()
                    deadline = time.monotonic() + 30
                    while not os.path.exists(self.pause[1] + ".go") and time.monotonic() < deadline:
                        time.sleep(0.01)
                if plog and (self.processed % 50 == 0):
                    plog.write("progress %d/%d\n" % (self.processed, len(todo)))
                    plog.flush()
            if plog:
                plog.write("done %d/%d\n" % (self.processed, len(todo)))
        finally:
            if plog:
                plog.close()
        return self.finish(c, kinds, dry=False)

    def finish(self, c, kinds, dry):
        a = self.a
        unm = c.execute("SELECT count(*) FROM v_unmapped_entries v JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.kind IN (%s)"
                        % ",".join("?" * len(kinds)), list(kinds)).fetchone()[0]
        res = {"schema": "import-run/1", "kinds": kinds, "dry_run": dry, "stats": self.stats, "unmapped_selected_kinds": unm}
        if not dry:
            rc, out = self.engine(["validate", "--db", a.db])
            res["engine_validate"] = {"rc": rc, "text": out.strip()[:300]}
            if rc != 0:
                raise Refusal("validate_failed", out.strip()[:300], 1)
            fk = c.execute("PRAGMA foreign_key_check").fetchall()
            ic = c.execute("PRAGMA integrity_check").fetchone()[0]
            res["foreign_key_check_rows"] = len(fk)
            res["integrity_check"] = ic
            if fk or ic != "ok":
                raise Refusal("integrity_failed", "foreign_key_check rows=%d integrity_check=%s" % (len(fk), ic), 1)
            if unm:
                raise Refusal("incomplete", "%d selected entries are still unmapped (v_unmapped_entries)" % unm, 1)
        if a.category_out:
            with open(a.category_out, "w", encoding="utf-8", newline="\n") as f:
                f.write("locator\tlegacy_id\traw_category\tproposed_category\n")
                for r in sorted(self.cat_rows):
                    f.write("\t".join(r) + "\n")
        if a.report:
            with open(a.report, "w", encoding="utf-8", newline="\n") as f:
                json.dump(res, f, sort_keys=True, indent=1)
                f.write("\n")
        log("ok %s" % json.dumps(self.stats, sort_keys=True))
        return 0


def main(argv):
    ap = argparse.ArgumentParser(prog="import.sh", add_help=True)
    ap.add_argument("--db", required=True)
    ap.add_argument("--freeze-json", dest="freeze_json", required=True)
    ap.add_argument("--engine", required=True)
    ap.add_argument("--kinds", default="issue_file")
    ap.add_argument("--category-map", dest="category_map", default=os.path.join(HERE, "category_map.yaml"))
    ap.add_argument("--backup-record", dest="backup_record")
    ap.add_argument("--progress-log", dest="progress_log")
    ap.add_argument("--category-out", dest="category_out")
    ap.add_argument("--report")
    ap.add_argument("--busy-timeout-ms", dest="busy_timeout_ms", type=int, default=5000)
    ap.add_argument("--dry-run", dest="dry_run", action="store_true")
    try:
        a = ap.parse_args(argv)
    except SystemExit as e:
        return 2 if e.code else 0
    try:
        return Importer(a).run()
    except Refusal as r:
        sys.stderr.write("import: REFUSED reason=%s %s\n" % (r.reason, r.detail))
        return r.code
    except sqlite3.Error as e:
        cl = classify_db_error(str(e))
        sys.stderr.write("import: REFUSED reason=%s %s\n" % (cl or "db_error", e))
        return 21 if cl else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
