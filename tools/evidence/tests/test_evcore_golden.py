#!/usr/bin/env python3
"""T050 round 3 golden and unit tests of tools/evidence/evcore.py (review findings I5, I7, X5, X9, X10).

The expected values below were computed OUTSIDE evcore (jq -S -c | sha256sum, section 7 of docs/06) and pinned as
literals, so a change of the canonicalisation or of the hash construction cannot go unnoticed by agreeing with itself.
Output: `ok   name` / `FAIL name` lines like the shell tests; exit 1 on any FAIL.
"""
import atexit, hashlib, json, os, shutil, sys, tempfile, subprocess, time
sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.realpath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
for k in ("EV", "EV_LEDGER", "EV_BLOBS", "EV_ANCHOR", "EV_SCHEMA", "EVREC_REDACT_VARS", "EVREC_PROC_ROOT", "EV_TEST_SOURCES"):
    os.environ.pop(k, None)
import evcore

_TMP = []
def mkdtemp():
    """Scratch directories are removed at exit (11.4.14; review minor 3)."""
    d = tempfile.mkdtemp(prefix="evcore-golden-")
    _TMP.append(d)
    return d
atexit.register(lambda: [shutil.rmtree(d, ignore_errors=True) for d in _TMP])

n = fails = 0
def check(name, cond, detail=""):
    global n, fails
    n += 1
    if cond:
        print("ok   " + name)
    else:
        fails += 1
        print("FAIL %s %s" % (name, detail))

E = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
Z = "0" * 64
R1 = {"schema": "ev/1", "seq": 1, "item": "CAT-001", "polarity": "PROBE", "iteration": 1, "started_at": "2026-10-05T10:00:00Z",
      "cwd": "/tmp/é-ü", "argv": ["true", "a b", "\"q\""], "exit_status": 3, "verdict": "fail", "duration_ms": 7,
      "stdout_sha256": E, "stderr_sha256": E, "target_class": "shell_script", "target_ref": "x", "target_fingerprint": Z, "prev_hash": Z}
H1 = "413af748bd2ccf8e5a3b3aed5cd0c07c83adc282e212e8d34a9b7ff3bd314cc0"
R2 = {"schema": "ev/1", "seq": 2, "item": "CAT-002", "polarity": "PROBE", "iteration": 2, "started_at": "2026-10-05T10:00:01Z",
      "cwd": "/tmp/w", "argv": ["echo", "ß"], "exit_status": 0, "verdict": "pass", "duration_ms": 0,
      "stdout_sha256": E, "stderr_sha256": E, "target_class": "shell_script", "target_ref": "y", "target_fingerprint": Z, "prev_hash": H1}
H2 = "7e7f041978f670b76bac8a48f23c2ef577c6f047622230cdbff35be267308e69"

# canonical JSON: sorted keys, compact, UTF-8 not escaped
check("canon: sorted keys, compact separators, non-ASCII kept as UTF-8 bytes",
      evcore.canon({"b": 1, "a": "é", "c": [1, "x"]}) == '{"a":"é","b":1,"c":[1,"x"]}'.encode("utf-8"))
check("canon: quote and backslash escaped as in jq", evcore.canon({"a": "\"\\"}) == b'{"a":"\\"\\\\"}')
# entry hash vectors computed with jq -S -c | sha256sum
check("entry_hash golden vector 1 (genesis prev, non-ASCII cwd, exit_status 3): " + H1[:12], evcore.compute_entry_hash(R1) == H1)
check("entry_hash golden vector 2 (prev = vector 1): " + H2[:12], evcore.compute_entry_hash(R2) == H2)
for field, val in (("exit_status", 4), ("verdict", "pass"), ("item", "CAT-009"), ("cwd", "/tmp/x"), ("argv", ["true"]), ("started_at", "2026-10-05T10:00:01Z"),
                   ("duration_ms", 8), ("stdout_sha256", Z), ("stderr_sha256", Z), ("target_fingerprint", H1), ("target_ref", "z"), ("seq", 2),
                   ("prev_hash", H1), ("iteration", 2), ("polarity", "NEEDLE"), ("target_class", "go_binary")):
    r = dict(R1); r[field] = val
    check("entry_hash covers field %s (changing it changes the hash)" % field, evcore.compute_entry_hash(r) != H1)
