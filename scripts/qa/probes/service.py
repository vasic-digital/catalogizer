#!/usr/bin/env python3
"""service.py - T216. Probe: a service answers AND identifies itself. `GET <url>` (a /health endpoint) must return JSON carrying the service's own build id.
Usage: service.py --url URL [--timeout S] [--id-key KEY]   (--id-key adds one more accepted identity field, e.g. a service that reports its build as `rev`)     Blocked reason: service_unreachable (no answer, or an answer that does not identify the service: the carrier case)."""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402

ID_KEYS = ("build_id", "buildId", "build", "commit")   # a bare `version` is not an identity: any service can print one (carrier case)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", required=True)
    ap.add_argument("--timeout", type=float, default=5)
    ap.add_argument("--id-key")
    ns = ap.parse_args()
    try:
        code, body = lib.http(ns.url, timeout=ns.timeout)
    except OSError as e:
        lib.finish("service", "blocked", "service_unreachable", {"url": ns.url, "error": str(e)[:200]})
    try:
        doc = json.loads(body.decode("utf-8", "replace"))
    except ValueError:
        doc = None
    ident = None
    if isinstance(doc, dict):
        for k in ((ns.id_key,) if ns.id_key else ()) + ID_KEYS:
            if isinstance(doc.get(k), str) and doc[k].strip():
                ident = (k, doc[k])
                break
    if code == 200 and ident:
        lib.finish("service", "present", None, {"url": ns.url, "http_status": code, "build_id": ident[1], "id_key": ident[0]})
    lib.finish("service", "blocked", "service_unreachable",
               {"url": ns.url, "http_status": code, "identified": False, "note": "the answer carries no build id: not the service (or not healthy)"})


if __name__ == "__main__":
    main()
