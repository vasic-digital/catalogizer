#!/usr/bin/env bash
# check_review_provenance.sh - T094a (FR-023, SC-012; constitution 11.4.209, 11.4.231(F.2)).
# Validates a [REVIEW] verdict file against its provenance record and against review-verdict.schema.json.
# Never starts a review.
#
# Usage: check_review_provenance.sh [--bootstrap] <verdict.json> [<provenance.json>]
#   default record: <verdict dir>/<verdict name without .json>.provenance.json
#   --bootstrap: the explicit, narrow adoption mode (round 4, review r3 B-1). tasks.md T046a step (1) and CENTRAL C6
#     (a project's first approval, which has no approved checker yet) run this checker "from the working tree" when NO
#     approved copy exists. In that one situation, and only with --bootstrap, the schema is read from the work tree
#     file at the fixed default path (never an override) and is accepted ONLY when its sha256 equals the sha256 the
#     verdict itself lists in reviewed_files for that path: the schema checked is exactly the one the owner is about to
#     approve (a verdict not listing it: bootstrap_schema_not_reviewed; a differing file: bootstrap_schema_sha_mismatch).
#     Refused (exit 2) when CPA_APPROVED_DIR or CPA_EXEC_SHA256 is set (an approved copy exists: use trusted mode), when a
#     CRP_* override is set, or outside a git work tree. Never a fallback: without --bootstrap an unset
#     CPA_APPROVED_DIR is still refused. The accept line is marked BOOTSTRAP. The T094c working-tree call of
#     `--list-trusted` (a mode T094d adds) needs the same mode: owed to T094d, see the guide.
# Exit 0: accepted. Exit 20: refused, ONE line "REFUSED reason=<code> ..." on stderr (values sanitised, one line).
# Python stages run as `python3 -I -` (isolated: the current directory, PYTHON* and the user site are not on sys.path;
#   review r4 I-1). Validator exit 0 = valid, 10 = the schema reports errors (verdict_schema_invalid), anything else
#   (3 = module missing / schema unusable, a crash, a kill) = verdict_schema_unavailable (review r4 m-1).
# Exit 2: usage error, or a required tool (jq, python3) is missing or UNUSABLE (cannot start after the environment scrub: "tool unusable").
# Reasons: verdict_unreadable, provenance_absent, provenance_unreadable, workflow_run_id_absent,
#   workflow_run_id_malformed, model_not_opus, effort_not_xhigh, effort_argument_absent, effort_argument_mismatch,
#   verdict_field_mismatch, verdict_sha_absent, verdict_sha_mismatch, verdict_schema_unavailable,
#   verdict_schema_invalid, bootstrap_schema_not_reviewed, bootstrap_schema_sha_mismatch.
#
# Environment:
#   CPA_APPROVED_DIR  the approved-copy root ($CPA_RUN/released, CENTRAL C5/C6 and the rev 32 rule): the verdict
#                     schema is read ONLY from there, never from HEAD or the working tree. Unset or empty: the
#                     schema check is refused (verdict_schema_unavailable), never skipped.
#   CRP_SCHEMA_REL    schema path under CPA_APPROVED_DIR: relative, no `..` segment, and its real path (symlinks
#                     resolved) must lie under the real path of CPA_APPROVED_DIR, else verdict_schema_unavailable. Default: specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
#                     (UNCONFIRMED: that released/ mirrors repository-relative paths is read from tasks.md T042's
#                     `released/scripts/build/remote/emit.sh` example, not observed from a real run directory).
#                     Both CRP_* variables are REFUSED (exit 2) when CPA_EXEC_SHA256 is set (the cpa-host trusted mode:
#                     cpa-host must also start the checker with an allow-list environment in every mode, never a
#                     deny-list of PATH and CRP_*; requirement for T042, see the guide) and in --bootstrap mode. An invalid CRP_RUN_ID_PATTERN is a configuration error (exit 2).
#   CRP_RUN_ID_PATTERN  Oniguruma regex (without anchors; \A and \z are added) the Workflow run id must match.
#                     Default: wf_[0-9a-f]{8}-[0-9a-f]{3}. That shape was OBSERVED on this host (every one of
#                     1656 Workflow run records); it is UNCONFIRMED as a general rule across harness versions,
#                     hence configurable.
#
# HONEST BOUNDARY (11.4.6): this proves the record is CONSISTENT with the verdict (run id well-formed, model
#   and effort as required, verdict sha bound, verdict schema-valid). It cannot prove the record was WRITTEN by a
#   Workflow run: any well-formed record that a typist produced passes. Authenticity comes only from where the
#   record is written (the Workflow run, CENTRAL C5 cpa-host --check-provenance on approved copies, T094b/T094d).
#   EFFORT IS THE REQUESTED ARGUMENT, NEVER OBSERVED: no Workflow run record on this host carries an effort key
#   (0 hits for effort/reasoningEffort/effortLevel, while the model key is recorded), so `effort` and
#   `effort_argument` can only be copied from what the dispatcher asked for. This check attests a requested
#   xhigh, not an observed one (record this on the KC-P1 item, T094b).
#   Consumers must pass an immutable copy of the verdict (cpa-host --commit materialisation): the verdict is
#   read exactly once, into a private copy, and every check and the sha256 run on that copy.
#   A GO with blocking_findings>0 is refused by the schema (verdict_schema_invalid) when the schema is available.
# Record fields (written by the Workflow run, never typed by the reviewer):
#   workflow_run_id, model, effort, effort_argument, verdict_sha256.
set -u
# Environment scrub (review r4 I-2). Bash has already imported exported functions and run BASH_ENV before this line, so
# the scrub removes what they left for every later command: all functions are unset (a function named jq, python3,
# cat... would replace the real tool), and the variables that redirect a child (the dynamic loader LD_*, the Python
# PYTHON*, BASH_ENV, ENV, CDPATH, GLOBIGNORE) are unset so that no child process inherits them. IFS is reset. This is the
# SECOND line: PATH cannot be restored here (the caller may legitimately shim it), and a hostile BASH_ENV can still
# shadow the BUILTINS used below (`unset`, `command`, `exec`, `read`...: after F1 every jq call goes through `command`, so a
# shadowed `command` is the same documented residual, O4, as before, now with one more name; round 6, PR-4); the FIRST line is
# the caller (cpa-host) starting this checker with an allow-list environment (`env -i`, the T042 allow-list; see the guide,
# "Environment").
{ while IFS= read -r _f; do unset -f -- "$_f"; done < <(compgen -A function); } 2>/dev/null
for _v in $(compgen -e); do
  case $_v in LD_*|PYTHON*|BASH_ENV|ENV|BASH_FUNC_*|CDPATH|GLOBIGNORE) unset "$_v" ;; esac
