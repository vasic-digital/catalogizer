#!/usr/bin/env bash
# Reviewer-style mutation suite for the interpreter allow-list (constitution 11.4.289(D), gate-logic UNIT test; paired 1.1 mutations).
# Stages (env STAGES, default all): table = ONE MEMBER AT A TIME table mutants against the verifier (the Go and Node in-process variants run in their own suites);
# vcode / ncode / gcode = source mutants of the verifier jq, the Node parser and the Go parser. Every mutant MUST be killed by the frozen option cases; a control run
# with the unmutated source MUST pass first. Serial; run under: systemd-run --user --scope -p MemoryMax=3G -p MemorySwapMax=0
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); ROOT=$(cd "$HERE/../.." && pwd)
STAGES=${STAGES:-table vcode ncode gcode}
M=$(mktemp -d "${TMPDIR:-/tmp}/lev-om.XXXXXX"); GM=$ROOT/catalog-api/internal/liveevidence_mut   # GM is a scratch copy of the Go package inside the module (go test needs it in-tree); the trap removes it on ANY exit
trap 'rm -rf "${M:?}" "${GM:?}"' EXIT
REALTBL=$ROOT/docs/qa/live-evidence.interpreter-options.json
export LIVE_EVIDENCE_INTERP_OPTIONS=$REALTBL LIVE_EVIDENCE_DOUBLES=$ROOT/docs/qa/live-evidence.doubles.txt
VSRC=$ROOT/scripts/test-infra/verify_live_evidence.sh
survived=0; killed=0
res() { if [ "$2" = killed ]; then killed=$((killed+1)); else survived=$((survived+1)); echo "SURVIVED: $1" >&2; fi; }
differs() { cmp -s "$1" "$2" && { echo "FAIL: mutant $3 identical to source (pattern lost)" >&2; survived=$((survived+1)); return 1; }; return 0; }
case " $STAGES " in *" table "*)
  # control: the unmutated table passes
  FAILFAST=1 bash "$HERE/test_option_cases.sh" "$VSRC" 'select(.member|startswith("node:benign:--no-warnings"))' >/dev/null 2>&1 || { echo "FAIL: control (unmutated table) must pass" >&2; survived=$((survived+1)); }
  while IFS=$'\t' read -r kind fam idx name; do
    mt=$M/t.json
    case $kind in
      rb) jq --arg f "$fam" --argjson i "$idx" '.families[$f].benign |= del(.[$i])' "$REALTBL" > "$mt"; member="$fam:benign:$name";;
      rd) jq --arg f "$fam" --argjson i "$idx" '.families[$f].dangerous |= del(.[$i])' "$REALTBL" > "$mt"; member="$fam:dangerous:$name";;
      mv) jq --arg f "$fam" --argjson i "$idx" --arg o "$name" '.families[$f].dangerous |= del(.[$i]) | .families[$f].benign += [{opt:$o,val:"none"}]' "$REALTBL" > "$mt"; member="$fam:dangerous:$name";;
    esac
    if LIVE_EVIDENCE_INTERP_OPTIONS=$mt FAILFAST=1 bash "$HERE/test_option_cases.sh" "$VSRC" "select(.member == \"$member\" or (.member | startswith(\"$member:\")))" >/dev/null 2>&1 </dev/null; then res "table $kind $fam $name" survived; else res "table $kind $fam $name" killed; fi
  done < <(jq -r '.families | to_entries[] | .key as $f | ((.value.benign | to_entries[] | "rb\t\($f)\t\(.key)\t\(.value.opt)"), (.value.dangerous | to_entries[] | "rd\t\($f)\t\(.key)\t\(.value)", (select(.value | startswith("prefix:") | not) | "mv\t\($f)\t\(.key)\t\(.value)")))' "$REALTBL")
