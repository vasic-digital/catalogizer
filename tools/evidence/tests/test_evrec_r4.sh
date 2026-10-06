#!/usr/bin/env bash
# T050 round 4 (independent review WF3-REVIEW-wp05-evrec, findings N1-N5 and the real-defect minors): cases written BEFORE
# the implementation changes and run RED against the round-3 tree first ($EV/wp05/T050r4-red.txt).
# Every refusal check is reason-aware (exit code AND reason=<name>). Secret-looking fixtures are assembled from pieces at run
# time so that no credential-shaped literal is committed (review minor 11).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; rm -rf "${S:?}"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "${S:?}/w"; mkdir -p "$S/w"; export EV=$S/w EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchor.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run "${1:-CAT-001}" "${2:-PROBE}" "${3:-1}" shell_script x -- "${@:4}"; }
refuse() { local name=$1 rc=$2 reason=$3; shift 3; local out r; out=$("$@" 2>&1); r=$?
  if [ "$r" = "$rc" ] && grep -q "reason=$reason" <<<"$out"; then ok "$name"; else bad "$name (rc=$r want $rc, reason=$reason; got: $(head -c 200 <<<"$out" | tr '\n' ' '))"; fi; }
want() { local name=$1 w=$2; shift 2; local r; "$@" >/dev/null 2>&1; r=$?; [ "$r" = "$w" ] && ok "$name" || bad "$name (rc=$r want $w)"; }
sha() { sha256sum | cut -d' ' -f1; }
OR=(--oracle specified --oracle-independent --evidence-class runtime)

# ===== N1: rerecord writes only into its own output directory, whatever --name and --out say (T050, 9.2)
fresh; mkdir -p "$S/n1"; printf '{"row":"R1"}\n' >"$S/n1/remote"; printf '{"row":"R1"}\n{"row":"L2"}\n' >"$S/n1/local"
printf '{"row":"shared-flake-1"}\n' >"$EV/flake_ledger.jsonl"; printf '{"row":"shared-def-1"}\n' >"$EV/deferrals.jsonl"; rec CAT-001 PROBE 1 true >/dev/null 2>&1
b1=$(sha <"$EV/flake_ledger.jsonl"); b2=$(sha <"$EV/deferrals.jsonl"); b3=$(sha <"$EV_LEDGER"); find "$EV" | sort | sha >"$S/n1/tree.before"
RR=("$EVREC" rerecord --append-only --onto "$S/n1/remote" --local "$S/n1/local")
refuse "N1: --name ../flake_ledger.jsonl (escape from --out) is refused" 64 name_not_plain_token "${RR[@]}" --out "$S/n1/out" --name ../../w/flake_ledger.jsonl
refuse "N1: --name <absolute path of a shared file> is refused" 64 name_not_plain_token "${RR[@]}" --out "$S/n1/out" --name "$EV/deferrals.jsonl"
for nm in "a/b" "." ".." "" "-x" ".hidden" "ledger-seq-map.json" "a b" "x/../y"; do
  refuse "N1: --name '$nm' is not a plain token and is refused" 64 name_not_plain_token "${RR[@]}" --out "$S/n1/out" --name "$nm"
