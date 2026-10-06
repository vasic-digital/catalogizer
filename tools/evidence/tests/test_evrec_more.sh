#!/usr/bin/env bash
# T050 companion to test_evrec.sh (T049): cases the T049 test does not isolate, added so that every check of
# the chain walk, the exit-status mapping and the append-time schema refusal is load-bearing under mutation.
# Same anti-vacuity rule as test_evrec.sh: with evrec/verify absent no check may pass.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"   # round 3 (I12)
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
rcof() { "$@" >/dev/null 2>&1; echo $?; }
want() { local name=$1 w=$2; shift 2; local r; r=$(rcof "$@"); [ "$r" = "$w" ] && ok "$name" || bad "$name (rc=$r want $w)"; }
fresh() { rm -rf "$S/w"; mkdir -p "$S/w"; export EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchor.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run "${1:-CAT-001}" "${2:-PROBE}" "${3:-1}" shell_script x -- "${@:4}"; }
build() { fresh; for i in 1 2 3 4; do rec CAT-001 PROBE "$i" true >/dev/null 2>&1; done; cp "$EV_LEDGER" "$S/orig.jsonl"; }

# --- each chain check fires on its own (isolation cases)
build; want "verify: pristine 4-entry ledger exits 0" 0 "$VERIFY"
build; python3 - "$EV_LEDGER" <<'PY'
import json,sys
L=[json.loads(l) for l in open(sys.argv[1])]; L[1]["item"]="CAT-999"
open(sys.argv[1],"w").write("".join(json.dumps(r,sort_keys=True,separators=(",",":"))+"\n" for r in L))
PY
want "verify: edited content (stale entry_hash) exits 1" 1 "$VERIFY"
build; python3 - "$EV_LEDGER" <<'PY'
import hashlib,json,sys
# entry 3 carries a wrong prev_hash but a CORRECT own hash; entry 4 still points at the old head: only the link check fires
L=[json.loads(l) for l in open(sys.argv[1])]
canon=lambda o: json.dumps(o,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
L[2]["prev_hash"]="f"*64; L[2].pop("entry_hash"); L[2]["entry_hash"]=hashlib.sha256(("f"*64).encode()+canon(L[2])).hexdigest()
open(sys.argv[1],"w").write("".join(json.dumps(r,sort_keys=True,separators=(",",":"))+"\n" for r in L))
PY
want "verify: wrong prev_hash with a valid own hash (link only) exits 1" 1 "$VERIFY"
build; python3 - "$EV_LEDGER" <<'PY'
import json,sys
L=[json.loads(l) for l in open(sys.argv[1])]; L[1],L[2]=L[2],L[1]
open(sys.argv[1],"w").write("".join(json.dumps(r,sort_keys=True,separators=(",",":"))+"\n" for r in L))
PY
want "verify: two entries reordered exits 1" 1 "$VERIFY"
build; python3 - "$EV_LEDGER" <<'PY'
import hashlib,json,sys
# renumber entry 3 and re-chain consistently from there: hashes and links all valid, only seq contiguity is wrong
L=[json.loads(l) for l in open(sys.argv[1])]
canon=lambda o: json.dumps(o,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
prev=L[1]["entry_hash"]
for i in (2,3):
    L[i]["seq"]=L[i]["seq"]+10; L[i]["prev_hash"]=prev; L[i].pop("entry_hash")
    L[i]["entry_hash"]=hashlib.sha256(prev.encode()+canon(L[i])).hexdigest(); prev=L[i]["entry_hash"]
open(sys.argv[1],"w").write("".join(json.dumps(r,sort_keys=True,separators=(",",":"))+"\n" for r in L))
PY
want "verify: consistent chain with non-contiguous seq exits 1" 1 "$VERIFY"
build; head -c -5 "$EV_LEDGER" >"$S/trunc"; cp "$S/trunc" "$EV_LEDGER"
want "verify: torn final line exits 1" 1 "$VERIFY"
build; : >"$EV_LEDGER"; want "verify: empty ledger is unverifiable, exit 3" 3 "$VERIFY"
build; rm -f "$EV_LEDGER"; want "verify: absent ledger is unverifiable, exit 3" 3 "$VERIFY"
build; cp "$S/orig.jsonl" "$EV_LEDGER"; v=$("$VERIFY" 2>&1); grep -q 'no anchor compared' <<<"$v" && ok "verify states that the anchor was not compared (T055)" || bad "verify silent about the missing anchor check"

# --- an append refuses a corrupt tail instead of extending it
build; python3 - "$EV_LEDGER" <<'PY'
import json,sys
L=[json.loads(l) for l in open(sys.argv[1])]; L[-1]["item"]="CAT-999"
open(sys.argv[1],"w").write("".join(json.dumps(r,sort_keys=True,separators=(",",":"))+"\n" for r in L))
PY
before=$(sha256sum "$EV_LEDGER" | cut -d' ' -f1); r=$(rcof rec CAT-001 PROBE 9 true)
{ [ "$r" != 0 ] && [ "$(sha256sum "$EV_LEDGER" | cut -d' ' -f1)" = "$before" ]; } && ok "append refused on an inconsistent last entry, ledger unchanged" || bad "append extended a corrupt ledger (rc=$r)"

# --- exit status mapping (docs/06 s3.1 rule 10): 0 pass, 1..125 fail, 126/127/128+ error
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; rec CAT-001 PROBE 2 bash -c 'exit 3' >/dev/null 2>&1
rec CAT-001 PROBE 3 /nonexistent/cmd-xyz >/dev/null 2>&1; rec CAT-001 PROBE 4 bash -c 'kill -9 $$' >/dev/null 2>&1
got=$(jq -r '[.exit_status,.verdict]|join(":")' "$EV_LEDGER" 2>/dev/null | tr '\n' ' ')
[ "$got" = "0:pass 3:fail 127:error 137:error " ] && ok "exit-status mapping: 0 pass, 3 fail, 127 error, SIGKILL 137 error" || bad "exit-status mapping got [$got]"
[ "$(jq -r '.argv|type' "$EV_LEDGER" | sort -u)" = array ] && ok "argv recorded as a list" || bad "argv not a list"
[ "$(jq -r 'select(.exit_status==3)|.argv|join("|")' "$EV_LEDGER")" = "bash|-c|exit 3" ] && ok "argv kept verbatim (-c and the quoted script survive)" || bad "argv not verbatim"
for k in stdout_sha256 stderr_sha256; do d=$(jq -r ".$k" <(head -1 "$EV_LEDGER")); [ -f "$EV_BLOBS/$d" ] && ok "blob for $k exists under its digest" || bad "no blob for $k"; done

# --- append-time schema refusal (a record that ev/1 refuses is never written)
fresh; before=$(rcof "$EVREC" run CAT-001 RED 1 shell_script "$root/tools/evidence/evrec" -- false); [ ! -s "$EV_LEDGER" ] && [ "$before" != 0 ] && ok "RED without oracle/evidence_class refused, nothing written" || bad "RED without oracle accepted (rc=$before)"
fresh; r=$(rcof "$EVREC" run CAT-001 RED 1 shell_script /nonexistent-target --oracle specified --oracle-independent --evidence-class runtime -- false)  # everything else valid: only the target is unresolvable
[ "$r" != 0 ] && [ ! -s "$EV_LEDGER" ] && ok "RED on a target that does not resolve is refused (no invented fingerprint)" || bad "unresolvable RED target accepted (rc=$r)"
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$root/tools/evidence/evrec" --oracle specified --oracle-independent --evidence-class runtime --test-source "$root/tools/evidence/tests/test_evrec_more.sh" -- bash -c 'exit 2' >/dev/null 2>&1
[ "$(jq -r '[.polarity,.verdict,(.oracle.strategy),.evidence_class,(.test_fingerprint|length)]|join(":")' "$EV_LEDGER" 2>/dev/null)" = "RED:fail:specified:runtime:64" ] && ok "RED entry with oracle, evidence class and test_fingerprint is accepted and valid" || bad "valid RED entry not recorded"
[ "$(jq -r .target_fingerprint "$EV_LEDGER" 2>/dev/null)" = "$(sha256sum "$root/tools/evidence/evrec" | cut -d' ' -f1)" ] && ok "target_fingerprint is read from the target at run time (sha256 of its bytes)" || bad "target_fingerprint is not the target's sha256"
# --- redaction also covers argv and stderr
fresh; export P2_SEC="argv-planted-$RANDOM$RANDOM" EVREC_REDACT_VARS=P2_SEC
"$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c 'echo "$0" >&2' "$P2_SEC" >/dev/null 2>&1
grep -rqF -- "$P2_SEC" "$EV_BLOBS" "$EV_LEDGER" 2>/dev/null && bad "secret leaked via argv or stderr" || ok "secret absent from argv, stderr blob and ledger"
unset P2_SEC EVREC_REDACT_VARS
# --- rerecord refuses a side that shares a prefix but carries a tampered entry (the verify of each side is load-bearing)
fresh; mkdir -p "$S/rr2"; mk2() { EV_LEDGER=$1 "$EVREC" run "$2" PROBE "$3" shell_script x -- true >/dev/null 2>&1; }
mk2 "$S/rr2/base.jsonl" CAT-001 1; mk2 "$S/rr2/base.jsonl" CAT-001 2; cp "$S/rr2/base.jsonl" "$S/rr2/remote.jsonl"; cp "$S/rr2/base.jsonl" "$S/rr2/local.jsonl"
mk2 "$S/rr2/remote.jsonl" CAT-002 1; mk2 "$S/rr2/local.jsonl" CAT-003 1
sed -i '3s/CAT-002/CAT-777/' "$S/rr2/remote.jsonl"          # remote side: shares entries 1-2, entry 3 edited without re-hashing
r=$(rcof "$EVREC" rerecord --onto "$S/rr2/remote.jsonl" --local "$S/rr2/local.jsonl" --out "$S/rr2/out")
[ "$r" != 0 ] && [ ! -e "$S/rr2/out/ledger.jsonl" ] && ok "rerecord refuses a tampered remote side that shares a prefix, nothing written" || bad "rerecord accepted a tampered side (rc=$r)"

# --- usage
want "evrec with an unknown subcommand exits 64" 64 "$EVREC" frobnicate
want "evrec run with a missing -- exits 64" 64 "$EVREC" run CAT-001 PROBE 1 shell_script x true

[ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: $(hermetic_host_calls) call(s) reached the host entry"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
