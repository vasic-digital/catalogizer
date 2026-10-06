#!/usr/bin/env bash
# test_check_review_provenance.sh - T094a test for scripts/review/check_review_provenance.sh (round 3).
# Host control-plane bash and jq (P0-P1 host exception: must exist before the first image).
# CRP_SCRIPT overrides the script under test (used by the paired-mutation runs).
# Refusals are asserted on the EXACT exit code (20) and reason; usage errors on exit 2.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SCRIPT=${CRP_SCRIPT:-$here/../check_review_provenance.sh}
repo=$(cd "$here/../../.." && pwd)
REL=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
pass=0; fail=0
sha() { sha256sum < "$1" | cut -d' ' -f1; }
ZERO64=$(printf '0%.0s' $(seq 64)); ONE64=$(printf '1%.0s' $(seq 64))

# approved copy of the schema (what cpa-host materialises as released/); the script must read it from here
mkdir -p "$tmp/approved/$(dirname "$REL")"
if [ -f "$repo/$REL" ]; then cp "$repo/$REL" "$tmp/approved/$REL"; else echo "FAIL schema absent in the repository: $REL"; echo "RESULT pass=0 fail=1"; exit 1; fi
export CPA_APPROVED_DIR=$tmp/approved
cd "$repo" || exit 1      # the working tree HAS the schema: a script reading it from here is caught by the s3 fixture

mkverdict() { # name model effort -> writes $tmp/<name>.json
  jq -n --arg m "$2" --arg e "$3" --arg z "$ZERO64" '{schema:"review-verdict/1",verdict:"GO",covers_runs:[],model:$m,effort:$e,blocking_findings:0,reviewed_files:[{path:"a.sh",sha256:$z}]}' > "$tmp/$1.json"
}
mkprov() { # name model effort effort_arg [with_sha=1] [run_id]
  local n=$1 m=$2 e=$3 ea=$4 ws=${5:-1} rid=${6-wf_00000001-abc}
  jq -n --arg m "$m" --arg e "$e" --arg ea "$ea" --arg rid "$rid" \
    '{workflow_run_id:$rid,model:$m,effort:$e,effort_argument:(if $ea=="" then null else $ea end)}' > "$tmp/$n.provenance.json"
  if [ "$ws" = 1 ]; then rebind "$n"; fi
}
rebind() { jq --arg s "$(sha "$tmp/$1.json")" '.verdict_sha256=$s' "$tmp/$1.provenance.json" > "$tmp/x" && mv "$tmp/x" "$tmp/$1.provenance.json"; }
editv() { jq "$2" "$tmp/$1.json" > "$tmp/x" && mv "$tmp/x" "$tmp/$1.json"; rebind "$1"; }       # edit verdict, rebind sha
editp() { jq "$2" "$tmp/$1.provenance.json" > "$tmp/x" && mv "$tmp/x" "$tmp/$1.provenance.json"; } # edit record only

ok() { pass=$((pass+1)); echo "ok   $1"; }
bad() { fail=$((fail+1)); echo "FAIL $1"; }
run() { out=$(bash "$SCRIPT" "$@" 2>&1); rc=$?; }   # sets out, rc
check() { # label want_rc reason_or_empty : judge the last run()
  local label=$1 want=$2 reason=$3
  if [ "$want" = 0 ]; then
    if [ $rc -eq 0 ]; then ok "$label"; else bad "$label (rc=$rc: $out)"; fi
  else
    if [ $rc -eq "$want" ] && [ -n "$reason" ] && printf '%s' "$out" | grep -q "^REFUSED reason=$reason\( \|\$\)"; then ok "$label (rc=$want $reason)"
    elif [ $rc -eq "$want" ] && [ -z "$reason" ]; then ok "$label (rc=$want)"
    else bad "$label (rc=$rc want $want reason=$reason: $out)"; fi
  fi
}
expect()  { run "$tmp/$4.json"; check "$1" "$2" "$3"; }             # label want_rc reason verdictname
expect2() { run "$4" "$5"; check "$1" "$2" "$3"; }                 # label want_rc reason verdictfile recordfile
refused() { expect "$1" 20 "$2" "$3"; }

# script must exist (RED while absent)
[ -f "$SCRIPT" ] || { echo "FAIL script absent: $SCRIPT"; echo "RESULT pass=0 fail=1"; exit 1; }

mkverdict good claude-opus-5 xhigh;     mkprov good "claude-opus-5" xhigh xhigh
expect "xhigh Opus record passes (control needle for every refusal below)" 0 "" good

# ---- original T094a fixtures
mkverdict hi claude-opus-5 high;        mkprov hi "claude-opus-5" high high
refused "high record refused" effort_not_xhigh hi
mkverdict q claude-opus-5 '?';          mkprov q "claude-opus-5" '?' ""
refused "? record refused" effort_not_xhigh q
mkverdict none claude-opus-5 xhigh      # no provenance file at all
refused "missing record refused" provenance_absent none
mkverdict claim claude-opus-5 xhigh;    mkprov claim "claude-opus-5" xhigh ""
refused "claimed xhigh without effort argument refused (control needle)" effort_argument_absent claim
mkverdict sonnet claude-sonnet-5-5 xhigh;   mkprov sonnet "claude-sonnet-5-5" xhigh xhigh
refused "non-Opus model refused" model_not_opus sonnet
mkverdict norun claude-opus-5 xhigh;    mkprov norun "claude-opus-5" xhigh xhigh 1 ""
refused "record without run id refused" workflow_run_id_absent norun
mkverdict mism claude-opus-5 xhigh;     mkprov mism "claude-opus-5" xhigh xhigh
editv mism '.effort="max"'
refused "verdict effort differs from record refused" verdict_field_mismatch mism
mkverdict byte claude-opus-5 xhigh;     mkprov byte "claude-opus-5" xhigh xhigh
jq --arg o "$ONE64" '.reviewed_files[0].sha256=$o' "$tmp/byte.json" > "$tmp/x" && mv "$tmp/x" "$tmp/byte.json"
refused "verdict changed by one byte refused (verdict_sha_mismatch)" verdict_sha_mismatch byte
mkverdict nosha claude-opus-5 xhigh;    mkprov nosha "claude-opus-5" xhigh xhigh 0
refused "record without verdict_sha256 refused (verdict_sha_absent)" verdict_sha_absent nosha

# ---- B3: verdict model must EQUAL the record model
mkverdict vm1 claude-opus-5-5 xhigh; mkprov vm1 "claude-opus-5" xhigh xhigh
refused "verdict model differs from record model refused (B3)" verdict_field_mismatch vm1
mkverdict vm2 opus-fake-claim xhigh; mkprov vm2 "claude-opus-5" xhigh xhigh
refused "verdict model 'opus-fake-claim' vs real record refused (B3)" verdict_field_mismatch vm2
mkverdict vm3 claude-opus-5 xhigh;   mkprov vm3 "claude-opus-5" xhigh xhigh
editv vm3 'del(.model)'
refused "verdict without model refused (B3)" verdict_field_mismatch vm3

