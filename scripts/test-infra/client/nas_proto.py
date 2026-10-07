#!/usr/bin/env python3
"""nas_proto.py - WP-12 READ-ONLY protocol survey of ONE Synology host (runs inside IMG-GO through scripts/containers/run_pinned.sh; stdlib only).

Usage:  nas_proto.py <rpc|ftp|ftps|sftp> <index 1-7> <ip> <out dir>

  rpc   ONC RPC discovery: portmap DUMP (TCP 111), NFS NULL calls for v2/v3/v4 on 2049, MOUNT EXPORT (the exports list) -> <out>/nfs-rpc-<n>.json and <out>/exports-<n>.txt
  ftp   plain FTP: banner, FEAT, login result, MLSD availability, bounded breadth-first listing, one bounded read per share
  ftps  explicit FTPS (AUTH TLS): the same plus TLS version, cipher, certificate subject/fingerprint sha256 and the TLS versions accepted
  sftp  SFTP v3 over `ssh -s sftp` (SSH_ASKPASS, password never in argv/env value): host key fingerprints, banner, kex, subsystem, listing, one bounded read per share

Credentials: the file <out>/cred (mode 0600; line 1 user, line 2 password), created by the host script and removed by it. They never reach argv, the environment, a log or the JSON.
READ-ONLY by construction: the only FTP verbs are AUTH/PBSZ/PROT/USER/PASS/FEAT/OPTS UTF8 ON (a session option)/TYPE/CWD/PASV/EPSV/MLSD/LIST/RETR/QUIT, the only SFTP packets INIT/OPENDIR/READDIR/OPEN(read)/READ/CLOSE;
tests/infra/test_nas_protocols.sh scans this file for any write-class verb. Rate: at most one listing/read request per second per host (Pacer). Bounds per share:
at most 30 listing requests, depth 3, 2000 entries. Entry names are never recorded: counts, sha256 of the sorted top-level names, an extension histogram, latency percentiles,
the bytes/second of discarding (never storing) the first 1 MiB of ONE file per share. Host addresses are never recorded (alias only).
Output: <out>/<proto>-<n>.json (schema wp12-nas-protocol/1).
"""
import base64
import ftplib
import hashlib
import json
import os
import re
import select
import signal
import socket
import ssl
import struct
import subprocess
import sys
import time

MAX_REQ_PER_SHARE = 30
MAX_DEPTH = 3
MAX_ENTRIES = 2000
READ_BYTES = 1048576
PORTMAP_PORT, NFS_PORT = 111, 2049          # module attributes so that the unit test can point the RPC code at an unprivileged local fixture
SKIP_PREFIX = ("@", "#")          # Synology system directories (@eaDir, #recycle, @tmp): counted, never descended
IPV4 = re.compile(r"\b\d{1,3}(?:\.\d{1,3}){3}\b")


def errcode(e):
    """exception type and the 3-digit reply code only: a server message can echo an entry name"""
    m = str(e)[:3]
    return "%s:%s" % (type(e).__name__, m if m.isdigit() else "-")


def mask(text):
    return IPV4.sub("<ip>", text)


class Pacer:
    """at most one request per second (sleeps the remainder of the second since the previous request)"""

    def __init__(self, min_gap=1.0):
        self.gap, self.last = min_gap, 0.0

    def wait(self):
        d = self.last + self.gap - time.monotonic()
        if d > 0:
            time.sleep(d)
        self.last = time.monotonic()


def pct(vals, p):
    if not vals:
        return None
    s = sorted(vals)
    k = max(0, min(len(s) - 1, int(round(p / 100.0 * (len(s) - 1)))))
    return round(s[k], 1)


def lat_summary(ms):
    return {"n": len(ms), "p50_ms": pct(ms, 50), "p95_ms": pct(ms, 95), "max_ms": pct(ms, 100)}


def ext_of(name):
    return name.rsplit(".", 1)[1].lower()[:12] if "." in name.strip(".") else "(none)"


