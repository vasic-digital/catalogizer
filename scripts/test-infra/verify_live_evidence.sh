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
ITBL=${LIVE_EVIDENCE_INTERP_OPTIONS:-$ROOT/docs/qa/live-evidence.interpreter-options.json}
[ -r "$ITBL" ] || { echo "verify_live_evidence: interpreter option table unreadable: $ITBL (REFUSED, fail closed)" >&2; exit 2; }
jq -e '(.families | type == "object" and length > 0) and (.unsupported_exe | type == "string") and ([.families[] | (.exe | type == "string") and (.benign | type == "array") and (.dangerous | type == "array") and ([.benign[] | (.opt | type == "string") and ((.val) as $v | ["none","next","eq","attached"] | index($v) != null)] | all)] | all)' "$ITBL" >/dev/null 2>&1 \
  || { echo "verify_live_evidence: interpreter option table malformed/empty: $ITBL (BLIND, REFUSED)" >&2; exit 2; }
[ -r "$DOUBLES" ] || { echo "verify_live_evidence: double register unreadable: $DOUBLES (REFUSED, fail closed)" >&2; exit 2; }
files=()
for p in "${paths[@]}"; do
  if [ -d "$p" ]; then while IFS= read -r f; do files+=("$f"); done < <(find "$p" -maxdepth 1 -name '*.json' | sort)
  elif [ -f "$p" ]; then files+=("$p")
  else echo "verify_live_evidence: no such file or directory: $p" >&2; exit 2; fi