r = dict(R1); r["entry_hash"] = "f" * 64
check("entry_hash is computed without the entry_hash field itself", evcore.compute_entry_hash(r) == H1)
check("entry_hash is not the hash of the canonical body alone (prev_hash is a hashed prefix)", hashlib.sha256(evcore.canon(R1)).hexdigest() != H1)
check("entry_hash is sha256(prev_hash ASCII || canonical body) exactly (re-derived here)", hashlib.sha256(R1["prev_hash"].encode("ascii") + evcore.canon(R1)).hexdigest() == H1)

# redaction: overlap, merge, patterns
def red(b, names):
    return evcore.redact_bytes(b, [(k, v.encode()) for k, v in names])
b, hit, *_ = red(b"x ghijklmnopqr y", [("A", "ghijklmn"), ("B", "klmnopqr")])
check("redaction: partially overlapping secrets leave no fragment of either", hit and b"ghij" not in b and b"opqr" not in b and b"klmn" not in b, repr(b))
b, hit, *_ = red(b"auth=SHORTKEYSUFFIXSECRET9", [("A", "SHORTKEY"), ("B", "SHORTKEYSUFFIXSECRET9")])
check("redaction: a secret that is a prefix of another (any value order) leaves no suffix", hit and b"SUFFIXSECRET9" not in b and b"SHORTKEY" not in b, repr(b))
b, hit, *_ = red(b"auth=SHORTKEYSUFFIXSECRET9", [("B", "SHORTKEYSUFFIXSECRET9"), ("A", "SHORTKEY")])
check("redaction: the same with the values in the other order", hit and b"SUFFIXSECRET9" not in b and b"SHORTKEY" not in b, repr(b))
b, hit, *_ = red(b"token=PLANTED123456", [("TOK", "PLANTED123456")])
check("redaction: a single named secret keeps the [REDACTED:NAME] marker", hit and b"[REDACTED:TOK" in b and b"PLANTED123456" not in b, repr(b))
b, hit, *_ = red(b"nothing to hide here, token words only", [])
check("redaction: clean bytes are returned unchanged with no hit", (not hit) and b == b"nothing to hide here, token words only", repr(b))
for label, raw in (("bearer", b"Authorization: Bearer " + b"abcDEF" + b"123456xyz789"), ("kv password", b"password=" + b"hunter2" + b"hunter2"),
                   ("url userinfo", b"https://u:" + b"pw1234" + b"5678@h.example/p"),
                   ("pem", b"-----BEGIN " + b"PRIVATE KEY-----\nMIIabc\n-----END " + b"PRIVATE KEY-----")):
    b, hit, *_ = red(raw, [])
    check("redaction: default pattern %s hit" % label, hit and b != raw, repr(b))

# lock: stale rules and release
live = subprocess.Popen(["sleep", "30"]); time.sleep(0.2)
try:
    cmd = open("/proc/%d/cmdline" % live.pid, "rb").read().replace(b"\0", b" ").decode().rstrip(" ")
    check("stale: live pid with its recorded cmdline is NOT stale", evcore._holder_stale({"pid": live.pid, "cmdline": cmd}) is False)
    check("stale: live pid with a different recorded cmdline (pid reuse) IS stale", evcore._holder_stale({"pid": live.pid, "cmdline": "other prog"}) is True)
    os.environ["EVREC_PROC_ROOT"] = mkdtemp()
    check("stale: live pid whose /proc cmdline is unreadable is NOT stale (never reaped on an unreadable identity)",
          evcore._holder_stale({"pid": live.pid, "cmdline": cmd}) is False)
    del os.environ["EVREC_PROC_ROOT"]
finally:
    live.kill(); live.wait()
check("stale: a dead pid IS stale", evcore._holder_stale({"pid": live.pid, "cmdline": "sleep 30"}) is True)
for bad in ({"pid": "x", "cmdline": "y"}, {"pid": 1, "cmdline": "init"}, {"pid": 0, "cmdline": "z"}, {"pid": 4242, "cmdline": 7}, {}):
    check("stale: unreadable holder identity %r is NOT stale" % (bad,), evcore._holder_stale(bad) is False)
