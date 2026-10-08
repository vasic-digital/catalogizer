#!/usr/bin/env bash
# test_compose_files.sh - T129, rewritten by WF17 fix round 5 (TI-G1). Oracle for the THREE compose files of the test-infrastructure stack: docker-compose.test-infra.yml, docker-compose.test-infra.nfs.yml (scanned
# by nothing before WF17) and docker-compose.build.yml. The scanner is tests/infra/compose_scan.py: it RENDERS each file with the real `podman-compose config` in an empty environment with a sentinel env
# file and ALLOW-LISTS the rendered tree (exact environment, command, healthcheck test, labels, loopback sentinel ports, volumes, cap_add, digest-pinned images equal to the lock, `pull_policy: never`, the exact
# service set, `x-podman: in_pod: false`) plus raw checks on the source (only `${TI_NAME}` / `${TI_NAME:?msg}` references; no default or substitution form). A finding never prints a value.
# Oracle strategy (11.4.245): SPECIFIED (docs/16 D-06/D-07 define the rules; the lock entries IMG-INFRA-* hold the digests) and DERIVED (the podman-compose render is the independent witness of what each
# `${...}` becomes); the controls are CM0 (a literal POSTGRES_PASSWORD MUST be caught) and the identity mutants (an equivalent file MUST NOT be flagged: 11.4.201(1)).
# Paired mutations: tests/infra/compose_mutants.py - the reviewer mutants A1-A7, S1-S21, CM0-CM8, CM10 and the WF12 set, the nfs file's own, adopted verbatim in effect; a gen_env.sh that writes mode 0644.
# Usage:  test_compose_files.sh        (COMPOSE_TEST_NO_MUTATIONS=1: scan and gen_env checks only)   Env: COMPOSE_TEST_ROOT (a directory holding the three files: RED = an export of git HEAD), COMPOSE_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
ROOT="${COMPOSE_TEST_ROOT:-$TI_REPO}"
INFRA="$ROOT/docker-compose.test-infra.yml"; NFSF="$ROOT/docker-compose.test-infra.nfs.yml"; BUILD="$ROOT/docker-compose.build.yml"
GEN="${COMPOSE_TEST_GEN:-$TI_REPO/scripts/test-infra/gen_env.sh}"
SCAN="$TI_TEST_HERE/compose_scan.py"
# 1. the files exist and parse
for f in "$INFRA" "$NFSF" "$BUILD"; do if [ -f "$f" ] && python3 -I -c "import yaml,sys;yaml.safe_load(open(sys.argv[1]))" "$f" 2>/dev/null; then ok "parses: $(basename "$f")"; else bad "missing or unparsable: $f"; fi; done
# 2. the structural scan of each file: no literal credential or port, digest-pinned, labelled, rootless, no pod, exact service set
for k in infra nfs build; do
  res="$(python3 -I "$SCAN" scan "$k" --root "$ROOT" 2>&1)"; rc=$?
  if [ "$rc" = 0 ]; then ok "compose_scan $k: clean (rendered with sentinels; environment, command, healthcheck, labels, ports, volumes, cap_add, images all on the allow-list)"
  else bad "compose_scan $k: $(printf '%s\n' "$res" | grep -c .) finding(s): $(printf '%s' "$res" | head -4 | tr '\n' ';' | cut -c1-400)"; fi
done
# 2b. control: the scanner is not blind. A planted literal MUST be reported by the same entry point on a scratch root (CM0), and the findings never print the value
CM="$TI_SCRATCH/cm0"; mkdir -p "$CM/build/containers"; cp "$INFRA" "$NFSF" "$BUILD" "$CM/"; cp "$ROOT/build/containers/images.lock.yaml" "$CM/build/containers/"
python3 -I - "$CM/docker-compose.test-infra.yml" <<'PY'
import sys
p = sys.argv[1]; s = open(p).read(); a = 'POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"'
assert s.count(a) == 1; open(p, "w").write(s.replace(a, 'POSTGRES_PASSWORD: "hunter2literal"'))
PY
res="$(python3 -I "$SCAN" scan infra --root "$CM" 2>&1)"; rc=$?
if [ "$rc" = 1 ] && printf '%s' "$res" | grep -q 'infra.postgres.environment.POSTGRES_PASSWORD'; then ok "control CM0: a literal POSTGRES_PASSWORD is reported at its location"; else bad "control CM0: the scanner did not report the planted literal (rc=$rc: $(printf '%s' "$res" | head -2 | tr '\n' ';' | cut -c1-200))"; fi
printf '%s' "$res" | grep -q 'hunter2literal' && bad "a finding PRINTS the offending value (it may be a credential)" || ok "a finding never prints the offending value"
# 2c. the repository's own pin checker agrees about the test-infra file (it is the T105 gate)
if [ -f "$INFRA" ] && [ "$ROOT" = "$TI_REPO" ]; then
  if bash "$TI_REPO/scripts/containers/check_pins.sh" docker-compose.test-infra.yml >"$TI_SCRATCH/pins.txt" 2>&1; then ok "check_pins.sh: docker-compose.test-infra.yml has no unpinned reference"; else bad "check_pins.sh reports: $(head -3 "$TI_SCRATCH/pins.txt" | tr '\n' ';')"; fi
