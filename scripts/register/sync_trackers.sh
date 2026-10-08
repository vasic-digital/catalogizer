#!/usr/bin/env bash
# sync_trackers.sh - T189 (docs/04 section 10.3): the one-way sync driver, register to external trackers. Host control plane like export.sh.
# Usage: sync_trackers.sh [--config .helix/reporting.yaml] [--db docs/<name>.db] [--dry-run] [--json <file under .audit/ or the evidence_dir>] [--tracker <name>]...
#                         [--limit N] [--max-attempts N] [--backoff-base S] [--adapter-timeout S] [--batch-size N] [--skip-batch-size N] [--breaker N]
#                         [--approved-by <name> --approve-first-sync <plan hash>] [--retry-indeterminate]
#        sync_trackers.sh -h
# What it does (docs/04 10.3, tasks.md T189/T191/T574a; fix round 2 of the WF23 review):
#  1. reads the consumer config (.helix/reporting.yaml: DATA, variable NAMES only, never values) and the register database READ-ONLY (python sqlite3, mode=ro).
#     A duplicate YAML key is REFUSED (the last one would silently win and could disable required_env). The adapter command is checked against a POSITIVE grammar
#     (see command_problem): no credential-named flag or key with a value, no short flag with a bare value, no Authorization/Bearer/header text, no user:password@ URL,
#     no credential-shaped token; env_passthrough is a mapping NAME -> template that MUST use {db} or {id} (a literal value could be a credential). The command text is
#     NEVER written to the register: reg_trackers.command_ref holds "<executable>#sha256:<16 hex>";
#  2. computes one state per tracker, in this order: enabled:false -> SKIPPED disabled_by_operator; no `command` key (a configured name without an adapter, or a
#     candidate_trackers entry) -> SKIPPED not_configured; an empty command, or an adapter whose executable (or interpreter script, also behind /usr/bin/env) does not
#     exist -> SKIPPED tracker_client_absent; a required_env name that is unset, EMPTY or blank -> SKIPPED credentials_absent with those NAMES in missing_env_names (the
#     values are never read into any output, row, journal or log, 11.4.10); otherwise the adapter runs;
#  3. FIRST-SYNC GATE (docs/04 10.3, 11.4.252): a tracker with an adapter and no SYNCED row yet is pushed to ONLY when the run carries --approved-by <name> (a name of
#     first_sync_approvers in the config) and --approve-first-sync <plan hash> (the hash a --dry-run printed for exactly this item set) and --limit N <= the pilot size
#     (first_sync_pilot_max, default 5); anything else is REFUSED first_sync_gate before any write. After the pilot has SYNCED rows the gate no longer applies;
#  4. selects, per tracker, the items whose latest reg_tracker_sync_log row is absent, SKIPPED, FAILED, or of a different item_revision (a SYNCED row of the same
#     revision is left alone). A tracker-level SKIPPED that repeats the latest SKIPPED row writes no new row. An item whose latest row is an indeterminate FAILED (timeout
#     124, crash 75) without a known remote reference is HELD (the remote effect is unknown): --retry-indeterminate releases it. Items of a confidential category
#     (confidential_categories, default danger_zone) on a public tracker (the default) or whose title/body match a credential pattern are WITHHELD: recorded FAILED 78/77
#     without an adapter call, the matched text is never printed;
#  5. runs the adapter (consumer-owned command; placeholders {db} {id}; item JSON on a stdin FILE, NO secrets in argv; stdout/stderr to files; env = PATH/HOME/LANG/TMPDIR
#     plus the declared required_env and env_passthrough names only) in its OWN process group; the group is killed on timeout and after the adapter returned (a stray
#     child never keeps pushing). One in-flight request per tracker, bounded attempts (--max-attempts, default 3, a timeout is not retried) with exponential backoff
#     (--backoff-base seconds), a circuit breaker (--breaker consecutive failures: after unreachable failures the rest is recorded SKIPPED unreachable; after FAILED
#     answers from a reachable tracker the rest is NOT attempted and not recorded). Adapter output = ONE JSON line {status SYNCED|SKIPPED|FAILED, skip_reason?,
#     missing_env?, exit_code?, remote_ref? (string or integer), remote_state?, evidence_path?}. A SYNCED claim needs process exit 0, a remote_ref and a non-empty
#     receipt file under THIS run's evidence directory, written during the call, that names the item id and the remote_ref; anything else is recorded FAILED;
#  6. WRITE-AHEAD (WF23 B1): .audit/sync/wal/<run>.jsonl (fsync) records an intent before every adapter call and the result right after it; a batch write adds `flushed`;
#     the next run reconciles every unclosed wal file first (results never written are written, intents without a result become FAILED 75), passing the recovered
#     remote_ref to the adapter so an update is never a second create. SIGTERM/SIGINT/SIGHUP and any exception flush what is pending (exit 6 / 5);
#  7. SINGLE WRITER: the driver never opens the database for writing. Every write is ONE `scripts/register/locked.sh` call per batch of rows
#     (`sqlite3 -bail <db> ".read <batch.sql>"`, PRAGMA foreign_keys=ON first, one transaction). locked.sh exit 21/22 are checked by reading the rows back;
#     a refusal for host memory/disk is retried a bounded number of times (never overridden);
#  8. mints one register item per SKIPPED/FAILED/unreachable tracker and reason through the reg_ids creation path under locked.sh (11.4.214: the key
#     `sync_trackers:<tracker>:<reason>` is stored in reg_ids.minted_by and looked up first). An existing minted item that is already terminal is reported as
#     owed_reopen and the run exits 7 (the reopen with its recurrence link needs a reported source entry, so it is owed to the register custody path);
#  9. reg_trackers.enabled follows the config (false for enabled:false), and a tracker the full config no longer names is retired (enabled=0);
# 10. prints per-tracker counts and the v_stale_tracker_sync summary; --json writes them (with an identity block).
# --dry-run: no adapter call, no database write, no lock, no file; the plan (and the first-sync plan hash) is printed.
# Exit: 0 ok (SKIPPED rows are honest results, not errors); 1 at least one item ended FAILED; 2 usage; 3 database missing/unreadable/not a register;
#       4 a register write failed (the unwritten rows stay in the wal and in .audit/sync/<run>/unwritten.jsonl); 5 internal error; 6 interrupted by a signal;
#       7 attention: owed reopen or held indeterminate items; 20 REFUSED reason=<code> (config_missing, config_malformed, config_secret_in_command,
#       config_secret_in_env, config_id_prefix_unsupported, first_sync_gate, sync_already_running, locked_missing, path_invalid).
# Test hooks (refused unless LOCKED_TEST_MODE=1): LOCKED (stand-in wrapper), LOCKED_ROOT (scratch repository root).
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then LOCKED="${LOCKED:-$SELF_DIR/locked.sh}"; ROOT="${LOCKED_ROOT:-$REAL_ROOT}"   # MUT:hook-gate
else unset LOCKED; LOCKED="$SELF_DIR/locked.sh"; ROOT="$REAL_ROOT"; fi
export SYNC_ROOT="$ROOT" SYNC_LOCKED="$LOCKED" SYNC_SELF="$SELF_DIR/sync_trackers.sh"
exec python3 -I - "$@" <<'PY'
import sys, os, re, json, time, shlex, shutil, hashlib, fcntl, sqlite3, subprocess, datetime, urllib.parse, signal
try:
    import yaml
