#!/bin/bash
# round-2 mutants (host:port + scheme matching, nginx Host); run from catalog-api.
set -u
run() { systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 nice -n 10 env GOMAXPROCS=2 go test -p 1 -parallel 2 -count=1 "$@" 2>&1; }
m() { # id file sed-expr pkg run
  local id=$1 f=$2 e=$3 pkg=$4 rx=$5 bak; bak=$(mktemp "${TMPDIR:-/tmp}/mut.XXXXXX")
  cp "$f" "$bak"; sed -i "$e" "$f"
  if cmp -s "$f" "$bak"; then echo "$id NOT-APPLIED"; rm -f "$bak"; return; fi
  out=$(run "$pkg" -run "$rx")
  if echo "$out" | grep -q '^FAIL\|--- FAIL\|build failed'; then r=KILLED; else r=SURVIVED; fi
  cp "$bak" "$f"; cmp -s "$f" "$bak" && c=restored-cmp-ok || c=RESTORE-MISMATCH
  rm -f "${bak:?}"; echo "$id $r ($c)"
}
H=handlers/websocket_handler.go
m R10-allowlist-ignores-scheme $H 's/if aok \&\& as == scheme \&\& ah == host/if aok \&\& ah == host/' ./handlers 'OriginChecker'
m R17-hostname-only $H 's/if hp == port {/if true {/' ./handlers 'OriginChecker'
m R18-portless-host-accepts-any-port $H 's/} else if !explicit || port == map\[string\]string{"http": "80", "https": "443"}\[scheme\] {/} else if true {/' ./handlers 'OriginChecker'
m R20-allowlist-ignores-port $H 's/as == scheme \&\& ah == host \&\& ap == port/as == scheme \&\& ah == host/' ./handlers 'OriginChecker'
m R21-ipv6-hostname-trim $H 's/hu.Hostname(), host)/hu.Host, host)/' ./handlers 'OriginChecker'
m R19-nginx-template-back-to-host ../catalog-web/nginx.conf.template 's/Host \$http_host;/Host $host;/' . 'TestShippedNginx'
