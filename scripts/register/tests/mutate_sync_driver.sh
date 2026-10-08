#!/usr/bin/env bash
# mutate_sync_driver.sh - the paired §1.1 mutations of scripts/register/sync_trackers.sh (T189), run against test_sync_driver.sh (T188) and test_sync_fix_r2.sh (WF23 fix round 2).
# Usage: mutate_sync_driver.sh <out.txt>   exit 0 only when every mutant is killed AT AN INTENDED CHECK (or reviewed-equivalent), the golden-good control passes on both suites
#        and the negative control survives both.
# Env:   MUT_ONLY=<regex>  run only the mutants whose name matches (the negative control always stays); MUT_DRY=1  generate and check the mutants, run nothing;
#        MUT_JOBS=<n>      parallel mutant runs (default 1: every run is a container-leg suite, run them one at a time on a shared host).
# Every mutant is ONE exact-string replacement in the driver (the string must occur exactly once, else exit 3), checked to differ from the original, to pass `bash -n` and to
# compile as Python. Each declares (a) the suite that is expected to kill it, with the sections of test_sync_fix_r2.sh to run first (R2_SECTIONS), and (b) the INTENDED
# checks: the id of the first failing check must match. SCORING (WF23 E1): a run is `env` (never a kill) when the suite failed without a FAIL line OR ANY line of the
# transcript names an environmental refusal (disk, memory, CPU budget, image lock): the old scorer looked at the first FAIL line only and counted a register write that an
# environmental refusal had failed as a kill. An env run is repeated (3 tries). A kill at a check that is not intended is MISATTR and fails the run, because the mutation
# may not even be reachable from that check. A mutant that survives its own suite is run against the other suite in full before it is called a survivor.
# Reviewed-equivalent survivors are listed in equivalent_sync_mutants.tsv and reported EQUIV, never counted as killed. GOLDEN_GOOD_CONTROL = the unmodified driver passes both
# suites; NEGATIVE CONTROL = a comment-only edit must survive both (proof that the runner does not kill mutants for the wrong reason).
. "$(dirname "$0")/mutscore.sh"
ROOT=$(cd "$(dirname "$0")/../../.." && pwd); OUT=${1:?out file}; R="$ROOT/scripts/register"; T="$R/tests"; SUT="$R/sync_trackers.sh"
W=$(mktemp -d "${TMPDIR:-/tmp}/mutsync.XXXXXX"); PC="${REG_SCRATCH:-$W}/mutsync-protocache.$$"; trap 'rm -rf "$W" "$PC"' EXIT
export R2_PROTO_CACHE="$PC"   # the r2 prototype register is built once per runner and cloned by every r2 run
python3 -I - "$SUT" "$W" <<'PY' || exit 3
import sys, os, re, json
src = open(sys.argv[1]).read(); W = sys.argv[2]
M = []
def m(name, old, new, suite, intended): M.append((name, old, new, suite, intended))
# ---- the 45 mutants of the first revision that still apply (anchors follow the current driver text)
m("hook_gate", 'if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then LOCKED="${LOCKED:-', 'if true; then LOCKED="${LOCKED:-', "old", "Z18")
m("empty_env_counts_as_set", 'miss = sorted(n for n in t["required_env"] if not envset(n))', 'miss = sorted(n for n in t["required_env"] if n not in os.environ)', "old", "E[1-8]|D5|D7")
m("creds_check_removed", 'if miss: return ("credentials_absent", miss)', 'if False: return ("credentials_absent", miss)', "old", "D5|E[1-8]")
m("client_check_removed", 'if not (t["cmd"] or "").strip() or client_absent(t): return ("tracker_client_absent", [])', 'if False: return ("tracker_client_absent", [])', "old", "E[1-8]|D5|D7")
m("disabled_ignored", 'if not t["enabled"]: return ("disabled_by_operator", [])', 'if False: return ("disabled_by_operator", [])', "old", "E[1-8]")
m("not_configured_runs", 'if not t["has_cmd"]: return ("not_configured", [])', 'if False: return ("not_configured", [])', "old", "C1|E[1-8]")
m("unchanged_resync", 'if last and last["status"] == "SYNCED" and last["rev"] == it["rev"]:', 'if False:', "old", "A9|A4|A10|F5|R1|E[1-8]")
m("revision_ignored", 'if last and last["status"] == "SYNCED" and last["rev"] == it["rev"]:', 'if last and last["status"] == "SYNCED":', "old", "A10|A11")
m("failed_not_reselected", 'if last and last["status"] == "SYNCED" and last["rev"] == it["rev"]:', 'if last and last["status"] in ("SYNCED", "FAILED") and last["rev"] == it["rev"]:', "old", "F5|F6")
m("attempts_unbounded_to_one", 'for attempt in range(1, A["attempts"] + 1):', 'for attempt in range(1, 2):', "old", "F2")
m("breaker_removed", 'if FAILS[name] >= A["breaker"]:', 'if False:', "old", "U[1-5]")
m("unreachable_not_counted_by_breaker", 'if res["reason"] == "unreachable": FAILS[name] += 1', 'pass', "old", "U[1-5]")
m("evidence_containment_removed", 'if full.startswith(evroot) and os.path.isfile(full)', 'if os.path.isfile(full)', "old", "V-outside|V1")
m("exit_code_not_trusted", 'if o["status"] == "FAILED" or rc != 0:', 'if o["status"] == "FAILED":', "old", "V-exit1|V1|V2")
m("synced_without_reference", 'if ref_out and isinstance(ep, str) and ep and "\\x00" not in ep and not os.path.isabs(ep):', 'if isinstance(ep, str) and ep and "\\x00" not in ep and not os.path.isabs(ep):', "old", "V-noref|V1|V2")
m("db_opened_for_writing", 'return sqlite3.connect("file:" + urllib.parse.quote(DBFILE) + "?mode=ro", uri=True, timeout=30)',
  'c = sqlite3.connect(DBFILE, timeout=30); c.execute("PRAGMA user_version=4242"); c.commit(); return c', "old", "D2|W3")
