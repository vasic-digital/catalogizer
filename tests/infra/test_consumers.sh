#!/usr/bin/env bash
# test_consumers.sh - WF12 F1 (RED first). Defect class C5: CONSUMERS OF A CHANGED CONTRACT NOT MIGRATED. T129 changed docker-compose.test-infra.yml and the postgres/redis services of
# docker-compose.build.yml from literal credentials and fixed ports to required `${TI_*:?}` variables from a per-run env file. Every consumer of the old contract must follow:
#   1 scripts/container-build.sh        (the documented containerized build entry point) must generate the env file and validate the compose file with it (--validate-only drives that REAL path)
#   2 scripts/setup-test-env.sh         must start the stack through scripts/test-infra/up.sh (real run, one service), never a bare compose call, and report a failed start as a failure
#   3 catalog-api/tests/infra_helper.go must carry no fixed port or literal credential and no instruction that now fails
#   4 documentation / compose headers   no live instruction may tell a reader to run the compose files bare (historical records under the allow-list are kept as written)
# Oracle strategy (11.4.245): SPECIFIED (a bare compose call is refused: measured as the control; the migrated entry points succeed) and INVARIANT (the enumeration of references is a
# `git grep` over every tracked file, with a control needle proving the scan sees a planted stale instruction).
# Paired mutations: a copy of container-build.sh that drops --env-file (validation must fail); a copy of setup-test-env.sh that calls compose bare (the real run must fail).
# Usage: test_consumers.sh  (CONS_NO_MUTATIONS=1: tests only)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
export TI_ROOT="$TI_REPO"
cd "$TI_REPO" || exit 2

# ---- control: a bare compose call IS refused (the reason the consumers needed migrating) ----
(env -i PATH="$PATH" HOME="$HOME" podman-compose -f docker-compose.build.yml config >/dev/null 2>&1); check "control: docker-compose.build.yml without the per-run env is refused (rc != 0)" "$([ $? -ne 0 ] && echo refused || echo rendered)" refused
(env -i PATH="$PATH" HOME="$HOME" podman-compose -f docker-compose.test-infra.yml config >/dev/null 2>&1); check "control: docker-compose.test-infra.yml without the per-run env is refused (rc != 0)" "$([ $? -ne 0 ] && echo refused || echo rendered)" refused

# ---- 1 container-build.sh ----
cb() { # cb <script path> -> runs the real validation path (TI_ROOT unset: a mutation copy in a scratch root must keep its per-run state THERE, not in the real repository)
  env -u TI_ROOT bash "$1" --validate-only 2>&1; }
o=$(cb "$TI_REPO/scripts/container-build.sh"); rc=$?
check "container-build.sh --validate-only exits 0 (the compose file renders with the generated per-run env)" "$rc" 0
case "$o" in *"Compose file is valid"*) ok "container-build.sh reports 'Compose file is valid'";; *) bad "container-build.sh did not validate ($(printf '%s' "$o" | tail -2 | tr '\n' ' ' | cut -c1-200))";; esac
check "container-build.sh left no per-run state directory behind" "$(ls -d "$TI_REPO"/.audit/test-infra/catalogizer-test-build-* 2>/dev/null | wc -l)" 0
check "container-build.sh --validate-only stopped before the signing keys (no key generation ran)" "$(printf '%s' "$o" | grep -c 'Checking signing keys')" 0

