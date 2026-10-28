#!/usr/bin/env bash
# Guard: no tracked compose file may give a PASSWORD/SECRET/TOKEN variable a
# non-empty `:-default`, and .env.security must not be tracked. Never prints
# secret values: reports file:line and variable name only.
# Control needle: the scanner is first run on a synthetic fixture that must
# be flagged, and on a clean fixture that must not (§11.4.201 / §11.4.273).
set -u
SELF="$(readlink -f "${BASH_SOURCE[0]}")"   # absolute BEFORE the cd (R2: runnable from any subdirectory)
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
# Fixtures that earlier scanner versions missed (each MUST be flagged):
#  M1 heredoc KEY=value literals inside a generator script (old full-distribute/install.sh shape),
#  M2 an all-digit literal (a counter is at most 4 digits), a compose `$$`-escaped literal,
#  flow-map `{KEY: x}`, and shell `export K=v` / `-e K=v`.
printf 'cat > .env <<EOF\nJWT_SECRET=distributed-catalogizer-secret\nADMIN_PASSWORD=admin123\nEOF\n' > "$tmp/m1.sh"
printf 'services:\n  a:\n    environment:\n      POSTGRES_PASSWORD: 12345678\n' > "$tmp/m2.yml"
printf 'services:\n  a:\n    environment:\n      D_PASSWORD: $$ecret1\n' > "$tmp/esc.yml"
printf 'services:\n  a:\n    environment: {D_PASSWORD: hunter2}\n' > "$tmp/flow.yml"
printf 'export DB_PASSWORD=hunter2\ndocker run -e POSTGRES_PASSWORD=hunter2 img\n' > "$tmp/inl.sh"
# T2: `*_TOKEN=literal` in shell (bare, exported, -e) and quoted lowercase secret keys in JSON text inside scripts.
printf 'GH_TOKEN=ghp_fakefixture123\nexport NPM_TOKEN=npm_fake1\ndocker run -e API_TOKEN=abc123x img\n' > "$tmp/tok.sh"
printf 'curl -d '"'"'{"admin_password":"admin123"}'"'"' http://x\nB="{\\"password\\":\\"hunter2x\\"}"\n' > "$tmp/json.sh"
for fx in m1.sh m2.yml esc.yml flow.yml inl.sh tok.sh json.sh; do
    python3 "$SCAN" "$tmp/empty_allow" "$tmp/$fx" >/dev/null && { echo "SELFTEST FAIL: scan blind to $fx"; exit 2; }
done
# ...and these clean shapes MUST NOT be flagged (counters, PID, command substitution, refs).
printf 'PASS=0\nPASSES=$((PASSES+1))\nTMP_PASSWORD=$(openssl rand -hex 8)\nX_SECRET=$$\nY_PASSWORD="${Y_PASSWORD:?req}"\nexport Z_TOKEN="${Z_TOKEN:-}"\n' > "$tmp/okc.sh"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/okc.sh" >/dev/null || { echo "SELFTEST FAIL: scan flags clean shell fixture"; exit 2; }
printf 'TOKEN=0\nTOKENS=3\nexport Z_TOKEN="${Z_TOKEN:?req}"\nGH_TOKEN="$(cat f)"\nB="{\\"password\\":\\"${PW}\\"}"\nc='"'"'{"secrets": ['"'"'\nVOL=api_keys:/app/keys\n' > "$tmp/okt.sh"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/okt.sh" >/dev/null || { echo "SELFTEST FAIL: scan flags clean T2 shell fixture"; exit 2; }
# TI-1: value-prefix literals (`/ [ { <`) that earlier versions treated as paths/placeholders.
# Each of the six MUST be flagged on its own; the must-not-flag set below keeps real paths
# (multi-segment absolute path, short-volume `name:/abs`), JSON openers and keyword placeholders clean.
ti=0
for bad in 'JWT_SECRET: /k7QzXw9pLm2' 'ADMIN_PASSWORD=[hunter2]' 'export DB_PASSWORD=/s3cr3t' \
           'API_TOKEN: {hunter2}' 'DB_PASSWORD=<hunter2>' 'ENCRYPTION_SECRET: /9f8e7d6c5b4a' \
           'GITHUB_TOKEN_ENV: ghp_fakefixture123' 'SECRET_ENV: hunter2' 'X_PASSWORD: x:/hunter2' 'a_password: a:/b' \
           'DB_PASSWORD: <password>hunter2' 'API_SECRET: <secretXYZ123>' \
           'POSTGRES_PASSWORD: 1234' 'DB_PASSWORD: "0000"' 'DB_PASSWORD: 1234 abcd' \
           'API_TOKEN_ENV: HUNTER2SECRET' 'AWS_ACCESS_KEY_ENV: AKIAIOSFODNN7EXAMPLE' 'X="${API_TOKEN_ENV:-hunter2}"' 'X="${AWS_SECRET_ENV:-ABCD1234EFGH5678}"' \
           'DB_PASSWORD: ${PREFIX}hunter2' 'API_TOKEN_ENV: AKIA_IOSFODNN7EXAMPLE' \
           'SIGN_SECRET: /k7Qz/Xw9pL' 'SIGN_KEY_SECRET: /k7Qz+Xw9/pL2==' 'B_SECRET: /ab/cd/ef=' ; do
    ti=$((ti+1)); ext=yml; case "$bad" in export*|X=*) ext=sh;; esac
    printf '%s\n' "$bad" > "$tmp/ti$ti.$ext"
    python3 "$SCAN" "$tmp/empty_allow" "$tmp/ti$ti.$ext" >/dev/null && { echo "SELFTEST FAIL: scan blind to TI-1 literal $ti"; exit 2; }
