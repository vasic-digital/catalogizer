#!/usr/bin/env bash
# WP-02 review WP-02-T028-r2 test (TDD, constitution 11.4.224): N1 class coverage, N2 verified live proofs and drift binding, N4 case behaviour,
# N5 the .env.example exemption, N7 mandatory capture age. Every case drives scripts/audit/index_health.sh through its real command line.
#
# Purpose   RED-first test for the round-9 review findings. Each case uses only options the previous tool already accepted, so against the
#           previous tool a case fails on the DEFECT (a forged proof accepted, an uncovered secret class indexed, a stale capture passed), never on argv.
# Usage     bash scripts/audit/tests/test_index_health_fix_r7.sh      Env: IH=<script under test> (default scripts/audit/index_health.sh)
# Exit      0 all ran and ok; 1 a case failed; 77 no case failed but a leg was skipped (SKIP is never a pass, distinct code, review N8)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
IH="${IH:-scripts/audit/index_health.sh}"
SCOPE="${SCOPE:-config/index/scope.yaml}"
T="$(mktemp -d "${TMPDIR:-/tmp}/ih_r7.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0; SKIPN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
skp() { SKIPN=$((SKIPN+1)); echo "SKIP $1 (reason recorded; not a pass)"; }
POS="a/main.go"; NEG="zz/none.go"; CNB="2026-10-06T00:00:00Z"; NOW="2026-10-08T12:00:00Z"
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
printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: 2026-10-07T10:00:00Z\n' > "$T/lu.txt"
base() {  # base [--opt value]...: defaults for every option the caller does not give
  local -A d=( [--scope]="$SCOPE" [--cg-status]="$T/st.json" [--cg-files]="$T/f_good.json" [--tracked]="$T/tracked.txt" [--needle-pos]="$POS" [--needle-neg]="$NEG" [--lumen-status]="$T/lu.txt" [--captured-not-before]="$CNB" )
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
  else bad "$1: want rc=$2 $3=$4 ${5:-}, got rc=$RC $3=$got reason=[$(reason "$3")]"; fi; }
p3with() {  # p3with <path>: the path is the only extra indexed file (and tracked); P3 judged
  files "$POS" a/b.go "$1" > "$T/f_p.json"; printf '%s\n' "$POS" a/b.go "$1" > "$T/tr_p.txt"
  base --cg-files "$T/f_p.json" --tracked "$T/tr_p.txt"
}
[ -x "$IH" ] || echo "NOTE: $IH absent or not executable (RED state)"

# ---- N1: every pattern of the scope's own baseline_excludes is covered by a forbidden class (one class source, 11.4.275(B); 11.4.273(f) set coverage)
# Round 10 (review R3-7): this test no longer re-implements the tool's baseline_probe; members of every class are judged from the HAND-WRITTEN table
# scripts/audit/tests/fixtures/baseline_members.tsv by test_index_health_fix_r8.sh (depth 0/1/2). Here only the pattern COUNT is read, to prove the instrument sees the set.
NPROBE="$(python3 - "$SCOPE" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
print(sum(1 for pats in (d.get("baseline_excludes") or {}).values() for pat in pats or [] if not pat.startswith("!")))
PY
)"
[ "$NPROBE" -ge 20 ] && ok "N1 the scope declares $NPROBE baseline patterns (instrument sees the pattern set)" || bad "N1 baseline pattern list too short: $NPROBE"
for fx in a/google-services.json a/credentials.json a/service-account-x.json a/.venv/x.py a/.pytest_cache/x a/logs/x.log a/.mypy_cache/x a/.ruff_cache/y a/.cache/z a/.compressed/q.bin; do
  p3with "$fx"; chk "N1 review fixture $fx -> P3 FAIL" 1 P3 FAIL forbidden_class_files_indexed
done
for ng in a/b.go a/logger/x.go a/.venv-notes.md a/google-services-readme.md a/credentials.go a/outline.md a/dist.go a/cachefile.go; do
  p3with "$ng"; chk "N1 control (false-positive guard): near-miss $ng -> P3 PASS" 3 P3 PASS
done
# the subset check itself: a baseline pattern no class covers makes the gate unusable (blind), not quietly clean
python3 - "$SCOPE" "$T/sc_extra.yaml" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1])); d["baseline_excludes"]["secrets"].append("*.zzvault")
yaml.safe_dump(d, open(sys.argv[2], "w"), sort_keys=False)
PY
base --scope "$T/sc_extra.yaml"; chk "N1 baseline pattern *.zzvault with no class -> P3 FAIL forbidden_classes_unusable (subset check)" 1 P3 FAIL baseline_pattern_uncovered
python3 - "$SCOPE" "$T/sc_nocred.yaml" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1])); d["forbidden_classes"] = [c for c in d["forbidden_classes"] if "service" not in c["regex"] and "credentials" not in c["regex"]]
yaml.safe_dump(d, open(sys.argv[2], "w"), sort_keys=False)
PY
base --scope "$T/sc_nocred.yaml"; chk "N1 scope with the credentials/service-account class removed -> P3 FAIL (a secret pattern lost its class)" 1 P3 FAIL forbidden_classes_unusable

