# IMG-KCOV

kcov v43 built from the upstream source tarball on top of IMG-TESTUTIL (shell coverage, constitution 11.4.224; the upstream kcov image lacks git, jq and python3).
`FROM ${TESTUTIL_REF}` is supplied by the builder from the IMG-TESTUTIL entry of `build/containers/images.lock.yaml`. The tarball is verified by SHA-256 in the `Containerfile`;
the publisher publishes no checksum, so the value is a pin of the bytes seen (`signature_status: UNKNOWN`). Built by the T006 procedure (evidence `wp09/build-kcov.txt`); run through `scripts/containers/run_pinned.sh IMG-KCOV -- <command>`.
This README and `digests.lock` were added by T106; the `Containerfile` is T006's, unchanged.
