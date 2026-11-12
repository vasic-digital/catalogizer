#!/bin/bash
# round-3 mutants (reviewer R23/R24); run from catalog-api.
set -u
run() { systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 nice -n 10 env GOMAXPROCS=2 go test -p 1 -parallel 2 -count=1 "$@" 2>&1; }
m() { local id=$1 f=$2 e=$3 bak; bak=$(mktemp "${TMPDIR:-/tmp}/mut.XXXXXX")
  cp "$f" "$bak"; sed -i "$e" "$f"
  if cmp -s "$f" "$bak"; then echo "$id NOT-APPLIED"; rm -f "$bak"; return; fi
  out=$(run ./handlers -run 'OriginChecker')
  if echo "$out" | grep -q '^FAIL\|--- FAIL\|build failed'; then r=KILLED; else r=SURVIVED; fi
  cp "$bak" "$f"; cmp -s "$f" "$bak" && c=restored-cmp-ok || c=RESTORE-MISMATCH
  rm -f "${bak:?}"; echo "$id $r ($c)"; }
H=handlers/websocket_handler.go
m R23-trust-x-forwarded-host $H 's|url.Parse("//" + r.Host)|url.Parse("//" + func() string { if f := r.Header.Get("X-Forwarded-Host"); f != "" { return f }; return r.Host }())|'
m R24-no-host-lowercase $H 's/return scheme, strings.ToLower(u.Hostname()), port, explicit, true/return scheme, u.Hostname(), port, explicit, true/'
echo END
