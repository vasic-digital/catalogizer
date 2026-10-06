#!/usr/bin/env python3
"""T050/T056 paired mutations for tools/evidence/evcore.py (round 3: author mutants M1.., the 21 mutants of the
independent review ported to the round-3 code as RV-X*, and one or more mutants per round-3 check as N-*).

Each mutant is a literal source edit applied to a scratch copy of the tool tree: an (old, new) pair edits evcore.py, a
(file, old, new) triple edits another file of the tree (used for the test helper hermetic.sh). The core test files
(test_evrec.sh, test_evrec_more.sh, test_evrec_r3.sh, test_evrec_golden.sh; plus test_evrec_hermetic.sh for the mutants
that name it) are run against it. A mutant is CAUSE-CAUGHT only when at least one of the `expect` check-name fragments
appears on a FAIL line (a mutant that crashes the whole suite for an unrelated reason does not count). A mutant whose
edited Python source does not compile is reported INVALID (not caught). Mutants that survive are reported, never hidden. Usage: run_mutations.py [--check] [-j N] [ID...]   (--check only validates the edits)
"""
import concurrent.futures
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.realpath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
FEATURE = "specs/001-full-project-audit-remediation"

M = []   # (id, [(old, new), ...], [expected FAIL-line fragments])


def m(i, edits, expect, tests=None):
    M.append((i, edits, expect, tests))


m("V1-skip-prev-link-check", [('if rec["prev_hash"] != prev:', 'if False:')], ["link only"])
m("V2-skip-entry-hash-check", [('if hashed != rec["entry_hash"]:', 'if False:')], ["edited content"])
m("V3-skip-seq-contiguity", [('if rec["seq"] != n:', 'if False:')], ["non-contiguous seq", "re-sequenced", "kept local seq"])
m("V4-verify-never-walks", [('entries, head = chain_walk(led)', 'entries, head = [], ZERO')],
  ["entry 2 deleted without repair", "edited content", "reordered", "torn final line"])
m("V5-verify-always-fails", [('entries, head = chain_walk(led)', 'raise Refuse("chain_failure", 1, "x")')],
  ["original 4-entry ledger exits 0", "original restored exits 0", "chain still verifies", "pristine"])
m("V6-empty-ledger-passes", [('raise Refuse("UNVERIFIED", 3, "ledger empty: nothing to verify")', 'return 0, ZERO')],
  ["empty ledger is unverifiable"])
m("V7-unreadable-ledger-passes", [('raise Refuse("UNVERIFIED", 3, "ledger unreadable: %s" % e.strerror)', 'return 0, ZERO')],
  ["absent ledger is unverifiable"])
m("V8-torn-line-accepted", [('raise Refuse("chain_failure", 1, "truncated final line (no newline)")', 'pass')],
  ["torn final line"])
m("V9-anchor-note-dropped", [('print("note: no anchor compared (T055); chain-only verification")', 'pass')],
  ["silent about the missing anchor"])
m("R1-duration-zero", [('dur = (time.monotonic_ns() - t0) // 1_000_000', 'dur = 0')], ["duration self-test"])
m("R2-argv-joined-string", [('"argv": r_argv,', '"argv": " ".join(r_argv),')],
  ["validate against ev/1", "argv recorded as a list", "concurrent appenders"])
m("R3-no-schema-validation-at-all", [('        errs = schema_errors(rec)\n        if errs:\n            raise Refuse("record_invalid", 65, "; ".join(errs))\n        for data in blobs:',
                                       '        for data in blobs:'),
                                      ('    _prevalidate(rec)\n', '    pass\n'),
                                      ('    errs = schema_errors(dict(rec, seq=1, prev_hash=ZERO, entry_hash=ZERO))\n    if errs:', '    errs = []\n    if errs:')], ["RED without oracle"])
m("R4-check-record-accepts-all", [('    errs = schema_errors(probe)\n    if errs:\n        raise Refuse("record_invalid", 65, "; ".join(errs))\n    print("record valid")',
                                    '    print("record valid")')], ["check-record refuses"])
m("R5-no-ledger-lock", [('    lock.acquire()\n    try:', '    try:')], ["concurrent appenders", "live lock holder", "ledger changed under a live lock"])
m("R6-reap-live-holder", [('    pid, cmd = h.get("pid"), h.get("cmdline")', '    return True\n    pid, cmd = h.get("pid"), h.get("cmdline")')],
  ["live lock holder", "live holder process untouched", "ledger unchanged after refused"])
m("R7-never-reap", [('    pid, cmd = h.get("pid"), h.get("cmdline")', '    return False\n    pid, cmd = h.get("pid"), h.get("cmdline")')],
  ["stale ledger lock with dead holder"])
m("R8-write-in-place-not-temp-rename",
  [('        atomic_write(led, raw + canon(rec) + b"\\n", fault=True)',
    '        with open(led, "ab") as _f:\n            _f.write(canon(rec) + b"\\n")\n        if os.environ.get("EVREC_FAULT"):\n            os.kill(os.getpid(), signal.SIGKILL)')],
  ["changed by a killed write"])
m("R9-no-redaction", [('    reds = redactions()', '    reds = []')], ["planted secret present", "leaked via argv"])
m("R10-redacted-flag-dropped", [('    if h1 or h2:\n        rec["redacted"] = True', '    if False:\n        rec["redacted"] = True')],
  ["lacks redacted", "stream-only redacted flags"])
m("R11-argv-not-redacted", [('    r_argv, red_hit = redact_argv(argv, reds)', '    r_argv, red_hit = list(argv), False')],
  ["leaked via argv"])
m("R12-reconcile-always-ok", [('    if mine != count:', '    if False:')], ["runner count 4 != 3"])
m("R13-never-raise-owed", [('if sz * RAISE_DEN >= bound * RAISE_NUM:', 'if False:')], ["silent above 75%"])
m("R14-always-raise-owed", [('if sz * RAISE_DEN >= bound * RAISE_NUM:', 'if True:')], ["names ledger_raise_owed below"])
m("R15-exit-status-unmapped", [('    if 1 <= rc <= 125:\n        return "fail"\n    return "error"', '    return "fail"')],
  ["exit-status mapping"])
m("R16-signal-not-128-plus", [('            rc = 128 + (-rc)', '            rc = 1')], ["exit-status mapping"])
m("R17-test-fingerprint-dropped", [('    if tfp:\n        rec["test_fingerprint"] = tfp', '    if False:\n        rec["test_fingerprint"] = tfp')],
  ["valid RED entry not recorded"])
m("R18-target-fp-invented", [('    return ZERO, False', '    return ZERO, True')],
  ["unresolvable RED target accepted"])
m("R19-target-fp-not-file-bytes", [('            return file_sha(ref), True\n        if os.path.isdir', '            return sha_bytes(ref.encode()), True\n        if os.path.isdir')],
  ["target_fingerprint is not"])
m("R20-append-onto-corrupt-tail", [('            if not ok:\n                raise Refuse("ledger_inconsistent"', '            if False:\n                raise Refuse("ledger_inconsistent"')],
  ["append extended a corrupt"])
m("R21-stream-digest-no-blob", [('    done = append_entry(rec, blobs=(out, err), token=token)', '    done = append_entry(rec, blobs=(), token=token)')],
  ["no blob for", "no blob stored"])
m("R22-usage-accepted", [('            raise Refuse("usage_error", 64, "evrec {%s} ..." % "|".join(cmds))', '            return 0')],
  ["unknown subcommand"])
m("RR1-keep-local-seq", [('            seq += 1\n            rec["seq"], rec["prev_hash"] = seq, prev', '            rec["seq"], rec["prev_hash"] = old, prev')],
  ["re-sequenced", "output accepted by verify"])
m("RR2-drop-remote-prefix-bytes", [('        data = rraw + b"".join(out_lines)', '        data = b"".join(out_lines)')],
  ["remote prefix differs", "re-sequenced"])
