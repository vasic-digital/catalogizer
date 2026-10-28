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

# --- Literal-secret scanner (scan_literal_secrets.py): bare literal values for
# secret-named keys (case-insensitive, PASSWORD/PASSWD/PASS/...), in compose files
# including deploy/infra-compose*.yml, plus inline ${X:-literal} defaults in shell
# generators. Intentional dev/test literals are allowed ONLY via a reasoned allowlist.
SCAN=scripts/tests/scan_literal_secrets.py
ALLOW=scripts/tests/default_secrets_allowlist.txt
: > "$tmp/empty_allow"
printf 'services:\n  a:\n    environment:\n      pgadmin_default_password: hunter2\n' > "$tmp/lc.yml"
printf 'services:\n  a:\n    environment:\n      - DB_PASS=hunter2\n      - SOME_PASSWD: x1\n' > "$tmp/pass.yml"
printf 'X="${ADMIN_PASSWORD:-hunter2}"\n' > "$tmp/gen.sh"
for fx in lc.yml pass.yml; do
    python3 "$SCAN" "$tmp/empty_allow" "$tmp/$fx" >/dev/null && { echo "SELFTEST FAIL: scan blind to $fx"; exit 2; }
done
python3 "$SCAN" --defaults-only "$tmp/empty_allow" "$tmp/gen.sh" >/dev/null && { echo "SELFTEST FAIL: scan blind to shell default"; exit 2; }
printf 'services:\n  a:\n    environment:\n      A_PASSWORD: ${A_PASSWORD:?req}\n      B_TOKEN: ${B_TOKEN:-}\n      C_PASSWORD_FILE: /run/x\n' > "$tmp/ok.yml"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/ok.yml" >/dev/null || { echo "SELFTEST FAIL: scan flags clean fixture"; exit 2; }
printf 'lc.yml:pgadmin_default_password  # fixture\n' > "$tmp/al_ok"; printf 'lc.yml:pgadmin_default_password\n' > "$tmp/al_noreason"
( cd "$tmp" && python3 "$OLDPWD/$SCAN" al_ok lc.yml >/dev/null ) || { echo "SELFTEST FAIL: reasoned allowlist not honoured"; exit 2; }
( cd "$tmp" && python3 "$OLDPWD/$SCAN" al_noreason lc.yml >/dev/null 2>&1 ) && { echo "SELFTEST FAIL: reason-less allowlist accepted"; exit 2; }

fail=0
files=$(git ls-files | grep -E '(^|/)(docker-|infra-)?compose[^/]*\.ya?ml$')
[ -n "$files" ] || { echo "FAIL: no compose files found (blind)"; exit 2; }
# shellcheck disable=SC2086
hits=$(scan $files)
if [ -n "$hits" ]; then echo "FAIL: non-empty default secrets in compose files:"; echo "$hits"; fail=1; fi
[ -f "$ALLOW" ] || { echo "FAIL: allowlist $ALLOW missing"; exit 2; }
# shellcheck disable=SC2086
lit=$(python3 "$SCAN" "$ALLOW" $files 2>&1); lrc=$?
if [ "$lrc" -ne 0 ]; then echo "FAIL: literal secrets in compose files (file:line:KEY):"; echo "$lit"; fail=1; fi
gens=$(git ls-files 'scripts/*.sh' | grep -v '^scripts/tests/')
# shellcheck disable=SC2086
glit=$(python3 "$SCAN" --defaults-only "$ALLOW" $gens 2>&1 | grep -v '^STALE ALLOWLIST'); [ -z "$glit" ] || { echo "FAIL: literal secret defaults in shell scripts (file:line:KEY):"; echo "$glit"; fail=1; }
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
