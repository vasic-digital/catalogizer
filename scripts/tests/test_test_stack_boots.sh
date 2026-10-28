#!/usr/bin/env bash
# Test: the environment the tracked test stack (docker-compose.test.yml) hands to
# catalog-api passes boot-time validation with NO config.json (the container case).
# Resolves JWT_SECRET / ADMIN_USERNAME / ADMIN_PASSWORD from the compose file
# (literal, or ${VAR:?} filled from a freshly generated install env), exports
# them, and runs the real config.LoadConfig test. Never prints a secret value.
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "SKIP - no go toolchain (not a PASS)"; exit 0; }
tmp="$(mktemp -d)"; trap 'rm -rf "${tmp:?}"' EXIT
# shellcheck disable=SC1091
. "$ROOT/scripts/lib/install_env.sh"
install_write_env "$tmp/.env" >/dev/null || { echo "FAIL - env generation"; exit 1; }
envval() { sed -n "s/^$1=//p" "$tmp/.env" | head -1; }

C="$ROOT/docker-compose.test.yml"
declare -A got
for k in JWT_SECRET ADMIN_USERNAME ADMIN_PASSWORD; do
    raw="$(sed -n "s/^ *- $k=//p" "$C" | head -1)"
    [ -n "$raw" ] || { echo "FAIL - $k not set in $C (blind)"; exit 2; }
    if [[ "$raw" =~ ^\$\{([A-Za-z0-9_]+):\?.*\}$ ]]; then got[$k]="$(envval "${BASH_REMATCH[1]}")"
    elif [[ "$raw" =~ ^\$\{([A-Za-z0-9_]+):-(.*)\}$ ]]; then got[$k]="${BASH_REMATCH[2]}"
    else got[$k]="$raw"; fi
    [ -n "${got[$k]}" ] || { echo "FAIL - $k resolves empty"; exit 1; }
done
out="$(cd "$ROOT/catalog-api" && STACK_BOOT_CHECK=1 JWT_SECRET="${got[JWT_SECRET]}" ADMIN_USERNAME="${got[ADMIN_USERNAME]}" \
    ADMIN_PASSWORD="${got[ADMIN_PASSWORD]}" GOFLAGS=-mod=mod GOPROXY=off \
    go test ./config -run TestLoadConfig_TestStackEnv_NoConfigFile -count=1 -v 2>&1)"; rc=$?
fail=0
if [ "$rc" -eq 0 ] && grep -q -- '--- PASS: TestLoadConfig_TestStackEnv' <<<"$out"; then echo "ok   - test-stack env boots with no config.json"
else echo "FAIL - test-stack env refused at boot (rc=$rc)"; grep -E 'Error:|invalid configuration' <<<"$out" | sed -E 's/(secret|password)[^ ]*/\1<redacted>/Ig' | head -4; fail=1; fi

