#!/usr/bin/env bash
# T045a test (TDD): gate CM-TOOLING-PROJECT-DECOUPLED (constitution 11.4.177, 11.4.29; FR-019, SC-010).
# The host entry point scripts/repo/host_entry/cpa-host is installed on the owner's shared PATH and works on any project: it operates on the
# invocation directory, keeps per-project trust state and must carry no literal of THIS project. The check below scans the script and every file it
# sources (`. file` / `source file`, resolved next to the script) for the closed list of literals and reports each hit by file and line.
# Control needle: a scratch copy with the project name inserted MUST be reported (the needle is run before the real files are trusted, 11.4.201(7)).
# Paired mutation: a check that skips the sourced files misses a literal planted in one, so the assertion on the real check catches that mutation.
# Usage: bash scripts/repo/tests/test_tooling_decoupled.sh   Env: TD_DISABLE_CHECK=1 simulates the absent check (the RED capture of the needle).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; . "$D0/scripts/repo/tests/lib_wp04b.sh"
H="$D0/scripts/repo/host_entry/cpa-host"
T="$(mktemp -d "${TMPDIR:-/tmp}/td_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
# the closed list: the project name, this repository's remote URLs and the absolute path of the checkout (read at run time, never typed here)
LITERALS=()
LITERALS+=("$(basename "$D0" | tr 'A-Z' 'a-z')"); LITERALS+=("$D0")
while IFS= read -r u; do [ -n "$u" ] && LITERALS+=("$u"); done < <(git -C "$D0" remote -v 2>/dev/null | awk '{print $2}' | sort -u)

sourced_of() { # sourced_of <file>: the files it sources, resolved next to it (one level of the closure per call)
  local f="$1" dir; dir="$(dirname "$f")"
  grep -hE '^[[:space:]]*(\.|source)[[:space:]]+' "$f" 2>/dev/null | sed -E 's/^[[:space:]]*(\.|source)[[:space:]]+//; s/[;&|].*$//; s/^"?//; s/"?[[:space:]]*$//' \
    | sed -E 's#\$\{?[A-Za-z_]*DIR\}?#'"$dir"'#; s#\$\(dirname[^)]*\)#'"$dir"'#' | while IFS= read -r s; do case "$s" in /*) [ -f "$s" ] && echo "$s" ;; *) [ -f "$dir/$s" ] && echo "$dir/$s" ;; esac; done
}
closure() { # closure <file> [follow=1]: the file and, transitively, every sourced file
  local todo=("$1") seen="" f s
  while [ "${#todo[@]}" -gt 0 ]; do f="${todo[0]}"; todo=("${todo[@]:1}"); case " $seen " in *" $f "*) continue ;; esac; seen="$seen $f"; echo "$f"
    [ "${2:-1}" = 1 ] || continue; while IFS= read -r s; do [ -n "$s" ] && todo+=("$s"); done < <(sourced_of "$f"); done
}
check() { # check <file> [follow]: prints `file:line: literal` per hit, exit 1 when there is one
  [ -z "${TD_DISABLE_CHECK:-}" ] || return 0
  local hit=0 f l; while IFS= read -r f; do
    for l in "${LITERALS[@]}"; do [ -n "$l" ] || continue
      grep -nIiF -- "$l" "$f" 2>/dev/null | sed "s#^#$f:#; s#\$#  <== $l#" | while IFS= read -r x; do echo "$x"; done
    done
  done < <(closure "$1" "${2:-1}") | sort -u | tee "$T/hits.txt"
  [ ! -s "$T/hits.txt" ]
}

[ -f "$H" ] || bad "the host entry point $H is absent"
# control needle: a scratch copy with the project name inserted is reported
mkdir -p "$T/needle"; cp "$H" "$T/needle/cpa-host" 2>/dev/null; echo "# $(basename "$D0")" >> "$T/needle/cpa-host"
out="$(check "$T/needle/cpa-host" 2>&1)"; rc=$?
eq "needle: the scratch copy with the project name inserted is reported (exit 1)" "$rc" 1; has "needle: the hit is named by file and line" "$out" "needle/cpa-host:"
# golden-true: the real files are clean
out="$(check "$H" 2>&1)"; rc=$?; eq "the host entry point and its sourced files carry no project literal" "$rc" 0; [ -z "$out" ] && ok "no hit printed" || bad "hits: $out"
# a literal in a SOURCED file is found by the real check and missed by the mutated one (the paired mutation)
mkdir -p "$T/src"; printf '#!/usr/bin/env bash\n. "$(dirname "$0")/lib.sh"\n' > "$T/src/cpa-host"; printf '# %s\n' "$(basename "$D0")" > "$T/src/lib.sh"
out="$(check "$T/src/cpa-host" 2>&1)"; rc=$?; eq "a literal planted in a sourced file is reported by the real check" "$rc" 1; has "named by its file" "$out" "src/lib.sh:"
out="$(check "$T/src/cpa-host" 0 2>&1)"; rc=$?; eq "mutation (skip the sourced files): the planted literal is MISSED, so the real-check assertion above catches it" "$rc" 0
# the literal list itself covers the three classes the task names
[ "${#LITERALS[@]}" -ge 2 ] && ok "the closed list holds the project name and the checkout path (+ $(( ${#LITERALS[@]} - 2 )) remote URL(s))" || bad "closed list too short"
# naming (11.4.29): the kebab-case exceptions are recorded in the companion
DOC="$D0/docs/scripts/commit-push-all.md"
for n in scripts/commit-push-all.sh cpa-host tools/evidence/wrap-go.sh wrap-bash.sh wrap-vitest.sh wrap-gradle.sh wrap-cargo.sh scripts/test-in-container.sh scripts/audit/android-container.sh scripts/bash-coverage.sh scripts/audit/adb-container.sh; do
  grep -qF -- "$n" "$DOC" 2>/dev/null && ok "kebab-case exception listed in the companion: $n" || bad "kebab-case exception not listed in docs/scripts/commit-push-all.md: $n"
done
grep -qiF "owner item" "$DOC" 2>/dev/null && ok "the companion records the cpa-host rename question as an owner item" || bad "the companion does not record the cpa-host rename question as an owner item"
fin
