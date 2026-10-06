#!/usr/bin/env bash
# mutate_register_ops.sh - paired mutations of the WP-06 register operation scripts (T064, T064a, T066, T067, T067a).
# Usage: mutate_register_ops.sh <locked|backup|dump|export|replay> <out.txt>     exit 1 when a mutant survives unreviewed.
# Every mutant is a sed edit of one marked line (# MUT:<name>) or one anchored statement of the script under test, run against that group's test with the
# mutant substituted through the group's env variable. A mutant identical to its source is an ERROR (exit 3), a mutant that does not parse is an ERROR.
# Each group also runs the GOLDEN-GOOD control (the unmodified script, must pass) and a NEGATIVE-CONTROL (a mutant that is a no-op comment edit must
# also pass: it proves the runner does not kill mutants for the wrong reason). Reviewed-equivalent survivors are listed in equivalent_ops_mutants.tsv
# and reported EQUIV, never counted as killed.
ROOT=$(cd "$(dirname "$0")/../../.." && pwd); G=${1:?group}; OUT=${2:?out file}; R="$ROOT/scripts/register"; T="$R/tests"
W=$(mktemp -d "${TMPDIR:-/tmp}/mutops.XXXXXX"); trap 'rm -rf "$W"' EXIT
case "$G" in
 locked) SUT=locked.sh; TEST=test_locked.sh; ENVV=LOCKED;;
 backup) SUT=backup_db.sh; TEST=test_backup_db.sh; ENVV=BACKUP;;
 dump)   SUT=dump.sh; TEST=test_dump.sh; ENVV=DUMP;;
 export) SUT=export.sh; TEST=test_export.sh; ENVV=EXPORT;;
 replay) SUT=replay.sh; TEST=test_replay.sh; ENVV=REPLAY;;
 *) echo "unknown group $G" >&2; exit 2;;
