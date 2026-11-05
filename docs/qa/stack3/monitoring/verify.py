#!/usr/bin/env python3
"""Live verification of the stack-3 monitoring add-on (11.4.289: real services, real data; no stubs). Prints one PASS/FAIL line per check.
Secrets are read from $QA_DIR/{qa,mon}.env and never printed. usage: verify.py [host]   exit 0 = all PASS."""
import json, os, re, sys, time, urllib.request, urllib.parse, urllib.error, base64, gzip
import http.cookiejar as cookiejar

QA = os.environ.get("QA_DIR", os.path.expanduser("~/.cache/claude-tmp/catqa3"))
H = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1"
P = dict(prom=39090, graf=33030, pga=33050, ngx=33080, site=33081, api=38082)
P.update({k: int(os.environ[k.upper() + "_PORT"]) for k in list(P) if os.environ.get(k.upper() + "_PORT")})
def envf(p):
    d = {}
    for l in open(p):
        if "=" in l and not l.startswith("#"):
            k, v = l.rstrip("\n").split("=", 1); d[k] = v
    return d
Q, M = envf(f"{QA}/qa.env"), envf(f"{QA}/mon.env")
res = []
def rec(name, ok, detail=""):
    res.append(ok); print(("PASS" if ok else "FAIL"), name, detail, flush=True)
def http(url, method="GET", headers=None, data=None, opener=None, timeout=15):
    rq = urllib.request.Request(url, data=data, method=method, headers=headers or {})
    try:
        r = (opener or urllib.request.build_opener()).open(rq, timeout=timeout)
        return r.status, dict(r.headers), r.read()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()
    except Exception as e:
        return 0, {}, str(e).encode()
def jget(url, method="GET", headers=None, data=None, opener=None):
    s, h, b = http(url, method, headers, data, opener)
    try: return s, json.loads(b)
    except Exception: return s, None

# ---------------- Prometheus
pu = f"http://{H}:{P['prom']}"
s, d = jget(f"{pu}/api/v1/targets")
tg = [t for t in (d or {}).get("data", {}).get("activeTargets", []) if t["labels"].get("job") == "catalogizer-api"]
rec("prom.target_catalogizer-api_up", s == 200 and len(tg) == 1 and tg[0]["health"] == "up", f"health={tg[0]['health'] if tg else None} lastError={tg[0]['lastError'] if tg else None!r}")
def q(expr):
    s, d = jget(f"{pu}/api/v1/query?query={urllib.parse.quote(expr)}")
    return (d or {}).get("data", {}).get("result", []) if s == 200 else None
r = q('up{job="catalogizer-api"}'); rec("prom.query_up==1", bool(r) and r[0]["value"][1] == "1", f"{[x['value'][1] for x in r or []]}")
r = q("sum(catalogizer_http_requests_total)"); rec("prom.query_http_requests_total_has_data", bool(r) and float(r[0]["value"][1]) > 0, f"sum={r[0]['value'][1] if r else None}")
r = q("catalogizer_runtime_goroutines"); rec("prom.query_goroutines_has_data", bool(r) and float(r[0]["value"][1]) > 0, f"value={r[0]['value'][1] if r else None}")
r = q("catalogizer_this_metric_does_not_exist_needle"); rec("prom.control_needle_absent_metric_empty", r == [], f"result={r}")
# negative controls against the API /metrics (the auth the code requires: admin bearer or METRICS_SCRAPE_TOKEN)
au = f"http://{H}:{P['api']}"
s, _, _ = http(f"{au}/metrics"); rec("api.metrics_no_auth_rejected", s in (401, 403), f"http={s}")
s, _, _ = http(f"{au}/metrics", headers={"Authorization": "Bearer " + "x" * 40}); rec("api.metrics_wrong_bearer_rejected", s in (401, 403), f"http={s}")
lg = json.dumps({"username": Q["ADMIN_USERNAME"], "password": Q["ADMIN_PASSWORD"]}).encode()
s, _, b = http(f"{au}/api/v1/auth/login", "POST", {"Content-Type": "application/json"}, lg)
tok = json.loads(b).get("session_token") if s == 200 else None
s2, _, b2 = http(f"{au}/metrics", headers={"Authorization": f"Bearer {tok}"}) if tok else (0, {}, b"")
rec("api.metrics_valid_admin_bearer_ok", s2 == 200 and b"catalogizer_http_requests_total" in b2, f"http={s2}")

