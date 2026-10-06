#!/usr/bin/env bash
# test_locked.sh - T064 (docs/04 section 12.2): scripts/register/locked.sh, the single-writer wrapper. RED while the wrapper is absent.
# Env LOCKED substitutes a mutant copy (mutate_locked.sh). Container leg: see clib.sh (logging podman shim, real RUNP and image).
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead T064
J() { python3 -I -c 'import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
print(len(rows)) if sys.argv[2]=="n" else print(json.dumps(rows[int(sys.argv[2])].get(sys.argv[3])))' "$@"; }
if [ ! -f "$LOCKED" ]; then bad "T0 wrapper absent: $LOCKED"; finish; exit 1; fi
R=$(mkroot a); R2=$(mkroot b)

echo "== usage and argument handling =="
out=$(lk "$R" 2>&1); rc=$?; [ $rc -eq 2 ] && printf '%s' "$out" | grep -qi usage && ok "L1 no arguments: exit 2 and a usage text" || bad "L1 rc=$rc [$out]"
out=$(lk "$R" --bogus -- true 2>&1); rc=$?; [ $rc -eq 2 ] && ok "L2 unknown option exit 2" || bad "L2 rc=$rc"
out=$(env LOCKED_ROOT="$R" bash "$LOCKED" -- true 2>&1); rc=$?; [ $rc -eq 20 ] && printf '%s' "$out" | grep -q test_hook_outside_test_mode && ok "L3 LOCKED_ROOT without LOCKED_TEST_MODE=1 is refused (a hook cannot redirect the register)" || bad "L3 rc=$rc [$out]"

echo "== composed call, lock file, pass-through =="
: >"$SHIM_LOG"
out=$(lk "$R" -- sh -c 'echo hello; exit 7' 2>&1); rc=$?
assert_eq "L4 the wrapped command's exit status is passed through" "$rc" 7
printf '%s' "$out" | grep -q '^hello' && ok "L5 the wrapped command's stdout is passed through" || bad "L5 [$out]"
run=$(grep -E '^run ' "$SHIM_LOG" | head -1)
case "$run" in *"-v $R/docs:/src/docs:rw"*"localhost/catalogizer-testutil@sha256:"*"sh -c echo hello; exit 7"*) ok "L6 composed call is RUNP --rw docs IMG-TESTUTIL -- <cmd> (docs bound rw, the pinned image, the command)";; *) bad "L6 [$run]";; esac
case "$run" in *.audit/scratch*) bad "L6b scratch bind present in a register write: $run";; *) ok "L6b no scratch bind in a register write";; esac
[ -f "$R/docs/.register.lock" ] && ok "L7 the lock file is docs/.register.lock" || bad "L7 lock file absent"
git -C "$ROOT" check-ignore -q docs/.register.lock && ok "L8 docs/.register.lock is ignored by git (never tracked, T004)" || bad "L8 not ignored"

echo "== two concurrent mints never overlap =="
cdb "$R" w.db || { bad "setup"; finish; exit 1; }
rm -f "$R/docs/ov.log"
SLP='echo "s $(date +%s%N)" >> /src/docs/ov.log; '"$(mint_cmd w.db)"'; rc=$?; sleep 1; echo "e $(date +%s%N)" >> /src/docs/ov.log; exit $rc'
( lk "$R" -- sh -c "$SLP" >"$T_SCR/m1.out" 2>&1; echo $? >"$T_SCR/m1.rc" ) &
( lk "$R" -- sh -c "$SLP" >"$T_SCR/m2.out" 2>&1; echo $? >"$T_SCR/m2.rc" ) &
wait
assert_eq "L9 both concurrent mints succeed" "$(cat "$T_SCR/m1.rc")$(cat "$T_SCR/m2.rc")" "00"
i1=$(tail -1 "$T_SCR/m1.out"); i2=$(tail -1 "$T_SCR/m2.out")
[ -n "$i1" ] && [ -n "$i2" ] && [ "$i1" != "$i2" ] && ok "L10 distinct ids minted ($i1, $i2)" || bad "L10 [$i1] [$i2]"
cat "$T_SCR/m1.out" "$T_SCR/m2.out" | grep -qi 'database is locked' && bad "L11 'database is locked' seen" || ok "L11 no 'database is locked' error"
python3 -I - "$R/docs/ov.log" <<'PY' && ok "L12 the two intervals do not overlap (the lock serialises the writers)" || bad "L12 intervals overlap: $(cat "$R/docs/ov.log" | tr '\n' ' ')"
import sys
ev=[l.split() for l in open(sys.argv[1]) if l.strip()]
assert len(ev)==4, ev
# events appear in time order in the shared log; a serialised run is s e s e, an overlapping one s s e e
assert [e[0] for e in ev]==['s','e','s','e'], [e[0] for e in ev]
PY

