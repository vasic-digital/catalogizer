#!/usr/bin/env bash
# Mutants for test_ctrctl_routing.sh: each reintroduces a raw podman call or bypasses the helper; the routing test must FAIL on every one.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ctrmut.XXXXXX")"; : "${ROOT:?}"; trap 'rm -rf -- "${ROOT:?}"' EXIT
pass=0; fail=0
mut() { # <name> <file> <sed-expr>
  local d="$ROOT/$1/stack3"; mkdir -p "$d"; cp -r "$HERE/../lib" "$ROOT/$1/lib"; ( cd "$HERE" && cp -r lib.sh up.sh sonarqube "$d/" ); sed -i -E "$3" "$d/$2"
  cmp -s "$HERE/$2" "$d/$2" && { echo "FAIL $1 (mutation did not apply)"; fail=$((fail+1)); return; }
  if bash "$HERE/test_ctrctl_routing.sh" --root "$d" >/dev/null 2>&1; then echo "FAIL $1 SURVIVED"; fail=$((fail+1)); else echo "PASS $1 killed"; pass=$((pass+1)); fi
}
mut M1-api-raw-run        up.sh            's/^ctr_up "\$\{STACK\}-api" /podman run -d --pod "$POD" --name "${STACK}-api" /'
mut M2-sonar-raw-run      sonarqube/up.sh  's/^  ctr_up "\$\{STACK\}-sonar" /  podman run -d --pod "$POD" --name "${STACK}-sonar" /'
mut M3-lib-raw-start      lib.sh           's/qa_stack_start "\$n"/podman start "$n"/'
mut M4-sonarlib-raw-start sonarqube/lib.sh 's/qa_stack_start "\$n"/podman start "$n"/'
mut M5-lib-drops-failure  lib.sh           's/qa_stack_start "\$n" \|\| \{[^}]*\}/qa_stack_start "$n" || true/'
mut M6-up-no-ctr_up       up.sh            's/ctr_up /echo /'
echo "pass=$pass fail=$fail"; [ "$fail" = 0 ]
