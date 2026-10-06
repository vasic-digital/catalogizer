#!/usr/bin/env python3
"""metadata.py - T216. Probe: an external metadata provider (TMDB and similar) accepts one real authenticated call.
Usage: metadata.py --url URL --key-env VAR [--header NAME (default X-Api-Key)] [--timeout S]
Blocked reasons: credential_absent (variable unset/empty), credential_rejected (401/403), quota_exhausted (429), geo_restricted (451), service_unreachable."""
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", required=True)
    ap.add_argument("--key-env", required=True)
    ap.add_argument("--header", default="X-Api-Key")
    ap.add_argument("--timeout", type=float, default=5)
    ns = ap.parse_args()
    key = os.environ.get(ns.key_env, "")
    if not key:
        lib.finish("metadata", "blocked", "credential_absent", {"env": ns.key_env, "set": False})
    ev = {"url": ns.url, "key_env": ns.key_env}
    try:
        code, _ = lib.http(ns.url, headers={ns.header: key}, timeout=ns.timeout)
    except OSError as e:
        lib.finish("metadata", "blocked", "service_unreachable", dict(ev, error=str(e)[:200]))
    ev["http_status"] = code
    if 200 <= code < 300:
        lib.finish("metadata", "present", None, ev)
    reason = {401: "credential_rejected", 403: "credential_rejected", 429: "quota_exhausted", 451: "geo_restricted"}.get(code, "service_unreachable")
    lib.finish("metadata", "blocked", reason, ev)


if __name__ == "__main__":
    main()
