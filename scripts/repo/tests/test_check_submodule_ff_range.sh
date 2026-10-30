#!/usr/bin/env bash
# Test (TDD, RED before scripts/repo/check_submodule_ff_range.sh and the S3 row existed) for the wiring of scripts/check_submodule_ff.sh into CPA stage S3.
# UNIT-CLASS (constitution 11.4.289(C)): it drives the REAL wrapper, the REAL gate and the REAL validate_cheap.sh against throwaway git repositories with real
# local submodules built under mktemp -d; it is gate-logic evidence, not evidence about any real push of this project. Nothing outside $T is written.
# Usage: bash scripts/repo/tests/test_check_submodule_ff_range.sh [--quiet]   Env: WRAP=<wrapper> VC=<validate_cheap.sh> (mutation driver)
#        bash scripts/repo/tests/test_check_submodule_ff_range.sh --mutants    (10 lightweight mutants; each must make this test fail)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; QUIET=0
if [ "${1:-}" = --mutants ]; then
  M="$(mktemp -d "${TMPDIR:-/tmp}/subffr_mut.XXXXXX")"; trap 'rm -rf "${M:?}"' EXIT; bad=0
  mut() { # mut <name> <file> <sed expr>
    cp "$D0/scripts/repo/$2" "$M/$2"; sed -i "$3" "$M/$2"
    if cmp -s "$D0/scripts/repo/$2" "$M/$2"; then echo "NOT-APPLIED $1"; bad=1; return; fi
    local w="$D0/scripts/repo/check_submodule_ff_range.sh" v="$D0/scripts/repo/validate_cheap.sh"
    [ "$2" = check_submodule_ff_range.sh ] && w="$M/$2"; [ "$2" = validate_cheap.sh ] && v="$M/$2"
    if WRAP="$w" VC="$v" bash "$0" --quiet >/dev/null 2>&1; then echo "SURVIVED $1"; bad=1; else echo "killed   $1"; fi
  }
  mut NONFF_not_failed        check_submodule_ff_range.sh 's/^  exit 10$/  exit 14/'
  mut BLIND_read_as_pass      check_submodule_ff_range.sh 's/^\[ -z "\$blind" \] || defer.*$/true/'
  mut no_remote_read_as_pass  check_submodule_ff_range.sh 's/^\[ -n "\$tips" \] || defer.*$/[ -n "$tips" ] || exit 0/'
  mut exit1_without_line_pass check_submodule_ff_range.sh 's/^       \[ -n "\$lines" \] || lines=.*$/       :/'
  mut budget_not_clamped      check_submodule_ff_range.sh 's/^\[ "\${#BUDGET}" -le 6 \].*$/:/'
  mut budget_octal_failopen   check_submodule_ff_range.sh 's/^\(\[ "\${#BUDGET}" -le 6 \].*\)BUDGET=\$((10#\$BUDGET)); /\1/'
  mut budget_exhausted_bare_continue check_submodule_ff_range.sh 's/^  \[ "\$left" -gt 0 \] || {.*$/  [ "$left" -gt 0 ] || continue/'
  mut budget_strip_zero_only  check_submodule_ff_range.sh 's/BUDGET=\$((10#\$BUDGET))/BUDGET=${BUDGET#0}/'
  mut timeout_read_as_pass    check_submodule_ff_range.sh 's/^    124) blind=.*$/    124) ;;/'
  mut deferral_read_as_fail   validate_cheap.sh           "s/^    if rc == 14: emit('deferred'.*\$/    pass/"
  exit "$bad"
