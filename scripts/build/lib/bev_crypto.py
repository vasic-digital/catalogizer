#!/usr/bin/env python3
"""bev_crypto.py - strict parse, validation, HKDF-SHA256 per-build key and HMAC-SHA256 verification for build-event/1
(T005b slice). The driver secret is read from a file and never passed on a command line (no ps exposure).
Commands:
  derive <statedir> <build_id> <run_id> <checkout>      -> per-build key (hex) on stdout (hub-to-stdin use only)
  verify-event <statedir> <builds_root> <checkout>      reads ONE event from stdin (a single read; the bytes verified are
                                                        the bytes acted on) and prints, line 1 a status
                                                        ok | other_run | bad | unknown_build | malformed:<why> | secret:<why>,
                                                        and for ok line 2 = the verified event as canonical JSON (with hmac),
                                                        line 3 = sha256 of the canonical bytes without hmac.
  sign <statedir> <build_id> <run_id> <json> <checkout> -> signed event JSON (test/emitter helper)
  hkdf-selftest                                         -> RFC 5869 test cases 1 and 3 (the empty-salt branch used in
                                                        production), exit 0 on match
Canonical bytes: the event without 'hmac', JSON keys sorted by code point, compact separators, ensure_ascii=False,
UTF-8, integers in decimal (floats never occur: they are refused). See docs/scripts/event_core.md.
Info = len4be(build_id) || build_id || len4be(run_id) || run_id (UTF-8 byte lengths)."""
import hashlib, hmac, json, os, re, stat, struct, sys

MAX_INT = 2 ** 53 - 1          # integers above this are not exactly representable by every JSON consumer
MAX_EVENT_BYTES = 65536
BUILD_ID_RE = re.compile(r"^[A-Za-z0-9_-]{1,128}$")   # no '.', so '.' and '..' can never name a build directory
HEX64 = re.compile(r"^[0-9a-f]{64}$")
KEY_HEX_LEN = 64               # the driver secret is exactly 32 bytes (openssl rand -hex 32)
STR_MAX = 1024                 # host, sent_at, image_digest: build-event.schema.json carries the same maxLength
VARIANTS = ("primary", "repro-cold")
EXIT_CLASSES = {"succeeded", "build_failed", "test_failed", "infra_failed", "cancelled"}
COMMON = {"schema", "run_id", "build_id", "variant", "seq", "kind", "host", "sent_at", "hmac"}
HB = {"progress_offset", "stage", "elapsed_monotonic_ms"}
DONE = {"exit_class", "artifact_manifest_sha256", "image_digest", "remote_log_sha256", "peak_rss_bytes"}


class SecretError(Exception):
    pass


class Malformed(Exception):
    pass


class Unauth(Exception):
    pass


def hkdf(secret, info, n=32, salt=b""):
    prk = hmac.new(salt or b"\x00" * 32, secret, hashlib.sha256).digest()
    okm, t, i = b"", b"", 1
    while len(okm) < n:
        t = hmac.new(prk, t + info + bytes([i]), hashlib.sha256).digest(); okm += t; i += 1
    return okm[:n]


def enc(s):
    b = s.encode("utf-8"); return struct.pack(">I", len(b)) + b


