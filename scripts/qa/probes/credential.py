#!/usr/bin/env python3
"""credential.py - T216. Probe: a credential, referenced by variable NAME only, is present and (when a login URL is given) really accepted.
Usage: credential.py --env PASS_VAR [--user-env USER_VAR --login-url URL] [--timeout S]
Blocked reasons: credential_absent (variable unset or empty), credential_rejected (login answered 401/403), service_unreachable (login service down).
The values are read from the environment, sent only to the login URL, and appear nowhere in the output."""
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--env", required=True)
    ap.add_argument("--user-env")
    ap.add_argument("--login-url")
    ap.add_argument("--timeout", type=float, default=5)
    ns = ap.parse_args()
    pw = os.environ.get(ns.env, "")
    if not pw:
        lib.finish("credential", "blocked", "credential_absent", {"env": ns.env, "set": False})
    if not ns.login_url:
        lib.finish("credential", "present", None, {"env": ns.env, "set": True, "login_checked": False})
    user = os.environ.get(ns.user_env, "") if ns.user_env else ""
    if not user:
        lib.finish("credential", "blocked", "credential_absent", {"env": ns.user_env, "set": False, "needed_for": "login"})
    try:
        code, _ = lib.http(ns.login_url, "POST", body={"username": user, "password": pw}, timeout=ns.timeout)
    except OSError as e:
        lib.finish("credential", "blocked", "service_unreachable", {"login_url": ns.login_url, "error": str(e)[:200]})
    ev = {"env": ns.env, "user_env": ns.user_env, "login_url": ns.login_url, "http_status": code, "login_checked": True}
    if 200 <= code < 300:
        lib.finish("credential", "present", None, ev)
    if code in (401, 403):
        lib.finish("credential", "blocked", "credential_rejected", ev)
    lib.finish("credential", "blocked", "service_unreachable", dict(ev, note="login answered an unexpected status"))


if __name__ == "__main__":
    main()