def walk_share(list_dir, root, pacer, lat):
    """breadth-first, bounded; list_dir(path) -> [(name, is_dir, size, mode_text)]. Returns the share record and a candidate file for the read sample."""
    reqs, total, files, dirs, trunc, unmappable = 0, 0, 0, 0, False, 0
    ext, modes_f, modes_d = {}, {}, {}
    queue, cand_ge, cand_lt = [(0, root)], None, None
    top_names, first = [], True
    while queue:
        depth, path = queue.pop(0)
        if reqs >= MAX_REQ_PER_SHARE or total >= MAX_ENTRIES:
            trunc = True
            break
        pacer.wait()
        t0 = time.monotonic()
        try:
            ents = list_dir(path)
        except Exception as e:                       # one unreadable directory is recorded, the walk goes on
            lat["errors"].append(errcode(e))
            reqs += 1
            if first:
                return {"status": "list_error", "error": lat["errors"][-1]}, None   # type and reply code only: a message can echo an entry name
            continue
        lat["ms"].append((time.monotonic() - t0) * 1000)
        reqs += 1
        if first:
            top_names = sorted(n for n, _, _, _ in ents)
            first = False
        for name, is_dir, size, mode in ents:
            total += 1
            if "\x7f" in name or "\ufffd" in name:           # the server could not map the name (FTP charset): it cannot be addressed, so it is counted, never descended or read
                unmappable += 1
                dirs += 1 if is_dir else 0
                files += 0 if is_dir else 1
                continue
            if is_dir:
                dirs += 1
                if mode:
                    modes_d[mode] = modes_d.get(mode, 0) + 1
                if depth < MAX_DEPTH and not name.startswith(SKIP_PREFIX):
                    queue.append((depth + 1, path.rstrip("/") + "/" + name))
            else:
                files += 1
                e = ext_of(name)
                ext[e] = ext.get(e, 0) + 1
                if mode:
                    modes_f[mode] = modes_f.get(mode, 0) + 1
                if size is not None and size > 0 and not any(p in path for p in SKIP_PREFIX):
                    fp = path.rstrip("/") + "/" + name
                    if size >= READ_BYTES and (cand_ge is None or size < cand_ge[1]):
                        cand_ge = (fp, size)
                    elif size < READ_BYTES and (cand_lt is None or size > cand_lt[1]):
                        cand_lt = (fp, size)
    rec = {"status": "listed", "list_errors": sorted(set(lat["errors"]))[:5], "top_level_entries": len(top_names),
           "top_level_names_sha256": hashlib.sha256("\n".join(top_names).encode("utf-8", "replace")).hexdigest() + "",
           "sampled_entries": total, "entries_with_unmappable_names_not_descended": unmappable, "sampled_files": files, "sampled_dirs": dirs, "listing_requests": reqs, "truncated_by_bound": trunc,
           "max_depth": MAX_DEPTH, "ext_histogram_top25": [{"ext": k, "count": v} for k, v in sorted(ext.items(), key=lambda kv: -kv[1])[:25]],
           "mode_histogram_files_top6": dict(sorted(modes_f.items(), key=lambda kv: -kv[1])[:6]),
           "mode_histogram_dirs_top6": dict(sorted(modes_d.items(), key=lambda kv: -kv[1])[:6])}
    return rec, (cand_ge or cand_lt)


def read_summary(nbytes, t_start, t_first, t_end, size):
    el = max(t_end - t_start, 1e-9)
    after = max(t_end - t_first, 1e-9) if t_first else None
    return {"bytes_read": nbytes, "file_size_class": ("ge_1MiB" if size >= READ_BYTES else "lt_1MiB"), "total_s": round(el, 3),
            "ttfb_ms": round((t_first - t_start) * 1000, 1) if t_first else None,
            "mib_per_s_total": round(nbytes / el / 1048576, 2),
            "mib_per_s_after_first_byte": round(nbytes / after / 1048576, 2) if after and nbytes else None, "content_stored": False}


# ------------------------------------------------------------------ RPC / NFS
def rpc_call(ip, port, prog, vers, proc, body=b"", timeout=8):
    xid = int(time.time() * 1000) & 0xFFFFFFFF
    msg = struct.pack(">IIIIII", xid, 0, 2, prog, vers, proc) + struct.pack(">IIII", 0, 0, 0, 0) + body
    s = socket.create_connection((ip, port), timeout)
    try:
        s.sendall(struct.pack(">I", 0x80000000 | len(msg)) + msg)
        data = b""
        while True:
            h = b""
            while len(h) < 4:
                c = s.recv(4 - len(h))
                if not c:
                    raise IOError("closed")
                h += c
            n = struct.unpack(">I", h)[0]
            last, n = n >> 31, n & 0x7FFFFFFF
            buf = b""
            while len(buf) < n:
                c = s.recv(n - len(buf))
                if not c:
                    raise IOError("closed")
                buf += c
            data += buf
            if last:
                break
    finally:
        s.close()
    _, mt, rs = struct.unpack(">III", data[:12])
    if rs != 0:
        return "denied", b""
    _, ln = struct.unpack(">II", data[12:20])
    off = 20 + ((ln + 3) & ~3)
    ast = struct.unpack(">I", data[off:off + 4])[0]
    off += 4
    if ast == 2:
        return "mismatch", struct.pack(">II", *struct.unpack(">II", data[off:off + 8]))
    if ast != 0:
        return "accept_%d" % ast, b""
    return "ok", data[off:]


