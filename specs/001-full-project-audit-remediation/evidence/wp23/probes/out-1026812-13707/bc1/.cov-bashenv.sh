# written by bash-coverage.sh: every non-interactive bash that inherits BASH_ENV traces itself to its own descriptor
PS4='+COV:${BASH_SOURCE##*/}:${LINENO}:'
exec {COV_FD}>>"$COV_TRACE_FILE"
BASH_XTRACEFD=$COV_FD
set -x
