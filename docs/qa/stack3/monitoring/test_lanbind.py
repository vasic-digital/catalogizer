#!/usr/bin/env python3
"""CAT-VERIFY-LANBIND-1: unit tests of the LAN-exposure policy used by verify.py (real functions, no stack needed).
LAN-bound Grafana/pgAdmin/Prometheus are an intentional operator decision: they must be asserted LAN-reachable (and still
authenticated); PostgreSQL/Redis stay loopback-only. usage: python3 test_lanbind.py   exit 0 = all pass"""
import os, sys, unittest
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lanbind as L

LAN = "192.168.1.115"
PODMAN_PORT = """9090/tcp -> 192.168.1.115:39090
3000/tcp -> 192.168.1.115:33030
5050/tcp -> 127.0.0.1:33050
8088/tcp -> 0.0.0.0:33080
"""

class PodRun:
    """Models real podman: `podman port <pod-name>` fails rc=125 (a pod name is not a container); the pod's ports are read from
    its infra container (as up.sh does). Records the commands run."""
    def __init__(self, pod="catqa3mon", infra="95325aab87bb", out=PODMAN_PORT): self.pod, self.infra, self.out, self.calls = pod, infra, out, []
    def __call__(self, cmd, **k):
        self.calls.append(cmd)
        class R: pass
        r = R(); r.stdout = ""; r.returncode = 125
        if cmd[:3] == ["podman", "pod", "inspect"] and cmd[3] == self.pod: r.stdout, r.returncode = self.infra + "\n", 0
        elif cmd[:2] == ["podman", "port"] and cmd[2] == self.infra: r.stdout, r.returncode = self.out, 0
        return r

class FakeRun:
    def __init__(self, exc=None, out="", rc=0): self.exc, self.out, self.rc = exc, out, rc
    def __call__(self, *a, **k):
        if self.exc: raise self.exc
        class R: pass
        r = R(); r.stdout, r.returncode = self.out, self.rc; return r

class Policy(unittest.TestCase):
    def test_classify(self):
        self.assertEqual(L.classify_bind("0.0.0.0", LAN), "lan")
        self.assertEqual(L.classify_bind(LAN, LAN), "lan")
        self.assertEqual(L.classify_bind("127.0.0.1", LAN), "loopback")
        self.assertEqual(L.classify_bind("10.9.9.9", LAN), "other")
        self.assertEqual(L.classify_bind("", LAN), "loopback")
        self.assertEqual(L.classify_bind(None, LAN), "loopback")
        self.assertEqual(L.classify_bind("127.0.1.1", LAN), "loopback")      # whole 127/8
        self.assertEqual(L.classify_bind("::1", LAN), "loopback")
        self.assertEqual(L.classify_bind("[::1]", LAN), "loopback")
        self.assertEqual(L.classify_bind("[::]", LAN), "lan")                 # IPv6 wildcard is NOT loopback
        self.assertEqual(L.classify_bind("::", LAN), "lan")
        self.assertEqual(L.classify_bind("1270.0.0.1", LAN), "other")        # not a 127/8 address

    def test_lan_bound_service_must_be_reachable(self):
        self.assertEqual(L.exposure_verdict("grafana", "lan", True, "env"), ("lan.grafana_reachable_on_LAN_IP_(declared:env)", True))
        name, ok = L.exposure_verdict("grafana", "lan", False, "env")
        self.assertFalse(ok)   # LAN-bound but not reachable = a real defect, not a pass

    def test_pod_inferred_lan_exposure_is_never_intended(self):
        # the expectation must not be read from the thing under test: pod-inferred (or default) LAN exposure is a FAIL, named as undeclared
        for src in ("pod", "default"):
            name, ok = L.exposure_verdict("grafana", "lan", True, src)
            self.assertFalse(ok)
            self.assertIn("undeclared", name); self.assertIn(src, name); self.assertNotIn("intended", name)

    def test_source_recorded_in_loopback_name_when_not_env(self):
        self.assertEqual(L.exposure_verdict("pgadmin", "loopback", False, "pod"), ("lan.pgadmin_NOT_reachable_on_LAN_IP_[pod-inferred]", True))
        self.assertEqual(L.exposure_verdict("pgadmin", "loopback", False, "env"), ("lan.pgadmin_NOT_reachable_on_LAN_IP", True))

    def test_loopback_service_must_not_be_reachable(self):
        self.assertEqual(L.exposure_verdict("pgadmin", "loopback", False, "env"), ("lan.pgadmin_NOT_reachable_on_LAN_IP", True))
        self.assertFalse(L.exposure_verdict("pgadmin", "loopback", True, "env")[1])

    def test_other_bind_is_skipped_not_passed(self):
        self.assertIsNone(L.exposure_verdict("prometheus", "other", True, "env"))

    def test_stateful_backends_are_always_loopback(self):
        for svc in ("postgresql", "redis"):
            self.assertEqual(L.exposure_verdict(svc, L.ALWAYS_LOOPBACK, False), (f"lan.{svc}_NOT_reachable_on_LAN_IP", True))
            self.assertFalse(L.exposure_verdict(svc, L.ALWAYS_LOOPBACK, True)[1])