m("wrapper_not_called", 'r = subprocess.run([LOCKED, "--op-id", opid, "--"] + cmd_after, stdin=subprocess.DEVNULL, capture_output=True, encoding="utf-8", errors="replace")',
  'r = subprocess.run(["true"], stdin=subprocess.DEVNULL, capture_output=True, encoding="utf-8", errors="replace")', "old", "E[1-8]|M[1-4]")
m("full_environment_to_adapter", 'env = {kk: os.environ[kk] for kk in ("PATH", "HOME", "LANG", "TMPDIR") if kk in os.environ}', 'env = dict(os.environ)', "old", "A5")
m("mint_dedup_removed", 'if ex and ex[1] is not None:', 'if False:', "old", "M[1-4]|R1|G[1-4]|E[1-8]")
m("driver_lock_removed", 'try: fcntl.flock(LOCKFD, fcntl.LOCK_EX | fcntl.LOCK_NB)', 'try: pass', "old", "X1")
m("unwritten_rows_not_kept", 'for r in rows_unwritten: fh.write(json.dumps({k: v for k, v in r.items()}, sort_keys=True) + "\\n")', 'pass', "old", "W5|W6")
m("write_failure_ignored", '    elif rc != 0:\n        rows_unwritten = rows\n', '    elif False:\n        rows_unwritten = rows\n', "old", "W5|W6")
m("dry_run_upserts_trackers", '    if not A["dry"]:\n        if not reconcile_wal():', '    if True:\n        if not reconcile_wal():', "old", "D[1-7]")
m("dry_run_calls_adapter", '            if A["dry"]: continue\n            KSEQ[0] += 1; k = KSEQ[0]', '            KSEQ[0] += 1; k = KSEQ[0]', "old", "D[1-7]")
m("unknown_tracker_key_allowed", 'if ex: bad("trackers[%d]: unknown key(s)', 'if False: bad("trackers[%d]: unknown key(s)', "old", "Z3")
m("secret_in_command_allowed", 'if cp: refuse("config_secret_in_command", ', 'if False: refuse("config_secret_in_command", ', "old", "Z6")
m("id_prefix_unchecked", 'if "\'%s-\'" % CFG["id_prefix"] not in (ids_sql[0] if ids_sql else ""):', 'if False:', "old", "Z7")
m("skipped_duplicate_rewritten", 'if last and last["status"] == "SKIPPED" and last["reason"] == reason and last["rev"] == it["rev"] and last["missing"] == miss:', 'if False and last["missing"] == miss:', "old", "R[12]|E7")
m("skip_reason_not_compared", 'last["reason"] == reason and last["rev"] == it["rev"] and last["missing"] == miss:', 'last["rev"] == it["rev"] and last["missing"] == miss:', "old", "G[1-4]|R[12]")
m("skip_names_not_compared", ' and last["missing"] == miss:', ':', "old", "G[1-4]|R[12]")
m("limit_ignored", 'if A["limit"] is not None and DONE[name] >= A["limit"]: continue', 'if False: continue', "old", "V[-0-9a-z]*|U[1-5]")
m("evidence_hash_of_path", '"sha256": sha_file(full), "size"', '"sha256": hashlib.sha256(full.encode()).hexdigest(), "size"', "old", "A3")
m("missing_names_dropped", 'add_row(dict(base, status="SKIPPED", skip_reason=reason, missing=miss))', 'add_row(dict(base, status="SKIPPED", skip_reason=reason, missing=[]))', "old", "E3|G[12]|V3")
m("schema_version_unchecked", 'if cfg.get("schema_version") != 1 or isinstance(cfg.get("schema_version"), bool): bad("schema_version must be 1")', 'pass', "old", "Z4")
m("trackers_type_unchecked", 'if not isinstance(cfg.get("trackers"), list): bad(', 'if False: bad(', "old", "Z5")
m("placeholder_unchecked", 'if re.search(r"\\{(?!db\\}|id\\})[^}]*\\}", tk): bad(', 'if False: bad(', "old", "Z8")
m("dotdot_path_allowed", ' or ".." in p.split("/")', '', "old", "Z9")
m("db_missing_exit_code", 'print("sync_trackers: database %s not found" % DB, file=sys.stderr); sys.exit(3)', 'print("sync_trackers: database %s not found" % DB, file=sys.stderr); sys.exit(1)', "old", "Z11")
m("failed_exit_zero", 'elif any(COUNT[n]["failed"] for n in COUNT): exit_code = 1', 'elif any(COUNT[n]["failed"] for n in COUNT): exit_code = 0', "old", "F1|V-.*")
m("write_failed_exit_zero", 'elif WRITE_FAILED[0]: exit_code = 4', 'elif WRITE_FAILED[0]: exit_code = 0', "old", "W5")
m("minted_items_not_synced_same_run", '        if fresh:', '        if False:', "old", "U[1-5]|F[1-6]|E[1-8]")
m("trackers_always_upserted", 'if have is None or tuple(have) != want:', 'if True:', "old", "R1")
m("process_exit_ignored_for_unreachable_bad_reason", 'if sr not in ADAPTER_SKIPS: return', 'if False: return', "old", "V-badskip|V1|V2")
m("adapter_timeout_ignored", 'state = wait_leader(p, A["timeout"])', 'state = wait_leader(p, 1e9)', "old", "V-hang|V1|V2")
m("missing_names_unvalidated", 'or any(not isinstance(x, str) or not ENV_RE.match(x) for x in miss) or (sr == "credentials_absent" and not miss):', ':', "old", "V-badnames|V1|V2")
# ---- the 18 mutants of the independent review (X01..X18), translated onto the current text
m("X01_backoff_removed", 'time.sleep(min(30.0, A["backoff"] * (2 ** (attempt - 1))))', 'pass', "r2:backoff", "XB1")
m("X02_breaker_never_reset", 'FAILS[name] = 0; REJECTS[name] = 0; c["synced"] += 1', 'c["synced"] += 1', "r2:breaker", "BB1")
m("X03_empty_receipt_size_check_removed", 'os.path.getsize(full) > 0 and os.path.getmtime(full) >= t0 - 1.0:', 'os.path.getmtime(full) >= t0 - 1.0:', "r2:receipt", "RA-emptyrec")
m("X04_lock_shared", 'fcntl.LOCK_EX | fcntl.LOCK_NB', 'fcntl.LOCK_SH | fcntl.LOCK_NB', "old", "X[0-2]")
m("X05_revision_ignores_body", 'json.dumps({"atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "status": it["status"]', 'json.dumps({"atm_id": it["atm_id"], "title": it["title"], "status": it["status"]', "r2:revision", "VB1")
m("X06_revision_ignores_status", 'json.dumps({"atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "status": it["status"], "type"', 'json.dumps({"atm_id": it["atm_id"], "title": it["title"], "body": it["body"], "type"', "r2:revision", "VB2")
m("X07_secret_key_regex_token_only", 'CRED_KEY = re.compile(r"(token|secret|passw|pwd|api[-_]?key|apikey|auth(?!or)|credential|bearer|private[-_]?key|access[-_]?key|cookie|jwt|signature|(^|[-_.])pat($|[-_.]))", re.I)',
  'CRED_KEY = re.compile(r"(token)", re.I)', "r2:secret", "S0[2-9]|S1[0-9]")
