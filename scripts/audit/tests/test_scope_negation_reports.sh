#!/usr/bin/env bash
# WP-02 review F6 test (TDD): the scope negation that re-includes first-party source must not admit generated coverage reports.
#
# Purpose   The negation `!/submodules/doc_processor/pkg/coverage/` (needed because the unanchored root rule `coverage/` and the engine's built-in
#           directory skips drop that Go package) also un-ignores every generated report in the directory. The rendered codegraph.json `exclude`
#           list (which wins over a gitignore negation, scope.example.yaml) must therefore drop the report types again. This test renders scope.yaml
#           through scope_render.py's own functions and judges, last matching pattern wins, whether each probe path is excluded.
# Usage     bash scripts/audit/tests/test_scope_negation_reports.sh      Env: SCOPE=<scope.yaml> (the mutation run points it at a mutant)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
SCOPE="${SCOPE:-config/index/scope.yaml}"
R=constitution/scripts/codegraph/scope_render.py
[ -f "$R" ] || { echo "SKIP scope_render.py absent (constitution submodule not present): reason recorded, not a pass (exit 77)"; exit 77; }
python3 - "$SCOPE" "$R" <<'PY'
import importlib.util, json, sys
scope, rpath = sys.argv[1:3]
spec = importlib.util.spec_from_file_location("scope_render", rpath)
sr = importlib.util.module_from_spec(spec); spec.loader.exec_module(sr)
cfg_text, block, summary, _s, _d = sr.render(scope, ".")
exclude = json.loads(cfg_text)["exclude"]
def excluded(path):
    verdict = False
    for q in exclude:                      # last matching pattern wins (the engine's `ignore` matcher)
        neg = q.startswith("!")
        if sr.gitignore_to_regex(q[1:] if neg else q).match(path):
            verdict = not neg
    return verdict
D = "submodules/doc_processor/pkg/coverage/"
fails = 0
def check(label, cond):
    global fails
    print(("ok   " if cond else "FAIL ") + label)
    fails += 0 if cond else 1
for rep in ("coverage.html", "lcov.info", "coverage.xml", "index.html", "coverage.out", "cover.profile", "coverage.json", "report.txt"):
    check("F6 generated report %s%s is excluded from the index" % (D, rep), excluded(D + rep))
for src in ("tracker.go", "tracker_test.go", "tracker_stress_test.go"):
    check("F6 control: first-party source %s%s is NOT excluded (instrument sees)" % (D, src), not excluded(D + src))
check("F6 control: the negation block still re-includes the package directory", "!/submodules/doc_processor/pkg/coverage/" in block)
check("F6 control: a report outside the package is not this rule's business (no over-broad exclude)", not excluded("catalog-web/src/coverage-helper.html"))
sys.exit(1 if fails else 0)
PY
rc=$?
# git side: the negation keeps the Go source un-ignored in a scratch repository carrying the repository's .gitignore (git cannot see inside the submodule)
S="$(mktemp -d "${TMPDIR:-/tmp}/negrep.XXXXXX")"; trap 'rm -rf "$S"' EXIT
git init -q "$S" && cp .gitignore "$S/.gitignore" && mkdir -p "$S/submodules/doc_processor/pkg/coverage"
if git -C "$S" check-ignore -q --no-index submodules/doc_processor/pkg/coverage/tracker.go; then echo "FAIL F6 git: tracker.go is ignored in the scratch repo"; rc=1
else echo "ok   F6 git: tracker.go is not ignored (negation effective)"; fi
# runner leg (review N8): the REAL runner's enumeration (scope_enumerate.js, the code that applies codegraph.json) on a scratch repository carrying the candidate
# configuration, so a drift of the runner's matcher is seen by this test; the matcher model above is only a model. Control: the OLD configuration (HEAD) lets
# the report through, proving the instrument sees it.
ENUM=constitution/scripts/codegraph/scope_enumerate.js
NODE="$(command -v node || true)"
if [ -f "$ENUM" ] && [ -n "$NODE" ] && [ -d "$HOME/.codegraph/versions/v1.6.0/lib/dist" ]; then
  enum_for() {  # enum_for <codegraph.json> -> prints the enumerated files of the scratch repo
    local E; E="$(mktemp -d "${TMPDIR:-/tmp}/negenum.XXXXXX")"
    git init -q "$E" && cp .gitignore "$E/.gitignore" && cp "$1" "$E/codegraph.json" && mkdir -p "$E/submodules/doc_processor/pkg/coverage"
    for f in tracker.go coverage.html coverage.xml lcov.info; do : > "$E/submodules/doc_processor/pkg/coverage/$f"; done
    git -C "$E" add -f -A . >/dev/null 2>&1   # the runner enumerates the git index: track the four probe files whatever .gitignore says
    "$NODE" "$ENUM" enumerate --dist "$HOME/.codegraph/versions/v1.6.0/lib/dist" --root "$E" --out "$E/.enum.out" >/dev/null 2>&1; local erc=$?
    [ "$erc" -eq 0 ] && [ -s "$E/.enum.out" ] && cat "$E/.enum.out" || echo "ENUM-FAILED rc=$erc"
    rm -rf "$E"
  }
  if git show HEAD:codegraph.json > "${TMPDIR:-/tmp}/negrep_head.$$.json" 2>/dev/null; then
    new="$(enum_for codegraph.json)"; old="$(enum_for "${TMPDIR:-/tmp}/negrep_head.$$.json")"; rm -f "${TMPDIR:-/tmp}/negrep_head.$$.json"
    case "$new" in *ENUM-FAILED*) echo "FAIL F6 runner: the enumeration did not run: $new"; rc=1 ;; *)
      if printf '%s\n' "$new" | grep -q 'pkg/coverage/tracker.go' && ! printf '%s\n' "$new" | grep -qE 'pkg/coverage/(coverage\.html|coverage\.xml|lcov\.info)'; then echo "ok   F6 runner: the real enumeration keeps tracker.go and drops the generated reports"; else echo "FAIL F6 runner: enumeration wrong: $(printf '%s' "$new" | tr '\n' ' ' | cut -c1-200)"; rc=1; fi
      if printf '%s\n' "$old" | grep -qE 'pkg/coverage/(coverage\.xml|coverage\.html)'; then echo "ok   F6 runner control: the HEAD configuration enumerates a report (instrument sees)"; else echo "SKIP F6 runner control: HEAD configuration already excluded the reports (cannot prove the instrument sees)"; skipped=1; fi ;;
    esac
  else echo "SKIP F6 runner leg: no HEAD codegraph.json to use as the control"; skipped=1; fi
else echo "SKIP F6 runner leg: scope_enumerate.js, node or the runner dist is absent"; skipped=1; fi
echo "SUMMARY rc=$rc skipped=${skipped:-0}"
[ "$rc" -eq 0 ] || exit 1
[ "${skipped:-0}" -eq 0 ] || exit 77
exit 0
