"""T216 - availability probe tests (doc12 10.3). One golden-present and one golden-absent fixture per probe, plus the carrier cases (11.4.201):
a decoy that answers like the dependency but is not it must not read as present, and a credential value must never appear in a probe's output.
Run through `TIC tooling unit` (IMG-TESTUTIL). The fixtures are real local servers and a stub `adb`/`smbclient`/`podman`, never a mocked probe.
"""
import json
import os
import socket
import stat
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

import pytest

HERE = os.path.dirname(os.path.abspath(__file__))
PROBES = os.path.join(os.path.dirname(HERE), "probes")
REASONS = {"service_unreachable", "credential_absent", "credential_rejected", "device_absent", "device_wrong_identity",
           "device_unauthorised", "geo_restricted", "quota_exhausted", "licence_absent", "host_resource_unavailable"}


def probe(name, *args, env=None):
    path = os.path.join(PROBES, name)
    e = {"PATH": os.environ.get("PATH", "/usr/bin:/bin")}
    e.update(env or {})
    p = subprocess.run([sys.executable, "-I", path, *args], capture_output=True, text=True, env=e, timeout=60)
    return p


def result(p):
    return json.loads(p.stdout.strip().splitlines()[-1])


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def _send(self, code, body, ctype="application/json"):
        data = body if isinstance(body, bytes) else json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/health":
            return self._send(200, {"status": "ok", "build_id": "b-123"})
        if self.path == "/versiononly":  # a decoy: prints a version, which any service can do
            return self._send(200, {"status": "healthy", "version": "1.0.0"})
        if self.path == "/carrier":  # answers 200 but does not identify itself
            return self._send(200, b"<html>welcome</html>", "text/html")
        if self.path.startswith("/meta"):
            key = self.headers.get("X-Api-Key", "")
            if key == "good-key":
                return self._send(200, {"ok": True})
            if key == "quota-key":
                return self._send(429, {"error": "quota"})
            return self._send(401, {"error": "bad key"})
        self._send(404, {})

    def do_POST(self):
        n = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(n) or b"{}")
        if self.path == "/login":
            if body.get("username") == "qa-user" and body.get("password") == "s3cret-value-xyz":
                return self._send(200, {"session_token": "t"})
            return self._send(401, {"error": "no"})
        self._send(404, {})


@pytest.fixture(scope="module")
def server():
    srv = HTTPServer(("127.0.0.1", 0), Handler)
    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()
    yield "http://127.0.0.1:%d" % srv.server_address[1]
    srv.shutdown()


def closed_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def exists(name):
    return os.path.isfile(os.path.join(PROBES, name))


@pytest.mark.parametrize("name", ["lib.py", "service.py", "credential.py", "device.py", "nas.py", "metadata.py", "vision.py", "runtime.py"])
def test_probe_file_exists(name):
    assert exists(name), "scripts/qa/probes/%s is absent (T216 not implemented)" % name


# ---- service ----
def test_service_golden_present(server):
    p = probe("service.py", "--url", server + "/health")
    r = result(p)
    assert p.returncode == 0 and r["status"] == "present" and r["probe"] == "service" and r["evidence"]["build_id"] == "b-123"


def test_service_golden_absent():
    p = probe("service.py", "--url", "http://127.0.0.1:%d/health" % closed_port(), "--timeout", "2")
    r = result(p)
    assert p.returncode == 1 and r["status"] == "blocked" and r["reason"] == "service_unreachable"


def test_service_carrier_answers_but_does_not_identify(server):
    p = probe("service.py", "--url", server + "/carrier")
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "service_unreachable" and "build" in json.dumps(r["evidence"]).lower()


def test_service_a_bare_version_is_not_an_identity(server):
    p = probe("service.py", "--url", server + "/versiononly")
    assert p.returncode == 1 and result(p)["reason"] == "service_unreachable" and result(p)["evidence"]["identified"] is False


def test_service_custom_id_key_is_honoured(server):
    p = probe("service.py", "--url", server + "/versiononly", "--id-key", "version")
    assert p.returncode == 0 and result(p)["evidence"]["build_id"] == "1.0.0"


# ---- credential ----
def test_credential_golden_present(server):
    p = probe("credential.py", "--env", "QA_PASS", "--user-env", "QA_USER", "--login-url", server + "/login",
              env={"QA_USER": "qa-user", "QA_PASS": "s3cret-value-xyz"})
    r = result(p)
    assert p.returncode == 0 and r["status"] == "present"
    assert "s3cret-value-xyz" not in p.stdout + p.stderr and "qa-user" not in p.stdout + p.stderr


def test_credential_golden_absent_unset():
    p = probe("credential.py", "--env", "QA_PASS_UNSET")
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "credential_absent" and r["evidence"]["env"] == "QA_PASS_UNSET"


