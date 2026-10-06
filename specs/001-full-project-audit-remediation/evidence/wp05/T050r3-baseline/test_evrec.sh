#!/usr/bin/env bash
# T049 - failing-first test of tools/evidence/evrec and tools/evidence/verify (docs/06 s13.3, s13.7, s16).
# Written BEFORE the recorder exists: every check below is RED while tools/evidence/evrec is absent.
#
# Interface this test assumes (ALL UNCONFIRMED until T050 implements it; T050 may change a name and then
# MUST change this test in the same commit, with the RED/GREEN record):
#   env   EV_LEDGER EV_ANCHOR EV_BLOBS   scratch ledger, anchor, blob dir (as in the docs/06 s13 POC)
#         EVREC_REPO_ROOT               repo root used for .audit/commit_turn.json and scripts/release/*
#         CPA_HOST_ENTRY                host entry point used for --exec-approved calls
#         EVREC_TURN_RUN_ID             caller run id for the commit-turn grant
#         EVREC_LOCK_TIMEOUT            seconds to wait for a live ledger-lock holder
#         EVREC_REDACT_VARS             names of env vars whose values are redacted from captured streams
#         EVREC_FAULT=kill_before_rename   test hook: the process is SIGKILLed after the temp write
#         EV_LEDGER_BOUND               byte bound used by `verify` for the 75% ledger_raise_owed check
#   cmds  evrec run ITEM POLARITY ITER CLASS REF -- argv...      (appends one ev/1 entry)
#         evrec check-record FILE                                (exit 0 valid, non-zero refused)
#         evrec reconcile --runner-count N                       (exit 0 iff N == entries written)
#         evrec rerecord --onto F --local F --out DIR | --append-only ...
#         evrec remap-refs --map FILE --files-from LIST
#         verify                                                 (chain walk of EV_LEDGER, exit 0/1)
# Anchor rows of the 13.3 table move to T055. Host bash/jq/python3 are used because RUNP IMG-KCOV
# (scripts/containers/run_pinned.sh) does not exist yet; rerun under RUNP when WP-09 lands (UNCONFIRMED).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify
SCHEMA=$root/specs/001-full-project-audit-remediation/contracts/evidence-record.schema.json
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
# anti-vacuity: with the recorder or verifier absent, an expected-refusal check would pass by exit 127;
# no check may pass then (a PASS without the tool under test is the bluff this task exists to prevent).
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
t()   { if "${@:2}" >/dev/null 2>&1; then ok "$1"; else bad "$1"; fi; }          # t NAME cmd...
tn()  { "${@:2}" >/dev/null 2>&1; local r=$?; if [ $r -eq 0 ] || [ $r -eq 126 ] || [ $r -eq 127 ]; then bad "$1 (rc=$r)"; else ok "$1"; fi; }          # expected non-zero
rc()  { "$@" >/dev/null 2>&1; echo $?; }
fresh() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchor.jsonl EV_BLOBS=$S/w/blobs; }
rec() { "$EVREC" run "${1:-CAT-001}" "${2:-PROBE}" "${3:-1}" shell_script x -- "${@:4}"; }
[ -x "$EVREC" ] || echo "NOTE: $EVREC is absent or not executable (expected RED before T050)"

# --- 13.3 chain rows: delete without repair exit 1, original exit 0
fresh; for i in 1 2 3 4; do rec CAT-001 PROBE "$i" true >/dev/null 2>&1; done
cp "$EV_LEDGER" "$S/orig.jsonl"
t   "verify: original 4-entry ledger exits 0"                 "$VERIFY"
sed '2d' "$S/orig.jsonl" >"$EV_LEDGER"
r=$(rc "$VERIFY"); [ "$r" = 1 ] && ok "verify: entry 2 deleted without repair exits 1" || bad "verify: delete w/o repair exit $r (want 1)"
cp "$S/orig.jsonl" "$EV_LEDGER"
t   "verify: original restored exits 0"                       "$VERIFY"
[ "$(wc -l <"$EV_LEDGER" 2>/dev/null)" = 4 ] && ok "ledger has 4 entries" || bad "ledger entry count != 4"
# every produced record validates against ev/1 (rev 6)
v=$(python3 - "$SCHEMA" "$EV_LEDGER" <<'PY' 2>&1
import json,sys,jsonschema
s=json.load(open(sys.argv[1])); n=0
for l in open(sys.argv[2]):
    jsonschema.Draft202012Validator(s).validate(json.loads(l)); n+=1
print("valid",n)
PY
); [ "$v" = "valid 4" ] && ok "produced entries validate against ev/1 schema" || bad "schema validation: $v"

