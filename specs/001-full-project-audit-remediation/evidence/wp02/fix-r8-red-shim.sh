#!/usr/bin/env bash
# fix-r8-red-shim.sh - lets the PRE-FIX index_health.sh (fix-r8-prefix-tool/index_health.sh.prefix) be driven by the round-8 test WITHOUT failing on argv
# (review WP-02-T028-r2 N6: a RED that fails because the old tool rejects new options is red for the wrong reason). It only
#   (1) drops the options the old tool does not know (--repo --live-policy --evidence-ledger --lumen-json --baseline) with their values, and
#   (2) rewrites each live-proofs entry to the OLD entry shape (verdict PASS when absent): evidence_path made absolute (IH_SHIM_REPO is exported by the test) and cmd := the
#       evidence file's `program:` value, which the old tool then tests for containment in the file and for its basename in its allow-list.
# Every judgement stays the old tool's own. Usage: IH=<this shim> OLD=<prefix tool> bash scripts/audit/tests/test_index_health_fix_r8.sh
set -u
OLD="${OLD:?OLD=path of the pre-fix tool}"
args=(); i=1; n=$#; argv=("$@"); LP=""
while [ $i -le $n ]; do
  a="${argv[$((i-1))]}"
  case "$a" in
    --repo|--live-policy|--evidence-ledger|--lumen-json|--baseline) i=$((i+2)); continue ;;
    --live-proofs) LP="${argv[$i]}"; args+=("$a" "@LP@"); i=$((i+2)); continue ;;
  esac
  args+=("$a"); i=$((i+1))
done
if [ -n "$LP" ]; then
  tmp="$(mktemp "${TMPDIR:-/tmp}/shim_lp.XXXXXX")"
  python3 - "$LP" "$tmp" "${IH_SHIM_REPO:-.}" <<'PY'
import json, os, sys
src, dst, repo = sys.argv[1:4]
try:
    d = json.load(open(src))
except Exception:
    open(dst, "w").write(open(src).read()); sys.exit(0)
for k, e in list(d.items()):
    if not isinstance(e, dict):
        continue
    p = e.get("evidence_path")
    if isinstance(p, str) and not os.path.isabs(p):
        p = os.path.join(repo, p); e["evidence_path"] = p
    prog = None
    try:
        for line in open(p):
            if line.startswith("program: "):
                prog = line[len("program: "):].strip(); break
    except Exception:
        pass
    if prog:
        e["cmd"] = prog
    e.setdefault("verdict", "PASS")   # the old entry shape REQUIRES the caller's own verdict (that is the defect under test)
json.dump(d, open(dst, "w"))
PY
  for j in "${!args[@]}"; do [ "${args[$j]}" = "@LP@" ] && args[$j]="$tmp"; done
fi
bash "$OLD" "${args[@]}"; rc=$?
[ -z "${tmp:-}" ] || rm -f "$tmp"
exit $rc