m("X08_env_expanded_into_argv", 'av = [x.replace("{db}", DB).replace("{id}", it["atm_id"]) for x in t["argv"]]', 'av = [os.path.expandvars(x).replace("{db}", DB).replace("{id}", it["atm_id"]) for x in t["argv"]]', "r2:revision", "VC1")
m("X09_remote_ref_never_passed", 'res = adapter_call(t, it, ref0, k)', 'res = adapter_call(t, it, None, k)', "r2:wal", "WB4|VB4")
m("X10_owed_reopen_dropped", 'if ex[1].endswith("(→ Fixed.md)") and not any(', 'if False and not any(', "r2:owed", "OWc")
m("X11_db_opened_rw", '"?mode=ro", uri=True, timeout=30)', '"?mode=rw", uri=True, timeout=30)', "r2:misc", "XC1")
m("X12_missing_env_lists_all_required", 'miss = sorted(n for n in t["required_env"] if not envset(n))', 'miss = sorted(t["required_env"]) if any(not envset(n) for n in t["required_env"]) else []', "r2:secret", "S8")
m("X13_passthrough_template_unchecked", 'tp = template_problem(k, x)', 'tp = None', "r2:secret", "S6[0-3]")
m("X14_client_script_check_removed", 'if "/" in a or a.endswith((".sh", ".py")):', 'if False:', "r2:secret", "S9")
m("X15_refs_first_not_latest", 'if r[6]: refs[(r[0], r[1])] = r[6]', 'if r[6]: refs.setdefault((r[0], r[1]), r[6])', "r2:revision", "VB4")
m("X16_revision_ignores_severity", '"severity": it["severity"]}, sort_keys=True', '}, sort_keys=True', "r2:revision", "VB3")
m("X17_tracker_boundary_flush_removed", '        if not A["dry"] and not flush(force=True): return False\n    return True', '        pass\n    return True', "r2:boundary", "XA1")
m("X18_remote_ref_unvalidated", 'if isinstance(rr, str) and REF_RE.match(rr): ref_out = rr', 'if isinstance(rr, str): ref_out = rr', "r2:wal", "WE1")
# ---- the defect classes of the review, one or more mutants each
m("N01_shape_check_removed", 'if SHAPE.search(tk): return "a credential-shaped token"', 'if False: return "a credential-shaped token"', "r2:secret", "S19")
m("N02_short_flag_rule_removed", 'if re.fullmatch(r"-[A-Za-z]", tk) and not interp_pos and', 'if False and not interp_pos and', "r2:secret", "S13")
m("N03_userinfo_check_removed", 'if USERINFO.search(tk): return "a user:password@ URL"', 'if False: return "a user:password@ URL"', "r2:secret", "S15")
m("N04_auth_text_check_removed", 'if AUTH_TEXT.search(tk) or tk in ("-H", "--header") or tk.lower().startswith("--header="): return "an authorization header"', 'if False: return "an authorization header"', "r2:secret", "S10|S14")
m("N05_separate_value_flag_rule_removed", 'if CRED_KEY.search(flag) and "file" not in flag.lower() and nxt is not None', 'if False and "file" not in flag.lower() and nxt is not None', "r2:secret", "S11|S12")
m("N06_key_value_rule_removed", 'if CRED_KEY.search(key) and not REF_VALUE.match(val) and not (', 'if False and not REF_VALUE.match(val) and not (', "r2:secret", "S0[1-9]|S1[0-9]")
m("N07_duplicate_keys_allowed", 'if dup: raise yaml.constructor.ConstructorError(', 'if False: raise yaml.constructor.ConstructorError(', "r2:secret", "S7-dup[12]")
m("N08_whitespace_credential_present", 'def envset(n): return bool(os.environ.get(n, "").strip())', 'def envset(n): return bool(os.environ.get(n, ""))', "r2:secret", "S8")
m("N09_lowercase_env_names_refused", 'ENV_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]{0,63}$")', 'ENV_RE = re.compile(r"^[A-Z_][A-Z0-9_]{0,63}$")', "r2:secret", "S8")
m("N10_env_wrapper_not_skipped", 'if av and os.path.basename(av[0]) == "env":', 'if False and os.path.basename(av[0]) == "env":', "r2:secret", "S9")
m("N11_json_path_unconfined", 'if A["json"] is not None and not (A["json"].startswith(".audit/") or A["json"].startswith(EVDIR.rstrip("/") + "/")):', 'if False:', "r2:secret", "S10")
m("N12_command_stored_as_text", 'return "%s#sha256:%s" % (os.path.basename(rav[0]) if rav else "-", sha_text(t["cmd"])[:16])', 'return t["cmd"]', "r2:revision", "VD1")
m("N13_decode_strict", 'so = fh.read(1 << 20).decode("utf-8", errors="replace")', 'so = fh.read(1 << 20).decode("utf-8")', "r2:wal", "WA1")
m("N14_wal_intent_removed", '            wal({"ev": "intent", "k": k, "tracker": name, "atm_id": it["atm_id"], "rev": it["rev"], "ref": ref0, "t": base["attempted_at"]})', '            pass', "r2:wal", "WC2")
m("N15_wal_result_removed", '            wal({"ev": "result", "k": k, "row": row})', '            pass', "r2:wal", "WB[1-4]|WC[1-4]")
m("N16_reconcile_skipped", 'if not reconcile_wal():', 'if False and not reconcile_wal():', "r2:wal", "WB[2-4]|WC[2-4]")
m("N17_wal_written_to_devnull", 'fd = os.open(WAL_PATH, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)', 'fd = os.open(os.devnull, os.O_WRONLY)', "r2:wal", "WA1|WB1|WG1")
m("N17b_intent_written_after_the_call", '            wal({"ev": "intent", "k": k, "tracker": name, "atm_id": it["atm_id"], "rev": it["rev"], "ref": ref0, "t": base["attempted_at"]})\n            res = adapter_call(t, it, ref0, k)',
  '            res = adapter_call(t, it, ref0, k)\n            wal({"ev": "intent", "k": k, "tracker": name, "atm_id": it["atm_id"], "rev": it["rev"], "ref": ref0, "t": base["attempted_at"]})', "r2:wal", "WI1|WC2")