done
printf 'VOL=api_keys:/app/keys\nJWT_SECRET: /run/secrets/jwt\nDB_PASSWORD: <your-password>\nAPI_TOKEN: {}\nSECRET_X: [\nSIGN_SECRET: {"a": 1}\nTLS_SECRET: key_vol:/etc/tls\nX_SECRET: ${SECRETS}/db\n  needs: {service: x, credential_env: [QA_USER]}\n' > "$tmp/ti_ok.yml"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/ti_ok.yml" >/dev/null || { echo "SELFTEST FAIL: scan flags TI-1 clean fixture"; exit 2; }
# R3: an expansion followed by a space-separated command is not a literal tail
printf 'SECRET="${ref}" cmd args\nTOKEN_X=$ref other\n' > "$tmp/r3_ok.sh"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/r3_ok.sh" >/dev/null || { echo "SELFTEST FAIL: scan flags 'SECRET=\"\${ref}\" cmd'"; exit 2; }
printf 'services:\n  a:\n    command: TOKEN=$$(curl x)\n' > "$tmp/okc.yml"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/okc.yml" >/dev/null || { echo "SELFTEST FAIL: scan flags compose command substitution"; exit 2; }
printf 'services:\n  a:\n    environment:\n      A_PASSWORD: ${A_PASSWORD:?req}\n      B_TOKEN: ${B_TOKEN:-}\n      C_PASSWORD_FILE: /run/x\n' > "$tmp/ok.yml"
python3 "$SCAN" "$tmp/empty_allow" "$tmp/ok.yml" >/dev/null || { echo "SELFTEST FAIL: scan flags clean fixture"; exit 2; }
printf 'lc.yml:pgadmin_default_password  # fixture\n' > "$tmp/al_ok"; printf 'lc.yml:pgadmin_default_password\n' > "$tmp/al_noreason"
( cd "$tmp" && python3 "$OLDPWD/$SCAN" al_ok lc.yml >/dev/null ) || { echo "SELFTEST FAIL: reasoned allowlist not honoured"; exit 2; }
( cd "$tmp" && python3 "$OLDPWD/$SCAN" al_noreason lc.yml >/dev/null 2>&1 ) && { echo "SELFTEST FAIL: reason-less allowlist accepted"; exit 2; }

# The generator scan used on real scripts is the SAME function proven here on the heredoc fixtures.
gen_scan() { python3 "$SCAN" "$ALLOW" "$@" 2>&1 | grep -v '^STALE ALLOWLIST'; }
for fx in m1.sh inl.sh; do
    [ -n "$(gen_scan "$tmp/$fx")" ] || { echo "SELFTEST FAIL: generator scan blind to $fx"; exit 2; }
done
[ -z "$(gen_scan "$tmp/okc.sh")" ] || { echo "SELFTEST FAIL: generator scan flags clean shell fixture"; exit 2; }

fail=0
# F1: the real compose checks live in a function so the SAME path is self-tested on a fixture that
# contains literals (a discarded result would otherwise exit 0 on a bad compose file).
check_compose() {
    local rc=0 h l lrc
    h=$(scan "$@")
    if [ -n "$h" ]; then echo "FAIL: non-empty default secrets in compose files:"; echo "$h"; rc=1; fi
    l=$(python3 "$SCAN" "$ALLOW" "$@" 2>&1); lrc=$?
    if [ "$lrc" -ne 0 ]; then echo "FAIL: literal secrets in compose files (file:line:KEY):"; echo "$l"; rc=1; fi
    return $rc
}
[ -f "$ALLOW" ] || { echo "FAIL: allowlist $ALLOW missing"; exit 2; }
printf 'services:\n  a:\n    environment:\n      POSTGRES_PASSWORD: hunter2\n' > "$tmp/c_lit.yml"
printf 'services:\n  a:\n    environment:\n      X_TOKEN: ${X_TOKEN:-hunter2}\n' > "$tmp/c_def.yml"
for fx in c_lit.yml c_def.yml; do
    check_compose "$tmp/$fx" >/dev/null && { echo "SELFTEST FAIL: compose check passes literal fixture $fx"; exit 2; }
