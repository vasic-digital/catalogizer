#!/usr/bin/env bash
# test_decision_intake.sh (tasks.md T011, WP-01): asserts the owner-decision intake file
#   $FEAT/decisions/owner-decisions.yaml against docs/21 section 8 and tasks.md.
# Usage:   bash scripts/governance/tests/test_decision_intake.sh
# Env:     FEAT          feature dir (default: specs/001-full-project-audit-remediation under the repo root)
#          INTAKE_FILE   intake file to check (default: $FEAT/decisions/owner-decisions.yaml)
#          PROGRESS_FILE progress.yml holding the relayed owner answers (default: $FEAT/progress.yml)
# Exit:    0 all assertions hold; 1 at least one FAIL (each FAIL is printed with the offending id).
# DEPENDENCY: the owner_answers stage binds the intake to $FEAT/progress.yml (the source of the relayed
#          answers). progress.yml is conductor execution state and may be untracked; it MUST be committed
#          together with owner-decisions.yaml, this test and $EV/wp01/* (see $EV/wp01/README.md),
#          otherwise a fresh clone fails this stage with a prerequisite message (never a silent pass).
# Needs:   python3 with PyYAML. Reads docs/21 and tasks.md at run time (no copy of their tables here).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
export FEAT="${FEAT:-$ROOT/specs/001-full-project-audit-remediation}"
export INTAKE_FILE="${INTAKE_FILE:-$FEAT/decisions/owner-decisions.yaml}"
export DOCS21="$FEAT/docs/21-master-plan-phases-risks-and-traceability.md"
export TASKS_MD="$FEAT/tasks.md"
export PROGRESS_FILE="${PROGRESS_FILE:-$FEAT/progress.yml}"
python3 - <<'PY'
import os, re, sys
try:
    import yaml
except Exception as e:
    print("FAIL prerequisite: PyYAML not importable: %s" % e); sys.exit(1)

fails = []
def fail(msg): fails.append(msg); print("FAIL " + msg)
def ok(msg): print("PASS " + msg)

intake_path, docs_path, tasks_path = os.environ["INTAKE_FILE"], os.environ["DOCS21"], os.environ["TASKS_MD"]
for p, what in ((docs_path, "docs/21"), (tasks_path, "tasks.md")):
    if not os.path.isfile(p):
        fail("prerequisite: %s absent at %s" % (what, p)); sys.exit(1)
if not os.path.isfile(intake_path):
    fail("intake file absent: %s" % intake_path); print("RESULT FAIL (%d)" % len(fails)); sys.exit(1)
try:
    data = yaml.safe_load(open(intake_path, encoding="utf-8"))
except Exception as e:
    fail("intake file is not valid YAML: %s" % e); print("RESULT FAIL (%d)" % len(fails)); sys.exit(1)
if not isinstance(data, dict):
    fail("intake top level is not a mapping"); print("RESULT FAIL (%d)" % len(fails)); sys.exit(1)

# ---- parse docs/21 section 8 tables ------------------------------------------------
lines = open(docs_path, encoding="utf-8").read().split("\n")
start = next((i for i, l in enumerate(lines) if l.startswith("### 8.1 ")), None)
if start is None:
    fail("docs/21: heading '### 8.1 ' not found (instrument blind)"); print("RESULT FAIL (%d)" % len(fails)); sys.exit(1)
end = next((i for i in range(start, len(lines)) if lines[i].startswith("## 9.")), None)
if end is None:
    fail("docs/21: heading '## 9.' not found after section 8.1 (instrument blind)"); print("RESULT FAIL (%d)" % len(fails)); sys.exit(1)
doc_groups, doc_ods, unparsed = {}, {}, []
for l in lines[start:end]:
    m = re.match(r"\|\s*(ODG-\d+|OD-\d+)\s*\|", l)
    if not m: continue
    cells = [c.strip() for c in l.strip().strip("|").split("|")]
    rid = m.group(1)
    if rid.startswith("ODG-"):
        if len(cells) != 8: unparsed.append(rid); continue
        doc_groups[rid] = dict(topic=cells[1], options=cells[2], rec=cells[3], blocks=cells[4], source=cells[5], rids=cells[6], default=cells[7])
    else:
        if len(cells) != 5: unparsed.append(rid); continue
        doc_ods[rid] = dict(topic=cells[1], default=cells[2], blocks=cells[3], closest=cells[4])
if unparsed: fail("docs/21 section 8 rows with an unexpected cell count: %s" % ", ".join(unparsed))
EXPECT_GROUPS = ["ODG-%02d" % i for i in range(1, 44)]
EXPECT_ODS = ["OD-14","OD-20","OD-23","OD-25","OD-26","OD-36","OD-43","OD-45","OD-49","OD-55","OD-57","OD-68","OD-74","OD-75","OD-76"]
if sorted(doc_groups) != EXPECT_GROUPS: fail("docs/21 section 8.1-8.5 does not carry exactly ODG-01..ODG-43 (parsed %d)" % len(doc_groups))
if "OD-60" not in doc_ods: fail("docs/21 section 8.6 control: OD-60 row not parsed (instrument blind)")
del_od60 = doc_ods.pop("OD-60", None)
if sorted(doc_ods) != sorted(EXPECT_ODS): fail("docs/21 section 8.6 open rows differ from the expected 15: %s" % sorted(doc_ods))

WP = re.compile(r"WP-\d+[A-Z]?")
def doc_blocks(cell):
    if cell.lower().startswith("none"): return [], cell
    ids = sorted(set(WP.findall(cell)))
    return ids, (cell if not ids else None)

TOKENS = ("Yes", "Partial", "No", "UNCONFIRMED")
def lead_token(cell):
    m = re.match(r"(Yes|Partial|No|UNCONFIRMED)\b", cell)
    return m.group(1) if m else None

# ---- structure ------------------------------------------------------------------------
groups, rods = data.get("groups"), data.get("research_ods")
if not isinstance(groups, list): fail("`groups` missing or not a list"); groups = []
if not isinstance(rods, list): fail("`research_ods` missing or not a list"); rods = []
gid = [g.get("id") for g in groups if isinstance(g, dict)]
rid = [r.get("id") for r in rods if isinstance(r, dict)]
if len(gid) != len(groups) or len(rid) != len(rods): fail("a groups/research_ods entry is not a mapping with an id")
if sorted(gid) != EXPECT_GROUPS or len(gid) != 43:
    fail("groups must hold exactly ODG-01..ODG-43 once each; missing=%s extra/dup=%s" % (sorted(set(EXPECT_GROUPS)-set(gid)), sorted(x for x in set(gid) if gid.count(x)>1 or x not in EXPECT_GROUPS)))
else: ok("groups: exactly 43 ids ODG-01..ODG-43, no duplicates")
if sorted(rid) != sorted(EXPECT_ODS) or len(rid) != 15:
    fail("research_ods must hold exactly the 15 open ids %s; got %s" % (EXPECT_ODS, sorted(rid)))
