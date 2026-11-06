# Catalogizer live test servers (host anton, 192.168.1.115)

All serve the SAME corpus `/home/milosvasic/Testing-servers/corpus` (manifest.sha256, 12 files incl. unicode/space names, 0-byte, 5 MB, wav/mp3/mp4/srt/jpg/png/pdf/txt). The corpus is mounted read-only; only `rw/` is writable (host dir mode 777, dedicated test dir). Credentials: user `catsvc`, password in the 0600 file `/home/milosvasic/.cache/claude-tmp/catqa3/testservers.env` (TS_USER/TS_PASS/TS_HOST). HTTP and NFS are anonymous read-only.

| Protocol | Endpoint | Server | Catalogizer config |
|---|---|---|---|
| SMB | `192.168.1.115:1445`, share `corpus` (445 is the host's own samba, untouched) | samba 4.19 (container) | host `192.168.1.115:1445` (client does SplitHostPort), share `corpus`, `smb://192.168.1.115:1445/corpus` |
| FTP | `ftp://192.168.1.115:42021` pasv 42100-42110 | rclone serve ftp | host/port, user/pass |
| FTPS explicit (AUTH TLS) | `ftp://192.168.1.115:42022` pasv 42111-42120, self-signed cert (`tls/cert.pem`, SAN anton.local + IP) | pyftpdlib TLS | TLS explicit, skip-verify (or trust the cert) |
| SFTP | `sftp://192.168.1.115:42023` fixed host key `tls/ssh_host_ed25519` | rclone serve sftp | user/pass |
| WebDAV | `http://192.168.1.115:42080/` basic auth | rclone serve webdav | url + user/pass |
| HTTP static | `http://192.168.1.115:42081/` anonymous RO | rclone serve http | |
| NFS v3 | `192.168.1.115:/` port=42049,mountport=42049 anonymous RO | rclone serve nfs (userspace) | Catalogizer NFS client mounts via the kernel (needs root): options `ro,nfsvers=3,tcp,nolock,port=42049,mountport=42049`, or point `mount_point` at a pre-mounted dir |

Scripts: `up.sh` (idempotent), `down.sh`, `verify.sh` (3 runs x 12 files sha256 vs manifest per protocol + wrong-password and write controls; `TS_NFS_MNT=<premounted dir>` adds NFS). Ports: `ports.env`. Containers are rootless podman (`cattsrv-*`, host network, `--restart on-failure:3`; not reboot-persistent, rerun up.sh).
