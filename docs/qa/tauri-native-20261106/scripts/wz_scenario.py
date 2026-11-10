#!/usr/bin/env python3
"""Installer-wizard full flow: Welcome -> Protocol(Local Files) -> Local config -> Config Management (Save via native dialog) -> Summary.
WZ_MODE=red  : expects the pre-fix ACL failure on Save ("not allowed by ACL").
WZ_MODE=green: expects the native save dialog, a written JSON file, and no error banner.
Every action is see-before-act (11.4.193): screenshot + OCR, assert target text visible, act, screenshot + OCR verify."""
import sys, json, os, time
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "wz_lib.py")).read())
MODE = os.environ.get("WZ_MODE", "green")
CHK = {}
def chk(name, ok, **kw):
    CHK[name] = bool(ok); log("CHECK " + name, ok=bool(ok), **kw)

# W1 welcome screen
fn, w, t, ok = see("welcome", expect=["Welcome", "Installation Wizard"])
chk("W1_welcome_visible", ok)
fn, w, t, ok = nav("w1", "next")
chk("W2_next_to_protocol_step2", ok)
# protocol selection: pick Local Files
p, w, t = find_scroll("proto", "Local Files", exact=False, tries=6)
chk("W3_local_files_card_visible", p is not None)
click(*p); time.sleep(1.2)
fn, w, t, ok = see("local_selected", expect=["Local Files"])
fn, w, t, ok = nav("w2", "next")
chk("W4_next_to_local_config_step3", ok)
fn, w, t, ok = see("local_cfg", expect=["Configured Sources"])
chk("W5_local_source_listed", ok, ocr=t[:120])
fn, w, t, ok = nav("w3", "next")
chk("W6_next_to_config_mgmt_step4", ok)
wheel(60, btn="4")
fn, w, t, ok = see("cfgmgmt_top", expect=["Save Configuration", "Load Configuration"])
chk("W7_save_and_load_buttons_visible", ok)
pos = find(w, "Save Configuration")
click(*pos); time.sleep(3)
fn, w, t, ok = see("after_save_click")
low = t.lower()
if MODE == "red":
    chk("W8_RED_acl_error_shown", "not allowed" in low and "acl" in low)
else:
    chk("W8_no_acl_error", "not allowed" not in low)
    wins = px("sh", "-c", "for w in $(xdotool search --onlyvisible --name .); do xdotool getwindowname $w; done").stdout.decode()
    log("windows", names=wins.split("\n"))
    chk("W9_native_save_dialog_open", "Save Configuration File" in wins, ocr=t[:160])
    if "Save Configuration File" in wins:
        # dialog is up and seen; the filename field has focus in the GTK chooser: replace its content with a path in a throwaway dir
        px("sh", "-c", "mkdir -p /tmp/h/wz_out && rm -f /tmp/h/wz_out/qa-config.json")
        key("ctrl+a"); typ("/tmp/h/wz_out/qa-config.json"); time.sleep(1)
        fn, w, t, ok = see("dialog_typed", expect=["qa-config.json"])
        chk("W10_dialog_shows_typed_path", ok)
        key("Return"); time.sleep(3)
        r = px("sh", "-c", "test -s /tmp/h/wz_out/qa-config.json && sha256sum /tmp/h/wz_out/qa-config.json && python3 -c \"import json;d=json.load(open('/tmp/h/wz_out/qa-config.json'));print('keys',sorted(d.keys()),'accesses',len(d.get('accesses',[])),'sources',len(d.get('sources',[])))\"")
        out = r.stdout.decode(); log("saved_file", out=out.strip().replace("\n", " | "))
        chk("W11_config_file_written_and_valid_json", r.returncode == 0 and "sources" in out)
        fn, w, t, ok = see("after_dialog", absent=["not allowed"])
        chk("W12_no_error_banner_after_save", ok)
fn, w, t, ok = nav("w4", "next")
chk("W13_next_to_summary_step5", ok)
fn, w, t, ok = see("summary", expect=["Wizard Complete"])
chk("W14_summary_visible", ok)
json.dump(CHK, open(os.path.join(OUT, "checks.json"), "w"), indent=1)
finish()
print("RESULT", MODE, "ALL_OK=%s" % all(CHK.values()), json.dumps(CHK))
