#!/usr/bin/env bash
# T021 supplementary test (TDD) for scripts/audit/index_health.sh: cases the T016 test does not isolate (it cannot tell a missing
# positive needle from a missing tracked file) plus P3, option hygiene and the honest SKIP/FAIL reasons.
# Usage: bash scripts/audit/tests/test_index_health_extra.sh      Env: IH=<script under test>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
IH="${IH:-scripts/audit/index_health.sh}"
T="$(mktemp -d "${TMPDIR:-/tmp}/ih_x.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
POS="a/main.go"; NEG="zz/none.go"
printf '%s\n' "$POS" "a/b.go" > "$T/tracked.txt"
cat > "$T/st.json" <<'J'
{"index":{"state":"complete","pendingRefs":0,"reindexRecommended":false},"pendingChanges":{"added":0,"modified":0,"removed":0},"worktreeMismatch":null}
J
files() { python3 - "$@" <<'PY'
import json, sys
print(json.dumps([{"path": p} for p in sys.argv[1:]]))
PY
}
files "$POS" a/b.go > "$T/f_good.json"
printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: 2026-10-05T10:00:00Z\n' > "$T/lu.txt"
base() {  # base [--opt value]... : defaults below are used for every option the caller does not give (no duplicates)
  local -A d=( [--cg-status]="$T/st.json" [--tracked]="$T/tracked.txt" [--needle-pos]="$POS" [--needle-neg]="$NEG" [--lumen-status]="$T/lu.txt" )
  local -a a=(); local k
  local -A given=(); local i=1
  while [ $i -le $# ]; do k="${!i}"; given[$k]=1; i=$((i+1)); done
  for k in "${!d[@]}"; do [ -n "${given[$k]:-}" ] || a+=("$k" "${d[$k]}"); done
  "$IH" --out "$T/o.json" "${a[@]}" "$@" >"$T/stdout" 2>"$T/stderr"; RC=$?
}
pv() { jq -r --arg id "$1" '([.codegraph.proofs[]?, .lumen.proofs[]?, .parity?] | map(select(.id? == $id)) | .[0].verdict) // "MISSING"' "$T/o.json" 2>/dev/null || echo MISSING; }
reason() { jq -r --arg id "$1" '([.codegraph.proofs[]?, .lumen.proofs[]?, .parity?] | map(select(.id? == $id)) | .[0] | ((.reasons // []) + [.reason // empty]) | join(",")) // ""' "$T/o.json" 2>/dev/null; }
chk() { # chk <name> <want rc> <proof> <want verdict> [reason substring]
  local got; got="$(pv "$3")"
  if [ "$RC" -eq "$2" ] && [ "$got" = "$4" ] && { [ -z "${5:-}" ] || case "$(reason "$3")" in *"$5"*) true;; *) false;; esac; }; then ok "$1"
  else bad "$1: want rc=$2 $3=$4 ${5:-}, got rc=$RC $3=$got reason=[$(reason "$3")]"; fi; }