def test_credential_carrier_empty_value_is_absent():
    p = probe("credential.py", "--env", "QA_PASS", env={"QA_PASS": ""})
    assert p.returncode == 1 and result(p)["reason"] == "credential_absent"


def test_credential_rejected(server):
    p = probe("credential.py", "--env", "QA_PASS", "--user-env", "QA_USER", "--login-url", server + "/login",
              env={"QA_USER": "qa-user", "QA_PASS": "wrong-value-abc"})
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "credential_rejected"
    assert "wrong-value-abc" not in p.stdout + p.stderr


def test_credential_set_without_login_url_is_present_by_name_only():
    p = probe("credential.py", "--env", "QA_PASS", env={"QA_PASS": "x"})
    r = result(p)
    assert p.returncode == 0 and r["evidence"]["login_checked"] is False


# ---- device (stub adb) ----
def make_adb(tmp_path, state="device", serialno="EMU-1", pkg=True, serial_known=True):
    script = tmp_path / "adb"
    script.write_text("""#!/bin/sh
# stub adb: argv is `-s SERIAL get-state | shell getprop ro.serialno | shell pm path PKG`
[ "$1" = "-s" ] || exit 2
shift; S="$1"; shift
[ "%(known)s" = "1" ] || { echo "error: device '$S' not found" >&2; exit 1; }
case "$1 $2 $3" in
  "get-state  ") if [ "%(state)s" = "unauthorized" ]; then echo "error: device unauthorized." >&2; exit 1; fi; echo "%(state)s";;
  "shell getprop ro.serialno") echo "%(serialno)s";;
  "shell pm path") if [ "%(pkg)s" = "1" ]; then echo "package:/data/app/x/base.apk"; else exit 1; fi;;
  *) exit 2;;
esac
""" % {"state": state, "serialno": serialno, "pkg": "1" if pkg else "0", "known": "1" if serial_known else "0"})
    script.chmod(script.stat().st_mode | stat.S_IEXEC)
    return str(script)


def test_device_golden_present(tmp_path):
    p = probe("device.py", "--serial", "emulator-5554", "--expect-serialno", "EMU-1", "--adb", make_adb(tmp_path))
    r = result(p)
    assert p.returncode == 0 and r["status"] == "present" and r["evidence"]["serialno"] == "EMU-1"


def test_device_golden_absent(tmp_path):
    p = probe("device.py", "--serial", "emulator-5554", "--adb", make_adb(tmp_path, serial_known=False))
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "device_absent"


def test_device_unauthorised(tmp_path):
    p = probe("device.py", "--serial", "emulator-5554", "--adb", make_adb(tmp_path, state="unauthorized"))
    assert p.returncode == 1 and result(p)["reason"] == "device_unauthorised"


def test_device_wrong_identity_carrier(tmp_path):
    """A device is present and in `device` state but is not the intended one (the 11.4.200 sibling-board case)."""
    p = probe("device.py", "--serial", "emulator-5554", "--expect-serialno", "EMU-1", "--adb", make_adb(tmp_path, serialno="OTHER-9"))
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "device_wrong_identity" and r["evidence"]["serialno"] == "OTHER-9"


def test_device_package_absent_for_androidtv_class(tmp_path):
    p = probe("device.py", "--serial", "emulator-5554", "--expect-serialno", "EMU-1", "--package", "com.catalogizer.androidtv",
              "--adb", make_adb(tmp_path, pkg=False))
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "device_absent" and r["evidence"]["package_absent"] == "com.catalogizer.androidtv"


def test_device_no_adb_binary_is_honestly_absent(tmp_path):
    p = probe("device.py", "--serial", "emulator-5554", "--adb", str(tmp_path / "no-such-adb"))
    r = result(p)
    assert p.returncode == 1 and r["reason"] == "device_absent" and "adb" in json.dumps(r["evidence"])


# ---- nas (stub smbclient + real TCP listener) ----
def make_smbclient(tmp_path, mode):
    s = tmp_path / "smbclient"
    s.write_text("""#!/bin/sh
case "%s" in
  ok) echo "Sharename Type Comment"; echo "media Disk"; exit 0;;
  rejected) echo "session setup failed: NT_STATUS_LOGON_FAILURE" >&2; exit 1;;
esac
""" % mode)
    s.chmod(s.stat().st_mode | stat.S_IEXEC)
    return str(s)


@pytest.fixture
def listener():
    srv = socket.socket()
    srv.bind(("127.0.0.1", 0))
    srv.listen(5)
    yield srv.getsockname()[1]
    srv.close()


def test_nas_golden_present(tmp_path, listener):
    p = probe("nas.py", "--host", "127.0.0.1", "--port", str(listener), "--user-env", "NAS_U", "--pass-env", "NAS_P",
              "--smbclient", make_smbclient(tmp_path, "ok"), env={"NAS_U": "ro-user", "NAS_P": "nas-secret-1"})
    r = result(p)
    assert p.returncode == 0 and r["status"] == "present" and "nas-secret-1" not in p.stdout + p.stderr


