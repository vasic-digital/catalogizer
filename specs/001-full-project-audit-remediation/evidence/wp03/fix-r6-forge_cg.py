#!/usr/bin/env python3
"""forge_cg.py REPO PATCH_COMMIT NEW_PARENT : rewrite the commit-graph so PATCH_COMMIT's first parent is NEW_PARENT (generation numbers and
corrected dates recomputed for the whole graph, trailer checksum recomputed). Used by the probe and by the regression test (fixture only)."""
import sys, struct, hashlib, subprocess, os
repo, pc, np_ = sys.argv[1:4]
gd = subprocess.check_output(["git","-C",repo,"rev-parse","--git-dir"]).decode().strip()
gd = gd if os.path.isabs(gd) else os.path.join(repo, gd)
path = os.path.join(gd,"objects","info","commit-graph")
b = bytearray(open(path,"rb").read())
assert b[:4]==b"CGPH" and b[4]==1 and b[5]==1
nch = b[6]
ch = {}
for i in range(nch+1):
    cid, off = struct.unpack(">4sQ", b[8+12*i:8+12*i+12]); ch[cid]=off
order = sorted(ch, key=lambda k: ch[k])
def span(cid):
    nxt = [ch[k] for k in ch if ch[k] > ch[cid]]
    return ch[cid], min(nxt)
o, e = span(b"OIDL"); n = (e-o)//20
oids = [b[o+20*i:o+20*i+20].hex() for i in range(n)]
cd, _ = span(b"CDAT")
def full(x): return subprocess.check_output(["git","-C",repo,"rev-parse",x]).decode().strip()
ip, inp = oids.index(full(pc)), oids.index(full(np_))
par = {}; date = {}
for i in range(n):
    r = cd+36*i
    p1,p2 = struct.unpack(">II", b[r+20:r+28]); gt, dt = struct.unpack(">II", b[r+28:r+36])
    par[i] = [p for p in (p1,p2) if p != 0x70000000]
    date[i] = ((gt & 3) << 32) | dt
par[ip] = [inp] + par[ip][1:]
lvl = {}; cor = {}
def walk(i):
    if i in lvl: return
    for p in par[i]: walk(p)
    lvl[i] = 1 + max([lvl[p] for p in par[i]] or [0])
    cor[i] = max([date[i]] + [cor[p]+1 for p in par[i]])
for i in range(n): walk(i)
for i in range(n):
    r = cd+36*i
    p1 = par[i][0] if par[i] else 0x70000000
    struct.pack_into(">I", b, r+20, p1)
    gt_hi = (date[i] >> 32) & 3
    struct.pack_into(">I", b, r+28, (lvl[i] << 2) | gt_hi)
if b"GDA2" in ch:
    g,_ = span(b"GDA2")
    for i in range(n):
        off = cor[i]-date[i]; assert off < 0x80000000
        struct.pack_into(">I", b, g+4*i, off)
b[-20:] = hashlib.sha1(bytes(b[:-20])).digest()
os.chmod(path,0o644); open(path,"wb").write(bytes(b))
