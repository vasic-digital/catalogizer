#!/usr/bin/env python3
"""nas_proto_unit.py - unit oracle of scripts/test-infra/client/nas_proto.py (WP-12). Oracle strategy (11.4.245): SPECIFIED (RFC 5531/1813 XDR layouts, the owner's bounds: <= 30 listing
requests per share, depth 3, no entry name recorded, <= 1 request per second) and INVARIANT (the JSON of a walk over sentinel names contains no sentinel). A real TCP fixture speaks the ONC RPC
framing for the RPC code (portmap DUMP, NFS NULL v3 ok / v4 PROG_MISMATCH, MOUNT EXPORT, MNT ACCES); no mock of the code under test. Run: python3 -I nas_proto_unit.py  (exit 0 = all pass)"""
import importlib.util, json, os, socket, struct, sys, tempfile, threading, time

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("nas_proto", os.path.join(HERE, "..", "..", "scripts", "test-infra", "client", "nas_proto.py"))
np = importlib.util.module_from_spec(spec)
spec.loader.exec_module(np)
FAILS = 0


def check(name, got, want):
    global FAILS
    if got == want:
        print("PASS:", name)
    else:
        FAILS += 1
        print("FAIL: %s (got %r want %r)" % (name, got, want))


def xs(s):
    b = s.encode()
    return struct.pack(">I", len(b)) + b + b"\0" * ((4 - len(b) % 4) % 4)


# ---- XDR parsers
exp = b"".join([struct.pack(">I", 1) + xs("/volume1/DATA") + struct.pack(">I", 1) + xs("*") + struct.pack(">I", 0),
                struct.pack(">I", 1) + xs("/volume1/Other") + struct.pack(">I", 0)]) + struct.pack(">I", 0)
check("parse_exports: two exports, groups", np.parse_exports(exp), [("/volume1/DATA", ["*"]), ("/volume1/Other", [])])
pm = struct.pack(">IIIII", 1, 100003, 3, 6, 2049) + struct.pack(">IIIII", 1, 100005, 3, 6, 892) + struct.pack(">I", 0)
check("parse_portmap: two registrations", np.parse_portmap(pm), [(100003, 3, 6, 2049), (100005, 3, 6, 892)])
check("parse_exports: empty list", np.parse_exports(struct.pack(">I", 0)), [])

# ---- LIST / helpers
check("parse_list_line: file", np.parse_list_line("-rw-r--r--   1 owner users   12345 Jan  1 10:00 a b.mkv"), ("a b.mkv", False, 12345, "-rw-r--r--"))
check("parse_list_line: dir", np.parse_list_line("drwxr-xr-x   2 owner users    4096 Jan  1 10:00 sub")[1], True)
check("parse_list_line: symlink is skipped", np.parse_list_line("lrwxrwxrwx   1 o g 3 Jan  1 10:00 l"), None)
check("pct: p50/p95/max of 1..20", (np.pct(list(range(1, 21)), 50), np.pct(list(range(1, 21)), 95), np.pct(list(range(1, 21)), 100)), (11, 19, 20))
check("pct: empty is None", np.pct([], 50), None)
check("mask: an IPv4 address never survives", np.mask("connect to 192.0.2.7 failed"), "connect to <ip> failed")
check("ext_of", (np.ext_of("A.MKV"), np.ext_of("noext"), np.ext_of(".hidden")), ("mkv", "(none)", "(none)"))

# ---- bounded walk over a synthetic tree with sentinel names
SENT = "SENTINEL-NAME-9f3a"
calls = []


def fake_list(path):
    calls.append(path)
    depth = path.count("/") - 1
    ents = [("%s-d%d-%d" % (SENT, depth, i), True, 0, "755") for i in range(4)] + [(SENT + "-f%d.MKV" % depth, False, 2 * 1048576 + depth, "644"), ("@eaDir", True, 0, "755"), (SENT + "-small.txt", False, 100, "644")]
    return ents


np.Pacer.wait = lambda self: None          # the pacing is tested separately; the walk bounds must not cost wall-clock time
lat = {"ms": [], "errors": []}
rec, cand = np.walk_share(fake_list, "/S", np.Pacer(), lat)
check("walk: at most 30 listing requests", rec["listing_requests"], 30)
check("walk: truncated flag set when the bound stops it", rec["truncated_by_bound"], True)
check("walk: never descends into @eaDir", any("@eaDir" in c for c in calls), False)
check("walk: depth <= 3 below the share root", max(c.count("/") for c in calls) - 1 <= 3, True)
check("walk: the read candidate is the SMALLEST file >= 1 MiB", cand[1], 2 * 1048576)
check("walk: the record holds no entry name", SENT in json.dumps(rec), False)
check("walk: the extension histogram counts files", rec["ext_histogram_top25"][0]["ext"], "mkv")
rec2, cand2 = np.walk_share(lambda p: [(SENT + "x", False, 5, "644")], "/S", np.Pacer(), {"ms": [], "errors": []})
check("walk: a share with only a small file picks it (< 1 MiB class)", cand2[1], 5)
rec3, _ = np.walk_share(lambda p: (_ for _ in ()).throw(IOError("550 " + SENT)), "/S", np.Pacer(), {"ms": [], "errors": []})
check("walk: an unreadable share is a recorded result without the message (it can echo a name)", (rec3["status"], SENT in json.dumps(rec3)), ("list_error", False))
l = {"ms": [], "errors": []}
def flaky(p):
    if p == "/S":
        return [("d1", True, 0, "755"), ("d2", True, 0, "755")]
    raise IOError("550 " + SENT)
