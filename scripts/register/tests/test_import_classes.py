#!/usr/bin/env python3
"""test_import_classes.py - T168 (WP-20): class-exhaustive tests of the Stage 1/2 importer scripts/register/import.sh beyond the four legs of
test_import.sh (T163). Real SQLite (the image's sqlite3 and python sqlite3), the real engine binary, scratch databases only (never
docs/workable_items.db). Each leg builds its own tiny frozen snapshot, enumerates it with the real enumerate_sources.py (Stage 0) and
applies the SQL to a fresh register DB, then runs the importer.

Failure classes (the class inventory; a leg per class, a control needle where an empty result would be read as "nothing happened"):
  F1 freeze            missing freeze json, invalid json, missing snapshot dir, missing manifest, malformed manifest, manifest sha tampered,
                       snapshot file changed after the manifest (freeze_snapshot_moved), file added after the manifest
  F2 entry             changed entry at the same locator (entry_changed), missing ticket file, non-UTF-8 ticket, unsafe locator
  F3 duplicate         the same ticket listed by a second source: linked duplicate_of, never minted twice; re-run adds nothing
  F4 partial/resume    a crash after each of the 5 stages of an entry (closed-class entry and open entry), then a re-run: the state equals a clean run
  F5 concurrent        a held flock (import_already_running, nothing written) and five true parallel pairs
  F6 unmapped          kinds other than issue_file stay unmapped and are reported; --kinds for them is refused
  F7 db locked         an exclusive writer holds the database: exit 21 db_locked, nothing written, a re-run completes
  F8 disk              a file-size limit below the database growth: exit 21, integrity ok, a re-run completes; the free-space precondition
  F9 backup            the real register name needs a valid unchanged-database backup record
  F10 content          status/severity/category/type mapping, one severity_governs per item, legacy order, component_id NULL, FK check,
                       snapshot-not-live, empty body, dry run, usage, test-hook gate, progress log, category TSV (raw + proposed, not applied)
Env: IMPORT_SH (default scripts/register/import.sh), WI, ENUMERATE_SOURCES. Exit 0 = all legs ok.
"""
import hashlib, json, os, re, resource, shutil, signal, sqlite3, subprocess, sys, tempfile, threading, time

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(REG))
IMP = os.environ.get("IMPORT_SH", os.path.join(REG, "import.sh"))
WI = os.environ.get("WI", os.path.join(ROOT, "submodules/constitution/scripts/workable-items/bin/workable-items-linux"))
ENUM = os.environ.get("ENUMERATE_SOURCES", os.path.join(REG, "enumerate_sources.py"))
SCR = tempfile.mkdtemp(prefix="impcls.", dir=os.environ.get("TMPDIR") or None)
PASS = FAIL = 0
CTR = [0]


def ok(label):
    global PASS
    PASS += 1
    print("ok   " + label)


def bad(label, info=""):
    global FAIL
    FAIL += 1
    print("FAIL " + label + (" :: " + str(info)[:600] if info else ""))


def check(label, cond, info=""):
    ok(label) if cond else bad(label, info)


def run(args, env=None, cwd=None, pre=None, timeout=300):
    e = dict(os.environ)
    e.update(env or {})
    r = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, env=e, cwd=cwd, preexec_fn=pre, timeout=timeout)
    return r.returncode, r.stdout


def ticket(i, status="fixed", sev="high", cat="functional", title=None, body="body text of the ticket", fm=True, extra=""):
    t = title or ("Ticket %d" % i)
    head = "---\nid: HELIX-%03d\nseverity: %s\ncategory: %s\nstatus: %s\nplatform: api\nfound_date: 2026-03-29\n---\n\n" % (i, sev, cat, status) if fm else ""
    return head + "# " + t + "\n\n" + body + "\n" + extra


BASE = {      # path -> text ; entry order is the enumerator's (docs/issues/*.md sorted, then issues/*.md)
    "docs/issues/HELIX-100-a.md": ticket(100, "resolved", "S2", "functional", "Closed bug", "closed bug body"),
    "docs/issues/HELIX-100-b.md": ticket(100, "closed", "critical", "ux", "Closed ux task", "closed ux body"),
    "docs/issues/HELIX-101-wontfix.md": ticket(101, "wontfix", "low", "content", "Wont fix content", "wontfix body"),
    "docs/issues/HELIX-102-open.md": ticket(102, "open", "bogus", "performance", "Open performance", "open perf body"),
    "docs/issues/HELIX-103-bare.md": "# Bare ticket without front matter\n\n",
    "docs/issues/HELIX-104-fixed-long.md": ticket(104, "fixed", "cosmetic", "functional", "Long closed", ("é" * 1400) + "\n"),
    "issues/ANR-2026-01-01-x.md": "# ANR title\n\nID: ANR-2026-01-01-001\nStatus: RESOLVED\n",
}


