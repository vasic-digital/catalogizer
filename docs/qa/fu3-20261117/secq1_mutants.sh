#!/bin/bash
# usage: secq1_mutants.sh <before|after>  (run from the scratch clone root). Mutates one production line at a time, runs the request-path suite.
set -u
PHASE=$1; OUT=docs/qa/fu3-20261117/secq1_mutants_$PHASE.txt; : > $OUT
mut() { # name file old new
  local f=$2; local orig; orig=$(mktemp); cp "$f" "$orig"
  python3 - "$f" "$3" "$4" <<'PY'
import sys
f,old,new=sys.argv[1:4]
s=open(f).read(); assert s.count(old)==1,(old,s.count(old)); open(f,"w").write(s.replace(old,new))
PY
  echo "## mutant $1" >>$OUT
  (cd catalog-api && GOMAXPROCS=2 nice -n 10 go test -p 1 -timeout 60m -count=1 ./handlers -run 'TestSECQ1_Live' 2>&1 | grep -E '^(--- FAIL|ok|FAIL)') >>$OUT
  cp "$orig" "$f"
}
mut A-create-check-disabled catalog-api/handlers/scan_handler.go 'if _, err := netguard.CheckOperatorURL(ctx, target, "http", "https"); err != nil {' 'if _, err := netguard.CheckOperatorURL(ctx, target, "http", "https"); err != nil && false {'
mut B-redirect-check-removed catalog-api/filesystem/webdav_client.go 'CheckRedirect: netguard.OperatorCheckRedirectFor(effectiveClass(approved)), ' ''
mut C-approved-class-ignored catalog-api/filesystem/storage_host.go 'if c == netguard.ClassPublic {
		return netguard.ClassPublic
	}
	return netguard.ClassPrivate' 'return netguard.ClassPrivate'
mut D-client-unguarded-plain-http catalog-api/filesystem/webdav_client.go 'return &http.Client{CheckRedirect: netguard.OperatorCheckRedirectFor(effectiveClass(approved)), Transport: operatorStorageTransportFor(approved)}' 'return &http.Client{}'
echo done >>$OUT
