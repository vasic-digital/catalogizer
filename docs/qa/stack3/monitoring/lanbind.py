"""LAN-exposure policy for verify.py (CAT-VERIFY-LANBIND-1). Pure functions, unit-tested by test_lanbind.py.
Operator decision: Prometheus (basic auth), Grafana and pgAdmin MAY be LAN-bound (PROM_BIND / GRAFANA_BIND / PGADMIN_BIND / LAN_BIND);
a LAN-bound service must then be LAN-reachable AND still reject anonymous access. PostgreSQL and Redis are always loopback-only."""
import re, urllib.parse

ALWAYS_LOOPBACK = "always-loopback"
BIND_VARS = {"prometheus": ("PROM_BIND",), "grafana": ("GRAFANA_BIND", "LAN_BIND"), "pgadmin": ("PGADMIN_BIND", "LAN_BIND")}

def classify_bind(value, lan):
    v = (value or "").strip().strip("[]")
    if not v or v == "localhost" or v == "::1" or re.fullmatch(r"127(\.\d{1,3}){3}", v): return "loopback"
    if v in ("0.0.0.0", "::", lan): return "lan"        # "::" / "[::]" is the IPv6 wildcard: reachable from the LAN
    return "other"

def parse_podman_port(text):
    """`podman port <pod>` lines '9090/tcp -> 192.168.1.115:39090' (or '[::]:33050') -> {39090: '192.168.1.115'}"""
    out = {}
    for line in text.splitlines():
        m = re.match(r"^\s*\d+/\w+\s*->\s*(\S+):(\d+)\s*$", line)
        if m: out[int(m.group(2))] = m.group(1)
    return out

def read_published(pod, run):
    """What the running pod publishes. A pod's published ports live on its INFRA container: `podman port <podname>` fails rc=125
    ('no container with name'), so resolve the infra container id first (as up.sh does). Any failure -> {} (then the loopback default
    applies, source 'default')."""
    try:
        i = run(["podman", "pod", "inspect", pod, "--format", "{{.InfraContainerID}}"], capture_output=True, text=True, timeout=30)
        infra = (i.stdout or "").strip() if i.returncode == 0 else ""
        if not re.fullmatch(r"[0-9a-f]{12,64}", infra): return {}
        r = run(["podman", "port", infra], capture_output=True, text=True, timeout=30)
    except Exception:   # subprocess.TimeoutExpired, FileNotFoundError, OSError
        return {}
    return parse_podman_port(r.stdout) if r.returncode == 0 else {}

def resolve_bind(name, env, published, host_port):
    """-> (bind value, source). Precedence as up.sh/lib.sh: the service's own env var, then LAN_BIND (Grafana/pgAdmin only) = 'env';
    else what the running pod publishes = 'pod' (the thing under test, so never an authority); else the loopback default = 'default'."""
    for v in BIND_VARS[name]:
        if env.get(v): return env[v], "env"
    if host_port in published: return published[host_port], "pod"
    return "127.0.0.1", "default"

def exposure_verdict(name, bind_class, reachable, source="env"):
    """-> (check name, ok) or None when no assertion is possible (bound to some other interface).
    LAN exposure is only acceptable when DECLARED (env); a LAN exposure inferred from the pod under test is reported as undeclared and FAILs."""
    tag = "" if source == "env" else "_[pod-inferred]" if source == "pod" else f"_[{source}]"
    if bind_class in ("loopback", ALWAYS_LOOPBACK): return (f"lan.{name}_NOT_reachable_on_LAN_IP{tag}", not reachable)
    if bind_class == "lan":
        if source == "env": return (f"lan.{name}_reachable_on_LAN_IP_(declared:env)", bool(reachable))
        return (f"lan.{name}_LAN_exposure_undeclared_(source={source};_set_{BIND_VARS[name][0]})", False)
    return None

def auth_required_ok(name, status):
    """Anonymous request through the LAN IP must be refused: 401. 403/404/5xx/0 are not proof (anonymous-Viewer Grafana, a wrong path and a
    dead service all look different from 'login required'). A bare 400 is never proof (pgAdmin checks CSRF before auth); pgAdmin has its own
    check, pgadmin_anonymous_rejected."""
    return status == 401

_CSRF = re.compile(rb'"csrfToken"\s*:\s*"([^"]+)"')

def pgadmin_anonymous_rejected(get_login, get_protected, host):
    """pgAdmin refuses a tokenless request with 400 'CSRF token is missing' BEFORE any authentication check, so that 400 proves nothing.
    Take a real CSRF token + session from an anonymous GET /login, then do the anonymous protected GET carrying it (redirects not followed):
    login is required iff that gets 401, or a 302/303 (not 301/307/308) whose Location path is exactly /login on the probed host. get_login() -> (status, body);
    get_protected(csrf) -> (status, headers); host = the 'host:port' actually probed (an absolute Location must name exactly it, scheme http). Anything else (200, 400, 0, 5xx, a redirect elsewhere) is not proof."""
    st, body = get_login()
    m = _CSRF.search(body or b"") if st == 200 else None
    if not m: return False
    st, h = get_protected(m.group(1).decode())
    if st == 401: return True
    if st in (302, 303):
        loc = {k.lower(): v for k, v in (h or {}).items()}.get("location", "")
        u = urllib.parse.urlsplit(loc)
        return u.path == "/login" and (u.netloc == "" and u.scheme == "" or u.scheme == "http" and u.netloc.lower() == host.lower())
    return False
