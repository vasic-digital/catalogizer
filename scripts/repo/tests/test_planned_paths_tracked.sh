#!/usr/bin/env bash
# T004 test: planned tracked deliverables must NOT be ignored; control paths MUST stay ignored.
# Oracle: `git check-ignore -q --no-index <path>` exit status only (never -v: it prints negation matches).
# Usage: scripts/repo/tests/test_planned_paths_tracked.sh   (run from anywhere inside the repo)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
EV=specs/001-full-project-audit-remediation/evidence
planned=(
  tools/evidence/evrec tools/evidence/matrix/x tools/evidence/analyzers/x tools/perf/x
  tools/audit/rust_ast/Cargo.toml tools/audit/rust_ast/Cargo.lock
  build/containers/images.lock.yaml build/containers/go/Containerfile
  build/containers/testutil/Containerfile build/containers/testutil/requirements.txt
  build/containers/kcov/Containerfile build/components.json build/hosts.env.example
  coverage/exclusions/catalog-api.yaml
  scripts/build/verify_artifact.sh scripts/build/tests/test_verify_artifact.sh
  scripts/build/dispatch.sh scripts/build/event_hub.sh scripts/build/callbacks.tsv
  scripts/build/remote/x scripts/build/tests/test_dispatch_events.sh
  scripts/coverage/check_exclusions.sh scripts/coverage/tests/test_check_exclusions.sh
  "$EV/coverage_baseline/catalog-api/baseline.json"
  "$EV/coverage_baseline/catalog-web/baseline.json"
  "$EV/coverage_baseline/catalog-api/exclusions-mutation.txt"
  docs/workable_items.db config/index/scope.yaml config/index/lumen_scope.json
  scripts/repo/host_entry/cpa-host scripts/repo/host_entry/INSTALL.md
  scripts/audit/api/probe scripts/security/dast/probe scripts/qa/tests/probe
)
control=(
  .env build/hosts.env build/output/x.bin build/containers/go/x.bin tools/jdk/x
  tools/evidence/.cache tools/audit/rust_ast/target/x
  docs/workable_items.db-wal docs/workable_items.db-shm docs/workable_items.db-journal
  docs/workable_items.db.bak-20260101T000000Z docs/.register.lock
  .audit/longops/x .audit/commit-push/x/report.json .audit/pending_pins.tsv
  .audit/commit_turn.json .audit/out/x/y.json .audit/out/commit-turn-reap/x.json
  .audit/merge-resolution/x/y
  coverage/lcov.info coverage/other/x tools/audit/other/x tools/foo
  build/containers/hosts.env build/containers/sub/hosts.env tools/evidence/sub/.cache/x
)
# N9: secrets and caches stay ignored inside every negated tree (blanket ignore was lost when the tree was negated)
trees=(build/containers tools/evidence tools/perf tools/audit/rust_ast coverage/exclusions scripts/build scripts/coverage)
secret_names=(id_rsa id_rsa.pub id_rsa_old id_rsa2 id_ecdsa id_dsa x.key x.pem x.p12 credentials.json .npmrc secrets.env __pycache__/x.pyc .venv/pyvenv.cfg .pytest_cache/x)
for t in "${trees[@]}"; do for n in "${secret_names[@]}"; do control+=("$t/sub/$n"); done
  # W6: example/template siblings stay trackable INSIDE the negated trees too
  planned+=("$t/sub/id_rsa.example" "$t/sub/id_ed25519.sample" "$t/sub/id_ecdsa.template" "$t/sub/id_dsa.example"); done
# Repo-wide secret-like files (root and nested) must be ignored; example/template siblings must stay trackable.
secret_pats=(id_rsa id_rsa.pub id_rsa_old id_rsa2 id_rsa.old id_ecdsa id_ecdsa.pub id_dsa id_dsa.pub id_ed25519 id_ed25519.pub x.key x.pem x.p12 x.pfx credentials.json .npmrc secrets.env .netrc x.kdbx)
for n in "${secret_pats[@]}"; do control+=("$n" "some/nested/dir/$n"); done
planned+=(id_rsa.example nested/id_ed25519.example server.pem.example nested/x.key.example credentials.json.example
  nested/.npmrc.example secrets.env.example .netrc.example nested/x.kdbx.example nested/x.p12.example x.pfx.example id_rsa.sample nested/id_rsa.template id_ed25519.sample nested/id_ed25519.template id_ecdsa.example nested/id_dsa.sample id_ecdsa.template id_ecdsa.sample nested/id_dsa.template id_dsa.example)