else: ok("research_ods: exactly the 15 open ungrouped decisions (OD-60 absent)")
if set(gid) & set(rid): fail("ids duplicated across groups and research_ods: %s" % sorted(set(gid)&set(rid)))
elif not (gid or rid): fail("duplicate check covered 0 ids (nothing to assert)")
else: ok("no id duplicated across %d ids in both lists" % (len(gid) + len(rid)))

by_g = {g["id"]: g for g in groups if isinstance(g, dict) and "id" in g}
by_r = {r["id"]: r for r in rods if isinstance(r, dict) and "id" in r}

def nonempty(v): return isinstance(v, str) and v.strip() not in ("", "-")
bad = []
n_rows = 0
def norm(x): return re.sub(r"\s+", " ", str(x if x is not None else "")).strip()
for rowid, row, is_group in [(k, v, True) for k, v in by_g.items()] + [(k, v, False) for k, v in by_r.items()]:
    n_rows += 1
    if not (nonempty(row.get("topic")) or nonempty(row.get("options"))): bad.append("%s: no topic or options" % rowid)
    if is_group:
        d = doc_groups.get(rowid)
        if d and nonempty(d["rec"]) and not nonempty(row.get("recommendation")): bad.append("%s: recommendation missing though docs/21 gives one" % rowid)
        if not isinstance(row.get("research_ids"), list): bad.append("%s: research_ids is not a list" % rowid)
        elif not all(re.fullmatch(r"OD-\d{2}", str(x)) for x in row["research_ids"]): bad.append("%s: research_ids holds a non OD-nn value" % rowid)
        elif d and sorted(map(str, row["research_ids"])) != sorted(set(re.findall(r"OD-\d+", d["rids"]))):
            bad.append("%s: research_ids %s != OD ids %s of the docs/21 Research ids cell" % (rowid, sorted(map(str, row["research_ids"])), sorted(set(re.findall(r"OD-\d+", d["rids"])))))
    elif "research_ids" in row: bad.append("%s: research_ids belongs to groups only" % rowid)
if bad: [fail(b) for b in bad]
elif n_rows == 0: fail("structure check covered 0 rows (nothing to assert)")
else: ok("every one of %d rows has topic or options, groups carry recommendation and research_ids" % n_rows)

# ---- topic / options / recommendation equal docs/21 (whitespace-normalised) -------------------
bad, n_eq = [], 0
for rowid, row in list(by_g.items()) + list(by_r.items()):
    d = doc_groups.get(rowid) or doc_ods.get(rowid)
    if not d: continue
    n_eq += 1
    pairs = [("topic", row.get("topic"), d["topic"])]
    if rowid in doc_groups: pairs += [("options", row.get("options"), d["options"]), ("recommendation", row.get("recommendation"), d["rec"])]
    for fld, have, want in pairs:
        if norm(have) != norm(want) and not (norm(want) in ("", "-") and norm(have) in ("", "-")):
            bad.append("%s: %s %r != docs/21 cell %r" % (rowid, fld, norm(have)[:80], norm(want)[:80]))
if bad: [fail(b) for b in bad]
elif n_eq == 0: fail("topic/options/recommendation check covered 0 rows (nothing to assert)")
else: ok("topic, options and recommendation equal the docs/21 cells for all %d rows" % n_eq)

# ---- closest_group of the research_ods equals the docs/21 section 8.6 cell -----------------
bad, n_cg = [], 0
for rowid, row in by_r.items():
    d = doc_ods.get(rowid)
    if not d: continue
    n_cg += 1
    m = re.match(r"(ODG-\d+)\b", d["closest"])
    want = m.group(1) if m else None
    if d["closest"].strip() not in ("-", "") and not m: bad.append("%s: docs/21 closest-group cell %r is neither '-' nor starts with an ODG id (instrument blind)" % (rowid, d["closest"][:40])); continue
    if "closest_group" not in row: bad.append("%s: closest_group key missing" % rowid)
    elif row["closest_group"] != want: bad.append("%s: closest_group %r != docs/21 section 8.6 cell value %r" % (rowid, row["closest_group"], want))
if bad: [fail(b) for b in bad]
elif n_cg == 0: fail("closest_group check covered 0 rows (nothing to assert)")
else: ok("closest_group equals the docs/21 section 8.6 cell (id, or null for '-') for all %d research_ods" % n_cg)

# ---- blocks / blocks_scope (parsed from docs/21 at test time) ---------------------------
bad = []
n_blocks = 0
for rowid, row in list(by_g.items()) + list(by_r.items()):
    d = (doc_groups.get(rowid) or doc_ods.get(rowid))
    if not d: continue
    n_blocks += 1
    want, scope_cell = doc_blocks(d["blocks"])
    have = row.get("blocks")
    if not isinstance(have, list): bad.append("%s: blocks is not a list" % rowid); continue
    if sorted(map(str, have)) != want: bad.append("%s: blocks %s != docs/21 Blocks cell set %s" % (rowid, sorted(map(str, have)), want))
    if not want:
        if (row.get("blocks_scope") or "").strip() != d["blocks"]: bad.append("%s: blocks is empty so blocks_scope must hold the docs/21 cell text verbatim" % rowid)
    elif row.get("blocks_scope"): bad.append("%s: blocks_scope set although blocks is not empty" % rowid)
if bad: [fail(b) for b in bad]
elif n_blocks != len(doc_groups) + len(doc_ods):
    fail("blocks check covered %d rows, docs/21 has %d (rows missing from the intake)" % (n_blocks, len(doc_groups) + len(doc_ods)))
else: ok("blocks equal the docs/21 section 8 Blocks cells as sets for all %d checked rows; empty cells carry blocks_scope verbatim" % n_blocks)

# ---- status / source / blocks_text equal docs/21 (status: every imported decision is Operator-blocked, docs/21 rule 6) ----
bad, n_ssb = [], 0
for rowid, row in list(by_g.items()) + list(by_r.items()):
    d = doc_groups.get(rowid) or doc_ods.get(rowid)
    if not d: continue
    n_ssb += 1
    if row.get("status") != "Operator-blocked": bad.append("%s: status %r != Operator-blocked" % (rowid, row.get("status")))
    if norm(row.get("blocks_text")) != norm(d["blocks"]): bad.append("%s: blocks_text %r != docs/21 Blocks cell %r" % (rowid, norm(row.get("blocks_text"))[:60], norm(d["blocks"])[:60]))
    if rowid in doc_groups and norm(row.get("source")) != norm(d["source"]): bad.append("%s: source %r != docs/21 Source cell %r" % (rowid, norm(row.get("source"))[:60], norm(d["source"])[:60]))
if bad: [fail(b) for b in bad]
elif n_ssb == 0: fail("status/source/blocks_text check covered 0 rows (nothing to assert)")
else: ok("status is Operator-blocked and blocks_text and (groups) source equal the docs/21 cells for all %d rows" % n_ssb)

