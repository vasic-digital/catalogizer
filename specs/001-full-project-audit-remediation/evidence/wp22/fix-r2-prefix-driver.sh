#!/usr/bin/env bash
# sync_trackers.sh - T189 (docs/04 section 10.3): the one-way sync driver, register to external trackers. Host control plane like export.sh.
# Usage: sync_trackers.sh [--config .helix/reporting.yaml] [--db docs/<name>.db] [--dry-run] [--json <repo-relative file>] [--tracker <name>]...
#                         [--limit N] [--max-attempts N] [--backoff-base S] [--adapter-timeout S] [--batch-size N] [--skip-batch-size N] [--breaker N]
#        sync_trackers.sh -h
# What it does (docs/04 10.3, tasks.md T189/T191/T574a):
#  1. reads the consumer config (.helix/reporting.yaml: DATA, variable NAMES only, never values) and the register database READ-ONLY (python sqlite3, mode=ro);
#  2. computes one state per tracker, in this order: enabled:false -> SKIPPED disabled_by_operator; no `command` key (a configured name without an adapter, or a
#     candidate_trackers entry) -> SKIPPED not_configured; an empty command, or an adapter whose executable (or script) does not exist -> SKIPPED
#     tracker_client_absent; a required_env name that is unset or EMPTY -> SKIPPED credentials_absent with those NAMES in missing_env_names (the values are never
#     read into any output, row, journal or log, 11.4.10); otherwise the adapter runs;
#  3. selects, per tracker, the items whose latest reg_tracker_sync_log row is absent, SKIPPED, FAILED, or of a different item_revision (a SYNCED row of the same
#     revision is left alone). A tracker-level SKIPPED that repeats the latest SKIPPED row (same reason, same names, same revision) writes no new row;
#  4. runs the adapter (consumer-owned command; placeholders {db} {id}; item JSON on stdin, NO secrets in argv; env = PATH/HOME/LANG/TMPDIR plus the declared
#     required_env and env_passthrough names only): one in-flight request per tracker, bounded attempts (--max-attempts, default 3) with exponential backoff
#     (--backoff-base seconds, default 1), a circuit breaker (--breaker consecutive persistent failures, default 3: the rest of that tracker's items are recorded
#     SKIPPED unreachable without a call). Adapter output = ONE JSON line {status SYNCED|SKIPPED|FAILED, skip_reason?, missing_env?, exit_code?, remote_ref?,
#     evidence_path?}. A SYNCED claim needs process exit 0, a remote_ref and an existing non-empty evidence file inside the repository; anything else is recorded
#     FAILED (the database CHECKs would refuse it anyway). The driver, never the adapter, writes the log;
#  5. SINGLE WRITER (round-23 review I6): the driver never opens the database for writing. Every write is ONE `scripts/register/locked.sh` call per batch of rows
#     (`sqlite3 -bail <db> ".read <batch.sql>"`, PRAGMA foreign_keys=ON first, one transaction), so each batch is a journal row of .audit/register/journal.jsonl;
#  6. mints one register item per SKIPPED/FAILED/unreachable tracker and reason through the reg_ids creation path under locked.sh (11.4.214: the key
#     `sync_trackers:<tracker>:<reason>` is stored in reg_ids.minted_by and looked up first, so one tracker and reason is never minted twice; an existing item that is
#     already terminal is reported as owed_reopen, not minted again). disabled_by_operator is a recorded operator decision and mints nothing (UNCONFIRMED reading
#     of "SKIPPED tracker", tasks.md T191);
#  7. prints per-tracker counts and the v_stale_tracker_sync summary; --json writes them (with an identity block) to a repository-relative file.
# --dry-run: no adapter call, no database write, no lock, no file; the plan is printed (and written to --json when given).
# Exit: 0 ok (SKIPPED rows are honest results, not errors); 1 at least one item ended FAILED; 2 usage; 3 database missing/unreadable/not a register;
#       4 a register write failed (the unwritten rows are in .audit/sync/<run>/unwritten.jsonl, nothing further is attempted); 20 REFUSED reason=<code>
#       (config_missing, config_malformed, config_secret_in_command, config_id_prefix_unsupported, sync_already_running, locked_missing, path_invalid).
# Test hooks (refused unless LOCKED_TEST_MODE=1): LOCKED (stand-in wrapper), LOCKED_ROOT (scratch repository root).
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then LOCKED="${LOCKED:-$SELF_DIR/locked.sh}"; ROOT="${LOCKED_ROOT:-$REAL_ROOT}"   # MUT:hook-gate
else unset LOCKED; LOCKED="$SELF_DIR/locked.sh"; ROOT="$REAL_ROOT"; fi
export SYNC_ROOT="$ROOT" SYNC_LOCKED="$LOCKED" SYNC_SELF="$SELF_DIR/sync_trackers.sh"
exec python3 -I - "$@" <<'PY'
import sys, os, re, json, time, shlex, shutil, hashlib, fcntl, sqlite3, subprocess, datetime, urllib.parse
try:
    import yaml
except Exception:
    yaml = None