# Review r3 I5: Python bytecode and tool caches are ignored repo-wide, at the root, outside the 7 negated trees and inside them. The file-only
# paths (x.pyc outside a cache dir, a non-.pyc file inside __pycache__/) tell the two rule kinds apart, so removing either one is caught.
control+=(scripts/audit/__pycache__/org_of.cpython-314.pyc scripts/audit/__pycache__/x.txt __pycache__/x.txt x.pyc nested/y.pyo a/b/z.pyd scripts/audit/x.pyc
  .venv/pyvenv.cfg scripts/audit/.venv/lib/x .pytest_cache/x scripts/audit/.pytest_cache/x .mypy_cache/x scripts/audit/.mypy_cache/y .ruff_cache/x scripts/audit/.ruff_cache/y
  docs/x/.venv/pyvenv.cfg)
# Review r3 M6 (Gc): every example/template negation is probed at the root AND nested, so an anchored copy of it (!/id_ecdsa.sample) is caught
for k in id_rsa id_ecdsa id_dsa id_ed25519; do for e in example sample template; do planned+=("$k.$e" "deep/er/$k.$e"); done; done
# run_checks <git work tree dir>: prints PASS/FAIL lines, sets RC_FAIL (number of failures)
run_checks() {
  local d="$1" p rc; RC_FAIL=0
  for p in "${planned[@]}"; do
    (cd "$d" && git -c core.excludesFile=/dev/null check-ignore -q --no-index "$p"); rc=$?
    if [ "$rc" -eq 1 ]; then echo "PASS planned not-ignored: $p"; else echo "FAIL planned $p rc=$rc (want 1)"; RC_FAIL=$((RC_FAIL+1)); fi
  done
  for p in "${control[@]}"; do
    (cd "$d" && git -c core.excludesFile=/dev/null check-ignore -q --no-index "$p"); rc=$?
    if [ "$rc" -eq 0 ]; then echo "PASS control ignored: $p"; else echo "FAIL control $p rc=$rc (want 0)"; RC_FAIL=$((RC_FAIL+1)); fi
  done
}
fail=0
run_checks "$PWD"; fail=$RC_FAIL
echo "TOTAL planned=${#planned[@]} control=${#control[@]} failures=$fail"

