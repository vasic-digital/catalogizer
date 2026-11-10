#!/usr/bin/env bash
# usage: runall.sh RUNNO  (fresh QA container, then scenario)
set -u
D="${QA_SCRATCH:?set QA_SCRATCH to the scratch dir that holds repo/ out/ evid/}"
SP="${QA_DRIVERS:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
R=${1:?}
(cd "$D/repo" && QA_BIND="${QA_BIND:?set QA_BIND to the host address to publish noVNC on (127.0.0.1 for a tunnel)}" scripts/tauri/run-qa.sh desktop "$D/out" >/dev/null)
sleep 12
( set -a; . "${QA_ENV_FILE:?path to the 0600 QA env file}"; set +a
  export QA_PW="$ADMIN_PASSWORD" QA_USER="$ADMIN_USERNAME"
  unset ADMIN_PASSWORD JWT_SECRET POSTGRES_PASSWORD METRICS_SCRAPE_TOKEN
  export QA_API_URL=http://host.containers.internal:38082 QA_ALT_URL=http://host.containers.internal:33080
  nice -n 10 python3 "$SP"/scenario.py "$D/evid/"run$R )
echo "run$R exit=$?"