def parse_portmap(b):
    out, off = [], 0
    while off + 4 <= len(b) and struct.unpack(">I", b[off:off + 4])[0] == 1:
        p, v, pr, po = struct.unpack(">IIII", b[off + 4:off + 20])
        off += 20
        out.append((p, v, pr, po))
    return out


def parse_exports(b):
    """MOUNT EXPORT reply: [follows, dirpath, [follows, group]...]* -> [(path, [groups])]"""
    out, off = [], 0

    def xstr():
        nonlocal off
        n = struct.unpack(">I", b[off:off + 4])[0]
        s = b[off + 4:off + 4 + n].decode("utf-8", "replace")
        off += 4 + ((n + 3) & ~3)
        return s
    while off + 4 <= len(b) and struct.unpack(">I", b[off:off + 4])[0] == 1:
        off += 4
        path, groups = xstr(), []
        while struct.unpack(">I", b[off:off + 4])[0] == 1:
            off += 4
            groups.append(xstr())
        off += 4
        out.append((path, groups))
    return out


def do_rpc(idx, ip, out):
    rec = {"schema": "wp12-nas-protocol/1", "alias": "Synology%s" % idx, "protocol": "nfs-rpc", "writes_performed": 0}
    t0 = time.monotonic()
    try:
        st, body = rpc_call(ip, PORTMAP_PORT, 100000, 2, 4)
    except Exception as e:
        rec.update(status="unreachable", error=mask(str(e))[:80])
        return rec, []
    rec["portmap_ms"] = round((time.monotonic() - t0) * 1000, 1)
    reg = parse_portmap(body) if st == "ok" else []
    names = {100003: "nfs", 100005: "mountd", 100021: "nlockmgr", 100024: "status", 100000: "portmap"}
    rec["portmap_registrations"] = sorted({"%s v%d %s:%d" % (names.get(p, str(p)), v, "tcp" if pr == 6 else "udp" if pr == 17 else str(pr), po) for p, v, pr, po in reg})
    mport = next((po for p, v, pr, po in reg if p == 100005 and pr == 6), None)
    nvers = {}
    for v in (2, 3, 4):
        time.sleep(1)
        try:
            t = time.monotonic()
            s, b = rpc_call(ip, NFS_PORT, 100003, v, 0)
            nvers["v%d" % v] = {"null_call": s, "rtt_ms": round((time.monotonic() - t) * 1000, 1)}
            if s == "mismatch":
                nvers["v%d" % v]["server_supports_range"] = list(struct.unpack(">II", b))
        except Exception as e:
            nvers["v%d" % v] = {"null_call": "error", "error": mask(str(e))[:60]}
    rec["nfs_versions_tcp_2049"] = nvers
    exports = []
    if mport:
        for v in (3,):
            time.sleep(1)
            try:
                t = time.monotonic()
                s, b = rpc_call(ip, mport, 100005, v, 5)
                rec["mountd"] = {"tcp_port_registered": True, "version": v, "export_call": s, "rtt_ms": round((time.monotonic() - t) * 1000, 1)}
                if s == "ok":
                    exports = parse_exports(b)
            except Exception as e:
                rec["mountd"] = {"tcp_port_registered": True, "export_call": "error", "error": mask(str(e))[:60]}
    else:
        rec["mountd"] = {"tcp_port_registered": False}
    # MNT attempts on the paths a DSM share would have (volumeN/<share> from the SMB survey) and one control path that cannot exist: when the control answers the same code, the code does not discriminate
    MNT = {0: "OK", 1: "PERM", 2: "NOENT", 5: "IO", 13: "ACCES", 20: "NOTDIR", 22: "INVAL", 63: "NAMETOOLONG", 10004: "NOTSUPP", 10006: "SERVERFAULT"}
    cands = ["/wp12-control-no-such-export"]
    try:
        shares_f = [l.strip() for l in open(os.path.join(out, "shares-%s.txt" % idx)) if re.match(r"^[A-Za-z0-9._-]{1,40}$", l.strip())]
    except OSError:
        shares_f = []
    cands += ["/volume%d" % v for v in (1, 2)] + ["/volume%d/%s" % (v, sh) for sh in shares_f for v in (1, 2, 3)]
    attempts, mounted = [], []
    if mport:
        for p in cands:
            time.sleep(1)
            enc = p.encode()
            arg = struct.pack(">I", len(enc)) + enc + b"\0" * ((4 - len(enc) % 4) % 4)
            try:
                s, b = rpc_call(ip, mport, 100005, 3, 1, arg)
                code = struct.unpack(">I", b[:4])[0] if s == "ok" else None
                attempts.append({"path": "(control)" if p.startswith("/wp12") else p, "reply": s, "mnt3_status": code, "mnt3_name": MNT.get(code, str(code))})
                if code == 0:
                    mounted.append(p)
                    time.sleep(1)
                    rpc_call(ip, mport, 100005, 3, 3, arg)   # UMNT: removes this client's entry from the server's mount list (bookkeeping only)
            except Exception as e:
                attempts.append({"path": p, "reply": "error", "error": mask(str(e))[:60]})
    rec["mount_attempts"] = attempts
    rec["mountable_paths"] = mounted
    rec["exports"] = [{"path": p, "access": ("wildcard" if any(g == "*" for g in gs) or not gs else "restricted"), "client_entries": len(gs)} for p, gs in exports]
    rec["status"] = "ok" if exports or mounted else ("no_exports_listed_all_mounts_refused" if st == "ok" and attempts else "no_exports_listed" if st == "ok" else "portmap_" + st)
    with open(os.path.join(out, "exports-%s.txt" % idx), "w") as f:
        for p in [e[0] for e in exports] + [m for m in mounted if m not in [e[0] for e in exports]]:
            f.write(p + "\n")
    return rec, exports


