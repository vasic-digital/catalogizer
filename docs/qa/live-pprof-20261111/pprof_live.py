#!/usr/bin/env python3
"""Live pprof protection checks against QA stack 3. usage: pprof_live.py <phase: off|on> <round>
Creds only from the 0600 qa.env (read in-process, never printed). Non-admin user is registered with a run-time random password and deleted at the end."""
import sys, os, json, time, hashlib, secrets, threading, urllib.request, urllib.error
PH, R = sys.argv[1], sys.argv[2]
QA = os.path.expanduser("~/.cache/claude-tmp/catqa3")
E = {}
for l in open(QA + "/qa.env"):
    if "=" in l and not l.startswith("#"):
        k, v = l.rstrip("\n").split("=", 1); E[k] = v
B = os.environ.get("API", "http://192.168.1.17:38082")
def raw(m, p, hdr=None, body=None, timeout=120):
    t0 = time.monotonic()
    r = urllib.request.Request(B + p, data=body, method=m, headers=hdr or {})
    try:
        x = urllib.request.urlopen(r, timeout=timeout); s, hd, b, ck = x.status, dict(x.headers), x.read(), x.headers.get_all('Set-Cookie') or []
    except urllib.error.HTTPError as e:
        s, hd, b, ck = e.code, dict(e.headers), e.read(), e.headers.get_all('Set-Cookie') or []
    return s, hd, b, ck, time.monotonic() - t0
def J(m, p, body=None, tok=None):
    h = {"Content-Type": "application/json"}
    if tok: h["Authorization"] = "Bearer " + tok
    s, hd, b, ck, d = raw(m, p, h, json.dumps(body).encode() if body is not None else None)
    try: return s, json.loads(b)
    except Exception: return s, {}
res = []
def chk(n, ok, d=None):
    r = {"phase": PH, "round": R, "check": n, "result": "PASS" if ok else "FAIL", "detail": d}
    res.append(r); print(json.dumps(r, default=str)[:500], flush=True)
def sha(b): return hashlib.sha256(b).hexdigest()
s, j = J("POST", "/api/v1/auth/login", {"username": E["ADMIN_USERNAME"], "password": E["ADMIN_PASSWORD"]})
tok = j["session_token"]
AH = {"Authorization": "Bearer " + tok}
H = J("GET", "/health")[1]; fp = H.get("build_number")
chk("env.health_fingerprint", H.get("status") == "healthy", fp)
if PH == "off":
    for p in ("/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/heap"):
        s = raw("GET", p, AH)[0]; chk("RED.flag_unset_admin_404 " + p, s == 404, s)
    json.dump(res, open("res_%s_%s.json" % (PH, R), "w"), default=str); sys.exit(0)
# --- user setup
_, h, _, ck, _ = raw("GET", "/api/v1/admin/users?limit=1", AH)
cv = [x for x in ck if x.startswith("csrf-dev=")][0].split(";")[0]; xt = h.get("X-Csrf-Token") or h.get("X-CSRF-Token")
un = "pprofq_%d_%s" % (int(time.time()), secrets.token_hex(2)); pw = "Rq-" + secrets.token_urlsafe(18) + "-9z"
sr, b = J("POST", "/api/v1/auth/register", {"username": un, "email": un + "@example.test", "password": pw, "first_name": "P", "last_name": "Q"})
uid = (b.get("user") or b).get("id")
pa = raw("PUT", "/api/v1/admin/users/%s" % uid, {**AH, "Content-Type": "application/json", "Cookie": cv, "X-CSRF-Token": xt}, json.dumps({"status": "active", "role": "user"}).encode())[0]
ut = J("POST", "/api/v1/auth/login", {"username": un, "password": pw})[1].get("session_token")
chk("setup.nonadmin_user_active", bool(uid) and pa == 200 and bool(ut), {"register": sr, "approve": pa})
UH = {"Authorization": "Bearer " + ut}
try:
    paths = ("/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/heap", "/debug/pprof/goroutine?debug=1", "/debug/pprof/profile?seconds=1", "/debug/pprof/trace?seconds=1")
    for p in paths:
        chk("anon_401 " + p, raw("GET", p)[0] == 401, raw("GET", p)[0])
    chk("scrape_token_rejected_401", raw("GET", "/debug/pprof/cmdline", {"Authorization": "Bearer " + E["METRICS_SCRAPE_TOKEN"]})[0] == 401)
    for p in paths:
        s = raw("GET", p, UH)[0]; chk("nonadmin_403 " + p, s == 403, s)
    for p in ("/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/heap", "/debug/pprof/goroutine?debug=1"):
        s, hd, b, _, d = raw("GET", p, AH); chk("admin_200 " + p, s == 200 and len(b) > 0, {"status": s, "bytes": len(b), "sha256": sha(b)[:16], "secs": round(d, 2)})
    # cap: invalid / above-cap values rejected before any sampling
    for q in ("seconds=61", "seconds=0", "seconds=-1", "seconds=abc", "seconds=3600"):
        for ep in ("profile", "trace"):
            s, hd, b, _, d = raw("GET", "/debug/pprof/%s?%s" % (ep, q), AH)
            chk("cap_400 %s?%s" % (ep, q), s == 400 and d < 5, {"status": s, "secs": round(d, 2), "body": b[:80].decode("utf8", "replace")})
    s, hd, b, _, d = raw("GET", "/debug/pprof/profile?seconds=2", AH)
    chk("profile_2s_200_duration", s == 200 and 1.8 <= d <= 6 and len(b) > 0, {"status": s, "secs": round(d, 2), "bytes": len(b), "sha256": sha(b)[:16]})
    # one at a time: second concurrent sampling request gets 429
    out = {}
    def first(): out["a"] = raw("GET", "/debug/pprof/profile?seconds=5", AH)
    t = threading.Thread(target=first); t.start(); time.sleep(1.5)
    s2 = raw("GET", "/debug/pprof/trace?seconds=1", AH)
    t.join(); a = out["a"]
    chk("concurrent_second_429", s2[0] == 429 and a[0] == 200, {"second": s2[0], "first": a[0], "first_secs": round(a[4], 2)})
    # slot released afterwards
    s3 = raw("GET", "/debug/pprof/trace?seconds=1", AH); chk("slot_released_after", s3[0] == 200, s3[0])
    # exactly the cap: 60 s accepted and honoured (does not run beyond ~60 s)
    s, hd, b, _, d = raw("GET", "/debug/pprof/profile?seconds=60", AH, timeout=150)
    chk("cap_60s_accepted_and_bounded", s == 200 and 58 <= d <= 66 and len(b) > 0, {"status": s, "secs": round(d, 2), "bytes": len(b), "sha256": sha(b)[:16]})
finally:
    cleanup = raw("DELETE", "/api/v1/users/%s" % uid, {**AH, "Content-Type": "application/json", "Cookie": cv, "X-CSRF-Token": xt})[0]
    chk("cleanup_user", cleanup in (200, 204), cleanup)
json.dump(res, open("res_%s_%s.json" % (PH, R), "w"), default=str)
print("SUMMARY", PH, R, sum(r["result"] == "PASS" for r in res), "PASS", sum(r["result"] == "FAIL" for r in res), "FAIL")
