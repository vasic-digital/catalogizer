#!/bin/bash
set -u
cd /home/milosvasic/.cache/claude-tmp/altsvc5/catalog-api
export GOMAXPROCS=2
D=../docs/qa/altsvc5-20261117
cp main_h3.go /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed
run() { # name
  go test -race -c -o ../../altsvc5mut.test . >$D/mutant-$1-build.txt 2>&1 || { echo "$1 BUILD-FAIL"; return; }
  for i in 1 2 3; do nice -n 10 ../../altsvc5mut.test -test.run 'TestAltSvc5' -test.count=1 >$D/mutant-$1-run$i.txt 2>&1; echo "$1 run$i rc=$? races=$(grep -c 'DATA RACE' $D/mutant-$1-run$i.txt) $(grep -E '^--- FAIL' $D/mutant-$1-run$i.txt|tr '\n' ' ')"; done
}
# M1: revert to shared config
cp /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed main_h3.go
sed -i 's/httpsTLS := tlsConfig.Clone()/httpsTLS := tlsConfig/; s/h3TLS := tlsConfig.Clone()/h3TLS := tlsConfig/' main_h3.go; run M1_shared
# M2: clone only the https server
cp /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed main_h3.go
sed -i 's/h3TLS := tlsConfig.Clone()/h3TLS := tlsConfig/' main_h3.go; run M2_clone_only_https
# M3: clone only the h3 server
cp /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed main_h3.go
sed -i 's/httpsTLS := tlsConfig.Clone()/httpsTLS := tlsConfig/' main_h3.go; run M3_clone_only_h3
# M4: clone h3 late (after servers start), https keeps original
cp /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed main_h3.go
sed -i 's/h3TLS := tlsConfig.Clone()/h3TLS := tlsConfig/; s/httpsTLS := tlsConfig.Clone()/httpsTLS := tlsConfig/' main_h3.go
sed -i 's/http3Server := &http3.Server{Addr: addr, Handler: router, TLSConfig: h3TLS}/http3Server := \&http3.Server{Addr: addr, Handler: router, TLSConfig: tlsConfig.Clone()}/' main_h3.go; run M4_clone_late
# M5: both servers share ONE clone (distinct from original, but still shared)
cp /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed main_h3.go
sed -i 's/h3TLS := tlsConfig.Clone()/h3TLS := httpsTLS/' main_h3.go; run M5_shared_clone
cp /home/milosvasic/.cache/claude-tmp/main_h3.go.fixed main_h3.go
echo restored; git diff --stat main_h3.go | tail -1
