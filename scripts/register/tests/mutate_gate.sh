#!/usr/bin/env bash
# mutate_gate.sh - G-GATE paired mutations (T063): every mutant copy of gate.sh must make test_gate.sh FAIL.
# Mutants delete one step call or one marked guard line (# GUARD_*, # LIST_SOURCE), force GATE OK, or remove one
# comparison of step_reference_diff / step_evidence_rehash (WF2 review I-8: the object-type half of the schema
# comparison had no golden-bad fixture). A mutant that is identical to gate.sh is an ERROR (exit 3).
# Usage: mutate_gate.sh <out.tsv>   exit 1 when a mutant survives. <out.tsv> and <out.tsv>.summary are written here.
ROOT=$(cd "$(dirname "$0")/../../.." && pwd); OUT=${1:?out file}
G="$ROOT/scripts/register/gate.sh"; T="$ROOT/scripts/register/tests/test_gate.sh"; W=$(mktemp -d); trap 'rm -rf "$W"' EXIT
mk() { sed -E "$2" "$G" > "$W/$1.sh"; cmp -s "$G" "$W/$1.sh" && { echo "mutant $1 is identical to gate.sh (marker missing)" >&2; exit 3; }
  bash -n "$W/$1.sh" 2>/dev/null || { echo "mutant $1 does not parse (it would be killed for the wrong reason)" >&2; exit 3; }; }