echo "== journal and input capture =="
rm -rf "$R/.audit/register"; : >"$SHIM_LOG"
printf 'select 1;\n' >"$R/.audit/in1.sql"; printf 'select 2;\n' >"$R/docs/in2.sql"
lk "$R" -- sh -c 'echo a' >/dev/null 2>&1; lk "$R" -- sh -c 'cat "$0" "$1" >/dev/null' .audit/in1.sql /src/docs/in2.sql nothere.sql >/dev/null 2>&1
JR="$R/.audit/register/journal.jsonl"
assert_eq "L13 two commands give two journal rows" "$(J "$JR" n 2>/dev/null)" 2
for f in time op_id argv exit db_sha_before db_sha_after ids_minted; do python3 -I -c 'import json,sys;r=json.loads(open(sys.argv[1]).read().splitlines()[1]);sys.exit(0 if sys.argv[2] in r else 1)' "$JR" $f && ok "L14 journal row has field $f" || bad "L14 field $f missing"; done
s1=$(fsha "$R/.audit/in1.sql"); s2=$(fsha "$R/docs/in2.sql")
[ -f "$R/.audit/register/inputs/$s1" ] && [ "$(fsha "$R/.audit/register/inputs/$s1")" = "$s1" ] && ok "L15 a repository-relative file argument is copied to inputs/<sha256> (hash equals name)" || bad "L15"
[ -f "$R/.audit/register/inputs/$s2" ] && [ "$(fsha "$R/.audit/register/inputs/$s2")" = "$s2" ] && ok "L16 a /src/<p> container-path argument is copied the same way" || bad "L16"
assert_eq "L17 an argument naming no file copies nothing (exactly two input files)" "$(ls "$R/.audit/register/inputs" | wc -l)" 2
python3 -I -c 'import json,sys;r=json.loads(open(sys.argv[1]).read().splitlines()[1]);sys.exit(0 if r["argv"][0]=="sh" and r["exit"]==0 else 1)' "$JR" && ok "L18 the row records the inner argv and the exit status" || bad "L18"

echo "== --out writes only there =="
before=$(fsha "$R/docs/w.db"); mkdir -p "$T_SCR/o1"; : >"$SHIM_LOG"
lk "$R" --out "$T_SCR/o1" -- sh -c 'echo x > /out/f; touch /src/docs/should-not-exist 2>/dev/null; echo done' >/dev/null 2>&1
[ -f "$T_SCR/o1/f" ] && ok "L19 --out directory is /out" || bad "L19"
[ ! -e "$R/docs/should-not-exist" ] && ok "L20 a command run with --out cannot write docs/" || bad "L20 docs written"
grep -E '^run ' "$SHIM_LOG" | head -1 | grep -q ":/src/docs:rw" && bad "L21 --out run mounted docs rw" || ok "L21 no --rw docs for a --out run"
assert_eq "L22 register DB unchanged by the --out run" "$(fsha "$R/docs/w.db")" "$before"

echo "== stdin is unsupported =="
before=$(fsha "$R/docs/w.db"); n0=$("$SQLITE3" "$R/docs/w.db" 'select count(*) from reg_ids' 2>/dev/null)
echo "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('stdin','manual');" | lk "$R" -- sqlite3 /src/docs/w.db >/dev/null 2>&1
assert_eq "L23 SQL piped on stdin writes no row and leaves the DB sha256 unchanged" "$(fsha "$R/docs/w.db")" "$before"
lk "$R" -h 2>&1 | grep -qi 'stdin' && ok "L24 usage text states that stdin is unsupported" || bad "L24"

