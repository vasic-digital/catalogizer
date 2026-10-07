#!/usr/bin/env bash
# mutate_reverify_gate.sh - G-GATE paired mutations of the completion gate (T180): every mutant of gate.sh (the --completion
# step) and every mutant of register_ext.sql (the ('v_reverify_queue','view_not_done') registry row, docs/04 §12.3 step 3) must
# make test_reverify_gate.sh FAIL. Negative control (11.4.201 / 1.1): the UNMUTATED gate and a comment-only (noop) mutant must
# PASS the same test, else a kill proves nothing; the run exits 5 when a control fails. A mutant identical to its source is an
# ERROR (exit 3). Usage: mutate_reverify_gate.sh <out.tsv>   exit 1 when a mutant survives.
# Output: <out.tsv> (mutant<TAB>killed<TAB>first failing check) and <out.tsv>.summary.
ROOT=$(cd "$(dirname "$0")/../../.." && pwd); OUT=${1:?out file}
G="$ROOT/scripts/register/gate.sh"; X="$ROOT/scripts/register/register_ext.sql"; T="$ROOT/scripts/register/tests/test_reverify_gate.sh"
W=$(mktemp -d "${REG_SCRATCH:-${TMPDIR:-/tmp}}/mutrv.XXXXXX"); trap 'rm -rf "$W"' EXIT
mkdir -p "$W/g" "$W/x"
mk()  { sed -E "$2" "$G" > "$W/g/$1.sh"; cmp -s "$G" "$W/g/$1.sh" && { echo "mutant $1 is identical to gate.sh (marker missing)" >&2; exit 3; }
        bash -n "$W/g/$1.sh" 2>/dev/null || { echo "mutant $1 does not parse (it would be killed for the wrong reason)" >&2; exit 3; }; }
mkx() { sed -E "$2" "$X" > "$W/x/$1.sql"; cmp -s "$X" "$W/x/$1.sql" && { echo "ext mutant $1 is identical to register_ext.sql (marker missing)" >&2; exit 3; }; }
# --- gate.sh mutants: one per guard / branch of step_not_done and the completion verdict
mk skip_step_not_done        's/^\[ \$COMPLETION = 1 \] && step_not_done$/:/'
mk completion_flag_ignored   's/^  --completion\) COMPLETION=1; shift;;/  --completion) shift;;/'
mk completion_always_on      's/ALLOW_URL=0; COMPLETION=0/ALLOW_URL=0; COMPLETION=1/'
mk guard_nd_regex            '/# GUARD_ND_REGEX$/d'
mk guard_nd_isview           '/# GUARD_ND_ISVIEW$/d'
mk guard_nd_numeric          '/# GUARD_ND_NUMERIC$/d'
mk guard_nd_dbrow_regex      '/# GUARD_ND_DBROW_REGEX$/d'
mk guard_nd_exit             '/# GUARD_ND_EXIT$/d'
mk list_from_db              "s/^(  views=)\\\$\\(qr \"SELECT name FROM reg_gate_checks WHERE kind='view_not_done'/\\1\$(q \"SELECT name FROM reg_gate_checks WHERE kind='view_not_done'/"
mk list_wrong_kind           "s/WHERE kind='view_not_done' ORDER BY name/WHERE kind='view_empty' ORDER BY name/"
mk empty_list_ok             '/reference gate registry holds no view_not_done row/d'
mk threshold_gt1             's/if \[ "\$n" != 0 \]; then echo "NOT DONE/if [ "$n" -gt 1 ]; then echo "NOT DONE/'
mk not_done_line_dropped     's/ echo "NOT DONE \$v rows=\$n";//'
mk views_not_counted         's/ ND_VIEWS=\$\(\(ND_VIEWS\+1\)\);//'
mk rows_off_by_one           's/ND_ROWS=\$\(\(ND_ROWS\+n\)\)/ND_ROWS=$((ND_ROWS+1))/'
mk exit_code_zero            's/exit 4; fi   # GUARD_ND_EXIT/exit 0; fi   # GUARD_ND_EXIT/'
mk exit_code_one             's/exit 4; fi   # GUARD_ND_EXIT/exit 1; fi   # GUARD_ND_EXIT/'
mk completion_fail_ignored   's/^if \[ \$rc != 0 \]; then echo "GATE FAILED"; exit 1; fi$/if [ $rc != 0 ] \&\& [ $COMPLETION = 0 ]; then echo "GATE FAILED"; exit 1; fi/'
mk info_line_dropped         '/echo "INFO not_done checked=\$checked"/d'
mk partial_prefix_gate       's/^PFX=GATE; \[ \$COMPLETION = 1 \] && PFX=COMPLETION$/PFX=GATE/'
mk ok_line_is_gate_ok        's/echo "COMPLETION OK"; exit 0/echo "GATE OK"; exit 0/'
mk not_done_outranks_fail    's/^if \[ \$rc != 0 \]; then echo "GATE FAILED"; exit 1; fi$/if [ $COMPLETION = 1 ] \&\& [ "$ND_VIEWS" -gt 0 ]; then echo "COMPLETION NOT DONE views=$ND_VIEWS rows=$ND_ROWS"; exit 4; fi\nif [ $rc != 0 ]; then echo "GATE FAILED"; exit 1; fi/'
# --- register_ext.sql mutants: the registry row (G-GATE of T180) and its neighbours
mkx ext_row_removed          "s/\\('v_reverify_queue','view_not_done'\\),//"
mkx ext_row_kind_report      "s/\\('v_reverify_queue','view_not_done'\\)/('v_reverify_queue','view_report')/"
mkx ext_row_kind_empty       "s/\\('v_reverify_queue','view_not_done'\\)/('v_reverify_queue','view_empty')/"
mkx ext_row_other_view       "s/\\('v_reverify_queue','view_not_done'\\)/('v_stale_tracker_sync','view_not_done')/"
mkx ext_check_drops_kind     "s/,'view_report','view_not_done'\\)\\)\\);/,'view_report'))); /"
# --- negative controls (must PASS the test: a gate that is unmutated, or changed in a comment only)
cp "$G" "$W/g/CONTROL_unmutated.sh"
sed -E 's/^# gate.sh - register gate/# gate.sh (control, comment only) - register gate/' "$G" > "$W/g/CONTROL_noop_comment.sh"
cmp -s "$G" "$W/g/CONTROL_noop_comment.sh" && { echo "control noop is identical" >&2; exit 3; }
[ -z "${MUT_DRY:-}" ] || { echo "generated $(ls "$W"/g/*.sh | wc -l) gate mutants + $(ls "$W"/x/*.sql | wc -l) ext mutants, all differ from their source"; exit 0; }
{ echo "# identity: task=T180 utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)"
  echo "# gate_sha256=$(sha256sum "$G" | cut -d' ' -f1) ext_sha256=$(sha256sum "$X" | cut -d' ' -f1) test_sha256=$(sha256sum "$T" | cut -d' ' -f1) runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1)"
  echo "# mutant<TAB>killed<TAB>first failing check of test_reverify_gate.sh (CONTROL_* rows must read NO)"; } > "$OUT"
