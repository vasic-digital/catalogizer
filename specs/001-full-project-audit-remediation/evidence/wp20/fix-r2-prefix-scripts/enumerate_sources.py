#!/usr/bin/env python3
"""enumerate_sources.py - Stage 0 source enumerator of the findings register (WP-20, tasks T165; doc03 section 10
step 2, promoted from doc04 section 14.4 scan_issue_files.py).

Reads ONLY the frozen snapshot named by the required --freeze-json (never the live tree, never a database).
Its entry runs scripts/register/check_freeze_snapshot.sh on the snapshot and manifest the freeze json names and
stops with `freeze_snapshot_moved` BEFORE it reads any snapshot file. It never opens a database: it writes
  <out>/source_entries.sql          INSERT OR IGNORE rows for reg_sources and reg_source_entries (+ count updates)
  <out>/source_entries.sql.sha256   the bare 64-hex sha256 of the .sql file and a newline (no file name)
  <out>/source-class-kinds.json     class -> reg_sources.kind mapping (closed set of 9) and the selection rules
  <out>/enumeration-stats.json      per-class file and entry counts, skipped files (honest gaps)
and nothing else; a refusal writes no file under <out>.

Usage: enumerate_sources.py --freeze-json <path> --out <dir> [--check-script <path>] [--list-classes]
Freeze json (UNCONFIRMED vs the T161 freeze.json, which is not written yet): {"snapshot": "<abs dir>",
  "manifest": "<abs manifest path>", "frozen_at": "<UTC time, optional>", "head": "<sha, optional>"}.
Exit codes: 0 ok; 2 usage; 20 refusal (`enumerate_sources: REFUSED reason=<code> ...` on stderr).
Determinism: sources and entries are emitted in a fixed order; two runs over the same snapshot give a
byte-identical .sql. Rules version: enumerate_sources.py@1 (the `parser` column of every source row).
"""
import fnmatch
import hashlib
import json
import os
import re
import subprocess
import sys

PARSER = "enumerate_sources.py@1"
KINDS = ("issue_file", "md_tracker", "report_doc", "qa_bank", "qa_results", "workable_items_db",
         "external_ticket", "constitution_conflict", "code_marker")
GOVERNANCE_ROOT = ("AGENTS.md", "CLAUDE.md", "CONSTITUTION.md", "GEMINI.md", "GETTING_STARTED.md",
                   "MEMORY.md", "QUICK_REFERENCE.md", "README.md")
MARKER_EXT = {".go", ".kt", ".kts", ".java", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".py", ".rs", ".sh", ".md",
              ".yaml", ".yml", ".json", ".sql", ".c", ".h", ".cpp", ".cc", ".hpp", ".swift", ".gradle", ".toml",
              ".html", ".css", ".vue", ".txt", ".bash"}
MAX_SCAN_BYTES = 2 * 1024 * 1024
MARKER_RE = re.compile(r"\b(TODO|FIXME|HACK|XXX)\b")
SKIP_RES = (
    ("go", re.compile(r"\bt\.(?:Skip|SkipNow|Skipf)\(")),
    ("go", re.compile(r"\b(?:b|tb)\.(?:Skip|SkipNow|Skipf)\(")),
    ("ts", re.compile(r"(?<![\w.])(?:it|test|describe)\.skip\(")),
    ("ts", re.compile(r"(?<![\w.$])(?:xit|xdescribe|xtest)\(")),
    ("kt", re.compile(r"@Ignore\b")),
    ("rs", re.compile(r"#\[ignore\b")),
    ("py", re.compile(r"@pytest\.mark\.skip\b|@unittest\.skip\b")),
)
CHECKBOX_RE = re.compile(r"^\s*[-*+]\s+\[( |x|X)\]\s+(.*)$")
CROSS_RE = re.compile("[❌✗✘]")
WARN_RE = re.compile("⚠")
ID_RES = (re.compile(r"CATAPI-DEFECT-\d+"), re.compile(r"(?:FIX|DEFER)-QA-\d{4}-\d{2}-\d{2}-\d{3}"),
          re.compile(r"FIX-OC\d+-\d+"), re.compile(r"FINDING-\d+"))
STATUS_WORDS_TASK = ("Not Started", "In Progress", "Complete", "Blocked")
STATUS_WORDS_CONFLICT = ("FIXED", "DECIDED", "OPEN", "NOTE", "UNCONFIRMED")


class Refusal(Exception):
    def __init__(self, reason, detail=""):
        Exception.__init__(self, reason)
        self.reason, self.detail = reason, detail