except Exception:
    yaml = None

ROOT = os.environ["SYNC_ROOT"]; LOCKED = os.environ["SYNC_LOCKED"]; SELF = os.environ["SYNC_SELF"]
WIIN = "/src/submodules/constitution/scripts/workable-items/bin/workable-items-linux"
NAME_RE = re.compile(r"^[a-z][a-z0-9_]{0,31}$")
ENV_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]{0,63}$")
ATM_RE = re.compile(r"^[A-Z]{2,5}-[0-9]{3,}$")
REF_RE = re.compile(r"^[\x21-\x7e]{1,512}$")
STATE_RE = re.compile(r"^[\x20-\x7e]{1,64}$")
SKIP_REASONS = ("credentials_absent", "tracker_client_absent", "unreachable", "not_configured", "disabled_by_operator")
ADAPTER_SKIPS = ("credentials_absent", "tracker_client_absent", "unreachable")
EXIT_TIMEOUT = 124; EXIT_CRASH = 75; EXIT_SECRET = 77; EXIT_CONFIDENTIAL = 78
INDETERMINATE = (EXIT_TIMEOUT, EXIT_CRASH)
TOP_KEYS = {"schema_version", "db", "id_prefix", "default_severity", "default_reported_by", "evidence_dir", "sync_command", "workable_items_bin", "trackers", "candidate_trackers",
            "first_sync_approvers", "first_sync_pilot_max", "confidential_categories"}   # the first keys are the shared report_item.sh contract
TRK_KEYS = {"name", "kind", "command", "required_env", "env_passthrough", "enabled", "public"}
CATEGORIES = ("bug", "error", "gap", "misalignment", "shortcoming", "weak_spot", "danger_zone", "documentation", "dependency", "test_gap", "governance")
USAGE = ("usage: sync_trackers.sh [--config <file>] [--db docs/<name>.db] [--dry-run] [--json <file>] [--tracker <name>]... [--limit N] "
         "[--max-attempts N] [--backoff-base S] [--adapter-timeout S] [--batch-size N] [--skip-batch-size N] [--breaker N] "
         "[--approved-by <name> --approve-first-sync <plan hash>] [--retry-indeterminate]")
LOCK_RETRIES = 6; LOCK_RETRY_SLEEP = 15.0   # bounded retries of a host memory/disk refusal of the wrapper (nothing ran); never overridden
ENV_REFUSAL = re.compile(r"REFUSED reason=(memory_budget_unavailable|cpu_budget_unavailable|disk_below_headroom|disk_free_unreadable)")

def now(): return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
def now_us(): return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")
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
def sha_text(s): return hashlib.sha256(s.encode("utf-8")).hexdigest()
def q(v):  # SQL literal: only validated strings and integers reach here; NUL is refused anyway
    if v is None: return "NULL"
    if isinstance(v, int): return str(int(v))
    s = str(v)
    if "\x00" in s: raise ValueError("NUL in SQL value")
    return "'" + s.replace("'", "''") + "'"

# ---------------------------------------------------------------- arguments
A = {"config": ".helix/reporting.yaml", "db": None, "dry": False, "json": None, "trackers": [], "limit": None, "attempts": 3, "backoff": 1.0,
     "timeout": 120.0, "batch": 10, "skipbatch": 200, "breaker": 3, "approved_by": None, "approve_plan": None, "retry_indet": False}
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
    elif a == "--retry-indeterminate": A["retry_indet"] = True; i += 1; continue
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
    elif a == "--approved-by": A["approved_by"] = need(a)
    elif a == "--approve-first-sync": A["approve_plan"] = need(a)
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

# ---------------------------------------------------------------- command grammar (WF23 A1)
CRED_KEY = re.compile(r"(token|secret|passw|pwd|api[-_]?key|apikey|auth(?!or)|credential|bearer|private[-_]?key|access[-_]?key|cookie|jwt|signature|(^|[-_.])pat($|[-_.]))", re.I)
SHAPE = re.compile(r"(?:^|[^A-Za-z0-9])(?:gh[pousr]_[A-Za-z0-9]{8,}|github_pat_[A-Za-z0-9_]{8,}|glpat-[A-Za-z0-9_-]{8,}|xox[abprs]-[A-Za-z0-9-]{8,}|AKIA[0-9A-Z]{12,}|AIza[0-9A-Za-z_-]{20,}|sk-[A-Za-z0-9_-]{16,}|eyJ[A-Za-z0-9_-]{10,}\.)")
REF_VALUE = re.compile(r"^\$(\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*)$|^\{(db|id)\}$")
USERINFO = re.compile(r"[A-Za-z][A-Za-z0-9+.-]*://[^/\s]*@|(?:^|[\s=])[^\s/:@=]+:[^\s/@=]+@[^\s/]+")
AUTH_TEXT = re.compile(r"authorization|bearer|private-token|x-api-key|x-auth|proxy-authorization", re.I)
KV = re.compile(r"(?:^|[\s;&|])(-{0,2}[A-Za-z0-9_.-]+)=([^\s]*)")
INTERP = ("python3", "python", "bash", "sh", "node", "ruby", "perl", "env")
def command_problem(toks):
    """None when the adapter command is a plain command line (executable, script, flag words, placeholders); else the class of credential-bearing form found"""
    for n, tk in enumerate(toks):
        if USERINFO.search(tk): return "a user:password@ URL"
        if AUTH_TEXT.search(tk) or tk in ("-H", "--header") or tk.lower().startswith("--header="): return "an authorization header"
        if SHAPE.search(tk): return "a credential-shaped token"
        for m in KV.finditer(tk):
            key, val = m.group(1), m.group(2)
            if CRED_KEY.search(key) and not REF_VALUE.match(val) and not ("file" in key.lower() and val and not SHAPE.search(val) and "@" not in val):
                return "a secret-looking key with a value"
        if tk.startswith("-") and "=" not in tk:
            flag = tk.lstrip("-"); nxt = toks[n + 1] if n + 1 < len(toks) else None
            if CRED_KEY.search(flag) and "file" not in flag.lower() and nxt is not None and not REF_VALUE.match(nxt) and not nxt.startswith("-"):
                return "a secret-looking flag with a separate value"
            interp_pos = n == 1 and os.path.basename(toks[0]) in INTERP
            if re.fullmatch(r"-[A-Za-z]", tk) and not interp_pos and nxt is not None and not nxt.startswith(("-", "{", "$")) and "/" not in nxt and not nxt.endswith((".sh", ".py")):
                return "a short flag with a bare value (use a long flag, a placeholder or the environment)"
    return None
def template_problem(name, tpl):
    if re.search(r"\{(?!db\}|id\})[^}]*\}", tpl): return "only the placeholders {db} and {id} exist"
    if "{db}" not in tpl and "{id}" not in tpl: return "a template without {db} or {id} is a literal value (a credential could sit there); put secrets in the environment and name them in required_env"
    if SHAPE.search(tpl) or USERINFO.search(tpl) or AUTH_TEXT.search(tpl): return "a credential-shaped template"
    return None