esac
declare -a M=()   # name|file|envvar|sed
add() { M+=("$1|$2|$3|$4"); }
case "$G" in
 locked)
  add no_flock locked.sh LOCKED 's/^  flock 9   # MUT:flock$/  :/'
  add grant_ignored locked.sh LOCKED 's/^  held\(\) \{.*# MUT:grant-ignored$/  held() { return 1; }/'
  add reap_direct locked.sh LOCKED 's#"\$\{CPA_HOST_ENTRY:-\$HOME/.local/bin/cpa-host\}" --exec-approved scripts/release/commit_turn_check.sh --reap#bash scripts/release/commit_turn_check.sh --reap#'
  add test_hooks_honoured locked.sh LOCKED 's/^if \[ -n "\$\{LOCKED_ROOT.*# MUT:test-hooks$/if false; then   # MUT:test-hooks/'
  add scratch_any_subcommand locked.sh LOCKED 's/^if \[ -n "\$\{LOCKED_SCRATCH_DB:-\}" \] \&\& \[ "\$SUB" != import-sql \]; then refuse.*# MUT:scratch-subcommand$/:/'
  add scratch_path_unchecked locked.sh LOCKED 's/^    case "\$canon" in .*# MUT:scratch-canonical$/    :/'
  add import_sha_unchecked locked.sh LOCKED 's/\[ "\$have" = "\$want" \] \|\| refuse import_sha256_mismatch.*# MUT:import-sha$/:/'
  add import_without_backup locked.sh LOCKED 's/^    "\$BACKUP" --record .*# MUT:import-backup$/    :/'
  add scratch_headroom_unset locked.sh LOCKED 's/^    export DISK_HEADROOM_OUT_DIR=.*# MUT:scratch-headroom$/    :/'
  add capture_src_only locked.sh LOCKED 's/\*\) p="\$a";; esac   # MUT:capture-rel/*) return 0;; esac/'
  add scratch_binds_docs locked.sh LOCKED 's/then RARGS=\(--rw .audit\/scratch\); else RARGS=\(--rw docs\); fi   # MUT:scratch-bind/then RARGS=(--rw docs); else RARGS=(--rw docs); fi/'
  add out_run_binds_docs locked.sh LOCKED 's/RARGS=\(--out "\$OUT"\); else RARGS=\(--rw docs\); fi   # MUT:out-bind/RARGS=(--rw docs --out "$OUT"); else RARGS=(--rw docs); fi/'
  add exit_status_lost locked.sh LOCKED 's/^exit "\$RC"   # MUT:exit$/exit 0/'
  add scratch_import_runs_backup locked.sh LOCKED 's/^  if \[ "\$SCRATCH" = 1 \]; then$/  if false; then/'
  add negative_control_comment locked.sh LOCKED 's/^# Exit: the command.s status;/# Exit (control): the command status;/';;
 backup)
  add cp_al_backup backup_db.sh BACKUP 's#^"\$LOCKED" --op-id "\$OP" -- sqlite3 .*\# MUT:backup-method$#"$LOCKED" --op-id "$OP" -- sqlite3 "/src/$DBREL" "PRAGMA wal_checkpoint(TRUNCATE);" ".output /out/source.dump.sql" ".dump" >"$OUT1/step1.out" 2>"$OUT1/step1.err"; cp -al "$DBREL" "$BAKREL"#'
  add failed_backup_kept backup_db.sh BACKUP 's/^fail\(\) \{ rm -f -- "\$BAKREL"   # MUT:fail-keeps-file$/fail() { :/'
  add integrity_unchecked backup_db.sh BACKUP 's/^\[ "\$INTEG" = ok \] \|\| fail .*$/:/'
  add restore_dump_unchecked backup_db.sh BACKUP 's/^cmp -s "\$OUT1\/source.dump.sql" .*fail "restore probe.*$/:/'
  add no_checkpoint backup_db.sh BACKUP 's/"PRAGMA wal_checkpoint\(TRUNCATE\);" "\.backup/".backup/'
  add record_without_rows_check backup_db.sh BACKUP 's/^\[ "\$SROWS" = "\$BROWS" \] \|\| fail .*$/:/'
  add negative_control_comment backup_db.sh BACKUP 's/^# Exit: 0 ok;/# Exit (control): 0 ok;/';;
 dump)
  add pragma_kept dump.sh DUMP 's/ \| grep -v "\^PRAGMA" > "\$out.tmp"   # MUT:pragma-filter/ > "$out.tmp"/'
  add nondeterministic dump.sh DUMP 's/\| grep -v "\^PRAGMA" > "\$out.tmp"   # MUT:pragma-filter/| grep -v "^PRAGMA" > "$out.tmp"; date +%s%N >> "$out.tmp"/'
  add no_checkpoint dump.sh DUMP 's/^sqlite3 "\$db" "PRAGMA wal_checkpoint\(TRUNCATE\);" >\/dev\/null$/:/'
  add paths_unchecked dump.sh DUMP 's/^safe\(\) \{ case "\$1" in .*; esac; return 0; \}$/safe() { return 0; }/'
  add db_suffix_unchecked dump.sh DUMP 's/^  case "\$DB" in docs\/\*\.db\) ;; \*\) refuse .*$/  :/'
  add out_suffix_unchecked dump.sh DUMP 's/^  case "\$OUT" in docs\/\*\.sql\) ;; \*\) refuse .*$/  :/'
  add negative_control_comment dump.sh DUMP 's/^# Exit: 0 ok;/# Exit (control): 0 ok;/';;
 export)
  add check_fingerprint_ignored export.sh EXPORT 's/^\[ "\$fp" = "\$rfp" \] \|\| P=.*# MUT:check-fingerprint$/:/'
  add check_file_hash_ignored export.sh EXPORT 's/^  \[ "\$\(sha256sum "\$f" .*# MUT:check-file-hash$/  :/'
  add check_manifest_ignored export.sh EXPORT 's/^  while read -r h n; do .*# MUT:check-manifest$/  while false; do :; done < \/dev\/null/'
  add diff_gate_removed export.sh EXPORT 's/^case "\$dv" in \*"in sync"\*\) ;; \*\) echo "export: engine diff.*# MUT:diff-gate$/:/'
  add fingerprint_includes_bookkeeping export.sh EXPORT 's/^FPCMD=.*$/FPCMD="sha256sum | cut -d'"'"' '"'"' -f1"/'
  add siblings_marked_written export.sh EXPORT 's/else s2=\$Z; st=skipped_tool_absent; fi/else s2=$Z; st=written; fi/'
  add check_writes_checkpoint export.sh EXPORT 's/^\[ ! -s "\$db-wal" \] \|\| \{ echo "export-check: REFUSED.*$/:/'
  add reconcile_nondeterministic_header reconcile.sh RECONCILE 's/echo "\| Created \| generated \|"/echo "| Created | $(date +%s%N) |"/'
  add reconcile_view_dropped reconcile.sh RECONCILE 's/FROM v_unmapped_entries ORDER BY 1,2,3"$/FROM v_unmapped_entries WHERE 0 ORDER BY 1,2,3"/'
  add negative_control_comment export.sh EXPORT 's/^# Exit: 0 ok; 1 STALE;/# Exit (control): 0 ok; 1 STALE;/';;
 replay)
  add skip_mint_rows replay.sh REPLAY 's/^    if False: continue   # MUT:skip-mints$/    if r.get("ids_minted"): skipped.append({"op_id": r["op_id"], "reason": "mutant"}); continue/'
  add no_collision_refusal replay.sh REPLAY 's/^        if i in remote_ids: refuse\(/        if False: refuse(/'
  add since_unchecked replay.sh REPLAY 's/^    else: refuse\("since_not_found".*$/    else: idx = -1/'
  add gate_unchecked replay.sh REPLAY 's/^if g.returncode != 0 or not any\(l.startswith\("GATE OK"\) for l in g.stdout.splitlines\(\)\): refuse/if False: refuse/'
  add failed_rows_replayed replay.sh REPLAY 's/^    if r.get\("exit"\) != 0: skipped.append.*$/    pass/'
  add input_not_rebound replay.sh REPLAY 's/^        a = argv\[int\(k\)\]; argv\[int\(k\)\] = .*$/        pass/'
  add regenerated_rows_replayed replay.sh REPLAY 's/^    if "# register-regenerate:" in txt: skipped.append.*$/    pass/'
  add db_arg_not_rewritten replay.sh REPLAY 's/a.replace\("\/src\/docs\/workable_items.db", "\/out\/replay.db"\)/a/'
  add negative_control_comment replay.sh REPLAY 's/^# Exit: 0 ok; 2 usage; 20 refused\./# Exit (control): 0 ok; 2 usage; 20 refused./';;
