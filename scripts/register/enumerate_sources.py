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

Usage: enumerate_sources.py --freeze-json <path> --out <dir> [--list-classes]
There is NO option that replaces the mandatory entry check (round-23 review B3): the check is always
scripts/register/check_freeze_snapshot.sh beside this file.
Freeze json (UNCONFIRMED vs the T161 freeze.json, which is not written yet): {"snapshot": "<abs dir>",
  "manifest": "<abs manifest path>", "remotes": [<name> | {"name":..,"url":..}, ...] (REQUIRED, may be []; T165 "remote and named
  service" needs it and the T161 text records none: a request to the T161 owner, see evidence wp20/fix-r2-open-requests.md),
  "frozen_at": "<UTC time, optional>", "head": "<sha, optional>"}. A remote url is stored without its userinfo (never a credential).
Exit codes: 0 ok; 2 usage; 20 refusal (`enumerate_sources: REFUSED reason=<code> ...` on stderr).
Determinism: sources and entries are emitted in a fixed order; two runs over the same snapshot give a
byte-identical .sql. PyYAML is REQUIRED (refused `pyyaml_missing` otherwise: the bank walk and the strict front-matter check differ
without it). Rules version: enumerate_sources.py@2 (the `parser` column of every source row).
Re-import safety: the .sql starts with `.bail on` and a gate that aborts the whole import when an entry already in the register has a
different entry_sha256 at the same locator (a changed line is never kept stale silently, round-23 review B2).
"""
import fnmatch
import hashlib
import json
import os
import re
import subprocess
import sys

PARSER = "enumerate_sources.py@2"
KINDS = ("issue_file", "md_tracker", "report_doc", "qa_bank", "qa_results", "workable_items_db",
         "external_ticket", "constitution_conflict", "code_marker")
EXTRA_CLASSES = ("XID", "EXT")     # not doc03 source classes: the cross-source defect-id index and the external sources
GOVERNANCE_ROOT = ("AGENTS.md", "CLAUDE.md", "CONSTITUTION.md", "GEMINI.md", "GETTING_STARTED.md",
                   "MEMORY.md", "QUICK_REFERENCE.md", "README.md")
MAX_SCAN_BYTES = 2 * 1024 * 1024
MARKER_RE = re.compile(r"\b(TODO|FIXME|HACK|XXX)\b")
# the tag is part of the locator suffix; every regex of one tag shares ONE hit counter per line (round-23 review B1)
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
_B = r"(?<![A-Za-z0-9-])"
# Defect-id families. Census (git grep over the HEAD snapshot, 2026-10-08, specs/ and submodules/ excluded): CATAPI-DEFECT-N,
# FIX-QA-date-N, DEFER-QA-date-N, DEFER-NNN, FIX-OCn-N, FINDING-N, HQA-DOCS-N, HQA-NNNN, HQA-PHASEn-X-N, FIX-OBS-N, FIX-BROWSER-N,
# FIX-NNN, BUG-NNN, FIX-CONCURRENCY-date, FIX-CATAPI-date-WORD. An id-like token no family matches is listed in the stats
# (`id_like_unmatched`), never dropped silently. The suffix variants (`DEFER-QA-...-memory-alerts`) match by their prefix.
ID_RES = tuple(re.compile(_B + x) for x in (
    r"CATAPI-DEFECT-\d+", r"(?:FIX|DEFER)-QA-\d{4}-\d{2}-\d{2}-\d{3}", r"DEFER-\d{3}", r"FIX-OC\d+-\d+", r"FINDING-\d+",
    r"HQA-DOCS-\d+", r"HQA-\d{4}", r"HQA-PHASE\d+-[A-Z]+-\d+", r"FIX-(?:OBS|BROWSER)-\d+", r"FIX-\d{3}", r"BUG-\d{3}",
    r"FIX-CONCURRENCY-\d{4}-\d{2}-\d{2}", r"FIX-CATAPI-\d{4}-\d{2}-\d{2}-[A-Z]+"))
ID_LIKE_RE = re.compile(_B + r"(?:FIX|DEFER|FINDING|HQA|CATAPI|BUG)(?:-[A-Za-z0-9]+)+")
# carriers: documents and tests that QUOTE ids as examples or fixtures are not sources of ids
ID_SCAN_EXCLUDE = ("specs/", ".audit/", "submodules/", "scripts/register/tests/")
SERVICES = (   # named external services (doc03 5.20) -> marker paths looked up in the snapshot ("dir/" = any file below it)
    ("firebase-crashlytics", (".firebaserc", "firebase.json")),
    ("sonarqube", ("sonar-project.properties", "sonarqube/")),
    ("snyk", (".snyk", ".snyk.json")),
    ("trivy", (".trivyignore", ".trivy.yaml", "config/trivy/")),
)
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
KEY_RE = re.compile(r"^([A-Za-z_][\w\-]*)[ \t]*:(?:[ \t]+(.*))?$")
BLOCK_RE = re.compile(r"^[>|][+\-0-9]*$")


def _scalar(v):
    """one plain, quoted or block-free scalar: unquote, drop a trailing ` #comment` of an unquoted value (YAML rule: `#` after
    white space starts a comment)."""
    v = v.strip()
    if len(v) >= 2 and v[0] in "\"'":
        q = v[0]
        end = v.find(q, 1)
        while q == "'" and end != -1 and v[end + 1:end + 2] == "'":      # '' is an escaped quote inside single quotes
            end = v.find(q, end + 2)
        if end != -1 and (v[end + 1:].strip() == "" or v[end + 1:].lstrip().startswith("#")):
            inner = v[1:end]
            return inner.replace("''", "'") if q == "'" else inner
    return re.sub(r"[ \t]+#.*$", "", v).strip()


def parse_frontmatter(text):
    """Tolerant front-matter reader: returns (dict, body, errors). It is NOT a YAML parser (399 of the 1,778 docs/issues
    blocks are not valid YAML: an unquoted colon inside a plain value, see strict_frontmatter and OD-17). It reads what YAML reads
    for every form that occurs and what YAML refuses for the colon-in-value case: `key: value`, `key : value`, quoted values,
    a trailing ` # comment`, block scalars (`>` folds, `|` keeps newlines) and indented continuation lines of a plain value.
    A colon inside a value is data. errors lists reasons ('no_front_matter', 'unterminated')."""
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
    fm, last, block, parts = {}, None, None, []

    def close():
        nonlocal block, parts
        if block is not None and last is not None:
            fm[last] = (" " if block[0] == ">" else "\n").join(parts).strip()
        block, parts = None, []
    for l in lines[1:end]:
        if block is not None:
            if l.startswith((" ", "\t")) or not l.strip():
                parts.append(l.strip())
                continue
            close()
        m = KEY_RE.match(l)
        if m:
            last = m.group(1)
            raw = (m.group(2) or "").strip()
            if BLOCK_RE.match(raw):
                block, parts = raw, []
                fm[last] = ""
            else:
                fm[last] = _scalar(raw)
        elif last and l.startswith((" ", "\t")):
            fm[last] = (fm[last] + " " + l.strip()).strip()
    close()
    return fm, "\n".join(lines[end + 1:]), []


def strict_frontmatter(text):
    """Strict reader: yaml.safe_load of the block between the first two `---` lines -> (dict or None, error name or None).
    Used only to MEASURE how many blocks a real YAML parser refuses (a fact, OD-17); no entry field comes from it."""
    import yaml
    lines = [l[:-1] if l.endswith("\r") else l for l in text.split("\n")]
    if not lines or lines[0].strip() != "---":
        return None, "no_front_matter"
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            try:
                d = yaml.safe_load("\n".join(lines[1:i]))
            except Exception as e:
                return None, type(e).__name__
            return (d, None) if isinstance(d, dict) else (None, "not_a_mapping")
    return None, "unterminated"


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
        self.freeze = {}          # the freeze json (remotes), set by run()
        self.class_files = {}     # class -> set of files its sources selected (filled in SOURCES order)
        self._ids = None

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

    def scannable(self, p):
        """-> (text or None, skip reason or None): a regular, non-binary file of at most MAX_SCAN_BYTES (any extension)."""
        if p in self.links:
            return None, "symlink"
        try:
            if os.path.getsize(self.abspath(p)) > MAX_SCAN_BYTES:
                return None, "too_large"
        except OSError:
            return None, "unreadable"
        raw = self.raw(p)
        if b"\0" in raw[:8192]:
            return None, "binary"
        return raw.decode("utf-8", "replace"), None

    def id_index(self):
        """distinct defect id -> {"occ": [(path, line_no, line)], "files": n} over every scannable text file outside ID_SCAN_EXCLUDE;
        plus the id-like tokens no family matches (the census residue)."""
        if self._ids is None:
            idx, unmatched, skipped = {}, {}, {}
            for p in self.files:
                if p.startswith(ID_SCAN_EXCLUDE):
                    continue
                t, why = self.scannable(p)
                if t is None:
                    skipped[why] = skipped.get(why, 0) + 1
                    continue
                if not ID_LIKE_RE.search(t):
                    continue
                for n, l in enumerate(t.split("\n"), 1):
                    l = l[:-1] if l.endswith("\r") else l
                    hits = set()
                    for rx in ID_RES:
                        for m in rx.finditer(l):
                            hits.add(m.group(0))
                            idx.setdefault(m.group(0), {"occ": [], "files": set()})
                            idx[m.group(0)]["occ"].append((p, n, l))
                            idx[m.group(0)]["files"].add(p)
                    for m in ID_LIKE_RE.finditer(l):
                        if not any(rx.match(l, m.start()) for rx in ID_RES):
                            unmatched[m.group(0)] = unmatched.get(m.group(0), 0) + 1
            self._ids = (idx, unmatched, skipped)
        return self._ids


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
    strict_bad = st.setdefault("frontmatter_strict_yaml_invalid", [])
    for p in files:
        fm, body, errs = parse_frontmatter(snap.text(p))
        _d, serr = strict_frontmatter(snap.text(p))
        if serr:
            strict_bad.append([p, serr])
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


def status_glyphs(lines):
    """legend lines `- <glyph> **Not Started**` -> {glyph: word} for the task status words (the file's own legend, never a fixed table)."""
    g = {}
    for l in lines:
        m = re.match(r"^\s*[-*+]\s+(\S+)\s+\*\*([^*]+)\*\*", l)
        if m and m.group(2).strip() in STATUS_WORDS_TASK:
            g[m.group(1).replace("\ufe0f", "")] = m.group(2).strip()
    return g


def rule_task_rows(snap, files, st):
    out = []
    for p in files:
        glyphs = status_glyphs(lines_of(snap, p))
        for n, l in enumerate(lines_of(snap, p), 1):
            if re.match(r"^\| *[0-9]+\.[0-9]+", l):
                cells = [c.strip() for c in l.strip().strip("|").split("|")]
                status = next((c for c in cells if c in STATUS_WORDS_TASK), None) or \
                    next((glyphs[c.replace("\ufe0f", "")] for c in cells if c.replace("\ufe0f", "") in glyphs), None)
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
    rule.checkbox = checkbox
    return rule


def make_rule_files(checkbox=False):
    """one entry per file, plus one entry per checkbox line when `checkbox`. Defect ids are NOT found here: they are one cross-source
    index (rule_ids), one entry per distinct id, so the same id in two sources is never two rows."""
    def rule(snap, files, st):
        out = []
        for p in files:
            out.append(file_entry(snap, p, title=first_heading(snap.text(p)) if p.endswith(".md") else None))
            if checkbox:
                for n, l in enumerate(lines_of(snap, p), 1):
                    m = CHECKBOX_RE.match(l)
                    if m:
                        out.append(E("%s:L%d" % (p, n), sha_text(l), title=m.group(2),
                                     status="checked" if m.group(1) in "xX" else "unchecked"))
        return out
    rule.checkbox = checkbox
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
    """yield (key, dict, has_id) for every element of a list of dicts that holds at least one `id`. An element WITHOUT an id is a case
    too (round-23 review A6: it was dropped silently); its key is `@<index>` (its position in the list) and has_id is False."""
    if isinstance(obj, list) and obj and all(isinstance(x, dict) for x in obj) and any(x.get("id") for x in obj):
        for i, x in enumerate(obj):
            if x.get("id"):
                yield str(x["id"]), x, True
            else:
                yield "@%d" % i, x, False
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
    import yaml        # run() refused `pyyaml_missing` before any rule runs
    for p in files:
        out.append(file_entry(snap, p, title=os.path.basename(p)))
        if os.path.splitext(p)[1] not in (".yaml", ".yml", ".json") or p in snap.links:
            continue
        data, err = None, None
        try:
            if p.endswith(".json"):
                data = json.loads(snap.text(p))
            else:
                data = yaml.safe_load(snap.text(p))
        except Exception as e:  # unparsable bank: the file row stays, the case rows are regex-found below
            err = type(e).__name__
        seen = {}
        if err is None:
            cases = list(walk_cases(data))
        else:
            st.setdefault("bank_parse_errors", []).append([p, err])
            cases = [(m.group(1).strip("'\""), {"id": m.group(1)}, True) for m in
                     re.finditer(r"(?m)^\s*-?\s*id:\s*(\S+)\s*$", snap.text(p))]
        for cid, c, has_id in cases:
            seen[cid] = seen.get(cid, 0) + 1
            loc = "%s#%s" % (p, cid) + ("~%d" % seen[cid] if seen[cid] > 1 else "")
            out.append(E(loc, sha_text(json.dumps(c, sort_keys=True, default=str)), legacy_id=cid if has_id else None,
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


STATUS_BOLD_RE = re.compile(r"\*\*(FIXED|DECIDED|OPEN|NOTE|UNCONFIRMED)\b[^*]*\*\*")
STATUS_WORD_RE = re.compile(r"\b(FIXED|DECIDED|OPEN|NOTE|UNCONFIRMED)\b")


def rule_conflicts(snap, files, st):
    """S-21: one entry per numbered item and per sub-item. Status = the distinct status words of the item's WHOLE block (its first
    line and every continuation line), joined by `,` in the order FIXED, DECIDED, OPEN, NOTE, UNCONFIRMED; an item with no status word
    of its own takes the one the section preamble gives it ("5 DECIDED (operator), 6 OPEN, ..."). Sub-items are the indented bullet
    lines and, when a block holds two or more bold status markers (`**FIXED:**`, `**OPEN:**`), one segment per marker (doc03 5.17:
    13a-13d, 14a-14b); a segment's status is its own marker."""
    out = []
    for p in files:
        ls = lines_of(snap, p)
        start = next((i for i, l in enumerate(ls) if re.match(r"^##\s+Known Conflicts", l)), None)
        if start is None:
            st.setdefault("conflicts_heading_missing", []).append(p)
            continue
        stop = len(ls)
        for i in range(start + 1, len(ls)):
            if re.match(r"^##\s", ls[i]):
                stop = i
                break
        first = next((i for i in range(start + 1, stop) if re.match(r"^\d+\.\s", ls[i])), stop)
        preface = {}
        for m in re.finditer(r"\b(\d+)\s+(FIXED|DECIDED|OPEN|NOTE|UNCONFIRMED)\b", " ".join(ls[start + 1:first])):
            preface.setdefault(int(m.group(1)), m.group(2))
        starts = [i for i in range(first, stop) if re.match(r"^\d+\.\s", ls[i])] + [stop]
        for a, b in zip(starts, starts[1:]):
            item = int(re.match(r"^(\d+)\.", ls[a]).group(1))
            block = ls[a:b]
            while block and not block[-1].strip():
                block = block[:-1]
            words = {w for l in block for w in STATUS_WORD_RE.findall(l)}
            order = [w for w in STATUS_WORDS_CONFLICT if w in words]
            status = ",".join(order) if order else preface.get(item)
            out.append(E("%s:L%d#item%d" % (p, a + 1, item), sha_text(ls[a]), legacy_id=str(item),
                         title=re.sub(r"^\d+\.\s+", "", ls[a]), status=status))
            sub = 0
            marks = [(i, m.group(1)) for i in range(a, a + len(block)) for m in STATUS_BOLD_RE.finditer(ls[i])]
            for i in range(a + 1, a + len(block)):
                l = ls[i]
                if re.match(r"^\s{2,}(?:[-*]|\d+[.)]|\([a-z0-9]+\)|[a-z]\))\s+", l):
                    sub += 1
                    w = [x for x in STATUS_WORDS_CONFLICT if re.search(r"\b%s\b" % x, l)]
                    out.append(E("%s:L%d#item%d.%d" % (p, i + 1, item, sub), sha_text(l), legacy_id="%d.%d" % (item, sub),
                                 title=l.strip(), status=w[0] if w else None))
            if len(marks) >= 2:
                for k, (i, word) in enumerate(marks):
                    end = marks[k + 1][0] if k + 1 < len(marks) else a + len(block)
                    seglines = ls[i:max(end, i + 1)]
                    sub += 1
                    text = " ".join(x.strip() for x in seglines if x.strip())
                    if i == a:
                        text = re.sub(r"^\d+\.\s+", "", text)
                    out.append(E("%s:L%d#item%d.%d" % (p, i + 1, item, sub), sha_text("\n".join(seglines)), legacy_id="%d.%d" % (item, sub),
                                 title=text, status=word))
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
    """S-23: every NON-BINARY file of the snapshot, whatever its extension (round-23 review A5: an extension allow-list silently
    dropped Dockerfile, Makefile, .rb, .proto, .ipynb, ...). Files not scanned are counted by reason (symlink, too_large, binary,
    unreadable), never silently."""
    out, skipped, scanned = [], {"too_large": 0, "binary": 0, "symlink": 0, "unreadable": 0}, 0
    tag_of = {}
    for tag, r in SKIP_RES:
        tag_of.setdefault(tag, []).append(r)
    for p in files:
        t, why = snap.scannable(p)
        if t is None:
            skipped[why] += 1
            continue
        scanned += 1
        if not MARKER_RE.search(t) and not any(r.search(t) for _, r in SKIP_RES):
            continue
        for n, l in enumerate(t.split("\n"), 1):
            l = l[:-1] if l.endswith("\r") else l
            seen = {}
            for m in MARKER_RE.finditer(l):      # a repeated marker on one line is one row per occurrence: #2, #3, ...
                k = seen[m.group(1)] = seen.get(m.group(1), 0) + 1
                out.append(E("%s:L%d:%s" % (p, n, m.group(1)) + ("#%d" % k if k > 1 else ""), sha_text(p + "\0" + l), title=l.strip(), status="marker"))
            for tag in sorted(tag_of):           # ONE hit counter per tag and line across all regexes of the tag (round-23 review B1)
                hits = sorted(m.start() for r in tag_of[tag] for m in r.finditer(l))
                for k, _pos in enumerate(hits, 1):
                    out.append(E("%s:L%d:skip" % (p, n) + ("" if tag == "go" else "-" + tag) + ("#%d" % k if k > 1 else ""), sha_text(p + "\0" + l), title=l.strip(), status="skipped_test"))
    skipped["scanned_files"] = scanned
    st["skipped"] = skipped
    return out


def rule_ids(cls):
    """cross-source defect-id index: ONE entry per distinct id of the whole snapshot (doc03 section 9: an id referenced only in prose is
    still imported), attributed to class S-14 or S-15 when its first occurrence inside those file sets exists, else to XID. Locator
    `<path>:L<n>#<id>` of that occurrence; the title is that line. Carriers (ID_SCAN_EXCLUDE) are not scanned."""
    def rule(snap, files, st):
        idx, unmatched, skipped = snap.id_index()
        out = []
        for i in sorted(idx):
            occ = sorted(idx[i]["occ"], key=lambda o: (o[0].encode("utf-8"), o[1]))
            home = None
            for c in ("S-14", "S-15"):
                cf = snap.class_files.get(c, set())
                pick = next((o for o in occ if o[0] in cf), None)
                if pick:
                    home = (c, pick)
                    break
            c, pick = home if home else ("XID", occ[0])
            if c != cls:
                continue
            p, n, l = pick
            out.append(E("%s:L%d#%s" % (p, n, i), sha_text(p + "\0" + l), legacy_id=i, title=l.strip()))
            st.setdefault("legacy_ids", {})[i] = {"first": "%s:L%d" % (p, n), "files": len(idx[i]["files"])}
        if cls == "XID":
            st["id_like_unmatched"] = {k: unmatched[k] for k in sorted(unmatched)}
            st["id_scan_excluded_prefixes"] = list(ID_SCAN_EXCLUDE)
            st["id_scan_skipped_files"] = skipped
        return out
    rule.__name__ = "rule_ids_" + cls.replace("-", "")
    return rule


def sanitize_url(u):
    """remote url without its userinfo (a token in a url is a credential, 11.4.10); anything unparsable is reduced to its host-less form."""
    u = str(u or "").strip()
    return re.sub(r"^([A-Za-z][A-Za-z0-9+.\-]*://)[^/@]*@", r"\1", u)


def rule_remotes(snap, files, st):
    """external sources 1/2: one entry per git remote the freeze json records (name and sanitized url; never queried here)."""
    out = []
    rem = snap.freeze.get("remotes")
    st["remotes_recorded"] = len(rem or [])
    for r in rem or []:
        name = r["name"] if isinstance(r, dict) else str(r)
        url = sanitize_url(r.get("url", "")) if isinstance(r, dict) else ""
        out.append(E("remote:%s" % name, sha_text("remote\0%s\0%s" % (name, url)), title=("%s %s" % (name, url)).strip(), status="not_queried"))
    return out


def rule_services(snap, files, st):
    """external sources 2/2: one entry per NAMED service of the closed list SERVICES (doc03 5.20), whether or not its marker files are
    in the snapshot: the status says which (marker_found / marker_absent), so a service never disappears silently."""
    out = []
    names = set(snap.files)
    for svc, markers in SERVICES:
        found = sorted(m for m in markers if (any(f.startswith(m) for f in names) if m.endswith("/") else m in names))
        out.append(E("service:%s" % svc, sha_text("service\0%s\0%s" % (svc, ",".join(found))), title="%s (%s)" % (svc, ", ".join(found) or "no marker file in the snapshot"),
                     status="marker_found" if found else "marker_absent"))
    return out


# ---------------------------------------------------------------------------------------------- plan documents (S-26 seeds, S-27 innovation entries)
# W2 (T170, T171): the plan-document seeds of docs/21 section 9.1 and the innovation entries of docs/21 section 9.5 are Stage 0 sources of kind `report_doc`,
# read from the frozen snapshot like every class. The parsing lives HERE (one parser: the importer scripts/register/import_plan.py loads these functions).
PLAN_DIR = "specs/001-full-project-audit-remediation/docs"
DOC21 = PLAN_DIR + "/21-master-plan-phases-risks-and-traceability.md"
DOC18 = PLAN_DIR + "/18-research-product-innovation-and-game-changers.md"
SEV_RE = re.compile(r"^(critical|high|medium|low|info|cosmetic)(?:[ -]+(?:high|medium|low))?(?: if confirmed)?$", re.I)


def split_cells(line):
    return [c.strip() for c in re.split(r"(?<!\\)\|", line.strip().strip("|"))]


def md_section(lines, heading_re):
    """-> (start index, end index) of the lines UNDER the first heading matching heading_re up to the next heading of the same or a higher level"""
    for i, l in enumerate(lines):
        m = re.match(heading_re, l)
        if m:
            lvl = len(l) - len(l.lstrip("#"))
            for j in range(i + 1, len(lines)):
                if re.match(r"^#{1,%d}\s" % lvl, lines[j]) or lines[j].strip() == "---":
                    return i + 1, j
            return i + 1, len(lines)
    return None


def table_rows(lines, a, b):
    """-> [(line number, cells)] of the table rows (not the header separator) between lines a and b"""
    out = []
    for i in range(a, b):
        l = lines[i]
        if l.startswith("|") and not re.match(r"^\|[\s:|-]+\|?\s*$", l):
            out.append((i + 1, split_cells(l)))
    return out


def _expand_token(tok):
    tok = tok.strip().strip("`").strip()
    m = re.match(r"^(.*?)(\d+)\.\.(.*?)(\d+)$", tok)
    if not m:
        return [tok] if tok else []
    p, a, _q, b = m.group(1), m.group(2), m.group(3), m.group(4)
    if int(b) < int(a):
        refuse("seed_range_invalid", tok)
    return [p + str(i).zfill(len(a)) for i in range(int(a), int(b) + 1)]


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def expand_seed_ids(source, ids):
    """one docs/21 section 9.1 row -> [{"doc": "docNN", "id": ..., "part": None|"R-ADJ", "name": None|text}]"""
    dm = re.search(r"doc(\d{2})", source)
    if not dm:
        refuse("seed_row_unparsable", "no docNN in %r" % source)
    doc = "doc" + dm.group(1)
    t = ids.replace("`", "")
    out = []
    if doc == "doc13" and re.match(r"^\d+ measured defect classes", t):
        inner = re.search(r"\((.*)\)\s*$", t)
        if not inner:
            refuse("seed_row_unparsable", t)
        for name in [x.strip() for x in inner.group(1).split(", ") if x.strip()]:
            out.append({"doc": doc, "id": "class-" + slug(name), "part": None, "name": name})
        return out
    if doc == "doc18" and "candidates" in t:
        left, _p, right = t.partition(" plus ")
        for i in re.findall(r"T\d+-[A-F]", re.sub(r"\([^)]*\)", "", left)):
            out.append({"doc": doc, "id": i, "part": None, "name": None})
        for i in re.findall(r"T\d+-[A-F]", re.sub(r"\([^)]*\)", "", right)):
            out.append({"doc": doc, "id": i + "(R-ADJ)", "part": "R-ADJ", "name": None})
        return out
    t = re.sub(r"\([^)]*\)", "", t)
    main, _p, extra = t.partition(", plus ")
    for tok in main.split(","):
        for i in _expand_token(tok):
            out.append({"doc": doc, "id": i, "part": None, "name": None})
    if extra.strip():
        out.append({"doc": doc, "id": slug(re.sub(r"^the\s+", "", extra.strip())), "part": None, "name": extra.strip()})
    return out


def parse_seed_table(lines):
    """docs/21 section 9.1 -> [row dict]; refuses when a row's expanded ids do not equal its Count cell or the rows do not equal the total row"""
    sec = md_section(lines, r"^### 9\.1\s")
    if sec is None:
        refuse("seed_section_missing", "no `### 9.1` heading in docs/21")
    rows, total_row = [], None
    for n, c in table_rows(lines, *sec):
        if len(c) < 5 or c[0] == "Source":
            continue
        if "Total itemised" in c[0]:
            total_row = int(re.sub(r"[^0-9]", "", c[3]) or -1)
            continue
        cnt = int(re.sub(r"[^0-9]", "", c[3]) or -1)
        seeds = expand_seed_ids(c[0], c[1])
        if len(seeds) != cnt:
            refuse("seed_count_mismatch", "docs/21 line %d: %r expands to %d ids, the Count cell says %d" % (n, c[1][:60], len(seeds), cnt))
        rows.append({"line": n, "source": c[0], "ids": c[1], "app": c[2], "count": cnt, "sev": c[4], "doc": seeds[0]["doc"] if seeds else None, "seeds": seeds})
    if total_row is None or total_row != sum(r["count"] for r in rows):
        refuse("seed_count_mismatch", "the Total itemised row (%s) is not the sum of the rows (%d)" % (total_row, sum(r["count"] for r in rows)))
    keys = [(sd["doc"], sd["id"]) for r in rows for sd in r["seeds"]]
    if len(keys) != len(set(keys)):
        refuse("seed_duplicate", "a (document, id) pair occurs twice in docs/21 section 9.1")
    return rows


def parse_family_table(lines, seed_keys):
    """docs/21 section 9.3 -> [{"family", "members": [(doc, id, part_flag)]}]; every member must be a seed of 9.1"""
    sec = md_section(lines, r"^### 9\.3\s")
    if sec is None:
        refuse("family_section_missing", "no `### 9.3` heading in docs/21")
    fams = []
    for n, c in table_rows(lines, *sec):
        if len(c) != 2 or c[0] in ("Family",):
            continue
        members = []
        for seg in c[1].split(";"):
            part = "(part)" in seg
            radj = re.search(r"\(R-ADJ part\b", seg.replace("`", "")) is not None          # the member names the R-ADJ part of a mixed candidate: its seed id carries the suffix
            seg = re.sub(r"\((?!part\))[^)]*\)", "", seg.replace("`", "")).replace("(part)", "").strip()
            m = re.match(r"^doc(\d{2})\s+(.*)$", seg)
            if not m:
                continue
            doc, rest = "doc" + m.group(1), m.group(2).strip()
            cm = re.match(r'^class\s+"([^"]+)"$', rest)
            if cm:
                members.append((doc, "class-" + slug(cm.group(1)), part))
                continue
            for tok in rest.split(","):
                for i in _expand_token(tok):
                    members.append((doc, i + "(R-ADJ)" if radj else i, part))
        # a Tn-X token tagged `R-ADJ part` in the family cell names the R-ADJ seed id of the same candidate
        fams.append({"line": n, "family": c[0], "members": members, "cell": c[1]})
    cm = re.search(r"These (\d+) families list (\d+) member ids", "\n".join(lines[sec[0]:sec[1]]))
    if cm and (len(fams), sum(len(f["members"]) for f in fams)) != (int(cm.group(1)), int(cm.group(2))):
        refuse("family_count_mismatch", "docs/21 section 9.3 states %s families and %s member ids, the table parses to %d and %d" % (
            cm.group(1), cm.group(2), len(fams), sum(len(f["members"]) for f in fams)))
    return fams


def doc_files(snap_files):
    """docNN -> snapshot path of the plan document NN-*.md"""
    out = {}
    for p in snap_files:
        m = re.match(re.escape(PLAN_DIR) + r"/(\d{2})-[^/]*\.md$", p)
        if m:
            out["doc" + m.group(1)] = p
    return out


def _hint_ranges(source, lines):
    """sections the docs/21 Source cell cites (`§15`, `§4, §14.1`, `§5.1`) -> [(a, b)] line ranges; [] when none can be found"""
    rng = []
    for num in re.findall(r"§\s*(\d+(?:\.\d+)*)", source):
        r = md_section(lines, r"^#{1,6}\s+%s\.?(?:\s|$)" % re.escape(num))
        if r:
            rng.append(r)
    return rng


def locate_seed(lines, sd, source):
    """-> (line number or None, text or None): the line of the source document that carries the seed, searched under the cited sections first.
    A seed id is the LEADING token of a table row's first cell, a heading or a list item; a mention in the middle of a sentence is never the seed."""
    rng = _hint_ranges(source, lines) or [(0, len(lines))]
    om = re.match(r"^T-(\d+)$", sd["id"]) if sd["doc"] == "doc20" else None
    if om:        # docs/21 assigned the labels T-1..T-4 to the rows of one source table (docs/21 9.1 doc20 row): the N-th data row of the cited section's first table
        for a, b in rng:
            rows = [(n, l) for n, l in ((i + 1, lines[i]) for i in range(a, b)) if l.startswith("|") and not re.match(r"^\|[\s:|-]+\|?\s*$", l)][1:]
            if len(rows) >= int(om.group(1)):
                return rows[int(om.group(1)) - 1][0], rows[int(om.group(1)) - 1][1]
        return None, None
    names = []
    if sd["name"]:
        names = [sd["name"].lower()]
    key = sd["id"].replace("(R-ADJ)", "")
    lead = re.compile(r"^\s*(?:[-*+]\s+|#{1,6}\s+|\|\s*)?[*`_\[]*" + re.escape(key) + r"(?![A-Za-z0-9])")
    for passno in (0, 1):
        for a, b in (rng if passno == 0 else [(0, len(lines))]):
            for i in range(a, b):
                l = lines[i]
                if names:
                    cells = split_cells(l) if l.startswith("|") else [l]
                    if any(names[0] in c.lower() for c in cells[:2]):
                        return i + 1, l
                elif lead.match(l):
                    return i + 1, l
        if rng == [(0, len(lines))]:
            break
    return None, None


def header_above(lines, line_no):
    """-> the cells of the table header above a table row (the line before the separator row), or None"""
    i = line_no - 2
    while i >= 1 and lines[i].startswith("|"):
        if re.match(r"^\|[\s:|-]+\|?\s*$", lines[i]) and lines[i - 1].startswith("|"):
            return split_cells(lines[i - 1])
        i -= 1
    return None


def row_title_severity(text, key, lines=None, line_no=None):
    """-> (title, severity label or None) from a located line; for a table row the severity is read from the column whose header names it
    (`Severity`, `Sev`, `Rating`), else from a cell that starts with a severity word"""
    if text is None:
        return None, None
    if text.startswith("|"):
        cells = split_cells(text)
        plain = [re.sub(r"[*`]", "", c).strip() for c in cells]
        sev = None
        hdr = header_above(lines, line_no) if lines is not None else None
        if hdr and len(hdr) == len(cells):
            for j, h in enumerate(hdr):
                if re.search(r"sever|rating|\bsev\b", h, re.I):
                    sev = clean(plain[j], 100) or None          # the cell as written (`Critical (if confirmed)`, `Low-Med`): the importer normalises it, the register keeps the raw label
                    break
        if sev is None:
            for c in plain[1:]:
                if SEV_RE.match(c):
                    sev = c
                    break
        body = [c for c in plain[1:] if c and not SEV_RE.match(c)]
        title = max(body, key=len) if body else None
        return (clean(title, 160) if title else None), sev
    t = re.sub(r"^\s*(?:[-*+]\s+|#{1,6}\s+)?[*`_\[]*" + re.escape(key.replace("(R-ADJ)", "")) + r"[*`_\]]*\s*[:.\-–—)]*\s*", "", text)
    return clean(re.sub(r"[*`]", "", t), 160) or None, None


def seed_entries(snap_lines_of, files_by_doc, rows):
    """-> list of seed dicts with location facts. snap_lines_of(path) -> lines. Used by the enumerator and by the importer (one reader)."""
    cache = {}
    out = []
    for r in rows:
        for sd in r["seeds"]:
            p = files_by_doc.get(sd["doc"])
            line_no = text = None
            cand_title = None
            if p:
                if p not in cache:
                    cache[p] = snap_lines_of(p)
                if sd["doc"] == "doc18":      # a doc18 seed is a CANDIDATE: its source row is the candidate-table row, not the routing-table row
                    if ("cands", p) not in cache:
                        cache[("cands", p)] = parse_doc18_candidates(cache[p])
                    cc = cache[("cands", p)].get(sd["id"].replace("(R-ADJ)", ""))
                    if cc:
                        line_no, text, cand_title = cc["line"], cache[p][cc["line"] - 1], clean(cc["cells"].get("Candidate", ""), 160)
                if text is None:
                    line_no, text = locate_seed(cache[p], sd, r["source"])
            title, sev = row_title_severity(text, sd["id"], cache.get(p), line_no)
            if cand_title:
                title = cand_title
            if sd["name"] and not title:
                title = sd["name"]
            out.append(dict(sd, row_line=r["line"], source=r["source"], app=r["app"], rated=("unrated" not in r["sev"].lower()), sev_cell=r["sev"],
                            src_path=p, src_line=line_no, src_text=text, title=title or ("%s %s (%s)" % (sd["doc"], sd["id"], clean(r["app"], 60))), severity=sev if "unrated" not in r["sev"].lower() else None,
                            located=text is not None))
    return out


def rule_plan_seeds(snap, files, st):
    out = []
    for p in files:
        lines = lines_of(snap, p)
        rows = parse_seed_table(lines)
        seeds = seed_entries(lambda q: lines_of(snap, q), doc_files(snap.files), rows)
        for sd in seeds:
            basis = "%s\0%s\0%s\0%s" % (sd["doc"], sd["id"], lines[sd["row_line"] - 1], sd["src_text"] or "")
            out.append(E("%s#seed:%s:%s" % (p, sd["doc"], sd["id"]), sha_text(basis), legacy_id="%s:%s" % (sd["doc"], sd["id"]), title=sd["title"],
                         status=("rated" if sd["rated"] else "unrated"), severity=sd["severity"]))
        st["plan_seeds"] = {"seed_rows": len(rows), "seeds": len(seeds), "located_in_source_document": sum(1 for s in seeds if s["located"]),
                            "unlocated": [("%s:%s" % (s["doc"], s["id"])) for s in seeds if not s["located"]], "rated": sum(1 for s in seeds if s["rated"]),
                            "unrated": sum(1 for s in seeds if not s["rated"])}
    return out


def parse_innovation_table(lines):
    """docs/21 section 9.5 -> [{"id", "kind": "PROPOSAL"|"PROPOSAL part", "text", "rank", "line"}]; refuses when the table disagrees with its Counts line"""
    sec = md_section(lines, r"^### 9\.5\s")
    if sec is None:
        refuse("innovation_section_missing", "no `### 9.5` heading in docs/21")
    ents = []
    for n, c in table_rows(lines, *sec):
        if len(c) >= 4 and re.match(r"^T\d+-[A-F]$", c[0]):
            kind = c[1].replace("`", "").strip()
            if kind not in ("PROPOSAL", "PROPOSAL part"):
                refuse("innovation_kind_unknown", "docs/21 line %d: %r" % (n, kind))
            ents.append({"id": c[0], "kind": kind, "text": c[2], "rank": c[3], "line": n})
    cm = re.search(r"Counts:\s*(\d+) entries \((\d+) `PROPOSAL`, (\d+) `PROPOSAL` parts\)", "\n".join(lines[sec[0]:sec[1] + 40]))
    if not cm:
        refuse("innovation_counts_missing", "docs/21 section 9.5 has no `Counts: N entries (a `PROPOSAL`, b `PROPOSAL` parts)` line")
    n_all, n_prop, n_part = int(cm.group(1)), int(cm.group(2)), int(cm.group(3))
    if (len(ents), sum(1 for e in ents if e["kind"] == "PROPOSAL"), sum(1 for e in ents if e["kind"] == "PROPOSAL part")) != (n_all, n_prop, n_part):
        refuse("innovation_count_mismatch", "table has %d entries (%d + %d), the Counts line says %d (%d + %d)" % (
            len(ents), sum(1 for e in ents if e["kind"] == "PROPOSAL"), sum(1 for e in ents if e["kind"] == "PROPOSAL part"), n_all, n_prop, n_part))
    ids = [e["id"] for e in ents]
    if len(ids) != len(set(ids)):
        refuse("innovation_duplicate", "an entry id occurs twice in docs/21 section 9.5")
    return ents


def parse_doc18_candidates(lines):
    """docs/18 candidate tables (header `| ID | Candidate | Tag | Touches | ...`) -> {id: {"line","cells","tag_class"}}; tag_class from the Tag cell:
    both words -> mixed, one word -> that word. Every table with that header is read."""
    out, hdr = {}, None
    for i, l in enumerate(lines):
        if re.match(r"^\| ID \| Candidate \| Tag \|", l):
            hdr = split_cells(l)
            continue
        if hdr is not None:
            if not l.startswith("|"):
                hdr = None
                continue
            if re.match(r"^\|[\s:|-]+\|?\s*$", l):
                continue
            c = split_cells(l)
            if re.match(r"^T\d+-[A-F]$", c[0]) and len(c) == len(hdr):
                tag = c[2]
                a, b = "R-ADJ" in tag, "PROPOSAL" in tag
                if c[0] in out:
                    refuse("candidate_duplicate", "doc18 candidate %s occurs twice" % c[0])
                out[c[0]] = {"line": i + 1, "cells": dict(zip(hdr, c)), "tag": tag, "tag_class": "mixed" if (a and b) else ("R-ADJ" if a else ("PROPOSAL" if b else "none"))}
    return out


def rule_innovation(snap, files, st):
    out = []
    for p in files:
        lines = lines_of(snap, p)
        ents = parse_innovation_table(lines)
        cand = {}
        d18 = doc_files(snap.files).get("doc18")
        if d18:
            cand = parse_doc18_candidates(lines_of(snap, d18))
        for e in ents:
            c = cand.get(e["id"])
            basis = "%s\0%s\0%s\0%s" % (e["id"], e["kind"], lines[e["line"] - 1], json.dumps(c["cells"], sort_keys=True) if c else "")
            out.append(E("%s#innovation:%s%s" % (p, e["id"], ":PROPOSAL-part" if e["kind"] == "PROPOSAL part" else ""), sha_text(basis), legacy_id="doc18:%s" % e["id"], title=clean(e["text"], 160),
                         status=e["kind"], severity=None))
        st["innovation"] = {"entries": len(ents), "proposal": sum(1 for e in ents if e["kind"] == "PROPOSAL"), "proposal_part": sum(1 for e in ents if e["kind"] == "PROPOSAL part"),
                            "doc18_candidates_read": len(cand), "entry_without_doc18_row": [e["id"] for e in ents if e["id"] not in cand]}
    return out


# ---------------------------------------------------------------------------------------------- class table
# doc03 section 7 counts these two docs/ files in the S-12 row (37 and 7 unchecked boxes); no other class selects them
S12_EXTRA = ("docs/COMPREHENSIVE_PACKAGE_SUMMARY.md", "docs/README_IMPLEMENTATION_PACKAGE.md")


def root_other_reports(files, claimed):
    return [p for p in files if (("/" not in p and p.endswith(".md") and p not in GOVERNANCE_ROOT) or p in S12_EXTRA) and p not in claimed]


def sel_glob(*pats):
    return lambda files, claimed: [p for p in files if p not in claimed and any(gl(p, x) for x in pats)]


def sel_exact(*names):
    return lambda files, claimed: [p for p in files if p in names and p not in claimed]


def sel_s22(files, claimed):
    return [p for p in files if p not in claimed and os.path.basename(p) in ("CLAUDE.md", "AGENTS.md")
            and "/" in p and not p.startswith(("submodules/", ".audit/"))]


def sel_all(files, claimed):
    return list(files)


def sel_idscan(files, claimed):
    return [p for p in files if not p.startswith(ID_SCAN_EXCLUDE)]


def sel_none(files, claimed):
    return []


def is_noclaim(cls, loc):
    """sources that scan or index without claiming files for a class (the marker scan, the id index, the external sources)"""
    return cls in ("S-23", "S-27") or loc.startswith(("legacy-ids:", "external:"))


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
    ("S-12", "report_doc", "/*.md (other root status reports)+docs/COMPREHENSIVE_PACKAGE_SUMMARY.md+docs/README_IMPLEMENTATION_PACKAGE.md", None, make_rule_files(checkbox=True)),   # selector special-cased
    ("S-13", "report_doc", "docs/status/*.md", sel_glob("docs/status/*.md"), make_rule_files()),
    ("S-14", "report_doc", "docs/audits/*.md", sel_glob("docs/audits/*.md"), make_rule_files(checkbox=True)),
    ("S-14", "report_doc", "docs/*AUDIT*.md", sel_glob("docs/*AUDIT*.md"), make_rule_files(checkbox=True)),
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
    ("S-15", "qa_results", "docs/qa/**+docs/reports/qa-sessions/**", sel_glob("docs/qa/**", "docs/reports/qa-sessions/**"), make_rule_files()),
    ("S-14", "report_doc", "legacy-ids:defect ids first seen in the S-14 files", sel_idscan, rule_ids("S-14")),
    ("S-15", "qa_results", "legacy-ids:defect ids first seen in the S-15 files", sel_idscan, rule_ids("S-15")),
    ("XID", "report_doc", "legacy-ids:defect ids referenced only outside the S-14 and S-15 files", sel_idscan, rule_ids("XID")),
    ("S-16", "report_doc", "docs/security/**", sel_glob("docs/security/**"), rule_security),
    ("S-23", "code_marker", "marker and skipped-test scan of every text file of the snapshot", sel_all, rule_markers),
    ("S-26", "report_doc", "plan-document seeds (docs/21 section 9.1)", sel_exact(DOC21), rule_plan_seeds),
    ("S-27", "report_doc", "innovation entries (docs/21 section 9.5)", lambda files, claimed: [p for p in files if p == DOC21], rule_innovation),
    ("EXT", "external_ticket", "external:git remotes recorded in the freeze json", sel_none, rule_remotes),
    ("EXT", "external_ticket", "external:named services (firebase-crashlytics, sonarqube, snyk, trivy)", sel_none, rule_services),
]
CLASS_IDS = ["S-%02d" % i for i in range(1, 28)]


def sql_q(v):
    if v is None:
        return "NULL"
    return "'" + str(v).replace("'", "''") + "'"


def enumerate_snapshot(snap, frozen_at):
    claimed, stats, plan = {}, {"classes": {}, "frozen_at_basis": None}, []
    cb_done = set()
    for cls, kind, loc, sel, rule in SOURCES:
        assert kind in KINDS
        noclaim = is_noclaim(cls, loc)
        if cls == "S-23":
            files = sel(snap.files, {})
        elif loc.startswith("/*.md"):
            files = root_other_reports(snap.files, claimed)
        else:
            files = sel(snap.files, claimed)
        if not noclaim:
            for p in files:
                claimed[p] = loc
            snap.class_files.setdefault(cls, set()).update(files)
            if getattr(rule, "checkbox", False):
                cb_done.update(files)
        st = {}
        ents = rule(snap, files, st)
        seen, uniq = set(), []
        for e in sorted(ents, key=lambda x: x.locator.encode("utf-8")):
            if e.locator in seen:
                refuse("duplicate_entry_locator", "%s %s" % (loc, e.locator))
            seen.add(e.locator)
            uniq.append(e)
        if loc.startswith("external:"):
            csha = sha_text("".join("%s\t%s\n" % (e.locator, e.sha) for e in uniq))
        else:
            csha = sha_text("".join("%s\t%s\n" % (p, snap.sha(p)) for p in files))
        plan.append((cls, kind, loc, files, uniq, csha))
        c = stats["classes"].setdefault(cls, {"sources": [], "files": 0, "entries": 0})
        c["sources"].append(loc)
        if not loc.startswith(("legacy-ids:", "external:")):
            c["files"] += len(files)
        c["entries"] += len(uniq)
        for k in ("frontmatter_problems", "bank_parse_errors", "conflicts_heading_missing", "skipped", "frontmatter_strict_yaml_invalid",
                  "legacy_ids", "id_like_unmatched", "id_scan_excluded_prefixes", "id_scan_skipped_files", "remotes_recorded", "plan_seeds", "innovation"):
            if k in st:
                c.setdefault(k, st[k])
        if "frontmatter_strict_yaml_invalid" in st:
            c["frontmatter_strict_yaml_invalid_count"] = len(st["frontmatter_strict_yaml_invalid"])
        if not files and not noclaim:
            c["empty_source"] = c.get("empty_source", []) + [loc]
    stats["root_files_not_sources"] = [p for p in snap.files if "/" not in p and p.endswith(".md") and p in GOVERNANCE_ROOT]
    # procedural checklists and every other Markdown file with checkbox lines no checkbox source enumerates: listed, not ignored (doc03 3.1 rule 2)
    ex = []
    for p in snap.files:
        if p.endswith(".md") and p not in cb_done and p not in snap.links:
            n = sum(1 for l in lines_of(snap, p) if CHECKBOX_RE.match(l))
            if n:
                ex.append([p, n])
    stats["checkbox_lines_not_enumerated"] = {"files": len(ex), "lines": sum(n for _p, n in ex), "list": ex}
    stats["total_entries"] = sum(c["entries"] for c in stats["classes"].values())
    return plan, stats


def render_block(sources, scanned_at, header):
    """-> SQL text. sources = [(kind, locator, parser, rows, csha_or_None)], rows = [E]. One implementation for the enumerator and the lead
    scan. `.bail on` plus a gate table make a re-import refuse as a whole when an entry already in the register has a different
    entry_sha256 at the same locator (INSERT OR IGNORE would keep the stale row silently, round-23 review B2)."""
    out = [".bail on", "-- " + header, "PRAGMA foreign_keys=ON;", "BEGIN;"]
    for kind, loc, parser, rows, csha in sources:
        out.append("INSERT OR IGNORE INTO reg_sources(kind,locator,parser) VALUES (%s,%s,%s);" % (sql_q(kind), sql_q(loc), sql_q(parser)))
    out.append("CREATE TEMP TABLE _enum_new(n INTEGER PRIMARY KEY, src TEXT NOT NULL, loc TEXT NOT NULL, legacy_id TEXT, title TEXT, status TEXT, sev TEXT, sha TEXT NOT NULL, scanned TEXT NOT NULL);")
    n, batch = 0, []
    for kind, loc, parser, rows, csha in sources:
        for e in rows:
            n += 1
            batch.append("(%d,%s,%s,%s,%s,%s,%s,%s,%s)" % (n, sql_q(loc), sql_q(e.locator), sql_q(e.legacy_id), sql_q(e.title), sql_q(e.status),
                                                           sql_q(e.severity), sql_q(e.sha), sql_q(scanned_at)))
            if len(batch) >= 100:
                out.append("INSERT INTO _enum_new VALUES " + ",".join(batch) + ";")
                batch = []
    if batch:
        out.append("INSERT INTO _enum_new VALUES " + ",".join(batch) + ";")
    out.append("CREATE TEMP TABLE _enum_gate(stale_entries_changed_at_same_locator INTEGER CHECK(stale_entries_changed_at_same_locator=0));")
    out.append("INSERT INTO _enum_gate SELECT count(*) FROM _enum_new x JOIN reg_sources s ON s.locator=x.src "
               "JOIN reg_source_entries e ON e.source_id=s.source_id AND e.locator=x.loc WHERE e.entry_sha256<>x.sha;")
    out.append("INSERT OR IGNORE INTO reg_source_entries(source_id,locator,legacy_id,title,raw_status,raw_severity,entry_sha256,scanned_at) "
               "SELECT s.source_id,x.loc,x.legacy_id,x.title,x.status,x.sev,x.sha,x.scanned FROM _enum_new x JOIN reg_sources s ON s.locator=x.src ORDER BY x.n;")
    for kind, loc, parser, rows, csha in sources:
        if csha is None:
            out.append("UPDATE reg_sources SET last_scanned_at=%s, scanned_entry_count=%d WHERE locator=%s;" % (sql_q(scanned_at), len(rows), sql_q(loc)))
        else:
            out.append("UPDATE reg_sources SET content_sha256=%s, last_scanned_at=%s, scanned_entry_count=%d WHERE locator=%s;" %
                       (sql_q(csha), sql_q(scanned_at), len(rows), sql_q(loc)))
    out.append("DROP TABLE _enum_gate;")
    out.append("DROP TABLE _enum_new;")
    out.append("COMMIT;")
    return "\n".join(out) + "\n"


def render_sql(plan, scanned_at):
    return render_block([(kind, loc, PARSER, ents, csha) for cls, kind, loc, files, ents, csha in plan], scanned_at,
                        "generated by enumerate_sources.py@2; INSERT OR IGNORE only (idempotent); no DELETE, no UPDATE of an entry")


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


def write_sql_files(out_dir, name, sqltxt):
    """<name>.sql plus TWO bare-hash files with the same one line: `<name>.sql.sha256` (the T165 text) and `<name>.sha256`, the sibling
    scripts/register/locked.sh import-sql reads (stem = the file name without .sql)."""
    os.makedirs(out_dir, exist_ok=True)
    sqlp = os.path.join(out_dir, name + ".sql")
    with open(sqlp, "w", encoding="utf-8", newline="\n") as f:
        f.write(sqltxt)
    h = sha_text(sqltxt)
    for suffix in (".sql.sha256", ".sha256"):
        with open(os.path.join(out_dir, name + suffix), "w") as f:
            f.write(h + "\n")
    return h


def run(argv):
    a = {"freeze": None, "out": None, "list": False}
    i = 0
    while i < len(argv):
        k = argv[i]
        if k == "--freeze-json" and i + 1 < len(argv):
            a["freeze"], i = argv[i + 1], i + 2
        elif k == "--out" and i + 1 < len(argv):
            a["out"], i = argv[i + 1], i + 2
        elif k == "--list-classes":
            a["list"], i = True, i + 1
        else:
            sys.stderr.write("enumerate_sources: usage: --freeze-json <path> --out <dir> | --list-classes\n")
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
            import yaml  # noqa: F401
        except ImportError:
            refuse("pyyaml_missing", "PyYAML is required: without it the bank walk and the strict front-matter check differ (round-23 review B4)")
        try:
            fj = json.load(open(a["freeze"], encoding="utf-8"))
            snapdir, manifest = fj["snapshot"], fj["manifest"]
        except (OSError, ValueError, KeyError) as e:
            refuse("freeze_json_invalid", "%s: %s" % (a["freeze"], type(e).__name__))
        if not os.path.isdir(snapdir) or not os.path.isfile(manifest):
            refuse("freeze_json_invalid", "snapshot or manifest path does not exist")
        if not isinstance(fj.get("remotes"), list):
            refuse("freeze_remotes_missing", "the freeze json has no `remotes` list (T165: one row per remote); `[]` states there are none")
        chk = os.path.join(os.path.dirname(os.path.abspath(__file__)), "check_freeze_snapshot.sh")
        # ENTRY CHECK: before any snapshot file is read (round-31 review I1); not replaceable from the command line
        r = subprocess.run([chk, snapdir, manifest], stdout=subprocess.PIPE, stderr=subprocess.PIPE)  # MUT:entry-check
        if r.returncode != 0:
            err = r.stderr.decode("utf-8", "replace")
            sys.stderr.write(err)
            mm = re.search(r"REFUSED reason=(freeze_manifest_invalid|freeze_special_file|freeze_path_unsafe)", err)
            refuse(mm.group(1) if mm else "freeze_snapshot_moved", "check_freeze_snapshot.sh exited %d" % r.returncode)
        snap = Snapshot(snapdir)
        snap.freeze = fj
        scanned_at = fj.get("frozen_at") or "1970-01-01T00:00:00Z"
        plan, stats = enumerate_snapshot(snap, scanned_at)
        stats["frozen_at_basis"] = "freeze_json" if fj.get("frozen_at") else "constant_fallback"
        stats["freeze_head"] = fj.get("head")
        sqltxt = render_sql(plan, scanned_at)
    except Refusal as e:
        sys.stderr.write("enumerate_sources: REFUSED reason=%s %s\n" % (e.reason, e.detail))
        return 20
    h = write_sql_files(a["out"], "source_entries", sqltxt)
    json.dump(kinds_doc(), open(os.path.join(a["out"], "source-class-kinds.json"), "w"), indent=1, sort_keys=True)
    json.dump(stats, open(os.path.join(a["out"], "enumeration-stats.json"), "w"), indent=1, sort_keys=True)
    print("enumerate_sources: ok entries=%d sql_sha256=%s" % (stats["total_entries"], h))
    return 0


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:]))
