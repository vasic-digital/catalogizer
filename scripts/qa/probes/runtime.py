#!/usr/bin/env python3
"""runtime.py - T216. Probe: a rootless container runtime is usable (`podman info` reports Rootless true).
Usage: runtime.py [--podman PATH] [--timeout S]     Blocked reason: host_resource_unavailable (no podman, it fails, or it is not rootless)."""
import argparse
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--podman", default="podman")
    ap.add_argument("--timeout", type=float, default=20)
    ns = ap.parse_args()
    try:
        p = subprocess.run([ns.podman, "info", "--format", "{{.Host.Security.Rootless}}"], capture_output=True, text=True, timeout=ns.timeout)
    except FileNotFoundError:
        lib.finish("runtime", "blocked", "host_resource_unavailable", {"podman": ns.podman, "error": "podman not found"})
    except subprocess.TimeoutExpired:
        lib.finish("runtime", "blocked", "host_resource_unavailable", {"podman": ns.podman, "error": "podman info timed out"})
    out = p.stdout.strip()
    if p.returncode == 0 and out == "true":
        lib.finish("runtime", "present", None, {"podman": ns.podman, "rootless": True})
    lib.finish("runtime", "blocked", "host_resource_unavailable", {"podman": ns.podman, "rc": p.returncode, "rootless": out or None, "stderr": p.stderr.strip()[:200]})


if __name__ == "__main__":
    main()
