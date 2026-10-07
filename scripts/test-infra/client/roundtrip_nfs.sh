#!/usr/bin/env bash
# roundtrip_nfs.sh - T134 (inside IMG-INFRA-CLIENT): NFSv3 round trip through the libnfs user-space utilities (no kernel mount): write files of several sizes (0 bytes,
# 1 byte, 20000 bytes, 1 MiB, a unicode name), list them with their sizes, read them back and compare sha256, check that a second CREATE of an existing name is refused
# (NFS3ERR_EXIST) with the original content intact, and confirm that
# a mount of an unexported path is refused. The server is unfsd 0.11.0; its export is a tmpfs of the container.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of NFS nfs)"; IP="$(getent hosts "$H" | awk '{ print $1; exit }')"; N="$(nonce)"
U="nfs://$IP/export"
mk() { head -c "$2" /dev/urandom >"/tmp/$1"; sha256sum "/tmp/$1" | cut -d' ' -f1; }
put_get() { # put_get <local name> <size> <remote name>: write, read back, compare sha256
  local want; want="$(mk "$1" "$2")"; nfs-cp "/tmp/$1" "$U/$3" >/dev/null 2>&1 || { echo "nfs-cp of $3 failed"; return 1; }
  expect_eq "$(nfs-cat "$U/$3" 2>/dev/null | sha256sum | cut -d' ' -f1)" "$want"; }
size_is() { local line; line="$(nfs-ls "$U" 2>/dev/null | awk -v n="$1" '{ if ($NF == n) print $5 }' | head -1)"; expect_eq "$line" "$2"; }
dup_create_refused() { local o; mk ow2.bin 3000 >/dev/null; o="$(nfs-cp /tmp/ow2.bin "$U/ow-$N.bin" 2>&1)" && { echo "a second create of an existing name was accepted"; return 1; }; case "$o" in *NFS3ERR_EXIST*) ;; *) echo "refused, but not with NFS3ERR_EXIST: ${o:0:100}"; return 1;; esac
  expect_eq "$(nfs-cat "$U/ow-$N.bin" | sha256sum | cut -d' ' -f1)" "$(sha256sum /tmp/ow1.bin | cut -d' ' -f1)"; }
unexported() { local o; o="$(timeout 20 nfs-ls "nfs://$IP/no-such-export" 2>&1)"; case "$o" in *MNT3ERR_*) return 0;; *) echo "not refused by the server's MOUNT protocol (no MNT3ERR_*): ${o:0:120}"; return 1;; esac; }   # WF12 F8: an unreachable server also "fails"
step resolve_server test -n "$IP"
step write_read_empty put_get e.bin 0 "empty-$N.bin"
step write_read_one_byte put_get b.bin 1 "one-$N.bin"
step write_read_20000 put_get m.bin 20000 "mid-$N.bin"
step write_read_1mib put_get l.bin 1048576 "big-$N.bin"
step write_read_unicode_name put_get u.bin 5000 $'für-Élise-千-'"$N.bin"
step listing_shows_size size_is "mid-$N.bin" 20000
step listing_shows_1mib size_is "big-$N.bin" 1048576
write_original() { mk ow1.bin 3000 >/dev/null; nfs-cp /tmp/ow1.bin "$U/ow-$N.bin" >/dev/null 2>&1; }
step write_original write_original
step second_create_refused_content_intact dup_create_refused
step unexported_path_refused unexported
finish nfs
