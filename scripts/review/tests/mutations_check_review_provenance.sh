#!/usr/bin/env bash
# mutations_check_review_provenance.sh - paired mutations for scripts/review/check_review_provenance.sh (T094a).
# Each mutant is a literal python str.replace on a copy; the diff against the original is printed, so a
# recorded run is reproducible from its own output. A mutant must make the suite FAIL (killed).
# Usage: mutations_check_review_provenance.sh   (exit 0 only when every mutant is killed and every anchor applied)
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
orig=$here/../check_review_provenance.sh
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
survived=0; n=0; equiv=0
mut() { # id description (a mutant whose id starts with EQ is a documented equivalent: it must SURVIVE) 'old' 'new'
  local id=$1 desc=$2 old=$3 new=$4 m=$tmp/$1.sh out rc
  [ -z "${MUT_ONLY:-}" ] || case " $MUT_ONLY " in *" $id "*) ;; *) return;; esac   # MUT_ONLY="id id": run just those mutants (a subset run, e.g. the mutants of a new fix)
  n=$((n+1))
  OLD=$old NEW=$new python3 - "$orig" "$m" <<'PY' || { echo "MUTANT $id: ANCHOR NOT FOUND (mutation not applied)"; survived=$((survived+1)); return; }
import os,sys
s=open(sys.argv[1]).read(); o=os.environ["OLD"]
if s.count(o)!=1: sys.exit(1)
open(sys.argv[2],"w").write(s.replace(o,os.environ["NEW"]))
PY
  echo "--- MUTANT $id: $desc"; diff "$orig" "$m" | sed 's/^/    /'
  [ -z "${MUT_ANCHORS_ONLY:-}" ] || { echo "    anchor applied exactly once (MUT_ANCHORS_ONLY: the suite was not run)"; return; }   # checks only that every mutant still patches the script
  out=$(CRP_SCRIPT=$m bash "$here/test_check_review_provenance.sh" 2>&1); rc=$?
  case $id in EQ*) if [ $rc -eq 0 ]; then echo "    SURVIVED as documented EQUIVALENT mutant (redundant by construction, see the mutant description)"; equiv=$((equiv+1)); else echo "    KILLED (unexpected: not equivalent)"; survived=$((survived+1)); fi; return;; esac
  if [ $rc -ne 0 ]; then echo "    KILLED ($(printf '%s' "$out" | tail -1); first: $(printf '%s' "$out" | grep -m1 '^FAIL' | cut -c1-80))"
  else echo "    SURVIVED ($(printf '%s' "$out" | tail -1))"; survived=$((survived+1)); fi
}
mut c1 "delete the verdict-model equality check" \
 "jq -e --slurpfile r \"\$pc\" '.model==\$r[0].model' \"\$vc\"" 'true'