m("N18_group_kill_removed", '    kill_group(p)   # a timed-out adapter, and any child an adapter that returned left behind, must not keep pushing', '    pass', "r2:proc", "PA[12]")
m("N19_killpg_guard_removed", 'if not isinstance(pgid, int) or pgid <= 1 or pgid == os.getpgrp(): return', 'if False: return', "r2:proc", "PA3")
m("N20_timeout_retried", 'if last.get("indeterminate") and not ref: break', 'if False: break', "r2:proc", "PA1")
m("N21_held_removed", 'if st is None and last and last["status"] == "FAILED" and last["exit"] in INDETERMINATE and not refs.get((name, it["atm_id"])) and not A["retry_indet"]:', 'if False:', "r2:wal", "WC2")
m("N22_retry_flag_ignored", 'and not refs.get((name, it["atm_id"])) and not A["retry_indet"]:', 'and not refs.get((name, it["atm_id"])):', "r2:wal", "WC4")
m("N23_signals_not_handled", 'for _s in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP): signal.signal(_s, _sig)', 'pass', "r2:wal", "WF1")
m("N24_rejecting_counted_unreachable", 'if REJECTS[name] == 0:   # every failure of the streak was "unreachable"', 'if True:   # every failure of the streak was "unreachable"', "r2:breaker", "BA1")
m("N25_enabled_always_1", 'json.dumps(t["required_env"]), 1 if t["enabled"] else 0)', 'json.dumps(t["required_env"]), 1)', "r2:enabled", "EA1")
m("N26_retire_removed", 'if name not in ALL_NAMES and have[3] == 1:', 'if False:', "r2:enabled", "EA3")
m("N27_retire_on_subset_run", '    if not A["trackers"]:   # a full run:', '    if True:   # a full run:', "r2:enabled", "EA2")
m("N28_receipt_path_unbound", 'if full.startswith(evroot) and os.path.isfile(full) and os.path.getsize(full) > 0', 'if os.path.isfile(full) and os.path.getsize(full) > 0', "r2:receipt", "RA-evany")
m("N29_receipt_mtime_unchecked", ' and os.path.getmtime(full) >= t0 - 1.0:', ':', "r2:receipt", "RA-stale")
m("N30_receipt_content_unchecked", 'if it["atm_id"].encode() in body and ref_out.encode() in body:', 'if True:', "r2:receipt", "RA-(norefrec|otheritem|wrongitem|emptyrec)")
m("N31_receipt_item_only", 'if it["atm_id"].encode() in body and ref_out.encode() in body:', 'if it["atm_id"].encode() in body:', "r2:receipt", "RA-norefrec")
m("N32_receipt_ref_only", 'if it["atm_id"].encode() in body and ref_out.encode() in body:', 'if ref_out.encode() in body:', "r2:receipt", "RA-wrongitem")
m("N33_gate_removed", 'first_sync_gate(PLAN, PLAN_HASH)   # REFUSED before any write', 'pass', "r2:gate", "GA1")
m("N34_gate_hash_unchecked", 'if A["approve_plan"] != plan_hash: no(', 'if False: no(', "r2:gate", "GA3")
m("N35_gate_limit_unchecked", 'if A["limit"] is None or A["limit"] > PILOT_MAX: no(', 'if False: no(', "r2:gate", "GA3")
m("N36_gate_approver_unchecked", 'if A["approved_by"] not in APPROVERS: no(', 'if False: no(', "r2:gate", "GA3")
m("N37_gate_empty_roster_allowed", 'if not APPROVERS: no(', 'if False: no(', "r2:gate", "GA7")
m("N38_plan_ignores_items", 'if its: plan[t["name"]] = {"command_ref": cmd_ref(t), "items": its}', 'if its: plan[t["name"]] = {"command_ref": cmd_ref(t), "items": []}', "r2:gate", "GA4")
m("N39_gate_persists_after_pilot", 'if t["state"] is not None or synced.get(t["name"], 0) > 0: continue', 'if t["state"] is not None: continue', "r2:gate", "GA6")
m("N40_confidential_filter_removed", 'if t["public"] and it.get("category") in CONF_CATS: return (EXIT_CONFIDENTIAL, "confidential_withheld")', 'if False: return (EXIT_CONFIDENTIAL, "confidential_withheld")', "r2:outbound", "OA1")
m("N41_secret_scan_removed", 'if any(r.search(text) for r in LEAK_RES): return (EXIT_SECRET, "secret_in_payload")', 'if False: return (EXIT_SECRET, "secret_in_payload")', "r2:outbound", "OA1")
m("N42_withheld_rerecorded", 'if last and last["status"] == "FAILED" and last["exit"] == blocked[0] and last["rev"] == it["rev"]: c["withheld_unchanged"] += 1; continue', 'if False: c["withheld_unchanged"] += 1; continue', "r2:outbound", "OA4")
m("N43_public_default_false", 'pub = t.get("public", True)', 'pub = t.get("public", False)', "r2:outbound", "OA1")
m("N44_locked_21_22_unverified", 'if rc in (21, 22):   # locked.sh:', 'if False:   # locked.sh:', "r2:wal", "WD[12]")
m("N45_integer_ref_dropped", 'if isinstance(rr, int) and not isinstance(rr, bool) and rr >= 0: rr = str(rr)', 'pass', "r2:wal", "WE1")
m("N46_bad_ref_dropped", 'elif rr not in (None, "", False): bad_ref = sanitize_ref(rr)', 'elif False: bad_ref = sanitize_ref(rr)', "r2:wal", "WE1")
m("N47_first_mint_failure_blocks_all", '        mint_round()\n    items = read(load_items); latest, refs = read(load_log)', '        if not mint_round()[1]: sys.exit(4)\n    items = read(load_items); latest, refs = read(load_log)', "r2:misc", "XD2")
m("N48_mint_failure_reason_hidden", '                for l in tail_lines(se): print("sync_trackers: locked: " + l, file=sys.stderr)\n                WRITE_FAILED[0] = True', '                WRITE_FAILED[0] = True', "r2:misc", "XD1")
m("N49_non_json_names_crash", '    except ValueError:\n        return []\n    return v if isinstance(v, list) else []', '    except ValueError:\n        raise\n    return v', "r2:enabled", "EB1")
m("N50_attention_exit_dropped", 'elif OWED_REOPEN or any(COUNT[n]["held_indeterminate"] for n in COUNT): exit_code = 7', 'elif OWED_REOPEN or any(COUNT[n]["held_indeterminate"] for n in COUNT): exit_code = 0', "r2:owed", "OWc")
m("N51_drift_not_reported", 'if res.get("remote_state") is not None and res["remote_state"].lower() != (it["status"] or "").lower():', 'if False:', "r2:revision", "VA2")
m("N52_wal_unwritable_ignored", 'def wal(rec):\n    if A["dry"]: return', 'def wal(rec):\n    return\n    if A["dry"]: return', "r2:wal", "WB1|WG1")
m("N53_wal_replayed_into_any_db", 'if e.get("db") != DB: continue   # records of another register are not ours to replay', 'if False: continue', "r2:wal", "WH1")
m("negative_control_comment", '# Exit: 0 ok (SKIPPED rows are honest results, not errors)', '# Exit (control): 0 ok (SKIPPED rows are honest results, not errors)', "old", "-")
names = set(); meta = []
for n, old, new, suite, intended in M:
    assert n not in names, n; names.add(n)
    c = src.count(old)
    if c != 1:
        # the first comment anchor of the negative control may sit in the header; every other anchor must be unique
        print("mutant %s: old string occurs %d times" % (n, c), file=sys.stderr); sys.exit(3)
    mut = src.replace(old, new)
    if mut == src: print("mutant %s identical" % n, file=sys.stderr); sys.exit(3)
    d = os.path.join(W, "t", n, "scripts", "register"); os.makedirs(d)
    p = os.path.join(d, "sync_trackers.sh"); open(p, "w").write(mut); os.chmod(p, 0o755)
    body = mut.split("exec python3 -I - \"$@\" <<'PY'\n")[1].rsplit("\nPY\n", 1)[0]
    try: compile(body, n, "exec")
    except SyntaxError as e: print("mutant %s does not compile: %s" % (n, e), file=sys.stderr); sys.exit(3)
    meta.append("%s\t%s\t%s" % (n, suite, intended))
