#!/usr/bin/env python3
"""selftest.py - T218 (doc12 9.3, 11.4.107 (10), document 06 section 10). Self-validation of the vision analyzers.
An analyzer is TRUSTED only after it FAILS the golden-bad recording and still PASSES the golden-good and the negative control (a legitimately different valid recording).
Legs: frame (stdlib frame analyzer), calibration (the colour oracle against submodules/helix_qa/data/vision_gt: red circle seen, three blue circles counted,
no red in the blue image), ocr (tesseract on the three recordings; BLOCKED where the engine is absent: never a pass, never faked).
Usage: selftest.py [--out FILE]        writes the evidence JSON (schema analyzers-selftest/1) and prints a one-line summary.
Exits: 0 every leg trusted; 4 the frame and calibration legs are trusted but the OCR leg is BLOCKED (engine absent); 1 any leg untrusted; 2 usage / fixtures missing."""
import argparse
import hashlib
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import frame_analyzer as fa  # noqa: E402
import ocr_analyzer as oa  # noqa: E402

FIX = os.path.join(HERE, "fixtures")
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
GT = os.path.join(ROOT, "submodules", "helix_qa", "data", "vision_gt")
CASES = (("golden_good", "Sign in", "pass"), ("golden_bad", "Sign in", "fail"), ("negative_control", "Welcome", "pass"))


def frames(name):
    d = os.path.join(FIX, name)
    return [os.path.join(d, f) for f in sorted(os.listdir(d)) if f.endswith(".png")]


def sha(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()


def leg(check):
    out = {}
    for name, _, want in CASES:
        out[name] = check(name)
    ok = all(out[n] == w for n, _, w in CASES)
    out["trusted"] = ok
    return out


def run_selftest(frame_analyzer=None, ocr_check=None, tesseract="tesseract"):
    fan = frame_analyzer or fa.analyze
    ocr = ocr_check or oa.check_text
    rep = {"schema": "analyzers-selftest/1", "analyzers": {}, "sha256_fixtures": {}}
    for name, _, _ in CASES:
        for p in frames(name):
            rep["sha256_fixtures"][os.path.relpath(p, ROOT)] = sha(p)
    rep["analyzers"]["frame"] = leg(lambda n: fan(frames(n))["verdict"])
    # calibration of the colour oracle on the vision_gt ground-truth images
    cal = {"red_circle_red_fraction": None, "three_blue_circles_red_fraction": None, "three_blue_circles_blobs": None, "red_circle_blue_blobs": None}
    try:
        for n in ("red_circle", "three_blue_circles", "text_helix", "spatial_left_right"):
            rep["sha256_fixtures"][os.path.relpath(os.path.join(GT, n + ".png"), ROOT)] = sha(os.path.join(GT, n + ".png"))
        cal["red_circle_red_fraction"] = round(fa.colour_fraction(os.path.join(GT, "red_circle.png"), "red"), 4)
        cal["three_blue_circles_red_fraction"] = round(fa.colour_fraction(os.path.join(GT, "three_blue_circles.png"), "red"), 4)
        cal["three_blue_circles_blobs"] = fa.count_blobs(os.path.join(GT, "three_blue_circles.png"), "blue")
        cal["red_circle_blue_blobs"] = fa.count_blobs(os.path.join(GT, "red_circle.png"), "blue")
        cal["trusted"] = cal["red_circle_red_fraction"] > 0.02 and cal["three_blue_circles_red_fraction"] < 0.001 and cal["three_blue_circles_blobs"] == 3 and cal["red_circle_blue_blobs"] == 0
    except (OSError, ValueError) as e:
        cal["trusted"] = False
        cal["error"] = str(e)[:150]
    rep["analyzers"]["calibration"] = cal
    # OCR leg
    probe = ocr(frames("golden_good"), must_contain=[CASES[0][1]], tesseract=tesseract) if ocr_check or oa.engine_path(tesseract) else {"status": "blocked", "reason": "host_resource_unavailable"}
    if probe["status"] == "blocked":
        rep["analyzers"]["ocr"] = {"status": "blocked", "reason": probe.get("reason") or "host_resource_unavailable", "trusted": False,
                                   "note": "tesseract is carried by IMG-QA only; this leg is BLOCKED here, never passed"}
    else:
        def ocr_verdict(n):
            exp = [c[1] for c in CASES if c[0] == n][0]
            return ocr(frames(n), must_contain=[exp], tesseract=tesseract)["status"]
        o = leg(ocr_verdict)
        o["status"] = "ran"
        o["engine_version"] = oa.engine_version(tesseract)
        rep["analyzers"]["ocr"] = o
    return rep


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--out")
    try:
        ns = ap.parse_args(argv)
    except SystemExit as e:
        sys.exit(2 if e.code not in (0, None) else 0)
    for n, _, _ in CASES:
        if not os.path.isdir(os.path.join(FIX, n)):
            sys.stderr.write("selftest: fixture directory %s is missing\n" % n)
            sys.exit(2)
    rep = run_selftest()
    a = rep["analyzers"]
    base_ok = a["frame"]["trusted"] and a["calibration"]["trusted"]
    if base_ok and a["ocr"].get("trusted"):
        code = 0
    elif base_ok and a["ocr"]["status"] == "blocked":
        code = 4
    else:
        code = 1
    rep["exit_code"] = code
    if ns.out:
        with open(ns.out, "w") as fh:
            json.dump(rep, fh, indent=1, sort_keys=True)
            fh.write("\n")
    print("analyzers selftest: frame=%s calibration=%s ocr=%s exit=%d" % (
        "trusted" if a["frame"]["trusted"] else "UNTRUSTED", "trusted" if a["calibration"]["trusted"] else "UNTRUSTED",
        a["ocr"]["status"] if a["ocr"]["status"] == "blocked" else ("trusted" if a["ocr"]["trusted"] else "UNTRUSTED"), code))
    sys.exit(code)


if __name__ == "__main__":
    main()