[ -x "$IH" ] || echo "NOTE: $IH absent or not executable (RED state)"
# X1 positive needle absent from BOTH lists (tracked complete): only the needle can fail P2
printf 'a/b.go\n' > "$T/tr_noposs.txt"; files a/b.go > "$T/f_nopos.json"
"$IH" --out "$T/o.json" --cg-status "$T/st.json" --cg-files "$T/f_nopos.json" --tracked "$T/tr_noposs.txt" --needle-pos "$POS" --needle-neg "$NEG" --lumen-status "$T/lu.txt" >/dev/null 2>&1; RC=$?
chk "X1 positive needle missing while tracked set is complete -> P2 FAIL" 1 P2 FAIL positive_needle_missing
# X2 absent required field with the field's own check otherwise satisfiable: pendingChanges removed
jq 'del(.pendingChanges)' "$T/st.json" > "$T/st_nopc.json"
base --cg-files "$T/f_good.json" --cg-status "$T/st_nopc.json"
chk "X2 absent pendingChanges -> P1 FAIL required_field_absent" 1 P1 FAIL required_field_absent
# X3 Chunks: 0 -> P7 FAIL
printf 'Files: 2 | Indexed: 2 | Chunks: 0 | Stale: no\nCaptured: 2026-10-05T10:00:00Z\n' > "$T/lu_c0.txt"
base --cg-files "$T/f_good.json" --lumen-status "$T/lu_c0.txt"; chk "X3 Chunks 0 -> P7 FAIL" 1 P7 FAIL chunks_zero
# X4 Files != Indexed
printf 'Files: 3 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: 2026-10-05T10:00:00Z\n' > "$T/lu_fi.txt"
base --cg-files "$T/f_good.json" --lumen-status "$T/lu_fi.txt"; chk "X4 Files != Indexed -> P7 FAIL" 1 P7 FAIL files_ne_indexed
# X5 status without Captured line -> P7 FAIL (cannot date the evidence)
printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\n' > "$T/lu_nocap.txt"
base --cg-files "$T/f_good.json" --lumen-status "$T/lu_nocap.txt"; chk "X5 no Captured line -> P7 FAIL" 1 P7 FAIL status_field_absent
# X6 secret-class file indexed -> P3 FAIL
files "$POS" a/b.go a/.env.distributed > "$T/f_sec.json"; printf '%s\n' "$POS" a/b.go a/.env.distributed > "$T/tr_sec.txt"
base --cg-files "$T/f_sec.json" --tracked "$T/tr_sec.txt"; chk "X6 .env.distributed indexed -> P3 FAIL" 1 P3 FAIL secret_class_files_indexed
# X7 .env.example is allowed
files "$POS" a/b.go .env.example > "$T/f_ex.json"; printf '%s\n' "$POS" a/b.go .env.example > "$T/tr_ex.txt"
base --cg-files "$T/f_ex.json" --tracked "$T/tr_ex.txt"; chk "X7 .env.example allowed -> P3 PASS (false-positive guard)" 0 P3 PASS
# X8 third-party root hit -> P3 FAIL ; X9 own-org root without file -> P3 FAIL
printf 'a/b.go\n' > "$T/tp.txt"; base --cg-files "$T/f_good.json" --third-party-roots "$T/tp.txt"
chk "X8 file under a third-party root -> P3 FAIL" 1 P3 FAIL third_party_files_indexed
printf 'own/x\n' > "$T/own.txt"; base --cg-files "$T/f_good.json" --own-org-roots "$T/own.txt"
chk "X9 own-org root with no indexed file -> P3 FAIL" 1 P3 FAIL own_org_root_without_indexed_file
# X10 tracked file not indexed -> P2 FAIL missing list
files "$POS" > "$T/f_miss.json"; base --cg-files "$T/f_miss.json"; chk "X10 tracked file not indexed -> P2 FAIL" 1 P2 FAIL tracked_files_not_indexed
# X11 existing lumen-bin and no status: still FAIL (probe refused, it would write the index)
"$IH" --out "$T/o.json" --cg-status "$T/st.json" --cg-files "$T/f_good.json" --tracked "$T/tracked.txt" --needle-pos "$POS" --needle-neg "$NEG" --lumen-bin /bin/true >/dev/null 2>&1; RC=$?
chk "X11 executable --lumen-bin without a status capture -> P-Lumen FAIL (probe refused)" 1 P-Lumen FAIL probe_not_run
# X12 no --lumen-files: P8 is an honest SKIP with a reason, verdict still PASS and complete=false
base --cg-files "$T/f_good.json"; chk "X12 no --lumen-files -> P8 SKIP" 0 P8 SKIP no_lumen_files
[ "$(jq -r '.complete' "$T/o.json")" = false ] && ok "X12b skipped proofs make complete=false" || bad "X12b complete flag"
# X13 option hygiene: option-like value, unknown option, duplicate option, bad tolerance -> rc 2
"$IH" --out "$T/o.json" --cg-status --cg-files >/dev/null 2>&1; [ $? -eq 2 ] && ok "X13a option-like value -> rc 2" || bad "X13a"
base --bogus x; [ "$RC" -eq 2 ] && ok "X13b unknown option -> rc 2" || bad "X13b rc=$RC"
base --cg-files "$T/f_good.json" --cg-files "$T/f_good.json"; [ "$RC" -eq 2 ] && ok "X13c duplicate option -> rc 2" || bad "X13c rc=$RC"
base --cg-files "$T/f_good.json" --parity-tolerance-pct -1; [ "$RC" -eq 2 ] && ok "X13d negative tolerance -> rc 2" || bad "X13d rc=$RC"
# X14 unreadable --cg-status is a FAIL of P1 (could not look), not a crash
base --cg-files "$T/f_good.json" --cg-status "$T/none.json"; chk "X14 unreadable status file -> P1 FAIL" 1 P1 FAIL status_file_unreadable
# X15 determinism: two runs on identical inputs write byte-identical JSON
base --cg-files "$T/f_good.json"; cp "$T/o.json" "$T/o1.json"; base --cg-files "$T/f_good.json"
cmp -s "$T/o.json" "$T/o1.json" && ok "X15 identical inputs -> byte-identical output" || bad "X15 output differs between runs"
# X16 evidence identity: the input hash recorded equals sha256 of the file judged
want="$(sha256sum "$T/f_good.json" | cut -d' ' -f1)"; got="$(jq -r '.inputs.cg_files.sha256' "$T/o.json")"
[ "$want" = "$got" ] && ok "X16 inputs.cg_files.sha256 equals the sha256 of the file" || bad "X16 want $want got $got"
# ---- round 5 (WF5 review R4, M5-2, M5-3) -----------------------------------------------------------------------------
# R4: a third-party root line that is not canonical (CRLF, trailing space, ./ prefix, leading /, //, control char, .. component) must
# FAIL P3 (fail closed), never silently match nothing; a canonical root (also with one trailing slash) still works both ways
for v in 'a\r' 'a ' ' a' './a' '/a' 'a//b' 'a/./b' 'a/../b' 'a\001'; do
  printf "$v\n" > "$T/tp_bad.txt"; base --cg-files "$T/f_good.json" --third-party-roots "$T/tp_bad.txt"
  chk "R4 non-canonical third-party root line [$(printf '%s' "$v" | od -An -c | tr -s ' ' | tr -d '\n')] -> P3 FAIL third_party_root_not_canonical" 1 P3 FAIL third_party_root_not_canonical