# ---- 2 setup-test-env.sh (a REAL start of one service) ----
ID=$(ti_new_id); TI_IDS+=("$ID")
o=$(bash "$TI_REPO/scripts/setup-test-env.sh" --build-id "$ID" --services redis 2>&1); rc=$?
check "setup-test-env.sh exits 0 and starts the stack through up.sh" "$rc" 0
OP=$(printf '%s\n' "$o" | sed -n 's/^op_id=//p' | head -1)
case "$OP" in catalogizer-test-"$ID"-up-*) ok "setup-test-env.sh printed the op_id of the start ($OP)";; *) bad "setup-test-env.sh printed no op_id ($(printf '%s' "$o" | tail -3 | tr '\n' ' ' | cut -c1-200))";; esac
check "the redis container of the project runs" "$(podman ps -q --filter "label=catalogizer.test_project=catalogizer-test-$ID" | wc -l)" 1
check "the lease of the start is a live keeper" "$(ti_lease_state "$ID" "$OP")" ok
printf '%s\n' "$o" | grep -q '^TI_PORT_REDIS=[0-9]\+$' && ok "setup-test-env.sh shows the random redis port" || bad "no TI_PORT_REDIS line"
printf '%s\n' "$o" | grep -qE '(2121|1445)|testuser|testpass' && bad "setup-test-env.sh output carries a fixed port or the old literal credential" || ok "setup-test-env.sh output carries no fixed port and no literal credential"
ti_down "$ID" >/dev/null 2>&1
o=$(bash "$TI_REPO/scripts/setup-test-env.sh" --build-id 'Bad Id' --services redis 2>&1); rc=$?
check "setup-test-env.sh reports a failed start as a FAILURE (exit != 0), not as 'not available'" "$([ "$rc" -ne 0 ] && echo failed || echo swallowed)" failed

# ---- 3 infra_helper.go: no fixed contract left ----
H=catalog-api/tests/infra_helper.go
check "infra_helper.go: no fixed host port (2121, 1445, 8081, 2049) and no literal testuser/testpass" "$(grep -cE '"localhost:(2121|1445|8081|2049)"|"testuser"|"testpass"' "$H")" 0
check "infra_helper.go: no instruction to run the compose file bare" "$(grep -c 'podman-compose -f docker-compose' "$H")" 0
grep -q 'CATALOGIZER_TEST_INFRA_ENV' "$H" && ok "infra_helper.go reads the per-run env file (CATALOGIZER_TEST_INFRA_ENV)" || bad "infra_helper.go does not read the per-run env file"

# ---- 4 enumeration: no live instruction runs the compose files bare ----
STALE='(podman|docker)-compose -f docker-compose\.(test-infra|build)\.yml +(up|down|config)|docker compose -f docker-compose\.(test-infra|build)\.yml'
# historical records keep what they said at the time: plans of record, research, security scans, spec evidence
ALLOW=(':!specs' ':!docs/research' ':!docs/security' ':!docs/plans/archive' ':!docs/superpowers' ':!tests/infra')
hits=$(git grep -nE "$STALE" -- . "${ALLOW[@]}" | cut -c1-160)
if [ -z "$hits" ]; then ok "no tracked live file instructs a bare run of docker-compose.test-infra.yml / docker-compose.build.yml"; else bad "stale instruction(s): $(printf '%s' "$hits" | head -3 | tr '\n' ';')"; fi
printf 'run: podman-compose -f docker-compose.test-infra.yml up -d\n' >"$TI_SCRATCH/needle.md"
grep -qE "$STALE" "$TI_SCRATCH/needle.md" && ok "control needle: the scan pattern sees a planted stale instruction" || bad "control needle: the scan is blind"