# ---- I1: record model = exact id of a closed allow-list, a string
mkverdict m1 "claude-sonnet-5-5 (not opus)" xhigh; mkprov m1 "claude-sonnet-5-5 (not opus)" xhigh xhigh
refused "substring 'opus' in a non-Opus model refused (I1)" model_not_opus m1
mkverdict m2 claude-opus-5 xhigh; mkprov m2 "claude-opus-5" xhigh xhigh
editp m2 '.model=["claude-sonnet-5-5","opus"]'
refused "record model as an array refused (I1)" model_not_opus m2
mkverdict m3 opus xhigh; mkprov m3 "opus" xhigh xhigh
refused "bare 'opus' refused, not an allow-list id (I1)" model_not_opus m3
mkverdict m4 Claude-Opus-5 xhigh; mkprov m4 "Claude-Opus-5" xhigh xhigh
refused "case variant refused, exact match only (I1)" model_not_opus m4
mkverdict m5 claude-opus-5x xhigh; mkprov m5 "claude-opus-5x" xhigh xhigh
refused "id with a suffix refused (I1)" model_not_opus m5

# ---- I-1a: several allow-list ids joined by spaces are not an allow-list id (both r1 fixes were incomplete)
mkverdict j1 "claude-opus-5 claude-opus-5-5" xhigh; mkprov j1 "claude-opus-5 claude-opus-5-5" xhigh xhigh
refused "two adjacent allow-list ids joined by a space refused (I-1a)" model_not_opus j1
mkverdict j2 "claude-opus-4-1 claude-opus-4" xhigh; mkprov j2 "claude-opus-4-1 claude-opus-4" xhigh xhigh
refused "ids 'claude-opus-4-1 claude-opus-4' joined refused (I-1a)" model_not_opus j2

# ---- I-1b: lossy bash string handling (trailing newlines) must not make values equal
mkverdict n1 claude-opus-5 xhigh; mkprov n1 "claude-opus-5" xhigh xhigh
editv n1 '.model="claude-opus-5\n"'
refused "verdict model with a trailing newline differs from the record (I-1b)" verdict_field_mismatch n1
mkverdict n2 claude-opus-5 xhigh; mkprov n2 "claude-opus-5" xhigh xhigh
editv n2 '.effort="xhigh\n\n"'
refused "verdict effort with trailing newlines differs from the record (I-1b)" verdict_field_mismatch n2
mkverdict n3 claude-opus-5 xhigh; mkprov n3 "claude-opus-5" xhigh xhigh
editp n3 '.effort="xhigh\n"'
refused "record effort 'xhigh' + newline is not xhigh (I-1b)" effort_not_xhigh n3
mkverdict n4 claude-opus-5 xhigh; mkprov n4 "claude-opus-5" xhigh xhigh
editp n4 '.effort_argument="xhigh\n"'
refused "record effort_argument 'xhigh' + newline is not xhigh (I-1b)" effort_argument_mismatch n4
mkverdict n5 claude-opus-5 xhigh; mkprov n5 "claude-opus-5" xhigh xhigh
editp n5 '.model="claude-opus-5\n\n"'
refused "record model with trailing newlines refused (I-1b)" model_not_opus n5

# ---- I2: exactly one top-level JSON object in each file
mkverdict d1 claude-opus-5 xhigh; mkprov d1 "claude-opus-5" xhigh xhigh
printf '%s\n' '{"verdict":"NO-GO","blocking_findings":3}' >> "$tmp/d1.json"; rebind d1
refused "second JSON document in the verdict refused (I2)" verdict_unreadable d1
mkverdict d2 claude-opus-5 xhigh; mkprov d2 "claude-opus-5" xhigh xhigh
printf '%s\n' '{"workflow_run_id":"other"}' >> "$tmp/d2.provenance.json"
refused "second JSON document in the record refused (I2)" provenance_unreadable d2
mkverdict d3 claude-opus-5 xhigh; mkprov d3 "claude-opus-5" xhigh xhigh
printf '[%s]\n' "$(cat "$tmp/d3.json")" > "$tmp/d3.json"; rebind d3
refused "top-level array verdict refused (I2)" verdict_unreadable d3

# ---- m-4: strict JSON: BOM and invalid UTF-8 refused (not only a second document)
mkverdict bom1 claude-opus-5 xhigh; mkprov bom1 "claude-opus-5" xhigh xhigh
{ printf '\xef\xbb\xbf'; cat "$tmp/bom1.json"; } > "$tmp/x" && mv "$tmp/x" "$tmp/bom1.json"; rebind bom1
refused "UTF-8 BOM in the verdict refused (m-4)" verdict_unreadable bom1
mkverdict bom2 claude-opus-5 xhigh; mkprov bom2 "claude-opus-5" xhigh xhigh
{ printf '\xef\xbb\xbf'; cat "$tmp/bom2.provenance.json"; } > "$tmp/x" && mv "$tmp/x" "$tmp/bom2.provenance.json"
refused "UTF-8 BOM in the record refused (m-4)" provenance_unreadable bom2
mkverdict u1 claude-opus-5 xhigh; mkprov u1 "claude-opus-5" xhigh xhigh
printf '{"schema":"review-verdict/1","note":"\xff\xfe"}' > "$tmp/u1.json"; rebind u1
refused "invalid UTF-8 bytes in the verdict refused (m-4)" verdict_unreadable u1
mkverdict u2 claude-opus-5 xhigh; mkprov u2 "claude-opus-5" xhigh xhigh
printf '{"workflow_run_id":"\xff"}' > "$tmp/u2.provenance.json"
refused "invalid UTF-8 bytes in the record refused (m-4)" provenance_unreadable u2

# ---- c2: the explicit record argument is the record used, not the default-path one
mkverdict e1 claude-opus-5 xhigh; mkprov e1 "claude-opus-5" high high      # default-path record is bad
cp "$tmp/e1.provenance.json" "$tmp/e1.bad.json"
jq -n --arg s "$(sha "$tmp/e1.json")" '{workflow_run_id:"wf_00000001-abc",model:"claude-opus-5",effort:"xhigh",effort_argument:"xhigh",verdict_sha256:$s}' > "$tmp/e1.good.json"
expect2 "explicit good record used although default-path record is bad (c2)" 0 "" "$tmp/e1.json" "$tmp/e1.good.json"
mkverdict e2 claude-opus-5 xhigh; mkprov e2 "claude-opus-5" xhigh xhigh      # default-path record is good
cp "$tmp/e1.bad.json" "$tmp/e2.bad.json"
expect2 "explicit bad record refused although default-path record is good (c2)" 20 effort_not_xhigh "$tmp/e2.json" "$tmp/e2.bad.json"

