#!/usr/bin/env python3
"""T050/T056 paired mutations for tools/evidence/evcore.py.

Each mutant is a literal source edit of evcore.py applied to a scratch copy of the tool tree; both test files
(test_evrec.sh, test_evrec_more.sh) are run against it. A mutant is CAUSE-CAUGHT only when at least one of the
`expect` check-name fragments appears on a FAIL line (a mutant that crashes the whole suite for an unrelated reason
does not count). Mutants that survive are reported, never hidden. Usage: run_mutations.py [-j N] [ID...]
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


def m(i, edits, expect):
    M.append((i, edits, expect))


m("V1-skip-prev-link-check", [('if rec["prev_hash"] != prev:', 'if False:')], ["link only"])
m("V2-skip-entry-hash-check", [('if compute_entry_hash(rec) != rec["entry_hash"]:', 'if False:')], ["edited content"])
m("V3-skip-seq-contiguity", [('if rec["seq"] != n:', 'if False:')], ["non-contiguous seq", "re-sequenced", "kept local seq"])
m("V4-verify-never-walks", [('n, head = chain_walk(led)', 'n, head = 0, ZERO')],
  ["entry 2 deleted without repair", "edited content", "reordered", "torn final line"])
m("V5-verify-always-fails", [('n, head = chain_walk(led)', 'raise Refuse("chain_failure", 1, "x")')],
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
m("R3-no-schema-validation-on-append", [('        errs = schema_errors(rec)\n        if errs:\n            raise Refuse("record_invalid", 65, "; ".join(errs))\n        atomic_write(led',
                                          '        atomic_write(led')], ["RED without oracle"])
m("R4-check-record-accepts-all", [('    errs = schema_errors(probe)\n    if errs:\n        raise Refuse("record_invalid", 65, "; ".join(errs))\n    print("record valid")',
                                    '    print("record valid")')], ["check-record refuses"])
m("R5-no-ledger-lock", [('    lock.acquire()\n    try:', '    try:')], ["concurrent appenders"])
m("R6-reap-live-holder", [('    pid, cmd = h.get("pid"), h.get("cmdline")', '    return True\n    pid, cmd = h.get("pid"), h.get("cmdline")')],
  ["live lock holder", "live holder process untouched", "ledger unchanged after refused"])
m("R7-never-reap", [('    pid, cmd = h.get("pid"), h.get("cmdline")', '    return False\n    pid, cmd = h.get("pid"), h.get("cmdline")')],
  ["stale ledger lock with dead holder"])
m("R8-write-in-place-not-temp-rename",
  [('        atomic_write(led, raw + canon(rec) + b"\\n", fault=True)',
    '        with open(led, "ab") as _f:\n            _f.write(canon(rec) + b"\\n")\n        if os.environ.get("EVREC_FAULT"):\n            os.kill(os.getpid(), signal.SIGKILL)')],
  ["changed by a killed write"])
m("R9-no-redaction", [('    reds = redactions()', '    reds = []')], ["planted secret present", "leaked via argv"])
m("R10-redacted-flag-dropped", [('    if redacted:\n        rec["redacted"] = True', '    if False:\n        rec["redacted"] = True')],
  ["lacks redacted"])
m("R11-argv-not-redacted", [('        redacted |= h\n        r_argv.append(b.decode("utf-8", "replace") if h else a)', '        r_argv.append(a)')],
  ["leaked via argv"])
m("R12-reconcile-always-ok", [('    if len(lines) != int(args[1]):', '    if False:')], ["runner count 4 != 3"])
m("R13-never-raise-owed", [('if sz * RAISE_DEN >= bound * RAISE_NUM:', 'if False:')], ["silent above 75%"])
m("R14-always-raise-owed", [('if sz * RAISE_DEN >= bound * RAISE_NUM:', 'if True:')], ["names ledger_raise_owed below"])
m("R15-exit-status-unmapped", [('    if 1 <= rc <= 125:\n        return "fail"\n    return "error"', '    return "fail"')],
  ["exit-status mapping"])
m("R16-signal-not-128-plus", [('            rc = 128 + (-rc)', '            rc = 1')], ["exit-status mapping"])
m("R17-test-fingerprint-dropped", [('    if tfp:\n        rec["test_fingerprint"] = tfp', '    if False:\n        rec["test_fingerprint"] = tfp')],
  ["valid RED entry not recorded"])
m("R18-target-fp-invented", [('    return sha_bytes(b"UNRESOLVED:" + ref.encode()), False', '    return sha_bytes(b"UNRESOLVED:" + ref.encode()), True')],
  ["unresolvable RED target accepted"])
m("R19-target-fp-not-file-bytes", [('            return sha_bytes(f.read()), True\n    if os.path.isdir', '            return sha_bytes(ref.encode()), True\n    if os.path.isdir')],
  ["target_fingerprint is not"])
m("R20-append-onto-corrupt-tail", [('            if not ok:\n                raise Refuse("ledger_inconsistent"', '            if False:\n                raise Refuse("ledger_inconsistent"')],
  ["append extended a corrupt"])
m("R21-stream-digest-no-blob", [('    osha, esha = store_blob(out), store_blob(err)', '    osha, esha = sha_bytes(out), sha_bytes(err)')],
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
  [('    commit_turn_check(allow_reap=True)                  # before anything is written (or run)', '    pass'),
   ('        commit_turn_check(allow_reap=False)', '        pass')],
  ["no run id", "different run id"])
m("C2-unreadable-grant-ignored", [('    if err:\n        raise Refuse("commit_turn_held", 76, err)\n    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    if allow_reap:',
                                    '    if err:\n        return\n    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    if allow_reap:')],
  ["mode 000 grant not refused", "non-JSON grant not refused"])
m("C3-matching-run-id-refused", [('    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    if allow_reap:',
                                  '    if False:\n        return\n    if allow_reap:')], ["matching run id", "stub called for matching run id"])
m("C4-matching-run-id-calls-reaper", [('    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    if allow_reap:',
                                      '    if False:\n        return\n    if allow_reap:'),
                                     ('    raise Refuse("commit_turn_held", 76, "grant held by run_id %s" % grant["run_id"])',
                                      '    if grant["run_id"] == os.environ.get("EVREC_TURN_RUN_ID"):\n        return\n    raise Refuse("commit_turn_held", 76, "grant held by run_id %s" % grant["run_id"])')],
  ["stub called for matching run id"])
m("C5-reaper-never-called", [('            subprocess.run([entry,', '            (lambda *a, **k: None)([entry,')], ["--reap call count", "dead-holder"])
m("C6-reap-direct-not-exec-approved",
  [('[entry, "--exec-approved", "scripts/release/commit_turn_check.sh", "--reap"]', '["bash", os.path.join(repo_root(), "scripts/release/commit_turn_check.sh"), "--reap"]')],
  ["no --exec-approved entry"])
m("C7-reap-by-grant-age", [('    raise Refuse("commit_turn_held", 76, "grant held by run_id %s" % grant["run_id"])', '    return')],
  ["no run id", "live old grant"])
m("C8-host-refusal-crash-allowed", [('        except (OSError, subprocess.SubprocessError):\n            pass\n        grant, err = _read_grant()\n        if grant is None and err is None:\n            return',
                                     '        except (OSError, subprocess.SubprocessError):\n            pass\n        return')], ["host refusing", "stores changed under a held grant"])
m("C9-blobs-written-before-check",
  [('    commit_turn_check(allow_reap=True)                  # before anything is written (or run)', '    os.makedirs(blobs_path(), exist_ok=True); open(os.path.join(blobs_path(), "pre"), "w").close()')],
  ["stores changed under a held grant"])
m("C10-reaper-called-repeatedly", [('    raise Refuse("commit_turn_held", 76, "grant held by run_id %s" % grant["run_id"])', '    return commit_turn_check(allow_reap=True)')],
  ["stub call count", "--reap call count", "no run id"])


def run_one(mut, scratch_base):
    mid, edits, expect = mut
    d = tempfile.mkdtemp(prefix="mut-", dir=scratch_base)
    try:
        for sub in ("tools/evidence/tests", FEATURE + "/contracts", "scripts/repo"):
            os.makedirs(os.path.join(d, sub))
        for f in ("evrec", "verify", "evcore.py"):
            shutil.copy2(os.path.join(ROOT, "tools/evidence", f), os.path.join(d, "tools/evidence", f))
        for f in ("test_evrec.sh", "test_evrec_more.sh"):
            shutil.copy2(os.path.join(HERE, f), os.path.join(d, "tools/evidence/tests", f))
        shutil.copy2(os.path.join(ROOT, FEATURE, "contracts/evidence-record.schema.json"), os.path.join(d, FEATURE, "contracts"))
        shutil.copy2(os.path.join(ROOT, "scripts/repo/check_classes.tsv"), os.path.join(d, "scripts/repo"))
        p = os.path.join(d, "tools/evidence/evcore.py")
        src = open(p).read()
        for old, new in edits:
            if src.count(old) != 1:
                return mid, "INVALID", "edit text found %d times: %r" % (src.count(old), old[:60]), []
            src = src.replace(old, new)
        open(p, "w").write(src)
        fails = []
        for t in ("test_evrec.sh", "test_evrec_more.sh"):
            try:
                r = subprocess.run(["bash", os.path.join(d, "tools/evidence/tests", t)], capture_output=True, text=True, timeout=240)
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
    jobs = 3
    if a[:1] == ["-j"]:
        jobs, a = int(a[1]), a[2:]
    sel = [x for x in M if not a or x[0] in a]
    base = os.environ.get("TMPDIR") or "/tmp"
    res = []
    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for r in ex.map(lambda x: run_one(x, base), sel):
            res.append(r)
            mid, st, note, fails = r
            print("%-34s %-26s %s" % (mid, st, ("; ".join(f[5:70] for f in fails[:3]) or note)), flush=True)
    bad = [r for r in res if r[1] != "CAUGHT"]
    print("mutants=%d caught=%d not_caught=%d" % (len(res), len(res) - len(bad), len(bad)))
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())