done
refuse "N1: --out <subdirectory of the evidence dir> is refused (any depth)" 70 out_is_shared_store "${RR[@]}" --out "$EV/sub/deeper"
refuse "N1: --out <blob store> is refused" 70 out_is_shared_store "${RR[@]}" --out "$EV/blobs"
refuse "N1: --out <subdirectory of the blob store> is refused" 70 out_is_shared_store "${RR[@]}" --out "$EV/blobs/x"
mkdir -p "$S/n1/bl"; ln -s "$EV" "$S/n1/lnk"
refuse "N1: --out through a symlink into the evidence dir is refused" 70 out_is_shared_store "${RR[@]}" --out "$S/n1/lnk/sub"
refuse "N1: --out inside a relocated blob store (EV_BLOBS elsewhere) is refused" 70 out_is_shared_store env EV_BLOBS="$S/n1/bl" "${RR[@]}" --out "$S/n1/bl/x"
[ "$(sha <"$EV/flake_ledger.jsonl")" = "$b1" ] && [ "$(sha <"$EV/deferrals.jsonl")" = "$b2" ] && [ "$(sha <"$EV_LEDGER")" = "$b3" ] && ok "N1: the shared flake ledger, deferrals and ledger are byte-unchanged after all refused attempts" || bad "N1: a shared file changed"
[ "$(find "$EV" | sort | sha)" = "$(cat "$S/n1/tree.before")" ] && ok "N1: no file was created anywhere in the evidence directory by the refused attempts" || bad "N1: the evidence directory tree changed"
# the evidence directory and the ledger directory are separate protections: a ledger kept elsewhere does not unprotect $EV
mkdir -p "$S/n1/otherled"; refuse "N1: --out inside the evidence dir is refused even when the ledger lives in another directory" 70 out_is_shared_store env EV_LEDGER="$S/n1/otherled/ledger.jsonl" "${RR[@]}" --out "$EV/sub2"
want "N1: a plain --name into a separate --out still works" 0 "${RR[@]}" --out "$S/n1/out2" --name merged.jsonl
[ "$(tr -d '\n' <"$S/n1/out2/merged.jsonl" 2>/dev/null)" = '{"row":"R1"}{"row":"L2"}' ] && ok "N1: that output holds remote then the local suffix" || bad "N1: output content wrong"

# ===== N2: redaction covers the JSON key-value form, argv credential-flag pairs, Authorization schemes, cookies, JWT shapes
mk() { printf '%s' "$1$2$3"; }
V1=$(mk Hunter2 Hunt Er2); V2=$(mk Pw0rd Spaced 9); V3=$(mk AbCdEf 12 3456); V4=$(mk 'dXNlcjpw' 'YXNzd29y' 'ZDEyMw=='); V5=$(mk sess 'ionvalue' 77)
JWT=$(mk 'eyJhbGciOiJIUzI1NiJ9' '.eyJzdWIiOiJ4eXoifQ' '.abcDEFghi123')
stdout_case() { # stdout_case NAME PAYLOAD SECRET... : the secrets must not survive in blob or ledger, `redacted` must be set
  local name=$1 payload=$2; shift 2; fresh; P=$payload; export P
  "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c 'printf "%s\n" "$P"' >/dev/null 2>&1; unset P
  local blob; blob=$EV_BLOBS/$(jq -r .stdout_sha256 "$EV_LEDGER" 2>/dev/null); local leak=0 s
  for s in "$@"; do grep -qF -- "$s" "$blob" 2>/dev/null && leak=1; grep -qF -- "$s" "$EV_LEDGER" 2>/dev/null && leak=1; done
  [ -s "$blob" ] || leak=1
  if [ $leak = 0 ] && [ "$(jq -r .redacted "$EV_LEDGER")" = true ]; then ok "$name"; else bad "$name (secret survived or redacted flag missing: $(head -c 120 "$blob" 2>/dev/null | tr '\n' ' '))"; fi; }