class Resolve(unittest.TestCase):
    def test_env_wins_then_lan_bind_then_published_then_default(self):
        pub = L.parse_podman_port(PODMAN_PORT)
        self.assertEqual(pub[39090], "192.168.1.115")
        self.assertEqual(L.resolve_bind("grafana", {"GRAFANA_BIND": "0.0.0.0"}, pub, 33030), ("0.0.0.0", "env"))
        self.assertEqual(L.resolve_bind("grafana", {"LAN_BIND": "0.0.0.0"}, pub, 33030), ("0.0.0.0", "env"))
        self.assertEqual(L.resolve_bind("prometheus", {"LAN_BIND": "0.0.0.0"}, pub, 39090), ("192.168.1.115", "pod"))  # LAN_BIND never applies to prometheus
        self.assertEqual(L.resolve_bind("grafana", {}, pub, 33030), ("192.168.1.115", "pod"))
        self.assertEqual(L.resolve_bind("pgadmin", {}, pub, 33050), ("127.0.0.1", "pod"))
        self.assertEqual(L.resolve_bind("pgadmin", {}, {}, 33050), ("127.0.0.1", "default"))

    def test_service_specific_bind_beats_lan_bind(self):
        self.assertEqual(L.resolve_bind("grafana", {"GRAFANA_BIND": "127.0.0.1", "LAN_BIND": "0.0.0.0"}, {}, 33030), ("127.0.0.1", "env"))
        self.assertEqual(L.resolve_bind("pgadmin", {"PGADMIN_BIND": "127.0.0.1", "LAN_BIND": "0.0.0.0"}, {}, 33050), ("127.0.0.1", "env"))

    def test_podman_timeout_or_missing_is_empty_not_a_crash(self):
        import subprocess
        self.assertEqual(L.read_published("catqa3mon", FakeRun(exc=subprocess.TimeoutExpired("podman", 30))), {})
        self.assertEqual(L.read_published("catqa3mon", FakeRun(exc=FileNotFoundError())), {})
        self.assertEqual(L.read_published("catqa3mon", FakeRun(rc=125, out=PODMAN_PORT)), {})
        self.assertEqual(L.read_published("catqa3mon", PodRun())[39090], "192.168.1.115")

    def test_pod_name_resolves_via_infra_container(self):
        # defect (live, stack3): `podman port catqa3mon` -> rc=125 'no container with name' -> {} -> false loopback default
        run = PodRun()
        self.assertEqual(L.read_published("catqa3mon", run)[39090], "192.168.1.115")      # golden-good
        self.assertNotIn(["podman", "port", "catqa3mon"], run.calls)                      # the pod name is never passed to `podman port`

    def test_inspect_failure_with_id_on_stdout_is_empty(self):
        class Bad(PodRun):
            def __call__(self, cmd, **k):
                r = super().__call__(cmd, **k)
                if cmd[:3] == ["podman", "pod", "inspect"]: r.returncode = 125; r.stdout = "0123456789ab\n"
                return r
        self.assertEqual(L.read_published("catqa3mon", Bad(infra="0123456789ab")), {})
    def test_infra_id_must_be_hex(self):
        self.assertEqual(L.read_published("catqa3mon", PodRun(infra="--all")), {})
        for bad in ("short", "abc123", "a" * 65): self.assertEqual(L.read_published("catqa3mon", PodRun(infra=bad)), {}, bad)

    def test_unknown_pod_or_empty_infra_is_empty(self):
        self.assertEqual(L.read_published("nosuchpod", PodRun()), {})                    # golden-bad: unresolvable pod -> {} (default applies)
        self.assertEqual(L.read_published("catqa3mon", PodRun(infra="")), {})            # negative control: no infra id -> never `podman port ""`
        self.assertEqual(L.read_published("catqa3mon", PodRun(out="")), {})

    def test_ipv6_wildcard_published_parses(self):
        self.assertEqual(L.parse_podman_port("5050/tcp -> [::]:33050\n")[33050], "[::]")

    def test_parse_ignores_garbage(self):
        self.assertEqual(L.parse_podman_port("not a line\n\n"), {})

    def test_reproduces_observed_stale_failure(self):
        # The pre-fix verify.py asserted NOT_reachable for LAN-bound services (3 FAILs observed). With the pod publishing on the LAN IP
        # the verdict must now demand reachability instead.
        pub = L.parse_podman_port(PODMAN_PORT)
        for name, port in (("prometheus", 39090), ("grafana", 33030)):
            val, src = L.resolve_bind(name, {}, pub, port)
            cls = L.classify_bind(val, LAN)
            self.assertEqual(cls, "lan")
            self.assertFalse(L.exposure_verdict(name, cls, True, src)[1])          # undeclared: not a pass
            val, src = L.resolve_bind(name, {"PROM_BIND": LAN, "GRAFANA_BIND": LAN}, pub, port)
            self.assertTrue(L.exposure_verdict(name, L.classify_bind(val, LAN), True, src)[1])  # declared: pass

