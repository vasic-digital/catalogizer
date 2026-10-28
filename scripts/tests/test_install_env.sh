#!/usr/bin/env bash
# Test: the installer's env-generation path (no containers started) writes a
# .env that satisfies every `${VAR:?}` of docker-compose.yml, with random,
# non-placeholder secrets, mode 0600, and prints no secret value.
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "${tmp:?}"' EXIT
fail=0
ok()  { echo "ok   - $1"; }
bad() { echo "FAIL - $1"; fail=1; }

LIB="$ROOT/scripts/lib/install_env.sh"
[ -f "$LIB" ] || { echo "FAIL - $LIB missing"; exit 1; }
# shellcheck disable=SC1090
. "$LIB"

out1="$(install_write_env "$tmp/env1" 2>&1)"; rc1=$?
out2="$(install_write_env "$tmp/env2" 2>&1)"
[ "$rc1" -eq 0 ] && ok "install_write_env succeeds" || bad "install_write_env rc=$rc1"

mode="$(stat -c %a "$tmp/env1")"
[ "$mode" = 600 ] && ok "mode is 0600" || bad "mode is $mode, want 600"

val() { sed -n "s/^$2=//p" "$1" | head -1; }

# Every required compose variable is present and non-empty.
# Every non-test-infra compose file (test-infra/test/build/qa files carry their
# own TI_*/FTPS_*/SFTP_* inputs, not installer secrets).
files="$(cd "$ROOT" && ls docker-compose.yml docker-compose.dev.yml docker-compose.security.yml deploy/infra-compose.yml 2>/dev/null)"
[ "$(echo "$files" | wc -l)" -ge 4 ] || { echo "FAIL - compose set shrank (blind)"; exit 2; }
req="$(cd "$ROOT" && grep -hoE '\$\{[A-Za-z0-9_]+:\?' $files | tr -d '${:?' | sort -u)"
[ -n "$req" ] || { echo "FAIL - no :? vars found (blind)"; exit 2; }
for v in $req; do
    x="$(val "$tmp/env1" "$v")"
    [ -n "$x" ] && ok "$v present" || bad "$v missing/empty in generated env"
done
for v in DATABASE_PASSWORD POSTGRES_PASSWORD JWT_SECRET ADMIN_PASSWORD GRAFANA_PASSWORD MINIO_ROOT_PASSWORD SONAR_DB_PASSWORD PGADMIN_DEFAULT_PASSWORD STORAGE_SECRET_KEY; do
    a="$(val "$tmp/env1" "$v")"; b="$(val "$tmp/env2" "$v")"
    [ "${#a}" -ge 24 ] && ok "$v length>=24" || bad "$v too short (${#a})"
    [ -n "$a" ] && [ "$a" != "$b" ] && ok "$v differs between runs (random)" || bad "$v not random"
    case "$(printf %s "$a" | tr A-Z a-z)" in
        *change*|*your-super*|*placeholder*|admin_password|catalogizer_password) bad "$v looks like a placeholder" ;;
        *) ok "$v not a placeholder" ;;
    esac
    # Secrets must never be echoed.
    case "$out1$out2" in *"$a"*) bad "$v value was printed" ;; *) ok "$v not printed" ;; esac
done
[ "$(val "$tmp/env1" DATABASE_PASSWORD)" = "$(val "$tmp/env1" POSTGRES_PASSWORD)" ] \
    && ok "DATABASE_PASSWORD matches POSTGRES_PASSWORD" || bad "DATABASE_PASSWORD != POSTGRES_PASSWORD"
# The ADMIN_PASSWORD / JWT_SECRET literals of the old heredoc are gone.
grep -qE '^(DATABASE_PASSWORD=catalogizer_password|JWT_SECRET=your-super|ADMIN_PASSWORD=admin_password)' "$tmp/env1" \
    && bad "old literal defaults still written" || ok "no old literal defaults"

# Wiring: install.sh must use the generator and carry no literal credential.
grep -q 'install_write_env' "$ROOT/scripts/install.sh" && ok "install.sh calls install_write_env" || bad "install.sh does not call install_write_env"
grep -qE '^(DATABASE_PASSWORD|JWT_SECRET|ADMIN_PASSWORD|GRAFANA_PASSWORD|MINIO_ROOT_PASSWORD)=[^$[:space:]]' "$ROOT/scripts/install.sh" \
    && bad "install.sh still contains a literal credential line" || ok "install.sh has no literal credential line"

# SonarQube and its database must be fed by ONE variable (they must be equal).
jdbc="$(sed -n 's/^ *SONAR_JDBC_PASSWORD: *\${\([A-Z_]*\):?.*/\1/p' "$ROOT/docker-compose.security.yml")"
dbpw="$(awk '/sonarqube-db:/{f=1} f&&/POSTGRES_PASSWORD:/{print; exit}' "$ROOT/docker-compose.security.yml" | sed -n 's/.*\${\([A-Z_]*\):?.*/\1/p')"
{ [ -n "$jdbc" ] && [ "$jdbc" = "$dbpw" ]; } && ok "sonar app and db share one variable ($jdbc)" || bad "sonar vars differ: '$jdbc' vs '$dbpw'"

# Never overwrite an existing file.
echo "KEEP=1" > "$tmp/env3"
install_write_env "$tmp/env3" >/dev/null 2>&1; rc=$?
{ [ "$rc" -ne 0 ] && [ "$(cat "$tmp/env3")" = "KEEP=1" ]; } && ok "existing file refused, untouched" || bad "existing file overwritten"

# A dangling symlink at the target must be refused and its destination never created.
ln -s "$tmp/victim" "$tmp/env4"
install_write_env "$tmp/env4" >/dev/null 2>&1; rc=$?
{ [ "$rc" -ne 0 ] && [ ! -e "$tmp/victim" ]; } && ok "dangling symlink refused, victim not created" || bad "dangling symlink followed (rc=$rc)"
# The generator must not leak its locals into the caller's shell (run it in THIS shell, not a $(...) subshell).
install_write_env "$tmp/env5" >/dev/null 2>&1
{ [ -z "${_minio_user:-}" ] && [ -z "${_pgadmin_pw:-}" ] && [ -z "${_sonar_pw:-}" ] && [ -z "${_db_pw:-}" ]; } && ok "no secret locals leak" || bad "secret locals leaked into caller"

[ "$fail" -eq 0 ] && echo "PASS" || echo "FAILED"
exit $fail
