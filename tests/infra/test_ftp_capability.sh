#!/usr/bin/env bash
# test_ftp_capability.sh - WF12 F13 (RED first: before this test the claim "AUDIT_WRITE alone suffices" had no captured evidence). Oracle for the one capability docker-compose.test-infra.yml adds to
# the ftp service. Oracle strategy (11.4.245): SPECIFIED by the compose file's own claim, as a CONTROL PAIR on a real stack: with `cap_add: [AUDIT_WRITE]` the ftp service starts and answers its
# protocol probe (exit 0); the SAME compose file without that capability (a scratch copy, TI_COMPOSE_FILE) never becomes ready (exit 1, the project is torn down). The container's exit state of the
# capability-less start is recorded (what podman really said), not assumed; `privileged` and every other capability stay absent from the real file (checked by test_compose_files.sh).
# Usage: test_ftp_capability.sh   Env: FTPCAP_EV (directory that receives ftp-capability.txt)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
export TI_ROOT="$TI_REPO"
UP="$TI_REPO/scripts/test-infra/up.sh"
python3 -I - "$TI_REPO/docker-compose.test-infra.yml" "$TI_SCRATCH/nocap.yml" <<'PY' || { bad "cannot derive the capability-less compose file"; ti_summary; exit 1; }
import sys
s = open(sys.argv[1]).read(); a = "    cap_add:\n      - AUDIT_WRITE\n"
assert s.count(a) == 1
open(sys.argv[2], "w").write(s.replace(a, ""))
PY
N=$(ti_new_id); TI_IDS+=("$N"); PN=$(ti_project "$N")
nout=$(TI_COMPOSE_FILE="$TI_SCRATCH/nocap.yml" bash "$UP" --build-id "$N" --services ftp --timeout 40 2>&1); nrc=$?
check "WITHOUT AUDIT_WRITE the ftp service never becomes ready (up exits 1)" "$nrc" 1
out=$nout
case "$out" in *"services not ready"*|*"podman-compose up failed"*) ok "the failure is the readiness / start failure ($(printf '%s' "$out" | tail -1 | cut -c1-120))";; *) bad "unexpected failure text: $(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
LOGS=$(ls "$TI_REPO"/.audit/out/$PN-logs/log-*.txt 2>/dev/null | head -1)
check "the failed start left no container and no lease" "$(podman ps -a -q --filter "label=catalogizer.test_project=$PN" | wc -l)$([ -d "$TI_REPO/.audit/longops/claims/$PN" ] && echo held || echo free)" 0free
Y=$(ti_new_id); TI_IDS+=("$Y"); PY=$(ti_project "$Y")
out=$(bash "$UP" --build-id "$Y" --services ftp 2>&1); rc=$?
check "WITH AUDIT_WRITE (the committed compose file) the ftp service starts and answers its probe (up exits 0)" "$rc" 0
if [ -n "${FTPCAP_EV:-}" ]; then
  mkdir -p "$FTPCAP_EV"
  { echo "# WF12 F13: control pair for cap_add AUDIT_WRITE on the ftp service (tests/infra/test_ftp_capability.sh)"
    echo "without_capability_up_exit=$nrc (measured); last line of that run's up.sh output: $(printf '%s' "$nout" | tail -1 | cut -c1-200)"
    echo "with_capability_up_exit=$rc (measured); last line of that run's up.sh output: $(printf '%s' "$out" | tail -1 | cut -c1-200)"
    [ -z "$LOGS" ] || { echo "--- kept log of the capability-less start (first 12 lines) ---"; head -12 "$LOGS"; rm -rf -- "$(dirname "$LOGS")"; }
  } >"$FTPCAP_EV/ftp-capability.txt"
fi
rm -rf -- "$TI_REPO/.audit/out/$PN-logs"
ti_summary
