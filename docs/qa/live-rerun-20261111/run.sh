#!/bin/bash
# usage: run.sh <d5d6|d7|d8> <run#>   (creds read from 0600 env file, never printed)
OUT=/home/milosvasic/Projects/catalogizer/docs/qa/live-rerun-20261111
QA=/home/milosvasic/Projects/catalogizer/docs/qa
PW=/home/milosvasic/.cache/claude-tmp/axe.ilzZrW/node_modules
CH=/home/milosvasic/.cache/ms-playwright/chromium_headless_shell-1228/chrome-headless-shell-linux64/chrome-headless-shell
set -a; . /home/milosvasic/.cache/claude-tmp/catqa3/qa.env; set +a
export BASE=http://192.168.1.17:33020 U="$ADMIN_USERNAME" P="$ADMIN_PASSWORD"
export NODE_MODULES_DIR=$PW CHROMIUM_PATH=$CH NM=$PW/ CHROME=$CH
unset JWT_SECRET POSTGRES_PASSWORD METRICS_SCRAPE_TOKEN
c=$1; n=$2
case $c in
 d5d6) cmd="node $QA/ui-d5d6-20261105/axe_live.cjs";;
 d7) cmd="node $QA/ui-d7d8-20261105/d7_axe.cjs";;
 d8) cmd="node $QA/ui-d7d8-20261105/d8_repro.cjs";;
esac
date -u +%FT%TZ > $OUT/${c}_run$n.start
nice -n 10 systemd-run --user --scope -q -p MemoryMax=5G -p MemorySwapMax=0 $cmd >$OUT/${c}_run$n.json 2>$OUT/${c}_run$n.err
echo "rc=$?" > $OUT/${c}_run$n.rc
date -u +%FT%TZ > $OUT/${c}_run$n.end
sed -i "s/$ADMIN_PASSWORD/<REDACTED>/g" $OUT/${c}_run$n.* 2>/dev/null
echo "$c run$n $(cat $OUT/${c}_run$n.rc)"
