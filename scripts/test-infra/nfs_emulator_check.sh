#!/usr/bin/env bash
# nfs_emulator_check.sh - sourced by nfs_emulator_live.sh. NFS3W-FU-m2: this check is necessary IN ADDITION TO go test rc==0 (it never replaces it). A log is
# acceptable only if every required top-level test printed its own anchored PASS line and no anchored `--- FAIL:` or `panic:` line is present. A SKIP, a zero-test run, or a subtest-only PASS (indented) does not count.
# nfs_emulator_required_tests prints the required names; nfs_emulator_missing_pass <log> prints each required test with no top-level PASS line and
# returns 0 only when none is missing.
nfs_emulator_required_tests() {
  printf '%s\n' TestUnfs3ExportsListsSeededExport TestUnfs3ListStatReadMatchServerManifest TestUnfs3ListDirectoryMatchesServerManifest \
    TestUnfs3MissingPathAndWritesRefused TestNFS3FactoryLifecycleLive
}
nfs_emulator_missing_pass() {
  local log=${1:?log path}; local t miss=0
  [ -s "$log" ] || { echo "(empty or missing log)"; return 1; }
  if grep -q -e '^--- FAIL:' -e '^panic:' "$log"; then echo "(log contains an anchored --- FAIL: or panic: line)"; miss=1; fi
  while IFS= read -r t; do
    grep -q -- "^--- PASS: ${t} " "$log" || { echo "$t"; miss=1; }
  done < <(nfs_emulator_required_tests)
  return "$miss"
}
