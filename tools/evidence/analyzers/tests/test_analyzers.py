"""T218 - tests of the vision analyzers under tools/evidence/analyzers (doc12 9.3, doc06 10, 11.4.107 (10)).
Golden-good (known text present), golden-bad (text missing + error overlay + frozen) and negative control (a legitimately different valid recording).
The analyzer is trusted only after it FAILS the golden-bad. The vision_gt ground-truth images of submodules/helix_qa calibrate the colour oracle.
Run through `TIC tooling unit` (IMG-TESTUTIL: python3 stdlib only; the OCR leg needs tesseract, which only IMG-QA carries: its test uses a tesseract STUB
CLI to prove the analyzer's own logic, and the real engine leg is reported BLOCKED where tesseract is absent, never faked).
"""
import importlib
import json
import os
import stat
import subprocess
import sys

import pytest

HERE = os.path.dirname(os.path.abspath(__file__))
AN = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(AN)))
FIX = os.path.join(AN, "fixtures")
GT = os.path.join(ROOT, "submodules", "helix_qa", "data", "vision_gt")
sys.path.insert(0, AN)


def frames(name):
    d = os.path.join(FIX, name)
    return [os.path.join(d, f) for f in sorted(os.listdir(d)) if f.endswith(".png")]


def mod(name):
    return importlib.import_module(name)


def test_modules_exist():
    for n in ("pngio.py", "frame_analyzer.py", "ocr_analyzer.py", "selftest.py"):
        assert os.path.isfile(os.path.join(AN, n)), "tools/evidence/analyzers/%s is absent (T218 not implemented)" % n


def test_fixtures_exist():
    for n in ("golden_good", "golden_bad", "negative_control"):
        assert len(frames(n)) == 3


def test_pngio_reads_vision_gt_ground_truth():
    pngio = mod("pngio")
    w, h, px = pngio.read_rgb(os.path.join(GT, "red_circle.png"))
    assert (w, h) == (300, 300) and len(px) == 300 * 300
    w, h, px = pngio.read_rgb(os.path.join(GT, "spatial_left_right.png"))   # a palette PNG
    assert (w, h) == (400, 200) and len(set(px)) == 3
    w, h, px = pngio.read_rgb(os.path.join(GT, "text_helix.png"))           # a greyscale PNG
    assert (w, h) == (400, 150)


def test_colour_oracle_calibrated_on_vision_gt():
    fa = mod("frame_analyzer")
    red = fa.colour_fraction(os.path.join(GT, "red_circle.png"), "red")
    blue_img_red = fa.colour_fraction(os.path.join(GT, "three_blue_circles.png"), "red")
    assert red > 0.02, red                      # the red circle is seen
    assert blue_img_red < 0.001, blue_img_red   # negative control: no red in the blue-circles image
    assert fa.count_blobs(os.path.join(GT, "three_blue_circles.png"), "blue") == 3
    assert fa.count_blobs(os.path.join(GT, "red_circle.png"), "blue") == 0


def test_frame_analyzer_golden_good_passes():
    fa = mod("frame_analyzer")
    r = fa.analyze(frames("golden_good"))
    assert r["verdict"] == "pass", r
    assert not r["blank"] and not r["error_overlay"] and not r["frozen"] and r["motion"] > 0


def test_frame_analyzer_golden_bad_fails_with_pinpoint():
    fa = mod("frame_analyzer")
    r = fa.analyze(frames("golden_bad"))
    assert r["verdict"] == "fail", r
    assert r["error_overlay"] and r["frozen"]
    assert "error_overlay" in r["reasons"] and r["frame_index"] == 0     # a pinpoint of the first offending frame


def test_frame_analyzer_negative_control_passes():
    fa = mod("frame_analyzer")
    r = fa.analyze(frames("negative_control"))
    assert r["verdict"] == "pass", r


def test_blank_frame_is_detected(tmp_path):
    fa = mod("frame_analyzer")
    import zlib, struct
    def png(path, w, h, rgb):
        raw = b"".join(b"\x00" + bytes(rgb) * w for _ in range(h))
        def chunk(t, d):
            c = struct.pack(">I", len(d)) + t + d
            return c + struct.pack(">I", zlib.crc32(t + d) & 0xffffffff)
        open(path, "wb").write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))
    paths = []
    for i in range(3):
        p = str(tmp_path / ("b%d.png" % i)); png(p, 64, 36, (0, 0, 0)); paths.append(p)
    r = fa.analyze(paths)
    assert r["verdict"] == "fail" and r["blank"]


