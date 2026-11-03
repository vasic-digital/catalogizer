#!/usr/bin/env bash
# verify_live_evidence.sh - verifier of LIVE-EVIDENCE records (constitution 11.4.289(D)). Guide: docs/scripts/verify_live_evidence.md. Schema: docs/qa/live-evidence.schema.json.
# A record that claims class LIVE is RECLASSIFIED UNIT (exit 1) when what answered is a registered double, does not match the artifact the claim names, or is not identified.
# Loopback, test-started, scratch-dir real services stay LIVE: only the identity of the answering executable/image decides.
# Exit: 0 all records fine | 1 at least one LIVE claim reclassified UNIT (or --require-live met a UNIT or a SKIP: a SKIP is never live proof) | 2 invalid record / usage / unreadable register (fail closed).
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
def is_sha($s): ($s // "") | test("^(sha256:)?[0-9a-fA-F]{64}$");
def interp($a): (($a.exe_path // "") | split("/") | (last // "")) | test("^(python[0-9.]*|pypy[0-9.]*|node|nodejs|deno|bun|java|ruby[0-9.]*|perl[0-9.]*|php[0-9.]*|lua[0-9.]*|bash|sh|dash|zsh|ksh|fish)$"; "i");
def identity_reasons:  # IDENTITY-CHECK
  . as $r
  | ($r.answering // null) as $a
  | ($r.expected // null) as $e
  | ($a // {}) as $A
  | ($e // {}) as $E
  | (($A.kind // "") == "process" and interp($A)) as $ip
  | (if $ip then $E.script_sha256 else ($E.exe_sha256 // $E.image_digest) end) as $bound
  | ($a == null or ($a.kind | present | not)
     or ($a.kind != "process" and $a.kind != "container" and $a.kind != "socket")
     or ($a.kind == "process" and (($a.pid | type) != "number" or ($a.exe_path | present | not) or ($a.exe_sha256 | present | not)))
     or ($a.kind == "container" and ($a.image_digest | present | not))
     or ($a.kind == "socket" and (($a.pid | type) != "number" or ($a.exe_path | present | not) or ($a.exe_sha256 | present | not)))) as $amiss
  | [ (if ($r.artifact.fingerprint | present) then empty else "artifact_fingerprint_missing" end),  # R:fp_missing
      (if (($r.endpoint.host | present) and ($r.endpoint.port != null)) then empty else "endpoint_identity_missing" end),  # R:endpoint_missing
      (if $r.verdict == "PASS" and (($r.runs.n | type) != "number" or $r.runs.n < 3) then "runs_below_3" else empty end),  # R:runs_below_3
      (if ($r.resolve_error | present) then "listener_unresolved_or_ambiguous" else empty end),  # R:resolve_error
      (if $amiss then "answering_identity_missing" else empty end),  # R:answering_missing
      (if ($amiss | not) and is_double($A) then "registered_double" else empty end),  # R:double
      (if ($amiss | not) and ($A.kind == "process") and ($A.cmdline | present | not) then "endpoint_identity_incomplete" else empty end),  # R:cmdline_missing
      (if ($amiss | not) and ($E.exe_sha256 | present | not) and ($E.image_digest | present | not) then "expected_identity_missing" else empty end),  # R:expected_missing
      (if ($amiss | not) and ($E.exe_sha256 | present) and ($A.kind != "container") and norm($E.exe_sha256) != norm($A.exe_sha256) then "identity_mismatch" else empty end),  # R:exe_mismatch
      (if ($amiss | not) and ($E.image_digest | present) and ($A.kind == "container") and norm($E.image_digest) != norm($A.image_digest) then "identity_mismatch" else empty end),  # R:image_mismatch
      (if ($amiss | not) and $ip and (($A.script_path | present | not) or ($A.script_sha256 | present | not)) then "interpreter_script_missing" else empty end),  # R:script_missing
      (if ($amiss | not) and $ip and ($A.script_sha256 | present) and (($E.script_sha256 | present | not) or norm($E.script_sha256) != norm($A.script_sha256)) then "interpreter_script_unbound" else empty end),  # R:script_unbound
      (if ($amiss | not) and ($bound | present) and is_sha($r.artifact.fingerprint) and norm($r.artifact.fingerprint) != norm($bound) then "fingerprint_mismatch" else empty end),  # R:fp_mismatch
      empty ];  # END-IDENTITY
def skip_set: ["hardware_not_present","credentials_absent","topology_unsupported","operator_attended","structurally_impossible","feature_disabled_by_config","artifact_not_yet_built"];
def invalid_reasons:
  . as $r
  | [ (if $r.schema == "catalogizer.live-evidence/1" then empty else "schema" end),  # R:schema
      (if ($r.test_id | present) and ($r.command | present) and ($r.timestamp | present) then empty else "required_field_missing" end),
      (if ($r.timestamp | type) == "string" and ($r.timestamp | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$")) then empty else "timestamp_not_rfc3339" end),  # R:timestamp
      (if ($r.class == "LIVE" or $r.class == "UNIT") then empty else "class" end),  # R:class
      (if ($r.verdict == "PASS" or $r.verdict == "FAIL" or $r.verdict == "SKIP") then empty else "verdict" end),  # R:verdict
      (if $r.verdict == "SKIP" and (skip_set | index($r.skip_reason) | not) then "skip_reason_not_in_closed_set" else empty end),  # R:skip_reason
      (if ($r.endpoint.port != null) and ((($r.endpoint.port | type) != "number") or ($r.endpoint.port != ($r.endpoint.port | floor)) or $r.endpoint.port < 1 or $r.endpoint.port > 65535) then "port_not_integer" else empty end),  # R:port
      (if ($r.runs | type) != "object" then "runs_missing" else empty end),  # R:runs_missing
      (if $r.verdict != "SKIP" and (($r.runs.n | type) != "number" or $r.runs.n < 1 or $r.runs.identical != true or ($r.runs.signature | present | not)) then "runs_not_identical_or_missing" else empty end),  # R:runs_ident
      empty ];
. as $r
| if ($r | type) != "object" then {class:"INVALID", reasons:["not_an_object"]}
  else ($r | invalid_reasons) as $inv
  | if ($inv | length) > 0 then {class:"INVALID", reasons:$inv}
  elif $r.verdict == "SKIP" then {class:"SKIP", reasons:[$r.skip_reason]}
  elif $r.class == "UNIT" then {class:"UNIT", reasons:["declared_unit"]}
  else ($r | identity_reasons) as $ir | if ($ir | length) == 0 then {class:"LIVE", reasons:[]} else {class:"UNIT", reasons:$ir} end
  end
  end
JQ
reg=$(grep -v '^[[:space:]]*#' "$DOUBLES")
worst=0
for f in "${files[@]}"; do
  if ! jq -e . "$f" >/dev/null 2>&1; then echo "$f class=INVALID reasons=unparseable_json"; worst=2; continue; fi
  nvals=$(jq -s 'length' "$f" 2>/dev/null)   # exactly one JSON value per record file: a second value could hide a UNIT verdict
  if [ "$nvals" != 1 ]; then echo "$f class=INVALID reasons=not_exactly_one_json_value(${nvals:-?})"; worst=2; continue; fi
  if ! res=$(jq -c --arg reg "$reg" "$PROG" "$f" 2>&1); then echo "$f class=INVALID reasons=verifier_error:$(printf %s "$res" | head -c 200)"; worst=2; continue; fi
  cls=$(jq -r .class <<<"$res"); rs=$(jq -r '.reasons | join(",")' <<<"$res")
  echo "$f class=$cls reasons=${rs:-none}"
  declared=$(jq -r '.class // ""' "$f")
  case $cls in
    INVALID) worst=2;;
    SKIP) [ "$REQUIRE_LIVE" = 1 ] && { [ "$worst" = 2 ] || worst=1; };;
    UNIT) if [ "$declared" = LIVE ] || [ "$REQUIRE_LIVE" = 1 ]; then [ "$worst" = 2 ] || worst=1; fi;;
  esac
done
exit "$worst"
