"""ocr_analyzer.py - T218 (doc12 9.3, 11.4.160). Text assertions on recorded frames through the `tesseract` CLI.
check_text(paths, must_contain=(), must_not_contain=(), tesseract="tesseract", timeout=60) ->
  {status pass|fail|blocked, reason, engine, frames, found[], missing[], forbidden_found[], text_chars}
  blocked / host_resource_unavailable  the engine is absent on this host (only IMG-QA carries tesseract): never a pass, never a fake
  fail                                 a required text is missing, a forbidden text is present, or the engine read no text at all (a blind engine certifies nothing)
Texts are compared case-insensitively on whitespace-normalised OCR output of all frames together. OCR output is evidence, never a model verdict (11.4.269)."""
import os
import re
import shutil
import subprocess


def engine_path(tesseract):
    if os.sep in tesseract:
        return tesseract if os.path.isfile(tesseract) and os.access(tesseract, os.X_OK) else None
    return shutil.which(tesseract)


def engine_version(tesseract="tesseract"):
    exe = engine_path(tesseract)
    if not exe:
        return None
    try:
        p = subprocess.run([exe, "--version"], capture_output=True, text=True, timeout=20)
        return (p.stdout or p.stderr).splitlines()[0].strip() if (p.stdout or p.stderr) else "unknown"
    except (OSError, subprocess.TimeoutExpired):
        return None


def _norm(s):
    return re.sub(r"\s+", " ", s).strip().lower()


def check_text(paths, must_contain=(), must_not_contain=(), tesseract="tesseract", timeout=60):
    exe = engine_path(tesseract)
    res = {"status": "blocked", "reason": "host_resource_unavailable", "engine": tesseract, "frames": len(list(paths)), "found": [], "missing": [],
           "forbidden_found": [], "text_chars": 0}
    if not exe:
        res["detail"] = "tesseract not found"
        return res
    text = ""
    for p in paths:
        try:
            r = subprocess.run([exe, p, "stdout"], capture_output=True, text=True, timeout=timeout)
        except (OSError, subprocess.TimeoutExpired) as e:
            res.update(status="blocked", reason="host_resource_unavailable", detail="tesseract failed: %s" % str(e)[:100])
            return res
        if r.returncode != 0:
            res.update(status="blocked", reason="host_resource_unavailable", detail="tesseract rc=%d" % r.returncode)
            return res
        text += "\n" + r.stdout
    hay = _norm(text)
    res["text_chars"] = len(hay)
    res["reason"] = None
    res.pop("detail", None)
    res["found"] = [t for t in must_contain if _norm(t) in hay]
    res["missing"] = [t for t in must_contain if _norm(t) not in hay]
    res["forbidden_found"] = [t for t in must_not_contain if _norm(t) in hay]
    if not hay:
        res.update(status="fail", reason="no_text_recognised")
    elif res["missing"] or res["forbidden_found"]:
        res["status"] = "fail"
        res["reason"] = "missing_text" if res["missing"] else "forbidden_text"
    else:
        res["status"] = "pass"
    return res
