#!/usr/bin/env bash
# T050 round 3 (independent review WF2-REVIEW-wp05-evrec, findings I1-I12 and minors): cases written BEFORE the
# implementation changes and run RED against the round-2 tree first (evidence/wp05/T050r3-red.txt).
# Every refusal check is reason-aware: it asserts the exit code AND `reason=<name>` on the tool's output, so a mutant
# that replaces a named refusal by a crash or by another refusal is caught (review mutants X13-X16).
# Same anti-vacuity rule as the other test files: with evrec/verify absent no check may pass.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchor.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run "${1:-CAT-001}" "${2:-PROBE}" "${3:-1}" shell_script x -- "${@:4}"; }
# refuse NAME RC REASON cmd...   -> ok iff the command exits RC and its stdout+stderr carry reason=REASON
refuse() { local name=$1 rc=$2 reason=$3; shift 3; local out r; out=$("$@" 2>&1); r=$?
  if [ "$r" = "$rc" ] && grep -q "reason=$reason" <<<"$out"; then ok "$name"; else bad "$name (rc=$r want $rc, reason=$reason; got: $(head -c 160 <<<"$out" | tr '\n' ' '))"; fi; }
want() { local name=$1 w=$2; shift 2; local r; "$@" >/dev/null 2>&1; r=$?; [ "$r" = "$w" ] && ok "$name" || bad "$name (rc=$r want $w)"; }
sha() { sha256sum | cut -d' ' -f1; }
EMPTY=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855; ZERO=$(printf '0%.0s' $(seq 1 64))
waitexec() { for _ in $(seq 1 200); do [ "$(tr '\0' ' ' </proc/$1/cmdline 2>/dev/null)" = "$2 " ] && return 0; sleep 0.02; done; return 1; }
# rechain FILE : re-chain every entry of FILE consistently (seq, prev, hash) with python, for hand-edited fixtures
rechain() { python3 - "$1" <<'PY'
import hashlib,json,sys
canon=lambda o: json.dumps(o,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
L=[json.loads(l) for l in open(sys.argv[1],encoding="utf-8")]; prev="0"*64; out=[]
for i,r in enumerate(L,1):
    r["seq"]=i; r["prev_hash"]=prev; r.pop("entry_hash",None)
    r["entry_hash"]=hashlib.sha256(prev.encode()+canon(r)).hexdigest(); prev=r["entry_hash"]; out.append(canon(r)+b"\n")
open(sys.argv[1],"wb").write(b"".join(out))
PY
}

# ===== I1: a live lock holder whose /proc cmdline cannot be read is never reaped (11.4.180, 11.4.201(4))
fresh; mkdir -p "$S/noproc"; sleep 60 & live=$!; waitexec $live "sleep 60"
printf '{"pid":%s,"cmdline":"sleep 60"}\n' "$live" >"$EV_LEDGER.lock"
refuse "I1: live holder with unreadable /proc cmdline is not reaped (lock_held)" 75 lock_held env EVREC_PROC_ROOT="$S/noproc" EVREC_LOCK_TIMEOUT=1 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
kill -0 "$live" 2>/dev/null && ok "I1: that holder process untouched" || bad "I1: holder killed"
[ ! -e "$EV_LEDGER" ] && ok "I1: no ledger written under the unreadable-cmdline holder" || bad "I1: ledger written"
kill $live 2>/dev/null; rm -f "$EV_LEDGER.lock"
fresh; for w in a b c; do ( for i in 1 2 3 4 5 6; do EVREC_PROC_ROOT=$S/noproc "$EVREC" run CAT-001 PROBE "$i" shell_script x -- true >/dev/null 2>&1; done ) & done; wait
cnt=$(wc -l <"$EV_LEDGER" | tr -d ' '); dup=$(jq -r .seq "$EV_LEDGER" | sort -n | uniq -d | wc -l | tr -d ' ')
[ "$cnt" = 18 ] && [ "$dup" = 0 ] && ok "I1: 3 writers x 6 appends with unreadable /proc: 18 entries, no duplicate seq" || bad "I1: entries=$cnt duplicate seq values=$dup (want 18, 0)"
want "I1: that ledger verifies" 0 "$VERIFY"
# X10 / X9: a live pid with a DIFFERENT recorded cmdline is PID reuse (reaped); an unreadable holder identity is never reaped
fresh; sleep 60 & live=$!; waitexec $live "sleep 60"
printf '{"pid":%s,"cmdline":"some other program"}\n' "$live" >"$EV_LEDGER.lock"
want "I1: live pid with a different recorded cmdline (pid reuse) is reaped, append succeeds" 0 env EVREC_LOCK_TIMEOUT=2 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
kill $live 2>/dev/null
for h in '{"pid":"x","cmdline":"y"}' '{"pid":1,"cmdline":"init"}' '{"pid":4242,"cmdline":7}' 'not json at all' '["list"]'; do
  fresh; printf '%s\n' "$h" >"$EV_LEDGER.lock"
  refuse "I1: unreadable holder identity $h is never reaped (lock_held)" 75 lock_held env EVREC_LOCK_TIMEOUT=1 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
  [ -e "$EV_LEDGER.lock" ] && ok "I1: its lock file is still there" || bad "I1: lock file removed for $h"
done

# ===== I2: the commit-turn grant is re-checked after the command and inside the lock, before any blob is written
mkrepo() { rm -rf "$S/ct"; mkdir -p "$S/ct/.audit"; export EVREC_REPO_ROOT=$S/ct; unset EVREC_TURN_RUN_ID; }
grantfile() { sleep 120 & GH=$!; waitexec $GH "sleep 120"; printf '{"run_id":"other","pid":%s,"cmdline":"sleep 120","started_at":"2026-10-05T00:00:00Z"}\n' "$GH" >"$1"; }
noblobs() { [ ! -d "$EV_BLOBS" ] || [ -z "$(ls -A "$EV_BLOBS" 2>/dev/null)" ]; }
fresh; mkrepo; grantfile "$S/grant.json"
out2=$("$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c "echo ran >>\"$S/ran\"; cp \"$S/grant.json\" \"$S/ct/.audit/commit_turn.json\"" 2>&1); r2=$?
{ [ "$r2" = 76 ] && grep -q 'reason=commit_turn_held' <<<"$out2" && grep -q 'where=post_command' <<<"$out2"; } && ok "I2: grant appearing DURING the command: refused after the run (76, commit_turn_held, where=post_command)" || bad "I2: post-command check: rc=$r2 $out2"
[ -s "$S/ran" ] && ok "I2: the command did run once before the grant appeared (stated in the refusal, not hidden)" || bad "I2: command never ran"
[ ! -e "$EV_LEDGER" ] && noblobs && ok "I2: no ledger and no blob written under the foreign grant" || bad "I2: stores written under a foreign grant ($(ls "$EV_BLOBS" 2>/dev/null | wc -l) blobs)"
kill $GH 2>/dev/null
# in-lock re-check: the grant appears between the post-command check and the lock (test hook EVREC_FAULT=stall_before_lock)
fresh; mkrepo; grantfile "$S/grant.json"
( EVREC_FAULT=stall_before_lock "$EVREC" run CAT-001 PROBE 1 shell_script x -- true >"$S/o3" 2>&1; echo $? >"$S/rc3" ) & ev=$!
for _ in $(seq 1 300); do [ -e "$EV_LEDGER.stalled" ] && break; sleep 0.02; done
[ -e "$EV_LEDGER.stalled" ] && ok "I2: recorder reached the stall point after the command" || bad "I2: stall point never reached"
cp "$S/grant.json" "$S/ct/.audit/commit_turn.json"; : >"$EV_LEDGER.go"; wait $ev
{ [ "$(cat "$S/rc3")" = 76 ] && grep -q 'reason=commit_turn_held' "$S/o3" && grep -q 'where=in_lock' "$S/o3"; } && ok "I2: grant appearing before the lock: refused inside the lock (where=in_lock)" || bad "I2: in-lock check: rc=$(cat "$S/rc3") $(head -c 200 "$S/o3")"
[ ! -e "$EV_LEDGER" ] && noblobs && ok "I2: nothing written after the in-lock refusal" || bad "I2: stores written after the in-lock refusal"
kill $GH 2>/dev/null; hermetic_repo
[ "$(hermetic_host_calls)" = 0 ] && ok "I2: the reaper was never called (post-command and in-lock checks do not reap)" || bad "I2: reaper called"

printf '#!/usr/bin/env bash\nexit 20\n' >"$S/refuse_host.sh"; chmod +x "$S/refuse_host.sh"
fresh; mkrepo; grantfile "$S/grant.json"; cp "$S/grant.json" "$S/ct/.audit/commit_turn.json"; rm -f "$S/m2"
refuse "C8: a held grant and a refusing host entry: refused before the command runs (commit_turn_held, where=pre_run)" 76 commit_turn_held env CPA_HOST_ENTRY="$S/refuse_host.sh" "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c "touch $S/m2"
[ ! -e "$S/m2" ] && [ ! -e "$EV_LEDGER" ] && ok "C8: the command was not run under the held grant" || bad "C8: the command ran under a held grant"
kill $GH 2>/dev/null; hermetic_repo

# ===== I3: verify refuses unknown and positional arguments and reports the file it read
fresh; for i in 1 2 3; do rec CAT-001 PROBE "$i" echo hi >/dev/null 2>&1; done; cp "$EV_LEDGER" "$S/good.jsonl"
sed '2d' "$S/good.jsonl" >"$S/bad.jsonl"
refuse "I3: verify with a positional file is refused (usage_error)" 64 usage_error "$VERIFY" "$S/bad.jsonl"
refuse "I3: verify --bogus is refused (usage_error)" 64 usage_error "$VERIFY" --bogus
refuse "I3: verify --ledger without a value is refused (usage_error)" 64 usage_error "$VERIFY" --ledger
refuse "I3: verify --ledger twice is refused (usage_error)" 64 usage_error "$VERIFY" --ledger "$S/good.jsonl" --ledger "$S/bad.jsonl"
out=$("$VERIFY" --ledger "$S/bad.jsonl" 2>&1); r=$?
{ [ "$r" = 1 ] && grep -qF "ledger=$S/bad.jsonl" <<<"$out"; } && ok "I3: verify --ledger names the file it read and fails the tampered one although EV_LEDGER is pristine" || bad "I3: --ledger tampered rc=$r $out"
grep -qF "ledger=$EV_LEDGER" <<<"$("$VERIFY" 2>&1)" && ok "I3: verify names the default ledger it read" || bad "I3: default ledger not named"

# ===== I4: verify checks blobs, ev/1 validity, canonical form; the producer cannot choose the validator
fresh; rec CAT-001 PROBE 1 echo hello >/dev/null 2>&1; rec CAT-001 PROBE 2 echo world >/dev/null 2>&1
d=$(jq -r '.stdout_sha256' <(head -1 "$EV_LEDGER")); cp "$EV_BLOBS/$d" "$S/blob.keep"
printf forged >"$EV_BLOBS/$d"; refuse "I4: forged stdout blob: refused (blob_mismatch, exit 1)" 1 blob_mismatch "$VERIFY"
cp "$S/blob.keep" "$EV_BLOBS/$d"; want "I4: blob restored verifies" 0 "$VERIFY"
rm -rf "$EV_BLOBS"; refuse "I4: every blob deleted: UNVERIFIED (blob_missing, exit 3), not OK" 3 blob_missing "$VERIFY"
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; rec CAT-001 PROBE 2 true >/dev/null 2>&1
cp "$EV_LEDGER" "$S/l.keep"
jq -c 'del(.argv)' "$S/l.keep" >"$EV_LEDGER"; rechain "$EV_LEDGER"
refuse "I4: re-chained entry without argv is refused by verify (schema_invalid)" 1 schema_invalid "$VERIFY"
jq -c 'if .seq==2 then .verdict="fail" else . end' "$S/l.keep" >"$EV_LEDGER"; rechain "$EV_LEDGER"
refuse "I4: re-chained entry with exit_status 0 and verdict fail is refused (schema_invalid)" 1 schema_invalid "$VERIFY"
echo '{}' >"$S/empty_schema.json"
refuse "I4: EV_SCHEMA={} does not weaken verify" 1 schema_invalid env EV_SCHEMA="$S/empty_schema.json" "$VERIFY"
jq -c 'del(.argv)' "$S/l.keep" >"$EV_LEDGER"; rechain "$EV_LEDGER"
refuse "I4: under PYTHONOPTIMIZE=1 the refusal is the same (no assert-based checks)" 1 schema_invalid env PYTHONOPTIMIZE=1 "$VERIFY"
refuse "I4: evrec ignores EV_SCHEMA={} (RED without oracle still record_invalid)" 65 record_invalid env EV_SCHEMA="$S/empty_schema.json" "$EVREC" run CAT-001 RED 1 shell_script "$EVREC" -- false
cp "$S/l.keep" "$EV_LEDGER"; jq -c '.polarity="BASELINE"|.oracle={"strategy":"specified","independent_of_sut":true}|.evidence_class="runtime"' "$S/l.keep" >"$EV_LEDGER"; rechain "$EV_LEDGER"
refuse "I8: a BASELINE entry with the unresolved-fingerprint sentinel is refused by verify (unresolved_fingerprint)" 1 unresolved_fingerprint "$VERIFY"
jq -c 'del(.schema)' "$S/l.keep" >"$EV_LEDGER"; rechain "$EV_LEDGER"
refuse "X12: a re-chained entry without the schema key is an undecodable entry (chain_failure)" 1 chain_failure "$VERIFY"
# minor 4: byte malleability (a duplicate key ahead of the real one leaves grep-based tools reading another value)
cp "$S/l.keep" "$EV_LEDGER"; sed -i '1s/^{/{"item":"CAT-999",/' "$EV_LEDGER"
refuse "I4: duplicate key in a line is refused (non_canonical_line)" 1 non_canonical_line "$VERIFY"
python3 - "$S/l.keep" "$EV_LEDGER" <<'PY'
import json,sys
L=[json.loads(l) for l in open(sys.argv[1])]
open(sys.argv[2],"w").write("".join(json.dumps(r,sort_keys=True)+"\n" for r in L))   # same entries and hashes, default spaced separators
PY
refuse "I4: a hash-valid line that is not byte-canonical (spaced separators) is refused (non_canonical_line)" 1 non_canonical_line "$VERIFY"
# minor 2: an anchor file that exists but cannot be compared is UNVERIFIED, not 0
cp "$S/l.keep" "$EV_LEDGER"; printf '{"anchor":1}\n' >"$EV_ANCHOR"
refuse "I4: anchor present and not compared: UNVERIFIED exit 3" 3 anchor_not_compared "$VERIFY"; rm -f "$EV_ANCHOR"
refuse "I4: a non-integer EV_LEDGER_BOUND is a usage error, not a traceback or a chain failure" 64 usage_error env EV_LEDGER_BOUND=lots "$VERIFY"

# ===== I5: golden vectors for the docs/06 s7 construction, computed independently with jq | sha256sum
mkline() { # mkline SEQ PREV VARIANT -> prints the ledger line; the wrong VARIANTs hash another construction
  local seq=$1 prev=$2 var=${3:-good} body h
  body=$(jq -n -S -c --argjson seq "$seq" --arg prev "$prev" --arg e "$EMPTY" --arg z "$ZERO" --arg cwd "/tmp/é-ü" \
    '{schema:"ev/1",seq:$seq,item:"CAT-001",polarity:"PROBE",iteration:$seq,started_at:"2026-10-05T10:00:00Z",cwd:$cwd,argv:["true"],exit_status:3,verdict:"fail",duration_ms:7,stdout_sha256:$e,stderr_sha256:$e,target_class:"shell_script",target_ref:"x",target_fingerprint:$z,prev_hash:$prev}')
  case $var in
    good)       h=$(printf '%s%s' "$prev" "$body" | sha) ;;
    noprefix)   h=$(printf '%s' "$body" | sha) ;;
    ascii)      h=$(printf '%s%s' "$prev" "$(jq -S -c -a . <<<"$body")" | sha) ;;
    skipexit)   h=$(printf '%s%s' "$prev" "$(jq -S -c 'del(.exit_status,.verdict)' <<<"$body")" | sha) ;;
  esac
  jq -S -c --arg h "$h" '. + {entry_hash:$h}' <<<"$body"
}
goldledger() { local var=$1 prev=$ZERO line; : >"$EV_LEDGER"; for s in 1 2; do line=$(mkline $s $prev $var); echo "$line" >>"$EV_LEDGER"; prev=$(jq -r .entry_hash <<<"$line"); done; }
fresh; mkdir -p "$EV_BLOBS"; : >"$EV_BLOBS/$EMPTY"
goldledger good; want "I5: golden-good: a ledger hashed independently with jq|sha256sum (non-ASCII cwd, exit_status 3) verifies" 0 "$VERIFY"
for var in noprefix ascii skipexit; do goldledger $var; refuse "I5: construction '$var' differs from docs/06 s7: refused as a bad hash" 1 chain_failure "$VERIFY"
  grep -q 'bad hash at line 1' <<<"$("$VERIFY" 2>&1)" && ok "I5: '$var' named as bad hash at line 1" || bad "I5: '$var' not named as bad hash"; done