# ---------------------------------------------------------------- config
def unique_loader():
    class L(yaml.SafeLoader): pass
    def ctor(loader, node, deep=False):
        loader.flatten_mapping(node); seen = set()
        for kn, _ in node.value:
            k = loader.construct_object(kn, deep=deep)
            try: dup = k in seen
            except TypeError: raise yaml.constructor.ConstructorError("while constructing a mapping", node.start_mark, "unhashable key", kn.start_mark)
            if dup: raise yaml.constructor.ConstructorError("while constructing a mapping", node.start_mark, "duplicate key %r" % (k,), kn.start_mark)
            seen.add(k)
        return yaml.SafeLoader.construct_mapping(loader, node, deep)
    L.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, ctor)
    return L
def cmd_ref(t):
    if not t["has_cmd"] or not (t["cmd"] or "").strip(): return None
    rav = real_argv(t["argv"])
    return "%s#sha256:%s" % (os.path.basename(rav[0]) if rav else "-", sha_text(t["cmd"])[:16])
def real_argv(av):
    """argv without a leading `env [-i] [-u NAME] NAME=value...` (the executable the adapter really starts)"""
    i = 0
    if av and os.path.basename(av[0]) == "env":
        i = 1
        while i < len(av):
            a = av[i]
            if a in ("-u", "-C", "-S", "--unset", "--chdir", "--split-string"): i += 2
            elif a.startswith("-"): i += 1
            elif re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", a): i += 1
            else: break
    return av[i:]
def load_config():
    full = os.path.join(ROOT, A["config"])
    if not os.path.isfile(full): refuse("config_missing", A["config"])
    if yaml is None: refuse("config_malformed", "PyYAML is not available to python3")
    try:
        with open(full, encoding="utf-8") as f: cfg = yaml.load(f, Loader=unique_loader())
    except (yaml.YAMLError, OSError, UnicodeDecodeError) as e:
        why = "duplicate key" if "duplicate key" in str(e) else "not parseable YAML"
        refuse("config_malformed", "%s is %s" % (A["config"], why))
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
    appr = cfg.get("first_sync_approvers", [])
    if not isinstance(appr, list) or any(not isinstance(x, str) or not re.fullmatch(r"[A-Za-z0-9_.@-]{1,64}", x) for x in appr): bad("first_sync_approvers must be a list of names")
    pilot = cfg.get("first_sync_pilot_max", 5)
    if isinstance(pilot, bool) or not isinstance(pilot, int) or not (1 <= pilot <= 100000): bad("first_sync_pilot_max must be an integer 1..100000")
    conf = cfg.get("confidential_categories", ["danger_zone"])
    if not isinstance(conf, list) or any(x not in CATEGORIES for x in conf): bad("confidential_categories must be a list of register categories")
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
        pub = t.get("public", True)
        if not isinstance(pub, bool): bad("trackers[%d].public must be true or false" % n)
        v = t.get("required_env", [])
        if not isinstance(v, list) or any(not isinstance(x, str) or not ENV_RE.match(x) for x in v): bad("trackers[%d].required_env must be a list of variable NAMES" % n)
        req = sorted(set(v))
        v = t.get("env_passthrough", {})
        # shared contract with report_item.sh: a mapping NAME -> value template using {db}/{id} (report_item.sh calls .items() on it: the list form is NOT accepted)
        if not isinstance(v, dict): bad("trackers[%d].env_passthrough must be a mapping NAME -> template (the shared contract with report_item.sh)" % n)
        for k, x in v.items():
            if not isinstance(k, str) or not ENV_RE.match(k) or not isinstance(x, str): bad("trackers[%d].env_passthrough must map variable NAMES to string templates" % n)
            tp = template_problem(k, x)
            if tp: refuse("config_secret_in_env", "trackers[%d].env_passthrough.%s: %s" % (n, k, tp))
        passthrough = dict(sorted(v.items()))
        cmd = t.get("command", None); has_cmd = "command" in t
        if has_cmd and not isinstance(cmd, str): bad("trackers[%d].command must be a string" % n)
        toks = []
        if has_cmd and cmd.strip():
            try: toks = shlex.split(cmd)
            except ValueError: bad("trackers[%d].command is not splittable" % n)
            for tk in toks:
                if re.search(r"\{(?!db\}|id\})[^}]*\}", tk): bad("trackers[%d].command: only the placeholders {db} and {id} exist" % n)
            cp = command_problem(toks)
            if cp: refuse("config_secret_in_command", "trackers[%d].command carries %s; names only, the value belongs in the environment" % (n, cp))
        kind = t.get("kind", nm)
        if not isinstance(kind, str) or not re.fullmatch(r"[A-Za-z0-9_.-]{1,48}", kind): bad("trackers[%d].kind is not a plain word" % n)
        trackers.append({"name": nm, "kind": kind, "has_cmd": has_cmd, "cmd": cmd if has_cmd else None, "argv": toks, "required_env": req,
                         "env_passthrough": passthrough, "enabled": en, "public": pub, "candidate": False})
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
                         "env_passthrough": {}, "enabled": True, "public": True, "candidate": True})
    return cfg, trackers, ev, appr, pilot, conf

CFG, TRACKERS, EVDIR, APPROVERS, PILOT_MAX, CONF_CATS = load_config()
if A["json"] is not None and not (A["json"].startswith(".audit/") or A["json"].startswith(EVDIR.rstrip("/") + "/")):
    refuse("path_invalid", "--json must lie under .audit/ or the configured evidence_dir (%s), got %r" % (EVDIR, A["json"]))
DB = A["db"] or CFG["db"]
if not re.fullmatch(r"docs/[A-Za-z0-9_.-]+\.db", DB): refuse("path_invalid", "--db must be docs/<name>.db")
CFG_SHA = sha_file(os.path.join(ROOT, A["config"]))
ALL_NAMES = {x["name"] for x in TRACKERS}
for t in A["trackers"]:
    if t not in ALL_NAMES: usage("--tracker '%s' is neither configured nor a candidate" % t)
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
WAL_DIR = os.path.join(ROOT, ".audit", "sync", "wal"); WAL_PATH = os.path.join(WAL_DIR, RUN_ID + ".jsonl")

