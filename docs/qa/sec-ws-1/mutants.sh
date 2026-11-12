#!/bin/bash
# paired mutants for SEC-WS-1; run from catalog-api. Each: mutate, run, cmp-verified restore.
set -u
run() { systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 nice -n 10 env GOMAXPROCS=2 go test -p 1 -parallel 2 -count=1 "$@" 2>&1; }
m() { # id file sed-expr
  local id=$1 f=$2 e=$3 bak; bak=$(mktemp "${TMPDIR:-/tmp}/mut.XXXXXX")
  cp "$f" "$bak"; sed -i "$e" "$f"
  if cmp -s "$f" "$bak"; then echo "$id NOT-APPLIED"; rm -f "$bak"; return; fi
  out=$(run ./handlers -run 'WebSocket|OriginChecker'; run . -run 'TestWSRoute|TestMainGoMountsWebSocket')
  if echo "$out" | grep -q '^FAIL\|--- FAIL\|build failed'; then r=KILLED; else r=SURVIVED; fi
  cp "$bak" "$f"; cmp -s "$f" "$bak" && c=restored-cmp-ok || c=RESTORE-MISMATCH
  rm -f "${bak:?}"; echo "$id $r ($c)"
}
m M1-drop-auth-middleware main_wsroute.go 's/mw.RequireAuth(), //'
m M2-checkorigin-true handlers/websocket_handler.go 's/return strings.EqualFold(u.Host, r.Host)/XX/; s/^\t\tif origin == "" {$/\t\tif true {/'
m M3-ignore-allowlist handlers/websocket_handler.go 's/if strings.EqualFold(a, origin) {/if false \&\& strings.EqualFold(a, origin) {/'
m M4-empty-origin-denied handlers/websocket_handler.go '/if origin == "" {/{n;s/return true/return false/}'
m M5-drop-same-host handlers/websocket_handler.go 's/if strings.EqualFold(u.Host, r.Host) {/if false {/'
m M6-drop-session-check middleware/auth.go 's/if m.sessionCheck != nil {/if false \&\& m.sessionCheck != nil {/'
m M7-allowlist-substring handlers/websocket_handler.go 's/strings.EqualFold(a, origin)/strings.Contains(origin, a)/'
m M8-raw-route-in-main main.go 's/registerWebSocketRoute(router, jwtMiddleware, wsHandler)/router.GET("\/ws", wsHandler.HandleConnection)/'