class Env:
    """a frozen snapshot of the given tickets + its manifest + freeze json + an enumerated fresh register DB"""

    def __init__(self, name, files=None, extra_files=None):
        CTR[0] += 1
        self.d = os.path.join(SCR, "%s-%d" % (name, CTR[0]))
        self.tree = os.path.join(self.d, "tree")
        self.fj = os.path.join(self.d, "freeze.json")
        self.mf = os.path.join(self.d, "freeze.manifest.json")
        self.db = os.path.join(self.d, "reg.db")
        os.makedirs(self.tree)
        self.files = dict(files if files is not None else BASE)
        self.files.update(extra_files or {})
        for p, t in self.files.items():
            fp = os.path.join(self.tree, p)
            os.makedirs(os.path.dirname(fp), exist_ok=True)
            open(fp, "wb").write(t if isinstance(t, bytes) else t.encode("utf-8"))
        self.freeze()
        rc, out = run([os.path.join(REG, "apply_ext.sh"), "--db", self.db])
        assert rc == 0, out
        self.enumerate()

    def freeze(self):
        r = subprocess.run([sys.executable, os.path.join(REG, "snapshot_manifest.py"), self.tree], stdout=open(self.mf, "w"))
        assert r.returncode == 0
        json.dump({"snapshot": self.tree, "manifest": self.mf, "frozen_at": "2026-10-07T00:00:00Z", "head": "scratch", "remotes": []}, open(self.fj, "w"))

    def enumerate(self):
        out = os.path.join(self.d, "enum")
        shutil.rmtree(out, ignore_errors=True)
        rc, o = run([sys.executable, ENUM, "--freeze-json", self.fj, "--out", out])
        assert rc == 0, o
        rc, o = run(["sqlite3", self.db, ".read " + os.path.join(out, "source_entries.sql")])
        assert rc == 0, o

    def imp(self, *extra, env=None, cwd=None, pre=None):
        return run([IMP, "--db", self.db, "--freeze-json", self.fj, "--engine", WI] + list(extra), env=env, cwd=cwd, pre=pre)

    def q(self, sql, *a):
        c = sqlite3.connect(self.db)
        try:
            return c.execute(sql, a).fetchall()
        finally:
            c.close()

    def n(self, table):
        return self.q("SELECT count(*) FROM " + table)[0][0]

    def counts(self):
        return tuple(self.n(t) for t in ("reg_ids", "reg_item_ext", "items", "reg_source_map", "reg_status_log"))

    def state(self):
        """the normalised register state: every table the importer writes, without timestamps"""
        return json.dumps({
            "ids": self.q("SELECT seq,atm_id,minted_by,mint_basis FROM reg_ids ORDER BY seq"),
            "ext": self.q("SELECT atm_id,category,defect_layer,component_id,severity,severity_source,custody_basis,reverify_required,legacy_status FROM reg_item_ext ORDER BY atm_id"),
            "items": self.q("SELECT atm_id,type,status,severity,title,description,current_location,created_by FROM items ORDER BY atm_id"),
            "map": self.q("SELECT entry_id,atm_id,relation,match_basis,severity_governs,mapped_by FROM reg_source_map ORDER BY entry_id"),
            "log": self.q("SELECT atm_id,from_status,to_status FROM reg_status_log ORDER BY log_id"),
        }, sort_keys=True)


def reason(out):
    m = re.search(r"REFUSED reason=(\w+)", out)
    return m.group(1) if m else None


def clean_state():
    e = Env("clean")
    rc, out = e.imp()
    assert rc == 0, out
    return e.state()


CLEAN = None


# ================================================================================================ F1 freeze
def leg_freeze():
    e = Env("f1")
    base = e.counts()
    rc, out = e.imp()       # control: the unmodified env imports (an instrument that cannot succeed proves no refusal)
    check("F1 control: the unmodified snapshot imports (exit 0)", rc == 0, out)
    for name, mut, want in [
        ("freeze json missing", lambda e: os.remove(e.fj), "freeze_missing"),
        ("freeze json not JSON", lambda e: open(e.fj, "w").write("{not json"), "freeze_invalid"),
        ("freeze json without snapshot key", lambda e: json.dump({"manifest": e.mf}, open(e.fj, "w")), "freeze_invalid"),
        ("snapshot dir missing", lambda e: shutil.rmtree(e.tree), "freeze_missing"),
        ("manifest file missing", lambda e: os.remove(e.mf), "freeze_missing"),
        ("manifest malformed", lambda e: open(e.mf, "w").write("[1,2"), "freeze_manifest_invalid"),
        ("manifest sha tampered (bad checksum)", lambda e: tamper_manifest(e), "freeze_snapshot_moved"),
        ("ticket changed after the manifest was written", lambda e: open(os.path.join(e.tree, "docs/issues/HELIX-101-wontfix.md"), "a").write("late edit\n"), "freeze_snapshot_moved"),
        ("file added after the manifest was written", lambda e: open(os.path.join(e.tree, "docs/issues/HELIX-999-new.md"), "w").write("x\n"), "freeze_snapshot_moved"),
        ("file removed after the manifest was written", lambda e: os.remove(os.path.join(e.tree, "docs/issues/HELIX-101-wontfix.md")), "freeze_snapshot_moved"),
    ]:
        x = Env("f1m")
        before = x.counts()
        mut(x)
        rc, out = x.imp()
        check("F1 %s: exit 20 reason %s, nothing written" % (name, want), rc == 20 and reason(out) == want and x.counts() == before, "rc=%s reason=%s counts=%s out=%s" % (rc, reason(out), x.counts(), out))


