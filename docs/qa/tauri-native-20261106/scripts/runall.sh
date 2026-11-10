#!/usr/bin/env bash
# usage: runall.sh RUNNO  (fresh QA container, then scenario)
set -u
D=$(cat /home/milosvasic/.cache/claude-tmp/scratch_tauriqa.path)
SP=/home/milosvasic/.cache/claude-tmp/claude-1000/-home-milosvasic-Projects-catalogizer/261db589-353c-4105-9405-009dbfe0bd33/scratchpad
R=${1:?}
(cd $D/repo && QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop $D/out >/dev/null)
sleep 12
( set -a; . /home/milosvasic/.cache/claude-tmp/catqa3/qa.env; set +a
  export QA_PW="$ADMIN_PASSWORD" QA_USER="$ADMIN_USERNAME"
  unset ADMIN_PASSWORD JWT_SECRET POSTGRES_PASSWORD METRICS_SCRAPE_TOKEN
  export QA_API_URL=http://host.containers.internal:38082 QA_ALT_URL=http://host.containers.internal:33080
  nice -n 10 python3 $SP/scenario.py $D/evid/run$R )
echo "run$R exit=$?"