open(os.path.join(W, "meta.tsv"), "w").write("\n".join(meta) + "\n")
PY
NAMES=$( { awk -F'\t' '$2 ~ /^r2/ {print $1}' "$W/meta.tsv"; awk -F'\t' '$2=="old" {print $1}' "$W/meta.tsv"; } | tr '\n' ' ')   # the r2 suite's mutants first: survivors of the newer tests show up early
for n in $NAMES; do bash -n "$W/t/$n/scripts/register/sync_trackers.sh" || { echo "mutant $n does not parse" >&2; exit 3; }; done
if [ -n "${MUT_ONLY:-}" ]; then n2=""; for n in $NAMES; do { [[ "$n" =~ $MUT_ONLY ]] || { [ "$n" = negative_control_comment ] && [ -z "${MUT_DEV_SKIP_CONTROLS:-}" ]; }; } && n2="$n2 $n"; done; NAMES=$n2; fi
[ -z "${MUT_DRY:-}" ] || { echo "$(echo $NAMES | wc -w) mutants generated, all differ from the driver, parse and compile"; exit 0; }
{ echo "# identity: group=sync utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)"
  echo "# runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1) test_sha256=$(sha256sum "$T/test_sync_driver.sh" | cut -d' ' -f1) r2_test_sha256=$(sha256sum "$T/test_sync_fix_r2.sh" | cut -d' ' -f1) sut_sha256=$(sha256sum "$SUT" | cut -d' ' -f1)"
  echo "# mutant<TAB>verdict (yes = killed at an intended check, MISATTR, ENV, NO, EQUIV)<TAB>first failing check (fail-fast)<TAB>suite that decided<TAB>intended checks"; } >"$OUT"