# ---- default / default_detail -----------------------------------------------------------
bad = []
n_def = 0
for rowid, row in list(by_g.items()) + list(by_r.items()):
    n_def += 1
    df = row.get("default")
    if df not in TOKENS: bad.append("%s: default %r is outside the closed set %s" % (rowid, df, TOKENS)); continue
    d = doc_groups.get(rowid) or doc_ods.get(rowid)
    if d and lead_token(d["default"]) != df: bad.append("%s: default %s != leading token %s of the docs/21 Default cell" % (rowid, df, lead_token(d["default"])))
    detail = row.get("default_detail")
    if d:
        rest = d["default"][len(df):].strip()
        got = str(detail if detail is not None else "").strip()
        if got != rest: bad.append("%s: default_detail %r != the rest of the docs/21 Default cell after the leading token %r (the token is not repeated)" % (rowid, got, rest))
for k in ("ODG-34", "ODG-35"):
    r = by_g.get(k)
    if r and (r.get("default") != "UNCONFIRMED" or not nonempty(r.get("default_detail"))): bad.append("%s: must be default UNCONFIRMED with default_detail" % k)
for k in ("ODG-39", "ODG-40", "ODG-42", "ODG-43"):
    r = by_g.get(k)
    if r and r.get("default") != "No": bad.append("%s: default must be No" % k)
for k in ("ODG-39", "ODG-40", "ODG-41", "ODG-42", "ODG-43"):
    r = by_g.get(k)
    if r and r.get("research_ids") != []: bad.append("%s: research_ids must be an empty list" % k)
r = by_g.get("ODG-42")
if r and sorted(r.get("blocks") or []) != ["WP-72", "WP-74", "WP-74R"]: bad.append("ODG-42: blocks must be WP-72, WP-74, WP-74R")
for k, need in {"ODG-11": {"OD-11"}, "ODG-13": {"OD-30"}, "ODG-15": {"OD-32"}, "ODG-18": {"OD-47"}, "ODG-19": {"OD-39"}, "ODG-33": {"OD-58", "OD-61"}}.items():
    r = by_g.get(k)
    if r and not need <= set(r.get("research_ids") or []): bad.append("%s: research_ids must include %s" % (k, sorted(need)))
r = by_r.get("OD-76")
if r and (r.get("default") != "No" or "blobs" not in str(r.get("default_detail", ""))): bad.append("OD-76: default No with the in-tree blobs partial-proceed note in default_detail")
if bad: [fail(b) for b in bad]
elif n_def == 0: fail("default check covered 0 rows (nothing to assert)")
else: ok("default is a bare closed-set token, default_detail holds the rest without the token (%d rows); ODG-34/35/39/40/42/43 and OD-76 shapes hold" % n_def)

# ---- fixed needles for the T011-named examples (written here, independent of the docs/21 parser) ----
NEEDLES = {"OD-76": ["WP-05", "WP-74"], "OD-20": ["WP-61", "WP-71"], "OD-26": ["WP-61", "WP-70"],
           "ODG-40": ["WP-35", "WP-57"], "ODG-42": ["WP-72", "WP-74", "WP-74R"],
           "ODG-19": [], "ODG-34": [], "ODG-35": [], "ODG-43": []}
SCOPE_NEEDLES = {"ODG-19": "all review gates", "ODG-34": "all phases"}
bad = []
for k, want in NEEDLES.items():
    row = by_g.get(k) or by_r.get(k)
    if row is None: bad.append("needle %s: row absent from the intake" % k); continue
    if sorted(map(str, row.get("blocks") or [])) != want: bad.append("needle %s: blocks %s != fixed expectation %s" % (k, row.get("blocks"), want))
    if not want and not nonempty(row.get("blocks_scope")): bad.append("needle %s: empty blocks needs a non-empty blocks_scope" % k)
for k, txt in SCOPE_NEEDLES.items():
    row = by_g.get(k)
    if row is not None and (row.get("blocks_scope") or "").strip() != txt: bad.append("needle %s: blocks_scope %r != %r" % (k, row.get("blocks_scope"), txt))
if bad: [fail(b) for b in bad]
else: ok("fixed needles hold for OD-76, OD-20, OD-26, ODG-40, ODG-42 and the empty-blocks ODG-19/34/35/43 (%d rows)" % len(NEEDLES))

# ---- tasks.md two-way rule ----------------------------------------------------------------
group_of_od = {}
for g in by_g.values():
    for x in g.get("research_ids") or []: group_of_od.setdefault(x, g["id"])
tl = open(tasks_path, encoding="utf-8").read().split("\n")
pkg, found, bad = None, 0, []
MARK = re.compile(r"BLOCKED-ON\s+((?:ODG?-\d+)(?:\s*(?:,|and|/)\s*ODG?-\d+)*)")
for n, l in enumerate(tl, 1):
    h = re.match(r"#{2,4}\s+(.*)", l)
    if h:
        m = re.match(r"(WP-\d+[A-Z]?)\b", h.group(1))
        pkg = m.group(1) if m else None
        continue
    if pkg is None: continue
    for m in MARK.finditer(l):
        for dec in re.findall(r"ODG?-\d+", m.group(1)):
            found += 1
            key = dec if dec.startswith("ODG-") else group_of_od.get(dec, dec)
            row = by_g.get(key) or by_r.get(key)
            if row is None: bad.append("tasks.md:%d %s names %s which is in no intake row" % (n, pkg, dec))
            elif pkg not in (row.get("blocks") or []): bad.append("tasks.md:%d BLOCKED-ON %s in %s but %s blocks %s" % (n, dec, pkg, key, row.get("blocks")))
if found == 0: fail("tasks.md control: no BLOCKED-ON marker inside a work package was parsed (instrument blind)")
elif bad: [fail(b) for b in bad]
else: ok("tasks.md: all %d BLOCKED-ON markers inside work packages are listed in the decision's blocks" % found)

# ---- no credential value field -------------------------------------------------------------
KEY = re.compile(r"(?i)(^|_)(password|passwd|secret|token|api_?key|private_?key|credential_?value|value)($|_)")
NAME_ONLY = re.compile(r"(?i)_(name|names|ref|var|env)$")  # a field that names an env var, never holds the value
VAL = re.compile(r"(AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{20,}|-----BEGIN [A-Z ]*PRIVATE KEY-----|AIza[0-9A-Za-z_-]{30,})")
FREE = re.compile(r"(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key)\s*[:=]\s*(?![<$]|\.\.\.)\S{6,}")  # free text 'password=value'; a placeholder <...> or $VAR is not a value
bad = []
n_str = [0]
def walk(o, path):
    if isinstance(o, str): n_str[0] += 1
    if isinstance(o, dict):
        for k, v in o.items():
            if KEY.search(str(k)) and not NAME_ONLY.search(str(k)): bad.append("%s.%s: credential-like field name" % (path, k))
            walk(v, "%s.%s" % (path, k))
    elif isinstance(o, list):
        for i, v in enumerate(o): walk(v, "%s[%d]" % (path, i))
    elif isinstance(o, str) and (VAL.search(o) or FREE.search(o)): bad.append("%s: credential-like value pattern (value not printed)" % path)
