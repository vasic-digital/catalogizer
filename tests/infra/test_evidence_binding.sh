#!/usr/bin/env bash
# test_evidence_binding.sh - WF17 fix round 5, TI-H1. Drives the REAL tests/infra/check_evidence_binding.sh against a fixture git repository (a real `git init`, real commits, real files; no podman).
# Oracle strategy (11.4.245): SPECIFIED (a header hash is the sha256 of the file under test; a manifest lists exactly the tracked files with their hashes) and DERIVED (the fixture's expected hashes come from
# sha256sum, not from the checker). Rows: B1 a bound fixture is accepted (control: the known-good header) | B2 a header hash edited -> exit 1 naming the file | B3 the file under test changed after the
# capture -> exit 1 | B4 a manifest naming an UNTRACKED file -> exit 1 | B5 a tracked file missing from the manifest -> exit 1 | B6 a manifest hash edited -> exit 1 | B7 `ABSENT` for a file that exists -> exit 1 |
# B8 worktree mode sees an uncommitted edit to the file under test while head mode judges the committed state only | B9 usage and non-repository errors | B10 the repository's own wf17 evidence is bound (head mode).
# Paired mutations: the header comparison removed; the tracked-only rule removed; the completeness rule removed; the manifest hash check removed (each must be caught by its row); an identity mutant must SURVIVE.
# Usage: test_evidence_binding.sh    (EB_NO_MUTATIONS=1: tests only)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
CK="${EB_SUT:-$TI_REPO/tests/infra/check_evidence_binding.sh}"
[ -f "$CK" ] && ok "script present: $CK" || { bad "script absent: $CK"; ti_summary; exit 1; }
mkfx() { # mkfx <dir>: a repository with src/a.sh, src/b.sh and an evidence directory ev/ holding two captures + a manifest, all committed
  local d=$1; rm -rf -- "${d:?}"; mkdir -p "$d/src" "$d/ev"; git -C "$d" init -q; git -C "$d" config user.email t@t; git -C "$d" config user.name t
  echo 'echo a' >"$d/src/a.sh"; echo 'echo b' >"$d/src/b.sh"
  { echo "# identity: fixture"; echo "# sha256 src/a.sh $(sha256sum "$d/src/a.sh" | cut -d' ' -f1)"; echo "# sha256 src/zz-absent.sh ABSENT"; echo "PASS: x"; } >"$d/ev/run1.txt"
  { echo "# identity: fixture 2"; echo "# sha256 src/b.sh $(sha256sum "$d/src/b.sh" | cut -d' ' -f1)"; echo "PASS: y"; } >"$d/ev/run2.txt"
  ( cd "$d/ev" && sha256sum ./run1.txt ./run2.txt >SHA256SUMS )
  git -C "$d" add -A && git -C "$d" commit -q -m fixture
}
run() { local d=$1; shift; "$CK" --root "$d" "$@" 2>&1; }
regen() { ( cd "$FX/ev" && sha256sum ./run1.txt ./run2.txt >SHA256SUMS ); }
FX="$TI_SCRATCH/fx"; mkfx "$FX"
o=$(run "$FX" ev); rc=$?; check "B1: a bound fixture is accepted (exit 0)" "$rc" 0
case "$o" in *"headers=3 manifests=1 findings=0"*) ok "B1: the checker saw 3 headers and 1 manifest (it is not blind)";; *) bad "B1: unexpected summary: $(printf '%s' "$o" | tail -1)";; esac
# B2
mkfx "$FX"; sed -i 's/^# sha256 src\/a.sh [0-9a-f]\{64\}/# sha256 src\/a.sh 0000000000000000000000000000000000000000000000000000000000000000/' "$FX/ev/run1.txt"; regen; git -C "$FX" commit -qam edit
o=$(run "$FX" ev); rc=$?; check "B2: an edited header hash is refused (exit 1)" "$rc" 1; case "$o" in *"ev/run1.txt:2: header hash of src/a.sh"*) ok "B2: the finding names the file and line";; *) bad "B2: wrong finding: $(printf '%s' "$o" | head -2 | tr '\n' ' ')";; esac
# B3
mkfx "$FX"; echo 'echo changed' >"$FX/src/a.sh"; git -C "$FX" commit -qam change-src
o=$(run "$FX" ev); rc=$?; check "B3: the file under test changed after the capture: refused (exit 1)" "$rc" 1
# B4
mkfx "$FX"; echo untracked >"$FX/ev/extra.log"; echo "$(sha256sum "$FX/ev/extra.log" | cut -d' ' -f1)  ./extra.log" >>"$FX/ev/SHA256SUMS"; git -C "$FX" commit -qam manifest-names-untracked
o=$(run "$FX" ev); rc=$?; check "B4: a manifest naming an untracked file is refused (exit 1)" "$rc" 1; case "$o" in *"extra.log is not a tracked file"*) ok "B4: the finding says it is not tracked";; *) bad "B4: wrong finding: $(printf '%s' "$o" | head -2 | tr '\n' ' ')";; esac
# B5
mkfx "$FX"; echo late >"$FX/ev/late.txt"; git -C "$FX" add -A; git -C "$FX" commit -qm late-file
o=$(run "$FX" ev); rc=$?; check "B5: a tracked file missing from the manifest is refused (exit 1)" "$rc" 1; case "$o" in *"late.txt is not listed"*) ok "B5: the finding names the unlisted file";; *) bad "B5: wrong finding: $(printf '%s' "$o" | head -2 | tr '\n' ' ')";; esac
# B6
mkfx "$FX"; sed -i '1s/^[0-9a-f]\{64\}/1111111111111111111111111111111111111111111111111111111111111111/' "$FX/ev/SHA256SUMS"; git -C "$FX" commit -qam bad-manifest
o=$(run "$FX" ev); rc=$?; check "B6: an edited manifest hash is refused (exit 1)" "$rc" 1
# B7
mkfx "$FX"; sed -i 's/src\/zz-absent.sh/src\/b.sh/' "$FX/ev/run1.txt"; regen; git -C "$FX" commit -qam absent-lie
o=$(run "$FX" ev); rc=$?; check "B7: a header claiming ABSENT for a file that exists is refused (exit 1)" "$rc" 1
# B8
mkfx "$FX"; echo 'echo uncommitted' >"$FX/src/a.sh"
o=$(run "$FX" --against worktree ev); rc=$?; check "B8: worktree mode sees the uncommitted change to the file under test and refuses (exit 1)" "$rc" 1
o=$(run "$FX" ev); rc=$?; check "B8: head mode judges the COMMITTED state only and accepts it (exit 0): what a clean checkout holds" "$rc" 0
{ echo "# identity: fixture"; echo "# sha256 src/a.sh $(sha256sum "$FX/src/a.sh" | cut -d' ' -f1)"; echo "# sha256 src/zz-absent.sh ABSENT"; echo "PASS: x"; } >"$FX/ev/run1.txt"; regen
o=$(run "$FX" --against worktree ev); rc=$?; check "B8: worktree mode accepts the consistent uncommitted tree (exit 0)" "$rc" 0
# B11 (--manifests-only: historical evidence keeps its old headers but its manifests must verify)
mkfx "$FX"; sed -i 's/^# sha256 src\/a.sh [0-9a-f]\{64\}/# sha256 src\/a.sh 0000000000000000000000000000000000000000000000000000000000000000/' "$FX/ev/run1.txt"; regen; git -C "$FX" commit -qam stale-header
o=$(run "$FX" ev); rc=$?; check "B11 control: the stale header is refused without the flag (exit 1)" "$rc" 1
o=$(run "$FX" --manifests-only ev); rc=$?; check "B11: --manifests-only accepts the stale header (exit 0)" "$rc" 0
echo late >"$FX/ev/late.txt"; git -C "$FX" add -A; git -C "$FX" commit -qm late-file
o=$(run "$FX" --manifests-only ev); rc=$?; check "B11: --manifests-only still refuses an unlisted tracked file (exit 1)" "$rc" 1
# B9
o=$("$CK" 2>&1); check "B9: no directory is a usage error (exit 2)" "$?" 2
o=$("$CK" --against nonsense ev 2>&1); check "B9: a bad --against value is a usage error (exit 2)" "$?" 2
mkdir -p "$TI_SCRATCH/nogit"; o=$(GIT_CEILING_DIRECTORIES="$TI_SCRATCH" "$CK" --root "$TI_SCRATCH/nogit" ev 2>&1); check "B9: a directory that is not a git repository exits 3" "$?" 3
# B10 (only when the repository's own HEAD already carries this suite's evidence directory)
EVD=specs/001-full-project-audit-remediation/evidence/wp12/wf17
if [ -n "$(git -C "$TI_REPO" ls-tree -r --name-only HEAD -- "$EVD" 2>/dev/null)" ] && [ "${EB_SKIP_REPO:-0}" != 1 ]; then
  o=$("$CK" --root "$TI_REPO" "$EVD" 2>&1); rc=$?; if [ "$rc" = 0 ]; then ok "B10: the repository's committed wf17 evidence is bound"; else echo "$o" | tail -3; bad "B10: the committed wf17 evidence is not bound"; fi