# ---------------- Grafana
gu = f"http://{H}:{P['graf']}"
ba = lambda u, p: {"Authorization": "Basic " + base64.b64encode(f"{u}:{p}".encode()).decode()}
GA = ba(M["GF_SECURITY_ADMIN_USER"], M["GF_SECURITY_ADMIN_PASSWORD"])
s, d = jget(f"{gu}/api/datasources/uid/prometheus-qa/health", headers=GA)
rec("grafana.datasource_health_OK", s == 200 and (d or {}).get("status") == "OK", f"http={s} msg={(d or {}).get('message')!r}")
s, _, _ = http(f"{gu}/api/datasources", headers=ba("no-such-user", "wrong-password")); rec("grafana.wrong_password_rejected", s == 401, f"http={s}")
s, d = jget(f"{gu}/api/dashboards/uid/catalogizer-api-qa", headers=GA)
panels = (d or {}).get("dashboard", {}).get("panels", []); rec("grafana.dashboard_provisioned", s == 200 and len(panels) == 7, f"panels={len(panels)}")
now = int(time.time() * 1000)
def panel_query(pn):
    now = int(time.time() * 1000)
    body = {"from": str(now - 3600_000), "to": str(now), "queries": [
        {"refId": t["refId"], "datasource": t["datasource"], "expr": t["expr"], "instant": False, "range": True, "intervalMs": 15000, "maxDataPoints": 200} for t in pn["targets"]]}
    return jget(f"{gu}/api/ds/query", "POST", {**GA, "Content-Type": "application/json"}, json.dumps(body).encode())
for pn in panels:
    must = pn["id"] in (1, 4, 5, 6)   # latency/error panels may legitimately be empty when no such series exists; the others must have data
    for _try in range(18):            # a fresh Prometheus needs >=2 scrapes before rate() has data: poll up to ~90s for must-have panels
        s, dd = panel_query(pn); nvals = 0; errs = []
        for ref, rr in ((dd or {}).get("results") or {}).items():
            if rr.get("error"): errs.append(rr["error"][:80])
            for fr in rr.get("frames", []): nvals += sum(len(c) for c in fr["data"]["values"][1:])
        if s == 200 and not errs and (nvals > 0 or not must): break
        time.sleep(5)
    ok = s == 200 and not errs and (nvals > 0 or not must)
    rec(f"grafana.panel{pn['id']}_{pn['title'][:28].replace(' ','_')}", ok, f"http={s} datapoints={nvals} errors={errs}")

# ---------------- pgAdmin
pg = f"http://{H}:{P['pga']}"
s, _, b = http(f"{pg}/login"); rec("pgadmin.login_page_reachable", s == 200 and b"pgAdmin" in b, f"http={s}")
CSRF = re.compile(rb'"csrfToken"\s*:\s*"([^"]+)"')
def pgsession(pw, email=None):
    """fresh cookie session -> login attempt -> returns (opener, csrf-of-authenticated-session or None)."""
    op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookiejar.CookieJar()))
    s, _, b = http(f"{pg}/login", opener=op); m = CSRF.search(b)
    f = urllib.parse.urlencode({"email": email or M["PGADMIN_DEFAULT_EMAIL"], "password": pw, "csrf_token": m.group(1).decode() if m else "", "language": "en"}).encode()
    http(f"{pg}/authenticate/login", "POST", {"Content-Type": "application/x-www-form-urlencoded", "X-pgA-CSRFToken": m.group(1).decode() if m else ""}, f, opener=op)
    s, _, b = http(f"{pg}/browser/js/utils.js", opener=op); mt = re.search(rb"csrf_token'\]\s*=\s*'([^']+)'", b); c = mt.group(1).decode() if mt else ""
    s, d = jget(f"{pg}/browser/server_group/obj/", headers={"X-pgA-CSRFToken": c}, opener=op)   # authenticated iff JSON list (login-redirect HTML otherwise)
    return op, (c if (s == 200 and isinstance(d, list)) else None)
