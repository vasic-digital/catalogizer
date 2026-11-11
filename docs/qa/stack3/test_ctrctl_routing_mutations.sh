#!/usr/bin/env bash
# Mutants for test_ctrctl_routing.sh: each reintroduces a raw podman call or bypasses the helper; the routing test must FAIL on every one.
set -uo pipefail
unset GOLDEN_UPDATE
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ctrmut.XXXXXX")"; : "${ROOT:?}"; trap 'rm -rf -- "${ROOT:?}"' EXIT
pass=0; fail=0
mut() { # <name> <file> <sed-expr>
  local d="$ROOT/$1/stack3"; mkdir -p "$d"; cp -r "$HERE/../lib" "$ROOT/$1/lib"; ( cd "$HERE" && cp -r lib.sh up.sh sonarqube "$d/"; ln -s "$HERE/golden" "$ROOT/$1/golden" ); sed -i -E "$3" "$d/$2"
  cmp -s "$HERE/$2" "$d/$2" && { echo "FAIL $1 (mutation did not apply)"; fail=$((fail+1)); return; }
  if bash "$HERE/test_ctrctl_routing.sh" --root "$d" >/dev/null 2>&1; then echo "FAIL $1 SURVIVED"; fail=$((fail+1)); else echo "PASS $1 killed"; pass=$((pass+1)); fi
}
mut M1-api-raw-run        up.sh            's/^ctr_up "\$\{STACK\}-api" /podman run -d --pod "$POD" --name "${STACK}-api" /'
mut M2-sonar-raw-run      sonarqube/up.sh  's/^  ctr_up "\$\{STACK\}-sonar" /  podman run -d --pod "$POD" --name "${STACK}-sonar" /'
mut M3-lib-raw-start      lib.sh           's/qa_stack_start "\$n"/podman start "$n"/'
mut M4-sonarlib-raw-start sonarqube/lib.sh 's/qa_stack_start "\$n"/podman start "$n"/'
mut M5-lib-drops-failure  lib.sh           's/qa_stack_start "\$n" \|\| \{[^}]*\}/qa_stack_start "$n" || true/'
mut M6-up-no-ctr_up       up.sh            's/ctr_up /echo /'
# FU-6..11 mutants
mut N1-detach-flag-after-image up.sh           '$a podman run --pod "$POD" --name x img -d'
mut N2-container-start         up.sh           '$a podman container start x'
mut N3-create-failure-ignored  lib.sh          's/(podman create --pod "\$POD" --name "\$n" "\$@" >\/dev\/null) \|\| return 1/\1 || true/'
mut N5-sonar-create-ignored    sonarqube/lib.sh 's/(podman create --pod "\$POD" --name "\$n" "\$@" >\/dev\/null) \|\| return 1/\1 || true/'
mut N4-restart-dropped         up.sh           's/ --restart on-failure:3//'
mut N6-redis-health-dropped    up.sh           "s/--health-cmd 'redis-cli ping \| grep -q PONG' //"
mut N7-name-guard-removed      lib.sh          '/_qa_stack_check_name "\$n"/d'
mut N8-sonar-name-guard-removed sonarqube/lib.sh '/_qa_stack_check_name "\$n"/d'
mut N9-backend-cache-off       lib.sh          's/_QA_STACK_BACKEND_CACHE="\$b"; /_QA_STACK_BACKEND_CACHE=""; /'
mut N10-sonar-backend-cache-off sonarqube/lib.sh 's/_QA_STACK_BACKEND_CACHE="\$b"; /_QA_STACK_BACKEND_CACHE=""; /'
# round-1 review mutants (F1/F3/F4/F6)
mut R1-cache-reset-removed       lib.sh           '/^unset _QA_STACK_BACKEND_CACHE/d'
mut R1b-sonar-cache-reset-removed sonarqube/lib.sh '/^unset _QA_STACK_BACKEND_CACHE/d'
mut R4-cache-key-ignored         lib.sh           's/ && \[ "\$\{_QA_STACK_BACKEND_CACHE_KEY:-\}" = "\$\(_qa_stack_cache_key\)" \]//'
mut R4b-sonar-cache-key-ignored  sonarqube/lib.sh 's/ && \[ "\$\{_QA_STACK_BACKEND_CACHE_KEY:-\}" = "\$\(_qa_stack_cache_key\)" \]//'
mut R6-failure-cached            lib.sh           's/\[\[ "\$b" == containers\\ \* \]\]/true/'
mut R6b-sonar-failure-cached     sonarqube/lib.sh 's/\[\[ "\$b" == containers\\ \* \]\]/true/'
mut R7-none-not-refused          lib.sh           's/\[ "\$b" = none \] && \{/false \&\& {/'
mut R7b-sonar-none-not-refused   sonarqube/lib.sh 's/\[ "\$b" = none \] && \{/false \&\& {/'
mut R2-pod-start                 up.sh            '$a podman pod start "$POD"'
mut R3-restart                   up.sh            '$a podman restart "${STACK}-api"'
mut R5-root-derivation           lib.sh           '/_qa_stack_cache_key()/s#/\.\./\.\./\.\.#/../..#'
mut R5b-sonar-root-derivation    sonarqube/lib.sh '/_qa_stack_cache_key()/s#/\.\./\.\./\.\.#/../..#'
# F2: GOLDEN_UPDATE must be refused with --root and must never read as a pass after regenerating (run on a COPY, the real golden is never touched)
f2check() { # <label> <sed-expr or ''>   -> 0 when the F2 protections hold
  local d="$ROOT/f2$1"; mkdir -p "$d"; cp -r "$HERE/../lib" "$d/lib"; mkdir -p "$d/stack3"; ( cd "$HERE" && cp -r lib.sh up.sh sonarqube golden test_ctrctl_routing.sh "$d/stack3/" )
  [ -n "$2" ] && sed -i -E "$2" "$d/stack3/test_ctrctl_routing.sh"
  local r1 r2; echo "# stale" >> "$d/stack3/golden/up_create_argv.txt"
  GOLDEN_UPDATE=1 bash "$d/stack3/test_ctrctl_routing.sh" --root "$d/stack3" >/dev/null 2>"$d/err1"; r1=$?
  [ "$r1" = 2 ] && grep -q 'refused together with --root' "$d/err1" && grep -q '^# stale' "$d/stack3/golden/up_create_argv.txt" || return 1   # refused, golden untouched
  GOLDEN_UPDATE=1 bash "$d/stack3/test_ctrctl_routing.sh" >"$d/out" 2>"$d/err"; r2=$?
  [ "$r1" != 0 ] && [ "$r2" != 0 ] && grep -q 'golden regenerated, review the diff' "$d/err" && ! grep -q '^# stale' "$d/stack3/golden/up_create_argv.txt"
}
f2check ok '' && { echo "PASS F2 protections hold on the real test"; pass=$((pass+1)); } || { echo "FAIL F2 protections"; fail=$((fail+1)); }
f2check m1 's/GOLDEN_UPDATE:-0\}" = 1 \] && \{ echo "ERROR/GOLDEN_UPDATE:-0}" = 9 ] \&\& { echo "ERROR/' && { echo "FAIL F2-M1 root-refusal removed SURVIVED"; fail=$((fail+1)); } || { echo "PASS F2-M1 root-refusal removed killed"; pass=$((pass+1)); }
f2check m2 's/exit 3; fi/exit 0; fi/' && { echo "FAIL F2-M2 update-exit-zero SURVIVED"; fail=$((fail+1)); } || { echo "PASS F2-M2 update-exit-zero killed"; pass=$((pass+1)); }
echo "pass=$pass fail=$fail"; [ "$fail" = 0 ]
