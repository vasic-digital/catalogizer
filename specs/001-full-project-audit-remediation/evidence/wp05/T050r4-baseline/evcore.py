"""evcore - shared implementation of tools/evidence/evrec and tools/evidence/verify (tasks.md T050, round 3).

Reference implementation in python3 (stdlib plus the optional `jsonschema` package for the ev/1
schema check). Written test-first against tools/evidence/tests/test_evrec.sh.

Chain definition (docs/06 section 7, the ev/1 contract): entry_hash = sha256(prev_hash || canonical_json(entry
without entry_hash)); prev_hash of seq 1 is 64 zeros; canonical JSON = keys sorted, no whitespace, UTF-8
(ensure_ascii false). This is NOT continuum's construction (see evidence/wp05/T050-implementation.md, DR-E1).

Exit codes (named reasons are printed on stderr as `reason=<name>`):
  verify  0 verified, 1 chain / blob / schema failure, 3 unverifiable (blob missing, anchor present and not compared,
          ledger absent or empty), 64 usage_error (docs/06 section 8; 2 anchor disagreement is T055)
  evrec   0 done, 64 usage_error, 65 record_invalid, 66 ledger_inconsistent, 67 schema_unavailable,
          68 side_unverifiable / no_common_prefix / map_incomplete (rerecord, remap-refs),
          69 interpreter_without_test_sources (docs/06 rule 11), 70 out_is_shared_store (rerecord --out guard),
          75 lock_held, 76 commit_turn_held, 77 target_unreadable
"""
import datetime
import hashlib
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import time

ZERO = "0" * 64
FEATURE = "specs/001-full-project-audit-remediation"
DEFAULT_BOUND = 33554432          # fallback only; the bound is read from scripts/repo/check_classes.tsv
DEFAULT_BLOB_BOUND = 1024000       # fallback only: check_classes.tsv `evidence large_file`
RAISE_NUM, RAISE_DEN = 3, 4        # ledger_raise_owed at >= 75% of the bound


class Refuse(Exception):
    def __init__(self, reason, code, detail=""):
        super().__init__(reason)
        self.reason, self.code, self.detail = reason, code, detail


def eprint(*a):
    print(*a, file=sys.stderr)


# ----------------------------------------------------------------------------- paths

def tool_root():
    return os.path.dirname(os.path.dirname(os.path.dirname(os.path.realpath(__file__))))


def repo_root():
    return os.environ.get("EVREC_REPO_ROOT") or tool_root()


def ev_dir():
    return os.environ.get("EV") or os.path.join(tool_root(), FEATURE, "evidence")


def ledger_path():
    return os.environ.get("EV_LEDGER") or os.path.join(ev_dir(), "ledger.jsonl")


def blobs_path():
    return os.environ.get("EV_BLOBS") or os.path.join(ev_dir(), "blobs")


def anchor_path():
    return os.environ.get("EV_ANCHOR") or os.path.join(ev_dir(), "anchors.jsonl")


def schema_path():
    # Not overridable from the environment (review I4): a producer that could point the validator at `{}` would
    # record and verify any entry. The contract file beside the tool is the only schema.
    return os.path.join(tool_root(), FEATURE, "contracts", "evidence-record.schema.json")


# ----------------------------------------------------------------------------- hashing / canonical json

def sha_bytes(b):
    return hashlib.sha256(b).hexdigest()