walk(data, "$")
if bad: [fail(b) for b in bad]
elif n_str[0] == 0: fail("credential scan covered 0 string values (nothing to assert)")
else: ok("no credential value field or credential-shaped value in %d string values" % n_str[0])

# ---- owner_answers reserved slots ------------------------------------------------------------
oa = data.get("owner_answers")
if not isinstance(oa, list): fail("`owner_answers` missing or not a list")
else:
    slots = {}
    for e in oa:
        if isinstance(e, dict) and "slot" in e: slots.setdefault(e["slot"], []).append(e)
    bad = []
    for s in ("ODG-42", "C2-custody", "floors-adoption"):
        es = slots.get(s, [])
        if len(es) != 1: bad.append("reserved slot %s must be present exactly once (found %d)" % (s, len(es))); continue
        t, d = (es[0].get("text") or ""), (es[0].get("date") or "")
        if bool(str(t).strip()) != bool(str(d).strip()): bad.append("slot %s: text and date must be both empty (unanswered) or both filled" % s)
    known = set(by_g) | set(by_r) | {"C1", "C2", "FR-017", "HC-0"}
    for e in oa:
        if not isinstance(e, dict) or "slot" in e: continue
        eid = e.get("id", "?")
        if "relayed_by" in e and e.get("verbatim") is not False: bad.append("owner_answers %s: relayed entry must have verbatim: false (got %r)" % (eid, e.get("verbatim")))
        if "relayed_by" in e and not str(e.get("relayed_by") or "").strip(): bad.append("owner_answers %s: relayed_by is empty" % eid)
        if not str(e.get("text") or "").strip() or not str(e.get("date") or "").strip(): bad.append("owner_answers %s: text and date must be non-empty" % eid)
        ans = e.get("answers")
        if not isinstance(ans, list) or not ans: bad.append("owner_answers %s: answers must be a non-empty list" % eid)
        else:
            for a in ans:
                if str(a) not in known: bad.append("owner_answers %s: answers names unknown id %s" % (eid, a))
    if set(slots) - {"ODG-42", "C2-custody", "floors-adoption"}: bad.append("unknown reserved slot(s): %s" % sorted(set(slots) - {"ODG-42","C2-custody","floors-adoption"}))
    if bad: [fail(b) for b in bad]
    else: ok("owner_answers holds the reserved slots ODG-42, C2-custody, floors-adoption (a slot is not a group)")


