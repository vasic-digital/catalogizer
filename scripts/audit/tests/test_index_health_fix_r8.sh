#!/usr/bin/env bash
# WP-02 fix round 8 test (TDD, constitution 11.4.224 / 11.4.276 structural round): every proof verdict is DERIVED from its evidence.
# Reviews WP-02-T028 / -r2 / -r3. Design: specs/001-full-project-audit-remediation/evidence/wp02/fix-r8-design.md (section numbers cited below).
#
# Purpose   L = live-proof evidence (R3-1, design 4/5), D = drift report binding (R3-2), C = P3 classes at every depth (R3-3, R3-4, R3-7),
#           K = coverage-probe depth (reviewer mutants R1..R4 made non-equivalent), W = scope-window and policy rules.
#           Every case drives scripts/audit/index_health.sh through its real command line against a scratch git repository.
# Usage     bash scripts/audit/tests/test_index_health_fix_r8.sh      Env: IH=<script under test> (default scripts/audit/index_health.sh)
# Exit      0 all ok; 1 a case failed; 77 no case failed but a leg was skipped (SKIP is never a pass)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
RR="$PWD"
IH="${IH:-$RR/scripts/audit/index_health.sh}"
SCOPE="${SCOPE:-$RR/config/index/scope.yaml}"
MEMBERS="$RR/scripts/audit/tests/fixtures/baseline_members.tsv"
T="$(mktemp -d "${TMPDIR:-/tmp}/ih_r8.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0; SKIPN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
skp() { SKIPN=$((SKIPN+1)); echo "SKIP $1 (reason recorded; not a pass)"; }
ago() { date -u -d "$1 ago" +%Y-%m-%dT%H:%M:%SZ; }
ahead() { date -u -d "$1" +%Y-%m-%dT%H:%M:%SZ; }
CAP="$(ago '2 hours')"; CNB="$(ago '1 day')"
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
printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: %s\n' "$CAP" > "$T/lu.txt"
printf '%s\n' "$POS" a/b.go > "$T/lumen_files.txt"

# ---- the scratch repository, fixture programs, policy -----------------------------------------------------------------
R="$T/repo"; mkdir -p "$R/ev" "$T/bin" "$T/evil"; export IH_SHIM_REPO="$R"
git -C "$R" init -q . && git -C "$R" config user.email t@t && git -C "$R" config user.name t && git -C "$R" config commit.gpgsign false
for p in codegraph lumen_verify.sh lumen; do printf '#!/bin/sh\nexit 0\n' > "$T/bin/$p"; chmod +x "$T/bin/$p"; done
printf '#!/bin/sh\nexit 0\n' > "$T/evil/codegraph"; chmod +x "$T/evil/codegraph"
printf 'not executable\n' > "$T/bin/noexec"; : > "$T/bin/codegraph.dir.keep"
POLICY="$T/policy.yaml"
cat > "$POLICY" <<EOF
schema: live-policy/1
max_age_hours: 72
clock_skew_seconds: 300
programs:
  P4: ["$T/bin/codegraph", "$T/bin/lumen_verify.sh"]
  P5: ["$T/bin/codegraph", "/usr/bin/git"]
  P6: ["$T/bin/lumen", "$T/bin/lumen_verify.sh"]
fallback_programs: ["/usr/bin/grep"]
thresholds: {p4_min_queries: 4, p4_min_recall: 1.0, p6_expected_dims: 1024}
EOF
SCSHA="$(sha256sum "$SCOPE" | cut -d' ' -f1)"

# evidence writer, test side (shares nothing with the tool): evw <proof> <outfile> <program> [k=v | -k | +q=row | !q]...
cat > "$T/evw.py" <<'PY'
import sys
proof, out, prog = sys.argv[1:4]
ov = sys.argv[4:]
import os
scope_sha, cap = os.environ["EV_SCOPE_SHA"], os.environ["EV_CAP"]
L = [("schema", "live-evidence/1"), ("proof", proof), ("program", prog), ("argv", "--golden g.json --k 5"), ("exit", "0"),
     ("captured_at", cap), ("scope_sha256", scope_sha)]
if proof == "P4":
    L += [("q", "id=G-CG-1 expect=answerable result=correct"), ("q", "id=G-CG-2 expect=answerable result=correct"),
          ("q", "id=G-CG-3 expect=answerable result=correct"), ("q", "id=G-CG-4 expect=answerable result=correct"),
          ("q", "id=G-CG-N expect=unanswerable result=refused"), ("queries", "5"), ("correct", "4"), ("recall", "1.0")]
elif proof == "P5":
    L += [("last_indexed", os.environ["EV_IDX"]), ("last_commit", os.environ["EV_COMMIT"])]
elif proof == "P6":
    L += [("embed_dims", "1024"), ("embed_vectors", "3"), ("embed_norm", "0.9987")]
for o in ov:
    if o.startswith("!"):
        L = [kv for kv in L if kv[0] != o[1:]]
    elif o.startswith("-"):
        L = [kv for kv in L if kv[0] != o[1:]]
    elif o.startswith("+"):
        k, _, v = o[1:].partition("=")
        L.append((k, v))
    else:
        k, _, v = o.partition("=")
        for i, kv in enumerate(L):
            if kv[0] == k:
                L[i] = (k, v)
                break
        else:
            L.append((k, v))