def refuse(reason, detail=""):
    raise Refusal(reason, detail)


def sha_bytes(b):
    return hashlib.sha256(b).hexdigest()


def sha_text(s):
    return sha_bytes(s.encode("utf-8"))


def clean(s, n=200):
    s = re.sub(r"[\x00-\x1f\x7f]", " ", s or "").strip()
    return s[:n]


# ---------------------------------------------------------------------------------------------- front matter
def parse_frontmatter(text):
    """Tolerant front-matter reader: returns (dict, body, errors). A value is everything after the first
    `key:` (a colon inside the value is data, which strict yaml.safe_load refuses in the docs/issues corpus);
    indented continuation lines extend the previous value. errors lists reasons ('no_front_matter',
    'unterminated')."""
    lines = text.split("\n")
    lines = [l[:-1] if l.endswith("\r") else l for l in lines]
    if not lines or lines[0].strip() != "---":
        return {}, text, ["no_front_matter"]
    end = None
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            end = i
            break
    if end is None:
        return {}, text, ["unterminated"]
    fm, last = {}, None
    for l in lines[1:end]:
        m = re.match(r"^([A-Za-z_][\w\-]*):(?:[ \t]+(.*))?$", l)
        if m:
            last = m.group(1)
            v = (m.group(2) or "").strip()
            if len(v) >= 2 and v[0] == v[-1] and v[0] in "\"'":
                v = v[1:-1]
            fm[last] = v
        elif last and l.startswith((" ", "\t")):
            fm[last] = (fm[last] + " " + l.strip()).strip()
    return fm, "\n".join(lines[end + 1:]), []


def first_heading(body):
    for l in body.split("\n"):
        if l.startswith("# "):
            return l[2:].strip()
    return None


# ---------------------------------------------------------------------------------------------- snapshot
class Snapshot:
    def __init__(self, root):
        self.root = root
        self.files = []        # sorted posix relative paths (regular files and symlinks)
        self.links = set()
        rb = os.fsencode(root)
        acc = []
        for dp, dns, fns in os.walk(rb, followlinks=False):
            for n in list(dns) + list(fns):
                full = os.path.join(dp, n)
                if os.path.islink(full) or n in fns:
                    rel = os.path.relpath(full, rb)
                    try:
                        relt = rel.decode("utf-8")
                    except UnicodeDecodeError:
                        refuse("path_not_utf8", repr(rel))
                    acc.append(relt)
                    if os.path.islink(full):
                        self.links.add(relt)
        self.files = sorted(set(acc), key=lambda p: p.encode("utf-8"))
        self._h, self._t = {}, {}

    def abspath(self, p):
        return os.path.join(self.root, p)

    def sha(self, p):
        if p not in self._h:
            full = self.abspath(p)
            if p in self.links:
                self._h[p] = sha_bytes(os.fsencode(os.readlink(full)))
            else:
                h = hashlib.sha256()
                with open(full, "rb") as f:
                    for c in iter(lambda: f.read(1 << 20), b""):
                        h.update(c)
                self._h[p] = h.hexdigest()
        return self._h[p]

    def raw(self, p):
        with open(self.abspath(p), "rb") as f:
            return f.read()

    def text(self, p):
        if p not in self._t:
            self._t[p] = "" if p in self.links else self.raw(p).decode("utf-8", "replace")
        return self._t[p]


def gl(path, pattern):
    """glob on posix paths: `*` does not cross `/`, `**` does."""
    rx = ""
    i = 0
    while i < len(pattern):
        c = pattern[i]
        if pattern.startswith("**", i):
            rx += ".*"
            i += 2
            if pattern.startswith("/", i):
                rx = rx[:-2] + "(?:.*/)?"
                i += 1
            continue
        rx += "[^/]*" if c == "*" else ("[^/]" if c == "?" else re.escape(c))
        i += 1
    return re.match("^" + rx + "$", path) is not None


class E:
    __slots__ = ("locator", "legacy_id", "title", "status", "severity", "sha")

    def __init__(self, locator, sha, legacy_id=None, title=None, status=None, severity=None):
        self.locator, self.sha, self.legacy_id = locator, sha, legacy_id
        self.title, self.status, self.severity = clean(title) if title else None, status, severity


# ---------------------------------------------------------------------------------------------- entry rules
def file_entry(snap, p, **kw):
    return E(p, snap.sha(p), **kw)


def lines_of(snap, p):
    t = snap.text(p)
    return [l[:-1] if l.endswith("\r") else l for l in t.split("\n")]