else echo "SKIP: B10: no committed wf17 evidence at HEAD yet (or EB_SKIP_REPO=1)"; fi
if [ "${EB_NO_MUTATIONS:-0}" != 1 ] && [ "${EB_TEST_MUTANT:-0}" != 1 ]; then
  REC="${EB_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$REC"
  ebmut() { # ebmut <name> <caught|survive> <old> <new>: a copy of the checker with ONE change; this same test re-run against it (EB_SUT) must FAIL (caught) / PASS (survive)
    local name=$1 want=$2 d="$TI_SCRATCH/mut-$1" out rc
    mkdir -p "$d"; cp "$CK" "$d/check_evidence_binding.sh"
    python3 -I - "$d/check_evidence_binding.sh" "$3" "$4" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
    out=$(EB_SUT="$d/check_evidence_binding.sh" EB_TEST_MUTANT=1 EB_NO_MUTATIONS=1 EB_SKIP_REPO=1 bash "${BASH_SOURCE[0]}" 2>&1); rc=$?
    if [ "$want" = survive ]; then
      if [ "$rc" -eq 0 ]; then ok "identity mutant $name SURVIVED (as required)"; echo "$name SURVIVED-AS-REQUIRED" >>"$REC"; else bad "identity mutant $name FAILED the suite"; echo "$name FAILED" >>"$REC"; fi
    elif [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-110))"; echo "$name CAUGHT" >>"$REC"
    else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$REC"; fi
  }
  ebmut header_comparison_removed caught 'elif hashlib.sha256(ref).hexdigest() != h: findings.append("%s:%d: header hash' 'elif False: findings.append("%s:%d: header hash'
  ebmut tracked_only_rule_removed caught 'if p not in T: findings.append(' 'if False: findings.append('
  ebmut completeness_rule_removed caught 'if n not in listed: findings.append(' 'if False: findings.append('
  ebmut manifest_hash_check_removed caught 'elif hashlib.sha256(f).hexdigest() != h: findings.append("%s:%d: %s hashes to' 'elif False: findings.append("%s:%d: %s hashes to'
  ebmut identity_noop survive 'nfiles = 0; nhdr = 0; nman = 0' 'nfiles = 0; nhdr = 0; nman = 0; _x = 0'
  [ -z "${EB_EV:-}" ] || cp "$REC" "$EB_EV/evidence-binding-mutations.txt"
fi
ti_summary
