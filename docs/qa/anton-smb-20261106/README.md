# Anton SMB live target (CAT-SYN-LIVE-1 substitute)
Target: SMB server on host `Anton.local` (= this machine, 192.168.1.115, Samba 4.23.6) - real SMB, but self-hosted loopback-class, NOT an external server.
Shares: Public (rw, /home/milosvasic/Share), Music (ro for our user: mkdir ACCESS_DENIED), Projects (ro: mkdir ACCESS_DENIED).
Writes: only new top-level dir `Testing` on share `Public` (did not exist before). Pre-existing Public content (Misc, Incoming, .DS_Store) untouched.
Corpus: 14 files, 5.4 MB; manifest.sha256. Round-trip: 3 download runs, sha256 identical to manifest (download_run{1,2,3}.sha256).
No credentials in this directory.