# ---- 5 every script that tears a stack down names the operation it owns (down.sh refuses any other caller of a live lease, WF12 F6) ----
miss=$(git grep -nE 'down\.sh"? +--build-id' -- scripts ':!scripts/test-infra/down.sh' | grep -vE '^[^:]*:[0-9]+:[[:space:]]*#' | grep -v -e '--op-id' | cut -c1-140)
if [ -z "$miss" ]; then ok "every non-test caller of down.sh passes --op-id (nfs_attempt.sh, up.sh, setup-test-env.sh)"; else bad "a caller of down.sh names no operation: $(printf '%s' "$miss" | head -2 | tr '\n' ';')"; fi
printf 'bash "$HERE/down.sh" --build-id "$BID" >/dev/null\n' >"$TI_SCRATCH/needle2.sh"
[ -n "$(grep -E 'down\.sh"? +--build-id' "$TI_SCRATCH/needle2.sh" | grep -v -e '--op-id')" ] && ok "control needle: the caller scan sees a planted down.sh call without --op-id" || bad "control needle: the caller scan is blind"
callers_count=$(git grep -cE 'down\.sh"? +--build-id' -- scripts ':!scripts/test-infra/down.sh' | awk -F: '{s+=$2} END {print s+0}')
[ "$callers_count" -ge 3 ] && ok "the caller scan sees $callers_count down.sh call sites (not blind)" || bad "the caller scan sees only $callers_count call sites"

# ---- paired mutations ----
if [ "${CONS_NO_MUTATIONS:-0}" != 1 ]; then
  MUTLOG="${CONS_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  # a copy of container-build.sh without --env-file (in a repository-shaped scratch root): the validation must FAIL
  d="$TI_REPO/.audit/scratch/cons-mut-$$"; rm -rf -- "${d:?}"; mkdir -p "$d/scripts"; cp "$TI_REPO/scripts/container-build.sh" "$d/scripts/"; cp -r "$TI_REPO/scripts/test-infra" "$d/scripts/test-infra"; cp "$TI_REPO/docker-compose.build.yml" "$TI_REPO/docker-compose.test-infra.yml" "$d/"
  sed -i 's/^ENV_ARGS=(--env-file "$BUILD_STATE_DIR\/env")/ENV_ARGS=()/' "$d/scripts/container-build.sh"
  grep -q '^ENV_ARGS=()' "$d/scripts/container-build.sh" || bad "mutation container_build_no_env_file: anchor missing"
  o=$(cb "$d/scripts/container-build.sh"); rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$o" | grep -q 'validation failed'; then ok "mutation container_build_no_env_file CAUGHT (rc=$rc: $(printf '%s' "$o" | tail -1 | cut -c1-80))"; echo "container_build_no_env_file CAUGHT" >>"$MUTLOG"; else bad "mutation container_build_no_env_file SURVIVED (rc=$rc)"; echo "container_build_no_env_file SURVIVED" >>"$MUTLOG"; fi
  rm -rf -- "${d:?}"
  # a copy of setup-test-env.sh that calls the compose file bare: no started, leased stack may result
  d="$TI_REPO/.audit/scratch/cons-mut2-$$"; rm -rf -- "${d:?}"; mkdir -p "$d/scripts"; cp "$TI_REPO/scripts/setup-test-env.sh" "$d/scripts/"; cp -r "$TI_REPO/scripts/test-infra" "$d/scripts/test-infra"; cp "$TI_REPO/docker-compose.test-infra.yml" "$d/"
  python3 -I - "$d/scripts/setup-test-env.sh" <<'PY'
import sys
p = sys.argv[1]; s = open(p).read(); a = 'OUT="$(bash scripts/test-infra/up.sh "${ARGS[@]}")" || {'
assert s.count(a) == 1
open(p, "w").write(s.replace(a, 'OUT="$(podman-compose -f docker-compose.test-infra.yml up -d redis 2>&1)" || {'))
PY
  MID=$(ti_new_id); TI_IDS+=("$MID")
  o=$(bash "$d/scripts/setup-test-env.sh" --build-id "$MID" --services redis 2>&1); rc=$?
  state=$(printf '%s\n' "$o" | grep -c '^op_id=')
  if [ "$rc" -ne 0 ] && [ "$state" -eq 0 ]; then ok "mutation setup_bare_compose CAUGHT (rc=$rc, op_id lines=$state)"; echo "setup_bare_compose CAUGHT" >>"$MUTLOG"; else bad "mutation setup_bare_compose SURVIVED (rc=$rc op_id lines=$state)"; echo "setup_bare_compose SURVIVED" >>"$MUTLOG"; fi
  rm -rf -- "${d:?}"
  [ -z "${CONS_EV:-}" ] || cp "$MUTLOG" "$CONS_EV/consumers-mutations.txt"
fi
ti_summary
