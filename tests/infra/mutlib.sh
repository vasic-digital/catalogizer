#!/usr/bin/env bash
# mutlib.sh - the shared paired-mutation harness of the WF17 round 5 closure tests (test_sweep_contract.sh, test_ownership.sh, test_signals.sh, test_inputs.sh, test_build_stack.sh, test_nfs_attempt.sh).
# Sourced after tests/infra/lib.sh. A mutant is a COPY of scripts/test-infra with ONE load-bearing change (every anchor must occur exactly once, else the mutation is itself broken and FAILS the run);
# the same test file is re-run against the copy (TI_SUT_DIR) and must FAIL for a mutant (CAUGHT) and must PASS for an identity mutant (SURVIVED as required: the oracle must not fail on an equivalent source).
# Usage:  MUT_ENV=<PREFIX>   the variable prefix of the test (<PREFIX>_TEST_MUTANT=1 marks a mutant run; <PREFIX>_NO_MUTATIONS=1 skips the batteries)
#         mut_batch_begin <record file>      then   mut <name> <file> <old> <new> [<old2> <new2>...]   and   mut_id <name> <file> <old> <new>...   then   mut_batch_end
# A mutant run exports TI_IDS_LOG so the build ids it started are torn down by the REAL scripts afterwards (a mutant down.sh may not clean its own project).
MUT_REC=""
mut_batch_begin() { MUT_REC="$1"; : >"$MUT_REC"; }
mut_make() { # mut_make <name> <file> <old> <new> [<old2> <new2>...] ; the pseudo-pair `@@FILE <other file>` switches the target file for the pairs that follow
  local name=$1 file=$2 d="$TI_REPO/.audit/scratch/ti-mut-${MUT_ENV}-$1"; shift 2
  rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/scripts/test-infra"/*.sh "$TI_REPO/scripts/test-infra"/*.py "$d/"; cp -r "$TI_REPO/scripts/test-infra/client" "$d/client"
  mkdir -p "$d/compose"; cp "$TI_REPO"/docker-compose.build.yml "$TI_REPO"/docker-compose.test-infra.yml "$TI_REPO"/docker-compose.test-infra.nfs.yml "$d/compose/"; cp "$TI_REPO/scripts/container-build.sh" "$TI_REPO/scripts/setup-test-env.sh" "$d/compose/"
  python3 -I - "$d" "$file" "$@" <<'PY'
import sys
d, cur = sys.argv[1], sys.argv[2]; a = sys.argv[3:]
i = 0; texts = {}
def load(f):
    if f not in texts: texts[f] = open(d + "/" + f).read()
    return texts[f]
while i < len(a):
    if a[i] == "@@FILE":
        cur = a[i + 1]; i += 2; continue
    s = load(cur)
    if s.count(a[i]) != 1:
        print("mutation anchor count %d for %r in %s" % (s.count(a[i]), a[i][:90], cur)); sys.exit(1)
    texts[cur] = s.replace(a[i], a[i + 1]); i += 2
for f, t in texts.items(): open(d + "/" + f, "w").write(t)
PY
}
mut_run() { # mut_run <name> <caught|survive>
  local name=$1 want=$2 d=".audit/scratch/ti-mut-${MUT_ENV}-$1" out rc i; local flag="${MUT_ENV}_TEST_MUTANT" nom="${MUT_ENV}_NO_MUTATIONS"
  : >"$TI_SCRATCH/ids-$name.log"
  out=$(env TI_IDS_LOG="$TI_SCRATCH/ids-$name.log" TI_SUT_DIR="$d" "$flag=1" "$nom=1" TI_FAILFAST=1 QUIET=1 bash "${MUT_SELF:-${BASH_SOURCE[1]}}" 2>&1); rc=$?
  for i in $(cat "$TI_SCRATCH/ids-$name.log" 2>/dev/null); do TI_DOWN="$TI_REPO/scripts/test-infra/down.sh" ti_down "$i" >/dev/null 2>&1 || ti_scrap "$i"; done
  if [ "$want" = survive ]; then
    if [ "$rc" -eq 0 ]; then ok "identity mutant $name SURVIVED (as required)"; echo "$name SURVIVED-AS-REQUIRED" >>"$MUT_REC"; else bad "identity mutant $name FAILED the suite ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-120))"; echo "$name FAILED-BUT-IDENTITY" >>"$MUT_REC"; fi
  elif [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-120))"; echo "$name CAUGHT" >>"$MUT_REC"
  else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUT_REC"; fi
  rm -rf -- "${TI_REPO:?}/${d:?}"
}
mut()    { local name=$1; mut_make "$@" && mut_run "$name" caught  || bad "mutation $name: anchor missing"; }
mut_id() { local name=$1; mut_make "$@" && mut_run "$name" survive || bad "identity mutant $name: anchor missing"; }
mut_batch_end() { [ -z "${1:-}" ] || cp "$MUT_REC" "$1"; }