ROOT = os.environ["SYNC_ROOT"]; LOCKED = os.environ["SYNC_LOCKED"]; SELF = os.environ["SYNC_SELF"]
WIIN = "/src/submodules/constitution/scripts/workable-items/bin/workable-items-linux"
NAME_RE = re.compile(r"^[a-z][a-z0-9_]{0,31}$")
ENV_RE = re.compile(r"^[A-Z_][A-Z0-9_]{0,63}$")
ATM_RE = re.compile(r"^[A-Z]{2,5}-[0-9]{3,}$")
REF_RE = re.compile(r"^[\x21-\x7e]{1,512}$")
SKIP_REASONS = ("credentials_absent", "tracker_client_absent", "unreachable", "not_configured", "disabled_by_operator")
ADAPTER_SKIPS = ("credentials_absent", "tracker_client_absent", "unreachable")
TOP_KEYS = {"schema_version", "db", "id_prefix", "default_severity", "default_reported_by", "evidence_dir", "sync_command", "workable_items_bin", "trackers", "candidate_trackers"}   # the first keys are the shared report_item.sh contract
TRK_KEYS = {"name", "kind", "command", "required_env", "env_passthrough", "enabled"}
USAGE = ("usage: sync_trackers.sh [--config <file>] [--db docs/<name>.db] [--dry-run] [--json <file>] [--tracker <name>]... [--limit N] "
         "[--max-attempts N] [--backoff-base S] [--adapter-timeout S] [--batch-size N] [--skip-batch-size N] [--breaker N]")

def now(): return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
def usage(msg=""):
    if msg: print("sync_trackers: " + msg, file=sys.stderr)
    print(USAGE, file=sys.stderr); sys.exit(2)
def refuse(reason, detail="", code=20):
    print("sync_trackers: REFUSED reason=%s %s" % (reason, detail), file=sys.stderr); sys.exit(code)
def sha_file(p):
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""): h.update(b)
    return h.hexdigest()
def q(v):  # SQL literal: only validated strings and integers reach here; NUL is refused anyway
    if v is None: return "NULL"
    if isinstance(v, int): return str(int(v))
    s = str(v)
    if "\x00" in s: raise ValueError("NUL in SQL value")
    return "'" + s.replace("'", "''") + "'"

# ---------------------------------------------------------------- arguments
A = {"config": ".helix/reporting.yaml", "db": None, "dry": False, "json": None, "trackers": [], "limit": None, "attempts": 3, "backoff": 1.0,
     "timeout": 120.0, "batch": 10, "skipbatch": 200, "breaker": 3}
argv = sys.argv[1:]; i = 0
def need(n):
    if i + 1 >= len(argv): usage(n + " needs a value")
    return argv[i + 1]
def pint(n, v, lo, hi):
    if not re.fullmatch(r"[0-9]{1,6}", v) or not (lo <= int(v) <= hi): usage("%s must be an integer %d..%d" % (n, lo, hi))
    return int(v)
def pnum(n, v, lo, hi):
    if not re.fullmatch(r"[0-9]{1,5}(\.[0-9]{1,3})?", v) or not (lo <= float(v) <= hi): usage("%s must be a number %s..%s" % (n, lo, hi))
    return float(v)
while i < len(argv):
    a = argv[i]
    if a in ("-h", "--help"): usage()
    elif a == "--dry-run": A["dry"] = True; i += 1; continue
    elif a == "--config": A["config"] = need(a)
    elif a == "--db": A["db"] = need(a)
    elif a == "--json": A["json"] = need(a)
    elif a == "--tracker": A["trackers"].append(need(a))
    elif a == "--limit": A["limit"] = pint(a, need(a), 1, 1000000)
    elif a == "--max-attempts": A["attempts"] = pint(a, need(a), 1, 10)
    elif a == "--backoff-base": A["backoff"] = pnum(a, need(a), 0, 60)
    elif a == "--adapter-timeout": A["timeout"] = pnum(a, need(a), 1, 3600)
    elif a == "--batch-size": A["batch"] = pint(a, need(a), 1, 1000)
    elif a == "--skip-batch-size": A["skipbatch"] = pint(a, need(a), 1, 2000)
    elif a == "--breaker": A["breaker"] = pint(a, need(a), 1, 1000)
    else: usage("unknown argument '%s'" % a)
    i += 2
def relpath(p, what):
    if not p or not re.fullmatch(r"[A-Za-z0-9_./-]+", p) or p.startswith("/") or ".." in p.split("/") or p.startswith("-"):
        refuse("path_invalid", "%s: %r" % (what, p))
    return p
relpath(A["config"], "--config")
if A["json"] is not None: relpath(A["json"], "--json")
for t in A["trackers"]:
    if not NAME_RE.match(t): usage("--tracker '%s' is not a tracker name" % t)

