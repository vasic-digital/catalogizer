#!/usr/bin/env bash
# usage: runwz.sh MODE RUNNO
set -u
D="${QA_SCRATCH:?set QA_SCRATCH to the scratch dir that holds repo/ out/ evid/}"
SP="${QA_DRIVERS:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
M=${1:?}; R=${2:?}
(cd "$D/repo" && QA_BIND="${QA_BIND:?set QA_BIND to the host address to publish noVNC on (127.0.0.1 for a tunnel)}" scripts/tauri/run-qa.sh wizard "$D/out" >/dev/null)
sleep 10
WZ_MODE=$M QA_OUT="$D/evid/"wz_${M}_run$R nice -n 10 python3 "$SP"/wz_scenario.py > "$D/evid_wz_${M}_run$R.log" 2>&1
echo "wz $M run$R exit=$?" 