def rule_issue_file(snap, files, st):
    out = []
    for p in files:
        fm, body, errs = parse_frontmatter(snap.text(p))
        if errs:
            st.setdefault("frontmatter_problems", []).append([p] + errs)
        out.append(file_entry(snap, p, legacy_id=fm.get("id") or None, title=first_heading(body),
                              status=fm.get("status") or None, severity=fm.get("severity") or None))
    return out


def rule_anr(snap, files, st):
    out = []
    for p in files:
        t = snap.text(p)
        m = re.search(r"ANR-\d{4}-\d{2}-\d{2}-\d{3}", t)
        s = re.search(r"(?im)^\W*status\W{0,4}\s*[:\-]?\s*\**\s*([A-Za-z ]{3,20})", t)
        out.append(file_entry(snap, p, legacy_id=m.group(0) if m else None, title=first_heading(t) if t.startswith("#") else
                              first_heading(t), status=(s.group(1).strip() if s else None)))
    return out


def rule_task_rows(snap, files, st):
    out = []
    for p in files:
        for n, l in enumerate(lines_of(snap, p), 1):
            if re.match(r"^\| *[0-9]+\.[0-9]+", l):
                cells = [c.strip() for c in l.strip().strip("|").split("|")]
                status = next((c for c in cells if c in STATUS_WORDS_TASK), None)
                out.append(E("%s:L%d" % (p, n), sha_text(l), legacy_id=cells[0],
                             title=cells[1] if len(cells) > 1 else None, status=status))
    return out


def marks_re(*res):
    return lambda l: any(r.search(l) for r in res)


def make_rule_lines(checkbox=False, marks=()):
    """one entry per checkbox line (if checkbox) and per mark line; a line is one entry."""
    mk = marks_re(*marks) if marks else None

    def rule(snap, files, st):
        out = []
        for p in files:
            for n, l in enumerate(lines_of(snap, p), 1):
                m = CHECKBOX_RE.match(l) if checkbox else None
                if m:
                    out.append(E("%s:L%d" % (p, n), sha_text(l), title=m.group(2),
                                 status="checked" if m.group(1) in "xX" else "unchecked"))
                elif mk and mk(l):
                    out.append(E("%s:L%d" % (p, n), sha_text(l), title=l.strip(),
                                 status="cross" if CROSS_RE.search(l) else "warn"))
        return out
    return rule


def make_rule_files(checkbox=False, ids=False):
    """one entry per file, plus checkbox lines and/or distinct-id entries (id deduped across the source)."""
    def rule(snap, files, st):
        out, seen = [], st.setdefault("_seen_ids", set())
        for p in files:
            out.append(file_entry(snap, p, title=first_heading(snap.text(p)) if p.endswith(".md") else None))
            if checkbox or ids:
                for n, l in enumerate(lines_of(snap, p), 1):
                    m = CHECKBOX_RE.match(l) if checkbox else None
                    if m:
                        out.append(E("%s:L%d" % (p, n), sha_text(l), title=m.group(2),
                                     status="checked" if m.group(1) in "xX" else "unchecked"))
                    if ids:
                        for rx in ID_RES:
                            for mm in rx.finditer(l):
                                if mm.group(0) not in seen:
                                    seen.add(mm.group(0))
                                    out.append(E("%s:L%d#%s" % (p, n, mm.group(0)), sha_text(p + "\0" + l),
                                                 legacy_id=mm.group(0), title=l.strip()))
        return out
    return rule


def rule_landmines(snap, files, st):
    out, seen = [], set()
    for p in files:
        for n, l in enumerate(lines_of(snap, p), 1):
            for mm in re.finditer(r"RULE-[A-Za-z0-9]+-\d+", l):
                if mm.group(0) not in seen:
                    seen.add(mm.group(0))
                    out.append(E("%s:L%d#%s" % (p, n, mm.group(0)), sha_text(p + "\0" + l),
                                 legacy_id=mm.group(0), title=l.strip()))
    return out


def walk_cases(obj, path=""):
    """yield (key, dict) for every list element that is a dict with an `id`/`name` found anywhere in obj."""
    if isinstance(obj, list) and obj and all(isinstance(x, dict) for x in obj) and any(x.get("id") for x in obj):
        for x in obj:
            if x.get("id"):
                yield str(x["id"]), x
        return
    if isinstance(obj, dict):
        for k, v in obj.items():
            for r in walk_cases(v, path + "/" + str(k)):
                yield r
    elif isinstance(obj, list):
        for v in obj:
            for r in walk_cases(v, path):
                yield r