stdout_case "N2: JSON login body {\"username\":..,\"password\":..} on stdout" "{\"username\":\"admin\",\"password\":\"$V1\"}" "$V1"
stdout_case "N2: JSON token {\"access_token\":\"<jwt>\"} on stdout" "{\"access_token\":\"$JWT\"}" "$JWT"
stdout_case "N2: JSON \"api_key\": \"...\" with a space after the colon" "{\"api_key\": \"$V3\"}" "$V3"
stdout_case "N2: JSON client_secret and refresh_token keys" "{\"client_secret\":\"$V2\",\"refresh_token\":\"$V5\"}" "$V2" "$V5"
stdout_case "N2: single-quoted key form 'password': 'x'" "{'password': '$V1'}" "$V1"
stdout_case "N2: password value containing a space inside JSON quotes" "{\"password\":\"$V2 and more\"}" "$V2" "and more"
stdout_case "N2: stream text '--password VALUE' pair" "run --password $V2 --verbose" "$V2"
stdout_case "N2: Authorization: Basic <base64> header" "Authorization: Basic $V4" "$V4"
stdout_case "N2: Authorization with a lower-case header and an unknown scheme" "authorization: Token $V3" "$V3"
stdout_case "N2: Cookie header value" "Cookie: session=$V5; theme=dark" "$V5"
stdout_case "N2: Set-Cookie header value" "Set-Cookie: sid=$V5; Path=/; HttpOnly" "$V5"
stdout_case "N2: a bare three-part JWT in prose" "got $JWT from the server" "$JWT"
stdout_case "N2: x-api-key header" "X-Api-Key: $V3" "$V3"
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c ':' --password "$V2" >/dev/null 2>&1
if ! grep -qF -- "$V2" "$EV_LEDGER" && [ "$(jq -r '.argv|index("--password")!=null' "$EV_LEDGER")" = true ] && [ "$(jq -r .redacted "$EV_LEDGER")" = true ]; then ok "N2: argv pair '--password VALUE' redacts the value, keeps the flag name, sets redacted"; else bad "N2: argv pair: $(jq -c .argv "$EV_LEDGER")"; fi
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c ':' --token "$V2" -p "$V5" --secret-key "$V3" --api-key="$V1" >/dev/null 2>&1
leak=0; for s in "$V2" "$V3" "$V1"; do grep -qF -- "$s" "$EV_LEDGER" && leak=1; done
[ $leak = 0 ] && ok "N2: argv pairs for --token, --secret-key and the --api-key=VALUE form are redacted" || bad "N2: argv credential flags leaked: $(jq -c .argv "$EV_LEDGER")"
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c ':' '{"password":"'"$V1"'"}' >/dev/null 2>&1
grep -qF -- "$V1" "$EV_LEDGER" && bad "N2: JSON password inside an argv element leaked" || ok "N2: JSON password inside an argv element is redacted"
fresh; P='{"username":"admin","note":"hello world","status":"ok basic configuration","retries":3} --verbose value'; export P
"$EVREC" run CAT-001 PROBE 1 shell_script x -- bash -c 'printf "%s\n" "$P"' >/dev/null 2>&1; unset P
[ "$(jq -r '.redacted // "absent"' "$EV_LEDGER")" = absent ] && cmp -s <(printf '%s\n' '{"username":"admin","note":"hello world","status":"ok basic configuration","retries":3} --verbose value') "$EV_BLOBS/$(jq -r .stdout_sha256 "$EV_LEDGER")" && ok "N2: benign JSON and flags are NOT redacted (no false positive)" || bad "N2: benign text was altered or flagged"

# ===== N3: rule 11 covers launchers (the project runner run_pinned.sh, kcov, container runners, remote shells), not only shells
mkdir -p "$S/n3/bin"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/n3/testA.sh"; printf '#!/usr/bin/env bash\nexit 0\n' >"$S/n3/testB.sh"; chmod +x "$S/n3/"test*.sh
printf '#!/usr/bin/env bash\nshift; [ "$1" = -- ] && shift; exec "$@"\n' >"$S/n3/run_pinned.sh"; chmod +x "$S/n3/run_pinned.sh"
cp "$S/n3/run_pinned.sh" "$S/n3/mylaunch.sh"
for L in kcov podman docker ssh adb pytest bats vitest jest xvfb-run strace uv poetry bundle rake flutter ctest; do
  printf '#!/bin/sh\nexit 0\n' >"$S/n3/bin/$L"; chmod +x "$S/n3/bin/$L"
  fresh; refuse "N3: RED through launcher '$L' without a declared test source is refused (interpreter_without_test_sources)" 69 interpreter_without_test_sources env PATH="$S/n3/bin:$PATH" "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- "$L" arg