done
IFS=$' \t\n'
refuse() { # one line, no control characters, bounded length
  local d; d=$(printf '%s' "${2:-}" | tr -c '[:print:]' '?' | head -c 300)
  printf 'REFUSED reason=%s %s\n' "$1" "$d" >&2; exit 20
}
bootstrap=0
if [ "${1:-}" = "--bootstrap" ]; then bootstrap=1; shift; fi
[ $# -ge 1 ] && [ $# -le 2 ] || { echo "usage: $0 [--bootstrap] <verdict.json> [<provenance.json>]" >&2; exit 2; }
[ $# -eq 2 ] && [ -z "$2" ] && { echo "usage: the provenance argument must not be empty" >&2; exit 2; }
# configuration the caller must not be able to bend (review r3 I-1, B-1): trusted (cpa-host) and bootstrap modes
if [ -n "${CPA_EXEC_SHA256:-}" ] || [ "$bootstrap" = 1 ]; then
  if [ -n "${CRP_SCHEMA_REL+x}" ] || [ -n "${CRP_RUN_ID_PATTERN+x}" ]; then
    echo "CRP_SCHEMA_REL and CRP_RUN_ID_PATTERN are refused in trusted and bootstrap modes" >&2; exit 2; fi
fi
if [ "$bootstrap" = 1 ]; then
  if [ -n "${CPA_APPROVED_DIR:-}" ] || [ -n "${CPA_EXEC_SHA256:-}" ]; then
    echo "--bootstrap refused: an approved copy exists (CPA_APPROVED_DIR or CPA_EXEC_SHA256 set); use the trusted mode" >&2; exit 2; fi
fi
command -v jq >/dev/null 2>&1 || { echo "jq required" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "python3 required" >&2; exit 2; }
verdict=$1
prov=${2:-${verdict%.json}.provenance.json}

w=$(mktemp -d) || { echo "mktemp failed" >&2; exit 2; }
chmod 700 "$w"; trap 'rm -rf "$w"' EXIT
# review r5 F1: jq auto-loads $HOME/.jq, and a definition there can shadow builtins (test, any) and switch the run-id and
# model checks off. Every jq call runs with HOME pointing at the private, empty, 0700 work dir (jq 1.8 reads nothing else
# by default). Defined AFTER the environment scrub, so it is this script's own function, not an imported one.
jq() { HOME=$w command jq "$@"; }   # HOME is the private work dir ITSELF, never its parent (round 6, PR-1: the parent is the shared TMPDIR)
# review r5 F2: a tool that cannot start (for example one that needs LD_LIBRARY_PATH, which the scrub removed) is a
# tool problem (exit 2, "tool unusable"), never a defect of the record (exit 20).
jq -n 1 >/dev/null 2>&1 || { echo "jq unusable: it cannot start in the scrubbed environment (tool unusable)" >&2; exit 2; }
python3 -I -c '' >/dev/null 2>&1 || { echo "python3 unusable: it cannot start in the scrubbed environment (tool unusable)" >&2; exit 2; }
vc=$w/verdict.json; pc=$w/prov.json
# single read of each file (TOCTOU-safe): every later check works on these private copies
[ -f "$verdict" ] && cat -- "$verdict" > "$vc" 2>/dev/null || refuse verdict_unreadable "$verdict"

# strict JSON: exactly one top-level object, UTF-8 without BOM, no NaN/Infinity (python json.load parity)
strict() { python3 -I - "$1" <<'PY'
import json, sys
def bad(c): raise ValueError(c)
b = open(sys.argv[1], 'rb').read()
if b.startswith(b'\xef\xbb\xbf'): sys.exit(1)
try:
    s = b.decode('utf-8')
    o = json.loads(s, parse_constant=bad)
except Exception:
    sys.exit(1)
sys.exit(0 if isinstance(o, dict) else 1)
PY
}
strict "$vc" || refuse verdict_unreadable "not exactly one strict UTF-8 JSON object: $verdict"
[ -f "$prov" ] || refuse provenance_absent "$prov"
cat -- "$prov" > "$pc" 2>/dev/null || refuse provenance_unreadable "$prov"
strict "$pc" || refuse provenance_unreadable "not exactly one strict UTF-8 JSON object: $prov"

show() { jq -c --arg k "$1" '.[$k]' "$2" 2>/dev/null | head -c 120; }  # JSON-escaped, one line

# run id: a non-empty string of the configured shape (all comparisons inside jq: no bash string handling)
jq -e '(.workflow_run_id|type=="string") and (.workflow_run_id|length>0)' "$pc" >/dev/null 2>&1 || refuse workflow_run_id_absent
pat=${CRP_RUN_ID_PATTERN:-'wf_[0-9a-f]{8}-[0-9a-f]{3}'}
jq -n --arg p "\\A(?:$pat)\\z" '""|test($p)' >/dev/null 2>&1; [ $? -le 1 ] || { echo "CRP_RUN_ID_PATTERN is not a valid regular expression" >&2; exit 2; }
jq -e --arg p "\\A(?:$pat)\\z" '.workflow_run_id|test($p)' "$pc" >/dev/null 2>&1 \
  || refuse workflow_run_id_malformed "record workflow_run_id=$(show workflow_run_id "$pc")"
rid=$(jq -j '.workflow_run_id' "$pc" | tr -c '[:print:]' '?')

# model identity: a JSON string equal to one id of the closed allow-list (no substring, no array, no joining).
# UNCONFIRMED: the list is this task's choice; extend it only by an owner-approved edit of this file.
OPUS_IDS=(claude-opus-5 claude-opus-5-5 claude-opus-4-7 claude-opus-4-6 claude-opus-4-5 claude-opus-4-1 claude-opus-4)
ids=$(printf '%s\n' "${OPUS_IDS[@]}" | jq -R . | jq -s -c .)
jq -e --argjson ids "$ids" '(.model|type=="string") and (.model as $m | any($ids[]; .==$m))' "$pc" >/dev/null 2>&1 \
  || refuse model_not_opus "record model=$(show model "$pc")"
jq -e '.effort=="xhigh"' "$pc" >/dev/null 2>&1 || refuse effort_not_xhigh "record effort=$(show effort "$pc")"
# the record must show the effort argument the run was started with
jq -e '.effort_argument!=null and .effort_argument!=""' "$pc" >/dev/null 2>&1 || refuse effort_argument_absent
jq -e '.effort_argument=="xhigh"' "$pc" >/dev/null 2>&1 \
  || refuse effort_argument_mismatch "record effort_argument=$(show effort_argument "$pc")"

# the verdict's own claims must equal the record (JSON value equality inside jq: no newline stripping)
jq -e --slurpfile r "$pc" '.effort==$r[0].effort' "$vc" >/dev/null 2>&1 \
  || refuse verdict_field_mismatch "effort verdict=$(show effort "$vc") record=$(show effort "$pc")"
jq -e --slurpfile r "$pc" '.model==$r[0].model' "$vc" >/dev/null 2>&1 \
  || refuse verdict_field_mismatch "model verdict=$(show model "$vc") record=$(show model "$pc")"

# the record binds the verdict bytes: exact equality of the full lowercase hex digest
jq -e '(.verdict_sha256|type=="string") and (.verdict_sha256|length>0)' "$pc" >/dev/null 2>&1 || refuse verdict_sha_absent
asha=$(sha256sum < "$vc" | cut -d' ' -f1)
jq -e --arg a "$asha" '.verdict_sha256==$a' "$pc" >/dev/null 2>&1 \
  || refuse verdict_sha_mismatch "record=$(show verdict_sha256 "$pc") actual=$asha"

# the verdict against review-verdict.schema.json, read from the approved copy only (CENTRAL C5/C6, rev 32);
# --bootstrap (T046a step 1, no approved copy yet): the work-tree schema bound to the verdict's own reviewed_files entry
DEFAULT_REL=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
sc=$w/schema.json
if [ "$bootstrap" = 1 ]; then
  root=$(git rev-parse --show-toplevel 2>/dev/null) && [ -n "$root" ] || { echo "--bootstrap needs a git work tree (the current directory is not in one)" >&2; exit 2; }
  rroot=$(realpath -e -- "$root" 2>/dev/null) || { echo "--bootstrap: cannot resolve the work tree" >&2; exit 2; }
  cand=$(realpath -e -- "$root/$DEFAULT_REL" 2>/dev/null) || refuse verdict_schema_unavailable "schema absent in the work tree: $DEFAULT_REL"
  case $cand in "$rroot"/*) : ;; *) refuse verdict_schema_unavailable "schema path leaves the work tree: $DEFAULT_REL" ;; esac
  [ -f "$cand" ] && cat -- "$cand" > "$sc" 2>/dev/null || refuse verdict_schema_unavailable "schema unreadable: $DEFAULT_REL"
  ssha=$(sha256sum < "$sc" | cut -d' ' -f1)
  jq -e --arg p "$DEFAULT_REL" '([.reviewed_files[]? | select(type=="object" and .path==$p)] | length)==1' "$vc" >/dev/null 2>&1 \
    || refuse bootstrap_schema_not_reviewed "reviewed_files lists $DEFAULT_REL not exactly once"
  jq -e --arg p "$DEFAULT_REL" --arg a "$ssha" '[.reviewed_files[] | select(type=="object" and .path==$p) | .sha256] | .[0]==$a' "$vc" >/dev/null 2>&1 \
    || refuse bootstrap_schema_sha_mismatch "work-tree schema sha256=$ssha differs from the reviewed_files entry"
  mode_note=" (BOOTSTRAP working-tree schema bound to reviewed_files sha256=$ssha)"
else
  [ -n "${CPA_APPROVED_DIR:-}" ] || refuse verdict_schema_unavailable "CPA_APPROVED_DIR unset: the schema is never read from HEAD or the working tree"
  rel=${CRP_SCHEMA_REL:-$DEFAULT_REL}
  case $rel in
    /*|..|../*|*/..|*/../*) refuse verdict_schema_unavailable "schema path must be relative without a .. segment: $rel" ;;
  esac
  base=$(realpath -e -- "$CPA_APPROVED_DIR" 2>/dev/null) || refuse verdict_schema_unavailable "CPA_APPROVED_DIR not resolvable"
  cand=$(realpath -e -- "$CPA_APPROVED_DIR/$rel" 2>/dev/null) || refuse verdict_schema_unavailable "schema absent: $CPA_APPROVED_DIR/$rel"
  case $cand in "$base"/*) : ;; *) refuse verdict_schema_unavailable "schema path leaves CPA_APPROVED_DIR: $rel" ;; esac
  [ -f "$cand" ] && cat -- "$cand" > "$sc" 2>/dev/null || refuse verdict_schema_unavailable "schema unreadable: $rel"
  mode_note=""
fi
schema=$sc

verr=$(python3 -I - "$schema" "$vc" <<'PY'
import json, sys
try:
    from jsonschema.validators import validator_for
except Exception:
    print("UNAVAILABLE jsonschema module missing"); sys.exit(3)
try:
    sch = json.load(open(sys.argv[1], encoding='utf-8'))
    V = validator_for(sch); V.check_schema(sch)
except Exception as e:
    print("UNAVAILABLE schema unusable: %s" % type(e).__name__); sys.exit(3)
try:
    doc = json.load(open(sys.argv[2], encoding='utf-8'))
    errs = sorted(V(sch).iter_errors(doc), key=lambda e: list(map(str, e.path)))
except Exception as e:
    print("UNAVAILABLE schema validation failed: %s" % type(e).__name__); sys.exit(3)
if errs:
    print("; ".join("%s: %s" % ("/".join(map(str, e.path)) or "(root)", e.message[:80]) for e in errs[:3])); sys.exit(10)
sys.exit(0)
PY
); src=$?
case $src in
  0) : ;;
  10) refuse verdict_schema_invalid "$verr" ;;
  *) refuse verdict_schema_unavailable "${verr:-validator exited with status $src}" ;;
esac
echo "OK review provenance$mode_note: run=$rid model=$(show model "$pc") effort=$(show effort "$pc")"
exit 0