done
# R1: the use site is a function (`compose_gate`, sets `fail` itself) driven end-to-end in a subshell with a literal
# fixture: an ignored result fails the self-test; a second `fail` initialisation is caught by the count below.
compose_gate() { check_compose "$@" || fail=1; }
( fail=$((0)); compose_gate "$tmp/c_lit.yml" >/dev/null; [ "$fail" -eq 1 ] ) || { echo "SELFTEST FAIL: compose_gate ignores a failing compose check"; exit 2; }
# structural guard: exactly ONE `fail` initialisation (a second, later one discards findings)
n0=$(grep -cE '(^|[;[:space:]])[f]ail=0' "$SELF")
[ "$n0" -eq 1 ] || { echo "SELFTEST FAIL: expected exactly one fail initialisation, found $n0"; exit 2; }
f0=$(grep -nE '(^|[;[:space:]])[f]ail=0' "$SELF" | head -1 | cut -d: -f1); f1=$(grep -n '[f]ail=1' "$SELF" | head -1 | cut -d: -f1)
[ -n "$f0" ] && [ "$f0" -lt "$f1" ] || { echo "SELFTEST FAIL: fail init not before first failure assignment"; exit 2; }
files=$(git ls-files | grep -E '(^|/)(docker-|infra-)?compose[^/]*\.ya?ml$')
[ -n "$files" ] || { echo "FAIL: no compose files found (blind)"; exit 2; }
# shellcheck disable=SC2086
compose_gate $files
# TI-2: skip ONLY the explicit test roots listed as data (test dirs hold deliberate canary fixtures);
# any other `/tests/` directory is scanned. Filter is a function so it is self-tested below.
ROOTS=scripts/tests/generator_scan_test_roots.txt
[ -f "$ROOTS" ] || { echo "FAIL: $ROOTS missing"; exit 2; }
roots=$(grep -vE '^[[:space:]]*(#|$)' "$ROOTS")
[ -n "$roots" ] || { echo "FAIL: no test roots listed (blind)"; exit 2; }
while IFS= read -r r; do
    case "$r" in */tests) ;; *) echo "FAIL: test root '$r' must end in /tests (too broad)"; fail=1; continue;; esac
    git ls-files --error-unmatch -- "$r" >/dev/null 2>&1 || [ -n "$(git ls-files -- "$r" | head -1)" ] || { echo "FAIL: test root '$r' has no tracked files (stale)"; fail=1; }
done <<<"$roots"
drop_test_roots() { local f r keep; while IFS= read -r f; do keep=1; while IFS= read -r r; do case "$f" in "$r"/*) keep=0; break;; esac; done <<<"$roots"; [ "$keep" = 1 ] && echo "$f"; done; }
# self-test: a listed root is dropped, an UNLISTED /tests/ dir is kept (kills a blanket '/tests/' skip)
first_root=$(head -1 <<<"$roots")
[ -z "$(printf '%s\n' "$first_root/x.sh" | drop_test_roots)" ] || { echo "SELFTEST FAIL: listed test root not skipped"; exit 2; }
[ -n "$(printf '%s\n' "${first_root}X/a.sh" | drop_test_roots)" ] || { echo "SELFTEST FAIL: sibling dir '${first_root}X' wrongly skipped (prefix must be a whole path component)"; exit 2; }
[ -n "$(printf '%s\n' 'scripts/unlisted_dir/tests/x.sh' | drop_test_roots)" ] || { echo "SELFTEST FAIL: unlisted /tests/ dir wrongly skipped"; exit 2; }
gens=$(git ls-files 'scripts/*.sh' | drop_test_roots)
# shellcheck disable=SC2086
glit=$(gen_scan $gens); [ -z "$glit" ] || { echo "FAIL: literal secret defaults in shell scripts (file:line:KEY):"; echo "$glit"; fail=1; }
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
# R2: the test must also work from a subdirectory (one nested run; NDS_NESTED stops recursion).
if [ -z "${NDS_NESTED:-}" ]; then
    ( cd "$(dirname "$SELF")" && NDS_NESTED=1 bash "$SELF" >/dev/null 2>&1 ) || { echo "FAIL: test does not pass when run from a subdirectory"; fail=1; }
fi
[ "$fail" -eq 0 ] && echo "PASS: no default secrets, .env.security untracked+ignored, example present"
exit $fail