m("RR3-no-common-prefix-check", [('    if cp == 0:', '    if False:')], ["without a common prefix"])
m("RR4-no-side-verify", [('chain_walk(o[k])\n            except Refuse as e:', 'pass\n            except Refuse as e:')],
  ["rerecord accepted a tampered side"])
m("RR5-seqmap-not-written", [('    if seqmap is not None:', '    if False:')], ["no seq map"])
m("RR6-append-only-drops-local", [('data, seqmap = rraw + b"".join(l + b"\\n" for l in suffix), None', 'data, seqmap = rraw, None')],
  ["append-only"])
m("RR7-local-fields-rewritten", [('            rec["seq"], rec["prev_hash"] = seq, prev\n', '            rec["seq"], rec["prev_hash"] = seq, prev\n            rec["item"] = "CAT-000"\n')],
  ["local fields changed"])
m("RM1-remap-noop", [('plan.append((fp, pat.sub(lambda mo: "ledger#%d" % m[int(mo.group(1))], txt)))', 'plan.append((fp, txt))')],
  ["remap-refs result"])
m("RM2-remap-accepts-uncovered", [('        if missing:', '        if False:'), ('"ledger#%d" % m[int(mo.group(1))]', '"ledger#%d" % m.get(int(mo.group(1)), int(mo.group(1)))')],
  ["remap-refs refuses a reference"])
m("RM3-remap-sequential-bug", [('pat.sub(lambda mo: "ledger#%d" % m[int(mo.group(1))], txt)',
                                 'pat.sub(lambda mo: "ledger#%d" % m[int(mo.group(1))], pat.sub(lambda mo: "ledger#%d" % m[int(mo.group(1))], txt))')],
  ["remap-refs result"])
m("C1-no-commit-turn-check",
  [('    commit_turn_check(allow_reap=True, where="pre_run")   # before anything is written (or run)', '    pass'),
   ('        commit_turn_check(allow_reap=False, where="in_lock")        # re-read inside the lock; the reaper already ran once', '        pass'),
   ('    commit_turn_check(allow_reap=False, where="post_command", note=note)   # a grant that appeared during the command', '    pass')],
  ["no run id", "different run id"])
m("C2-unreadable-grant-ignored", [('    if err:\n        raise Refuse("commit_turn_held", 76, err + tail)\n    if _is_holder(grant):\n        return\n    if allow_reap:',
                                    '    if err:\n        return\n    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    if allow_reap:')],
  ["mode 000 grant not refused", "non-JSON grant not refused"])
m("C3-matching-run-id-refused", [('    if _is_holder(grant):\n        return\n    if allow_reap:',
                                  '    if False:\n        return\n    if allow_reap:')], ["matching run id", "stub called for matching run id"])
m("C4-matching-run-id-calls-reaper", [('    if _is_holder(grant):\n        return\n    if allow_reap:',
                                      '    if False:\n        return\n    if allow_reap:'),
                                     ('    raise Refuse("commit_turn_held", 76, "grant held by run_id %s%s" % (grant["run_id"], tail))',
                                      '    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    raise Refuse("commit_turn_held", 76, "grant held by run_id %s%s" % (grant["run_id"], tail))')],
  ["stub called for matching run id"])
m("C5-reaper-never-called", [('            subprocess.run([entry,', '            (lambda *a, **k: None)([entry,')], ["--reap call count", "dead-holder"])
m("C6-reap-direct-not-exec-approved",
  [('[entry, "--exec-approved", "scripts/release/commit_turn_check.sh", "--reap"]', '["bash", os.path.join(repo_root(), "scripts/release/commit_turn_check.sh"), "--reap"]')],
  ["no --exec-approved entry"])
m("C7-reap-by-grant-age", [('    raise Refuse("commit_turn_held", 76, "grant held by run_id %s%s" % (grant["run_id"], tail))', '    return')],
  ["no run id", "live old grant"])
m("C8-host-refusal-crash-allowed", [('        except (OSError, subprocess.SubprocessError):\n            pass\n        grant, err = _read_grant()\n        if grant is None and err is None:\n            return',
                                     '        except (OSError, subprocess.SubprocessError):\n            pass\n        return')], ["host refusing", "stores changed under a held grant", "command ran under a held grant"])
m("C9-blobs-written-before-check",
  [('    commit_turn_check(allow_reap=True, where="pre_run")   # before anything is written (or run)', '    os.makedirs(blobs_path(), exist_ok=True); open(os.path.join(blobs_path(), "pre"), "w").close()')],
  ["stores changed under a held grant"])
m("C10-reaper-called-repeatedly", [('    raise Refuse("commit_turn_held", 76, "grant held by run_id %s%s" % (grant["run_id"], tail))', '    return commit_turn_check(allow_reap=True, where=where)')],
  ["stub call count", "--reap call count", "no run id"])


# ---- the 21 mutants of the independent review (WF2-REVIEW-wp05-evrec, section 3 I5), ported to the round-3 code.
#      Each one SURVIVED the round-2 suites; each must now be caught by a check that names its cause.
m("RV-X1-hash-skips-exit_status-and-verdict", [('body = {k: v for k, v in rec.items() if k != "entry_hash"}',
   'body = {k: v for k, v in rec.items() if k not in ("entry_hash", "exit_status", "verdict")}')], ["golden", "bad hash", "I5"])
m("RV-X2-hash-drops-prev_hash-prefix", [('return sha_bytes(rec["prev_hash"].encode("ascii") + canon(body))', 'return sha_bytes(canon(body))')], ["golden", "I5"])
m("RV-X3-canon-ensure_ascii-true", [('separators=(",", ":"), ensure_ascii=False)', 'separators=(",", ":"), ensure_ascii=True)')], ["golden", "I5"])
m("RV-X4-no-in-lock-grant-recheck", [('        commit_turn_check(allow_reap=False, where="in_lock")        # re-read inside the lock; the reaper already ran once', '        pass')],
  ["in-lock", "in_lock"])
m("RV-X5-release-deletes-foreign-lock", [('                if h.get("pid") == os.getpid():', '                if True:')], ["lock.release"])
m("RV-X6-append-skips-seq-count-check", [(' and last["seq"] == len(lines)', '')], ["X6"])
m("RV-X7-verdict-126-as-fail", [('    if 1 <= rc <= 125:', '    if 1 <= rc <= 126:')], ["X7"])
m("RV-X8-dir-fingerprint-ignores-content", [('h.update(os.fsencode(os.path.relpath(p, ref)) + b"\\0" + file_sha(p).encode() + b"\\n")',
   'h.update(os.path.relpath(p, ref).encode() + b"\\n")')], ["X8"])
m("RV-X9-unreadable-holder-reaped", [('        return False                                   # unreadable holder identity: conservative, never reaped', '        return True')],
  ["I1", "stale:"])
m("RV-X10-pid-reuse-not-detected", [('    return cur != cmd.rstrip(" ")', '    return False')], ["pid reuse", "stale:"])
m("RV-X11-flake-ledger-not-measured", [('    for p in (led, flake):', '    for p in (led,):')], ["X11"])
m("RV-X12-verify-ignores-schema-key", [('or any(k not in rec for k in ("schema", "seq", "prev_hash", "entry_hash"))', 'or any(k not in rec for k in ("seq", "prev_hash", "entry_hash"))')], ["X12"])
m("RV-X13-no-common-prefix-crashes", [('        raise Refuse("no_common_prefix", 68, "the two sides share no leading entry")', '        raise RuntimeError("crash")')], ["X13"])
m("RV-X14-map-incomplete-crashes", [('            raise Refuse("map_incomplete", 68,', '            raise RuntimeError(')], ["X14"])
m("RV-X15-lock-held-crashes", [('                raise Refuse("lock_held", 75, why)', '                raise RuntimeError(why)')], ["X15", "lock_held", "I1"])
m("RV-X16-check-record-refusal-crashes", [('        raise Refuse("record_invalid", 65, "; ".join(errs))\n    print("record valid")', '        raise RuntimeError("x")\n    print("record valid")')], ["X16", "minor5"])
m("RV-X17-seqmap-old-equals-new", [('seqmap.append({"old": old, "new": seq, "digest": prev})', 'seqmap.append({"old": seq, "new": seq, "digest": prev})')], ["seq map"])
m("RV-X18-rerecord-skips-schema-check", [('            errs = schema_errors(rec)\n            if errs:\n                raise Refuse("record_invalid", 65, "re-chained', '            errs = []\n            if errs:\n                raise Refuse("record_invalid", 65, "re-chained')], ["X18"])
m("RV-X19-check-record-refuses-everything", [('    print("record valid")\n    return 0', '    raise Refuse("record_invalid", 65, "always")')], ["X19"])
m("RV-X20-pre-release-flag-dropped", [('    if opts.get("pre_release"):\n        rec["pre_release"] = True', '    if False:\n        rec["pre_release"] = True')], ["X20"])
m("RV-X21-unset-runid-counts-as-holder", [('    if _is_holder(grant):\n        return\n    if allow_reap:',
   '    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID", grant["run_id"]):\n        return\n    if allow_reap:')], ["no run id", "stores changed", "--reap call count"])

