#!/usr/bin/env bash
# T033 test (TDD, written BEFORE the verifier): the verifier's JSON validates against
# specs/001-full-project-audit-remediation/contracts/repo-verification-report.schema.json (repo-verification-report/1).
#
# Purpose   (1) fixture legs prove the schema check itself can see: a golden-good report passes; a report missing `summary`,
#           a report missing `no_remote`, a report whose `tool` is not the constant verify_repo.sh, a row with
#           `excepted: true` and a null or empty `exception_reason` (schema allOf[0]) and a row carrying an extra `reason`
#           property (additionalProperties false) are each REJECTED; (2) real-output legs run scripts/repo/verify_repos.sh on a
#           fixture tree in both `--json` and `--json-out` form and require the output to validate.
# Usage     RUNP IMG-TESTUTIL -- bash scripts/repo/tests/test_verify_schema.sh   (python3 jsonschema, jq, git)
#           Env: VR=<verifier path> (default scripts/repo/verify_repos.sh)
# RED       run before T032: the fixture legs pass, the real-output legs fail because the verifier does not exist.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
FEAT=specs/001-full-project-audit-remediation
SCHEMA="$FEAT/contracts/repo-verification-report.schema.json"
T="$(mktemp -d "${TMPDIR:-/tmp}/vs_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
valid() {  # valid <file> -> 0 when the file validates (stderr gets the first error)
  python3 - "$SCHEMA" "$1" <<'PY'
import json, sys
import jsonschema
schema, doc = json.load(open(sys.argv[1])), json.load(open(sys.argv[2]))
# The contract's $id is the RELATIVE string `repo-verification-report/1`. jsonschema 4.10 (the pinned images, python3-jsonschema 4.10.3)
# mis-joins a relative $id for the `#/$defs/...` refs (RefResolutionError: unknown url type), jsonschema >= 4.18 accepts it. The contract
# file is NOT changed: the in-memory copy gets an explicit absolute base (the relative $id resolved against a fixed non-routable base),
# which every jsonschema version resolves identically and which changes no constraint.
sid = schema.get("$id", "")
if "://" not in sid:
    schema = dict(schema, **{"$id": "https://contract.invalid/" + sid})
errs = sorted(jsonschema.Draft202012Validator(schema).iter_errors(doc), key=lambda e: list(map(str, e.path)))
for e in errs[:3]: print("schema error: " + e.message[:160], file=sys.stderr)
sys.exit(1 if errs else 0)
PY
}
# golden-good: the stored POC report plus the field the schema has required since contracts revision 9
jq '. + {no_remote:false}' "$FEAT/poc/repo_verify/results/run1.json" > "$T/good.json"
valid "$T/good.json" 2>/dev/null && ok "golden-good report validates" || bad "golden-good report rejected (the schema check is blind or the fixture is wrong)"
valid "$FEAT/poc/repo_verify/results/run1.json" 2>"$T/run1.err" && bad "POC run1.json validates although it lacks no_remote" || { grep -q "'no_remote' is a required property" "$T/run1.err" && ok "POC run1.json rejected for exactly the missing no_remote" || bad "POC run1.json rejected for another reason: $(cat "$T/run1.err")"; }
reject() {  # reject <name> <jq filter producing the broken report>
  jq "$2" "$T/good.json" > "$T/$1.json"
  if valid "$T/$1.json" 2>/dev/null; then bad "$1: the broken report was ACCEPTED"; else ok "$1: rejected"; fi
}
reject missing_summary 'del(.summary)'
reject missing_no_remote 'del(.no_remote)'
reject wrong_tool '.tool="verify_repos.sh"'
reject excepted_null_reason '.repos[0].excepted=true | .repos[0].exception_reason=null'
reject excepted_empty_reason '.repos[0].excepted=true | .repos[0].exception_reason=""'
reject extra_reason_property '.repos[0].reason="x"'
# real-output legs on a fixture tree (clean main repository plus a dirty submodule-free sibling is not needed: one repo is enough)
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
mkdir -p "$T/fx" && git init -q --bare -b main "$T/fx/r_a.git" && git clone -q "$T/fx/r_a.git" "$T/w_a" 2>/dev/null
( cd "$T/w_a" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null )
[ -x "$VR" ] || echo "NOTE: $VR is absent or not executable (RED: the real-output legs must fail)"
"$VR" --root "$T/w_a" --owned-orgs fx --exceptions /dev/null --quiet --json "$T/real_json.json" >/dev/null 2>&1
if [ -s "$T/real_json.json" ] && valid "$T/real_json.json" 2>"$T/e1"; then ok "real output via --json validates"; else bad "real output via --json: absent or invalid ($(head -c 200 "$T/e1" 2>/dev/null))"; fi
"$VR" --root "$T/w_a" --owned-orgs fx --exceptions /dev/null --quiet --json-out "$T/real_jsonout.json" >/dev/null 2>&1
if [ -s "$T/real_jsonout.json" ] && valid "$T/real_jsonout.json" 2>"$T/e2"; then ok "real output via --json-out validates"; else bad "real output via --json-out: absent or invalid ($(head -c 200 "$T/e2" 2>/dev/null))"; fi
# review fixes (WF-REVIEW): the report shapes the fixes introduce must validate too
# (a) an excepted row (6-column exception row with matching hashes), (b) a repository with no remote (unproven, empty remotes),
# (c) an uninitialised submodule row, (d) a detached owned submodule row
echo b >> "$T/w_a/f"
printf '.\tdirty\tf\t%s\t%s\tschema fixture\n' "$(sha256sum < "$T/w_a/f" | cut -c1-64)" "$(git -C "$T/w_a" cat-file blob HEAD:f | sha256sum | cut -c1-64)" > "$T/exc6.tsv"
"$VR" --root "$T/w_a" --owned-orgs fx --exceptions "$T/exc6.tsv" --quiet --json "$T/real_exc.json" >/dev/null 2>&1; rc=$?
if [ "$rc" -eq 0 ] && jq -e '.repos[0].excepted==true' "$T/real_exc.json" >/dev/null 2>&1 && valid "$T/real_exc.json" 2>"$T/e3"; then ok "real output with an excepted row validates (exit 0, excepted true)"; else bad "excepted-row output: rc=$rc or invalid ($(head -c 200 "$T/e3" 2>/dev/null))"; fi
git -C "$T/w_a" checkout -q -- f
git init -q -b main "$T/w_nr"; ( cd "$T/w_nr" && echo a > f && git add f && $G commit -qm c1 )
"$VR" --root "$T/w_nr" --owned-orgs fx --exceptions /dev/null --quiet --json "$T/real_nr.json" >/dev/null 2>&1; rc=$?
if [ "$rc" -eq 14 ] && valid "$T/real_nr.json" 2>"$T/e4"; then ok "real output for a repository with no remote validates (exit 14, unproven)"; else bad "no-remote output: rc=$rc or invalid ($(head -c 200 "$T/e4" 2>/dev/null))"; fi
mkdir -p "$T/ext" && git init -q --bare -b main "$T/ext/r_s.git" && git clone -q "$T/ext/r_s.git" "$T/w_s" 2>/dev/null
( cd "$T/w_s" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null )
( cd "$T/w_a" && $G submodule add -q "$T/ext/r_s.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null && git submodule deinit -q -f s 2>/dev/null; rm -rf s; mkdir s )
"$VR" --root "$T/w_a" --owned-orgs fx --exceptions /dev/null --quiet --json "$T/real_un.json" >/dev/null 2>&1; rc=$?
if [ "$rc" -eq 0 ] && jq -e '[.repos[]|select(.path=="s")|.pin_state]==["uninitialised"]' "$T/real_un.json" >/dev/null 2>&1 && valid "$T/real_un.json" 2>"$T/e5"; then ok "real output with an uninitialised submodule row validates"; else bad "uninitialised-row output: rc=$rc or invalid ($(head -c 200 "$T/e5" 2>/dev/null))"; fi
mkdir -p "$T/fx" && git init -q --bare -b main "$T/fx/r_o.git" && git clone -q "$T/fx/r_o.git" "$T/w_o" 2>/dev/null
( cd "$T/w_o" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null )
git clone -q "$T/w_a" "$T/w_a2" 2>/dev/null; rm -rf "$T/w_a2"
git init -q -b main "$T/w_p"; ( cd "$T/w_p" && echo a > f && git add f && $G commit -qm c1 && $G submodule add -q "$T/fx/r_o.git" os 2>/dev/null && $G commit -qm addos && git -C os checkout -q --detach )
"$VR" --root "$T/w_p" --owned-orgs fx --exceptions /dev/null --quiet --json "$T/real_det.json" >/dev/null 2>&1; rc=$?
if jq -e '[.repos[]|select(.path=="os")|.branch]==[""] and ([.repos[]|select(.path=="os")|.remotes[0].class]==["SAME"])' "$T/real_det.json" >/dev/null 2>&1 && valid "$T/real_det.json" 2>"$T/e6"; then ok "real output with a detached owned submodule row validates (class SAME)"; else bad "detached-row output: rc=$rc or invalid ($(head -c 200 "$T/e6" 2>/dev/null))"; fi
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) verifier=$(sha256sum "$VR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) jq=$(jq --version) jsonschema=$(python3 -c 'import jsonschema;print(jsonschema.__version__)') utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
