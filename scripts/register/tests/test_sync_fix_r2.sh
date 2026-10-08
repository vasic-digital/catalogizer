#!/usr/bin/env bash
# test_sync_fix_r2.sh - WF23 fix round 2 of the WP-22 review (T188 convergence, 11.4.276): the defect CLASSES of the independent review of scripts/register/sync_trackers.sh,
# each pinned by checks that FAIL on the pre-fix driver. Real SQLite, the real engine, the REAL scripts/register/locked.sh (RUNP, podman, IMG-TESTUTIL via the logging shim
# of clib.sh) against SCRATCH roots cloned from one prototype register; the stand-ins are the tracker ADAPTER (a consumer script by contract) and, where a wrapper exit
# status is the subject, a thin wrapper that delegates to the real locked.sh.
# Env: SYNC substitutes a mutant or the pre-fix driver; R2_SECTIONS="secret wal ..." runs only those sections (mutation runs); SYNC_FAILFAST=1 stops at the first FAIL.
# Sections: secret (A1-A4, C5, F6, F7a/c), wal (B1, B3, B4, B5), proc (B2), breaker (C1), enabled (C2, F7d), receipt (C3), gate (D1), outbound (D2), revision (X05/X06/X09/X15/X16,
# X08, A1 persistence, D3), boundary (X17), backoff (X01), owed (C4), misc (X11, F7g/h).
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead WF23-fix-r2
if [ "${SYNC_FAILFAST:-}" = 1 ]; then bad() { FAIL=$((FAIL+1)); echo "FAIL $*"; echo "RESULT pass=$PASS fail=$FAIL (fail-fast, mutation runs)"; exit 1; }; fi
SYNC=${SYNC:-$REG_DIR/sync_trackers.sh}
echo "# sha256 sync_trackers=$(fsha "$SYNC" | cut -c1-64) config=$(fsha "$ROOT/.helix/reporting.yaml" | cut -c1-64)"
if [ ! -f "$SYNC" ]; then bad "S0 sync driver absent: $SYNC"; finish; exit 1; fi
. "$(dirname "$0")/sync_common.sh"
CANARY_TOKEN="tok-r2-canary-8e41b7"; CANARY_BODY="bodysecret-r2-canary-33d9a0"
LEAK="$T_SCR/leakscan.txt"; : >"$LEAK"; export R2_TOKEN="$CANARY_TOKEN"
sect() { [ -z "${R2_SECTIONS:-}" ] && return 0; case " $R2_SECTIONS " in *" $1 "*) return 0;; esac; return 1; }

# ---------------------------------------------------------------- the r2 adapter: per-item and per-tracker modes, a call counter, the reference it was handed
RAD='#!/usr/bin/env bash
R=$(cd "$(dirname "$0")" && pwd)
in=$(cat)
eval "$(printf "%s" "$in" | python3 -I -c "import json,sys,shlex;j=json.load(sys.stdin);print(\"id=%s tr=%s evd=%s ref=%s key=%s\" % tuple(shlex.quote(str(j.get(k,\"none\"))) for k in (\"atm_id\",\"tracker\",\"evidence_dir\",\"remote_ref\",\"idempotency_key\")))")"
n=$(( $(cat "$R/count.$tr" 2>/dev/null || echo 0) + 1 )); echo $n >"$R/count.$tr"
printf "%s tracker=%s ref=%s key=%s ts=%s argv=[%s]\n" "$id" "$tr" "$ref" "$key" "$(date +%s.%N)" "$*" >>"$R/adapter.log"
mode=$(cat "$R/mode.$tr.$id" 2>/dev/null || cat "$R/mode.$tr" 2>/dev/null || echo ok)
rec() { mkdir -p "$R/$evd"; printf "{\"receipt\":\"%s\",\"remote_ref\":\"%s\"}\n" "$id" "$1" >"$R/$evd/$tr-$id.json"; }
okout() { printf "{\"status\":\"SYNCED\",\"remote_ref\":%s,\"evidence_path\":\"%s/%s-%s.json\"%s}\n" "$1" "$evd" "$tr" "$id" "${2:-}"; }
r="$tr#$id#$n"
case "$mode" in
 ok) rec "$r"; okout "\"$r\"";;
 probe) rec "$r"; python3 -I -c "import sqlite3,sys,urllib.parse;c=sqlite3.connect(\"file:\"+urllib.parse.quote(sys.argv[1])+\"?mode=ro\",uri=True);print(c.execute(\"select count(*) from reg_tracker_sync_log where tracker_id=\x27p\x27 and status=\x27SYNCED\x27\").fetchone()[0])" "$R/docs/workable_items.db" >>"$R/q.sawp"; okout "\"$r\"";;
 drift) rec "$r"; okout "\"$r\"" ",\"remote_state\":\"closed\"";;
 nonutf8) rec "$r"; printf "caf\xe9 remote echo\n"; okout "\"$r\"";;
 killdriver) rec "$r"; okout "\"$r\""; kill -9 $PPID; sleep 5;;
 bg) rec "$r"; okout "\"$r\""; ( sleep 4; echo "late $id" >>"$R/late.log" ) & exit 0;;
 hang) ( sleep 4; echo "late $id" >>"$R/late.log" ) & sleep 30;;
 reject) echo "HTTP 422 Unprocessable Entity (the tracker answered)";;
 failrej) echo "{\"status\":\"FAILED\"}"; exit 3;;
 numref) rec 42; okout 42;;
 spaceref) rec "a b"; okout "\"a b\"";;
 evany) printf "{\"status\":\"SYNCED\",\"remote_ref\":\"x#%s\",\"evidence_path\":\".helix/reporting.yaml\"}\n" "$id";;
 stale) mkdir -p "$R/$evd"; printf "{\"receipt\":\"%s\",\"remote_ref\":\"%s\"}" "$id" "$r" >"$R/$evd/$tr-$id.json"; touch -d 2001-01-01 "$R/$evd/$tr-$id.json"; okout "\"$r\"";;
 slow2) sleep 8; rec "$r"; okout "\"$r\"";;
 wrongitem) r="$tr#X$n"; mkdir -p "$R/$evd"; printf "{\"receipt\":\"CAT-999\",\"remote_ref\":\"%s\"}" "$r" >"$R/$evd/$tr-$id.json"; okout "\"$r\"";;
 walcheck) printf "%s\n" "$(cat "$R"/.audit/sync/wal/*.jsonl 2>/dev/null | grep -c "\"atm_id\": \"$id\", .*\"ev\": \"intent\"")" >>"$R/walseen"; rec "$r"; okout "\"$r\"";;
 elsewhere) mkdir -p "$R/elsewhere"; printf "{\"receipt\":\"%s\",\"remote_ref\":\"%s\"}" "$id" "$r" >"$R/elsewhere/$tr-$id.json"; okout "\"$r\"" | sed "s#\"evidence_path\":\"[^\"]*\"#\"evidence_path\":\"elsewhere/$tr-$id.json\"#";;
 emptyrec) mkdir -p "$R/$evd"; : >"$R/$evd/$tr-$id.json"; okout "\"$r\"";;
 norefrec) mkdir -p "$R/$evd"; printf "{\"receipt\":\"%s\"}" "$id" >"$R/$evd/$tr-$id.json"; okout "\"$r\"";;
 otheritem) mkdir -p "$R/$evd"; printf "{\"receipt\":\"CAT-999\",\"remote_ref\":\"zzz#1\"}" >"$R/$evd/$tr-$id.json"; okout "\"$r\"";;
