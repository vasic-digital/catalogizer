"""evcore - shared implementation of tools/evidence/evrec and tools/evidence/verify (tasks.md T050).

Reference implementation in python3 (stdlib plus the optional `jsonschema` package for the ev/1
schema check). Written test-first against tools/evidence/tests/test_evrec.sh.

Chain definition (docs/06 section 7, the ev/1 contract): entry_hash = sha256(prev_hash || canonical_json(entry
without entry_hash)); prev_hash of seq 1 is 64 zeros; canonical JSON = keys sorted, no whitespace, UTF-8
(ensure_ascii false). This is NOT continuum's construction (see evidence/wp05/T050-implementation.md, DR-E1).

Exit codes (named reasons are printed on stderr as `reason=<name>`):
  verify  0 verified, 1 chain failure, 3 unverifiable (docs/06 section 8; 2 anchor disagreement is T055)
  evrec   0 done, 64 usage_error, 65 record_invalid, 66 ledger_inconsistent, 67 schema_unavailable,
          68 side_unverifiable / no_common_prefix / map_incomplete (rerecord, remap-refs), 75 lock_held,
          76 commit_turn_held, 77 target_unreadable
"""
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
    return os.environ.get("EV_SCHEMA") or os.path.join(tool_root(), FEATURE, "contracts", "evidence-record.schema.json")


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


def schema_errors(rec):
    return sorted(e.message[:200] for e in validator().iter_errors(rec))


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

def _cmdline(pid):
    try:
        with open("/proc/%d/cmdline" % pid, "rb") as f:
            return f.read().replace(b"\0", b" ").decode("utf-8", "replace").rstrip(" ")
    except OSError:
        return None


def _holder_stale(h):
    """True only on PROVEN staleness: holder pid dead, or its real cmdline differs from the recorded one."""
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
    return cur is None or cur != cmd.rstrip(" ")


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


def commit_turn_check(allow_reap=True):
    """Refuse (commit_turn_held) while another run holds the grant. At most one reaper call, only through the
    host entry point with --exec-approved; an absent or failing reaper leaves the grant standing."""
    grant, err = _read_grant()
    if grant is None and err is None:
        return
    if err:
        raise Refuse("commit_turn_held", 76, err)
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
            raise Refuse("commit_turn_held", 76, err)
        if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):
            return
    raise Refuse("commit_turn_held", 76, "grant held by run_id %s" % grant["run_id"])


# ----------------------------------------------------------------------------- redaction

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


def redact_bytes(b, reds):
    hit = False
    for name, v in reds:
        if v in b:
            b = b.replace(v, b"[REDACTED:%s]" % name.encode())
            hit = True
    return b, hit


# ----------------------------------------------------------------------------- run

def fingerprint_target(ref):
    if os.path.isfile(ref):
        with open(ref, "rb") as f:
            return sha_bytes(f.read()), True
    if os.path.isdir(ref):
        h = hashlib.sha256()
        for root, dirs, files in os.walk(ref):
            dirs.sort()
            for fn in sorted(files):
                p = os.path.join(root, fn)
                if os.path.isfile(p):
                    with open(p, "rb") as f:
                        h.update(os.path.relpath(p, ref).encode() + b"\0" + sha_bytes(f.read()).encode() + b"\n")
        return h.hexdigest(), True
    return sha_bytes(b"UNRESOLVED:" + ref.encode()), False


def test_fingerprint(argv0, sources):
    f = argv0 if "/" in argv0 else shutil.which(argv0)
    if not f or not os.path.isfile(f) or not os.access(f, os.R_OK):
        return None
    lines = []
    for p in [f] + list(sources):
        with open(p, "rb") as fh:
            lines.append(sha_bytes(fh.read()))
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


def append_entry(rec_body):
    led = ledger_path()
    lock = LedgerLock(led, float(os.environ.get("EVREC_LOCK_TIMEOUT", "30")))
    lock.acquire()
    try:
        commit_turn_check(allow_reap=False)             # re-read inside the lock; the reaper already ran once
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
        atomic_write(led, raw + canon(rec) + b"\n", fault=True)
        return rec
    finally:
        lock.release()