# ---- I-2: fixtures that kill the reviewer's survivors R1, R2, R3, R4, R6b and the behavioural c3
mkverdict r1 claude-opus-5 xhigh; mkprov r1 "claude-opus-5" xhigh xhigh
editv r1 'del(.effort)'
refused "verdict lacking effort refused (R1)" verdict_field_mismatch r1
mkverdict r2 claude-opus-5 xhigh; mkprov r2 "claude-opus-5" xhigh xhigh
editp r2 '.effort_argument="high"'
refused "record effort xhigh with effort_argument high refused (R2)" effort_argument_mismatch r2
mkverdict r3 claude-opus-5 '?'; mkprov r3 "claude-opus-5" '?' xhigh
refused "record effort ? with effort_argument xhigh refused: 'accept ?' behaviour (c3)" effort_not_xhigh r3
mkverdict r6a claude-opus-5 xhigh; mkprov r6a "claude-opus-5" xhigh xhigh
jq --arg s "$(sha "$tmp/r6a.json" | cut -c1-63)" '.verdict_sha256=$s' "$tmp/r6a.provenance.json" > "$tmp/x" && mv "$tmp/x" "$tmp/r6a.provenance.json"
refused "record sha that is a proper prefix of the actual sha refused (R6b)" verdict_sha_mismatch r6a
mkverdict r6b claude-opus-5 xhigh; mkprov r6b "claude-opus-5" xhigh xhigh
jq --arg s "2" '.verdict_sha256=$s' "$tmp/r6b.provenance.json" > "$tmp/x" && mv "$tmp/x" "$tmp/r6b.provenance.json"
refused "record sha '2' refused (R6b)" verdict_sha_mismatch r6b
mkverdict r6c claude-opus-5 xhigh; mkprov r6c "claude-opus-5" xhigh xhigh
a=$(sha "$tmp/r6c.json"); last=${a: -1}; if [ "$last" = 0 ]; then nl=1; else nl=0; fi
jq --arg s "${a%?}$nl" '.verdict_sha256=$s' "$tmp/r6c.provenance.json" > "$tmp/x" && mv "$tmp/x" "$tmp/r6c.provenance.json"
refused "record sha differing only in the last hex character refused (R6)" verdict_sha_mismatch r6c
run; check "no arguments: usage error is exit 2 (R4)" 2 ""
run "$tmp/good.json" "$tmp/good.provenance.json" extra; check "three arguments: usage error is exit 2 (R4)" 2 ""

# ---- I-3: the run id is a well-formed Workflow run id (shape observed on this host, UNCONFIRMED in general)
for spec in 'blank|" "|workflow_run_id_malformed' 'wrongshape|"wf-run-0001"|workflow_run_id_malformed' 'trailingnl|"wf_00000001-abc\n"|workflow_run_id_malformed' \
            'embedded|"xx-wf_00000001-abc-yy"|workflow_run_id_malformed' 'bool|true|workflow_run_id_absent' 'obj|{}|workflow_run_id_absent' \
            'num|0|workflow_run_id_absent' 'arr|["x"]|workflow_run_id_absent' 'null|null|workflow_run_id_absent'; do
  IFS='|' read -r nm val why <<<"$spec"
  mkverdict "i3$nm" claude-opus-5 xhigh; mkprov "i3$nm" "claude-opus-5" xhigh xhigh
  editp "i3$nm" ".workflow_run_id=$val"
  refused "run id $nm refused (I-3)" "$why" "i3$nm"
done
mkverdict rp claude-opus-5 xhigh; mkprov rp "claude-opus-5" xhigh xhigh 1 "wf-run-0001"
out=$(CRP_RUN_ID_PATTERN='wf-run-[0-9]+' bash "$SCRIPT" "$tmp/rp.json" 2>&1); rc=$?; check "run id pattern is configurable (CRP_RUN_ID_PATTERN)" 0 ""

