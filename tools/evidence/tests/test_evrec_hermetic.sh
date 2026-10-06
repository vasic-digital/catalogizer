#!/usr/bin/env bash
# T050 round 3 (review finding I12): proves the other test files are hermetic. A copy of the tool tree is made in a
# scratch checkout whose .audit/commit_turn.json holds a LIVE foreign grant, HOME holds a logging stand-in for the
# owner's host entry (~/.local/bin/cpa-host), and the caller-environment variables the tools read are set to hostile
# values. test_evrec.sh, test_evrec_more.sh and test_evrec_r3.sh are then run from that copy: they must all pass
# (a test that read the scratch checkout's grant would go red) and the stand-in must never be called.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$S"' EXIT
fails=0; n=0
ok()  { n=$((n+1)); if [ -x "$root/tools/evidence/evrec" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
F=specs/001-full-project-audit-remediation
mkdir -p "$S/repo/tools/evidence/tests" "$S/repo/$F/contracts" "$S/repo/scripts/repo" "$S/repo/.audit" "$S/home/.local/bin"
cp "$root/tools/evidence/evrec" "$root/tools/evidence/verify" "$root/tools/evidence/evcore.py" "$S/repo/tools/evidence/"
# WP-05 tools written after round 3 (T051..T055): the copied suites call them, so the scratch checkout carries them too (additive, round 8)
for f in evanchor.py evparse.py evverdict.py verdict wrap-go.sh wrap-bash.sh wrap-vitest.sh wrap-gradle.sh wrap-cargo.sh census_ab_pass.py; do
  [ -e "$root/tools/evidence/$f" ] && cp "$root/tools/evidence/$f" "$S/repo/tools/evidence/"; done
[ -d "$root/tools/evidence/lib" ] && cp -r "$root/tools/evidence/lib" "$S/repo/tools/evidence/"
cp "$here"/*.sh "$here"/*.py "$S/repo/tools/evidence/tests/"
cp "$root/$F/contracts/evidence-record.schema.json" "$S/repo/$F/contracts/"; cp "$root/scripts/repo/check_classes.tsv" "$S/repo/scripts/repo/"
sleep 300 & holder=$!
for _ in $(seq 1 200); do [ "$(tr '\0' ' ' </proc/$holder/cmdline 2>/dev/null)" = "sleep 300 " ] && break; sleep 0.02; done
printf '{"run_id":"somebody-else","pid":%s,"cmdline":"sleep 300","started_at":"2026-10-05T00:00:00Z"}\n' "$holder" >"$S/repo/.audit/commit_turn.json"
printf '#!/usr/bin/env bash\necho "$*" >>"%s/host_calls"\nexit 99\n' "$S" >"$S/home/.local/bin/cpa-host"; chmod +x "$S/home/.local/bin/cpa-host"; : >"$S/host_calls"
# hostile caller environment (every variable the tools read)
export HOME=$S/home EVREC_TURN_RUN_ID=hostile EVREC_REDACT_VARS="PATH HOME" EV_TEST_SOURCES=/nonexistent EV_SCHEMA=/nonexistent EV_LEDGER_BOUND=1 EV_BLOB_BOUND=1 EVREC_LOCK_TIMEOUT=not-a-number \
       EV_FLAKE_LEDGER=/nonexistent EVREC_PROC_ROOT=/nonexistent PYTHONOPTIMIZE=1 EV=$S/hostile-ev EV_LEDGER=$S/hostile-ledger EV_BLOBS=$S/hostile-blobs EV_ANCHOR=$S/hostile-anchor CT_REFUSE=1
unset CPA_HOST_ENTRY EVREC_REPO_ROOT
mkdir -p "$S/hostile-cwd"; printf hostile >"$S/hostile-cwd/x"; cd "$S/hostile-cwd"   # a file named x in the caller's cwd: PROBE target refs resolve against the cwd
for t in test_evrec.sh test_evrec_more.sh test_evrec_r3.sh test_evrec_golden.sh; do
  out=$(bash "$S/repo/tools/evidence/tests/$t" 2>&1); r=$?
  if [ $r = 0 ]; then ok "hermetic: $t passes from a checkout holding a live foreign grant and with a hostile environment ($(tail -1 <<<"$out"))"; else bad "hermetic: $t rc=$r: $(grep '^FAIL' <<<"$out" | head -3 | tr '\n' '|')"; fi
done
calls=$(wc -l <"$S/host_calls" | tr -d ' ')
[ "$calls" = 0 ] && ok "hermetic: the stand-in for ~/.local/bin/cpa-host was called 0 times" || bad "hermetic: the owner's host entry stand-in was called $calls time(s): $(head -2 "$S/host_calls" | tr '\n' '|')"
[ ! -e "$S/hostile-ledger" ] && [ ! -d "$S/hostile-blobs" ] && ok "hermetic: nothing was written to the hostile caller-supplied store paths" || bad "hermetic: a test wrote to caller-supplied EV_LEDGER / EV_BLOBS"
kill $holder 2>/dev/null
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
