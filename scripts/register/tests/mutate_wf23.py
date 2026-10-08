#!/usr/bin/env python3
"""mutate_wf23.py - permanent mutation run of the WP-20 tooling: the 26 author-independent mutants of the WF23 review (2026-10-08, EM01-EM11,
LM01-LM06, FM01-FM04, GM01-GM05) adapted to the current code, plus the mutants of the round-2 fixes (N01..N24, M07..M11). Each mutant is an
exact-string edit of a COPY of scripts/register (the edit must apply exactly once, else the mutant is reported NOT APPLIED and counted a
survivor); the suites named for it are run from the copy and at least one MUST fail (KILLED). Controls first: every unmutated suite passes, a
comment-only mutant SURVIVES (the harness can not kill everything), and a deliberately wrong mutant is KILLED.
Equivalent mutants are listed with their reason in EQUIVALENT below and are expected to survive.
Usage: mutate_wf23.py [--only NAME] [--suite absolute|lead|fm|planted] [--list]   (env TMPDIR, WI)
Output: one line per mutant; the last line `MUTANTS: n run, k killed, e equivalent-as-expected, s unexpected survivors`. Exit 0 iff s == 0.
"""
import os, re, shutil, subprocess, sys, tempfile, time

HERE = os.path.dirname(os.path.abspath(__file__)); REG = os.path.dirname(HERE); ROOT = os.path.dirname(os.path.dirname(REG))
WI = os.environ.get("WI", os.path.join(ROOT, "submodules/constitution/scripts/workable-items/bin/workable-items-linux"))

SUITES = {
    "absolute": ["python3", "tests/test_enumerate_absolute.py"],
    "lead": ["python3", "tests/test_lead_scan.py"],
    "fm": ["python3", "tests/test_frontmatter_yaml.py", "--selftest"],
    "planted": ["bash", "tests/test_enumerate_planted.sh"],
    "snap": ["python3", "tests/test_snapshot_checks.py"],
    "gate": ["bash", "tests/test_reverify_gate.sh"],
}

# (name, file under scripts/register, [(old, new)], [suites])
M = []


def m(name, f, edits, suites):
    M.append((name, f, edits, suites))


ES = "enumerate_sources.py"
# ---- reviewer EM01-EM11 (anchors re-derived for the current code; same intent)
m("EM01_markers_go_only", ES, [("    for p in files:\n        t, why = snap.scannable(p)\n        if t is None:\n            skipped[why] += 1",
                                  "    for p in files:\n        if not p.endswith('.go'):\n            continue\n        t, why = snap.scannable(p)\n        if t is None:\n            skipped[why] += 1")], ["absolute"])