;; esac
# NOTE: EQUIVALENT mutant, deliberately not listed: dropping the '(iresult($A).script == null) or' disjunct of script_not_in_cmdline changes nothing, because scriptmatch(null) is false (tok must be a string) so the right disjunct already fires; the relative-script dotdot/dot-slash mutants were removed with the code they mutated (a relative token never binds; scriptmatch-relative-accepted + absolute-script-endswith cover that path).
# NOTE: no verifier mutant for the ctl() token check: it is defense-in-depth, EQUIVALENT in the verifier (cmdline_malformed INVALID pre-empts it); Node and Go mutants for it are killed.
case " $STAGES " in *" vcode "*)
  FAILFAST=1 bash "$HERE/test_option_cases.sh" "$VSRC" 'true' >/dev/null 2>&1 || { echo "FAIL: verifier control run must pass" >&2; survived=$((survived+1)); }
  i=0
  while IFS=$'\t' read -r name from to; do
    if [ -n "${ONLY:-}" ] && ! [[ $name =~ $ONLY ]]; then continue; fi   # ONLY=<regex> runs just the matching source mutants
    i=$((i+1)); mv_=$M/v$i.sh
    python3 - "$VSRC" "$mv_" "$from" "$to" <<'PY'
import sys
s=open(sys.argv[1]).read(); f=sys.argv[3].replace('\\n','\n').replace('\\t','\t'); t=sys.argv[4].replace('\\n','\n').replace('\\t','\t')
open(sys.argv[2],'w').write(s.replace(f,t,1))
PY
    chmod +x "$mv_"; differs "$mv_" "$VSRC" "v:$name" || continue
    vout=$("$mv_" "$HERE/fixtures/golden_good.json" 2>&1 </dev/null); case $vout in *" class="*) case $vout in *verifier_error*) echo "FAIL: verifier mutant $name does not run (not a kill): $vout" >&2; survived=$((survived+1)); continue;; esac;; *) echo "FAIL: verifier mutant $name does not run (not a kill): $vout" >&2; survived=$((survived+1)); continue;; esac
    if FAILFAST=1 bash "$HERE/test_option_cases.sh" "$mv_" 'true' >/dev/null 2>&1 </dev/null && bash "$HERE/test_verifier_binding.sh" "$mv_" >/dev/null 2>&1 </dev/null; then res "vcode $name" survived; else res "vcode $name" killed; fi
  done <<'MUT'