goldledger good; jq -c 'if .seq==2 then .exit_status=1 else . end' "$EV_LEDGER" | python3 -c '
import json,sys
for l in sys.stdin: sys.stdout.write(json.dumps(json.loads(l),sort_keys=True,separators=(",",":"),ensure_ascii=False)+"\n")' >"$S/flip.jsonl"; cp "$S/flip.jsonl" "$EV_LEDGER"
grep -q 'bad hash at line 2' <<<"$("$VERIFY" 2>&1)" && ok "I5: flipping exit_status alone (hash not recomputed) is a bad hash" || bad "I5: exit_status flip not detected as bad hash"

# ===== I6: owed relocations of large in-tree blobs; rerecord --out may not target the shared store
fresh; out=$(EV_BLOB_BOUND=1000 "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c 'head -c 2000 /dev/zero | tr "\0" a' 2>&1)
grep -q 'owed_relocation blob=' <<<"$out" && grep -q 'bytes=2000' <<<"$out" && grep -q 'bound=1000' <<<"$out" && ok "I6: evrec run lists a blob above the bound as owed_relocation (bytes and bound named)" || bad "I6: no owed_relocation for a 2000 byte blob: $out"
out=$(EV_BLOB_BOUND=1000 "$VERIFY" 2>&1); grep -q 'owed_relocation blob=' <<<"$out" && ok "I6: verify lists the same large blob" || bad "I6: verify lists no owed_relocation"
fresh; out=$(EV_BLOB_BOUND=1000 "$EVREC" run CAT-001 PROBE 1 shell_script x -- echo small 2>&1); grep -q owed_relocation <<<"$out" && bad "I6: small blob listed" || ok "I6: a small blob is not listed"
rm -f "$S/m3"; fresh
refuse "I6: a non-integer EV_BLOB_BOUND is refused (usage_error) before the command runs" 64 usage_error env EV_BLOB_BOUND=lots "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c "touch $S/m3"
[ ! -e "$S/m3" ] && [ ! -e "$EV_LEDGER" ] && ok "I6: the command was not run and nothing was written" || bad "I6: command ran under a bad EV_BLOB_BOUND"
fresh; out=$("$EVREC" run CAT-001 PROBE 1 shell_script x -- echo hello 2>&1); out2=$("$VERIFY" 2>&1)
grep -q owed_relocation <<<"$out$out2" && bad "I6: a small blob is listed in the default environment: $out $out2" || ok "I6: nothing is listed as owed in the default environment (no leaked EV_BLOB_BOUND)"
bound=$(awk -F'\t' '$1=="evidence"&&$2=="large_file"{print $3}' "$root/scripts/repo/check_classes.tsv")
fresh; out=$(EV_BLOB_BOUND= "$EVREC" run CAT-001 PROBE 2 shell_script x -- bash -c "head -c $((bound+5)) /dev/zero | tr '\\0' b" 2>&1)
grep -q "owed_relocation blob=.* bytes=$((bound+5)) bound=$bound" <<<"$out" && ok "I6: the default blob bound is the check_classes.tsv 'evidence large_file' value ($bound); an empty EV_BLOB_BOUND counts as unset" || bad "I6: default bound is not $bound: $out"
fresh; for i in 1 2 3; do rec CAT-00$i PROBE 1 true >/dev/null 2>&1; done; before=$(sha256sum "$EV_LEDGER" | sha)
refuse "I6: rerecord --out <shared ledger dir> is refused (out_is_shared_store)" 70 out_is_shared_store "$EVREC" rerecord --onto "$EV_LEDGER" --local "$EV_LEDGER" --out "$(dirname "$EV_LEDGER")"
[ "$(sha256sum "$EV_LEDGER" | sha)" = "$before" ] && ok "I6: the shared ledger is byte-unchanged after the refused --out run" || bad "I6: shared ledger changed by rerecord --out"
refuse "I6: rerecord --out <shared dir> --name <other file> is refused too (the directory is the shared store)" 70 out_is_shared_store "$EVREC" rerecord --onto "$EV_LEDGER" --local "$EV_LEDGER" --out "$(dirname "$EV_LEDGER")" --name other.jsonl
[ ! -e "$(dirname "$EV_LEDGER")/other.jsonl" ] && ok "I6: no stray output file in the shared directory" || bad "I6: other.jsonl written into the shared directory"
cp "$EV_LEDGER" "$S/side1.jsonl"; cp "$EV_LEDGER" "$S/side2.jsonl"; EV_LEDGER=$S/side2.jsonl rec CAT-004 PROBE 1 true >/dev/null 2>&1
want "I6: rerecord --out <elsewhere> succeeds" 0 "$EVREC" rerecord --onto "$S/side1.jsonl" --local "$S/side2.jsonl" --out "$S/outdir"
[ "$(sha256sum "$EV_LEDGER" | sha)" = "$before" ] && ok "I6: the shared ledger is byte-unchanged after a legitimate rerecord (store_not_rerecorded shape)" || bad "I6: shared ledger changed"
refuse "I6: rerecord --out writing over one of its own input files is refused (out_is_shared_store)" 70 out_is_shared_store "$EVREC" rerecord --onto "$S/side1.jsonl" --local "$S/side2.jsonl" --out "$S" --name side1.jsonl
sleep 60 & live=$!; waitexec $live "sleep 60"; printf '{"pid":%s,"cmdline":"sleep 60"}\n' "$live" >"$S/outdir2.lock.tmp"; mkdir -p "$S/outdir2"; mv "$S/outdir2.lock.tmp" "$S/outdir2/ledger.jsonl.lock"
refuse "I6: rerecord takes the lock of the output ledger (a live holder: lock_held)" 75 lock_held env EVREC_LOCK_TIMEOUT=1 "$EVREC" rerecord --onto "$S/side1.jsonl" --local "$S/side2.jsonl" --out "$S/outdir2"
kill $live 2>/dev/null
[ "$(jq -S -c '.' "$S/outdir/ledger-seq-map.json")" = "$(jq -S -c -n --arg d "$(tail -1 "$S/outdir/ledger.jsonl" | jq -r .entry_hash)" '[{old:4,new:4,digest:$d}]')" ] && ok "I6: seq map content is exactly [{old,new,digest=new entry_hash}]" || bad "I6: seq map content: $(cat "$S/outdir/ledger-seq-map.json")"
# (the seq map above has old==new==4 because side2 appended entry 4 onto an identical prefix; a differing case follows)
mk() { EV_LEDGER=$1 "$EVREC" run "$2" PROBE "$3" shell_script x -- true >/dev/null 2>&1; }
rm -rf "$S/rr"; mkdir -p "$S/rr"; mk "$S/rr/base.jsonl" CAT-001 1; cp "$S/rr/base.jsonl" "$S/rr/remote.jsonl"; cp "$S/rr/base.jsonl" "$S/rr/local.jsonl"
mk "$S/rr/remote.jsonl" CAT-002 1; mk "$S/rr/local.jsonl" CAT-003 1; mk "$S/rr/local.jsonl" CAT-003 2
"$EVREC" rerecord --onto "$S/rr/remote.jsonl" --local "$S/rr/local.jsonl" --out "$S/rr/out" >/dev/null 2>&1
want_map=$(jq -S -c -n --argjson d "$(jq -s -c '[.[2:][].entry_hash]' "$S/rr/out/ledger.jsonl")" '[{old:2,new:3,digest:$d[0]},{old:3,new:4,digest:$d[1]}]')
[ "$(jq -S -c . "$S/rr/out/ledger-seq-map.json")" = "$want_map" ] && ok "I6: seq map of a diverged rerecord maps old 2,3 to new 3,4 with the new entry hashes" || bad "I6: diverged seq map $(cat "$S/rr/out/ledger-seq-map.json") want $want_map"

# ===== I7: redaction (overlap, default patterns, target_ref)
fresh; export RS_A=SHORTKEY RS_B=SHORTKEYSUFFIXSECRET9 EVREC_REDACT_VARS="RS_A RS_B"
rec CAT-001 PROBE 1 bash -c 'echo "auth=SHORTKEYSUFFIXSECRET9 and SHORTKEY"' >/dev/null 2>&1
grep -rqF -e SUFFIXSECRET9 -e SHORTKEY "$EV_BLOBS" "$EV_LEDGER" && bad "I7: prefix-overlapping secrets leaked (longer value's suffix visible)" || ok "I7: a secret that is a prefix of another leaves no fragment of either"
export RS_A=ghijklmn RS_B=klmnopqr
rec CAT-001 PROBE 2 bash -c 'echo "x ghijklmnopqr y"' >/dev/null 2>&1
grep -rqF -e ghij -e opqr -e klmn "$EV_BLOBS" "$EV_LEDGER" && bad "I7: partially overlapping secrets leaked a fragment" || ok "I7: partially overlapping secrets leave no fragment"
unset RS_A RS_B EVREC_REDACT_VARS
fresh; export STREAMSEC=Stream0nlySecretValue1 EVREC_REDACT_VARS=STREAMSEC
rec CAT-001 PROBE 1 bash -c 'printf "%s\n" "$STREAMSEC"' >/dev/null 2>&1; rec CAT-001 PROBE 2 bash -c 'printf "%s\n" "$STREAMSEC" >&2' >/dev/null 2>&1
{ ! grep -rqF Stream0nlySecretValue1 "$EV_BLOBS" "$EV_LEDGER"; } && ok "I7: a named secret printed only on stdout or only on stderr is absent from blobs and ledger" || bad "I7: stream-only secret stored"
[ "$(jq -s -r '[.[0].redacted,.[1].redacted]|join(",")' "$EV_LEDGER")" = "true,true" ] && ok "I7: redacted: true is set for a stdout-only and for a stderr-only hit (argv carries no secret)" || bad "I7: stream-only redacted flags: $(jq -s -c '[.[].redacted]' "$EV_LEDGER")"
unset STREAMSEC EVREC_REDACT_VARS
fresh; tok=Zx9QwErTyUiOp0123456789abcd
rec CAT-001 PROBE 1 bash -c "echo 'Authorization: Bearer $tok'" >/dev/null 2>&1
grep -rqF "$tok" "$EV_BLOBS" "$EV_LEDGER" && bad "I7: Bearer token stored verbatim with EVREC_REDACT_VARS unset" || ok "I7: Bearer token redacted by the default pattern scan (no variable named)"
[ "$(jq -r '.redacted' "$EV_LEDGER")" = true ] && ok "I7: entry records redacted: true for a default-pattern hit" || bad "I7: redacted flag missing"
fresh; ak="AKIA$(printf 'ABCDEFGHIJKLMNOP')"; pw='hunter2hunter2'
rec CAT-001 PROBE 1 bash -c "echo $ak; echo password=$pw; echo https://svc-user:$pw@example.invalid/p" >/dev/null 2>&1
grep -rqF -e "$ak" -e "$pw" "$EV_BLOBS" "$EV_LEDGER" && bad "I7: AWS key id, password= value or URL userinfo password stored" || ok "I7: AWS key id, password= value and URL userinfo password redacted"
fresh; gh="ghp_$(printf 'A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8')"
rec CAT-001 PROBE 1 bash -c "echo $gh" >/dev/null 2>&1
grep -rqF "$gh" "$EV_BLOBS" "$EV_LEDGER" && bad "I7: GitHub token stored" || ok "I7: a GitHub token is redacted"
fresh; mkdir -p "$S/token=Cwd123secretXYZ"; ( cd "$S/token=Cwd123secretXYZ" && "$EVREC" run CAT-001 PROBE 1 shell_script x -- true >/dev/null 2>&1 )
{ ! grep -qF Cwd123secretXYZ "$EV_LEDGER"; } && [ "$(jq -r .redacted "$EV_LEDGER")" = true ] && ok "I7: a secret in the working directory name is redacted from the ledger" || bad "I7: cwd secret stored: $(jq -c .cwd "$EV_LEDGER")"
fresh; pk="-----BEGIN RSA PRIVATE KEY-----"; body="MIIBOgIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu"
rec CAT-001 PROBE 1 bash -c "echo '$pk'; echo $body; echo '-----END RSA PRIVATE KEY-----'" >/dev/null 2>&1
grep -rqF "$body" "$EV_BLOBS" "$EV_LEDGER" && bad "I7: private key body stored" || ok "I7: a PEM private key block is redacted"
fresh; "$EVREC" run CAT-001 PROBE 1 remote_service "https://svc-user:pass1234word@example.invalid/x" -- true >/dev/null 2>&1
{ ! grep -qF pass1234word "$EV_LEDGER"; } && [ "$(jq -r '.redacted' "$EV_LEDGER")" = true ] && ok "I7: a secret in target_ref is redacted from the ledger and redacted: true is set" || bad "I7: target_ref secret in the ledger: $(cat "$EV_LEDGER")"
fresh; rec CAT-001 PROBE 1 bash -c 'echo "plain output without secrets 12345"' >/dev/null 2>&1
[ "$(jq -r '.redacted // "absent"' "$EV_LEDGER")" = absent ] && ok "I7: a clean entry carries no redacted flag (no false positive)" || bad "I7: redacted set on a clean entry"

# ===== I8: an unresolved target fingerprint is never accepted for verdict-bearing polarities and is distinguishable
for pol in BASELINE PREFLIGHT NEEDLE ANALYZER_FIXTURE; do fresh
  refuse "I8: $pol on an unresolvable target is refused (target_unreadable)" 77 target_unreadable "$EVREC" run CAT-001 $pol 1 go_binary /no/such/binary --oracle specified --oracle-independent --evidence-class runtime -- true
  [ ! -e "$EV_LEDGER" ] && ok "I8: $pol wrote nothing" || bad "I8: $pol wrote an entry"; done
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1
[ "$(jq -r .target_fingerprint "$EV_LEDGER")" = "$ZERO" ] && ok "I8: a PROBE on an unresolved target carries the all-zero sentinel, not a digest of the ref" || bad "I8: unresolved PROBE fingerprint is $(jq -r .target_fingerprint "$EV_LEDGER")"

# ===== I9: docs/06 rule 11, test sources must be declared when argv[0] is an interpreter
for pol in RED GREEN MUTATION; do fresh; ex=0; [ $pol = RED ] && ex=1
  mut=(); [ $pol = MUTATION ] && mut=(--mutation-json '{"operator":"x","location":"y","author":"reviewer","result":"caught"}') && ex=1
  refuse "I9: $pol with bash -c and no declared test source is refused (interpreter_without_test_sources)" 69 interpreter_without_test_sources \
    "$EVREC" run CAT-001 $pol 1 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime "${mut[@]}" -- bash -c "exit $ex"; done
fresh; printf 'echo A\nexit 1\n' >"$S/testA.sh"; printf 'echo B\nexit 1\n' >"$S/testB.sh"
"$EVREC" run CAT-001 RED 1 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime --test-source "$S/testA.sh" -- bash "$S/testA.sh" >/dev/null 2>&1
"$EVREC" run CAT-001 RED 2 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime --test-source "$S/testB.sh" -- bash "$S/testB.sh" >/dev/null 2>&1
"$EVREC" run CAT-001 RED 3 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime --test-source "$S/testA.sh" -- bash "$S/testA.sh" >/dev/null 2>&1
t=$(jq -r .test_fingerprint "$EV_LEDGER" | tr '\n' ' ')
read -r t1 t2 t3 _ <<<"$t"; [ -n "${t1:-}" ] && [ "${t1:-}" != "${t2:-}" ] && [ "${t1:-}" = "${t3:-}" ] && ok "I9: declared sources make the test_fingerprint differ for different test bytes and agree for the same ones" || bad "I9: test_fingerprints $t"
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime -- bash -c 'exit 1' >/dev/null 2>&1; [ ! -e "$EV_LEDGER" ] && ok "I9: nothing written for the undeclared interpreter RED" || bad "I9: undeclared interpreter RED written"
fresh; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/direct.sh"; chmod +x "$S/direct.sh"
want "I9: a script executed directly is its own test bytes: no source declaration needed" 0 "$EVREC" run CAT-001 RED 1 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime -- "$S/direct.sh"
want "I9: PROBE with an interpreter and no sources is allowed (no verdict claim)" 0 "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c 'exit 0'
want "I9: EV_TEST_SOURCES counts as a declaration" 0 env EV_TEST_SOURCES="$S/testA.sh" "$EVREC" run CAT-001 RED 9 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime -- bash -c 'exit 1'

# ===== I10: oracle independence is declared by the caller, never asserted by the recorder
fresh; rm -f "$S/ran10"
refuse "I10: --oracle without --oracle-independent is refused (oracle_independence_undeclared) before the command runs" 64 oracle_independence_undeclared \
  "$EVREC" run CAT-001 RED 1 shell_script "$EVREC" --oracle specified --evidence-class runtime --test-source "$S/testA.sh" -- bash -c "touch $S/ran10; exit 1"
[ ! -e "$S/ran10" ] && [ ! -e "$EV_LEDGER" ] && ok "I10: the command did not run and nothing was written" || bad "I10: command ran or entry written"
refuse "I10: --oracle-independent without --oracle is refused (usage_error)" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script x --oracle-independent -- true
"$EVREC" run CAT-001 RED 1 shell_script "$EVREC" --oracle derived --oracle-independent --evidence-class runtime --test-source "$S/testA.sh" -- bash -c 'exit 1' >/dev/null 2>&1
[ "$(jq -c .oracle "$EV_LEDGER")" = '{"independent_of_sut":true,"strategy":"derived"}' ] && ok "I10: the declared oracle is recorded as given" || bad "I10: oracle $(jq -c .oracle "$EV_LEDGER")"

# ===== I11: everything is validated BEFORE the recorded command runs
pre() { # pre NAME RC REASON cmd... ; the command line must carry the marker side effect, which must not happen
  local name=$1; shift; rm -f "$S/marker"; fresh; refuse "$name" "$@"
  [ ! -e "$S/marker" ] && [ ! -e "$EV_LEDGER" ] && { [ ! -d "$EV_BLOBS" ] || [ -z "$(ls -A "$EV_BLOBS")" ]; } && ok "$name: command not run, nothing stored" || bad "$name: marker=$([ -e "$S/marker" ] && echo yes || echo no) ledger=$([ -e "$EV_LEDGER" ] && echo yes || echo no)"; }
M="touch $S/marker"
pre "I11: RED without oracle (schema refusal) before the run" 65 record_invalid "$EVREC" run CAT-001 RED 1 shell_script "$EVREC" --test-source "$S/testA.sh" -- bash -c "$M; exit 1"
pre "I11: invalid --mutation-json before the run" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script x --mutation-json '{bad' -- bash -c "$M"
pre "I11: non-UTF-8 argv byte before the run" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c "$M" $'\xff'
pre "I11: missing --test-source file before the run" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script x --test-source "$S/no-such-source" -- bash -c "$M"
pre "I11: a flag standing where REF belongs is refused, not taken as the target (minor 11)" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script --frobnicate -- bash -c "$M"
pre "I11: unknown flag before the run" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script x --frobnicate -- bash -c "$M"
pre "I11: --pre-release without --test-state-go (schema) before the run" 65 record_invalid "$EVREC" run CAT-001 PROBE 1 shell_script x --pre-release -- bash -c "$M"
mkdir -p "$S/bad-$(printf '\xff')"; rm -f "$S/marker"; fresh
( cd "$S/bad-"$'\xff' && refuse "I11: non-UTF-8 cwd is refused before the run (usage_error)" 64 usage_error "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c "$M" ); [ ! -e "$S/marker" ] && ok "I11: non-UTF-8 cwd: command not run" || bad "I11: command ran in a non-UTF-8 cwd"
# schema_unavailable before the run: a tree copy without the contract file
rm -rf "$S/noschema"; mkdir -p "$S/noschema/tools/evidence" "$S/noschema/specs"; cp "$root/tools/evidence/evrec" "$root/tools/evidence/verify" "$root/tools/evidence/evcore.py" "$S/noschema/tools/evidence/"
rm -f "$S/marker"; fresh; refuse "I11: schema_unavailable is refused before the command runs (67)" 67 schema_unavailable "$S/noschema/tools/evidence/evrec" run CAT-001 PROBE 1 shell_script x -- bash -c "$M"
[ ! -e "$S/marker" ] && ok "I11: schema_unavailable: command not run" || bad "I11: command ran without a schema"
# outcome-dependent refusal happens after the run but writes no blob and no entry
fresh; refuse "I11: GREEN that fails is refused after the run (record_invalid) and leaves no blob" 65 record_invalid "$EVREC" run CAT-001 GREEN 1 shell_script "$EVREC" --oracle specified --oracle-independent --evidence-class runtime --test-source "$S/testA.sh" -- bash -c 'echo out; exit 1'
[ ! -e "$EV_LEDGER" ] && noblobs && ok "I11: no orphan blob after the post-run refusal" || bad "I11: orphan blobs after a post-run refusal"

# ===== minors and the reviewer's surviving mutants
# X19: golden-good check-record; reasons for refusals
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; jq -c 'del(.prev_hash,.entry_hash,.seq)' "$EV_LEDGER" >"$S/rec.json"
out=$("$EVREC" check-record "$S/rec.json" 2>&1); r=$?; { [ $r = 0 ] && [ "$out" = "record valid" ]; } && ok "X19: golden-good check-record accepts a valid unchained record ('record valid')" || bad "X19: check-record valid: rc=$r $out"
jq -c '.started_at="not-a-date-at-allZ"' "$S/rec.json" >"$S/rec2.json"; refuse "minor5: check-record asserts the date-time format (record_invalid)" 65 record_invalid "$EVREC" check-record "$S/rec2.json"
jq -c 'del(.argv)' "$S/rec.json" >"$S/rec3.json"; refuse "X16: check-record refusal is the named record_invalid" 65 record_invalid "$EVREC" check-record "$S/rec3.json"
# X20: every optional recorder flag lands in the entry
fresh; printf 'x\n' >"$S/gos1.json"; printf 'y\n' >"$S/gos2.json"
dg=sha256:$(printf 'c%.0s' $(seq 1 64))
"$EVREC" run CAT-001 GREEN 1 shell_script "$EVREC" --oracle golden_master --oracle-independent --evidence-class user_visible --precondition-provenance observed --closes-item \
  --mutation-json '{"operator":"flip","location":"a.go:1","author":"reviewer","result":"caught"}' --container-image-digest "$dg" --pre-release --test-state-go gos1.json --test-state-go gos2.json \
  --test-source "$S/testA.sh" -- bash -c 'exit 0' >/dev/null 2>&1
got=$(jq -c '[.pre_release,.test_state_gos,.closes_item,.container_image_digest,.precondition_provenance,.evidence_class,.mutation.result]' "$EV_LEDGER" 2>/dev/null)
[ "$got" = "[true,[\"gos1.json\",\"gos2.json\"],true,\"$dg\",\"observed\",\"user_visible\",\"caught\"]" ] && ok "X20: --pre-release, --test-state-go, --closes-item, --mutation-json, --container-image-digest, --precondition-provenance, --evidence-class all land in the entry" || bad "X20: optional fields: $got"
# X7: 126 is an error verdict; X8: directory fingerprint reads content
fresh; printf '#!/bin/sh\nexit 0\n' >"$S/noexec.sh"; chmod 644 "$S/noexec.sh"
rec CAT-001 PROBE 1 "$S/noexec.sh" >/dev/null 2>&1
[ "$(jq -r '[.exit_status,.verdict]|join(":")' "$EV_LEDGER")" = "126:error" ] && ok "X7: a non-executable command is exit 126, verdict error (not fail)" || bad "X7: 126 mapping: $(jq -c '[.exit_status,.verdict]' "$EV_LEDGER")"
fresh; mkdir -p "$S/tdir/sub"; printf 'one\n' >"$S/tdir/a.txt"; printf 'two\n' >"$S/tdir/sub/b.txt"
exp=$( { printf '%s\0%s\n' a.txt "$(sha256sum <"$S/tdir/a.txt" | cut -d' ' -f1)"; printf '%s\0%s\n' sub/b.txt "$(sha256sum <"$S/tdir/sub/b.txt" | cut -d' ' -f1)"; } | sha )
"$EVREC" run CAT-001 PROBE 1 shell_script "$S/tdir" -- true >/dev/null 2>&1
[ "$(jq -r .target_fingerprint "$EV_LEDGER")" = "$exp" ] && ok "X8: directory fingerprint equals the independently computed sha256 over sorted 'relpath NUL filehash' lines" || bad "X8: dir fingerprint $(jq -r .target_fingerprint "$EV_LEDGER") want $exp"
printf 'CHANGED\n' >"$S/tdir/a.txt"; "$EVREC" run CAT-001 PROBE 2 shell_script "$S/tdir" -- true >/dev/null 2>&1
[ "$(jq -r .target_fingerprint "$EV_LEDGER" | sort -u | wc -l | tr -d ' ')" = 2 ] && ok "X8: changing a file's content changes the directory fingerprint" || bad "X8: directory fingerprint ignores content"
# X11: the flake ledger beside the ledger is measured too
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; head -c 3000 /dev/zero | tr '\0' 'f' >"$S/w/flake_ledger.jsonl"
out=$(EV_LEDGER_BOUND=3500 "$VERIFY" 2>&1); grep -q 'size flake_ledger.jsonl' <<<"$out" && grep -q 'ledger_raise_owed: flake_ledger.jsonl' <<<"$out" && ok "X11: the flake ledger is measured against the bound and named by ledger_raise_owed" || bad "X11: flake ledger not measured: $out"
# X6: append refuses a last entry whose seq differs from the line count; reasons for append/rerecord refusals
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; jq -c '.seq=7' "$EV_LEDGER" >"$S/seq7.jsonl"; cp "$S/seq7.jsonl" "$EV_LEDGER"
python3 - "$EV_LEDGER" <<'PY'
import hashlib,json,sys
canon=lambda o: json.dumps(o,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
r=json.loads(open(sys.argv[1]).read()); r.pop("entry_hash"); r["entry_hash"]=hashlib.sha256(r["prev_hash"].encode()+canon(r)).hexdigest(); open(sys.argv[1],"wb").write(canon(r)+b"\n")
PY
refuse "X6: a self-consistent last entry with seq 7 on line 1 refuses the append (ledger_inconsistent)" 66 ledger_inconsistent "$EVREC" run CAT-001 PROBE 2 shell_script x -- true
fresh; mk "$S/rr/a.jsonl" CAT-001 1; mk "$S/rr/z.jsonl" CAT-009 1
refuse "X13: rerecord of sides with no common prefix names no_common_prefix" 68 no_common_prefix "$EVREC" rerecord --onto "$S/rr/a.jsonl" --local "$S/rr/z.jsonl" --out "$S/rr/o9"
printf 'garbage\n' >"$S/rr/garbage.jsonl"; refuse "rerecord of a side verify rejects names side_unverifiable" 68 side_unverifiable "$EVREC" rerecord --onto "$S/rr/garbage.jsonl" --local "$S/rr/a.jsonl" --out "$S/rr/o10"
printf 'see ledger#4 and ledger#9\n' >"$S/rr/doc.md"; printf '%s\n' "$S/rr/doc.md" >"$S/rr/list"; printf '[{"old":4,"new":5,"digest":"x"}]\n' >"$S/rr/map.json"
refuse "X14: remap-refs with an uncovered reference names map_incomplete" 68 map_incomplete "$EVREC" remap-refs --map "$S/rr/map.json" --files-from "$S/rr/list"
grep -q 'ledger#4 and ledger#9' "$S/rr/doc.md" && ok "X14: nothing was rewritten on the map_incomplete refusal" || bad "X14: file changed on refusal"
fresh; sleep 60 & live=$!; waitexec $live "sleep 60"; printf '{"pid":%s,"cmdline":"sleep 60"}\n' "$live" >"$EV_LEDGER.lock"
refuse "X15: a live lock holder yields the named lock_held (75)" 75 lock_held env EVREC_LOCK_TIMEOUT=1 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true; kill $live 2>/dev/null
refuse "usage: evrec run with a non-numeric ITER names usage_error" 64 usage_error "$EVREC" run CAT-001 PROBE one shell_script x -- true
refuse "usage: unknown subcommand names usage_error" 64 usage_error "$EVREC" frobnicate
fresh; refuse "reconcile: runner count mismatch names count_mismatch" 1 count_mismatch "$EVREC" reconcile --runner-count 3

hermetic_repo
[ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: $(hermetic_host_calls) call(s) reached the host entry"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