d = mkdtemp(); led = os.path.join(d, "ledger.jsonl")
lk = evcore.LedgerLock(led, 2); lk.acquire()
open(led + ".lock", "w").write(json.dumps({"pid": 999999, "cmdline": "foreign"}) + "\n")
lk.release()
check("lock.release does not delete a lock that another pid now holds (X5)", os.path.exists(led + ".lock"))
os.unlink(led + ".lock")
lk2 = evcore.LedgerLock(led, 2); lk2.acquire(); lk2.release()
check("lock.release deletes the caller's own lock", not os.path.exists(led + ".lock"))
# X18: a re-chained record is validated against ev/1 (the sides are deep-verified first, so this is only reachable by a
# schema that rejects a record the verifier of the side accepted; forced here by a stand-in schema_errors)
d = mkdtemp()
os.environ["EV_BLOBS"] = os.path.join(d, "blobs"); os.environ["EV_LEDGER"] = os.path.join(d, "shared.jsonl")
def mkside(name, items):
    path = os.path.join(d, name); prev = Z; out = b""
    for i, it in enumerate(items, 1):
        r = dict(R1, seq=i, item=it, prev_hash=prev); r["entry_hash"] = evcore.compute_entry_hash(r); prev = r["entry_hash"]; out += evcore.canon(r) + b"\n"
    os.makedirs(os.path.join(d, "blobs"), exist_ok=True); open(os.path.join(os.path.join(d, "blobs"), E), "wb").close()
    open(path, "wb").write(out); return path
remote = mkside("remote.jsonl", ["CAT-001", "CAT-002"]); local = mkside("local.jsonl", ["CAT-001", "CAT-003", "CAT-004"])
outd = os.path.join(mkdtemp(), "out")        # outside the shared ledger directory (rerecord refuses --out inside it, review N1)
orig = evcore.schema_errors
evcore.schema_errors = lambda rec: ["forced re-chain refusal"] if rec.get("seq") == 4 else orig(rec)
try:
    evcore.cmd_rerecord(["--onto", remote, "--local", local, "--out", outd])
    check("X18: a re-chained record that ev/1 refuses stops rerecord (record_invalid)", False, "no refusal")
except evcore.Refuse as e:
    check("X18: a re-chained record that ev/1 refuses stops rerecord (record_invalid)", e.reason == "record_invalid" and not os.path.exists(os.path.join(outd, "ledger.jsonl")), e.reason)
finally:
    evcore.schema_errors = orig

# ---- round 4: the relative $id of the contract is made absolute in memory only (jsonschema 4.10 in the pinned image)
sch = json.load(open(evcore.schema_path()))
ps = evcore.portable_schema(sch)
check("portable_schema: the relative $id %r becomes an absolute URN" % sch.get("$id"), ps["$id"].startswith("urn:") and ps["$id"] != sch["$id"], repr(ps.get("$id")))
check("portable_schema: nothing else of the contract changes and the input is not modified", {k: v for k, v in ps.items() if k != "$id"} == {k: v for k, v in sch.items() if k != "$id"} and sch["$id"] == "ev/1")
check("portable_schema: an absolute $id is left as it is", evcore.portable_schema({"$id": "https://x.invalid/s"})["$id"] == "https://x.invalid/s")
check("portable_schema: a schema without $id is left as it is", evcore.portable_schema({"type": "object"}) == {"type": "object"})

# ---- round 4: W3-1 (EPERM holder is never reaped), W3-6 (the lock records its own real identity), redaction patterns, m4
live = subprocess.Popen(["sleep", "30"]); time.sleep(0.2)
try:
    cmd = open("/proc/%d/cmdline" % live.pid, "rb").read().replace(b"\0", b" ").decode().rstrip(" ")
    real_kill = os.kill
    def eperm(pid, sig):
        raise PermissionError(1, "Operation not permitted")
    os.kill = eperm
    try:
        check("W3-1: a holder that answers EPERM (alive, other uid) with its recorded cmdline is NOT stale", evcore._holder_stale({"pid": live.pid, "cmdline": cmd}) is False)
        os.environ["EVREC_PROC_ROOT"] = mkdtemp()
        check("W3-1: a holder that answers EPERM and whose cmdline cannot be read is NOT stale", evcore._holder_stale({"pid": live.pid, "cmdline": cmd}) is False)
        del os.environ["EVREC_PROC_ROOT"]
        check("W3-1: a holder that answers EPERM whose live cmdline differs (pid reuse) IS stale", evcore._holder_stale({"pid": live.pid, "cmdline": "other prog"}) is True)
    finally:
        os.kill = real_kill
finally:
    live.kill(); live.wait()