# ---- owner_answers carry progress.yml faithfully (2026-10-05 intake; binding source = progress.yml) ----
pp = os.environ["PROGRESS_FILE"]
if not os.path.isfile(pp): fail("prerequisite: progress.yml absent at %s (the owner_answers stage binds the intake to it; commit it with the intake, see specs/001-full-project-audit-remediation/evidence/wp01/README.md)" % pp)
else:
    raw_text = open(pp, encoding="utf-8").read()   # read ONCE: the parsed tree and the raw lines come from the same bytes (review m8)
    prog = yaml.safe_load(raw_text) or {}
    raw_prog = raw_text.split("\n")
    # Fixed key -> decision id mapping written here (independent of the intake): review_substrate answers ODG-19; the hc0 confirm_*/accept_* keys answer C1, C2, FR-017, HC-0.
    ALIAS = {"review_substrate": ["ODG-19"], "confirm_C1_remote_event_driven_builds": ["C1"], "confirm_C2_slsa_l2_minimum": ["C2"],
             "confirm_FR017_latest_stable": ["FR-017"], "accept_plan_start_phase_0": ["HC-0"],
             # batch3/batch4 keys (2026-10-05) that answer a decision but carry no id in their name; the mapping is written here (owner brief), independent of the intake:
             "CAT_vs_constitution_ATM": ["ODG-11"], "atm_id_columns": ["ODG-11"], "image_pulls": ["ODG-07"],
             "removals": ["ODG-23", "OD-75"], "llms_verifier": ["ODG-14"], "internal_media": ["ODG-23", "OD-68"], "Firebase": ["OD-43", "OD-45"]}
    BLK = re.compile(r"owner_answers_(\d{4}-\d{2}-\d{2})")
    def keyids(key):
        if key in ALIAS: return list(ALIAS[key])
        m = re.match(r"(ODG?)-(\d+)((?:_\d+)*)(?:_|$)", key)
        if not m: return []   # no decision id: carried in owner_answers_without_decision_id
        return ["%s-%s" % (m.group(1), m.group(2))] + ["%s-%s" % (m.group(1), x) for x in re.findall(r"_(\d+)", m.group(3))]
    src = {}   # dotted source key -> (value, date, key)
    blk_of = {}   # dotted source key -> the owner_answers_<date> block name that holds it
    nondict = []  # owner_answers_<date> keys whose value is not a mapping: their answers would be invisible to this test (review D1)
    def blocks(o, path):
        if isinstance(o, dict):
            for k, v in o.items():
                if BLK.match(str(k)):
                    if isinstance(v, dict):
                        for kk, vv in v.items():
                            src[".".join(path + [str(k), str(kk)])] = (norm(vv), BLK.match(str(k)).group(1), str(kk)); blk_of[".".join(path + [str(k), str(kk)])] = str(k)
                    else: nondict.append(".".join(path + [str(k)]))
                else: yield from blocks(v, path + [str(k)])
        return iter(())
    list(blocks(prog, []))
    # 2026-10-07 split (T012a): blocks dated on or after CUT are checked by the stage "owner answers of 2026-10-07" below; this legacy stage
    # keeps its exact 2026-10-05/06 semantics (one entry per decision id, notes = comment + fixed flag), so a later answer never makes it ambiguous.
    CUT = "2026-10-07"
    src_new = {k: v for k, v in src.items() if v[1] >= CUT}
    src = {k: v for k, v in src.items() if v[1] < CUT}
    # review WF3 I5: an answer block that does not match owner_answers_<date> would be invisible to this stage while the PASS line
    # still claims fidelity. Closed allow-list of the top-level keys of progress.yml; every other key must be a dated answer block,
    # and no key anywhere may look like an answer block (contains 'answer') unless it is one.
    TOP_ALLOWED = {"feature", "phase", "phase_status", "hc0", "risks_recorded", "tasks"}
    BLK_FULL = re.compile(r"owner_answers_\d{4}-\d{2}-\d{2}(?:_[A-Za-z0-9]+)*$")
    for k in prog:
        if str(k) in TOP_ALLOWED: continue
        if BLK_FULL.match(str(k)): continue
        fail("progress.yml top-level key %r is neither on the closed allow-list %s nor a dated owner_answers_<yyyy-mm-dd>[_suffix] block (its answers would be invisible to this test)" % (str(k), sorted(TOP_ALLOWED)))
    def answerlike(o, path):
        if isinstance(o, dict):
            for k, v in o.items():
                if re.search(r"answer", str(k), re.I) and not BLK_FULL.match(str(k)): fail("progress.yml key %s looks like an answer block but does not match owner_answers_<yyyy-mm-dd>[_suffix]" % ".".join(path + [str(k)]))
                answerlike(v, path + [str(k)])
        elif isinstance(o, list):
            for i, v in enumerate(o): answerlike(v, path + [str(i)])
    answerlike(prog, [])
    if BLK_FULL.match("owner_answers_batch5") or not BLK_FULL.match("owner_answers_2026-10-06_batch5"): fail("I5 control needle: the answer-block name pattern is wrong (instrument blind)")
    for x in nondict: fail("progress.yml %s is not a mapping of answer keys (a list or scalar block would hide its answers from this test)" % x)
    if not src: fail("progress.yml control: no owner_answers_<date> block parsed (instrument blind)")
    ids_of = {sk: keyids(k) for sk, (v, d, k) in src.items()}
    answered = sorted({d for l in ids_of.values() for d in l})
    if len([x for x in answered if x.startswith("ODG-")]) < 40 or len([x for x in answered if x.startswith("OD-")]) < 14:
        fail("progress.yml parse control: only %d ODG and %d OD ids extracted (instrument blind)" % (len([x for x in answered if x.startswith("ODG-")]), len([x for x in answered if x.startswith("OD-")])))
    entries = [e for e in (oa or []) if isinstance(e, dict) and "slot" not in e and str(e.get("date")) < CUT]
    unk = data.get("owner_answers_without_decision_id")
    unk = [e for e in (unk if isinstance(unk, list) else []) if str(e.get("date")) < CUT]
    SUF = " (relayed by the conductor, not a verbatim owner quotation)."
    def trailing_comment(s):
        """YAML-aware: the text after the first '#' that is outside a quoted scalar and preceded by whitespace (review D2)."""
        q = None; i = 0
        while i < len(s):
            c = s[i]
            if q is None:
                if c in "\"'" and (i == 0 or s[i-1] in " \t"): q = c
                elif c == "#" and i > 0 and s[i-1] in " \t": return s[i+1:].strip()
            elif q == '"':
                if c == "\\": i += 1
                elif c == '"': q = None
            else:
                if c == "'":
                    if i + 1 < len(s) and s[i+1] == "'": i += 1
                    else: q = None
            i += 1
        return None
    def block_range(bname):
        """raw_prog line indexes of the lines nested under the first line that opens `bname:` (indent greater than the header's)."""
        for i, l in enumerate(raw_prog):
            m = re.match(r"(\s*)%s:" % re.escape(bname), l)
            if m:
                ind = len(m.group(1)); j = i + 1
                while j < len(raw_prog) and (not raw_prog[j].strip() or raw_prog[j].lstrip().startswith("#") or len(raw_prog[j]) - len(raw_prog[j].lstrip()) > ind): j += 1
                return range(i + 1, j)
        return range(0)
    def comment_of(sk):
        key = src[sk][2]; rng = block_range(blk_of[sk])
        for i in rng:
            m = re.match(r"\s*%s:(\s.*)$" % re.escape(key), raw_prog[i])
            if m: return trailing_comment(m.group(1))
        return None
    bad, referenced = [], {}
    def check_entry(e, label, want_ids):
        sks = e.get("source_keys")
        if not isinstance(sks, list) or not sks: bad.append("%s: source_keys missing (an entry with no progress.yml source is invented)" % label); return
        vals = []
        for sk in sks:
            if sk not in src: bad.append("%s: source key %s is not in progress.yml" % (label, sk)); continue
            referenced.setdefault(sk, []).append(label)
            vals.append(src[sk][0])
            if str(e.get("date")) != src[sk][1]: bad.append("%s: date %s != the date %s of its progress.yml block %s" % (label, e.get("date"), src[sk][1], sk))
            for a in (e.get("answers") or []):
                if a not in ids_of[sk]: bad.append("%s: answers %s is not derived from source key %s" % (label, a, sk))
            if want_ids is None and ids_of[sk]: bad.append("%s: source key %s answers decision ids %s but the entry is in the id-less list" % (label, sk, ids_of[sk]))
        if len(vals) == len(sks) and norm(e.get("text")) != norm("; ".join(vals) + SUF):
            bad.append("%s: text %r != progress.yml value(s) %r plus the relay suffix (meaning drift or invented content)" % (label, norm(e.get("text"))[:70], "; ".join(vals)[:70]))
        if e.get("relayed_by") != "conductor": bad.append("%s: relayed_by must be conductor" % label)
        if e.get("verbatim") is not False: bad.append("%s: verbatim must be false" % label)
        if "note" not in e: bad.append("%s: note key missing (null or text)" % label)
        if "amends" not in e: bad.append("%s: amends key missing (null until T012a records amendments)" % label)
        elif e.get("amends") is not None: bad.append("%s: a relayed entry must carry amends: null until T012a records amendments (got %r: invented provenance)" % (label, e.get("amends")))
        note = str(e.get("note") or "")
        for tok in ("OPEN", "UNCONFIRMED"):
            if any(tok in v for v in vals) and tok not in note: bad.append("%s: progress.yml value carries %s but the note does not" % (label, tok))
        if len(sks) == 1 and sks[0] in src:
            c = comment_of(sks[0])
            if c and norm(c) not in norm(note): bad.append("%s: progress.yml comment %r is not carried in the note (conductor caveat dropped)" % (label, c))
    for dec in answered:
        hits = [e for e in entries if dec in [str(a) for a in (e.get("answers") or [])]]
        if len(hits) != 1: bad.append("%s: answered in progress.yml but %d owner_answers entries name it (need exactly 1)" % (dec, len(hits))); continue
        check_entry(hits[0], dec, [dec])
    for e in entries:
        for a in (e.get("answers") or []):
            if str(a) not in answered: bad.append("%s: entry answers %s which progress.yml does not answer (invented entry)" % (e.get("id"), a))
    for e in unk: check_entry(e, str(e.get("id")), None)
    for sk, (v, d, k) in src.items():
        if sk not in referenced: bad.append("%s: progress.yml answer (block date %s) is carried by no owner_answers entry (later or unknown block)" % (sk, d))
    n_unk_keys = sum(1 for l in ids_of.values() if not l)
    if len(unk) != n_unk_keys: bad.append("owner_answers_without_decision_id must list the %d progress.yml answers without a decision id (found %d)" % (n_unk_keys, len(unk)))
    if prog.get("risks_recorded") != data.get("risks_recorded"): bad.append("risks_recorded in the intake differs from progress.yml risks_recorded (conductor caveats dropped)")
    NOTE_FLAGS = {  # the fixed review-flag texts of the notes, written here (review D3-D5): a note is [comment part] + exactly this text
    'ODG-07': 'Conductor-stated facts, not owner-confirmed: capacity 16 CPU, about 31 GiB RAM, podman 5.7.0 rootless; shared build and measurement host under the 12.6 60 percent ceiling (see risks_recorded). Reachability, roles, and the earlier thinker.local and amber.local proposal are not recorded as answered.',
    'C1': 'See risks_recorded: the 11.4.173 remote-distribution deviation is an owner decision to be cited in HC-0.',
    'FR-017': '',
    'ODG-02': 'Risk noted by the owner choice: history still contains the credentials (constitution 11.4.209(D) treats a committed plaintext credential as compromised); the owner is asked at HC-0 to accept this residual risk explicitly.',
    'ODG-03': 'Risk noted by the owner choice: history still contains the credentials (constitution 11.4.209(D) treats a committed plaintext credential as compromised); the owner is asked at HC-0 to accept this residual risk explicitly.',
    'ODG-05': "OPEN: partial answer. The emulator acceptance scope (hardware-independent behaviour only, or broader) is not answered by 'emulators only for now'.",
    'ODG-08': 'OPEN: the owner has not yet named the NFS host. The 2026-10-05 batch4 answer narrows the scope to a user-space NFS server only for now; real NFS host proof stays UNMET until a host is named.',
    "ODG-11": "UNCONFIRMED: the location docs/workable_items.db is kept but marked UNCONFIRMED in the relayed answer; the constitution submodule ATM text needs a separate approved upstream change. OPEN: the owner answer CAT is carried here as an owner answer, NOT as the recommendation (the group row keeps the question and recommendation as recorded before the answer); it deviates from the constitution, because the constitution (11.4.54, 11.4.248) mandates the ATM prefix, and this project's CAT is an owner-approved exception to that literal, recorded, with the upstream change request pending (see risks_recorded).",
    'ODG-14': 'OPEN: the disposition waits for the history investigation S-LLMV-1 (equals the recommendation). The batch3 owner statement (a real submodule at submodules/llms_verifier tracking the latest main) is a direction, not yet a recorded decision; the conversion and the helix_qa layout fix are tracked work, not done.',
    'ODG-16': "OPEN (flagged by the independent review, not decided here): conflicts with the C2-custody requirement (tasks.md T012a slot C2-custody and T446: the signing key is held only on the build host, out of the producing stream's reach). The agents run as uid 1000 on this single-uid host, the same user whose keyring would hold the key. UNCONFIRMED: whether the keyring is unlock-gated against the stream. The batch3 answer adds that the key is locked by a passphrase only the owner enters at signing time and records the honest strength as policy (not mechanism); that the passphrase gate holds against the stream is likewise UNCONFIRMED. The reserved slot C2-custody stays empty; the owner decides at HC-0.",
    'ODG-17': 'OPEN: partial answer. The SC-005 sample size and the register category mapping are unanswered; legacy targets wait for the baselines.',
    'ODG-19': 'UNCONFIRMED: a prior independent review ran at an unrecorded effort (source: tasks.md T011a and the message of commit a27d72a5); an xhigh-proven re-review needs the Workflow path.',
    'ODG-23': 'OPEN: deferral, not a decision: waits for the history investigation, then a per-item decision. The batch3 answers (remove only with evidence of non-use, in separate commits; wire in and test the internal media) are conditions for that per-item decision, not decisions.',
    'ODG-40': "OPEN: partial answer. Only 'shipped code' is answered; the server-image and tool-only lists and a licence for each own-organisation repository without one are unanswered.",
    'ODG-41': 'OPEN: the relayed answer maps to none of options (a) compare against the reference tips with named exceptions, (b) push freeze, (c) re-run until no upstream moves; the owner says which option it is.',
    'ODG-42': 'OPEN: deferral, not a decision: waits for the T525 per-class report (batch4: per-class report in phase 2, then decide). UNCONFIRMED: T525 concerns ODG-43 (document classes C and D) only and does not produce the headerless count; confirm at HC-0 whether the owner meant ODG-42 to wait on it. The reserved slot ODG-42 stays empty.',
    'ODG-43': 'OPEN: deferral, not a decision: waits for the T525 per-class claim and mismatch counts.',
    'OD-23': 'OPEN: deferral, not a decision: the production open path is read first, then the owner confirms.',
    'OD-43': "OPEN: needs the owner's Firebase CLI login (owner action, once) and an explicit confirmation before any enabling or creation of anything new; the batch4 answer names the existing project catalogizer-7a3f1.",
    'OD-45': "OPEN: needs the owner's Firebase CLI login (owner action, once) and an explicit confirmation before any enabling or creation of anything new; the batch4 answer names the existing project catalogizer-7a3f1.",
    'OD-49': 'OPEN: deferral, not a decision: decided from the census.',
    'OD-68': 'OPEN: the owner does not know; to be verified. The batch3 answer (wire in and test the internal media) is said to resolve it, UNCONFIRMED until the production open path is read.',
    'OD-75': 'OPEN: the owner does not know; to be investigated. The batch3 removal answer applies only with evidence of non-use.',
    'OAU-2026-10-05-4': 'OPEN: the owner has not answered the four points of the constitution continuum chain (hash construction, record schema, exit codes, anchor format); the python chain of docs/06 is interim until a container build exists.',
    'OAU-2026-10-05-2': 'UNCONFIRMED: that swap satisfies the 32 GiB MemAvailable check; the index-writer swap is not confirmed.',
    }
    CPFX = "Conductor caveat (progress.yml comment): "
    def exact_note(e, label):
        """a note is exactly: [CPFX + the progress.yml comment + '.'] + [' ' +] the fixed flag text for the entry; nothing else may appear."""
        sks = e.get("source_keys") or []
        c = comment_of(sks[0]) if len(sks) == 1 and sks[0] in src else None
        key = str(e.get("id")) if e in unk else str((e.get("answers") or [None])[0])
        flag = NOTE_FLAGS.get(key, "")
        want = ""
        if c: want = CPFX + c + ("" if c.endswith(".") else ".")
        if flag: want = (want + " " if want else "") + flag
        got = str(e.get("note") or "")
        if norm(got) != norm(want): bad.append("%s: note %r != the comment-part plus fixed review-flag text %r (an unsourced or altered note)" % (label, norm(got)[:90], norm(want)[:90]))
    for e in entries: exact_note(e, str(e.get("id")))
    for e in unk: exact_note(e, str(e.get("id")))
    unused = sorted(set(NOTE_FLAGS) - {str(e.get("id")) for e in unk} - {str(a) for e in entries for a in (e.get("answers") or [])})
    if unused: bad.append("NOTE_FLAGS names ids with no entry: %s" % unused)
    # fixed expectations for deferrals, partial answers, and the review findings (written here, independent of the intake)
    by_dec = {str(a): e for e in entries for a in (e.get("answers") or [])}
    def note_of(d): return str((by_dec.get(d) or {}).get("note") or "")
    for d in ("ODG-14", "ODG-23", "ODG-42", "ODG-43", "OD-23", "OD-49", "OD-68", "OD-75", "ODG-05", "ODG-17", "ODG-40", "ODG-41"):
        if "OPEN" not in note_of(d): bad.append("%s: a deferral or partial answer must carry an OPEN note" % d)
    if "T525" not in note_of("ODG-42") or "UNCONFIRMED" not in note_of("ODG-42"): bad.append("ODG-42: note must say UNCONFIRMED that T525 produces the headerless count")
    if "ODG-43" not in str(by_dec.get("ODG-43", {}).get("answers")) or "ODG-42" in note_of("ODG-43"): bad.append("ODG-43: its note must not conflate ODG-42")
    for tok in ("OPEN", "C2-custody", "UNCONFIRMED"):
        if tok not in note_of("ODG-16"): bad.append("ODG-16: note must flag the custody conflict (missing %r)" % tok)
    if "progress.yml" in note_of("ODG-19") or "T011a" not in note_of("ODG-19"): bad.append("ODG-19: note must cite tasks.md T011a and not attribute the fact to progress.yml")
    if "XYZ" in " ".join(note_of(d) for d in by_dec): bad.append("an invented XYZ-NNN UNCONFIRMED note is present (progress.yml does not mark it)")
    for d in ("ODG-02", "ODG-03"):
        if "residual risk" not in note_of(d): bad.append("%s: note must ask the owner to accept the residual risk" % d)
    if bad: [fail(b) for b in bad]
    elif not answered: fail("owner_answers coverage covered 0 ids (nothing to assert)")
    else: ok("owner_answers carry progress.yml faithfully: %d answered ids + %d id-less answers over %d source keys; text = progress.yml value + relay suffix, dates from block names, every note = progress.yml comment + a fixed review-flag text, no invented or unsourced entry" % (len(answered), len(unk), len(src)))