danger-skipped	elif isdanger($f; $x) then	elif false then
danger-no-eq-split	($t | if startswith("--") and contains("=") then split("=")[0] else . end) as $n	($t) as $n
step-no-eq-split	| (if $has then ($x | split("=")[0]) else $x end) as $n	| $x as $n
prefix-dropped	($t | startswith($d[7:]))	false
only-first-two-tokens	if $i >= ($t | length) then {st: "ok", script: null}	if $i >= 2 or $i >= ($t | length) then {st: "ok", script: null}
attached-re-dropped	($x | .[($b.opt | length):] | test($b.re))	true
eq-re-dropped	(if $has and (($e.re | not) or ($v | test($e.re))) then {adv: 1}	(if $has then {adv: 1}
next-eq-re-dropped	(if (($e.re | not) or ($v | test($e.re))) and ($e.script | not) then {adv: 1}	(if ($e.script | not) then {adv: 1}
next-value-re-dropped	and (($e.re | not) or ($t[$i + 1] | test($e.re))) then	then
none-accepts-value	if $e.val == "none" then (if $has then {st: "unknown"} else {adv: 1} end)	if $e.val == "none" then {adv: 1}
unknown-option-accepted	if $at != null then {adv: 1} else {st: "unknown"} end	{adv: 1}
script-binding-dropped	(scriptmatch($A.script_path; iresult($A).script) | not)	false
positional-script-ignored	(if (($f | has("positional_script")) and $f.positional_script == false) then null else $x end)	$x
unsupported-dropped	if $f == null and iunsupported($b) then	if false then
argv0-scanned	.[1:]))	.[0:]))
next-adv-1	else {adv: 2} end)	else {adv: 1} end)
jar-script-dropped	(if $e.script then {script: $t[$i + 1]} else {adv: 2} end)	{adv: 2}
eq-needs-no-value	elif $e.val == "eq" then (if $has and	elif $e.val == "eq" then (if
next-eq-script-allowed	and ($e.script | not) then {adv: 1}	then {adv: 1}
next-value-dash-allowed	(($t[$i + 1] | startswith("-")) | not) and	true and
absolute-script-endswith	($tok | startswith("/")) and ($sp == $tok);	($tok | startswith("/")) and ($sp | endswith($tok));
scriptmatch-relative-accepted	($tok | startswith("/")) and ($sp == $tok);	($sp | endswith("/" + ($tok | ltrimstr("./"))));
env-unrec-dropped	and ($A.env_checked != true) then "env_unrecorded"	and false then "env_unrecorded"
env-injected-dropped	and ($A.env_checked == true) and ((($A.env_injected	and false and ((($A.env_injected
plus-token-positional	elif (($x | startswith("-") or startswith("+")) | not)	elif (($x | startswith("-")) | not)
fp-0x-dropped	|0x[0-9a-f])")	)")
fp-dash-dropped	sha-?256[:=]	sha256[:=]
fp-nonascii-dropped	 or test("[^\\x{20}-\\x{7e}]")	
host-type-dropped	((($r.endpoint.host | type) == "string") and 	(
interp-sh-dropped	|bash|sh|dash|	|bash|dash|
unknown-kind-accepted	     or ($k != "process" and $k != "container" and $k != "socket")	
envinj-nonarray-dropped	$ip and ($A.env_checked == true) and ((($A.env_injected // []) | type) != "array" or	$ip and ($A.env_checked == true) and (false or
env-unrec-interp-only	($k != "container") and ($A.env_checked != true) then "env_unrecorded"	$ip and ($A.env_checked != true) then "env_unrecorded"
env-absent-counts-clean	($k != "container") and ($A.env_checked != true) then "env_unrecorded"	($k != "container") and ($A.env_checked == false) then "env_unrecorded"
env-nonbool-counts-clean	($k != "container") and ($A.env_checked != true) then "env_unrecorded"	($k != "container") and (($A.env_checked // false) == false and ($A.env_checked | type) != "string") then "env_unrecorded"
loader-env-first-only	| map(select(. as $e | ($itbl[0].env_loader // []) | index($e))) | length) > 0)	| .[0:1] | map(select(. as $e | ($itbl[0].env_loader // []) | index($e))) | length) > 0)
loader-env-interp-only	($ip | not) and ($k != "container") and ((($A.env_injected	$ip and ($k != "container") and ((($A.env_injected
loader-env-dropped	($ip | not) and ($k != "container") and ((($A.env_injected	false and ($k != "container") and ((($A.env_injected
loader-env-nonarray-dropped	($ip | not) and ($k != "container") and ((($A.env_injected // []) | type) != "array" or	($ip | not) and ($k != "container") and (false or
loader-env-set-ignored	| index($e))) | length) > 0)) then "loader_env_injection"	| index($e))) | length) > 99)) then "loader_env_injection"
MUT
;; esac
case " $STAGES " in *" ncode "*)
  mkdir -p "$M/node"; cp "$HERE/option_cases.json" "$HERE/test_option_cases_node.mjs" "$HERE/test_env_node.mjs" "$HERE/test_host_node.mjs" "$M/node/"
  runn() { ( cd "$M/node" && LIVE_EVIDENCE_INTERP_OPTIONS=$REALTBL node test_option_cases_node.mjs >/dev/null 2>&1 </dev/null && node test_env_node.mjs >/dev/null 2>&1 </dev/null && node test_host_node.mjs >/dev/null 2>&1 </dev/null ); }
  cp "$HERE/index.mjs" "$M/node/index.mjs"; runn || { echo "FAIL: node control run must pass" >&2; survived=$((survived+1)); }
  while IFS=$'\t' read -r name from to; do
    if [ -n "${ONLY:-}" ] && ! [[ $name =~ $ONLY ]]; then continue; fi   # ONLY=<regex> runs just the matching source mutants
    python3 - "$HERE/index.mjs" "$M/node/index.mjs" "$from" "$to" <<'PY'
import sys
s=open(sys.argv[1]).read(); f=sys.argv[3].replace('\\n','\n').replace('\\t','\t'); t=sys.argv[4].replace('\\n','\n').replace('\\t','\t')
open(sys.argv[2],'w').write(s.replace(f,t,1))
PY
    differs "$M/node/index.mjs" "$HERE/index.mjs" "n:$name" || continue
    node --check "$M/node/index.mjs" >/dev/null 2>&1 || { echo "FAIL: node mutant $name has a syntax error (not a kill)" >&2; survived=$((survived+1)); continue; }
    if runn; then res "ncode $name" survived; else res "ncode $name" killed; fi
  done <<'MUT'
ctl-dropped	if (CTL.test(x)) return	if (false) return
danger-skipped	if (isDanger(fam, x)) return	if (false) return
danger-no-eq-split	const n = tok.startsWith('--') && tok.includes('=') ? tok.split('=')[0] : tok;	const n = tok;
step-no-eq-split	const n = has ? x.split('=')[0] : x;	const n = x;
prefix-dropped	tok.startsWith(d.slice(7))	false
only-first-two-tokens	for (let i = 0; i < toks.length;)	for (let i = 0; i < Math.min(toks.length, 2);)
attached-re-dropped	 && new RegExp(b.re).test(x.slice(b.opt.length))	
re-dropped	const reok = (b, val) => !b.re || new RegExp(b.re).test(val);	const reok = () => true;
none-accepts-value	if (has) return { st: 'unknown', script: null }; i += 1; }	i += 1; }
unknown-option-accepted	if (at) i += 1; else return { st: 'unknown', script: null };	i += 1;
positional-script-ignored	fam.positional_script === false ? null : x	x
unsupported-dropped	f === null && new RegExp(t.unsupported_exe, 'iu')	false && new RegExp(t.unsupported_exe, 'iu')
argv0-scanned	const toks = args.slice(1);	const toks = args;
next-adv-1	i += 2;	i += 1;
jar-script-dropped	if (e.script) return { st: 'ok', script: toks[i + 1] };	
eq-needs-no-value	else if (e.val === 'eq') { if (has && reok(e, v)) i += 1;	else if (e.val === 'eq') { if (reok(e, v)) i += 1;
next-eq-script-allowed	if (reok(e, v) && !e.script) i += 1;	if (reok(e, v)) i += 1;
next-value-dash-allowed	!toks[i + 1].startsWith('-') && reok	reok
renamed-guard-dropped	if (ifam(TABLE, path.basename(exe)) === null) return null;	
plus-token-positional	if (!x.startsWith('-') && !x.startsWith('+')) return	if (!x.startsWith('-')) return
fold-flag-dropped	new RegExp(t.families[k].exe, 'iu')	new RegExp(t.families[k].exe, 'i')
relative-bound	if (!path.isAbsolute(cand)) return null;	if (false) return null;
env-empty-counts	&& kv.slice(i + 1) !== ''	
env-key-set-ignored	(keys.has(k) || pfx.some	(false || pfx.some
env-prefix-dropped	|| pfx.some(	|| false && pfx.some(
env-bare-prefix-counts	k.length > p.length && 	
env-prefix-lookalike	k.startsWith(p)	k.includes(p)
env-unreadable-clean	catch { return { env_checked: false }; }	catch { return { env_checked: true }; }
localhost-v6-mapping	host === 'localhost' ? '127.0.0.1'	host === 'localhost' ? '::1'
unresolvable-host-open	const want = host === '' || host == null ? null : (host === 'localhost'	const want = host === '' || host == null || host.endsWith('.invalid') ? null : (host === 'localhost'
MUT
;; esac
case " $STAGES " in *" gcode "*)
  PK=$ROOT/catalog-api/internal/liveevidence
  rung() { ( cd "$ROOT/catalog-api" && systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 go test ./internal/liveevidence_mut/ -run 'TestFrozenOptionCases|TestReviewShapesAreNotBound|TestSyntheticTable|TestScriptOfBinding|TestEnvOfEveryKey|TestRelativeScriptTokenNeverBinds|TestUnresolvableHostFailsClosedSelfContained|TestAbsoluteScriptThatCannotBeHashedStaysUnbound' >/dev/null 2>&1 ); }
  mkdir -p "$GM"; cp "$PK"/*.go "$PK"/interp_options.json "$GM/"; rm -f "$GM/liveevidence_test.go"
  rung || { echo "FAIL: go control run must pass" >&2; survived=$((survived+1)); }
  while IFS=$'\t' read -r name from to; do
    if [ -n "${ONLY:-}" ] && ! [[ $name =~ $ONLY ]]; then continue; fi   # ONLY=<regex> runs just the matching source mutants
    python3 - "$PK/liveevidence.go" "$GM/liveevidence.go" "$from" "$to" <<'PY'
import sys
s=open(sys.argv[1]).read(); f=sys.argv[3].replace('\\n','\n').replace('\\t','\t'); t=sys.argv[4].replace('\\n','\n').replace('\\t','\t')
open(sys.argv[2],'w').write(s.replace(f,t,1))
PY
    differs "$GM/liveevidence.go" "$PK/liveevidence.go" "g:$name" || continue
    ( cd "$ROOT/catalog-api" && systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 go test -count=1 -run '^$' ./internal/liveevidence_mut/ >/dev/null 2>&1 </dev/null ) || { echo "FAIL: go mutant $name does not compile (a compile error is not a kill)" >&2; survived=$((survived+1)); continue; }
    if rung; then res "gcode $name" survived; else res "gcode $name" killed; fi
  done <<'MUT'
ctl-dropped	if ctlRE.MatchString(x) {	if false {
danger-skipped	if fam.danger(x) {	if false {
danger-no-eq-split	if strings.HasPrefix(tok, "--") && strings.Contains(tok, "=") {	if false {
step-no-eq-split	has := strings.HasPrefix(x, "--") && strings.Contains(x, "=")	has := false
prefix-dropped	if strings.HasPrefix(tok, strings.TrimPrefix(d, "prefix:")) {	if false {
only-first-two-tokens	for i := 0; i < len(toks); {	for i := 0; i < len(toks) && i < 2; {
attached-re-dropped	&& regexp.MustCompile(b.Re).MatchString(strings.TrimPrefix(x, b.Opt))	
re-dropped	func reOK(e optEntry, v string) bool { return e.Re == "" ||	func reOK(e optEntry, v string) bool { return true ||
none-accepts-value	case "none":\n\t\t\tif has {\n\t\t\t\treturn "unknown", ""\n\t\t\t}\n\t\t\ti++	case "none":\n\t\t\ti++
unknown-option-accepted	\t\t\tif !ok {\n\t\t\t\treturn "unknown", ""\n\t\t\t}\n\t\t\ti++	\t\t\t_ = ok\n\t\t\ti++
positional-script-ignored	if fam.PositionalScript != nil && !*fam.PositionalScript {	if false {
unsupported-dropped	if f == nil && regexp.MustCompile("(?i)"+t.UnsupportedExe).MatchString(base) {	if false {
argv0-scanned	toks = args[1:]	toks = args
next-adv-1	i += 2	i += 1
jar-script-dropped	if e.Script {\n\t\t\t\t\treturn "ok", toks[i+1]\n\t\t\t\t}\n\t\t\t\ti += 2	i += 2
eq-needs-no-value	if !has || !reOK(*e, v) {	if !reOK(*e, v) {
next-eq-script-allowed	if !reOK(*e, v) || e.Script {	if !reOK(*e, v) {
next-value-dash-allowed	!strings.HasPrefix(toks[i+1], "-") &&	
renamed-guard-dropped	if defaultInterpTable.family(filepath.Base(exe)) == nil {	if false {
plus-token-positional	if !strings.HasPrefix(x, "-") && !strings.HasPrefix(x, "+") {	if !strings.HasPrefix(x, "-") {
relative-bound	if !filepath.IsAbs(cand) {	if false {
env-empty-counts	kv[i+1:] != "" && !seen[kv[:i]]	!seen[kv[:i]]
env-key-set-ignored	(keys[kv[:i]] || hasEnvPrefix(t.EnvPrefixes, kv[:i]))	hasEnvPrefix(t.EnvPrefixes, kv[:i])
env-prefix-dropped	|| hasEnvPrefix(t.EnvPrefixes, kv[:i])	|| false
env-bare-prefix-counts	len(k) > len(p) && 	
env-prefix-lookalike	strings.HasPrefix(k, p)	strings.Contains(k, p)
env-unreadable-clean	if readErr != nil {\n\t\treturn false, nil	if false {\n\t\treturn false, nil
unresolvable-host-open	if !ok {\n\t\treturn in // unresolvable host	if !ok && false {\n\t\treturn in // unresolvable host
script-hash-not-checked-file	sum, err := FileSHA256(cand)\n\tif err != nil {\n\t\treturn "", ""\n\t}\n\treturn cand, sum	sum, _ := FileSHA256(cand)\n\treturn cand, sum
MUT
  rm -rf "${GM:?}"
;; esac
echo "option mutants: killed=$killed survived=$survived"
[ "$survived" = 0 ] && [ "$killed" -gt 0 ]
