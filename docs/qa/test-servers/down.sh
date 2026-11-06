#!/usr/bin/env bash
set -euo pipefail
for n in smb ftp ftps sftp webdav http nfs; do podman rm -f "cattsrv-$n" >/dev/null 2>&1 || true; done
echo stopped