# ---------------------------------------------------------------- config
def load_config():
    full = os.path.join(ROOT, A["config"])
    if not os.path.isfile(full): refuse("config_missing", A["config"])
    if yaml is None: refuse("config_malformed", "PyYAML is not available to python3")
    try:
        with open(full, encoding="utf-8") as f: cfg = yaml.safe_load(f)
    except (yaml.YAMLError, OSError, UnicodeDecodeError):
        refuse("config_malformed", "%s is not parseable YAML" % A["config"])
    def bad(why): refuse("config_malformed", "%s: %s" % (A["config"], why))
    if not isinstance(cfg, dict): bad("top level is not a mapping")
    extra = set(cfg) - TOP_KEYS
    if extra: bad("unknown top-level key(s): " + ", ".join(sorted(map(str, extra))))
    if cfg.get("schema_version") != 1 or isinstance(cfg.get("schema_version"), bool): bad("schema_version must be 1")
    if not isinstance(cfg.get("db"), str) or not re.fullmatch(r"docs/[A-Za-z0-9_.-]+\.db", cfg["db"]): bad("db must be docs/<name>.db")
    if not isinstance(cfg.get("id_prefix"), str) or not re.fullmatch(r"[A-Z]{2,5}", cfg["id_prefix"]): bad("id_prefix must be 2-5 capital letters")
    if not isinstance(cfg.get("trackers"), list): bad("trackers must be a list (use [] for none)")
    ev = cfg.get("evidence_dir", ".audit/sync/evidence")
    if not isinstance(ev, str): bad("evidence_dir must be a string")
    relpath(ev, "evidence_dir")
    seen = set(); trackers = []
    for n, t in enumerate(cfg["trackers"]):
        if not isinstance(t, dict): bad("trackers[%d] is not a mapping" % n)
        ex = set(t) - TRK_KEYS
        if ex: bad("trackers[%d]: unknown key(s) %s (a mistyped required_env would silently disable the credentials check)" % (n, ", ".join(sorted(map(str, ex)))))
        nm = t.get("name")
        if not isinstance(nm, str) or not NAME_RE.match(nm): bad("trackers[%d].name must match %s" % (n, NAME_RE.pattern))
        if nm in seen: bad("duplicate tracker name '%s'" % nm)
        seen.add(nm)
        en = t.get("enabled", True)
        if not isinstance(en, bool): bad("trackers[%d].enabled must be true or false" % n)
        lists = {}
        v = t.get("required_env", [])
        if not isinstance(v, list) or any(not isinstance(x, str) or not ENV_RE.match(x) for x in v): bad("trackers[%d].required_env must be a list of variable NAMES" % n)
        lists["required_env"] = sorted(set(v))
        v = t.get("env_passthrough", {})
        # shared contract with report_item.sh: a mapping NAME -> value template ({db}/{id}); a plain list of NAMES (taken from the environment) is accepted too
        if isinstance(v, list):
            if any(not isinstance(x, str) or not ENV_RE.match(x) for x in v): bad("trackers[%d].env_passthrough list entries must be variable NAMES" % n)
            lists["env_passthrough"] = {x: None for x in sorted(set(v))}
        elif isinstance(v, dict):
            if any(not isinstance(k, str) or not ENV_RE.match(k) or not isinstance(x, str) or re.search(r"\{(?!db\}|id\})[^}]*\}", x) for k, x in v.items()):
                bad("trackers[%d].env_passthrough must map variable NAMES to string templates using only {db} and {id}" % n)
            lists["env_passthrough"] = dict(sorted(v.items()))
        else: bad("trackers[%d].env_passthrough must be a mapping or a list of NAMES" % n)
        cmd = t.get("command", None); has_cmd = "command" in t
        if has_cmd and not isinstance(cmd, str): bad("trackers[%d].command must be a string" % n)
        toks = []
        if has_cmd and cmd.strip():
            try: toks = shlex.split(cmd)
            except ValueError: bad("trackers[%d].command is not splittable" % n)
            for tk in toks:
                if re.search(r"\{(?!db\}|id\})[^}]*\}", tk): bad("trackers[%d].command: only the placeholders {db} and {id} exist" % n)
                m = re.match(r"^-{0,2}[A-Za-z_]*(token|secret|password|passwd|apikey|api_key)[A-Za-z_]*=(.+)$", tk, re.I)
                if m and not m.group(2).startswith(("$", "{")) and "file" not in tk.split("=")[0].lower():
                    refuse("config_secret_in_command", "trackers[%d].command carries a value after '=' for a secret-looking key; names only, the value belongs in the environment" % n)
        kind = t.get("kind", nm)
        if not isinstance(kind, str) or not re.fullmatch(r"[A-Za-z0-9_.-]{1,48}", kind): bad("trackers[%d].kind is not a plain word" % n)
        trackers.append({"name": nm, "kind": kind, "has_cmd": has_cmd, "cmd": cmd if has_cmd else None, "argv": toks, "required_env": lists["required_env"],
                         "env_passthrough": lists["env_passthrough"], "enabled": en, "candidate": False})
    cands = cfg.get("candidate_trackers", [])
    if not isinstance(cands, list): bad("candidate_trackers must be a list")
    for n, c in enumerate(cands):
        if isinstance(c, str): c = {"name": c}
        if not isinstance(c, dict) or set(c) - {"name", "kind"}: bad("candidate_trackers[%d] must be a name or {name, kind}" % n)
        nm = c.get("name")
        if not isinstance(nm, str) or not NAME_RE.match(nm): bad("candidate_trackers[%d].name must match %s" % (n, NAME_RE.pattern))
        if nm in seen: continue   # a configured tracker wins over the candidate of the same name
        seen.add(nm)
        trackers.append({"name": nm, "kind": c.get("kind", nm) if isinstance(c.get("kind", nm), str) else nm, "has_cmd": False, "cmd": None, "argv": [], "required_env": [],
                         "env_passthrough": {}, "enabled": True, "candidate": True})
    return cfg, trackers, ev