# ------------------------------------------------------------------ FTP / FTPS
def cert_info(der):
    info = {"cert_sha256": hashlib.sha256(der).hexdigest()}
    try:
        r = subprocess.run(["openssl", "x509", "-inform", "DER", "-noout", "-subject", "-issuer", "-dates"], input=der, capture_output=True, timeout=10)
        txt = r.stdout.decode("utf-8", "replace")
        kv = dict(l.split("=", 1) for l in txt.splitlines() if "=" in l)
        subj, iss = kv.get("subject", "").strip(), kv.get("issuer", "").strip()
        info.update(subject_sha256=hashlib.sha256(subj.encode()).hexdigest(), issuer_sha256=hashlib.sha256(iss.encode()).hexdigest(),
                    self_signed=(subj == iss and subj != ""), not_before=kv.get("notBefore", "").strip(), not_after=kv.get("notAfter", "").strip())
    except Exception as e:
        info["x509_parse_error"] = type(e).__name__
    return info


def parse_list_line(line):
    """unix-style LIST line -> (name, is_dir, size, perm) or None"""
    m = re.match(r"^([\-dlbcps][rwxsStT\-]{9})\+?\s+\d+\s+\S+\s+\S+\s+(\d+)\s+\S+\s+\S+\s+\S+\s+(.+)$", line)
    if not m:
        return None
    perm, size, name = m.group(1), int(m.group(2)), m.group(3)
    if perm[0] == "l" or name in (".", ".."):
        return None
    return name, perm[0] == "d", size, perm


