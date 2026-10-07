#!/usr/bin/env bash
# T043 runner: paired mutations of scripts/commit-push-all.sh and scripts/repo/host_entry/cpa-host (data: cpa_mutants.py).
# For every mutant: a throwaway copy of the scripts the matrix reads (scripts/repo, scripts/longops, scripts/audit, scripts/commit-push-all.sh) with the
# mutation applied (each `old` text must be present, else the mutant is `inapplicable`, a runner defect), then the section of
# scripts/repo/tests/test_commit_push_all.sh that is meant to catch it, run with CPA_SRC=<mutated copy>. CAUGHT = the section exits non-zero.
# The control (the unmutated copy through each section used) must pass, else the run is invalid. Equivalent mutants (a second layer enforces the same
# property) are listed and never counted as caught.
# A mutant is CAUGHT only when the section exits non-zero AND prints at least one `FAIL` assertion line (a crash, a timeout or a runner fault is no catch),
# and, when the mutant names an `expect` text (the 7th field of its entry), when a FAIL line contains it: the INTENDED assertion failed, not any one (WF11 F15).
# A replacement entry (old, new, file) mutates that file of the copy instead of the mutant's own file (a property enforced in two scripts needs both).
# Usage: bash scripts/repo/tests/run_wp04i_mutations.sh [--jobs N] [--only <id-regex>]      Exit 0 only when every non-equivalent mutant is caught
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; JOBS=4; ONLY=""
while [ $# -gt 0 ]; do case "$1" in --jobs) JOBS="$2"; shift 2 ;; --only) ONLY="$2"; shift 2 ;; *) echo "usage: $0 [--jobs N] [--only substr]" >&2; exit 2 ;; esac; done
M="$(mktemp -d "${TMPDIR:-/tmp}/cpa_mut.XXXXXX")"; trap 'rm -rf "$M"' EXIT
cp "$D0/scripts/repo/tests/test_commit_push_all.sh" "$M/matrix.sh"     # a snapshot: editing the live test during a run cannot corrupt it
mkdir -p "$M/base"; for d in scripts/repo scripts/longops scripts/audit; do mkdir -p "$M/base/$d"; done
cp -a "$D0"/scripts/repo/. "$M/base/scripts/repo/" 2>/dev/null; rm -rf "$M/base/scripts/repo/tests"
cp -a "$D0"/scripts/longops/*.sh "$M/base/scripts/longops/"; cp "$D0"/scripts/audit/org_of.py "$M/base/scripts/audit/"; cp "$D0/scripts/commit-push-all.sh" "$M/base/scripts/"
python3 -I - "$D0/scripts/repo/tests" "$M" "$ONLY" <<'PY' || exit 2
import sys, os, shutil, json, re
sys.path.insert(0, sys.argv[1]); import cpa_mutants as cm
m = sys.argv[2]; only = sys.argv[3]; out = []
for t in cm.MUTANTS:
    mid, f, reps, sect, desc = t[:5]; equiv = t[5] if len(t) > 5 else ''; expect = t[6] if len(t) > 6 else ''
    if only and not re.search(only, mid): continue
    dst = os.path.join(m, mid); shutil.copytree(os.path.join(m, 'base'), dst, symlinks=True)
    ok = True; texts = {}
    for r in reps:
        old, new = r[0], r[1]; fp = os.path.join(dst, r[2] if len(r) > 2 else f)
        s = texts.get(fp)
        if s is None: s = open(fp).read()
        if old not in s: ok = False; break
        texts[fp] = s.replace(old, new)
    if ok:
        for fp, s in texts.items(): open(fp, 'w').write(s)
    out.append({'id': mid, 'section': sect, 'desc': desc, 'applied': ok, 'equivalent': equiv, 'expect': expect})
json.dump(out, open(os.path.join(m, 'mutants.json'), 'w'))
PY
SECTIONS="$(jq -r '[.[]|select(.applied)|.section]|unique|.[]' "$M/mutants.json")"
echo "== control: the unmutated copy through each section used"
runctl() { local sec="$1" f; f="$M/control.$(echo "$sec" | tr ' ' '_')"; CPA_SRC="$M/base" CPA_ONLY="$sec" bash "$M/matrix.sh" > "$f.out" 2>&1; echo "$?" > "$f.rc"; }
runone() { # runone <id> <section>
  local id="$1" sec="$2"; CPA_SRC="$M/$id" CPA_ONLY="$sec" bash "$M/matrix.sh" > "$M/$id.out" 2>&1; echo "$?" > "$M/$id.rc"
}
export -f runone runctl; export M
printf '%s\n' "$SECTIONS" | xargs -P "$JOBS" -I{} -d '\n' bash -c 'runctl "$0"' {}
cc=0; while IFS= read -r sec; do
  [ -n "$sec" ] || continue; f="$M/control.$(echo "$sec" | tr ' ' '_')"; rc="$(cat "$f.rc")"
  printf 'control %-12s exit=%s  %s\n' "$sec" "$rc" "$(tail -1 "$f.out")"; [ "$rc" = 0 ] || cc=1
done <<< "$SECTIONS"
echo "== mutants (jobs=$JOBS)"
jq -r '.[]|select(.applied and (.equivalent=="") )|[.id,.section]|@tsv' "$M/mutants.json" | while IFS=$'\t' read -r id sec; do printf '%s\t%s\n' "$id" "$sec"; done \
  | xargs -P "$JOBS" -L1 -d '\n' bash -c 'IFS=$'"'"'\t'"'"' read -r id sec <<< "$0"; runone "$id" "$sec"'
caught=0; surv=0; inapp=0; equiv=0; total=0
printf '%-34s %-12s %-10s %s\n' "mutant" "section" "result" "first failing line (or the models-what)"
while IFS=$'\t' read -r id sec applied eq expect desc; do
  total=$((total+1))
  if [ "$applied" != true ]; then inapp=$((inapp+1)); printf '%-34s %-12s %-10s %s\n' "$id" "$sec" "INAPPLICABLE" "$desc"; continue; fi
  if [ "$eq" != "-" ]; then equiv=$((equiv+1)); printf '%-34s %-12s %-10s %s\n' "$id" "$sec" "equivalent" "$eq"; continue; fi
  rc="$(cat "$M/$id.rc" 2>/dev/null || echo ?)"
  if [ "$rc" != 0 ] && [ "$rc" != "?" ] && grep -q '^FAIL' "$M/$id.out" && { [ "$expect" = "-" ] || grep '^FAIL' "$M/$id.out" | grep -qF -- "$expect"; }; then
    caught=$((caught+1)); printf '%-34s %-12s %-10s %s\n' "$id" "$sec" "CAUGHT" "$(grep -m1 '^FAIL' "$M/$id.out" | cut -c1-110)"
  else surv=$((surv+1)); printf '%-34s %-12s %-10s %s\n' "$id" "$sec" "SURVIVED" "$desc (rc=$rc; intended assertion: ${expect})"; fi
done < <(jq -r '.[]|[.id,.section,(.applied|tostring),(if .equivalent=="" then "-" else .equivalent end),(if .expect=="" then "-" else .expect end),.desc]|@tsv' "$M/mutants.json")   # "-": an empty field would collapse under read
echo "---- mutants: $total total, $caught caught, $surv survived, $equiv equivalent (second layer, not counted), $inapp inapplicable"
[ "$surv" = 0 ] && [ "$inapp" = 0 ] && [ "$cc" = 0 ]