mut c2 "ignore the explicit record argument" 'prov=${2:-${verdict%.json}.provenance.json}' 'prov=${verdict%.json}.provenance.json'
mut c3 "accept effort ? (record effort only needs to exist)" "jq -e '.effort==\"xhigh\"' \"\$pc\"" "jq -e '.effort!=null' \"\$pc\""
mut c4 "drop the run id presence check" "jq -e '(.workflow_run_id|type==\"string\") and (.workflow_run_id|length>0)' \"\$pc\"" 'true'
mut c4b "run id shape check becomes string-type only" "'.workflow_run_id|test(\$p)'" "'.workflow_run_id|type==\"string\"'"
mut c4c "run id pattern unanchored" 'jq -e --arg p "\\A(?:$pat)\\z"' 'jq -e --arg p "$pat"'
mut c5 "skip the verdict_sha256 comparison" "'.verdict_sha256==\$a'" 'true'
mut i1a "allow-list match becomes a substring match" "(.model as \$m | any(\$ids[]; .==\$m))" "(.model|test(\"opus\"))"
mut i1b "allow-list match against the space-joined ids (joined ids pass)" "(.model as \$m | any(\$ids[]; .==\$m))" "((\$ids|join(\" \")) as \$j | (.model as \$m | \$j|contains(\$m)))"
mut EQ1 "drop the record model string-type check" "(.model|type==\"string\") and (.model as" "(.model as"
mut i1c "allow-list match case-insensitive" "any(\$ids[]; .==\$m)" "any(\$ids[]; ascii_downcase==(\$m|ascii_downcase))"
mut i2a "verdict: accept multiple documents / BOM / bad UTF-8 (jq parse only)" 'strict "$vc" ||' 'jq -e . "$vc" >/dev/null 2>&1 ||'
mut i2b "record: accept multiple documents / BOM / bad UTF-8 (jq parse only)" 'strict "$pc" ||' 'jq -e . "$pc" >/dev/null 2>&1 ||'
mut i2c "strict(): drop the object-type requirement" 'sys.exit(0 if isinstance(o, dict) else 1)' 'sys.exit(0)'
mut EQ2 "strict(): drop the explicit UTF-8 BOM check (python json.loads on str rejects a BOM itself; the explicit check is defence in depth)" "if b.startswith(b'\\xef\\xbb\\xbf'): sys.exit(1)" 'pass'
mut m4b "strict(): decode invalid UTF-8 leniently" "s = b.decode('utf-8')" "s = b.decode('utf-8','replace')"
mut r1 "R1: skip the effort equality when the verdict has no effort" "'.effort==\$r[0].effort' \"\$vc\"" "'(.effort==null) or .effort==\$r[0].effort' \"\$vc\""
mut r2 "R2: effort_argument may be any non-null value" "jq -e '.effort_argument==\"xhigh\"' \"\$pc\"" "jq -e '.effort_argument!=null' \"\$pc\""
mut r3 "R3: refusals exit 1, not 20" "\"\$1\" \"\$d\" >&2; exit 20" "\"\$1\" \"\$d\" >&2; exit 1"
mut r4 "R4: usage error exits 0" '[ $# -le 2 ] || { echo "usage: $0 [--bootstrap] <verdict.json> [<provenance.json>]" >&2; exit 2; }' '[ $# -le 2 ] || { echo "usage: $0 [--bootstrap] <verdict.json> [<provenance.json>]" >&2; exit 0; }'
mut r6 "R6: sha compared on 16 characters" "'.verdict_sha256==\$a'" "'(.verdict_sha256[:16])==(\$a[:16])'"
mut r6b "R6b: sha compared as a prefix" "'.verdict_sha256==\$a'" "'.verdict_sha256 as \$s | \$a|startswith(\$s)'"
mut s1 "B-1: schema-invalid verdict accepted" '10) refuse verdict_schema_invalid "$verr" ;;' '10) : ;;'
mut s2 "B-1: unset CPA_APPROVED_DIR no longer refused (falls back to the working tree)" '[ -n "${CPA_APPROVED_DIR:-}" ] || refuse verdict_schema_unavailable "CPA_APPROVED_DIR unset: the schema is never read from HEAD or the working tree"' 'CPA_APPROVED_DIR=${CPA_APPROVED_DIR:-$PWD}'
mut s3 "B-1: schema read from the working tree, not CPA_APPROVED_DIR" '  base=$(realpath -e -- "$CPA_APPROVED_DIR" 2>/dev/null) || refuse verdict_schema_unavailable "CPA_APPROVED_DIR not resolvable"
  cand=$(realpath -e -- "$CPA_APPROVED_DIR/$rel" 2>/dev/null)' '  base=$(realpath -e -- "$PWD" 2>/dev/null) || refuse verdict_schema_unavailable "CPA_APPROVED_DIR not resolvable"
  cand=$(realpath -e -- "$PWD/$rel" 2>/dev/null)'
mut s4 "B-1: unavailable schema/validator (status 3) accepted" '  *) refuse verdict_schema_unavailable "${verr:-validator exited with status $src}" ;;' '  3) : ;;
  *) refuse verdict_schema_unavailable "${verr:-validator exited with status $src}" ;;'