def do_ftp(idx, ip, out, tls):
    proto = "ftps" if tls else "ftp"
    rec = {"schema": "wp12-nas-protocol/1", "alias": "Synology%s" % idx, "protocol": proto, "writes_performed": 0, "names_recorded": False, "content_recorded": False,
           "limits": {"max_listing_requests_per_share": MAX_REQ_PER_SHARE, "max_depth": MAX_DEPTH, "max_entries": MAX_ENTRIES, "read_sample_bytes": READ_BYTES, "max_requests_per_second": 1}}
    user, pw = open(os.path.join(out, "cred")).read().split("\n")[:2]
    pacer = Pacer()
    if tls:
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        ctx.set_ciphers("DEFAULT:@SECLEVEL=0")
        ctx.minimum_version = ssl.TLSVersion.MINIMUM_SUPPORTED
    state = {"ftp": None}

    def connect(login=True):
        pacer.wait()
        t0 = time.monotonic()
        f = ftplib.FTP_TLS(context=ctx, timeout=15) if tls else ftplib.FTP(timeout=15)
        f.encoding = "latin-1"
        f.connect(ip, 21)
        rec.setdefault("connect_ms", []).append(round((time.monotonic() - t0) * 1000, 1))
        if "banner" not in rec:
            rec["banner"] = mask(f.getwelcome())[:100]
        if tls:
            t1 = time.monotonic()
            r = f.auth()
            rec.setdefault("auth_tls_ms", []).append(round((time.monotonic() - t1) * 1000, 1))
            if "tls" not in rec:
                sk = f.sock
                rec["tls"] = {"auth_reply": r[:3], "negotiated_version": sk.version(), "cipher": sk.cipher()[0], **cert_info(sk.getpeercert(True))}
        if login:
            t2 = time.monotonic()
            f.login(user, pw)
            rec.setdefault("login_ms", []).append(round((time.monotonic() - t2) * 1000, 1))
            if tls:
                f.prot_p()
            f.voidcmd("TYPE I")
        return f
    try:
        f = connect(login=False)
    except Exception as e:
        rec.update(status="unreachable_or_refused", error=mask(type(e).__name__ + ": " + str(e))[:100])
        return rec
    # FEAT before login (some servers refuse it until the login; then it is retried after)
    feat = []
    try:
        feat = [l.strip().upper() for l in f.sendcmd("FEAT").splitlines()[1:-1]]
    except Exception:
        pass
    # login (plain FTP: a server may require TLS and answer 5xx; that is a result, not an error)
    try:
        t2 = time.monotonic()
        f.login(user, pw)
        rec["login_ms"] = [round((time.monotonic() - t2) * 1000, 1)]
        rec["login"] = "ok"
        if tls:
            f.prot_p()
        f.voidcmd("TYPE I")
    except ftplib.error_perm as e:
        rec.update(login="refused", login_reply=mask(str(e))[:100], status="login_refused")
        try:
            f.close()
        except Exception:
            pass
        return rec
    except Exception as e:
        rec.update(login="error", status="login_error", error=mask(type(e).__name__ + ": " + str(e))[:100])
        return rec
    if not feat:
        try:
            feat = [l.strip().upper() for l in f.sendcmd("FEAT").splitlines()[1:-1]]
        except Exception:
            pass
    rec["feat"] = sorted({x.split(" ")[0] for x in feat})
    if "UTF8" in rec["feat"]:
        f.encoding = "utf-8"                      # the server advertises UTF8: paths with non-ASCII names must round-trip
        try:                                      # session option only (measured: without it this server answers 0x7f for every non-ASCII name and the directory cannot be addressed)
            rec["opts_utf8_on"] = f.sendcmd("OPTS UTF8 ON")[:3]
        except Exception as e:
            rec["opts_utf8_on"] = "error:" + str(e)[:3]
    state["ftp"] = f
    if tls:
        # which TLS versions the server accepts (two extra handshakes, paced)
        acc = {}
        for name, ver in (("TLSv1.2", ssl.TLSVersion.TLSv1_2), ("TLSv1.3", ssl.TLSVersion.TLSv1_3)):
            pacer.wait()
            try:
                c2 = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
                c2.check_hostname = False
                c2.verify_mode = ssl.CERT_NONE
                c2.minimum_version = c2.maximum_version = ver
                g = ftplib.FTP_TLS(context=c2, timeout=10)
                g.connect(ip, 21)
                g.auth()
                acc[name] = True
                g.close()
            except Exception as e:
                acc[name] = False
        rec["tls"]["accepts"] = acc

    # MLSD availability
    use_mlsd = False
    pacer.wait()
    try:
        t = time.monotonic()
        list(f.mlsd("/"))
        rec["mlsd"] = {"supported": True, "root_listing_ms": round((time.monotonic() - t) * 1000, 1)}
        use_mlsd = True
    except Exception as e:
        rec["mlsd"] = {"supported": False, "error": mask(str(e))[:60]}

    def list_dir(path):
        if state["ftp"] is None:
            state["ftp"] = connect()
        g = state["ftp"]
        out_ents = []
        if use_mlsd:
            for name, facts in g.mlsd(path):
                ty = facts.get("type", "")
                if ty in ("cdir", "pdir") or name in (".", ".."):
                    continue
                if ty.startswith("OS.unix=slink") or ty == "slink":
                    continue
                perm = facts.get("unix.mode") or facts.get("perm")
                out_ents.append((name, ty == "dir", int(facts["size"]) if facts.get("size", "").isdigit() else None, perm))
        else:
            lines = []
            g.cwd(path)
            g.retrlines("LIST", lines.append)
            for l in lines:
                p = parse_list_line(l)
                if p:
                    out_ents.append(p)
        return out_ents

    def read_sample(fp, size):
        g = state["ftp"]
        t0 = time.monotonic()
        t_first, n = None, 0
        conn = g.transfercmd("RETR " + fp)
        try:
            while n < READ_BYTES:
                chunk = conn.recv(min(65536, READ_BYTES - n))
                if not chunk:
                    break
                if t_first is None:
                    t_first = time.monotonic()
                n += len(chunk)
        finally:
            t_end = time.monotonic()
            try:
                conn.close()
            except Exception:
                pass
        state["ftp"] = None                      # an abandoned transfer leaves the control channel undefined: drop it, reconnect when needed
        try:
            g.close()
        except Exception:
            pass
        return read_summary(n, t0, t_first, t_end, size)

    lat = {"ms": [], "errors": []}
    pacer.wait()
    try:
        t = time.monotonic()
        root = list_dir("/")
        lat["ms"].append((time.monotonic() - t) * 1000)
    except Exception as e:
        rec.update(status="root_listing_failed", error=mask(type(e).__name__ + ": " + str(e))[:100])
        return rec
    shares, all_ms = [], list(lat["ms"])
    tops = [n for n, d, _, _ in root if d]
    rec["root_entries"] = len(root)
    rec["root_names_sha256"] = hashlib.sha256("\n".join(sorted(n for n, _, _, _ in root)).encode("utf-8", "replace")).hexdigest()
    for name in sorted(tops):
        if name.startswith(SKIP_PREFIX) or not re.match(r"^[A-Za-z0-9._ -]{1,64}$", name):
            continue
        slat = {"ms": [], "errors": []}
        try:
            srec, cand = walk_share(list_dir, "/" + name, pacer, slat)
        except Exception as e:
            srec, cand = {"status": "walk_error", "error": type(e).__name__}, None
        srec["share"] = name
        srec["listing_latency"] = lat_summary(slat["ms"])
        all_ms += slat["ms"]
        if srec.get("status") == "listed":
            if cand:
                pacer.wait()
                try:
                    srec["read_sample"] = read_summary_wrap(read_sample, cand)
                except Exception as e:
                    srec["read_sample"] = {"status": "read_error", "error": errcode(e)}
            else:
                srec["read_sample"] = {"status": "no_file_in_sample"}
        shares.append(srec)
    rec["shares"] = shares
    rec["listing_latency_all"] = lat_summary(all_ms)
    rec["status"] = "ok"
    try:
        if state["ftp"]:
            state["ftp"].quit()
    except Exception:
        pass
    return rec