# ---------------------------------------------------------------- single-driver lock (not taken by --dry-run: it writes nothing)
LOCKFD = None
if not A["dry"]:
    os.makedirs(os.path.join(ROOT, ".audit", "sync"), exist_ok=True)
    LOCKFD = open(os.path.join(ROOT, ".audit", "sync", "driver.lock"), "a")
    try: fcntl.flock(LOCKFD, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError: refuse("sync_already_running", "another sync_trackers.sh holds .audit/sync/driver.lock")
    if not (os.path.isfile(LOCKED) and os.access(LOCKED, os.X_OK)): refuse("locked_missing", "scripts/register/locked.sh is not executable")

class Interrupted(BaseException):
    pass
# ---------------------------------------------------------------- write-ahead journal (WF23 B1)
WAL_DIR_SYNCED = [False]
def wal(rec):
    if A["dry"]: return
    rec = dict(rec, db=DB)   # every record names its database: a journal is only ever replayed into the register it was written for
    os.makedirs(WAL_DIR, exist_ok=True)
    fd = os.open(WAL_PATH, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
    try:
        os.write(fd, (json.dumps(rec, sort_keys=True, ensure_ascii=True) + "\n").encode("ascii")); os.fsync(fd)
    finally:
        os.close(fd)
    if not WAL_DIR_SYNCED[0]:
        dfd = os.open(WAL_DIR, os.O_RDONLY)
        try: os.fsync(dfd)
        finally: os.close(dfd)
        WAL_DIR_SYNCED[0] = True

# ---------------------------------------------------------------- writes: ONLY through locked.sh
NCALL = [0]; LOCKED_CALLS = []
def run_locked(label, cmd_after):
    r = None
    IN_WRITE[0] = True
    try:
        for attempt in range(LOCK_RETRIES + 1):
            NCALL[0] += 1
            opid = "%s-%s-%d" % (RUN_ID, label, NCALL[0])
            r = subprocess.run([LOCKED, "--op-id", opid, "--"] + cmd_after, stdin=subprocess.DEVNULL, capture_output=True, encoding="utf-8", errors="replace")
            LOCKED_CALLS.append({"op_id": opid, "label": label, "exit": r.returncode})
            if r.returncode != 0 and ENV_REFUSAL.search(r.stderr or "") and attempt < LOCK_RETRIES:   # the host refused to start the container: nothing ran, ask again later
                time.sleep(LOCK_RETRY_SLEEP); continue
            break
    finally:
        IN_WRITE[0] = False
    if PENDING_SIG[0] is not None:
        n = PENDING_SIG[0]; PENDING_SIG[0] = None; raise Interrupted(n)
    return r.returncode, r.stdout, r.stderr
def write_batch(label, sql):
    os.makedirs(RUN_DIR, exist_ok=True)
    f = os.path.join(RUN_DIR, "batch-%03d-%s.sql" % (NCALL[0] + 1, label))
    with open(f, "w", encoding="utf-8") as fh: fh.write(sql)
    rel = os.path.relpath(f, ROOT)
    return run_locked(label, ["sqlite3", "-bail", "/src/" + DB, ".read /src/" + rel])
def tail_lines(se, n=3):
    return [l for l in (se or "").strip().splitlines()[-n:]]

# ---------------------------------------------------------------- tracker state
def client_absent(t):
    av = real_argv(t["argv"])
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
def envset(n): return bool(os.environ.get(n, "").strip())
def tracker_state(t):
    """(skip_reason, missing_env) when the tracker cannot run an adapter at all, else None"""
    if not t["enabled"]: return ("disabled_by_operator", [])
    if not t["has_cmd"]: return ("not_configured", [])
    if not (t["cmd"] or "").strip() or client_absent(t): return ("tracker_client_absent", [])
    miss = sorted(n for n in t["required_env"] if not envset(n))
    if miss: return ("credentials_absent", miss)
    return None
for t in TRACKERS: t["state"] = tracker_state(t)

def revision(it):
    return hashlib.sha256(json.dumps({"atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "status": it["status"], "type": it["type"],
                                      "severity": it["severity"]}, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode("utf-8")).hexdigest()
def load_items(c):
    rows = c.execute("select i.atm_id,i.title,i.description,i.status,i.type,coalesce(x.severity,i.severity,''),i.current_location,i.representation,coalesce(x.category,'') "
                     "from items i left join reg_item_ext x on x.atm_id=i.atm_id order by i.atm_id").fetchall()
    best = {}
    for r in rows:
        rank = (0 if r[6] == "Issues" else 1, 0 if r[7] == "section" else 1)
        if r[0] not in best or rank < best[r[0]][0]:
            best[r[0]] = (rank, {"atm_id": r[0], "title": r[1], "body": r[2], "status": r[3], "type": r[4], "severity": r[5], "category": r[8]})
    out = []
    for k in sorted(best):
        it = best[k][1]; it["rev"] = revision(it); out.append(it)
    return out
def jlist(s):
    try:
        v = json.loads(s) if s else []
    except ValueError:
        return []
    return v if isinstance(v, list) else []
def load_log(c):
    latest = {}; refs = {}
    for r in c.execute("select tracker_id,atm_id,status,skip_reason,missing_env_names,item_revision,remote_ref,sync_id,exit_code from reg_tracker_sync_log order by sync_id"):
        latest[(r[0], r[1])] = {"status": r[2], "reason": r[3], "missing": jlist(r[4]), "rev": r[5], "sync_id": r[7], "exit": r[8]}
        if r[6]: refs[(r[0], r[1])] = r[6]
    return latest, refs
def load_trackers_db(c):
    return {r[0]: r[1:] for r in c.execute("select tracker_id,kind,command_ref,required_env,enabled from reg_trackers")}
def load_synced(c):
    return {r[0]: r[1] for r in c.execute("select tracker_id,count(*) from reg_tracker_sync_log where status='SYNCED' group by 1")}

# ---------------------------------------------------------------- counters
COUNT = {t["name"]: {"synced": 0, "skipped": {}, "failed": 0, "unchanged": 0, "unchanged_skipped": 0, "breaker_skipped": 0, "breaker_open": 0, "held_indeterminate": 0,
                     "withheld": 0, "withheld_unchanged": 0, "selected": 0} for t in TRACKERS}
FAILS = {t["name"]: 0 for t in TRACKERS}; REJECTS = {t["name"]: 0 for t in TRACKERS}; DONE = {t["name"]: 0 for t in TRACKERS}   # DONE: --limit is per tracker for the whole run
PENDING = []; PEND_CALLS = [0]; UNWRITTEN = []; WRITE_FAILED = [False]; KSEQ = [0]; DRIFT = []; RECOVERED = []; RETIRED = []
OBSERVED = {}   # tracker -> set of reasons seen this run (mint keys)
def bump_skip(name, reason): COUNT[name]["skipped"][reason] = COUNT[name]["skipped"].get(reason, 0) + 1
def observe(name, reason): OBSERVED.setdefault(name, set()).add(reason)
def row_key(r): return (r["tracker_id"], r["atm_id"], r["attempted_at"])
def rows_present(rows):
    lo = min(r["attempted_at"] for r in rows)
    def f(c): return {(a, b, d) for a, b, d in c.execute("select tracker_id,atm_id,attempted_at from reg_tracker_sync_log where attempted_at>=?", (lo,))}
    return read(f)
def batch_sql(rows):
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
    return "\n".join(sql) + "\n"
def flush(force=False, label="log"):
    if not PENDING: return True
    if not force and PEND_CALLS[0] < A["batch"] and len(PENDING) < A["skipbatch"]: return True
    rows = list(PENDING); PENDING.clear(); PEND_CALLS[0] = 0
    rc, so, se = write_batch(label, batch_sql(rows))
    if rc in (21, 22):   # locked.sh: 21 = the write ran but its journal row could not be written; 22 = the container's exit status is unknown. Read the rows back.
        present = rows_present(rows); absent = [r for r in rows if row_key(r) not in present]
        print("sync_trackers: locked.sh exit %d (%s): %d of %d row(s) verified present in the register by reading them back" %
              (rc, "write ran, journal row missing" if rc == 21 else "container exit status unknown", len(rows) - len(absent), len(rows)), file=sys.stderr)
        if not absent:
            wal({"ev": "flushed", "ks": [r["k"] for r in rows if "k" in r]}); return True
        rows_unwritten = absent
    elif rc != 0:
        rows_unwritten = rows
    else:
        wal({"ev": "flushed", "ks": [r["k"] for r in rows if "k" in r]}); return True
    os.makedirs(RUN_DIR, exist_ok=True)
    with open(os.path.join(RUN_DIR, "unwritten.jsonl"), "a", encoding="utf-8") as fh:
        for r in rows_unwritten: fh.write(json.dumps({k: v for k, v in r.items()}, sort_keys=True) + "\n")
    UNWRITTEN.extend(rows_unwritten); WRITE_FAILED[0] = True
    print("sync_trackers: register write failed (locked.sh exit %d); %d row(s) NOT recorded, listed in %s and kept in the write-ahead journal %s for the next run" %
          (rc, len(rows_unwritten), os.path.relpath(os.path.join(RUN_DIR, "unwritten.jsonl"), ROOT), os.path.relpath(WAL_PATH, ROOT)), file=sys.stderr)
    for l in tail_lines(se): print("sync_trackers: locked: " + l, file=sys.stderr)
    return False
def add_row(row, call=False):
    PENDING.append(row)
    if call: PEND_CALLS[0] += 1
    return flush()

# ---------------------------------------------------------------- adapter
IN_WRITE = [False]; PENDING_SIG = [None]
def _sig(n, _f):
    if IN_WRITE[0]: PENDING_SIG[0] = n; return   # never tear a register write in half: the signal is raised right after the wrapper call returns
    raise Interrupted(n)
def kill_group(p):
    """the adapter runs in its own session (start_new_session): its pid is its pgid. Never a pgid <= 1 or our own (constitution 11.4.263)."""
    pgid = p.pid
    if not isinstance(pgid, int) or pgid <= 1 or pgid == os.getpgrp(): return
    try:
        if os.getpgid(pgid) != pgid: return
        os.killpg(pgid, signal.SIGTERM)
    except (ProcessLookupError, PermissionError, OSError): return
    time.sleep(0.2)
    try: os.killpg(pgid, signal.SIGKILL)
    except (ProcessLookupError, PermissionError, OSError): pass
def wait_leader(p, timeout):
    """wait for the leader WITHOUT reaping it (so its pid and process group stay reserved while we signal the group)"""
    deadline = time.monotonic() + timeout; delay = 0.01
    while True:
        try:
            r = os.waitid(os.P_PID, p.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
        except ChildProcessError:
            return "exited"
        if r is not None: return "exited"
        if time.monotonic() >= deadline: return "timeout"
        time.sleep(delay); delay = min(delay * 2, 0.2)
def sanitize_ref(v):
    return "".join(ch if "\x21" <= ch <= "\x7e" else "?" for ch in str(v))[:512] or None
def adapter_once(t, it, ref, k):
    av = [x.replace("{db}", DB).replace("{id}", it["atm_id"]) for x in t["argv"]]
    env = {kk: os.environ[kk] for kk in ("PATH", "HOME", "LANG", "TMPDIR") if kk in os.environ}
    for n in t["required_env"]:
        if envset(n): env[n] = os.environ[n]
    for n, tpl in t["env_passthrough"].items():
        env[n] = tpl.replace("{db}", DB).replace("{id}", it["atm_id"])
    payload = {"db": DB, "tracker": t["name"], "atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "status": it["status"], "type": it["type"],
               "severity": it["severity"], "evidence_dir": EV_REL, "idempotency_key": "%s:%s:%s" % (t["name"], it["atm_id"], it["rev"][:16])}
    if ref: payload["remote_ref"] = ref
    os.makedirs(os.path.join(ROOT, EV_REL), exist_ok=True)
    ad = os.path.join(RUN_DIR, "adapter"); os.makedirs(ad, exist_ok=True)
    pin, pout, perr = (os.path.join(ad, "%d.%s" % (k, x)) for x in ("in", "out", "err"))
    with open(pin, "w", encoding="utf-8") as fh: fh.write(json.dumps(payload))
    t0 = time.time()
    fin = open(pin, "rb"); fout = open(pout, "wb"); ferr = open(perr, "wb")
    try:
        try:
            p = subprocess.Popen(av, stdin=fin, stdout=fout, stderr=ferr, cwd=ROOT, env=env, start_new_session=True)
        except OSError:
            return {"kind": "failed", "exit": 127, "why": "launch_failed", "ref": None}
    finally:
        fin.close(); fout.close(); ferr.close()
    try:
        state = wait_leader(p, A["timeout"])
    except BaseException:
        kill_group(p)
        try: p.wait(timeout=5)
        except Exception: pass
        raise
    kill_group(p)   # a timed-out adapter, and any child an adapter that returned left behind, must not keep pushing
    try: rc = p.wait(timeout=10)
    except subprocess.TimeoutExpired: rc = -9
    if state == "timeout":
        return {"kind": "failed", "exit": EXIT_TIMEOUT, "why": "timeout", "ref": None, "indeterminate": True}
    with open(pout, "rb") as fh: so = fh.read(1 << 20).decode("utf-8", errors="replace")
    lines = [l for l in so.splitlines() if l.strip()]
    try:
        o = json.loads(lines[-1]) if lines else None
    except ValueError:
        o = None
    if rc < 0: rc = 128 - rc   # killed by a signal: a non-zero exit code the CHECK accepts
    if not isinstance(o, dict) or o.get("status") not in ("SYNCED", "SKIPPED", "FAILED"):
        return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "bad_output", "ref": None}
    rr = o.get("remote_ref"); ref_out = None; bad_ref = None
    if isinstance(rr, int) and not isinstance(rr, bool) and rr >= 0: rr = str(rr)
    if isinstance(rr, str) and REF_RE.match(rr): ref_out = rr
    elif rr not in (None, "", False): bad_ref = sanitize_ref(rr)
    rstate = o.get("remote_state") if isinstance(o.get("remote_state"), str) and STATE_RE.match(o.get("remote_state")) else None
    keep_ref = ref_out or bad_ref
    if o["status"] == "SKIPPED":
        sr = o.get("skip_reason")
        if sr not in ADAPTER_SKIPS: return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "bad_skip_reason", "ref": keep_ref}
        miss = o.get("missing_env", [])
        if not isinstance(miss, list) or any(not isinstance(x, str) or not ENV_RE.match(x) for x in miss) or (sr == "credentials_absent" and not miss):
            return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "bad_missing_env", "ref": keep_ref}
        return {"kind": "skipped", "reason": sr, "missing": sorted(set(miss)), "exit": rc if rc >= 0 else None, "ref": ref_out}
    if o["status"] == "FAILED" or rc != 0:
        return {"kind": "failed", "exit": rc if rc != 0 else 70, "why": "adapter_failed", "ref": keep_ref}
    ep = o.get("evidence_path")
    ok = False; ev = None
    if ref_out and isinstance(ep, str) and ep and "\x00" not in ep and not os.path.isabs(ep):
        full = os.path.realpath(os.path.join(ROOT, ep)); evroot = os.path.realpath(os.path.join(ROOT, EV_REL)) + os.sep
        if full.startswith(evroot) and os.path.isfile(full) and os.path.getsize(full) > 0 and os.path.getmtime(full) >= t0 - 1.0:
            with open(full, "rb") as fh: body = fh.read(1 << 20)
            if it["atm_id"].encode() in body and ref_out.encode() in body:   # the receipt names the item and the reference the adapter returned
                ok = True; ev = {"path": os.path.relpath(full, os.path.realpath(ROOT)), "sha256": sha_file(full), "size": os.path.getsize(full)}
    if not ok: return {"kind": "failed", "exit": 65, "why": "synced_without_ref_or_evidence", "ref": keep_ref}
    return {"kind": "synced", "ref": ref_out, "evidence": ev, "remote_state": rstate}
def adapter_call(t, it, ref, k):
    last = None
    for attempt in range(1, A["attempts"] + 1):
        last = adapter_once(t, it, ref, k); last["attempts"] = attempt
        if last["kind"] in ("synced",): break
        if last["kind"] == "skipped" and last["reason"] != "unreachable": break
        if last.get("indeterminate") and not ref: break   # a timed-out create may have landed: a second create could duplicate it
        if attempt < A["attempts"]: time.sleep(min(30.0, A["backoff"] * (2 ** (attempt - 1))))
    return last

# ---------------------------------------------------------------- outbound filter (WF23 D2)
LEAK_RES = [SHAPE, re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----"), USERINFO,
            re.compile(r"(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|private[_-]?token|client[_-]?secret)\b\s*[:=]\s*[\"']?[^\s\"']{6,}"),
            re.compile(r"(?i)authorization:\s*(bearer|basic)\s+\S{8,}")]
def outbound_problem(t, it):
    """(exit code, reason) when the item must not leave the register for this tracker, else None. The matched text is never reported."""
    if t["public"] and it.get("category") in CONF_CATS: return (EXIT_CONFIDENTIAL, "confidential_withheld")
    text = "%s\n%s" % (it["title"] or "", it["body"] or "")
    if any(r.search(text) for r in LEAK_RES): return (EXIT_SECRET, "secret_in_payload")
    return None

# ---------------------------------------------------------------- the pass over items
def process(items, latest, refs):
    for t in TRACKERS:
        name = t["name"]; c = COUNT[name]; st = t["state"]
        for it in items:
            last = latest.get((name, it["atm_id"]))
            if last and last["status"] == "SYNCED" and last["rev"] == it["rev"]:
                c["unchanged"] += 1; continue
            blocked = outbound_problem(t, it) if st is None else None
            if blocked:
                if last and last["status"] == "FAILED" and last["exit"] == blocked[0] and last["rev"] == it["rev"]: c["withheld_unchanged"] += 1; continue
                c["withheld"] += 1
                if not A["dry"] and not add_row(dict(tracker_id=name, atm_id=it["atm_id"], rev=it["rev"], attempted_at=now_us(), status="FAILED", exit_code=blocked[0])): return False
                continue
            if st is None and last and last["status"] == "FAILED" and last["exit"] in INDETERMINATE and not refs.get((name, it["atm_id"])) and not A["retry_indet"]:
                c["held_indeterminate"] += 1; continue
            if A["limit"] is not None and DONE[name] >= A["limit"]: continue
            c["selected"] += 1; DONE[name] += 1
            base = {"tracker_id": name, "atm_id": it["atm_id"], "rev": it["rev"], "attempted_at": now_us()}
            if st is not None:
                reason, miss = st; observe_it = reason != "disabled_by_operator"
                if last and last["status"] == "SKIPPED" and last["reason"] == reason and last["rev"] == it["rev"] and last["missing"] == miss:
                    c["unchanged_skipped"] += 1
                else:
                    bump_skip(name, reason)
                    if not A["dry"] and not add_row(dict(base, status="SKIPPED", skip_reason=reason, missing=miss)): return False
                if observe_it: observe(name, reason)
                continue
            if FAILS[name] >= A["breaker"]:
                if REJECTS[name] == 0:   # every failure of the streak was "unreachable"
                    c["breaker_skipped"] += 1; bump_skip(name, "unreachable"); observe(name, "unreachable")
                    if not A["dry"] and not add_row(dict(base, status="SKIPPED", skip_reason="unreachable")): return False
                else:                    # the tracker ANSWERED (rejected): it is not unreachable; the rest is not attempted and not recorded as skipped
                    c["breaker_open"] += 1
                continue
            if A["dry"]: continue
            KSEQ[0] += 1; k = KSEQ[0]; ref0 = refs.get((name, it["atm_id"]))
            wal({"ev": "intent", "k": k, "tracker": name, "atm_id": it["atm_id"], "rev": it["rev"], "ref": ref0, "t": base["attempted_at"]})
            res = adapter_call(t, it, ref0, k)
            if res["kind"] == "synced":
                FAILS[name] = 0; REJECTS[name] = 0; c["synced"] += 1; refs[(name, it["atm_id"])] = res["ref"]
                row = dict(base, k=k, status="SYNCED", exit_code=0, remote_ref=res["ref"], evidence=res["evidence"])
                if res.get("remote_state") is not None and res["remote_state"].lower() != (it["status"] or "").lower():
                    DRIFT.append({"tracker": name, "atm_id": it["atm_id"], "remote_state": res["remote_state"], "item_status": it["status"]})
            elif res["kind"] == "skipped":
                if res["reason"] == "unreachable": FAILS[name] += 1
                bump_skip(name, res["reason"]); observe(name, res["reason"])
                row = dict(base, k=k, status="SKIPPED", skip_reason=res["reason"], missing=res["missing"], exit_code=res["exit"], remote_ref=res["ref"])
            else:
                FAILS[name] += 1; REJECTS[name] += 1; c["failed"] += 1; observe(name, "failed")
                if res.get("ref"): refs[(name, it["atm_id"])] = res["ref"]
                row = dict(base, k=k, status="FAILED", exit_code=res["exit"], remote_ref=res["ref"])
            wal({"ev": "result", "k": k, "row": row})
            if not add_row(row, call=True): return False
        # rows of one tracker are flushed at the tracker boundary: the next tracker's summary and a crash never see a half-written tracker
        if not A["dry"] and not flush(force=True): return False
    return True

# ---------------------------------------------------------------- reconciliation of earlier runs (WF23 B1/B3)
def reconcile_wal():
    """rows that an earlier run pushed (or may have pushed) but never recorded: write them now, before anything is pushed again"""
    if not os.path.isdir(WAL_DIR): return True
    for fn in sorted(os.listdir(WAL_DIR)):
        path = os.path.join(WAL_DIR, fn)
        if not fn.endswith(".jsonl") or path == WAL_PATH: continue
        intents = {}; results = {}; flushed = set(); closed = False
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            for ln in fh:
                try: e = json.loads(ln)
                except ValueError: continue   # a torn last line
                if e.get("db") != DB: continue   # records of another register are not ours to replay
                if e.get("ev") == "intent": intents[e["k"]] = e
                elif e.get("ev") == "result": results[e["k"]] = e["row"]
                elif e.get("ev") == "flushed": flushed.update(e.get("ks", []))
                elif e.get("ev") == "closed": closed = True
        if closed: continue
        todo = []
        for k, row in sorted(results.items()):
            if k in flushed: continue
            row = dict(row)
            ev = row.get("evidence")
            if ev:
                fp = os.path.join(ROOT, ev["path"])
                if not (os.path.isfile(fp) and sha_file(fp) == ev["sha256"]):   # the receipt changed or is gone: keep the reference, do not claim SYNCED
                    row.pop("evidence"); row.update(status="FAILED", exit_code=65)
            todo.append(row)
        for k, e in sorted(intents.items()):
            if k in results: continue
            todo.append({"k": k, "tracker_id": e["tracker"], "atm_id": e["atm_id"], "rev": e["rev"], "attempted_at": e["t"], "status": "FAILED", "exit_code": EXIT_CRASH, "remote_ref": e.get("ref")})
        if todo:
            present = rows_present(todo)
            todo = [r for r in todo if row_key(r) not in present]
        if todo:
            PENDING.extend(todo)
            if not flush(force=True, label="recover"): return False
            RECOVERED.extend({"tracker": r["tracker_id"], "atm_id": r["atm_id"], "status": r["status"], "wal": fn} for r in todo)
            print("sync_trackers: recovered %d row(s) of %s that were pushed but never recorded" % (len(todo), fn), file=sys.stderr)
        with open(path, "a", encoding="utf-8") as fh: fh.write(json.dumps({"ev": "closed", "by": RUN_ID, "db": DB}) + "\n")
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
            "if [ -z \"$id\" ]; then id=$(sqlite3 \"$db\" \"PRAGMA foreign_keys=ON; INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('$key','reporting_directive'); SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1;\") || exit 3; fi\n"
            "if [ -z \"$(sqlite3 \"$db\" \"SELECT 1 FROM items WHERE atm_id='$id' LIMIT 1\")\" ]; then %s add Task %s --db \"$db\" --id \"$id\" --prefix %s --title '%s' --description '%s' >/dev/null || exit 4; fi\n"
            "if [ -z \"$(sqlite3 \"$db\" \"SELECT 1 FROM reg_item_ext WHERE atm_id='$id' LIMIT 1\")\" ]; then sqlite3 \"$db\" \"PRAGMA foreign_keys=ON; INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$id','gap','artifact','%s');\" || exit 5; fi\n"
            "echo \"$id\"\n") % (db, key, WIIN, MINT_SEV[reason], CFG["id_prefix"], title, desc, MINT_SEV[reason].lower())
