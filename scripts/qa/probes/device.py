#!/usr/bin/env python3
"""device.py - T216. Probe: an Android (or Android TV) emulator/device is present, authorised and the INTENDED one (11.4.111, 11.4.200).
Usage: device.py --serial S [--expect-serialno X] [--package P] [--adb PATH] [--timeout S]
  adb -s S get-state must print `device`; with --expect-serialno, `getprop ro.serialno` must equal it (else device_wrong_identity: a present device that is
  not the intended one); with --package (the Android TV class: com.catalogizer.androidtv), `pm path P` must answer (else device_absent, evidence package_absent).
Blocked reasons: device_absent (no adb, no such device, offline, package absent), device_unauthorised, device_wrong_identity.
Owner decision: emulators only; a class with no emulator running is reported device_absent, never simulated."""
import argparse
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402


def adb(path, serial, args, timeout):
    return subprocess.run([path, "-s", serial] + args, capture_output=True, text=True, timeout=timeout)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--serial", required=True)
    ap.add_argument("--expect-serialno")
    ap.add_argument("--package")
    ap.add_argument("--adb", default="adb")
    ap.add_argument("--timeout", type=float, default=10)
    ns = ap.parse_args()
    ev = {"serial": ns.serial, "adb": ns.adb}
    try:
        p = adb(ns.adb, ns.serial, ["get-state"], ns.timeout)
    except FileNotFoundError:
        lib.finish("device", "blocked", "device_absent", dict(ev, error="adb binary not found"))
    except subprocess.TimeoutExpired:
        lib.finish("device", "blocked", "device_absent", dict(ev, error="adb get-state timed out"))
    text = (p.stdout + p.stderr).lower()
    if "unauthorized" in text:
        lib.finish("device", "blocked", "device_unauthorised", dict(ev, adb_says=(p.stderr or p.stdout).strip()[:200]))
    if p.returncode != 0 or p.stdout.strip() != "device":
        lib.finish("device", "blocked", "device_absent", dict(ev, rc=p.returncode, adb_says=(p.stderr or p.stdout).strip()[:200]))
    ev["state"] = "device"
    if ns.expect_serialno:
        try:
            q = adb(ns.adb, ns.serial, ["shell", "getprop", "ro.serialno"], ns.timeout)
        except subprocess.TimeoutExpired:
            lib.finish("device", "blocked", "device_absent", dict(ev, error="getprop timed out"))
        got = q.stdout.strip()
        ev["serialno"] = got
        ev["expected_serialno"] = ns.expect_serialno
        if q.returncode != 0 or got != ns.expect_serialno:
            lib.finish("device", "blocked", "device_wrong_identity", ev)
    else:
        ev["identity_checked"] = False
    if ns.package:
        try:
            r = adb(ns.adb, ns.serial, ["shell", "pm", "path", ns.package], ns.timeout)
        except subprocess.TimeoutExpired:
            lib.finish("device", "blocked", "device_absent", dict(ev, error="pm path timed out"))
        if r.returncode != 0 or "package:" not in r.stdout:
            lib.finish("device", "blocked", "device_absent", dict(ev, package_absent=ns.package))
        ev["package"] = ns.package
    lib.finish("device", "present", None, ev)


if __name__ == "__main__":
    main()
