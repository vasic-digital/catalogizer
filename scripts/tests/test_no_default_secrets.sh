#!/usr/bin/env bash
# Guard: no tracked compose file may give a PASSWORD/SECRET/TOKEN variable a
# non-empty `:-default`, and .env.security must not be tracked. Never prints
# secret values: reports file:line and variable name only.
# Control needle: the scanner is first run on a synthetic fixture that must
# be flagged, and on a clean fixture that must not (§11.4.201 / §11.4.273).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
# Covers `${X:-v}` and the colon-less `${X-v}` form, for names containing
# PASSWORD/SECRET/TOKEN/API_KEY/ACCESS_KEY/PRIVATE_KEY/CREDENTIAL.
PATTERN='\$\{[A-Za-z0-9_]*(PASSWORD|SECRET|TOKEN|API_KEY|ACCESS_KEY|PRIVATE_KEY|CREDENTIAL)[A-Za-z0-9_]*:?-[^}]+\}'

scan() { grep -nE "$PATTERN" "$@" 2>/dev/null | sed -E 's/:?-[^}]*\}/-<redacted>}/'; }

tmp=$(mktemp -d); trap 'rm -rf "${tmp:?}"' EXIT
# One bad fixture per form: each MUST be flagged on its own (kills a scanner
# that only knows `:-` or only PASSWORD/SECRET/TOKEN names).
i=0
for bad in '${MY_PASSWORD:-hunter2}' '${STORAGE_API_KEY:-fixture}' '${AWS_ACCESS_KEY:-AKIAFAKE}' \
           '${MY_PASSWORD-hunter2}' '${DB_SECRET:-s3}' '${GH_TOKEN-t}'; do
    i=$((i+1)); printf 'x: %s\n' "$bad" > "$tmp/bad$i.yml"
    [ -n "$(scan "$tmp/bad$i.yml")" ] || { echo "SELFTEST FAIL: scanner blind to bad fixture $i"; exit 2; }
done
printf 'x: ${MY_PASSWORD:?required}\ny: ${REDIS_PASSWORD:-}\nz: ${SNYK_TOKEN-}\nw: ${LOG_LEVEL:-info}\n' > "$tmp/good.yml"
[ -z "$(scan "$tmp/good.yml")" ] || { echo "SELFTEST FAIL: scanner flags clean fixture"; exit 2; }

fail=0
files=$(git ls-files | grep -E '(^|/)(docker-)?compose[^/]*\.ya?ml$|(^|/)docker-compose[^/]*\.ya?ml$')
[ -n "$files" ] || { echo "FAIL: no compose files found (blind)"; exit 2; }
# shellcheck disable=SC2086
hits=$(scan $files)
if [ -n "$hits" ]; then echo "FAIL: non-empty default secrets in compose files:"; echo "$hits"; fail=1; fi
if git ls-files --error-unmatch .env.security >/dev/null 2>&1; then echo "FAIL: .env.security is tracked"; fail=1; fi
git check-ignore -q .env.security || { echo "FAIL: .env.security not gitignored"; fail=1; }
[ -f .env.security.example ] || { echo "FAIL: .env.security.example missing"; fail=1; }
# SNYK_TOKEN must stay OPTIONAL (empty default) so the skip-Snyk path keeps working.
grep -qF 'SNYK_TOKEN: ${SNYK_TOKEN:-}' docker-compose.security.yml || { echo "FAIL: SNYK_TOKEN must be optional (\${SNYK_TOKEN:-}) in docker-compose.security.yml"; fail=1; }
# Every `${VAR:?...}` in tracked compose files must be documented in .env.example
# (as KEY= or "# KEY=") or be emitted by the test-infra generators. Names only.
# shellcheck disable=SC2086
req=$(grep -hoE '\$\{[A-Za-z0-9_]+:\?' $files | tr -d '${:?' | sort -u)
[ -n "$req" ] || { echo "FAIL: no :? variables found (blind)"; exit 2; }
for v in $req; do
    grep -qE "^#?[[:space:]]*$v=" .env.example && continue
    grep -rqE "\b$v=" scripts/test-infra 2>/dev/null && continue
    echo "FAIL: required variable $v is not documented in .env.example"; fail=1
done
[ "$fail" -eq 0 ] && echo "PASS: no default secrets, .env.security untracked+ignored, example present"
exit $fail