fi
# 3. gen_env.sh: mode 0600, outside version control, defines every variable the files need, differs between runs
if [ -f "$GEN" ]; then
  G1="$TI_SCRATCH/g1"; G2="$TI_SCRATCH/g2"; mkdir -p "$G1" "$G2"
  TI_STATE_DIR="$G1" bash "$GEN" --build-id gen1 >"$TI_SCRATCH/gen1.out" 2>&1; check "gen_env.sh exits 0 (run 1)" "$?" 0
  TI_STATE_DIR="$G2" bash "$GEN" --build-id gen1 >"$TI_SCRATCH/gen2.out" 2>&1; check "gen_env.sh exits 0 (run 2)" "$?" 0
  E1="$G1/catalogizer-test-gen1/env"; E2="$G2/catalogizer-test-gen1/env"
  check "env file mode is 0600" "$(stat -c %a "$E1" 2>/dev/null)" 600
  check "ports file holds no credential variable" "$(grep -cE '^TI_[A-Z_]*(PASSWORD|USER|DB)=' "$G1/catalogizer-test-gen1/ports.env" 2>/dev/null)" 0
  used="$(grep -ho '\${TI_[A-Z][A-Z_]*' "$INFRA" "$NFSF" "$BUILD" 2>/dev/null | sed 's/^\${//' | sort -u | grep -v '^TI_MINIO_IMAGE$' | grep -v '^TI_NFS_IMAGE$')"
  miss=""; for v in $used; do grep -q "^$v=" "$E1" 2>/dev/null || miss="$miss $v"; done
  if [ -z "$used" ]; then bad "the compose files reference no TI_* variable at all (everything is literal)"
  elif [ -z "$miss" ]; then ok "gen_env.sh defines every TI_* variable the three compose files reference ($(echo $used | wc -w))"; else bad "variables referenced but not generated:$miss"; fi
  check "gen_env.sh writes the checkout label TI_ROOT_HASH" "$(grep -c '^TI_ROOT_HASH=[0-9a-f]\{16\}$' "$E1")" 1
  if [ "$(grep '^TI_POSTGRES_PASSWORD=' "$E1")" != "$(grep '^TI_POSTGRES_PASSWORD=' "$E2")" ] && [ "$(grep '^TI_PORT_POSTGRES=' "$E1")" != "$(grep '^TI_PORT_POSTGRES=' "$E2")" ]; then ok "credentials and ports differ between two runs (random per run)"; else bad "two runs produced the same credential or the same port"; fi
  pw="$(sed -n 's/^TI_POSTGRES_PASSWORD=//p' "$E1")"; if [ "${#pw}" -ge 24 ]; then ok "generated password has >= 24 characters"; else bad "generated password too short (${#pw})"; fi
  # outside version control: the real state directory is git-ignored
  if git -C "$TI_REPO" check-ignore -q ".audit/test-infra/catalogizer-test-x/env"; then ok "the default env file location is git-ignored"; else bad "the default env file location is NOT git-ignored"; fi
  # the files render with the generated env (every ${...} substituted), and WITHOUT it podman-compose refuses (required variables)
  rendered="$(cd "$ROOT" && podman-compose -f "$INFRA" --env-file "$E1" config 2>"$TI_SCRATCH/cfg.err")"; rc=$?
  if [ "$rc" = 0 ] && ! printf '%s' "$rendered" | grep -q '\${'; then ok "podman-compose config renders docker-compose.test-infra.yml with the generated env, nothing unsubstituted"; else bad "config did not render cleanly rc=$rc: $(tail -2 "$TI_SCRATCH/cfg.err" | tr '\n' ' ')"; fi
  (cd "$ROOT" && env -i PATH="$PATH" HOME="$HOME" podman-compose -f "$INFRA" config >/dev/null 2>&1); if [ $? -ne 0 ]; then ok "without the env file the compose file is refused (required variables)"; else bad "the compose file rendered without the per-run env (a variable has a literal default)"; fi