def rule_bank(snap, files, st):
    out = []
    try:
        import yaml
    except ImportError:
        yaml = None
    for p in files:
        out.append(file_entry(snap, p, title=os.path.basename(p)))
        if os.path.splitext(p)[1] not in (".yaml", ".yml", ".json") or p in snap.links:
            continue
        data, err = None, None
        try:
            if p.endswith(".json"):
                data = json.loads(snap.text(p))
            elif yaml is not None:
                data = yaml.safe_load(snap.text(p))
            else:
                err = "pyyaml_missing"
        except Exception as e:  # unparsable bank: the file row stays, the case rows are regex-found below
            err = type(e).__name__
        seen = {}
        if err is None:
            cases = list(walk_cases(data))
        else:
            st.setdefault("bank_parse_errors", []).append([p, err])
            cases = [(m.group(1).strip("'\""), {"id": m.group(1)}) for m in
                     re.finditer(r"(?m)^\s*-?\s*id:\s*(\S+)\s*$", snap.text(p))]
        for cid, c in cases:
            seen[cid] = seen.get(cid, 0) + 1
            loc = "%s#%s" % (p, cid) + ("~%d" % seen[cid] if seen[cid] > 1 else "")
            out.append(E(loc, sha_text(json.dumps(c, sort_keys=True, default=str)), legacy_id=cid,
                         title=(c.get("name") or c.get("title")) if isinstance(c, dict) else None))
    return out


def rule_bluff_and_anchors(snap, files, st):
    out, seen = [], set()
    for p in files:
        if p.endswith("bluff-baseline.txt"):
            for n, l in enumerate(lines_of(snap, p), 1):
                if l.strip() and not l.lstrip().startswith("#"):
                    out.append(E("%s:L%d" % (p, n), sha_text(l), title=l.strip()))
        else:
            for n, l in enumerate(lines_of(snap, p), 1):
                m = re.match(r"^\|\s*(CAP-\d+)\s*\|", l)
                if m and m.group(1) not in seen:
                    seen.add(m.group(1))
                    out.append(E("%s:L%d#%s" % (p, n, m.group(1)), sha_text(l), legacy_id=m.group(1),
                                 title=l.strip("| "), status=("pending" if "pending" in l else None)))
    return out


def rule_conflicts(snap, files, st):
    out = []
    for p in files:
        ls = lines_of(snap, p)
        start = next((i for i, l in enumerate(ls) if re.match(r"^##\s+Known Conflicts", l)), None)
        if start is None:
            st.setdefault("conflicts_heading_missing", []).append(p)
            continue
        item, sub = 0, 0
        for i in range(start + 1, len(ls)):
            l = ls[i]
            if re.match(r"^##\s", l):
                break
            m = re.match(r"^(\d+)\.\s+(.*)$", l)
            if m:
                item, sub = int(m.group(1)), 0
                words = [w for w in STATUS_WORDS_CONFLICT if re.search(r"\b%s\b" % w, m.group(2)[:400])]
                out.append(E("%s:L%d#item%d" % (p, i + 1, item), sha_text(l), legacy_id=str(item),
                             title=m.group(2), status=words[0] if words else None))
            elif item and re.match(r"^\s{2,}(?:[-*]|\d+[.)]|\([a-z0-9]+\)|[a-z]\))\s+", l):
                sub += 1
                words = [w for w in STATUS_WORDS_CONFLICT if re.search(r"\b%s\b" % w, l)]
                out.append(E("%s:L%d#item%d.%d" % (p, i + 1, item, sub), sha_text(l), legacy_id="%d.%d" % (item, sub),
                             title=l.strip(), status=words[0] if words else None))
    return out


def rule_providers(snap, files, st):
    """S-25: provider rows of the rotation action list (names only: no line text is stored, never a value)
    plus one file entry for every other S-25 document."""
    out = []
    for p in files:
        if os.path.basename(p) == "SECURITY_KEY_ROTATION_REQUIRED.md":
            hdr = False
            for n, l in enumerate(lines_of(snap, p), 1):
                if re.match(r"^\|\s*Provider\s*\|", l):
                    hdr = True
                    continue
                if hdr and l.startswith("|") and not re.match(r"^\|[\s:|-]+\|?\s*$", l):
                    cells = [c.strip() for c in l.strip().strip("|").split("|")]
                    var = re.search(r"`([A-Z0-9_]+)`", cells[1]) if len(cells) > 1 else None
                    out.append(E("%s:L%d#%s" % (p, n, cells[0]), sha_text(p + "\0" + l), legacy_id=var.group(1) if var else None,
                                 title=cells[0]))
                elif hdr and not l.startswith("|"):
                    hdr = False
        else:
            out.append(file_entry(snap, p, title=first_heading(snap.text(p)) if p.endswith(".md") else None))
    return out


