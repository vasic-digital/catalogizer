#!/usr/bin/env bash
# Mutation test for scripts/audit/index_health.sh (T021, constitution 1.1): every mutant below breaks ONE check and a covering test
# (run with IH pointing at the mutant) MUST then FAIL.
# Round 10 (reviews WP-02-T028-r3): (1) the harness runs the mutants in PARALLEL (JOBS, default 8) and stops a mutant at its first failing test;
# (2) a test that exits 77 (a skipped leg) is NEVER a kill: SKIPPED-NOT-KILLED, counted with the survivors (R3-5, mut_verdict.sh);
# (3) marker mutants `mark@@name@@marker@@old@@new` edit ONE condition on the line carrying `# MUT:<marker>`; (4) the test list is overridable (MUT_TESTS).
# Usage: [ONLY=<mutant-name regex>] [JOBS=n] [MUT_TESTS="t1 t2"] bash scripts/audit/tests/mutate_index_health.sh
#        exit 0 = every mutant caught (or listed as EQUIVALENT, name contains -eq-) and the unmutated script passes.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
. scripts/audit/tests/mut_verdict.sh
SRC=scripts/audit/index_health.sh
if [ -n "${MUT_TESTS:-}" ]; then read -r -a TESTS <<<"$MUT_TESTS"; else
  TESTS=(scripts/audit/tests/test_index_health.sh scripts/audit/tests/test_index_health_extra.sh scripts/audit/tests/test_index_health_fix_r7.sh scripts/audit/tests/test_index_health_fix_r8.sh); fi
