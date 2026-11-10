import os, sys, time, subprocess, re
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "qa.py")).read())
def check(name, ok, note=""):
    CHK[name] = bool(ok); log("CHECK:" + name, ok=bool(ok), note=note)
def saveclick(label):
    for _ in range(10): px("xdotool", "click", "5")
    px("xdotool", "mousemove", "1100", "400")
    for _ in range(10): px("xdotool", "click", "5")
    time.sleep(1)
    fn, w, t, ok = see(label + "_scrolled", expect=["Auto-start"])
    p = None
    for ph in ("Save Settings", "Save", "ttings", "Settings"):
        for l in w:
            for (tt, x, y, ww, hh) in l:
                if y > 600 and (tt.lower() == ph.lower() if ph in ("Save", "Settings") else ph.lower() in tt.lower()):
                    p = p or (x + ww // 2, y + hh // 2)
        if p: break
    if not p:
        rw = ocr_region(fn, 520, 770, 800, 90)
        log("save_region_ocr", words=[t[0] for t in rw])
        for i, (tt, x, y, ww, hh) in enumerate(rw):
            if tt.lower().startswith("save") or tt.lower() == "settings":
                p = (x + ww // 2, y + hh // 2); break
    assert p, "Save button not visible"
    log("save_button_at", at=p)
    click(*p); time.sleep(2.5)
def setfield(label, url):
    for _ in range(10): px("xdotool", "click", "4")
    time.sleep(.8)
    fn, w, t, ok = see(label + "_top", expect=["Server URL"])
    p = find(w, "Server URL")
    assert p, "Server URL label not visible"
    click(550, p[1] + 40); time.sleep(.4)
    key("ctrl+a"); typ(url)
    see(label + "_typed", expect=["Server URL"])
def dialog(label):
    fn, w, t, ok = see(label, expect=["Only continue"])
    return w, t, ok
def dlg_geom():
    r = px("sh", "-c", "for id in $(xdotool search --name 'Confirm server change'); do xdotool getwindowgeometry $id; done")
    out = r.stdout.decode()
    m = re.findall(r"Position: (\d+),(\d+).*?Geometry: (\d+)x(\d+)", out, re.S)
    return [tuple(map(int, g)) for g in m]
def press(w, word):
    c = find(w, "Cancel", exact=True)
    assert c, "Cancel not visible"
    g = dlg_geom()
    log("native_dialog_windows", geometries=g)
    assert g, "no native window titled 'Confirm server change' (X11)"
    x, y, gw, gh = max(g, key=lambda t: t[2] * t[3])
    # button row: both buttons share the OCR'd Cancel row; halves of the dialog width
    p = (x + gw // 4, c[1]) if word == "Chang" else (x + 3 * gw // 4, c[1])
    log("press", button=word, at=p)
    click(*p); time.sleep(2.5)

def goto_settings(label):
    # sidebar user section (Settings/Logout) sits at the bottom of the full-height page: scroll until OCR sees it
    px("xdotool", "mousemove", "800", "500")
    for i in range(12):
        for _ in range(15): px("xdotool", "click", "5")
        time.sleep(.6)
        fn, w, t, ok = see("%s_scan%d" % (label, i), expect=["Logout"])
        if ok:
            p = None
            for l in w:
                for (tt, x, y, ww, hh) in l:
                    if tt.lower() == "settings" and x < 400:
                        p = p or (x + ww // 2, y + hh // 2)
            if p:
                click(*p); time.sleep(2); return True
    return False