d = mkdtemp(); led = os.path.join(d, "ledger.jsonl")
lk = evcore.LedgerLock(led, 2); lk.acquire()
h = json.loads(open(led + ".lock").read())
mine = evcore._cmdline(os.getpid())
check("W3-6: the lock records the holder's own pid", h.get("pid") == os.getpid(), repr(h))
check("W3-6: the lock records the holder's real, non-empty cmdline (the 11.4.180 identity)", bool(mine) and h.get("cmdline") == mine, repr(h))
check("W3-6: a second reader with the same /proc does NOT find the live lock stale", evcore._holder_stale(h) is False)
lk.release()
os.environ["EVREC_PROC_ROOT"] = mkdtemp()
lk = evcore.LedgerLock(led, 2); lk.acquire()
h = json.loads(open(led + ".lock").read())
del os.environ["EVREC_PROC_ROOT"]
check("W3-6: a lock taken while /proc was unreadable records NO cmdline (never an empty string)", h.get("cmdline") in (None,) and h.get("cmdline_unreadable") is True, repr(h))
check("W3-6: and a reader with a readable /proc does not reap that live holder", evcore._holder_stale(h) is False)
dead = dict(h, pid=999999)
check("W3-6: but a holder with an unreadable-identity marker whose pid is dead IS stale", evcore._holder_stale(dead) is True)
lk.release()

# redaction: quoted JSON keys, flag pairs, header schemes, cookies, token shapes (review N2); fixtures assembled from pieces
def piece(*a): return "".join(a).encode()
SECRET = piece("Hunt", "er2", "Hunt", "er2")
for label, raw in (("json password", b'{"username":"admin","password":"' + SECRET + b'"}'),
                   ("json spaced api_key", b'{"api_key": "' + piece("AbCdEf", "123456") + b'"}'),
                   ("json single quotes", b"{'secret': '" + SECRET + b"'}"),
                   ("json value with a space", b'{"password":"' + SECRET + b' and more"}'),
                   ("flag pair", b"run --password " + SECRET + b" --x"),
                   ("basic", b"Authorization: Basic " + piece("dXNlcjpw", "YXNzd29y", "ZDEyMw==")),
                   ("cookie", b"Cookie: session=" + SECRET + b"; a=b"),
                   ("set-cookie", b"Set-Cookie: sid=" + SECRET + b"; Path=/"),
                   ("jwt", b"x " + piece("eyJhbGciOiJIUzI1NiJ9", ".eyJzdWIiOiJ4eXoifQ", ".abcDEFghi123") + b" y"),
                   ("slack", b"t " + piece("xox", "b-", "1234567890-abcdefghij") + b" t"),
                   ("stripe", b"k " + piece("sk_", "live_", "abcdefghij1234567890") + b" k")):
    b, hit, *_ = red(raw, [])
    check("redaction r4: %s is redacted with no fragment left" % label, hit and SECRET not in b and b"abcDEFghi123" not in b and b"123456" not in b and b"and more" not in b and b"abcdefghij" not in b, repr(b))
for label, raw in (("benign json", b'{"username":"admin","note":"hello world","retries":3}'), ("prose with basic", b"ok basic configuration of the tool"),
                   ("flag without credential name", b"--verbose value --name other")):
    b, hit, *_ = red(raw, [])
    check("redaction r4: %s is left unchanged" % label, (not hit) and b == raw, repr(b))
a, hit = evcore.redact_argv(["bash", "-c", ":", "--password", SECRET.decode(), "--verbose", "x", "--token=" + SECRET.decode(), "--secret-key", "S3cretV4lue"], [])
check("redact_argv: the value after a credential flag is redacted and the flag name kept", hit and "--password" in a and SECRET.decode() not in " ".join(a) and "S3cretV4lue" not in " ".join(a) and "--verbose" in a and "x" in a, repr(a))

# m4: verify without its schema is UNVERIFIED (3), not an undocumented 67
d = mkdtemp(); os.environ["EV_LEDGER"] = os.path.join(d, "l.jsonl"); os.environ["EV_BLOBS"] = os.path.join(d, "b")
open(os.environ["EV_LEDGER"], "wb").write(evcore.canon(dict(R1, entry_hash=H1)) + b"\n")
orig_sp, evcore._validator = evcore.schema_path, None
evcore.schema_path = lambda: os.path.join(d, "no-such-schema.json")
import io, contextlib
buf = io.StringIO()
with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
    rc = evcore.verify_main([])
evcore.schema_path, evcore._validator = orig_sp, None
check("m4: verify without a readable schema exits 3 (UNVERIFIED), not 67", rc == 3 and "schema_unavailable" in buf.getvalue(), "rc=%s %s" % (rc, buf.getvalue()[:120]))
sys.exit(1 if fails else 0)