rec4, _ = np.walk_share(flaky, "/S", np.Pacer(), l)
check("walk: a failing subdirectory error keeps only the type and reply code, never the name", (rec4["list_errors"], SENT in json.dumps(rec4)), (["OSError:550"], False))

# ---- unmappable names are counted, not descended
seen = []
rec5, _ = np.walk_share(lambda p: (seen.append(p) or [("a\x7f\x7fb", True, 0, "755")]), "/S", np.Pacer(), {"ms": [], "errors": []})
check("walk: a name the server could not map is counted and not descended", (rec5["entries_with_unmappable_names_not_descended"], len(seen)), (1, 1))

# ---- read_summary
r = np.read_summary(1048576, 10.0, 10.1, 10.2, 2 * 1048576)
check("read_summary: 1 MiB in 0.2 s total = 5 MiB/s, 10 MiB/s after the first byte", (r["mib_per_s_total"], r["mib_per_s_after_first_byte"], r["content_stored"]), (5.0, 10.0, False))

# ---- Pacer: two consecutive waits are >= 0.2 s apart for gap 0.2 (the real gap is 1.0)
import importlib
spec2 = importlib.util.spec_from_file_location("nas_proto2", os.path.join(HERE, "..", "..", "scripts", "test-infra", "client", "nas_proto.py"))
np2 = importlib.util.module_from_spec(spec2); spec2.loader.exec_module(np2)
p = np2.Pacer(0.2); p.wait(); t0 = time.monotonic(); p.wait(); check("Pacer: consecutive requests are at least the gap apart", time.monotonic() - t0 >= 0.19, True)
check("Pacer: the production gap is one second", np2.Pacer().gap, 1.0)

# ---- RPC against a real TCP fixture
def reply(xid, body, accept=0, extra=b""):
    return struct.pack(">IIIIII", xid, 1, 0, 0, 0, accept) + extra + body


class Fx(threading.Thread):
    def __init__(self):
        super().__init__(daemon=True); self.s = socket.socket(); self.s.bind(("127.0.0.1", 0)); self.s.listen(8); self.port = self.s.getsockname()[1]; self.seen = []

    def run(self):
        while True:
            try:
                c, _ = self.s.accept()
            except OSError:
                return
            h = c.recv(4); n = struct.unpack(">I", h)[0] & 0x7FFFFFFF; m = b""
            while len(m) < n:
                m += c.recv(n - len(m))
            xid, _, _, prog, vers, proc = struct.unpack(">IIIIII", m[:24]); body = m[24 + 16:]
            self.seen.append((prog, vers, proc))
            if prog == 100000 and proc == 4:
                out = reply(xid, struct.pack(">IIIII", 1, 100003, 3, 6, self.port) + struct.pack(">IIIII", 1, 100005, 3, 6, self.port) + struct.pack(">I", 0))
            elif prog == 100003 and vers == 4:
                out = reply(xid, struct.pack(">II", 2, 3), accept=2)
            elif prog == 100003:
                out = reply(xid, b"")
            elif prog == 100005 and proc == 5:
                out = reply(xid, exp)
            elif prog == 100005 and proc == 1:
                out = reply(xid, struct.pack(">I", 13))
            else:
                out = reply(xid, b"", accept=1)
            c.sendall(struct.pack(">I", 0x80000000 | len(out)) + out); c.close()


fx = Fx(); fx.start()
np.PORTMAP_PORT = np.NFS_PORT = fx.port
real_sleep = time.sleep; time.sleep = lambda s: None
with tempfile.TemporaryDirectory() as td:
    open(os.path.join(td, "shares-5.txt"), "w").write("DATA20\nbad name;rm\n")
    rec, ex = np.do_rpc("5", "127.0.0.1", td)
    check("rpc: NFS v3 NULL ok, v4 PROG_MISMATCH with the supported range", (rec["nfs_versions_tcp_2049"]["v3"]["null_call"], rec["nfs_versions_tcp_2049"]["v4"]["null_call"], rec["nfs_versions_tcp_2049"]["v4"]["server_supports_range"]), ("ok", "mismatch", [2, 3]))
    check("rpc: the exports list is parsed", [e["path"] for e in rec["exports"]], ["/volume1/DATA", "/volume1/Other"])
    check("rpc: wildcard vs restricted access is recorded, not the client entries", [e["access"] for e in rec["exports"]], ["wildcard", "wildcard"])
    names = [a["mnt3_name"] for a in rec["mount_attempts"]]
    check("rpc: every MNT attempt answered ACCES, including the control path", (set(names), rec["mount_attempts"][0]["path"]), ({"ACCES"}, "(control)"))
    check("rpc: an unsafe share name in the shares file is never used for a MNT", any("bad" in a["path"] for a in rec["mount_attempts"]), False)
    check("rpc: nothing mountable -> exports file lists only the exported paths", open(os.path.join(td, "exports-5.txt")).read().split(), ["/volume1/DATA", "/volume1/Other"])
    check("rpc: only read-class procedures were called (NULL 0, DUMP 4, EXPORT 5, MNT 1)", sorted({p for _, _, p in fx.seen}), [0, 1, 4, 5])
    check("rpc: the record names the writes performed", rec["writes_performed"], 0)
time.sleep = real_sleep
fx.s.close()
print("SUMMARY fail=%d" % FAILS)
sys.exit(1 if FAILS else 0)
