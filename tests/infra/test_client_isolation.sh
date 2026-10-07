#!/usr/bin/env bash
# test_client_isolation.sh - WF12 F3 (RED first). Oracle for scripts/test-infra/run_client.sh and nas_readonly_leg.sh: a client container must NOT see the repository's `.env` (NAS credentials)
# nor any project's per-run credential file. Real stack (redis), real IMG-INFRA-CLIENT container. Oracle strategy (11.4.245): SPECIFIED (what the container sees is what is mounted) and INVARIANT
# (a control needle: the view still holds the client's own lib.sh, the project's own credential still arrives by --env-file; decoys planted OUTSIDE the view must be absent). No file content is
# ever read or printed: existence and readability only.
# Decoys: a file under .audit/test-infra/<decoy project>/env (mode 0600, the shape of a per-run credential file) and one under .audit/scratch/<decoy>/ (the sanctioned scratch root); the real
# `.env` is checked when it exists on this host.
# Paired mutations (copies of the scripts): the client sees the repository again; the view also copies the repository root's files; the path allow-list is removed.
# Usage: test_client_isolation.sh  (ISO_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh run_client.sh client/isolation_probe.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
ID=$(ti_new_id); TI_IDS+=("$ID"); P=$(ti_project "$ID")
out=$(bash "$TI_REPO/$SD/up.sh" --build-id "$ID" --services redis 2>&1); rc=$?
check "up exits 0" "$rc" 0
[ "$rc" = 0 ] || { echo "  up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }
R="zziso$RANDOM"
DEC1="$TI_REPO/.audit/test-infra/catalogizer-test-$R/env"; mkdir -p "$(dirname "$DEC1")"; ( umask 077; echo "TI_REDIS_PASSWORD=decoy-never-a-secret" >"$DEC1" ); TI_FOREIGN_DIRS+=("$TI_REPO/.audit/test-infra/catalogizer-test-$R")
DEC2="$TI_REPO/.audit/scratch/decoy-$R/secret.env"; mkdir -p "$(dirname "$DEC2")"; echo "x=y" >"$DEC2"; TI_FOREIGN_DIRS+=("$TI_REPO/.audit/scratch/decoy-$R")
OWNENV=".audit/test-infra/$P/env"
paths=(".audit/test-infra/catalogizer-test-$R/env" ".audit/scratch/decoy-$R/secret.env" "$OWNENV" "docker-compose.test-infra.yml" ".git/config")
[ -f "$TI_REPO/.env" ] && paths+=(".env")
# battery <sut dir>: prints one FAIL line per violated expectation; exit status = count
battery() {
  local d=$1 n=0 o rc
  o=$(bash "$TI_REPO/$d/run_client.sh" --build-id "$ID" -- bash "/src/$d/client/isolation_probe.sh" "${paths[@]}" 2>&1); rc=$?
  [ "$rc" = 0 ] || { echo "FAIL the probe exited $rc ($(printf '%s' "$o" | tail -1 | cut -c1-100))"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -qx 'NEEDLE present' || { echo "FAIL control needle: the client does not see its own lib.sh (the probe is blind)"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -qx 'ENV TI_REDIS_PASSWORD set' || { echo "FAIL the project's own credential did not reach the client by --env-file"; n=$((n+1)); }
  local p; for p in "${paths[@]}"; do printf '%s\n' "$o" | grep -qx "PATH $p absent" || { echo "FAIL /src/$p is visible to the client ($(printf '%s\n' "$o" | grep -F "PATH $p " | head -1))"; n=$((n+1)); }; done
  printf '%s\n' "$o" | grep -E '^TOP ' | grep -qE '(^| )\.env( |$)|(^| )\.audit( |$)' && { echo "FAIL the top of /src lists .env or .audit ($(printf '%s\n' "$o" | grep '^TOP '))"; n=$((n+1)); }
  # a /src path outside the allowed directories is refused with its reason, and no container runs for it
  o=$(bash "$TI_REPO/$d/run_client.sh" --build-id "$ID" -- cat /src/.env 2>&1); rc=$?
  { [ "$rc" -ne 0 ] && printf '%s' "$o" | grep -q 'reason=client_path_not_in_view'; } || { echo "FAIL a /src/.env command was not refused as client_path_not_in_view (rc=$rc)"; n=$((n+1)); }
  return "$n"
}
res=$(battery "$SD"); n=$?
if [ "$n" -eq 0 ]; then ok "the client sees only its scripts: no .env, no per-run credential file of any project, no .git; its own credential arrives by --env-file; /src/.env is refused by name"; else bad "$n violation(s): $(printf '%s' "$res" | tr '\n' ';' | cut -c1-500)"; fi
check "no view directory is left behind" "$(ls -d "$TI_REPO"/.audit/scratch/ti-view.* 2>/dev/null | wc -l)" 0

if [ "${ISO_NO_MUTATIONS:-0}" != 1 ] && [ "${ISO_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${ISO_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mut() { local name=$1 file=$2 old=$3 new=$4 d="$TI_REPO/.audit/scratch/ti-mut-iso-$1" r n
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$SD"/*.sh "$d/"; cp -r "$TI_REPO/$SD/client" "$d/client"
    python3 -I - "$d/$file" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
    r=$(battery ".audit/scratch/ti-mut-iso-$name"); n=$?
    if [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${d:?}"; }
  mut client_sees_repository run_client.sh '(cd "$VIEW" && RUNP_PRINT_ARGV=1' '(cd "$TI_ROOT" && RUNP_PRINT_ARGV=1'
  mut view_copies_repo_root_files run_client.sh 'ti_view_dir "$VIEW" "${VDIRS[@]}" ||' 'ti_view_dir "$VIEW" "${VDIRS[@]}" . ||'
  mut allow_list_removed run_client.sh '      case "$dir/" in scripts/test-infra/*|.audit/scratch/*) ;; *) ti_refuse client_path_not_in_view "$a is outside the directories a client may see (scripts/test-infra/, .audit/scratch/)";; esac' '      :'
  [ -z "${ISO_EV:-}" ] || cp "$MUTLOG" "$ISO_EV/client-isolation-mutations.txt"
fi
ti_summary