JOBS="${JOBS:-8}"
T="$(mktemp -d "${TMPDIR:-/tmp}/ih_mut.XXXXXX")"; trap 'rm -rf "$T"' EXIT
# name|old text (must occur exactly once)|new text      (the legacy form: m01..m61)
MUTS=(
'm01-good-needs-pendingrefs-1|elif pr != 0:  # MUT:p1-pendingrefs|elif pr != 1:  # MUT:p1-pendingrefs'
'm02-pendingrefs-ignored|elif pr != 0:  # MUT:p1-pendingrefs|elif False:  # MUT:p1-pendingrefs'
'm03-absent-field-read-as-zero|return (False, None)|return (True, 0)'
'm04-worktree-ignored|if lookup(st, "worktreeMismatch")[1] is not None:  # MUT:p1-worktree|if False:  # MUT:p1-worktree'
'm05-negative-needle-ignored|if needles["needle_neg"]["found"]:  # MUT:p2-neg|if False:  # MUT:p2-neg'
'm06-positive-needle-ignored|if not needles["needle_pos"]["found"]:  # MUT:p2-pos|if False:  # MUT:p2-pos'
'm07-stale-ignored|if m["stale"].lower() != "no":  # MUT:p7-stale|if False:  # MUT:p7-stale'
'm08-no-status-passes|lumen_rows.append(row("P-Lumen", "FAIL", reason="no_status_capture_and_no_probe"))  # MUT:plumen-nostatus|lumen_rows.append(row("P-Lumen", "PASS", reason="x"))  # MUT:plumen-nostatus'
'm09-bin-absent-passes|lumen_rows.append(row("P-Lumen", "FAIL", reason="lumen_bin_not_executable_or_absent", path=b))|lumen_rows.append(row("P-Lumen", "PASS", reason="x", path=b))'
'm10-parity-tolerance-ignored|ok = pct <= tol  # MUT:p8-tol|ok = True  # MUT:p8-tol'
'm11-out-optional|if req not in opts:  # MUT:require-out|if req not in opts and req != "--out":  # MUT:require-out'
'm12-schema-marker|"schema": "index-health/1"|"schema": "index-health/2"'
'm13-p2-needles-not-recorded|indexed=len(indexed), missing_count=len(missing), missing=missing[:50], **needles)|indexed=len(indexed), missing_count=len(missing), missing=missing[:50])'
'm15-chunks-zero-ok|if int(m["chunks"]) <= 0:|if False:'
'm16-files-ne-indexed-ok|if m["files"] != m["indexed"]:|if False:'
'm17-secret-class-ignored|if secret:|if False:'
'm18-env-example-flagged|if os.path.basename(p) in (".env.example", ".env.sample"):|if False:'
'm19-third-party-ignored|if third:|if False:'
'm20-own-org-ignored|if own_missing:|if False:'
'm21-tracked-missing-ignored|if missing:|if False:'
'm22-bin-probe-passes|lumen_rows.append(row("P-Lumen", "FAIL", reason="probe_not_run_read_only_gate_needs_status_capture", path=b))|lumen_rows.append(row("P-Lumen", "PASS", path=b))'
'm23-skip-p8-not-incomplete|"complete": not skips,|"complete": True,'
'm24-hash-not-of-file|inputs[key] = {"path": path, "sha256": hashlib.sha256(data).hexdigest(), "readable": True}|inputs[key] = {"path": path, "sha256": hashlib.sha256(path.encode()).hexdigest(), "readable": True}'
'm25-dup-option-allowed|if a in opts:|if False:'
'm26-empty-tracked-not-blind|    if not tracked:\n        why.append("tracked_list_empty_blind")\n|'
'm27-state-not-complete-ignored|    if lookup(st, "index", "state")[1] != "complete":\n        why.append("index.state_not_complete")\n|'
'm28-noncanonical-third-party-root-ignored|bad_roots = [r for r in roots if noncanonical_root(r)]|bad_roots = []'
'm29-eq-noncanonical-no-control-char-check-covered-by-C-category|if r != r.strip() or any(ord(c) < 32 or ord(c) == 127 for c in r):|if r != r.strip():'
'm30-noncanonical-no-whitespace-check|if r != r.strip() or any(ord(c) < 32 or ord(c) == 127 for c in r):|if any(ord(c) < 32 or ord(c) == 127 for c in r):'
'm31-noncanonical-no-dot-component-check|any(c in ("", ".", "..") for c in comps)|any(c in ("",) for c in comps)'
'm14-verdict-never-fail|if any(r["verdict"] == "FAIL" for r in all_rows):  # MUT:verdict|if False:  # MUT:verdict'
# round 8 (WP-02-T028 review): the five reviewer-authored survivors (M1 reindex, M2 pendingChanges, M3 captured time, M6 key extensions via the class source, M8 own-org prefix boundary) and the new checks
'm32-reindex-ignored|if lookup(st, "index", "reindexRecommended")[1] is not False:|if False:'
'm33-pendingchanges-ignored|        elif v != 0:\n            why.append("pendingChanges.%s_nonzero" % k)|        elif False:\n            why.append("pendingChanges.%s_nonzero" % k)'
'm34-captured-unparseable-ok|why.append("captured_time_unparseable")  # MUT:p7-captured|pass  # MUT:p7-captured'
'm35-ownorg-prefix-boundary|p.startswith(r.rstrip("/") + "/") for p in indexed))  # MUT:ownorg-boundary|p.startswith(r.rstrip("/")) for p in indexed))  # MUT:ownorg-boundary'
'm36-notbefore-ignored|if cnb is not None and cap < cnb:  # MUT:p7-notbefore|if False:  # MUT:p7-notbefore'
'm37-class-selftest-skipped|if not rx.search(c["positive"]) or rx.search(c["negative"]):|if False:'
'm38-only-first-class-used|        for name, rx in classes:\n            if rx.search(p):|        for name, rx in classes[:1]:\n            if rx.search(p):'
'm39-no-classes-not-blind|if not isinstance(fc, list) or not fc:|if False:'
'm40-skip-not-incomplete|elif skips:  # MUT:skip-incomplete|elif False:  # MUT:skip-incomplete'
'm41-incomplete-exits-0|"INCOMPLETE": 3}|"INCOMPLETE": 0}'
'm42-drift-rc-ignored|if rc != 0]  # MUT:drift-rc|if False]  # MUT:drift-rc'
'm43-lumenignore-always-pass|"PASS" if a == b else "FAIL", cmd="committed|"PASS", cmd="committed'
'm44-fresh-empty-not-blind|    if not b.strip():|    if False:'
# round 9 (review WP-02-T028-r2): the reviewer survivor R5 (exemption widened) and one mutant per new check
'm48-live-hash-unchecked|if hashlib.sha256(data).hexdigest() != e["evidence_sha256"]:  # MUT:live-hash|if False:  # MUT:live-hash'
'm50-fallback-counts-as-live|if e["kind"] == "fallback":  # MUT:live-fallback|if False:  # MUT:live-fallback'
'm51-drift-not-bound-to-scope|if (inputs.get("scope") or {}).get("sha256") != d["scope_sha256"]:  # MUT:drift-bind-scope|if False:  # MUT:drift-bind-scope'
'm52-drift-not-bound-to-lumenignore|if (inputs.get("lumenignore") or {}).get("sha256") != d["lumenignore_sha256"]:  # MUT:drift-bind-lumenignore|if False:  # MUT:drift-bind-lumenignore'
'm53-cnb-optional|if ("--lumen-status" in opts or "--live-proofs" in opts) and cnb is None:  # MUT:cnb-required|if False:  # MUT:cnb-required'
'm55-env-example-exemption-widened|if os.path.basename(p) in (".env.example", ".env.sample"):  # MUT:exemption-exact|if p.endswith((".example", ".sample")):  # MUT:exemption-exact'
'm56-classes-case-sensitive|rx = re.compile(c["regex"], re.IGNORECASE)|rx = re.compile(c["regex"])'
'm61-baseline-not-a-mapping-ok|    if be is not None and not isinstance(be, dict):|    if False:'
)

