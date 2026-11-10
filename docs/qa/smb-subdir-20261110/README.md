# SMB sub-directory storage roots (CAT-SMB-SUBDIR-1)
A storage root of protocol `smb` may now name `share/sub/dir` in its `path`; scanning, listing, download and copy are confined to that subtree.
Live check: `./run.sh <0600 env file>` (reads ANTON_HOST/ANTON_USER/ANTON_PASS by name) against share `Public`, subdir `Testing` on the host Samba.
Asserts: plain `Public` still lists `Testing`; `Public/Testing` lists the corpus and not the share root; `..`, `../..`, backslash and mixed traversal are clamped to the sub root; names outside the subtree are unreachable; `docs/book.txt` sha256 equals the corpus manifest.
Not executed in CI. Result of the run (if any) is recorded in the commit message / hand-off, not here.
