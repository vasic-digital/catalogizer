#!/usr/bin/env bash
# helixqa_llmctl_go_test.sh - run the HelixQA llmctl provider Go tests inside the pinned rootless IMG-GO container.
#
# Usage: helixqa_llmctl_go_test.sh [--red] [--count N] [--run REGEX] [--log FILE]
#   --red       copy ONLY the *_test.go files (no implementation): the expected RED (build fails) baseline.
#   --count N   go test -count (default 1)
#   --run RE    go test -run
#   --log FILE  absolute log path (default $PWD/.audit/scratch/llmctl/test.log)
# Why a scratch module: submodules/helix_qa/go.mod requires go 1.26 while IMG-GO is 1.25, and the packages under
# test (pkg/llmctl, pkg/llm/llmctl_provider*.go + provider.go + providers_registry.go) are standard-library only, so they are copied into
# /out/w with a go 1.25 stub go.mod (module path digital.vasic.helixqa kept). Nothing outside /out is written.
# Host safety: one container, GOMAXPROCS=1, go -p 1, RUNP_MEMORY <= 3 GiB.
set -u
cd "$(dirname "$0")/../.." || exit 9
RED=0; COUNT=1; RUN=""; LOG="$PWD/.audit/scratch/llmctl/test.log"
while [ $# -gt 0 ]; do
  case "$1" in
    --red) RED=1;; --count) COUNT=$2; shift;; --run) RUN=$2; shift;; --log) LOG=$2; shift;;
    *) echo "usage: see header" >&2; exit 2;;
  esac; shift
done
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1 RUNP_MEMORY=${RUNP_MEMORY:-3221225472}
OUT=$PWD/.audit/scratch/llmctl/out; mkdir -p "$OUT" "$(dirname "$LOG")"
H=/src/submodules/helix_qa
if [ "$RED" = 1 ]; then
  CP="mkdir -p w/pkg/llmctl w/pkg/llm && cp $H/pkg/llmctl/*_test.go w/pkg/llmctl/ && cp $H/pkg/llm/llmctl_provider_test.go w/pkg/llm/"
else
  CP="mkdir -p w/pkg/llmctl w/pkg/llm && cp $H/pkg/llmctl/*.go w/pkg/llmctl/ && cp $H/pkg/llm/provider.go $H/pkg/llm/providers_registry.go $H/pkg/llm/llmctl_provider.go $H/pkg/llm/llmctl_provider_test.go w/pkg/llm/"
fi
RUNARG=""; [ -n "$RUN" ] && RUNARG="-run '$RUN'"
bash scripts/containers/run_pinned.sh --out "$OUT" IMG-GO -- bash -c "rm -rf /out/w; cd /out && $CP && cd w && printf 'module digital.vasic.helixqa\ngo 1.25\n' > go.mod && export GOTOOLCHAIN=local HOME=/out/home GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=1 GOFLAGS=-mod=mod CGO_ENABLED=0 && go vet ./... && go test -p 1 -count=$COUNT $RUNARG -v ./... 2>&1; echo go_rc=\$?" > "$LOG" 2>&1
grep -E "^(--- |FAIL|ok|PASS|go_rc|#|.*cannot|.*undefined)" "$LOG" | tail -80