op, c = pgsession("definitely-wrong-password", "no-such-user@example.invalid")  
rec("pgadmin.wrong_password_not_authenticated", c is None, f"authenticated={c is not None}")
op, csrf = pgsession(M["PGADMIN_DEFAULT_PASSWORD"])
rec("pgadmin.login_ok_authenticated_session", bool(csrf), f"csrf_len={len(csrf or '')}")
csrf = csrf or ""
hd = {"X-pgA-CSRFToken": csrf, "Content-Type": "application/json"}
s, d = jget(f"{pg}/browser/server_group/obj/", headers=hd, opener=op)
gid = next((g["id"] for g in (d if isinstance(d, list) else []) if g.get("name") == "Catalogizer QA"), None)
s, d = jget(f"{pg}/browser/server/obj/{gid}/", headers=hd, opener=op) if gid else (0, None)
srv = {x.get("name"): x.get("id") for x in (d if isinstance(d, list) else [])}
rec("pgadmin.preconfigured_server_listed", "catqa3-postgres" in srv, f"http={s} group={gid} servers={sorted(srv)}")
sid = srv.get("catqa3-postgres")
s, d = jget(f"{pg}/browser/server/connect/{gid}/{sid}", "POST", hd, b"{}", opener=op) if sid else (0, None)
rec("pgadmin.server_connection_works", s == 200 and (d or {}).get("success") == 1 and (d or {}).get("data", {}).get("connected") is True, f"http={s} success={(d or {}).get('success')} connected={(d or {}).get('data', {}).get('connected')}")
if sid: http(f"{pg}/browser/server/disconnect/{gid}/{sid}", "DELETE", hd, opener=op)

# ---------------- nginx
nu, su = f"http://{H}:{P['ngx']}", f"http://{H}:{P['site']}"
SEC = ["x-content-type-options", "x-frame-options", "referrer-policy", "permissions-policy", "cross-origin-opener-policy"]
def lc(h): return {k.lower(): v for k, v in h.items()}
s, h, b = http(f"{nu}/health"); h = lc(h); rec("nginx.GET_/health_proxied_to_api", s == 200 and b'"status"' in b, f"http={s} body={b[:60]!r}")
rec("nginx.security_headers_on_/health", all(k in h for k in SEC), f"missing={[k for k in SEC if k not in h]}")
rec("nginx.server_tokens_off", "nginx/" not in h.get("server", ""), f"server={h.get('server')!r}")
s, h, b = http(f"{nu}/api/v1/auth/status"); s2, h2, b2 = http(f"{nu}/api/v1/auth/login", "POST", {"Content-Type": "application/json"}, json.dumps({"username": "nobody-x", "password": "wrong-pw-1"}).encode())
rec("nginx.api_v1_route_reaches_api_(login_wrong_creds_401_json)", s2 == 401 and b2[:1] == b"{", f"http={s2} body={b2[:60]!r}")
s, h, b = http(f"{nu}/api/v1/auth/login", "POST", {"Content-Type": "application/json"}, lg); rec("nginx.api_login_valid_creds_200", s == 200 and b"session_token" in b, f"http={s}")
try: tok2 = json.loads(b).get("session_token") if s == 200 else None
except Exception: tok2 = None
if tok2:
    s_, _, _ = http(f"{nu}/api/v1/auth/logout", "POST", {"Authorization": f"Bearer {tok2}"}); rec("nginx.logout_revokes_proxy_login_session", s_ == 200, f"logout={s_}")