esac
# build mutants inside a mirrored tree so REAL_ROOT-relative lookups (build/containers/images.lock.yaml) resolve
for e in "${M[@]}"; do IFS='|' read -r name file envv sedx <<<"$e"
  d="$W/t/$name/scripts/register"; mkdir -p "$d"; ln -s "$ROOT/build" "$W/t/$name/build"; ln -s "$ROOT/scripts/containers" "$W/t/$name/scripts/containers"
  sed -E "$sedx" "$R/$file" >"$d/$file"
  cmp -s "$R/$file" "$d/$file" && { echo "mutant $name is identical to $file (marker or anchor missing)" >&2; exit 3; }
  case "$file" in *.sh) bash -n "$d/$file" 2>/dev/null || { echo "mutant $name does not parse" >&2; exit 3; };; esac
  if [ "$file" = replay.sh ]; then python3 -I -c 'import sys;s=open(sys.argv[1]).read();b=s.split("exec python3 -I - <<'"'"'PY'"'"'\n")[1].rsplit("\nPY\n",1)[0];compile(b,"m","exec")' "$d/$file" 2>/dev/null || { echo "mutant $name python does not parse" >&2; exit 3; }; fi
  for s in $R/*.sh; do [ -e "$d/$(basename "$s")" ] || ln -s "$s" "$d/$(basename "$s")"; done
done
[ -z "${MUT_DRY:-}" ] || { echo "group $G: ${#M[@]} mutants generated, all differ from their source"; exit 0; }
{ echo "# identity: group=$G utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)"
  echo "# runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1) test_sha256=$(sha256sum "$T/$TEST" | cut -d' ' -f1) sut_sha256=$(sha256sum "$R/$SUT" | cut -d' ' -f1)"
  echo "# mutant<TAB>killed<TAB>first failing check of $TEST"; } >"$OUT"
runm() { local name=$1 file=$2 envv=$3 d="$W/t/$1/scripts/register" r rc
  mkdir -p "$W/s.$name"; local try
  for try in 1 2 3; do r=$(env REG_SCRATCH="$W/s.$name" "$envv=$d/$file" bash "$T/$TEST" 2>&1); rc=$?
    # an environmental refusal in the FIRST failing line (shared host: disk headroom, image lock, memory budget) is not a verdict: retry, at most twice
    [ $rc -ne 0 ] && printf '%s' "$r" | grep -m1 '^FAIL' | grep -Eq "$ENV_RE" || break; done
  [ $rc -ne 0 ] && k=yes || k=NO; printf '%s\t%s\t%s\n' "$name" "$k" "$(printf '%s' "$r" | grep -m1 '^FAIL' | cut -c1-110)" >"$W/$name.res"; }
ENV_RE="disk_below_headroom|disk_free_unreadable|lock_unreadable|memory_budget_unavailable|image_not_present_locally|cpu_budget_unavailable"
JOBS=${MUT_JOBS:-3}; running=0
for e in "${M[@]}"; do IFS='|' read -r name file envv sedx <<<"$e"
  runm "$name" "$file" "$envv" & running=$((running+1)); [ "$running" -ge "$JOBS" ] && { wait -n; running=$((running-1)); }
done; wait
for e in "${M[@]}"; do IFS='|' read -r name _ <<<"$e"; cat "$W/$name.res" >>"$OUT"; done
# golden-good control: the unmodified script passes
mkdir -p "$W/s.golden"; for try in 1 2 3; do g=$(env REG_SCRATCH="$W/s.golden" bash "$T/$TEST" 2>&1); grc=$?; [ $grc -ne 0 ] && printf '%s' "$g" | grep -Eq "$ENV_RE" || break; done
printf 'GOLDEN_GOOD_CONTROL\t%s\t%s\n' "$([ $grc -eq 0 ] && echo pass || echo FAIL)" "$(printf '%s' "$g" | tail -n1)" >>"$OUT"
# negative control: the no-op comment mutant must NOT be killed
EQF="$T/equivalent_ops_mutants.tsv"
awk -F'\t' -v g="$G" 'NR==FNR{if($0!~/^#/ && NF>=3 && $1==g) r[$2]=$3; next} !/^#/ && ($1 in r) && $2=="NO"{print $1"\tEQUIV\t"r[$1]; next} {print}' "$EQF" "$OUT" >"$OUT.eq" 2>/dev/null && mv "$OUT.eq" "$OUT"
nc=$(awk -F'\t' '$1=="negative_control_comment"{print $2}' "$OUT")
surv=$(awk -F'\t' '!/^#/ && $2=="NO" && $1!="negative_control_comment"' "$OUT" | wc -l); eqn=$(awk -F'\t' '!/^#/ && $2=="EQUIV"' "$OUT" | wc -l)
n=$(( ${#M[@]} - 1 )); ncok=$([ "$nc" = NO ] && echo ok || echo BROKEN)
echo "group=$G mutants=$n killed=$((n-surv-eqn)) equivalent_reviewed=$eqn survived_unreviewed=$surv golden_good=$([ $grc -eq 0 ] && echo pass || echo FAIL) negative_control=$ncok" | tee "$OUT.summary"
[ $surv -eq 0 ] && [ $grc -eq 0 ] && [ "$ncok" = ok ]
