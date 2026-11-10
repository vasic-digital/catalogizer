#!/usr/bin/env bash
# usage: runwz.sh MODE RUNNO
set -u
D=$(cat /home/milosvasic/.cache/claude-tmp/scratch_tauriqa.path)
SP=/home/milosvasic/.cache/claude-tmp/claude-1000/-home-milosvasic-Projects-catalogizer/261db589-353c-4105-9405-009dbfe0bd33/scratchpad
M=${1:?}; R=${2:?}
(cd $D/repo && QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh wizard $D/out >/dev/null)
sleep 10
WZ_MODE=$M QA_OUT=$D/evid/wz_${M}_run$R nice -n 10 python3 $SP/wz_scenario.py > $D/evid_wz_${M}_run$R.log 2>&1
echo "wz $M run$R exit=$?" 