def canon(o):
    return json.dumps(o, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def compute_entry_hash(rec):
    body = {k: v for k, v in rec.items() if k != "entry_hash"}
    return sha_bytes(rec["prev_hash"].encode("ascii") + canon(body))


def verdict_of(rc):
    if rc == 0:
        return "pass"
    if 1 <= rc <= 125:
        return "fail"
    return "error"


# ----------------------------------------------------------------------------- schema

_validator = None


def validator():
    global _validator
    if _validator is None:
        try:
            import jsonschema
            with open(schema_path(), "rb") as f:
                s = json.load(f)
            _validator = jsonschema.Draft202012Validator(s)
        except Exception as e:                      # fail closed: no schema check means no record
            raise Refuse("schema_unavailable", 67, "%s: %s" % (type(e).__name__, e))
    return _validator


_TS = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z")


def extra_checks(rec):
    """Checks the Draft 2020-12 validator does not make without a format checker (review minor 5)."""
    out = []
    ts = rec.get("started_at") if isinstance(rec, dict) else None
    if isinstance(ts, str):
        ok = _TS.fullmatch(ts) is not None
        if ok:
            try:
                datetime.datetime.strptime(ts[:19], "%Y-%m-%dT%H:%M:%S")
            except ValueError:
                ok = False
        if not ok:
            out.append("started_at %r is not an RFC 3339 UTC date-time" % ts[:40])
    return out


def schema_errors(rec):
    return sorted(e.message[:200] for e in validator().iter_errors(rec)) + extra_checks(rec)


# ----------------------------------------------------------------------------- atomic file write

def fsync_dir(path):
    fd = os.open(path or ".", os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def atomic_write(path, data, fault=False):
    d = os.path.dirname(path) or "."
    os.makedirs(d, exist_ok=True)
    tmp = "%s.tmp.%d" % (path, os.getpid())
    with open(tmp, "wb") as f:
        f.write(data)
        f.flush()
        os.fsync(f.fileno())
    if fault and os.environ.get("EVREC_FAULT") == "kill_before_rename":
        os.kill(os.getpid(), signal.SIGKILL)         # test hook: the previous ledger must stay intact
    os.rename(tmp, path)
    fsync_dir(d)


# ----------------------------------------------------------------------------- ledger lock (11.4.180)

def _proc_root():
    return os.environ.get("EVREC_PROC_ROOT") or "/proc"     # test seam: lets a test make /proc unreadable


def _cmdline(pid):
    try:
        with open("%s/%d/cmdline" % (_proc_root(), pid), "rb") as f:
            return f.read().replace(b"\0", b" ").decode("utf-8", "replace").rstrip(" ")
    except OSError:
        return None


def _holder_stale(h):
    """True only on PROVEN staleness: holder pid dead, or its real cmdline is READABLE and differs from the recorded
    one (pid reuse). A live holder whose cmdline cannot be read (hidepid, no /proc, non-Linux host) is NOT proven stale
    and is never reaped (11.4.180, 11.4.201(4); review I1)."""
    pid, cmd = h.get("pid"), h.get("cmdline")
    if not isinstance(pid, int) or pid <= 1 or not isinstance(cmd, str):
        return False                                   # unreadable holder identity: conservative, never reaped
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    except PermissionError:
        pass
    cur = _cmdline(pid)
    if cur is None:
        return False                                   # alive, identity unreadable: not proven stale
    return cur != cmd.rstrip(" ")


def lock_timeout():
    raw = os.environ.get("EVREC_LOCK_TIMEOUT") or "30"
    try:
        return float(raw)
    except ValueError:
        raise Refuse("usage_error", 64, "EVREC_LOCK_TIMEOUT=%r is not a number" % raw)


class LedgerLock:
    def __init__(self, ledger, timeout):
        self.lock, self.guard, self.timeout = ledger + ".lock", ledger + ".lock.guard", timeout

    def _with_guard(self, fn):
        import fcntl
        os.makedirs(os.path.dirname(self.guard) or ".", exist_ok=True)
        fd = os.open(self.guard, os.O_CREAT | os.O_RDWR, 0o644)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX)
            return fn()
        finally:
            os.close(fd)

    def _try(self):
        me = json.dumps({"pid": os.getpid(), "cmdline": _cmdline(os.getpid()) or "",
                         "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())})
        while True:
            try:
                fd = os.open(self.lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o644)
            except FileExistsError:
                try:
                    with open(self.lock, "rb") as f:
                        h = json.loads(f.read().decode("utf-8"))
                    if not isinstance(h, dict):
                        raise ValueError
                except (OSError, ValueError):
                    return False, "unreadable lock file"
                if _holder_stale(h):
                    try:
                        os.unlink(self.lock)           # reap, then retry the exclusive create under the guard
                    except FileNotFoundError:
                        pass
                    continue
                return False, "held by pid %s (%s)" % (h.get("pid"), h.get("cmdline"))
            with os.fdopen(fd, "w") as f:
                f.write(me + "\n")
                f.flush()
                os.fsync(f.fileno())
            return True, ""

    def acquire(self):
        deadline = time.monotonic() + self.timeout
        while True:
            ok, why = self._with_guard(self._try)
            if ok:
                return
            if time.monotonic() >= deadline:
                raise Refuse("lock_held", 75, why)
            time.sleep(0.05)

    def release(self):
        def rel():
            try:
                with open(self.lock, "rb") as f:
                    h = json.loads(f.read().decode("utf-8"))
                if h.get("pid") == os.getpid():
                    os.unlink(self.lock)
            except (OSError, ValueError):
                pass
        self._with_guard(rel)


# ----------------------------------------------------------------------------- commit-turn freeze (rev 29/30)

def _read_grant():
    g = os.path.join(repo_root(), ".audit", "commit_turn.json")
    if not os.path.lexists(g):
        return None, None
    try:
        with open(g, "rb") as f:
            d = json.loads(f.read().decode("utf-8"))
        if not isinstance(d, dict) or not isinstance(d.get("run_id"), str):
            raise ValueError("not a grant object")
        return d, None
    except (OSError, ValueError) as e:                  # unreadable or not JSON: conservative default (11.4.201(4))
        return None, "unreadable grant: %s" % type(e).__name__


def commit_turn_check(allow_reap=True, where="pre_run", note=""):
    """Refuse (commit_turn_held) while another run holds the grant. At most one reaper call, only through the
    host entry point with --exec-approved; an absent or failing reaper leaves the grant standing. The check runs
    at three points (pre_run, post_command, in_lock; review I2) and the refusal names which one fired."""
    tail = " (where=%s%s)" % (where, note)
    grant, err = _read_grant()
    if grant is None and err is None:
        return
    if err:
        raise Refuse("commit_turn_held", 76, err + tail)
    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):
        return
    if allow_reap:
        entry = os.environ.get("CPA_HOST_ENTRY") or os.path.join(os.path.expanduser("~"), ".local", "bin", "cpa-host")
        try:
            subprocess.run([entry, "--exec-approved", "scripts/release/commit_turn_check.sh", "--reap"],
                           cwd=repo_root(), stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, timeout=60)
        except (OSError, subprocess.SubprocessError):
            pass
        grant, err = _read_grant()
        if grant is None and err is None:
            return
        if err:
            raise Refuse("commit_turn_held", 76, err + tail)
        if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):
            return
    raise Refuse("commit_turn_held", 76, "grant held by run_id %s%s" % (grant["run_id"], tail))


# ----------------------------------------------------------------------------- redaction (11.4.10)

# Default credential patterns (docs/06 rule 8: "argv and streams are scanned for credential patterns"). Each entry is
# (label, compiled regex, group to redact). Named variables (EVREC_REDACT_VARS) are redacted in addition.
_NAMEPART = rb"[A-Za-z0-9_.-]*"
SECRET_PATTERNS = [
    ("bearer", re.compile(rb"(?i)\bBearer[ \t]+([A-Za-z0-9._~+/=-]{8,})"), 1),
    ("kv_secret", re.compile(rb"(?i)(?<![A-Za-z0-9])" + _NAMEPART + rb"(?:password|passwd|secret|token|api[_-]?key|access[_-]?key)"
                             + _NAMEPART + rb"[ \t]*[=:][ \t]*[\"']?([^\s\"']{6,})"), 1),
    ("aws_key_id", re.compile(rb"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b"), 0),
    ("github_token", re.compile(rb"\bgh[pousr]_[A-Za-z0-9]{36,}\b"), 0),
    ("url_userinfo", re.compile(rb"://[^\s/:@]+:([^\s/@]+)@"), 1),
    ("pem_private_key", re.compile(rb"-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|\Z)"), 0),
]


def redactions():
    out = []
    for name in re.split(r"[,\s]+", os.environ.get("EVREC_REDACT_VARS", "").strip()):
        if not name:
            continue
        v = os.environ.get(name)
        if v is None or v == "":
            continue
        if len(v) < 6:
            eprint("evrec: redact_var_too_short name=%s (value shorter than 6, NOT redacted)" % name)
            continue
        out.append((name, v.encode("utf-8")))
    return out


def redact_bytes(b, reds, patterns=True):
    """Return (bytes, hit). Every occurrence of every named value (overlapping occurrences and values that are
    prefixes or overlaps of each other included) and every default-pattern match is located on the ORIGINAL bytes,
    overlapping spans are merged, and each merged span becomes one [REDACTED:labels] marker, so no fragment of a
    longer or overlapping secret survives (review I7)."""
    spans = []
    for name, v in reds:
        i = b.find(v)
        while i != -1:
            spans.append((i, i + len(v), name))
            i = b.find(v, i + 1)
    if patterns:
        for label, rx, grp in SECRET_PATTERNS:
            for m in rx.finditer(b):
                s, e = m.span(grp)
                if e > s:
                    spans.append((s, e, "pattern:" + label))
    if not spans:
        return b, False
    spans.sort(key=lambda x: (x[0], -x[1]))
    merged = []
    for s, e, lab in spans:
        if merged and s <= merged[-1][1]:
            merged[-1][1] = max(merged[-1][1], e)
            merged[-1][2].add(lab)
        else:
            merged.append([s, e, {lab}])
    out, pos = [], 0
    for s, e, labs in merged:
        out.append(b[pos:s])
        ordered = sorted(l for l in labs if not l.startswith("pattern:")) + sorted(l for l in labs if l.startswith("pattern:"))
        out.append(b"[REDACTED:" + "+".join(ordered).encode() + b"]")
        pos = e
    out.append(b[pos:])
    return b"".join(out), True


def redact_text(t, reds):
    b, hit = redact_bytes(t.encode("utf-8", "surrogateescape"), reds)
    return (b.decode("utf-8", "replace") if hit else t), hit


# ----------------------------------------------------------------------------- run

def file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def fingerprint_target(ref):
    """(digest, resolved). An unresolved target gets the all-zero sentinel (no real SHA-256 digest equals it and it
    is not derived from the ref, so it cannot be mistaken for a read fingerprint; review I8). Streams, never slurps."""
    try:
        if os.path.isfile(ref):
            return file_sha(ref), True
        if os.path.isdir(ref):
            h = hashlib.sha256()
            for root, dirs, files in os.walk(ref):
                dirs.sort()
                for fn in sorted(files):
                    p = os.path.join(root, fn)
                    if os.path.isfile(p):
                        h.update(os.path.relpath(p, ref).encode() + b"\0" + file_sha(p).encode() + b"\n")
            return h.hexdigest(), True
    except OSError as e:
        raise Refuse("target_unreadable", 77, "target %s cannot be read: %s" % (ref, e.strerror))
    return ZERO, False


INTERPRETERS = {"bash", "sh", "dash", "zsh", "ksh", "fish", "csh", "tcsh", "python", "perl", "ruby", "node", "nodejs", "deno",
                "bun", "php", "lua", "go", "npx", "npm", "pnpm", "yarn", "java", "kotlin", "gradle", "gradlew", "mvn", "cargo",
                "dotnet", "env", "pwsh", "powershell", "make", "tclsh", "rscript", "awk", "gawk", "xargs", "timeout", "nice",
                "sudo", "time", "stdbuf", "nohup", "script", "cmd"}


def is_interpreter(argv0):
    base = os.path.basename(argv0).lower()
    return base in INTERPRETERS or re.sub(r"[0-9.]+$", "", base) in INTERPRETERS


def test_fingerprint(argv0, sources):
    f = argv0 if "/" in argv0 else shutil.which(argv0)
    if not f or not os.path.isfile(f) or not os.access(f, os.R_OK):
        return None
    lines = [file_sha(p) for p in [f] + list(sources)]
    return sha_bytes(("\n".join(lines) + "\n").encode())


def store_blob(data):
    d = sha_bytes(data)
    bdir = blobs_path()
    p = os.path.join(bdir, d)
    if not os.path.exists(p):
        os.makedirs(bdir, exist_ok=True)
        tmp = "%s.tmp.%d" % (p, os.getpid())
        with open(tmp, "wb") as f:
            f.write(data)
            f.flush()
            os.fsync(f.fileno())
        os.rename(tmp, p)
    return d


def read_ledger_lines(path):
    if not os.path.exists(path):
        return b"", []
    with open(path, "rb") as f:
        raw = f.read()
    if raw and not raw.endswith(b"\n"):
        raise Refuse("ledger_inconsistent", 66, "ledger does not end with a newline (torn write?)")
    return raw, [l for l in raw.split(b"\n") if l]


def append_entry(rec_body, blobs=()):
    """Append one entry under the ledger lock. Order inside the lock: grant re-check, ledger tail check, record
    validation, THEN the blobs, THEN the atomic ledger write, so a refusal leaves no blob behind (review I2, I11)."""
    led = ledger_path()
    lock = LedgerLock(led, lock_timeout())
    lock.acquire()
    try:
        commit_turn_check(allow_reap=False, where="in_lock")        # re-read inside the lock; the reaper already ran once
        raw, lines = read_ledger_lines(led)
        prev, seq = ZERO, 1
        if lines:
            try:
                last = json.loads(lines[-1])
                ok = compute_entry_hash(last) == last["entry_hash"] and last["seq"] == len(lines)
            except Exception:
                ok = False
            if not ok:
                raise Refuse("ledger_inconsistent", 66, "last entry fails its own hash or seq != line count; run verify")
            prev, seq = last["entry_hash"], len(lines) + 1
        rec = dict(rec_body, seq=seq, prev_hash=prev)
        rec["entry_hash"] = compute_entry_hash(rec)
        errs = schema_errors(rec)
        if errs:
            raise Refuse("record_invalid", 65, "; ".join(errs))
        for data in blobs:
            store_blob(data)
        atomic_write(led, raw + canon(rec) + b"\n", fault=True)
        return rec
    finally:
        lock.release()


def class_bound(cls, key, env, default):
    """Bound from the environment override (empty counts as unset), else scripts/repo/check_classes.tsv."""
    if os.environ.get(env):
        try:
            return int(os.environ[env]), env
        except ValueError:
            raise Refuse("usage_error", 64, "%s=%r is not an integer" % (env, os.environ[env]))
    p = os.path.join(tool_root(), "scripts", "repo", "check_classes.tsv")
    try:
        with open(p, encoding="utf-8") as f:
            for l in f:
                c = l.rstrip("\n").split("\t")
                if len(c) >= 3 and c[0] == cls and c[1] == key and c[2].isdigit():
                    return int(c[2]), "scripts/repo/check_classes.tsv"
    except OSError:
        pass
    return default, "built-in fallback (check_classes.tsv unreadable)"


def owed_relocations(sizes):
    """sizes: {digest: bytes}. Lines for blobs above the `evidence large_file` bound (T050: large in-tree blobs are
    listed as owed relocations; the relocation itself is a held, reviewed step, never done by the recorder)."""
    bound, _ = class_bound("evidence", "large_file", "EV_BLOB_BOUND", DEFAULT_BLOB_BOUND)
    return ["owed_relocation blob=%s bytes=%d bound=%d" % (d, sz, bound) for d, sz in sorted(sizes.items()) if sz > bound]


def _stall_for_test(led):
    """EVREC_FAULT=stall_before_lock: touch <ledger>.stalled and wait (at most 30 s) for <ledger>.go. Test hook that makes
    the window between the post-command grant check and the lock deterministic (review I2)."""
    open(led + ".stalled", "w").close()
    deadline = time.monotonic() + 30
    while not os.path.exists(led + ".go") and time.monotonic() < deadline:
        time.sleep(0.02)


_FLAGS = {"--oracle": "oracle", "--evidence-class": "evidence_class", "--precondition-provenance": "pp",
          "--mutation-json": "mutation", "--container-image-digest": "digest", "--test-source": "test_source",
          "--test-state-go": "state_go"}
_BOOLS = {"--closes-item": "closes", "--pre-release": "pre_release", "--oracle-independent": "independent"}
_USAGE = "evrec run ITEM POLARITY ITER CLASS REF [opts] -- argv..."


def parse_run(args):
    pos, opts, i = [], {"test_source": [], "state_go": []}, 0
    while i < len(args) and args[i] != "--":
        a = args[i]
        if a in _FLAGS:
            if i + 1 >= len(args) or args[i + 1] == "--":
                raise Refuse("usage_error", 64, "%s needs a value" % a)
            k = _FLAGS[a]
            if isinstance(opts.get(k), list):
                opts[k].append(args[i + 1])
            else:
                opts[k] = args[i + 1]
            i += 2
        elif a in _BOOLS:
            opts[_BOOLS[a]] = True
            i += 1
        elif a.startswith("--"):
            raise Refuse("usage_error", 64, "unknown flag %s; %s" % (a, _USAGE))
        else:
            pos.append(a)
            i += 1
    if len(pos) != 5 or i >= len(args) or len(args) - i < 2:
        raise Refuse("usage_error", 64, _USAGE)
    return pos, opts, args[i + 1:]


def _prevalidate(base):
    """Refuse BEFORE the command runs when no outcome (pass 0, fail 1, error 127) could make the record valid."""
    best = None
    for rc in (0, 1, 127):
        r = dict(base, exit_status=rc, verdict=verdict_of(rc), seq=1, prev_hash=ZERO, entry_hash=ZERO)
        errs = schema_errors(r)
        if not errs:
            return
        if best is None or len(errs) < len(best):
            best = errs
    raise Refuse("record_invalid", 65, "; ".join(best) + " (checked before the command ran)")


def cmd_run(args):
    pos, opts, argv = parse_run(args)
    item, polarity, it, tclass, ref = pos
    if not it.isdigit() or int(it) < 1:
        raise Refuse("usage_error", 64, "ITER must be a positive integer")
    # --- everything that needs no I/O and every flag is checked BEFORE the command runs (review I11)
    cwd = os.getcwd()
    for label, val in [("argv", a) for a in argv] + [("cwd", cwd), ("item", item), ("target_ref", ref)]:
        try:
            val.encode("utf-8")
        except UnicodeEncodeError:
            raise Refuse("usage_error", 64, "%s is not valid UTF-8; the record cannot hold it (the command was not run)" % label)
    if opts.get("mutation"):
        try:
            mutation = json.loads(opts["mutation"])
        except ValueError as e:
            raise Refuse("usage_error", 64, "--mutation-json is not JSON: %s" % e)
        if not isinstance(mutation, dict):
            raise Refuse("usage_error", 64, "--mutation-json must be a JSON object")
    if opts.get("oracle") and not opts.get("independent"):
        raise Refuse("oracle_independence_undeclared", 64,
                     "--oracle names a strategy; the caller must declare --oracle-independent (the recorder never asserts independence)")
    if opts.get("independent") and not opts.get("oracle"):
        raise Refuse("usage_error", 64, "--oracle-independent without --oracle")
    lock_timeout()
    owed_relocations({})                                  # validates EV_BLOB_BOUND now, not after the entry is written
    commit_turn_check(allow_reap=True, where="pre_run")   # before anything is written (or run)

    reds = redactions()
    sources = list(opts["test_source"]) + [s for s in os.environ.get("EV_TEST_SOURCES", "").split() if s]
    for s in sources:
        if not (os.path.isfile(s) and os.access(s, os.R_OK)):
            raise Refuse("usage_error", 64, "test source %s is not a readable file (the command was not run)" % s)
    if polarity in ("RED", "GREEN", "MUTATION") and not sources and is_interpreter(argv[0]):
        raise Refuse("interpreter_without_test_sources", 69,
                     "argv[0] %s is an interpreter: its test sources must be declared with --test-source or EV_TEST_SOURCES "
                     "(docs/06 rule 11), otherwise test_fingerprint hashes only the interpreter" % os.path.basename(argv[0]))
    tfp = test_fingerprint(argv[0], sources)            # taken BEFORE the run
    target_fp, resolved = fingerprint_target(ref)
    if not resolved and polarity != "PROBE":
        raise Refuse("target_unreadable", 77, "target %s does not resolve; no fingerprint can be read from it" % ref)

    r_argv, red_hit = [], False
    for a in argv:
        t, h = redact_text(a, reds)
        r_argv.append(t)
        red_hit |= h
    r_cwd, h = redact_text(cwd, reds)
    red_hit |= h
    r_ref, h = redact_text(ref, reds)
    red_hit |= h
    started = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    rec = {"schema": "ev/1", "item": item, "polarity": polarity, "iteration": int(it), "started_at": started,
           "cwd": r_cwd, "argv": r_argv, "duration_ms": 0, "stdout_sha256": ZERO, "stderr_sha256": ZERO,
           "target_class": tclass, "target_ref": r_ref, "target_fingerprint": target_fp}
    if tfp:
        rec["test_fingerprint"] = tfp
    if opts.get("oracle"):
        rec["oracle"] = {"strategy": opts["oracle"], "independent_of_sut": True}   # True only because the caller declared it
    if opts.get("evidence_class"):
        rec["evidence_class"] = opts["evidence_class"]
    if opts.get("pp"):
        rec["precondition_provenance"] = opts["pp"]
    if opts.get("closes"):
        rec["closes_item"] = True
    if opts.get("mutation"):
        rec["mutation"] = mutation
    if opts.get("digest"):
        rec["container_image_digest"] = opts["digest"]
    if opts.get("pre_release"):
        rec["pre_release"] = True
    if opts["state_go"]:
        rec["test_state_gos"] = list(opts["state_go"])
    if red_hit:
        rec["redacted"] = True
    _prevalidate(rec)

    t0 = time.monotonic_ns()
    try:
        p = subprocess.run(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        rc, out, err = p.returncode, p.stdout, p.stderr
        if rc < 0:
            rc = 128 + (-rc)
    except FileNotFoundError as e:
        rc, out, err = 127, b"", ("evrec: %s\n" % e).encode()
    except PermissionError as e:
        rc, out, err = 126, b"", ("evrec: %s\n" % e).encode()
    dur = (time.monotonic_ns() - t0) // 1_000_000

    note = " command_ran=true exit=%d nothing_stored" % rc
    commit_turn_check(allow_reap=False, where="post_command", note=note)   # a grant that appeared during the command
    if os.environ.get("EVREC_FAULT") == "stall_before_lock":
        _stall_for_test(ledger_path())

    out, h1 = redact_bytes(out, reds)
    err, h2 = redact_bytes(err, reds)
    rec["duration_ms"], rec["exit_status"], rec["verdict"] = dur, rc, verdict_of(rc)
    rec["stdout_sha256"], rec["stderr_sha256"] = sha_bytes(out), sha_bytes(err)   # digest of the stored (redacted) bytes
    if h1 or h2:
        rec["redacted"] = True
    errs = schema_errors(dict(rec, seq=1, prev_hash=ZERO, entry_hash=ZERO))
    if errs:
        raise Refuse("record_invalid", 65, "; ".join(errs) + " (the command ran, exit %d; nothing stored)" % rc)
    done = append_entry(rec, blobs=(out, err))
    print("recorded seq=%d item=%s pol=%s it=%s exit=%d verdict=%s" % (done["seq"], item, polarity, it, rc, done["verdict"]))
    for line in owed_relocations({done["stdout_sha256"]: len(out), done["stderr_sha256"]: len(err)}):
        print(line)
    return 0


# ----------------------------------------------------------------------------- check-record / reconcile

def cmd_check_record(args):
    if len(args) != 1:
        raise Refuse("usage_error", 64, "evrec check-record FILE")
    try:
        with open(args[0], "rb") as f:
            rec = json.loads(f.read().decode("utf-8"))
    except (OSError, ValueError) as e:
        raise Refuse("record_invalid", 65, "unreadable or not JSON: %s" % type(e).__name__)
    if not isinstance(rec, dict):
        raise Refuse("record_invalid", 65, "record is not a JSON object")
    probe = dict(rec)
    probe.setdefault("seq", 1)                          # a record is checked before it is chained
    probe.setdefault("prev_hash", ZERO)
    probe.setdefault("entry_hash", ZERO)
    errs = schema_errors(probe)
    if errs:
        raise Refuse("record_invalid", 65, "; ".join(errs))
    print("record valid")
    return 0


def cmd_reconcile(args):
    if len(args) != 2 or args[0] != "--runner-count" or not args[1].isdigit():
        raise Refuse("usage_error", 64, "evrec reconcile --runner-count N")
    _, lines = read_ledger_lines(ledger_path())
    if len(lines) != int(args[1]):
        raise Refuse("count_mismatch", 1, "runner ran %s commands, ledger holds %d entries" % (args[1], len(lines)))
    print("reconciled %d == %d" % (len(lines), len(lines)))
    return 0


# ----------------------------------------------------------------------------- verify (chain walk, schema, blobs)

class _DupKey(Exception):
    pass


def _no_dups(pairs):
    d = {}
    for k, v in pairs:
        if k in d:
            raise _DupKey(k)
        d[k] = v
    return d


def chain_walk(path, deep=True):
    """Return (entries, last_hash). Raises Refuse code 1 (chain failure) or 3 (unverifiable). With deep=True every
    entry is also validated against ev/1 (the tool's own schema, never the caller's), must be byte-canonical, and an
    unresolved-fingerprint sentinel is refused outside PROBE (review I4, I8, minors 3 and 4)."""
    try:
        with open(path, "rb") as f:
            raw = f.read()
    except OSError as e:
        raise Refuse("UNVERIFIED", 3, "ledger unreadable: %s" % e.strerror)
    if not raw:
        raise Refuse("UNVERIFIED", 3, "ledger empty: nothing to verify")
    if not raw.endswith(b"\n"):
        raise Refuse("chain_failure", 1, "truncated final line (no newline)")
    prev, n, entries = ZERO, 0, []
    for line in raw.split(b"\n")[:-1]:
        n += 1
        try:
            rec = json.loads(line.decode("utf-8"), object_pairs_hook=_no_dups)
        except _DupKey as e:
            raise Refuse("non_canonical_line", 1, "duplicate key %s at line %d" % (e, n))
        except Exception:
            raise Refuse("chain_failure", 1, "undecodable entry at line %d" % n)
        if not isinstance(rec, dict) or any(k not in rec for k in ("schema", "seq", "prev_hash", "entry_hash")) \
                or not isinstance(rec["prev_hash"], str) or not isinstance(rec["entry_hash"], str):
            raise Refuse("chain_failure", 1, "undecodable entry at line %d" % n)
        if rec["prev_hash"] != prev:
            raise Refuse("chain_failure", 1, "broken link at line %d (seq=%s)" % (n, rec["seq"]))
        try:
            hashed = compute_entry_hash(rec)
        except (UnicodeError, TypeError, ValueError):
            raise Refuse("chain_failure", 1, "undecodable entry at line %d" % n)
        if hashed != rec["entry_hash"]:
            raise Refuse("chain_failure", 1, "bad hash at line %d (seq=%s)" % (n, rec["seq"]))
        if rec["seq"] != n:
            raise Refuse("chain_failure", 1, "seq %s at line %d is not contiguous" % (rec["seq"], n))
        if deep:
            if line != canon(rec):
                raise Refuse("non_canonical_line", 1, "line %d is not the canonical JSON of its own entry" % n)
            errs = schema_errors(rec)
            if errs:
                raise Refuse("schema_invalid", 1, "line %d (seq=%s) is not a valid ev/1 entry: %s" % (n, rec["seq"], "; ".join(errs[:3])))
            if rec.get("target_fingerprint") == ZERO and rec.get("polarity") != "PROBE":
                raise Refuse("unresolved_fingerprint", 1, "line %d (seq=%s) %s carries the unresolved-target sentinel; only PROBE may" % (n, rec["seq"], rec.get("polarity")))
        prev = rec["entry_hash"]
        entries.append(rec)
    return entries, prev


def check_blobs(entries, bdir):
    """docs/06 s3.1 rule 2 and s16: each stream digest must resolve to a blob that hashes to its own name. A blob that
    does not hash to its name is a failure (1); a missing blob leaves that stream UNVERIFIED (3)."""
    missing, mismatched = [], []
    for i, rec in enumerate(entries, 1):
        for key in ("stdout_sha256", "stderr_sha256"):
            d = rec[key]
            p = os.path.join(bdir, d)
            if not os.path.isfile(p):
                missing.append("line %d %s %s" % (i, key[:6], d[:12]))
            elif file_sha(p) != d:
                mismatched.append("line %d %s blob %s does not hash to its name" % (i, key[:6], d[:12]))
    if mismatched:
        raise Refuse("blob_mismatch", 1, "; ".join(mismatched[:3]))
    if missing:
        raise Refuse("blob_missing", 3, "%d blob(s) missing in %s (stream UNVERIFIED): %s" % (len(missing), bdir, "; ".join(missing[:3])))


def ledger_bound():
    return class_bound("evidence-ledger", "large_file", "EV_LEDGER_BOUND", DEFAULT_BOUND)


def cmd_verify(args):
    led = blobs = None
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--ledger" and led is None and i + 1 < len(args) and not args[i + 1].startswith("--"):
            led = args[i + 1]
        elif a == "--blobs" and blobs is None and i + 1 < len(args) and not args[i + 1].startswith("--"):
            blobs = args[i + 1]
        else:
            raise Refuse("usage_error", 64, "verify [--ledger FILE] [--blobs DIR]; refused argument %r" % a)
        i += 2
    default = led is None
    led = led or ledger_path()
    bdir = blobs or os.environ.get("EV_BLOBS") or (blobs_path() if default else os.path.join(os.path.dirname(os.path.abspath(led)), "blobs"))
    print("verify: ledger=%s blobs=%s" % (led, bdir))
    owed_relocations({})                                 # validates EV_BLOB_BOUND up front
    if os.environ.get("EV_SCHEMA"):
        print("note: EV_SCHEMA is ignored; verify uses the tool's own contracts/evidence-record.schema.json")
    bound, src = ledger_bound()
    entries, head = chain_walk(led)
    check_blobs(entries, bdir)
    a = anchor_path()
    if os.path.exists(a) and os.path.getsize(a) > 0:
        raise Refuse("anchor_not_compared", 3, "chain, schema and blobs of %d entries verified, but anchor %s is present and NOT compared "
                     "(anchor check is T055): UNVERIFIED against the anchor" % (len(entries), a))
    print("OK chain=%d entries head=%s (chain, ev/1 schema, canonical form and blobs verified)" % (len(entries), head[:12]))
    print("note: no anchor compared (T055); chain-only verification")
    flake = os.environ.get("EV_FLAKE_LEDGER") or os.path.join(os.path.dirname(led), "flake_ledger.jsonl")
    for p in (led, flake):
        if os.path.exists(p):
            sz = os.path.getsize(p)
            print("size %s = %d bytes of bound %d (%s)" % (os.path.basename(p), sz, bound, src))
            if sz * RAISE_DEN >= bound * RAISE_NUM:
                print("ledger_raise_owed: %s is at or above 75%% of its evidence-ledger bound; the raise step is a held, "
                      "reviewed, tracked item owned by ST-QA, never a silent growth" % os.path.basename(p))
    if os.path.isdir(bdir):
        sizes = {fn: os.path.getsize(os.path.join(bdir, fn)) for fn in os.listdir(bdir)
                 if re.fullmatch(r"[0-9a-f]{64}", fn) and os.path.isfile(os.path.join(bdir, fn))}
        for line in owed_relocations(sizes):
            print(line)
    return 0


# ----------------------------------------------------------------------------- rerecord / remap-refs

def _common_prefix(a, b):
    n = 0
    for x, y in zip(a, b):
        if x != y:
            break
        n += 1
    return n


def _guard_out(o):
    """rerecord --out must never write into the shared store (review I6b, 9.2): refuse when the output directory is the
    shared ledger's directory or the evidence directory, or when an output file is one of the inputs or the shared
    ledger / anchor."""
    rp = os.path.realpath
    outdir = rp(o["out"])
    shared_dirs = {rp(os.path.dirname(ledger_path()) or "."), rp(ev_dir())}
    protected = {rp(o["onto"]), rp(o["local"]), rp(ledger_path()), rp(anchor_path())}
    outs = {rp(os.path.join(outdir, o["name"])), rp(os.path.join(outdir, "ledger-seq-map.json"))}
    if outdir in shared_dirs or outs & protected:
        raise Refuse("out_is_shared_store", 70, "--out %s would write into the shared store or over an input; rerecord writes a "
                     "separate output directory only (the shared ledger is never rewritten here)" % o["out"])


def cmd_rerecord(args):
    o = {"append_only": False, "name": "ledger.jsonl"}
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--append-only":
            o["append_only"] = True
            i += 1
        elif a in ("--onto", "--local", "--out", "--name") and i + 1 < len(args):
            o[a[2:]] = args[i + 1]
            i += 2
        else:
            raise Refuse("usage_error", 64, "evrec rerecord --onto F --local F --out DIR [--append-only] [--name N]")
    if not all(k in o for k in ("onto", "local", "out")):
        raise Refuse("usage_error", 64, "evrec rerecord needs --onto, --local and --out")
    _guard_out(o)
    sides = []
    for k in ("onto", "local"):
        if o["append_only"]:
            try:
                raw, lines = read_ledger_lines(o[k])
                for l in lines:
                    json.loads(l)
            except (OSError, ValueError, Refuse) as e:
                raise Refuse("side_unverifiable", 68, "%s side: %s" % (k, e))
        else:
            try:
                chain_walk(o[k])
            except Refuse as e:
                raise Refuse("side_unverifiable", 68, "%s side rejected by verify: %s %s" % (k, e.reason, e.detail))
            raw, lines = read_ledger_lines(o[k])
        sides.append((raw, lines))
    (rraw, rlines), (_, llines) = sides
    cp = _common_prefix(rlines, llines)
    if cp == 0:
        raise Refuse("no_common_prefix", 68, "the two sides share no leading entry")
    suffix = llines[cp:]
    if o["append_only"]:
        data, seqmap = rraw + b"".join(l + b"\n" for l in suffix), None
    else:
        last = json.loads(rlines[-1])
        prev, seq = last["entry_hash"], len(rlines)
        out_lines, seqmap = [], []
        for l in suffix:
            rec = json.loads(l)
            old = rec["seq"]
            seq += 1
            rec["seq"], rec["prev_hash"] = seq, prev
            rec.pop("entry_hash", None)
            rec["entry_hash"] = compute_entry_hash(rec)
            errs = schema_errors(rec)
            if errs:
                raise Refuse("record_invalid", 65, "re-chained record seq %d: %s" % (seq, "; ".join(errs)))
            prev = rec["entry_hash"]
            out_lines.append(canon(rec) + b"\n")
            seqmap.append({"old": old, "new": seq, "digest": prev})
        data = rraw + b"".join(out_lines)
    lock = LedgerLock(os.path.join(o["out"], o["name"]), lock_timeout())
    lock.acquire()                                       # one writer per output ledger (11.4.180)
    try:
        atomic_write(os.path.join(o["out"], o["name"]), data)
        if seqmap is not None:
            atomic_write(os.path.join(o["out"], "ledger-seq-map.json"), canon(seqmap) + b"\n")
    finally:
        lock.release()
    print("rerecord: %d common, %d local entries re-recorded into %s" % (cp, len(suffix), o["out"]))
    return 0


def cmd_remap_refs(args):
    o, i = {}, 0
    while i < len(args):
        if args[i] in ("--map", "--files-from") and i + 1 < len(args):
            o[args[i][2:]] = args[i + 1]
            i += 2
        else:
            raise Refuse("usage_error", 64, "evrec remap-refs --map FILE --files-from LIST")
    if "map" not in o or "files-from" not in o:
        raise Refuse("usage_error", 64, "evrec remap-refs needs --map and --files-from")
    try:
        with open(o["map"], "rb") as f:
            m = {int(e["old"]): int(e["new"]) for e in json.loads(f.read().decode("utf-8"))}
        with open(o["files-from"], encoding="utf-8") as f:
            files = [l.rstrip("\n") for l in f if l.strip()]
    except (OSError, ValueError, KeyError, TypeError) as e:
        raise Refuse("usage_error", 64, "map or list unreadable: %s" % type(e).__name__)
    pat = re.compile(r"ledger#(\d+)\b")
    plan = []
    for fp in files:
        try:
            with open(fp, "rb") as f:
                txt = f.read().decode("utf-8")
        except OSError as e:
            raise Refuse("usage_error", 64, "%s unreadable: %s" % (fp, e.strerror))
        missing = sorted({int(x) for x in pat.findall(txt) if int(x) not in m})
        if missing:
            raise Refuse("map_incomplete", 68, "%s references ledger#%s not covered by the map; nothing written"
                         % (fp, ",".join(map(str, missing))))
        plan.append((fp, pat.sub(lambda mo: "ledger#%d" % m[int(mo.group(1))], txt)))
    for fp, new in plan:                                  # two-phase: nothing is written unless every file is covered
        atomic_write(fp, new.encode("utf-8"))
    print("remapped %d file(s)" % len(plan))
    return 0


# ----------------------------------------------------------------------------- entry points

def evrec_main(argv):
    cmds = {"run": cmd_run, "check-record": cmd_check_record, "reconcile": cmd_reconcile,
            "rerecord": cmd_rerecord, "remap-refs": cmd_remap_refs}
    try:
        if not argv or argv[0] not in cmds:
            raise Refuse("usage_error", 64, "evrec {%s} ..." % "|".join(cmds))
        return cmds[argv[0]](argv[1:])
    except Refuse as r:
        eprint("evrec: refused reason=%s %s" % (r.reason, r.detail))
        return r.code


def verify_main(argv):
    try:
        return cmd_verify(argv)
    except Refuse as r:
        print("%s: %s" % ("FAIL" if r.code == 1 else r.reason, r.detail))
        eprint("verify: reason=%s" % r.reason)
        return r.code
