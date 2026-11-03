#!/usr/bin/env bash
# verify_live_evidence.sh - verifier of LIVE-EVIDENCE records (constitution 11.4.289(D)). Guide: docs/scripts/verify_live_evidence.md. Schema: docs/qa/live-evidence.schema.json.
# A record that claims class LIVE is RECLASSIFIED UNIT (exit 1) when what answered is a registered double, does not match the artifact the claim names, or is not identified.
# Loopback, test-started, scratch-dir real services stay LIVE: only the identity of the answering executable/image decides.
# Exit: 0 all records fine | 1 at least one LIVE claim reclassified UNIT (or --require-live met a UNIT) | 2 invalid record / usage / unreadable register (fail closed).
set -u
export LC_ALL=C
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
DOUBLES=${LIVE_EVIDENCE_DOUBLES:-$ROOT/docs/qa/live-evidence.doubles.txt}
REQUIRE_LIVE=0
paths=()
usage(){ echo "usage: verify_live_evidence.sh [--require-live] [--doubles FILE] <record.json|dir>..." >&2; exit 2; }
while [ $# -gt 0 ]; do
  case $1 in
    --require-live) REQUIRE_LIVE=1;;
    --doubles) [ $# -ge 2 ] || usage; DOUBLES=$2; shift;;
    -h|--help) usage;;
    -*) echo "verify_live_evidence: unknown option $1" >&2; usage;;
    *) paths+=("$1");;
  esac
  shift
