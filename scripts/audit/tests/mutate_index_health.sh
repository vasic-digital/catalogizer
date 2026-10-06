#!/usr/bin/env bash
# Mutation test for scripts/audit/index_health.sh (T021, constitution 1.1): every mutant below breaks ONE check and the
# T016 test or the T021 supplementary test (run with IH pointing at the mutant) MUST then FAIL.
# Usage: bash scripts/audit/tests/mutate_index_health.sh      exit 0 = every mutant caught (or listed as EQUIVALENT, name contains -eq-) and the unmutated script passes.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
SRC=scripts/audit/index_health.sh; TEST=scripts/audit/tests/test_index_health.sh; TEST2=scripts/audit/tests/test_index_health_extra.sh
T="$(mktemp -d "${TMPDIR:-/tmp}/ih_mut.XXXXXX")"; trap 'rm -rf "$T"' EXIT
SURV=0; N=0
# name|old text (must occur exactly once)|new text
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
'm18-env-example-flagged|os.path.basename(p) not in (".env.example", ".env.sample"))|True)'
'm19-third-party-ignored|if third:|if False:'
'm20-own-org-ignored|if own_missing:|if False:'
'm21-tracked-missing-ignored|if missing:|if False:'
'm22-bin-probe-passes|lumen_rows.append(row("P-Lumen", "FAIL", reason="probe_not_run_read_only_gate_needs_status_capture", path=b))|lumen_rows.append(row("P-Lumen", "PASS", path=b))'
'm23-skip-p8-not-incomplete|"complete": not any(r["verdict"] == "SKIP" for r in all_rows),|"complete": True,'
'm24-hash-not-of-file|inputs[key] = {"path": path, "sha256": hashlib.sha256(data).hexdigest(), "readable": True}|inputs[key] = {"path": path, "sha256": hashlib.sha256(path.encode()).hexdigest(), "readable": True}'
'm25-dup-option-allowed|if a in opts:|if False:'
'm26-empty-tracked-not-blind|    if not tracked:\n        why.append("tracked_list_empty_blind")\n|'
'm27-state-not-complete-ignored|    if lookup(st, "index", "state")[1] != "complete":\n        why.append("index.state_not_complete")\n|'
'm28-noncanonical-third-party-root-ignored|bad_roots = [r for r in roots if noncanonical_root(r)]|bad_roots = []'
'm29-eq-noncanonical-no-control-char-check-covered-by-C-category|if r != r.strip() or any(ord(c) < 32 or ord(c) == 127 for c in r):|if r != r.strip():'
'm30-noncanonical-no-whitespace-check|if r != r.strip() or any(ord(c) < 32 or ord(c) == 127 for c in r):|if any(ord(c) < 32 or ord(c) == 127 for c in r):'
'm31-noncanonical-no-dot-component-check|any(c in ("", ".", "..") for c in comps)|any(c in ("",) for c in comps)'
'm14-verdict-always-pass|verdict = "FAIL" if any(r["verdict"] == "FAIL" for r in all_rows) else "PASS"  # MUT:verdict|verdict = "PASS"  # MUT:verdict'
)
python3 - "$SRC" "$T" "${MUTS[@]}" <<'PY' || exit 2
import sys
src, t = sys.argv[1:3]
text = open(src).read()
for spec in sys.argv[3:]:
    name, old, new = spec.split("|", 2)
    old, new = old.replace("\\n", "\n"), new.replace("\\n", "\n")
    if text.count(old) != 1:
        print("HARNESS ERROR: %s: old text occurs %d times" % (name, text.count(old))); sys.exit(3)
    open("%s/%s.sh" % (t, name), "w").write(text.replace(old, new))
PY
bash "$TEST" >"$T/base.out" 2>&1; brc=$?; bash "$TEST2" >>"$T/base.out" 2>&1; brc=$((brc+$?))
echo "baseline (unmutated) rc=$brc $(tail -1 "$T/base.out")"
[ "$brc" -eq 0 ] || { echo "baseline FAILS: mutation results meaningless"; exit 2; }
for f in "$T"/m*.sh; do
  n="$(basename "$f" .sh)"; chmod +x "$f"; N=$((N+1))
  IH="$f" bash "$TEST" >"$T/$n.out" 2>&1; rc=$?; IH="$f" bash "$TEST2" >>"$T/$n.out" 2>&1; rc=$((rc+$?))
  if [ "$rc" -ne 0 ]; then echo "CAUGHT   $n ($(grep -c '^FAIL' "$T/$n.out") failing check(s): $(grep '^FAIL' "$T/$n.out" | head -1 | cut -c1-70))"
  elif case "$n" in *-eq-*) true ;; *) false ;; esac; then echo "EQUIVALENT $n (survived as documented: every control character is a Unicode C* category character and noncanonical_root refuses those in its next loop; WF7 equivalence entry)"
  else echo "SURVIVED $n"; SURV=$((SURV+1)); fi
done
echo "MUTATIONS total=$N survived=$SURV"
[ "$SURV" -eq 0 ]
