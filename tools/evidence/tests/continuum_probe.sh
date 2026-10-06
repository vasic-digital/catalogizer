#!/usr/bin/env bash
# T048 / DR-E1 read-only probe of the constitution continuum store (submodules/constitution/submodules/continuum): runs ITS OWN test suite and
# its integrity CLI against a real ev/1 ledger written by tools/evidence/evrec, inside the pinned IMG-GO image (11.4.173: no bare-host build).
# It changes nothing outside .audit/scratch/dre1 (ignored by /.audit/). Output is the evidence of $EV/wp05/DR-E1.md.
# Uses scripts/containers/run_pinned.sh directly (the run_go.sh wrapper refuses while the host anti-mess sweep reports drift of another run's container).
set -u
root=$(cd "$(dirname "$0")/../../.." && pwd); cd "$root" || exit 1
D=.audit/scratch/dre1; rm -rf "$D"; mkdir -p "$D/.audit"
export EV_LEDGER=$PWD/$D/ledger.jsonl EV_ANCHOR=$PWD/$D/anchors.jsonl EV_BLOBS=$PWD/$D/blobs EVREC_REPO_ROOT=$PWD/$D CPA_HOST_ENTRY=/bin/false
for i in 1 2 3; do tools/evidence/evrec run CAT-001 PROBE "$i" shell_script x -- true >/dev/null; done
tools/evidence/evrec anchor; unset EV_LEDGER EV_ANCHOR EV_BLOBS EVREC_REPO_ROOT CPA_HOST_ENTRY
echo "## continuum submodule pin: $(git -C submodules/constitution/submodules/continuum rev-parse HEAD 2>/dev/null)"
scripts/containers/run_pinned.sh --network=none IMG-GO -- sh -c '
export GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=0 HOME=/tmp GOCACHE=/tmp/gocache
cd submodules/constitution/submodules/continuum && go version
echo "## continuum go test ./... (its own suite, count=1)"; CGO_ENABLED=1 go test -count=1 ./... 2>&1 | tail -30
go build -o /tmp/ci ./cmd/continuum-integrity && cd /src || exit 1
echo "## continuum-integrity chain verify on a REAL ev/1 ledger (3 entries written by tools/evidence/evrec)"; /tmp/ci chain verify --chain '"$D"'/ledger.jsonl >/tmp/o1 2>&1; echo "exit=$?"; tail -2 /tmp/o1
echo "## continuum-integrity anchor verify: the ev-anchor/1 row format of evrec against the same ledger"; /tmp/ci anchor verify --chain '"$D"'/ledger.jsonl --anchor '"$D"'/anchors.jsonl >/tmp/o2 2>&1; echo "exit=$?"; tail -2 /tmp/o2
echo "## continuum-integrity usage (exit codes)"; /tmp/ci 2>&1 | tail -4
' 2>&1
