#!/usr/bin/env bash
# test_wf22_run_go.sh - drives scripts/wf22/run_go.sh through its real invocation path with a shim for the
# container runner (run_pinned.sh) that executes the inner `bash -c` string, and a recording fake `go`.
# Unit-class evidence (the container runtime is a shim). Asserts: the inner command really runs go (a go_rc= line),
# the cgo env reaches go with -w kept and the HAVE_USLEEP define present via CGO_CPPFLAGS (CGO_CFLAGS stays free of
# -O2/-g replacement), and a failing go is reported (go_rc=3), not swallowed.
set -u
SRC=$(cd "$(dirname "$0")/../../.." && pwd)
T=$(mktemp -d) || exit 2
trap 'rm -rf "${T:?}"' EXIT
mkdir -p "$T/scripts/wf22" "$T/scripts/containers" "$T/catalog-api" "$T/bin"
cp "${WF22_RUN_GO:-$SRC/scripts/wf22/run_go.sh}" "$T/scripts/wf22/run_go.sh"
cat > "$T/scripts/containers/run_pinned.sh" <<'SH'
#!/usr/bin/env bash
# shim: run_pinned.sh --out DIR IMG -- bash -c STRING  -> run STRING locally
while [ "$1" != "--" ]; do shift; done; shift; exec "$@"
SH
cat > "$T/bin/go" <<'SH'
#!/usr/bin/env bash
echo "FAKEGO args=$* CGO_CFLAGS=[${CGO_CFLAGS-unset}] CGO_CPPFLAGS=[${CGO_CPPFLAGS-unset}]"
exit "${FAKE_GO_RC:-0}"
SH
chmod +x "$T/scripts/containers/run_pinned.sh" "$T/bin/go"
pass=0; fail=0
chk() { if [ "$2" = ok ]; then pass=$((pass+1)); echo "PASS: $1"; else fail=$((fail+1)); echo "FAIL: $1"; fi; }
run() { PATH="$T/bin:$PATH" HOME="$T/home" FAKE_GO_RC=$1 bash "$T/scripts/wf22/run_go.sh" "$T/log$1" 'test ./x' 20 >"$T/out$1" 2>&1; }
run 0; L=$(cat "$T/log0" 2>/dev/null)
case "$L" in *go_rc=0*) chk "a go command runs and go_rc=0 is logged" ok;; *) chk "a go command runs and go_rc=0 is logged (log: $L)" no;; esac
case "$L" in *"FAKEGO args=test ./x"*) chk "go received its arguments" ok;; *) chk "go received its arguments" no;; esac
case "$L" in *"CGO_CFLAGS=[-w]"*) chk "CGO_CFLAGS keeps -w only (default -O2 -g not replaced)" ok;; *) chk "CGO_CFLAGS keeps -w only" no;; esac
case "$L" in *"CGO_CPPFLAGS=[-DHAVE_USLEEP=1]"*) chk "CGO_CPPFLAGS carries -DHAVE_USLEEP=1" ok;; *) chk "CGO_CPPFLAGS carries -DHAVE_USLEEP=1" no;; esac
run 3; L=$(cat "$T/log3" 2>/dev/null)
case "$L" in *go_rc=3*) chk "a failing go is reported as go_rc=3" ok;; *) chk "a failing go is reported as go_rc=3" no;; esac
echo "RESULT pass=$pass fail=$fail"; [ "$fail" = 0 ]
