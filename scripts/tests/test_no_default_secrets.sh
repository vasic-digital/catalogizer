#!/usr/bin/env bash
# Guard: no tracked compose file may give a PASSWORD/SECRET/TOKEN variable a
# non-empty `:-default`, and .env.security must not be tracked. Never prints
# secret values: reports file:line and variable name only.
# Control needle: the scanner is first run on a synthetic fixture that must
# be flagged, and on a clean fixture that must not (§11.4.201 / §11.4.273).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
PATTERN='\$\{[A-Z0-9_]*(PASSWORD|SECRET|TOKEN)[A-Z0-9_]*:-[^}]+\}'

scan() { grep -nE "$PATTERN" "$@" 2>/dev/null | sed -E 's/:-[^}]*\}/:-<redacted>}/'; }

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
printf 'x: ${MY_PASSWORD:-hunter2}\n' > "$tmp/bad.yml"
printf 'x: ${MY_PASSWORD:?required}\ny: ${REDIS_PASSWORD:-}\n' > "$tmp/good.yml"
[ -n "$(scan "$tmp/bad.yml")" ]  || { echo "SELFTEST FAIL: scanner blind to bad fixture"; exit 2; }
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
[ "$fail" -eq 0 ] && echo "PASS: no default secrets, .env.security untracked+ignored, example present"
exit $fail