SCAN_TS = re.compile(r"^(?P<tool>.+?)-(?P<ts>\d{8}_\d{6})(?P<ext>\.[a-z]+)$")


def rule_security(snap, files, st):
    """S-16: a file row for every document, plus vulnerability records of the LATEST scan file per tool and
    app and a scan-failed row for a failed latest scan (snyk errors)."""
    out, latest = [], {}
    for p in files:
        out.append(file_entry(snap, p))
        m = SCAN_TS.match(os.path.basename(p))
        if m and m.group("ext") in (".json", ".txt"):
            k = (m.group("tool"), m.group("ext"))
            if k not in latest or m.group("ts") > latest[k][0]:
                latest[k] = (m.group("ts"), p)
    for k in sorted(latest):
        p = latest[k][1]
        tool = k[0]
        try:
            if p.endswith(".txt"):
                for n, l in enumerate(lines_of(snap, p), 1):
                    mm = re.match(r"^Vulnerability #\d+:\s*(\S+)", l)
                    if mm:
                        out.append(E("%s:L%d#%s" % (p, n, mm.group(1)), sha_text(p + "\0" + l), legacy_id=mm.group(1), title=l))
                continue
            data = json.loads(snap.text(p) or "null")
        except Exception as e:
            out.append(E("%s#scan-failed" % p, snap.sha(p), title="unparsable scan file: " + type(e).__name__, status="scan-failed"))
            continue
        recs = []
        if tool.startswith("gosec") and isinstance(data, dict):
            for i, it in enumerate(data.get("Issues") or []):
                recs.append(("%d:%s:%s:%s" % (i, it.get("rule_id"), it.get("file"), it.get("line")), it))
        elif tool.startswith("npm-audit") and isinstance(data, dict):
            v = data.get("vulnerabilities") or data.get("advisories") or {}
            for name in sorted(v) if isinstance(v, dict) else []:
                recs.append(("vuln:" + str(name), v[name]))
        elif tool.startswith("nancy") and isinstance(data, dict):
            for i, it in enumerate(data.get("vulnerable") or []):
                recs.append(("%d:%s" % (i, it.get("Coordinates") if isinstance(it, dict) else it), it))
        elif tool.startswith("snyk") and isinstance(data, dict):
            if data.get("error") or data.get("ok") is False and not data.get("vulnerabilities"):
                out.append(E("%s#scan-failed" % p, snap.sha(p), title=clean(str(data.get("error") or "ok=false")), status="scan-failed"))
            for i, it in enumerate(data.get("vulnerabilities") or []):
                recs.append(("%d:%s" % (i, it.get("id") if isinstance(it, dict) else it), it))
        for rid, rec in recs:
            out.append(E("%s#%s" % (p, rid), sha_text(json.dumps(rec, sort_keys=True, default=str)), legacy_id=rid.split(":")[-1][:80] or None))
    return out


def rule_files_only(snap, files, st):
    return [file_entry(snap, p, title=first_heading(snap.text(p)) if p.endswith(".md") else None) for p in files]


def rule_markers(snap, files, st):
    out, skipped = [], {"too_large": 0, "binary": 0, "symlink": 0}
    for p in files:
        if os.path.splitext(p)[1].lower() not in MARKER_EXT:
            continue
        if p in snap.links:
            skipped["symlink"] += 1
            continue
        try:
            if os.path.getsize(snap.abspath(p)) > MAX_SCAN_BYTES:
                skipped["too_large"] += 1
                continue
        except OSError:
            continue
        raw = snap.raw(p)
        if b"\0" in raw[:8192]:
            skipped["binary"] += 1
            continue
        t = raw.decode("utf-8", "replace")
        if not MARKER_RE.search(t) and not any(r.search(t) for _, r in SKIP_RES):
            continue
        for n, l in enumerate(t.split("\n"), 1):
            l = l[:-1] if l.endswith("\r") else l
            seen = {}
            for m in MARKER_RE.finditer(l):      # a repeated marker on one line is one row per occurrence: #2, #3, ...
                k = seen[m.group(1)] = seen.get(m.group(1), 0) + 1
                out.append(E("%s:L%d:%s" % (p, n, m.group(1)) + ("#%d" % k if k > 1 else ""), sha_text(p + "\0" + l), title=l.strip(), status="marker"))
            for tag, r in SKIP_RES:
                for k, _m in enumerate(r.finditer(l), 1):
                    out.append(E("%s:L%d:skip" % (p, n) + ("" if tag == "go" else "-" + tag) + ("#%d" % k if k > 1 else ""), sha_text(p + "\0" + l), title=l.strip(), status="skipped_test"))
    st["skipped"] = skipped
    return out


