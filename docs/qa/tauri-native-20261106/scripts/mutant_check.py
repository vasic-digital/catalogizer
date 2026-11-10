# R5 mutant (always approve, native dialog never shown): the live first-time check must FAIL.
import os, sys
os.environ["QA_OUT"] = sys.argv[1]; os.environ["QA_CT"] = "catalogizer-tauri-qa-mutant"
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "scenario_lib.py")).read())
A = os.environ["QA_API_URL"]
fn, w, t, ok = see("m1_start", expect=["Server URL"])
p = find(w, "http://localhost:8080"); assert p
click(*p); time.sleep(.4); typ(A)
saveclick("m2")
fn, w, t, ok = see("m3_after_save", expect=["Only continue"])
dialog_shown = ok
log("MUTANT_dialog_shown", shown=dialog_shown)
fn, w, t, ok = see("m4_state", expect=["Sign in", "Connecting to"])
log("MUTANT_went_to_login_without_confirmation", went=ok)
finish()
# live check C1 semantics: dialog must appear. Mutant => FAIL expected => exit 0 means "mutant killed".
sys.exit(0 if (not dialog_shown and ok) else 1)
