#!/usr/bin/env bash
# reconcile.sh - T067 (docs/04 section 11.2, "to be written"): the reconciliation, re-verification, reopen, tracker and findings reports of
# the register as CSV and Markdown, generated from the DB (sqlite3 query to CSV; `.mode markdown` to the report), never by hand.
# Usage: reconcile.sh --db <path> --out-dir <dir>
# It runs INSIDE IMG-TESTUTIL (called by scripts/register/export.sh through scripts/register/locked.sh) with the image's `sqlite3` on PATH; on a
# host it needs a `sqlite3` on PATH (a read-only use; the register is written only through locked.sh). It writes, into <dir>:
#   reconciliation.csv unmapped_entries.csv legacy_id_collisions.csv reverify_queue.csv reopen_counts.csv stale_tracker_sync.csv findings.csv
#   Reconciliation.md   (one section per view, each a Markdown table, headed by the revision header table; class `generated`, T040b)
# Every output is a pure function of the DB content: no clock, no host name, no counter (two runs on an unchanged DB are byte-identical);
# "Last modified" of the header is the largest items.last_modified (or `none`).
# Empty views (WF10 F12): sqlite prints no header line for an empty result, so every view's column list is held in HDR below; an empty view is written as its header
# line alone (CSV) and its table header (Markdown) with `Rows: 0`; for a non-empty view the first CSV line must equal HDR, for an EMPTY view the column list of a TEMP view
# over the query must (WF13 N9); else exit 4, the table is out of sync.
# Exit: 0 ok; 2 usage; 3 database unreadable; 4 query failed.
set -u
DB=""; OUT=""
while [ $# -gt 0 ]; do case "$1" in
  --db|--out-dir) [ $# -ge 2 ] || { echo "reconcile: $1 needs a value" >&2; exit 2; }; if [ "$1" = --db ]; then DB="$2"; else OUT="$2"; fi; shift 2;;
  *) echo "reconcile: unknown argument '$1'" >&2; exit 2;; esac; done
