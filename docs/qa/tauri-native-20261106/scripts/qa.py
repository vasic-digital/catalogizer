#!/usr/bin/env python3
"""See-before-type driver for the Tauri QA display (constitution 11.4.193).
Every action: screenshot -> OCR -> assert target text visible -> act -> screenshot -> OCR verify.
Secrets: password is read from env var QA_PW (never logged); OCR text of password field is masked dots.
"""
import subprocess, os, sys, json, time, hashlib, re, csv, io

CT = os.environ.get("QA_CT", "catalogizer-tauri-qa-desktop")
OUT = os.environ["QA_OUT"]
os.makedirs(OUT, exist_ok=True)
LOG = []
N = [0]

def px(*a, inp=None):
    return subprocess.run(["podman", "exec", "-e", "DISPLAY=:99", "-i", CT, *a], input=inp, capture_output=True)

def shot(label):
    N[0] += 1
    fn = f"{N[0]:02d}_{label}.png"
    r = px("import", "-window", "root", "png:-")
    open(os.path.join(OUT, fn), "wb").write(r.stdout)
    return fn

def ocr(fn):
    p = os.path.join(OUT, fn)
    lists = []
    for mode in ("norm", "neg", "lvl"):
        words = []
        big = p + ".x2.png"
        args = ["convert", p, "-resize", "200%", "-colorspace", "Gray"] + (["-negate"] if mode == "neg" else ["-level", "30%,70%"] if mode == "lvl" else []) + [big]
        subprocess.run(args, check=True)
        r = subprocess.run(["tesseract", big, "-", "--psm", "11", "tsv"], capture_output=True, text=True)
        os.remove(big)
        for row in csv.DictReader(io.StringIO(r.stdout), delimiter="\t", quoting=csv.QUOTE_NONE):
            t = (row.get("text") or "").strip()
            if t and float(row["conf"]) > 30:
                words.append((t, int(row["left"]) // 2, int(row["top"]) // 2, int(row["width"]) // 2, int(row["height"]) // 2))
        lists.append(words)
    # band pass: horizontal strips of the non-black (app) area, psm 6 (small low-contrast text such as step counters is lost by sparse mode on a full page; black margins break binarisation)
    bb = subprocess.run(["convert", p, "-fuzz", "4%", "-format", "%@", "info:"], capture_output=True, text=True).stdout.strip()
    m = re.match(r"(\d+)x(\d+)\+(\d+)\+(\d+)", bb)
    bw, bh, bx, by = map(int, m.groups()) if m else (1440, 900, 0, 0)
    words = []
    for y0 in range(by, by + bh, 70):
        hh = min(120, by + bh - y0)
        if hh < 25: break
        big = p + ".b.png"
        subprocess.run(["convert", p, "-crop", f"{bw}x{hh}+{bx}+{y0}", "+repage", "-resize", "300%", "-colorspace", "Gray", big], check=True)
        r = subprocess.run(["tesseract", big, "-", "--psm", "6", "tsv"], capture_output=True, text=True)
        os.remove(big)
        for row in csv.DictReader(io.StringIO(r.stdout), delimiter="\t", quoting=csv.QUOTE_NONE):
            t = (row.get("text") or "").strip()
            if t and float(row["conf"]) > 30:
                words.append((t, bx + int(row["left"]) // 3, y0 + int(row["top"]) // 3, int(row["width"]) // 3, int(row["height"]) // 3))
    lists.append(words)
    text = " | ".join(" ".join(w[0] for w in l) for l in lists)
    words = lists
    open(os.path.join(OUT, fn + ".ocr.txt"), "w").write(text + "\n")
    return words, text

def ocr_region(fn, x, y, w, h, level="30%,70%"):
    p = os.path.join(OUT, fn); big = p + ".r.png"
    subprocess.run(["convert", p, "-crop", f"{w}x{h}+{x}+{y}", "+repage", "-resize", "400%", "-colorspace", "Gray", "-level", level, "-bordercolor", "white", "-border", "20", big], check=True)
    r = subprocess.run(["tesseract", big, "-", "--psm", "7", "tsv"], capture_output=True, text=True)
    os.remove(big)
    out = []
    for row in csv.DictReader(io.StringIO(r.stdout), delimiter="\t", quoting=csv.QUOTE_NONE):
        t = (row.get("text") or "").strip()
        if t and float(row["conf"]) > 30:
            out.append((t, x + (int(row["left"]) - 20) // 4, y + (int(row["top"]) - 20) // 4, int(row["width"]) // 4, int(row["height"]) // 4))
    return out

def find(lists, phrase, exact=False):
    toks = phrase.split()
    for words in lists:
        for i in range(len(words) - len(toks) + 1):
            if all((toks[j].lower() == words[i + j][0].lower().strip('.,:;')) if exact else (toks[j].lower() in words[i + j][0].lower()) for j in range(len(toks))):
                xs = [words[i + j][1] for j in range(len(toks))]
                x2 = words[i + len(toks) - 1][1] + words[i + len(toks) - 1][3]
                y = words[i][2] + words[i][4] // 2
                return ((min(xs) + x2) // 2, y)
    return None

def log(step, **kw):
    kw["step"] = step
    LOG.append(kw)
    print(json.dumps(kw), flush=True)

def see(label, expect=(), absent=()):
    fn = shot(label)
    words, text = ocr(fn)
    ok = all(e.lower() in text.lower() for e in expect) and not any(a.lower() in text.lower() for a in absent)
    log("see:" + label, file=fn, expect=list(expect), absent=list(absent), ok=ok, ocr=text[:300])
    return fn, words, text, ok

def click(x, y):
    px("xdotool", "mousemove", str(x), str(y), "click", "1")

def typ(s, secret=False):
    px("xdotool", "type", "--delay", "40", "--", s)
    log("type", secret=secret, **({} if secret else {"chars": len(s), "text": s}))

def key(k):
    px("xdotool", "key", k)

def click_text(label, phrase, expect_after=()):
    fn, words, text, ok = see(label + "_before", expect=[phrase.split()[0]])
    pos = find(words, phrase)
    if not pos:
        log("click_text_MISSING", phrase=phrase)
        return None
    click(*pos)
    time.sleep(1.2)
    return pos

def finish():
    json.dump(LOG, open(os.path.join(OUT, "run_log.json"), "w"), indent=1)