def test_empty_or_unreadable_input_is_blind_not_clean(tmp_path):
    fa = mod("frame_analyzer")
    assert fa.analyze([])["verdict"] == "fail"
    bad = tmp_path / "x.png"
    bad.write_bytes(b"not a png")
    r = fa.analyze([str(bad)])
    assert r["verdict"] == "fail" and "unreadable" in r["reasons"]


def make_tesseract(tmp_path, text):
    s = tmp_path / "tesseract"
    s.write_text("#!/bin/sh\n# stub: tesseract IN stdout\ncat <<'EOF'\n%s\nEOF\n" % text)
    s.chmod(s.stat().st_mode | stat.S_IEXEC)
    return str(s)


def test_ocr_blocked_when_engine_absent(tmp_path):
    ocr = mod("ocr_analyzer")
    r = ocr.check_text(frames("golden_good"), must_contain=["Sign in"], tesseract=str(tmp_path / "no-tesseract"))
    assert r["status"] == "blocked" and r["reason"] == "host_resource_unavailable"


def test_ocr_logic_with_a_stub_engine(tmp_path):
    ocr = mod("ocr_analyzer")
    good = make_tesseract(tmp_path, "Sign in\nContinue")
    assert ocr.check_text(frames("golden_good"), must_contain=["Sign in"], tesseract=good)["status"] == "pass"
    r = ocr.check_text(frames("golden_good"), must_contain=["Sign in"], must_not_contain=["Continue"], tesseract=good)
    assert r["status"] == "fail" and "Continue" in json.dumps(r)
    r = ocr.check_text(frames("golden_bad"), must_contain=["Sign in"], tesseract=make_tesseract(tmp_path, "ERROR 500"))
    assert r["status"] == "fail" and r["missing"] == ["Sign in"]


def test_ocr_empty_output_is_not_a_pass(tmp_path):
    ocr = mod("ocr_analyzer")
    r = ocr.check_text(frames("golden_good"), must_contain=[], tesseract=make_tesseract(tmp_path, ""))
    assert r["status"] != "pass"      # a blind engine (no text at all) never certifies a recording


def test_selftest_trusts_only_after_failing_golden_bad(tmp_path):
    st = mod("selftest")
    rep = st.run_selftest()
    fl = rep["analyzers"]["frame"]
    assert fl["golden_good"] == "pass" and fl["golden_bad"] == "fail" and fl["negative_control"] == "pass" and fl["trusted"] is True
    # an analyzer that passes everything is NOT trusted (it cannot fail the golden-bad)
    rep2 = st.run_selftest(frame_analyzer=lambda paths, **kw: {"verdict": "pass", "reasons": []})
    assert rep2["analyzers"]["frame"]["trusted"] is False and rep2["analyzers"]["frame"]["golden_bad"] == "pass"
    # an analyzer that fails everything is not trusted either (false-positive: the negative control)
    rep3 = st.run_selftest(frame_analyzer=lambda paths, **kw: {"verdict": "fail", "reasons": ["x"]})
    assert rep3["analyzers"]["frame"]["trusted"] is False


def test_selftest_cli_writes_evidence_and_exit_code(tmp_path):
    out = tmp_path / "analyzers_selftest.json"
    p = subprocess.run([sys.executable, "-I", os.path.join(AN, "selftest.py"), "--out", str(out)], capture_output=True, text=True)
    rep = json.loads(out.read_text())
    assert rep["schema"] == "analyzers-selftest/1" and rep["analyzers"]["frame"]["trusted"] is True
    ocr_status = rep["analyzers"]["ocr"]["status"]
    # exit 0 only when every leg is trusted; a leg whose engine is absent is BLOCKED (exit 4), never a silent pass
    if ocr_status == "blocked":
        assert p.returncode == 4 and rep["analyzers"]["ocr"]["reason"] == "host_resource_unavailable"
    else:
        assert p.returncode == 0 and rep["analyzers"]["ocr"]["trusted"] is True
    assert len(rep["sha256_fixtures"]) >= 9      # identity: the fixture files the verdict was computed on