def cmd_run(args):
    pos, opts, i = [], {"test_source": [], "state_go": []}, 0
    flagmap = {"--oracle": "oracle", "--evidence-class": "evidence_class", "--precondition-provenance": "pp",
               "--mutation-json": "mutation", "--container-image-digest": "digest", "--test-source": "test_source",
               "--test-state-go": "state_go"}
    boolflags = {"--closes-item": "closes", "--pre-release": "pre_release"}
    while i < len(args) and args[i] != "--":
        a = args[i]
        if a in flagmap and i + 1 < len(args):
            k = flagmap[a]
            if isinstance(opts.get(k), list):
                opts[k].append(args[i + 1])
            else:
                opts[k] = args[i + 1]
            i += 2
        elif a in boolflags:
            opts[boolflags[a]] = True
            i += 1
        else:
            pos.append(a)
            i += 1
    if len(pos) != 5 or i >= len(args) or len(args) - i < 2:
        raise Refuse("usage_error", 64, "evrec run ITEM POLARITY ITER CLASS REF [opts] -- argv...")
    item, polarity, it, tclass, ref = pos
    argv = args[i + 1:]
    if not it.isdigit() or int(it) < 1:
        raise Refuse("usage_error", 64, "ITER must be a positive integer")

    commit_turn_check(allow_reap=True)                  # before anything is written (or run)

    reds = redactions()
    sources = list(opts["test_source"]) + [s for s in os.environ.get("EV_TEST_SOURCES", "").split() if s]
    tfp = test_fingerprint(argv[0], sources)            # taken BEFORE the run
    target_fp, resolved = fingerprint_target(ref)
    if not resolved and polarity in ("RED", "GREEN", "MUTATION", "REOPEN"):
        raise Refuse("target_unreadable", 77, "target %s does not resolve; no fingerprint can be read from it" % ref)

    started = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
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

    redacted = False
    r_argv = []
    for a in argv:
        b, h = redact_bytes(a.encode("utf-8", "surrogateescape"), reds)
        redacted |= h
        r_argv.append(b.decode("utf-8", "replace") if h else a)
    out, h1 = redact_bytes(out, reds)
    err, h2 = redact_bytes(err, reds)
    redacted |= h1 or h2
    osha, esha = store_blob(out), store_blob(err)        # digest of the stored (redacted) bytes

    rec = {"schema": "ev/1", "item": item, "polarity": polarity, "iteration": int(it), "started_at": started,
           "cwd": os.getcwd(), "argv": r_argv, "exit_status": rc, "verdict": verdict_of(rc), "duration_ms": dur,
           "stdout_sha256": osha, "stderr_sha256": esha, "target_class": tclass, "target_ref": ref,
           "target_fingerprint": target_fp}
    if tfp:
        rec["test_fingerprint"] = tfp
    if redacted:
        rec["redacted"] = True
    if opts.get("oracle"):
        rec["oracle"] = {"strategy": opts["oracle"], "independent_of_sut": True}
    if opts.get("evidence_class"):
        rec["evidence_class"] = opts["evidence_class"]
    if opts.get("pp"):
        rec["precondition_provenance"] = opts["pp"]
    if opts.get("closes"):
        rec["closes_item"] = True
    if opts.get("mutation"):
        rec["mutation"] = json.loads(opts["mutation"])
    if opts.get("digest"):
        rec["container_image_digest"] = opts["digest"]
    if opts.get("pre_release"):
        rec["pre_release"] = True
    if opts["state_go"]:
        rec["test_state_gos"] = list(opts["state_go"])
    done = append_entry(rec)
    print("recorded seq=%d item=%s pol=%s it=%s exit=%d verdict=%s" % (done["seq"], item, polarity, it, rc, done["verdict"]))
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
    probe.setdefault("prev_hash", ZERO)                 # a record is checked before it is chained
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