done
fresh; refuse "N3: RED through the project runner run_pinned.sh without a declared test source is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/run_pinned.sh" IMG-KCOV -- bash "$S/n3/testA.sh"
refuse "N3: GREEN through run_pinned.sh without a declared test source is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 GREEN 1 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/run_pinned.sh" IMG-KCOV -- bash "$S/n3/testB.sh"
refuse "N3: MUTATION through run_pinned.sh without a declared test source is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 MUTATION 1 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/run_pinned.sh" IMG-KCOV -- bash "$S/n3/testB.sh"
refuse "N3: RED python3 -c without a declared test source is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- python3 -c 'raise SystemExit(1)'
refuse "N3: RED env bash -c without a declared test source is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- env bash -c 'exit 1'
refuse "N3: RED python3.12-style versioned interpreter without a source is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- python3.12 -c 'x'
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" --test-source "$S/n3/testA.sh" -- "$S/n3/run_pinned.sh" IMG-KCOV -- bash "$S/n3/testA.sh" >/dev/null 2>&1
"$EVREC" run CAT-001 GREEN 1 shell_script "$S/n3" "${OR[@]}" --test-source "$S/n3/testB.sh" -- "$S/n3/run_pinned.sh" IMG-KCOV -- bash "$S/n3/testB.sh" >/dev/null 2>&1
[ "$(jq -r .test_fingerprint "$EV_LEDGER" | sort -u | wc -l | tr -d ' ')" = 2 ] && ok "N3: with declared sources RED and GREEN through run_pinned.sh get different test_fingerprints" || bad "N3: fingerprints equal: $(jq -r .test_fingerprint "$EV_LEDGER" | tr '\n' ' ')"
# the fingerprint formula is pinned: sha256 of the lines sha256(argv0 file), then each declared source, then (only for a command outside every list) each file operand
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" --test-source "$S/n3/testA.sh" -- bash "$S/n3/testA.sh" >/dev/null 2>&1
want_fp=$( { sha256sum <"$(command -v bash)" | cut -d' ' -f1; sha256sum <"$S/n3/testA.sh" | cut -d' ' -f1; } | sha )
[ "$(jq -r .test_fingerprint "$EV_LEDGER")" = "$want_fp" ] && ok "N3: test_fingerprint of an interpreter with a declared source is exactly sha256(sha256(argv0)\\nsha256(source)\\n); operands are not added" || bad "N3: test_fingerprint of an interpreter with a declared source is $(jq -r .test_fingerprint "$EV_LEDGER") want $want_fp"
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/mylaunch.sh" X -- bash "$S/n3/testA.sh" >/dev/null 2>&1
want_fp=$( { sha256sum <"$S/n3/mylaunch.sh" | cut -d' ' -f1; sha256sum <"$S/n3/testA.sh" | cut -d' ' -f1; } | sha )
[ "$(jq -r .test_fingerprint "$EV_LEDGER")" = "$want_fp" ] && ok "N3: test_fingerprint of an unlisted launcher is exactly sha256(sha256(argv0)\\nsha256(file operand)\\n)" || bad "N3: unlisted-launcher fingerprint $(jq -r .test_fingerprint "$EV_LEDGER") want $want_fp"
# a launcher outside every list: the file operands are hashed, so different test bytes cannot share one fingerprint
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/mylaunch.sh" X -- bash "$S/n3/testA.sh" >/dev/null 2>&1
"$EVREC" run CAT-001 GREEN 1 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/mylaunch.sh" X -- bash "$S/n3/testB.sh" >/dev/null 2>&1
fp=$(jq -r .test_fingerprint "$EV_LEDGER" 2>/dev/null | sort -u | wc -l | tr -d ' ')
[ "$fp" = 2 ] && ok "N3: an unlisted launcher: RED and GREEN over different test files get different test_fingerprints (operands hashed)" || bad "N3: unlisted launcher gives $fp distinct fingerprint(s) over different test bytes"
"$EVREC" run CAT-001 GREEN 2 shell_script "$S/n3" "${OR[@]}" -- "$S/n3/mylaunch.sh" X -- bash "$S/n3/testB.sh" >/dev/null 2>&1
t=$(jq -r .test_fingerprint "$EV_LEDGER" | tail -2 | sort -u | wc -l | tr -d ' ')
[ "$t" = 1 ] && ok "N3: the same operand bytes give the same test_fingerprint again (deterministic)" || bad "N3: same operands gave different fingerprints"

