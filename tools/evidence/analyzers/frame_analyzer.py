"""frame_analyzer.py - T218 (doc12 9.3). Deterministic frame analyzers on the standard library: blank frame, error overlay (a large red band), frozen sequence,
motion between frames, a colour oracle and a blob counter (calibrated on submodules/helix_qa/data/vision_gt/). No model, no network: the verdict never depends on
an LLM (11.4.269). An unreadable or empty input is a FAIL, never a clean result (11.4.201).
analyze(paths, expect_motion=False) -> {verdict pass|fail, reasons[], frame_index (first offending frame or None), blank, error_overlay, frozen, motion, red_fraction}
"""
import pngio

COLOURS = {
    "red": lambda r, g, b: r > 150 and g < 90 and b < 90,
    "blue": lambda r, g, b: b > 150 and r < 110 and g < 110,
    "green": lambda r, g, b: g > 150 and r < 110 and b < 110,
}
OVERLAY_RED_FRACTION = 0.30      # a frame that is >= 30% saturated red is an error overlay, not content
BLANK_DOMINANT_FRACTION = 0.999  # one colour covering >= 99.9% of the frame is blank
MIN_BLOB_PIXELS = 50


def colour_fraction(path, colour):
    w, h, px = pngio.read_rgb(path)
    pred = COLOURS[colour]
    return sum(1 for (r, g, b) in px if pred(r, g, b)) / float(w * h)


def count_blobs(path, colour):
    """Number of 4-connected regions of the colour with at least MIN_BLOB_PIXELS pixels."""
    w, h, px = pngio.read_rgb(path)
    pred = COLOURS[colour]
    mask = [pred(*p) for p in px]
    seen = bytearray(w * h)
    blobs = 0
    for start in range(w * h):
        if not mask[start] or seen[start]:
            continue
        stack, size = [start], 0
        seen[start] = 1
        while stack:
            i = stack.pop()
            size += 1
            x, y = i % w, i // w
            for j in ((i - 1) if x else -1, (i + 1) if x < w - 1 else -1, (i - w) if y else -1, (i + w) if y < h - 1 else -1):
                if j >= 0 and mask[j] and not seen[j]:
                    seen[j] = 1
                    stack.append(j)
        if size >= MIN_BLOB_PIXELS:
            blobs += 1
    return blobs


def _stats(px):
    n = len(px)
    counts = {}
    red = 0
    for p in px:
        counts[p] = counts.get(p, 0) + 1
        if COLOURS["red"](*p):
            red += 1
    return max(counts.values()) / float(n), red / float(n)


def _mean_abs_diff(a, b):
    if len(a) != len(b):
        return 255.0
    tot = 0
    for (r1, g1, b1), (r2, g2, b2) in zip(a, b):
        tot += abs(r1 - r2) + abs(g1 - g2) + abs(b1 - b2)
    return tot / (3.0 * len(a))


def analyze(paths, expect_motion=False):
    res = {"verdict": "fail", "reasons": [], "frame_index": None, "blank": False, "error_overlay": False, "frozen": False, "motion": 0.0, "red_fraction": 0.0}
    paths = list(paths)
    if not paths:
        res["reasons"].append("no_frames")
        return res
    frames = []
    for i, p in enumerate(paths):
        try:
            frames.append(pngio.read_rgb(p)[2])
        except (ValueError, OSError, Exception) as e:  # noqa: B014 - any decode failure is an unreadable frame
            res["reasons"].append("unreadable")
            res["frame_index"] = i
            res["detail"] = "%s: %s" % (p, str(e)[:100])
            return res
    first_blank = first_overlay = None
    max_red = 0.0
    for i, px in enumerate(frames):
        dom, red = _stats(px)
        max_red = max(max_red, red)
        if dom >= BLANK_DOMINANT_FRACTION and first_blank is None:
            first_blank = i
        if red >= OVERLAY_RED_FRACTION and first_overlay is None:
            first_overlay = i
    diffs = [_mean_abs_diff(frames[i], frames[i + 1]) for i in range(len(frames) - 1)]
    res["motion"] = sum(diffs) / len(diffs) if diffs else 0.0
    res["frozen"] = bool(diffs) and all(d == 0 for d in diffs)
    res["blank"] = first_blank is not None
    res["error_overlay"] = first_overlay is not None
    res["red_fraction"] = round(max_red, 4)
    idx = []
    if res["blank"]:
        res["reasons"].append("blank")
        idx.append(first_blank)
    if res["error_overlay"]:
        res["reasons"].append("error_overlay")
        idx.append(first_overlay)
    if expect_motion and res["frozen"]:
        res["reasons"].append("frozen")
        idx.append(0)
    res["frame_index"] = min(idx) if idx else None
    res["verdict"] = "fail" if res["reasons"] else "pass"
    return res