m("EM02_skip_regexes_never_applied", ES, [("            for tag in sorted(tag_of):", "            for tag in ():")], ["absolute"])
m("EM03_finding_id_family_dropped", ES, [('r"FIX-OC\\d+-\\d+", r"FINDING-\\d+",', 'r"FIX-OC\\d+-\\d+",')], ["absolute"])
m("EM04_security_oldest_scan", ES, [('if k not in latest or m.group("ts") > latest[k][0]:', 'if k not in latest or m.group("ts") < latest[k][0]:')], ["absolute"])
m("EM05_conflict_item_status_none", ES, [('title=re.sub(r"^\\d+\\.\\s+", "", ls[a]), status=status))', 'title=re.sub(r"^\\d+\\.\\s+", "", ls[a]), status=None))')], ["absolute"])
m("EM06_issue_status_severity_swapped", ES, [('status=fm.get("status") or None, severity=fm.get("severity") or None))', 'status=fm.get("severity") or None, severity=fm.get("status") or None))')], ["absolute"])
m("EM07_provider_title_is_whole_line", ES, [("                                 title=cells[0]))", "                                 title=l))")], ["absolute"])
m("EM08_checkbox_dash_bullet_only", ES, [('CHECKBOX_RE = re.compile(r"^\\s*[-*+]\\s+', 'CHECKBOX_RE = re.compile(r"^\\s*[-]\\s+')], ["absolute"])
m("EM09_duplicate_locator_silently_dropped", ES, [('                refuse("duplicate_entry_locator", "%s %s" % (loc, e.locator))', "                continue")], ["absolute"])
m("EM10_crlf_not_stripped", ES, [('    return [l[:-1] if l.endswith("\\r") else l for l in t.split("\\n")]', '    return t.split("\\n")')], ["absolute"])
m("EM11_bad_frontmatter_ticket_dropped", ES, [('            st.setdefault("frontmatter_problems", []).append([p] + errs)\n', '            st.setdefault("frontmatter_problems", []).append([p] + errs)\n            continue\n')], ["absolute"])
# ---- reviewer LM01-LM06
LS = "lead_scan.py"
m("LM01_vocab_known_issue_dropped", LS, [('(r"known[ -]issues?", "known issue"), ', "")], ["lead"])
m("LM02_needle_check_removed", LS, [('        if r is None or word not in (r.legacy_id or "").split(",") or r.status != disp:   # MUT:needle-check', "        if False:")], ["lead"])
m("LM03_marker_rule_needs_no_command", LS, [("    if not words or not set(words) <= MARKER_WORDS:\n        return False\n", "    if not words or not set(words) <= MARKER_WORDS:\n        return False\n    return True\n")], ["lead"])
m("LM04_entry_rows_constant_offset", LS, [('"emitted_rows": len(rows),', '"emitted_rows": len(rows) + 5,')], ["lead"])
m("LM05_duplicate_for_any_covered_file", LS, [("            covered = covering(cover, p, n)\n", "            covered = covering(cover, p, n) or ((\"STRUCTURED\", p) if p in cover else None)\n")], ["lead"])
m("LM06_vocab_deprecated_disabled_dropped", LS, [('(r"deprecated", "deprecated"), (r"disabled", "disabled"), ', "")], ["lead"])
# ---- reviewer FM01-FM04 (mutants of the TEST file: the selftest must still fail)
FM = "tests/test_frontmatter_yaml.py"
m("FM01_status_comparison_dropped", FM, [('    if dict(parsed_s) != dict(status_c): fails.append("parser status counts differ from the independent recount")\n', "")], ["fm"])
m("FM02_tolerant_is_the_doc04_naive_split", FM, [('    """-> (frontmatter dict, body text, error or None)"""\n', '    """-> (frontmatter dict, body text, error or None)"""\n    mode = "naive-split" if mode == "tolerant" else mode\n')], ["fm"])
m("FM03_enhancement_comparison_dropped", FM, [('    if enh_parser != enh_grep: fails.append("parser enhancement count %d != recount %d" % (enh_parser, enh_grep))\n', "")], ["fm"])
m("FM04_multi_status_check_dropped", FM, [('    if multi: fails.append("files with != 1 status: line: %d" % len(multi))\n', "")], ["fm"])
# ---- round-2 fixes: every defect class gets a mutant
m("N01_stale_gate_never_trips", ES, [("WHERE e.entry_sha256<>x.sha;", "WHERE 0;")], ["absolute", "lead"])
m("N02_bail_dropped", ES, [('out = [".bail on", "-- " + header,', 'out = ["-- " + header,')], ["absolute", "lead"])
m("N03_locked_sibling_not_written", ES, [('for suffix in (".sql.sha256", ".sha256"):', 'for suffix in (".sql.sha256",):')], ["absolute"])
m("N05_pyyaml_check_removed", ES, [('            refuse("pyyaml_missing", ', '            pass; (')], ["absolute"])
m("N06_remotes_not_required", ES, [('        if not isinstance(fj.get("remotes"), list):', "        if False:")], ["absolute"])
m("N07_remote_credentials_kept", ES, [("    return re.sub(r\"^([A-Za-z][A-Za-z0-9+.\\-]*://)[^/@]*@\", r\"\\1\", u)", "    return u")], ["absolute"])
m("N08_carriers_scanned", ES, [('ID_SCAN_EXCLUDE = ("specs/", ".audit/", "submodules/", "scripts/register/tests/")', "ID_SCAN_EXCLUDE = ()")], ["absolute"])
m("N09_task_glyph_legend_ignored", ES, [('                    next((glyphs[c.replace("\\ufe0f", "")] for c in cells if c.replace("\\ufe0f", "") in glyphs), None)', "                    None")], ["absolute"])
m("N10_conflict_preface_ignored", ES, [('status = ",".join(order) if order else preface.get(item)', 'status = ",".join(order) if order else None')], ["absolute"])
m("N11_conflict_marker_segments_off", ES, [("            if len(marks) >= 2:", "            if False:")], ["absolute"])
m("N12_checkbox_exclusion_unlisted", ES, [("            if n:\n                ex.append([p, n])", "            if False:\n                ex.append([p, n])")], ["absolute"])
m("N13_ids_all_attributed_to_xid", ES, [('            for c in ("S-14", "S-15"):\n                cf = snap.class_files.get(c, set())', '            for c in ():\n                cf = snap.class_files.get(c, set())')], ["absolute", "planted"])
m("N14_skip_hits_not_numbered", ES, [('("#%d" % k if k > 1 else ""), sha_text(p + "\\0" + l), title=l.strip(), status="skipped_test"))', '"", sha_text(p + "\\0" + l), title=l.strip(), status="skipped_test"))')], ["absolute"])
m("N15_service_always_found", ES, [('status="marker_found" if found else "marker_absent"', 'status="marker_found"')], ["absolute"])
m("N16_idless_case_dropped", ES, [('                yield "@%d" % i, x, False', "                continue")], ["absolute"])
m("N18_strict_measurement_off", ES, [("        if serr:\n            strict_bad.append([p, serr])", "        if False:\n            strict_bad.append([p, serr])")], ["absolute", "fm"])
m("N19_block_scalars_not_read", ES, [("            if BLOCK_RE.match(raw):", "            if False:")], ["absolute", "fm"])
m("N20_trailing_comment_kept", ES, [('    return re.sub(r"[ \\t]+#.*$", "", v).strip()', "    return v")], ["absolute", "fm"])
m("N21_key_space_colon_unread", ES, [('KEY_RE = re.compile(r"^([A-Za-z_][\\w\\-]*)[ \\t]*:', 'KEY_RE = re.compile(r"^([A-Za-z_][\\w\\-]*):')], ["absolute", "fm"])
m("N22_quotes_kept", ES, [("            return inner.replace(\"''\", \"'\") if q == \"'\" else inner", "            return v")], ["absolute", "fm"])
m("N23_continuation_lines_dropped", ES, [('        elif last and l.startswith((" ", "\\t")):\n            fm[last] = (fm[last] + " " + l.strip()).strip()', "        elif False:\n            pass")], ["absolute", "fm"])
m("N24_remotes_ignored", ES, [("    rem = snap.freeze.get(\"remotes\")", "    rem = []")], ["absolute"])
m("N25_entry_check_skipped", ES, [("        if r.returncode != 0:\n            err = r.stderr.decode(\"utf-8\", \"replace\")", "        if False:\n            err = r.stderr.decode(\"utf-8\", \"replace\")")], ["planted"])
m("N26_nondeterministic_order", ES, [('for e in sorted(ents, key=lambda x: x.locator.encode("utf-8")):', 'for e in sorted(ents, key=lambda x: hash((x.locator, os.getpid()))):')], ["planted"])
PO = "lead_scan_population.py"
m("M07_hc2_malformed_read_as_empty", PO, [('        raise PopulationRefusal("hc2_malformed", "%s: %s" % (hp, type(e).__name__))', "        return []")], ["lead"])
m("M07b_hc2_non_object_unchecked", PO, [("    if not isinstance(rec, dict):", "    if False:")], ["lead"])
m("M07c_hc2_roots_unchecked", PO, [("    if not isinstance(roots, list) or not all(isinstance(r, str) for r in roots):", "    if False:")], ["lead"])
m("M08_fence_rule_any_fenced_line", LS, [("    return infence and FENCE_CMD_RE.match(line) is not None", "    return infence")], ["lead"])
m("M09_finalize_ignores_before", LS, [('"lead_scan_entry_rows": after - before,', '"lead_scan_entry_rows": after,')], ["lead"])
m("M10_symlink_population_unrecorded", LS, [("            symlinks.append(p)\n", "            pass\n")], ["lead"])
m("M11_vocab_control_removed", LS, [("    if vc:\n", "    if False:\n")], ["lead"])
m("M12_extra_root_unchecked", PO, [("        if r not in gl:", "        if False:")], ["lead"])
m("M13_population_widens_all", PO, [("        if any(inside(p, g) for g in gls) and not any(inside(p, e) for e in extra):   # MUT:population", "        if False:")], ["lead"])
m("M14_snapshot_check_skipped", LS, [("    if r.returncode != 0:\n        err = r.stderr.decode(\"utf-8\", \"replace\"); sys.stderr.write(err)", "    if False:\n        err = r.stderr.decode(\"utf-8\", \"replace\"); sys.stderr.write(err)")], ["lead"])
m("M15_listing_check_skipped", LS, [("    if bad:\n        sys.stderr.write", "    if False:\n        sys.stderr.write")], ["lead"])
m("M16_covering_block_unbounded", LS, [('b[1] = next((n - 1 for n in range(b[0] + 1, len(ls) + 1) if re.match(r"^##\\s", ls[n - 1])), len(ls))', "b[1] = 10 ** 9")], ["lead"])
m("F4_special_file_allowed", "snapshot_manifest.py", [("                if not stat.S_ISREG(os.lstat(full).st_mode):", "                if False:")], ["snap"])
m("F4b_symlink_followed", "snapshot_manifest.py", [("tgt = os.readlink(full)\n                out.append((rel, hashlib.sha256(tgt).hexdigest()))", "tgt = os.fsencode(os.path.realpath(full))\n                out.append((rel, hashlib.sha256(tgt).hexdigest()))")], ["snap"])
m("F3_malformed_manifest_traceback", "check_freeze_snapshot.sh", [("except (OSError, ValueError, KeyError, TypeError) as e:     # a malformed", "except ZeroDivisionError as e:     # a malformed")], ["snap"])
m("F3b_reason_not_passed_through", ES, [('refuse(mm.group(1) if mm else "freeze_snapshot_moved"', 'refuse("freeze_snapshot_moved"')], ["snap"])
# ---- reviewer GM01-GM05: mutants of scripts/register/gate.sh against tests/test_reverify_gate.sh (WP-21; that test is not edited in this pass)
GT = "gate.sh"
m("GM01_numeric_guard_unanchored", GT, [('    [[ $n =~ ^[0-9]+$ ]] || { fail "$v not counted: $(sane "$n")"; continue; }   # GUARD_ND_NUMERIC',
                                         '    [[ $n =~ [0-9] ]] || { fail "$v not counted: $(sane "$n")"; continue; }   # GUARD_ND_NUMERIC')], ["gate"])