done
[ ${#files[@]} -gt 0 ] || { echo "verify_live_evidence: no records found (empty is not a pass)" >&2; exit 2; }

read -r -d '' DEFS <<'JQ'
def ctl: (type == "string") and (test("[\\x{1}-\\x{1f}\\x{7f}]") or contains("\u0000"));
def glob2re: gsub("(?<c>[.+^$(){}|\\[\\]\\\\])"; "\\\(.c)") | gsub("\\*"; ".*") | gsub("\\?"; ".") | "^" + . + "$";
def rules($k): [$reg | split("\n")[] | select(startswith($k + ":")) | .[($k|length)+1:] | gsub("^\\s+|\\s+$"; "") | select(length > 0)];
def present: . != null and . != "";
# ONE normaliser for every hash comparison (exe, image digest, script, fingerprint, register rules): trim, lowercase, strip an optional sha256: prefix
def norm($s): if ($s | type) == "string" then ($s | ascii_downcase | ltrimstr("sha256:")) else "" end;
def hexok($s): (($s | type) == "string") and (norm($s) | test("\\A[0-9a-f]{64}\\z"));
def is_sha($s): hexok($s);
# a value that LOOKS like a hash (sha256: prefix in any case, or a long run of hex incl. fullwidth digits) but does not normalise to 64 lowercase hex
def hashlike($s): (($s | type) == "string") and (($s | ascii_downcase | test("\\A(sha-?256[:=]|0x[0-9a-f])")) or ($s | test("\\A[0-9a-fA-F\\x{ff10}-\\x{ff19}\\x{ff21}-\\x{ff26}\\x{ff41}-\\x{ff46}]{32,}\\z")));
def is_double($a):
  ([rules("sha256")[] | norm(.)] as $hashes
   | [rules("name")[] | glob2re] as $names
   | [rules("cmd")[] | glob2re] as $cmds
   | [rules("image")[] | glob2re] as $imgs
   | (($a.exe_path // "") | split("/") | (last // "")) as $base
   | ([norm($a.exe_sha256), norm($a.image_digest), norm($a.script_sha256)] | map(select(. != ""))) as $ids   # a sha256: rule matches the exe hash, the image digest AND the script hash
   | ($a.cmdline // "") as $cmd
   | ($a.image // "") as $img
   | (norm($a.image_digest)) as $d
   | ($hashes | any(. as $h | $ids | index($h)))
     or ($names | any(. as $r | $base | test($r; "i")))
     or ($cmds | any(. as $r | $cmd | test($r; "i")))
     or ($imgs | any(. as $r | ($img | test($r; "i")) or ($d | test($r; "i")))));
def interp($a): (($a.runtime_hint | present) or ((($a.exe_path // "") | split("/") | (last // "")) | test("^(python[0-9.]*|pypy[0-9.]*|node|nodejs|deno|bun|java|ruby[0-9.]*|perl[0-9.]*|php[0-9.]*|lua[0-9.]*|bash|sh|dash|zsh|ksh|fish)$"; "i")));
# ALLOW-LIST option parser (table: $itbl[0], docs/qa/live-evidence.interpreter-options.json). A "--name=value" token is split on the first "=" BEFORE any lookup.
# An option not listed as benign for the exe family is ambiguous (UNIT); options that load/run code the script hash does not cover are named "dangerous" (UNIT, own reason).
def ifam($b): ([$itbl[0].families | to_entries[] | select(.value.exe as $re | $b | test($re; "i")) | .value] | first // null);
def iunsupported($b): ($b | test($itbl[0].unsupported_exe; "i"));
def isdanger($f; $t):
  ($t | if startswith("--") and contains("=") then split("=")[0] else . end) as $n
  | any(($f.dangerous // [])[]; . as $d | if ($d | startswith("prefix:")) then ($t | startswith($d[7:])) else $d == $n end);
def iparse($f; $t):   # tokens AFTER argv0 -> {st: ok|codeload|unknown, script}
  def step($i):
    $t[$i] as $x
    | ($x | startswith("--") and contains("=")) as $has
    | (if $has then ($x | split("=")[0]) else $x end) as $n
    | (if $has then ($x | .[($n | length) + 1:]) else null end) as $v
    | ([($f.benign // [])[] | select(.opt == $n)] | first // null) as $e
    | if $e != null then
        if $e.val == "none" then (if $has then {st: "unknown"} else {adv: 1} end)
        elif $e.val == "eq" then (if $has and (($e.re | not) or ($v | test($e.re))) then {adv: 1} else {st: "unknown"} end)
        elif $e.val == "next" then
          (if $has then (if (($e.re | not) or ($v | test($e.re))) and ($e.script | not) then {adv: 1} else {st: "unknown"} end)
           elif $i + 1 < ($t | length) and (($t[$i + 1] | startswith("-")) | not) and (($e.re | not) or ($t[$i + 1] | test($e.re))) then (if $e.script then {script: $t[$i + 1]} else {adv: 2} end)
           else {st: "unknown"} end)
        else {st: "unknown"} end
      else
        ([($f.benign // [])[] | select(.val == "attached" and (. as $b | ($x | startswith($b.opt)) and ($x | .[($b.opt | length):] | test($b.re))))] | first // null) as $at
        | if $at != null then {adv: 1} else {st: "unknown"} end
      end;
  def go($i):
    if $i >= ($t | length) then {st: "ok", script: null}
    else $t[$i] as $x
      | if ($x | ctl) then {st: "unknown", script: null}
        elif (($x | startswith("-") or startswith("+")) | not) then {st: "ok", script: (if (($f | has("positional_script")) and $f.positional_script == false) then null else $x end)}
        elif isdanger($f; $x) then {st: "codeload", script: null}
        else step($i) as $r
          | if $r.st == "unknown" then {st: "unknown", script: null}
            elif $r.script != null then {st: "ok", script: $r.script}
            else go($i + $r.adv) end
        end
    end;
  go(0);
def scriptmatch($sp; $tok):   # cooperative-producer binding: ONLY an absolute script token binds, and it must equal script_path exactly (a relative token is never guessed)
  (($sp | type) == "string") and (($tok | type) == "string") and ($tok | startswith("/")) and ($sp == $tok);
def iresult($A):   # the parse of THIS answer's command line; {st, script}
  (($A.exe_path // "") | split("/") | (last // "")) as $b
  | (ifam($b)) as $f
  | if $f == null and iunsupported($b) then {st: "unsupported", script: null}
    else iparse($f // {}; (($A.cmdline // "") | split(" ") | map(select(length > 0)) | .[1:]))
    end;
def idkeys: ["exe_sha256","image_digest","script_sha256"];
def identity_reasons:  # IDENTITY-CHECK
  . as $r
  | ($r.answering // null) as $a
  | ($r.expected // null) as $e
  | ($a // {}) as $A
  | ($e // {}) as $E
  | ($A.kind // "") as $k
  | (($k == "process" or $k == "socket") and interp($A)) as $ip     # a socket answer is held to the same interpreter/script/cmdline rules as a process answer
  | (if $k == "container" then "image_digest" else "exe_sha256" end) as $reqkey                       # the expected key the answering kind needs
  | (if $k == "container" then "image_digest" elif $ip then "script_sha256" else "exe_sha256" end) as $bk   # the identity of the answering kind the fingerprint binds to
  | ($a == null or ($a.kind | present | not)
     or ($k != "process" and $k != "container" and $k != "socket")
     or (($k == "process" or $k == "socket") and (($a.pid | type) != "number" or $a.pid < 1 or $a.pid != ($a.pid | floor) or ($a.exe_path | present | not) or ($a.exe_sha256 | present | not)))
     or ($k == "container" and ($a.image_digest | present | not))) as $amiss
  | [ (if ($r.artifact.fingerprint | present) then empty else "artifact_fingerprint_missing" end),  # R:fp_missing
      (if ((($r.endpoint.host | type) == "string") and ($r.endpoint.host | present) and ($r.endpoint.port != null)) then empty else "endpoint_identity_missing" end),  # R:endpoint_missing
      (if $r.verdict == "PASS" and (($r.runs.n | type) != "number" or $r.runs.n < 3) then "runs_below_3" else empty end),  # R:runs_below_3
      (if ($r.resolve_error | present) then "listener_unresolved_or_ambiguous" else empty end),  # R:resolve_error
      (if $amiss then "answering_identity_missing" else empty end),  # R:answering_missing
      (if ($amiss | not) and is_double($A) then "registered_double" else empty end),  # R:double
      (if ($amiss | not) and ($k != "container") and ($A.cmdline | present | not) then "endpoint_identity_incomplete" else empty end),  # R:cmdline_missing
      (if ($amiss | not) and ([idkeys[] | select($E[.] | present)] | length) == 0 then "expected_identity_missing" else empty end),  # R:expected_missing
      (if ($amiss | not) and ([idkeys[] | select($E[.] | present)] | length) > 0 and ($E[$reqkey] | present | not) then "expected_kind_mismatch" else empty end),  # R:kind_mismatch
      (if ($amiss | not) and ([idkeys[] as $f | select(($E[$f] | present) and norm($E[$f]) != norm($A[$f]))] | length) > 0 then "identity_mismatch" else empty end),  # R:identity_mismatch
      (if ($amiss | not) and $ip and (($A.script_path | present | not) or ($A.script_sha256 | present | not)) then "interpreter_script_missing" else empty end),  # R:script_missing
      (if ($amiss | not) and $ip and iresult($A).st == "codeload" then "interpreter_code_loading_option" else empty end),  # R:codeload
      (if ($amiss | not) and $ip and iresult($A).st == "unknown" then "interpreter_option_unknown" else empty end),  # R:optunknown
      (if ($amiss | not) and $ip and iresult($A).st == "unsupported" then "interpreter_family_unsupported" else empty end),  # R:unsupported
      (if ($amiss | not) and $ip and iresult($A).st == "ok" and ($A.script_path | present) and ((iresult($A).script == null) or (scriptmatch($A.script_path; iresult($A).script) | not)) then "script_not_in_cmdline" else empty end),  # R:scriptcmd
      (if ($amiss | not) and $ip and ($A.env_checked != true) then "env_unrecorded" else empty end),  # R:envunrec
      (if ($amiss | not) and $ip and ($A.env_checked == true) and ((($A.env_injected // []) | type) != "array" or (($A.env_injected // []) | length) > 0) then "interpreter_env_injection" else empty end),  # R:envinj
      (if ($amiss | not) and ($ip | not) and ($k != "container") and ((($A.env_injected // []) | type) != "array" or ((($A.env_injected // []) | map(select(. as $e | ($itbl[0].env_loader // []) | index($e))) | length) > 0)) then "loader_env_injection" else empty end),  # R:loaderenv (F1: loader keys bind every process kind)
      (if ($amiss | not) and $ip and ($E.script_sha256 | present | not) then "interpreter_script_unbound" else empty end),  # R:script_unbound
      (if ($amiss | not) and is_sha($r.artifact.fingerprint) and ((($A[$bk] | present) | not) or (norm($r.artifact.fingerprint) != norm($A[$bk]))) then "fingerprint_mismatch" else empty end),  # R:fp_mismatch
      empty ];  # END-IDENTITY
def skip_set: ["hardware_not_present","credentials_absent","topology_unsupported","operator_attended","structurally_impossible","feature_disabled_by_config","artifact_not_yet_built"];
def invalid_reasons:
  . as $r
  | [ (if $r.schema == "catalogizer.live-evidence/1" then empty else "schema" end),  # R:schema
      (if ($r.test_id | present) and ($r.command | present) and ($r.timestamp | present) then empty else "required_field_missing" end),
      (if ($r.timestamp | type) == "string" and ($r.timestamp | test("\\A[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})\\z") and ($r.timestamp | test("\\A[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:([0-5][0-9]|60)(\\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])\\z"))) then empty else "timestamp_not_rfc3339" end),  # R:timestamp
      (if ($r.class == "LIVE" or $r.class == "UNIT") then empty else "class" end),  # R:class
      (if ($r.verdict == "PASS" or $r.verdict == "FAIL" or $r.verdict == "SKIP") then empty else "verdict" end),  # R:verdict
      (if $r.verdict == "SKIP" and (skip_set | index($r.skip_reason) | not) then "skip_reason_not_in_closed_set" else empty end),  # R:skip_reason
      (if ([($r.answering // {}) | (.exe_sha256, .image_digest, .script_sha256)] + [($r.expected // {}) | (.exe_sha256?, .image_digest?, .script_sha256?)] | map(select(. != null and (hexok(.) | not))) | length) > 0 then "hash_malformed" else empty end),  # R:hash_malformed
      (if ($r.artifact.fingerprint != null) and (hashlike($r.artifact.fingerprint) and (hexok($r.artifact.fingerprint) | not)) then "fingerprint_malformed" else empty end),  # R:fp_malformed
      (if ($r.artifact.fingerprint != null) and ((($r.artifact.fingerprint | type) != "string") or ($r.artifact.fingerprint | test("\\A\\s*\\z") or ctl or test("[^\\x{20}-\\x{7e}]"))) then "fingerprint_malformed" else empty end),  # R:fp_type
      (if ($r.expected != null) and ((($r.expected | type) != "object") or ([$r.expected | keys[] | select(. as $k | idkeys | index($k) | not)] | length) > 0) then "expected_unknown_key" else empty end),  # R:expected_unknown
      (if (($r.answering // {}).exe_path != null) and ((($r.answering.exe_path | type) != "string") or ($r.answering.exe_path | endswith("/"))) then "exe_path_malformed" else empty end),  # R:exe_path
      (if (($r.answering // {}).exe_path != null) and (($r.answering.exe_path | type) == "string") and (($r.answering.exe_path | startswith("/") | not) or ($r.answering.exe_path | ctl)) then "exe_path_malformed" else empty end),  # R:exe_abs
      (if (($r.answering // {}).cmdline != null) and ((($r.answering.cmdline | type) != "string") or ($r.answering.cmdline | ctl)) then "cmdline_malformed" else empty end),  # R:cmdline_type
      (if (($r.answering // {}).script_path != null) and ((($r.answering.script_path | type) != "string") or ($r.answering.script_path | ctl)) then "script_path_malformed" else empty end),  # R:script_path_type
      (if ($r.endpoint.port != null) and ((($r.endpoint.port | type) != "number") or ($r.endpoint.port != ($r.endpoint.port | floor)) or $r.endpoint.port < 1 or $r.endpoint.port > 65535) then "port_not_integer" else empty end),  # R:port
      (if ($r.runs | type) != "object" then "runs_missing" else empty end),  # R:runs_missing
      (if $r.verdict != "SKIP" and (($r.runs.n | type) != "number" or $r.runs.n < 1 or $r.runs.identical != true or ($r.runs.signature | present | not)) then "runs_not_identical_or_missing" else empty end),  # R:runs_ident
      (if $r.verdict != "SKIP" and (($r.runs.n | type) == "number") and ($r.runs.n != ($r.runs.n | floor)) then "runs_not_identical_or_missing" else empty end),  # R:runs_n_int
      (if $r.verdict != "SKIP" and (($r.runs.signature | type) != "string" or ($r.runs.signature | test("\\A\\s*\\z"))) then "runs_not_identical_or_missing" else empty end),  # R:runs_sig_type
      empty ];
JQ
read -r -d '' MAIN <<'JQ'
. as $r
| if ($r | type) != "object" then {class:"INVALID", reasons:["not_an_object"]}
  else ($r | invalid_reasons) as $inv
  | if ($inv | length) > 0 then {class:"INVALID", reasons:($inv | reduce .[] as $x ([]; if index($x) then . else . + [$x] end))}
  elif $r.verdict == "SKIP" then {class:"SKIP", reasons:[$r.skip_reason]}
  elif $r.class == "UNIT" then {class:"UNIT", reasons:["declared_unit"]}
  else ($r | identity_reasons) as $ir | if ($ir | length) == 0 then {class:"LIVE", reasons:[]} else {class:"UNIT", reasons:$ir} end
  end
  end
JQ
PROG="$DEFS
$MAIN"
reg=$(grep -v '^[[:space:]]*#' "$DOUBLES")
# control needle (11.4.273): a register the instrument cannot see is BLIND. (1) at least one rule must load; (2) EVERY loaded rule must match a known-bad probe
# synthesised from that very rule (the needle), proving the double detection can actually fire for the rules it claims to apply.
if [ "$(printf '%s\n' "$reg" | grep -Ec '^(sha256|name|cmd|image):[^[:space:]]')" -lt 1 ]; then
  echo "verify_live_evidence: double register has no rules: $DOUBLES (BLIND, REFUSED)" >&2; exit 2
fi
NEEDLE='def sample: gsub("\\*"; "a") | gsub("\\?"; "a");
  ([rules("sha256")[] | {exe_sha256: .}] + [rules("name")[] | {exe_path: ("/x/" + sample)}] + [rules("cmd")[] | {cmdline: sample}] + [rules("image")[] | {image: sample}]) as $probes
  | {n: ($probes | length), missed: ([$probes[] | select(is_double(.) | not)] + [rules("sha256")[] | select(hexok(.) | not) | {malformed_sha256_rule: .}])}'
needle=$(jq -n -c --arg reg "$reg" --slurpfile itbl "$ITBL" "$DEFS
$NEEDLE" 2>&1) || { echo "verify_live_evidence: register control needle failed to run: $needle (BLIND, REFUSED)" >&2; exit 2; }
if [ "$(jq -r '.n' <<<"$needle")" -lt 1 ] || [ "$(jq -r '.missed | length' <<<"$needle")" != 0 ]; then
  echo "verify_live_evidence: register rule does not match its own known-bad probe: $needle (BLIND, REFUSED)" >&2; exit 2
fi
worst=0
for f in "${files[@]}"; do
  if ! jq -e . "$f" >/dev/null 2>&1; then echo "$f class=INVALID reasons=unparseable_json"; worst=2; continue; fi
  nvals=$(jq -s 'length' "$f" 2>/dev/null)   # exactly one JSON value per record file: a second value could hide a UNIT verdict
  if [ "$nvals" != 1 ]; then echo "$f class=INVALID reasons=not_exactly_one_json_value(${nvals:-?})"; worst=2; continue; fi
  if ! res=$(jq -c --arg reg "$reg" --slurpfile itbl "$ITBL" "$PROG" "$f" 2>&1); then echo "$f class=INVALID reasons=verifier_error:$(printf %s "$res" | head -c 200)"; worst=2; continue; fi
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