# mark@@name@@marker@@old condition@@new condition: the line carrying `# MUT:<marker>` must hold `old` exactly once; it is replaced by `new` there only
MARKS=(
'mark@@n01-policy-optional@@policy-required@@"--live-proofs" in opts and "--live-policy" not in opts@@False'
'mark@@n02-ledger-optional@@ledger-required@@("--live-proofs" in opts or "--drift-report" in opts) and "--evidence-ledger" not in opts@@False'
'mark@@n03-cnb-age-unbounded@@cnb-age-bound@@cnb < NOW - POLICY["max_age"]@@False'
'mark@@n04-cnb-future-ok@@cnb-future@@cnb > NOW + datetime.timedelta(seconds=POLICY["skew"])@@False'
'mark@@n05-ledger-untracked-ok@@ledger-tracked@@not tracked_clean(lp)@@False'
'mark@@n06-evidence-untracked-ok@@evidence-tracked@@not tracked_clean(ap)@@False'
'mark@@n07-evidence-not-in-ledger-ok@@evidence-in-ledger@@rel.startswith("..") or rel not in L[1]@@False'
'mark@@n08-ledger-hash-ignored@@ledger-hash@@hashlib.sha256(data).hexdigest() not in L[1][rel] or len(L[1][rel]) != 1@@False'
'mark@@n09-program-relative-ok@@program-absolute@@not isinstance(prog, str) or not os.path.isabs(prog)@@False'
'mark@@n10-program-basename-only@@program-allowlist@@os.path.normpath(prog) != prog or not any(program_regex(x).fullmatch(prog) for x in patterns)@@not any(os.path.basename(prog) == os.path.basename(x) for x in patterns)'
'mark@@n11-program-missing-ok@@program-exists@@not (os.path.isfile(prog) and os.access(prog, os.X_OK))@@False'
'mark@@n12-evidence-duplicate-key-ok@@evidence-dup@@k in d@@False'
'mark@@n13-evidence-schema-unchecked@@evidence-schema@@d.get("schema") != EVIDENCE_SCHEMA@@False'
'mark@@n14-p4-row-unchecked@@p4-row-strict@@not (kv.get("id") and kv.get("expect") in ("answerable", "unanswerable") and kv.get("result") in ("correct", "wrong", "refused"))@@False'
'mark@@n15-p4-queries-claim-unchecked@@p4-summary@@claim[0] != total@@False'
'mark@@n16-p4-correct-claim-unchecked@@p4-summary@@claim[1] != len(correct)@@False'
'mark@@n17-p4-recall-claim-unchecked@@p4-summary@@abs(claim[2] - recall) > 0.0005@@False'
'mark@@n18-p4-min-queries-off@@p4-min-queries@@total < POLICY["min_queries"]@@False'
'mark@@n19-p4-control-absent-ok@@p4-control@@not controls or any(@@any('
'mark@@n20-p4-control-answered-ok@@p4-control@@any(r["result"] != "refused" for r in controls)@@False'
'mark@@n21-p4-recall-threshold-off@@p4-recall@@recall + 1e-12 < POLICY["min_recall"]@@False'
'mark@@n22-p5-future-commit-ok@@p5-commit-future@@lc > NOW + SKEW_TD@@False'
'mark@@n23-p5-order-ignored@@p5-order@@li < lc@@False'
'mark@@n24-p6-dims-ignored@@p6-dims@@int(d["embed_dims"]) != POLICY["dims"]@@False'
'mark@@n25-p6-vectors-ignored@@p6-vectors@@int(d["embed_vectors"]) < 1@@False'
'mark@@n26-p6-norm-ignored@@p6-norm@@not math.isfinite(norm) or norm <= 0@@False'
'mark@@n27-p6-nan-norm-ok@@p6-norm@@not math.isfinite(norm) or norm <= 0@@norm <= 0'
'mark@@n28-zero-digest-ok@@zero-digest@@is_zero_digest(e["evidence_sha256"])@@False'
'mark@@n29-evidence-proof-slot-unchecked@@evidence-proof@@d["proof"] != k@@False'
'mark@@n30-evidence-exit-ignored@@evidence-exit@@d["exit"] != "0"@@False'
'mark@@n31-entry-cmd-unchecked@@entry-cmd@@e["cmd"] != d["program"] + " " + d["argv"]@@False'
'mark@@n32-evidence-scope-unchecked@@evidence-scope@@d["scope_sha256"] != (inputs.get("scope") or {}).get("sha256")@@False'
'mark@@n33-entry-captured-at-unchecked@@entry-captured-at@@e["captured_at"] != d["captured_at"]@@False'
'mark@@n34-live-future-ok@@live-future@@when > NOW + SKEW_TD@@False'
'mark@@n35-live-notbefore-ignored@@live-fresh@@when < cnb@@False'
'mark@@n36-entry-verdict-unchecked@@entry-verdict@@e.get("verdict") is not None and e["verdict"] != verdict@@False'
'mark@@n37-drift-head-unknown-ok@@drift-head-known@@git("rev-parse", "--verify", "-q", head + "^{commit}").returncode != 0@@False'
'mark@@n38-drift-head-not-ancestor-ok@@drift-head-ancestor@@git("merge-base", "--is-ancestor", full, "HEAD").returncode != 0 or ldir is None or ldir.startswith("..")@@False'
'mark@@n39-drift-head-any-change-ok@@drift-head-only-ledger-dir@@any(not c.startswith(pre) for c in changed)@@False'
'mark@@n40-lumen-json-optional@@lumen-json-required@@"--lumen-json" not in opts@@False'
'mark@@n41-drift-zero-digest-ok@@drift-zero@@any(is_zero_digest(d[k]) for k in ("scope_sha256", "lumen_scope_sha256", "lumenignore_sha256"))@@False'
'mark@@n42-drift-lumen-binding-off@@drift-bind-lumen-json@@(inputs.get("lumen_json") or {}).get("sha256") != d["lumen_scope_sha256"]@@False'
'mark@@n43-drift-rc-crosscheck-off@@drift-rc-crosscheck@@(seen["lumenignore_fresh"] == 0) != (a_ == b_ and bool(b_.strip()))@@False'
'mark@@n44-p7-future-ok@@p7-future@@cap > NOW + SKEW_TD@@False'
'mark@@n45-probe-depth-a-only@@probe-depths@@PROBE_DEPTHS = ("", "a/", "a/b/")@@PROBE_DEPTHS = ("a/",)'
'mark@@n46-probe-depth-root-only@@probe-depths@@PROBE_DEPTHS = ("", "a/", "a/b/")@@PROBE_DEPTHS = ("",)'
'mark@@n47-probe-depth-2-dropped@@probe-depths@@PROBE_DEPTHS = ("", "a/", "a/b/")@@PROBE_DEPTHS = ("", "a/")'
'mark@@n48-probe-depth-1-dropped@@probe-depths@@PROBE_DEPTHS = ("", "a/", "a/b/")@@PROBE_DEPTHS = ("", "a/b/")'
'mark@@n49-probe-depth-0-dropped@@probe-depths@@PROBE_DEPTHS = ("", "a/", "a/b/")@@PROBE_DEPTHS = ("a/", "a/b/")'
'mark@@n50-probe-keeps-doublestar@@strip-doublestar@@p.startswith("**/")@@False'
'mark@@n51-baseline-coverage-unchecked@@baseline-coverage@@not all(any(rx.search(pr) for _, rx in out) for pr in baseline_probes(pat.strip()))@@False'
'mark@@n52-baseline-predicate-off@@baseline-predicate@@baseline_hit(br, p)@@False'
'mark@@n53-baseline-negation-narrowing-off@@baseline-narrowing@@(v + "/") in SCOPE_NEGATIONS@@False'
'mark@@n54-baseline-negation-narrowing-always@@baseline-narrowing@@(v + "/") in SCOPE_NEGATIONS@@True'
'mark@@n55-baseline-topdir-file-matches@@baseline-topdir@@len(comps) > 1 and comps[0] == vl@@comps[0] == vl'
'mark@@n56-baseline-dir-matches-file@@baseline-dir@@vl in comps[:-1]@@vl in comps'
'mark@@n57-baseline-seg-last-only@@baseline-seg@@vl in comps@@vl == comps[-1]'
'mark@@n58-baseline-exact-prefix@@baseline-exact@@pl == vl@@pl.startswith(vl)'
'mark@@n59-baseline-case-sensitive-path@@baseline-case@@pl, vl = p.lower(), v.lower()@@pl, vl = p, v.lower()'
'mark@@n60-baseline-case-sensitive-value@@baseline-case@@pl, vl = p.lower(), v.lower()@@pl, vl = p.lower(), v'
)
python3 - "$SRC" "$T" "${MUTS[@]}" "${MARKS[@]}" <<'PY' || exit 2
import sys
src, t = sys.argv[1:3]
text = open(src).read()
for spec in sys.argv[3:]:
    if spec.startswith("mark@@"):
        _, name, marker, old, new = spec.split("@@")
        lines = text.split("\n")
        idx = [i for i, l in enumerate(lines) if "# MUT:" + marker in l and old in l]
        if len(idx) != 1 or lines[idx[0]].count(old) != 1:
            print("HARNESS ERROR: %s: marker %s with the old condition occurs on %d line(s)" % (name, marker, len(idx))); sys.exit(3)
        lines[idx[0]] = lines[idx[0]].replace(old, new)
        open("%s/%s.sh" % (t, name), "w").write("\n".join(lines))
        continue
    name, old, new = spec.split("|", 2)
    old, new = old.replace("\\n", "\n"), new.replace("\\n", "\n")
    if text.count(old) != 1:
        print("HARNESS ERROR: %s: old text occurs %d times" % (name, text.count(old))); sys.exit(3)
    open("%s/%s.sh" % (t, name), "w").write(text.replace(old, new))