# ===== N4: reconcile counts only the declared run's entries (a shared ledger must not give a false PASS)
fresh; "$EVREC" run CAT-009 PROBE 1 shell_script x -- true >/dev/null 2>&1                 # a foreign entry already in the ledger
export EVREC_RUN_TOKEN=run4
"$EVREC" run CAT-001 RED 1 shell_script "$S/n3" "${OR[@]}" --test-source "$S/n3/testA.sh" -- bash "$S/n3/testA.sh" >/dev/null 2>&1
refuse "N4: a failing GREEN is refused after the run (nothing stored)" 65 record_invalid "$EVREC" run CAT-001 GREEN 1 shell_script "$S/n3" "${OR[@]}" --test-source "$S/n3/testA.sh" -- bash "$S/n3/testA.sh"
for i in 1 2 3; do "$EVREC" run CAT-001 GREEN "$i" shell_script "$S/n3" "${OR[@]}" --test-source "$S/n3/testB.sh" -- bash "$S/n3/testB.sh" >/dev/null 2>&1; done
unset EVREC_RUN_TOKEN
[ "$(wc -l <"$EV_LEDGER" | tr -d ' ')" = 5 ] && ok "N4: fixture: the ledger holds 5 entries, one foreign (the runner ran 5 commands)" || bad "N4: fixture ledger has $(wc -l <"$EV_LEDGER") entries"
refuse "N4: reconcile --run run4 --runner-count 5 is a mismatch (the foreign entry does not count, the failed GREEN left no entry)" 1 count_mismatch "$EVREC" reconcile --run run4 --runner-count 5
want "N4: reconcile --run run4 --runner-count 4 passes (its own 4 stored entries)" 0 "$EVREC" reconcile --run run4 --runner-count 4
EVREC_RUN_TOKEN=run4 want "N4: the token may come from EVREC_RUN_TOKEN" 0 "$EVREC" reconcile --runner-count 4
refuse "N4: another run's token sees 0 entries of run4" 1 count_mismatch "$EVREC" reconcile --run other --runner-count 4
refuse "N4: unscoped reconcile on a ledger that holds tokened entries is refused" 1 reconcile_unscoped_on_tokened_ledger "$EVREC" reconcile --runner-count 5
refuse "N4: a token that is not a plain token is refused" 64 usage_error "$EVREC" reconcile --run 'a b' --runner-count 4
refuse "N4: reconcile with an unknown option is refused" 64 usage_error "$EVREC" reconcile --runner-count 4 --frob x
refuse "N4: EVREC_RUN_TOKEN that is not a plain token refuses the run before the command runs" 64 usage_error env EVREC_RUN_TOKEN='a b' "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
# tampering with the run index cannot raise the count
cp "$EV_LEDGER.runs" "$S/runs.bak"; sed -i 's/"entry_hash":"[0-9a-f]\{4\}/"entry_hash":"0000/' "$EV_LEDGER.runs"
refuse "N4: a run-index row whose entry_hash does not match the ledger entry is not counted" 1 count_mismatch "$EVREC" reconcile --run run4 --runner-count 4
cp "$S/runs.bak" "$EV_LEDGER.runs"
fresh; "$EVREC" run CAT-001 PROBE 1 shell_script x -- true >/dev/null 2>&1; "$EVREC" run CAT-001 PROBE 2 shell_script x -- true >/dev/null 2>&1
want "N4: legacy: untokened runs on a ledger without tokened entries still reconcile unscoped" 0 "$EVREC" reconcile --runner-count 2