LOGIN_HTML = b'<script>window.x={"csrfToken": "tok123"}</script>'

def pg_double(login=(200, LOGIN_HTML), tokenless=(400, {}), tokened=(302, {"Location": "/login?next=%2Fbrowser"})):
    seen = []
    def get_login(): return login
    def get_protected(csrf):
        seen.append(csrf); return tokened if csrf else tokenless
    return get_login, get_protected, seen

class PgAnon(unittest.TestCase):
    def test_golden_good_redirect_to_login(self):
        g, p, seen = pg_double()
        self.assertTrue(L.pgadmin_anonymous_rejected(g, p)); self.assertEqual(seen, ["tok123"])   # the request carried the CSRF token
        self.assertTrue(L.pgadmin_anonymous_rejected(*pg_double(tokened=(303, {"Location": "http://h:1/login"}))[:2]))
        self.assertTrue(L.pgadmin_anonymous_rejected(*pg_double(tokened=(401, {}))[:2]))
    def test_login_disabled_pgadmin_must_fail(self):
        # tokenless -> 400 (CSRF), tokened anonymous -> 200 + data: login is NOT required
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(tokened=(200, {}))[:2]))
    def test_redirect_elsewhere_fails(self):
        for loc in ("/somewhere", "http://evil/x", "/loginx", ""):
            self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(tokened=(302, {"Location": loc}))[:2]), loc)
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(tokened=(302, {}))[:2]))
    def test_no_token_or_dead_login_fails(self):
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(login=(200, b"no token here"))[:2]))
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(login=(0, b"refused"))[:2]))
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(tokened=(400, {}))[:2]))   # still CSRF-rejected even with token: proves nothing
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(tokened=(0, {}))[:2]))
        self.assertFalse(L.pgadmin_anonymous_rejected(*pg_double(tokened=(500, {}))[:2]))

class Auth(unittest.TestCase):
    def test_auth_required_verdicts(self):
        self.assertTrue(L.auth_required_ok("prometheus", 401))
        self.assertFalse(L.auth_required_ok("prometheus", 200))
        self.assertFalse(L.auth_required_ok("prometheus", 0))      # unreachable proves nothing
        self.assertTrue(L.auth_required_ok("grafana", 401))
        self.assertFalse(L.auth_required_ok("grafana", 403))       # 403 is not "login required"; only 401 proves anonymous is refused
        self.assertFalse(L.auth_required_ok("prometheus", 403))
        for n in ("prometheus", "grafana", "pgadmin"):
            self.assertFalse(L.auth_required_ok(n, 404), n)           # wrong path / dead route is a failure, not a rejection
            self.assertFalse(L.auth_required_ok(n, 500), n)
        # a bare 400 (CSRF checked before auth) proves nothing about login being required: never an auth-required PASS, for any service
        for n in ("prometheus", "grafana", "pgadmin"): self.assertFalse(L.auth_required_ok(n, 400), n)
        self.assertFalse(L.auth_required_ok("pgadmin", 302))      # a bare 302 (no Location check) is not enough either
        self.assertTrue(L.auth_required_ok("pgadmin", 401))
        self.assertTrue(L.auth_required_ok("pgadmin", 401))
        self.assertFalse(L.auth_required_ok("pgadmin", 200))

if __name__ == "__main__":
    unittest.main(verbosity=1)