printf 'forbidden_classes:\n  - {name: k, regex: "\\\\.key$", positive: "a/b.key", negative: "a/b.txt"}\n' > "$T/sc_nobase.yaml"; base --scope "$T/sc_nobase.yaml"
chk "N1 golden-false: a scope with sound classes and no baseline_excludes key is not refused -> P3 PASS" 3 P3 PASS
printf 'baseline_excludes: not-a-mapping\nforbidden_classes:\n  - {name: k, regex: "\\\\.key$", positive: "a/b.key", negative: "a/b.txt"}\n' > "$T/sc_badbase.yaml"; base --scope "$T/sc_badbase.yaml"
chk "N1 malformed baseline_excludes (not a mapping) -> P3 FAIL baseline_excludes_malformed" 1 P3 FAIL baseline_excludes_malformed

# ---- N4: case behaviour is pinned to the measured matcher: the runner's `ignore` library matches case-insensitively (default ignorecase true)
for fx in a/server.PEM a/Release.JKS a/.ENV a/x.KEY a/Google-Services.JSON a/SECRETS/q.txt docs/security/SECRETS_MANAGEMENT.md a/App.P12; do
  p3with "$fx"; chk "N4 case variant $fx -> P3 FAIL" 1 P3 FAIL forbidden_class_files_indexed
done
IGN="$HOME/.codegraph/versions/v1.6.0/lib/node_modules/ignore"
NODE="$(command -v node || true)"
if [ -n "$NODE" ] && [ -d "$IGN" ]; then
  out="$("$NODE" -e '
const ig=require(process.argv[1]); const ex=require("fs").existsSync("codegraph.json")?JSON.parse(require("fs").readFileSync("codegraph.json","utf8")).exclude:[];
const m=ig().add(ex); const r=[];
for (const p of ["a/server.PEM","a/.ENV","a/x.KEY","docs/security/SECRETS_MANAGEMENT.md"]) r.push(p+"="+m.ignores(p));
console.log(r.join(" "));' "$IGN" 2>&1)"
  case "$out" in
    "a/server.PEM=true a/.ENV=true a/x.KEY=true docs/security/SECRETS_MANAGEMENT.md=true") ok "N4 measured: the runner's ignore library (codegraph.json exclude) excludes every case variant, i.e. is case-insensitive" ;;
    *) bad "N4 measured matcher behaviour differs from the P3 case rule: $out" ;;
  esac
  gi="$(printf 'x' | "$NODE" -e 'const ig=require(process.argv[1]); console.log(ig().add(["*.pem"]).ignores("a/zz_probe.PEM"), ig({ignorecase:false}).add(["*.pem"]).ignores("a/zz_probe.PEM"))' "$IGN" 2>&1)"
  [ "$gi" = "true false" ] && ok "N4 control needle: the library does distinguish the modes (default true, ignorecase:false false)" || bad "N4 control needle: want 'true false', got [$gi]"
else skp "N4 runner ignore library or node absent ($IGN)"; fi

# ---- N5: the .env.example exemption is exactly two names; the reviewer mutant (any .example/.sample) must not pass unnoticed
for fx in a/secrets/db.yaml.example a/.env.example.bak a/.env.sample.bak a/secrets/token.sample; do
  p3with "$fx"; chk "N5 $fx -> P3 FAIL (exemption is not widened to .example/.sample suffixes)" 1 P3 FAIL forbidden_class_files_indexed
done
for ex in .env.example .env.sample; do
  p3with "$ex"; chk "N5 control: $ex stays exempt -> P3 PASS" 3 P3 PASS
done

# ---- N7: the capture age bound is mandatory whenever a capture or a live proof is judged
"$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_good.json" --tracked "$T/tracked.txt" --needle-pos "$POS" --needle-neg "$NEG" --lumen-status "$T/lu.txt" >/dev/null 2>"$T/e"; RC=$?
{ [ "$RC" -eq 2 ] && grep -q "captured-not-before" "$T/e"; } && ok "N7 --lumen-status without --captured-not-before -> usage rc=2 naming the option" || bad "N7 want rc=2, got rc=$RC"
printf 'Files: 2 | Indexed: 2 | Chunks: 9 | Stale: no\nCaptured: 2000-01-01T00:00:00Z\n' > "$T/lu_old.txt"
"$IH" --out "$T/o.json" --scope "$SCOPE" --cg-status "$T/st.json" --cg-files "$T/f_good.json" --tracked "$T/tracked.txt" --needle-pos "$POS" --needle-neg "$NEG" --lumen-status "$T/lu_old.txt" >/dev/null 2>&1; RC=$?
[ "$RC" -ne 0 ] && [ "$RC" -ne 3 ] && ok "N7 a year-2000 capture without the bound can no longer reach PASS/INCOMPLETE (rc=$RC)" || bad "N7 year-2000 capture without bound: rc=$RC (want usage 2)"
base --lumen-status "$T/lu_old.txt"; chk "N7 control: with the bound a year-2000 capture -> P7 FAIL" 1 P7 FAIL captured_before_required_time
base; chk "N7 control: a fresh capture with the bound -> P7 PASS" 3 P7 PASS

# ---- N2 (live proofs, drift binding) moved to test_index_health_fix_r8.sh in round 10 (verdicts derived from ledger-bound evidence; review R3-1/R3-2).

echo "SUMMARY pass=$PASSN fail=$FAILN skip=$SKIPN"
[ "$FAILN" -eq 0 ] || exit 1
[ "$SKIPN" -eq 0 ] || exit 77
exit 0