# ---- one or more mutants per round-3 check
m("N-I1-unreadable-cmdline-reaped (the round-2 behaviour)", [('    if cur is None:\n        return False                                   # alive, identity unreadable: not proven stale', '    if cur is None:\n        return True')],
  ["I1:", "stale:"])
m("N-I2a-no-post-command-check", [('    commit_turn_check(allow_reap=False, where="post_command", note=note)   # a grant that appeared during the command', '    pass')], ["post-command", "post_command", "stores written under a foreign grant"])
m("N-I2b-blobs-stored-before-the-in-lock-check",
  [('        for data in blobs:\n            store_blob(data)\n        atomic_write', '        atomic_write'),
   ('        commit_turn_check(allow_reap=False, where="in_lock")        # re-read inside the lock; the reaper already ran once',
    '        for data in blobs:\n            store_blob(data)\n        commit_turn_check(allow_reap=False, where="in_lock")')], ["after the in-lock refusal"])
m("N-I2c-stall-hook-never-fires", [('    if os.environ.get("EVREC_FAULT") == "stall_before_lock":\n        _stall_for_test(ledger_path())', '    pass')], ["stall point never reached"])
m("N-I3a-verify-ignores-unknown-args", [('        else:\n            raise Refuse("usage_error", 64, "verify [--ledger FILE] [--blobs DIR]; refused argument %r" % a)', '        else:\n            pass')], ["I3:"])
m("N-I3b-verify-does-not-name-the-file", [('    print("verify: ledger=%s blobs=%s" % (led, bdir))', '    pass')], ["names the file it read", "default ledger"])
m("N-I3c-ledger-option-ignored", [('    led = led or ledger_path()', '    led = ledger_path()')], ["--ledger tampered"])
m("N-I4a-blobs-never-checked", [('    check_blobs(entries, bdir)', '    pass')], ["I4: forged", "I4: every blob deleted"])
m("N-I4b-blob-hash-never-compared", [('            elif file_sha(p) != d:', '            elif False:')], ["I4: forged"])
m("N-I4c-missing-blob-passes", [('    if missing:\n        raise Refuse("blob_missing", 3,', '    if False:\n        raise Refuse("blob_missing", 3,')], ["I4: every blob deleted"])
m("N-I4d-schema-never-checked-in-verify", [('            errs = schema_errors(rec)\n            if errs:\n                raise Refuse("schema_invalid"', '            errs = []\n            if errs:\n                raise Refuse("schema_invalid"')],
  ["I4: re-chained entry", "I4: EV_SCHEMA", "I4: under PYTHONOPTIMIZE"])
m("N-I4e-producer-chooses-the-schema", [('    return os.path.join(tool_root(), FEATURE, "contracts", "evidence-record.schema.json")', '    return os.environ.get("EV_SCHEMA") or os.path.join(tool_root(), FEATURE, "contracts", "evidence-record.schema.json")')],
  ["EV_SCHEMA"])
m("N-I4f-canonical-form-not-required", [('            if line != canon(rec):', '            if False:')], ["hash-valid line that is not byte-canonical", "duplicate key"])
m("N-I4g-anchor-present-is-ok", [('    if os.path.exists(a) and os.path.getsize(a) > 0:\n        raise Refuse("anchor_not_compared"', '    if False:\n        raise Refuse("anchor_not_compared"')], ["anchor present"])
m("N-I4h-bad-bound-crashes", [('            raise Refuse("usage_error", 64, "%s=%r is not an integer" % (env, os.environ[env]))', '            raise ValueError(env)')], ["EV_LEDGER_BOUND"])
m("N-I4i-assert-style-checks-under-optimize", [('            errs = schema_errors(rec)\n            if errs:\n                raise Refuse("schema_invalid"', '            errs = schema_errors(rec)\n            assert not errs\n            if False:\n                raise Refuse("schema_invalid"')], ["PYTHONOPTIMIZE"])
m("N-I5-unicode-escape-in-line-check", [('            if line != canon(rec):', '            if line != json.dumps(rec, sort_keys=True, separators=(",", ":")).encode():')], ["golden-good"])
m("N-I6a-run-lists-no-owed-relocation", [('    for line in owed_relocations({done["stdout_sha256"]: len(out), done["stderr_sha256"]: len(err)}):', '    for line in []:')], ["no owed_relocation"])
m("N-I6b-verify-lists-no-owed-relocation", [('        for line in owed_relocations(sizes):', '        for line in []:')], ["I6: verify lists"])
m("N-I6c-bound-ignored", [('if sz > bound]', 'if sz > bound * 1000]')], ["I6:"])
m("N-I6d-empty-bound-env-is-not-unset", [('    if os.environ.get(env):\n        try:', '    if env in os.environ:\n        try:')], ["default bound is not"])
m("N-I6e-rerecord-out-guard-removed", [('    _guard_out(o)\n', '    pass\n')], ["I6: rerecord --out"])
m("N-I6f-out-guard-only-checks-files", [('    if any(_inside(outdir, sd) for sd in shared_dirs) or outs & protected:', '    if outs & protected:')], ["<shared dir> --name <other file>"])
m("N-I6g-out-guard-only-checks-the-directory", [('    if any(_inside(outdir, sd) for sd in shared_dirs) or outs & protected:', '    if any(_inside(outdir, sd) for sd in shared_dirs):')], ["over one of its own input files"])
m("N-I6h-rerecord-takes-no-lock", [('        lock.acquire()                                   # one writer per output ledger (11.4.180)', '        pass')], ["takes the lock of the output ledger"])
m("N-I7a-overlapping-spans-not-merged", [('        if merged and s <= merged[-1][1]:', '        if False:')], ["leaked", "redaction:"])
m("N-I7b-default-patterns-off", [('    if patterns:\n        for label, rx, grp in SECRET_PATTERNS:', '    if False:\n        for label, rx, grp in SECRET_PATTERNS:')], ["I7:", "default pattern"])
m("N-I7c-target-ref-not-redacted", [('    r_ref, h = redact_text(ref, reds)', '    r_ref, h = ref, False')], ["target_ref"])
m("N-I7d-cwd-not-redacted", [('    r_cwd, h = redact_text(cwd, reds)', '    r_cwd, h = cwd, False')], ["cwd secret stored"])
m("N-I7e-streams-without-patterns", [('    out, h1 = redact_bytes(out, reds)\n    err, h2 = redact_bytes(err, reds)', '    out, h1 = redact_bytes(out, reds, patterns=False)\n    err, h2 = redact_bytes(err, reds, patterns=False)')], ["I7: Bearer", "I7: AWS", "I7: a PEM"])
m("N-I7f-named-values-not-redacted-in-streams", [('    out, h1 = redact_bytes(out, reds)\n    err, h2 = redact_bytes(err, reds)', '    out, h1 = redact_bytes(out, [], patterns=True)\n    err, h2 = redact_bytes(err, [], patterns=True)')], ["planted secret present", "leaked", "stream-only secret stored"])
for lab, rxstart in (("bearer", '("bearer", re.compile(rb"(?i)\\bBearer'), ("aws", '("aws_key_id", re.compile(rb"\\b(?:AKIA'), ("github", '("github_token", re.compile(rb"\\bgh'),
                     ("url_userinfo", '("url_userinfo", re.compile(rb"://'), ("pem", '("pem_private_key", re.compile(rb"-----BEGIN')):
    # the never-matching prefix goes AFTER a leading global flag: before it, `(?i)` is not at the start and the module cannot be imported
    # (review W7-6: round 3-6 "caught" the bearer mutant by crashing every call)
    m("N-I7g-pattern-%s-never-matches" % lab, [(rxstart, rxstart.replace('rb"(?i)', 'rb"(?i)(?!x)x', 1) if 'rb"(?i)' in rxstart else rxstart.replace('re.compile(rb"', 're.compile(rb"(?!x)x', 1))],
      ["I7:", "default pattern"] + (["W7-6: a bare Bearer"] if lab == "bearer" else []))
