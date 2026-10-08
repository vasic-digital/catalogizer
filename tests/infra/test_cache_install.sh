#!/usr/bin/env bash
# test_cache_install.sh - WF17 fix round 5, TI-C5 (corpus cache install race). Two REAL up.sh starts on an EMPTY corpus cache both miss it, both seed, and both reach the install: the install must be ONE rename onto the cache
# name that fails for the loser (mv -T onto a non-empty directory), never a nesting of the loser's tree INSIDE the winner's cache (the old `[ -d ] || mv` / plain `mv` form). The race window is widened by the test hook
# TI_TEST_SLEEP_CACHE_INSTALL (honoured only under TI_TEST_MODE=1): both starts finish seeding, both sleep, then both rename.
# Oracle strategy (11.4.245): SPECIFIED (the cache directory holds exactly `corpus` and `corpus.sha256`; both starts exit 0) and INVARIANT (no `.tmp.<pid>` sibling and no nested tree remains).
# Rows: R1 both starts exit 0 | R2 the cache holds exactly corpus + corpus.sha256 | R3 no `<cache>.tmp.*` sibling is left | R4 the cache digest file equals the digest the starts printed (the cache is the winner's, intact) | R5 a THIRD start on the
# now-warm cache reuses it without seeding.
# Paired mutations: the install is a plain `mv` (the loser's tree nests inside the cache); the loser's temporary tree is never removed. Identity mutant (must SURVIVE).
# Usage: test_cache_install.sh   (CACHEI_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=CACHEI; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"; export TI_ROOT="$TI_REPO"
for f in up.sh down.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
CACHE="$TI_REPO/.audit/scratch/cache-install-$$/cache"; mkdir -p "$(dirname "$CACHE")"; TI_FOREIGN_DIRS+=("$(dirname "$CACHE")")
A=$(ti_new_id); B=$(ti_new_id); TI_IDS+=("$A" "$B")
export TI_CORPUS_CACHE_DIR="$CACHE" TI_TEST_SLEEP_CACHE_INSTALL=6
bash "$TI_REPO/$SD/up.sh" --build-id "$A" --services redis >"$TI_SCRATCH/a.out" 2>&1 & pa=$!; TI_BG_PIDS+=("$pa")
bash "$TI_REPO/$SD/up.sh" --build-id "$B" --services redis >"$TI_SCRATCH/b.out" 2>&1 & pb=$!; TI_BG_PIDS+=("$pb")
wait "$pa"; ra=$?; wait "$pb"; rb=$?
check "R1: start A (cache miss, concurrent) exits 0" "$ra" 0
check "R1: start B (cache miss, concurrent) exits 0" "$rb" 0
[ "$ra" = 0 ] && [ "$rb" = 0 ] || { echo "  A said: $(tail -2 "$TI_SCRATCH/a.out" | tr '\n' ' ' | cut -c1-200)"; echo "  B said: $(tail -2 "$TI_SCRATCH/b.out" | tr '\n' ' ' | cut -c1-200)"; }
check "R2: the cache holds exactly corpus and corpus.sha256 (nothing nested inside it)" "$(ls -A "$CACHE" 2>/dev/null | tr '\n' ' ')" "corpus corpus.sha256 "
check "R3: no <cache>.tmp.* sibling is left behind" "$(ls -d "$CACHE".tmp.* 2>/dev/null | wc -l)" 0
da=$(sed -n 's/^corpus_sha256=//p' "$TI_SCRATCH/a.out"); db=$(sed -n 's/^corpus_sha256=//p' "$TI_SCRATCH/b.out")
check "R4: both starts report the same corpus digest, and it is the one in the cache" "$da/$db" "$(cat "$CACHE/corpus.sha256" 2>/dev/null | awk '{print $1}')/$(cat "$CACHE/corpus.sha256" 2>/dev/null | awk '{print $1}')"
# R5
for i in "$A" "$B"; do TI_DOWN="$TI_REPO/$SD/down.sh" ti_down "$i" >/dev/null 2>&1; done
C=$(ti_new_id); TI_IDS+=("$C"); unset TI_TEST_SLEEP_CACHE_INSTALL
bash "$TI_REPO/$SD/up.sh" --build-id "$C" --services redis >"$TI_SCRATCH/c.out" 2>&1; rc=$?
check "R5: a start on the warm cache exits 0" "$rc" 0
check "R5: it did not seed (no seed log with a tooling run, no seed output directory)" "$([ -d "$TI_REPO/.audit/out/$(ti_project "$C")-seed" ] && echo seeded || echo reused)" reused
if [ "${CACHEI_NO_MUTATIONS:-0}" != 1 ] && [ "${CACHEI_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${CACHEI_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  mut install_is_plain_mv up.sh 'mv -T -- "${CACHE:?}.tmp.$$" "$CACHE" 2>/dev/null || true' 'mv -- "${CACHE:?}.tmp.$$" "$CACHE" 2>/dev/null || true'
  mut loser_tree_never_removed up.sh 'rm -rf -- "${CACHE:?}.tmp.$$" "${SEEDOUT:?}"' 'rm -rf -- "${SEEDOUT:?}"'
  mut_id identity_noop up.sh 'SEEDOUT="$CACHE"' 'SEEDOUT="$CACHE"; :'
  mut_batch_end "${CACHEI_EV:+$CACHEI_EV/cache-install-mutations.txt}"
fi
ti_summary