PY
base_rc=0; : > "$T/base.out"
for tt in "${TESTS[@]}"; do bash "$tt" >>"$T/base.out" 2>&1; r=$?; [ "$r" -eq 0 ] || base_rc=$r; done
echo "baseline (unmutated) rc=$base_rc $(tail -1 "$T/base.out")"
[ "$base_rc" -eq 0 ] || { echo "baseline FAILS: mutation results meaningless"; exit 2; }
runone() {  # runone <mutant file>: tests in order, stop at the first real failure; prints one verdict line to $T/<name>.verdict
  local f="$1" n; n="$(basename "$f" .sh)"; chmod +x "$f"
  local rcs=() r tt
  for tt in "${TESTS[@]}"; do
    IH="$f" bash "$tt" >>"$T/$n.out" 2>&1; r=$?; rcs+=("$r")
    [ "$r" -eq 0 ] || [ "$r" -eq 77 ] || break
  done
  r="$(mut_combine "${rcs[@]}")"
  case "$(mut_class "$r")" in
    CAUGHT) echo "CAUGHT   $n ($(grep -c '^FAIL' "$T/$n.out") failing check(s): $(grep '^FAIL' "$T/$n.out" | head -1 | cut -c1-70))" ;;
    SKIPPED-NOT-KILLED) echo "SKIPPED-NOT-KILLED $n (a test exited 77: a skipped leg is not a kill)" ;;
    *) if case "$n" in *-eq-*) true ;; *) false ;; esac; then echo "EQUIVALENT $n (survived as documented)"; else echo "SURVIVED $n"; fi ;;
  esac > "$T/$n.verdict"
}
for f in "$T"/[mn]*.sh; do
  n="$(basename "$f" .sh)"
  [ -z "${ONLY:-}" ] || printf '%s' "$n" | grep -Eq "$ONLY" || continue   # ONLY=<regex> re-runs a subset
  while [ "$(jobs -rp | wc -l)" -ge "$JOBS" ]; do wait -n; done
  runone "$f" &
done
wait
N=0; SURV=0
for v in "$T"/*.verdict; do
  [ -e "$v" ] || continue
  N=$((N+1)); cat "$v"
  case "$(cut -d' ' -f1 "$v")" in SURVIVED|SKIPPED-NOT-KILLED) SURV=$((SURV+1)) ;; esac
done
echo "MUTATIONS total=$N survived=$SURV"
[ "$SURV" -eq 0 ]