mut m1 "m-1: the model check reads the original verdict path" "'.model==\$r[0].model' \"\$vc\"" "'.model==\$r[0].model' \"\$verdict\""
mut m2 "m-2: sha256sum of the original path (escaped digest, second read)" 'asha=$(sha256sum < "$vc" | cut' 'asha=$(sha256sum "$verdict" | cut'
mut m3 "m-3: refusal detail not sanitised" "d=\$(printf '%s' \"\${2:-}\" | tr -c '[:print:]' '?' | head -c 300)" 'd=${2:-}'
# ---- round 4: reviewer survivors NM1-NM5, NM7 (review r3) and the mutants of the new checks
mut nm1 "NM1: strict() accepts NaN/Infinity" "o = json.loads(s, parse_constant=bad)" "o = json.loads(s)"
mut nm2 "NM2: allow-list loses claude-opus-5-5 (false refusal of the id of every real run record)" "claude-opus-5 claude-opus-5-5 claude-opus-4-7" "claude-opus-5 claude-opus-4-7"
mut nm3 "NM3: drop V.check_schema(sch)" "V = validator_for(sch); V.check_schema(sch)" "V = validator_for(sch)"
mut nm4 "NM4: drop the -f guard on the verdict (a FIFO hangs)" '[ -f "$verdict" ] && cat -- "$verdict" > "$vc" 2>/dev/null || refuse' 'cat -- "$verdict" > "$vc" 2>/dev/null || refuse'
mut nm4b "NM4: drop the -f guard on the record (a FIFO hangs)" '[ -f "$prov" ] || refuse provenance_absent "$prov"' ':'
mut nm5 "NM5: the OK line prints a fixed model" 'model=$(show model "$pc") effort=$(show effort "$pc")"' 'model=\"claude-opus-5\" effort=$(show effort "$pc")"'
mut nm5b "m-2: the run id is printed with jq -r (a stray ? from the newline)" "jq -j '.workflow_run_id'" "jq -r '.workflow_run_id'"
mut nm7 "NM7: the record check is a plain json.load (no object / NaN rule)" 'strict "$pc" || refuse provenance_unreadable' "python3 -c 'import json,sys;json.load(open(sys.argv[1]))' \"\$pc\" || refuse provenance_unreadable"
mut i1x1 "I-1: the .. / absolute segment rule dropped (containment alone would still hold for the escaping .. case)" '/*|..|../*|*/..|*/../*) refuse' '/*) refuse'
mut i1x2 "I-1: realpath containment under CPA_APPROVED_DIR dropped (symlink escape)" 'case $cand in "$base"/*) : ;; *) refuse verdict_schema_unavailable "schema path leaves CPA_APPROVED_DIR: $rel" ;; esac' ':'
mut EQ3 "I-1: drop the absolute-path rule (equivalent: \$CPA_APPROVED_DIR/\$rel with an absolute rel resolves under the base and is not found)" '/*|..|../*|*/..|*/../*) refuse' '..|../*|*/..|*/../*) refuse'
mut i1x3 "I-1: CRP_* overrides no longer refused in trusted mode" 'if [ -n "${CPA_EXEC_SHA256:-}" ] || [ "$bootstrap" = 1 ]; then' 'if [ "$bootstrap" = 1 ]; then'
mut i1x4 "I-1: CRP_* overrides no longer refused in bootstrap mode" 'if [ -n "${CPA_EXEC_SHA256:-}" ] || [ "$bootstrap" = 1 ]; then' 'if [ -n "${CPA_EXEC_SHA256:-}" ]; then'
mut m5 "m-5: an invalid CRP_RUN_ID_PATTERN is no longer a configuration error" '[ $? -le 1 ] || { echo "CRP_RUN_ID_PATTERN is not a valid regular expression" >&2; exit 2; }' ':'
mut n1 "nit: an empty second argument falls back to the default record" '[ $# -eq 2 ] && [ -z "$2" ] && { echo "usage: the provenance argument must not be empty" >&2; exit 2; }' ':'
mut m4 "m-4: an exception while validating is no longer caught (traceback)" 'except Exception as e:
    print("UNAVAILABLE schema validation failed: %s" % type(e).__name__); sys.exit(3)' 'except KeyError as e:
    print("UNAVAILABLE schema validation failed: %s" % type(e).__name__); sys.exit(3)'
mut b1 "B-1: bootstrap does not require the schema in reviewed_files" 'refuse bootstrap_schema_not_reviewed "reviewed_files lists $DEFAULT_REL not exactly once"' ':'
mut b2 "B-1: bootstrap does not compare the schema sha256" 'refuse bootstrap_schema_sha_mismatch "work-tree schema sha256=$ssha differs from the reviewed_files entry"' ':'
mut b3 "B-1: bootstrap allowed although an approved copy exists" 'if [ -n "${CPA_APPROVED_DIR:-}" ] || [ -n "${CPA_EXEC_SHA256:-}" ]; then
    echo "--bootstrap refused' 'if false; then
    echo "--bootstrap refused'
mut b4 "B-1: an unset CPA_APPROVED_DIR silently falls back to the bootstrap path" 'if [ "$bootstrap" = 1 ]; then
  root=' 'if [ "$bootstrap" = 1 ] || [ -z "${CPA_APPROVED_DIR:-}" ]; then
  root='
