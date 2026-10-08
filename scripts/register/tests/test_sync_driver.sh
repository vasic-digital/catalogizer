#!/usr/bin/env bash
# test_sync_driver.sh - T188 (docs/04 section 10.3, tasks.md WP-22): scripts/register/sync_trackers.sh (T189) and the consumer config .helix/reporting.yaml (T190).
# RED while the driver is absent. Real SQLite, the real engine binary and the REAL scripts/register/locked.sh (RUNP, podman, IMG-TESTUTIL via the logging shim of clib.sh)
# against SCRATCH roots; the stub is only the tracker ADAPTER (a consumer script by contract) and, for two cases, a stand-in locked.sh that proves who writes.
# Env: SYNC substitutes a mutant copy of the driver (mutate_sync_driver.sh). Container leg and its recorded deviation: see clib.sh.
# Classes covered (the T188 convergence self-check, docs/scripts/sync_trackers.md): environment variable missing / empty / set (value never leaves the process),
# config missing / malformed / mistyped key / secret in command / wrong prefix, database missing / not SQLite, every tracker state and exit code of the adapter
# contract (SYNCED, SKIPPED unreachable, FAILED with bounded attempts, circuit breaker, garbage output, SYNCED without reference / with exit 1 / with outside evidence),
# idempotent re-run, revision change, dry run, single writer (journal chain, stub wrapper, write failure), concurrent driver, minting once per tracker and reason.
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead T188
if [ "${SYNC_FAILFAST:-}" = 1 ]; then bad() { FAIL=$((FAIL+1)); echo "FAIL $*"; echo "RESULT pass=$PASS fail=$FAIL (fail-fast, mutation runs)"; exit 1; }; fi   # the mutation runner needs the first failing check only
SYNC=${SYNC:-$REG_DIR/sync_trackers.sh}
. "$(dirname "$0")/sync_common.sh"   # envretry wrappers of lk/tool, pq, ADAPTER, mkfix, cfg, drv (first-sync approval of the run's own plan)
echo "# sha256 sync_trackers=$(fsha "$SYNC" | cut -c1-64) config=$(fsha "$ROOT/.helix/reporting.yaml" | cut -c1-64)"
if [ ! -f "$SYNC" ]; then bad "S0 sync driver absent: $SYNC"; finish; exit 1; fi
CANARY_TOKEN="tok-canary-9f3a71c2"; CANARY_OTHER="leak-canary-55d0b8e4"
LEAK="$T_SCR/leakscan.txt"; : >"$LEAK"

echo "== T190: the repository's own consumer config =="
RA=$(mkfix A 3) || { bad "setup A"; finish; exit 1; }
cp "$ROOT/.helix/reporting.yaml" "$RA/.helix/real-config.yaml"
out=$(drv "$RA" --config .helix/real-config.yaml --dry-run 2>&1); rc=$?
[ $rc -eq 0 ] && for t in github_issues gitflic gitlab gitverse crashlytics sonar; do printf '%s' "$out" | grep -q "tracker=$t state=not_configured"; done && ok "C1 the repository config loads: the six candidates (GitHub issues, GitFlic, GitLab, GitVerse, Crashlytics, Sonar) are each not_configured, trackers: []" || bad "C1 rc=$rc [$out]"
python3 -I - "$ROOT/.helix/reporting.yaml" <<'PY' && ok "C2 the config shape: db docs/workable_items.db, id_prefix CAT, trackers [] and no value of any kind after a credential-looking key" || bad "C2 config shape"
import sys,yaml,re
c=yaml.safe_load(open(sys.argv[1])); assert c["db"]=="docs/workable_items.db" and c["id_prefix"]=="CAT" and c["trackers"]==[] and c["schema_version"]==1
txt=open(sys.argv[1]).read()
assert not re.search(r"(?i)(token|secret|password|api_?key)\s*[:=]\s*\S", "\n".join(l for l in txt.splitlines() if not l.lstrip().startswith("#")))
PY

echo "== fixture A: three items, six trackers in every state =="
cfg "$RA" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
evidence_dir: .audit/sync/evidence
trackers:
  - name: alpha
    command: bash $RA/adapter.sh {id}
    required_env: [SYNC_TEST_TOKEN]
    env_passthrough: {SYNC_TEST_DB: "{db}"}
  - name: beta
    command: no-such-cli-xyz --id {id}
    required_env: [SYNC_TEST_TOKEN]
  - name: gamma
  - name: delta
    enabled: false
    command: bash $RA/adapter.sh {id}
  - name: eps
    command: ""