def tamper_manifest(e):
    m = json.load(open(e.mf))
    m["entries"][0]["sha256"] = "0" * 64
    json.dump(m, open(e.mf, "w"))


# ================================================================================================ F2 entry
def leg_entry():
    # changed entry at the same locator: the snapshot AND its manifest are consistent, the register still holds the old entry_sha256
    e = Env("f2a")
    before = e.counts()
    p = os.path.join(e.tree, "docs/issues/HELIX-102-open.md")
    open(p, "a").write("an edit made after Stage 0 enumerated this ticket\n")
    e.freeze()
    rc, out = e.imp()
    check("F2 changed entry at the same locator (snapshot consistent): exit 20 entry_changed naming it, nothing written",
          rc == 20 and reason(out) == "entry_changed" and "HELIX-102-open.md" in out and e.counts() == before, "rc=%s %s" % (rc, out))
    # control: after re-enumerating the changed snapshot the register holds the new hash... the entry row keeps the OLD sha (INSERT OR IGNORE): still refused
    e2 = Env("f2b")
    os.remove(os.path.join(e2.tree, "docs/issues/HELIX-103-bare.md"))
    e2.freeze()
    rc, out = e2.imp()
    check("F2 missing ticket file (entry in the register, file gone from a consistent snapshot): exit 20 ticket_missing, nothing written",
          rc == 20 and reason(out) == "ticket_missing" and "HELIX-103-bare.md" in out and e2.n("reg_ids") == 0, "rc=%s %s" % (rc, out))
    e3 = Env("f2c", extra_files={"docs/issues/HELIX-105-latin1.md": b"---\nid: HELIX-105\nseverity: low\nstatus: open\n---\n\n# Latin one\n\ncaf\xe9\n"})
    rc, out = e3.imp()
    check("F2 non-UTF-8 ticket: exit 20 ticket_not_utf8 naming it, nothing written", rc == 20 and reason(out) == "ticket_not_utf8" and "HELIX-105-latin1.md" in out and e3.n("reg_ids") == 0, "rc=%s %s" % (rc, out))
    e4 = Env("f2d")
    e4.q("SELECT 1")
    c = sqlite3.connect(e4.db)
    sid = c.execute("SELECT source_id FROM reg_sources WHERE kind='issue_file' LIMIT 1").fetchone()[0]
    c.execute("INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (?,?,?)", (sid, "../outside.md", "a" * 64))
    c.commit()
    c.close()
    rc, out = e4.imp()
    check("F2 unsafe locator (..): exit 20 locator_unsafe, nothing written", rc == 20 and reason(out) == "locator_unsafe" and e4.n("reg_ids") == 0, "rc=%s %s" % (rc, out))
    c = sqlite3.connect(e4.db)
    c.execute("UPDATE reg_source_entries SET locator='/etc/passwd' WHERE locator='../outside.md'")
    c.commit()
    c.close()
    rc, out = e4.imp()
    check("F2 absolute locator: exit 20 locator_unsafe, nothing written", rc == 20 and reason(out) == "locator_unsafe" and e4.n("reg_ids") == 0, "rc=%s %s" % (rc, out))
    # a locator that stays inside the snapshot but is not canonical (two locators for one file would mint twice)
    e4b = Env("f2f")
    c = sqlite3.connect(e4b.db)
    sid = c.execute("SELECT source_id FROM reg_sources WHERE kind='issue_file' LIMIT 1").fetchone()[0]
    c.execute("INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (?,?,?)", (sid, "docs/issues/../issues/HELIX-101-wontfix.md", "b" * 64))
    c.commit()
    c.close()
    rc, out = e4b.imp()
    check("F2 non-canonical locator (docs/issues/../issues/x.md, inside the snapshot): exit 20 locator_unsafe, nothing written", rc == 20 and reason(out) == "locator_unsafe" and e4b.n("reg_ids") == 0, "rc=%s %s" % (rc, out))
    # a symlink in the snapshot that points out of it
    e5 = Env("f2e")
    os.remove(os.path.join(e5.tree, "docs/issues/HELIX-103-bare.md"))
    os.symlink("/etc/hostname", os.path.join(e5.tree, "docs/issues/HELIX-103-bare.md"))
    e5.freeze()
    rc, out = e5.imp()
    check("F2 locator that is a symlink leaving the snapshot: refused, nothing written", rc == 20 and reason(out) in ("locator_unsafe", "entry_changed") and e5.n("reg_ids") == 0, "rc=%s %s" % (rc, out))