# ---- owner answers of 2026-10-07 (T012a, T014): relayed answers carried faithfully, HC-0 record consistent with progress.yml ----
if os.path.isfile(pp):
    bad = []
    CUT7 = "2026-10-07"
    ALIAS7 = {"HC-0_confirmations": ["HC-0", "ODG-07", "ODG-11"], "Firebase_login_owner": ["OD-43", "OD-45"]}
    # independent expectation, written here: the decision ids the 2026-10-07 owner answers touch (owner memory file project_owner_decisions_2026_10_07)
    WANT7 = {"ODG-05", "ODG-07", "ODG-08", "ODG-11", "ODG-16", "ODG-17", "ODG-32", "ODG-40", "OD-43", "OD-45", "HC-0"}
    def keyids7(key):
        if key in ALIAS7: return list(ALIAS7[key])
        m = re.match(r"(ODG?)-(\d+)((?:_\d+)*)(?:_|$)", key)
        if not m: return []
        return ["%s-%s" % (m.group(1), m.group(2))] + ["%s-%s" % (m.group(1), x) for x in re.findall(r"_(\d+)", m.group(3))]
    ids7 = {sk: keyids7(k) for sk, (v, d, k) in src_new.items()}
    got7 = {x for l in ids7.values() for x in l}
    if len(src_new) < 12: bad.append("2026-10-07 control: only %d answer keys parsed from progress.yml (instrument blind or answers missing)" % len(src_new))
    if got7 != WANT7: bad.append("2026-10-07 answered decision ids %s != expected %s" % (sorted(got7), sorted(WANT7)))
    for sk, (v, d, k) in src_new.items():
        if not str(v).strip(): bad.append("%s: empty answer value" % sk)
    e7 = [e for e in (oa or []) if isinstance(e, dict) and "slot" not in e and str(e.get("date")) >= CUT7]
    u7 = [e for e in (data.get("owner_answers_without_decision_id") or []) if isinstance(e, dict) and str(e.get("date")) >= CUT7]
    ref7 = {}
    for e in e7 + u7:
        label = str(e.get("id"))
        sks = e.get("source_keys")
        if not isinstance(sks, list) or len(sks) != 1: bad.append("%s: source_keys must list exactly one progress.yml key" % label); continue
        sk = sks[0]
        if sk not in src_new: bad.append("%s: source key %s is not a 2026-10-07 progress.yml answer" % (label, sk)); continue
        ref7.setdefault(sk, []).append(label)
        v, d, k = src_new[sk]
        if str(e.get("date")) != d: bad.append("%s: date %s != block date %s" % (label, e.get("date"), d))
        if norm(e.get("text")) != norm(v + " (relayed by the conductor, not a verbatim owner quotation)."): bad.append("%s: text != progress.yml value plus the relay suffix (meaning drift)" % label)
        if e.get("relayed_by") != "conductor" or e.get("verbatim") is not False: bad.append("%s: relayed entry must have relayed_by conductor and verbatim false" % label)
        if e.get("relay_marker") != "relayed 2026-10-07": bad.append("%s: relay_marker must be 'relayed 2026-10-07'" % label)
        if "amends" not in e or e.get("amends") is not None: bad.append("%s: amends must be null (no document is amended by this intake)" % label)
        if "note" not in e: bad.append("%s: note key missing" % label)
        if e in e7:
            if sorted(str(a) for a in (e.get("answers") or [])) != sorted(ids7[sk]): bad.append("%s: answers %s != ids derived from the key %s" % (label, e.get("answers"), ids7[sk]))
        elif ids7[sk]: bad.append("%s: id-less list but the key answers %s" % (label, ids7[sk]))
        note = str(e.get("note") or "")
        for tok in ("OPEN", "UNCONFIRMED"):
            if tok in v and tok not in note: bad.append("%s: the answer carries %s but the note does not" % (label, tok))
    for sk in src_new:
        if len(ref7.get(sk, [])) != 1: bad.append("%s: carried by %d entries (need exactly 1)" % (sk, len(ref7.get(sk, []))))
    # decision rows point at their 2026-10-07 entries (two-way); status stays as the intake had it (changing it is T072's act)
    rows7 = {**by_g, **by_r}
    for dec in sorted(WANT7 - {"HC-0"}):
        want_refs = sorted(str(e.get("id")) for e in e7 if dec in [str(a) for a in (e.get("answers") or [])])
        row = rows7.get(dec) or {}
        if sorted(str(x) for x in (row.get("answer_refs") or [])) != want_refs: bad.append("%s: answer_refs %s != the 2026-10-07 entries naming it %s" % (dec, row.get("answer_refs"), want_refs))
    for rid, row in rows7.items():
        if rid not in WANT7 and row.get("answer_refs"): bad.append("%s: answer_refs set but no 2026-10-07 answer names it" % rid)
    try:
        if int(data.get("revision") or 0) < 5: bad.append("owner-decisions.yaml revision must be 5 or more (got %r)" % data.get("revision"))
    except Exception: bad.append("owner-decisions.yaml revision is not an integer: %r" % data.get("revision"))
    # the reserved slots: the 2026-10-07 round fills none of them (only a verbatim owner text may)
    for s_ in (e for e in (oa or []) if isinstance(e, dict) and "slot" in e):
        if str(s_.get("text") or "").strip(): bad.append("reserved slot %s is filled: only a verbatim owner text may fill it" % s_.get("slot"))
    # owner request list carries the answers
    rlp = os.path.join(os.environ["FEAT"], "decisions", "owner-request-list.md")
    rlt = open(rlp, encoding="utf-8").read() if os.path.isfile(rlp) else ""
    i7 = rlt.find("## 10. Answers of 2026-10-07")
    if i7 < 0: bad.append("owner-request-list.md lacks the section '## 10. Answers of 2026-10-07'")
    else:
        sec = rlt[i7:]
        if "relayed 2026-10-07" not in sec: bad.append("owner-request-list.md section 10 lacks the marker 'relayed 2026-10-07'")
        for dec in sorted(WANT7):
            if dec not in sec: bad.append("owner-request-list.md section 10 does not mention %s" % dec)
    # HC-0 evidence record (T014) consistent with progress.yml, sealed by SHA256SUMS
    import json, hashlib
    hcd = os.path.join(os.environ["FEAT"], "evidence", "hc")
    hcp = os.path.join(hcd, "HC-0.json")
    if not os.path.isfile(hcp): bad.append("$EV/hc/HC-0.json absent")
    else:
        try: hc = json.load(open(hcp, encoding="utf-8"))
        except Exception as ex: hc = None; bad.append("HC-0.json is not valid JSON: %s" % ex)
        if isinstance(hc, dict):
            idh = hc.get("identity") or {}
            for f_ in ("generated_at", "git_head", "host", "tool"):
                if not str(idh.get(f_) or "").strip(): bad.append("HC-0.json identity.%s missing" % f_)
            if not re.fullmatch(r"[0-9a-f]{40}", str(idh.get("git_head") or "")): bad.append("HC-0.json identity.git_head is not a 40-hex commit id")
            for f_ in ("date", "delivered", "owner_answers", "launcher_trust_answer", "remaining"):
                if f_ not in hc: bad.append("HC-0.json lacks field %s" % f_)
            if not hc.get("delivered"): bad.append("HC-0.json delivered is empty")
            if "acceptance_statement" not in hc and "open_questions" not in hc: bad.append("HC-0.json has neither acceptance_statement nor open_questions")
            if "entry_incomplete" in hc: bad.append("HC-0.json still carries entry_incomplete although the host identity is answered (2026-10-05 and 2026-10-07)")
            if hc.get("host_identity_answered") is not True: bad.append("HC-0.json host_identity_answered must be true (ODG-07 answered)")
            rem = hc.get("remaining")
            if not isinstance(rem, list): bad.append("HC-0.json remaining must be a list")
            else:
                pr = (prog.get("hc0") or {}).get("recorded")
                if pr is not (len(rem) == 0): bad.append("progress.yml hc0.recorded (%r) disagrees with HC-0.json remaining (%d entries): recorded may be true only when nothing remains" % (pr, len(rem)))
            lt = hc.get("launcher_trust_answer") or {}
            if lt.get("verbatim") is True and not str(lt.get("text") or "").strip(): bad.append("HC-0.json launcher_trust_answer claims verbatim without text")
            if lt.get("verbatim") is not True and "UNCONFIRMED" not in str(lt.get("status") or ""): bad.append("HC-0.json launcher_trust_answer is not verbatim and not marked UNCONFIRMED")
    sp = os.path.join(hcd, "SHA256SUMS")
    if not os.path.isfile(sp): bad.append("$EV/hc/SHA256SUMS absent")
    else:
        sums = open(sp, encoding="utf-8").read()
        nsum = 0
        for ln in sums.splitlines():
            m_ = re.fullmatch(r"([0-9a-f]{64})  (\S+)", ln)
            if not m_: bad.append("SHA256SUMS: malformed line %r" % ln[:60]); continue
            fp = os.path.join(hcd, m_.group(2)); nsum += 1
            if not os.path.isfile(fp): bad.append("SHA256SUMS: %s absent" % m_.group(2)); continue
            if hashlib.sha256(open(fp, "rb").read()).hexdigest() != m_.group(1): bad.append("SHA256SUMS: %s does not verify" % m_.group(2))
        if nsum == 0 or "  HC-0.json" not in sums: bad.append("SHA256SUMS lists no HC-0.json (instrument blind)")
    if bad: [fail(b) for b in bad]
    else: ok("owner answers of 2026-10-07: %d relayed keys carried by %d entries, answer_refs two-way for %d ids, request list section 10, HC-0.json sealed and consistent with progress.yml" % (len(src_new), len(e7) + len(u7), len(WANT7) - 1))

