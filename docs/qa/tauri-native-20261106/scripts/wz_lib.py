import os, sys, time, subprocess, re
os.environ.setdefault("QA_CT", "catalogizer-tauri-qa-wizard")
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "qa.py")).read())
def wheel(n, btn="5", at=(720, 500)):
    px("xdotool", "mousemove", str(at[0]), str(at[1]))
    for _ in range(n): px("xdotool", "click", btn)
    time.sleep(.8)
def find_scroll(label, phrase, exact=True, tries=6, expect=()):
    """scroll down until OCR sees phrase (see-before-act); returns (pos, words, text)"""
    for i in range(tries):
        fn, w, t, ok = see("%s_look%d" % (label, i), expect=expect)
        p = find(w, phrase, exact=exact)
        if p: return p, w, t
        wheel(8)
    return None, None, None
def nav(label, direction="next", expect_step=None):
    """click Next/Previous anchored on the OCR'd 'Step N of 4' label (button text itself is not OCR-readable: white on blue)"""
    p, w, t = find_scroll(label + "_nav", "Step", tries=8)
    assert p, "Step label never visible"
    dx = 436 if direction == "next" else -400
    click(p[0] + dx, p[1]); time.sleep(2)
    fn, w, t, ok = see(label + "_after", expect=[expect_step] if expect_step else [])
    return fn, w, t, ok