# The QA compose + the QA scripts must read the admin password from the environment
# (they log in to the stack above) and must refuse to run without it.
grep -q 'admin123' "$ROOT/docker-compose.qa.yml" && { echo "FAIL - docker-compose.qa.yml hardcodes a login password"; fail=1; } || echo "ok   - qa compose has no hardcoded login password"
n=$(grep -c 'ADMIN_PASSWORD=\${ADMIN_PASSWORD:?' "$ROOT/docker-compose.qa.yml"); [ "$n" -ge 2 ] && echo "ok   - qa compose passes ADMIN_PASSWORD to both QA services" || { echo "FAIL - qa compose passes ADMIN_PASSWORD to $n services (want 2)"; fail=1; }
# TI-3: the literal `admin123` grep above only knows one value. Also run the shared literal-secret
# scanner (scripts/tests/scan_literal_secrets.py, empty allowlist) over the QA compose and QA scripts,
# so any other hardcoded secret-named literal there is caught too. Control needle: a literal fixture
# must be flagged by the same invocation, so a blind scanner cannot give a clean result.
SCAN="$ROOT/scripts/tests/scan_literal_secrets.py"; : > "$tmp/empty_allow"
printf 'X_PASSWORD=hunter2\n' > "$tmp/needle.sh"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/needle.sh" >/dev/null && { echo "FAIL - scanner blind to control needle"; exit 2; }
qa_files=("$ROOT/docker-compose.qa.yml" "$ROOT/docker-compose.test.yml")
# A5: QA_USER_PASSWORD equal to ADMIN_PASSWORD must be refused (naming QA_USER_PASSWORD).
out4="$(env ADMIN_PASSWORD=same-value-x QA_USER_PASSWORD=same-value-x bash "$ROOT/scripts/run-full-qa.sh" 2>&1 </dev/null)"; r4=$?
{ [ "$r4" -ne 0 ] && grep -q 'QA_USER_PASSWORD must differ' <<<"$out4"; } && echo "ok   - run-full-qa.sh refuses QA_USER_PASSWORD == ADMIN_PASSWORD" || { echo "FAIL - run-full-qa.sh accepts QA_USER_PASSWORD == ADMIN_PASSWORD (rc=$r4)"; fail=1; }
# Aliasing guard: no line of run-full-qa.sh may be a plain alias of ADMIN_PASSWORD (whole RHS is the expansion)
# (e.g. `PW=$ADMIN_PASSWORD`, which could then feed the testuser_qa payload). Limit: indirect forms (eval, printf -v) are not seen.
# F6: alias = any assignment whose RHS BEGINS with the expansion (trailing comment / suffix included).
printf 'QPW=$ADMIN_PASSWORD # reuse\nQPW="${ADMIN_PASSWORD}x"\nQPW=${ADMIN_PASSWORD}\n' > "$tmp/alias_bad.sh"
printf 'ADMIN_PASSWORD="${ADMIN_PASSWORD:?req}"\nTOKEN=$(curl -d "${ADMIN_PASSWORD}" x)\nQ="${QA_USER_PASSWORD}"\n' > "$tmp/alias_ok.sh"
alias_scan() { grep -nE '^[[:space:]]*(export[[:space:]]+|local[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*=["'"'"']?\$\{?ADMIN_PASSWORD' "$1" | grep -vE '^[0-9]+:[[:space:]]*ADMIN_PASSWORD=' ; }
[ "$(alias_scan "$tmp/alias_bad.sh" | wc -l)" -eq 3 ] && [ -z "$(alias_scan "$tmp/alias_ok.sh")" ] || { echo "FAIL - alias scan self-test"; exit 2; }
if alias_scan "$ROOT/scripts/run-full-qa.sh" | grep -q .; then echo "FAIL - run-full-qa.sh aliases ADMIN_PASSWORD into another variable"; fail=1; else echo "ok   - run-full-qa.sh does not alias ADMIN_PASSWORD"; fi
for sc in run-full-qa run-api-validation run-helixqa run-helixqa-web run-helixqa-androidtv; do qa_files+=("$ROOT/scripts/$sc.sh"); done
scan_out="$(python3 "$SCAN" "$tmp/empty_allow" "${qa_files[@]}" 2>&1)"; scan_rc=$?
if [ "$scan_rc" -eq 0 ]; then echo "ok   - scanner finds no literal secrets in QA compose/scripts"; else echo "FAIL - literal secrets in QA compose/scripts (file:line:KEY):"; echo "$scan_out"; fail=1; fi
# PD-1: the QA user must not reuse the admin password; it needs its own required QA_USER_PASSWORD.
if grep 'auth/register.*testuser_qa' "$ROOT/scripts/run-full-qa.sh" | grep -q 'ADMIN_PASSWORD'; then echo "FAIL - run-full-qa.sh registers testuser_qa with ADMIN_PASSWORD"; fail=1; else echo "ok   - testuser_qa does not reuse ADMIN_PASSWORD"; fi
out3="$(env -u QA_USER_PASSWORD ADMIN_PASSWORD=x bash "$ROOT/scripts/run-full-qa.sh" 2>&1 </dev/null)"; r3=$?
{ [ "$r3" -ne 0 ] && grep -q 'QA_USER_PASSWORD' <<<"$out3"; } && echo "ok   - run-full-qa.sh refuses to run without QA_USER_PASSWORD" || { echo "FAIL - run-full-qa.sh runs without QA_USER_PASSWORD (rc=$r3)"; fail=1; }
for sc in run-full-qa run-api-validation run-helixqa run-helixqa-web run-helixqa-androidtv; do
    f="$ROOT/scripts/$sc.sh"
    if grep -q 'admin123' "$f"; then echo "FAIL - $sc.sh hardcodes a password"; fail=1; continue; fi
    out2="$(env -u ADMIN_PASSWORD bash "$f" 2>&1 </dev/null)"; r2=$?
    { [ "$r2" -ne 0 ] && grep -q 'ADMIN_PASSWORD' <<<"$out2"; } && echo "ok   - $sc.sh refuses to run without ADMIN_PASSWORD" || { echo "FAIL - $sc.sh runs without ADMIN_PASSWORD (rc=$r2)"; fail=1; }
done
[ "$fail" -eq 0 ] && echo PASS || echo FAILED
exit $fail