m("N-I7h-kv-pattern-never-matches", [('("kv_secret", re.compile(rb"(?i)" + _NAMERUN', '("kv_secret", re.compile(rb"(?!x)x" + _NAMERUN')], ["I7:", "default pattern kv"])
m("N-I8a-unresolved-fingerprint-allowed-in-verify", [('            if rec.get("target_fingerprint") == ZERO and rec.get("polarity") != "PROBE":', '            if False:')], ["unresolved-fingerprint sentinel"])
m("N-I8b-baseline-with-unresolved-target-accepted", [('    if not resolved and polarity != "PROBE":', '    if not resolved and polarity in ("RED", "GREEN", "MUTATION", "REOPEN"):')], ["I8:"])
m("N-I8c-sentinel-is-a-hash-of-the-ref", [('    return ZERO, False', '    return sha_bytes(b"UNRESOLVED:" + ref.encode()), False')], ["unresolved PROBE fingerprint is"])
m("N-I9a-interpreter-rule-off", [('    if polarity in ("RED", "GREEN", "MUTATION") and not sources and is_interpreter(argv[0]):', '    if False:')], ["I9:"])
m("N-I9b-bash-not-an-interpreter", [('INTERPRETERS = {"bash", "sh",', 'INTERPRETERS = {"sh",')], ["I9:"])
m("N-I9c-env-sources-not-counted", [('    sources = list(opts["test_source"]) + [s for s in os.environ.get("EV_TEST_SOURCES", "").split() if s]', '    sources = list(opts["test_source"])')], ["EV_TEST_SOURCES"])
m("N-I9d-rule-also-hits-probe", [('    if polarity in ("RED", "GREEN", "MUTATION") and not sources and is_interpreter(argv[0]):', '    if not sources and is_interpreter(argv[0]):')], ["PROBE with an interpreter"])
m("N-I10a-independence-asserted-by-the-recorder", [('    if opts.get("oracle") and not opts.get("independent"):', '    if False:')], ["I10:"])
m("N-I10b-independent-without-oracle-ok", [('    if opts.get("independent") and not opts.get("oracle"):', '    if False:')], ["--oracle-independent without --oracle"])
m("N-I11a-no-pre-run-validation", [('    _prevalidate(rec)\n', '    pass\n')], ["I11: RED without oracle", "I11: --pre-release", "schema_unavailable"])
m("N-I11b-bad-mutation-json-crashes", [('        except ValueError as e:\n            raise Refuse("usage_error", 64, "--mutation-json is not JSON: %s" % e)', '        except KeyError as e:\n            raise Refuse("usage_error", 64, "--mutation-json is not JSON: %s" % e)')], ["I11: invalid --mutation-json"])
m("N-I11c-encoding-not-checked", [('        try:\n            val.encode("utf-8")', '        try:\n            pass')], ["non-UTF-8"])
m("N-I11d-test-source-not-checked", [('        if not (os.path.isfile(s) and os.access(s, os.R_OK)):', '        if False:')], ["missing --test-source"])
m("N-I11e-unknown-flag-is-positional", [('        elif a.startswith("--"):\n            raise Refuse("usage_error", 64, "unknown flag', '        elif False:\n            raise Refuse("usage_error", 64, "unknown flag')], ["flag standing where REF belongs", "unknown flag"])
m("N-I11f-blobs-stored-before-validation", [('    errs = schema_errors(dict(rec, seq=1, prev_hash=ZERO, entry_hash=ZERO))\n    if errs:', '    store_blob(out); store_blob(err)\n    errs = schema_errors(dict(rec, seq=1, prev_hash=ZERO, entry_hash=ZERO))\n    if errs:')], ["orphan blobs"])
m("N-I11g-wrong-exit-for-oracle", [('        raise Refuse("oracle_independence_undeclared", 64,', '        raise Refuse("oracle_independence_undeclared", 65,')], ["I10:"])
m("N-I7i-stderr-hit-does-not-set-redacted", [('    if h1 or h2:\n        rec["redacted"] = True', '    if h1:\n        rec["redacted"] = True')], ["redacted flags"])
m("N-m5-date-format-not-checked", [('        if not ok:\n            out.append("started_at', '        if False:\n            out.append("started_at')], ["minor5"])
m("N-m10-blob-bound-not-validated-before-the-run", [('    lock_timeout()\n    owed_relocations({})', '    lock_timeout()')], ["non-integer EV_BLOB_BOUND", "command ran under a bad EV_BLOB_BOUND"])
# ---- hermeticity (finding I12): mutants of the test helper, caught only by test_evrec_hermetic.sh
m("H-I12a-repo-root-not-set", [("tools/evidence/tests/hermetic.sh", 'CPA_HOST_ENTRY=$H/host_entry.sh EVREC_REPO_ROOT=$H/repo\n}', 'CPA_HOST_ENTRY=$H/host_entry.sh\n}'),
                               ("tools/evidence/tests/hermetic.sh", 'hermetic_repo() { export EVREC_REPO_ROOT=$H/repo CPA_HOST_ENTRY=$H/host_entry.sh;', 'hermetic_repo() { export CPA_HOST_ENTRY=$H/host_entry.sh;')], ["hermetic:"], ["test_evrec_hermetic.sh"])
m("H-I12b-test-sources-env-not-unset", [("tools/evidence/tests/hermetic.sh", 'EV_ANCHOR EV_SCHEMA EV_TEST_SOURCES EV_LEDGER_BOUND', 'EV_ANCHOR EV_SCHEMA EV_LEDGER_BOUND')], ["hermetic:"], ["test_evrec_hermetic.sh"])
m("H-I12c-lock-timeout-env-not-unset", [("tools/evidence/tests/hermetic.sh", 'EVREC_FAULT EVREC_LOCK_TIMEOUT EVREC_TURN_RUN_ID', 'EVREC_FAULT EVREC_TURN_RUN_ID')], ["hermetic:"], ["test_evrec_hermetic.sh"])
m("H-I12d-blob-bound-env-not-unset", [("tools/evidence/tests/hermetic.sh", 'EV_LEDGER_BOUND EV_BLOB_BOUND EV_FLAKE_LEDGER', 'EV_LEDGER_BOUND EV_FLAKE_LEDGER')], ["hermetic:"], ["test_evrec_hermetic.sh"])
m("H-I12e-proc-root-env-not-unset", [("tools/evidence/tests/hermetic.sh", 'EVREC_TURN_RUN_ID EVREC_PROC_ROOT EVREC_RUN_TOKEN', 'EVREC_TURN_RUN_ID EVREC_RUN_TOKEN')], ["hermetic:"], ["test_evrec_hermetic.sh"])
m("H-I12f-cwd-not-isolated", [("tools/evidence/tests/hermetic.sh", 'mkdir -p "$H/repo/.audit" "$H/home" "$H/cwd"; cd "$H/cwd" || return 1', 'mkdir -p "$H/repo/.audit" "$H/home" "$H/cwd"')], ["hermetic:"], ["test_evrec_hermetic.sh"])
# H-I12b (host entry not stubbed) is not a separate mutant: without the repo-root override (H-I12a) the leaked grant makes the
# suites fail AND reach the host-entry stand-in; with the override the reaper path is never reached outside the ct fixtures,
# which set their own CPA_HOST_ENTRY. Stated in T050r3-implementation.md.