# ---------------------------------------------------------------------------------------------- class table
def root_other_reports(files, claimed):
    return [p for p in files if "/" not in p and p.endswith(".md") and p not in GOVERNANCE_ROOT and p not in claimed]


def sel_glob(*pats):
    return lambda files, claimed: [p for p in files if p not in claimed and any(gl(p, x) for x in pats)]


def sel_exact(*names):
    return lambda files, claimed: [p for p in files if p in names and p not in claimed]


def sel_s22(files, claimed):
    return [p for p in files if p not in claimed and os.path.basename(p) in ("CLAUDE.md", "AGENTS.md")
            and "/" in p and not p.startswith(("submodules/", ".audit/"))]


def sel_all(files, claimed):
    return list(files)


# (class, kind, source locator, selector, rule). Order matters: the first source that selects a file claims it.
SOURCES = [
    ("S-25", "report_doc", "S-25:SECURITY_KEY_ROTATION_REQUIRED.md", sel_exact("SECURITY_KEY_ROTATION_REQUIRED.md"), rule_providers),
    ("S-25", "report_doc", "S-25:SECURITY_AUDIT_REPORT.md", sel_exact("SECURITY_AUDIT_REPORT.md"), rule_providers),
    ("S-25", "report_doc", "docs/security/firebase-api-key-exposure-20260629.md", sel_exact("docs/security/firebase-api-key-exposure-20260629.md"), rule_providers),
    ("S-03", "md_tracker", "TASK_TRACKER.md", sel_exact("TASK_TRACKER.md"), rule_task_rows),
    ("S-04", "md_tracker", "MASTER_EXECUTION_CHECKLIST.md", sel_exact("MASTER_EXECUTION_CHECKLIST.md"), make_rule_lines(checkbox=True)),
    ("S-05", "md_tracker", "docs/MASTER_EXECUTION_CHECKLIST.md", sel_exact("docs/MASTER_EXECUTION_CHECKLIST.md"), make_rule_lines(checkbox=True)),
    ("S-06", "md_tracker", "COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md", sel_exact("COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md"), make_rule_lines(checkbox=True)),
    ("S-07", "md_tracker", "COMPREHENSIVE_UNFINISHED_WORK_REPORT.md", sel_exact("COMPREHENSIVE_UNFINISHED_WORK_REPORT.md"), make_rule_lines(checkbox=True, marks=(CROSS_RE,))),
    ("S-08", "report_doc", "docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md", sel_exact("docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md"), make_rule_lines(marks=(CROSS_RE, WARN_RE))),
    ("S-09", "report_doc", "UNFINISHED_WORK_ANALYSIS.md+UNFINISHED_WORK_AND_ISSUES.md+FINAL_UNFINISHED_WORK_REPORT.md",
     sel_exact("UNFINISHED_WORK_ANALYSIS.md", "UNFINISHED_WORK_AND_ISSUES.md", "FINAL_UNFINISHED_WORK_REPORT.md"), make_rule_lines(marks=(CROSS_RE,))),
    ("S-10", "md_tracker", "docs/OPEN_POINTS_CLOSURE.md", sel_exact("docs/OPEN_POINTS_CLOSURE.md"), make_rule_lines(checkbox=True)),
    ("S-11", "report_doc", "docs/LANDMINES.md", sel_exact("docs/LANDMINES.md"), rule_landmines),
    ("S-12", "report_doc", "/*.md (other root status reports)", None, make_rule_files(checkbox=True)),   # selector special-cased
    ("S-13", "report_doc", "docs/status/*.md", sel_glob("docs/status/*.md"), make_rule_files()),
    ("S-14", "report_doc", "docs/audits/*.md", sel_glob("docs/audits/*.md"), make_rule_files(ids=True)),
    ("S-14", "report_doc", "docs/*AUDIT*.md", sel_glob("docs/*AUDIT*.md"), make_rule_files(ids=True)),
    ("S-01", "issue_file", "docs/issues/*.md", sel_glob("docs/issues/*.md"), rule_issue_file),
    ("S-02", "issue_file", "issues/*.md", sel_glob("issues/*.md"), rule_anr),
    ("S-17", "qa_bank", "challenges/helixqa-banks/*.yaml", sel_glob("challenges/helixqa-banks/*.yaml"), rule_bank),
    ("S-18", "qa_bank", "challenges/data/challenges_bank.json", sel_exact("challenges/data/challenges_bank.json"), rule_bank),
    ("S-19", "qa_bank", "submodules/helix_qa/banks/**", sel_glob("submodules/helix_qa/banks/**"), rule_bank),
    ("S-20", "report_doc", "submodules/helix_qa/challenges/baselines/bluff-baseline.txt+submodules/helix_qa/docs/behavior-anchors.md",
     sel_exact("submodules/helix_qa/challenges/baselines/bluff-baseline.txt", "submodules/helix_qa/docs/behavior-anchors.md"), rule_bluff_and_anchors),
    ("S-21", "constitution_conflict", ".specify/memory/constitution.md", sel_exact(".specify/memory/constitution.md"), rule_conflicts),
    ("S-22", "report_doc", "module CLAUDE.md and AGENTS.md (outside submodules)", sel_s22, rule_files_only),
    ("S-24", "report_doc", ".implementation/**", sel_glob(".implementation/**"), rule_files_only),
    ("S-15", "qa_results", "docs/qa/**+docs/reports/qa-sessions/**", sel_glob("docs/qa/**", "docs/reports/qa-sessions/**"), make_rule_files(ids=True)),
    ("S-16", "report_doc", "docs/security/**", sel_glob("docs/security/**"), rule_security),
    ("S-23", "code_marker", "marker and skipped-test scan of every text file of the snapshot", sel_all, rule_markers),
]
CLASS_IDS = ["S-%02d" % i for i in range(1, 26)]