esac'
# a thin wrapper that delegates to the real locked.sh unless STUB_MODE selects a failure of one kind of write
STUB="$T_SCR/stub2_locked.sh"; cat >"$STUB" <<'SH'
#!/usr/bin/env bash
case "${STUB_MODE:-pass}" in
 logfail)   case "$2" in *-log-*) echo "boom-log" >&2; exit 1;; esac;;
 mintfail)  case "$2" in *-mint-*) echo "boom-mint" >&2; exit 1;; esac;;
 write21)   case "$2" in *-log-*) bash "$REAL_LOCKED" "$@"; rc=$?; [ "$rc" -eq 0 ] && exit 21; exit "$rc";; esac;;
 unknown22) case "$2" in *-log-*) echo "boom-22" >&2; exit 22;; esac;;
esac
exec bash "$REAL_LOCKED" "$@"
SH
chmod +x "$STUB"; export REAL_LOCKED="$REG_DIR/locked.sh"

# ---------------------------------------------------------------- fixtures: ONE prototype register (three items), cloned per scenario
PROTO=""
proto() {  # R2_PROTO_CACHE=<dir> (mutation runs): the prototype is built once and reused by every run that points at the same directory
  [ -n "$PROTO" ] && [ -d "$PROTO" ] && return 0
  if [ -n "${R2_PROTO_CACHE:-}" ] && [ -d "$R2_PROTO_CACHE/proto" ]; then PROTO="$R2_PROTO_CACHE/proto"; return 0; fi
  PROTO=$(mkfix proto 3) || { bad "setup proto"; finish; exit 1; }
  if [ -n "${R2_PROTO_CACHE:-}" ]; then mkdir -p "$R2_PROTO_CACHE" && cp -a "$PROTO" "$R2_PROTO_CACHE/proto.tmp" && mv "$R2_PROTO_CACHE/proto.tmp" "$R2_PROTO_CACHE/proto"; fi
  return 0; }
proto   # built here, in the main shell: a first call inside $(newp ...) would build it in a subshell and lose the variable
newp() {  # newp NAME -> a clone of the prototype with the r2 adapter and the p tracker (config: first-sync approver owner, pilot 1000)
  proto; local r="$T_SCR/$1"; cp -a "$PROTO" "$r" || return 1
  printf '%s\n' "$RAD" >"$r/adapter.sh"; : >"$r/adapter.log"
  pcfg "$r"; echo "$r"; }
pcfg() {
  cfg "$1" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: p, command: "bash $1/adapter.sh --repo r2 {id}", required_env: [R2_TOKEN], env_passthrough: {R2_DB: "{db}"}}
EOF
}
ycfg() { if grep -q first_sync_approvers "$SYNC"; then cat >"$1/.helix/reporting.yaml"; else grep -v '^first_sync_' >"$1/.helix/reporting.yaml"; fi; }   # ycfg ROOT <<EOF : a config written as is (first-sync keys dropped for a driver without the gate)
calls() { cat "$1/count.${2:-p}" 2>/dev/null || echo 0; }
step() {  # step ROOT LABEL sql-or-engine-command : a fixture step through the wrapper; a refused step is a failure of the TEST, not a verdict about the driver
  local r=$1 label=$2; shift 2; lk "$r" -- sh -c "$*" >/dev/null 2>"$T_SCR/step.err" || { bad "$label the fixture step was refused by the wrapper: $(head -c 300 "$T_SCR/step.err")"; return 1; }; }

if sect secret; then
echo "== secret handling and configuration classes (A1-A4, C5, F6, F7) =="
Q=$(newp Q) || { bad "setup Q"; finish; exit 1; }
sdry() {  # sdry 'command text' -> "rc|output" of a dry run with that adapter command
  printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - {name: a, command: "%s"%s}\n' "$1" "${2:-}" >"$Q/.helix/s.yaml"
  local o r; o=$(tool "$Q" "$SYNC" --config .helix/s.yaml --dry-run 2>&1 </dev/null); r=$?; printf '%s|%s' "$r" "$o"; }