# ---- round 4: the reviewer's guard-removing mutants (W3-*) and one or more mutants per round-4 guard (N1-N5, m1-m4)
m("W3-1-eperm-holder-reaped", [('    except PermissionError:\n        pass\n    if unreadable:', '    except PermissionError:\n        return True\n    if unreadable:')], ["W3-1"])
m("W3-2-missing-before-mismatched", [('    if mismatched:\n        raise Refuse("blob_mismatch", 1, "; ".join(mismatched[:3]))\n    if missing:\n        raise Refuse("blob_missing", 3, "%d blob(s) missing in %s (stream UNVERIFIED): %s" % (len(missing), bdir, "; ".join(missing[:3])))',
                                       '    if missing:\n        raise Refuse("blob_missing", 3, "%d blob(s) missing in %s (stream UNVERIFIED): %s" % (len(missing), bdir, "; ".join(missing[:3])))\n    if mismatched:\n        raise Refuse("blob_mismatch", 1, "; ".join(mismatched[:3]))')], ["W3-2"])
m("W3-3-python-not-an-interpreter", [('"python", "perl",', '"perl",')], ["N3: RED python3"])
m("W3-4-env-not-an-interpreter", [('"dotnet", "env", "pwsh",', '"dotnet", "pwsh",')], ["env bash"])
m("W3-5-impossible-date-accepted", [('datetime.datetime.strptime(ts[:19], "%Y-%m-%dT%H:%M:%S")', 'ts[:19]')], ["W3-5"])
m("W3-6-lock-records-empty-cmdline", [('        ident = {"cmdline": own} if own else {"cmdline": None, "cmdline_unreadable": True}', '        ident = {"cmdline": ""}')], ["W3-6"])
m("W3-7-explicit-ledger-skips-blobs", [('    check_blobs(entries, bdir)\n    a = anchor_path()', '    if default:\n        check_blobs(entries, bdir)\n    a = anchor_path()')], ["W3-7"])
m("W3-8-malformed-grant-is-no-grant", [('            raise ValueError("not a grant object")', '            return None, None')], ["W3-8"])
m("W3-9-in-lock-check-reaps", [('commit_turn_check(allow_reap=False, where="in_lock")', 'commit_turn_check(allow_reap=True, where="in_lock")')], ["I2"])
m("W3-10-name-guard-removed", [('    _guard_name(o["name"])\n    _guard_out(o)', '    _guard_out(o)')], ["N1: --name"])
m("N1-name-guard-allows-separator", [('_NAME_RX = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*")', '_NAME_RX = re.compile(r"[A-Za-z0-9][A-Za-z0-9._/-]*")')], ["N1: --name 'a/b'"])
m("N1-name-guard-allows-reserved", [('name == "ledger-seq-map.json" or name.endswith(_RESERVED_NAME_SUFFIX) or ".tmp." in name', 'False')], ["ledger-seq-map.json"])
m("N1-blob-store-not-shared", [('    shared_dirs = {rp(os.path.dirname(ledger_path()) or "."), rp(ev_dir()), rp(blobs_path())}', '    shared_dirs = {rp(os.path.dirname(ledger_path()) or "."), rp(ev_dir())}')], ["relocated blob store"])
m("N1-ev-dir-not-shared", [('    shared_dirs = {rp(os.path.dirname(ledger_path()) or "."), rp(ev_dir()), rp(blobs_path())}', '    shared_dirs = {rp(os.path.dirname(ledger_path()) or "."), rp(blobs_path())}')], ["evidence dir"])
m("N1-inside-only-equal", [('    return path == base or path.startswith(base.rstrip(os.sep) + os.sep)', '    return path == base')], ["any depth"])
m("N2-no-json-secret", [('SECRET_PATTERNS = [', '_NEVER = re.compile(rb"(?!x)x")\nSECRET_PATTERNS = ['), ('    ("json_secret", re.compile(', '    ("json_secret", _NEVER or re.compile(')], ["N2: password value containing a space"])
m("N2-no-flag-pair-pattern", [('SECRET_PATTERNS = [', '_NEVER = re.compile(rb"(?!x)x")\nSECRET_PATTERNS = ['), ('    ("flag_pair", re.compile(', '    ("flag_pair", _NEVER or re.compile(')], ["N2: stream text", "W7-1: -password V (single dash at the start)"])
m("N2-no-authorization-pattern", [('SECRET_PATTERNS = [', '_NEVER = re.compile(rb"(?!x)x")\nSECRET_PATTERNS = ['), ('    ("authorization", re.compile(', '    ("authorization", _NEVER or re.compile(')], ["N2: Authorization"])
m("N2-no-cookie-pattern", [('SECRET_PATTERNS = [', '_NEVER = re.compile(rb"(?!x)x")\nSECRET_PATTERNS = ['), ('    ("cookie", re.compile(', '    ("cookie", _NEVER or re.compile(')], ["N2: Cookie"])
m("N2-no-jwt-pattern", [('SECRET_PATTERNS = [', '_NEVER = re.compile(rb"(?!x)x")\nSECRET_PATTERNS = ['), ('    ("jwt", re.compile(', '    ("jwt", _NEVER or re.compile(')], ["N2: a bare three-part JWT"])
m("N2-argv-pair-not-redacted", [('        prev_cred = _is_cred_flag(a)', '        prev_cred = False')], ["N2: argv pair"])
m("N2-argv-flag-regex-too-narrow", [('_CRED_WORD = re.compile(r"(?i)password|passwd|secret|token|api[_-]?key|access[_-]?key")', '_CRED_WORD = re.compile(r"(?i)password")')], ["N2: argv credential flags"])
m("N2-redact-everything", [('    ("kv_secret", re.compile(', '    ("everything", re.compile(rb"(?s)(.+)"), 1),\n    ("kv_secret", re.compile(')], ["benign text was altered"])
m("N3-launcher-run_pinned-not-listed", [('"run_pinned", "run_pinned.sh", ', '')], ["run_pinned.sh without a declared test source"])
m("N3-launcher-kcov-not-listed", [('"kcov", "podman",', '"podman",')], ["launcher 'kcov'"])
m("N3-launcher-podman-docker-ssh-not-listed", [('"podman", "docker", "nerdctl", "ssh", "adb",', '"nerdctl", "adb",')], ["launcher 'podman'", "launcher 'docker'", "launcher 'ssh'"])
m("N3-operands-not-hashed", [('[] if (sources or is_interpreter(argv[0])) else operand_files(argv)', '[]')], ["unlisted launcher"])
m("N3-operands-always-hashed", [('[] if (sources or is_interpreter(argv[0])) else operand_files(argv)', 'operand_files(argv)')], ["N3: test_fingerprint of an interpreter with a declared source"])
m("N4-token-ignored-by-run", [('    token = run_token()\n', '    token = None\n')], ["N4: reconcile --run run4 --runner-count 4"])
m("N4-run-index-not-written", [('        if token:                                       # the run index', '        if False:                                       # the run index')], ["N4: reconcile --run run4 --runner-count 4"])
m("N4-run-index-hash-not-compared", [('hashes.get(r["seq"]) == r.get("entry_hash")', 'True')], ["run-index row whose entry_hash"])
m("N4-run-index-run-not-compared", [('r.get("run") == token', 'True')], ["another run's token"])
m("N4-unscoped-on-tokened-allowed", [('        if os.path.exists(rp) and os.path.getsize(rp) > 0:\n            raise Refuse("reconcile_unscoped_on_tokened_ledger"', '        if False:\n            raise Refuse("reconcile_unscoped_on_tokened_ledger"')], ["unscoped reconcile on a ledger"])
m("N4-token-not-validated", [('    if RUN_TOKEN_RX.fullmatch(t) is None:', '    if False:')], ["token that is not a plain token", "EVREC_RUN_TOKEN that is not"])
m("N4-env-token-ignored-by-reconcile", [('    t = explicit if explicit is not None else os.environ.get("EVREC_RUN_TOKEN")', '    t = explicit')], ["token may come from EVREC_RUN_TOKEN"])
m("N4-reconcile-accepts-unknown-option", [('        else:\n            raise Refuse("usage_error", 64, usage)\n        i += 2', '        else:\n            pass\n        i += 2')], ["unknown option"])
m("m1-ledger-anchor-from-default", [('    a = anchor_path() if default or os.environ.get("EV_ANCHOR") else os.path.join(os.path.dirname(os.path.abspath(led)), "anchors.jsonl")', '    a = anchor_path()')], ["m1:"])
m("m2-remap-drops-mode", [('mode=stat.S_IMODE(os.stat(real).st_mode)', 'mode=None')], ["m2: mode is"])
m("m2-remap-replaces-symlink", [('        real = os.path.realpath(fp)                       # write through a symlink, keep the link (review m2)', '        real = fp')], ["m2: symlink replaced"])
m("m4-deleted-cwd-traceback", [('    except OSError:\n        raise Refuse("usage_error", 64, "the current working directory no longer exists; the command was not run")', '    except ZeroDivisionError:\n        raise')], ["m4:"])
m("m4-schema-unavailable-is-67", [('code = 3 if r.reason == "schema_unavailable" else r.code', 'code = r.code')], ["m4: verify without a readable schema"])
m("N5-unreadable-marker-never-reaped-even-dead", [('    if unreadable:\n        return False                                   # alive, identity never recorded: not proven stale', '    if unreadable:\n        return True')], ["W3-6: a lock taken while", "W3-6: and a reader"])
m("N5-unreadable-marker-ignored", [('    unreadable = (cmd is None and h.get("cmdline_unreadable") is True) or cmd == ""', '    unreadable = False')], ["W3-6: and a reader", "W3-6: but a holder"])