# ----------------------------------------------------------------------------- verify (chain walk)

def chain_walk(path):
    """Return (n_entries, last_hash). Raises Refuse code 1 (chain failure) or 3 (unverifiable)."""
    try:
        with open(path, "rb") as f:
            raw = f.read()
    except OSError as e:
        raise Refuse("UNVERIFIED", 3, "ledger unreadable: %s" % e.strerror)
    if not raw:
        raise Refuse("UNVERIFIED", 3, "ledger empty: nothing to verify")
    if not raw.endswith(b"\n"):
        raise Refuse("chain_failure", 1, "truncated final line (no newline)")
    prev, n = ZERO, 0
    for line in raw.split(b"\n")[:-1]:
        n += 1
        try:
            rec = json.loads(line.decode("utf-8"))
            assert isinstance(rec, dict)
            for k in ("schema", "seq", "prev_hash", "entry_hash"):
                assert k in rec
        except Exception:
            raise Refuse("chain_failure", 1, "undecodable entry at line %d" % n)
        if rec["prev_hash"] != prev:
            raise Refuse("chain_failure", 1, "broken link at line %d (seq=%s)" % (n, rec["seq"]))
        if compute_entry_hash(rec) != rec["entry_hash"]:
            raise Refuse("chain_failure", 1, "bad hash at line %d (seq=%s)" % (n, rec["seq"]))
        if rec["seq"] != n:
            raise Refuse("chain_failure", 1, "seq %s at line %d is not contiguous" % (rec["seq"], n))
        prev = rec["entry_hash"]
    return n, prev


def ledger_bound():
    if os.environ.get("EV_LEDGER_BOUND"):
        return int(os.environ["EV_LEDGER_BOUND"]), "EV_LEDGER_BOUND"
    p = os.path.join(tool_root(), "scripts", "repo", "check_classes.tsv")
    try:
        with open(p, encoding="utf-8") as f:
            for l in f:
                c = l.rstrip("\n").split("\t")
                if len(c) >= 3 and c[0] == "evidence-ledger" and c[1] == "large_file" and c[2].isdigit():
                    return int(c[2]), "scripts/repo/check_classes.tsv"
    except OSError:
        pass
    return DEFAULT_BOUND, "built-in fallback (check_classes.tsv unreadable)"


def cmd_verify(args):
    led = ledger_path()
    if args and args[0] == "--ledger" and len(args) > 1:
        led = args[1]
    n, head = chain_walk(led)
    print("OK chain=%d entries head=%s" % (n, head[:12]))
    a = anchor_path()
    if os.path.exists(a) and os.path.getsize(a) > 0:
        print("note: anchor %s present and NOT compared (anchor check is T055, UNVERIFIED against the anchor)" % a)
    else:
        print("note: no anchor compared (T055); chain-only verification")
    bound, src = ledger_bound()
    flake = os.environ.get("EV_FLAKE_LEDGER") or os.path.join(os.path.dirname(led), "flake_ledger.jsonl")
    for p in (led, flake):
        if os.path.exists(p):
            sz = os.path.getsize(p)
            print("size %s = %d bytes of bound %d (%s)" % (os.path.basename(p), sz, bound, src))
            if sz * RAISE_DEN >= bound * RAISE_NUM:
                print("ledger_raise_owed: %s is at or above 75%% of its evidence-ledger bound; the raise step is a held, "
                      "reviewed, tracked item owned by ST-QA, never a silent growth" % os.path.basename(p))
    return 0


# ----------------------------------------------------------------------------- rerecord / remap-refs

def _common_prefix(a, b):
    n = 0
    for x, y in zip(a, b):
        if x != y:
            break
        n += 1
    return n


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
    atomic_write(os.path.join(o["out"], o["name"]), data)
    if seqmap is not None:
        atomic_write(os.path.join(o["out"], "ledger-seq-map.json"), canon(seqmap) + b"\n")
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
