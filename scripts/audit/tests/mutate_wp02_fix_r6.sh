#!/usr/bin/env bash
# Mutation test (constitution 1.1) for the WP-02 review-fix round 6 and 7 additions: scope_drift_check.sh, lumenignore_skeleton.py (round 7), the --allow-roots path of
# scope_to_lumen_json.py and the F6 report excludes of config/index/scope.yaml. Every mutant breaks ONE check and its test MUST then FAIL.
# Usage: bash scripts/audit/tests/mutate_wp02_fix_r6.sh      exit 0 = every mutant caught and the unmutated tests pass.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
. scripts/audit/tests/mut_verdict.sh
T="$(mktemp -d "${TMPDIR:-/tmp}/wp02r6_mut.XXXXXX")"; trap 'rm -rf "$T"' EXIT
SURV=0; N=0
# kind@@name@@source file@@old text (exactly once)@@new text
MUTS=(
'sdc@@s01-lumenignore-cmp-dropped@@scripts/audit/scope_drift_check.sh@@elif [ -s "$T/fresh.lumenignore" ] && cmp -s "$T/fresh.lumenignore" "$LI"; then RC=0@@elif true; then RC=0'
'sdc@@s02-exit-ignores-diff@@scripts/audit/scope_drift_check.sh@@[ "$BAD" -eq 0 ]@@true'
'sdc@@s03-absent-tool-passes@@scripts/audit/scope_drift_check.sh@@[ "$ABSENT" -eq 0 ] || exit 3@@true'
'sdc@@s04-regen-noop@@scripts/audit/scope_drift_check.sh@@then cp "$T/fresh.lumenignore" "$LI"; fi@@then true; fi'
'sdc@@s05-empty-fresh-ok@@scripts/audit/scope_drift_check.sh@@elif [ -s "$T/fresh.lumenignore" ] && cmp@@elif cmp'
'sk@@k05-gitmodules-check-off@@scripts/audit/lumenignore_skeleton.py@@if gpath and gpath not in gitlinks and not any(p.startswith(gpath + "/") for p in listed):  # MUT:skeleton-gitmodules@@if False:  # MUT:skeleton-gitmodules'
'sk@@k06-allowed-root-check-off@@scripts/audit/lumenignore_skeleton.py@@if not any(p == x or p.startswith(x + "/") for p in listed):  # MUT:skeleton-allowed-root@@if False:  # MUT:skeleton-allowed-root'
'sk@@k07-gitmodules-needs-gitlink-only@@scripts/audit/lumenignore_skeleton.py@@gpath not in gitlinks and not any(p.startswith(gpath + "/") for p in listed)@@gpath not in gitlinks'
'stj@@j01-sibling-default-off@@scripts/audit/scope_to_lumen_json.py@@roots_file = sib if os.path.isfile(sib) else None@@roots_file = None'
'stj@@j02-roots-not-merged@@scripts/audit/scope_to_lumen_json.py@@(list(have) if isinstance(have, list) else []) + extra["lumen_allow_roots"]@@(list(have) if isinstance(have, list) else [])'
'stj@@j03-malformed-roots-not-refused@@scripts/audit/scope_to_lumen_json.py@@fail(3, "malformed allow-roots file %s: want a mapping with a list of strings under lumen_allow_roots" % roots_file)@@roots_file = None'
'sdc@@s06-render-from-live-directory@@scripts/audit/scope_drift_check.sh@@python3 "$GL" --repo "$T/skel" --scope@@python3 "$GL" --repo . --scope'
'sdc@@s07-report-scope-binding-zero@@scripts/audit/scope_drift_check.sh@@"scope_sha256": sha(sys.argv[4])@@"scope_sha256": "0" * 64'
'sdc@@s08-report-lumenignore-binding-zero@@scripts/audit/scope_drift_check.sh@@"lumenignore_sha256": sha(sys.argv[6])@@"lumenignore_sha256": "0" * 64'
'sdc@@s09-report-head-empty@@scripts/audit/scope_drift_check.sh@@HEAD_ID="$(git rev-parse --verify -q HEAD 2>/dev/null || echo unborn)"@@HEAD_ID=""'
'sdc@@s10-report-is-a-bare-list@@scripts/audit/scope_drift_check.sh@@"checks": [{"check": n, "rc": int(rc)} for n, rc in rows]}@@"checks": [{"check": n, "rc": int(rc)} for n, rc in rows], "schema2": 1}; doc = doc["checks"]'
'sk@@k01-host-local-list-empty@@scripts/audit/lumenignore_skeleton.py@@HOST_LOCAL = (".git", ".claude", ".remember")@@HOST_LOCAL = ()'
'sk@@k02-no-submodule-recursion@@scripts/audit/lumenignore_skeleton.py@@"ls-files", "-z", "-s", "--recurse-submodules"]@@"ls-files", "-z", "-s"]'
'sk@@k03-gitlink-as-file@@scripts/audit/lumenignore_skeleton.py@@            if is_link:\n                mkdir(path)@@            if False:\n                mkdir(path)'
'sk@@k04-untracked-listed@@scripts/audit/lumenignore_skeleton.py@@    for name in (a.host_local if a.host_local is not None else HOST_LOCAL):\n        mkdir(name)@@    for name in (a.host_local if a.host_local is not None else HOST_LOCAL):\n        mkdir(name)\n    for name in sorted(os.listdir(a.repo)):\n        mkdir(name) if os.path.isdir(os.path.join(a.repo, name)) else None'
'stj@@j04-no-allow-roots-check-off@@scripts/audit/scope_to_lumen_json.py@@if a.out and not have_roots and not a.no_allow_roots:@@if False:'
'stj@@j05-opt-out-ignored@@scripts/audit/scope_to_lumen_json.py@@if a.out and not have_roots and not a.no_allow_roots:@@if a.out and not have_roots:'
'stj@@j06-check-also-refused@@scripts/audit/scope_to_lumen_json.py@@if a.out and not have_roots and not a.no_allow_roots:@@if not have_roots and not a.no_allow_roots:'
'neg@@n01-html-exclude-dropped@@config/index/scope.yaml@@  - /submodules/doc_processor/pkg/coverage/*.html\n@@'
'neg@@n02-info-exclude-dropped@@config/index/scope.yaml@@  - /submodules/doc_processor/pkg/coverage/*.info\n@@'
'neg@@n03-xml-exclude-dropped@@config/index/scope.yaml@@  - /submodules/doc_processor/pkg/coverage/*.xml\n@@'
)
python3 - "$T" "${MUTS[@]}" <<'PY' || exit 2
import sys
t = sys.argv[1]
for spec in sys.argv[2:]:
    kind, name, src, old, new = spec.split("@@")
    old, new = old.replace("\\n", "\n"), new.replace("\\n", "\n")
    text = open(src).read()
    if text.count(old) != 1:
        print("HARNESS ERROR: %s: old text occurs %d times" % (name, text.count(old))); sys.exit(3)
    open("%s/%s.%s" % (t, name, kind), "w").write(text.replace(old, new))