def test_nas_golden_absent_unreachable(tmp_path):
    p = probe("nas.py", "--host", "127.0.0.1", "--port", str(closed_port()), "--user-env", "NAS_U", "--pass-env", "NAS_P",
              "--smbclient", make_smbclient(tmp_path, "ok"), "--timeout", "2", env={"NAS_U": "u", "NAS_P": "p"})
    assert p.returncode == 1 and result(p)["reason"] == "service_unreachable"


def test_nas_credential_absent_and_rejected(tmp_path, listener):
    p = probe("nas.py", "--host", "127.0.0.1", "--port", str(listener), "--user-env", "NAS_U", "--pass-env", "NAS_P",
              "--smbclient", make_smbclient(tmp_path, "ok"))
    assert p.returncode == 1 and result(p)["reason"] == "credential_absent"
    p = probe("nas.py", "--host", "127.0.0.1", "--port", str(listener), "--user-env", "NAS_U", "--pass-env", "NAS_P",
              "--smbclient", make_smbclient(tmp_path, "rejected"), env={"NAS_U": "u", "NAS_P": "nas-secret-2"})
    assert p.returncode == 1 and result(p)["reason"] == "credential_rejected" and "nas-secret-2" not in p.stdout + p.stderr


# ---- metadata provider ----
def test_metadata_golden_present(server):
    p = probe("metadata.py", "--url", server + "/meta", "--key-env", "META_KEY", "--header", "X-Api-Key", env={"META_KEY": "good-key"})
    assert p.returncode == 0 and result(p)["status"] == "present"


def test_metadata_golden_absent_key_unset(server):
    p = probe("metadata.py", "--url", server + "/meta", "--key-env", "META_KEY", "--header", "X-Api-Key")
    assert p.returncode == 1 and result(p)["reason"] == "credential_absent"


def test_metadata_rejected_and_quota(server):
    p = probe("metadata.py", "--url", server + "/meta", "--key-env", "META_KEY", "--header", "X-Api-Key", env={"META_KEY": "bad-key"})
    assert p.returncode == 1 and result(p)["reason"] == "credential_rejected"
    p = probe("metadata.py", "--url", server + "/meta", "--key-env", "META_KEY", "--header", "X-Api-Key", env={"META_KEY": "quota-key"})
    assert p.returncode == 1 and result(p)["reason"] == "quota_exhausted"


# ---- vision ----
def test_vision_golden_present(server):
    p = probe("vision.py", "--hosts-env", "HELIX_VISION_HOSTS", env={"HELIX_VISION_HOSTS": server})
    assert p.returncode == 0 and result(p)["status"] == "present"


def test_vision_golden_absent_unset_and_unreachable():
    p = probe("vision.py", "--hosts-env", "HELIX_VISION_HOSTS")
    assert p.returncode == 1 and result(p)["reason"] == "service_unreachable"
    p = probe("vision.py", "--hosts-env", "HELIX_VISION_HOSTS", "--timeout", "2",
              env={"HELIX_VISION_HOSTS": "http://127.0.0.1:%d" % closed_port()})
    assert p.returncode == 1 and result(p)["reason"] == "service_unreachable"


# ---- container runtime (stub podman) ----
def make_podman(tmp_path, rootless):
    s = tmp_path / "podman"
    s.write_text("#!/bin/sh\necho %s\n" % rootless)
    s.chmod(s.stat().st_mode | stat.S_IEXEC)
    return str(s)


def test_runtime_golden_present_and_absent(tmp_path):
    p = probe("runtime.py", "--podman", make_podman(tmp_path, "true"))
    assert p.returncode == 0 and result(p)["status"] == "present"
    p = probe("runtime.py", "--podman", make_podman(tmp_path, "false"))
    assert p.returncode == 1 and result(p)["reason"] == "host_resource_unavailable"
    p = probe("runtime.py", "--podman", str(tmp_path / "missing"))
    assert p.returncode == 1 and result(p)["reason"] == "host_resource_unavailable"


def test_every_blocked_reason_is_in_the_closed_set(server, tmp_path):
    outs = [probe("service.py", "--url", "http://127.0.0.1:%d/health" % closed_port(), "--timeout", "2"),
            probe("credential.py", "--env", "NOPE_X"), probe("device.py", "--serial", "s", "--adb", str(tmp_path / "x")),
            probe("runtime.py", "--podman", str(tmp_path / "x"))]
    for p in outs:
        assert result(p)["reason"] in REASONS


def test_usage_error_exits_two():
    p = probe("service.py")
    assert p.returncode == 2
