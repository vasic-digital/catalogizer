Live SMB target: samba smbd 4.23.6 run rootless on 127.0.0.1:4450 with a private tdbsam, share in a scratch dir (credentials in a 0600 file, never recorded).
Run: SMB_LIVE_HOST=127.0.0.1 SMB_LIVE_PORT=4450 SMB_LIVE_USER=.. SMB_LIVE_PASS=.. go test -tags live_smb -run TestLiveSMBCopy ./filesystem