m("N5-portable-schema-off", [('    if isinstance(s, dict) and isinstance(s.get("$id"), str) and ":" not in s["$id"]:', '    if False:')], ["portable_schema"])

# ---- round 5 (WF5-REVIEW findings F1 F2 F3 F5 F6 F7): one or more mutants per guard, each caught by a named test_evrec_r5.sh check
m("F1-kv-pattern-quadratic", [('_NAMERUN = rb"(?<!" + _NM + rb")(?=" + _NM + rb"*?" + _CRED + rb")" + _NM + rb"*+"', '_NAMERUN = rb"(?=" + _NM + rb"*?" + _CRED + rb")" + _NM + rb"*+"')], ["F1: redact_bytes on", "R6-1: redact_bytes on"])
m("F1-flag-pair-quadratic", [('("flag_pair", re.compile(rb"(?i)(?<!" + _FM + rb")-{1,2}+', '("flag_pair", re.compile(rb"(?i)-{1,2}+')], ["F1: redact_bytes on", "R6-1: redact_bytes on"])
m("F1-json-tail-quadratic", [('_NM + rb"*?" + _CRED + rb")" + _NM + rb"*+"', '_NM + rb"*?" + _CRED + rb")" + _NM + rb"*"')], ["F1: redact_bytes on", "R6-1: redact_bytes on"])
m("F1-argv-flag-quadratic", [('    return _FLAG_SHAPE.fullmatch(a) is not None and _CRED_WORD.search(a) is not None',
                             '    return re.fullmatch(r"(?i)--?[A-Za-z0-9_-]*(?:password|passwd|secret|token|api[_-]?key|access[_-]?key)[A-Za-z0-9_-]*", a) is not None')], ["F1: redact_argv"])
m("F1-kv-pattern-dropped", [('("kv_secret", re.compile(rb"(?i)" + _NAMERUN', '("kv_secret", re.compile(rb"(?!x)x" + _NAMERUN')], ["F1: NOT redacted"])
m("F2-empty-run-id-is-a-holder", [('        if not isinstance(d, dict) or not isinstance(d.get("run_id"), str) or not d["run_id"].strip():', '        if not isinstance(d, dict) or not isinstance(d.get("run_id"), str):'),
                                  ('    me = os.environ.get("EVREC_TURN_RUN_ID") or None\n    return me is not None and grant["run_id"] == me', '    return grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID")')], ["F2:"])
m("F2-whitespace-run-id-accepted", [('or not d["run_id"].strip():', ':'), ], ["F2: a whitespace-only run_id"])
m("F3-opt-equals-value-not-hashed", [('            cand = a.split("=", 1)[1]', '            continue')], ["F3: --opt=FILE launcher gives", "F3: --opt=FILE fingerprint"])
m("F3-directory-operand-not-hashed", [('(os.path.isfile(cand) or os.path.isdir(cand))', '(os.path.isfile(cand))')], ["F3: directory operand gives"])
m("F3-realpath-of-argv0-ignored", [('        return bool(f) and _is_interp_name(os.path.basename(os.path.realpath(f)))', '        return False')], ["F3: RED through a renamed symlink"])
m("F3-oversized-directory-silently-skipped", [('            if e.reason == "too_large":\n                raise Refuse("operand_directory_too_large"', '            if False:\n                raise Refuse("operand_directory_too_large"')], ["F3: a directory operand over EVREC_OPERAND_DIR_MAX"])
m("F3-dir-digest-ignores-content", [('h.update(os.fsencode(os.path.relpath(p, path)) + b"\\0" + d.encode() + b"\\n")', 'h.update(os.fsencode(os.path.relpath(p, path)) + b"\\0")')], ["F3: directory operand gives"])
m("F5-ev-ledger-ignores-anchor-beside", [('    if os.environ.get("EV_LEDGER"):\n        return os.path.join(os.path.dirname(os.path.abspath(os.environ["EV_LEDGER"])), "anchors.jsonl")\n', '')], ["F5: EV_LEDGER with anchors.jsonl beside it"])
m("F5-explicit-anchor-ignored", [('    if os.environ.get("EV_ANCHOR"):\n        return os.environ["EV_ANCHOR"]\n', '')], ["F5: an explicit EV_ANCHOR still wins"])
m("F6-duplicate-index-row-counted-twice", [('                        seen.add(r["seq"])', '                        seen.add(object())')], ["F6: with that run-index row duplicated"])
m("F6-seq-not-an-int-crashes", [('isinstance(r.get("seq"), int) \\\n                            and hashes.get(r["seq"])', 'hashes.get(r.get("seq"))')], ["F6:"])
m("F7-existing-directory-not-refused", [('        if os.path.isdir(p):                              # review F7', '        if False:                                         # review F7')], ["F7: --out <parent of EV>", "F7: --name that is an existing directory"])
m("F7-tmp-left-after-failed-write", [('            os.unlink(tmp)                               # a failed write leaves no temporary file behind (review F7)', '            pass')], ["F7: atomic_write leftovers"])
m("F7-lock-error-traceback", [('    except OSError as e:\n        raise Refuse("out_unwritable", 70, "cannot take the output lock', '    except ZeroDivisionError as e:\n        raise Refuse("out_unwritable", 70, "cannot take the output lock')], ["F7: an unwritable --out"])