open(out, "w").write("# live-evidence written by the TEST harness\n" + "".join("%s: %s\n" % kv for kv in L))
PY
export EV_SCOPE_SHA="$SCSHA" EV_CAP="$CAP" EV_IDX="$(ago '1 hour')" EV_COMMIT="$(ago '5 hours')"
evw() { python3 "$T/evw.py" "$@"; }
seal() {  # seal: ledger of every file under ev/ except SHA256SUMS and drift reports named in NOLEDGER, then commit
  ( cd "$R/ev" && : > SHA256SUMS && for f in $(ls | grep -v '^SHA256SUMS$' | grep -vE "${NOLEDGER:-^$}"); do sha256sum "$f" >> SHA256SUMS; done )
  git -C "$R" add -A >/dev/null && git -C "$R" commit -q -m seal --allow-empty
}
entry() {  # entry <proof> <jq filter applied to the entry>: the live-proofs entry that names ev/<proof>.txt
  local p="$1" f="${2:-.}" prog cmd argv
  prog="$(sed -n 's/^program: //p' "$R/ev/$p.txt" | head -1)"; argv="$(sed -n 's/^argv: //p' "$R/ev/$p.txt" | head -1)"
  jq -nc --arg c "$prog $argv" --arg p "ev/$p.txt" --arg h "$(sha256sum "$R/ev/$p.txt" | cut -d' ' -f1)" --arg t "$CAP" \
     '{cmd:$c,evidence_path:$p,evidence_sha256:$h,captured_at:$t,kind:"live"}' | jq -c "$f"
}
P4PROG="$T/bin/codegraph"; P5PROG="$T/bin/codegraph"; P6PROG="$T/bin/lumen"
setup() {  # setup: three good evidence files, sealed, and a live.json that names them
  rm -f "$R"/ev/*; evw P4 "$R/ev/P4.txt" "$P4PROG"; evw P5 "$R/ev/P5.txt" "$P5PROG"; evw P6 "$R/ev/P6.txt" "$P6PROG"; seal; mklive
}
mklive() { jq -nc --argjson a "$(entry P4 "${1:-.}")" --argjson b "$(entry P5 "${2:-.}")" --argjson c "$(entry P6 "${3:-.}")" '{P4:$a,P5:$b,P6:$c}' > "$T/live.json"; }
base() {  # base [--opt value]...
  local -A d=( [--scope]="$SCOPE" [--cg-status]="$T/st.json" [--cg-files]="$T/f_good.json" [--tracked]="$T/tracked.txt" [--needle-pos]="$POS" [--needle-neg]="$NEG" [--lumen-status]="$T/lu.txt" [--captured-not-before]="$CNB" [--lumen-files]="$T/lumen_files.txt" [--live-proofs]="$T/live.json" [--live-policy]="$POLICY" [--evidence-ledger]="$R/ev/SHA256SUMS" [--repo]="$R" )
  local -a a=(); local -A g=(); local i=1 k
  while [ $i -le $# ]; do k="${!i}"; g[$k]=1; i=$((i+1)); done
  for k in "${!d[@]}"; do [ -n "${g[$k]:-}" ] || a+=("$k" "${d[$k]}"); done
  "$IH" --out "$T/o.json" "${a[@]}" "$@" >"$T/stdout" 2>"$T/stderr"; RC=$?
}
pv() { jq -r --arg id "$1" '([.codegraph.proofs[]?, .lumen.proofs[]?, .parity?] | map(select(.id? == $id)) | .[0].verdict) // "MISSING"' "$T/o.json" 2>/dev/null || echo MISSING; }
reason() { jq -r --arg id "$1" '([.codegraph.proofs[]?, .lumen.proofs[]?, .parity?] | map(select(.id? == $id)) | .[0] | ((.reasons // []) + [.reason // empty] + (.forbidden_class_problems // [])) | join(",")) // ""' "$T/o.json" 2>/dev/null; }
chk() { # chk <name> <want rc> <proof> <want verdict> [reason substring]
  local got; got="$(pv "$3")"
  if [ "$RC" -eq "$2" ] && [ "$got" = "$4" ] && { [ -z "${5:-}" ] || case "$(reason "$3")" in *"$5"*) true;; *) false;; esac; }; then ok "$1"
  else bad "$1: want rc=$2 $3=$4 ${5:-}, got rc=$RC $3=$got reason=[$(reason "$3")] stderr=[$(head -c 160 "$T/stderr" | tr '\n' ' ')]"; fi; }
chkp() { # chkp <name> <proof> <want verdict> [reason substring]: proof verdict only (other proofs may be SKIP/FAIL for lack of inputs)
  local got; got="$(pv "$2")"
  if [ "$got" = "$3" ] && { [ -z "${4:-}" ] || case "$(reason "$2")" in *"$4"*) true;; *) false;; esac; }; then ok "$1"
  else bad "$1: want $2=$3 ${4:-}, got $2=$got reason=[$(reason "$2")] rc=$RC stderr=[$(head -c 160 "$T/stderr" | tr '\n' ' ')]"; fi; }
chkrc() { [ "$RC" -eq "$2" ] && { [ -z "${3:-}" ] || grep -q -- "$3" "$T/stderr"; } && ok "$1" || bad "$1: want rc=$2 stderr~[${3:-}], got rc=$RC [$(head -c 200 "$T/stderr" | tr '\n' ' ')]"; }
[ -x "$IH" ] || echo "NOTE: $IH absent or not executable (RED state)"

# ---- L: live proofs: verdict = f(evidence) ----------------------------------------------------------------------------
setup; base
{ [ "$RC" -eq 3 ] || [ "$RC" -eq 0 ]; } && [ "$(pv P4)" = PASS ] && [ "$(pv P5)" = PASS ] && [ "$(pv P6)" = PASS ] && ok "L00 golden-false: three consistent ledger-bound evidence files -> P4 P5 P6 PASS" || bad "L00 golden-false: rc=$RC P4=$(pv P4)/$(reason P4) P5=$(pv P5)/$(reason P5) P6=$(pv P6)/$(reason P6)"
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '!q' '+q=id=G-CG-1 expect=answerable result=correct' '+q=id=G-CG-2 expect=answerable result=correct' '+q=id=G-CG-3 expect=answerable result=correct' '+q=id=G-CG-4 expect=answerable result=wrong' '+q=id=G-CG-N expect=unanswerable result=refused' 'correct=3' 'recall=0.75'; seal; mklive . . .; base
chk "L01 consistent evidence whose derived recall is 0.75 (< 1.0) -> P4 FAIL (derived, not the entry's word)" 1 P4 FAIL p4_recall_below_threshold
# the reviewer's repro R3-1: evidence reads recall 0.10, entry says PASS
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '!q' '+q=id=G-1 expect=answerable result=correct' '+q=id=G-2 expect=answerable result=wrong' '+q=id=G-3 expect=answerable result=wrong' '+q=id=G-4 expect=answerable result=wrong' '+q=id=G-5 expect=answerable result=wrong' '+q=id=G-6 expect=answerable result=wrong' '+q=id=G-7 expect=answerable result=wrong' '+q=id=G-8 expect=answerable result=wrong' '+q=id=G-9 expect=answerable result=wrong' '+q=id=G-10 expect=answerable result=wrong' '+q=id=G-N expect=unanswerable result=refused' 'queries=11' 'correct=1' 'recall=0.10'; seal; mklive '. + {verdict:"PASS"}' . .; base
chk "L02 evidence 'recall 0.10', entry verdict PASS -> P4 FAIL (R3-1 repro)" 1 P4 FAIL p4_recall_below_threshold
setup; mklive '. + {verdict:"FAIL"}' . .; base
chk "L02b entry verdict FAIL over good evidence -> P4 FAIL entry_verdict_disagrees_with_evidence" 1 P4 FAIL entry_verdict_disagrees_with_evidence
setup; mklive '. + {verdict:"PASS"}' . .; base
chk "L02c control: entry verdict PASS equal to the derived verdict -> P4 PASS" 3 P4 PASS
setup; mklive '.kind="guess"' . .; base; chk "L03 kind outside {live,fallback} -> P4 FAIL" 1 P4 FAIL live_proof_entry_malformed
setup; evw P4 "$R/ev/P4.txt" /usr/bin/grep; seal; mklive '.kind="fallback"' . .; base; chk "L04 kind fallback (grep evidence, even a perfect one) derives SKIP, never a live PASS" 3 P4 SKIP fallback
setup; mklive '.kind="fallback"' . .; base; chk "L04b fallback evidence whose program is off the fallback allow-list -> P4 FAIL program_not_allowed" 1 P4 FAIL program_not_allowed
setup; mklive '.cmd="/usr/bin/true --trust-me"' . .; base; chk "L05 entry cmd that is not 'program argv' of the evidence -> P4 FAIL" 1 P4 FAIL evidence_cmd_mismatch
setup; git -C "$R" rm -q --cached ev/P4.txt; git -C "$R" commit -q -m untrack; base
chk "L06 evidence file not tracked in the repository -> P4 FAIL (hand-written, nobody committed it)" 1 P4 FAIL evidence_not_tracked
setup; printf '# edited after commit\n' >> "$R/ev/P4.txt"; mklive; base
chk "L06b evidence file modified after its commit -> P4 FAIL" 1 P4 FAIL evidence_not_tracked
git -C "$R" checkout -q -- ev/P4.txt
setup; NOLEDGER='^P4.txt$' seal; mklive; base
chk "L07 evidence tracked but absent from the ledger -> P4 FAIL" 1 P4 FAIL evidence_not_in_ledger
setup; sed -i "s/^[0-9a-f]\{64\}  P4.txt/$(printf 'a%.0s' $(seq 64))  P4.txt/" "$R/ev/SHA256SUMS"; git -C "$R" commit -qam badledger; base
chk "L08 ledger line with another sha256 than the evidence bytes -> P4 FAIL" 1 P4 FAIL ledger_hash_mismatch
setup; printf '\n' >> "$R/ev/SHA256SUMS"; base
chk "L08b ledger file modified after its commit -> FAIL ledger_not_tracked" 1 P4 FAIL ledger_not_tracked
git -C "$R" checkout -q -- ev/SHA256SUMS
setup; mklive '.evidence_sha256=("0"*64)' . .; base; chk "L09 evidence_sha256 = 64 zeros -> P4 FAIL" 1 P4 FAIL evidence_hash_zero
setup; mklive '.evidence_sha256=("b"*64)' . .; base; chk "L10 evidence_sha256 differing from the bytes -> P4 FAIL" 1 P4 FAIL evidence_hash_mismatch
setup; mklive '.captured_at="2000-01-01T00:00:00Z"' . .; base; chk "L11 entry captured_at differing from the evidence line -> P4 FAIL" 1 P4 FAIL captured_at_mismatch
OLDCAP="$(ago '10 days')"; setup; EV_CAP="$OLDCAP" evw P4 "$R/ev/P4.txt" "$P4PROG"; seal; mklive '.captured_at="'"$OLDCAP"'"' . .; base
chk "L12 evidence captured 10 days ago (older than max_age_hours and --captured-not-before) -> P4 FAIL" 1 P4 FAIL live_proof_stale
TWO="$(ago '2 days')"; setup; EV_CAP="$TWO" evw P4 "$R/ev/P4.txt" "$P4PROG"; seal; mklive '.captured_at="'"$TWO"'"' . .; base
chk "L12b evidence captured 2 days ago with --captured-not-before 1 day ago -> P4 FAIL live_proof_stale (the caller's bound is applied)" 1 P4 FAIL live_proof_stale
FUT="$(ahead '2 days')"; setup; EV_CAP="$FUT" evw P4 "$R/ev/P4.txt" "$P4PROG"; seal; mklive '.captured_at="'"$FUT"'"' . .; base
chk "L13 captured_at 2099-style future date -> P4 FAIL captured_in_future (R3-1 repro)" 1 P4 FAIL captured_in_future
F99="2099-01-01T00:00:00Z"; setup; EV_CAP="$F99" evw P6 "$R/ev/P6.txt" "$P6PROG"; seal; mklive . . '.captured_at="'"$F99"'"'; base
chk "L13b year 2099 on P6 -> P6 FAIL captured_in_future" 1 P6 FAIL captured_in_future
setup; EV_CAP="yesterday-ish" evw P4 "$R/ev/P4.txt" "$P4PROG"; seal; mklive '.captured_at="yesterday-ish"' . .; base; chk "L14 unparseable captured_at -> P4 FAIL" 1 P4 FAIL captured_at_unparseable
# evidence header fields
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'schema=live-evidence/9'; seal; mklive; base; chk "L15 wrong evidence schema -> P4 FAIL" 1 P4 FAIL evidence_schema_invalid
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'proof=P5'; seal; mklive; base; chk "L16 evidence for another proof entered in the P4 slot -> P4 FAIL" 1 P4 FAIL evidence_proof_mismatch
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'exit=1'; seal; mklive; base; chk "L17 evidence exit 1 -> P4 FAIL" 1 P4 FAIL evidence_exit_nonzero
setup; evw P4 "$R/ev/P4.txt" "$T/evil/codegraph"; seal; mklive; base; chk "L18 program /tmp/evil/codegraph (basename codegraph, not in the closed absolute allow-list) -> P4 FAIL" 1 P4 FAIL program_not_allowed
setup; evw P4 "$R/ev/P4.txt" "codegraph"; seal; mklive; base; chk "L19 relative program name -> P4 FAIL program_not_absolute" 1 P4 FAIL program_not_absolute
setup; evw P4 "$R/ev/P4.txt" "$T/bin/../evil/codegraph"; seal; mklive; base; chk "L19b dot-dot program path -> P4 FAIL" 1 P4 FAIL program_not_allowed
ln -sf "$T/evil/codegraph" "$T/bin/codegraph_link"
setup; evw P4 "$R/ev/P4.txt" "$T/bin/codegraph_link"; seal; mklive; base; chk "L19c symlink to an off-list program -> P4 FAIL" 1 P4 FAIL program_not_allowed
mv "$T/bin/lumen_verify.sh" "$T/bin/lumen_verify.sh.gone"
setup; evw P4 "$R/ev/P4.txt" "$T/bin/lumen_verify.sh"; seal; mklive; base; chk "L20 allow-listed path that does not exist -> P4 FAIL program_not_executable" 1 P4 FAIL program_not_executable
mv "$T/bin/lumen_verify.sh.gone" "$T/bin/lumen_verify.sh"
setup; evw P5 "$R/ev/P5.txt" "$T/bin/lumen"; seal; mklive; base; chk "L20b program allowed for P6 only (lumen) used for P5 -> P5 FAIL" 1 P5 FAIL program_not_allowed
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '-argv'; seal; mklive; base; chk "L21 evidence without argv -> P4 FAIL evidence_field_missing" 1 P4 FAIL evidence_field_missing
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '+exit=0'; seal; mklive; base; chk "L21b duplicate header key -> P4 FAIL evidence_field_duplicate" 1 P4 FAIL evidence_field_duplicate
printf 'codegraph trust me\n' > "$R/ev/P4.txt"; seal; mklive; base
chk "L21c command-text-only evidence (R3-1 repro A) -> P4 FAIL evidence_schema_invalid" 1 P4 FAIL evidence_schema_invalid
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" "scope_sha256=$(printf 'c%.0s' $(seq 64))"; seal; mklive; base; chk "L22 evidence for another scope (scope_sha256 mismatch) -> P4 FAIL" 1 P4 FAIL evidence_scope_mismatch
# P4 parser fields
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'queries=9'; seal; mklive; base; chk "L23 claimed queries differing from the q rows -> P4 FAIL p4_summary_mismatch" 1 P4 FAIL p4_summary_mismatch
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'correct=5'; seal; mklive; base; chk "L24 claimed correct differing from the q rows -> P4 FAIL p4_summary_mismatch" 1 P4 FAIL p4_summary_mismatch
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'recall=0.5'; seal; mklive; base; chk "L25 claimed recall differing from correct/answerable -> P4 FAIL p4_summary_mismatch" 1 P4 FAIL p4_summary_mismatch
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '!q' '+q=id=G-CG-1 expect=answerable result=correct' '+q=id=G-CG-2 expect=answerable result=correct' '+q=id=G-CG-3 expect=answerable result=correct' '+q=id=G-CG-4 expect=answerable result=correct' '+q=id=G-CG-N expect=unanswerable result=correct' 'queries=5' 'correct=4' 'recall=1.0'; seal; mklive; base
chk "L26 the unanswerable control answered instead of refused -> P4 FAIL p4_control_not_refused" 1 P4 FAIL p4_control_not_refused
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '!q' '+q=id=G-CG-1 expect=answerable result=correct' '+q=id=G-CG-2 expect=answerable result=correct' '+q=id=G-CG-N expect=unanswerable result=refused' 'queries=3' 'correct=2' 'recall=1.0'; seal; mklive; base
chk "L27 only 3 queries (< p4_min_queries 4) -> P4 FAIL p4_too_few_queries" 1 P4 FAIL p4_too_few_queries
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" '!q' '+q=id=G-CG-1 expect=answerable result=correct' '+q=id=G-CG-2 expect=answerable result=correct' '+q=id=G-CG-3 expect=answerable result=correct' '+q=id=G-CG-4 expect=answerable result=correct' 'queries=4' 'correct=4' 'recall=1.0'; seal; mklive; base
chk "L27b no unanswerable control row at all -> P4 FAIL p4_control_not_refused (a control that is absent proves nothing)" 1 P4 FAIL p4_control_not_refused
setup; evw P4 "$R/ev/P4.txt" "$P4PROG" 'q=id=G-CG-1 expect=maybe result=correct'; seal; mklive; base; chk "L27c q row with an unknown expect value -> P4 FAIL p4_row_malformed" 1 P4 FAIL p4_row_malformed
# P5
setup; EV_IDX="$(ago '10 hours')" EV_COMMIT="$(ago '2 hours')" evw P5 "$R/ev/P5.txt" "$P5PROG"; seal; mklive; base; chk "L28 index older than the last commit -> P5 FAIL p5_index_older_than_commit" 1 P5 FAIL p5_index_older_than_commit
setup; EV_COMMIT="$(ahead '3 days')" evw P5 "$R/ev/P5.txt" "$P5PROG"; seal; mklive; base; chk "L29 last_commit in the future -> P5 FAIL p5_commit_in_future" 1 P5 FAIL p5_commit_in_future
setup; evw P5 "$R/ev/P5.txt" "$P5PROG" 'last_indexed=soon'; seal; mklive; base; chk "L30 unparseable last_indexed -> P5 FAIL p5_time_unparseable" 1 P5 FAIL p5_time_unparseable
setup; evw P5 "$R/ev/P5.txt" "$P5PROG" '-last_commit'; seal; mklive; base; chk "L30b missing last_commit -> P5 FAIL evidence_field_missing" 1 P5 FAIL evidence_field_missing
# P6
setup; evw P6 "$R/ev/P6.txt" "$P6PROG" 'embed_dims=768'; seal; mklive; base; chk "L31 embedding of the wrong dimension -> P6 FAIL p6_dims_mismatch" 1 P6 FAIL p6_dims_mismatch
setup; evw P6 "$R/ev/P6.txt" "$P6PROG" 'embed_vectors=0'; seal; mklive; base; chk "L32 zero vectors -> P6 FAIL p6_no_vectors" 1 P6 FAIL p6_no_vectors
setup; evw P6 "$R/ev/P6.txt" "$P6PROG" 'embed_norm=0.0'; seal; mklive; base; chk "L33 zero norm -> P6 FAIL p6_norm_not_positive" 1 P6 FAIL p6_norm_not_positive
setup; evw P6 "$R/ev/P6.txt" "$P6PROG" 'embed_norm=nan'; seal; mklive; base; chk "L33b NaN norm -> P6 FAIL p6_norm_not_positive" 1 P6 FAIL p6_norm_not_positive
setup; evw P6 "$R/ev/P6.txt" "$P6PROG" 'embed_dims=lots'; seal; mklive; base; chk "L33c non-numeric dims -> P6 FAIL p6_field_not_number" 1 P6 FAIL p6_field_not_number
# window and policy
setup; base --captured-not-before "$(ago '30 days')"
chkrc "L34 --captured-not-before older than max_age_hours (a caller cannot widen the window to 1970) -> usage rc=2" 2 "older"
setup; base --captured-not-before "$(ahead '2 days')"
chkrc "L35 --captured-not-before in the future -> usage rc=2" 2 "future"
setup; printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: %s\n' "$(ahead '3 days')" > "$T/lu_fut.txt"; base --lumen-status "$T/lu_fut.txt"
chk "L36 Lumen status Captured in the future -> P7 FAIL captured_in_future" 1 P7 FAIL captured_in_future
printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: %s\n' "$(ago '20 days')" > "$T/lu_old.txt"
setup; base --lumen-status "$T/lu_old.txt" --captured-not-before "$(ago '2 days')"; chk "L36b control: capture older than --captured-not-before -> P7 FAIL" 1 P7 FAIL captured_before_required_time
setup; base --live-policy "$T/no/such.yaml"; chk "W01 unreadable --live-policy -> P4 FAIL live_policy_unusable" 1 P4 FAIL live_policy_unusable
printf 'schema: live-policy/1\n' > "$T/pol_bad.yaml"; setup; base --live-policy "$T/pol_bad.yaml"; chk "W02 policy without programs -> P4 FAIL live_policy_unusable" 1 P4 FAIL live_policy_unusable
setup; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_good.json" --tracked "$T/tracked.txt" --needle-pos "$POS" --needle-neg "$NEG" --captured-not-before "$CNB" --live-proofs "$T/live.json" --repo "$R" >/dev/null 2>"$T/stderr"; RC=$?
chkrc "W03 --live-proofs without --live-policy -> usage rc=2" 2 "live-policy"
setup; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_good.json" --tracked "$T/tracked.txt" --needle-pos "$POS" --needle-neg "$NEG" --captured-not-before "$CNB" --live-proofs "$T/live.json" --live-policy "$POLICY" --repo "$R" >/dev/null 2>"$T/stderr"; RC=$?
chkrc "W04 --live-proofs without --evidence-ledger -> usage rc=2" 2 "evidence-ledger"

setup; base --live-policy "$RR/config/index/live_proof_policy.yaml"
case "$(reason P4)" in *live_policy_unusable*) bad "W05 the shipped config/index/live_proof_policy.yaml is unusable: $(reason P4)" ;; *program_not_allowed*) ok "W05 the shipped policy parses; a fixture program is correctly not on its absolute allow-list" ;; *) bad "W05 shipped policy: P4 reason [$(reason P4)]" ;; esac

# ---- D: the drift report is bound to real values ----------------------------------------------------------------------
printf '/dir/\n' > "$T/li_a.txt"; cp "$T/li_a.txt" "$T/li_b.txt"; printf '/dir/\n/other/\n' > "$T/li_c.txt"
printf '{"allow":["a"],"deny":["b"],"root_files":true}\n' > "$T/lumen_scope.json"
LIS="$(sha256sum "$T/li_a.txt" | cut -d' ' -f1)"; LJS="$(sha256sum "$T/lumen_scope.json" | cut -d' ' -f1)"
mkdrift() {  # mkdrift <jq filter on the report> : head = current HEAD (the report is committed AFTER, so HEAD moves by the seal: an ancestor with only ledger-dir changes)
  jq -nc --arg h "$(git -C "$R" rev-parse HEAD)" --arg s "$SCSHA" --arg j "$LJS" --arg l "$LIS" \
    '{schema:"scope-drift/1",head:$h,scope_sha256:$s,lumen_scope_sha256:$j,lumenignore_sha256:$l,
      checks:[{check:"scope_render_check",rc:0},{check:"scope_to_lumen_json_check",rc:0},{check:"lumenignore_fresh",rc:0}]}' | jq "${1:-.}" > "$R/ev/drift.json"
}
dbase() { base --lumen-json "$T/lumen_scope.json" --lumenignore "$T/li_a.txt" --lumenignore-fresh "$T/li_b.txt" --drift-report "$R/ev/drift.json" "$@"; }
setup; mkdrift; seal; dbase
chk "D01 golden-false: report bound to the real scope, lumen json, lumenignore and HEAD -> P-ScopeDrift PASS" 0 P-ScopeDrift PASS
setup; mkdrift '.head="deadbeef"'; seal; dbase; chk "D02 fabricated head 'deadbeef' (R3-2 repro) -> FAIL drift_head_unknown" 1 P-ScopeDrift FAIL drift_head_unknown
setup; mkdrift; seal; echo x > "$R/other.txt"; git -C "$R" add -A; git -C "$R" commit -q -m "unrelated change after the report"; dbase
chk "D03 report older than HEAD with a non-evidence change in between -> FAIL drift_head_not_current" 1 P-ScopeDrift FAIL drift_head_not_current
setup; git -C "$R" checkout -q -b side && echo x > "$R/ev/side.txt" && git -C "$R" add -A && git -C "$R" commit -q -m side && SIDE="$(git -C "$R" rev-parse HEAD)" && git -C "$R" checkout -q - ; mkdrift ".head=\"$SIDE\""; seal; dbase
chk "D03b head on a side branch (not an ancestor of HEAD) whose diff is confined to the ledger directory -> FAIL drift_head_not_current" 1 P-ScopeDrift FAIL drift_head_not_current
git -C "$R" branch -q -D side
setup; mkdrift '.lumen_scope_sha256=("0"*64)'; seal; dbase; chk "D04 lumen_scope_sha256 = 64 zeros (R3-2 repro) -> FAIL drift_digest_zero" 1 P-ScopeDrift FAIL drift_digest_zero
setup; mkdrift '.lumen_scope_sha256=("d"*64)'; seal; dbase; chk "D05 lumen_scope_sha256 of another file -> FAIL drift_report_not_for_this_lumen_scope" 1 P-ScopeDrift FAIL drift_report_not_for_this_lumen_scope
setup; mkdrift; seal; base --lumenignore "$T/li_a.txt" --lumenignore-fresh "$T/li_b.txt" --drift-report "$R/ev/drift.json"
chk "D06 --drift-report without --lumen-json -> FAIL lumen_json_not_supplied (binding not checked is not a pass)" 1 P-ScopeDrift FAIL lumen_json_not_supplied
setup; mkdrift; NOLEDGER='^drift.json$' seal; dbase; chk "D07 drift report absent from the ledger -> FAIL evidence_not_in_ledger" 1 P-ScopeDrift FAIL evidence_not_in_ledger
setup; mkdrift; seal; base --lumen-json "$T/lumen_scope.json" --lumenignore "$T/li_a.txt" --lumenignore-fresh "$T/li_c.txt" --drift-report "$R/ev/drift.json"
chk "D08 report says lumenignore_fresh rc 0 but the gate's own compare differs -> FAIL drift_disagrees_with_lumenignore_compare" 1 P-ScopeDrift FAIL drift_disagrees_with_lumenignore_compare
setup; mkdrift '.scope_sha256=("e"*64)'; seal; dbase; chk "D09 scope_sha256 of another scope -> FAIL drift_report_not_for_this_scope" 1 P-ScopeDrift FAIL drift_report_not_for_this_scope
setup; mkdrift '.lumenignore_sha256=("f"*64)'; seal; dbase; chk "D09b lumenignore_sha256 of another file -> FAIL drift_report_not_for_this_lumenignore" 1 P-ScopeDrift FAIL drift_report_not_for_this_lumenignore
setup; mkdrift '.schema="scope-drift/2"'; seal; dbase; chk "D09c wrong schema -> FAIL drift_report_unusable" 1 P-ScopeDrift FAIL drift_report_unusable
setup; mkdrift '.checks[1].rc=1'; seal; dbase; chk "D09d a failing check -> FAIL drift:scope_to_lumen_json_check" 1 P-ScopeDrift FAIL drift:scope_to_lumen_json_check
setup; mkdrift 'del(.checks[2])'; seal; dbase; chk "D09e a missing required check -> FAIL missing_check" 1 P-ScopeDrift FAIL missing_check
setup; mkdrift; seal; dbase
{ [ "$RC" -eq 0 ] && [ "$(jq -r .verdict "$T/o.json")" = PASS ] && [ "$(jq -r .complete "$T/o.json")" = true ]; } && ok "D10 golden-false: every proof ran, all evidence derived -> verdict PASS rc=0" || bad "D10 full run: rc=$RC verdict=$(jq -r .verdict "$T/o.json") skipped=$(jq -rc .skipped_proofs "$T/o.json") P-LumenIgnore=$(pv P-LumenIgnore)/$(reason P-LumenIgnore)"

# ---- C: P3 classes at EVERY depth, from the hand-written member table (R3-3, R3-4, R3-7) -------------------------------
p3run() { files "$POS" a/b.go "$1" > "$T/f_p.json"; printf '%s\n' "$POS" a/b.go "$1" > "$T/tr_p.txt"
  "$IH" --out "$T/o.json" --scope "${2:-$SCOPE}" --cg-status "$T/st.json" --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt" --needle-pos "$POS" --needle-neg "$NEG" >"$T/stdout" 2>"$T/stderr"; RC=$?; }
NM="$(grep -vc '^#' "$MEMBERS")"
[ "$NM" -ge 70 ] && ok "C00 the member table holds $NM rows (instrument sees the table)" || bad "C00 member table too short: $NM"
while IFS="$(printf '\t')" read -r mpath mwant mwhy; do
  case "$mpath" in '#'*|'') continue ;; esac
  p3run "$mpath"
  if [ "$mwant" = FAIL ]; then chkp "C $mpath -> P3 FAIL ($mwhy)" P3 FAIL
  else chkp "C $mpath -> P3 PASS ($mwhy)" P3 PASS; fi
done < "$MEMBERS"

# ---- B0: baseline matcher kinds with an extra row each
printf 'zzexact   secret   exact:q/exact.txt\n' > "$T/baseline_exact.txt"
for pp in q/exact.txt:FAIL q/other/exact.txt:PASS q/exact.txt.bak:PASS; do
  p3run "${pp%%:*}"; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt" --needle-pos "$POS" --needle-neg "$NEG" --baseline "$T/baseline_exact.txt" >/dev/null 2>&1; RC=$?
  chkp "B05 exact:q/exact.txt vs ${pp%%:*} -> P3 ${pp##*:}" P3 "${pp##*:}"
done

# ---- K: the coverage probe runs at every depth and the reviewer mutants R1..R4 are non-equivalent -----------------------
mkscope() {  # mkscope <name> <python expression on the loaded scope dict d>: scope variant for a coverage fixture
  python3 - "$SCOPE" "$T/$1.yaml" "$2" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
exec(sys.argv[3])
yaml.safe_dump(d, open(sys.argv[2], "w"), sort_keys=False)
PY
}
# R1 (reviewer DATA mutant): secrets_dir regex '(^|/)secrets/' -> '/secrets/' misses root-level secrets/
mkscope k_r1 'for c in d["forbidden_classes"]:
    if c["name"] == "secrets_dir": c["regex"] = "/secrets/"'
p3run "a/b.go" "$T/k_r1.yaml"; chkp "K01 R1: class '/secrets/' misses root-level secrets/ -> the tool itself says baseline_pattern_uncovered (P3 FAIL)" P3 FAIL baseline_pattern_uncovered
p3run "secrets/db.yaml" "$T/k_r1.yaml"; chkp "K01b R1: root-level secrets/db.yaml indexed under the mutant class -> still P3 FAIL (via the constitution baseline predicate)" P3 FAIL
# coverage fixtures: a baseline pattern `**/zzdir/` and a class whose regex covers only some probe depths
addzz() { mkscope "$1" 'd["baseline_excludes"]["caches"].append("**/zzdir/")
d["forbidden_classes"].append({"name": "zz", "regex": "'"$2"'", "positive": "'"$3"'", "negative": "q/other.py"})'; }
addzz k_all '^(zzdir|a/zzdir|a/b/zzdir)/' 'zzdir/x.py'
p3run a/b.go "$T/k_all.yaml"; chkp "K02 golden-false: a class covering exactly depth 0, 1 and 2 of the pattern -> no uncovered report (kills R2 'keep **/')" P3 PASS
addzz k_nonroot '^[^/]+/zzdir/' 'a/zzdir/x'
p3run a/b.go "$T/k_nonroot.yaml"; chkp "K03 class that misses depth 0 (nested only) -> baseline_pattern_uncovered (kills 'probe at a/ only', R4)" P3 FAIL baseline_pattern_uncovered:**/zzdir/
addzz k_rootonly '^zzdir/' 'zzdir/x.py'
p3run a/b.go "$T/k_rootonly.yaml"; chkp "K04 class that covers only the root form -> baseline_pattern_uncovered (kills 'probe at root only')" P3 FAIL baseline_pattern_uncovered:**/zzdir/
addzz k_gap1 '^zzdir/|^[^/]+/[^/]+/zzdir/' 'zzdir/x.py'
p3run a/b.go "$T/k_gap1.yaml"; chkp "K05 class that misses depth 1 only -> baseline_pattern_uncovered (kills 'drop probe depth 1')" P3 FAIL baseline_pattern_uncovered:**/zzdir/
addzz k_nodeep '^(zzdir|a/zzdir)/' 'zzdir/x.py'
p3run a/b.go "$T/k_nodeep.yaml"; chkp "K06 class that misses depth 2 only -> baseline_pattern_uncovered (kills 'drop probe depth 2')" P3 FAIL baseline_pattern_uncovered:**/zzdir/
mkscope k_anch 'd["baseline_excludes"]["build_outputs"].append("/zzout/")
d["forbidden_classes"].append({"name": "zzo", "regex": "^zzout/", "positive": "zzout/x", "negative": "a/zzout/x"})'
p3run a/b.go "$T/k_anch.yaml"; chkp "K07 golden-false: an anchored pattern /zzout/ is probed at the root only (a root-only class covers it)" P3 PASS

# ---- B: the constitution baseline file is applied directly (R3-4) -----------------------------------------------------
printf 'zzclass   exclude  topdir:zzcurated\nzzseg     secret   seg:zzseg\n' > "$T/base_extra.txt"
cat "$RR/constitution/scripts/codegraph/scope_baseline.txt" "$T/base_extra.txt" > "$T/baseline2.txt"
p3run zzcurated/x.go; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt" --needle-pos "$POS" --needle-neg "$NEG" --baseline "$T/baseline2.txt" >/dev/null 2>&1; RC=$?
chkp "B01 an extra baseline class (topdir:zzcurated) in the --baseline file is judged -> P3 FAIL baseline_class_files_indexed" P3 FAIL baseline_class_files_indexed
p3run q/zzseg/x.go; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt" --needle-pos "$POS" --needle-neg "$NEG" --baseline "$T/baseline2.txt" >/dev/null 2>&1; RC=$?
chkp "B02 seg:zzseg matches any path component -> P3 FAIL" P3 FAIL baseline_class_files_indexed
p3run a/b.go; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt" --needle-pos "$POS" --needle-neg "$NEG" --baseline "$T/no/such" >/dev/null 2>&1; RC=$?
chkp "B03 unreadable --baseline -> P3 FAIL baseline_unreadable" P3 FAIL baseline_unreadable
printf 'bad line without kind\n' > "$T/baseline_bad.txt"; "$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt" --needle-pos "$POS" --needle-neg "$NEG" --baseline "$T/baseline_bad.txt" >/dev/null 2>&1; RC=$?
chkp "B04 malformed baseline row -> P3 FAIL baseline_malformed (never skipped)" P3 FAIL baseline_malformed

echo "SUMMARY pass=$PASSN fail=$FAILN skip=$SKIPN"
[ "$FAILN" -eq 0 ] || exit 1
[ "$SKIPN" -eq 0 ] || exit 77
exit 0