else bad "gen_env.sh absent: $GEN"; fi
# 4. paired mutations
if [ "${COMPOSE_TEST_NO_MUTATIONS:-0}" != 1 ]; then
  MUTLOG="${COMPOSE_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  res="$(python3 -I "$SCAN" mutants "$TI_TEST_HERE/compose_mutants.py" --root "$ROOT" 2>&1)"; rc=$?
  printf '%s\n' "$res" >>"$MUTLOG"
  n=$(printf '%s\n' "$res" | grep -c '^MUTANT .* CAUGHT '); sv=$(printf '%s\n' "$res" | grep -c '^MUTANT .* SURVIVED$'); br=$(printf '%s\n' "$res" | grep -c ' BROKEN '); idn=$(printf '%s\n' "$res" | grep -c 'SURVIVED-AS-REQUIRED')
  if [ "$rc" = 0 ] && [ "$n" -ge 60 ] && [ "$idn" -ge 4 ]; then ok "compose mutants: $n CAUGHT (reviewer A1-A7, S1-S21, CM0-CM8, CM10 and the nfs/build/WF12 sets), $idn identity mutants SURVIVED as required, 0 survivors"
  else bad "compose mutants: rc=$rc caught=$n survived=$sv broken=$br identity-survived=$idn: $(printf '%s\n' "$res" | grep -vE 'CAUGHT|SURVIVED-AS-REQUIRED' | head -3 | tr '\n' ';')"; fi
  # control: the mutation harness itself can fail (a mutant whose anchor does not exist is BROKEN and fails the run)
  printf 'MUTANTS = [dict(name="broken_anchor", file="docker-compose.test-infra.yml", edits=[("NO SUCH TEXT", "x")], expect="caught")]\n' >"$TI_SCRATCH/broken_mutants.py"
  python3 -I "$SCAN" mutants "$TI_SCRATCH/broken_mutants.py" --root "$ROOT" >"$TI_SCRATCH/broken.out" 2>&1; rc=$?
  if [ "$rc" = 1 ] && grep -q 'BROKEN' "$TI_SCRATCH/broken.out"; then ok "harness control: a mutant with a missing anchor is reported BROKEN (exit 1)"; else bad "harness control: a broken mutant did not fail the harness (rc=$rc)"; fi
  if [ -f "$GEN" ]; then # a gen_env.sh copy that writes mode 0644 must fail the mode check
    mkdir -p "$TI_SCRATCH/gm"; cp "$GEN" "$TI_SCRATCH/gm/gen_mode.sh"; cp "$(dirname "$GEN")/lib.sh" "$TI_SCRATCH/gm/lib.sh"
    python3 -I - "$TI_SCRATCH/gm/gen_mode.sh" <<'PY' && {
import sys
s = open(sys.argv[1]).read(); assert s.count("chmod 600 \"$TMP\"") == 1; open(sys.argv[1], "w").write(s.replace("chmod 600 \"$TMP\"", "chmod 644 \"$TMP\""))
PY
      TI_ROOT="$TI_REPO" TI_STATE_DIR="$TI_SCRATCH/g3" bash "$TI_SCRATCH/gm/gen_mode.sh" --build-id gen1 >/dev/null 2>&1
      m="$(stat -c %a "$TI_SCRATCH/g3/catalogizer-test-gen1/env" 2>/dev/null)"
      if [ "$m" != 600 ]; then ok "mutation gen_env_mode644 CAUGHT (mode $m != 600)"; echo "gen_env_mode644 CAUGHT" >>"$MUTLOG"; else bad "mutation gen_env_mode644 SURVIVED"; echo "gen_env_mode644 SURVIVED" >>"$MUTLOG"; fi; }
  fi
  [ -z "${COMPOSE_EV:-}" ] || cp "$MUTLOG" "$COMPOSE_EV/compose-mutations.txt"
fi
ti_summary