# ---- round 6 (WF6-REVIEW findings R6-1 R6-2 R6-4 R6-5 R6-6 R6-7 R6-8 R6-11): the reviewer mutants WF6-1..7 re-expressed on the unbounded code, plus one or more per new guard
m("R6-bound-name-run-128", [('_NAMERUN = rb"(?<!" + _NM + rb")(?=" + _NM + rb"*?" + _CRED + rb")" + _NM + rb"*+"', '_NAMERUN = rb"(?<!" + _NM + rb")(?=" + _NM + rb"*?" + _CRED + rb")" + _NM + rb"{0,128}"')], ["R6-2: NOT redacted"])
m("R6-bound-name-run-100", [('_NAMERUN = rb"(?<!" + _NM + rb")(?=" + _NM + rb"*?" + _CRED + rb")" + _NM + rb"*+"', '_NAMERUN = rb"(?<!" + _NM + rb")(?=" + _NM + rb"*?" + _CRED + rb")" + _NM + rb"{0,100}"')], ["R6-2: NOT redacted"])
m("R6-bound-flag-run-8", [('rb"(?i)(?<!" + _FM + rb")-{1,2}+(?=" + _FM + rb"*?" + _CRED + rb")" + _FM + rb"*+[ \\t]++', 'rb"(?i)(?<!" + _FM + rb")-{1,2}+(?=" + _FM + rb"*?" + _CRED + rb")" + _FM + rb"{0,8}+[ \\t]++')], ["R6-2: NOT redacted", "W7-1: single-dash flag"])
m("R6-bound-flag-lookahead-128", [('-{1,2}+(?=" + _FM + rb"*?" + _CRED', '-{1,2}+(?=" + _FM + rb"{0,128}?" + _CRED')], ["R6-2: NOT redacted", "W7-1: single-dash flag"])
m("R6-bound-json-run-128", [('("json_secret", re.compile(rb"(?i)[\\"\']" + _NAMERUN +', '("json_secret", re.compile(rb"(?i)[\\"\']" + _NM + rb"{0,128}" + _CRED + _NM + rb"{0,128}" +')], ["R6-2: NOT redacted", "W7-5: a spaced JSON value"])   # re-authored (review W7-7): the first form dropped the credential-word requirement
m("R6-jwt-old-quadratic-pattern", [('("jwt", re.compile(_JWT), 1)', '("jwt", re.compile(rb"\\beyJ[A-Za-z0-9_-]{8,}\\.[A-Za-z0-9_-]{8,}\\.[A-Za-z0-9_-]*"), 0)')], ["R6-1: redact_bytes on"])
m("R6-jwt-after-dash-lost", [('rb"*?(?<![A-Za-z0-9_])"', 'rb"*?(?<![A-Za-z0-9_-])"')], ["R6-1: a JWT after"])
m("R6-jwt-pattern-dropped", [('("jwt", re.compile(_JWT), 1)', '("jwt", re.compile(rb"(?!x)x"), 1)')], ["R6-1: a bare JWT"])
m("R6-nonutf8-dir-name-traceback", [('        h.update(os.fsencode(os.path.relpath(p, path)) + b"\\0" + d.encode() + b"\\n")', '        h.update(os.path.relpath(p, path).encode() + b"\\0" + d.encode() + b"\\n")')], ["R6-4: a directory operand holding a non-UTF-8"])
m("R6-nonutf8-target-name-traceback", [('h.update(os.fsencode(os.path.relpath(p, ref)) +', 'h.update(os.path.relpath(p, ref).encode() +')], ["R6-4: a target directory holding a non-UTF-8"])
m("R6-unreadable-file-traceback", [('        raise _OperandSkip("unreadable", "%s cannot be read: %s" % (p, e.strerror or e))', '        raise')], ["R6-4: a directory operand holding a file that cannot be read"])
m("R6-unlistable-dir-ignored", [('os.walk(path, onerror=listing_error)', 'os.walk(path)')], ["R6-4: a directory operand holding an unlistable"])
m("R6-unreadable-refused-on-probe", [('            if not claims_test:\n                continue\n', '            if False:\n                continue\n')], ["R6-4: the same operand on a PROBE", "R6-5: the same directory over the byte cap on a PROBE"])
m("R6-byte-cap-ignored", [('                if total > byte_limit:', '                if False:')], ["R6-5: a directory operand holding more bytes", "R6-5: a directory operand holding a 4 GiB", "W7-2: a 4 GiB mode-000"])
m("R6-dirs-not-counted", [('        entries += len(dirs) + len(fnames)', '        entries += len(fnames)')], ["R6-5: a directory operand with more entries"])
m("WF6-7-cap-off-by-one", [('        if entries > limit:', '        if entries > limit + 1:')], ["R6-5: N+1=4 files"])
m("R6-cap-off-by-one-low", [('        if entries > limit:', '        if entries >= limit:')], ["R6-5: exactly N=3 files"])
m("R6-caps-not-validated-at-start", [('        validate_operand_caps()\n', '')], ["R6-8"])
m("R6-cap-zero-accepted", [('    if re.fullmatch(r"[1-9][0-9]*", raw) is None:', '    if re.fullmatch(r"[0-9][0-9]*", raw) is None:')], ["R6-8: EVREC_OPERAND_DIR_MAX=0", "R6-8: EVREC_OPERAND_DIR_BYTES_MAX=0"])
m("R6-cap-non-integer-is-default", [('        raise Refuse("usage_error", 64, "%s=%r is not a positive integer" % (name, raw))', '        return default')], ["R6-8"])
m("WF6-3-dir-digest-drops-relpath", [('h.update(os.fsencode(os.path.relpath(p, path)) + b"\\0" + d.encode() + b"\\n")', 'h.update(d.encode() + b"\\n")')], ["R6-6: name-swapped directories collide"])   # fragment corrected (review W7-7): it matched only the check's ok text
m("WF6-5-one-level-symlink-only", [('        return bool(f) and _is_interp_name(os.path.basename(os.path.realpath(f)))', '        return bool(f) and _is_interp_name(os.path.basename(os.path.join(os.path.dirname(f), os.readlink(f)) if os.path.islink(f) else f))')], ["R6-6: RED through a TWO-level"])
m("WF6-6-mutation-not-refused-over-cap", [('    claims_test = polarity in ("RED", "GREEN", "MUTATION")', '    claims_test = polarity in ("RED", "GREEN")')], ["R6-6: a MUTATION record over the directory cap"])
m("WF6-4-is-holder-empty-env-holds", [('    me = os.environ.get("EVREC_TURN_RUN_ID") or None\n', '    me = os.environ.get("EVREC_TURN_RUN_ID")\n')], ["R6-6: _is_holder", "F2:"])
m("R6-dead-namemax-back", [('_NM = rb"[A-Za-z0-9_.-]"\n', '_NAMEMAX = 128\n_NM = rb"[A-Za-z0-9_.-]"\n')], ["R6-7"])

# mutants proven behaviour-equivalent (review W7-7): reported EQUIVALENT with the reason, not counted as not-caught
EQUIVALENT = {"F1-json-tail-quadratic": "`*` vs `*+` after a name run that the next token (a quote) cannot match inside: backtracking can never succeed, so the match "
              "set is identical; the run-start look-behind, not the possessive, makes the scan linear (2200 differential cases, 0 differences, linear at 400 KB)"}