mut b5 "B-1: bootstrap schema may be a symlink leaving the work tree" 'case $cand in "$rroot"/*) : ;; *) refuse verdict_schema_unavailable "schema path leaves the work tree: $DEFAULT_REL" ;; esac' ':'
# ---- round 5: review r4 I-1, I-2, I-3 (reviewer mutants RM1-RM5, RM7) and m-1
mut rm1 "RM1: bootstrap 'exactly once' becomes 'at least once'" "| length)==1'" "| length)>=1'"
mut rm2 "RM2: any validator status other than 0/10 is accepted (fail-open when the validator is killed)" '*) refuse verdict_schema_unavailable "${verr:-validator exited with status $src}" ;;' '*) : ;;'
mut rm2b "m-1: an uncaught Python exception (status 1) is reported as a verdict about the document" '  10) refuse verdict_schema_invalid "$verr" ;;' '  1|10) refuse verdict_schema_invalid "$verr" ;;'
mut rm3 "RM3: the containment base is not canonicalised" 'base=$(realpath -e -- "$CPA_APPROVED_DIR" 2>/dev/null) || refuse verdict_schema_unavailable "CPA_APPROVED_DIR not resolvable"' 'base=$CPA_APPROVED_DIR'
mut rm4 "RM4: an empty-string effort_argument is no longer 'absent'" "jq -e '.effort_argument!=null and .effort_argument!=\"\"' \"\$pc\"" "jq -e '.effort_argument!=null' \"\$pc\""
mut rm5 "RM5: bootstrap uses the cwd, not the work-tree root" 'root=$(git rev-parse --show-toplevel 2>/dev/null) && [ -n "$root" ]' 'root=$PWD; [ -n "$root" ]'
mut rm7 "RM7: the 300-byte bound on a refusal detail dropped" "tr -c '[:print:]' '?' | head -c 300)" "tr -c '[:print:]' '?')"
mut i1p1 "I-1: strict() runs python3 without -I (cwd on sys.path)" 'strict() { python3 -I - "$1" <<'"'"'PY'"'" 'strict() { python3 - "$1" <<'"'"'PY'"'"
mut i1p2 "I-1: the validator stage runs python3 without -I (cwd on sys.path)" 'verr=$(python3 -I - "$schema" "$vc" <<'"'"'PY'"'" 'verr=$(python3 - "$schema" "$vc" <<'"'"'PY'"'"
mut i2s1 "I-2: exported shell functions are no longer unset" '{ while IFS= read -r _f; do unset -f -- "$_f"; done < <(compgen -A function); } 2>/dev/null' ':'
mut i2s2 "I-2: no variable is scrubbed" '  case $_v in LD_*|PYTHON*|BASH_ENV|ENV|BASH_FUNC_*|CDPATH|GLOBIGNORE) unset "$_v" ;; esac' '  :'
mut i2s3 "I-2: LD_* no longer scrubbed" 'case $_v in LD_*|PYTHON*|BASH_ENV' 'case $_v in PYTHON*|BASH_ENV'
mut i2s4 "I-2: PYTHON* no longer scrubbed" 'case $_v in LD_*|PYTHON*|BASH_ENV' 'case $_v in LD_*|BASH_ENV'
mut i2s5 "I-2: BASH_ENV and ENV no longer scrubbed" 'case $_v in LD_*|PYTHON*|BASH_ENV|ENV|BASH_FUNC_*' 'case $_v in LD_*|PYTHON*|BASH_FUNC_*'
mut EQ4 "I-2: the IFS reset dropped (equivalent: bash never imports IFS from the environment, test 'exported IFS')" "IFS=\$' \\t\\n'" ':'
# ---- round 6: review r5 F1 (jq reads $HOME/.jq) and F2 (unusable tool reported as a record defect)
mut r5f1a "R5 F1: the jq wrapper that points HOME at the private work dir is removed" 'jq() { HOME=$w command jq "$@"; }' ':'
mut r5f1b "R5 F1: the jq wrapper keeps the caller HOME" 'jq() { HOME=$w command jq "$@"; }' 'jq() { command jq "$@"; }'
mut r5f2a "R5 F2: the jq start smoke test dropped" 'jq -n 1 >/dev/null 2>&1 || {' 'true || {'
mut r5f2b "R5 F2: the python3 start smoke test dropped" "python3 -I -c '' >/dev/null 2>&1 || {" 'true || {'
# ---- round 7 (WF6-REVIEW provenance-eventcore PR-1, PR-2): reviewer mutants RVP5 and RVP3 of the round-6 code
mut r6pr1 "R6 PR-1: the jq HOME is the work dir's PARENT (TMPDIR, a shared directory) instead of the private work dir" 'jq() { HOME=$w command jq "$@"; }' 'jq() { HOME=$w/.. command jq "$@"; }'
mut r6pr2 "R6 PR-2: the python3 start smoke test runs without -I (the user site and PYTHON* are honoured)" "python3 -I -c '' >/dev/null 2>&1 || {" "python3 -c '' >/dev/null 2>&1 || {"
echo "MUTATIONS total=$n survived_or_unapplied=$survived documented_equivalent=$equiv"
[ $survived -eq 0 ]