candidate_trackers: [cand1]
EOF
sha0=$(fsha "$RA/$DB"); jn0=$(wc -l <"$RA/.audit/register/journal.jsonl")
echo "== dry run: no side effect =="
out=$( (export OTHER_SECRET="$CANARY_OTHER"; drv "$RA" --dry-run --json .audit/dry.json 2>&1) ); rc=$?
[ $rc -eq 0 ] && ok "D1 dry run exits 0" || bad "D1 rc=$rc [$out]"
assert_eq "D2 dry run: database byte-identical" "$(fsha "$RA/$DB")" "$sha0"
assert_eq "D3 dry run: no wrapper call, no journal row" "$(wc -l <"$RA/.audit/register/journal.jsonl")" "$jn0"
assert_eq "D4 dry run: no adapter call, no batch directory, no lock file" "$(wc -c <"$RA/adapter.log")$([ -e "$RA/.audit/sync" ] && echo SYNCDIR)" "0"
printf '%s' "$out" | grep -q 'tracker=alpha state=credentials_absent' && printf '%s' "$out" | grep -q 'item would_mint tracker=alpha reason=credentials_absent' && ok "D5 the plan names alpha credentials_absent and the item it would mint" || bad "D5 [$out]"
python3 -I -c 'import json,sys;j=json.load(open(sys.argv[1]));assert j["dry_run"] is True and j["locked_calls"]==[] and j["identity"]["driver_sha256"] and j["identity"]["config_sha256"]' "$RA/.audit/dry.json" && ok "D6 --json of a dry run carries the identity block and no wrapper call" || bad "D6"

echo ok >"$RA/adapter.mode"
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RA" --dry-run --tracker alpha 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'tracker=alpha state=adapter selected=3' && assert_eq "D7 dry run with the token SET: the plan selects the 3 items but no adapter runs, no database write, no .audit/sync directory" "$(wc -c <"$RA/adapter.log")/$(fsha "$RA/$DB")/$([ -e "$RA/.audit/sync" ] && echo SYNCDIR || echo none)" "0/$sha0/none" || bad "D7 rc=$rc [$out]"