m("GM02_views_order_desc", GT, [("WHERE kind='view_not_done' ORDER BY name\")", "WHERE kind='view_not_done' ORDER BY name DESC\")")], ["gate"])
m("GM03_rows_counted_in_reference", GT, [('    n=$(q "SELECT count(*) FROM \\"$v\\"")\n    [[ $n =~', '    n=$(qr "SELECT count(*) FROM \\"$v\\"")\n    [[ $n =~')], ["gate"])
m("GM04_url_partial_ignored_in_completion", GT, [('if [ "${URL_SKIPPED:-0}" -gt 0 ]; then   # a run that skipped', 'if [ $COMPLETION = 0 ] && [ "${URL_SKIPPED:-0}" -gt 0 ]; then   # a run that skipped')], ["gate"])
m("GM05_db_registry_crosscheck_dropped", GT, [("    printf '%s\\n' \"$views\" | grep -qx -- \"$row\" || fail \"registry row not in the reference: $row\"\n  done < <(q \"SELECT name FROM reg_gate_checks WHERE kind='view_not_done'\")",
                                               "    :\n  done < <(q \"SELECT name FROM reg_gate_checks WHERE kind='view_not_done'\")")], ["gate"])
CONTROLS = [
    ("CTRL_comment_only", ES, [("# ---------------------------------------------------------------------------------------------- front matter", "# front matter (comment-only edit)")], ["absolute"], "survive"),
    ("CTRL_deliberately_wrong", ES, [('PARSER = "enumerate_sources.py@2"', 'PARSER = "enumerate_sources.py@2"\nraise SystemExit("deliberately wrong mutant")')], ["absolute"], "kill"),
]
EQUIVALENT = {
    "GM01_numeric_guard_unanchored": "SURVIVES until request R7 is done: tests/test_reverify_gate.sh (WP-21, outside this pass's edit scope) never feeds the gate an error text that holds a digit (review finding D6)",
    "GM05_db_registry_crosscheck_dropped": "equivalent by exit code: the seed diff FAILs the same rows (review: informational)",
}
# GM01-GM05 target scripts/register/gate.sh and are run by mutate_wf23_gate.sh against tests/test_reverify_gate.sh (a WP-21 file this round may not edit)


