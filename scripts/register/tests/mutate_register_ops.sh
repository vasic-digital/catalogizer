#!/usr/bin/env bash
# mutate_register_ops.sh - paired mutations of the WP-06 register operation scripts (T064, T064a, T066, T067, T067a).
# Usage: mutate_register_ops.sh <locked|backup|dump|export|replay> <out.txt>     exit 1 when a mutant survives unreviewed.
# Every mutant is a sed edit of one marked line (# MUT:<name>) or one anchored statement of the script under test, run against that group's test with the
# mutant substituted through the group's env variable. A mutant identical to its source is an ERROR (exit 3), a mutant that does not parse is an ERROR.
# Each group also runs the GOLDEN-GOOD control (the unmodified script, must pass) and a NEGATIVE-CONTROL (a mutant that is a no-op comment edit must
# also pass: it proves the runner does not kill mutants for the wrong reason). Reviewed-equivalent survivors are listed in equivalent_ops_mutants.tsv
# and reported EQUIV, never counted as killed.
# Scoring (WF10 M1): a mutant is run against the group's own test first and, only while it survives, against the follow-up tests listed for the group (TESTS:
# file|sections|NO_E2E): replay depends on the journal fields locked.sh writes, so the locked.sh mutants are also scored against test_replay.sh, and every group is also
# scored against the sections of test_fix_r1.sh that pin the WF10 review fixes. A mutant killed by a follow-up test records that test's name in the transcript.
ROOT=$(cd "$(dirname "$0")/../../.." && pwd); G=${1:?group}; OUT=${2:?out file}; R="$ROOT/scripts/register"; T="$R/tests"
W=$(mktemp -d "${TMPDIR:-/tmp}/mutops.XXXXXX"); trap 'rm -rf "$W"' EXIT
case "$G" in
 locked) SUT=locked.sh; TEST=test_locked.sh; ENVV=LOCKED; TESTS=("test_replay.sh||" "test_fix_r1.sh|F3 F4 F5 F6 F13 F11|1");;
 backup) SUT=backup_db.sh; TEST=test_backup_db.sh; ENVV=BACKUP; TESTS=("test_fix_r1.sh|F7 F14 F8 M6|1");;
 dump)   SUT=dump.sh; TEST=test_dump.sh; ENVV=DUMP; TESTS=("test_fix_r1.sh|F8 F10|1");;
 export) SUT=export.sh; TEST=test_export.sh; ENVV=EXPORT; TESTS=("test_fix_r1.sh|F8 F9 F12|1");;
 replay) SUT=replay.sh; TEST=test_replay.sh; ENVV=REPLAY; TESTS=("test_fix_r1.sh|F1 F2 F8|1");;
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
  add RMa_db_inputs_copied locked.sh LOCKED 's/^  case "\$p" in \*\.db\|\*\.db-wal\|\*\.db-shm\|\*\.sqlite\|\*\.sqlite3\) return 0;; esac   # MUT:capture-db-skip$/  :/'
  add signal_not_forwarded locked.sh LOCKED 's/^  if our_child "\$RUNP_PID"; then kill -s "\$1" "\$RUNP_PID" 2>\/dev\/null; fi   # MUT:signal-forward$/  :/'
  add lock_claim_unchecked locked.sh LOCKED 's/^  lock_claim_proven \|\| refuse lock_claim_invalid .*# MUT:lock-claim$/  :/'
  add journal_precheck_removed locked.sh LOCKED 's/^if \[ -e "\$JOURNAL" \].*# MUT:journal-precheck$/:/'
  add pending_marker_removed locked.sh LOCKED 's/^  \( set -o noclobber; printf .*# MUT:pending-marker$/  :/'
  add reaper_unbounded locked.sh LOCKED 's/timeout "\$REAPER_TIMEOUT" "\$\{CPA_HOST_ENTRY/"${CPA_HOST_ENTRY/'
  add reaper_inherits_lock_fd locked.sh LOCKED 's/<\/dev\/null 9>&- \|\| true   # MUT:reap-direct/<\/dev\/null || true   # MUT:reap-direct/'
  add snapshot_status_ignored locked.sh LOCKED 's/^  if \[ "\$rc" -ne 0 \] \|\| \[ ! -s "\$tmp" \]; then rm -f "\$tmp"; echo "\?fail"; return 0; fi/  :/'
  add negative_control_comment locked.sh LOCKED 's/^# Exit: the command.s status;/# Exit (control): the command status;/';;
 backup)
  add cp_al_backup backup_db.sh BACKUP 's#^"\$LOCKED" --op-id "\$OP" -- sqlite3 .*\# MUT:backup-method$#"$LOCKED" --op-id "$OP" -- sqlite3 "/src/$DBREL" "PRAGMA wal_checkpoint(TRUNCATE);" ".output /out/source.dump.sql" ".dump" ".output stdout" ".shell sha256sum /src/$DBREL >/out/source.sha256" >"$OUT1/step1.out" 2>"$OUT1/step1.err"; rm -f "$BAKREL"; cp -al "$DBREL" "$BAKREL"#'
  add failed_backup_kept backup_db.sh BACKUP 's/^fail\(\) \{ \[ "\$OWN" = 1 \] \&\& rm -f -- "\$BAKREL"   # MUT:fail-keeps-file$/fail() { :/'
  add integrity_unchecked backup_db.sh BACKUP 's/^\[ "\$INTEG" = ok \] \|\| fail .*$/:/'
  add restore_dump_unchecked backup_db.sh BACKUP 's/^cmp -s "\$OUT1\/source.dump.sql" .*fail "restore probe.*$/:/'
  add no_checkpoint backup_db.sh BACKUP 's/"PRAGMA wal_checkpoint\(TRUNCATE\);" "\.backup/".backup/'
  add record_without_rows_check backup_db.sh BACKUP 's/^\[ "\$SROWS" = "\$BROWS" \] \|\| fail .*$/:/'
  add backup_name_not_unique backup_db.sh BACKUP 's/^BAKREL="\$DBREL\.bak-\$UTC-\$\$-\$NS"$/BAKREL="$DBREL.bak-$UTC"/'
  add source_hash_after_unlock backup_db.sh BACKUP 's/^(BSHA="\$\(sha256sum -- "\$BAKREL" \| cut -d. . -f1\)")$/\1; SSHA="$(sha256sum -- "$DBREL" | cut -d" " -f1)"/'
  add locked_hook_outside_test_mode backup_db.sh BACKUP 's/^if \[ "\$\{LOCKED_TEST_MODE:-\}" = 1 \]; then LOCKED=(.*)   # MUT:hook-gate$/if true; then LOCKED=\1/'
  add integrity_gate_both_layers_removed backup_db.sh BACKUP 's/^  \[ "\$ic" = ok \] \|\| exit 0$/  :/;s/^\[ "\$INTEG" = ok \] \|\| fail .*$/:/'
  add negative_control_comment backup_db.sh BACKUP 's/^# Exit: 0 ok;/# Exit (control): 0 ok;/';;
 dump)
  add pragma_kept dump.sh DUMP 's/ \| grep -v "\^PRAGMA" > "\$out.tmp"   # MUT:pragma-filter/ > "$out.tmp"/'
  add nondeterministic dump.sh DUMP 's@ \| grep -v "\^PRAGMA" > "\$out.tmp"@ | { date +%s%N | sed "s/^/-- /"; grep -v "^PRAGMA"; } > "$out.tmp"@'
  add no_checkpoint dump.sh DUMP 's/^r=\$\(sqlite3 "\$db" "PRAGMA wal_checkpoint\(TRUNCATE\);"\) \|\| .*$/r="0|0|0"/'
  add paths_unchecked dump.sh DUMP 's/^safe\(\) \{ case "\$1" in .*; esac; return 0; \}$/safe() { return 0; }/'
  add db_suffix_unchecked dump.sh DUMP 's/^  case "\$DB" in docs\/\*\.db\) ;; \*\) refuse .*$/  :/'
  add out_suffix_unchecked dump.sh DUMP 's/^  case "\$OUT" in docs\/\*\.sql\) ;; \*\) refuse .*$/  :/'
  add checkpoint_checks_removed dump.sh DUMP 's/^case "\$r" in 0\\\|\*\) ;; .*# MUT:checkpoint-result$/:/;s/^\[ ! -s "\$db-wal" \] \|\| \{ echo "dump: REFUSED.*$/:/'
  add dump_output_unchecked dump.sh DUMP 's/^  \[ -s "\$HOSTOUT" \] .*# MUT:output-check$/  :/'
  add locked_hook_outside_test_mode dump.sh DUMP 's/^if \[ "\$\{LOCKED_TEST_MODE:-\}" = 1 \]; then LOCKED=(.*)   # MUT:hook-gate$/if true; then LOCKED=\1/'
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
  add check_manifest_names_unchecked export.sh EXPORT 's/^  \[ "\$want" = "\$have" \] \|\| P=.*# MUT:check-manifest-names$/  :/'
  add check_manifest_missing_ok export.sh EXPORT 's/^else P="\$P; export-manifest.sha256 is missing"; fi$/else :; fi/'
  add check_regenerate_removed export.sh EXPORT 's/^  for n in @CSVS@; do cmp -s .*# MUT:check-regenerate$/  :/'
  add check_verdict_unchecked export.sh EXPORT 's/^  if \[ \$rc -eq 0 \] \&\& ! printf .*# MUT:verdict-check$/  :/'
  add export_output_unchecked export.sh EXPORT 's/^  \[ -z "\$miss" \] \|\| \{ echo "export: FAILED.*# MUT:output-check$/  :/'
  add locked_hook_outside_test_mode export.sh EXPORT 's/^if \[ "\$\{LOCKED_TEST_MODE:-\}" = 1 \]; then LOCKED=(.*)   # MUT:hook-gate$/if true; then LOCKED=\1/'
  add reconcile_empty_header_dropped reconcile.sh RECONCILE 's/^  else printf .%s\\n. "\$\{HDR\[\$n\]\}" >"\$OUT\/\.\$n\.csv\.tmp\.\$\$"; rows=0; fi   # MUT:empty-header$/  else : >"$OUT\/.$n.csv.tmp.$$"; rows=-1; fi/'
  add negative_control_comment export.sh EXPORT 's/^# Exit: 0 ok; 1 STALE,/# Exit (control): 0 ok; 1 STALE,/';;
 replay)
  add skip_mint_rows replay.sh REPLAY 's/^    if False: continue   # MUT:skip-mints$/    if r.get("ids_minted"): skipped.append({"op_id": r["op_id"], "reason": "mutant"}); continue/'
  add no_collision_refusal replay.sh REPLAY 's/^        if i in remote_ids: refuse\(/        if False: refuse(/'
  add since_unchecked replay.sh REPLAY 's/^    else: refuse\("since_not_found".*$/    else: idx = -1/'
  add gate_unchecked replay.sh REPLAY 's/^if g.returncode != 0 or not any\(l.startswith\("GATE OK"\) for l in g.stdout.splitlines\(\)\): refuse/if False: refuse/'
  add failed_rows_replayed replay.sh REPLAY 's/^    if r.get\("exit"\) != 0: skipped.append.*$/    pass/'
  add input_not_rebound replay.sh REPLAY 's/^        a = argv\[int\(k\)\]; argv\[int\(k\)\] = .*$/        pass/'
  add regenerated_rows_replayed replay.sh REPLAY 's/^    if "# register-regenerate:" in txt: skipped.append.*$/    pass/'
  add db_arg_not_rewritten replay.sh REPLAY 's/a.replace\("\/src\/docs\/workable_items.db", "\/out\/replay.db"\)/a/'
  add base_row_any_mode replay.sh REPLAY 's/^    if is_reg\(r\) and r\.get\("db_sha_after"\) == SINCE and not r\.get\("wal_bytes_after"\): idx = i   # MUT:base-row/    if r.get("db_sha_after") == SINCE: idx = i   # MUT:base-row/'
  add ids_unknown_unchecked replay.sh REPLAY 's/^    if r\.get\("ids_snapshot", "ok"\) != "ok": refuse\(.*# MUT:ids-unknown$/    pass/'
  add RMb_id_lost_unchecked replay.sh REPLAY 's/^if missing: refuse\(/if False: refuse(/'
  add RMc_input_sha_unchecked replay.sh REPLAY 's/ or sha_file\(src\) != h:/:/'
  add RMd_wal_rows_skipped replay.sh REPLAY 's/ and not r\.get\("wal_bytes_after"\) and not r\.get\("wal_bytes_before"\):/:/'
  add pending_ops_unchecked replay.sh REPLAY 's/^if os\.path\.isdir\(pend\) and os\.listdir\(pend\): refuse\(/if False: refuse(/'
  add locked_hook_outside_test_mode replay.sh REPLAY 's/^if \[ "\$\{LOCKED_TEST_MODE:-\}" = 1 \]; then ROOT=(.*); else ROOT="\$REAL_ROOT"; unset LOCKED BACKUP DUMP EXPORT; fi$/ROOT=\1/'
  add negative_control_comment replay.sh REPLAY 's/^# Exit: 0 ok; 2 usage; 20 refused\./# Exit (control): 0 ok; 2 usage; 20 refused./';;