def sql_q(v):
    if v is None:
        return "NULL"
    return "'" + str(v).replace("'", "''") + "'"


def enumerate_snapshot(snap, frozen_at):
    claimed, stats, plan = {}, {"classes": {}, "frozen_at_basis": None}, []
    for cls, kind, loc, sel, rule in SOURCES:
        assert kind in KINDS
        if cls == "S-23":
            files = sel(snap.files, {})
        elif loc.startswith("/*.md"):
            files = root_other_reports(snap.files, claimed)
        else:
            files = sel(snap.files, claimed)
        if cls != "S-23":
            for p in files:
                claimed[p] = loc
        st = {}
        ents = rule(snap, files, st)
        seen, uniq = set(), []
        for e in sorted(ents, key=lambda x: x.locator.encode("utf-8")):
            if e.locator in seen:
                refuse("duplicate_entry_locator", "%s %s" % (loc, e.locator))
            seen.add(e.locator)
            uniq.append(e)
        csha = sha_text("".join("%s\t%s\n" % (p, snap.sha(p)) for p in files))
        plan.append((cls, kind, loc, files, uniq, csha))
        c = stats["classes"].setdefault(cls, {"sources": [], "files": 0, "entries": 0})
        c["sources"].append(loc)
        c["files"] += len(files)
        c["entries"] += len(uniq)
        for k in ("frontmatter_problems", "bank_parse_errors", "conflicts_heading_missing", "skipped"):
            if k in st:
                c.setdefault(k, st[k])
        if not files and cls != "S-23":
            c["empty_source"] = c.get("empty_source", []) + [loc]
    stats["root_files_not_sources"] = [p for p in snap.files if "/" not in p and p.endswith(".md") and p in GOVERNANCE_ROOT]
    stats["total_entries"] = sum(c["entries"] for c in stats["classes"].values())
    return plan, stats


def render_sql(plan, scanned_at):
    out = ["-- generated by enumerate_sources.py@1; INSERT OR IGNORE only (idempotent); no DELETE, no UPDATE of an entry",
           "PRAGMA foreign_keys=ON;", "BEGIN;"]
    for cls, kind, loc, files, ents, csha in plan:
        out.append("INSERT OR IGNORE INTO reg_sources(kind,locator,parser) VALUES (%s,%s,%s);" % (sql_q(kind), sql_q(loc), sql_q(PARSER)))
    for cls, kind, loc, files, ents, csha in plan:
        sid = "(SELECT source_id FROM reg_sources WHERE locator=%s)" % sql_q(loc)
        for e in ents:
            out.append("INSERT OR IGNORE INTO reg_source_entries(source_id,locator,legacy_id,title,raw_status,raw_severity,entry_sha256,scanned_at) "
                       "VALUES (%s,%s,%s,%s,%s,%s,%s,%s);" % (sid, sql_q(e.locator), sql_q(e.legacy_id), sql_q(e.title), sql_q(e.status),
                                                              sql_q(e.severity), sql_q(e.sha), sql_q(scanned_at)))
        out.append("UPDATE reg_sources SET content_sha256=%s, last_scanned_at=%s, scanned_entry_count=%d WHERE locator=%s;" %
                   (sql_q(csha), sql_q(scanned_at), len(ents), sql_q(loc)))
    out.append("COMMIT;")
    return "\n".join(out) + "\n"