# ===== N5 (reviewer mutants W3-2, W3-5, W3-7, W3-8 as named checks)
# W3-2: a mismatched blob is FAIL (1) even when another blob is missing (a forgery must not be downgraded to UNVERIFIED)
fresh; rec CAT-001 PROBE 1 bash -c 'echo one' >/dev/null 2>&1; rec CAT-001 PROBE 2 bash -c 'echo two' >/dev/null 2>&1
b1=$(jq -r 'select(.seq==1).stdout_sha256' "$EV_LEDGER"); b2=$(jq -r 'select(.seq==2).stdout_sha256' "$EV_LEDGER")
printf 'forged\n' >"$EV_BLOBS/$b1"; rm -f "${EV_BLOBS:?}/${b2:?}"
refuse "W3-2: one forged blob plus one missing blob is FAIL 1 blob_mismatch, not UNVERIFIED 3" 1 blob_mismatch "$VERIFY"
# W3-7: an explicit --ledger still checks blobs
fresh; rec CAT-001 PROBE 1 bash -c 'echo one' >/dev/null 2>&1; mkdir -p "$S/w2"; cp "$EV_LEDGER" "$S/w2/ledger.jsonl"; cp -r "$EV_BLOBS" "$S/w2/blobs"
want "W3-7: verify --ledger beside its own blobs dir passes when intact" 0 env -u EV_BLOBS "$VERIFY" --ledger "$S/w2/ledger.jsonl"
printf 'forged\n' >"$S/w2/blobs/$(jq -r .stdout_sha256 "$S/w2/ledger.jsonl")"
refuse "W3-7: verify --ledger with a forged blob is FAIL 1 blob_mismatch" 1 blob_mismatch env -u EV_BLOBS "$VERIFY" --ledger "$S/w2/ledger.jsonl"
refuse "W3-7: verify --ledger --blobs DIR with a forged blob is FAIL 1 too" 1 blob_mismatch "$VERIFY" --ledger "$S/w2/ledger.jsonl" --blobs "$S/w2/blobs"
# m1: --ledger resolves the anchor beside the ledger, like the blobs
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; rm -rf "${S:?}/w3"; mkdir -p "$S/w3"; cp "$EV_LEDGER" "$S/w3/ledger.jsonl"; cp -r "$EV_BLOBS" "$S/w3/blobs"; printf '{"anchor":1}\n' >"$S/w3/anchors.jsonl"
refuse "m1: verify --ledger with anchors.jsonl beside the ledger is UNVERIFIED 3 anchor_not_compared" 3 anchor_not_compared env -u EV_ANCHOR -u EV_BLOBS "$VERIFY" --ledger "$S/w3/ledger.jsonl"
# W3-8: a JSON grant that is not a grant object is a held grant (conservative default, 11.4.201(4))
for body in '{}' '{"run_id":7}' '[]' '"x"' 'null' '{"run_id":null}'; do
  fresh; rm -rf "${S:?}/ct"; mkdir -p "$S/ct/.audit"; export EVREC_REPO_ROOT=$S/ct; printf '%s\n' "$body" >"$S/ct/.audit/commit_turn.json"
  refuse "W3-8: grant body $body is a malformed grant: commit_turn_held" 76 commit_turn_held "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
  [ ! -e "$EV_LEDGER" ] && ok "W3-8: nothing recorded under the malformed grant $body" || bad "W3-8: ledger written under $body"
done
hermetic_repo
# W3-5: an impossible calendar date is not an RFC 3339 date-time
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1
for ts in 2026-02-30T00:00:00Z 2026-13-01T00:00:00Z 2026-04-31T10:00:00Z 2026-10-05T24:00:00Z 2026-10-05T10:61:00Z; do
  jq -c --arg t "$ts" '.started_at=$t' "$EV_LEDGER" >"$S/rec.json"
  refuse "W3-5: check-record refuses started_at $ts" 65 record_invalid "$EVREC" check-record "$S/rec.json"
done
jq -c '.started_at="2026-02-28T23:59:59Z"' "$EV_LEDGER" >"$S/rec.json"; want "W3-5: a real date (2026-02-28T23:59:59Z) is accepted" 0 "$EVREC" check-record "$S/rec.json"

# ===== m2: remap-refs keeps the executable bit and writes through a symlink (the link stays a link)
mkdir -p "$S/m2"; printf 'see ledger#4\n' >"$S/m2/real.md"; chmod 755 "$S/m2/real.md"; ln -s real.md "$S/m2/link.md"
printf '[{"old":4,"new":9,"digest":"x"}]\n' >"$S/m2/map.json"; printf '%s\n' "$S/m2/link.md" >"$S/m2/list"
"$EVREC" remap-refs --map "$S/m2/map.json" --files-from "$S/m2/list" >/dev/null 2>&1
[ "$(stat -c %a "$S/m2/real.md")" = 755 ] && ok "m2: the executable bit survives remap-refs" || bad "m2: mode is $(stat -c %a "$S/m2/real.md")"
[ -L "$S/m2/link.md" ] && grep -q 'ledger#9' "$S/m2/real.md" && ok "m2: remap-refs through a symlink keeps the link and updates its target" || bad "m2: symlink replaced or target not updated"

# ===== m4: a deleted working directory is a named refusal, not a traceback
fresh; mkdir -p "$S/gone"; out=$( cd "$S/gone" && rmdir "$S/gone" && "$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1 ); r=$?
if [ "$r" = 64 ] && grep -q 'reason=usage_error' <<<"$out" && ! grep -q Traceback <<<"$out"; then ok "m4: evrec run in a deleted cwd is usage_error 64, no traceback"; else bad "m4: rc=$r $(head -c 160 <<<"$out" | tr '\n' ' ')"; fi

hermetic_repo
[ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: $(hermetic_host_calls) call(s) reached the host entry"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