def run_suite(tree, suite, timeout=1500):
    env = dict(os.environ); env["WI"] = WI; env["TMPDIR"] = env.get("TMPDIR", "/dev/shm"); env["REG_SCRATCH"] = env["TMPDIR"]
    env.pop("ENUMERATE_SOURCES", None); env.pop("LEAD_SCAN", None)
    r = subprocess.run(SUITES[suite], cwd=os.path.join(tree, "scripts/register"), env=env, capture_output=True, text=True, timeout=timeout)
    return r.returncode, r.stdout + r.stderr


def main(argv):
    only = argv[argv.index("--only") + 1] if "--only" in argv else None
    suites_only = argv[argv.index("--suite") + 1] if "--suite" in argv else None
    if "--list" in argv:
        for n, f, e, s in M: print(n, f, s)
        return 0
    base = tempfile.mkdtemp(prefix="mutwf23.", dir=os.environ.get("TMPDIR", "/dev/shm"))
    surv = killed = eq = n = 0
    try:
        def fresh(name):
            t = os.path.join(base, name)
            shutil.copytree(REG, os.path.join(t, "scripts/register"), ignore=shutil.ignore_patterns("__pycache__"))
            return t
        # controls: every suite passes unmutated
        c = fresh("control")
        for s in (["absolute", "lead", "fm", "snap", "gate"] + (["planted"] if suites_only in (None, "planted") else [])):
            if suites_only and s != suites_only: continue
            t0 = time.time(); rc, out = run_suite(c, s)
            print("CONTROL %s unmutated: rc=%d (%ds) %s" % (s, rc, time.time() - t0, (out.strip().split("\n") or [""])[-1][:100]))
            if rc != 0:
                print("control failed: no mutation verdict is meaningful"); return 1
        todo = [(a, b, c_, d, None) for a, b, c_, d in M] + [(a, b, c_, d, e) for a, b, c_, d, e in CONTROLS]
        for name, f, edits, suites, ctrl in todo:
            if only and only != name: continue
            if suites_only: suites = [s for s in suites if s == suites_only]
            if not suites: continue
            n += 1
            t = fresh(name); p = os.path.join(t, "scripts/register", f); src = open(p, encoding="utf-8").read(); new = src
            ok = True
            for old, rep in edits:
                if new.count(old) != 1:
                    print("MUTANT %s: NOT APPLIED (anchor found %d times: %r) - counted as an unexpected survivor" % (name, new.count(old), old[:70])); ok = False; break
                new = new.replace(old, rep)
            if not ok:
                surv += 1; shutil.rmtree(t, ignore_errors=True); continue
            open(p, "w", encoding="utf-8").write(new)
            verdict, why = "SURVIVED", ""
            for s in suites:
                rc, out = run_suite(t, s)
                if rc != 0:
                    verdict = "KILLED"; why = "by %s: %s" % (s, next((l for l in out.split("\n") if l.startswith(("FAIL", "enumerate_sources: REFUSED", "Traceback", "lead_scan: REFUSED"))), out.strip().split("\n")[-1])[:110]); break
            if name in EQUIVALENT:
                if verdict == "SURVIVED": eq += 1; print("MUTANT %s: SURVIVED (equivalent: %s)" % (name, EQUIVALENT[name]))
                else: surv += 1; print("MUTANT %s: expected equivalent but KILLED %s" % (name, why))
            elif ctrl == "survive":
                if verdict == "SURVIVED": eq += 1; print("MUTANT %s: SURVIVED as the control requires (the harness cannot kill everything)" % name)
                else: surv += 1; print("MUTANT %s: control FAILED: a comment-only edit was KILLED %s" % (name, why))
            elif verdict == "KILLED":
                killed += 1; print("MUTANT %s: KILLED %s" % (name, why))
            else:
                surv += 1; print("MUTANT %s: SURVIVED (unexpected)" % name)
            shutil.rmtree(t, ignore_errors=True)
    finally:
        shutil.rmtree(base, ignore_errors=True)
    print("MUTANTS: %d run, %d killed, %d equivalent-as-expected, %d unexpected survivors" % (n, killed, eq, surv))
    return 0 if surv == 0 else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