def secret(statedir, checkout=None):
    """Read the driver secret. Refuses (SecretError) when the state directory or key file is unsafe: a symlinked state
    directory or key, a state directory inside the checkout, a key that is not a regular file owned by this user with no
    group/other permission bits, a key whose sha256 differs from the one recorded in build_hmac.key.meta (or no record)."""
    if os.path.islink(statedir):
        raise SecretError("state_dir_symlink")
    real = os.path.realpath(statedir)
    if checkout is not None:
        co = os.path.realpath(checkout)
        if real == co or real.startswith(co + os.sep):
            raise SecretError("state_dir_inside_checkout")
    p = os.path.join(statedir, "build_hmac.key")
    try:
        st = os.lstat(p)
    except OSError:
        raise SecretError("key_absent")
    if stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode):
        raise SecretError("key_not_regular_file")
    if st.st_uid != os.geteuid():
        raise SecretError("key_wrong_owner")
    if st.st_mode & 0o077:
        raise SecretError("key_mode_not_0600")
    try:
        with open(p, "rb") as fh:
            raw = fh.read()
    except OSError:
        raise SecretError("key_unreadable")      # a read error is a secret problem, never an event defect
    want = None
    try:
        for line in open(p + ".meta", encoding="utf-8"):
            if line.startswith("sha256="):
                want = line.strip().split("=", 1)[1]
    except OSError:
        pass
    if want is None:
        raise SecretError("meta_absent")
    if not hmac.compare_digest(want, hashlib.sha256(raw).hexdigest()):
        raise SecretError("meta_sha_mismatch")
    try:
        txt = raw.decode("ascii").strip()
    except UnicodeDecodeError:
        raise SecretError("key_not_hex")
    if not re.fullmatch(r"[0-9a-f]*", txt):
        raise SecretError("key_not_hex")
    if len(txt) != KEY_HEX_LEN:
        raise SecretError("key_bad_length")      # an empty or short key would make every event forgeable
    return bytes.fromhex(txt)


def key(statedir, build_id, run_id, checkout=None):
    return hkdf(secret(statedir, checkout), enc(build_id) + enc(run_id))