MINTED = []; OWED_REOPEN = []
def mint_round():
    new = 0; allok = True
    for name in sorted(OBSERVED):
        for reason in sorted(OBSERVED[name]):
            if reason not in MINT_SEV: continue
            ex = existing_mint(name, reason)
            if ex and ex[1] is not None:
                if ex[1].endswith("(→ Fixed.md)") and not any(m["tracker"] == name and m["reason"] == reason and m["atm_id"] == ex[0] for m in OWED_REOPEN):
                    OWED_REOPEN.append({"tracker": name, "reason": reason, "atm_id": ex[0], "status": ex[1]})
                if not any(m["tracker"] == name and m["reason"] == reason for m in MINTED):
                    MINTED.append({"tracker": name, "reason": reason, "atm_id": ex[0], "action": "existing"})
                continue
            if A["dry"]:
                MINTED.append({"tracker": name, "reason": reason, "atm_id": None, "action": "would_mint"}); continue
            rc, so, se = run_locked("mint", ["bash", "-c", mint_script(name, reason)])
            if rc != 0:   # one tracker's failed mint must not stop the others
                print("sync_trackers: minting the item for %s/%s failed (locked.sh exit %d)" % (name, reason, rc), file=sys.stderr)
                for l in tail_lines(se): print("sync_trackers: locked: " + l, file=sys.stderr)
                WRITE_FAILED[0] = True; allok = False; continue
            got = existing_mint(name, reason)
            MINTED.append({"tracker": name, "reason": reason, "atm_id": got[0] if got else None, "action": "minted"}); new += 1
    return new, allok

