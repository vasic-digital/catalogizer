#!/usr/bin/env python3
# Native desktop QA scenario (see-before-type, constitution 11.4.193). Usage: scenario.py <outdir>
# Needs env QA_USER/QA_PW (never printed), QA_API_URL, QA_ALT_URL.
import os, sys, time, subprocess, re
os.environ["QA_OUT"] = sys.argv[1]
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "qa.py")).read())
A = os.environ["QA_API_URL"]; B = os.environ["QA_ALT_URL"]
CHK = {}
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

# S1 first-time setup
fn, w, t, ok = see("s1_start", expect=["Server URL"])
p = find(w, "http://localhost:8080"); assert p
click(*p); time.sleep(.4); typ(A)
saveclick("s2")
w, t, ok = dialog("s2_dialog_first")
check("C1_native_dialog_first_time", ok and "continue" in t.lower())
press(w, "Cancel")
fn, w, t, ok = see("s3_after_cancel", expect=["Auto-start"], absent=["Only continue", "Welcome to Catalogizer"])
check("C2_cancel_stays_on_settings", ok)
saveclick("s4")
w, t, ok = dialog("s4_dialog_again")
press(w, "Chang")
fn, w, t, ok = see("s5_login_screen", expect=["Sign in", "Connecting to"], absent=["Only continue"])
check("C3_confirm_applies_navigates_to_login", ok)
# S5 login
p = find(w, "Enter your username"); assert p
click(*p); time.sleep(.4); typ(os.environ["QA_USER"]); key("Tab"); time.sleep(.3)
typ(os.environ["QA_PW"], secret=True)
fn, w, t, ok = see("s6_filled", expect=["Username"])
key("Return"); time.sleep(6)
fn, w, t, ok = see("s7_home", expect=["Welcome back"], absent=["Login failed", "Sign in to access"])
check("C4_login_works_home_shown", ok)
# S6 change URL, cancel
assert goto_settings("s7b"), "Settings link never visible"
setfield("s8", B)
saveclick("s9")
w, t, ok = dialog("s9_dialog_change")
check("C5_change_dialog_has_current_and_new", ok and "current" in t.lower() and "new" in t.lower() and "38082" in t and "33080" in t)
press(w, "Cancel")
fn, w, t, ok = see("s10_after_cancel", expect=["Auto-start"], absent=["Only continue"])
check("C6_cancel_stays_on_settings", ok)
saveclick("s11")
w, t, ok = dialog("s11_dialog_again")
check("C7_old_url_still_current_after_cancel", ok and "current" in t.lower() and "38082" in t)
press(w, "Chang")
fn, w, t, ok = see("s12_after_confirm", expect=["Logout"], absent=["Only continue", "Sign in to access"])
check("C8_confirm_applies_session_ok", ok)
# S7 rejected URLs
first = True
for tag, bad in (("s13_file", "file:///etc/passwd"), ("s14_creds", "http://user:pw@evil.example")):
    if first:
        assert goto_settings(tag + "_nav"), "Settings link never visible"; first = False
    setfield(tag, bad)
    saveclick(tag + "_save")
    fn, w, t, ok = see(tag + "_result", expect=["Auto-start"], absent=["Only continue", "Catalogizer wants"])
    check("C9_%s_rejected_no_dialog" % tag, ok)
setfield("s15", A)
saveclick("s15_save")
w, t, ok = dialog("s15_dialog_after_rejects")
check("C10_url_unchanged_after_rejects", ok and "current" in t.lower() and "33080" in t)
press(w, "Cancel")
see("s16_end", absent=["Only continue"])
check("ALL", all(CHK.values()))
json.dump(CHK, open(os.path.join(OUT, "checks.json"), "w"), indent=1)
finish()
sys.exit(0 if CHK["ALL"] else 1)