s, h, b = http(f"{nu}/"); h = lc(h); rec("nginx.web_index", s == 200 and (b"<div id=\"root\"" in b or b"<!doctype html" in b.lower()), f"http={s} ctype={h.get('content-type')}")
s, h, b = http(f"{nu}/", headers={"Accept-Encoding": "gzip"}); h = lc(h); rec("nginx.gzip_applied_to_web_index", h.get("content-encoding") == "gzip" and gzip.decompress(b)[:1] == b"<", f"content-encoding={h.get('content-encoding')}")
s, h, b = http(f"{su}/"); h = lc(h); rec("nginx.website_index_via_proxy", s == 200 and b"<html" in b.lower(), f"http={s}")
rec("nginx.security_headers_on_website", all(k in h for k in SEC), f"missing={[k for k in SEC if k not in h]}")
s, h, b = http(f"{nu}/api/v1/nonexistent-route-needle"); rec("nginx.control_unknown_api_route_404_from_api", s == 404, f"http={s}")

# ---------------- F1/F8: exposure (what the pod can reach on host loopback; what is reachable on the LAN IP)
import socket, subprocess
def inpod(port):
    r = subprocess.run(["podman", "exec", "catqa3mon-nginx", "sh", "-c", f"nc -z -w2 127.0.0.1 {port}"], capture_output=True, timeout=30)
    return r.returncode == 0
PGP, API = int(os.environ.get("PG_PORT", 35434)), P["api"]
rec("pod.reaches_postgres_port_(control_needle_must_be_reachable)", inpod(PGP), f"port={PGP}")
rec("pod.reaches_api_port_(control_needle_must_be_reachable)", inpod(API), f"port={API}")
for port, what in ((36381, "catqa3 redis (host loopback)"), (11434, "host loopback 11434"), (35433, "host loopback 35433 (other listener)")):
    s_ = socket.socket(); s_.settimeout(2); lo_listening = s_.connect_ex(("127.0.0.1", port)) == 0; s_.close()
    rec(f"pod.CANNOT_reach_{port}_{what.split()[0]}", not inpod(port), f"host_listening={lo_listening}")
lan = subprocess.run("ip -4 route get 1.1.1.1 | sed -n 's/.* src \\([0-9.]*\\).*/\\1/p'", shell=True, capture_output=True, text=True).stdout.strip()
def lan_open(port):
    s_ = socket.socket(); s_.settimeout(3); ok = s_.connect_ex((lan, port)) == 0; s_.close(); return ok
lan_ok = bool(re.fullmatch(r"\d+\.\d+\.\d+\.\d+", lan))
rec("lan.ip_detected", lan_ok, f"ip={lan}")
if lan_ok:
    rec("lan.nginx_reachable_(control_needle)", lan_open(P["ngx"]), f"{lan}:{P['ngx']}")
    for name, port, bindvar in (("prometheus", P["prom"], "PROM_BIND"), ("grafana", P["graf"], "GRAFANA_BIND"), ("pgadmin", P["pga"], "PGADMIN_BIND")):
        opt = os.environ.get(bindvar) or os.environ.get("LAN_BIND" if name != "prometheus" else bindvar)
        if opt in ("0.0.0.0", lan): print(f"SKIP lan.{name}_not_reachable (LAN opt-in {bindvar}={opt})"); continue
        rec(f"lan.{name}_NOT_reachable_on_LAN_IP", not lan_open(port), f"{lan}:{port}")

# ---------------- F2: revoke the admin sessions this run created
for t_ in [t for t in (tok,) if t]:
    s_, _, _ = http(f"{au}/api/v1/auth/logout", "POST", {"Authorization": f"Bearer {t_}"})
    s2_, _, _ = http(f"{au}/metrics", headers={"Authorization": f"Bearer {t_}"})
    rec("api.logout_revokes_verify_session_token", s_ == 200 and s2_ in (401, 403), f"logout={s_} metrics_after={s2_}")

print(f"SUMMARY pass={sum(res)} fail={len(res) - sum(res)} total={len(res)}")
sys.exit(0 if all(res) else 1)
