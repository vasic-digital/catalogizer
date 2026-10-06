#!/usr/bin/env python3
"""vision.py - T216. Probe: at least one vision model host named in HELIX_VISION_HOSTS (comma separated base URLs) answers a health request.
Usage: vision.py [--hosts-env HELIX_VISION_HOSTS] [--path /health] [--timeout S]
Blocked reason: service_unreachable (variable unset, empty, or no host answered 2xx). Advisory use only: a vision model never gives a verdict (11.4.269)."""
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--hosts-env", default="HELIX_VISION_HOSTS")
    ap.add_argument("--path", default="/health")
    ap.add_argument("--timeout", type=float, default=5)
    ns = ap.parse_args()
    raw = os.environ.get(ns.hosts_env, "")
    hosts = [h.strip().rstrip("/") for h in raw.split(",") if h.strip()]
    if not hosts:
        lib.finish("vision", "blocked", "service_unreachable", {"env": ns.hosts_env, "set": bool(raw), "hosts": 0})
    tried = []
    for h in hosts:
        try:
            code, _ = lib.http(h + ns.path, timeout=ns.timeout)
        except OSError as e:
            tried.append({"host": h, "error": str(e)[:120]})
            continue
        if 200 <= code < 300:
            lib.finish("vision", "present", None, {"env": ns.hosts_env, "host": h, "http_status": code})
        tried.append({"host": h, "http_status": code})
    lib.finish("vision", "blocked", "service_unreachable", {"env": ns.hosts_env, "tried": tried})


if __name__ == "__main__":
    main()