CFG, TRACKERS, EVDIR = load_config()
DB = A["db"] or CFG["db"]
if not re.fullmatch(r"docs/[A-Za-z0-9_.-]+\.db", DB): refuse("path_invalid", "--db must be docs/<name>.db")
CFG_SHA = sha_file(os.path.join(ROOT, A["config"]))
for t in A["trackers"]:
    if t not in {x["name"] for x in TRACKERS}: usage("--tracker '%s' is neither configured nor a candidate" % t)
if A["trackers"]: TRACKERS = [t for t in TRACKERS if t["name"] in A["trackers"]]

# ---------------------------------------------------------------- database (read-only)
DBFILE = os.path.join(ROOT, DB)
def ro():
    return sqlite3.connect("file:" + urllib.parse.quote(DBFILE) + "?mode=ro", uri=True, timeout=30)
def read(fn):
    """open read-only, run fn(conn), close at once (a reader left open would block the checkpoint of a backup)"""
    try:
        c = ro()
    except sqlite3.Error as e:
        print("sync_trackers: database %s cannot be opened read-only: %s" % (DB, e), file=sys.stderr); sys.exit(3)
    try:
        return fn(c)
    except sqlite3.Error as e:
        print("sync_trackers: database %s is not readable as a register: %s" % (DB, e), file=sys.stderr); sys.exit(3)
    finally:
        c.close()
if not os.path.isfile(DBFILE):
    print("sync_trackers: database %s not found" % DB, file=sys.stderr); sys.exit(3)
def probe(c):
    names = {r[0] for r in c.execute("select name from sqlite_master where type in ('table','view')")}
    miss = [n for n in ("reg_meta", "reg_ids", "reg_trackers", "reg_tracker_sync_log", "reg_evidence", "reg_item_ext", "items", "v_stale_tracker_sync") if n not in names]
    return miss, c.execute("select sql from sqlite_master where name='reg_ids'").fetchone()
miss, ids_sql = read(probe)
if miss:
    print("sync_trackers: %s is not a register database (missing %s)" % (DB, ", ".join(miss)), file=sys.stderr); sys.exit(3)
if "'%s-'" % CFG["id_prefix"] not in (ids_sql[0] if ids_sql else ""):
    refuse("config_id_prefix_unsupported", "reg_ids generates ids with another prefix than '%s-' (docs/04 DDL); the config id_prefix must match" % CFG["id_prefix"])
DB_SHA0 = sha_file(DBFILE)

