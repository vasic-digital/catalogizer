# Dialog closed with the window X (and with Escape): must behave as Cancel (stay on Settings, no navigation).
import os, sys
os.environ["QA_OUT"] = sys.argv[1]
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "scenario_lib.py")).read())
A = os.environ["QA_API_URL"]
fn, w, t, ok = see("x1_start", expect=["Server URL"])
p = find(w, "http://localhost:8080"); click(*p); time.sleep(.4); typ(A)
res = {}
for how in ("window_X", "Escape"):
    saveclick("x_" + how)
    fn, w, t, ok = see("x_%s_dialog" % how, expect=["Only continue"])
    res[how + "_dialog_shown"] = ok
    g = dlg_geom(); log("geom", g=g)
    x, y, gw, gh = max(g, key=lambda q: q[2] * q[3])
    if how == "window_X":
        click(x + gw - 12, y - 31)  # titlebar X (decoration sits above the client area)
    else:
        key("Escape")
    time.sleep(2.5)
    fn, w, t, ok = see("x_%s_after" % how, expect=["Auto-start"], absent=["Only continue", "Sign in to access"])
    res[how + "_stays_on_settings_no_navigation"] = ok
log("RESULT", **res)
json.dump(res, open(os.path.join(OUT, "checks.json"), "w"), indent=1)
finish()
sys.exit(0 if all(res.values()) else 1)