# ================================================================================================ F3 duplicate
def leg_duplicate():
    e = Env("f3")
    c = sqlite3.connect(e.db)
    c.execute("INSERT INTO reg_sources(kind,locator,parser) VALUES ('issue_file','second-source-listing','test')")
    sid = c.execute("SELECT source_id FROM reg_sources WHERE locator='second-source-listing'").fetchone()[0]
    sha = hashlib.sha256(open(os.path.join(e.tree, "docs/issues/HELIX-101-wontfix.md"), "rb").read()).hexdigest()
    c.execute("INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (?,?,?)", (sid, "docs/issues/HELIX-101-wontfix.md", sha))
    c.commit()
    c.close()
    rc, out = e.imp()
    check("F3 import with a duplicate locator exits 0", rc == 0, out)
    n_items = e.n("items")
    check("F3 the duplicate mints no second item (7 tickets -> 7 items, 8 entries mapped)", n_items == 7 and e.n("reg_source_map") == 8, (n_items, e.n("reg_source_map")))
    rows = e.q("SELECT m.relation,m.severity_governs,(SELECT atm_id FROM reg_source_map h JOIN reg_source_entries he USING(entry_id) WHERE he.locator=e.locator AND h.relation='primary') FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE e.locator='docs/issues/HELIX-101-wontfix.md' ORDER BY m.relation")
    check("F3 the second listing is relation duplicate_of onto the head's item, severity_governs 0", len(rows) == 2 and rows[0][0] == "duplicate_of" and rows[0][1] == 0 and rows[1][0] == "primary" and rows[0][2] == e.q("SELECT m.atm_id FROM reg_source_map m JOIN reg_source_entries x USING(entry_id) WHERE m.relation='duplicate_of'")[0][0], rows)
    s1 = e.state()
    rc, out = e.imp()
    check("F3 a second run changes nothing (state identical)", rc == 0 and e.state() == s1)
    # head already mapped in an earlier run, duplicate listed later
    e2 = Env("f3b")
    rc, out = e2.imp()
    assert rc == 0
    c = sqlite3.connect(e2.db)
    c.execute("INSERT INTO reg_sources(kind,locator,parser) VALUES ('issue_file','late-listing','test')")
    sid = c.execute("SELECT source_id FROM reg_sources WHERE locator='late-listing'").fetchone()[0]
    sha = hashlib.sha256(open(os.path.join(e2.tree, "docs/issues/HELIX-102-open.md"), "rb").read()).hexdigest()
    c.execute("INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (?,?,?)", (sid, "docs/issues/HELIX-102-open.md", sha))
    c.commit()
    c.close()
    items, ids = e2.n("items"), e2.n("reg_ids")
    rc, out = e2.imp()
    check("F3 a duplicate listed after the head was imported is linked to the head's item (no new item, no new id)", rc == 0 and e2.n("items") == items and e2.n("reg_ids") == ids and e2.q("SELECT count(*) FROM reg_source_map WHERE relation='duplicate_of'")[0][0] == 1, out)


# ================================================================================================ F4 resume
def leg_resume():
    global CLEAN
    for stage in ("after_mint", "after_ext", "after_add", "after_close", "before_map"):
        for nth, what in ((1, "closed-class entry"), (3, "queued entry")):
            e = Env("f4")
            rc, out = e.imp(env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "%s:%d" % (stage, nth)})
            check("F4 crash at %s of the %s (#%d): the run dies (137) with the work in progress" % (stage, what, nth), rc == 137, "rc=%s %s" % (rc, out))
            mid_unmapped = e.n("v_unmapped_entries")
            rc, out = e.imp()
            check("F4 re-run after the %s crash exits 0 and the state equals a clean run (no duplicate id, no orphan)" % stage,
                  rc == 0 and e.state() == CLEAN and e.n("v_ids_without_item") == 0 and e.n("reg_ids") == 7, "rc=%s mid_unmapped=%s ids=%s out=%s" % (rc, mid_unmapped, e.n("reg_ids"), out))
            rc, out = e.imp()
            check("F4 and a further run is a no-op", rc == 0 and e.state() == CLEAN)
    # the minted-id marker is what makes the resume exact
    e = Env("f4m")
    e.imp(env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "after_ext:2"})
    rows = e.q("SELECT minted_by FROM reg_ids ORDER BY seq")
    check("F4 minted_by names the register-import actor and the entry (resume key)", len(rows) == 2 and all(re.fullmatch(r"register-import:e\d+", r[0]) for r in rows), rows)


# ================================================================================================ F5 concurrent
def leg_concurrent():
    e = Env("f5")
    holder = subprocess.Popen([sys.executable, "-c", "import fcntl,os,sys,time\nf=os.open(sys.argv[1],os.O_RDONLY);fcntl.flock(f,fcntl.LOCK_EX);print('held',flush=True);time.sleep(30)", e.db], stdout=subprocess.PIPE, text=True)
    holder.stdout.readline()
    try:
        before = e.counts()
        rc, out = e.imp()
        check("F5 a second importer while the database lock is held: exit 20 import_already_running, nothing written", rc == 20 and reason(out) == "import_already_running" and e.counts() == before, "rc=%s %s" % (rc, out))
    finally:
        holder.kill()
        holder.wait()
    rc, out = e.imp()
    check("F5 after the holder is gone the import completes (state equals a clean run)", rc == 0 and e.state() == CLEAN, out)
    for i in range(5):
        x = Env("f5p")
        procs = [subprocess.Popen([IMP, "--db", x.db, "--freeze-json", x.fj, "--engine", WI], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True) for _ in range(2)]
        res = [(p.wait(), p.stdout.read()) for p in procs]
        codes = sorted(r[0] for r in res)
        rc, out = x.imp()       # whatever happened, one more run must finish it
        check("F5 parallel pair %d: exit codes in {0,20,21} with at least one 0 or a clean completion, final state equals a clean run, ids unique" % i,
              all(c in (0, 20, 21) for c in codes) and rc == 0 and x.state() == CLEAN and x.n("v_ids_without_item") == 0, "codes=%s final=%s" % (codes, rc))