# ---------------------------------------------------------------- first-sync gate (docs/04 10.3, WF23 D1)
def first_sync_plan(items, latest, synced):
    """{tracker: [[atm_id, rev], ...]} for the trackers that would push for the first time (adapter state, no SYNCED row), and the hash of exactly that plan"""
    plan = {}
    for t in TRACKERS:
        if t["state"] is not None or synced.get(t["name"], 0) > 0: continue
        its = [[it["atm_id"], it["rev"]] for it in items if not (latest.get((t["name"], it["atm_id"])) or {}).get("status") == "SYNCED"]
        if its: plan[t["name"]] = {"command_ref": cmd_ref(t), "items": its}
    return plan, (sha_text(json.dumps({"config_sha256": CFG_SHA, "db": DB, "plan": plan}, sort_keys=True)) if plan else None)
def first_sync_gate(plan, plan_hash):
    if not plan or A["dry"]: return
    names = ",".join(sorted(plan))
    def no(why): refuse("first_sync_gate", "tracker(s) %s have never SYNCED: %s. Run `--dry-run` for the plan hash, have the owner review it, then run a pilot: --approved-by <approver> --approve-first-sync <hash> --limit N (N <= %d). Nothing was written." % (names, why, PILOT_MAX))
    if not APPROVERS: no("the config names no first_sync_approvers")
    if A["approved_by"] not in APPROVERS: no("--approved-by is missing or not in first_sync_approvers")
    if A["approve_plan"] != plan_hash: no("--approve-first-sync is missing or is not the plan hash of the current item set")
    if A["limit"] is None or A["limit"] > PILOT_MAX: no("a first sync is a pilot: --limit N with N <= %d is required" % PILOT_MAX)

