#!/bin/bash
# Mutation matrix for ALTSVC4. Run from the clone root. Each mutant is applied to the production
# sources, the test binary is compiled once, then the PRE-EXISTING tests (survival run) and the NEW
# ALTSVC4 tests (kill run) are executed against it.
set -u
cd "${ROOT:?}/catalog-api"
OLD='TestAltSvcStartH3Stack_(CertErrorNotAdvertised|UDPBindFailureNotAdvertised|AdvertisedOnlyAfterConfirmedBind)$|TestAltSvcRealSocket|TestAltSvcDocumentedExceptions|TestAltSvcReachOnRealRouter|TestAltSvcNotAdvertised|TestAltSvcMiddleware$|TestMainPackageAuthSurfaceClosed|TestAuthSurfaceGuard_'
NEW='TestAltSvcStartH3Stack_(ServeFailureClearsFlag|NormalClose|TCPBindFailure|BothBindsFail|UDPSocketReleased)|TestAltSvcRealHTTP3Request|TestAltSvcAdvertisementAndWiringPins'
cp main_h3.go "${TMPD:?}/main_h3.go.orig"; cp main.go "${TMPD}/main.go.orig"
mut() { # name file python-expr-old python-new
  python3 - "$2" "$3" "$4" <<'P'
import sys
f,a,b=sys.argv[1:]
s=open(f).read()
assert a in s,"mutation target missing"
open(f,'w').write(s.replace(a,b,1))
P
}
run() {
  name=$1
  GOMAXPROCS=2 nice -n 10 go test -c -o "$TMPD/t.bin" . >"$TMPD/build_$name.txt" 2>&1 || { echo "$name BUILD-FAIL"; return; }
  "$TMPD/t.bin" -test.count=1 -test.run "$OLD" >"$TMPD/old_$name.txt" 2>&1; o=$?
  "$TMPD/t.bin" -test.count=1 -test.run "$NEW" >"$TMPD/new_$name.txt" 2>&1; n=$?
  echo "$name old_suite_rc=$o (0=mutant SURVIVES old tests) new_suite_rc=$n (1=KILLED by new tests)"
}
restore() { cp "$TMPD/main_h3.go.orig" main_h3.go; cp "$TMPD/main.go.orig" main.go; }
run BASELINE_R2
mut M1 main_h3.go $'\t\t\th3Advertised.Store(false)\n' ''; run M1; restore
mut M5 main_h3.go $'zap.Error(err))\n\t} else {' $'zap.Error(err))\n\t\treturn nil, nil\n\t} else {'; run M5; restore
mut M6b main_h3.go 'http3Server := &http3.Server{Addr: addr, Handler: router,' 'http3Server := &http3.Server{Addr: addr, Handler: http.NotFoundHandler(),'; run M6b; restore
mut M7 main_h3.go $'\th3Advertised.Store(true) // after the bind, before serving; cleared if the server fails\n' ''; mut M7 main_h3.go $'\tudpConn, err := net.ListenPacket' $'\th3Advertised.Store(true)\n\tudpConn, err := net.ListenPacket'; run M7; restore
mut M2 main.go 'httpsServer, http3Server := startH3Stack(logger, srv.Handler, cfg.Server.Host, h3Port, getOrCreateSelfSignedCert)' $'_, _ = startH3Stack(logger, srv.Handler, cfg.Server.Host, h3Port, getOrCreateSelfSignedCert)\n\tvar httpsServer *http.Server\n\tvar http3Server *http3.Server'
grep -q 'quic-go/quic-go/http3' main.go || sed -i '0,/^import (/s//import (\n\t"github.com\/quic-go\/quic-go\/http3"/' main.go
run M2; restore
mut N1 main_h3.go $'\t\tdefer udpConn.Close() // Close/Shutdown of http3Server does not close a socket passed to Serve\n' ''; run N1; restore