[ -n "$DB" ] && [ -n "$OUT" ] || { echo "usage: reconcile.sh --db <path> --out-dir <dir>" >&2; exit 2; }
[ -f "$DB" ] || { echo "reconcile: database $DB not found" >&2; exit 3; }
mkdir -p -- "$OUT" || exit 3
q() { sqlite3 "$DB" "$@"; }
q 'select 1 from reg_meta limit 1' >/dev/null 2>&1 || { echo "reconcile: $DB is not a register database (no reg_meta)" >&2; exit 3; }
# name | title | query (explicit ORDER BY on every column: a view's own row order is not part of its contract)
VIEWS=(
"reconciliation|Reconciliation (SC-001, v_reconciliation)|SELECT kind,source,entry_locator,legacy_id,relation,atm_id,status FROM v_reconciliation ORDER BY 1,2,3,4,5,6,7"
"unmapped_entries|Unmapped source entries (v_unmapped_entries)|SELECT entry_id,source,locator FROM v_unmapped_entries ORDER BY 1,2,3"
"legacy_id_collisions|Legacy id collisions (v_legacy_id_collisions)|SELECT legacy_id,entries,distinct_titles FROM v_legacy_id_collisions ORDER BY 1,2,3"
"reverify_queue|Re-verification queue (v_reverify_queue)|SELECT atm_id,legacy_status,status,severity FROM v_reverify_queue ORDER BY CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,1,2,3"
"reopen_counts|Most-reopened ranking (v_reopen_counts)|SELECT atm_id,reopens FROM v_reopen_counts ORDER BY 2 DESC,1"
"stale_tracker_sync|Tracker status report (v_stale_tracker_sync)|SELECT tracker_id,atm_id,last_status FROM v_stale_tracker_sync ORDER BY 1,2,3"
"findings|Findings report (reg_findings JOIN reg_evidence)|SELECT f.finding_id,f.unit_alias,f.atm_id,f.category,f.severity,f.location_path,f.detector,IFNULL(e.path,'') AS evidence_path,IFNULL(e.sha256,'') AS evidence_sha256 FROM reg_findings f LEFT JOIN reg_evidence e ON e.evidence_id=f.evidence_id ORDER BY f.finding_seq"
)
declare -A HDR=(
  [reconciliation]="kind,source,entry_locator,legacy_id,relation,atm_id,status"
  [unmapped_entries]="entry_id,source,locator"
  [legacy_id_collisions]="legacy_id,entries,distinct_titles"
  [reverify_queue]="atm_id,legacy_status,status,severity"
  [reopen_counts]="atm_id,reopens"
  [stale_tracker_sync]="tracker_id,atm_id,last_status"
  [findings]="finding_id,unit_alias,atm_id,category,severity,location_path,detector,evidence_path,evidence_sha256"
)
LM="$(q "select IFNULL(max(last_modified),'none') from items")" || exit 4
T="$OUT/.Reconciliation.md.tmp.$$"
{
  echo "# Register reconciliation report"; echo
  echo "| Field | Value |"; echo "|---|---|"
  echo "| Revision | 1 |"; echo "| Created | generated |"; echo "| Last modified | $LM |"
  echo "| Status | generated from the register database by scripts/register/reconcile.sh; hand edits are forbidden (docs/04 section 11.1) |"
  echo "| Source | docs/workable_items.db (views v_reconciliation, v_unmapped_entries, v_legacy_id_collisions, v_reverify_queue, v_reopen_counts, v_stale_tracker_sync, reg_findings) |"
  echo
  echo "## Table of contents"; echo
  for v in "${VIEWS[@]}"; do IFS='|' read -r n t _ <<<"$v"; echo "- [$t](#$n)"; done
  echo
} >"$T" || exit 4
for v in "${VIEWS[@]}"; do
  IFS='|' read -r n t sqlq <<<"$v"
  # the query text holds no '|' (checked: the three-field split above) except inside this array's last field, which is re-joined
  sqlq="${v#*|*|}"
  q -csv -header "$sqlq" >"$OUT/.$n.csv.tmp.$$" 2>"$OUT/.err.$$" || { echo "reconcile: query $n failed: $(cat "$OUT/.err.$$")" >&2; rm -f "$OUT"/.*.tmp.$$ "$OUT/.err.$$"; exit 4; }
  if [ -s "$OUT/.$n.csv.tmp.$$" ]; then
    [ "$(head -n1 "$OUT/.$n.csv.tmp.$$")" = "${HDR[$n]}" ] || { echo "reconcile: the header of view $n is [$(head -n1 "$OUT/.$n.csv.tmp.$$")], the HDR table says [${HDR[$n]}]" >&2; rm -f "$OUT"/.*.tmp.$$ "$OUT/.err.$$"; exit 4; }   # MUT:header-table
    rows="$(( $(wc -l <"$OUT/.$n.csv.tmp.$$") - 1 ))"
  else
    # an empty view prints no header line: the hand-kept column list is checked against the view's own columns (a TEMP view over the query; PRAGMA table_info answers for an empty result)
    hv="$(q "CREATE TEMP VIEW _hdr AS $sqlq; SELECT group_concat(name,',') FROM (SELECT name FROM pragma_table_info('_hdr') ORDER BY cid)" 2>"$OUT/.err.$$")" || { echo "reconcile: reading the columns of the empty view $n failed: $(cat "$OUT/.err.$$")" >&2; rm -f "$OUT"/.*.tmp.$$ "$OUT/.err.$$"; exit 4; }
    [ "$hv" = "${HDR[$n]}" ] || { echo "reconcile: the header of view $n (empty) is [$hv], the HDR table says [${HDR[$n]}]" >&2; rm -f "$OUT"/.*.tmp.$$ "$OUT/.err.$$"; exit 4; }   # MUT:empty-header-check
    printf '%s\n' "${HDR[$n]}" >"$OUT/.$n.csv.tmp.$$"; rows=0; fi   # MUT:empty-header
  mv -f -- "$OUT/.$n.csv.tmp.$$" "$OUT/$n.csv"
  { echo "<a id=\"$n\"></a>"; echo "## $t"; echo; echo "Rows: $rows"; echo
    if [ "$rows" -gt 0 ]; then q -markdown -header "$sqlq" 2>>"$OUT/.err.$$"
    else IFS=, read -r -a cols <<<"${HDR[$n]}"; printf '| %s |\n' "$(IFS='|'; echo "${cols[*]}" | sed 's/|/ | /g')"; printf '|%s\n' "$(printf -- '---|%.0s' "${cols[@]}")"; fi
    echo; } >>"$T"
done
rm -f "$OUT/.err.$$"
mv -f -- "$T" "$OUT/Reconciliation.md" || exit 4
echo "reconcile: wrote $(ls "$OUT"/*.csv | wc -l) CSV files and Reconciliation.md into $OUT"