# ---------------------------------------------------------------- main
def tracker_row_upserts():
    dbt = read(load_trackers_db); stmts = []
    for t in TRACKERS:
        want = (t["kind"], cmd_ref(t), json.dumps(t["required_env"]), 1 if t["enabled"] else 0)
        have = dbt.get(t["name"])
        if have is None or tuple(have) != want:
            stmts.append("INSERT INTO reg_trackers(tracker_id,kind,command_ref,required_env,enabled) VALUES (%s,%s,%s,%s,%d) ON CONFLICT(tracker_id) DO UPDATE SET kind=excluded.kind,command_ref=excluded.command_ref,required_env=excluded.required_env,enabled=excluded.enabled;"
                         % (q(t["name"]), q(t["kind"]), q(want[1]), q(want[2]), want[3]))
    if not A["trackers"]:   # a full run: a tracker the config no longer names is retired, so v_stale_tracker_sync stops listing it
        for name, have in sorted(dbt.items()):
            if name not in ALL_NAMES and have[3] == 1:
                stmts.append("UPDATE reg_trackers SET enabled=0 WHERE tracker_id=%s;" % q(name)); RETIRED.append(name)
    return stmts

exit_code = 0; CRASH = [None]
ITEMS0 = read(load_items); LATEST0, REFS0 = read(load_log); SYNCED0 = read(load_synced)
PLAN, PLAN_HASH = first_sync_plan(ITEMS0, LATEST0, SYNCED0)
first_sync_gate(PLAN, PLAN_HASH)   # REFUSED before any write
okp = True
for _s in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP): signal.signal(_s, _sig)
try:
    if not A["dry"]:
        if not reconcile_wal():
            print("sync_trackers: the unrecorded rows of an earlier run could not be written; nothing is pushed", file=sys.stderr); sys.exit(4)
        stm = tracker_row_upserts()
        if stm:
            rc, so, se = write_batch("trackers", "PRAGMA foreign_keys=ON;\nBEGIN IMMEDIATE;\n" + "\n".join(stm) + "\nCOMMIT;\n")
            if rc != 0:
                print("sync_trackers: registering the trackers failed (locked.sh exit %d)" % rc, file=sys.stderr)
                for l in tail_lines(se): print("sync_trackers: locked: " + l, file=sys.stderr)
                sys.exit(4)
    # tracker-level reasons are known before any item is touched: mint their items first so the same pass records them
    for t in TRACKERS:
        if t["state"] is not None and t["state"][0] != "disabled_by_operator": observe(t["name"], t["state"][0])
    if OBSERVED:
        # only trackers with at least one item to report on matter, but the tracker itself is the unit (docs/04 10.3): a register with zero items still mints
        mint_round()
    items = read(load_items); latest, refs = read(load_log)
    okp = process(items, latest, refs)
    if okp and not A["dry"]:
        mint_round()
        fresh = [i for i in read(load_items) if i["atm_id"] not in {x["atm_id"] for x in items}]
        if fresh:
            latest, refs = read(load_log)
            okp = process(fresh, latest, refs)
    if okp and not A["dry"]: okp = flush(force=True)
    if okp and not A["dry"] and not WRITE_FAILED[0]: wal({"ev": "closed", "by": RUN_ID})