RUN_ID = "sync-%s-%d" % (datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%S"), os.getpid())
RUN_DIR = os.path.join(ROOT, ".audit", "sync", RUN_ID)
EV_REL = EVDIR.rstrip("/") + "/tracker-sync/" + RUN_ID

# ---------------------------------------------------------------- single-driver lock (not taken by --dry-run: it writes nothing)
LOCKFD = None
if not A["dry"]:
    os.makedirs(os.path.join(ROOT, ".audit", "sync"), exist_ok=True)
    LOCKFD = open(os.path.join(ROOT, ".audit", "sync", "driver.lock"), "a")
    try: fcntl.flock(LOCKFD, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError: refuse("sync_already_running", "another sync_trackers.sh holds .audit/sync/driver.lock")
    if not (os.path.isfile(LOCKED) and os.access(LOCKED, os.X_OK)): refuse("locked_missing", "scripts/register/locked.sh is not executable")

# ---------------------------------------------------------------- writes: ONLY through locked.sh
NCALL = [0]; LOCKED_CALLS = []
def run_locked(label, cmd_after):
    NCALL[0] += 1
    opid = "%s-%s-%d" % (RUN_ID, label, NCALL[0])
    r = subprocess.run([LOCKED, "--op-id", opid, "--"] + cmd_after, stdin=subprocess.DEVNULL, capture_output=True, text=True)
    LOCKED_CALLS.append({"op_id": opid, "label": label, "exit": r.returncode})
    return r.returncode, r.stdout, r.stderr
def write_batch(label, sql):
    os.makedirs(RUN_DIR, exist_ok=True)
    f = os.path.join(RUN_DIR, "batch-%03d-%s.sql" % (NCALL[0] + 1, label))
    with open(f, "w", encoding="utf-8") as fh: fh.write(sql)
    rel = os.path.relpath(f, ROOT)
    return run_locked(label, ["sqlite3", "-bail", "/src/" + DB, ".read /src/" + rel])

# ---------------------------------------------------------------- tracker state
def client_absent(t):
    av = t["argv"]
    if not av: return True
    exe = av[0]
    if "/" in exe:
        p = exe if os.path.isabs(exe) else os.path.join(ROOT, exe)
        if not (os.path.isfile(p) and os.access(p, os.X_OK)): return True
    elif shutil.which(exe) is None: return True
    if os.path.basename(exe) in ("bash", "sh", "python3", "python"):
        for a in av[1:]:
            if a.startswith("-"): continue
            if "/" in a or a.endswith((".sh", ".py")):
                p = a if os.path.isabs(a) else os.path.join(ROOT, a)
                if not os.path.isfile(p): return True
            break
    return False
def tracker_state(t):
    """(skip_reason, missing_env) when the tracker cannot run an adapter at all, else None"""
    if not t["enabled"]: return ("disabled_by_operator", [])
    if not t["has_cmd"]: return ("not_configured", [])
    if not (t["cmd"] or "").strip() or client_absent(t): return ("tracker_client_absent", [])
    miss = sorted(n for n in t["required_env"] if not os.environ.get(n))
    if miss: return ("credentials_absent", miss)
    return None
for t in TRACKERS: t["state"] = tracker_state(t)

def revision(it):
    return hashlib.sha256(json.dumps({"atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "status": it["status"], "type": it["type"],
                                      "severity": it["severity"]}, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode("utf-8")).hexdigest()
def load_items(c):
    rows = c.execute("select i.atm_id,i.title,i.description,i.status,i.type,coalesce(x.severity,i.severity,''),i.current_location,i.representation "
                     "from items i left join reg_item_ext x on x.atm_id=i.atm_id order by i.atm_id").fetchall()
    best = {}
    for r in rows:
        rank = (0 if r[6] == "Issues" else 1, 0 if r[7] == "section" else 1)
        if r[0] not in best or rank < best[r[0]][0]:
            best[r[0]] = (rank, {"atm_id": r[0], "title": r[1], "body": r[2], "status": r[3], "type": r[4], "severity": r[5]})
    out = []
    for k in sorted(best):
        it = best[k][1]; it["rev"] = revision(it); out.append(it)
    return out
def load_log(c):
    latest = {}; refs = {}
    for r in c.execute("select tracker_id,atm_id,status,skip_reason,missing_env_names,item_revision,remote_ref,sync_id from reg_tracker_sync_log order by sync_id"):
        latest[(r[0], r[1])] = {"status": r[2], "reason": r[3], "missing": r[4], "rev": r[5], "sync_id": r[7]}
        if r[6]: refs[(r[0], r[1])] = r[6]
    return latest, refs
def load_trackers_db(c):
    return {r[0]: r[1:] for r in c.execute("select tracker_id,kind,command_ref,required_env,enabled from reg_trackers")}

# ---------------------------------------------------------------- counters
COUNT = {t["name"]: {"synced": 0, "skipped": {}, "failed": 0, "unchanged": 0, "unchanged_skipped": 0, "breaker_skipped": 0, "selected": 0} for t in TRACKERS}
FAILS = {t["name"]: 0 for t in TRACKERS}; DONE = {t["name"]: 0 for t in TRACKERS}   # DONE: --limit is per tracker for the whole run
PENDING = []; PEND_CALLS = [0]; UNWRITTEN = []; WRITE_FAILED = [False]
OBSERVED = {}   # tracker -> set of reasons seen this run (mint keys)
def bump_skip(name, reason): COUNT[name]["skipped"][reason] = COUNT[name]["skipped"].get(reason, 0) + 1
def observe(name, reason): OBSERVED.setdefault(name, set()).add(reason)

def flush(force=False):
    if not PENDING: return True
    if not force and PEND_CALLS[0] < A["batch"] and len(PENDING) < A["skipbatch"]: return True
    rows = list(PENDING); PENDING.clear(); PEND_CALLS[0] = 0
    sql = ["PRAGMA foreign_keys=ON;", "BEGIN IMMEDIATE;"]
    for r in rows:
        ev = r.get("evidence")
        if ev:
            sql.append("INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES (%s,'tracker_receipt','artifact',%s,%s,%d,'sync_trackers.sh',%s);"
                       % (q(r["atm_id"]), q(ev["path"]), q(ev["sha256"]), ev["size"], q(r["attempted_at"])))
        sql.append("INSERT INTO reg_tracker_sync_log(tracker_id,atm_id,status,skip_reason,missing_env_names,exit_code,remote_ref,evidence_id,item_revision,attempted_at) "
                   "VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);" % (q(r["tracker_id"]), q(r["atm_id"]), q(r["status"]), q(r.get("skip_reason")),
                   q(json.dumps(r["missing"]) if r.get("missing") else None), q(r.get("exit_code")), q(r.get("remote_ref")),
                   "last_insert_rowid()" if ev else "NULL", q(r["rev"]), q(r["attempted_at"])))
    sql.append("COMMIT;")
    rc, so, se = write_batch("log", "\n".join(sql) + "\n")
    if rc != 0:
        os.makedirs(RUN_DIR, exist_ok=True)
        with open(os.path.join(RUN_DIR, "unwritten.jsonl"), "a", encoding="utf-8") as fh:
            for r in rows: fh.write(json.dumps({k: v for k, v in r.items()}, sort_keys=True) + "\n")
        UNWRITTEN.extend(rows); WRITE_FAILED[0] = True
        print("sync_trackers: register write failed (locked.sh exit %d); %d row(s) NOT recorded, listed in %s" % (rc, len(rows), os.path.relpath(os.path.join(RUN_DIR, "unwritten.jsonl"), ROOT)), file=sys.stderr)
        for l in (se or "").strip().splitlines()[-3:]: print("sync_trackers: locked: " + l, file=sys.stderr)
        return False
    return True
def add_row(row, call=False):
    PENDING.append(row)
    if call: PEND_CALLS[0] += 1
    return flush()

# ---------------------------------------------------------------- adapter
def adapter_once(t, it, ref):
    av = [x.replace("{db}", DB).replace("{id}", it["atm_id"]) for x in t["argv"]]
    env = {k: os.environ[k] for k in ("PATH", "HOME", "LANG", "TMPDIR") if k in os.environ}
    for n in t["required_env"]:
        if os.environ.get(n): env[n] = os.environ[n]
    for n, tpl in t["env_passthrough"].items():
        if tpl is None:
            if os.environ.get(n): env[n] = os.environ[n]
        else: env[n] = tpl.replace("{db}", DB).replace("{id}", it["atm_id"])
    payload = {"db": DB, "tracker": t["name"], "atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "status": it["status"], "type": it["type"],
               "severity": it["severity"], "evidence_dir": EV_REL}
    if ref: payload["remote_ref"] = ref
    os.makedirs(os.path.join(ROOT, EV_REL), exist_ok=True)
    try:
        p = subprocess.run(av, input=json.dumps(payload), capture_output=True, text=True, timeout=A["timeout"], cwd=ROOT, env=env)
        rc, so = p.returncode, p.stdout
    except subprocess.TimeoutExpired:
        return {"kind": "failed", "exit": 124, "why": "timeout", "ref": None}
    except OSError:
        return {"kind": "failed", "exit": 127, "why": "launch_failed", "ref": None}
    lines = [l for l in (so or "").splitlines() if l.strip()]
    try:
        o = json.loads(lines[-1]) if lines else None
    except ValueError:
        o = None
    if not isinstance(o, dict) or o.get("status") not in ("SYNCED", "SKIPPED", "FAILED"):
        return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "bad_output", "ref": None}
    ref_out = o.get("remote_ref") if isinstance(o.get("remote_ref"), str) and REF_RE.match(o["remote_ref"]) else None
    if o["status"] == "SKIPPED":
        sr = o.get("skip_reason")
        if sr not in ADAPTER_SKIPS: return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "bad_skip_reason", "ref": ref_out}
        miss = o.get("missing_env", [])
        if not isinstance(miss, list) or any(not isinstance(x, str) or not ENV_RE.match(x) for x in miss) or (sr == "credentials_absent" and not miss):
            return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "bad_missing_env", "ref": ref_out}
        return {"kind": "skipped", "reason": sr, "missing": sorted(set(miss)), "exit": rc if rc >= 0 else None, "ref": ref_out}
    if o["status"] == "FAILED" or rc != 0:
        return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "adapter_failed", "ref": ref_out}
    ep = o.get("evidence_path")
    ok = False
    if ref_out and isinstance(ep, str) and ep and "\x00" not in ep and not os.path.isabs(ep):
        full = os.path.realpath(os.path.join(ROOT, ep))
        if full.startswith(os.path.realpath(ROOT) + os.sep) and os.path.isfile(full) and os.path.getsize(full) > 0:
            ok = True; ev = {"path": os.path.relpath(full, os.path.realpath(ROOT)), "sha256": sha_file(full), "size": os.path.getsize(full)}
    if not ok: return {"kind": "failed", "exit": 65, "why": "synced_without_ref_or_evidence", "ref": ref_out}
    return {"kind": "synced", "ref": ref_out, "evidence": ev}