# ================================================================================================ F6 unmapped
def leg_unmapped():
    e = Env("f6", extra_files={"TASK_TRACKER.md": "# T\n\n**Status:**\n- ⬜ **Not Started**\n\n| ID | Task | Pri | Status |\n|---|---|---|---|\n| 0.1 | first | high | ⬜ |\n",
                               "MASTER_EXECUTION_CHECKLIST.md": "# C\n\n- [ ] one\n- [ ] two\n"})
    total = e.n("reg_source_entries")
    tickets = e.q("SELECT count(*) FROM reg_source_entries x JOIN reg_sources s USING(source_id) WHERE s.kind='issue_file'")[0][0]
    check("F6 control needle: the snapshot holds entries of other kinds (%d entries, %d tickets)" % (total, tickets), total > tickets >= 7, (total, tickets))
    rc, out = e.imp("--report", os.path.join(e.d, "rep.json"))
    left = e.n("v_unmapped_entries")
    check("F6 every ticket is mapped; the other kinds stay unmapped (v_unmapped_entries = %d)" % (total - tickets), rc == 0 and left == total - tickets and e.q("SELECT count(*) FROM v_unmapped_entries v JOIN reg_source_entries x USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.kind='issue_file'")[0][0] == 0, (rc, left, total, tickets))
    rep = json.load(open(os.path.join(e.d, "rep.json")))
    check("F6 the run report names the selected-kind unmapped count 0", rep.get("unmapped_selected_kinds") == 0 and rep["stats"]["entries_selected"] == tickets, rep)
    e2 = Env("f6b")
    rc, out = e2.imp("--kinds", "md_tracker")
    check("F6 an unsupported kind is refused (kind_unsupported), nothing written", rc == 20 and reason(out) == "kind_unsupported" and e2.n("reg_ids") == 0, out)
    rc, out = e2.imp("--kinds", "issue_file,report_doc")
    check("F6 a supported kind mixed with an unsupported one is refused as a whole", rc == 20 and reason(out) == "kind_unsupported" and e2.n("reg_ids") == 0, out)