def read_summary_wrap(fn, cand):
    r = fn(cand[0], cand[1])
    r["status"] = "ok"
    return r


# ------------------------------------------------------------------ SFTP (protocol v3 over `ssh -s sftp`)
class Sftp:
    def __init__(self, proc):
        self.p, self.rid = proc, 0

    def _read(self, n):
        buf = b""
        while len(buf) < n:
            r, _, _ = select.select([self.p.stdout], [], [], 30)
            if not r:
                raise TimeoutError("sftp read timeout")
            c = os.read(self.p.stdout.fileno(), n - len(buf))
            if not c:
                raise IOError("sftp channel closed")
            buf += c
        return buf

    def send(self, typ, payload):
        data = bytes([typ]) + payload
        self.p.stdin.write(struct.pack(">I", len(data)) + data)
        self.p.stdin.flush()

    def recv(self):
        n = struct.unpack(">I", self._read(4))[0]
        d = self._read(n)
        return d[0], d[1:]

    def req(self, typ, payload):
        self.rid += 1
        self.send(typ, struct.pack(">I", self.rid) + payload)
        t, d = self.recv()
        return t, d[4:]


def s_str(b):
    return struct.pack(">I", len(b)) + b


def parse_attrs(d, off):
    flags = struct.unpack(">I", d[off:off + 4])[0]
    off += 4
    size = perm = None
    if flags & 1:
        size = struct.unpack(">Q", d[off:off + 8])[0]
        off += 8
    if flags & 2:
        off += 8
    if flags & 4:
        perm = struct.unpack(">I", d[off:off + 4])[0]
        off += 4
    if flags & 8:
        off += 8
    if flags & 0x80000000:
        cnt = struct.unpack(">I", d[off:off + 4])[0]
        off += 4
        for _ in range(cnt):
            for _ in range(2):
                ln = struct.unpack(">I", d[off:off + 4])[0]
                off += 4 + ln
    return size, perm, off