JOBS=${MUT_JOBS:-3}; running=0
run1() {  # kind file
  local b; b=$(basename "$2"); b=${b%.*}
  mkdir -p "$W/s.$b"
  if [ "$1" = g ]; then r=$(REG_SCRATCH="$W/s.$b" GATE="$2" bash "$T" 2>&1); rc=$?
  else r=$(REG_SCRATCH="$W/s.$b" REG_EXT_SQL="$2" bash "$T" 2>&1); rc=$?; fi
  [ $rc -ne 0 ] && k=yes || k=NO
  printf '%s\t%s\t%s\n' "$b" "$k" "$(printf '%s' "$r" | grep -m1 '^FAIL' | cut -c1-110)" > "$W/$b.res"; }
for f in "$W"/g/*.sh; do run1 g "$f" & running=$((running+1)); [ "$running" -ge "$JOBS" ] && { wait -n; running=$((running-1)); }; done
for f in "$W"/x/*.sql; do run1 x "$f" & running=$((running+1)); [ "$running" -ge "$JOBS" ] && { wait -n; running=$((running-1)); }; done
wait
for f in "$W"/g/*.sh "$W"/x/*.sql; do b=$(basename "$f"); b=${b%.*}; cat "$W/$b.res" >> "$OUT"; done
ctl_bad=$(awk -F'\t' '!/^#/ && $1 ~ /^CONTROL_/ && $2!="NO"' "$OUT" | wc -l)
surv=$(awk -F'\t' '!/^#/ && $1 !~ /^CONTROL_/ && $2=="NO"' "$OUT" | wc -l)
n=$(awk -F'\t' '!/^#/ && $1 !~ /^CONTROL_/' "$OUT" | wc -l)
echo "reverify gate mutants=$n killed=$((n-surv)) survived=$surv controls_failed=$ctl_bad" | tee "$OUT.summary"
[ "$ctl_bad" -eq 0 ] || { echo "NEGATIVE CONTROL FAILED: a control was killed, the kills prove nothing" >&2; exit 5; }
[ $surv -eq 0 ]