def adapter_call(t, it, ref):
    last = None
    for attempt in range(1, A["attempts"] + 1):
        last = adapter_once(t, it, ref); last["attempts"] = attempt
        if last["kind"] in ("synced",): break
        if last["kind"] == "skipped" and last["reason"] != "unreachable": break
        if attempt < A["attempts"]: time.sleep(min(30.0, A["backoff"] * (2 ** (attempt - 1))))
    return last

# ---------------------------------------------------------------- the pass over items
def process(items, latest, refs):
    for t in TRACKERS:
        name = t["name"]; c = COUNT[name]; st = t["state"]
        for it in items:
            last = latest.get((name, it["atm_id"]))
            if last and last["status"] == "SYNCED" and last["rev"] == it["rev"]:
                c["unchanged"] += 1; continue
            if A["limit"] is not None and DONE[name] >= A["limit"]: continue
            c["selected"] += 1; DONE[name] += 1
            base = {"tracker_id": name, "atm_id": it["atm_id"], "rev": it["rev"], "attempted_at": now()}
            if st is not None:
                reason, miss = st; observe_it = reason != "disabled_by_operator"
                if last and last["status"] == "SKIPPED" and last["reason"] == reason and last["rev"] == it["rev"] and (json.loads(last["missing"]) if last["missing"] else []) == miss:
                    c["unchanged_skipped"] += 1
                else:
                    bump_skip(name, reason)
                    if not A["dry"] and not add_row(dict(base, status="SKIPPED", skip_reason=reason, missing=miss)): return False
                if observe_it: observe(name, reason)
                continue
            if FAILS[name] >= A["breaker"]:
                c["breaker_skipped"] += 1; bump_skip(name, "unreachable"); observe(name, "unreachable")
                if not A["dry"] and not add_row(dict(base, status="SKIPPED", skip_reason="unreachable")): return False
                continue
            if A["dry"]: continue
            res = adapter_call(t, it, refs.get((name, it["atm_id"])))
            if res["kind"] == "synced":
                FAILS[name] = 0; c["synced"] += 1; refs[(name, it["atm_id"])] = res["ref"]
                ok = add_row(dict(base, status="SYNCED", exit_code=0, remote_ref=res["ref"], evidence=res["evidence"]), call=True)
            elif res["kind"] == "skipped":
                if res["reason"] == "unreachable": FAILS[name] += 1
                bump_skip(name, res["reason"]); observe(name, res["reason"])
                ok = add_row(dict(base, status="SKIPPED", skip_reason=res["reason"], missing=res["missing"], exit_code=res["exit"], remote_ref=res["ref"]), call=True)
            else:
                FAILS[name] += 1; c["failed"] += 1; observe(name, "failed")
                ok = add_row(dict(base, status="FAILED", exit_code=res["exit"], remote_ref=res["ref"]), call=True)
            if not ok: return False
        # rows of one tracker are flushed at the tracker boundary: the next tracker's summary and a crash never see a half-written tracker
        if not A["dry"] and not flush(force=True): return False
    return True

