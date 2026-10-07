#!/usr/bin/env bash
# keyring.sh - T005b: the kernel keyring as a CACHE of the driver secret. The durable source of truth is the mode-0600 file
# ${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/<checkout-id>/build_hmac.key (outside the checkout, its sha256 recorded at creation); the session keyring
# (@s) entry `catalogizer:build_hmac:<checkout-id>` only avoids nothing durable: a reboot or a logout empties it and it is REFILLED from the file.
#   keyring.sh refill <statedir> <checkout>   verify the file (event_core.sh derive-key, which reads and validates the secret exactly as a pump does) and (re)load the
#                                             keyring entry from it when it is absent or differs; exit 20 `REFUSED reason=driver_secret_lost` when the file is lost or
#                                             altered, and then the entry is REMOVED (a cache never outlives its file), never recreated from anything else
#   keyring.sh status <checkout>              prints present|absent, exit 0|1
# The secret never appears on a command line: it is passed to keyctl on stdin (`keyctl padd`). Honest limit: event_core.sh derive-key (the one consumer today) reads the
# FILE, so no current consumer reads the cache; the cache exists so that a later consumer can read it and so that the refill after a reboot is exercised and proven.
# Needs keyctl (keyutils); without it: exit 21 `keyctl_absent`.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
EC="$here/../event_core.sh"
refuse() { printf 'REFUSED reason=%s %s\n' "$1" "${2:-}" >&2; exit "${3:-20}"; }
command -v keyctl >/dev/null 2>&1 || refuse keyctl_absent "" 21
desc() { printf 'catalogizer:build_hmac:%s' "$(printf '%s' "$1" | sha256sum | cut -d' ' -f1)"; }
find_key() { keyctl search @s user "$1" 2>/dev/null; }
case "${1:-}" in
  refill)
    [ $# -eq 3 ] || exit 2
    sd=$2; co=$3; d=$(desc "$co"); f="$sd/build_hmac.key"
    # the same read-and-validate a pump does: a lost or altered file fails here
    if ! bash "$EC" derive-key "$sd" keyring-probe keyring-probe >/dev/null 2>&1 || [ ! -s "$f" ]; then
      id=$(find_key "$d") && keyctl unlink "$id" @s >/dev/null 2>&1
      refuse driver_secret_lost "the secret file is lost or altered; the keyring cache entry was removed"
    fi
    want=$(cat "$f"); id=$(find_key "$d")
    if [ -n "$id" ] && [ "$(keyctl pipe "$id" 2>/dev/null)" = "$want" ]; then echo "keyring current"; exit 0; fi
    [ -z "$id" ] || keyctl unlink "$id" @s >/dev/null 2>&1
    printf '%s' "$want" | keyctl padd user "$d" @s >/dev/null || refuse keyring_write_failed ""
    echo "keyring refilled";;
  status)
    [ $# -eq 2 ] || exit 2
    if [ -n "$(find_key "$(desc "$2")")" ]; then echo present; else echo absent; exit 1; fi;;
  *) echo "usage: $0 refill <statedir> <checkout> | status <checkout>" >&2; exit 2;;
esac