echo "== missing env: SKIPPED credentials_absent, NAMES only =="
out=$( (export OTHER_SECRET="$CANARY_OTHER" SYNC_TEST_TOKEN=""; drv "$RA" --json .audit/run1.json 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && ok "E1 run 1 (token empty counts as unset) exits 0: SKIPPED rows are honest results" || { bad "E1 rc=$rc [$out]"; }
assert_eq "E2 alpha: 8 items (3 + 5 minted), every latest row SKIPPED credentials_absent" "$(pq "$RA" "select count(*)||'/'||sum(status='SKIPPED' and skip_reason='credentials_absent') from reg_tracker_sync_log where tracker_id='alpha'")" "8/8"
assert_eq "E3 alpha: missing_env_names holds the NAME only, as a JSON array" "$(pq "$RA" "select distinct missing_env_names from reg_tracker_sync_log where tracker_id='alpha'")" '["SYNC_TEST_TOKEN"]'
assert_eq "E4 beta: tracker_client_absent (CLI missing); eps: tracker_client_absent (empty command)" "$(pq "$RA" "select group_concat(x) from (select distinct tracker_id||':'||skip_reason x from reg_tracker_sync_log where tracker_id in ('beta','eps') order by 1)")" "beta:tracker_client_absent,eps:tracker_client_absent"
assert_eq "E5 gamma and cand1: not_configured; delta: disabled_by_operator" "$(pq "$RA" "select group_concat(x) from (select distinct tracker_id||':'||skip_reason x from reg_tracker_sync_log where tracker_id in ('gamma','cand1','delta') order by 1)")" "cand1:not_configured,delta:disabled_by_operator,gamma:not_configured"
assert_eq "E6 no adapter ran" "$(wc -c <"$RA/adapter.log")" "0"
assert_eq "E7 6 trackers x 8 items = 48 rows and no item with a NONE last_status in v_stale_tracker_sync" "$(pq "$RA" "select count(*) from reg_tracker_sync_log")/$(pq "$RA" "select count(*) from v_stale_tracker_sync where last_status is null")" "48/0"
assert_eq "E8 the CHECK shapes hold: no SYNCED row, every SKIPPED row has its reason" "$(pq "$RA" "select count(*) from reg_tracker_sync_log where status<>'SKIPPED' or skip_reason is null")" "0"

echo "== minting: one item per tracker and reason, once (11.4.214) =="
assert_eq "M1 five items minted through reg_ids: alpha/credentials_absent, beta+eps/tracker_client_absent, gamma+cand1/not_configured (delta, an operator decision, mints none)" "$(pq "$RA" "select group_concat(minted_by) from (select minted_by from reg_ids where minted_by like 'sync_trackers:%' order by minted_by)")" "sync_trackers:alpha:credentials_absent,sync_trackers:beta:tracker_client_absent,sync_trackers:cand1:not_configured,sync_trackers:eps:tracker_client_absent,sync_trackers:gamma:not_configured"
assert_eq "M2 each minted id is a Task item with a reg_item_ext row" "$(pq "$RA" "select count(*) from reg_ids r join items i on i.atm_id=r.atm_id join reg_item_ext x on x.atm_id=r.atm_id where r.minted_by like 'sync_trackers:%' and i.type='Task'")" "5"
python3 -I -c 'import json,sys;j=json.load(open(sys.argv[1]));assert sorted(m["action"] for m in j["minted"])==["minted"]*5 and j["exit"]==0 and j["trackers"]["alpha"]["missing_env_names"]==["SYNC_TEST_TOKEN"]' "$RA/.audit/run1.json" && ok "M3 the --json evidence lists the five minted items and the NAME of the missing variable" || bad "M3"
python3 -I -c 'import json,sys;j=json.load(open(sys.argv[1]));st=dict(((a,b),c) for a,b,c in j["v_stale_tracker_sync"]);assert all(b=="SKIPPED" for (a,b) in st) and sum(st.values())==40' "$RA/.audit/run1.json" && ok "M4 the v_stale_tracker_sync summary: 40 pairs (5 enabled trackers x 8 items; delta, disabled by the operator, is enabled=0 and not listed), every one SKIPPED (none NONE)" || bad "M4"

echo "== idempotent re-run =="
sha1=$(fsha "$RA/$DB"); jn1=$(wc -l <"$RA/.audit/register/journal.jsonl")
out=$( (export SYNC_TEST_TOKEN=""; drv "$RA" 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "R1 an unchanged re-run: database byte-identical, no wrapper call, no minted twin" "$(fsha "$RA/$DB")/$(wc -l <"$RA/.audit/register/journal.jsonl")/$(pq "$RA" "select count(*) from reg_ids where minted_by like 'sync_trackers:%'")" "$sha1/$jn1/5" || bad "R1 rc=$rc [$out]"
printf '%s' "$out" | grep -q 'tracker=gamma .*unchanged_skipped=8' && ok "R2 the repeated SKIPPED results are counted unchanged_skipped, not rewritten" || bad "R2 [$out]"

echo "== token set: the adapter runs, the value never leaves the environment =="
echo ok >"$RA/adapter.mode"
out=$( (export OTHER_SECRET="$CANARY_OTHER" SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RA" --tracker alpha 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && ok "A1 run 3 (token set, adapter ok) exits 0" || bad "A1 rc=$rc [$out]"
assert_eq "A2 alpha: all 8 items SYNCED with exit_code 0, a remote_ref and an evidence row" "$(pq "$RA" "select count(*)||'/'||sum(l.exit_code=0 and l.remote_ref like 'stub#CAT-%' and e.kind='tracker_receipt') from reg_tracker_sync_log l join reg_evidence e on e.evidence_id=l.evidence_id where l.status='SYNCED' and l.tracker_id='alpha'")" "8/8"
ev_ok=1; while IFS='|' read -r p h; do [ "$(fsha "$RA/$p")" = "$h" ] || ev_ok=0; done < <(pq "$RA" "select path,sha256 from reg_evidence where kind='tracker_receipt'")
[ "$ev_ok" = 1 ] && [ "$(pq "$RA" "select count(*) from reg_evidence where kind='tracker_receipt'")" = 8 ] && ok "A3 every recorded evidence row names a file whose sha256 matches" || bad "A3 evidence rows do not match the files"
assert_eq "A4 the adapter ran once per item, for alpha only" "$(wc -l <"$RA/adapter.log")" "8"
grep -q 'SYNC_TEST_TOKEN' "$RA/adapter.log" && grep -q 'SYNC_TEST_DB' "$RA/adapter.log" && ! grep -q 'OTHER_SECRET' "$RA/adapter.log" && ok "A5 the adapter env holds the declared names (required_env, env_passthrough) and NOT an undeclared secret" || bad "A5 env names [$(head -c 300 "$RA/adapter.log")]"
! grep -q "$CANARY_TOKEN" "$RA/adapter.log" "$RA/adapter.stdin.last" && ok "A6 the token value is in no adapter argv and not in the stdin JSON" || bad "A6 token value reached argv or stdin"
python3 -I -c 'import json,sys;j=json.load(open(sys.argv[1]));assert j["db"]=="docs/workable_items.db" and j["atm_id"].startswith("CAT-") and j["tracker"]=="alpha" and "evidence_dir" in j and "title" in j' "$RA/adapter.stdin.last" && ok "A7 the stdin JSON is {db, tracker, atm_id, title, body, status, type, severity, evidence_dir}" || bad "A7"
assert_eq "A8 alpha SYNCED on top of its older SKIPPED rows: the latest status is SYNCED for all 8 (v_stale_tracker_sync)" "$(pq "$RA" "select count(*)||'/'||sum(last_status='SYNCED') from v_stale_tracker_sync where tracker_id='alpha'")" "8/8"
echo "== re-run after SYNCED: no push =="
jn2=$(wc -l <"$RA/.audit/register/journal.jsonl"); sha2=$(fsha "$RA/$DB")
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RA" --tracker alpha 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "A9 a re-run with unchanged item revisions: adapter log, database and journal unchanged" "$(wc -l <"$RA/adapter.log")/$(fsha "$RA/$DB")/$(wc -l <"$RA/.audit/register/journal.jsonl")" "8/$sha2/$jn2" || bad "A9 rc=$rc [$out]"
echo "== a changed item revision pushes that item only =="
lk "$RA" -- sh -c "$WI_IN update --id CAT-002 --db /src/$DB --title changed-title" >/dev/null 2>&1 || bad "A10pre the fixture step (title change of CAT-002) was refused by the wrapper: the checks below prove nothing"
assert_eq "A10pre the fixture step took effect: CAT-002 now carries the new title" "$(pq "$RA" "select title from items where atm_id='CAT-002'")" "changed-title"
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RA" --tracker alpha 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "A10 exactly one new adapter call, for CAT-002" "$(wc -l <"$RA/adapter.log")/$(tail -n1 "$RA/adapter.log" | cut -d' ' -f1)" "9/CAT-002" || bad "A10 rc=$rc [$out]"
assert_eq "A11 CAT-002 now has two SYNCED rows with two distinct item_revision values" "$(pq "$RA" "select count(*)||'/'||count(distinct item_revision) from reg_tracker_sync_log where tracker_id='alpha' and atm_id='CAT-002' and status='SYNCED'")" "2/2"

echo "== FAILED: bounded attempts, then re-selected =="
echo fail >"$RA/adapter.mode"; lk "$RA" -- sh -c "$WI_IN update --id CAT-003 --db /src/$DB --title changed-again" >/dev/null 2>&1 || bad "F0pre the fixture step (title change of CAT-003) was refused by the wrapper: the checks below prove nothing"
assert_eq "F0pre the fixture step took effect: CAT-003 now carries the new title" "$(pq "$RA" "select title from items where atm_id='CAT-003'")" "changed-again"
n0=$(wc -l <"$RA/adapter.log")
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RA" --tracker alpha --max-attempts 3 --backoff-base 0 --breaker 50 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 1 ] && ok "F1 an item that ends FAILED makes the driver exit 1" || bad "F1 rc=$rc [$out]"
assert_eq "F2 CAT-003 attempted exactly --max-attempts (3) times" "$(tail -n +$((n0+1)) "$RA/adapter.log" | grep -c '^CAT-003 ')" "3"
assert_eq "F3 the latest CAT-003 row is FAILED with exit_code 3 (the adapter's)" "$(pq "$RA" "select status||'/'||exit_code from reg_tracker_sync_log where tracker_id='alpha' and atm_id='CAT-003' order by sync_id desc limit 1")" "FAILED/3"
assert_eq "F4 the failure is itself tracked: one item for alpha/failed" "$(pq "$RA" "select count(*) from reg_ids where minted_by='sync_trackers:alpha:failed'")" "1"
echo ok >"$RA/adapter.mode"
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RA" --tracker alpha 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "F5 once the adapter works, the FAILED item is selected again and SYNCED; nothing else is pushed twice" "$(pq "$RA" "select status from reg_tracker_sync_log where tracker_id='alpha' and atm_id='CAT-003' order by sync_id desc limit 1")/$(pq "$RA" "select count(*) from v_stale_tracker_sync where tracker_id='alpha' and last_status<>'SYNCED'")" "SYNCED/0" || bad "F5 rc=$rc [$out]"
assert_eq "F6 the minted alpha/failed item was minted once and itself synced" "$(pq "$RA" "select r.atm_id is not null and l.status='SYNCED' from reg_ids r join reg_tracker_sync_log l on l.atm_id=r.atm_id and l.tracker_id='alpha' where r.minted_by='sync_trackers:alpha:failed' order by l.sync_id desc limit 1")" "1"

echo "== a changed tracker state is recorded again (reason and NAMES are compared) =="
RC=$(mkfix C 2) || { bad "setup C"; finish; exit 1; }
cfg "$RC" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: zeta, command: "bash $RC/adapter.sh {id}", required_env: [SYNC_TEST_Z1]}
  - {name: eta}
EOF
out=$( (unset SYNC_TEST_Z1 SYNC_TEST_Z2; drv "$RC" 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "G1 first run: zeta credentials_absent [Z1], eta not_configured, 4 items (2 + 2 minted) x 2 trackers = 8 rows" "$(pq "$RC" "select count(*) from reg_tracker_sync_log")/$(pq "$RC" "select group_concat(x) from (select distinct tracker_id||':'||skip_reason||':'||ifnull(missing_env_names,'') x from reg_tracker_sync_log order by 1)")" '8/eta:not_configured:,zeta:credentials_absent:["SYNC_TEST_Z1"]' || bad "G1 rc=$rc [$out]"
cfg "$RC" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: zeta, command: "bash $RC/adapter.sh {id}", required_env: [SYNC_TEST_Z1, SYNC_TEST_Z2]}
  - {name: eta, command: "no-such-cli-xyz {id}"}
EOF
out=$( (unset SYNC_TEST_Z1 SYNC_TEST_Z2; drv "$RC" 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "G2 after the config changed: every item has a NEW latest row with the new NAMES for zeta (Z1 and Z2) and the new reason for eta (tracker_client_absent); the reg_trackers row follows the config (the command is stored as executable#sha256:<16 hex>, never as text)" "$(pq "$RC" "select group_concat(x) from (select distinct l.tracker_id||':'||l.skip_reason||':'||ifnull(l.missing_env_names,'') x from reg_tracker_sync_log l where sync_id in (select max(sync_id) from reg_tracker_sync_log group by tracker_id,atm_id) order by 1)")/$(pq "$RC" "select required_env from reg_trackers where tracker_id='zeta'")/$(pq "$RC" "select command_ref from reg_trackers where tracker_id='eta'")" 'eta:tracker_client_absent:,zeta:credentials_absent:["SYNC_TEST_Z1", "SYNC_TEST_Z2"]/["SYNC_TEST_Z1", "SYNC_TEST_Z2"]/no-such-cli-xyz#sha256:'"$(printf '%s' 'no-such-cli-xyz {id}' | sha256sum | cut -c1-16)" || bad "G2 rc=$rc [$out]"
assert_eq "G3 the new reason is tracked once: eta/tracker_client_absent minted one item; the old eta/not_configured item is kept (never deleted or reused)" "$(pq "$RC" "select group_concat(m) from (select minted_by m from reg_ids where minted_by like 'sync_trackers:eta:%' order by 1)")" "sync_trackers:eta:not_configured,sync_trackers:eta:tracker_client_absent"

cfg "$RC" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: zeta, command: "bash $RC/adapter.sh {id}", required_env: [SYNC_TEST_Z1, SYNC_TEST_Z2]}
  - {name: eta, command: "no-such-cli-xyz {id}"}
  - {name: theta}
EOF
lk "$RC" -- sqlite3 /src/$DB "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('sync_trackers:theta:not_configured','manual');" >/dev/null 2>&1 || bad "G4pre the fixture step (the interrupted mint) was refused by the wrapper: G4 proves nothing"
assert_eq "G4pre the fixture step took effect: the reg_ids row exists and has no item" "$(pq "$RC" "select count(*)||'/'||count(i.atm_id) from reg_ids r left join items i on i.atm_id=r.atm_id where r.minted_by='sync_trackers:theta:not_configured'")" "1/0"
out=$( (unset SYNC_TEST_Z1 SYNC_TEST_Z2; drv "$RC" 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && assert_eq "G4 an id that was minted for theta/not_configured but never got its item (an interrupted mint) is completed, not minted twice: one reg_ids row, one item, one reg_item_ext row" "$(pq "$RC" "select count(*)||'/'||count(i.atm_id)||'/'||count(x.atm_id) from reg_ids r left join items i on i.atm_id=r.atm_id left join reg_item_ext x on x.atm_id=r.atm_id where r.minted_by='sync_trackers:theta:not_configured'")" "1/1/1" || bad "G4 rc=$rc [$out]"

echo "== single writer: the journal is the whole story =="
python3 -I - "$RA/.audit/register/journal.jsonl" "$(fsha "$RA/$DB")" <<'PY' && ok "W1 the journal chain is unbroken: every row's db_sha_before is the previous row's db_sha_after, the last db_sha_after is the database on disk, and every driver row recorded its SQL batch as an input" || bad "W1 journal chain"
import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
reg=[r for r in rows if r.get("mode")=="register"]
for a,b in zip(reg,reg[1:]): assert b["db_sha_before"]==a["db_sha_after"], (a["op_id"],b["op_id"])
assert reg[-1]["db_sha_after"]==sys.argv[2]
drv=[r for r in reg if r["op_id"].startswith("sync-")]
assert len(drv)>=10, len(drv)
logs=[r for r in drv if "-log-" in r["op_id"]]
assert logs and all(r.get("input_args") for r in logs), "a batch without its recorded input"
PY
nlog=$(pq "$RA" "select count(*) from reg_tracker_sync_log"); nj=$(python3 -I -c 'import json,sys
n=0
for l in open(sys.argv[1]):
    r=json.loads(l)
    if r["op_id"].startswith("sync-") and "-log-" in r["op_id"] and r["exit"]==0: n+=1
print(n)' "$RA/.audit/register/journal.jsonl")
[ "$nj" -ge 3 ] && [ "$nlog" -ge $((2 * nj)) ] && ok "W2 $nlog sync-log rows were written by $nj journaled batches (at least two rows per batch on average), never row by row" || bad "W2 rows=$nlog batches=$nj"
STUB="$T_SCR/stub_locked.sh"; cat >"$STUB" <<'SH'
#!/usr/bin/env bash
# stand-in locked.sh: records its call, writes NOTHING. A driver that opens the database itself would change it.
echo "$*" >>"$STUB_LOG"
case "${STUB_MODE:-ok}" in ok) exit 0;; trackers_only) case "$2" in *-trackers-*) exit 0;; *) echo "boom" >&2; exit 1;; esac;; esac
SH
chmod +x "$STUB"
RS=$(mkfix S 2) || { bad "setup S"; finish; exit 1; }
cfg "$RS" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: alpha, command: "bash $RS/adapter.sh {id}", required_env: [SYNC_TEST_TOKEN]}
  - {name: gamma}
EOF
shaS=$(fsha "$RS/$DB"); STUB_LOG="$T_SCR/stub.log"; : >"$STUB_LOG"
FS=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; pflags "$RS") ); out=$( (export STUB_LOG STUB_MODE=ok SYNC_TEST_TOKEN="$CANARY_TOKEN"; LOCKED="$STUB" tool "$RS" "$SYNC" $FS 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ "$(fsha "$RS/$DB")" = "$shaS" ] && [ -s "$STUB_LOG" ] && ok "W3 with a no-op wrapper the database is byte-identical: the driver process itself never writes it (a copy that opens the database for writing fails here)" || bad "W3 sha changed or the wrapper was not called (rc=$rc) [$out]"
grep -q -- '--op-id sync-.*-trackers-1 -- sqlite3 -bail /src/docs/workable_items.db' "$STUB_LOG" && ok "W4 every write is a wrapper call of the form locked.sh --op-id <id> -- sqlite3 -bail /src/<db> '.read <batch>'" || bad "W4 [$(head -c 300 "$STUB_LOG")]"
: >"$STUB_LOG"; echo ok >"$RS/adapter.mode"
FS=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; pflags "$RS" --tracker alpha) ); out=$( (export STUB_LOG STUB_MODE=trackers_only SYNC_TEST_TOKEN="$CANARY_TOKEN"; LOCKED="$STUB" tool "$RS" "$SYNC" --tracker alpha $FS 2>&1) ); rc=$?
RUN=$(dirname "$(ls "$RS"/.audit/sync/sync-*/unwritten.jsonl 2>/dev/null | tail -n1)")
[ $rc -eq 4 ] && printf '%s' "$out" | grep -q 'register write failed' && [ -s "$RUN/unwritten.jsonl" ] && ok "W5 a failed register write: exit 4, the unwritten rows (with their remote_ref) are kept in unwritten.jsonl, the run stops" || bad "W5 rc=$rc [$out]"
python3 -I -c 'import json,sys;r=[json.loads(l) for l in open(sys.argv[1])];assert len(r)==2 and all(x["status"]=="SYNCED" and x["remote_ref"].startswith("stub#") for x in r)' "$RUN/unwritten.jsonl" && ok "W6 both pushed items' references survive in the recovery file (no remote issue becomes unreachable from the register)" || bad "W6"

echo "== concurrent driver: two real drivers =="
RX=$(mkfix X 1) || { bad "setup X"; finish; exit 1; }
cfg "$RX" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: alpha, command: "bash $RX/adapter.sh {id}", required_env: [SYNC_TEST_TOKEN]}
EOF
echo slow >"$RX/adapter.mode"; LK="$RX/.audit/sync/driver.lock"
( export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RX" >"$T_SCR/x1.first.out" 2>&1; echo $? >"$T_SCR/x1.first.rc" ) & HP=$!
n=0; while [ $n -lt 300 ] && ! { [ -e "$LK" ] && ! flock -n "$LK" true 2>/dev/null; }; do n=$((n+1)); sleep 0.2; done   # wait until the first driver really holds the lock
[ -e "$LK" ] && ! flock -n "$LK" true 2>/dev/null && ok "X0 control: the first driver holds .audit/sync/driver.lock (a second exclusive flock is refused)" || bad "X0 the first driver never took the lock"
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RX" 2>&1) ); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=sync_already_running' && ok "X1 a second REAL driver while the first is inside an adapter call is REFUSED sync_already_running (20)" || bad "X1 rc=$rc [$out]"
wait "$HP" 2>/dev/null
assert_eq "X2 the first driver was not disturbed: exit 0 and its item SYNCED" "$(cat "$T_SCR/x1.first.rc" 2>/dev/null)/$(pq "$RX" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "0/1"

echo "== adapter contract violations are never recorded SYNCED =="
RB=$(mkfix B 4) || { bad "setup B"; finish; exit 1; }
cfg "$RB" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: alpha, command: "bash $RB/adapter.sh {id}", required_env: [SYNC_TEST_TOKEN]}
EOF
printf '{"receipt":"CAT-001","remote_ref":"stub#CAT-001","note":"evidence outside the repository (a real, non-empty file next to the scratch root that names the item and the reference: only the containment check can refuse it)"}\n' >"$T_SCR/outside-evidence.json"
for m in garbage noref exit1 outside noevidence badskip badnames hang; do echo $m >"$RB/adapter.mode"
  out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RB" --tracker alpha --limit 1 --max-attempts 1 --backoff-base 0 --adapter-timeout 1 2>&1) ); rc=$?
  got="$(pq "$RB" "select status from reg_tracker_sync_log where tracker_id='alpha' and atm_id='CAT-001' order by sync_id desc limit 1")/$([ $rc -eq 1 ] && echo exit1)"
  if [ "$got" = "FAILED/exit1" ]; then ok "V-$m adapter output '$m' is recorded FAILED (exit $rc), never SYNCED"; else bad "V-$m adapter output '$m': got [$got] want [FAILED/exit1] driver rc=$rc output: $(printf '%s' "$out" | tr '\n' ' ' | cut -c1-600)"; fi; done
assert_eq "V1 no SYNCED row exists after eight contract violations" "$(pq "$RB" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "0"
assert_eq "V2 the exit codes: unusable output 70, missing reference or evidence 65, the process exit code beats the adapter's own claim (exit1: 1), an adapter SKIPPED outside its closed reasons 70, invalid variable names 70, a hung adapter 124 (timeout)" "$(pq "$RB" "select group_concat(exit_code) from (select exit_code from reg_tracker_sync_log where tracker_id='alpha' and status='FAILED' order by sync_id)")" "70,65,1,65,65,70,70,124"
echo skipcreds >"$RB/adapter.mode"
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RB" --tracker alpha --limit 1 --max-attempts 1 --backoff-base 0 --retry-indeterminate 2>&1) ); rc=$?
[ $rc -eq 0 ] && assert_eq "V3 an adapter that reports SKIPPED credentials_absent with the NAME it lacks is recorded as such (not FAILED), the NAME kept" "$(pq "$RB" "select status||'/'||skip_reason||'/'||missing_env_names from reg_tracker_sync_log where tracker_id='alpha' and atm_id='CAT-001' order by sync_id desc limit 1")" 'SKIPPED/credentials_absent/["REMOTE_TOKEN"]' || bad "V3 rc=$rc [$out]"
echo unreachable >"$RB/adapter.mode"; n0=$(wc -l <"$RB/adapter.log")
out=$( (export SYNC_TEST_TOKEN="$CANARY_TOKEN"; drv "$RB" --tracker alpha --max-attempts 2 --backoff-base 0 --breaker 2 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
[ $rc -eq 0 ] && ok "U1 an unreachable tracker is an honest SKIPPED result: exit 0" || bad "U1 rc=$rc [$out]"
assert_eq "U2 2 attempts per item for the first 2 items, then the circuit breaker: 4 adapter calls for 7 items" "$(( $(wc -l <"$RB/adapter.log") - n0 ))" "4"
assert_eq "U3 all 7 items (the 6 present and the one minted for alpha/unreachable) end SKIPPED unreachable" "$(pq "$RB" "select count(distinct atm_id)||'/'||min(skip_reason)||'/'||max(skip_reason) from (select atm_id,skip_reason from reg_tracker_sync_log where tracker_id='alpha' and sync_id in (select max(sync_id) from reg_tracker_sync_log where tracker_id='alpha' group by atm_id))")" "7/unreachable/unreachable"
printf '%s' "$out" | grep -q 'breaker_skipped=5' && ok "U4 five items (4 present + the minted one) were skipped by the breaker without a call" || bad "U4 [$out]"
assert_eq "U5 the unreachable tracker is tracked: one item alpha/unreachable" "$(pq "$RB" "select count(*) from reg_ids where minted_by='sync_trackers:alpha:unreachable'")" "1"

echo "== refusals and exit codes =="
rf() { local want=$1 reason=$2 label=$3; shift 3; local o r; o=$(drv "$RS" "$@" 2>&1); r=$?; if [ "$r" -eq "$want" ] && { [ -z "$reason" ] || printf '%s' "$o" | grep -q "reason=$reason"; }; then ok "$label"; else bad "$label (rc=$r) [$o]"; fi; }
rf 20 config_missing "Z1 a missing config file is REFUSED config_missing" --config .helix/none.yaml --dry-run
printf 'schema_version: [unclosed\n' >"$RS/.helix/bad1.yaml"; rf 20 config_malformed "Z2 unparseable YAML is REFUSED config_malformed" --config .helix/bad1.yaml --dry-run
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - {name: alpha, command: "x", requird_env: [A]}\n' >"$RS/.helix/bad2.yaml"; rf 20 config_malformed "Z3 a mistyped tracker key (requird_env would silently disable the credentials check) is REFUSED" --config .helix/bad2.yaml --dry-run
printf 'schema_version: 2\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers: []\n' >"$RS/.helix/bad3.yaml"; rf 20 config_malformed "Z4 a wrong schema_version is REFUSED" --config .helix/bad3.yaml --dry-run
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers: null\n' >"$RS/.helix/bad4.yaml"; rf 20 config_malformed "Z5 trackers: null (not a list) is REFUSED" --config .helix/bad4.yaml --dry-run
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - {name: alpha, command: "bash a.sh --token=abcd1234efgh"}\n' >"$RS/.helix/bad5.yaml"; rf 20 config_secret_in_command "Z6 a command that carries a secret value is REFUSED config_secret_in_command" --config .helix/bad5.yaml --dry-run
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: ZZZ\ntrackers: []\n' >"$RS/.helix/bad6.yaml"; rf 20 config_id_prefix_unsupported "Z7 an id_prefix the reg_ids DDL does not generate is REFUSED" --config .helix/bad6.yaml --dry-run
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - {name: alpha, command: "bash a.sh {oops}"}\n' >"$RS/.helix/bad7.yaml"; rf 20 config_malformed "Z8 an unknown command placeholder is REFUSED" --config .helix/bad7.yaml --dry-run
rf 20 path_invalid "Z9 a --config path with .. is REFUSED path_invalid" --config ../x.yaml --dry-run
rf 20 path_invalid "Z10 --db outside docs/ is REFUSED path_invalid" --db /etc/passwd --dry-run
rf 3 "" "Z11 a database that does not exist exits 3" --db docs/nope.db --dry-run
echo "this is not a database" >"$RS/docs/notdb.db"; rf 3 "" "Z12 a file that is not SQLite exits 3, nothing written" --db docs/notdb.db --dry-run
python3 -I -c 'import sqlite3,sys;c=sqlite3.connect(sys.argv[1]);c.execute("create table t(x)");c.commit()' "$RS/docs/plain.db"; rf 3 "" "Z13 a SQLite file that is not a register (no reg_meta) exits 3" --db docs/plain.db --dry-run
rf 2 "" "Z14 an unknown argument exits 2" --bogus
rf 2 "" "Z15 --tracker with an unknown name exits 2" --tracker nosuch --dry-run
rf 2 "" "Z16 --max-attempts 0 exits 2" --max-attempts 0 --dry-run
rf 2 "" "Z17 a missing option value exits 2" --json
out=$(env -u LOCKED_TEST_MODE LOCKED=/bin/true LOCKED_ROOT="$RS" bash "$SYNC" --config .helix/bad4.yaml --dry-run 2>&1); printf '%s' "$out" | grep -q "reason=config_missing" && ok "Z18 outside LOCKED_TEST_MODE the LOCKED and LOCKED_ROOT hooks are ignored (a config that exists only in the scratch root is not found: the real root is used)" || bad "Z18 [$out]"
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers: []\n' >"$RS/.helix/empty.yaml"
out=$(drv "$RS" --config .helix/empty.yaml --dry-run 2>&1); printf '%s' "$out" | grep -q '0 trackers configured; 0 pushes; nothing claimed as synced' && ok "Z19 trackers: [] and no candidates: the honest report is '0 trackers configured; 0 pushes; nothing claimed as synced'" || bad "Z19 [$out]"

echo "== the register's own CHECKs (docs/04 10.3) =="
python3 -I - "$RA/$DB" "$T_SCR/chk.db" <<'PY' && ok "K1 a VALID SYNCED row is accepted (positive control); a SYNCED row without an evidence record is rejected by the DDL CHECK; so are SYNCED with a non-zero exit_code or no reference, SKIPPED without a reason, FAILED with exit 0, and a reason outside the closed set, none of them by a foreign key" || bad "K1 DDL CHECKs"
import sqlite3,sys,urllib.parse
s=sqlite3.connect("file:"+urllib.parse.quote(sys.argv[1])+"?mode=ro",uri=True); d=sqlite3.connect(sys.argv[2]); s.backup(d); s.close()
d.execute("PRAGMA foreign_keys=ON")
Q="INSERT INTO reg_tracker_sync_log(tracker_id,atm_id,status,skip_reason,exit_code,remote_ref,evidence_id,item_revision,attempted_at) VALUES ('alpha','CAT-001',?,?,?,?,?,'rev','2026-10-07T00:00:00Z')"
ev=d.execute("select min(evidence_id) from reg_evidence where kind='tracker_receipt'").fetchone()[0]; assert ev, "no receipt row in the fixture"
d.execute("SAVEPOINT p"); d.execute(Q,("SYNCED",None,0,"r",ev)); d.execute("ROLLBACK TO p")   # positive control: a valid row IS accepted, so a rejection below is the CHECK and not a broken fixture
for row in [("SYNCED",None,0,"r",None),("SYNCED",None,1,"r",ev),("SYNCED",None,0,None,ev),("SKIPPED",None,None,None,None),("FAILED",None,0,None,None),("SKIPPED","nonsense",None,None,None)]:
    try: d.execute(Q,row)
    except sqlite3.IntegrityError as e:
        assert "FOREIGN KEY" not in str(e), "rejected by a foreign key, not by the CHECK: %r %s" % (row, e)
        continue
    raise SystemExit("accepted: %r" % (row,))
PY

echo "== leak scan: no credential value in any output, row, journal or batch file =="
dbhas() {  # dbhas DBFILE NEEDLE : exit 0 when a line of the database dump holds the needle
  python3 -I - "$1" "$2" <<'PY'
import sqlite3,sys,urllib.parse
c=sqlite3.connect("file:"+urllib.parse.quote(sys.argv[1])+"?mode=ro",uri=True)
sys.exit(0 if any(sys.argv[2] in l for l in c.iterdump()) else 1)
PY
}
leak_hits() {  # leak_hits NEEDLE : how many scanned places hold it (the output log, every fixture's .audit tree, every fixture's database dump)
  local needle=$1 n=0 d
  grep -qF -- "$needle" "$LEAK" && n=$((n+1))
  for d in "$RA" "$RS" "$RB" "$RC" "$RX"; do
    grep -rqF -- "$needle" "$d/.audit" 2>/dev/null && n=$((n+1))
    dbhas "$d/$DB" "$needle" && n=$((n+1))
  done
  echo "$n"
}
CTL="control-needle-leak-5c2e7a"
printf '%s\n' "$CTL" >>"$LEAK"; printf '%s\n' "$CTL" >"$RB/.audit/zz-leak-control.txt"
cp "$RC/$DB" "$T_SCR/ctl.db" && python3 -I -c 'import sqlite3,sys;c=sqlite3.connect(sys.argv[1]);c.execute("create table zz_ctl(x)");c.execute("insert into zz_ctl values (?)",(sys.argv[2],));c.commit()' "$T_SCR/ctl.db" "$CTL"
ctl_n=$(leak_hits "$CTL"); dbhas "$T_SCR/ctl.db" "$CTL" && ctl_db=found || ctl_db=blind
[ "$ctl_n" -ge 2 ] && [ "$ctl_db" = found ] && ok "L0 positive control: a needle planted in the output log, in a fixture .audit tree and in a database dump IS found by the leak scan ($ctl_n places, dump scan $ctl_db)" || bad "L0 the leak scan cannot see a planted needle (places=$ctl_n dump=$ctl_db): L1 would be a false null"
ctl_neg=$(leak_hits "absent-needle-never-written-3b91")
[ "$ctl_neg" = 0 ] && ok "L0b negative control: a needle that was never written is found nowhere" || bad "L0b the leak scan reports a needle that was never written ($ctl_neg)"
tok_hits=$(leak_hits "$CANARY_TOKEN"); oth_hits=$(leak_hits "$CANARY_OTHER")
[ "$tok_hits" = 0 ] && [ "$oth_hits" = 0 ] && ok "L1 neither canary value (the declared token, an undeclared secret) appears in any driver output, any fixture database dump, the journals, the batch files or the --json evidence of the five fixtures" || bad "L1 a canary value leaked (token places=$tok_hits other places=$oth_hits)"
finish