def do_sftp(idx, ip, out):
    rec = {"schema": "wp12-nas-protocol/1", "alias": "Synology%s" % idx, "protocol": "sftp", "writes_performed": 0, "names_recorded": False, "content_recorded": False,
           "limits": {"max_listing_requests_per_share": MAX_REQ_PER_SHARE, "max_depth": MAX_DEPTH, "max_entries": MAX_ENTRIES, "read_sample_bytes": READ_BYTES, "max_requests_per_second": 1}}
    user, pw = open(os.path.join(out, "cred")).read().split("\n")[:2]
    pacer = Pacer()
    # host keys (public fingerprints; ssh-keyscan sends no credentials)
    keys = []
    try:
        r = subprocess.run(["ssh-keyscan", "-T", "8", "-t", "rsa,ecdsa,ed25519", ip], capture_output=True, timeout=40)
        for l in r.stdout.decode().splitlines():
            parts = l.split()
            if len(parts) == 3 and not l.startswith("#"):
                blob = base64.b64decode(parts[2])
                keys.append({"type": parts[1], "fingerprint_sha256": "SHA256:" + base64.b64encode(hashlib.sha256(blob).digest()).decode().rstrip("=")})
    except Exception as e:
        rec["hostkey_error"] = type(e).__name__
    rec["host_keys"] = sorted(keys, key=lambda k: k["type"])
    cfg = os.path.join(out, "ssh_config")
    askp = os.path.join(out, "askpass")
    pwf = os.path.join(out, "cred")
    with open(os.open(cfg, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600), "w") as f:
        f.write("Host target\n  HostName %s\n  User %s\n" % (ip, user))
    with open(os.open(askp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o700), "w") as f:
        f.write("#!/bin/sh\nsed -n 2p '%s'\n" % pwf)
    env = {"PATH": os.environ.get("PATH", "/usr/bin:/bin"), "HOME": out, "SSH_ASKPASS": askp, "SSH_ASKPASS_REQUIRE": "force", "DISPLAY": ":0"}
    errf = os.path.join(out, "ssh_stderr")
    pacer.wait()
    t0 = time.monotonic()
    ef = open(errf, "wb")
    proc = subprocess.Popen(["ssh", "-v", "-F", cfg, "-o", "PreferredAuthentications=password,keyboard-interactive", "-o", "PubkeyAuthentication=no", "-o", "StrictHostKeyChecking=no",
                             "-o", "UserKnownHostsFile=/dev/null", "-o", "ConnectTimeout=10", "-o", "NumberOfPasswordPrompts=1", "-s", "target", "sftp"],
                            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=ef, env=env, close_fds=True)
    signal.alarm(900)

    def verbose_facts():
        facts = {}
        try:
            for l in open(errf, "r", errors="replace").read().splitlines():
                m = re.search(r"remote software version (\S+)", l)
                if m:
                    facts["server_software"] = m.group(1)
                m = re.search(r"kex: algorithm: (\S+)", l)
                if m:
                    facts["kex_algorithm"] = m.group(1)
                m = re.search(r"kex: server->client cipher: (\S+)", l)
                if m:
                    facts["cipher"] = m.group(1)
                m = re.search(r"Authenticated to \S+ .*using \"(\S+)\"", l) or re.search(r"Authentication succeeded \((\S+)\)", l)
                if m:
                    facts["auth_method"] = m.group(1)
                m = re.search(r"Server host key: (\S+) (SHA256:\S+)", l)
                if m:
                    facts["negotiated_host_key"] = {"type": m.group(1), "fingerprint_sha256": m.group(2)}
                if "Permission denied" in l:
                    facts["auth_denied"] = True
                if "Connection refused" in l:
                    facts["connect"] = "refused"
                elif "Connection timed out" in l or "timed out" in l:
                    facts["connect"] = "timed_out"
                elif "No route to host" in l:
                    facts["connect"] = "no_route"
        except Exception:
            pass
        return facts
    try:
        sf = Sftp(proc)
        sf.send(1, struct.pack(">I", 3))
        t, d = sf.recv()
        rec["connect_login_subsystem_ms"] = round((time.monotonic() - t0) * 1000, 1)
        if t != 2:
            raise IOError("no SFTP version packet (type %d)" % t)
        rec["sftp_protocol_version"] = struct.unpack(">I", d[:4])[0]
        rec["subsystem_sftp"] = "ok"
    except Exception as e:
        time.sleep(0.5)
        rec.update(status="session_failed", error=mask(type(e).__name__ + ": " + str(e))[:100], ssh=verbose_facts())
        try:
            proc.kill()
        except Exception:
            pass
        return rec
    rec["ssh"] = verbose_facts()

    def opendir(path):
        t, d = sf.req(11, s_str(path.encode()))
        if t != 102:
            raise IOError("opendir status %s" % (struct.unpack(">I", d[:4])[0] if t == 101 else t))
        return d[4:4 + struct.unpack(">I", d[:4])[0]]

    def list_dir(path):
        h = opendir(path)
        ents = []
        try:
            while True:
                t, d = sf.req(12, s_str(h))
                if t == 101:
                    break                       # SSH_FX_EOF
                if t != 104:
                    raise IOError("readdir type %d" % t)
                cnt, off = struct.unpack(">I", d[:4])[0], 4
                for _ in range(cnt):
                    ln = struct.unpack(">I", d[off:off + 4])[0]
                    name = d[off + 4:off + 4 + ln].decode("utf-8", "replace")
                    off += 4 + ln
                    ll = struct.unpack(">I", d[off:off + 4])[0]
                    off += 4 + ll
                    size, perm, off = parse_attrs(d, off)
                    if name in (".", ".."):
                        continue
                    kind = (perm or 0) & 0o170000
                    if kind == 0o120000:
                        continue
                    ents.append((name, kind == 0o040000, size, ("%o" % (perm & 0o7777)) if perm is not None else None))
        finally:
            sf.req(4, s_str(h))
        return ents

    def read_sample(fp, size):
        t0 = time.monotonic()
        t, d = sf.req(3, s_str(fp.encode()) + struct.pack(">I", 1) + struct.pack(">I", 0))   # SSH_FXF_READ only, empty attrs
        if t != 102:
            raise IOError("open(read) refused status %s" % (struct.unpack(">I", d[:4])[0] if t == 101 else t))
        h = d[4:4 + struct.unpack(">I", d[:4])[0]]
        n, t_first = 0, None
        try:
            while n < READ_BYTES:
                t, d = sf.req(5, s_str(h) + struct.pack(">QI", n, min(32768, READ_BYTES - n)))
                if t != 103:
                    break
                c = struct.unpack(">I", d[:4])[0]
                if t_first is None:
                    t_first = time.monotonic()
                n += c
        finally:
            t_end = time.monotonic()
            sf.req(4, s_str(h))
        return read_summary(n, t0, t_first, t_end, size)

    lat_all = []
    pacer.wait()
    try:
        t = time.monotonic()
        root = list_dir("/")
        lat_all.append((time.monotonic() - t) * 1000)
    except Exception as e:
        rec.update(status="root_listing_failed", error=mask(type(e).__name__ + ": " + str(e))[:100])
        proc.kill()
        return rec
    rec["root_entries"] = len(root)
    rec["root_names_sha256"] = hashlib.sha256("\n".join(sorted(n for n, _, _, _ in root)).encode("utf-8", "replace")).hexdigest()
    rec["root_mode_histogram_top6"] = {}
    for _, _, _, m in root:
        if m:
            rec["root_mode_histogram_top6"][m] = rec["root_mode_histogram_top6"].get(m, 0) + 1
    shares = []
    for name in sorted(n for n, dd, _, _ in root if dd):
        if name.startswith(SKIP_PREFIX) or not re.match(r"^[A-Za-z0-9._ -]{1,64}$", name):
            continue
        slat = {"ms": [], "errors": []}
        try:
            srec, cand = walk_share(list_dir, "/" + name, pacer, slat)
        except Exception as e:
            srec, cand = {"status": "walk_error", "error": type(e).__name__}, None
        srec["share"] = name
        srec["listing_latency"] = lat_summary(slat["ms"])
        lat_all += slat["ms"]
        if srec.get("status") == "listed":
            if cand:
                pacer.wait()
                try:
                    srec["read_sample"] = read_summary_wrap(read_sample, cand)
                except Exception as e:
                    srec["read_sample"] = {"status": "read_error", "error": errcode(e)}
            else:
                srec["read_sample"] = {"status": "no_file_in_sample"}
        shares.append(srec)
    rec["shares"] = shares
    rec["listing_latency_all"] = lat_summary(lat_all)
    rec["status"] = "ok"
    try:
        proc.stdin.close()
        proc.wait(timeout=5)
    except Exception:
        proc.kill()
    return rec