for st in engine_validate integrity fk missing_objects view_empty reference_diff engine_version evidence_rehash; do mk "skip_$st" "s/^step_$st( .*)?\$/:/"; done
mk guard_regex       '/# GUARD_REGEX$/d'
mk guard_isview      '/# GUARD_ISVIEW$/d'
mk guard_dbrow_regex '/# GUARD_DBROW_REGEX$/d'
mk list_from_db      's/^(  views=)\$\(qr /\1$(q /'
mk always_ok         's/^if \[ \$rc != 0 \]; then echo "GATE FAILED"; exit 1; fi$/echo "GATE OK"; exit 0/'
# the schema comparison (I-1, I-8)
mk no_unexpected_object_check   '/unexpected schema object in the database under test/d'
mk no_missing_object_check      '/schema object of the reference missing in the database under test/d'
mk defs_only_triggers_and_views 's/\(\$2 in x\) \|\| \(\$1=="trigger"\) \|\| \(\$1=="view"\)/($1=="trigger") || ($1=="view")/'   # G1 of the reviewer: tables and indexes never compared
mk defs_ignore_indexes          's/\(\$2 in x\) \|\| \(\$1=="trigger"\)/(($2 in x) \&\& $1!="index") || ($1=="trigger")/'      # G2 of the reviewer: index definitions ignored
mk defs_engine_objects_ignored  's/ \|\| \(\$1=="trigger"\) \|\| \(\$1=="view"\)//'
mk no_def_diff                  '/if ! diff "\$W\/a.def" "\$W\/b.def"/,/fi$/d'
mk no_seed_diff                 '/diff "\$W\/a.seed" "\$W\/b.seed"/d'
mk names_only_by_type_and_name  's/cut -d.\|. -f1-3 "\$W\/a.all"/cut -d"|" -f2 "$W\/a.all"/'
# engine version (m-4)
mk engine_version_not_compared  's/\[ -n "\$need" \] && \[ "\$have" = "\$need" \] \|\|/true ||/'
# evidence re-hash (I-3)
mk rehash_absent_file_ok        's/if \[ ! -f "\$f" \]; then fail/if false; then fail/'
mk rehash_hash_not_compared     's/\[ "\$h" = "\$sha" \] \|\| \{/true || {/'
mk rehash_size_not_compared     's/\[ "\$sz" = "\$size" \] \|\| fail/true || fail/'
mk rehash_tab_path_ok           's/\[ "\$bad" = 0 \] \|\| fail/true || fail/'
mk rehash_url_counted_verified  's/skipped=\$\(\(skipped\+1\)\); else fail/n=$((n+1)); else fail/'
# WF3 round 4: the object filter boundary (I1, W4) and the URL evidence rule (I2, W5)
mk object_filter_like_wildcard   "s/GLOB 'sqlite_stat\\[1-4\\]'\"\$/LIKE 'sqlite_stat_'\"/;s/(STAT_FILTER=.*name )GLOB 'sqlite_stat\\[1-4\\]'\)/\\1LIKE 'sqlite_stat_')/"
mk object_filter_prefix_widened  "s/(STAT_FILTER=.*name )GLOB 'sqlite_stat\\[1-4\\]'/\\1GLOB 'sqlite_*'/"
mk object_filter_name_only       "s/^STAT_FILTER=.*\$/STAT_FILTER=\"name NOT GLOB 'sqlite_*'\"/"   # WF5-1: the pre-fix filter restored (hides every object named sqlite_*, any type)
mk object_filter_any_type        "s/(STAT_FILTER=.*)type='table' AND name GLOB/\\1name GLOB/"      # WF5-1: stat names hidden for triggers/views/indexes too
mk stat_definition_not_pinned    '/definition is not SQLite/s/\*\) fail /*) : /'                     # WF5-1: a forged sqlite_stat1 accepted
mk stat_check_removed            '/^  while IFS= read -r r; do$/,/STAT_ALL" 2>&1\)$/d'
# WF6-2: the stat2-4 half of the pin code (reviewer mutants R1, R2) and the WF6-1 defect itself
mk stat_all_narrowed_to_stat1    "/^STAT_ALL=/s/sqlite_stat\\[1-4\\]/sqlite_stat1/"                              # R1: forged stat2/3/4 tables hidden entirely
mk stat_pins_234_removed         '/.sqlite_stat[234]\|sqlite_stat/d;s/(stat\).)\|\\$/\1) ;;/'                    # R2: SQLite's exact stat2/3/4 definitions refused
mk stat3_pin_mixed_case          's/(sqlite_stat3\(tbl,idx,)neq,nlt,ndlt/\1nEq,nLt,nDLt/'                         # WF6-1 restored for stat3
mk stat4_pin_mixed_case          's/(sqlite_stat4\(tbl,idx,)neq,nlt,ndlt/\1nEq,nLt,nDLt/'                         # WF6-1 restored for stat4
# WF7-4: the diagnostic strips quote characters again (a quoting-only forgery prints identical to SQLite's own text)
mk sane_full_strips_quotes       "s/LC_ALL=C tr -cd '\\[:print:\\]'/tr -cd '[:alnum:]_ .,;()*-'/"
mk url_skip_any_colon            's/^      \[A-Za-z\]\*:\/\/\*\)/      *:*)/'
mk url_any_kind_skipped          's/if \[ "\$kind" = tracker_receipt \]; then skipped/if true; then skipped/'
mk partial_exits_zero_no_flag    's/if \[ \$ALLOW_URL = 1 \]; then echo/if true; then echo/'
mk partial_ignored_gate_ok       's/if \[ "\$\{URL_SKIPPED:-0\}" -gt 0 \]; then/if false; then/'
mk url_skipped_not_counted       's/skipped=\$\(\(skipped\+1\)\)\; else fail/true; else fail/'
# robustness (m-1, m-2, m-3)
mk option_value_not_checked     's/\[ \$# -ge 2 \] \|\| \{ echo "gate: \$1 needs a value" >&2; exit 2; \}/true/'
mk backup_not_readonly          's/"\$SQLITE3" -readonly "\$DB" "\.backup/"$SQLITE3" "$DB" ".backup/'
mk output_not_flattened         "s/^flat\\(\\) \\{ printf '%s' \"\\\$\\*\" \\| tr .\\\\r\\\\n. '  '; \\}/flat() { printf '%s' \"\$*\"; }/"
mk missing_objects_not_sanitised 's/fail "v_gate_missing_objects: \$\(sane "\$m"\)"/fail "v_gate_missing_objects: $m"/'
# the two spoofing defences are individually redundant (sane() on each database-sourced name AND flat() on every message); both removed, the attack works
mk output_both_defences_removed "s/^flat\\(\\) \\{ printf '%s' \"\\\$\\*\" \\| tr .\\\\r\\\\n. '  '; \\}/flat() { printf '%s' \"\\\$*\"; }/;s/fail \"v_gate_missing_objects: \\\$\\(sane \"\\\$m\"\\)\"/fail \"v_gate_missing_objects: \\\$m\"/"
[ -z "${MUT_KEEP:-}" ] || { mkdir -p "$MUT_KEEP" && cp "$W"/*.sh "$MUT_KEEP"/; }
[ -z "${MUT_DRY:-}" ] || { echo "generated $(ls "$W"/*.sh | wc -l) mutants, all differ from gate.sh"; exit 0; }
{ echo "# identity: task=T063 utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)"
  echo "# gate_sha256=$(sha256sum "$G" | cut -d' ' -f1) test_sha256=$(sha256sum "$T" | cut -d' ' -f1) runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1)"
  echo "# mutant<TAB>killed<TAB>first failing check of test_gate.sh"; } > "$OUT"; surv=0
JOBS=${MUT_JOBS:-4}; running=0
for f in "$W"/*.sh; do b=$(basename "$f" .sh)
  ( mkdir -p "$W/s.$b"; r=$(REG_SCRATCH="$W/s.$b" GATE="$f" bash "$T" 2>&1); rc=$?
    [ $rc -ne 0 ] && k=yes || k=NO
    printf '%s\t%s\t%s\n' "$b" "$k" "$(printf '%s' "$r" | grep -m1 '^FAIL' | cut -c1-110)" > "$W/$b.res" ) &
  running=$((running+1)); [ "$running" -ge "$JOBS" ] && { wait -n; running=$((running-1)); }
done; wait
for f in "$W"/*.sh; do b=$(basename "$f" .sh); cat "$W/$b.res" >> "$OUT"; done
# reviewed-equivalent survivors (equivalent_gate_mutants.tsv): reported EQUIV, never counted as survivors; a stale row is an error
EQF="$ROOT/scripts/register/tests/equivalent_gate_mutants.tsv"; stale=0
if [ -r "$EQF" ]; then
  awk -F'\t' 'NR==FNR{if($0!~/^#/ && NF>=2) r[$1]=$2; next} !/^#/ && ($1 in r) && $2=="NO"{print $1"\tEQUIV\t"r[$1]; next} {print}' "$EQF" "$OUT" > "$OUT.eq" && mv "$OUT.eq" "$OUT"
  while IFS=$'\t' read -r name reason by; do case "$name" in ''|'#'*) continue;; esac
    grep -q "^$name	" "$OUT" || { echo "STALE equivalence entry (no such mutant): $name" >&2; stale=1; continue; }
    awk -F'\t' -v n="$name" '!/^#/ && $1==n && $2=="yes"{f=1} END{exit !f}' "$OUT" && { echo "STALE equivalence entry (a test kills it): $name" >&2; stale=1; }
  done < "$EQF"
fi
surv=$(awk -F'\t' '!/^#/ && $2=="NO"' "$OUT" | wc -l); eqn=$(awk -F'\t' '!/^#/ && $2=="EQUIV"' "$OUT" | wc -l)
n=$(ls "$W"/*.sh | wc -l)
echo "gate mutants=$n killed=$((n-surv-eqn)) equivalent_reviewed=$eqn survived_unreviewed=$surv" | tee "$OUT.summary"; [ "$stale" -eq 0 ] || exit 4; [ $surv -eq 0 ]