def canon(ev):
    e = dict(ev); e.pop("hmac", None)
    return json.dumps(e, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def mac(k, ev):
    return hmac.new(k, canon(ev), hashlib.sha256).hexdigest()


def _safe(s, n=64):
    """Text taken from an unauthenticated event, made safe to print: ASCII printable only, everything else as a \\u escape, at most n characters."""
    out = "".join(c if 32 <= ord(c) < 127 else ("\\u%04x" % ord(c) if ord(c) < 0x10000 else "\\U%08x" % ord(c)) for c in s[:n])
    return out + ("..." if len(s) > n else "")


def _no_dups(pairs):
    d = {}
    for k, v in pairs:
        if k in d:
            raise Malformed("duplicate key " + _safe(k))
        d[k] = v
    return d


def _nan(c):
    raise Malformed("non-finite number")


def _isint(v, lo, hi=MAX_INT):
    return type(v) is int and lo <= v <= hi


def _str(v, lo=1, hi=STR_MAX):
    return type(v) is str and lo <= len(v) <= hi


def _hex64(v):
    return type(v) is str and HEX64.fullmatch(v) is not None     # a JSON number of 64 digits is NOT a hash (the schema says type string)


def parse_validate(raw):
    """One strict parse of the bytes; returns the event dict or raises Malformed. Applies the same rules as
    build-event.schema.json (types, enums, required fields per kind, no extra fields, string lengths, integer bounds) plus
    what a schema cannot state: the 64 KiB cap on the raw event, strict duplicate-key and non-finite-number refusal."""
    if len(raw) > MAX_EVENT_BYTES:
        raise Malformed("too large")
    try:
        ev = json.loads(raw.decode("utf-8"), object_pairs_hook=_no_dups, parse_constant=_nan)
    except Malformed:
        raise
    except (ValueError, UnicodeDecodeError, RecursionError):
        raise Malformed("not JSON")
    if type(ev) is not dict:
        raise Malformed("not an object")
    if not (type(ev.get("hmac")) is str and HEX64.fullmatch(ev["hmac"])):
        raise Unauth("hmac missing or not 64 lowercase hex")
    kind = ev.get("kind")
    if kind not in ("accepted", "heartbeat", "completed"):
        raise Malformed("kind")
    allowed = COMMON | (HB if kind == "heartbeat" else DONE if kind == "completed" else set())
    if not COMMON <= set(ev):
        raise Malformed("missing common fields")
    if set(ev) - allowed:
        raise Malformed("unexpected field " + _safe(sorted(set(ev) - allowed)[0]))
    if ev["schema"] != "build-event/1":
        raise Malformed("schema")
    if not (type(ev["build_id"]) is str and BUILD_ID_RE.fullmatch(ev["build_id"])):
        raise Malformed("build_id")
    if not _str(ev["run_id"], 1, 128) or not _str(ev["host"]) or not _str(ev["sent_at"]):
        raise Malformed("run_id/host/sent_at")
    if ev["variant"] not in VARIANTS:
        raise Malformed("variant")
    if not _isint(ev["seq"], 1):
        raise Malformed("seq")
    if kind == "heartbeat":
        if not all(f in ev and _isint(ev[f], 0) for f in HB):
            raise Malformed("heartbeat fields")
    if kind == "completed":
        if not (ev.get("exit_class") in EXIT_CLASSES and _hex64(ev.get("artifact_manifest_sha256"))
                and _str(ev.get("image_digest")) and _hex64(ev.get("remote_log_sha256"))):
            raise Malformed("completed fields")
        if "peak_rss_bytes" in ev and not _isint(ev["peak_rss_bytes"], 0):
            raise Malformed("peak_rss_bytes")
    return ev


def verify_event(statedir, builds_root, checkout):
    ev = parse_validate(sys.stdin.buffer.read(MAX_EVENT_BYTES + 1))
    b = ev["build_id"]
    try:
        sub = json.load(open(os.path.join(builds_root, b, "submit.json"), encoding="utf-8"))
        exp_run = sub["run_id"]
        exp_variant = sub["variant"]
        if not _str(exp_run, 1, 128) or exp_variant not in VARIANTS:
            raise ValueError
    except OSError:
        print("unknown_build"); return
    except (ValueError, KeyError, TypeError):
        print("unknown_build"); return
    got = ev["hmac"]
    # the event is bound to the build's submit record: the run_id and variant it carries must be the recorded ones
    if ev["run_id"] != exp_run:
        if hmac.compare_digest(got, mac(key(statedir, b, ev["run_id"], checkout), ev)):
            print("other_run")                       # a correctly signed event of an earlier run of this build id
        elif hmac.compare_digest(got, mac(key(statedir, b, exp_run, checkout), ev)):
            print("malformed:run_id differs from submit.json")
        else:
            print("bad")
        return
    if not hmac.compare_digest(got, mac(key(statedir, b, exp_run, checkout), ev)):
        print("bad"); return
    if ev["variant"] != exp_variant:
        print("malformed:variant differs from submit.json"); return
    print("ok")
    print(json.dumps(ev, sort_keys=True, separators=(",", ":"), ensure_ascii=False))
    print(hashlib.sha256(canon(ev)).hexdigest())


def main(a):
    c = a[0]
    if c == "derive":
        print(key(a[1], a[2], a[3], a[4]).hex())     # the checkout argument is mandatory (IndexError -> exit 2)
    elif c == "verify-event":
        try:
            verify_event(a[1], a[2], a[3])
        except Malformed as e:
            print("malformed:" + str(e))
        except Unauth:
            print("bad")
        except SecretError as e:
            print("secret:" + str(e))
    elif c == "sign":
        ev = json.loads(a[4]); ev["hmac"] = mac(key(a[1], a[2], a[3], a[5]), ev); print(json.dumps(ev))
    elif c == "hkdf-selftest":
        ikm = bytes.fromhex("0b" * 22); salt = bytes.fromhex("000102030405060708090a0b0c")
        info = bytes.fromhex("f0f1f2f3f4f5f6f7f8f9")
        want1 = "3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865"
        want3 = "8da4e775a563c18f715f802a063c5a31b8a11f5c5ee1879ec3454e5f3c738d2d9d201395faa4b61a96c8"
        ok1 = hkdf(ikm, info, 42, salt).hex() == want1
        ok3 = hkdf(ikm, b"", 42, b"").hex() == want3          # RFC 5869 test case 3: empty salt, empty info
        sys.exit(0 if ok1 and ok3 else 1)
    else:
        sys.exit(2)


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except SecretError as e:
        print("REFUSED reason=secret_unusable " + str(e), file=sys.stderr); sys.exit(20)
    except (KeyError, ValueError, OSError, json.JSONDecodeError, IndexError) as e:
        print(f"error: {e}", file=sys.stderr); sys.exit(2)