# ---- B-1: verdict validated against review-verdict.schema.json read from CPA_APPROVED_DIR
mkverdict sc1 claude-opus-5 xhigh; mkprov sc1 "claude-opus-5" xhigh xhigh
editv sc1 'del(.schema) | del(.covers_runs) | .verdict="MAYBE" | .blocking_findings="zero"'
refused "verdict with no schema/covers_runs, verdict MAYBE, blocking_findings 'zero' refused (B-1 probe)" verdict_schema_invalid sc1
mkverdict sc2 claude-opus-5 xhigh; mkprov sc2 "claude-opus-5" xhigh xhigh
editv sc2 '.blocking_findings=2'
refused "GO with blocking_findings 2 refused by the schema (B-1)" verdict_schema_invalid sc2
mkverdict sc3 claude-opus-5 xhigh; mkprov sc3 "claude-opus-5" xhigh xhigh
editv sc3 '.reviewed_files[0].sha256="00"'
refused "reviewed_files entry with a short sha256 refused by the schema (B-1)" verdict_schema_invalid sc3
mkverdict sc4 claude-opus-5 xhigh; mkprov sc4 "claude-opus-5" xhigh xhigh
out=$(env -u CPA_APPROVED_DIR bash "$SCRIPT" "$tmp/sc4.json" 2>&1); rc=$?
check "CPA_APPROVED_DIR unset: schema check refused, never skipped (B-1)" 20 verdict_schema_unavailable
if printf '%s' "$out" | grep -q 'CPA_APPROVED_DIR unset'; then ok "unset case names CPA_APPROVED_DIR in its detail"; else bad "unset case detail: $out"; fi
mkdir -p "$tmp/empty"; out=$(CPA_APPROVED_DIR=$tmp/empty bash "$SCRIPT" "$tmp/sc4.json" 2>&1); rc=$?
check "approved dir without the schema file refused (B-1)" 20 verdict_schema_unavailable
mkdir -p "$tmp/badsch/$(dirname "$REL")"; printf '{' > "$tmp/badsch/$REL"
out=$(CPA_APPROVED_DIR=$tmp/badsch bash "$SCRIPT" "$tmp/sc4.json" 2>&1); rc=$?
check "unusable schema file refused as unavailable, not accepted (B-1)" 20 verdict_schema_unavailable
# the schema is read from the approved copy, never from the working tree (which holds a lax copy here)
mkdir -p "$tmp/strict/$(dirname "$REL")"; jq '.required += ["approval_marker"]' "$repo/$REL" > "$tmp/strict/$REL"
out=$(CPA_APPROVED_DIR=$tmp/strict bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "schema comes from CPA_APPROVED_DIR, not the working tree (s3)" 20 verdict_schema_invalid
mkdir -p "$tmp/alt/sub"; cp "$repo/$REL" "$tmp/alt/sub/s.json"
out=$(CPA_APPROVED_DIR=$tmp/alt CRP_SCHEMA_REL=sub/s.json bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "CRP_SCHEMA_REL selects the schema path under CPA_APPROVED_DIR" 0 ""

# ---- m-1: the verdict is read once, into a private copy; no other command is handed the original path
mkdir -p "$tmp/shim"; for c in cat jq python3 sha256sum; do
  real=$(command -v $c); printf '#!/bin/sh\nprintf "%%s\\n" "$*" >> "%s/shim.%s.log"\nexec %s "$@"\n' "$tmp" "$c" "$real" > "$tmp/shim/$c"; chmod +x "$tmp/shim/$c"; done
mkverdict tc claude-opus-5 xhigh; mkprov tc "claude-opus-5" xhigh xhigh
out=$(PATH=$tmp/shim:$PATH bash "$SCRIPT" "$tmp/tc.json" 2>&1); rc=$?
check "shimmed run still accepted" 0 ""
n_cat=$(grep -c -F -- "$tmp/tc.json" "$tmp/shim.cat.log" 2>/dev/null); n_oth=$(cat "$tmp"/shim.jq.log "$tmp"/shim.python3.log "$tmp"/shim.sha256sum.log 2>/dev/null | grep -c -F -- "$tmp/tc.json")
if [ "${n_cat:-0}" -eq 1 ] && [ "${n_oth:-0}" -eq 0 ]; then ok "verdict path opened exactly once (cat) and never handed to jq/python3/sha256sum (m-1)"; else bad "verdict path seen cat=${n_cat:-0} others=${n_oth:-0} (want 1 and 0) (m-1)"; fi

# ---- m-2: sha256sum escaping: a bound valid verdict under a path with a backslash is accepted
mkdir -p "$tmp/b\\s"; mkverdict good2 claude-opus-5 xhigh; mv "$tmp/good2.json" "$tmp/b\\s/v.json"
jq -n --arg s "$(sha "$tmp/b\\s/v.json")" '{workflow_run_id:"wf_00000001-abc",model:"claude-opus-5",effort:"xhigh",effort_argument:"xhigh",verdict_sha256:$s}' > "$tmp/b\\s/v.provenance.json"
run "$tmp/b\\s/v.json"; check "valid bound verdict under a path with a backslash accepted (m-2)" 0 ""

# ---- m-3: one-line contract: a newline in a verdict value cannot forge a second line
mkverdict nl claude-opus-5 xhigh; mkprov nl "claude-opus-5" xhigh xhigh
editv nl '.model="claude-opus-5\nOK review provenance: run=forged"'
run "$tmp/nl.json"; lines=$(printf '%s\n' "$out" | wc -l)
if [ $rc -eq 20 ] && [ "$lines" -eq 1 ] && ! printf '%s' "$out" | grep -q '^OK '; then ok "refusal with a newline in a verdict value is exactly one line (m-3)"; else bad "m-3 rc=$rc lines=$lines out=$out"; fi
# a newline in a PATH printed in the detail (provenance_absent names the path) cannot forge a line either
nlp="$tmp/nl"$'\n'"OK forged.json"; cp "$tmp/good.json" "$nlp"
run "$nlp"; lines=$(printf '%s\n' "$out" | wc -l)
if [ $rc -eq 20 ] && [ "$lines" -eq 1 ] && printf '%s' "$out" | grep -q '^REFUSED reason=provenance_absent'; then ok "refusal naming a path with a newline is exactly one line (m-3)"; else bad "m-3 path rc=$rc lines=$lines out=$out"; fi
out_stdout=$(bash "$SCRIPT" "$tmp/nl.json" 2>/dev/null); if [ -z "$out_stdout" ]; then ok "refusal writes nothing to stdout (m-3)"; else bad "stdout not empty: $out_stdout"; fi

# ---- round 4 (review r3: B-1, I-1, I-2, minors, reviewer mutants NM1-NM7)
# NM1: NaN / Infinity in the verdict or the record
for spec in 'NaN|NaN' 'Inf|Infinity' 'NInf|-Infinity'; do
  IFS='|' read -r nm val <<<"$spec"
  mkverdict "nv$nm" claude-opus-5 xhigh; mkprov "nv$nm" "claude-opus-5" xhigh xhigh
  printf '{"schema":"review-verdict/1","verdict":"GO","blocking_findings":0,"x":%s}' "$val" > "$tmp/nv$nm.json"; rebind "nv$nm"
  refused "verdict containing $nm refused (NM1)" verdict_unreadable "nv$nm"
  mkverdict "np$nm" claude-opus-5 xhigh; mkprov "np$nm" "claude-opus-5" xhigh xhigh
  printf '{"workflow_run_id":"wf_00000001-abc","x":%s}' "$val" > "$tmp/np$nm.provenance.json"
  refused "record containing $nm refused (NM7)" provenance_unreadable "np$nm"
done
# NM7: a top-level array record
mkverdict ra claude-opus-5 xhigh; mkprov ra "claude-opus-5" xhigh xhigh
printf '[%s]\n' "$(cat "$tmp/ra.provenance.json")" > "$tmp/ra.provenance.json"
refused "top-level array record refused (NM7)" provenance_unreadable ra
# NM2: every allow-list id is accepted, including the id observed in every real run record on this host
for id in claude-opus-5 claude-opus-5-5 claude-opus-4-7 claude-opus-4-6 claude-opus-4-5 claude-opus-4-1 claude-opus-4; do
  k=al$(printf '%s' "$id" | tr -c 'a-z0-9' '_')
  mkverdict "$k" "$id" xhigh; mkprov "$k" "$id" xhigh xhigh
  expect "allow-list id $id accepted (NM2, no false refusal)" 0 "" "$k"
done
# NM5 + m-2: the exact accept line, run id without a stray character
mkverdict okl claude-opus-5-5 xhigh; mkprov okl "claude-opus-5-5" xhigh xhigh
run "$tmp/okl.json"
if [ $rc -eq 0 ] && [ "$out" = 'OK review provenance: run=wf_00000001-abc model="claude-opus-5-5" effort="xhigh"' ]; then ok "exact OK line with the real model and run id (NM5, m-2)"; else bad "OK line: rc=$rc out=[$out]"; fi
# NM3 + m-4: a parseable but metaschema-invalid approved schema is one line, unavailable (not a traceback)
mkdir -p "$tmp/meta/$(dirname "$REL")"; printf '{"type":"nonsense"}' > "$tmp/meta/$REL"
out=$(CPA_APPROVED_DIR=$tmp/meta bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
lines=$(printf '%s\n' "$out" | wc -l)
if [ $rc -eq 20 ] && [ "$lines" -eq 1 ] && printf '%s' "$out" | grep -q '^REFUSED reason=verdict_schema_unavailable UNAVAILABLE schema unusable: '; then ok "metaschema-invalid approved schema: one line, unavailable (NM3)"; else bad "NM3 rc=$rc lines=$lines out=$out"; fi
# m-4: an exception while validating (an unresolvable $ref) is one line, unavailable, never a traceback
mkdir -p "$tmp/refs/$(dirname "$REL")"; printf '{"$ref":"nonexistent.schema.json"}' > "$tmp/refs/$REL"
out=$(CPA_APPROVED_DIR=$tmp/refs bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
lines=$(printf '%s\n' "$out" | wc -l)
if [ $rc -eq 20 ] && [ "$lines" -eq 1 ] && printf '%s' "$out" | grep -q '^REFUSED reason=verdict_schema_unavailable '; then ok "unresolvable \$ref while validating: one line, unavailable (m-4)"; else bad "m-4 rc=$rc lines=$lines out=$out"; fi
# NM4: a FIFO verdict / record is refused at once, never hangs (bounded by timeout, exit 124 would be a hang)
mkfifo "$tmp/fifo.json"; mkverdict ff claude-opus-5 xhigh
out=$(timeout 4 bash "$SCRIPT" "$tmp/fifo.json" 2>&1); rc=$?
if [ $rc -eq 20 ] && printf '%s' "$out" | grep -q '^REFUSED reason=verdict_unreadable'; then ok "FIFO verdict refused without hanging (NM4)"; else bad "NM4 fifo verdict rc=$rc out=$out"; fi
timeout 1 sh -c ": > '$tmp/fifo.json'" 2>/dev/null   # release a reader a hung mutant may leave behind
mkfifo "$tmp/ff.provenance.json"
out=$(timeout 4 bash "$SCRIPT" "$tmp/ff.json" 2>&1); rc=$?
if [ $rc -eq 20 ] && printf '%s' "$out" | grep -q '^REFUSED reason=provenance_absent'; then ok "FIFO record refused without hanging (NM4)"; else bad "NM4 fifo record rc=$rc out=$out"; fi
timeout 1 sh -c ": > '$tmp/ff.provenance.json'" 2>/dev/null
# odd / long run ids: one line, malformed
mkverdict lid claude-opus-5 xhigh; mkprov lid "claude-opus-5" xhigh xhigh 1 "wf_00000001-abc$(head -c 6000 /dev/zero | tr '\0' 'a')"
run "$tmp/lid.json"; lines=$(printf '%s\n' "$out" | wc -l)
if [ $rc -eq 20 ] && [ "$lines" -eq 1 ] && printf '%s' "$out" | grep -q '^REFUSED reason=workflow_run_id_malformed'; then ok "6000-character run id refused as malformed, one line"; else bad "long id rc=$rc lines=$lines"; fi
mkverdict uid claude-opus-5 xhigh; mkprov uid "claude-opus-5" xhigh xhigh 1 "wf_00000001-ab$(printf '\xc3\xa9')"
run "$tmp/uid.json"; check "non-ASCII run id refused as malformed" 20 workflow_run_id_malformed
mkverdict nid claude-opus-5 xhigh; mkprov nid "claude-opus-5" xhigh xhigh
editp nid '.workflow_run_id="wf_00000001-abc\nREFUSED forged"'
run "$tmp/nid.json"; lines=$(printf '%s\n' "$out" | wc -l)
if [ $rc -eq 20 ] && [ "$lines" -eq 1 ]; then ok "run id with an embedded newline refused, one line"; else bad "embedded nl rc=$rc lines=$lines"; fi
# nit: an explicit empty second argument is a usage error, not a silent fallback to the default record
run "$tmp/good.json" ""; check "empty second argument is a usage error (nit)" 2 ""

# ---- I-1: the trusted checker cannot be redirected by the environment
mkdir -p "$tmp/outside" "$tmp/escapes/$(dirname "$REL")"; printf '{}' > "$tmp/outside/lax.json"
mkverdict lx claude-opus-5 xhigh; mkprov lx "claude-opus-5" xhigh xhigh
editv lx 'del(.schema) | del(.covers_runs) | .verdict="MAYBE" | .blocking_findings="zero"'
run "$tmp/lx.json"; check "control: the approved schema refuses the lax-probe verdict" 20 verdict_schema_invalid
out=$(CRP_SCHEMA_REL=../outside/lax.json CPA_APPROVED_DIR=$tmp/approved bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "CRP_SCHEMA_REL with a .. segment refused (I-1)" 20 verdict_schema_unavailable
out=$(CRP_SCHEMA_REL=$tmp/outside/lax.json CPA_APPROVED_DIR=$tmp/approved bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "absolute CRP_SCHEMA_REL refused (I-1)" 20 verdict_schema_unavailable
out=$(CRP_SCHEMA_REL=sub/../sub/s.json CPA_APPROVED_DIR=$tmp/alt bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check ".. segment refused even when it stays inside (I-1)" 20 verdict_schema_unavailable
mkdir -p "$tmp/sym"; ln -s "$tmp/outside/lax.json" "$tmp/sym/link.json"
out=$(CRP_SCHEMA_REL=link.json CPA_APPROVED_DIR=$tmp/sym bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "symlink escaping CPA_APPROVED_DIR refused (I-1)" 20 verdict_schema_unavailable
mkdir -p "$tmp/sym2/real"; cp "$repo/$REL" "$tmp/sym2/real/s.json"; ln -s real/s.json "$tmp/sym2/link.json"
out=$(CRP_SCHEMA_REL=link.json CPA_APPROVED_DIR=$tmp/sym2 bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "symlink staying inside CPA_APPROVED_DIR accepted (I-1 golden-true)" 0 ""
out=$(CPA_EXEC_SHA256=abc CRP_RUN_ID_PATTERN='.*' bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "CRP_RUN_ID_PATTERN refused in trusted mode (I-1)" 2 ""
out=$(CPA_EXEC_SHA256=abc CRP_SCHEMA_REL=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "CRP_SCHEMA_REL refused in trusted mode even with a valid value (I-1)" 2 ""
out=$(CPA_EXEC_SHA256=abc bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "trusted mode without overrides accepted (I-1 golden-true)" 0 ""
out=$(CRP_RUN_ID_PATTERN='(' bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "invalid CRP_RUN_ID_PATTERN is a configuration error, exit 2 (m-5)" 2 ""

# ---- B-1: explicit bootstrap mode (T046a step 1, CENTRAL C6 first approval): no approved copy exists yet
addrf() { jq --arg p "$2" --arg s "$3" '.reviewed_files += [{path:$p,sha256:$s}]' "$tmp/$1.json" > "$tmp/x" && mv "$tmp/x" "$tmp/$1.json"; rebind "$1"; }
runb() { out=$(env -u CPA_APPROVED_DIR bash "$SCRIPT" --bootstrap "$@" 2>&1); rc=$?; }
SHA_SCH=$(sha "$repo/$REL")
mkverdict bs1 claude-opus-5 xhigh; mkprov bs1 "claude-opus-5" xhigh xhigh; addrf bs1 "$REL" "$SHA_SCH"
runb "$tmp/bs1.json"
if [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^OK review provenance (BOOTSTRAP'; then ok "bootstrap: working-tree schema bound to reviewed_files accepted (B-1)"; else bad "bootstrap accept rc=$rc out=$out"; fi
mkverdict bs2 claude-opus-5 xhigh; mkprov bs2 "claude-opus-5" xhigh xhigh
runb "$tmp/bs2.json"; check "bootstrap: verdict not listing the schema refused (B-1)" 20 bootstrap_schema_not_reviewed
mkverdict bs3 claude-opus-5 xhigh; mkprov bs3 "claude-opus-5" xhigh xhigh; addrf bs3 "$REL" "$ONE64"
runb "$tmp/bs3.json"; check "bootstrap: listed schema sha differs from the file refused (B-1)" 20 bootstrap_schema_sha_mismatch
mkverdict bs4 claude-opus-5 xhigh; mkprov bs4 "claude-opus-5" xhigh xhigh; addrf bs4 "$REL" "$SHA_SCH"
out=$(CPA_APPROVED_DIR=$tmp/approved bash "$SCRIPT" --bootstrap "$tmp/bs4.json" 2>&1); rc=$?
check "bootstrap refused when an approved copy exists (B-1: the narrow mode)" 2 ""
mkverdict bs5 claude-opus-5 xhigh; mkprov bs5 "claude-opus-5" xhigh xhigh; addrf bs5 "$REL" "$SHA_SCH"; editv bs5 '.blocking_findings=2'
runb "$tmp/bs5.json"; check "bootstrap still validates the verdict against the schema (B-1 needle)" 20 verdict_schema_invalid
mkverdict bs6 claude-opus-5 xhigh; mkprov bs6 "claude-opus-5" xhigh xhigh; addrf bs6 "$REL" "$SHA_SCH"
out=$(env -u CPA_APPROVED_DIR CRP_SCHEMA_REL=x bash "$SCRIPT" --bootstrap "$tmp/bs6.json" 2>&1); rc=$?
check "bootstrap refuses a CRP_SCHEMA_REL override (B-1)" 2 ""
out=$(env -u CPA_APPROVED_DIR CRP_RUN_ID_PATTERN='.*' bash "$SCRIPT" --bootstrap "$tmp/bs6.json" 2>&1); rc=$?
check "bootstrap refuses a CRP_RUN_ID_PATTERN override (B-1)" 2 ""
out=$(env -u CPA_APPROVED_DIR CPA_EXEC_SHA256=abc bash "$SCRIPT" --bootstrap "$tmp/bs6.json" 2>&1); rc=$?
check "bootstrap refused in trusted mode (CPA_EXEC_SHA256 set) (B-1)" 2 ""
mkverdict bs7 claude-opus-5 xhigh; mkprov bs7 "claude-opus-5" xhigh xhigh; addrf bs7 "$REL" "$SHA_SCH"
out=$(cd "$tmp" && env -u CPA_APPROVED_DIR bash "$SCRIPT" --bootstrap "$tmp/bs7.json" 2>&1); rc=$?
check "bootstrap outside a git work tree is a usage error (B-1)" 2 ""
# a lax schema in the working tree, with the verdict listing the REAL schema sha: refused (the file must equal what was reviewed)
mkdir -p "$tmp/lax-wt/$(dirname "$REL")"; (cd "$tmp/lax-wt" && git init -q . 2>/dev/null); printf '{}' > "$tmp/lax-wt/$REL"
out=$(cd "$tmp/lax-wt" && env -u CPA_APPROVED_DIR bash "$SCRIPT" --bootstrap "$tmp/bs6.json" 2>&1); rc=$?
check "bootstrap: lax working-tree schema that is not the reviewed one refused (B-1)" 20 bootstrap_schema_sha_mismatch
# a symlinked schema that leaves the work tree is refused
mkdir -p "$tmp/sym-wt/$(dirname "$REL")"; (cd "$tmp/sym-wt" && git init -q . 2>/dev/null); ln -s "$repo/$REL" "$tmp/sym-wt/$REL"
out=$(cd "$tmp/sym-wt" && env -u CPA_APPROVED_DIR bash "$SCRIPT" --bootstrap "$tmp/bs6.json" 2>&1); rc=$?
check "bootstrap: schema symlink leaving the work tree refused (B-1)" 20 verdict_schema_unavailable
# without --bootstrap the same working-tree situation is still refused (no silent fallback)
mkverdict bs8 claude-opus-5 xhigh; mkprov bs8 "claude-opus-5" xhigh xhigh; addrf bs8 "$REL" "$SHA_SCH"
out=$(env -u CPA_APPROVED_DIR bash "$SCRIPT" "$tmp/bs8.json" 2>&1); rc=$?
check "no --bootstrap and no CPA_APPROVED_DIR: refused, never a silent working-tree fallback (B-1)" 20 verdict_schema_unavailable
out=$(bash "$SCRIPT" --bootstrap 2>&1); rc=$?; check "--bootstrap alone is a usage error" 2 ""

# ---- round 5 (review r4: I-1 interpreter isolation, I-2 hostile environment, I-3 mutation adequacy, m-1 validator crash)
# I-1: an untracked jsonschema/ or json.py in the working directory must not bend the approved checker (python3 -I)
FAKEV=$tmp/fake_js; mkdir -p "$FAKEV/jsonschema"; : > "$FAKEV/jsonschema/__init__.py"
cat > "$FAKEV/jsonschema/validators.py" <<'PYF'
def validator_for(schema):
    class V:
        @staticmethod
        def check_schema(s): return None
        def __init__(self, s): pass
        def iter_errors(self, d): return []
    return V
PYF
FAKEJ=$tmp/fake_json; mkdir -p "$FAKEJ"
printf 'def loads(s, **k): return {}\ndef load(f, **k): return {}\n' > "$FAKEJ/json.py"
out=$(cd "$FAKEV" && bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "control: a cwd holding an accept-all jsonschema/ cannot disable the schema check (I-1)" 20 verdict_schema_invalid
out=$(cd "$FAKEJ" && bash "$SCRIPT" "$tmp/nvNaN.json" 2>&1); rc=$?
check "a cwd holding a lenient json.py cannot disable the strict-JSON verdict layer (I-1)" 20 verdict_unreadable
out=$(cd "$FAKEJ" && bash "$SCRIPT" "$tmp/good.json" "$tmp/npNaN.provenance.json" 2>&1); rc=$?
check "a cwd holding a lenient json.py cannot disable the strict-JSON record layer (I-1)" 20 provenance_unreadable
out=$(cd "$FAKEV" && bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "golden-true: the same hostile cwd with a valid verdict is still accepted (I-1)" 0 ""
out=$(PYTHONPATH=$FAKEV bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "PYTHONPATH holding an accept-all jsonschema/ cannot disable the schema check (I-2 E1)" 20 verdict_schema_invalid

# I-2: hostile environment is scrubbed (variables and exported functions) before any child runs
mkdir -p "$tmp/envshim"; real_jq=$(command -v jq); real_py=$(command -v python3)
for c in jq python3; do real=$(command -v $c)
  printf '#!/bin/sh\necho "LP=${LD_PRELOAD-unset} LL=${LD_LIBRARY_PATH-unset} PP=${PYTHONPATH-unset} PH=${PYTHONHOME-unset} PS=${PYTHONSTARTUP-unset} BE=${BASH_ENV-unset} EN=${ENV-unset}" >> "%s/env.log"\nexec %s "$@"\n' "$tmp" "$real" > "$tmp/envshim/$c"; chmod +x "$tmp/envshim/$c"; done
: > "$tmp/envlog_probe"; : > "$tmp/benv.sh"
out=$(PATH=$tmp/envshim:$PATH LD_LIBRARY_PATH=/nonexistent/ld PYTHONPATH=$FAKEV PYTHONHOME=/nonexistent/ph PYTHONSTARTUP=$tmp/benv.sh BASH_ENV=$tmp/benv.sh ENV=$tmp/benv.sh bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "run with hostile LD_*, PYTHON*, BASH_ENV, ENV set is still accepted (I-2)" 0 ""
n_env=$(wc -l < "$tmp/env.log" 2>/dev/null); n_dirty=$(grep -c -v 'LP=unset LL=unset PP=unset PH=unset PS=unset BE=unset EN=unset' "$tmp/env.log" 2>/dev/null)
if [ "${n_env:-0}" -gt 0 ] && [ "${n_dirty:-1}" -eq 0 ]; then ok "no child process saw LD_*, PYTHON*, BASH_ENV or ENV ($n_env child starts logged) (I-2)"; else bad "children saw hostile variables: starts=${n_env:-0} dirty=${n_dirty:-?} first: $(grep -m1 -v 'LP=unset LL=unset PP=unset PH=unset PS=unset BE=unset EN=unset' "$tmp/env.log" 2>/dev/null)"; fi
if [ -n "${LD_PRELOAD:-}" ]; then
  : > "$tmp/env.log"; out=$(PATH=$tmp/envshim:$PATH bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
  if [ $rc -eq 0 ] && ! grep -q 'LP=[^u]' "$tmp/env.log" && grep -q 'LP=unset' "$tmp/env.log"; then ok "the ambient LD_PRELOAD of this host is not passed to children (I-2)"; else bad "ambient LD_PRELOAD reached a child: rc=$rc $(head -1 "$tmp/env.log")"; fi
fi
# E2: exported shell functions shadowing jq / python3 (accept everything) must not forge an accept
mkverdict fz claude-opus-5 xhigh; mkprov fz "claude-sonnet-5-5" xhigh xhigh
out=$(bash -c 'jq() { return 0; }; python3() { return 0; }; export -f jq python3; exec bash "$@"' _ "$SCRIPT" "$tmp/fz.json" 2>&1); rc=$?
check "exported jq/python3 functions that accept everything cannot forge an accept (I-2 E2)" 20 model_not_opus
out=$(bash -c 'jq() { return 0; }; python3() { return 0; }; export -f jq python3; exec bash "$@"' _ "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "golden-true: the same exported functions with a valid verdict are scrubbed and the verdict accepted (I-2 E2)" 0 ""
# E2b (round 6): the jq wrapper now masks an exported jq function, so the python3 stage needs its own witness: an exported accept-all python3
# function must not skip the strict-JSON / schema layers (a schema-invalid verdict must still be refused)
out=$(bash -c 'python3() { return 0; }; export -f python3; exec bash "$@"' _ "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "an exported accept-all python3 function cannot skip the schema layer: the schema-invalid verdict is still refused (I-2 E2b)" 20 verdict_schema_invalid
# E3: BASH_ENV sourced before line 1 that exports PYTHONPATH and defines a function
printf 'export PYTHONPATH=%s\njq() { return 0; }\n' "$FAKEV" > "$tmp/hostile_env.sh"
out=$(BASH_ENV=$tmp/hostile_env.sh bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "a BASH_ENV that exports PYTHONPATH and defines jq cannot disable the checks (I-2 E3)" 20 verdict_schema_invalid
out=$(IFS=x bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "golden-true: an exported IFS (ignored by bash) does not matter (I-2)" 0 ""

# I-3 / m-1: validator crash is unavailable, never invalid, never accepted (RM2, m-1)
mkdir -p "$tmp/killshim"
printf '#!/bin/sh\n# kill the validator stage only (two file arguments: schema and verdict); strict() is handed one\nn=0; for a in "$@"; do [ -f "$a" ] && n=$((n+1)); done\nif [ $n -ge 2 ]; then kill -9 $$; fi\nexec %s "$@"\n' "$real_py" > "$tmp/killshim/python3"; chmod +x "$tmp/killshim/python3"
out=$(PATH=$tmp/killshim:$PATH bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "validator killed while checking a schema-invalid verdict: refused, not accepted (RM2)" 20 verdict_schema_unavailable
out=$(PATH=$tmp/killshim:$PATH bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "validator killed while checking a VALID verdict: unavailable, not 'invalid' (m-1)" 20 verdict_schema_unavailable
mkdir -p "$tmp/exit1shim"
printf '#!/bin/sh\n# the validator stage dies with status 1, as an uncaught Python exception would\nn=0; for a in "$@"; do [ -f "$a" ] && n=$((n+1)); done\nif [ $n -ge 2 ]; then exit 1; fi\nexec %s "$@"\n' "$real_py" > "$tmp/exit1shim/python3"; chmod +x "$tmp/exit1shim/python3"
out=$(PATH=$tmp/exit1shim:$PATH bash "$SCRIPT" "$tmp/lx.json" 2>&1); rc=$?
check "validator dying with status 1 (an uncaught exception) is unavailable, not a verdict about the document (m-1)" 20 verdict_schema_unavailable
# RM3: CPA_APPROVED_DIR spelled with a trailing slash, doubled slashes or through a symlink is accepted
out=$(CPA_APPROVED_DIR=$tmp/approved/ bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?; check "CPA_APPROVED_DIR with a trailing slash accepted (RM3)" 0 ""
out=$(CPA_APPROVED_DIR=$tmp//approved// bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?; check "CPA_APPROVED_DIR with doubled slashes accepted (RM3)" 0 ""
ln -s "$tmp/approved" "$tmp/approved_link"
out=$(CPA_APPROVED_DIR=$tmp/approved_link bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?; check "CPA_APPROVED_DIR given as a symlink to the approved copy accepted (RM3)" 0 ""
# RM1: the schema listed twice in reviewed_files is not 'exactly once' (either order)
mkverdict bt1 claude-opus-5 xhigh; mkprov bt1 "claude-opus-5" xhigh xhigh; addrf bt1 "$REL" "$SHA_SCH"; addrf bt1 "$REL" "$ONE64"
runb "$tmp/bt1.json"; check "bootstrap: schema listed twice (work-tree sha first) refused (RM1)" 20 bootstrap_schema_not_reviewed
mkverdict bt2 claude-opus-5 xhigh; mkprov bt2 "claude-opus-5" xhigh xhigh; addrf bt2 "$REL" "$ONE64"; addrf bt2 "$REL" "$SHA_SCH"
runb "$tmp/bt2.json"; check "bootstrap: schema listed twice (work-tree sha second) refused (RM1)" 20 bootstrap_schema_not_reviewed
# RM4: an empty-string effort_argument is 'absent', not a mismatch
mkverdict ea claude-opus-5 xhigh; mkprov ea "claude-opus-5" xhigh xhigh; editp ea '.effort_argument=""'
refused "empty-string effort_argument is effort_argument_absent (RM4)" effort_argument_absent ea
# RM5: bootstrap from a subdirectory of the work tree uses the work-tree root, not the cwd
out=$(cd "$repo/scripts" && env -u CPA_APPROVED_DIR bash "$SCRIPT" --bootstrap "$tmp/bs1.json" 2>&1); rc=$?
if [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^OK review provenance (BOOTSTRAP'; then ok "bootstrap from a subdirectory of the work tree accepted (RM5)"; else bad "RM5 rc=$rc out=$out"; fi
# RM7: the documented 300-byte bound on a refusal detail holds
longp="$tmp/$(head -c 3000 /dev/zero | tr '\0' 'p')"
out=$(bash "$SCRIPT" "$tmp/good.json" "$longp" 2>&1); rc=$?; blen=$(printf '%s\n' "$out" | wc -c)
if [ $rc -eq 20 ] && [ "$blen" -le $((34+300+1)) ] && [ "$blen" -gt 300 ] && printf '%s' "$out" | grep -q '^REFUSED reason=provenance_absent'; then ok "refusal naming a 3000-character path is bounded ($blen bytes) (RM7)"; else bad "RM7 rc=$rc bytes=$blen"; fi

# R5 F1: jq auto-loads $HOME/.jq; a definition there must not shadow builtins (test, any) and switch the checks off
mkdir -p "$tmp/hostile_home"
printf 'def test($re): true;\ndef any(g; c): true;\n' > "$tmp/hostile_home/.jq"
mkverdict hj1 claude-sonnet-5-5 xhigh; mkprov hj1 "claude-sonnet-5-5" xhigh xhigh
out=$(HOME=$tmp/hostile_home bash "$SCRIPT" "$tmp/hj1.json" 2>&1); rc=$?
check "hostile \$HOME/.jq shadowing any() cannot make a non-Opus record pass (R5 F1)" 20 model_not_opus
mkverdict hj2 claude-opus-5 xhigh; mkprov hj2 "claude-opus-5" xhigh xhigh 1 "I-typed-this"
out=$(HOME=$tmp/hostile_home bash "$SCRIPT" "$tmp/hj2.json" 2>&1); rc=$?
check "hostile \$HOME/.jq shadowing test() cannot make a malformed run id pass (R5 F1)" 20 workflow_run_id_malformed
out=$(HOME=$tmp/hostile_home bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "golden-true: a valid verdict with the hostile \$HOME/.jq is still accepted (R5 F1)" 0 ""
mkdir -p "$tmp/broken_home"; printf 'def test(: broken\n' > "$tmp/broken_home/.jq"
out=$(HOME=$tmp/broken_home bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "a syntactically broken \$HOME/.jq does not falsely refuse a valid verdict (R5 F1)" 0 ""

# R5 F2: a tool that cannot start is 'tool unusable' (exit 2), never a record defect (exit 20)
mkdir -p "$tmp/ldjq" "$tmp/ldpy"
printf '#!/bin/sh\n[ -n "${LD_LIBRARY_PATH:-}" ] || { echo "error while loading shared libraries" >&2; exit 127; }\nexec %s "$@"\n' "$real_jq" > "$tmp/ldjq/jq"; chmod +x "$tmp/ldjq/jq"
printf '#!/bin/sh\n[ -n "${LD_LIBRARY_PATH:-}" ] || { echo "error while loading shared libraries" >&2; exit 127; }\nexec %s "$@"\n' "$real_py" > "$tmp/ldpy/python3"; chmod +x "$tmp/ldpy/python3"
out=$(PATH=$tmp/ldjq:$PATH LD_LIBRARY_PATH=/x bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "jq that cannot start after the LD_* scrub is exit 2, not a record refusal (R5 F2)" 2 ""
if printf '%s' "$out" | grep -q 'jq' && printf '%s' "$out" | grep -qi 'unusable'; then ok "jq-unusable message names the tool (R5 F2)"; else bad "jq-unusable message: $out"; fi
out=$(PATH=$tmp/ldpy:$PATH LD_LIBRARY_PATH=/x bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "python3 that cannot start after the LD_* scrub is exit 2, not a record refusal (R5 F2)" 2 ""
if printf '%s' "$out" | grep -q 'python3' && printf '%s' "$out" | grep -qi 'unusable'; then ok "python3-unusable message names the tool (R5 F2)"; else bad "python3-unusable message: $out"; fi

# R6 PR-1: the private HOME must be the 0700 work dir itself, never a shared parent: a hostile .jq in $TMPDIR (the directory ABOVE the work dir) must not load
mkdir -p "$tmp/hostile_tmpdir"; cp "$tmp/hostile_home/.jq" "$tmp/hostile_tmpdir/.jq"
out=$(TMPDIR=$tmp/hostile_tmpdir bash "$SCRIPT" "$tmp/hj1.json" 2>&1); rc=$?
check "a hostile .jq in TMPDIR (the dir above the private work dir) cannot make a non-Opus record pass (R6 PR-1)" 20 model_not_opus
out=$(TMPDIR=$tmp/hostile_tmpdir bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
check "golden-true: a valid verdict with the hostile .jq in TMPDIR is still accepted (R6 PR-1)" 0 ""
# the HOME every jq call runs with is ONE private mktemp directory under TMPDIR: not TMPDIR, not the caller's HOME
mkdir -p "$tmp/homespy" "$tmp/homespy_tmp"
printf '#!/bin/sh\nprintf "%%s\\n" "$HOME" >> "%s/home.log"\nexec %s "$@"\n' "$tmp/homespy" "$real_jq" > "$tmp/homespy/jq"; chmod +x "$tmp/homespy/jq"
: > "$tmp/homespy/home.log"
out=$(PATH=$tmp/homespy:$PATH TMPDIR=$tmp/homespy_tmp HOME=$tmp/hostile_home bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
nh=$(sort -u "$tmp/homespy/home.log" | wc -l); h1=$(sort -u "$tmp/homespy/home.log" | head -1)
if [ $rc -eq 0 ] && [ "$nh" = 1 ] && [ -n "$h1" ] && [ "$h1" != "$tmp/homespy_tmp" ] && [ "$h1" != "$tmp/hostile_home" ] && [ "$(dirname "$h1")" = "$tmp/homespy_tmp" ]; then ok "every jq call runs with the SAME private work dir as HOME (a child of TMPDIR, neither TMPDIR nor the caller HOME) (R6 PR-1)"; else bad "R6 PR-1 jq HOME: rc=$rc distinct=$nh home='$h1'"; fi

# R6 PR-2: the python3 smoke test runs isolated (-I) like the real validator: a user-site usercustomize that exits must not read as 'python3 unusable'
mkdir -p "$tmp/ush"; ush=$(HOME=$tmp/ush python3 -c 'import site; print(site.getusersitepackages())' 2>/dev/null)
if [ -n "$ush" ]; then
  mkdir -p "$ush"; printf 'raise SystemExit(7)\n' > "$ush/usercustomize.py"
  HOME=$tmp/ush python3 -c '' >/dev/null 2>&1; urc=$?
  if [ "$urc" != 0 ]; then
    ok "control: a plain python3 with this HOME fails (rc=$urc) through usercustomize (the instrument sees the user site) (R6 PR-2)"
    out=$(HOME=$tmp/ush bash "$SCRIPT" "$tmp/good.json" 2>&1); rc=$?
    check "a user-site usercustomize that exits is NOT 'python3 unusable': the isolated smoke test and validator ignore it (R6 PR-2)" 0 ""
  else bad "R6 PR-2 control: python3 ignored the user-site fixture (rc=$urc), the isolation fixture cannot discriminate"; fi
else bad "R6 PR-2: python3 reported no user site directory"; fi

# ---- doc checks (text only; failures ARE counted in RESULT fail, passes are reported on the DOCCHECKS line only)
doc=$here/../../../docs/scripts/check_review_provenance.md
dpass=0; dfail=0
dchk() { if grep -q -- "$2" "$1"; then dpass=$((dpass+1)); else dfail=$((dfail+1)); echo "FAIL doc check: '$2' absent from $1"; fi; }
dchk "$SCRIPT" 'HONEST BOUNDARY'; dchk "$SCRIPT" 'EFFORT IS THE REQUESTED ARGUMENT'; dchk "$SCRIPT" 'CPA_APPROVED_DIR'
if [ -f "$doc" ] && [ -z "${CRP_SCRIPT:-}" ]; then
  dchk "$doc" 'requested, never observed'; dchk "$doc" 'CPA_APPROVED_DIR'; dchk "$doc" '| Revision |'; dchk "$doc" '| Last modified |'
  dchk "$doc" '--bootstrap'; dchk "$doc" 'T046a'; dchk "$doc" 'T094c'; dchk "$doc" 'scrubbed environment'
  dchk "$doc" 'tool unusable'; dchk "$doc" 'python3 -I'; dchk "$doc" 'allow-list environment'; dchk "$doc" 'BASH_ENV'; dchk "$doc" 'user site'
fi
echo "DOCCHECKS pass=$dpass fail=$dfail (a doc failure is added to RESULT fail; doc passes are not added to RESULT pass)"
[ $dfail -eq 0 ] || fail=$((fail+dfail))

echo "RESULT pass=$pass fail=$fail"
[ $fail -eq 0 ]
