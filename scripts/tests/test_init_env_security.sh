#!/usr/bin/env bash
# Executing test for scripts/lib/init_env_security.sh: the helper must create
# .env.security ONLY from .env.security.example (placeholders), mode 0600, never
# overwrite an existing file, and refuse when the target is tracked by git.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$HERE/../lib/init_env_security.sh"
fail=0
ok() { echo "ok   $1"; }
bad() { echo "FAIL $1"; fail=1; }
[ -f "$LIB" ] || { echo "FAIL: $LIB missing"; exit 1; }
# shellcheck disable=SC1090
. "$LIB"
T=$(mktemp -d ~/.cache/claude-tmp/ies.XXXXXX) || exit 2
trap 'rm -rf "${T:?}"' EXIT
cd "$T" && git init -q . || exit 2
printf 'SONAR_TOKEN=CHANGE_ME\nSNYK_TOKEN=CHANGE_ME\n' > .env.security.example

init_env_security . >/dev/null 2>&1; rc=$?
[ $rc -eq 0 ] && [ -f .env.security ] && ok "creates from example" || bad "creates from example (rc=$rc)"
cmp -s .env.security .env.security.example && ok "content == example (placeholders only)" || bad "content differs from example"
[ "$(stat -c %a .env.security)" = "600" ] && ok "mode 0600" || bad "mode not 0600"
! grep -qiE 'admin|dummy|hunter' .env.security && ok "no weak literal credentials" || bad "weak literal credential present"

echo 'SONAR_TOKEN=operator-set' > .env.security
init_env_security . >/dev/null 2>&1
grep -q operator-set .env.security && ok "never overwrites existing" || bad "overwrote existing file"

rm -f .env.security; git add -f .env.security.example; : > .env.security; git add -f .env.security
init_env_security . >/dev/null 2>&1; rc=$?
[ $rc -ne 0 ] && ok "refuses when .env.security is tracked" || bad "did not refuse tracked target"

rm -f .env.security.example; git rm -q --cached -f .env.security; rm -f .env.security
init_env_security . >/dev/null 2>&1; rc=$?
{ [ $rc -ne 0 ] && [ ! -e .env.security ]; } && ok "refuses without example (writes nothing)" || bad "wrote without example"
exit $fail