done
[ ${#paths[@]} -gt 0 ] || usage
command -v jq >/dev/null || { echo "verify_live_evidence: jq missing (cannot verify; REFUSED)" >&2; exit 2; }
[ -r "$DOUBLES" ] || { echo "verify_live_evidence: double register unreadable: $DOUBLES (REFUSED, fail closed)" >&2; exit 2; }
files=()
for p in "${paths[@]}"; do
  if [ -d "$p" ]; then while IFS= read -r f; do files+=("$f"); done < <(find "$p" -maxdepth 1 -name '*.json' | sort)
  elif [ -f "$p" ]; then files+=("$p")
  else echo "verify_live_evidence: no such file or directory: $p" >&2; exit 2; fi
done
[ ${#files[@]} -gt 0 ] || { echo "verify_live_evidence: no records found (empty is not a pass)" >&2; exit 2; }

read -r -d '' PROG <<'JQ'
def glob2re: gsub("(?<c>[.+^$(){}|\\[\\]\\\\?])"; "\\\(.c)") | gsub("\\*"; ".*") | "^" + . + "$";
def rules($k): [$reg | split("\n")[] | select(startswith($k + ":")) | .[($k|length)+1:] | gsub("^\\s+|\\s+$"; "") | select(length > 0)];
def present: . != null and . != "";
def is_double($a):
  ([rules("sha256")[] | ascii_downcase] as $hashes
   | [rules("name")[] | glob2re] as $names
   | [rules("cmd")[] | glob2re] as $cmds
   | [rules("image")[] | glob2re] as $imgs
   | (($a.exe_path // "") | split("/") | (last // "")) as $base
   | (($a.exe_sha256 // "") | ascii_downcase | ltrimstr("sha256:")) as $h
   | (($a.image_digest // "") | ascii_downcase | ltrimstr("sha256:")) as $d
   | ($a.cmdline // "") as $cmd
   | ($a.image // "") as $img
   | ([$hashes[] | ltrimstr("sha256:")] | any(. == $h and $h != "" or . == $d and $d != ""))
     or ($names | any(. as $r | $base | test($r; "i")))
     or ($cmds | any(. as $r | $cmd | test($r; "i")))
     or ($imgs | any(. as $r | ($img + " " + $d) | test($r; "i"))));
def norm($s): ($s // "") | ascii_downcase | ltrimstr("sha256:");
def identity_reasons:  # IDENTITY-CHECK
  . as $r
  | ($r.answering // null) as $a
  | ($r.expected // null) as $e
  | [ (if ($r.artifact.fingerprint | present) then empty else "artifact_fingerprint_missing" end),
      (if (($r.endpoint.host | present) and ($r.endpoint.port != null)) then empty else "endpoint_identity_missing" end),
      (if ($a == null or ($a.kind | present | not)
            or ($a.kind == "process" and (($a.pid | type) != "number" or ($a.exe_path | present | not) or ($a.exe_sha256 | present | not)))
            or ($a.kind == "container" and ($a.image_digest | present | not))
            or ($a.kind != "process" and $a.kind != "container" and $a.kind != "socket")
            or ($a.kind == "socket" and ($a.exe_sha256 | present | not)))
       then "answering_identity_missing"
       else
         ( (if is_double($a) then "registered_double" else empty end),
           (if $e == null or (($e.exe_sha256 | present | not) and ($e.image_digest | present | not)) then "expected_identity_missing"
            elif ($e.exe_sha256 | present) and norm($e.exe_sha256) != norm($a.exe_sha256) then "identity_mismatch"
            elif ($e.image_digest | present) and norm($e.image_digest) != norm($a.image_digest) then "identity_mismatch"
            else empty end) )
       end) ];  # END-IDENTITY
def skip_set: ["hardware_not_present","credentials_absent","topology_unsupported","operator_attended","structurally_impossible","feature_disabled_by_config","artifact_not_yet_built"];
def invalid_reasons:
  . as $r
  | [ (if $r.schema == "catalogizer.live-evidence/1" then empty else "schema" end),
      (if ($r.test_id | present) and ($r.command | present) and ($r.timestamp | present) then empty else "required_field_missing" end),
      (if ($r.class == "LIVE" or $r.class == "UNIT") then empty else "class" end),
      (if ($r.verdict == "PASS" or $r.verdict == "FAIL" or $r.verdict == "SKIP") then empty else "verdict" end),
      (if $r.verdict == "SKIP" and (skip_set | index($r.skip_reason) | not) then "skip_reason_not_in_closed_set" else empty end),
      (if $r.verdict != "SKIP" and (($r.runs.n | type) != "number" or $r.runs.n < 1 or $r.runs.identical != true or ($r.runs.signature | present | not)) then "runs_not_identical_or_missing" else empty end) ];
. as $r
| ($r | invalid_reasons) as $inv
| if ($r | type) != "object" then {class:"INVALID", reasons:["not_an_object"]}
  elif ($inv | length) > 0 then {class:"INVALID", reasons:$inv}
  elif $r.verdict == "SKIP" then {class:"SKIP", reasons:[$r.skip_reason]}
  elif $r.class == "UNIT" then {class:"UNIT", reasons:["declared_unit"]}
  else ($r | identity_reasons) as $ir | if ($ir | length) == 0 then {class:"LIVE", reasons:[]} else {class:"UNIT", reasons:$ir} end
  end
JQ
reg=$(grep -v '^[[:space:]]*#' "$DOUBLES")
worst=0
for f in "${files[@]}"; do
  if ! jq -e . "$f" >/dev/null 2>&1; then echo "$f class=INVALID reasons=unparseable_json"; worst=2; continue; fi
  if ! res=$(jq -c --arg reg "$reg" "$PROG" "$f" 2>&1); then echo "$f class=INVALID reasons=verifier_error:$(printf %s "$res" | head -c 200)"; worst=2; continue; fi
  cls=$(jq -r .class <<<"$res"); rs=$(jq -r '.reasons | join(",")' <<<"$res")
  echo "$f class=$cls reasons=${rs:-none}"
  declared=$(jq -r '.class // ""' "$f")
  case $cls in
    INVALID) worst=2;;
    UNIT) if [ "$declared" = LIVE ] || [ "$REQUIRE_LIVE" = 1 ]; then [ "$worst" = 2 ] || worst=1; fi;;
  esac
done
exit "$worst"
