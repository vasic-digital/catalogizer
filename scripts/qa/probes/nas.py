#!/usr/bin/env python3
"""nas.py - T216. Probe: a NAS / SMB source is reachable and the supplied credential (variable NAMES) really lists the shares.
Usage: nas.py --host H [--port 445] --user-env U --pass-env P [--smbclient PATH] [--timeout S]
Blocked reasons: credential_absent, service_unreachable (TCP connect or listing failed for a non-credential reason), credential_rejected
(NT_STATUS_LOGON_FAILURE / ACCESS_DENIED), host_resource_unavailable (no smbclient on the host). The password goes to smbclient through the
PASSWD environment variable, never argv; it appears nowhere in the output. For the Synology read-only account use SYNOLOGY_* variable names."""
import argparse
import os
import socket
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", required=True)
    ap.add_argument("--port", type=int, default=445)
    ap.add_argument("--user-env", required=True)
    ap.add_argument("--pass-env", required=True)
    ap.add_argument("--smbclient", default="smbclient")
    ap.add_argument("--timeout", type=float, default=8)
    ns = ap.parse_args()
    user, pw = os.environ.get(ns.user_env, ""), os.environ.get(ns.pass_env, "")
    for name, val in ((ns.user_env, user), (ns.pass_env, pw)):
        if not val:
            lib.finish("nas", "blocked", "credential_absent", {"env": name, "set": False})
    ev = {"host": ns.host, "port": ns.port, "user_env": ns.user_env, "pass_env": ns.pass_env}
    try:
        socket.create_connection((ns.host, ns.port), timeout=ns.timeout).close()
    except OSError as e:
        lib.finish("nas", "blocked", "service_unreachable", dict(ev, error=str(e)[:200]))
    env = dict(os.environ, PASSWD=pw)
    try:
        p = subprocess.run([ns.smbclient, "-L", "//%s" % ns.host, "-p", str(ns.port), "-U", user], capture_output=True, text=True, timeout=ns.timeout + 10, env=env)
    except FileNotFoundError:
        lib.finish("nas", "blocked", "host_resource_unavailable", dict(ev, error="smbclient not found"))
    except subprocess.TimeoutExpired:
        lib.finish("nas", "blocked", "service_unreachable", dict(ev, error="share listing timed out"))
    out = (p.stdout + p.stderr)
    if p.returncode == 0:
        lib.finish("nas", "present", None, dict(ev, listing=True))
    if "NT_STATUS_LOGON_FAILURE" in out or "NT_STATUS_ACCESS_DENIED" in out:
        lib.finish("nas", "blocked", "credential_rejected", dict(ev, smb_status=[w for w in out.split() if w.startswith("NT_STATUS_")][:1]))
    lib.finish("nas", "blocked", "service_unreachable", dict(ev, rc=p.returncode, smb_status=[w for w in out.split() if w.startswith("NT_STATUS_")][:1]))


if __name__ == "__main__":
    main()