def main():
    if len(sys.argv) != 5 or sys.argv[1] not in ("rpc", "ftp", "ftps", "sftp") or not re.match(r"^[1-7]$", sys.argv[2]) or not re.match(r"^\d{1,3}(\.\d{1,3}){3}$", sys.argv[3]):
        print(__doc__.split("\n\n")[0], file=sys.stderr)
        return 2
    proto, idx, ip, out = sys.argv[1:5]
    signal.signal(signal.SIGALRM, lambda *_: (_ for _ in ()).throw(TimeoutError("overall time budget")))
    signal.alarm(1500)
    if proto == "rpc":
        rec, _ = do_rpc(idx, ip, out)
        name = "nfs-rpc-%s.json" % idx
    elif proto in ("ftp", "ftps"):
        rec, name = do_ftp(idx, ip, out, proto == "ftps"), "%s-%s.json" % (proto, idx)
    else:
        rec, name = do_sftp(idx, ip, out), "sftp-%s.json" % idx
    rec["measured_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    with open(os.path.join(out, name), "w") as f:
        json.dump(rec, f, indent=1, sort_keys=True)
        f.write("\n")
    print("%s host=%s status=%s -> %s" % (proto, idx, rec.get("status"), name))
    return 0


if __name__ == "__main__":
    sys.exit(main())