# ---------------------------------------------------------------- minting (11.4.214: one item per tracker and reason, looked up first)
MINT_SEV = {"not_configured": "Low", "tracker_client_absent": "Medium", "credentials_absent": "Medium", "unreachable": "Medium", "failed": "Medium"}
def mint_key(tr, reason): return "sync_trackers:%s:%s" % (tr, reason)
def existing_mint(tr, reason):
    def f(c):
        r = c.execute("select r.atm_id,(select status from items i where i.atm_id=r.atm_id order by i.current_location limit 1) from reg_ids r where r.minted_by=? order by r.seq limit 1", (mint_key(tr, reason),)).fetchone()
        return r
    return read(f)
def mint_script(tr, reason):
    key = mint_key(tr, reason); db = "/src/" + DB
    title = "External tracker %s not synced: %s" % (tr, reason)
    desc = ("The register sync driver (scripts/register/sync_trackers.sh) recorded tracker %s as %s, so no register item reached it. Resolve the cause "
            "(docs/scripts/sync_trackers.md lists the closed reasons) or record the owner decision. Minted once per tracker and reason (11.4.214)." % (tr, reason))
    for s in (key, title, desc, MINT_SEV[reason], db):
        assert re.fullmatch(r"[A-Za-z0-9 _:./,()'-]+", s) and "'" not in s, s
    return ("set -u; db=%s; key='%s'\n"
            "id=$(sqlite3 \"$db\" \"SELECT atm_id FROM reg_ids WHERE minted_by='$key' ORDER BY seq LIMIT 1\") || exit 3\n"
            "if [ -z \"$id\" ]; then id=$(sqlite3 \"$db\" \"PRAGMA foreign_keys=ON; INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('$key','manual'); SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1;\") || exit 3; fi\n"
            "if [ -z \"$(sqlite3 \"$db\" \"SELECT 1 FROM items WHERE atm_id='$id' LIMIT 1\")\" ]; then %s add Task %s --db \"$db\" --id \"$id\" --prefix %s --title '%s' --description '%s' >/dev/null || exit 4; fi\n"
            "if [ -z \"$(sqlite3 \"$db\" \"SELECT 1 FROM reg_item_ext WHERE atm_id='$id' LIMIT 1\")\" ]; then sqlite3 \"$db\" \"PRAGMA foreign_keys=ON; INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$id','gap','artifact','%s');\" || exit 5; fi\n"
            "echo \"$id\"\n") % (db, key, WIIN, MINT_SEV[reason], CFG["id_prefix"], title, desc, MINT_SEV[reason].lower())
MINTED = []; OWED_REOPEN = []
def mint_round():
    new = 0
    for name in sorted(OBSERVED):
        for reason in sorted(OBSERVED[name]):
            if reason not in MINT_SEV: continue
            ex = existing_mint(name, reason)
            if ex and ex[1] is not None:
                if "(→ Fixed.md)" in ex[1]: OWED_REOPEN.append({"tracker": name, "reason": reason, "atm_id": ex[0], "status": ex[1]})
                if not any(m["tracker"] == name and m["reason"] == reason for m in MINTED):
                    MINTED.append({"tracker": name, "reason": reason, "atm_id": ex[0], "action": "existing"})
                continue
            if A["dry"]:
                MINTED.append({"tracker": name, "reason": reason, "atm_id": None, "action": "would_mint"}); continue
            rc, so, se = run_locked("mint", ["bash", "-c", mint_script(name, reason)])
            if rc != 0:
                print("sync_trackers: minting the item for %s/%s failed (locked.sh exit %d)" % (name, reason, rc), file=sys.stderr); WRITE_FAILED[0] = True; return new, False
            got = existing_mint(name, reason)
            MINTED.append({"tracker": name, "reason": reason, "atm_id": got[0] if got else None, "action": "minted"}); new += 1
    return new, True

# ---------------------------------------------------------------- main
def tracker_row_upserts():
    dbt = read(load_trackers_db); stmts = []
    for t in TRACKERS:
        want = (t["kind"], t["cmd"] if t["has_cmd"] else None, json.dumps(t["required_env"]), 1)
        have = dbt.get(t["name"])
        if have is None or tuple(have) != want:
            stmts.append("INSERT INTO reg_trackers(tracker_id,kind,command_ref,required_env,enabled) VALUES (%s,%s,%s,%s,1) ON CONFLICT(tracker_id) DO UPDATE SET kind=excluded.kind,command_ref=excluded.command_ref,required_env=excluded.required_env,enabled=excluded.enabled;"
                         % (q(t["name"]), q(t["kind"]), q(want[1]), q(want[2])))
    return stmts