except Interrupted as e:
    CRASH[0] = 6; print("sync_trackers: interrupted by signal %s; pending rows are flushed" % e.args[0], file=sys.stderr)
except Exception as e:
    CRASH[0] = 5; print("sync_trackers: internal error %s: %s; pending rows are flushed and the write-ahead journal %s is reconciled by the next run" %
                        (type(e).__name__, str(e)[:200].replace("\n", " "), os.path.relpath(WAL_PATH, ROOT)), file=sys.stderr)
if CRASH[0] is not None and not A["dry"] and PENDING:
    try: flush(force=True)
    except BaseException: pass
if CRASH[0] is not None: exit_code = CRASH[0]
elif WRITE_FAILED[0]: exit_code = 4
elif any(COUNT[n]["failed"] for n in COUNT): exit_code = 1
elif OWED_REOPEN or any(COUNT[n]["held_indeterminate"] for n in COUNT): exit_code = 7

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
rep = {"schema": "sync-trackers/2", "identity": ident(), "dry_run": A["dry"], "db": {"path": DB, "sha256_before": DB_SHA0, "sha256_after": DB_SHA1},
       "options": {"max_attempts": A["attempts"], "backoff_base": A["backoff"], "breaker": A["breaker"], "limit": A["limit"], "trackers": A["trackers"]},
       "first_sync": {"plan": PLAN, "plan_hash": PLAN_HASH, "approvers": APPROVERS, "pilot_max": PILOT_MAX, "approved_by": A["approved_by"] if PLAN and not A["dry"] else None},
       "trackers": {}, "minted": MINTED, "owed_reopen": OWED_REOPEN, "drift": DRIFT, "recovered": RECOVERED, "retired": RETIRED, "locked_calls": LOCKED_CALLS,
       "v_stale_tracker_sync": STALE, "unwritten_rows": len(UNWRITTEN), "exit": exit_code}
for t in TRACKERS:
    rep["trackers"][t["name"]] = {"kind": t["kind"], "candidate": t["candidate"], "public": t["public"], "state": (t["state"][0] if t["state"] else "adapter"),
                                  "missing_env_names": (t["state"][1] if t["state"] and t["state"][1] else []), "counts": COUNT[t["name"]]}
for t in TRACKERS:
    c = COUNT[t["name"]]; sk = ",".join("%s=%d" % kv for kv in sorted(c["skipped"].items())) or "0"
    print("sync_trackers: tracker=%s state=%s selected=%d synced=%d skipped=%s failed=%d unchanged=%d unchanged_skipped=%d breaker_skipped=%d breaker_open=%d held_indeterminate=%d withheld=%d%s"
          % (t["name"], rep["trackers"][t["name"]]["state"], c["selected"], c["synced"], sk, c["failed"], c["unchanged"], c["unchanged_skipped"], c["breaker_skipped"],
             c["breaker_open"], c["held_indeterminate"], c["withheld"],
             " missing_env_names=" + ",".join(rep["trackers"][t["name"]]["missing_env_names"]) if rep["trackers"][t["name"]]["missing_env_names"] else ""))
if not TRACKERS: print("sync_trackers: 0 trackers configured; 0 pushes; nothing claimed as synced")
if PLAN: print("sync_trackers: first_sync_gate trackers=%s plan_hash=%s approvers=%s pilot_max=%d%s" % (",".join(sorted(PLAN)), PLAN_HASH, ",".join(APPROVERS) or "NONE", PILOT_MAX, " (dry run: review this plan, then run a pilot with --approved-by and --approve-first-sync)" if A["dry"] else ""))
for m in MINTED: print("sync_trackers: item %s tracker=%s reason=%s id=%s" % (m["action"], m["tracker"], m["reason"], m["atm_id"]))
for m in OWED_REOPEN: print("sync_trackers: OWED reopen of %s (tracker=%s reason=%s is skipped again but the item is %s); exit 7 until the register custody path reopens it" % (m["atm_id"], m["tracker"], m["reason"], m["status"]))
if DRIFT: print("sync_trackers: drift %d item(s) whose remote_state differs from the register status: %s" % (len(DRIFT), ",".join("%s/%s" % (d["tracker"], d["atm_id"]) for d in DRIFT)))
if RETIRED: print("sync_trackers: retired tracker(s) no longer in the config: %s" % ",".join(RETIRED))
print("sync_trackers: v_stale_tracker_sync %s" % json.dumps(STALE))
print("sync_trackers: %s locked.sh call(s)%s" % (len(LOCKED_CALLS), " (dry run: none)" if A["dry"] else ""))
if A["json"]:
    jp = os.path.join(ROOT, A["json"]); os.makedirs(os.path.dirname(jp) or ".", exist_ok=True)
    tmp = jp + ".tmp.%d" % os.getpid()
    with open(tmp, "w", encoding="utf-8") as fh: json.dump(rep, fh, indent=1, sort_keys=True); fh.write("\n")
    os.replace(tmp, jp)
sys.exit(exit_code)
PY