# --- duration self-test: a 0.3 s command records 250..3000 ms
fresh; rec CAT-001 PROBE 1 sleep 0.3 >/dev/null 2>&1
d=$(jq -r '.duration_ms' "$EV_LEDGER" 2>/dev/null)
case $d in ''|*[!0-9]*) bad "duration self-test: not recorded ($d)";; *) [ "$d" -ge 250 ] && [ "$d" -le 3000 ] && ok "duration self-test ($d ms)" || bad "duration self-test out of range ($d ms)";; esac

# --- five missing-field records refused (start time, cwd, argv, exit status, duration) + argv as string
good=$(jq -c 'del(.prev_hash,.entry_hash)' "$S/orig.jsonl" | head -1)
for f in started_at cwd argv exit_status duration_ms; do
  jq -c "del(.$f)" <<<"$good" >"$S/miss_$f.json"; tn "check-record refuses record missing $f" "$EVREC" check-record "$S/miss_$f.json"; done
jq -c '.argv="true -x"' <<<"$good" >"$S/argv_str.json"; tn "check-record refuses argv as a string" "$EVREC" check-record "$S/argv_str.json"

# --- 13.7 production properties
# (a) two concurrent appenders: no gap, no duplicate
fresh; for w in a b; do ( for i in $(seq 1 15); do rec CAT-00$([ $w = a ] && echo 1 || echo 2) PROBE "$i" true >/dev/null 2>&1; done ) & done; wait
seqs=$(jq -r .seq "$EV_LEDGER" 2>/dev/null | sort -n | tr '\n' ' ')
[ "$seqs" = "$(seq -s ' ' 1 30) " ] && ok "concurrent appenders: seq 1..30, no gap, no duplicate" || bad "concurrent appenders: seqs=[$seqs]"
t   "concurrent appenders: chain still verifies"              "$VERIFY"
# (b) stale lock with dead holder reaped; live holder never stolen (11.4.180)
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1
( exit 0 ) & dead=$!; wait $dead
printf '{"pid":%s,"cmdline":"gone"}\n' "$dead" >"$EV_LEDGER.lock"
t   "stale ledger lock with dead holder is reaped, append succeeds" rec CAT-001 PROBE 2 true
sleep 60 & live=$!; for _ in $(seq 1 200); do [ "$(tr '\0' ' ' </proc/$live/cmdline 2>/dev/null)" = "sleep 60 " ] && break; sleep 0.02; done  # T050: wait for the exec, else the recorded cmdline is the forked parent
cmdl=$(tr '\0' ' ' </proc/$live/cmdline 2>/dev/null)
printf '{"pid":%s,"cmdline":"%s"}\n' "$live" "${cmdl% }" >"$EV_LEDGER.lock"
before=$(sha256sum "$EV_LEDGER" | cut -d' ' -f1)
tn  "live lock holder: append refused, not stolen"            env EVREC_LOCK_TIMEOUT=2 "$EVREC" run CAT-001 PROBE 3 shell_script x -- true
kill -0 "$live" 2>/dev/null && ok "live holder process untouched" || bad "live holder was killed"
[ "$(sha256sum "$EV_LEDGER" | cut -d' ' -f1)" = "$before" ] && ok "ledger unchanged after refused append" || bad "ledger changed under a live lock"
kill "$live" 2>/dev/null; rm -f "$EV_LEDGER.lock"
# (c) temp-then-rename: a killed write leaves the previous ledger intact
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; before=$(sha256sum "$EV_LEDGER" | cut -d' ' -f1)
EVREC_FAULT=kill_before_rename "$EVREC" run CAT-001 PROBE 2 shell_script x -- true >/dev/null 2>&1
[ "$(sha256sum "$EV_LEDGER" | cut -d' ' -f1)" = "$before" ] && ok "killed write leaves previous ledger intact" || bad "ledger changed by a killed write"
t   "ledger still verifies after killed write"                "$VERIFY"
# (d) planted secret redacted before the blob is stored
fresh; sec="evrec-planted-$RANDOM$RANDOM"; export PLANTED_TEST_SECRET=$sec EVREC_REDACT_VARS=PLANTED_TEST_SECRET
"$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c 'echo "token=$PLANTED_TEST_SECRET"' >/dev/null 2>&1
if [ -d "$EV_BLOBS" ] && [ -n "$(ls -A "$EV_BLOBS" 2>/dev/null)" ]; then
  grep -rqF -- "$sec" "$EV_BLOBS" "$EV_LEDGER" 2>/dev/null && bad "planted secret present in stored blob/ledger" || ok "planted secret absent from blobs and ledger"
  [ "$(jq -r '.redacted' "$EV_LEDGER" 2>/dev/null)" = true ] && ok "entry records redacted: true" || bad "entry lacks redacted: true"
