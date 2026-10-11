#!/usr/bin/env python3
"""T056 / T051..T055 paired mutations for the WP-05 evidence tooling written after the recorder: the verdict deriver, the anchors, the run
token, the report parsers and runner wrappers, the shared ab_pass_with_evidence definition and the TS-02 census.

Each mutant is a literal source edit (FILE, OLD, NEW) applied to a scratch copy of the tool tree (tools/evidence, the ev/1 schema,
scripts/repo/check_classes.tsv and the migrated suites); the named test files are then run against the copy. A mutant is CAUSE-CAUGHT only
when at least one `expect` fragment appears on a FAIL line of a test file (a mutant that breaks the suite for an unrelated reason does not
count); an edit that does not apply (OLD absent) or whose Python does not compile is INVALID, never "caught". Survivors are reported, never
hidden. The mutants that T056 names are tagged T056 in their id.

Usage: run_mutations_wp05b.py [--check] [-j N] [--list] [--baseline [TEST.sh ...]] [ID-SUBSTRING ...]

A run starts with the unmutated-baseline control (baseline()): if any suite FAILs without a mutation, the run stops with exit 2.
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
F = "specs/001-full-project-audit-remediation"
M = []   # (id, [(file, old, new)], [expect fragments], [tests])


def m(i, edits, expect, tests):
    M.append((i, edits, expect, tests))


EV, PA, VD, AN, CORE, LIB, CEN = "tools/evidence/evcore.py", "tools/evidence/evparse.py", "tools/evidence/evverdict.py", "tools/evidence/evanchor.py", \
    "tools/evidence/evcore.py", "tools/evidence/lib/ab_pass_with_evidence.sh", "tools/evidence/census_ab_pass.py"
TA, TV, TE, T2, TR, T9 = "test_anchors.sh", "test_verdict.sh", "test_evrec.sh", "test_ts02.sh", "test_anchors_r8.sh", "test_anchors_r9.sh"

# ---- anchors (T055 / T056: "remove the anchor comparison, the tamper test must fail")
m("T056-A1-anchor-comparison-removed-in-verify", [(CORE, "        rows, newest = evanchor.verify_anchors(a, entries)", "        rows, newest = None, None")],
  ["delete-and-recompute is caught", "truncated tail is caught", "caught by the anchor"], [TA])
m("T056-A2-head-not-compared", [(AN, '        if entries[c - 1]["entry_hash"] != r["head"]:', "        if False:")],
  ["delete-and-recompute is caught", "recomputed", "caught by the anchor", "rewritten history contradicts"], [TA])
m("T056-A3-count-not-compared", [(AN, "        if c > n:", "        if False:")], ["truncated tail is caught"], [TA])
m("A4-only-newest-row-compared", [(AN, "    for r in reversed(rows):", "    for r in rows[-1:]:")], ["old anchor row"], [TA])
m("A6-anchor-writer-skips-chain-check", [(AN, "        entries, head = evcore.chain_walk(led)  ", "        entries, head = (lambda p: (None, None))(led)  ")],
  ["refuses to anchor a ledger whose chain fails", "anchor row"], [TA])
m("A7-retired-scratch-flag-accepted", [(AN, '        else:\n            raise Refuse("usage_error", 64, usage)', '        elif a == "--probe-scratch-ok":\n            i += 1\n        else:\n            raise Refuse("usage_error", 64, usage)')],
  ["probe(retired flag)"], [TA])
m("A8-network-remote-probed", [(AN, '        if re.match(r"^[A-Za-z][A-Za-z0-9+.-]*://", remote) or re.match(r"^[^/]+@[^/]+:", remote):', "        if False:")],
  ["probe(url)"], [TA])
m("A9-rerecord-keeps-local-anchors", [(AN, "    replaced = [r for r in local_rows if r[\"count\"] > cp]", "    replaced = []")],
  ["ONE anchor over the re-chained head", "wrong anchors", "verify accepts the output"], [TA])
m("A10-mechanism-claim-not-refused", [(AN, '    if want == "mechanism":', "    if False:")], ["mechanism claimed without evidence", "R3: mechanism claim"], [TA, TR])
m("A11-denying-remote-gives-mechanism", [(AN, '    strength = "policy"  ', '    strength = "mechanism" if (probe and probe["rewrite_rejected"] is True) else "policy"  ')],
  ["probe(deny)", "R3: strength="], [TA, TR])
m("A12-existing-anchor-history-not-checked-when-adding", [(AN, "            verify_anchors(apath, entries)  ", "            None  ")], ["re-anchored a forged chain"], [TA])

# ---- 11.4.113: the probe reads configuration and pushes nothing
m("A14-one-deny-key-suffices", [(AN, '        rejected = all(c["effective"] is True for c in cfg.values())', '        rejected = any(c["effective"] is True for c in cfg.values())')],
  ["probe(onlynff)", "probe(onlydel)", "half-configured remote"], [TA])
m("A15-executable-hook-counts-as-proof", [(AN, '        rec["rewrite_rejected"] = rejected\n', '        rejected = rejected or hooks["pre-receive"] == "executable"\n        rec["rewrite_rejected"] = rejected\n')],
  ["probe(hook)"], [TA])
m("A16-global-config-ignored", [(AN, '    env["GIT_TERMINAL_PROMPT"] = "0"\n', '    env["GIT_TERMINAL_PROMPT"] = "0"\n    env["GIT_CONFIG_GLOBAL"] = "/dev/null"\n')],
  ["probe(global)"], [TA])
_PU, _FO = "pu" + "sh", "-" + "-for" + "ce"          # pieces: this runner must not itself contain a push, a forced push or a plus-ref (11.4.113 scan in test_anchors.sh)
_DEAD = '    if False:\n        _git(["%s", %s, remote, "HEAD:refs/heads/x"], remote, {})\n    env = dict(os.environ)\n    env["GIT_TERMINAL_PROMPT"] = "0"'
_ANCH = '    env = dict(os.environ)\n    env["GIT_TERMINAL_PROMPT"] = "0"'
m("A17-forced-push-readded", [(AN, _ANCH, _DEAD % (_PU, '"%s"' % _FO))], ["no-force-push scan"], [TA])           # dead code: never executed, the scan is textual
m("A18-plus-ref-push-readded", [(AN, _ANCH, _DEAD % (_PU, '"+" + "HEAD"'))], ["no-force-push scan"], [TA])
m("A18b-plain-push-built-in-code", [(AN, _ANCH, _DEAD % (_PU, '"origin"'))], ["no-force-push scan"], [TA])

# ---- fix round 8 (independent review WP-05: R1 to R5 and the five reviewer mutants REV-1, REV-3 to REV-6 that survived the first suites)
m("R8-A20-REV1-anchor-rows-may-go-backward", [(AN, '        if r["count"] < prev:', "        if False:")], ["REV-1:"], [TR])
m("R8-A21-REV3-rerecord-drops-anchor-at-common-prefix", [(AN, '    kept = [r for r in local_rows if rmax < r["count"] <= cp]', '    kept = [r for r in local_rows if rmax < r["count"] < cp]')], ["REV-3:"], [TR])
m("R8-A22-REV5-anchor-writer-takes-no-lock", [(AN, "    lock.acquire()\n    try:\n        entries, head = evcore.chain_walk(led)", "    try:\n        entries, head = evcore.chain_walk(led)")], ["REV-5:"], [TR])
m("R8-A23-REV6-count-zero-row-accepted", [(AN, 'and r["count"] >= 1 and', 'and r["count"] >= 0 and')], ["REV-6:"], [TR])
m("R8-D12-REV4-same-test-ignores-cwd", [(VD, 'for k in ("cwd", "target_class", "target_ref", "test_fingerprint"))\n                      and all(', 'for k in ("target_class", "target_ref", "test_fingerprint"))\n                      and all(')], ["REV-4:"], [TR])
m("R8-A24-R1-verify-absent-anchor-passes", [(CORE, "    if newest is None and not chain_only:", "    if False:")], ["R1:", "NO anchor to compare is UNVERIFIED"], [TR, TA])
m("R8-A25-R1-chain-only-switches-off-a-present-anchor", [(CORE, "    if os.path.exists(a) and (os.path.isdir(a) or os.path.getsize(a) > 0):\n        import evanchor", "    if not chain_only and os.path.exists(a) and (os.path.isdir(a) or os.path.getsize(a) > 0):\n        import evanchor")],
  ["R1: --chain-only does NOT switch off", "R5: --chain-only on a forged"], [TR])
m("R8-A26-R4-verify-trusts-a-mechanism-row", [(AN, '        if r["strength"] == "mechanism":', "        if False:")], ["R4:"], [TR])
m("R8-A27-R2-end-session-is-a-noop", [(CORE, "    return cmd_anchor([])\n\n\ndef cmd_anchor(args):", "    return 0\n\n\ndef cmd_anchor(args):")], ["R2:"], [TR])
m("R8-A28-R2-anchor-after-ignored", [(CORE, '    if opts.get("anchor_after"):', "    if False:")], ["R2: run --anchor-after", "R2: second --anchor-after"], [TR])
m("R8-D13-R5-deriver-skips-the-anchor-gate", [(VD, "        arows, newest = evcore.anchor_gate(led, default, entries, chain_only)", "        arows, newest = None, None")], ["R5:"], [TR])
m("R8-D14-R5-deriver-chain-only-is-default", [(VD, "evcore.anchor_gate(led, default, entries, chain_only)", "evcore.anchor_gate(led, default, entries, True)")], ["R5: unanchored ledger"], [TR])
m("R8-A29-R3-probe-claims-no-reconfigure-risk", [(AN, 'rec["producer_can_reconfigure"] = bool(os.access(remote, os.W_OK) or os.access(os.path.join(remote, "config"), os.W_OK))', 'rec["producer_can_reconfigure"] = False')], ["R3: probe record lacks the honest facts"], [TR])

# ---- fix round 9 (independent re-review WP-05-r2: F1 blocking RR2-4, F2, F3, F5)
m("R9-RR2-4-anchor-after-swallows-the-refusal", [(CORE, '            raise Refuse(e.reason, e.code, "%s (the entry above WAS recorded; the anchor was NOT written)" % e.detail)', "            return 0")], ["F1/RR2-4"], [T9])
m("R9-F2-chain-only-pass-stays-pass", [(VD, '    if newest is None and out["verdict"] == "PASS":', "    if False:")], ["F2: exit"], [T9])
m("R9-F2-chain-only-unverified-exit-0", [(VD, '(4 if out["verdict"] == "CHAIN-ONLY-UNVERIFIED" else 1)', '(0 if out["verdict"] == "CHAIN-ONLY-UNVERIFIED" else 1)')], ["F2: exit"], [T9])
m("R9-F3-run-skips-the-anchor-precheck", [(CORE, "        evanchor.precheck(ledger_path(), ap)", "        pass")], ["F3: run onto", "F3 truncation", "F3: an unreadable"], [T9])
m("R9-F3-precheck-compares-no-head", [(AN, "    if h != r[\"head\"]:\n        raise Refuse(\"anchor_disagrees\", 2, \"anchored count=%d head=%s but entry %d", "    if False:\n        raise Refuse(\"anchor_disagrees\", 2, \"anchored count=%d head=%s but entry %d")], ["F3 head"], [T9])
m("R9-F3-precheck-compares-no-count", [(AN, "    if c > n:\n        raise Refuse(\"anchor_disagrees\", 2, \"anchored count=%d head=%s but the ledger has only", "    if False:\n        raise Refuse(\"anchor_disagrees\", 2, \"anchored count=%d head=%s but the ledger has only")], ["F3 truncation"], [T9])
m("R9-F3-precheck-blocks-a-lagging-anchor", [(AN, "    if c > n:\n        raise Refuse(\"anchor_disagrees\", 2, \"anchored count=%d head=%s but the ledger has only", "    if c != n:\n        raise Refuse(\"anchor_disagrees\", 2, \"anchored count=%d head=%s but the ledger has only")], ["F3 control: lagging anchor blocked"], [T9])

# ---- verdict deriver (T054 / T056: "make the deriver accept a GREEN without a prior RED")
m("T056-D1-green-without-prior-red", [(VD, '    o["red_before_green"] = len(red) >= 1 and len(grn) >= 1 and max(e["seq"] for e in red) < min(e["seq"] for e in grn)',
                                      '    o["red_before_green"] = True')], ["case green_first", "case launder", "case reopen_pass", "case reopen_after_fail"], [TV])
m("D1b-green-without-any-red", [(VD, '    o["red_ok"] = len(red) >= 1 and all(genuine(e) for e in red)', '    o["red_ok"] = all(genuine(e) for e in red)'),
                                (VD, '    o["red_before_green"] = len(red) >= 1 and len(grn) >= 1 and max(e["seq"] for e in red) < min(e["seq"] for e in grn)',
                                 '    o["red_before_green"] = len(grn) >= 1'),
                                (VD, '    o["fingerprints_differ"] = len([f for f in rf if f not in gf]) == len(rf) and len(red) >= 1 and len(grn) >= 1',
                                 '    o["fingerprints_differ"] = len([f for f in rf if f not in gf]) == len(rf) and len(grn) >= 1')],
  ["case reopen_no_new"], [TV])
m("D2-red-accepts-any-nonzero", [(VD, "    return isinstance(x, int) and 1 <= x <= 125 and e.get(\"verdict\") == \"fail\"", "    return isinstance(x, int) and x != 0")],
  ["case exit127", "case exit126", "case signal"], [TV])
m("D3-same-test-ignores-test-fingerprint", [(VD, 'for k in ("cwd", "target_class", "target_ref", "test_fingerprint"))\n                      and all(', 'for k in ("cwd", "target_class", "target_ref"))\n                      and all('),
                                           (VD, 'e.get("test_fingerprint") is not None\n                              for e in both))', 'True\n                              for e in both))')],
  ["case typo_red", "case exit127"], [TV])
m("D4-green-iterations-not-distinct", [(VD, "len(grn) >= 3 and len(its) >= 3 and", "len(grn) >= 3 and")], ["case green_dup_iter"], [TV])
m("D5-fingerprints-new-dropped", [(VD, '    o["fingerprints_new"] = len([f for f in gf if f not in banned]) == len(grn)', '    o["fingerprints_new"] = True')],
  ["case green_on_reopen_fp", "case green_old_fp"], [TV])
m("D6-reopen-cuts-without-genuine-failure", [(VD, "if ck[\"pass\"] and genuine(o) and ref is not None", "if ck[\"pass\"] and ref is not None")], ["case reopen_pass", "case launder"], [TV])
m("D7-reopen-cuts-an-incomplete-cycle", [(VD, "        if ck[\"pass\"] and genuine(o)", "        if genuine(o)")], ["case reopen_after_fail", "case launder", "case reopen_pass"], [TV])
m("D8-same-test-compares-argv-with-token", [(VD, "def norm_argv(argv):\n    if not isinstance(argv, list):\n        return argv", "def norm_argv(argv):\n    return argv\n    if not isinstance(argv, list):\n        return argv")],
  ["T051a golden-good"], [TE])
m("D9-green-identical-unmasked", [(VD, "    t = argv_token(e.get(\"argv\"))\n    if t and bdir and isinstance(e.get(\"stdout_sha256\"), str):", "    t = None\n    if t and bdir and isinstance(e.get(\"stdout_sha256\"), str):")],
  ["T051a golden-good"], [TE])
m("D10-strict-mode-skips-schema", [(VD, "evcore.chain_walk(led, deep=not chain_only)", "evcore.chain_walk(led, deep=False)")], ["strict mode on a forged ledger"], [TV])
m("D11-deriver-trusts-a-broken-chain", [(VD, "    except evcore.Refuse as r:\n        sys.stderr.write(\"verdict: refused", "    except ZeroDivisionError as r:\n        sys.stderr.write(\"verdict: refused")],
  ["deleted entry", "strict mode refuses"], [TV])

# ---- run token (T051a / T056: "a deriver copy that skips the token comparison accepts the golden-bad fixture")
m("T056-K1-deriver-skips-token-comparison", [(VD, "    return (not notes), notes", "    return True, []")], ["T051a golden-bad"], [TE])
m("K2-token-rule-only-with-flag", [(VD, "    if not (state_delta or any(argv_token(e.get(\"argv\")) for e in red + grn)):", "    if not state_delta:")],
  ["rule not applied by token presence", "T051a negative control"], [TE])
m("K3-token-compared-against-any-blob-not-own", [(VD, "        elif not _blob_has(bdir, e.get(\"stdout_sha256\"), t):", "        elif False:")], ["T051a golden-bad"], [TE])
m("K4-token-shared-between-entries-allowed", [(VD, "        elif count.get(t, 0) > 1:", "        elif False:")], ["T051a: shared token accepted by the deriver"], [TE])
m("K5-verify-ignores-reused-token", [(CORE, "    reused = {t: s for t, s in toks.items() if len(s) > 1}", "    reused = {}")], ["verify on a reused token"], [TE])
m("K6-argv-token-redacted", [(CORE, "            if prev == \"--run-token\" and _TOKEN32.fullmatch(a):", "            if False:")], ["token missing or redacted in the recorded argv"], [TE])
m("K7-stream-token-redacted", [(CORE, "    if tok and _TOKPH not in b and tok.encode() in b:", "    if False:")], ["token not in the stdout blob"], [TE])
m("K8-token-not-random", [(CORE, "    print(os.urandom(16).hex())", "    print('0123456789abcdef0123456789abcdef')")], ["tokens equal or absent"], [TE])
m("K9-token-short", [(CORE, "    print(os.urandom(16).hex())", "    print(os.urandom(4).hex())")], ["evrec token printed"], [TE])

# ---- parsers (T051 / T053)
m("P-go1-test-level-failures-ignored", [(PA, '        else:\n            r.fail_name("%s %s" % (pkg, name))\n    for pkg, name in textfail', '        else:\n            pass\n    for pkg, name in textfail')],
  ["seeded failure is reported", "hidden failure behind a CUT"], ["test_wrap_go.sh"])
m("P-go2-truncation-not-detected", [(PA, "    if incomplete or unfinished_pkgs or truncated:\n        r.notes.append", "    if False:\n        r.notes.append")], ["started and never finished"], ["test_wrap_go.sh"])
m("P-go3-zero-tests-pass", [(PA, "        elif r.passed + r.failed == 0:\n            r.status, r.reason = \"error\", \"no_tests\"", "        elif False:\n            pass")],
  ["without test files ran zero tests", "zero tests", "no tests"], ["test_wrap_go.sh", "test_wrap_bash.sh", "test_wrap_vitest.sh", "test_wrap_gradle.sh", "test_wrap_cargo.sh"])
m("P-go4-build-failure-is-a-test-failure", [(PA, '        return r.error("build_failed")\n    if any(a == "fail" for a in pkg_term.values()):', '        return r.fail("build_failed")\n    if any(a == "fail" for a in pkg_term.values()):')],
  ["does not compile"], ["test_wrap_go.sh"])
m("P-bash1-summary-beats-fail-lines", [(PA, "        if _FAIL.match(line):\n            r.fail_name(line[:160])", "        if False:\n            pass")], ["liar", "seeded failure", "FAIL line wins"], ["test_wrap_bash.sh"])
m("P-bash2-exit-status-ignored", [(PA, "    if r.status is None and rc is not None and rc != 0 and r.passed + r.skipped >= 0:", "    if False:"),
                                  (PA, "        elif rc is not None and rc != 0:\n            r.status, r.reason = \"fail\", \"exit_without_fail_line\"", "        elif False:\n            pass")],
  ["death under set -e", "silent death"], ["test_wrap_bash.sh"])
m("P-bash3-lost-line-ignored", [(PA, "            if a != r.passed:", "            if False:")], ["lost line"], ["test_wrap_bash.sh"])
m("P-vt1-header-trusted-over-assertions", [(PA, "                elif st == \"failed\":\n                    r.fail_name(\"%s :: %s\"", "                elif st == \"failed\" and False:\n                    r.fail_name(\"%s :: %s\"")], ["recorded seeded failure", "assertion wins"], ["test_wrap_vitest.sh"])
m("P-vt2-truncated-report-passes", [(PA, "    if not stat:\n        return r.error(\"unparsable\")", "    if not stat:\n        return r.error(\"unparsable\")\n    stat = [x for x in stat if x[1] != \"failed\"] or stat")], ["TRUNCATED report"], ["test_wrap_vitest.sh"])
m("P-vt3-header-failures-ignored", [(PA, "        if isinstance(hdr, int) and hdr > r.failed:", "        if False:"),
                                    (PA, "        if d.get(\"success\") is False and r.failed == 0 and r.status is None:", "        if False:")], ["header that counts failures"], ["test_wrap_vitest.sh"])
m("P-gr1-header-counts-trusted", [(PA, "                    if tc.find(\"failure\") is not None or tc.find(\"error\") is not None:", "                    if False:")], ["seeded failure across", "testcase wins", "crashes"], ["test_wrap_gradle.sh"])
m("P-gr2-truncated-xml-passes", [(PA, "                if \"<failure\" in head or \"<error\" in head:\n                    r.fail_name(nm)", "                if False:\n                    r.fail_name(nm)")], ["truncated XML"], ["test_wrap_gradle.sh"])
m("P-cg1-last-summary-wins", [(PA, "        if _CT.match(line) and False:\n", "")] if False else [(PA, "            else:\n                r.fail_name(nm)\n            continue", "            else:\n                pass\n            continue")],
  ["seeded failure", "FAILED line before a later ok"], ["test_wrap_cargo.sh"])
m("P-cg2-build-error-is-pass-or-fail", [(PA, "    if build and not r.passed:\n        return r.error(\"build_failed\")", "    if build and not r.passed:\n        return r.fail(\"build_failed\")")], ["compile error"], ["test_wrap_cargo.sh"])
m("W1-token-not-exported-bash", [("tools/evidence/wrap-bash.sh", "env EVREC_RUN_TOKEN=\"$W_TOKEN\" ", "env ")], ["EVREC_RUN_TOKEN is exported"], ["test_wrap_bash.sh"])
m("W2-token-not-required", [("tools/evidence/lib/wrap_common.sh", "    \"\") wrap_refuse run_token_missing", "    \"@\") wrap_refuse run_token_missing")], ["no --run-token is refused"], ["test_wrap_bash.sh", "test_wrap_go.sh"])
m("W3-token-shape-not-checked", [("tools/evidence/lib/wrap_common.sh", "[[ $W_TOKEN =~ ^[0-9a-f]{32}$ ]] || wrap_refuse run_token_malformed", "true || wrap_refuse run_token_malformed")], ["malformed --run-token"], ["test_wrap_bash.sh"])
m("W4-wrapper-prints-the-token", [("tools/evidence/lib/wrap_common.sh", "  python3 -I \"$WRAP_HERE/evparse.py\"", "  echo \"wrap: token=$W_TOKEN\"; python3 -I \"$WRAP_HERE/evparse.py\"")], ["wrapper itself printed the token", "printed the run token"], ["test_wrap_bash.sh"])
m("W5-missing-runner-is-a-test-failure", [("tools/evidence/lib/wrap_common.sh", "wrap_refuse runner_not_executable \"$c is not in PATH\"", "{ echo bad >&2; exit 1; }"),
                                          ("tools/evidence/lib/wrap_common.sh", "wrap_refuse runner_not_executable \"$c is not an executable file\"", "{ echo bad >&2; exit 1; }")], ["does not exist is an error"], ["test_wrap_bash.sh"])

# ---- TS-02 (T052)
m("T52-L1-library-prints-pass-without-the-recorder", [(LIB, "  if ab_rec_out=$(\"${AB_EVREC}\" run", "  if true || ab_rec_out=$(\"${AB_EVREC}\" run")], ["golden-good: exactly one ev/1 entry", "recorder unavailable", "recorder refusing"], [T2])
m("T52-L2-library-never-calls-the-recorder", [(LIB, '  if ab_rec_out=$("${AB_EVREC}" run "${ab_item}" PROBE "${AB_ITER}" remote_service "${ab_evidence}" -- test -s "${ab_evidence}" 2>&1); then',
                                               '  if ab_rec_out=$(true); then')], ["golden-good: exactly one ev/1 entry", "recorder unavailable"], [T2])
m("T52-L3-empty-evidence-passes", [(LIB, '  if [ ! -s "${ab_evidence}" ]; then', '  if false; then')], ["negative control"], [T2])
m("T52-C1-census-counts-substrings", [(CEN, "            if c != \"c\":\n                mentions += 1\n                continue", "            if False:\n                continue")], ["decoys are not", "carriers"], [T2])
m("T52-C2-census-blind-heredoc", [(CEN, "        if hd is not None:\n            cmp = ", "        if False:\n            cmp = ")], ["decoys are not", "needle", "carriers"], [T2])
m("T52-C4-census-misses-function-keyword-form", [(CEN, "            if re.search(r\"(?:^|[;&|(){}\\s])function\\s+$\", before):", "            if False:")], ["control needle"], [T2])
# a restored per-script copy (done as a tree mutation by the capture script, see ts02_mutation below)


def apply(tree, edits):
    for f, old, new in edits:
        p = os.path.join(tree, f)
        s = open(p).read()
        if s.count(old) < 1:
            return "edit does not apply: %r not found in %s" % (old[:60], f)
        open(p, "w").write(s.replace(old, new, 1))
    return None


def build_tree(dst):
    shutil.copytree(os.path.join(ROOT, "tools/evidence"), os.path.join(dst, "tools/evidence"), ignore=shutil.ignore_patterns("__pycache__"))
    os.makedirs(os.path.join(dst, F, "contracts"))
    shutil.copy(os.path.join(ROOT, F, "contracts/evidence-record.schema.json"), os.path.join(dst, F, "contracts"))
    os.makedirs(os.path.join(dst, "scripts/repo"))
    shutil.copy(os.path.join(ROOT, "scripts/repo/check_classes.tsv"), os.path.join(dst, "scripts/repo"))
    shutil.copytree(os.path.join(ROOT, "scripts/testing"), os.path.join(dst, "scripts/testing"))
    shutil.copy(os.path.join(ROOT, F, "contracts/review-verdict.schema.json"), os.path.join(dst, F, "contracts"))   # test_anchors_r8.sh validates the WP-05 verdict file against it
    os.makedirs(os.path.join(dst, F, "evidence/reviews"))
    shutil.copy(os.path.join(ROOT, F, "evidence/reviews/WP-05.json"), os.path.join(dst, F, "evidence/reviews"))
    os.makedirs(os.path.join(dst, "docs/scripts"))      # test_anchors.sh scans the guide for a forced push (11.4.113); without it the UNMUTATED suite FAILs (review R6)
    shutil.copy(os.path.join(ROOT, "docs/scripts/evidence-recorder.md"), os.path.join(dst, "docs/scripts"))


def baseline(tests, prep=None):
    """The unmutated-baseline control (11.4.201(6); review WP-05 R6): the named test files run on an UNMUTATED scratch tree built exactly as the mutants'
    trees are, and every FAIL line is returned. A mutant 'caught' on a suite that already FAILs without any mutation is a false match (the scratch tree
    lacked the guide, so test_anchors.sh FAILed 'no-force-push scan found usage' and three mutants were recorded CAUGHT on that line alone).
    `prep(tree)` lets a test damage the tree to prove the control sees it."""
    tree = tempfile.mkdtemp(prefix="mutbase.")
    try:
        build_tree(tree)
        if prep:
            prep(tree)
        fails = []
        for t in tests:
            p = subprocess.run(["bash", os.path.join(tree, "tools/evidence/tests", t)], cwd=tree, capture_output=True, text=True, timeout=1500)
            fails += ["%s: %s" % (t, l) for l in (p.stdout + p.stderr).splitlines() if l.startswith("FAIL")]
            if p.returncode != 0 and not any(f.startswith(t + ":") for f in fails):
                fails.append("%s: exit %d with no FAIL line" % (t, p.returncode))
        return fails
    finally:
        shutil.rmtree(tree, ignore_errors=True)


def run_one(mut):
    i, edits, expect, tests = mut
    tree = tempfile.mkdtemp(prefix="mut.")
    try:
        build_tree(tree)
        err = apply(tree, edits)
        if err:
            return i, "INVALID", err
        for f, _o, _n in edits:
            if f.endswith(".py") and subprocess.run([sys.executable, "-I", "-c", "import ast,sys;ast.parse(open(sys.argv[1]).read())", os.path.join(tree, f)],
                                                     capture_output=True).returncode != 0:
                return i, "INVALID", "edited %s does not compile" % f
        fails = []
        for t in tests:
            p = subprocess.run(["bash", os.path.join(tree, "tools/evidence/tests", t)], cwd=tree, capture_output=True, text=True, timeout=1500)
            fails += [l for l in (p.stdout + p.stderr).splitlines() if l.startswith("FAIL")]
        if not expect:
            return i, ("CAUGHT-UNSPECIFIED" if fails else "NOT_CAUGHT"), "%d FAIL line(s); no expectation declared: reported, not counted as cause-caught" % len(fails)
        hit = [l for l in fails if any(x in l for x in expect)]
        if hit:
            return i, "CAUGHT", hit[0][:140]
        return i, ("NOT_CAUSE_CAUGHT" if fails else "NOT_CAUGHT"), ("%d unrelated FAIL line(s), e.g. %s" % (len(fails), fails[0][:100])) if fails else "no test failed"
    except subprocess.TimeoutExpired:
        return i, "INVALID", "timeout"
    finally:
        shutil.rmtree(tree, ignore_errors=True)


def main(argv):
    check = "--check" in argv
    if "--list" in argv:
        for i, e, x, t in M:
            print(i, [f for f, _o, _n in e], t)
        return 0
    j = 4
    if "-j" in argv:
        j = int(argv[argv.index("-j") + 1])
    if "--baseline" in argv:                               # --baseline TEST ... : only the control, on the named test files (default: every file the mutants run)
        names = [a for a in argv if a.endswith(".sh")] or sorted({t for mu in M for t in mu[3]})
        bf = baseline(names)
        print("baseline: %d test file(s), %d FAIL line(s) on the UNMUTATED tree" % (len(names), len(bf)))
        for f in bf:
            print("  " + f[:200])
        return 1 if bf else 0
    sel = [a for a in argv if not a.startswith("-") and not a.isdigit() and not a.endswith(".sh")]
    todo = [mu for mu in M if not sel or any(s in mu[0] for s in sel)]
    if check:
        bad = 0
        for i, edits, _x, _t in todo:
            tree = tempfile.mkdtemp(prefix="mutchk.")
            try:
                build_tree(tree)
                err = apply(tree, edits)
                print("%-55s %s" % (i, "ok" if not err else "INVALID: " + err)); bad += bool(err)
            finally:
                shutil.rmtree(tree, ignore_errors=True)
        return 1 if bad else 0
    needed = sorted({t for mu in todo for t in mu[3]})
    bf = baseline(needed) if needed else []                 # precondition: the suites are green WITHOUT a mutation, else a CAUGHT proves nothing
    if bf:
        print("BASELINE NOT GREEN: %d FAIL line(s) on the unmutated tree; no mutant verdict is meaningful (11.4.201(6))" % len(bf))
        for f in bf[:20]:
            print("  " + f[:200])
        return 2
    print("baseline green: %s run on the unmutated tree with 0 FAIL lines" % ", ".join(needed), flush=True)
    res = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=j) as ex:
        for r in ex.map(run_one, todo):
            res.append(r)
            print("%-55s %-18s %s" % r, flush=True)
    c = sum(1 for r in res if r[1] == "CAUGHT")
    unspec = [r[0] for r in res if r[1] == "CAUGHT-UNSPECIFIED"]
    bad = [r for r in res if r[1] in ("NOT_CAUGHT", "NOT_CAUSE_CAUGHT", "INVALID")]
    print("mutants=%d caught=%d reported_unspecified=%d not_caught_or_invalid=%d" % (len(res), c, len(unspec), len(bad)))
    for r in bad:
        print("SURVIVOR/INVALID: %s (%s) %s" % r)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.dont_write_bytecode = True
    sys.exit(main(sys.argv[1:]))
