# IMG-SIGVERIFY

`cosign` v3.1.3 for verifying publisher signatures of scanner images before their digests are recorded (T149, docs/16 section 7.1 step 5). Class `interpreter`: it reads signatures and registry metadata and compiles nothing. Run through `scripts/containers/run_pinned.sh IMG-SIGVERIFY -- cosign ...` with network access to the registries only.
The binary is checked against its SHA-256 in the `Containerfile`; the checksum equals the release's own `cosign_checksums.txt` (both read from GitHub on 2026-10-06), which makes it a pin of the bytes seen plus the publisher's published value, not a verified signature of the binary itself (UNCONFIRMED: the release's keyless signature of the checksums file was not verified, there is no earlier trusted cosign to do it).