def kinds_doc():
    cl = {}
    for cls, kind, loc, sel, rule in SOURCES:
        cl.setdefault(cls, {"kind": kind, "sources": [], "rule": rule.__name__})
        if cl[cls]["kind"] != kind:
            cl[cls]["kind"] = kind
        cl[cls]["sources"].append(loc)
    missing = [c for c in CLASS_IDS if c not in cl]
    return {"schema": "source-class-kinds/1", "closed_kind_set": list(KINDS), "classes": cl, "classes_missing": missing,
            "parser": PARSER}


def run(argv):
    a = {"freeze": None, "out": None, "check": None, "list": False}
    i = 0
    while i < len(argv):
        k = argv[i]
        if k == "--freeze-json" and i + 1 < len(argv):
            a["freeze"], i = argv[i + 1], i + 2
        elif k == "--out" and i + 1 < len(argv):
            a["out"], i = argv[i + 1], i + 2
        elif k == "--check-script" and i + 1 < len(argv):
            a["check"], i = argv[i + 1], i + 2
        elif k == "--list-classes":
            a["list"], i = True, i + 1
        else:
            sys.stderr.write("enumerate_sources: usage: --freeze-json <path> --out <dir> [--check-script <path>] | --list-classes\n")
            return 2
    if a["list"]:
        json.dump(kinds_doc(), sys.stdout, indent=1, sort_keys=True)
        print()
        return 0
    if not a["freeze"] or not a["out"]:
        sys.stderr.write("enumerate_sources: usage: --freeze-json <path> --out <dir> is required (no default snapshot, never the live tree)\n")
        return 2
    try:
        try:
            fj = json.load(open(a["freeze"], encoding="utf-8"))
            snapdir, manifest = fj["snapshot"], fj["manifest"]
        except (OSError, ValueError, KeyError) as e:
            refuse("freeze_json_invalid", "%s: %s" % (a["freeze"], type(e).__name__))
        if not os.path.isdir(snapdir) or not os.path.isfile(manifest):
            refuse("freeze_json_invalid", "snapshot or manifest path does not exist")
        chk = a["check"] or os.path.join(os.path.dirname(os.path.abspath(__file__)), "check_freeze_snapshot.sh")
        # ENTRY CHECK: before any snapshot file is read (round-31 review I1)
        r = subprocess.run([chk, snapdir, manifest], stdout=subprocess.PIPE, stderr=subprocess.PIPE)  # MUT:entry-check
        if r.returncode != 0:
            sys.stderr.write(r.stderr.decode("utf-8", "replace"))
            refuse("freeze_snapshot_moved", "check_freeze_snapshot.sh exited %d" % r.returncode)
        snap = Snapshot(snapdir)
        scanned_at = fj.get("frozen_at") or "1970-01-01T00:00:00Z"
        plan, stats = enumerate_snapshot(snap, scanned_at)
        stats["frozen_at_basis"] = "freeze_json" if fj.get("frozen_at") else "constant_fallback"
        stats["freeze_head"] = fj.get("head")
        sqltxt = render_sql(plan, scanned_at)
    except Refusal as e:
        sys.stderr.write("enumerate_sources: REFUSED reason=%s %s\n" % (e.reason, e.detail))
        return 20
    os.makedirs(a["out"], exist_ok=True)
    sqlp = os.path.join(a["out"], "source_entries.sql")
    with open(sqlp, "w", encoding="utf-8", newline="\n") as f:
        f.write(sqltxt)
    with open(sqlp + ".sha256", "w") as f:
        f.write(sha_text(sqltxt) + "\n")
    json.dump(kinds_doc(), open(os.path.join(a["out"], "source-class-kinds.json"), "w"), indent=1, sort_keys=True)
    json.dump(stats, open(os.path.join(a["out"], "enumeration-stats.json"), "w"), indent=1, sort_keys=True)
    print("enumerate_sources: ok entries=%d sql_sha256=%s" % (stats["total_entries"], sha_text(sqltxt)))
    return 0


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:]))