done
printf 'zz\n' > "$T/tp_ok.txt"; base --cg-files "$T/f_good.json" --third-party-roots "$T/tp_ok.txt"
chk "R4 control: a canonical third-party root with no indexed file -> P3 PASS (false-positive guard)" 0 P3 PASS
printf 'a/\n' > "$T/tp_slash.txt"; base --cg-files "$T/f_good.json" --third-party-roots "$T/tp_slash.txt"
chk "R4 control: a canonical root with ONE trailing slash that holds an indexed file -> P3 FAIL third_party_files_indexed" 1 P3 FAIL third_party_files_indexed
printf '\n   \nzz\n' > "$T/tp_blank.txt"; base --cg-files "$T/f_good.json" --third-party-roots "$T/tp_blank.txt"
chk "R4 control: blank lines in the roots file are ignored -> P3 PASS" 0 P3 PASS
# M5-2: an empty tracked list is a blind completeness check, a P2 FAIL
: > "$T/tr_empty.txt"; base --cg-files "$T/f_good.json" --tracked "$T/tr_empty.txt"
chk "M5-2 empty --tracked list -> P2 FAIL tracked_list_empty_blind" 1 P2 FAIL tracked_list_empty_blind
# M5-3: a CodeGraph status whose state is not complete is a P1 FAIL on that reason alone
jq '.index.state="indexing"' "$T/st.json" > "$T/st_idx.json"; base --cg-files "$T/f_good.json" --cg-status "$T/st_idx.json"
chk "M5-3 index.state indexing (everything else clean) -> P1 FAIL index.state_not_complete" 1 P1 FAIL index.state_not_complete
# ---- round 6 (WF6 N6-5, MR4): the P3 encoding class. A third-party root that can never match an indexed path for ENCODING reasons (a byte order
# mark, a zero-width or no-break space, an invalid UTF-8 byte, a backslash separator, a decomposed (NFD) spelling, DEL) must FAIL P3, never PASS.
mkdir -p "$T/p3enc"
files "a/main.go" "a/b.go" "café/x.go" > "$T/f_enc.json"
files "$(printf 'caf\303\251/x.go')" a/b.go a/main.go > "$T/f_enc.json"
for v in '\357\273\277a' 'a\342\200\213' 'a\377' 'a\\b' 'cafe\314\201' 'a\177' 'a\302\240' '\342\200\213a' 'a\302\240b' 'a\342\200\250b'; do
  printf "$v\n" > "$T/tp_enc.txt"; base --cg-files "$T/f_enc.json" --third-party-roots "$T/tp_enc.txt"
  chk "N6-5 encoding-class third-party root [$(printf "$v" | od -An -c | tr -s ' ' | tr -d '\n')] -> P3 FAIL third_party_root_not_canonical" 1 P3 FAIL third_party_root_not_canonical
done
printf 'caf\303\251\n' > "$T/tp_nfc.txt"; base --cg-files "$T/f_enc.json" --third-party-roots "$T/tp_nfc.txt"
chk "N6-5 golden-false: a non-ASCII root in NFC that holds an indexed file is a normal hit -> P3 FAIL third_party_files_indexed" 1 P3 FAIL third_party_files_indexed
printf 'na\303\257ve\n' > "$T/tp_nfc2.txt"; base --cg-files "$T/f_enc.json" --third-party-roots "$T/tp_nfc2.txt"
chk "N6-5 golden-false: a non-ASCII NFC root with no indexed file -> P3 PASS" 0 P3 PASS
printf 'a b\n' > "$T/tp_sp.txt"; base --cg-files "$T/f_enc.json" --third-party-roots "$T/tp_sp.txt"
chk "N6-5 golden-false: an inner ASCII space is a plain character -> P3 PASS" 0 P3 PASS
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