exit_code = 0
if not A["dry"]:
    stm = tracker_row_upserts()
    if stm:
        rc, so, se = write_batch("trackers", "PRAGMA foreign_keys=ON;\nBEGIN IMMEDIATE;\n" + "\n".join(stm) + "\nCOMMIT;\n")
        if rc != 0:
            print("sync_trackers: registering the trackers failed (locked.sh exit %d)" % rc, file=sys.stderr); sys.exit(4)
# tracker-level reasons are known before any item is touched: mint their items first so the same pass records them
for t in TRACKERS:
    if t["state"] is not None and t["state"][0] != "disabled_by_operator": observe(t["name"], t["state"][0])
ok_mint = True
if OBSERVED:
    # only trackers with at least one item to report on matter, but the tracker itself is the unit (docs/04 10.3): a register with zero items still mints
    _, ok_mint = mint_round()
if ok_mint:
    items = read(load_items); latest, refs = read(load_log)
    okp = process(items, latest, refs)
    if okp and not A["dry"]:
        _, ok_mint = mint_round()
        if ok_mint:
            fresh = [i for i in read(load_items) if i["atm_id"] not in {x["atm_id"] for x in items}]
            if fresh:
                latest, refs = read(load_log)
                okp = process(fresh, latest, refs)
    if okp and not A["dry"]: okp = flush(force=True)
else:
    okp = False
if WRITE_FAILED[0]: exit_code = 4
elif any(COUNT[n]["failed"] for n in COUNT): exit_code = 1

# ---------------------------------------------------------------- report
def stale(c):
    return [list(r) for r in c.execute("select tracker_id,coalesce(last_status,'NONE'),count(*) from v_stale_tracker_sync group by 1,2 order by 1,2")]
STALE = read(stale)
DB_SHA1 = sha_file(DBFILE)
def ident():
    def sh(p):
        try: return sha_file(p)
        except OSError: return None
    g = subprocess.run(["git", "-C", ROOT, "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip() or None
    return {"task": "T189/T191", "utc": now(), "host": os.uname().nodename.split(".")[0], "uid": os.getuid(), "git_head": g, "driver_sha256": sh(SELF),
            "locked_sha256": sh(LOCKED), "config": A["config"], "config_sha256": CFG_SHA, "run_id": RUN_ID}
rep = {"schema": "sync-trackers/1", "identity": ident(), "dry_run": A["dry"], "db": {"path": DB, "sha256_before": DB_SHA0, "sha256_after": DB_SHA1},
       "options": {"max_attempts": A["attempts"], "backoff_base": A["backoff"], "breaker": A["breaker"], "limit": A["limit"], "trackers": A["trackers"]},
       "trackers": {}, "minted": MINTED, "owed_reopen": OWED_REOPEN, "locked_calls": LOCKED_CALLS, "v_stale_tracker_sync": STALE,
       "unwritten_rows": len(UNWRITTEN), "exit": exit_code}
for t in TRACKERS:
    rep["trackers"][t["name"]] = {"kind": t["kind"], "candidate": t["candidate"], "state": (t["state"][0] if t["state"] else "adapter"),
                                  "missing_env_names": (t["state"][1] if t["state"] and t["state"][1] else []), "counts": COUNT[t["name"]]}
for t in TRACKERS:
    c = COUNT[t["name"]]; sk = ",".join("%s=%d" % kv for kv in sorted(c["skipped"].items())) or "0"
    print("sync_trackers: tracker=%s state=%s selected=%d synced=%d skipped=%s failed=%d unchanged=%d unchanged_skipped=%d breaker_skipped=%d%s"
          % (t["name"], rep["trackers"][t["name"]]["state"], c["selected"], c["synced"], sk, c["failed"], c["unchanged"], c["unchanged_skipped"], c["breaker_skipped"],
             " missing_env_names=" + ",".join(rep["trackers"][t["name"]]["missing_env_names"]) if rep["trackers"][t["name"]]["missing_env_names"] else ""))
if not TRACKERS: print("sync_trackers: 0 trackers configured; 0 pushes; nothing claimed as synced")
for m in MINTED: print("sync_trackers: item %s tracker=%s reason=%s id=%s" % (m["action"], m["tracker"], m["reason"], m["atm_id"]))
for m in OWED_REOPEN: print("sync_trackers: OWED reopen of %s (tracker=%s reason=%s is skipped again but the item is %s)" % (m["atm_id"], m["tracker"], m["reason"], m["status"]))
print("sync_trackers: v_stale_tracker_sync %s" % json.dumps(STALE))
print("sync_trackers: %s locked.sh call(s)%s" % (len(LOCKED_CALLS), " (dry run: none)" if A["dry"] else ""))
if A["json"]:
    jp = os.path.join(ROOT, A["json"]); os.makedirs(os.path.dirname(jp) or ".", exist_ok=True)
    tmp = jp + ".tmp.%d" % os.getpid()
    with open(tmp, "w", encoding="utf-8") as fh: json.dump(rep, fh, indent=1, sort_keys=True); fh.write("\n")
    os.replace(tmp, jp)
sys.exit(exit_code)
PY
