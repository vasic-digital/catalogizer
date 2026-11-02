# cgo_flags.sh - the ONE definition of the cgo preprocessor define every catalog-api go build/test/vet/run needs.
# Operator decision 2026-11-02: go-sqlcipher is compiled with -DHAVE_USLEEP=1 so SQLite sleeps in milliseconds.
# CGO_CPPFLAGS (not CGO_CFLAGS) is used on purpose: setting CGO_CFLAGS replaces go's default "-O2 -g".
# Append-safe and idempotent: a caller's CGO_CPPFLAGS is preserved, and sourcing twice adds the define once.
# Source it:  . "<repo>/scripts/lib/cgo_flags.sh"      (guarded by scripts/gates/cm_cgo_usleep_flag.sh)
case " ${CGO_CPPFLAGS:-} " in
  *" -DHAVE_USLEEP=1 "*) ;;
  *) CGO_CPPFLAGS="${CGO_CPPFLAGS:+$CGO_CPPFLAGS }-DHAVE_USLEEP=1" ;;
esac
export CGO_CPPFLAGS