# one suite run: runsuite <scratch dir> <driver or empty for the real one> <suite spec>  -> sets RC and TR (transcript file)
runsuite() { local sc=$1 drv=$2 spec=$3 f; f="$sc/transcript.txt"; mkdir -p "$sc"; local -a e=(REG_SCRATCH="${REG_SCRATCH:-$sc}" SYNC_FAILFAST=1); [ -z "$drv" ] || e+=(SYNC="$drv")
  case "$spec" in
    old) env "${e[@]}" bash "$T/test_sync_driver.sh" >"$f" 2>&1; RC=$?;;
    r2:*) env "${e[@]}" R2_SECTIONS="${spec#r2:}" bash "$T/test_sync_fix_r2.sh" >"$f" 2>&1; RC=$?;;
    r2) env "${e[@]}" bash "$T/test_sync_fix_r2.sh" >"$f" 2>&1; RC=$?;;
  esac; TR="$f"; }
# score a finished run: survived | killed | env (WF23 E1: ANY line of the transcript that names an environmental refusal, or no FAIL line at all, makes the run env)
sscore() { if [ "$RC" -eq 0 ]; then echo survived; elif ! grep -q '^FAIL' "$TR"; then echo env; elif grep -Eq "$ENV_RE" "$TR"; then echo env; else echo killed; fi; }
firstfail() { grep -m1 '^FAIL' "$TR" | cut -c1-170; }
decide() {  # decide <name> <driver> <suite spec> <intended> -> V (survived|killed|MISATTR|ENV), F (first failing line), S (suite)
  local name=$1 drv=$2 spec=$3 intended=$4 try v id
  for try in 1 2 3; do runsuite "$W/s.$name.$try" "$drv" "$spec"; v=$(sscore)
    if [ "$v" = killed ]; then id=$(grep -m1 '^FAIL' "$TR" | awk '{print $2}')
      if [ "$intended" = "-" ] || [[ "$id" =~ ^($intended)$ ]]; then V=killed; F=$(firstfail); S=$spec; return; fi
      V=MISATTR; F=$(firstfail); S=$spec; continue   # a kill at an unintended check may be a flake: try again, report MISATTR if it stays
    fi
    [ "$v" = env ] && { V=ENV; F=$(firstfail); S=$spec; continue; }
    V=survived; F=""; S=$spec; return
  done; }