# Paired mutations (review r2 G1-G5 and every other rule line of the T004 block): a scratch repository holds a copy of
# .gitignore with exactly one rule line removed; the checks run there and MUST fail. Baseline (unmutated copy) must pass.
# Lines proven equivalent (their effect is already provided by another rule) are listed with the reason and expected to survive.
equivalent=(
  "/tools/audit/rust_ast/target/|the repo-wide target/ rule already ignores it (r2 G5)"
  "/tools/evidence/.cache|also ignored by /tools/evidence/**/.cache (N9 rule), the two rules overlap"
)
SCR="$(mktemp -d "${TMPDIR:-/tmp}/gitignore-mut.XXXXXX")"; trap 'rm -rf "$SCR"' EXIT
BEGIN_MARK='# BEGIN helix-t004-repo-wide-secrets'; END_MARK='# END helix-t004-repo-wide-secrets'
# sweep <gitignore file> <label>: mutates every rule line strictly BETWEEN the BEGIN and END markers (a later block, for example
# T019's `# BEGIN helix-codegraph-scope`, is outside the sweep); sets SW_FAIL SW_TOTAL SW_CAUGHT SW_EQUIV; prints the per-line results.
sweep() {
  local gi="$1" label="$2" b e line m mf eq
  SW_FAIL=0; SW_TOTAL=0; SW_CAUGHT=0; SW_EQUIV=0
  b="$(grep -n "^$BEGIN_MARK" "$gi" | head -1 | cut -d: -f1)"; e="$(grep -n "^$END_MARK" "$gi" | head -1 | cut -d: -f1)"
  if [ -z "$b" ] || [ -z "$e" ] || [ "$e" -le "$b" ]; then echo "FAIL [$label] BEGIN/END markers not found or out of order in $gi"; SW_FAIL=$((SW_FAIL+1)); return; fi
  git init -q "$SCR/base"; cp "$gi" "$SCR/base/.gitignore"
  run_checks "$SCR/base" >/dev/null
  echo "[$label] scratch-repo baseline (unmutated copy): failures=$RC_FAIL"
  [ "$RC_FAIL" -eq 0 ] || { echo "FAIL [$label] baseline in scratch repo is not clean; mutation results would be meaningless"; SW_FAIL=$((SW_FAIL+1)); }
  while IFS= read -r line; do
    case "$line" in ''|'#'*) continue;; esac
    SW_TOTAL=$((SW_TOTAL+1))
    m="$SCR/m"; rm -rf "$m"; git init -q "$m"
    awk -v ln="$line" -v from="$b" -v to="$e" 'NR>from && NR<to && $0==ln && !done {done=1; next} {print}' "$gi" >"$m/.gitignore"
    if cmp -s "$m/.gitignore" "$gi"; then echo "FAIL mutation not applied: $line"; SW_FAIL=$((SW_FAIL+1)); continue; fi
    run_checks "$m" >"$SCR/m.out"; mf=$RC_FAIL
    eq=""; for e2 in "${equivalent[@]}"; do [ "${e2%%|*}" = "$line" ] && eq="${e2#*|}"; done
    if [ "$mf" -gt 0 ] && [ -z "$eq" ]; then SW_CAUGHT=$((SW_CAUGHT+1)); echo "PASS mutation removing '$line' caught ($mf check(s) red)"
    elif [ "$mf" -eq 0 ] && [ -n "$eq" ]; then SW_EQUIV=$((SW_EQUIV+1)); echo "PASS mutation removing '$line' survives as a documented equivalent: $eq"
    elif [ "$mf" -gt 0 ]; then echo "FAIL '$line' is listed equivalent but its removal is caught; fix the list"; SW_FAIL=$((SW_FAIL+1))
    else echo "FAIL mutation removing '$line' SURVIVED (suite stayed green)"; SW_FAIL=$((SW_FAIL+1)); fi
  done < <(sed -n "$((b+1)),$((e-1))p" "$gi")
}
sweep .gitignore real; fail=$((fail+SW_FAIL))
echo "MUTATIONS rule-lines=$SW_TOTAL caught=$SW_CAUGHT documented-equivalent=$SW_EQUIV"
REAL_TOTAL=$SW_TOTAL
# Review r3 (Gc, Gf): ANCHOR mutations. Every unanchored rule line (no slash except a trailing one) is rewritten with a leading slash, so it only matches
# at the root: `!id_ecdsa.sample` -> `!/id_ecdsa.sample`, `.npmrc` -> `/.npmrc`, `__pycache__/` -> `/__pycache__/`. The nested probes must catch every one of them.
anchor_sweep() { # $1 gitignore file; sets AS_FAIL AS_TOTAL AS_CAUGHT; prints per-line results
  local gi="$1" b e line m new
  AS_FAIL=0; AS_TOTAL=0; AS_CAUGHT=0
  b="$(grep -n "^$BEGIN_MARK" "$gi" | head -1 | cut -d: -f1)"; e="$(grep -n "^$END_MARK" "$gi" | head -1 | cut -d: -f1)"
  while IFS= read -r line; do
    case "$line" in ''|'#'*|/*|'!/'*) continue;; esac
    [[ "${line%/}" == */* ]] && continue
    AS_TOTAL=$((AS_TOTAL+1))
    case "$line" in '!'*) new="!/${line#!}";; *) new="/$line";; esac
    m="$SCR/ma"; rm -rf "$m"; git init -q "$m"
    awk -v ln="$line" -v nw="$new" -v from="$b" -v to="$e" 'NR>from && NR<to && $0==ln && !done {done=1; print nw; next} {print}' "$gi" >"$m/.gitignore"
    if cmp -s "$m/.gitignore" "$gi"; then echo "FAIL anchor mutation not applied: $line"; AS_FAIL=$((AS_FAIL+1)); continue; fi
    run_checks "$m" >"$SCR/ma.out"
    if [ "$RC_FAIL" -gt 0 ]; then AS_CAUGHT=$((AS_CAUGHT+1)); echo "PASS anchor mutation '$line' -> '$new' caught ($RC_FAIL check(s) red)"
    else echo "FAIL anchor mutation '$line' -> '$new' SURVIVED (suite stayed green)"; AS_FAIL=$((AS_FAIL+1)); fi
  done < <(sed -n "$((b+1)),$((e-1))p" "$gi")
}
anchor_sweep .gitignore; fail=$((fail+AS_FAIL))
echo "ANCHOR-MUTATIONS rule-lines=$AS_TOTAL caught=$AS_CAUGHT"
[ "$AS_TOTAL" -ge 30 ] || { echo "FAIL anchor sweep covered only $AS_TOTAL rule lines (expected at least 30): the sweep itself is broken"; fail=$((fail+1)); }
# G-R2 (r3 W7): a later block shaped like T019's scope_render.py --write output, appended AFTER the END marker with negations
# in its body, must NOT change the sweep (same rule-line count, no false SURVIVED).
T019="$SCR/gitignore.t019"; cp .gitignore "$T019"
printf '%s\n' '# BEGIN helix-codegraph-scope' '/third_party_x/' '!/third_party_x/keep_me.txt' '/vendor_y/*' '!/vendor_y/ours/' '# END helix-codegraph-scope' >>"$T019"
sweep "$T019" t019-appended >"$SCR/t019.out"; t019_fail=$SW_FAIL
if [ "$t019_fail" -eq 0 ] && [ "$SW_TOTAL" -eq "$REAL_TOTAL" ]; then echo "PASS T019-shaped block appended after END marker: sweep unchanged (rule-lines=$SW_TOTAL, no false SURVIVED)"
else echo "FAIL T019-shaped block appended after END marker changed the sweep (failures=$t019_fail rule-lines=$SW_TOTAL want $REAL_TOTAL)"; grep '^FAIL' "$SCR/t019.out" | head -5; fail=$((fail+1)); fi
echo "TOTAL planned=${#planned[@]} control=${#control[@]} failures=$fail"
[ "$fail" -eq 0 ]