CAN="CANARYR2v4417x9"
i=0; while IFS= read -r form; do i=$((i+1)); id=$(printf 'S%02d' $i)
  res=$(sdry "$form"); rc=${res%%|*}; out=${res#*|}; printf '%s\n' "$out" >>"$LEAK"
  if [ "$rc" = 20 ] && printf '%s' "$out" | grep -q 'reason=config_secret_in_command' && ! printf '%s' "$out" | grep -qF "$CAN"; then ok "$id a credential-bearing command is REFUSED config_secret_in_command and the value is not echoed: $(printf '%s' "$form" | sed "s/$CAN/<v>/")"; else bad "$id accepted or echoed: rc=$rc [$(printf '%s' "$out" | head -c 200)] for $(printf '%s' "$form" | sed "s/$CAN/<v>/")"; fi
done <<EOF
bash x.sh --token=$CAN
bash x.sh --access-token=$CAN
bash x.sh --private-token=$CAN
bash x.sh --api-key=$CAN
bash x.sh --api_key=$CAN
bash x.sh --password=$CAN
bash x.sh --secret=$CAN
bash x.sh --pat=$CAN
bash x.sh --auth=user:$CAN
bash x.sh --header=PRIVATE-TOKEN:$CAN
bash x.sh --token $CAN
bash x.sh --api-key $CAN
bash x.sh -t $CAN
curl -H 'Authorization: Bearer $CAN' https://api.example
bash x.sh https://user:$CAN@gitlab.example/api
env GITHUB_TOKEN=$CAN bash x.sh
bash x.sh --client_secret=$CAN
bash x.sh --Token=$CAN
bash x.sh ghp_aaaaaaaaaaaaaaaaaaaaaaaa
EOF
i=0; while IFS= read -r form; do i=$((i+1)); id=$(printf 'S5%d' $i)
  res=$(sdry "$form"); rc=${res%%|*}; out=${res#*|}
  if [ "$rc" = 0 ] && ! printf '%s' "$out" | grep -q 'REFUSED'; then ok "$id control: a plain adapter command is ACCEPTED: $form"; else bad "$id over-refusal: rc=$rc [$(printf '%s' "$out" | head -c 200)] for $form"; fi
done <<'EOF'
bash x.sh --repo owner/name --id {id}
python3 -m adapter --id {id}
bash x.sh --token-file=/etc/x/token
bash x.sh --token=$R2_NAME_ONLY
bash x.sh --token $R2_NAME_ONLY
bash x.sh issue create --label bug
EOF
res=$(sdry "bash x.sh" ", env_passthrough: {GITHUB_TOKEN: \"ghp_$CAN\"}"); rc=${res%%|*}; out=${res#*|}
[ "$rc" = 20 ] && printf '%s' "$out" | grep -q 'reason=config_secret_in_env' && ! printf '%s' "$out" | grep -qF "$CAN" && ok "S60 an env_passthrough LITERAL value is REFUSED config_secret_in_env and not echoed" || bad "S60 rc=$rc [$out]"
res=$(sdry "bash x.sh" ", env_passthrough: {R2_PLAIN: \"plain-literal\"}"); rc=${res%%|*}; [ "$rc" = 20 ] && ok "S61 any env_passthrough template without {db} or {id} is a literal: REFUSED" || bad "S61 rc=$rc"
res=$(sdry "bash x.sh" ", env_passthrough: {R2_DB: \"{db}\", R2_ID: \"x-{id}\"}"); rc=${res%%|*}; [ "$rc" = 0 ] && ok "S62 control: templates that use {db} and {id} are accepted" || bad "S62 rc=$rc [${res#*|}]"
res=$(sdry "bash x.sh" ", env_passthrough: [R2_NAME]"); rc=${res%%|*}; [ "$rc" = 20 ] && printf '%s' "${res#*|}" | grep -q 'reason=config_malformed' && ok "S63 the list form of env_passthrough is REFUSED (report_item.sh calls .items() on it: one file must serve both tools)" || bad "S63 rc=$rc [${res#*|}]"
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - name: a\n    command: "bash %s/adapter.sh"\n    required_env: [R2_DUP_TOKEN]\n    required_env: []\n' "$Q" >"$Q/.helix/dup1.yaml"
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - {name: a, command: "bash %s/adapter.sh"}\ntrackers: []\n' "$Q" >"$Q/.helix/dup2.yaml"
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - name: a\n    command: "bash %s/adapter.sh"\n    required_env: [R2_DUP_TOKEN]\n' "$Q" >"$Q/.helix/dup0.yaml"
for f in dup1 dup2; do o=$(tool "$Q" "$SYNC" --config .helix/$f.yaml --dry-run 2>&1); r=$?; [ "$r" = 20 ] && printf '%s' "$o" | grep -q 'reason=config_malformed' && ok "S7-$f a duplicate YAML key is REFUSED config_malformed (the last one would silently win)" || bad "S7-$f rc=$r [$o]"; done
o=$( (unset R2_DUP_TOKEN; tool "$Q" "$SYNC" --config .helix/dup0.yaml --dry-run 2>&1) ); printf '%s' "$o" | grep -q 'tracker=a state=credentials_absent .*missing_env_names=R2_DUP_TOKEN' && ok "S7-control the same config without the duplicate key checks the credential (credentials_absent R2_DUP_TOKEN)" || bad "S7-control [$o]"
printf 'schema_version: 1\ndb: docs/workable_items.db\nid_prefix: CAT\ntrackers:\n  - {name: a, command: "bash %s/adapter.sh {id}", required_env: [R2_WS_TOK, R2_OK_A, r2_lower_name]}\n' "$Q" >"$Q/.helix/env.yaml"
o=$( (export R2_WS_TOK='   ' R2_OK_A=x; unset r2_lower_name; tool "$Q" "$SYNC" --config .helix/env.yaml --dry-run 2>&1) )
printf '%s' "$o" | grep -q 'tracker=a state=credentials_absent .*missing_env_names=R2_WS_TOK,r2_lower_name$' && ok "S8 a whitespace-only credential counts as ABSENT (A4); a lower-case variable name is accepted (F7c); only the missing NAMES are listed, not the set one (R2_OK_A)" || bad "S8 [$o]"
o=$( (export R2_WS_TOK=v R2_OK_A=x r2_lower_name=y; tool "$Q" "$SYNC" --config .helix/env.yaml --dry-run 2>&1) ); printf '%s' "$o" | grep -q 'tracker=a state=adapter' && ok "S8-control with all three set the adapter state is reached" || bad "S8-control [$o]"
printf '#!/usr/bin/env bash\nexit 0\n' >"$Q/ok_r2.sh"; chmod +x "$Q/ok_r2.sh"
for cmd in "/usr/bin/env python3 $Q/missing_r2.py" "bash $Q/missing_r2.sh" "python3 $Q/missing_r2.py" "env R2_X=1 bash $Q/missing_r2.sh" "/usr/bin/env R2_X=1 no-such-cli-r2"; do
  res=$(sdry "$cmd"); printf '%s' "${res#*|}" | grep -q 'tracker=a state=tracker_client_absent' && ok "S9 missing client is tracker_client_absent: $(printf '%s' "$cmd" | sed "s#$Q#<root>#g")" || bad "S9 [$res] for $cmd"; done
for cmd in "bash $Q/ok_r2.sh" "env R2_X=1 bash $Q/ok_r2.sh" "/usr/bin/env bash $Q/ok_r2.sh"; do
  res=$(sdry "$cmd"); printf '%s' "${res#*|}" | grep -q 'tracker=a state=adapter' && ok "S9-control an existing client reaches the adapter state: $(printf '%s' "$cmd" | sed "s#$Q#<root>#g")" || bad "S9-control [$res] for $cmd"; done
cp "$Q/scripts/register/register_ext.sql" "$T_SCR/ddl.before"
o=$(tool "$Q" "$SYNC" --dry-run --json scripts/register/register_ext.sql 2>&1); r=$?
[ "$r" = 20 ] && printf '%s' "$o" | grep -q 'reason=path_invalid' && cmp -s "$Q/scripts/register/register_ext.sql" "$T_SCR/ddl.before" && ok "S10 --json outside .audit/ and the evidence directory is REFUSED path_invalid and replaces nothing" || bad "S10 rc=$r [$o]"
o=$(tool "$Q" "$SYNC" --dry-run --json .audit/r2-plan.json 2>&1); r=$?; [ "$r" = 0 ] && [ -s "$Q/.audit/r2-plan.json" ] && ok "S10-control --json under .audit/ is written" || bad "S10-control rc=$r [$o]"
fi

if sect wal; then
echo "== durability of remote effects: write-ahead journal, crash, undecodable output, wrapper exit 21/22, references (B1, B3, B4, B5) =="
W1=$(newp W1) || { bad "setup W1"; finish; exit 1; }
echo nonutf8 >"$W1/mode.p.CAT-002"
out=$(drv "$W1" --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "WA1 an adapter that prints one non-UTF-8 byte does not crash the driver (exit 0) and every push is recorded: 3 SYNCED rows, 3 adapter calls" "$rc/$(pq "$W1" "select count(*) from reg_tracker_sync_log where status='SYNCED'")/$(calls "$W1")" "0/3/3"
out=$(drv "$W1" --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "WA2 a re-run pushes nothing again (no duplicate remote issue, no duplicate row): still 3 adapter calls and 3 SYNCED rows" "$rc/$(calls "$W1")/$(pq "$W1" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "0/3/3"
W2=$(newp W2) || { bad "setup W2"; finish; exit 1; }
FS=$(pflags "$W2"); out=$( (LOCKED="$STUB" STUB_MODE=logfail tool "$W2" "$SYNC" $FS 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
assert_eq "WB1 a failed register write: exit 4, the three pushes happened (3 calls), no row is recorded, the wal holds the results" "$rc/$(calls "$W2")/$(pq "$W2" "select count(*) from reg_tracker_sync_log where status='SYNCED'")/$(ls "$W2"/.audit/sync/wal/*.jsonl 2>/dev/null | wc -l)" "4/3/0/1"
out=$(drv "$W2" 2>&1); rc=$?; printf '%s\n' "$out" >>"$LEAK"
assert_eq "WB2 the NEXT run reconciles the wal first: 3 SYNCED rows with their references are written, nothing is pushed again (still 3 calls)" "$rc/$(pq "$W2" "select count(*) from reg_tracker_sync_log where status='SYNCED' and remote_ref like 'p#CAT-%'")/$(calls "$W2")" "0/3/3"
printf '%s' "$out" | grep -q 'recovered 3 row' && ok "WB3 the recovery is reported ('recovered 3 row(s)')" || bad "WB3 [$out]"
step "$W2" WB4pre "$WI_IN update --id CAT-001 --db /src/$DB --title r2-changed-title" && assert_eq "WB4pre the fixture step took effect" "$(pq "$W2" "select title from items where atm_id='CAT-001'")" "r2-changed-title"
ref1=$(pq "$W2" "select remote_ref from reg_tracker_sync_log where atm_id='CAT-001' and status='SYNCED' order by sync_id desc limit 1")
out=$(drv "$W2" 2>&1); rc=$?
assert_eq "WB4 the changed item is an UPDATE: the adapter is handed the recovered remote_ref ($ref1), not nothing" "$rc/$(grep '^CAT-001 ' "$W2/adapter.log" | tail -n1 | sed 's/.* ref=\([^ ]*\) .*/\1/')" "0/$ref1"
W3=$(newp W3) || { bad "setup W3"; finish; exit 1; }
echo killdriver >"$W3/mode.p.CAT-002"
FS=$(pflags "$W3"); out=$(tool "$W3" "$SYNC" $FS 2>&1); rc=$?
assert_eq "WC1 control: the driver is killed (SIGKILL) by the adapter of CAT-002 after its push: exit 137, 2 calls, no row recorded" "$rc/$(calls "$W3")/$(pq "$W3" "select count(*) from reg_tracker_sync_log")" "137/2/0"
echo ok >"$W3/mode.p.CAT-002"
out=$(drv "$W3" 2>&1); rc=$?; printf '%s\n' "$out" >>"$LEAK"
assert_eq "WC2 after the crash: CAT-001 (pushed, never recorded) is recovered SYNCED, CAT-002 (outcome unknown) is FAILED 75 and HELD, CAT-003 is pushed; 3 calls in total (nothing pushed twice); exit 7" "$rc/$(pq "$W3" "select group_concat(atm_id||':'||status||':'||ifnull(exit_code,'')) from (select atm_id,status,exit_code from reg_tracker_sync_log order by atm_id)")/$(calls "$W3")" "7/CAT-001:SYNCED:0,CAT-002:FAILED:75,CAT-003:SYNCED:0/3"
printf '%s' "$out" | grep -q 'held_indeterminate=1' && ok "WC3 the held item is reported (held_indeterminate=1)" || bad "WC3 [$out]"
out=$(drv "$W3" --retry-indeterminate 2>&1); rc=$?
assert_eq "WC4 --retry-indeterminate releases the held item: pushed (4 calls), SYNCED, exit 0" "$rc/$(calls "$W3")/$(pq "$W3" "select status from reg_tracker_sync_log where atm_id='CAT-002' order by sync_id desc limit 1")" "0/4/SYNCED"
W4=$(newp W4) || { bad "setup W4"; finish; exit 1; }
FS=$(pflags "$W4"); out=$( (LOCKED="$STUB" STUB_MODE=write21 tool "$W4" "$SYNC" $FS 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
printf '%s' "$out" | grep -q 'verified present' && ! printf '%s' "$out" | grep -q 'NOT recorded' && assert_eq "WD1 locked.sh exit 21 (the write ran, its journal row did not): the rows are read back, found, NOT reported as unrecorded; exit 0, 3 SYNCED rows" "$rc/$(pq "$W4" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "0/3" || bad "WD1 rc=$rc [$out]"
W5=$(newp W5) || { bad "setup W5"; finish; exit 1; }
FS=$(pflags "$W5"); out=$( (LOCKED="$STUB" STUB_MODE=unknown22 tool "$W5" "$SYNC" $FS 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
printf '%s' "$out" | grep -q 'NOT recorded' && assert_eq "WD2 locked.sh exit 22 with nothing written: the read-back finds no row, the rows are reported NOT recorded, exit 4, 0 rows" "$rc/$(pq "$W5" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "4/0" || bad "WD2 rc=$rc [$out]"
W6=$(newp W6) || { bad "setup W6"; finish; exit 1; }
echo numref >"$W6/mode.p.CAT-001"; echo spaceref >"$W6/mode.p.CAT-002"
out=$(drv "$W6" --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "WE1 an integer remote_ref (a GitHub issue number) is SYNCED with the text '42'; a reference with a space is FAILED 65 and kept sanitized; the third item is SYNCED; exit 1" "$rc/$(pq "$W6" "select group_concat(atm_id||':'||status||':'||ifnull(exit_code,'')||':'||ifnull(remote_ref,'')) from (select atm_id,status,exit_code,remote_ref from reg_tracker_sync_log where atm_id in ('CAT-001','CAT-002','CAT-003') order by atm_id)")" "1/CAT-001:SYNCED:0:42,CAT-002:FAILED:65:a?b,CAT-003:SYNCED:0:p#CAT-003#3"
W10=$(newp W10) || { bad "setup W10"; finish; exit 1; }
echo walcheck >"$W10/mode.p"; : >"$W10/walseen"
out=$(drv "$W10" --max-attempts 1 2>&1); rc=$?
assert_eq "WI1 write-ahead means BEFORE: while the adapter runs, its item's intent record is already in the journal on disk (the adapter looked: 1 for each of the three items)" "$rc/$(tr '\n' ',' <"$W10/walseen")" "0/1,1,1,"
W7=$(newp W7) || { bad "setup W7"; finish; exit 1; }
echo slow2 >"$W7/mode.p.CAT-003"; FS=$(pflags "$W7")
env LOCKED_TEST_MODE=1 LOCKED_ROOT="$W7" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$SYNC" $FS >"$W7/term.out" 2>&1 & dp=$!
n=0; while [ "$(calls "$W7")" != 3 ] && [ $n -lt 600 ] && kill -0 $dp 2>/dev/null; do n=$((n+1)); sleep 0.5; done
sleep 1; kill -TERM $dp 2>/dev/null; wait $dp; rc=$?; sleep 1
assert_eq "WF1 SIGTERM while the third push is in flight: exit 6, the two finished pushes are flushed to the register (2 SYNCED rows), the in-flight adapter group is gone" "$rc/$(pq "$W7" "select count(*) from reg_tracker_sync_log where status='SYNCED'")/$(pgrep -f "$W7/adapter.sh" | wc -l)" "6/2/0"
grep -q 'interrupted by signal 15' "$W7/term.out" && ok "WF2 the interruption is reported (signal 15)" || bad "WF2 [$(head -c 300 "$W7/term.out")]"
out=$(drv "$W7" 2>&1); rc=$?
assert_eq "WF3 the next run holds the interrupted item (FAILED 75, exit 7) and pushes nothing again (still 3 calls)" "$rc/$(pq "$W7" "select status||exit_code from reg_tracker_sync_log where atm_id='CAT-003' order by sync_id desc limit 1")/$(calls "$W7")" "7/FAILED75/3"
W8=$(newp W8) || { bad "setup W8"; finish; exit 1; }
mkdir -p "$W8/.audit/sync/wal"; chmod 555 "$W8/.audit/sync/wal"; FS=$(pflags "$W8")
out=$(tool "$W8" "$SYNC" $FS 2>&1); rc=$?; chmod 755 "$W8/.audit/sync/wal"
assert_eq "WG1 write-ahead means ahead: when the journal cannot be written NOTHING is pushed (exit 5, 0 adapter calls, 0 rows)" "$rc/$(calls "$W8")/$(pq "$W8" "select count(*) from reg_tracker_sync_log")" "5/0/0"
W9=$(newp W9) || { bad "setup W9"; finish; exit 1; }
echo killdriver >"$W9/mode.p.CAT-002"; FS=$(pflags "$W9"); out=$(tool "$W9" "$SYNC" $FS 2>&1)
cp "$W9/$DB" "$W9/docs/other.db"; echo ok >"$W9/mode.p.CAT-002"
out=$(drv "$W9" --db docs/other.db --limit 1 2>&1); rc=$?
other75=$( (DB=docs/other.db; pq "$W9" "select count(*) from reg_tracker_sync_log where exit_code=75") )
out=$(drv "$W9" 2>&1); rc2=$?
assert_eq "WH1 a write-ahead journal is replayed only into the register it was written for: a run on docs/other.db does not recover the crashed run's rows (0 rows with exit 75 there); the run on the original database does (CAT-002 FAILED 75, CAT-001 SYNCED)" "$rc/$other75/$(pq "$W9" "select group_concat(atm_id||':'||status||':'||ifnull(exit_code,'')) from (select atm_id,status,exit_code from reg_tracker_sync_log where atm_id in ('CAT-001','CAT-002') order by atm_id)")" "0/0/CAT-001:SYNCED:0,CAT-002:FAILED:75"
fi

if sect proc; then
echo "== the adapter runs in its own process group (B2) =="
P1=$(newp P1) || { bad "setup P1"; finish; exit 1; }
( sleep 2; echo late >>"$P1/late.control" ) & sleep 3.5; wait
[ -s "$P1/late.control" ] && ok "PA0 control: a background child that nothing kills DOES write its late line (the observation below can see a late push)" || bad "PA0 control failed"
echo hang >"$P1/mode.p"
out=$(drv "$P1" --adapter-timeout 1 --max-attempts 3 --backoff-base 0 --limit 2 2>&1); rc=$?
sleep 6
assert_eq "PA1 a hanging adapter with a background child: FAILED 124 for both items, exit 1, NOT retried (2 calls although --max-attempts 3: the first push may have landed), and the background child was killed with its group (no late.log)" "$rc/$(pq "$P1" "select group_concat(status||exit_code) from reg_tracker_sync_log where atm_id in ('CAT-001','CAT-002')")/$(calls "$P1")/$([ -e "$P1/late.log" ] && echo late || echo none)" "1/FAILED124,FAILED124/2/none"
killpg_check() {  # killpg_check DRIVER : every os.killpg call lies inside a function whose body tests pgid <= 1 first (constitution 11.4.263); exit 0 = guarded
  python3 -I - "$1" <<'PY'
import ast,sys
src=open(sys.argv[1]).read(); body=src.split("exec python3 -I - \"$@\" <<'PY'\n")[1].rsplit("\nPY\n",1)[0]
t=ast.parse(body); calls=0
for fn in [n for n in ast.walk(t) if isinstance(n,ast.FunctionDef)]:
    inner=[n for n in ast.walk(fn) if isinstance(n,ast.Call) and isinstance(n.func,ast.Attribute) and n.func.attr=="killpg"]
    if not inner: continue
    calls+=len(inner); text=ast.get_source_segment(body,fn)
    first=min(n.lineno for n in inner)
    guard=[n for n in ast.walk(fn) if isinstance(n,ast.Compare) and any(isinstance(o,ast.LtE) for o in n.ops) and "pgid" in ast.dump(n) and n.lineno<first]
    if not guard: sys.exit(1)
sys.exit(0 if calls else 1)
PY
}
sed 's/pgid <= 1 or pgid == os.getpgrp()/False/' "$SYNC" >"$T_SCR/noguard.sh"
killpg_check "$SYNC" && ! killpg_check "$T_SCR/noguard.sh" && ok "PA3 every os.killpg call of the driver sits behind the pgid <= 1 guard (11.4.263); control: the same check FAILS on a copy with the guard removed" || bad "PA3 killpg guard check (driver=$(killpg_check "$SYNC" && echo guarded || echo UNGUARDED) control=$(killpg_check "$T_SCR/noguard.sh" && echo guarded || echo UNGUARDED)"
P2=$(newp P2) || { bad "setup P2"; finish; exit 1; }
echo bg >"$P2/mode.p"
out=$(drv "$P2" --adapter-timeout 2 --limit 1 2>&1); rc=$?; sleep 6
assert_eq "PA2 an adapter that returned SYNCED and left a child holding its output is SYNCED (not FAILED 124 and retried), and the stray child is killed (no late.log)" "$rc/$(pq "$P2" "select status from reg_tracker_sync_log where atm_id='CAT-001'")/$(calls "$P2")/$([ -e "$P2/late.log" ] && echo late || echo none)" "0/SYNCED/1/none"
fi

if sect breaker; then
echo "== circuit breaker: unreachable versus rejecting (C1) and consecutive failures (X02) =="
B1=$(newp B1) || { bad "setup B1"; finish; exit 1; }
echo reject >"$B1/mode.p"
out=$(drv "$B1" --breaker 2 --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "BA1 a tracker that ANSWERS every call (HTTP 422) is not unreachable: exit 1, 2 FAILED rows, NO SKIPPED unreachable row, no unreachable item minted, the failed item minted" "$rc/$(pq "$B1" "select count(*) from reg_tracker_sync_log where status='FAILED'")/$(pq "$B1" "select count(*) from reg_tracker_sync_log where skip_reason='unreachable'")/$(pq "$B1" "select count(*) from reg_ids where minted_by='sync_trackers:p:unreachable'")/$(pq "$B1" "select count(*) from reg_ids where minted_by='sync_trackers:p:failed'")" "1/2/0/0/1"
printf '%s' "$out" | grep -Eq 'breaker_open=[1-9]' && ok "BA2 the items the breaker did not attempt are counted breaker_open" || bad "BA2 [$out]"
B2=$(newp B2) || { bad "setup B2"; finish; exit 1; }
echo failrej >"$B2/mode.p.CAT-001"; echo failrej >"$B2/mode.p.CAT-003"
out=$(drv "$B2" --breaker 2 --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "BB1 non-consecutive failures (fail, ok, fail) do not trip a breaker of 2: all four items (3 + the minted failed item) were attempted, the two good ones and the minted one SYNCED" "$rc/$(calls "$B2")/$(pq "$B2" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "1/4/2"
fi

if sect enabled; then
echo "== reg_trackers.enabled, retirement, v_stale_tracker_sync (C2) and a foreign missing_env_names value (F7d) =="
E1=$(newp E1) || { bad "setup E1"; finish; exit 1; }
cfg "$E1" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: gone}
  - {name: dis, enabled: false, command: "bash $E1/adapter.sh {id}"}
  - {name: keep}
EOF
out=$(drv "$E1" 2>&1); rc=$?
assert_eq "EA1 reg_trackers.enabled follows the config: the disabled tracker is 0, the others 1" "$rc/$(pq "$E1" "select group_concat(tracker_id||'='||enabled) from (select * from reg_trackers order by 1)")" "0/dis=0,gone=1,keep=1"
cfg "$E1" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: dis, enabled: false, command: "bash $E1/adapter.sh {id}"}
  - {name: keep}
EOF
out=$(drv "$E1" --tracker keep 2>&1); rc=$?
assert_eq "EA2 a subset run (--tracker) cannot tell that a tracker left the config: nothing is retired" "$rc/$(pq "$E1" "select enabled from reg_trackers where tracker_id='gone'")" "0/1"
out=$(drv "$E1" 2>&1); rc=$?
assert_eq "EA3 a full run retires the tracker the config no longer names (enabled=0) and v_stale_tracker_sync no longer lists it; the output says so" "$rc/$(pq "$E1" "select enabled from reg_trackers where tracker_id='gone'")/$(pq "$E1" "select count(*) from v_stale_tracker_sync where tracker_id='gone'")/$(printf '%s' "$out" | grep -c 'retired tracker')" "0/0/0/1"
assert_eq "EA4 the disabled tracker is not listed either; the live one is (5 items: 3 + the two minted, all SKIPPED)" "$(pq "$E1" "select count(*) from v_stale_tracker_sync where tracker_id='dis'")/$(pq "$E1" "select count(*)||'/'||min(last_status) from v_stale_tracker_sync where tracker_id='keep'")" "0/5/SKIPPED"
step "$E1" EB1pre "sqlite3 /src/$DB \"UPDATE reg_tracker_sync_log SET missing_env_names='notjson' WHERE tracker_id='keep' AND atm_id='CAT-001'\"" && assert_eq "EB1pre the fixture step took effect" "$(pq "$E1" "select missing_env_names from reg_tracker_sync_log where tracker_id='keep' and atm_id='CAT-001' limit 1")" "notjson"
out=$(drv "$E1" 2>&1); rc=$?
assert_eq "EB1 a missing_env_names value that is not JSON (written by another tool) does not crash the selection: exit 0" "$rc" "0"
fi

if sect receipt; then
echo "== a SYNCED receipt belongs to this run, this item and this reference (C3, X03) =="
R1=$(newp R1) || { bad "setup R1"; finish; exit 1; }
for m in evany stale emptyrec norefrec otheritem wrongitem elsewhere; do echo $m >"$R1/mode.p"
  out=$(drv "$R1" --limit 1 --max-attempts 1 --backoff-base 0 2>&1); rc=$?
  assert_eq "RA-$m a receipt that is $m is never SYNCED: CAT-001 FAILED 65, exit 1" "$rc/$(pq "$R1" "select status||exit_code from reg_tracker_sync_log where atm_id='CAT-001' order by sync_id desc limit 1")" "1/FAILED65"; done
echo ok >"$R1/mode.p"
out=$(drv "$R1" --limit 1 --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "RA-ok control: a receipt written during the call, under this run's evidence directory, naming the item and the reference is SYNCED" "$rc/$(pq "$R1" "select status from reg_tracker_sync_log where atm_id='CAT-001' order by sync_id desc limit 1")/$(pq "$R1" "select count(*) from reg_evidence e join reg_tracker_sync_log l on l.evidence_id=e.evidence_id where e.path like '%/tracker-sync/sync-%/p-CAT-001.json'")" "0/SYNCED/1"
fi

if sect gate; then
echo "== first-sync gate: dry run, owner approval of the plan hash, pilot (D1) =="
G=$(newp G) || { bad "setup G"; finish; exit 1; }
cat >"$G/.helix/reporting.yaml" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
first_sync_approvers: [owner]
first_sync_pilot_max: 2
trackers:
  - {name: p, command: "bash $G/adapter.sh {id}", required_env: [R2_TOKEN]}
EOF
shaG=$(fsha "$G/$DB")
out=$(tool "$G" "$SYNC" 2>&1); rc=$?; printf '%s\n' "$out" >>"$LEAK"
assert_eq "GA1 a never-synced tracker with no approval: REFUSED first_sync_gate (20), no adapter call, the database untouched, no wal, no tracker row" "$rc/$(printf '%s' "$out" | grep -c 'reason=first_sync_gate')/$(calls "$G")/$([ "$(fsha "$G/$DB")" = "$shaG" ] && echo same)/$([ -e "$G/.audit/sync/wal" ] && echo wal || echo nowal)" "20/1/0/same/nowal"
dry=$(tool "$G" "$SYNC" --dry-run 2>&1); rc=$?; H=$(printf '%s\n' "$dry" | sed -n 's/.*first_sync_gate trackers=p plan_hash=\([0-9a-f]\{64\}\).*/\1/p')
[ "$rc" = 0 ] && [ -n "$H" ] && [ "$(fsha "$G/$DB")" = "$shaG" ] && ok "GA2 a dry run prints the plan hash of the first sync and writes nothing" || bad "GA2 rc=$rc [$dry]"
bh=$(printf '%s' "$H" | rev)
for v in "--approved-by owner --approve-first-sync $bh --limit 2" "--approved-by mallory --approve-first-sync $H --limit 2" "--approve-first-sync $H --limit 2" "--approved-by owner --approve-first-sync $H" "--approved-by owner --approve-first-sync $H --limit 3"; do
  out=$(tool "$G" "$SYNC" $v 2>&1); rc=$?
  [ "$rc" = 20 ] && printf '%s' "$out" | grep -q 'reason=first_sync_gate' && [ "$(calls "$G")" = 0 ] && ok "GA3 refused: $(printf '%s' "$v" | sed "s/$H/<plan>/; s/$bh/<other>/")" || bad "GA3 rc=$rc calls=$(calls "$G") [$out] for $v"; done
step "$G" GA4pre "$(mint_cmd workable_items.db)" || true
out=$(tool "$G" "$SYNC" --approved-by owner --approve-first-sync "$H" --limit 2 2>&1); rc=$?
[ "$rc" = 20 ] && [ "$(calls "$G")" = 0 ] && ok "GA4 an approval of an OLD plan is refused once the item set changed (a fourth item appeared after the dry run)" || bad "GA4 rc=$rc [$out]"
dry=$(tool "$G" "$SYNC" --dry-run 2>&1); H=$(printf '%s\n' "$dry" | sed -n 's/.*first_sync_gate trackers=p plan_hash=\([0-9a-f]\{64\}\).*/\1/p')
out=$(tool "$G" "$SYNC" --approved-by owner --approve-first-sync "$H" --limit 2 2>&1); rc=$?
assert_eq "GA5 the approved pilot (--limit 2) runs: exit 0, exactly 2 pushes, 2 SYNCED rows" "$rc/$(calls "$G")/$(pq "$G" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "0/2/2"
out=$(tool "$G" "$SYNC" 2>&1); rc=$?
assert_eq "GA6 after the pilot the gate no longer applies: a plain run pushes the rest (4 SYNCED rows)" "$rc/$(pq "$G" "select count(*) from reg_tracker_sync_log where status='SYNCED'")" "0/4"
G2=$(newp G2) || { bad "setup G2"; finish; exit 1; }
cat >"$G2/.helix/reporting.yaml" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: p, command: "bash $G2/adapter.sh {id}", required_env: [R2_TOKEN]}
EOF
dry=$(tool "$G2" "$SYNC" --dry-run 2>&1); H2=$(printf '%s\n' "$dry" | sed -n 's/.*first_sync_gate trackers=p plan_hash=\([0-9a-f]\{64\}\).*/\1/p')
out=$(tool "$G2" "$SYNC" --approved-by owner --approve-first-sync "$H2" --limit 1 2>&1); rc=$?
[ "$rc" = 20 ] && [ "$(calls "$G2")" = 0 ] && ok "GA7 a config with no first_sync_approvers cannot approve anything: REFUSED even with a matching hash" || bad "GA7 rc=$rc [$out]"
fi

if sect outbound; then
echo "== confidentiality filter and secret scan on the outbound payload (D2) =="
O=$(newp O) || { bad "setup O"; finish; exit 1; }
ycfg "$O" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
first_sync_approvers: [owner]
first_sync_pilot_max: 1000
trackers:
  - {name: pubt, command: "bash $O/adapter.sh {id}", required_env: [R2_TOKEN]}
  - {name: privt, public: false, command: "bash $O/adapter.sh {id}", required_env: [R2_TOKEN]}
EOF
step "$O" OApre1 "sqlite3 /src/$DB \"PRAGMA foreign_keys=ON; INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('CAT-002','danger_zone','runtime','high');\"" || true
step "$O" OApre2 "$WI_IN update --id CAT-003 --db /src/$DB --description 'Rotation notes for the staging host: password=$CANARY_BODY must be changed after the audit, and this sentence makes the text long enough.'" || true
assert_eq "OApre the fixture steps took effect (category row, secret in the body)" "$(pq "$O" "select category from reg_item_ext where atm_id='CAT-002'")/$(pq "$O" "select count(*) from items where atm_id='CAT-003' and description like '%password=%'")" "danger_zone/1"
out=$(drv "$O" --max-attempts 1 --backoff-base 0 2>&1); rc=$?
assert_eq "OA1 public tracker: CAT-001 SYNCED, the danger_zone item CAT-002 FAILED 78 (withheld), the item with a secret in its body CAT-003 FAILED 77; private tracker: CAT-002 SYNCED, CAT-003 FAILED 77; exit 0 (withholding is policy, not a failure)" "$rc/$(pq "$O" "select group_concat(tracker_id||atm_id||status||ifnull(exit_code,'')) from (select tracker_id,atm_id,status,exit_code from reg_tracker_sync_log where atm_id in ('CAT-001','CAT-002','CAT-003') order by 1,2)")" "0/privtCAT-001SYNCED0,privtCAT-002SYNCED0,privtCAT-003FAILED77,pubtCAT-001SYNCED0,pubtCAT-002FAILED78,pubtCAT-003FAILED77"
assert_eq "OA2 the adapter never saw the withheld items: no CAT-003 call at all, CAT-002 only for the private tracker; and it DID see CAT-001 on both trackers (the run happened, so the zeros are not vacuous)" "$(grep -c '^CAT-003 ' "$O/adapter.log")/$(grep '^CAT-002 ' "$O/adapter.log" | grep -c 'tracker=pubt')/$(grep -c '^CAT-001 ' "$O/adapter.log")/$(grep '^CAT-002 ' "$O/adapter.log" | grep -c 'tracker=privt')" "0/0/2/1"
anytable() {  # anytable DB NEEDLE TABLE-LIKE : exit 0 when a row of a matching table holds the needle
  python3 -I - "$1" "$2" "$3" <<'PY'
import sqlite3,sys,urllib.parse
c=sqlite3.connect("file:"+urllib.parse.quote(sys.argv[1])+"?mode=ro",uri=True)
tabs=[r[0] for r in c.execute("select name from sqlite_master where type='table' and name like ?", (sys.argv[3],))]
sys.exit(0 if any(sys.argv[2] in str(row) for t in tabs for row in c.execute("select * from %s" % t)) else 1)
PY
}
syncrows() {  # the journal rows of the DRIVER's own wrapper calls (op id sync-...); the fixture step that planted the secret is a wrapper call too and is not the driver
  python3 -I - "$1/.audit/register/journal.jsonl" <<'PY'
import json,sys
for l in open(sys.argv[1]):
    try: r=json.loads(l)
    except ValueError: continue
    if str(r.get("op_id","")).startswith("sync-"): print(json.dumps(r))
PY
}
leak=0; grep -qF "$CANARY_BODY" "$LEAK" && leak=1; grep -rqF "$CANARY_BODY" "$O/.audit/sync" "$O/.audit/register/inputs" "$O/adapter.log" 2>/dev/null && leak=1; printf '%s\n' "$out" | grep -qF "$CANARY_BODY" && leak=1
syncrows "$O" | grep -qF "$CANARY_BODY" && leak=1
ctl_j=0; grep -qF "$CANARY_BODY" "$O/.audit/register/journal.jsonl" && ctl_j=1   # control: the planting step IS in the journal, so a journal scan can see the canary
anytable "$O/$DB" "$CANARY_BODY" 'reg_%' && leak=1
ctl=0; anytable "$O/$DB" "$CANARY_BODY" 'items' && ctl=1
ran=$(wc -l <"$O/adapter.log")
[ "$ran" -ge 3 ] && [ "$leak" = 0 ] && [ "$ctl" = 1 ] && [ "$ctl_j" = 1 ] && ok "OA3 the secret found in a body is not in any driver output, any reg_* table, the driver's batch files, wal and journal rows, or the adapter log (controls: the same scans find it in the items table and in the journal row of the fixture step that planted it)" || bad "OA3 adapter_calls=$ran (need >= 3 for a non-vacuous scan) leak=$leak control_items=$ctl control_journal=$ctl_j"
n0=$(pq "$O" "select count(*) from reg_tracker_sync_log")
out=$(drv "$O" --max-attempts 1 2>&1); rc=$?
assert_eq "OA4 withheld items are not re-recorded on an unchanged re-run: no new row, exit 0" "$rc/$(pq "$O" "select count(*) from reg_tracker_sync_log")" "0/$n0"
ycfg "$O" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
first_sync_approvers: [owner]
first_sync_pilot_max: 1000
trackers:
  - {name: pubt, public: false, command: "bash $O/adapter.sh {id}", required_env: [R2_TOKEN]}
  - {name: privt, public: false, command: "bash $O/adapter.sh {id}", required_env: [R2_TOKEN]}
EOF
out=$(drv "$O" --max-attempts 1 2>&1); rc=$?
assert_eq "OA5 once the tracker is declared public: false, the previously withheld danger_zone item is pushed to it; the secret body stays withheld" "$rc/$(pq "$O" "select status from reg_tracker_sync_log where tracker_id='pubt' and atm_id='CAT-002' order by sync_id desc limit 1")/$(pq "$O" "select status||exit_code from reg_tracker_sync_log where tracker_id='pubt' and atm_id='CAT-003' order by sync_id desc limit 1")" "0/SYNCED/FAILED77"
fi

if sect revision; then
echo "== what a re-push needs: revision fields, the latest reference, references not expanded, the command not persisted, drift (X05 X06 X08 X09 X15 X16 A1 D3) =="
V=$(newp V) || { bad "setup V"; finish; exit 1; }
ycfg "$V" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
first_sync_approvers: [owner]
first_sync_pilot_max: 1000
trackers:
  - {name: p, command: "bash $V/adapter.sh --token \$R2_TOK --repo r2uniqueword {id}", required_env: [R2_TOKEN]}
EOF
export R2_TOK="$CANARY_TOKEN-ref"; echo drift >"$V/mode.p.CAT-001"
out=$(drv "$V" --max-attempts 1 2>&1); rc=$?
assert_eq "VA1 first push of three items: exit 0, 3 calls" "$rc/$(calls "$V")" "0/3"
printf '%s' "$out" | grep -q 'drift 1 item' && ok "VA2 an adapter remote_state that disagrees with the register status is reported as drift (D3)" || bad "VA2 [$out]"
echo ok >"$V/mode.p.CAT-001"
chg() { step "$V" "$1" "$2" || { RCV=99; return; }; out=$(drv "$V" --max-attempts 1 2>&1); RCV=$?; }
chg VB1pre "$WI_IN update --id CAT-001 --db /src/$DB --description 'A changed description of the first item, long enough to satisfy the description floor of the register.'"; rc=$RCV
assert_eq "VB1 a changed DESCRIPTION is pushed (an update of CAT-001 only): exit 0, 4 calls" "$rc/$(calls "$V")" "0/4"
chg VB2pre "$WI_IN update --id CAT-001 --db /src/$DB --status 'In progress'"; rc=$RCV
assert_eq "VB2 a changed STATUS is pushed: 5 calls" "$rc/$(calls "$V")" "0/5"
chg VB3pre "$WI_IN update --id CAT-001 --db /src/$DB --severity Medium"; rc=$RCV
assert_eq "VB3 a changed SEVERITY is pushed: 6 calls" "$rc/$(calls "$V")" "0/6"
refs=$(grep '^CAT-001 ' "$V/adapter.log" | sed 's/^[^ ]* tracker=p ref=\([^ ]*\) .*/\1/' | tr '\n' ' ')
outs=$(pq "$V" "select group_concat(remote_ref,' ') from (select remote_ref from reg_tracker_sync_log where atm_id='CAT-001' and status='SYNCED' order by sync_id)")
o1=$(echo $outs | cut -d' ' -f1); o2=$(echo $outs | cut -d' ' -f2); o3=$(echo $outs | cut -d' ' -f3)
assert_eq "VB4 every re-push is an UPDATE that carries the LATEST reference: the references handed in are none, then the 1st, 2nd and 3rd returned" "$refs" "none $o1 $o2 $o3 "
grep -q '\$R2_TOK' "$V/adapter.log" && ! grep -qF "$CANARY_TOKEN-ref" "$V/adapter.log" "$V/adapter.stdin.last" 2>/dev/null && ok "VC1 a \$NAME reference in the command reaches the adapter argv as the literal text; the value of the variable (set in the driver environment) does not" || bad "VC1 [$(head -c 300 "$V/adapter.log")]"
hits=0; for needle in r2uniqueword "$CANARY_TOKEN-ref"; do
  grep -rqF "$needle" "$V/.audit" && hits=$((hits+1))
  python3 -I - "$V/$DB" "$needle" <<'PY' && hits=$((hits+1))
import sqlite3,sys,urllib.parse
c=sqlite3.connect("file:"+urllib.parse.quote(sys.argv[1])+"?mode=ro",uri=True)
sys.exit(0 if any(sys.argv[2] in l for l in c.iterdump()) else 1)
PY
done
cr=$(pq "$V" "select command_ref from reg_trackers where tracker_id='p'"); ctlh=0; grep -rqF 'bash#sha256:' "$V/.audit" && ctlh=1
[ "$hits" = 0 ] && printf '%s' "$cr" | grep -Eq '^bash#sha256:[0-9a-f]{16}$' && [ "$ctlh" = 1 ] && ok "VD1 the command text is in no reg_ table, batch file, journal row or journal input; reg_trackers.command_ref is 'bash#sha256:<16 hex>' (control: the scan finds the hash form in the batch files)" || bad "VD1 hits=$hits command_ref=[$cr] control=$ctlh"
unset R2_TOK
fi

if sect boundary; then
echo "== a tracker's rows are recorded before the next tracker starts (X17) =="
T=$(newp T) || { bad "setup T"; finish; exit 1; }
ycfg "$T" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
first_sync_approvers: [owner]
first_sync_pilot_max: 1000
trackers:
  - {name: p, command: "bash $T/adapter.sh {id}", required_env: [R2_TOKEN]}
  - {name: q, command: "bash $T/adapter.sh {id}", required_env: [R2_TOKEN]}
EOF
echo probe >"$T/mode.q"
out=$(drv "$T" --batch-size 100 --max-attempts 1 2>&1); rc=$?
assert_eq "XA1 when tracker q starts, the three SYNCED rows of tracker p are already in the register (it read 3 each time)" "$rc/$(tr '\n' ',' <"$T/q.sawp" 2>/dev/null)" "0/3,3,3,"
fi

if sect backoff; then
echo "== bounded exponential backoff (X01) =="
K=$(newp K) || { bad "setup K"; finish; exit 1; }
echo failrej >"$K/mode.p"
out=$(drv "$K" --limit 1 --max-attempts 3 --backoff-base 1 --breaker 50 2>&1); rc=$?
gap=$(grep '^CAT-001 ' "$K/adapter.log" | sed 's/.* ts=\([0-9.]*\) .*/\1/' | python3 -I -c 'import sys;t=[float(x) for x in sys.stdin.read().split()];print("%.1f" % (t[-1]-t[0]) if len(t)==3 else "bad")')
python3 -I -c 'import sys;g=sys.argv[1];sys.exit(0 if g!="bad" and 2.8<=float(g)<=8 else 1)' "$gap" && ok "XB1 three attempts of one item are spaced by 1 s and 2 s of backoff: first-to-last call gap $gap s (rc $rc)" || bad "XB1 gap=$gap rc=$rc [$out]"
fi

if sect owed; then
echo "== 11.4.214: a recurrence on a CLOSED minted item is not silent (C4, X10) =="
C=$(newp C) || { bad "setup C"; finish; exit 1; }
cat >"$C/.helix/reporting.yaml" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
trackers:
  - {name: gone}
EOF
out=$(drv "$C" 2>&1); rc=$?
mid=$(pq "$C" "select atm_id from reg_ids where minted_by='sync_trackers:gone:not_configured'")
assert_eq "OWa control: before any closure the run exits 0 and reports no owed reopen" "$rc/$(printf '%s' "$out" | grep -c OWED)" "0/0"
step "$C" OWb "sqlite3 /src/$DB \"SELECT 'DROP TRIGGER '||name||';' FROM sqlite_master WHERE type='trigger' AND sql LIKE '%RAISE%' AND name NOT LIKE 'reg_ids%'\" | sqlite3 /src/$DB && sqlite3 /src/$DB \"PRAGMA foreign_keys=ON; UPDATE items SET status='Fixed (→ Fixed.md)' WHERE atm_id='$mid'\"" && assert_eq "OWb the fixture step took effect: the minted item is terminal" "$(pq "$C" "select status from items where atm_id='$mid'")" "Fixed (→ Fixed.md)"
out=$(drv "$C" --json .audit/owed.json 2>&1); rc=$?
assert_eq "OWc the tracker is skipped again while its item is Fixed: the run exits 7, prints OWED reopen of the item, lists it in the --json evidence, and mints no twin" "$rc/$(printf '%s' "$out" | grep -c "OWED reopen of $mid")/$(python3 -I -c 'import json,sys;print(json.load(open(sys.argv[1]))["owed_reopen"][0]["atm_id"])' "$C/.audit/owed.json")/$(pq "$C" "select count(*) from reg_ids where minted_by like 'sync_trackers:gone:%'")" "7/1/$mid/1"
fi

if sect misc; then
echo "== the read-only open (X11), a failed mint (F7g, F7h) =="
M=$(newp M) || { bad "setup M"; finish; exit 1; }
cp -a "$M" "$T_SCR/M_wal"
python3 -I - "$T_SCR/M_wal/$DB" <<'PY'
import sqlite3,sys,os,signal
c=sqlite3.connect(sys.argv[1]); c.execute("PRAGMA journal_mode=WAL"); c.execute("PRAGMA wal_autocheckpoint=0")
c.execute("INSERT INTO reg_trackers(tracker_id,kind) VALUES ('zz_wal','k')"); c.commit()
os.kill(os.getpid(), signal.SIGKILL)   # die with a committed, uncheckpointed WAL
PY
walsz=$(stat -c %s "$T_SCR/M_wal/$DB-wal" 2>/dev/null || echo 0); shaM=$(fsha "$T_SCR/M_wal/$DB")
cp -a "$T_SCR/M_wal" "$T_SCR/M_ctl"
python3 -I -c 'import sqlite3,sys;c=sqlite3.connect(sys.argv[1]);c.execute("select count(*) from reg_trackers").fetchone();c.close()' "$T_SCR/M_ctl/$DB"
[ "$walsz" -gt 0 ] && [ "$(fsha "$T_SCR/M_ctl/$DB")" != "$shaM" ] && ok "XC0 control: with a non-empty WAL, a read-write open and close DOES change the database file (the wal was $walsz bytes), so the check below can see a writing open" || bad "XC0 control: wal=$walsz same=$([ "$(fsha "$T_SCR/M_ctl/$DB")" = "$shaM" ] && echo yes)"
out=$(tool "$T_SCR/M_wal" "$SYNC" --dry-run 2>&1); rc=$?
assert_eq "XC1 the driver opens the register read-only: after a dry run and a real read the database FILE is byte-identical (a read-write open would checkpoint the WAL into it)" "$rc/$([ "$(fsha "$T_SCR/M_wal/$DB")" = "$shaM" ] && echo same || echo changed)" "0/same"
M2=$(newp M2) || { bad "setup M2"; finish; exit 1; }
ycfg "$M2" <<EOF
schema_version: 1
db: docs/workable_items.db
id_prefix: CAT
first_sync_approvers: [owner]
first_sync_pilot_max: 1000
trackers:
  - {name: p, command: "bash $M2/adapter.sh {id}", required_env: [R2_TOKEN]}
  - {name: gone}
EOF
FS=$(pflags "$M2"); out=$( (LOCKED="$STUB" STUB_MODE=mintfail tool "$M2" "$SYNC" $FS 2>&1) ); rc=$?; printf '%s\n' "$out" >>"$LEAK"
printf '%s' "$out" | grep -q 'locked: boom-mint' && ok "XD1 a failed mint prints the reason the wrapper gave (F7g), not only its exit status" || bad "XD1 [$out]"
assert_eq "XD2 a failed mint for one tracker does not stop the sync of the others (F7h): tracker p's three items are SYNCED, exit 4" "$rc/$(pq "$M2" "select count(*) from reg_tracker_sync_log where tracker_id='p' and status='SYNCED'")" "4/3"
fi
finish