runm() { local name=$1 spec intended d="$W/t/$1/scripts/register/sync_trackers.sh" other
  spec=$(awk -F'\t' -v n="$name" '$1==n{print $2}' "$W/meta.tsv"); intended=$(awk -F'\t' -v n="$name" '$1==n{print $3}' "$W/meta.tsv")
  decide "$name" "$d" "$spec" "$intended"
  if [ "$V" = survived ]; then   # a survivor of its own suite is run against the other suite in full before it is called a survivor
    case "$spec" in old) other=r2;; *) other=old;; esac
    decide "$name.other" "$d" "$other" "-"
    if [ "$V" = survived ] && [ "$other" = old ]; then decide "$name.r2full" "$d" r2 "-"; fi
    case "$V" in killed) V=killed_other; ;; esac
  fi
  case "$V" in killed) k=yes;; killed_other) k=OTHER;; MISATTR) k=MISATTR;; ENV) k=ENV;; *) k=NO;; esac
  printf '%s\t%s\t%s\t%s\t%s\n' "$name" "$k" "$F" "$S" "$intended" >"$W/$name.res"; }
JOBS=${MUT_JOBS:-1}; running=0
for n in $NAMES; do runm "$n" & running=$((running+1)); [ "$running" -ge "$JOBS" ] && { wait -n; running=$((running-1)); }; done; wait
for n in $NAMES; do cat "$W/$n.res" >>"$OUT"; done
gold() { local spec=$1 try; for try in 1 2 3; do runsuite "$W/s.golden.$try" "" "$spec" 2>/dev/null; v=$(sscore); [ "$v" = env ] || break; done; printf '%s' "$v/$(tail -n1 "$TR" | cut -c1-80)"; }
# the golden runs use the unmodified driver (SYNC empty: each suite falls back to the real one)
if [ -n "${MUT_DEV_SKIP_CONTROLS:-}" ]; then   # development only: the run is NOT evidence (exit 1, summary says so)
  printf 'GOLDEN_GOOD_CONTROL\tSKIPPED\tMUT_DEV_SKIP_CONTROLS\n' >>"$OUT"
