#!/usr/bin/env bash
# Sandbox test for restart_stack.sh: stub podman/podman-compose/curl on PATH, assert exit codes.
# Usage: test_restart_stack.sh   (RESTART_SCRIPT env overrides the script under test)
set -u
H="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; S="${RESTART_SCRIPT:-$H/restart_stack.sh}"
W="$(mktemp -d)" || exit 2; trap 'rm -rf "${W:?}"' EXIT
mkdir -p "$W/bin" "$W/stack"; printf 'POSTGRES_USER=u\nPOSTGRES_PASSWORD=x\n' > "$W/stack/qa.env"; : > "$W/stack/qa-compose.yml"
# stub behaviour via env: STUB_PG=ok|fail STUB_COMPOSE=ok|fail STUB_CURL=<api http code> STUB_WEB=<code> STUB_WEBSITE=<code>
# ports used by the test: api 38999, web 38998, website 38997
cat > "$W/bin/podman" <<'S'
#!/usr/bin/env bash
case "$1" in
 exec) case "$2" in *postgres) [ "$STUB_PG" = ok ] && exit 0 || exit 1;; *redis) [ "$STUB_PG" = ok ] && echo PONG; exit 0;; esac;;
 inspect) [ "${STUB_INSPECT:-ok}" = fail ] && exit 1; echo 10.0.0.1;; esac; exit 0
S
cat > "$W/bin/podman-compose" <<'S'
#!/usr/bin/env bash
echo compose-out; [ "$STUB_COMPOSE" = ok ] && exit 0 || exit 3
S
cat > "$W/bin/curl" <<'S'
#!/usr/bin/env bash
case "$*" in *:38998/*) printf '%s' "${STUB_WEB:-200}";; *:38997/*) printf '%s' "${STUB_WEBSITE:-200}";; *) printf '%s' "$STUB_CURL";; esac; exit 0
S
cat > "$W/bin/sleep" <<'S'
#!/usr/bin/env bash
exit 0
S
chmod +x "$W"/bin/*
fail=0
run() { # name expect_nonzero(1|0) env...
  local n="$1" want="$2"; shift 2
  env PATH="$W/bin:$PATH" TMPDIR="$W" "$@" bash "$S" "$W/stack" t 38999 38998 38997 >"$W/out" 2>&1; local rc=$?; [ -n "${SHOW:-}" ] && cat "$W/out"
  if { [ "$want" = 1 ] && [ $rc -ne 0 ]; } || { [ "$want" = 0 ] && [ $rc -eq 0 ]; }; then echo "PASS $n rc=$rc"; else echo "FAIL $n rc=$rc (wanted nonzero=$want)"; fail=1; fi
}
run happy_exit0                0 STUB_PG=ok   STUB_COMPOSE=ok   STUB_CURL=200
run db_never_ready_nonzero     1 STUB_PG=fail STUB_COMPOSE=ok   STUB_CURL=200
run api_never_healthy_nonzero  1 STUB_PG=ok   STUB_COMPOSE=ok   STUB_CURL=000
run compose_failure_nonzero    1 STUB_PG=ok   STUB_COMPOSE=fail STUB_CURL=200
run inspect_failure_nonzero    1 STUB_PG=ok   STUB_COMPOSE=ok   STUB_CURL=200 STUB_INSPECT=fail
run web_never_ready_nonzero    1 STUB_PG=ok   STUB_COMPOSE=ok   STUB_CURL=200 STUB_WEB=000
run website_never_ready_nonzero 1 STUB_PG=ok  STUB_COMPOSE=ok   STUB_CURL=200 STUB_WEBSITE=502
PATH="$W/bin:$PATH" bash "$S" >"$W/out" 2>&1; [ $? -ne 0 ] && echo "PASS missing_args_nonzero" || { echo "FAIL missing_args_nonzero"; fail=1; }
PATH="$W/bin:$PATH" bash "$S" "$W/nonexistent" t 1 2 3 >"$W/out" 2>&1; [ $? -ne 0 ] && echo "PASS bad_dir_nonzero" || { echo "FAIL bad_dir_nonzero"; fail=1; }
# mutation checks: the suite must FAIL on each broken copy of the script (proves the tests are load-bearing)
mut() { # name sed-expr
  local m="$W/mut_$1.sh"; sed -E "$2" "$S" > "$m"
  if cmp -s "$S" "$m"; then echo "FAIL mutation_$1 (sed did not change the script)"; fail=1; return; fi
  if RESTART_SCRIPT="$m" NO_MUT=1 bash "$H/test_restart_stack.sh" >/dev/null 2>&1; then echo "FAIL mutation_$1 survived"; fail=1; else echo "PASS mutation_$1 killed"; fi
}
if [ -z "${NO_MUT:-}" ]; then
  mut no_set_e        's/^set -euo pipefail/set -uo pipefail/'
  mut web_probe_gone  's/^\[ \$wok -eq 1 \].*$/:/'
  mut website_probe_gone 's/^\[ \$sok -eq 1 \].*$/:/'
  mut db_timeout_swallowed 's/^\[ \$ready -eq 1 \].*$/:/'
fi
exit $fail