else bad "no blob stored (cannot judge redaction)"; bad "entry lacks redacted: true"; fi
unset PLANTED_TEST_SECRET EVREC_REDACT_VARS
# (e) runner command count reconciles with entries written
fresh; for i in 1 2 3; do rec CAT-001 PROBE "$i" true >/dev/null 2>&1; done
t  "reconcile: runner count 3 == 3 entries"                  "$EVREC" reconcile --runner-count 3
tn "reconcile: runner count 4 != 3 entries refused"          "$EVREC" reconcile --runner-count 4

# --- verify names ledger_raise_owed above 75% of the bound and not below
fresh; for i in 1 2 3; do rec CAT-001 PROBE "$i" true >/dev/null 2>&1; done
sz=$(stat -c %s "$EV_LEDGER" 2>/dev/null || echo 0)
hi=$(EV_LEDGER_BOUND=$(( sz * 4 / 3 )) "$VERIFY" 2>&1); lo=$(EV_LEDGER_BOUND=$(( sz * 4 )) "$VERIFY" 2>&1)
grep -q ledger_raise_owed <<<"$hi" && ok "verify names ledger_raise_owed at >75% of bound" || bad "verify silent above 75% of bound"
grep -q ledger_raise_owed <<<"$lo" && bad "verify names ledger_raise_owed below 75%" || ok "verify silent below 75% of bound"

# --- re-record mode (P0-P1 store-conflict remediation)
mk() { EV_LEDGER=$1 "$EVREC" run "$2" PROBE "$3" shell_script x -- true >/dev/null 2>&1; }
fresh; rm -rf "$S/rr"; mkdir -p "$S/rr"
mk "$S/rr/base.jsonl" CAT-001 1; mk "$S/rr/base.jsonl" CAT-001 2
cp "$S/rr/base.jsonl" "$S/rr/remote.jsonl"; cp "$S/rr/base.jsonl" "$S/rr/local.jsonl"
mk "$S/rr/remote.jsonl" CAT-002 1; mk "$S/rr/local.jsonl" CAT-003 1; mk "$S/rr/local.jsonl" CAT-003 2
"$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out" >/dev/null 2>&1
out=$S/rr/out/ledger.jsonl
if [ -f "$out" ]; then
  head -c "$(stat -c %s "$S/rr/remote.jsonl")" "$out" | cmp -s - "$S/rr/remote.jsonl" && ok "rerecord: remote side byte for byte first" || bad "rerecord: remote prefix differs"
  [ "$(jq -r .seq "$out" | tr '\n' ' ')" = "1 2 3 4 5 " ] && ok "rerecord: local entries re-sequenced 4,5" || bad "rerecord: seq values wrong"
  [ "$(jq -r .item "$out" | tail -2 | tr '\n' ' ')" = "CAT-003 CAT-003 " ] && ok "rerecord: local entries keep other fields" || bad "rerecord: local fields changed"
  EV_LEDGER=$out t "rerecord: output accepted by verify" "$VERIFY"
  [ "$(jq -c '[.[]|length]' "$S/rr/out/ledger-seq-map.json" 2>/dev/null)" != "" ] && ok "rerecord: ledger-seq-map.json written" || bad "rerecord: no seq map"
  # mutation: keeping the local seq values makes verify reject
  jq -c 'if .seq>=4 then .seq-=1 else . end' "$out" >"$S/rr/mut.jsonl"; EV_LEDGER=$S/rr/mut.jsonl tn "rerecord mutation (kept local seq) rejected by verify" "$VERIFY"
