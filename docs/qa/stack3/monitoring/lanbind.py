"""LAN-exposure policy for verify.py (CAT-VERIFY-LANBIND-1). Pure functions, unit-tested by test_lanbind.py.
Operator decision: Prometheus (basic auth), Grafana and pgAdmin MAY be LAN-bound (PROM_BIND / GRAFANA_BIND / PGADMIN_BIND / LAN_BIND);
a LAN-bound service must then be LAN-reachable AND still reject anonymous access. PostgreSQL and Redis are always loopback-only."""
import re

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
    """What the running pod publishes. Any failure (timeout, no podman, non-zero) -> {} (then the loopback default applies, source 'default')."""
    try:
        r = run(["podman", "port", pod], capture_output=True, text=True, timeout=30)
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
    """Anonymous request through the LAN IP must be refused: 401 (pgAdmin also a redirect to login or 403). 403/404/5xx/0 are not proof for the others
    (anonymous-Viewer Grafana, a wrong path and a dead service all look different from 'login required')."""
    if name == "pgadmin": return status in (302, 401, 403)
    return status == 401