# ---- round 4 (review WF3 I2, I3): ONE precedence statement for the ODG-11 deviation; the answered-but-owed items are tracked ----
FEATD = os.environ["FEAT"]
def rd(rel):
    p = os.path.join(FEATD, rel)
    if not os.path.isfile(p): fail("round-4 stage: %s absent" % rel); return None
    return re.sub(r"\s+", " ", open(p, encoding="utf-8").read())
PREC = "owner-approved exception to that literal, recorded, with the upstream change request pending"
BANNED = re.compile(r"constitution wins|wins meanwhile|wins until|the constitution wins", re.I)
prec_files = ["progress.yml", "decisions/owner-decisions.yaml", "decisions/owner-request-list.md", "docs/04-findings-register-design.md",
              "docs/21-master-plan-phases-risks-and-traceability.md", "research.md", "docs/upstream/constitution-prefix-change-request.md"]
texts = {}
for rel in prec_files:
    t = rd(rel); texts[rel] = t
    if t is None: continue
    if BANNED.search(t): fail("%s: states the contradictory precedence ('the constitution wins ...'); the one statement is: this project's CAT is an %s" % (rel, PREC))
    if PREC not in t: fail("%s: does not carry the single ODG-11 precedence statement: ... %s" % (rel, PREC))
if not BANNED.search("x the constitution wins until y"): fail("precedence control needle not seen (instrument blind)")
# the owed items: the owner answered (batch 3) YES to the lowercase rename; it must be tracked, never silently dropped (I3a, I3b)
rl = texts.get("decisions/owner-request-list.md") or ""
for need, why in (("OWED", "an OWED marker for the lower-case rename"), ("cat_id", "the cat_id rename"), ("option B", "the engine-compatibility caveat (option B views, not yet accepted by the owner)"),
                  ("retained occurrences", "the retained-occurrences class list put to the owner"), ("docs/upstream/constitution-prefix-change-request.md", "the upstream prefix wording draft"),
                  ("docs/upstream/continuum-decision-brief.md", "the continuum decision brief")):
    if need not in rl: fail("owner-request-list.md lacks %s (%r)" % (why, need))
d4 = texts.get("docs/04-findings-register-design.md") or ""
if "OWED: rename of the lower-case identifiers" not in d4: fail("docs/04 does not track the answered lower-case rename as OWED ('OWED: rename of the lower-case identifiers')")
_r4 = [f for f in fails if "precedence" in f or "OWED" in f or "owner-request-list.md lacks" in f or "single ODG-11" in f or "contradictory" in f]
if not _r4: ok("round 4: one ODG-11 precedence statement in %d files; the answered lower-case rename, the upstream drafts and the retained-occurrence question are tracked" % len(prec_files))

print("RESULT %s (%d failures)" % ("FAIL" if fails else "PASS", len(fails)))
sys.exit(1 if fails else 0)
PY