else for m in "remote prefix" "re-sequenced" "other fields" "verify accepts" "seq map" "mutation"; do bad "rerecord: $m (no output)"; done; fi
# refusals: unverifiable side, no common prefix; nothing written
printf 'garbage\n' >"$S/rr/bad.jsonl"; rm -rf "$S/rr/o2"
tn "rerecord refuses a side verify rejects"                  "$EVREC" rerecord --onto "$S/rr/bad.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/o2"
[ ! -e "$S/rr/o2/ledger.jsonl" ] && ok "rerecord: nothing written on refusal" || bad "rerecord wrote on refusal"
mk "$S/rr/other.jsonl" CAT-009 1; rm -rf "$S/rr/o3"
tn "rerecord refuses sides without a common prefix"          "$EVREC" rerecord --onto "$S/rr/other.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/o3"
# --append-only for deferrals/flake ledgers (no chain fields) and remap-refs
printf '{"k":1}\n{"k":2}\n' >"$S/rr/dr.jsonl"; cp "$S/rr/dr.jsonl" "$S/rr/dl.jsonl"; printf '{"k":3}\n' >>"$S/rr/dr.jsonl"; printf '{"k":4}\n' >>"$S/rr/dl.jsonl"
"$EVREC" rerecord --append-only --onto "$S/rr/dr.jsonl" --local "$S/rr/dl.jsonl" --out "$S/rr/o4" >/dev/null 2>&1
[ "$(jq -r .k "$S/rr/o4/ledger.jsonl" 2>/dev/null | tr '\n' ' ')" = "1 2 3 4 " ] && ok "append-only: remote then local suffix, no chain fields" || bad "append-only rerecord"
printf 'see ledger#4 and ledger#5\n' >"$S/rr/doc.md"; printf '%s\n' "$S/rr/doc.md" >"$S/rr/list"
printf '[{"old":4,"new":5,"digest":"x"},{"old":5,"new":6,"digest":"y"}]\n' >"$S/rr/map.json"
"$EVREC" remap-refs --map "$S/rr/map.json" --files-from "$S/rr/list" >/dev/null 2>&1
grep -q 'ledger#5 and ledger#6' "$S/rr/doc.md" && ok "remap-refs rewrites every ledger#<seq>" || bad "remap-refs result: $(cat "$S/rr/doc.md")"
printf 'uncovered ledger#9\n' >"$S/rr/doc2.md"; printf '%s\n' "$S/rr/doc2.md" >"$S/rr/list2"
tn "remap-refs refuses a reference the map does not cover"   "$EVREC" remap-refs --map "$S/rr/map.json" --files-from "$S/rr/list2"

# --- commit-turn refusal (rev 29/30)
ct() { # ct NAME  -> builds scratch repo with stub release script and host entry
  rm -rf "$S/ct"; mkdir -p "$S/ct/.audit" "$S/ct/scripts/release"; export EVREC_REPO_ROOT=$S/ct
  export EV_LEDGER=$S/ct/ledger.jsonl EV_ANCHOR=$S/ct/anchor.jsonl EV_BLOBS=$S/ct/blobs; : >"$S/ct/calls.log"
  cat >"$S/ct/scripts/release/commit_turn_check.sh" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$CT_LOG"
if [ "${1:-}" = --reap ]; then g=$EVREC_REPO_ROOT/.audit/commit_turn.json; p=$(jq -r .pid "$g"); c=$(jq -r .cmdline "$g")
  if ! kill -0 "$p" 2>/dev/null || [ "$(tr '\0' ' ' </proc/$p/cmdline 2>/dev/null | sed 's/ $//')" != "$c" ]; then rm -f "$g"; fi; fi
STUB
  cat >"$S/ct/host_entry.sh" <<'STUB'
#!/usr/bin/env bash
echo "host $*" >>"$CT_HOST_LOG"
[ -n "${CT_REFUSE:-}" ] && { echo "release_seam_unreleased" >&2; exit 20; }
[ "$1" = --exec-approved ] && { shift; exec bash "$EVREC_REPO_ROOT/$1" "${@:2}"; }
STUB
  chmod +x "$S/ct/scripts/release/commit_turn_check.sh" "$S/ct/host_entry.sh"
  export CPA_HOST_ENTRY=$S/ct/host_entry.sh CT_LOG=$S/ct/calls.log CT_HOST_LOG=$S/ct/host.log; : >"$CT_HOST_LOG"; unset EVREC_TURN_RUN_ID
}
grant() { sleep 120 & holder=$!; for _ in $(seq 1 200); do [ "$(tr '\0' ' ' </proc/$holder/cmdline 2>/dev/null)" = "sleep 120 " ] && break; sleep 0.02; done  # T050: wait for the exec (fixture race under load)
  cm=$(tr '\0' ' ' </proc/$holder/cmdline | sed 's/ $//')
  printf '{"run_id":"other","paths_from_sha256":"%s","pid":%s,"cmdline":"%s","started_at":"%s"}\n' "$(printf x|sha256sum|cut -d' ' -f1)" "$holder" "$cm" "${1:-2026-10-05T00:00:00Z}" >"$S/ct/.audit/commit_turn.json"; }