fi
[ "${1:-}" = --quiet ] && QUIET=1
WRAP="${WRAP:-$D0/scripts/repo/check_submodule_ff_range.sh}"; VC="${VC:-$D0/scripts/repo/validate_cheap.sh}"
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/subffr_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
[ -x "$WRAP" ] || echo "NOTE: $WRAP is absent or not executable (RED state: every case must FAIL)"
# code root as CPA's released copy has it: wrapper next to the gate, the gate next to scripts/audit/{org_of.py,own_orgs.txt}
mkcode() { # mkcode <dir> <with gate 1|0>
  mkdir -p "$1/scripts/repo" "$1/scripts/audit"; cp "$WRAP" "$1/scripts/repo/check_submodule_ff_range.sh" 2>/dev/null
  cp "$D0/scripts/audit/org_of.py" "$D0/scripts/audit/own_orgs.txt" "$1/scripts/audit/"
  [ "$2" = 1 ] && cp "$D0/scripts/check_submodule_ff.sh" "$1/scripts/check_submodule_ff.sh"; chmod +x "$1"/scripts/*.sh "$1"/scripts/repo/*.sh 2>/dev/null; return 0
}
CODE="$T/code"; mkcode "$CODE" 1; CODE0="$T/code0"; mkcode "$CODE0" 0
# submodule history c1 -> c2 -> c3 (one real repository, used as a local submodule: a local-path URL is own-org by the gate's fail-closed rule)
SUB="$T/sub"; mkrepo "$SUB"; for i in 1 2 3; do echo "$i" > "$SUB/f"; commit_all "$SUB" "c$i"; done
C1="$(git -C "$SUB" rev-parse HEAD~2)"; C2="$(git -C "$SUB" rev-parse HEAD~1)"; C3="$(git -C "$SUB" rev-parse HEAD)"
fresh() { # a parent with the submodule pinned at c2, a bare origin holding it, origin/main fetched and set as upstream
  rm -rf "$T/P" "$T/O.git"; mkrepo "$T/P"; echo r > "$T/P/readme"; commit_all "$T/P" base
  git -C "$T/P" submodule -q add "$SUB" sub 2>/dev/null; git -C "$T/P/sub" checkout -q "$C2"; commit_all "$T/P" "pin c2"
  git clone -q --bare "$T/P" "$T/O.git"; git -C "$T/P" remote add origin "$T/O.git"; git -C "$T/P" fetch -q origin; git -C "$T/P" branch -q -u origin/main
}
pin() { git -C "$T/P/sub" checkout -q "$1"; commit_all "$T/P" "pin $1"; }
snap() { { git -C "$T/P" for-each-ref; git -C "$T/P" status --porcelain; git -C "$T/P" rev-parse HEAD; } | sha256sum | cut -d' ' -f1; }
run() { local c="${CODEROOT:-$CODE}"; "$c/scripts/repo/check_submodule_ff_range.sh" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
case_() { # case_ <label> <want rc> <want substring or ''>
  local b; b="$(snap)"; run --root "$T/P"; eq "$1: exit" "$RC" "$2"; [ -z "${3-}" ] || has "$1: report" "$(cat "$T/out")" "$3"; eq "$1: refs, work tree and HEAD unchanged" "$(snap)" "$b"
}
fresh; pin "$C3";                    case_ "forward move c2->c3 (FF)" 0 ""
fresh; pin "$C1";                    case_ "backward move c2->c1 (NONFF)" 10 "NONFF own"
has "NONFF report carries the remediation first" "$(head -1 "$T/out")" "never force-push"
fresh; git -C "$T/P" update-index --add --cacheinfo "160000,deadbeefdeadbeefdeadbeefdeadbeefdeadbeef,sub"; git -C "$T/P" commit -qm fake; case_ "pin absent from every store (gate BLIND) is a deferral" 14 "BLIND submodule_ff_range"
fresh;                               case_ "empty range, nothing to check (PASS)" 0 ""
rm -rf "$T/P"; mkrepo "$T/P"; echo r > "$T/P/readme"; commit_all "$T/P" base; case_ "no remote main and no upstream is a deferral, not a pass" 14 "no remote main tip"
fresh; pin "$C3"; CODEROOT="$CODE0" case_ "gate absent from the code root is a deferral" 14 "not in the code root"
fresh; pin "$C3"; SUBFF_RANGE_BUDGET=0 case_ "budget exhausted is a deferral, not a refusal 20" 14 "budget-exhausted"
fresh; pin "$C1"; git -C "$T/P" remote add mirror "$T/O.git"; git -C "$T/P" fetch -q mirror; case_ "two remotes with one tip: still NONFF 10" 10 "NONFF own"
fresh; pin "$C1"; run --root; eq "--root without a value: 20" "$RC" 20
run; eq "no --root: 20" "$RC" 20; run --bogus; eq "unknown option: 20" "$RC" 20; run --root "$T/nonexistent"; eq "missing root: 20" "$RC" 20
run --root -x; eq "option-like root: 20" "$RC" 20
# ---- a gate that exits 1 without any `NONFF own` line must still be a refusal 10 (11.4.201(4) fail-closed), never a pass ----
stubcode() { # stubcode <dir> <gate body>  (code root with a stub gate in place of the real one)
  mkcode "$1" 0; printf '#!/bin/bash\n%s\n' "$2" > "$1/scripts/check_submodule_ff.sh"; chmod +x "$1/scripts/check_submodule_ff.sh"
}
stubcode "$T/stub1" 'echo "internal error: something broke" >&2; exit 1'
stubcode "$T/stub2" 'echo "VERDICT: FAIL checked=1 own_nonff=1"; exit 1'
fresh; pin "$C3"; CODEROOT="$T/stub1" case_ "gate exit 1 with no NONFF line (stderr only) is a refusal, not a pass" 10 "no 'NONFF own' line parsed"
fresh; pin "$C3"; CODEROOT="$T/stub2" case_ "gate exit 1 with only a VERDICT FAIL line is a refusal, not a pass" 10 "VERDICT: FAIL checked=1"
# SUBFF_RANGE_BUDGET is clamped below validate_cheap's 120 s kill: a fake `timeout` on PATH records the value the wrapper really passes
mkdir -p "$T/fakebin"; printf '#!/bin/bash\necho "$1" > "%s/tmo"\nshift\nexec /usr/bin/timeout 60 "$@"\n' "$T" > "$T/fakebin/timeout"; chmod +x "$T/fakebin/timeout"
fresh; pin "$C3"; SUBFF_RANGE_BUDGET=100000 PATH="$T/fakebin:$PATH" case_ "huge SUBFF_RANGE_BUDGET still runs and passes" 0 ""
t="$(cat "$T/tmo" 2>/dev/null || echo none)"; ok=1; if [ "$t" != none ] && [ "$t" -le 100 ] 2>/dev/null; then ok=0; fi; eq "huge SUBFF_RANGE_BUDGET is clamped to at most 100 s (timeout got $t)" "$ok" 0
# budget normalisation: leading zeros are decimal (08/0099 must not fail bash arithmetic and fail open), 010 is 10 not octal 8
stubcode "$T/stub3" 'echo "NONFF own sub: x"; echo "VERDICT: FAIL checked=1 own_nonff=1"; exit 1'
for b in 08 0099; do fresh; pin "$C3"; SUBFF_RANGE_BUDGET=$b CODEROOT="$T/stub3" case_ "budget $b (leading zero) does not fail open" 10 "NONFF own"; done
rm -f "$T/tmo"; fresh; pin "$C3"; SUBFF_RANGE_BUDGET=010 PATH="$T/fakebin:$PATH" case_ "budget 010 runs and passes" 0 ""
eq "budget 010 is decimal 10 (timeout got $(cat "$T/tmo" 2>/dev/null))" "$(cat "$T/tmo" 2>/dev/null)" 10
# gate exceeding the budget: timeout (rc 124) is a deferral 14 naming the timeout, never a pass (the env prefixes above and here are not subshells, so FAIL counts reach the parent)
stubcode "$T/stub5" 'sleep 6; exit 0'
fresh; pin "$C3"; SUBFF_RANGE_BUDGET=1 CODEROOT="$T/stub5" case_ "gate slower than the budget is a deferral naming the timeout" 14 "timeout:"
# two distinct remote main tips: a gate failing on either one (both orders) must give 10
mktwo() { fresh; pin "$C3"; rm -rf "$T/M.git"; git clone -q --bare "$T/P" "$T/M.git"; git -C "$T/P" remote add mirror "$T/M.git"; git -C "$T/P" fetch -q mirror
  git -C "$T/P" update-ref refs/remotes/mirror/main "$(git -C "$T/P" rev-parse HEAD~1)"; }
for bad in origin mirror; do mktwo; echo "$(git -C "$T/P" rev-parse "refs/remotes/$bad/main")" > "$T/badtip"
  stubcode "$T/stub4" "[ \"\$1\" = \"\$(cat $T/badtip)\" ] && { echo 'NONFF own sub: x'; exit 1; }; exit 0"
  CODEROOT="$T/stub4" case_ "two tips, gate fails on the $bad one only: refusal" 10 "NONFF own"; done
# ---- through validate_cheap.sh with a registry holding the submodule_ff row: NONFF -> fail/exit 10, BLIND -> deferred/exit 0, FF -> clean ----
REG="$T/reg.tsv"; { printf 'check\tcommand\timage\tmode\tbaseline\tscope\tnote\n'; grep -E '^submodule_ff	' "$D0/scripts/repo/validate_checks.tsv"; } > "$T/reg.tsv"
eq "the repository registry carries exactly one submodule_ff row (changeset scope, plain)" "$(awk -F'\t' '$1=="submodule_ff" && $4=="plain" && $6=="changeset"' "$D0/scripts/repo/validate_checks.tsv" | wc -l)" 1
vc() { cp "$VC" "$CODE/scripts/repo/validate_cheap.sh"; cp "$D0"/scripts/repo/lib_safe.sh "$D0"/scripts/repo/check_class.sh "$D0"/scripts/repo/verdict_go.py "$CODE/scripts/repo/" 2>/dev/null
  cp "$D0"/scripts/repo/check_classes.tsv "$D0"/scripts/repo/check_exemptions.tsv "$D0"/scripts/repo/fixture_roots.txt "$D0"/scripts/repo/binary_suffixes.tsv "$CODE/scripts/repo/" 2>/dev/null
  printf 'readme\n' > "$T/cs.lst"; ( cd "$T/P" && "$CODE/scripts/repo/validate_cheap.sh" --root "$T/P" --adopt-working-tables --code-root "$CODE" --registry "$REG" --files-from "$T/cs.lst" ) >"$T/vout" 2>"$T/verr"; VRC=$?; }
fresh; pin "$C1"; vc; eq "validate_cheap, NONFF: exit 10" "$VRC" 10; has "validate_cheap, NONFF: fail row" "$(cat "$T/vout")" "fail	submodule_ff"
fresh; git -C "$T/P" update-index --add --cacheinfo "160000,deadbeefdeadbeefdeadbeefdeadbeefdeadbeef,sub"; git -C "$T/P" commit -qm fake; vc
eq "validate_cheap, BLIND: exit 0 (a deferral never blocks)" "$VRC" 0; has "validate_cheap, BLIND: deferred row" "$(cat "$T/vout")" "deferred	submodule_ff"; hasnot "validate_cheap, BLIND: no fail row" "$(cat "$T/vout")" "fail	"
fresh; pin "$C3"; vc; eq "validate_cheap, FF: exit 0" "$VRC" 0; hasnot "validate_cheap, FF: no deferred row" "$(cat "$T/vout")" "deferred"
[ "$QUIET" = 1 ] && { echo "---- $PASSN ok, $FAILN failed"; [ "$FAILN" = 0 ] && [ -x "$WRAP" ]; exit; }
export H="$WRAP"; fin