echo "== commit-turn freeze =="
CT="$R/.audit/commit_turn.json"; STUB="$T_SCR/cpa-host"; STUBLOG="$T_SCR/stub.log"
cat >"$STUB" <<'SH'
#!/bin/sh
echo "$*" >>"$STUBLOG"
[ "$1" = "--exec-approved" ] && [ "$2" = "scripts/release/commit_turn_check.sh" ] || exit 9
# stub reaper: removes the grant only when its holder pid is dead
pid=$(sed -n 's/.*"pid": *\([0-9]*\).*/\1/p' "$GRANT")
if ! kill -0 "$pid" 2>/dev/null; then rm -f "$GRANT"; fi
exit 0
SH
chmod +x "$STUB"
sleep 300 & HOLD=$!
echo "{\"run_id\":\"OTHER\",\"pid\":$HOLD,\"cmdline\":\"sleep 300\",\"started_at\":\"2025-01-01T00:00:00Z\"}" >"$CT"
before=$(fsha "$R/docs/w.db"); : >"$SHIM_LOG"; : >"$STUBLOG"
out=$(STUBLOG="$STUBLOG" GRANT="$CT" CPA_HOST_ENTRY="$STUB" lk "$R" -- sh -c "$(mint_cmd w.db)" 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'reason=commit_turn_held' && ok "L25 a grant naming another run (live holder) refuses with commit_turn_held" || bad "L25 rc=$rc [$out]"
assert_eq "L26 docs/ byte-unchanged and no podman run call during the refusal" "$(fsha "$R/docs/w.db")$(grep -cE '^run ' "$SHIM_LOG")" "${before}0"
assert_eq "L27 the reaper ran once, only through --exec-approved" "$(wc -l <"$STUBLOG")$(grep -c '^--exec-approved scripts/release/commit_turn_check.sh --reap$' "$STUBLOG")" "11"
kill "$HOLD" 2>/dev/null; wait "$HOLD" 2>/dev/null
echo "{\"run_id\":\"OTHER\",\"pid\":$HOLD,\"cmdline\":\"sleep 300\",\"started_at\":\"2025-01-01T00:00:00Z\"}" >"$CT"; : >"$STUBLOG"
out=$(STUBLOG="$STUBLOG" GRANT="$CT" CPA_HOST_ENTRY="$STUB" lk "$R" -- sh -c "$(mint_cmd w.db)" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "L28 a grant whose holder pid has exited is reaped by the one --reap call and the mint then succeeds" || bad "L28 rc=$rc [$out]"
echo '{"run_id":"OTHER","pid":1,"started_at":"x"}' >"$CT"
out=$(EVREC_TURN_RUN_ID=OTHER lk "$R" -- sh -c "$(mint_cmd w.db)" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "L29 EVREC_TURN_RUN_ID equal to the grant's run_id lets the mint succeed (golden-true)" || bad "L29 rc=$rc [$out]"
rm -f "$CT"; out=$(lk "$R" -- sh -c "$(mint_cmd w.db)" 2>&1); [ $? -eq 0 ] && ok "L30 no grant: the mint succeeds (golden-true)" || bad "L30 [$out]"
echo 'not json' >"$CT"; before=$(fsha "$R/docs/w.db")
out=$(CPA_HOST_ENTRY=/nonexistent lk "$R" -- sh -c "$(mint_cmd w.db)" 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q commit_turn_held && [ "$(fsha "$R/docs/w.db")" = "$before" ] && ok "L31 a grant body that is not JSON is refused commit_turn_held (an absent reaper counts as failing)" || bad "L31 rc=$rc [$out]"
echo '{"run_id":"OTHER","pid":1}' >"$CT"; chmod 000 "$CT"
out=$(CPA_HOST_ENTRY=/nonexistent lk "$R" -- sh -c "$(mint_cmd w.db)" 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q commit_turn_held && [ "$(fsha "$R/docs/w.db")" = "$before" ] && ok "L32 an unreadable grant file (mode 000) is refused commit_turn_held" || bad "L32 rc=$rc [$out]"
chmod 600 "$CT"; rm -f "$CT"

echo "== scratch import and import-sql =="
mkdir -p "$R/.audit/scratch" "$R/.audit/out/op1"
out=$(LOCKED_SCRATCH_DB="$R/.audit/scratch/s.db" lk "$R" -- true 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q scratch_db_subcommand_invalid && ok "L33 LOCKED_SCRATCH_DB with any other command is refused scratch_db_subcommand_invalid (20)" || bad "L33 rc=$rc [$out]"
SQL="$R/.audit/out/op1/x.sql"; printf 'CREATE TABLE zz(a);\nINSERT INTO zz VALUES (1);\n' >"$SQL"; sha256sum <"$SQL" | cut -d' ' -f1 >"$R/.audit/out/op1/x.sha256"
out=$(LOCKED_SCRATCH_DB="$R/.audit/scratch/s.db" lk "$R" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
v=$("$SQLITE3" "$R/.audit/scratch/s.db" 'select a from zz' 2>/dev/null)
[ $rc -eq 0 ] && [ "$v" = 1 ] && ok "L34 a scratch import writes the scratch database" || bad "L34 rc=$rc v=[$v] [$out]"
echo "$(printf 'x' | sha256sum | cut -d' ' -f1)" >"$R/.audit/out/op1/x.sha256"
before=$(fsha "$R/.audit/scratch/s.db"); out=$(LOCKED_SCRATCH_DB="$R/.audit/scratch/s.db" lk "$R" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q import_sha256_mismatch && ok "L35 an import whose sha256 differs from its sibling is refused import_sha256_mismatch" || bad "L35 rc=$rc [$out]"
echo "$(sha256sum <"$SQL" | cut -d' ' -f1)  x.sql" >"$R/.audit/out/op1/x.sha256"
out=$(LOCKED_SCRATCH_DB="$R/.audit/scratch/s.db" lk "$R" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q import_sha256_malformed && ok "L36 a sibling that is not one bare hash is refused import_sha256_malformed" || bad "L36 rc=$rc [$out]"
sha256sum <"$SQL" | cut -d' ' -f1 >"$R/.audit/out/op1/x.sha256"
out=$(LOCKED_SCRATCH_DB="$R/.audit/scratch/../out/s.db" lk "$R" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q scratch_db_path_invalid && ok "L37 a scratch path that leaves .audit/scratch (after '..' collapse) is refused scratch_db_path_invalid" || bad "L37 rc=$rc [$out]"
# scratch import composes --rw .audit/scratch and never --rw docs; docs/ keeps its hash
dsha=$(fsha "$R/docs/w.db"); : >"$SHIM_LOG"
LOCKED_SCRATCH_DB="$R/.audit/scratch/s2.db" lk "$R" import-sql .audit/out/op1/x.sql >/dev/null 2>&1
run=$(grep -E '^run ' "$SHIM_LOG" | head -1)
case "$run" in *"$R/.audit/scratch:/src/.audit/scratch:rw"*) ok "L38 a scratch import composes --rw .audit/scratch";; *) bad "L38 [$run]";; esac
case "$run" in *"$R/docs:/src/docs:rw"*) bad "L39 scratch import mounted docs rw";; *) ok "L39 a scratch import never mounts docs rw";; esac
assert_eq "L40 docs/workable_items-style register DB keeps its sha256 during a scratch import" "$(fsha "$R/docs/w.db")" "$dsha"
# symlinked .audit/scratch is refused by RUNP and the reason passed through, no podman run call
R3=$(mkroot c); rm -rf "$R3/.audit/scratch"; ln -s "$R3/docs" "$R3/.audit/scratch"; mkdir -p "$R3/.audit/out/op1"; cp "$SQL" "$R3/.audit/out/op1/"; cp "$R/.audit/out/op1/x.sha256" "$R3/.audit/out/op1/"
: >"$SHIM_LOG"; dsha=$(find "$R3/docs" -type f -not -name .register.lock | sort | xargs -r sha256sum | sha256sum)
out=$(LOCKED_SCRATCH_DB="$R3/.audit/scratch/s.db" lk "$R3" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -qE 'rw_source_not_canonical|scratch_db_path_invalid' && ok "L41 .audit/scratch as a symlink to docs is refused (no write into docs)" || bad "L41 rc=$rc [$out]"
assert_eq "L42 no podman run call and docs/ unchanged for the symlinked scratch" "$(grep -cE '^run ' "$SHIM_LOG")$(find "$R3/docs" -type f -not -name .register.lock | sort | xargs -r sha256sum | sha256sum)" "0$dsha"

echo "== register import-sql calls the pre-op backup helper once; a scratch import never does =="
BK="$T_SCR/bk.sh"; printf '#!/bin/sh\necho "$@" >> "%s"\nexit 0\n' "$T_SCR/bk.log" >"$BK"; chmod +x "$BK"; : >"$T_SCR/bk.log"
cdb "$R" workable_items.db
LK_BACKUP="$BK" lk "$R" import-sql .audit/out/op1/x.sql >/dev/null 2>&1; rc=$?
assert_eq "L43 a register import succeeds and calls the backup helper exactly once" "$rc $(wc -l <"$T_SCR/bk.log")" "0 1"
grep -q -- '--record' "$T_SCR/bk.log" && ok "L44 the backup helper is called with --record" || bad "L44 [$(cat "$T_SCR/bk.log")]"
[ "$("$SQLITE3" "$R/docs/workable_items.db" 'select a from zz' 2>/dev/null)" = 1 ] && ok "L45 the register copy received the imported rows" || bad "L45"
: >"$T_SCR/bk.log"; LK_BACKUP="$BK" LOCKED_SCRATCH_DB="$R/.audit/scratch/s3.db" lk "$R" import-sql .audit/out/op1/x.sql >/dev/null 2>&1
assert_eq "L46 a scratch import never calls the backup helper" "$(wc -l <"$T_SCR/bk.log")" 0
# a held commit-turn grant: a scratch import still succeeds (no docs write, no backup), a register import is refused
echo '{"run_id":"OTHER","pid":1,"started_at":"x"}' >"$R/.audit/commit_turn.json"; : >"$T_SCR/bk.log"; dsha=$(find "$R/docs" -type f | sort | xargs -r sha256sum | sha256sum)
o47=$(LK_BACKUP="$BK" LOCKED_SCRATCH_DB="$R/.audit/scratch/s4.db" CPA_HOST_ENTRY=/nonexistent lk "$R" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
[ $rc -eq 0 ] || echo "# L47 diagnostic (rc=$rc): $(printf '%s' "$o47" | tr '\n' ' ' | cut -c1-400)"
assert_eq "L47 a scratch import under a held grant exits 0, empty backup log, docs/ unchanged, scratch DB written" "$rc $(wc -l <"$T_SCR/bk.log") $(find "$R/docs" -type f | sort | xargs -r sha256sum | sha256sum | cut -c1-8) $([ -s "$R/.audit/scratch/s4.db" ] && echo w)" "0 0 ${dsha:0:8} w"
out=$(LK_BACKUP="$BK" CPA_HOST_ENTRY=/nonexistent lk "$R" import-sql .audit/out/op1/x.sql 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q commit_turn_held && ok "L48 a register import under the same grant is refused commit_turn_held" || bad "L48 rc=$rc [$out]"
rm -f "$R/.audit/commit_turn.json"

echo "== DISK_HEADROOM_OUT_DIR: set for a scratch import, never for a register write =="
SPY="$T_SCR/runp_spy.sh"; printf '#!/bin/sh\necho "HR=${DISK_HEADROOM_OUT_DIR-UNSET} ARGS=$*" >> "%s"\nexec "%s" "$@"\n' "$T_SCR/spy.log" "$RUNP" >"$SPY"; chmod +x "$SPY"; : >"$T_SCR/spy.log"
LK_RUNP="$SPY" LOCKED_SCRATCH_DB="$R/.audit/scratch/s5.db" lk "$R" --op-id spy1 import-sql .audit/out/op1/x.sql >/dev/null 2>&1
grep -- '--rw .audit/scratch' "$T_SCR/spy.log" | grep -q '^HR=.audit/out/spy1/disk/ ' && ok "L49a a scratch import sets DISK_HEADROOM_OUT_DIR=.audit/out/<op_id>/disk/ for its RUNP call" || bad "L49a [$(tr '\n' '|' <"$T_SCR/spy.log")]"
: >"$T_SCR/spy.log"; LK_RUNP="$SPY" lk "$R" -- true >/dev/null 2>&1
grep -- '--rw docs' "$T_SCR/spy.log" | grep -q '^HR=UNSET ' && ok "L49b the RUNP call of a register write leaves DISK_HEADROOM_OUT_DIR unset" || bad "L49b [$(tr '\n' '|' <"$T_SCR/spy.log")]"

echo "== the real leg: binaries recorded =="
if [ -n "${EV:-}" ]; then
  bj=$(lk "$R" --out "$T_SCR/o2" -- sh -c "sqlite3 --version | cut -d' ' -f1,2 | sed 's/^/sqlite3 /'; $WI_IN validate --db /out/probe.db | head -1; sha256sum /usr/bin/sqlite3 $WI_IN" 2>&1)
  python3 -I - "$EV/register-binaries.json" "$bj" "$(grep -A4 '^- id: IMG-TESTUTIL$' "$ROOT/build/containers/images.lock.yaml" | grep -o 'sha256:[0-9a-f]*' | head -1)" <<'PY'
import json,sys,re
txt=sys.argv[2]; sq=re.search(r'sqlite3 (\S+ \S+)',txt); hs=re.findall(r'^([0-9a-f]{64})  (\S+)$',txt,re.M)
json.dump({"image":"IMG-TESTUTIL","image_digest":sys.argv[3],"sqlite3_version":sq.group(1) if sq else None,"engine_validate_probe":"validate: OK" in txt,
 "sha256":{p:h for h,p in hs}},open(sys.argv[1],"w"),indent=1,sort_keys=True)
PY
  [ -s "$EV/register-binaries.json" ] && grep -q '"engine_validate_probe": true' "$EV/register-binaries.json" && ok "L50 register-binaries.json written (sqlite3 version, engine validate probe, both sha256)" || bad "L50"
else ok "L50 (EV unset: register-binaries.json not written in this run)"; fi
finish