PY
base=0
bash scripts/audit/tests/test_scope_drift_check.sh >"$T/b1" 2>&1 || base=1
bash scripts/audit/tests/test_scope_to_lumen_json.sh >"$T/b2" 2>&1 || base=1
bash scripts/audit/tests/test_scope_negation_reports.sh >"$T/b3" 2>&1 || base=1
bash scripts/audit/tests/test_lumenignore_skeleton_export.sh >"$T/b4" 2>&1 || base=1
echo "baseline (unmutated) rc=$base"; [ "$base" -eq 0 ] || { echo "baseline FAILS: results meaningless"; exit 2; }
for f in "$T"/*.sdc "$T"/*.sk "$T"/*.stj "$T"/*.neg; do
  n="$(basename "$f")"; N=$((N+1)); rc=0
  case "$f" in
    *.sdc) cp "$f" "$T/m.sh"; chmod +x "$T/m.sh"; LUMEN_SKELETON="$PWD/scripts/audit/lumenignore_skeleton.py" SDC="$T/m.sh" bash scripts/audit/tests/test_scope_drift_check.sh >"$T/o" 2>&1; rc=$? ;;
    *.sk) cp "$f" "$T/m_skel.py"; LUMEN_SKELETON="$T/m_skel.py" bash scripts/audit/tests/test_scope_drift_check.sh >"$T/o" 2>&1; r1=$?
          SK="$T/m_skel.py" bash scripts/audit/tests/test_lumenignore_skeleton_export.sh >>"$T/o" 2>&1; r2=$?; rc="$(mut_combine "$r1" "$r2")" ;;
    *.stj) cp "$f" "$T/m.py"; STJ="$T/m.py" bash scripts/audit/tests/test_scope_to_lumen_json.sh >"$T/o" 2>&1; rc=$? ;;
    *.neg) SCOPE="$f" bash scripts/audit/tests/test_scope_negation_reports.sh >"$T/o" 2>&1; rc=$? ;;
  esac
  case "$(mut_class "$rc")" in
    CAUGHT) echo "CAUGHT   $n ($(grep -c '^FAIL' "$T/o") failing check(s): $(grep '^FAIL' "$T/o" | head -1 | cut -c1-80))" ;;
    SKIPPED-NOT-KILLED) echo "SKIPPED-NOT-KILLED $n (a test exited 77: a skipped leg is not a kill)"; SURV=$((SURV+1)) ;;
    *) echo "SURVIVED $n"; SURV=$((SURV+1)) ;;
  esac
done
echo "MUTATIONS total=$N survived=$SURV"
[ "$SURV" -eq 0 ]