else
g1=$(gold old); g2=$(gold r2)
printf 'GOLDEN_GOOD_CONTROL\t%s\t%s\n' "$([ "${g1%%/*}" = survived ] && [ "${g2%%/*}" = survived ] && echo pass || echo FAIL)" "old: ${g1#*/} | r2: ${g2#*/}" >>"$OUT"
fi
EQF="$T/equivalent_sync_mutants.tsv"
if [ -f "$EQF" ]; then awk -F'\t' 'NR==FNR{if($0!~/^#/ && NF>=2) r[$1]=$2; next} !/^#/ && ($1 in r) && $2=="NO"{print $1"\tEQUIV\t"r[$1]; next} {print}' "$EQF" "$OUT" >"$OUT.eq" && mv "$OUT.eq" "$OUT"; fi
nc=$(awk -F'\t' '$1=="negative_control_comment"{print $2}' "$OUT")
cnt() { awk -F'\t' -v v="$1" '!/^#/ && $2==v && $1!="negative_control_comment"' "$OUT" | wc -l; }
surv=$(cnt NO); eqn=$(cnt EQUIV); envn=$(cnt ENV); mis=$(cnt MISATTR); oth=$(cnt OTHER); killed=$(cnt yes)
if [ -n "${MUT_DEV_SKIP_CONTROLS:-}" ]; then n=$(echo $NAMES | wc -w); ncok=SKIPPED; else n=$(( $(echo $NAMES | wc -w) - 1 )); ncok=$([ "$nc" = NO ] && echo ok || echo BROKEN); fi
gok=$(awk -F'\t' '$1=="GOLDEN_GOOD_CONTROL"{print $2}' "$OUT")
echo "group=sync mutants=$n killed_at_intended_check=$killed killed_only_by_the_other_suite=$oth equivalent_reviewed=$eqn survived_unreviewed=$surv misattributed=$mis environment_unscored=$envn golden_good=$gok negative_control=$ncok" | tee "$OUT.summary"
[ $surv -eq 0 ] && [ $envn -eq 0 ] && [ $mis -eq 0 ] && [ "$gok" = pass ] && [ "$ncok" = ok ]