esac
# build mutants inside a mirrored tree so REAL_ROOT-relative lookups (build/containers/images.lock.yaml) resolve
for e in "${M[@]}"; do IFS='|' read -r name file envv sedx <<<"$e"
  d="$W/t/$name/scripts/register"; mkdir -p "$d"; ln -s "$ROOT/build" "$W/t/$name/build"; ln -s "$ROOT/scripts/containers" "$W/t/$name/scripts/containers"
  sed -E "$sedx" "$R/$file" >"$d/$file"; chmod --reference="$R/$file" "$d/$file"   # backup_db.sh and replay.sh execute locked.sh directly: a mutant must keep the executable bit
  cmp -s "$R/$file" "$d/$file" && { echo "mutant $name is identical to $file (marker or anchor missing)" >&2; exit 3; }
  case "$file" in *.sh) bash -n "$d/$file" 2>/dev/null || { echo "mutant $name does not parse" >&2; exit 3; };; esac
  if [ "$file" = replay.sh ]; then python3 -I -c 'import sys;s=open(sys.argv[1]).read();b=s.split("exec python3 -I - <<'"'"'PY'"'"'\n")[1].rsplit("\nPY\n",1)[0];compile(b,"m","exec")' "$d/$file" 2>/dev/null || { echo "mutant $name python does not parse" >&2; exit 3; }; fi
  for s in $R/*.sh; do [ -e "$d/$(basename "$s")" ] || ln -s "$s" "$d/$(basename "$s")"; done
done
[ -z "${MUT_DRY:-}" ] || { echo "group $G: ${#M[@]} mutants generated, all differ from their source"; exit 0; }
{ echo "# identity: group=$G utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)"
  echo "# runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1) test_sha256=$(sha256sum "$T/$TEST" | cut -d' ' -f1) sut_sha256=$(sha256sum "$R/$SUT" | cut -d' ' -f1) followup_tests=${TESTS[*]} fix_r1_sha256=$(sha256sum "$T/test_fix_r1.sh" | cut -d' ' -f1)"
  echo "# mutant<TAB>killed<TAB>first failing check of $TEST"; } >"$OUT"
runm() { local name=$1 file=$2 envv=$3 d="$W/t/$1/scripts/register" r rc tst only noe by="$TEST"
  mkdir -p "$W/s.$name"; local try
  for spec in "$TEST||" "${TESTS[@]}"; do IFS='|' read -r tst only noe <<<"$spec"; by="$tst${only:+ [$only]}"
    for try in 1 2 3; do r=$(env REG_SCRATCH="$W/s.$name" ${only:+ONLY="$only"} ${noe:+NO_E2E=1} "$envv=$d/$file" bash "$T/$tst" 2>&1); rc=$?
      # an environmental refusal in the FIRST failing line (shared host: disk headroom, image lock, memory budget) is not a verdict: retry, at most twice
      [ $rc -ne 0 ] && printf '%s' "$r" | grep -m1 '^FAIL' | grep -Eq "$ENV_RE" || break; done
    [ $rc -ne 0 ] && break
  done
  [ $rc -ne 0 ] && k=yes || k=NO; printf '%s\t%s\t%s\n' "$name" "$k" "$by: $(printf '%s' "$r" | grep -m1 '^FAIL' | cut -c1-110)" >"$W/$name.res"; }
ENV_RE="disk_below_headroom|disk_free_unreadable|lock_unreadable|memory_budget_unavailable|image_not_present_locally|cpu_budget_unavailable"
JOBS=${MUT_JOBS:-3}; running=0
for e in "${M[@]}"; do IFS='|' read -r name file envv sedx <<<"$e"
  runm "$name" "$file" "$envv" & running=$((running+1)); [ "$running" -ge "$JOBS" ] && { wait -n; running=$((running-1)); }
done; wait
for e in "${M[@]}"; do IFS='|' read -r name _ <<<"$e"; cat "$W/$name.res" >>"$OUT"; done
# golden-good control: the unmodified script passes
mkdir -p "$W/s.golden"; for spec in "$TEST||" "${TESTS[@]}"; do IFS='|' read -r tst only noe <<<"$spec"
  for try in 1 2 3; do g=$(env REG_SCRATCH="$W/s.golden" ${only:+ONLY="$only"} ${noe:+NO_E2E=1} bash "$T/$tst" 2>&1); grc=$?; [ $grc -ne 0 ] && printf '%s' "$g" | grep -Eq "$ENV_RE" || break; done
  [ $grc -eq 0 ] || break; done
printf 'GOLDEN_GOOD_CONTROL\t%s\t%s\n' "$([ $grc -eq 0 ] && echo pass || echo FAIL)" "$(printf '%s' "$g" | tail -n1)" >>"$OUT"
# negative control: the no-op comment mutant must NOT be killed
EQF="$T/equivalent_ops_mutants.tsv"
awk -F'\t' -v g="$G" 'NR==FNR{if($0!~/^#/ && NF>=3 && $1==g) r[$2]=$3; next} !/^#/ && ($1 in r) && $2=="NO"{print $1"\tEQUIV\t"r[$1]; next} {print}' "$EQF" "$OUT" >"$OUT.eq" 2>/dev/null && mv "$OUT.eq" "$OUT"
nc=$(awk -F'\t' '$1=="negative_control_comment"{print $2}' "$OUT")
surv=$(awk -F'\t' '!/^#/ && $2=="NO" && $1!="negative_control_comment"' "$OUT" | wc -l); eqn=$(awk -F'\t' '!/^#/ && $2=="EQUIV"' "$OUT" | wc -l)
n=$(( ${#M[@]} - 1 )); ncok=$([ "$nc" = NO ] && echo ok || echo BROKEN)
echo "group=$G mutants=$n killed=$((n-surv-eqn)) equivalent_reviewed=$eqn survived_unreviewed=$surv golden_good=$([ $grc -eq 0 ] && echo pass || echo FAIL) negative_control=$ncok" | tee "$OUT.summary"
[ $surv -eq 0 ] && [ $grc -eq 0 ] && [ "$ncok" = ok ]
