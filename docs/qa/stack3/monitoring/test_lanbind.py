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

class Policy(unittest.TestCase):
    def test_classify(self):
        self.assertEqual(L.classify_bind("0.0.0.0", LAN), "lan")
        self.assertEqual(L.classify_bind(LAN, LAN), "lan")
        self.assertEqual(L.classify_bind("127.0.0.1", LAN), "loopback")
        self.assertEqual(L.classify_bind("10.9.9.9", LAN), "other")
        self.assertEqual(L.classify_bind("", LAN), "loopback")
        self.assertEqual(L.classify_bind(None, LAN), "loopback")

    def test_lan_bound_service_must_be_reachable(self):
        self.assertEqual(L.exposure_verdict("grafana", "lan", True), ("lan.grafana_reachable_on_LAN_IP_(intended)", True))
        name, ok = L.exposure_verdict("grafana", "lan", False)
        self.assertFalse(ok)   # LAN-bound but not reachable = a real defect, not a pass

    def test_loopback_service_must_not_be_reachable(self):
        self.assertEqual(L.exposure_verdict("pgadmin", "loopback", False), ("lan.pgadmin_NOT_reachable_on_LAN_IP", True))
        self.assertFalse(L.exposure_verdict("pgadmin", "loopback", True)[1])

    def test_other_bind_is_skipped_not_passed(self):
        self.assertIsNone(L.exposure_verdict("prometheus", "other", True))

    def test_stateful_backends_are_always_loopback(self):
        for svc in ("postgresql", "redis"):
            self.assertEqual(L.exposure_verdict(svc, L.ALWAYS_LOOPBACK, False), (f"lan.{svc}_NOT_reachable_on_LAN_IP", True))
            self.assertFalse(L.exposure_verdict(svc, L.ALWAYS_LOOPBACK, True)[1])

class Resolve(unittest.TestCase):
    def test_env_wins_then_lan_bind_then_published_then_default(self):
        pub = L.parse_podman_port(PODMAN_PORT)
        self.assertEqual(pub[39090], "192.168.1.115")
        self.assertEqual(L.resolve_bind("grafana", {"GRAFANA_BIND": "0.0.0.0"}, pub, 33030), "0.0.0.0")
        self.assertEqual(L.resolve_bind("grafana", {"LAN_BIND": "0.0.0.0"}, pub, 33030), "0.0.0.0")
        self.assertEqual(L.resolve_bind("prometheus", {"LAN_BIND": "0.0.0.0"}, pub, 39090), "192.168.1.115")  # LAN_BIND never applies to prometheus
        self.assertEqual(L.resolve_bind("grafana", {}, pub, 33030), "192.168.1.115")  # inferred from the running pod
        self.assertEqual(L.resolve_bind("pgadmin", {}, pub, 33050), "127.0.0.1")
        self.assertEqual(L.resolve_bind("pgadmin", {}, {}, 33050), "127.0.0.1")  # nothing known: lib.sh default, loopback

    def test_parse_ignores_garbage(self):
        self.assertEqual(L.parse_podman_port("not a line\n\n"), {})

    def test_reproduces_observed_stale_failure(self):
        # The pre-fix verify.py asserted NOT_reachable for LAN-bound services (3 FAILs observed). With the pod publishing on the LAN IP
        # the verdict must now demand reachability instead.
        pub = L.parse_podman_port(PODMAN_PORT)
        for name, port in (("prometheus", 39090), ("grafana", 33030)):
            cls = L.classify_bind(L.resolve_bind(name, {}, pub, port), LAN)
            self.assertEqual(cls, "lan")
            self.assertTrue(L.exposure_verdict(name, cls, True)[1])

class Auth(unittest.TestCase):
    def test_auth_required_verdicts(self):
        self.assertTrue(L.auth_required_ok("prometheus", 401))
        self.assertFalse(L.auth_required_ok("prometheus", 200))
        self.assertFalse(L.auth_required_ok("prometheus", 0))      # unreachable proves nothing
        self.assertTrue(L.auth_required_ok("grafana", 401))
        self.assertTrue(L.auth_required_ok("pgadmin", 302))
        self.assertTrue(L.auth_required_ok("pgadmin", 401))
        self.assertFalse(L.auth_required_ok("pgadmin", 200))

if __name__ == "__main__":
    unittest.main(verbosity=1)
