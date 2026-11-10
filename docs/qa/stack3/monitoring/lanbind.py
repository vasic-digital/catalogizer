"""LAN-exposure policy for verify.py (CAT-VERIFY-LANBIND-1). Pure functions, unit-tested by test_lanbind.py.
Operator decision: Prometheus (basic auth), Grafana and pgAdmin MAY be LAN-bound (PROM_BIND / GRAFANA_BIND / PGADMIN_BIND / LAN_BIND);
a LAN-bound service must then be LAN-reachable AND still reject anonymous access. PostgreSQL and Redis are always loopback-only."""
import re

ALWAYS_LOOPBACK = "always-loopback"
BIND_VARS = {"prometheus": ("PROM_BIND",), "grafana": ("GRAFANA_BIND", "LAN_BIND"), "pgadmin": ("PGADMIN_BIND", "LAN_BIND")}

def classify_bind(value, lan):
    if not value or value.startswith("127.") or value == "localhost" or value == "::1": return "loopback"
    if value in ("0.0.0.0", lan): return "lan"
    return "other"

def parse_podman_port(text):
    """`podman port <pod>` lines '9090/tcp -> 192.168.1.115:39090' -> {39090: '192.168.1.115'}"""
    out = {}
    for line in text.splitlines():
        m = re.match(r"^\s*\d+/\w+\s*->\s*(\S+):(\d+)\s*$", line)
        if m: out[int(m.group(2))] = m.group(1)
    return out

def resolve_bind(name, env, published, host_port):
    """Same precedence as up.sh/lib.sh: explicit env var, then LAN_BIND (Grafana/pgAdmin only), then what the running pod really publishes, then the loopback default."""
    for v in BIND_VARS[name]:
        if env.get(v): return env[v]
    return published.get(host_port) or "127.0.0.1"

def exposure_verdict(name, bind_class, reachable):
    """-> (check name, ok) or None when no assertion is possible (bound to some other interface)."""
    if bind_class in ("loopback", ALWAYS_LOOPBACK): return (f"lan.{name}_NOT_reachable_on_LAN_IP", not reachable)
    if bind_class == "lan": return (f"lan.{name}_reachable_on_LAN_IP_(intended)", bool(reachable))
    return None

def auth_required_ok(name, status):
    """Anonymous request through the LAN IP must be refused. status 0 (no answer) proves nothing, so it is a failure."""
    if name == "pgadmin": return status in (302, 401, 403)
    return status in (401, 403)