# ================================================================================================ F7 locked
def leg_locked():
    e = Env("f7")
    before = e.counts()
    c = sqlite3.connect(e.db, isolation_level=None)
    c.execute("BEGIN EXCLUSIVE")
    try:
        t0 = time.time()
        rc, out = e.imp("--busy-timeout-ms", "300")
        dt = time.time() - t0
    finally:
        c.execute("ROLLBACK")
        c.close()
    check("F7 an exclusive writer holds the database: exit 21 db_locked within the busy timeout, nothing written", rc == 21 and reason(out) == "db_locked" and dt < 30 and e.counts() == before, "rc=%s dt=%.1f %s" % (rc, dt, out))
    rc, out = e.imp()
    check("F7 after the writer is gone the re-run completes (state equals a clean run)", rc == 0 and e.state() == CLEAN, out)
    # the lock arrives in the middle of the run: the test hook pauses the importer after its 2nd entry, an exclusive writer takes the database, the importer resumes
    e2 = Env("f7b")
    flag = os.path.join(e2.d, "pause")
    proc = subprocess.Popen([IMP, "--db", e2.db, "--freeze-json", e2.fj, "--engine", WI, "--busy-timeout-ms", "200"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
                            env=dict(os.environ, IMPORT_TEST_MODE="1", IMPORT_PAUSE="2:" + flag))
    t0 = time.time()
    while not os.path.exists(flag + ".paused") and time.time() - t0 < 30 and proc.poll() is None:
        time.sleep(0.01)
    check("F7 control needle: the importer reached the pause after its 2nd entry (2 entries mapped)", os.path.exists(flag + ".paused") and e2.n("reg_source_map") == 2, e2.n("reg_source_map"))
    cc = sqlite3.connect(e2.db, isolation_level=None, timeout=0)
    cc.execute("BEGIN EXCLUSIVE")
    open(flag + ".go", "w").close()
    out = proc.stdout.read()
    rc = proc.wait()
    cc.execute("ROLLBACK")
    cc.close()
    check("F7 a writer arriving mid-run stops the importer: exit 21 db_locked, the finished entries are kept", rc == 21 and reason(out) == "db_locked" and e2.n("reg_source_map") == 2, "rc=%s %s mapped=%s" % (rc, out, e2.n("reg_source_map")))
    rc, out = e2.imp()
    check("F7 mid-run lock: the re-run completes and equals a clean run", rc == 0 and e2.state() == CLEAN, out)


# ================================================================================================ F8 disk
def leg_disk():
    e = Env("f8")
    before = e.counts()
    rc, out = e.imp(env={"IMPORT_TEST_MODE": "1", "IMPORT_MIN_FREE_BYTES": str(1 << 60)})
    check("F8 free-space precondition (not met): exit 20 disk_headroom, nothing written", rc == 20 and reason(out) == "disk_headroom" and e.counts() == before, out)
    rc, out = e.imp(env={"IMPORT_MIN_FREE_BYTES": "1"})
    check("F8 the free-space override is a test hook: refused outside IMPORT_TEST_MODE=1", rc == 20 and reason(out) == "test_hook_outside_test_mode" and e.counts() == before, out)
    lim = os.path.getsize(e.db)

    def limit():
        signal.signal(signal.SIGXFSZ, signal.SIG_IGN)
        resource.setrlimit(resource.RLIMIT_FSIZE, (lim, lim))
    rc, out = e.imp(pre=limit)
    ic = e.q("PRAGMA integrity_check")[0][0]
    check("F8 disk full (file size limit at the database size): exit 21 (disk_full or io_error), not a success", rc == 21 and reason(out) in ("disk_full", "io_error"), "rc=%s %s" % (rc, out))
    check("F8 the database is intact after the failed run (integrity_check ok, foreign_key_check empty)", ic == "ok" and e.q("PRAGMA foreign_key_check") == [], ic)
    rc, out = e.imp()
    check("F8 after the limit is lifted the re-run completes and equals a clean run", rc == 0 and e.state() == CLEAN, out)


# ================================================================================================ F9 backup
def leg_backup():
    e = Env("f9")
    real = os.path.join(e.d, "docs")
    os.makedirs(real)
    dbp = os.path.join(real, "workable_items.db")
    shutil.copy(e.db, dbp)
    base = [IMP, "--db", dbp, "--freeze-json", e.fj, "--engine", WI]

    def n_ids():
        c = sqlite3.connect(dbp)
        try:
            return c.execute("SELECT count(*) FROM reg_ids").fetchone()[0]
        finally:
            c.close()
    rc, out = run(base)
    check("F9 the register name without a backup record: exit 20 backup_record_required, nothing written", rc == 20 and reason(out) == "backup_record_required" and n_ids() == 0, out)
    rec = os.path.join(real, "rec.json")
    open(rec, "w").write("not json")
    rc, out = run(base + ["--backup-record", rec])
    check("F9 an unreadable record: exit 20 backup_record_invalid", rc == 20 and reason(out) == "backup_record_invalid" and n_ids() == 0, out)
    bak = os.path.join(real, "workable_items.db.bak-test")
    shutil.copy(dbp, bak)
    sha = hashlib.sha256(open(dbp, "rb").read()).hexdigest()
    json.dump({"backup_path": "docs/workable_items.db.bak-test", "source_sha256": "f" * 64, "integrity": "ok", "restore_probe": "equal"}, open(rec, "w"))
    rc, out = run(base + ["--backup-record", rec], cwd=e.d)
    check("F9 a record whose sha256 is not the database's: exit 20 backup_stale, nothing written", rc == 20 and reason(out) == "backup_stale" and n_ids() == 0, out)
    json.dump({"backup_path": "docs/workable_items.db.bak-missing", "source_sha256": sha, "integrity": "ok", "restore_probe": "equal"}, open(rec, "w"))
    rc, out = run(base + ["--backup-record", rec], cwd=e.d)
    check("F9 a record naming a backup file that does not exist: exit 20 backup_record_invalid", rc == 20 and reason(out) == "backup_record_invalid" and n_ids() == 0, out)
    json.dump({"backup_path": "docs/workable_items.db.bak-test", "source_sha256": sha, "integrity": "failed", "restore_probe": "equal"}, open(rec, "w"))
    rc, out = run(base + ["--backup-record", rec], cwd=e.d)
    check("F9 a record that reports a failed integrity check: exit 20 backup_record_invalid", rc == 20 and reason(out) == "backup_record_invalid" and n_ids() == 0, out)
    json.dump({"backup_path": "docs/workable_items.db.bak-test", "source_sha256": sha, "integrity": "ok", "restore_probe": "equal"}, open(rec, "w"))
    rc, out = run(base + ["--backup-record", rec], cwd=e.d)
    check("F9 control: a valid record for the unchanged database lets the import run", rc == 0 and n_ids() == 7, out)


# ================================================================================================ F10 content
def leg_content():
    e = Env("f10")
    cat = os.path.join(e.d, "cat.tsv")
    prog = os.path.join(e.d, "prog.log")
    rc, out = e.imp("--category-out", cat, "--progress-log", prog, "--report", os.path.join(e.d, "rep.json"))
    check("F10 import exits 0", rc == 0, out)
    items = {r[0]: r for r in e.q("SELECT i.atm_id,i.type,i.status,i.severity,x.custody_basis,x.reverify_required,x.legacy_status,x.category,x.component_id,x.severity_source,x.defect_layer,e.locator "
                                  "FROM items i JOIN reg_item_ext x USING(atm_id) JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries e USING(entry_id) WHERE m.relation='primary'")}
    by = {r[11]: r for r in items.values()}
    a, b, w, o, bare, lng, anr = (by["docs/issues/HELIX-100-a.md"], by["docs/issues/HELIX-100-b.md"], by["docs/issues/HELIX-101-wontfix.md"], by["docs/issues/HELIX-102-open.md"],
                                  by["docs/issues/HELIX-103-bare.md"], by["docs/issues/HELIX-104-fixed-long.md"], by["issues/ANR-2026-01-01-x.md"])
    check("F10 resolved + S2 + functional -> Bug, Fixed, severity high (IC-01 S2), legacy_import, reverify 1", a[1:7] == ("Bug", "Fixed (→ Fixed.md)", "high", "legacy_import", 1, "resolved"), a)
    check("F10 closed + critical + ux (no type mapped) -> Task defaulted, Completed, legacy_import, reverify 1", b[1:7] == ("Task", "Completed (→ Fixed.md)", "critical", "legacy_import", 1, "closed"), b)
    check("F10 wontfix -> Queued, machine_evidence, reverify 0, legacy_status wontfix (FR-008: not a permitted closure)", w[2] == "Queued" and w[4:7] == ("machine_evidence", 0, "wontfix"), w)
    check("F10 the open ticket -> Queued; unmapped severity 'bogus' defaults to medium and says so", o[2] == "Queued" and o[3] == "medium" and "DEFAULT - adjustable" in o[9] and o[4:6] == ("machine_evidence", 0), o)
    check("F10 a ticket without front matter imports Queued with a NULL legacy_status", bare[2] == "Queued" and bare[6] is None and bare[3] == "medium", bare)
    check("F10 the ANR file (RESOLVED, no severity) is closed-class: Completed, legacy_import", anr[2].startswith("Completed") and anr[4] == "legacy_import" and anr[5] == 1, anr)
    check("F10 cosmetic + functional + fixed -> Bug, Fixed (cosmetic kept)", lng[1] == "Bug" and lng[3] == "cosmetic" and lng[2].startswith("Fixed"), lng)
    check("F10 every item has defect_layer source (the lowest floor; it can only be raised) and the same interim category (the mapping is NOT applied)",
          {r[10] for r in items.values()} == {"source"} and {r[7] for r in items.values()} == {"shortcoming"}, {r[7] for r in items.values()})
    check("F10 component_id is NULL for every imported item and PRAGMA foreign_key_check prints nothing (T168 acceptance)",
          e.q("SELECT count(*) FROM reg_item_ext WHERE component_id IS NOT NULL")[0][0] == 0 and e.q("PRAGMA foreign_key_check") == [])
    check("F10 exactly one severity_governs=1 row per item (doc04 9.4)", e.q("SELECT count(*) FROM (SELECT atm_id FROM reg_source_map GROUP BY atm_id HAVING sum(severity_governs)<>1)")[0][0] == 0)
    check("F10 legacy order preserved: CAT ids ascend with the entry order", [r[0] for r in e.q("SELECT m.atm_id FROM reg_source_map m ORDER BY m.entry_id")] == sorted(r[0] for r in e.q("SELECT atm_id FROM reg_ids")))
    check("F10 every id is minted with mint_basis import; views are clean (v_legacy_import_unbacked, v_ids_without_item, v_custody_violations, v_items_without_mint)",
          e.q("SELECT count(*) FROM reg_ids WHERE mint_basis<>'import'")[0][0] == 0 and all(e.n(v) == 0 for v in ("v_legacy_import_unbacked", "v_ids_without_item", "v_items_without_mint", "v_duplicate_item_ids")))
    check("F10 closed-class items sit in v_reverify_queue; the queued ones do not", e.n("v_reverify_queue") == 4, e.n("v_reverify_queue"))
    d = e.q("SELECT description FROM items i JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries x USING(entry_id) WHERE x.locator='docs/issues/HELIX-100-a.md'")[0][0]
    check("F10 the description names the raw category and the PROPOSED category as not applied", "raw category: functional" in d and "NOT applied" in d and ": bug" in d, d)
    tsv = open(cat, encoding="utf-8").read().splitlines()
    check("F10 the category TSV lists every ticket with raw and proposed category", tsv[0] == "locator\tlegacy_id\traw_category\tproposed_category" and len(tsv) == 8 and "docs/issues/HELIX-100-a.md\tHELIX-100\tfunctional\tbug" in tsv and "docs/issues/HELIX-102-open.md\tHELIX-102\tperformance\tweak_spot" in tsv, tsv)
    check("F10 the progress log ends with done", open(prog).read().strip().endswith("done 7/7"), open(prog).read())
    # dry run
    e2 = Env("f10d")
    rc, out = e2.imp("--dry-run")
    check("F10 --dry-run exits 0 and writes nothing", rc == 0 and e2.n("reg_ids") == 0 and e2.n("items") == 0, out)
    # usage
    rc, out = run([IMP, "--db", e2.db])
    check("F10 missing arguments: exit 2", rc == 2, "rc=%s" % rc)
    rc, out = run([IMP, "--db", os.path.join(e2.d, "nope.db"), "--freeze-json", e2.fj, "--engine", WI])
    check("F10 a database that does not exist: exit 20 db_missing", rc == 20 and reason(out) == "db_missing", out)
    rc, out = run([IMP, "--db", e2.db, "--freeze-json", e2.fj, "--engine", os.path.join(e2.d, "no-engine")])
    check("F10 an engine path that is not an executable file: exit 20 engine_missing", rc == 20 and reason(out) == "engine_missing", out)
    bare_db = os.path.join(e2.d, "bare.db")
    sqlite3.connect(bare_db).close()
    rc, out = run([IMP, "--db", bare_db, "--freeze-json", e2.fj, "--engine", WI])
    check("F10 a database without the register DDL: exit 20 register_ddl_absent", rc == 20 and reason(out) == "register_ddl_absent", out)
    rc, out = e2.imp(env={"IMPORT_FAULT": "after_ext:1"})
    check("F10 the crash hook is refused outside IMPORT_TEST_MODE=1", rc == 20 and reason(out) == "test_hook_outside_test_mode" and e2.n("reg_ids") == 0, out)
    # snapshot, not live tree: the working directory holds a decoy docs/issues with different text
    e3 = Env("f10s")
    decoy = os.path.join(e3.d, "decoy")
    os.makedirs(os.path.join(decoy, "docs/issues"))
    open(os.path.join(decoy, "docs/issues/HELIX-101-wontfix.md"), "w").write("---\nid: HELIX-101\nstatus: wontfix\n---\n\n# Decoy\n\nDECOY LIVE TEXT\n")
    rc, out = e3.imp(cwd=decoy)
    d = e3.q("SELECT description FROM items i JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries x USING(entry_id) WHERE x.locator='docs/issues/HELIX-101-wontfix.md'")[0][0]
    check("F10 the text comes from the frozen snapshot, never the live tree (a decoy in the working directory is not read)", rc == 0 and "wontfix body" in d and "DECOY" not in d, d)
    # empty body: the Sources block alone passes the engine's description floor
    bd = e.q("SELECT description FROM items i JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries x USING(entry_id) WHERE x.locator='docs/issues/HELIX-103-bare.md'")[0][0]
    check("F10 an empty-body ticket imported: the description is the Sources block alone (it carries the engine's description floor)", bd.startswith("**Sources**") and len(bd) >= 40, bd)
    # engine validate on the result
    rc, out = run([WI, "validate", "--db", e.db])
    check("F10 the engine's own validate accepts the imported register", rc == 0 and "all invariants satisfied" in out, out)
    # description cap on the real distribution: every description <= 2048 bytes
    mx = e.q("SELECT max(length(CAST(description AS BLOB))) FROM items")[0][0]
    check("F10 no description exceeds 2048 bytes (max %d)" % mx, mx <= 2048)


def leg_cut():
    """the description cut falls inside a multi-byte character for some padding: 2-, 3- and 4-byte characters, four paddings each"""
    files = {}
    for pad in range(4):
        for name, ch in (("e2", "\u00e9"), ("e3", "\u20ac"), ("e4", "\U0001F600")):
            files["docs/issues/HELIX-2%d%s.md" % (pad, name)] = ticket(200 + pad, "open", "low", "ux", "Cut %s %d" % (name, pad), ("a" * pad) + ch * 900 + "\n")
    e = Env("cut", files=files)
    rc, out = e.imp()
    check("CUT import of 12 long multi-byte tickets exits 0", rc == 0, out)
    rows = e.q("SELECT x.locator, CAST(i.description AS BLOB) FROM items i JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries x USING(entry_id)")
    badrows = []
    for loc, raw in rows:
        try:
            txt = raw.decode("utf-8")
        except UnicodeDecodeError:
            badrows.append((loc, "invalid UTF-8"))
            continue
        body = open(os.path.join(e.tree, loc), encoding="utf-8").read().split("\n\n# ", 1)[1].split("\n\n", 1)[1].strip()
        kept = txt.split("\n\n**Sources**", 1)[0]
        if len(raw) > 2048 or "\ufffd" in txt or not body.startswith(kept) or len(kept) >= len(body) or "cut at" not in txt:
            badrows.append((loc, len(raw), "\ufffd" in txt, body.startswith(kept), len(kept), len(body)))
    check("CUT all %d stored descriptions: <= 2048 bytes, strict UTF-8, no U+FFFD, a proper prefix of the body, marked as cut" % len(rows), len(rows) == 12 and not badrows, badrows)
    e2 = Env("validate")
    c = sqlite3.connect(e2.db)
    c.execute("INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('planted','manual')")
    c.execute("INSERT INTO items(atm_id,type,status,severity,title,description,current_location,representation) VALUES ('CAT-001','Bug','Queued','High','planted','short','Issues','section')")
    c.commit()
    c.close()
    rc, out = e2.imp()
    check("CUT a register that the engine's validate refuses (a planted invalid item) ends the import with exit 1 validate_failed", rc == 1 and reason(out) == "validate_failed", "rc=%s %s" % (rc, out))


def main():
    global CLEAN
    try:
        assert os.path.isfile(IMP), "importer absent: %s" % IMP
    except AssertionError as ex:
        bad("importer present", ex)
        print("RESULT: pass=%d fail=%d" % (PASS, FAIL))
        return 1
    CLEAN = clean_state()
    legs = [leg_freeze, leg_entry, leg_duplicate, leg_resume, leg_concurrent, leg_unmapped, leg_locked, leg_disk, leg_backup, leg_content, leg_cut]
    only = os.environ.get("ONLY")
    for lg in legs:
        if only and only not in lg.__name__:
            continue
        print("== " + lg.__name__)
        try:
            lg()
        except Exception as ex:           # a crashed leg is a failure, never a skipped one
            import traceback
            bad(lg.__name__ + " crashed", traceback.format_exc())
    shutil.rmtree(SCR, ignore_errors=True)
    print("RESULT: pass=%d fail=%d" % (PASS, FAIL))
    return 1 if FAIL else 0


if __name__ == "__main__":
    sys.exit(main())