# ---- round 7 (WF7-REVIEW findings W7-1 W7-2 W7-3 W7-4 W7-5 W7-6 W7-8 W7-10): one or more paired mutants per fix
m("W7-1-inrun-pattern-dropped", [('("flag_pair_inrun", re.compile(rb"(?i)(?<!"', '("flag_pair_inrun", re.compile(rb"(?i)(?!x)x(?<!"')], ["W7-1: after"])
m("W7-1-inrun-underscore-marker-dropped", [('(?>" + _FM + rb"*?(?:--|_-))"', '(?>" + _FM + rb"*?(?:--))"')], ["W7-1: a_-password V"])
m("W7-1-inrun-lookahead-not-atomic", [('(?=(?>" + _FM + rb"*?(?:--|_-))"', '(?=(?:" + _FM + rb"*?(?:--|_-))"')], ["W7-1: redact_bytes on"])
m("W7-1-inrun-marker-not-required", [('(?=(?>" + _FM + rb"*?(?:--|_-))" + _FM + rb"*?" + _CRED', '(?=" + _FM + rb"*?" + _CRED')], ["W7-1: negative control, not redacted: reset-password"])
m("W7-1-inrun-value-may-start-with-dash", [('[^\\s-][^\\s]{3,})"), 1),\n    ("cookie"', '[^\\s]{4,})"), 1),\n    ("cookie"')], ["W7-1: negative control, not redacted: a credential flag whose value starts"])
m("W7-2-read-time-cap-dropped", [('            if n > budget:\n                raise _OperandSkip("too_large", "%s holds more than %d bytes" % (p, budget))\n            h.update(chunk)', '            h.update(chunk)')], ["W7-2:", "W7-3: a plain procfs operand"])
m("W7-2-read-budget-not-cumulative", [('        d, n = _operand_file_sha_n(p, remaining)', '        d, n = _operand_file_sha_n(p, byte_limit)')], ["W7-2: two size-0 files"])
m("W7-5-st-size-total-per-file", [('                    total += os.stat(p).st_size', '                    total = os.stat(p).st_size')], ["W7-5: three 600-byte files", "W7-5: three mode-000"])
m("W7-3-plain-file-uncapped", [('lines.append(_operand_file_sha(p, _operand_dir_bytes_max()))', 'lines.append(_operand_file_sha(p, 1 << 62))')], ["W7-3:"])
m("W7-3-plain-file-reason-is-directory", [('raise Refuse("operand_file_too_large", 69,', 'raise Refuse("operand_directory_too_large", 69,')], ["W7-3: a 4 GiB plain"])
m("W7-4-operand-files-requires-read-access", [('if cand and (os.path.isfile(cand) or os.path.isdir(cand)):', 'if cand and (os.path.isfile(cand) or os.path.isdir(cand)) and os.access(cand, os.R_OK):')], ["W7-4: a mode-000 plain", "W7-4: an execute-only"])
m("W7-4-dir-pass1-skips-unreadable-files", [('            if os.path.isfile(p):\n                try:', '            if os.path.isfile(p) and os.access(p, os.R_OK):\n                try:')], ["W7-4: a mode-000 file inside"])
m("W7-4-target-listing-error-ignored", [('os.walk(ref, onerror=listing_error)', 'os.walk(ref)')], ["W7-4: a TARGET directory"])
m("W7-8-cap-lenient-int", [('    if re.fullmatch(r"[1-9][0-9]*", raw) is None:', '    if not raw.strip().replace("_", "").lstrip("+").isdigit() or int(raw.replace("_", "")) <= 0:')], ["W7-8:"])
m("W7-8-leading-zeros-and-zero-allowed", [('    if re.fullmatch(r"[1-9][0-9]*", raw) is None:', '    if re.fullmatch(r"[0-9]+", raw) is None:')], ["W7-8:"])
m("W7-5-empty-cap-not-default", [('    if raw is None or raw == "":', '    if raw is None:')], ["W7-5: EVREC_OPERAND_DIR_MAX set to the empty", "W7-5: EVREC_OPERAND_DIR_BYTES_MAX set to the empty"])
m("W7-10-no-nonblock-open", [('os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW', 'os.O_RDONLY | os.O_NOFOLLOW')], ["W7-10:"])
m("W7-10-no-regular-file-check", [('        if not stat.S_ISREG(st.st_mode):', '        if False:')], ["W7-10:"])
m("W7-10-no-realpath-before-open", [('fd = os.open(os.path.realpath(p), ', 'fd = os.open(p, ')], ["W7-10: a symlink to a regular file"])

CORE = ("test_evrec.sh", "test_evrec_more.sh", "test_evrec_r3.sh", "test_evrec_r4.sh", "test_evrec_r5.sh", "test_evrec_r6.sh", "test_evrec_r7.sh", "test_evrec_golden.sh")


def run_one(mut, scratch_base, check_only=False):
    mid, edits, expect, tests = mut
    d = tempfile.mkdtemp(prefix="mut-", dir=scratch_base)
    try:
        for sub in ("tools/evidence/tests", FEATURE + "/contracts", "scripts/repo"):
            os.makedirs(os.path.join(d, sub))
        for f in ("evrec", "verify", "evcore.py"):
            shutil.copy2(os.path.join(ROOT, "tools/evidence", f), os.path.join(d, "tools/evidence", f))
        for f in os.listdir(HERE):
            if f.endswith((".sh", ".py")) and f not in ("run_mutations.py", "capture_evidence.sh"):
                shutil.copy2(os.path.join(HERE, f), os.path.join(d, "tools/evidence/tests", f))
        shutil.copy2(os.path.join(ROOT, FEATURE, "contracts/evidence-record.schema.json"), os.path.join(d, FEATURE, "contracts"))
        shutil.copy2(os.path.join(ROOT, "scripts/repo/check_classes.tsv"), os.path.join(d, "scripts/repo"))
        for e in edits:
            rel, old, new = e if len(e) == 3 else ("tools/evidence/evcore.py", e[0], e[1])
            p = os.path.join(d, rel)
            src = open(p).read()
            if src.count(old) != 1:
                return mid, "INVALID", "edit text found %d times: %r" % (src.count(old), old[:60]), []
            mutated = src.replace(old, new)
            if rel.endswith(".py"):
                try:
                    compile(mutated, rel, "exec")        # a mutant that does not compile is INVALID, never "caught" (review R6-11)
                except SyntaxError as err:
                    return mid, "INVALID", "mutated source does not compile: %s (line %s)" % (err.msg, err.lineno), []
            open(p, "w").write(mutated)
        # a file that compiles can still fail when IMPORTED (a regex with a misplaced global flag raises at import): that is INVALID,
        # never "caught by crashing every call" (review W7-6)
        if any((e[0] if len(e) == 3 else "tools/evidence/evcore.py").endswith(".py") for e in edits):
            imp = subprocess.run([sys.executable, "-B", "-I", "-c", "import sys; sys.path.insert(0, sys.argv[1]); import evcore",
                                  os.path.join(d, "tools/evidence")], capture_output=True, text=True, timeout=120)
            if imp.returncode != 0:
                return mid, "INVALID", "mutated source compiles but cannot be imported: %s" % (imp.stderr.strip().splitlines() or ["?"])[-1][:120], []
        if check_only:
            return mid, "CAUGHT", "", []
        fails = []
        for t in (tests or CORE):
            try:
                r = subprocess.run(["bash", os.path.join(d, "tools/evidence/tests", t)], capture_output=True, text=True, timeout=600)
                out = r.stdout + r.stderr
            except subprocess.TimeoutExpired as e:
                out = "FAIL timeout in %s\n" % t
            fails += [l for l in out.splitlines() if l.startswith("FAIL ")]
        caught = [l for l in fails if any(x in l for x in expect)]
        if caught:
            return mid, "CAUGHT", "", fails
        return mid, ("SURVIVED" if not fails else "CAUGHT-OTHER-CHECKS-ONLY"), "", fails
    finally:
        shutil.rmtree(d, ignore_errors=True)


def main():
    a = sys.argv[1:]
    jobs, check = 3, False
    while a[:1] in (["-j"], ["--check"]):
        if a[0] == "-j":
            jobs, a = int(a[1]), a[2:]
        else:
            check, a = True, a[1:]
    sel = [x for x in M if not a or x[0] in a]
    base = os.environ.get("TMPDIR") or "/tmp"
    res = []
    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for r in ex.map(lambda x: run_one(x, base, check), sel):
            res.append(r)
            mid, st, note, fails = r
            print("%-34s %-26s %s" % (mid, st, ("; ".join(f[5:70] for f in fails[:3]) or note)), flush=True)
    equiv = [r for r in res if r[1] == "SURVIVED" and r[0] in EQUIVALENT]
    for r in equiv:
        print("EQUIVALENT %-30s %s" % (r[0], EQUIVALENT[r[0]]))
    bad = [r for r in res if r[1] != "CAUGHT" and r not in equiv]
    print("mutants=%d caught=%d equivalent=%d not_caught=%d" % (len(res), len(res) - len(bad) - len(equiv), len(equiv), len(bad)))
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())