unchanged() { [ ! -e "$EV_LEDGER" ] || [ ! -s "$EV_LEDGER" ]; }
ct; grant
r=$("$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1 >/dev/null; echo "rc=$?"); grep -q commit_turn_held <<<"$r" && ok "grant held, no run id: refused commit_turn_held" || bad "no run id: $r"
unchanged && [ ! -d "$EV_BLOBS" -o -z "$(ls -A "$EV_BLOBS" 2>/dev/null)" ] && ok "stores byte-unchanged after refusal" || bad "stores changed under a held grant"
EVREC_TURN_RUN_ID=wrong "$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1 | grep -q commit_turn_held && ok "different run id: refused" || bad "different run id not refused"
: >"$CT_LOG"   # T050: the log is cumulative; the two refusals above legitimately called the reaper, so reset it before the matching-run-id call
EVREC_TURN_RUN_ID=other t "matching run id: append succeeds, stub never called" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
[ ! -s "$CT_LOG" ] && ok "matching run id never calls the stub" || bad "stub called for matching run id"
kill $holder 2>/dev/null
ct; t "no grant file: append succeeds" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
ct; grant; chmod 000 "$S/ct/.audit/commit_turn.json"
"$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1 | grep -q commit_turn_held && ok "unreadable grant (mode 000): commit_turn_held" || bad "mode 000 grant not refused"
chmod 600 "$S/ct/.audit/commit_turn.json"; echo 'not json' >"$S/ct/.audit/commit_turn.json"
"$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1 | grep -q commit_turn_held && ok "non-JSON grant: commit_turn_held" || bad "non-JSON grant not refused"
kill $holder 2>/dev/null
# reap before refusal: dead holder reaped by exactly the one --reap call through --exec-approved
ct; ( exit 0 ) & dp=$!; wait $dp
printf '{"run_id":"other","paths_from_sha256":"x","pid":%s,"cmdline":"gone","started_at":"2026-10-05T00:00:00Z"}\n' "$dp" >"$S/ct/.audit/commit_turn.json"
t "dead-holder grant reaped, append then succeeds" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
[ "$(grep -c -- '--reap' "$CT_LOG")" = 1 ] && ok "exactly one --reap call made" || bad "--reap call count: $(grep -c -- '--reap' "$CT_LOG" 2>/dev/null)"
grep -q -- '--exec-approved scripts/release/commit_turn_check.sh' "$CT_HOST_LOG" && ok "reap went through --exec-approved (host log)" || bad "no --exec-approved entry in host log"
# paired mutation: running the working-tree script directly leaves the host log empty -> the fixture FAILs
ct; ( exit 0 ) & dp=$!; wait $dp
printf '{"run_id":"other","paths_from_sha256":"x","pid":%s,"cmdline":"gone","started_at":"2026-10-05T00:00:00Z"}\n' "$dp" >"$S/ct/.audit/commit_turn.json"
bash "$S/ct/scripts/release/commit_turn_check.sh" --reap >/dev/null 2>&1
[ -s "$CT_HOST_LOG" ] && bad "mutation: direct run logged on host entry" || ok "mutation (direct working-tree run) leaves host log empty, caught"
# live holder with year-old started_at still refused; stub called exactly once; stores unchanged
ct; grant 2025-10-05T00:00:00Z
"$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1 | grep -q commit_turn_held && ok "live holder, year-old started_at: still refused" || bad "live old grant not refused"
[ "$(wc -l <"$CT_LOG")" = 1 ] && ok "stub called exactly once" || bad "stub call count $(wc -l <"$CT_LOG")"
unchanged && ok "stores unchanged (live old grant)" || bad "stores changed (live old grant)"
kill $holder 2>/dev/null
# host entry refusing (20 release_seam_unreleased): refusal stands, never crash, never append
ct; grant; CT_REFUSE=1 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true >"$S/o" 2>&1; r=$?
{ [ "$r" -ne 0 ] && grep -q commit_turn_held "$S/o" && unchanged; } && ok "host entry refusing: refusal stands, no append" || bad "host refusing: rc=$r $(head -c 200 "$S/o")"
kill $holder 2>/dev/null

echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
